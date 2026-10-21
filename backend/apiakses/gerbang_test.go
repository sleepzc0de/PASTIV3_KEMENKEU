package apiakses

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/audit"
)

// Tes gerbang tanpa database: Sumber dan penyimpan riwayat diganti tiruan, jam dikendalikan.

type sumberTiruan struct {
	mu          sync.Mutex
	kred        map[string]*Kredensial
	pengaturan  Pengaturan
	cariPanggil int
	galat       error
}

func (s *sumberTiruan) CariKredensial(_ context.Context, prefix string) (*Kredensial, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cariPanggil++
	if s.galat != nil {
		return nil, s.galat
	}
	if k, ok := s.kred[prefix]; ok {
		salin := *k
		return &salin, nil
	}
	return nil, nil
}

func (s *sumberTiruan) MuatPengaturan(context.Context) (Pengaturan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pengaturan, nil
}

func (s *sumberTiruan) panggilan() int { s.mu.Lock(); defer s.mu.Unlock(); return s.cariPanggil }

type penyimpanTiruan struct {
	mu    sync.Mutex
	log   []Penggunaan
	agg   []Agregat
	gagal bool
}

func (p *penyimpanTiruan) SimpanLog(_ context.Context, ps []Penggunaan) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.gagal {
		return context.DeadlineExceeded
	}
	p.log = append(p.log, ps...)
	return nil
}

func (p *penyimpanTiruan) SimpanAgregat(_ context.Context, as []Agregat) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.gagal {
		return context.DeadlineExceeded
	}
	p.agg = append(p.agg, as...)
	return nil
}

func (p *penyimpanTiruan) salinanLog() []Penggunaan {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Penggunaan(nil), p.log...)
}

const pepperUji = "pepper-uji-api-akses"

type lingkungan struct {
	l      *Layanan
	s      *sumberTiruan
	p      *penyimpanTiruan
	pen    *Pencatat
	jam    *jam
	r      *gin.Engine
	kunci  string
	prefix string
}

// buatLingkungan menyiapkan satu klien disetujui dengan kunci sah (kuncinya dikembalikan) dan rute uji di belakang gerbang.
func buatLingkungan(t *testing.T) *lingkungan {
	t.Helper()
	gin.SetMode(gin.TestMode)
	kunci, prefix, err := BuatKunci()
	if err != nil {
		t.Fatal(err)
	}
	j := &jam{t: time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)}
	p := &penyimpanTiruan{}
	pen := NewPencatat(p, 1000)
	s := &sumberTiruan{pengaturan: PengaturanBawaan(), kred: map[string]*Kredensial{}}
	l := NewLayanan(nil, s, pen, pepperUji)
	l.sekarang = j.now
	l.Pembatas.sekarang = j.now
	l.Gagal.sekarang = j.now
	s.kred[prefix] = &Kredensial{
		KunciID: 11, KlienID: 5, Prefix: prefix, Hash: l.HashKunci(kunci), KlienKode: "KL-ABCDEFGH", KlienNama: "Aplikasi Contoh", KlienStatus: StatusDisetujui,
		Dataset: []string{"aset.tanah"},
	}
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(audit.Middleware())
	g := r.Group(AwalanRute)
	g.GET("/data", l.Gerbang(true), func(c *gin.Context) {
		CatatDataset(c, "aset.tanah")
		CatatBaris(c, 3)
		c.JSON(200, gin.H{"ok": true})
	})
	g.GET("/saya", l.Gerbang(false), func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	g.GET("/dataset/:kode", l.Gerbang(true), func(c *gin.Context) {
		id := IdentitasDari(c)
		if !id.BolehDataset(c.Param("kode")) {
			TolakScope(c, c.Param("kode"))
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})
	g.GET("/rusak", l.Gerbang(true), func(c *gin.Context) { c.JSON(500, gin.H{"success": false}) })
	g.GET("/tidakada", l.Gerbang(true), func(c *gin.Context) { c.JSON(404, gin.H{"success": false}) })
	return &lingkungan{l: l, s: s, p: p, pen: pen, jam: j, r: r, kunci: kunci, prefix: prefix}
}

func (e *lingkungan) minta(path string, ubah func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, AwalanRute+path, nil)
	req.RemoteAddr = "203.0.113.7:4455"
	if ubah != nil {
		ubah(req)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

func (e *lingkungan) denganKunci(k string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+k) }
}

func badan(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("badan bukan JSON: %q", w.Body.String())
	}
	return m
}

func TestGerbangKunciSahDilayaniDenganHeaderBatas(t *testing.T) {
	e := buatLingkungan(t)
	w := e.minta("/data", e.denganKunci(e.kunci))
	if w.Code != 200 {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	h := w.Header()
	want := map[string]string{
		"X-RateLimit-Limit": "1000", "X-RateLimit-Remaining": "999", "X-Quota-Limit": "5000", "X-Quota-Remaining": "4999",
		"X-Quota-Reset": strconv.FormatInt(time.Date(2026, 10, 7, 11, 0, 0, 0, time.UTC).Unix(), 10),
	}
	for k, v := range want {
		if h.Get(k) != v {
			t.Errorf("header %s = %q, want %q", k, h.Get(k), v)
		}
	}
	if h.Get("X-RateLimit-Reset") == "" {
		t.Error("X-RateLimit-Reset harus ada")
	}
	if h.Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, data klien tidak boleh disimpan perantara", h.Get("Cache-Control"))
	}
	if h.Get("Retry-After") != "" {
		t.Error("Retry-After hanya untuk 429/503")
	}
}

func TestGerbangKunciViaXAPIKey(t *testing.T) {
	e := buatLingkungan(t)
	w := e.minta("/data", func(r *http.Request) { r.Header.Set("X-API-Key", e.kunci) })
	if w.Code != 200 {
		t.Errorf("X-API-Key harus diterima: %d", w.Code)
	}
}

func TestGerbangKunciDiQueryStringDiabaikan(t *testing.T) {
	e := buatLingkungan(t)
	for _, q := range []string{"?api_key=", "?key=", "?token=", "?access_token="} {
		w := e.minta("/data"+q+e.kunci, nil)
		if w.Code != 401 {
			t.Errorf("kunci di query string %s harus ditolak (alamat tercatat di log proxy): %d", q, w.Code)
		}
	}
}

// Semua kegagalan autentikasi harus tampak sama dari luar: status, badan, dan header (tidak membocorkan apakah kunci ada, dicabut, atau kedaluwarsa).
func TestGerbangSemuaKegagalanAutentikasiTampakSama(t *testing.T) {
	e := buatLingkungan(t)
	lain, lainPrefix, _ := BuatKunci()
	salahRahasia := e.prefix
	salahRahasia = "pasti_" + salahRahasia + "_" + strings.Repeat("A", 43)

	cabut := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	lalu := e.jam.now().Add(-time.Hour)
	tambah := func(nama string, ubah func(*Kredensial)) string {
		k, p, _ := BuatKunci()
		cred := &Kredensial{KunciID: 20, KlienID: 6, Prefix: p, Hash: e.l.HashKunci(k), KlienKode: "KL-" + nama, KlienNama: nama, KlienStatus: StatusDisetujui, Dataset: []string{"aset.tanah"}}
		ubah(cred)
		e.s.kred[p] = cred
		return k
	}
	dicabut := tambah("DICABUT", func(c *Kredensial) { c.DicabutAt = &cabut })
	kedaluwarsa := tambah("KEDALUWARSA", func(c *Kredensial) { c.KedaluwarsaAt = &lalu })
	menunggu := tambah("MENUNGGU", func(c *Kredensial) { c.KlienStatus = StatusMenunggu })
	ditolak := tambah("DITOLAK", func(c *Kredensial) { c.KlienStatus = StatusDitolak })
	klienDicabut := tambah("KLIENDICABUT", func(c *Kredensial) { c.KlienStatus = StatusDicabut })
	_ = lainPrefix

	kasus := map[string]func(*http.Request){
		"tanpa kunci":        nil,
		"kunci bentuk salah": e.denganKunci("bukan-kunci"),
		"prefix tidak ada":   e.denganKunci(lain),
		"rahasia salah":      e.denganKunci(salahRahasia),
		"kunci dicabut":      e.denganKunci(dicabut),
		"kunci kedaluwarsa":  e.denganKunci(kedaluwarsa),
		"klien menunggu":     e.denganKunci(menunggu),
		"klien ditolak":      e.denganKunci(ditolak),
		"klien dicabut":      e.denganKunci(klienDicabut),
		"skema salah":        func(r *http.Request) { r.Header.Set("Authorization", "Basic "+e.kunci) },
	}
	var acuanBadan string
	for nama, ubah := range kasus {
		w := e.minta("/data", ubah)
		if w.Code != 401 {
			t.Errorf("%s: status %d, want 401", nama, w.Code)
			continue
		}
		if acuanBadan == "" {
			acuanBadan = w.Body.String()
		}
		if w.Body.String() != acuanBadan {
			t.Errorf("%s: badan berbeda dari kegagalan lain:\n%s\nvs\n%s", nama, w.Body.String(), acuanBadan)
		}
		m := badan(t, w)
		if m["code"] != KodeKunciTidakValid || m["success"] != false {
			t.Errorf("%s: %v", nama, m)
		}
		for _, s := range []string{e.prefix, "DICABUT", "KEDALUWARSA", "KL-"} {
			if strings.Contains(w.Body.String(), s) {
				t.Errorf("%s: badan membocorkan %q", nama, s)
			}
		}
		if w.Header().Get("WWW-Authenticate") == "" {
			t.Errorf("%s: 401 harus menyertakan WWW-Authenticate", nama)
		}
		if w.Header().Get("X-RateLimit-Limit") != "" {
			t.Errorf("%s: header batas tidak boleh muncul untuk yang tidak terautentikasi", nama)
		}
	}
	if acuanBadan == "" {
		t.Fatal("tidak ada kasus yang diuji")
	}
	// Klien dengan kunci sah tidak terpengaruh.
	if w := e.minta("/data", e.denganKunci(e.kunci)); w.Code != 200 {
		t.Errorf("kunci sah tetap harus diterima: %d", w.Code)
	}
}

func TestGerbangTokenPenggunaBukanKunci(t *testing.T) {
	e := buatLingkungan(t)
	jwtPalsu := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMSIsInJvbGUiOiJzdXBlcmFkbWluIn0.abc123"
	if w := e.minta("/data", e.denganKunci(jwtPalsu)); w.Code != 401 {
		t.Errorf("JWT pengguna tidak boleh dikenali sebagai kunci API: %d", w.Code)
	}
}

func TestGerbangKesalahanDatabaseBukanKegagalanKunci(t *testing.T) {
	e := buatLingkungan(t)
	e.s.galat = context.DeadlineExceeded
	w := e.minta("/data", e.denganKunci(e.kunci))
	if w.Code != 503 {
		t.Fatalf("database macet harus 503, bukan 401: %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("503 harus menyertakan Retry-After")
	}
	if strings.Contains(w.Body.String(), "deadline") || strings.Contains(w.Body.String(), "context") {
		t.Errorf("galat internal bocor: %s", w.Body.String())
	}
	// Galat database tidak dihitung sebagai kegagalan kunci.
	if e.l.Gagal.Ukuran() != 0 {
		t.Error("galat database tidak boleh menambah hitungan percobaan gagal")
	}
}

func TestGerbangAllowlistIP(t *testing.T) {
	e := buatLingkungan(t)
	e.s.kred[e.prefix].IP = []string{"198.51.100.0/24", "203.0.113.50/32"}
	e.l.BuangCache()
	w := e.minta("/data", e.denganKunci(e.kunci)) // 203.0.113.7: di luar daftar
	if w.Code != 403 {
		t.Fatalf("IP di luar allowlist harus 403: %d", w.Code)
	}
	if m := badan(t, w); m["code"] != KodeAlamatDitolak {
		t.Errorf("code = %v", m["code"])
	}
	if strings.Contains(w.Body.String(), "198.51.100") {
		t.Error("isi allowlist tidak boleh bocor")
	}
	ok := e.minta("/data", func(r *http.Request) { e.denganKunci(e.kunci)(r); r.RemoteAddr = "198.51.100.77:1" })
	if ok.Code != 200 {
		t.Errorf("IP dalam allowlist harus diterima: %d", ok.Code)
	}
	// Header X-Forwarded-For tidak dipercaya tanpa proxy tepercaya: tidak bisa memalsukan IP yang diizinkan.
	palsu := e.minta("/data", func(r *http.Request) {
		e.denganKunci(e.kunci)(r)
		r.Header.Set("X-Forwarded-For", "198.51.100.77")
		r.Header.Set("X-Real-IP", "198.51.100.77")
	})
	if palsu.Code != 403 {
		t.Errorf("X-Forwarded-For/X-Real-IP palsu tidak boleh melewati allowlist: %d", palsu.Code)
	}
	// Penolakan IP tidak memakai jatah.
	if d := e.l.Pembatas.Lihat(5, e.l.Pengaturan(context.Background()).BatasUntuk(Override{}).KeBatas()); d.KuotaTerpakai != 1 {
		t.Errorf("hanya satu permintaan sah yang boleh terhitung: %+v", d)
	}
}

func TestGerbangBatasLajuMenghasilkan429(t *testing.T) {
	e := buatLingkungan(t)
	n := 3
	e.s.kred[e.prefix].Override = Override{LimitPermintaan: &n}
	e.l.BuangCache()
	for i := 0; i < 3; i++ {
		if w := e.minta("/data", e.denganKunci(e.kunci)); w.Code != 200 {
			t.Fatalf("permintaan %d: %d", i+1, w.Code)
		}
	}
	w := e.minta("/data", e.denganKunci(e.kunci))
	if w.Code != 429 {
		t.Fatalf("permintaan ke-4 harus 429: %d", w.Code)
	}
	m := badan(t, w)
	if m["code"] != KodeBatasLaju || m["success"] != false || m["retry_after_detik"].(float64) != 60 {
		t.Errorf("badan 429 = %v", m)
	}
	if w.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After = %q", w.Header().Get("Retry-After"))
	}
	if w.Header().Get("X-RateLimit-Remaining") != "0" || w.Header().Get("X-RateLimit-Limit") != "3" {
		t.Errorf("header sisa/limit pada 429: %v", w.Header())
	}
	if w.Header().Get("X-Quota-Remaining") != "4997" {
		t.Errorf("429 tidak memakai kuota: X-Quota-Remaining = %q", w.Header().Get("X-Quota-Remaining"))
	}
	// Jendela bergeser: setelah 60 detik klien dilayani lagi.
	e.jam.maju(61 * time.Second)
	if w := e.minta("/data", e.denganKunci(e.kunci)); w.Code != 200 {
		t.Errorf("setelah jendela lewat harus 200: %d", w.Code)
	}
}

func TestGerbangKuotaHabisDanEndpointTanpaKuota(t *testing.T) {
	e := buatLingkungan(t)
	k := 2
	e.s.kred[e.prefix].Override = Override{KuotaPermintaan: &k}
	e.l.BuangCache()
	e.minta("/data", e.denganKunci(e.kunci))
	e.minta("/data", e.denganKunci(e.kunci))
	w := e.minta("/data", e.denganKunci(e.kunci))
	if w.Code != 429 {
		t.Fatalf("kuota habis harus 429: %d", w.Code)
	}
	if m := badan(t, w); m["code"] != KodeKuotaHabis {
		t.Errorf("code = %v", m["code"])
	}
	wantTunggu := strconv.Itoa(3600) // permintaan terjadi tepat pada awal periode (10:00:00)
	if w.Header().Get("Retry-After") != wantTunggu {
		t.Errorf("Retry-After = %q, want %s", w.Header().Get("Retry-After"), wantTunggu)
	}
	if w.Header().Get("X-Quota-Remaining") != "0" || w.Header().Get("X-Quota-Limit") != "2" {
		t.Errorf("header kuota: %v", w.Header())
	}
	// Klien tetap bisa melihat profil/pemakaiannya sendiri walau kuota habis.
	if w := e.minta("/saya", e.denganKunci(e.kunci)); w.Code != 200 {
		t.Errorf("/saya tidak memakai kuota: %d", w.Code)
	}
	e.jam.maju(time.Hour)
	if w := e.minta("/data", e.denganKunci(e.kunci)); w.Code != 200 {
		t.Errorf("periode baru: kuota pulih, got %d", w.Code)
	}
}

func TestGerbangBatasDiubahTanpaDeploy(t *testing.T) {
	e := buatLingkungan(t)
	for i := 0; i < 1000; i++ {
		e.l.Pembatas.Coba(5, Batas{LimitPermintaan: 1000, LimitJendela: time.Minute, Kuota: 5000, KuotaPeriode: time.Hour}, true)
	}
	if w := e.minta("/data", e.denganKunci(e.kunci)); w.Code != 429 {
		t.Fatalf("bawaan 1000/60 detik sudah habis: harus 429, got %d", w.Code)
	}
	// Superadmin menaikkan bawaan global di database; berlaku setelah cache pengaturan kedaluwarsa (5 detik), tanpa memulai ulang.
	e.s.mu.Lock()
	e.s.pengaturan.LimitPermintaan = 2000
	e.s.mu.Unlock()
	if w := e.minta("/data", e.denganKunci(e.kunci)); w.Code != 429 {
		t.Log("(cache pengaturan masih berlaku)")
	}
	e.jam.maju(6 * time.Second)
	w := e.minta("/data", e.denganKunci(e.kunci))
	if w.Code != 200 {
		t.Fatalf("setelah pengaturan dinaikkan ke 2000 harus 200: %d", w.Code)
	}
	if w.Header().Get("X-RateLimit-Limit") != "2000" {
		t.Errorf("X-RateLimit-Limit = %q", w.Header().Get("X-RateLimit-Limit"))
	}
	// Override per klien menang atas bawaan global.
	n := 5
	e.s.kred[e.prefix].Override = Override{LimitPermintaan: &n}
	e.l.BuangCache()
	if w := e.minta("/data", e.denganKunci(e.kunci)); w.Header().Get("X-RateLimit-Limit") != "5" {
		t.Errorf("override klien: X-RateLimit-Limit = %q", w.Header().Get("X-RateLimit-Limit"))
	}
}

func TestGerbangScopeDataset(t *testing.T) {
	e := buatLingkungan(t)
	if w := e.minta("/dataset/aset.tanah", e.denganKunci(e.kunci)); w.Code != 200 {
		t.Errorf("dataset dalam scope: %d", w.Code)
	}
	w := e.minta("/dataset/aset.satker", e.denganKunci(e.kunci))
	if w.Code != 403 {
		t.Fatalf("dataset di luar scope harus 403: %d", w.Code)
	}
	if m := badan(t, w); m["code"] != KodeDatasetDitolak {
		t.Errorf("code = %v", m["code"])
	}
}

func TestGerbangKegagalanKunciMengunci(t *testing.T) {
	e := buatLingkungan(t)
	e.l.Gagal = NewPenjagaGagal(5, 10*time.Minute, 100)
	e.l.Gagal.sekarang = e.jam.now
	// Kunci sah dipakai dulu sehingga masuk cache.
	if w := e.minta("/data", e.denganKunci(e.kunci)); w.Code != 200 {
		t.Fatal("kunci sah harus diterima")
	}
	_, lain, _ := BuatKunci()
	for i := 0; i < 5; i++ {
		if w := e.minta("/data", e.denganKunci(lain)); w.Code != 401 {
			t.Fatalf("percobaan %d: %d", i+1, w.Code)
		}
	}
	panggilan := e.s.panggilan()
	w := e.minta("/data", e.denganKunci("pasti_"+strings.Repeat("a", 10)+"_"+strings.Repeat("B", 43)))
	if w.Code != 429 {
		t.Fatalf("setelah 5 kegagalan, kunci tidak sah dari IP yang sama harus 429: %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("429 pengunci harus menyertakan Retry-After")
	}
	if e.s.panggilan() != panggilan {
		t.Error("IP yang terkunci tidak boleh lagi membebani database dengan pencarian kunci")
	}
	// Kunci sah yang sudah ada di cache tetap dilayani dari IP yang sama (tidak ikut terkunci), IP lain tidak terpengaruh.
	if w := e.minta("/data", e.denganKunci(e.kunci)); w.Code != 200 {
		t.Errorf("kunci sah di cache tetap dilayani: %d", w.Code)
	}
	if w := e.minta("/data", func(r *http.Request) { e.denganKunci(lain)(r); r.RemoteAddr = "198.51.100.9:1" }); w.Code != 401 {
		t.Errorf("IP lain bukan terkunci: %d", w.Code)
	}
	// Terkunci hanya sementara.
	e.jam.maju(11 * time.Minute)
	if w := e.minta("/data", e.denganKunci(lain)); w.Code != 401 {
		t.Errorf("setelah jendela lewat kembali 401: %d", w.Code)
	}
}

func TestGerbangCachePencabutanBerlakuSetelahBuangCache(t *testing.T) {
	e := buatLingkungan(t)
	if w := e.minta("/data", e.denganKunci(e.kunci)); w.Code != 200 {
		t.Fatal("harus 200")
	}
	cabut := e.jam.now()
	e.s.kred[e.prefix].DicabutAt = &cabut
	// Superadmin mencabut lewat aplikasi: BuangCache dipanggil, jadi berlaku seketika.
	e.l.BuangCache()
	if w := e.minta("/data", e.denganKunci(e.kunci)); w.Code != 401 {
		t.Errorf("kunci yang baru dicabut harus langsung ditolak: %d", w.Code)
	}
	// Pencabutan langsung di database berlaku paling lambat setelah ttlKredensial.
	e2 := buatLingkungan(t)
	e2.minta("/data", e2.denganKunci(e2.kunci))
	c2 := e2.jam.now()
	e2.s.kred[e2.prefix].DicabutAt = &c2
	e2.jam.maju(ttlKredensial + time.Second)
	if w := e2.minta("/data", e2.denganKunci(e2.kunci)); w.Code != 401 {
		t.Errorf("setelah cache kedaluwarsa pencabutan harus berlaku: %d", w.Code)
	}
}

func TestGerbangKedaluwarsaOtomatis(t *testing.T) {
	e := buatLingkungan(t)
	kad := e.jam.now().Add(30 * time.Minute)
	e.s.kred[e.prefix].KedaluwarsaAt = &kad
	e.l.BuangCache()
	if w := e.minta("/data", e.denganKunci(e.kunci)); w.Code != 200 {
		t.Fatalf("sebelum kedaluwarsa: %d", w.Code)
	}
	e.jam.maju(31 * time.Minute)
	if w := e.minta("/data", e.denganKunci(e.kunci)); w.Code != 401 {
		t.Errorf("kunci kedaluwarsa harus ditolak walau masih di cache: %d", w.Code)
	}
}

func TestGerbangPencatatanPemakaian(t *testing.T) {
	e := buatLingkungan(t)
	e.pen.Mulai()
	n := 2
	e.s.kred[e.prefix].Override = Override{LimitPermintaan: &n}
	e.l.BuangCache()
	e.minta("/data", e.denganKunci(e.kunci))                // ok
	e.minta("/data", e.denganKunci(e.kunci))                // ok
	e.minta("/data", e.denganKunci(e.kunci))                // batas laju
	e.minta("/data", nil)                                   // tidak sah
	e.minta("/dataset/aset.satker", e.denganKunci(e.kunci)) // 429 juga (batas laju sudah habis)
	e.jam.maju(2 * time.Minute)
	n = 10 // batas dinaikkan superadmin (override klien memegang pointer yang sama)
	e.l.BuangCache()
	e.minta("/dataset/aset.satker", e.denganKunci(e.kunci)) // ditolak scope
	e.minta("/rusak", e.denganKunci(e.kunci))               // galat server
	e.minta("/tidakada", e.denganKunci(e.kunci))            // galat klien
	e.pen.Berhenti(context.Background())

	hitung := map[string]int{}
	for _, u := range e.p.salinanLog() {
		hitung[u.Hasil]++
		if u.Waktu.IsZero() || u.Metode != "GET" || u.IP == "" || u.Endpoint == "" || strings.HasPrefix(u.Endpoint, AwalanRute) {
			t.Errorf("entri tidak lengkap: %+v", u)
		}
		if u.Hasil != HasilTidakSah && u.KlienID != 5 {
			t.Errorf("klien tidak tercatat: %+v", u)
		}
		if u.Hasil == HasilTidakSah && u.KlienID != 0 {
			t.Errorf("kunci tidak dikenal tidak boleh dikaitkan ke klien: %+v", u)
		}
	}
	want := map[string]int{HasilOK: 2, HasilBatasLaju: 2, HasilTidakSah: 1, HasilDitolakScope: 1, HasilGalatServer: 1, HasilGalatKlien: 1}
	for k, v := range want {
		if hitung[k] != v {
			t.Errorf("hasil %s = %d, want %d (semua: %v)", k, hitung[k], v, hitung)
		}
	}
	for _, u := range e.p.salinanLog() {
		if u.Hasil == HasilOK && (u.Dataset != "aset.tanah" || u.Baris == nil || *u.Baris != 3) {
			t.Errorf("dataset/baris tidak tercatat: %+v", u)
		}
	}
	// Hitungan per jam konsisten dengan riwayat.
	var total int64
	for _, a := range e.p.agg {
		total += a.Jumlah
	}
	if total != 8 {
		t.Errorf("total agregat = %d, want 8", total)
	}
}

// Audit: kejadian keamanan masuk Log Audit tanpa kunci apa pun, dan hanya lewat kategori API Akses.
func TestGerbangKejadianKeamananMasukLogAudit(t *testing.T) {
	e := buatLingkungan(t)
	var mu sync.Mutex
	var entri []audit.Entri
	pr := audit.NewPerekam(func(_ context.Context, es []audit.Entri) error {
		mu.Lock()
		entri = append(entri, es...)
		mu.Unlock()
		return nil
	}, 100)
	pr.Mulai()
	audit.Pasang(pr)
	defer audit.Pasang(nil)

	_, lain, _ := BuatKunci()
	e.minta("/data", e.denganKunci(lain))                   // tidak sah
	e.minta("/dataset/aset.satker", e.denganKunci(e.kunci)) // di luar scope
	e.s.kred[e.prefix].IP = []string{"198.51.100.0/24"}
	e.l.BuangCache()
	e.minta("/data", e.denganKunci(e.kunci)) // IP ditolak
	pr.Berhenti(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if len(entri) != 3 {
		t.Fatalf("entri audit = %d, want 3: %+v", len(entri), entri)
	}
	for _, en := range entri {
		if en.Kategori != audit.KatAPIAkses {
			t.Errorf("kategori = %q", en.Kategori)
		}
		if en.Sukses {
			t.Errorf("kejadian keamanan harus bertanda gagal: %+v", en)
		}
		semua := en.Label + en.Aksi + en.Username + en.ObjekID + en.UserAgent
		for k, v := range en.Detail {
			semua += k + "=" + toString(v) + ";"
		}
		if strings.Contains(semua, e.kunci) || strings.Contains(semua, lain) || strings.Contains(semua, e.l.HashKunci(e.kunci)) {
			t.Errorf("kunci atau hash bocor ke Log Audit: %+v", en)
		}
	}
}

func toString(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
