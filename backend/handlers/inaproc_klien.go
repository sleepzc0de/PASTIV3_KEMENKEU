package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"pasti-v3-backend/config"
	"pasti-v3-backend/utils"
)

// Klien tunggal ke API Inaproc. Semua permintaan lewat panggilInaproc, yang:
//   - menunggu giliran pada pembatas bersama (batas per menit, kuota per jam, dan jeda bersama setelah 429; lihat inaproc_batas.go);
//   - memakai ulang koneksi (satu http.Client per timeout), bukan membuat koneksi TLS baru untuk tiap halaman;
//   - mencoba ulang gangguan sementara per halaman (galat jaringan dan 5xx tiga kali dengan jeda bertambah; 429 empat kali dengan menunggu
//     jeda bersama) sehingga satu halaman yang gagal tidak menggugurkan seluruh penarikan dan penarikan dilanjutkan dari halaman itu.
//
// Permintaan interaktif (tampilan langsung di halaman lama) tidak boleh menggantung: satu kali coba, menunggu giliran paling lama beberapa
// detik, dan bila kuota sedang habis dijawab 429 sintetis yang diteruskan apa adanya ke pengguna.

const (
	maksCoba5xx      = 3
	maksCoba429      = 4
	tungguInteraktif = 3 * time.Second
)

var jedaCoba5xx = []time.Duration{3 * time.Second, 10 * time.Second}

var (
	klienMu sync.Mutex
	klien   = map[time.Duration]*http.Client{}
)

// klienInaproc: http.Client yang dipakai ulang untuk timeout yang sama.
func klienInaproc() *http.Client {
	timeout := time.Duration(config.Cfg.HTTPClientTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	klienMu.Lock()
	defer klienMu.Unlock()
	if c, ok := klien[timeout]; ok {
		return c
	}
	c := utils.NewSSOHTTPClient()
	klien[timeout] = c
	return c
}

// bacaRetryAfter membaca header Retry-After (detik atau tanggal HTTP); 0 bila tidak ada atau tidak terbaca.
func bacaRetryAfter(h http.Header, sekarang time.Time) time.Duration {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if n, err := strconv.Atoi(v); err == nil {
		if n < 0 {
			return 0
		}
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(sekarang); d > 0 {
			return d
		}
	}
	return 0
}

func galatTerlaluSering(d time.Duration) []byte {
	pesan := "Kuota permintaan Inaproc sedang habis; coba lagi sekitar " + bulatkan(d).String() + " lagi"
	return []byte(fmt.Sprintf(`{"success":false,"message":%q,"error":{"code":"Too Many Requests","message":%q}}`, pesan, pesan))
}

// panggilInaproc: lihat komentar paket di atas.
func panggilInaproc(ctx context.Context, path string, params url.Values, interaktif bool) ([]byte, int, error) {
	cfg := config.Cfg
	reqURL := fmt.Sprintf("%s%s?%s", cfg.InaprocBaseURL, path, params.Encode())
	b := batas()
	c := klienInaproc()

	var coba5xx, coba429 int
	for {
		var maks time.Duration
		if interaktif {
			maks = tungguInteraktif
		}
		if err := b.Tunggu(ctx, maks); err != nil {
			if errors.Is(err, ErrKuotaHabis) {
				b.mu.Lock()
				d := b.waktuTungguLocked(b.sekarang(), 1)
				b.mu.Unlock()
				return galatTerlaluSering(d), http.StatusTooManyRequests, nil
			}
			return nil, 0, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Authorization", "Bearer "+cfg.InaprocToken)
		req.Header.Set("Accept", "application/json")

		resp, err := utils.DoWithRetry(c, req, 2)
		var body []byte
		status := 0
		if err == nil {
			body, err = io.ReadAll(resp.Body)
			status = resp.StatusCode
			header := resp.Header
			resp.Body.Close()
			if err == nil {
				switch {
				case status == http.StatusTooManyRequests:
					d := b.Laporkan429(bacaRetryAfter(header, b.sekarang()))
					log.Printf("[INAPROC] 429 dari %s: semua permintaan ditahan %s", path, bulatkan(d))
					coba429++
					if interaktif || coba429 >= maksCoba429 {
						return body, status, nil
					}
					if f := kabarDariCtx(ctx); f != nil {
						f(fmt.Sprintf("Inaproc membatasi laju permintaan (429). Menunggu %s lalu mencoba lagi (percobaan %d dari %d)", bulatkan(d), coba429+1, maksCoba429))
					}
					continue // Tunggu di awal putaran menahan sampai jeda bersama selesai
				case status >= 500 && status != http.StatusNotImplemented:
					coba5xx++
					if interaktif || coba5xx >= maksCoba5xx {
						return body, status, nil
					}
					log.Printf("[INAPROC] %d dari %s; mencoba lagi (percobaan %d dari %d)", status, path, coba5xx+1, maksCoba5xx)
					if !b.tidurFn(ctx, jedaCoba5xx[coba5xx-1]) {
						return nil, 0, ctx.Err()
					}
					continue
				}
				if status == http.StatusOK {
					b.LaporkanBerhasil()
				}
				return body, status, nil
			}
		}

		// Galat jaringan atau pembacaan badan.
		if ctx.Err() != nil {
			return nil, 0, ctx.Err()
		}
		coba5xx++
		if interaktif || coba5xx >= maksCoba5xx {
			return nil, 0, err
		}
		log.Printf("[INAPROC] galat jaringan ke %s: %v; mencoba lagi (percobaan %d dari %d)", path, err, coba5xx+1, maksCoba5xx)
		if !b.tidurFn(ctx, jedaCoba5xx[coba5xx-1]) {
			return nil, 0, ctx.Err()
		}
	}
}

// callInaprocEndpoint: panggilan latar (sinkronisasi); menunggu giliran selama perlu.
func callInaprocEndpoint(path string, params url.Values) ([]byte, int, error) {
	return panggilInaproc(context.Background(), path, params, false)
}

// callInaprocEndpointCtx sama dengan callInaprocEndpoint, tetapi permintaan dan penantiannya ikut dibatalkan bila ctx berakhir (penarikan
// terjadwal/antrean yang bisa dibatalkan).
func callInaprocEndpointCtx(ctx context.Context, path string, params url.Values) ([]byte, int, error) {
	return panggilInaproc(ctx, path, params, false)
}

// callInaprocEndpointInteraktif: panggilan untuk tampilan langsung (permintaan pengguna): satu kali coba, tidak menggantung bila kuota habis.
func callInaprocEndpointInteraktif(path string, params url.Values) ([]byte, int, error) {
	return panggilInaproc(context.Background(), path, params, true)
}
