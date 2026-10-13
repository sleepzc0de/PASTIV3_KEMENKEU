package serverlog

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// SimpanFunc menulis sekelompok kejadian gabungan ke penyimpanan.
type SimpanFunc func(ctx context.Context, gs []Gabungan) error

// Perekam menampung kejadian di antrean berbatas, menggabungkan yang sama (sidik jari sama) di memori, lalu menuliskannya ke penyimpanan secara berkelompok di
// latar belakang. Mencatat tidak pernah menunggu dan tidak pernah memblokir permintaan: bila antrean penuh (badai galat atau database macet) kejadian dijatuhkan dan
// dihitung. Bila penulisan gagal, kelompok disimpan di memori (dibatasi maksTertunda kelompok) dan dicoba lagi dengan jeda yang makin panjang.
//
// Memori yang dipakai selalu dibatasi: antrean (kapasitas) + penampung (maksTertunda kelompok), berapa pun banyaknya galat.
type Perekam struct {
	simpan  SimpanFunc
	ch      chan Entri
	selesai chan struct{}
	mu      sync.RWMutex // melindungi penutupan antrean dari pengiriman yang bersamaan
	tutup   bool

	diterima, ditulis, dijatuhkan, gagal atomic.Int64
	tertunda                             atomic.Int64

	interval     time.Duration // seberapa sering menulis
	maksKelompok int           // kelompok per sekali tulis
	maksTertunda int           // kelompok berbeda yang boleh menunggu di memori
	lapor        func(string)  // pelaporan galat perekam sendiri (langsung ke stderr, tidak lewat log standar supaya tidak berputar)
}

// NewPerekam membuat perekam dengan antrean berkapasitas kapasitas kejadian. Mulai harus dipanggil agar kejadian mulai ditulis.
func NewPerekam(simpan SimpanFunc, kapasitas int) *Perekam {
	if kapasitas < 1 {
		kapasitas = 2048
	}
	return &Perekam{
		simpan:       simpan,
		ch:           make(chan Entri, kapasitas),
		selesai:      make(chan struct{}),
		interval:     5 * time.Second,
		maksKelompok: 200,
		maksTertunda: 1000,
		lapor: func(s string) {
			fmt.Fprintf(os.Stderr, "%s [SERVERLOG] %s\n", time.Now().Format("2006/01/02 15:04:05"), s)
		},
	}
}

// Mulai menjalankan penulis latar belakang.
func (p *Perekam) Mulai() { go p.jalankan() }

// Catat memasukkan kejadian ke antrean tanpa menunggu. Mengembalikan false bila dijatuhkan.
func (p *Perekam) Catat(e Entri) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.tutup {
		return false
	}
	select {
	case p.ch <- e:
		p.diterima.Add(1)
		return true
	default:
		p.dijatuhkan.Add(1)
		return false
	}
}

// Statistik: keadaan perekam saat ini.
func (p *Perekam) Statistik() Statistik {
	return Statistik{
		Diterima: p.diterima.Load(), Ditulis: p.ditulis.Load(), Dijatuhkan: p.dijatuhkan.Load(), Gagal: p.gagal.Load(),
		Antrean: len(p.ch), Kapasitas: cap(p.ch), Tertunda: int(p.tertunda.Load()),
	}
}

func (p *Perekam) jalankan() {
	defer close(p.selesai)
	tik := time.NewTicker(p.interval)
	defer tik.Stop()

	agregat := make(map[string]*Gabungan)
	var urutan []string // urutan kelompok pertama kali muncul, supaya yang paling lama ditulis lebih dulu

	tambah := func(e Entri) {
		sid := e.sidik
		if sid == "" {
			sid = Sidik(e.Level, e.Sumber, e.Rute, e.Pesan)
		}
		if g, ada := agregat[sid]; ada {
			g.Jumlah++
			if e.Waktu.After(g.Akhir) {
				g.Akhir = e.Waktu
			}
			// Sampel terbaru lebih berguna untuk ditelusuri.
			if e.RequestID != "" {
				g.RequestID = e.RequestID
			}
			if e.UserID != "" {
				g.UserID = e.UserID
			}
			return
		}
		if len(agregat) >= p.maksTertunda {
			p.dijatuhkan.Add(1)
			return
		}
		agregat[sid] = &Gabungan{Entri: e, Sidik: sid, Jumlah: 1, Akhir: e.Waktu}
		urutan = append(urutan, sid)
		p.tertunda.Store(int64(len(urutan)))
	}

	var (
		jeda        time.Duration
		tundaSampai time.Time
		laporTerakh time.Time
	)
	// tulisSatu menulis satu kelompok (paling banyak maksKelompok); false bila gagal.
	tulisSatu := func() bool {
		n := len(urutan)
		if n > p.maksKelompok {
			n = p.maksKelompok
		}
		kel := make([]Gabungan, 0, n)
		for _, sid := range urutan[:n] {
			kel = append(kel, *agregat[sid])
		}
		ctx, batal := context.WithTimeout(context.Background(), 20*time.Second)
		var err error
		if !Aman("SERVERLOG", func() { err = p.simpan(ctx, kel) }) {
			err = fmt.Errorf("panik saat menulis")
		}
		batal()
		if err != nil {
			p.gagal.Add(1)
			if jeda == 0 {
				jeda = 5 * time.Second
			} else if jeda < time.Minute {
				jeda *= 2
			}
			tundaSampai = time.Now().Add(jeda)
			if time.Since(laporTerakh) >= time.Minute {
				laporTerakh = time.Now()
				p.lapor(fmt.Sprintf("gagal menulis %d kelompok log ke database (dicoba lagi, %d kelompok menunggu): %v", len(kel), len(urutan), err))
			}
			return false
		}
		jeda = 0
		var total int64
		for _, g := range kel {
			total += int64(g.Jumlah)
			delete(agregat, g.Sidik)
		}
		urutan = urutan[n:]
		if len(urutan) == 0 {
			urutan = urutan[:0:0] // lepaskan larik lama
		}
		p.ditulis.Add(total)
		p.tertunda.Store(int64(len(urutan)))
		return true
	}
	siram := func(paksa bool) {
		if !paksa && time.Now().Before(tundaSampai) {
			return
		}
		for len(urutan) > 0 {
			if !tulisSatu() {
				return
			}
		}
	}

	for {
		select {
		case e, ok := <-p.ch:
			if !ok {
				siram(true)
				return
			}
			tambah(e)
		case <-tik.C:
			siram(false)
		}
	}
}

// Berhenti berhenti menerima kejadian baru, menuliskan sisa antrean, lalu selesai (atau menyerah bila ctx habis).
func (p *Perekam) Berhenti(ctx context.Context) {
	p.mu.Lock()
	if !p.tutup {
		p.tutup = true
		close(p.ch)
	}
	p.mu.Unlock()
	select {
	case <-p.selesai:
	case <-ctx.Done():
	}
}
