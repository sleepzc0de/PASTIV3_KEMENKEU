package middleware

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/serverlog"
)

// Ketahanan: middleware yang menjaga server tetap sehat saat banyak pengguna mengakses bersamaan. Semuanya memakai memori yang dibatasi dan tidak pernah memblokir tanpa
// batas waktu: lebih baik menolak sebagian permintaan dengan cepat (429/503 beserta Retry-After) daripada membuat semua permintaan melambat sampai server jatuh.

// KodeTerlaluBanyak dan KodeSibuk adalah nilai field "code" pada respons penolakan, supaya frontend bisa membedakannya.
const (
	KodeTerlaluBanyak = "terlalu_banyak_permintaan"
	KodeSibuk         = "server_sibuk"
)

// Penghitung yang dibaca halaman Log Server (ditampilkan sebagai tekanan beban); hanya bertambah, direset saat proses dimulai ulang.
var (
	ditolakLaju    atomic.Int64
	ditolakBeban   atomic.Int64
	ditolakBerat   atomic.Int64
	serentakSaat   atomic.Int64
	serentakPuncak atomic.Int64
	beratSaat      atomic.Int64
	beratAntri     atomic.Int64
)

// StatistikKetahanan: ringkasan tekanan beban sejak proses dimulai.
type StatistikKetahananData struct {
	DitolakLaju      int64 `json:"ditolak_laju"`      // 429: melebihi batas laju per IP
	DitolakBeban     int64 `json:"ditolak_beban"`     // 503: permintaan serentak melewati batas
	DitolakBerat     int64 `json:"ditolak_berat"`     // 503: antrean rute berat penuh
	SerentakSekarang int64 `json:"serentak_sekarang"` // permintaan yang sedang diproses
	SerentakPuncak   int64 `json:"serentak_puncak"`
	BeratSekarang    int64 `json:"berat_sekarang"`
	BeratAntri       int64 `json:"berat_antri"`
	MaksSerentak     int   `json:"maks_serentak"`
	MaksBerat        int   `json:"maks_berat"`
	LajuAktif        bool  `json:"laju_aktif"`
}

// StatistikKetahanan membaca penghitung di atas beserta batas yang berlaku.
func StatistikKetahanan() StatistikKetahananData {
	s := StatistikKetahananData{
		DitolakLaju: ditolakLaju.Load(), DitolakBeban: ditolakBeban.Load(), DitolakBerat: ditolakBerat.Load(),
		SerentakSekarang: serentakSaat.Load(), SerentakPuncak: serentakPuncak.Load(), BeratSekarang: beratSaat.Load(), BeratAntri: beratAntri.Load(),
	}
	if c := config.Cfg; c != nil {
		s.MaksSerentak, s.MaksBerat, s.LajuAktif = c.MaksPermintaanSerentak, c.MaksBeratSerentak, c.RateLimitAktif
	}
	return s
}

// jalurKesehatan: /health dan /health/ready (pemeriksa kesehatan) tidak dibatasi supaya tetap menjawab saat server sibuk.
func jalurKesehatan(p string) bool { return p == "/health" || strings.HasPrefix(p, "/health/") }

func tolak(c *gin.Context, status int, pesan, kode string, tunggu time.Duration) {
	detik := int(math.Ceil(tunggu.Seconds()))
	if detik < 1 {
		detik = 1
	}
	c.Header("Retry-After", strconv.Itoa(detik))
	c.AbortWithStatusJSON(status, gin.H{"success": false, "message": pesan, "code": kode})
}

// ---------------------------------------------------------------- pembatas laju per kunci (token bucket)

// PembatasLaju membatasi laju per kunci (biasanya IP klien) dengan ember token: tiap kunci boleh melonjak sampai burst permintaan lalu terisi ulang sebanyak perDetik per
// detik. Jumlah kunci yang diingat dibatasi (maks) sehingga memori tidak bisa dikuras dengan banyak alamat berbeda.
type PembatasLaju struct {
	mu       sync.Mutex
	perDetik float64
	burst    float64
	maks     int
	ember    map[string]*emberToken
	bersih   time.Time
}

type emberToken struct {
	token    float64
	terakhir time.Time
}

// NewPembatasLaju membuat pembatas. perDetik dan burst harus positif.
func NewPembatasLaju(perDetik float64, burst, maks int) *PembatasLaju {
	if perDetik <= 0 {
		perDetik = 1
	}
	if burst < 1 {
		burst = 1
	}
	if maks < 1 {
		maks = 50000
	}
	return &PembatasLaju{perDetik: perDetik, burst: float64(burst), maks: maks, ember: make(map[string]*emberToken)}
}

// Izin mengambil satu token untuk kunci. Bila habis, mengembalikan false beserta berapa lama harus menunggu.
func (p *PembatasLaju) Izin(kunci string, sekarang time.Time) (bool, time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ada := p.ember[kunci]
	if !ada {
		if len(p.ember) >= p.maks {
			p.bersihkan(sekarang)
			if len(p.ember) >= p.maks {
				return true, 0 // tabel penuh oleh alamat yang masih aktif: jangan menolak pengguna sah, hanya tidak dilacak
			}
		}
		p.ember[kunci] = &emberToken{token: p.burst - 1, terakhir: sekarang}
		return true, 0
	}
	if dt := sekarang.Sub(e.terakhir).Seconds(); dt > 0 {
		e.token = math.Min(p.burst, e.token+dt*p.perDetik)
		e.terakhir = sekarang
	}
	if e.token >= 1 {
		e.token--
		return true, 0
	}
	return false, time.Duration((1 - e.token) / p.perDetik * float64(time.Second))
}

// bersihkan membuang ember yang sudah penuh kembali (kunci yang lama tidak aktif), paling sering sekali per detik.
func (p *PembatasLaju) bersihkan(sekarang time.Time) {
	if sekarang.Sub(p.bersih) < time.Second {
		return
	}
	p.bersih = sekarang
	for k, e := range p.ember {
		if e.token+sekarang.Sub(e.terakhir).Seconds()*p.perDetik >= p.burst {
			delete(p.ember, k)
		}
	}
}

// Ukuran: jumlah kunci yang sedang dilacak (untuk tes).
func (p *PembatasLaju) Ukuran() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.ember)
}

// BatasLaju: middleware yang menolak (429) klien yang melewati batas. p nil = tidak membatasi. /health dan OPTIONS tidak dihitung.
func BatasLaju(p *PembatasLaju, pesan string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if p == nil || c.Request.Method == http.MethodOptions || jalurKesehatan(c.Request.URL.Path) {
			c.Next()
			return
		}
		if ok, tunggu := p.Izin(c.ClientIP(), time.Now()); !ok {
			ditolakLaju.Add(1)
			tolak(c, http.StatusTooManyRequests, pesan, KodeTerlaluBanyak, tunggu)
			return
		}
		c.Next()
	}
}

func lajuAktif() bool { return config.Cfg != nil && config.Cfg.RateLimitAktif }

// BatasLajuUmum: batas untuk semua permintaan dari satu IP (bawaan 200/detik, lonjakan 600).
func BatasLajuUmum() gin.HandlerFunc {
	if !lajuAktif() || config.Cfg.RateLimitUmumPerDetik <= 0 {
		return BatasLaju(nil, "")
	}
	burst := config.Cfg.RateLimitUmumBurst
	if burst < config.Cfg.RateLimitUmumPerDetik {
		burst = config.Cfg.RateLimitUmumPerDetik
	}
	return BatasLaju(NewPembatasLaju(float64(config.Cfg.RateLimitUmumPerDetik), burst, 50000), "Terlalu banyak permintaan dari alamat Anda. Tunggu sebentar lalu coba lagi")
}

func lajuPerMenit(perMenit int, pesan string) gin.HandlerFunc {
	if !lajuAktif() || perMenit <= 0 {
		return BatasLaju(nil, "")
	}
	return BatasLaju(NewPembatasLaju(float64(perMenit)/60, perMenit, 20000), pesan)
}

// BatasLajuLogin: batas login dengan kata sandi per IP per menit (melengkapi penguncian akun, yang hanya berlaku untuk akun yang ada).
func BatasLajuLogin() gin.HandlerFunc {
	if config.Cfg == nil {
		return BatasLaju(nil, "")
	}
	return lajuPerMenit(config.Cfg.RateLimitLoginPerMenit, "Terlalu banyak percobaan login dari alamat Anda. Coba lagi beberapa saat lagi")
}

// BatasLajuCaptcha: batas pembuatan captcha per IP per menit (membuat gambar captcha memakan CPU).
func BatasLajuCaptcha() gin.HandlerFunc {
	if config.Cfg == nil {
		return BatasLaju(nil, "")
	}
	return lajuPerMenit(config.Cfg.RateLimitCaptchaPerMenit, "Terlalu banyak permintaan captcha dari alamat Anda. Coba lagi beberapa saat lagi")
}

// BatasLajuPendaftaranAPI: batas pengiriman formulir pendaftaran API Akses Data (publik, tanpa login) per IP: 5 per jam. Melengkapi captcha dan batas jumlah pendaftaran
// yang menunggu.
func BatasLajuPendaftaranAPI() gin.HandlerFunc {
	if !lajuAktif() {
		return BatasLaju(nil, "")
	}
	return BatasLaju(NewPembatasLaju(5.0/3600, 5, 20000), "Terlalu banyak pendaftaran dari alamat Anda. Coba lagi beberapa jam lagi")
}

// BatasLajuInfoAPI: batas pembacaan katalog pendaftaran (publik) per IP: 60 per menit.
func BatasLajuInfoAPI() gin.HandlerFunc {
	return lajuPerMenit(60, "Terlalu banyak permintaan dari alamat Anda. Tunggu sebentar lalu coba lagi")
}

// ---------------------------------------------------------------- batas permintaan serentak (pelindung beban)

// BatasSerentak membatasi jumlah permintaan yang diproses bersamaan. Yang melebihi batas menunggu paling lama tunggu; bila tetap tidak ada tempat, ditolak 503 dengan Retry-After,
// dan dicatat sebagai peringatan di log server. maks <= 0 = tidak membatasi. /health tidak dihitung supaya pemeriksa kesehatan tetap bisa menjawab saat server sibuk.
func BatasSerentak(maks int, tunggu time.Duration) gin.HandlerFunc {
	if maks <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	sem := make(chan struct{}, maks)
	return func(c *gin.Context) {
		if jalurKesehatan(c.Request.URL.Path) {
			c.Next()
			return
		}
		masuk := func() {
			n := serentakSaat.Add(1)
			for {
				p := serentakPuncak.Load()
				if n <= p || serentakPuncak.CompareAndSwap(p, n) {
					break
				}
			}
		}
		keluar := func() { serentakSaat.Add(-1); <-sem }

		select {
		case sem <- struct{}{}:
		default:
			// Penuh: antre sebentar (lonjakan singkat tertangani tanpa menolak siapa pun).
			t := time.NewTimer(tunggu)
			select {
			case sem <- struct{}{}:
				t.Stop()
			case <-t.C:
				ditolakBeban.Add(1)
				c.Set(serverlog.KunciSudahDicatat, true)
				serverlog.Catat(serverlog.Entri{Level: serverlog.Warn, Sumber: "BEBAN", Pesan: "server kewalahan: permintaan serentak melewati batas " + strconv.Itoa(maks) + ", permintaan ditolak (503)"})
				tolak(c, http.StatusServiceUnavailable, "Server sedang sibuk. Coba lagi sebentar lagi", KodeSibuk, 2*time.Second)
				return
			case <-c.Request.Context().Done():
				t.Stop()
				c.Abort()
				return
			}
		}
		masuk()
		defer keluar()
		c.Next()
	}
}

// ---------------------------------------------------------------- rute berat

// Berat membuat middleware untuk rute berat (ekspor Excel/PDF, pembuatan dokumen, analitik, peta): paling banyak maks yang berjalan bersamaan, sisanya antre paling lama tunggu
// lalu ditolak 503. Semua rute berat harus memakai SATU middleware dari satu panggilan ini supaya berbagi batas. Rute berat juga ditandai agar waktu lambatnya tidak
// dilaporkan sebagai masalah.
func Berat(maks int, tunggu time.Duration) gin.HandlerFunc {
	if maks < 1 {
		maks = 1
	}
	sem := make(chan struct{}, maks)
	return func(c *gin.Context) {
		c.Set(serverlog.KunciBerat, true)
		select {
		case sem <- struct{}{}:
		default:
			beratAntri.Add(1)
			t := time.NewTimer(tunggu)
			select {
			case sem <- struct{}{}:
				t.Stop()
				beratAntri.Add(-1)
			case <-t.C:
				beratAntri.Add(-1)
				ditolakBerat.Add(1)
				c.Set(serverlog.KunciSudahDicatat, true)
				serverlog.Catat(serverlog.Entri{Level: serverlog.Warn, Sumber: "BEBAN", Pesan: "antrean pekerjaan berat penuh (ekspor/dokumen/analitik), permintaan ditolak (503)", Rute: c.FullPath(), Metode: c.Request.Method})
				tolak(c, http.StatusServiceUnavailable, "Server sedang menyiapkan beberapa ekspor/dokumen lain. Coba lagi sebentar lagi", KodeSibuk, 5*time.Second)
				return
			case <-c.Request.Context().Done():
				t.Stop()
				beratAntri.Add(-1)
				c.Abort()
				return
			}
		}
		beratSaat.Add(1)
		defer func() { beratSaat.Add(-1); <-sem }()
		c.Next()
	}
}

var (
	onceBerat    sync.Once
	beratBersama gin.HandlerFunc
)

// BeratBawaan mengembalikan middleware rute berat milik proses ini, dibuat sekali dari konfigurasi (MAKS_BERAT_SERENTAK, ANTREAN_TUNGGU_DETIK; antre rute berat 3x lebih lama dari
// permintaan biasa). Semua pemanggil mendapat middleware yang sama sehingga rute berat di semua modul berbagi satu batas.
func BeratBawaan() gin.HandlerFunc {
	onceBerat.Do(func() {
		maks, tunggu := 4, 10*time.Second
		if c := config.Cfg; c != nil {
			if c.MaksBeratSerentak > 0 {
				maks = c.MaksBeratSerentak
			}
			if c.AntreanTungguDetik > 0 {
				tunggu = time.Duration(c.AntreanTungguDetik) * 3 * time.Second
			}
		}
		beratBersama = Berat(maks, tunggu)
	})
	return beratBersama
}

// SerentakBawaan membuat BatasSerentak dari konfigurasi.
func SerentakBawaan() gin.HandlerFunc {
	if c := config.Cfg; c != nil {
		tunggu := time.Duration(c.AntreanTungguDetik) * time.Second
		if tunggu <= 0 {
			tunggu = 3 * time.Second
		}
		return BatasSerentak(c.MaksPermintaanSerentak, tunggu)
	}
	return BatasSerentak(0, 0)
}

// Eksternal menandai rute yang sebagian besar waktunya menunggu layanan luar (SSO Kemenkeu, HRIS2, SLDK): lambatnya bukan masalah server ini, jadi tidak dilaporkan sebagai
// "permintaan lambat" di log server (galat atau waktu habis pada layanan luar tetap dicatat oleh penanganannya). Tidak membatasi jumlah permintaan.
func Eksternal() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(serverlog.KunciBerat, true)
		c.Next()
	}
}

// ---------------------------------------------------------------- batas ukuran badan

// BatasBadan membatasi ukuran badan permintaan (bawaan 12 MB, sedikit di atas batas unggahan Nginx 10 MB) supaya badan raksasa tidak menghabiskan memori. Bila terlampaui,
// pembacaan badan oleh handler gagal dan handler menjawab 400.
func BatasBadan(maks int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maks > 0 && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maks)
		}
		c.Next()
	}
}

// BatasBadanBawaan membuat BatasBadan dari konfigurasi (BATAS_BADAN_MB).
func BatasBadanBawaan() gin.HandlerFunc {
	mb := 12
	if config.Cfg != nil && config.Cfg.BatasBadanMB > 0 {
		mb = config.Cfg.BatasBadanMB
	}
	return BatasBadan(int64(mb) << 20)
}
