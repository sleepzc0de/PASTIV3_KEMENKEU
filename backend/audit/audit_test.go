package audit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------- perekam

type sink struct {
	mu      sync.Mutex
	entri   []Entri
	panggil int
	gagal   int // gagalkan sekian pemanggilan pertama
}

func (s *sink) simpan(_ context.Context, es []Entri) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panggil++
	if s.gagal > 0 {
		s.gagal--
		return errors.New("database mati")
	}
	s.entri = append(s.entri, es...)
	return nil
}

func (s *sink) semua() []Entri {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Entri(nil), s.entri...)
}

func TestPerekamMenulisBerkelompokDanMenyiramSaatBerhenti(t *testing.T) {
	s := &sink{}
	p := NewPerekam(s.simpan, 100)
	p.interval = time.Hour // hanya penyiraman saat berhenti dan saat kelompok penuh
	p.maksKelompok = 3
	p.Mulai()
	for i := 0; i < 7; i++ {
		if !p.Catat(Entri{Aksi: "a", Label: "x"}) {
			t.Fatalf("entri %d dijatuhkan", i)
		}
	}
	ctx, batal := context.WithTimeout(context.Background(), 5*time.Second)
	defer batal()
	p.Berhenti(ctx)
	if got := len(s.semua()); got != 7 {
		t.Errorf("ditulis %d entri, want 7", got)
	}
	if st := p.Statistik(); st.Diterima != 7 || st.Ditulis != 7 || st.Dijatuhkan != 0 || st.Gagal != 0 || st.Antrean != 0 {
		t.Errorf("statistik = %+v", st)
	}
	// setelah berhenti, entri baru ditolak tanpa panic
	if p.Catat(Entri{}) {
		t.Error("Catat setelah Berhenti harus ditolak")
	}
	p.Berhenti(ctx) // aman dipanggil berulang
}

func TestPerekamMenulisBerdasarkanInterval(t *testing.T) {
	s := &sink{}
	p := NewPerekam(s.simpan, 10)
	p.interval = 10 * time.Millisecond
	p.Mulai()
	defer p.Berhenti(context.Background())
	p.Catat(Entri{Aksi: "a"})
	for i := 0; i < 200 && len(s.semua()) == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if len(s.semua()) != 1 {
		t.Fatalf("entri belum ditulis oleh interval")
	}
}

func TestPerekamMenjatuhkanEntriBilaAntreanPenuhTanpaMenunggu(t *testing.T) {
	lepas := make(chan struct{})
	mulai := make(chan struct{}, 1)
	p := NewPerekam(func(ctx context.Context, es []Entri) error {
		select {
		case mulai <- struct{}{}:
		default:
		}
		<-lepas // database macet
		return nil
	}, 2)
	p.maksKelompok = 1
	p.Mulai()
	p.Catat(Entri{Aksi: "pertama"})
	<-mulai // penulis sedang macet pada entri pertama
	t0 := time.Now()
	diterima := 0
	for i := 0; i < 10; i++ {
		if p.Catat(Entri{Aksi: "x"}) {
			diterima++
		}
	}
	if time.Since(t0) > time.Second {
		t.Errorf("Catat menunggu %v; harus segera kembali", time.Since(t0))
	}
	if diterima != 2 {
		t.Errorf("diterima %d, want 2 (kapasitas antrean)", diterima)
	}
	if st := p.Statistik(); st.Dijatuhkan != 8 || st.Antrean != 2 {
		t.Errorf("statistik = %+v, want dijatuhkan 8 dan antrean 2", st)
	}
	close(lepas)
	p.Berhenti(context.Background())
}

func TestPerekamMencobaLagiSekaliLaluMenghitungGagal(t *testing.T) {
	// gagal sekali lalu berhasil: tertulis
	s := &sink{gagal: 1}
	p := NewPerekam(s.simpan, 10)
	p.Mulai()
	p.Catat(Entri{Aksi: "a"})
	p.Berhenti(context.Background())
	if len(s.semua()) != 1 || p.Statistik().Gagal != 0 {
		t.Errorf("setelah coba ulang: tertulis %d, statistik %+v", len(s.semua()), p.Statistik())
	}
	// gagal terus: dibuang dan dihitung
	s2 := &sink{gagal: 100}
	p2 := NewPerekam(s2.simpan, 10)
	p2.Mulai()
	p2.Catat(Entri{Aksi: "a"})
	p2.Catat(Entri{Aksi: "b"})
	p2.Berhenti(context.Background())
	if st := p2.Statistik(); st.Gagal != 2 || st.Ditulis != 0 {
		t.Errorf("statistik gagal = %+v", st)
	}
}

// ---------------------------------------------------------------- middleware

// mesin membuat router uji: header X-User mensimulasikan autentikasi (mengisi user_id dan username seperti AuthRequired), lalu middleware audit, lalu rute uji.
func mesin(t *testing.T) (*gin.Engine, func() []Entri) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tanpaPengguna = &pembatas{maks: 30}
	s := &sink{}
	p := NewPerekam(s.simpan, 1000)
	p.Mulai()
	Pasang(p)
	t.Cleanup(func() { Pasang(nil) })

	r := gin.New()
	r.Use(Middleware())
	// Autentikasi tiruan dipasang sebagai middleware tingkat grup (seperti AuthRequired) supaya urutannya sama dengan aplikasi: audit membungkus autentikasi.
	api := r.Group("/api/v1", func(c *gin.Context) {
		if u := c.GetHeader("X-User"); u != "" {
			c.Set("user_id", "11111111-1111-4111-8111-111111111111")
			c.Set("username", u)
			c.Set("peran", "satker")
		}
		if c.GetHeader("X-Tolak") != "" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	})
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	api.PUT("/users/:id", ok)
	api.GET("/users", ok)
	api.POST("/sapa/lain/:id", ok)
	api.GET("/inaproc/ekspor/:awalan/:nama", ok)
	api.POST("/auth/login", func(c *gin.Context) {
		if c.Query("gagal") != "" {
			c.Status(http.StatusUnauthorized)
			return
		}
		if c.Query("tandai") != "" {
			Tandai(c, Tanda{UserID: "22222222-2222-4222-8222-222222222222", Username: "budi", Detail: map[string]interface{}{"metode": "password"}})
		}
		c.Status(http.StatusOK)
	})
	api.POST("/auth/login-tolak", func(c *gin.Context) { // tidak ada di katalog
		Tandai(c, Tanda{Aksi: AksiLoginGagal, Label: "Login gagal", Username: "x", Kategori: KatAuth, Detail: map[string]interface{}{"alasan": "kata_sandi"}})
		c.Status(http.StatusForbidden)
	})
	api.POST("/diabaikan", func(c *gin.Context) { Tandai(c, Tanda{Abaikan: true}); c.Status(http.StatusOK) })
	return r, func() []Entri {
		p.Berhenti(context.Background())
		return s.semua()
	}
}

func kirim(r *gin.Engine, metode, url string, header ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(metode, url, nil)
	req.RemoteAddr = "10.0.0.5:4444"
	req.Header.Set("User-Agent", "UjiBrowser/1.0")
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestMiddlewareMencatatPerubahanDataPenggunaDenganLabelDanObjek(t *testing.T) {
	r, hasil := mesin(t)
	w := kirim(r, "PUT", "/api/v1/users/42", "X-User", "budi")
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("header X-Request-ID tidak dipasang")
	}
	es := hasil()
	if len(es) != 1 {
		t.Fatalf("entri = %d, want 1: %+v", len(es), es)
	}
	e := es[0]
	if e.Kategori != KatPengguna || e.Aksi != "pengguna.ubah" || e.Label != "Mengubah pengguna 42" || e.ObjekTipe != "pengguna" || e.ObjekID != "42" {
		t.Errorf("entri = %+v", e)
	}
	if e.Username != "budi" || e.UserID != "11111111-1111-4111-8111-111111111111" || e.Peran != "satker" || !e.Sukses || e.Status != 200 || e.Metode != "PUT" || e.Rute != "/api/v1/users/:id" {
		t.Errorf("identitas/hasil = %+v", e)
	}
	if e.IP != "10.0.0.5" || e.UserAgent != "UjiBrowser/1.0" || e.RequestID != w.Header().Get("X-Request-ID") || e.Waktu.IsZero() {
		t.Errorf("klien = ip %q ua %q req %q waktu %v", e.IP, e.UserAgent, e.RequestID, e.Waktu)
	}
	if p, _ := e.Detail["parameter"].(map[string]string); p["id"] != "42" {
		t.Errorf("detail = %+v", e.Detail)
	}
}

func TestMiddlewareTidakMencatatBacaBiasaTanpaPenggunaDanRuteTakAda(t *testing.T) {
	r, hasil := mesin(t)
	kirim(r, "GET", "/api/v1/users", "X-User", "budi")        // GET biasa
	kirim(r, "PUT", "/api/v1/users/42")                       // tanpa pengguna
	kirim(r, "GET", "/api/v1/tidak-ada", "X-User", "budi")    // 404
	kirim(r, "OPTIONS", "/api/v1/users/42", "X-User", "budi") // preflight CORS
	if es := hasil(); len(es) != 0 {
		t.Errorf("tidak boleh ada entri, dapat %d: %+v", len(es), es)
	}
}

func TestMiddlewareMencatatGETBacaDenganKueriYangDibersihkan(t *testing.T) {
	r, hasil := mesin(t)
	kirim(r, "GET", "/api/v1/inaproc/ekspor/tender/paket?tahun=2026&format=xlsx&token=rahasia&code=abc&state=zzz", "X-User", "budi")
	es := hasil()
	if len(es) != 1 {
		t.Fatalf("entri = %d", len(es))
	}
	e := es[0]
	if e.Kategori != KatEkspor || e.Aksi != "ekspor.pengadaan" || e.ObjekID != "paket" || !strings.Contains(e.Label, "tender/paket") {
		t.Errorf("entri = %+v", e)
	}
	if k, _ := e.Detail["kueri"].(string); k != "format=xlsx&tahun=2026" {
		t.Errorf("kueri = %q, want hanya format dan tahun (tanpa token/code/state)", k)
	}
}

func TestMiddlewareMenandaiAksesDitolak(t *testing.T) {
	r, hasil := mesin(t)
	kirim(r, "PUT", "/api/v1/users/7", "X-User", "budi", "X-Tolak", "1")
	kirim(r, "GET", "/api/v1/users", "X-User", "budi", "X-Tolak", "1") // GET biasa pun dicatat bila ditolak
	es := hasil()
	if len(es) != 2 {
		t.Fatalf("entri = %d: %+v", len(es), es)
	}
	for _, e := range es {
		if e.Aksi != AksiDitolak || e.Sukses || e.Status != 403 || !strings.HasPrefix(e.Label, "Akses ditolak: ") {
			t.Errorf("entri = %+v", e)
		}
	}
	if es[0].Detail["aksi_asal"] != "pengguna.ubah" {
		t.Errorf("aksi asal = %v", es[0].Detail["aksi_asal"])
	}
}

func TestMiddlewareLoginDitandaiHandlerDanTanpaTandaDianggapGagalBilaGalat(t *testing.T) {
	r, hasil := mesin(t)
	kirim(r, "POST", "/api/v1/auth/login?tandai=1")                // berhasil, pengguna dari handler
	kirim(r, "POST", "/api/v1/auth/login?gagal=1")                 // 401 tanpa tanda: bukan "berhasil"
	kirim(r, "POST", "/api/v1/auth/login-tolak", "X-User", "tamu") // 403 login ditolak: tetap aksi login gagal
	es := hasil()
	if len(es) != 3 {
		t.Fatalf("entri = %d: %+v", len(es), es)
	}
	if e := es[0]; e.Aksi != AksiLoginBerhasil || e.Username != "budi" || e.UserID != "22222222-2222-4222-8222-222222222222" || !e.Sukses || e.Detail["metode"] != "password" {
		t.Errorf("login berhasil = %+v", e)
	}
	if e := es[1]; e.Aksi != AksiLoginGagal || e.Sukses || e.Username != "" {
		t.Errorf("login gagal tanpa tanda = %+v", e)
	}
	if e := es[2]; e.Aksi != AksiLoginGagal || e.Sukses || e.Status != 403 || e.Detail["alasan"] != "kata_sandi" || e.Username != "x" {
		t.Errorf("login ditolak = %+v", e)
	}
}

func TestMiddlewareRuteTakAdaDiKatalogDicatatDenganKategoriDariAwalan(t *testing.T) {
	r, hasil := mesin(t)
	kirim(r, "POST", "/api/v1/sapa/lain/9", "X-User", "budi")
	es := hasil()
	if len(es) != 1 {
		t.Fatalf("entri = %d", len(es))
	}
	if e := es[0]; e.Kategori != KatSapa || e.Aksi != "POST /api/v1/sapa/lain/:id" || e.Label != "Menjalankan POST /api/v1/sapa/lain/:id" {
		t.Errorf("entri = %+v", e)
	}
}

func TestTandaiAbaikanDanPenggabungan(t *testing.T) {
	r, hasil := mesin(t)
	kirim(r, "POST", "/api/v1/diabaikan", "X-User", "budi")
	if es := hasil(); len(es) != 0 {
		t.Errorf("Abaikan diabaikan: %+v", es)
	}

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	Tandai(c, Tanda{Aksi: "a", Detail: map[string]interface{}{"x": 1}})
	f := false
	Tandai(c, Tanda{Label: "L", Sukses: &f, Detail: map[string]interface{}{"y": 2}})
	v, _ := c.Get(kunciTanda)
	tn := v.(Tanda)
	if tn.Aksi != "a" || tn.Label != "L" || tn.Sukses == nil || *tn.Sukses || tn.Detail["x"] != 1 || tn.Detail["y"] != 2 {
		t.Errorf("tanda gabungan = %+v", tn)
	}
}

func TestMiddlewareTanpaPerekamTidakMencatatDanTidakPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	Pasang(nil)
	r := gin.New()
	r.Use(Middleware())
	r.POST("/api/v1/users", func(c *gin.Context) { c.Set("user_id", "u"); c.Status(200) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/users", nil))
	if w.Code != 200 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestPembatasEntriTanpaPenggunaMelindungiTabelDariBanjir(t *testing.T) {
	r, hasil := mesin(t)
	for i := 0; i < 100; i++ {
		kirim(r, "POST", "/api/v1/auth/login?gagal=1")
	}
	n := len(hasil())
	if n < 1 || n > 30 {
		t.Errorf("entri tanpa pengguna = %d, want 1..30 per detik", n)
	}
}

// ---------------------------------------------------------------- katalog

func TestBersihkanKueri(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"", ""},
		{"tahun=2026&format=xlsx", "format=xlsx&tahun=2026"},
		{"q=budi&access_token=abc&Code=1&state=s&password=p&api_key=k&keyword=ok", "keyword=ok&q=budi"},
		{"%zz", ""},
	} {
		if got := bersihkanKueri(c.in); got != c.want {
			t.Errorf("bersihkanKueri(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := bersihkanKueri("q=" + strings.Repeat("a", 500)); len(got) > 310 || !strings.HasSuffix(got, "…") {
		t.Errorf("kueri panjang tidak dipotong: %d", len(got))
	}
}

func TestKategoriDariRute(t *testing.T) {
	for rute, want := range map[string]string{
		"/api/v1/auth/login": KatAuth, "/sso/callback/login": KatAuth, "/api/v1/users/:id": KatPengguna, "/api/v1/digitalisasi/x": KatDigitalisasi,
		"/api/v1/sapa/penjualan": KatSapa, "/api/v1/hris2/pegawai": KatHRIS2, "/api/v1/referensi/ue1/:kode": KatReferensi, "/api/v1/inaproc/penarikan": KatPengadaan,
		"/api/v1/audit/log": KatAudit, "/api/v1/satker/keterhubungan": KatLainnya, "/health": KatLainnya,
	} {
		if got := KategoriDariRute(rute); got != want {
			t.Errorf("KategoriDariRute(%q) = %q, want %q", rute, got, want)
		}
	}
}

func TestDaftarOpsiMemuatKategoriBerurutanDanAksiPenting(t *testing.T) {
	o := DaftarOpsi()
	if len(o.Kategori) != len(urutanKategori) || o.Kategori[0].Kode != KatAuth || o.Kategori[0].Label == "" {
		t.Errorf("kategori = %+v", o.Kategori)
	}
	ada := map[string]OpsiAksi{}
	for _, a := range o.Aksi {
		if _, dobel := ada[a.Kode]; dobel {
			t.Errorf("aksi ganda: %s", a.Kode)
		}
		ada[a.Kode] = a
		if a.Label == "" || strings.ContainsAny(a.Label, "{}") || strings.Contains(a.Label, "()") || strings.Contains(a.Label, "  ") || a.Label != strings.TrimSpace(a.Label) {
			t.Errorf("label aksi %s belum bersih: %q", a.Kode, a.Label)
		}
	}
	for _, k := range []string{AksiLoginBerhasil, AksiLoginGagal, AksiDitolak, "pengguna.hapus", "peran.beri", "ekspor.digitalisasi", "sapa.tahap.selesai", "hris2.cari", "audit.ekspor"} {
		if _, ok := ada[k]; !ok {
			t.Errorf("aksi %s tidak ada di opsi", k)
		}
	}
}

func TestPenyaringHalamanAman(t *testing.T) {
	for _, c := range []struct{ h, ph, off, lim int }{{0, 0, 0, 50}, {1, 10, 0, 10}, {3, 10, 20, 10}, {2, 9999, 200, 200}, {-5, -1, 0, 50}} {
		off, lim := Penyaring{Halaman: c.h, PerHalaman: c.ph}.halamanAman()
		if off != c.off || lim != c.lim {
			t.Errorf("halaman %d/%d = %d,%d want %d,%d", c.h, c.ph, off, lim, c.off, c.lim)
		}
	}
}

func TestBangunKondisiMemakaiParameterDanMenghindariKarakterKhusus(t *testing.T) {
	f := false
	where, args := bangunKondisi(Penyaring{
		Dari: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Sampai: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), UserID: "u", Username: "50%_a", Kategori: KatAuth, Aksi: "x.y", Sukses: &f, IP: "10.0", Q: "a'; DROP TABLE users;--",
	})
	if strings.Contains(where, "DROP") || strings.Contains(where, "50%") {
		t.Errorf("masukan pengguna masuk ke SQL: %s", where)
	}
	if len(args) != 9 {
		t.Errorf("argumen = %d, want 9", len(args))
	}
	for i := 1; i <= 9; i++ {
		if !strings.Contains(where, "@p"+string(rune('0'+i))) {
			t.Errorf("parameter @p%d tidak dipakai: %s", i, where)
		}
	}
	if args[3] != `%50\%\_a%` {
		t.Errorf("LIKE tidak di-escape: %v", args[3])
	}
	if w, a := bangunKondisi(Penyaring{}); w != "" || len(a) != 0 {
		t.Errorf("tanpa syarat: %q %v", w, a)
	}
}

func TestPotongMenjagaPanjangKolom(t *testing.T) {
	if got := potong("héllo wörld", 5); got != "héllo" {
		t.Errorf("potong = %q", got)
	}
	if got := potong("abc", 5); got != "abc" {
		t.Errorf("potong pendek = %q", got)
	}
}
