package serverlog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http/httptest"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// penampung: konsol palsu yang aman dipakai bersamaan.
type penampung struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (p *penampung) Write(x []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.b.Write(x)
}
func (p *penampung) String() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.b.String()
}

// simpanan: penyimpanan palsu untuk perekam.
type simpanan struct {
	mu     sync.Mutex
	semua  []Gabungan
	gagal  int // sisa pemanggilan yang harus gagal
	jumlah int
}

func (s *simpanan) simpan(_ context.Context, gs []Gabungan) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jumlah++
	if s.gagal > 0 {
		s.gagal--
		return errors.New("database tidak tersedia")
	}
	s.semua = append(s.semua, gs...)
	return nil
}
func (s *simpanan) salinan() []Gabungan {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Gabungan(nil), s.semua...)
}

func perekamCepat(s *simpanan, kapasitas int) *Perekam {
	r := NewPerekam(s.simpan, kapasitas)
	r.interval = 10 * time.Millisecond
	r.lapor = func(string) {}
	return r
}

func tunggu(t *testing.T, kondisi func() bool) {
	t.Helper()
	akhir := time.Now().Add(3 * time.Second)
	for time.Now().Before(akhir) {
		if kondisi() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("kondisi tidak terpenuhi dalam 3 detik")
}

func TestKlasifikasi(t *testing.T) {
	kasus := []struct {
		tag    string
		level  Level
		sumber string
		siklus bool
	}{
		{"INAPROC SYNC ERROR", Error, "INAPROC SYNC", false},
		{"ERROR", Error, "", false},
		{"FATAL", Error, "", false},
		{"AUTH WARN", Warn, "AUTH", false},
		{"WARN", Warn, "", false},
		{"INAPROC SYNC DEBUG", Debug, "INAPROC SYNC", false},
		{"INFO", Info, "", true},
		{"SSO INFO", Info, "SSO", false},
		{"SSO SUCCESS", Info, "SSO", false},
		{"DIGITALISASI", Info, "DIGITALISASI", false},
		{"", Info, "", false},
	}
	for _, k := range kasus {
		l, s, sk := klasifikasi(k.tag)
		if l != k.level || s != k.sumber || sk != k.siklus {
			t.Errorf("klasifikasi(%q) = (%s, %q, %v), mau (%s, %q, %v)", k.tag, l, s, sk, k.level, k.sumber, k.siklus)
		}
	}
}

func TestLepasWaktuDanTag(t *testing.T) {
	rest := lepasWaktu("2026/10/13 08:37:38 [INAPROC SYNC ERROR] gagal tarik: timeout")
	if rest != "[INAPROC SYNC ERROR] gagal tarik: timeout" {
		t.Fatalf("lepasWaktu: %q", rest)
	}
	if lepasWaktu("2026/10/13 08:37:38.123456 [WARN] x") != "[WARN] x" {
		t.Fatal("lepasWaktu dengan mikrodetik")
	}
	if lepasWaktu("tanpa waktu") != "tanpa waktu" {
		t.Fatal("tanpa waktu harus utuh")
	}
	tag, pesan := pisahTag("[INAPROC SYNC ERROR] gagal tarik")
	if tag != "INAPROC SYNC ERROR" || pesan != "gagal tarik" {
		t.Fatalf("pisahTag: %q %q", tag, pesan)
	}
	if tag, pesan := pisahTag("tanpa tag [x]"); tag != "" || pesan != "tanpa tag [x]" {
		t.Fatalf("tanpa tag: %q %q", tag, pesan)
	}
}

// Hanya error dan warning yang sampai ke penyimpanan; info dan debug tidak pernah, berapa pun banyaknya, dan di level warn tidak dicetak juga.
func TestHanyaErrorDanWarningDisimpan(t *testing.T) {
	kon := &penampung{}
	p := NewPencatat(kon, Warn, SimpanGalatDanPeringatan)
	s := &simpanan{}
	r := perekamCepat(s, 100)
	r.Mulai()
	p.PasangPerekam(r)

	l := log.New(p, "", log.LstdFlags)
	l.Println("[INAPROC SYNC DEBUG] detail 1")
	l.Printf("[DIGITALISASI] baris %d", 5)
	l.Println("[SSO SUCCESS] masuk")
	l.Println("[INAPROC SYNC ERROR] gagal tarik dataset 7")
	l.Println("[AUTH WARN] token ditolak")
	l.Println("[INFO] PASTI V3 Backend berjalan di port: 8686")

	tunggu(t, func() bool { return len(s.salinan()) == 2 })
	for _, g := range s.salinan() {
		if g.Level != Error && g.Level != Warn {
			t.Fatalf("level %s tidak boleh disimpan", g.Level)
		}
	}
	out := kon.String()
	if strings.Contains(out, "detail 1") || strings.Contains(out, "baris 5") || strings.Contains(out, "masuk") {
		t.Fatalf("info/debug tidak boleh dicetak pada level warn:\n%s", out)
	}
	if !strings.Contains(out, "[INAPROC SYNC ERROR]") || !strings.Contains(out, "[AUTH WARN]") {
		t.Fatalf("error dan warning harus dicetak:\n%s", out)
	}
	if !strings.Contains(out, "PASTI V3 Backend berjalan") {
		t.Fatalf("pesan siklus hidup [INFO] harus tetap dicetak:\n%s", out)
	}
}

func TestLevelInfoMencetakInfoTetapTidakMenyimpannya(t *testing.T) {
	kon := &penampung{}
	p := NewPencatat(kon, Info, SimpanGalatDanPeringatan)
	s := &simpanan{}
	r := perekamCepat(s, 100)
	r.Mulai()
	p.PasangPerekam(r)
	log.New(p, "", log.LstdFlags).Println("[DIGITALISASI] sinkronisasi dimulai")
	log.New(p, "", log.LstdFlags).Println("[INAPROC SYNC DEBUG] x")
	if !strings.Contains(kon.String(), "sinkronisasi dimulai") {
		t.Fatal("info harus dicetak pada LOG_LEVEL=info")
	}
	if strings.Contains(kon.String(), "INAPROC SYNC DEBUG") {
		t.Fatal("debug tidak dicetak pada LOG_LEVEL=info")
	}
	time.Sleep(60 * time.Millisecond)
	if n := len(s.salinan()); n != 0 {
		t.Fatalf("info tidak boleh disimpan, tersimpan %d", n)
	}
}

func TestModeSimpan(t *testing.T) {
	for _, k := range []struct {
		mode  ModeSimpan
		level Level
		mau   bool
	}{
		{SimpanGalatDanPeringatan, Error, true}, {SimpanGalatDanPeringatan, Warn, true}, {SimpanGalatDanPeringatan, Info, false},
		{SimpanGalat, Error, true}, {SimpanGalat, Warn, false},
		{SimpanMati, Error, false},
	} {
		if k.mode.boleh(k.level) != k.mau {
			t.Errorf("mode %s level %s: harusnya %v", k.mode, k.level, k.mau)
		}
	}
	if BacaModeSimpan("error") != SimpanGalat || BacaModeSimpan("off") != SimpanMati || BacaModeSimpan("") != SimpanGalatDanPeringatan || BacaModeSimpan("xyz") != SimpanGalatDanPeringatan {
		t.Fatal("BacaModeSimpan")
	}
	if BacaLevel("WARNING", Info) != Warn || BacaLevel("", Info) != Info || BacaLevel("aneh", Debug) != Debug {
		t.Fatal("BacaLevel")
	}
}

func TestSamarkanRahasia(t *testing.T) {
	kasus := map[string]string{
		"gagal: Authorization: Bearer abc.def.ghi":                      "abc.def.ghi",
		"koneksi password=Rahasia123;port=1433 gagal":                   "Rahasia123",
		"GET /x?token=abcdef123456&y=1":                                 "abcdef123456",
		"callback ?code=KODE-SSO-123&state=xyz":                         "KODE-SSO-123",
		"jwt eyJhbGciOiJIUzI1NiJ9.eyJ1c2VyX2lkIjoiMSJ9.c2lnbmF0dXJl ok": "eyJhbGciOiJIUzI1NiJ9",
		`json {"client_secret": "sangat-rahasia"}`:                      "sangat-rahasia",
	}
	for masuk, rahasia := range kasus {
		out := Samarkan(masuk)
		if strings.Contains(out, rahasia) {
			t.Errorf("Samarkan(%q) masih memuat %q: %q", masuk, rahasia, out)
		}
		if !strings.Contains(out, tersamar) {
			t.Errorf("Samarkan(%q) tidak menandai penyamaran: %q", masuk, out)
		}
	}
	// Teks biasa tidak boleh rusak.
	if s := "gagal membaca baris 12 dari tabel users: timeout"; Samarkan(s) != s {
		t.Fatalf("teks biasa berubah: %q", Samarkan(s))
	}
}

func TestLogTidakMemuatRahasiaDiKonsolMaupunPenyimpanan(t *testing.T) {
	kon := &penampung{}
	p := NewPencatat(kon, Warn, SimpanGalatDanPeringatan)
	s := &simpanan{}
	r := perekamCepat(s, 10)
	r.Mulai()
	p.PasangPerekam(r)
	log.New(p, "", 0).Println("[SSO ERROR] tukar kode gagal: https://sso/token?code=KODE-RAHASIA&client_secret=RAHASIA2")
	tunggu(t, func() bool { return len(s.salinan()) == 1 })
	for _, teks := range []string{kon.String(), s.salinan()[0].Pesan} {
		if strings.Contains(teks, "KODE-RAHASIA") || strings.Contains(teks, "RAHASIA2") {
			t.Fatalf("rahasia bocor: %q", teks)
		}
	}
}

func TestSidikMenyeragamkanAngka(t *testing.T) {
	a := Sidik(Error, "X", "", "gagal membaca baris 12 id 550e8400-e29b-41d4-a716-446655440000")
	b := Sidik(Error, "X", "", "gagal membaca baris 98 id 123e4567-e89b-12d3-a456-426614174000")
	if a != b {
		t.Fatal("pesan yang sama selain angka/UUID harus punya sidik jari yang sama")
	}
	if a == Sidik(Warn, "X", "", "gagal membaca baris 12 id 550e8400-e29b-41d4-a716-446655440000") {
		t.Fatal("level berbeda harus beda sidik jari")
	}
	if a == Sidik(Error, "Y", "", "gagal membaca baris 12") {
		t.Fatal("sumber berbeda harus beda sidik jari")
	}
	if len(a) != 40 {
		t.Fatalf("panjang sidik jari %d", len(a))
	}
}

// Seribu kejadian yang sama (angka berbeda) menjadi satu kelompok dengan hitungan 1000, bukan seribu baris.
func TestPerekamMenggabungKejadianBerulang(t *testing.T) {
	s := &simpanan{}
	r := perekamCepat(s, 5000)
	r.Mulai()
	for i := 0; i < 1000; i++ {
		e := siapkan(Entri{Waktu: time.Now(), Level: Error, Sumber: "INAPROC SYNC", Pesan: fmt.Sprintf("gagal tarik halaman %d", i)})
		if !r.Catat(e) {
			t.Fatal("antrean tidak boleh penuh")
		}
	}
	r.Berhenti(context.Background())
	var total, kelompok int
	for _, g := range s.salinan() {
		total += g.Jumlah
		kelompok++
	}
	if total != 1000 {
		t.Fatalf("jumlah kejadian tersimpan %d, mau 1000", total)
	}
	if kelompok > 3 { // satu kelompok; sedikit toleransi bila tertulis di tengah jalan
		t.Fatalf("seharusnya digabung, tetapi ada %d kelompok", kelompok)
	}
	if st := r.Statistik(); st.Ditulis != 1000 || st.Dijatuhkan != 0 {
		t.Fatalf("statistik: %+v", st)
	}
}

// Database mati: perekam tidak memblokir, menyimpan kelompok di memori (berbatas), dan menulisnya begitu database pulih.
func TestPerekamBertahanSaatDatabaseMati(t *testing.T) {
	s := &simpanan{gagal: 1}
	r := perekamCepat(s, 100)
	r.Mulai()
	r.Catat(siapkan(Entri{Waktu: time.Now(), Level: Error, Sumber: "DB", Pesan: "koneksi putus"}))
	tunggu(t, func() bool { return r.Statistik().Gagal >= 1 })
	if len(s.salinan()) != 0 {
		t.Fatal("belum boleh tertulis")
	}
	if r.Statistik().Tertunda != 1 {
		t.Fatalf("kelompok harus menunggu di memori: %+v", r.Statistik())
	}
	// Setelah jeda berlalu dan database pulih, tertulis.
	r.Berhenti(context.Background())
	if len(s.salinan()) != 1 || s.salinan()[0].Jumlah != 1 {
		t.Fatalf("setelah pulih harus tertulis: %+v", s.salinan())
	}
}

// Badai galat yang berbeda-beda tidak membuat memori tumbuh tanpa batas: yang melebihi penampung dijatuhkan dan dihitung.
func TestPerekamMemoriBerbatas(t *testing.T) {
	s := &simpanan{gagal: 1 << 30} // database tidak pernah pulih
	r := perekamCepat(s, 50)
	r.maksTertunda = 20
	r.Mulai()
	for i := 0; i < 500; i++ {
		// Pesan yang benar-benar berbeda (bukan hanya angka) supaya sidik jarinya berbeda.
		r.Catat(siapkan(Entri{Waktu: time.Now(), Level: Error, Sumber: "S", Pesan: "galat jenis " + strings.Repeat("x", i%40) + string(rune('a'+i%26))}))
		time.Sleep(100 * time.Microsecond)
	}
	time.Sleep(100 * time.Millisecond)
	st := r.Statistik()
	if st.Tertunda > 20 {
		t.Fatalf("penampung melebihi batas: %+v", st)
	}
	if st.Dijatuhkan == 0 {
		t.Fatalf("yang melebihi batas harus dihitung dijatuhkan: %+v", st)
	}
}

func TestPerekamAntreanPenuhTidakMemblokir(t *testing.T) {
	r := NewPerekam(func(context.Context, []Gabungan) error { return nil }, 2)
	// Tanpa Mulai: tidak ada yang mengosongkan antrean.
	selesai := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			r.Catat(Entri{Level: Error})
		}
		close(selesai)
	}()
	select {
	case <-selesai:
	case <-time.After(2 * time.Second):
		t.Fatal("Catat memblokir saat antrean penuh")
	}
	if st := r.Statistik(); st.Dijatuhkan != 98 || st.Diterima != 2 {
		t.Fatalf("statistik: %+v", st)
	}
}

func TestCatatSetelahBerhentiTidakPanik(t *testing.T) {
	r := perekamCepat(&simpanan{}, 10)
	r.Mulai()
	r.Berhenti(context.Background())
	if r.Catat(Entri{Level: Error}) {
		t.Fatal("tidak boleh menerima setelah berhenti")
	}
	r.Berhenti(context.Background()) // dua kali aman
}

// Kejadian yang sama dicetak sekali per jendela; sisanya dihitung dan dilaporkan pada cetakan berikutnya.
func TestKonsolMenekanPengulangan(t *testing.T) {
	kon := &penampung{}
	p := NewPencatat(kon, Warn, SimpanMati)
	t0 := time.Now()
	for i := 0; i < 500; i++ {
		p.Catat(Entri{Waktu: t0.Add(time.Duration(i) * time.Millisecond), Level: Error, Sumber: "DB", Pesan: fmt.Sprintf("koneksi putus (percobaan %d)", i)})
	}
	if n := strings.Count(kon.String(), "koneksi putus"); n != 1 {
		t.Fatalf("500 kejadian yang sama harus tercetak sekali, tercetak %d kali", n)
	}
	// Setelah jendela berlalu, cetakan berikutnya menyebut jumlah yang ditekan.
	p.Catat(Entri{Waktu: t0.Add(11 * time.Second), Level: Error, Sumber: "DB", Pesan: "koneksi putus (percobaan 999)"})
	if !strings.Contains(kon.String(), "499 kejadian serupa sebelumnya tidak dicetak") {
		t.Fatalf("jumlah yang ditekan tidak dilaporkan:\n%s", kon.String())
	}
	// Kejadian berbeda tetap tercetak.
	p.Catat(Entri{Waktu: t0, Level: Error, Sumber: "DB", Pesan: "pesan lain sama sekali"})
	if !strings.Contains(kon.String(), "pesan lain sama sekali") {
		t.Fatal("kejadian berbeda harus tercetak")
	}
}

func TestKejadianSebelumPerekamTerpasangTidakHilang(t *testing.T) {
	p := NewPencatat(&penampung{}, Warn, SimpanGalatDanPeringatan)
	p.Catat(Entri{Level: Warn, Sumber: "AWAL", Pesan: ".env tidak ditemukan"})
	s := &simpanan{}
	r := perekamCepat(s, 10)
	r.Mulai()
	p.PasangPerekam(r)
	tunggu(t, func() bool { return len(s.salinan()) == 1 })
	if s.salinan()[0].Sumber != "AWAL" {
		t.Fatalf("%+v", s.salinan()[0])
	}
}

func TestPesanBerbarisBanyakMenjadiDetail(t *testing.T) {
	p := NewPencatat(&penampung{}, Warn, SimpanGalatDanPeringatan)
	s := &simpanan{}
	r := perekamCepat(s, 10)
	r.Mulai()
	p.PasangPerekam(r)
	log.New(p, "", 0).Println("[HRIS2 ERROR] gagal\nbaris kedua\nbaris ketiga")
	tunggu(t, func() bool { return len(s.salinan()) == 1 })
	g := s.salinan()[0]
	if g.Pesan != "gagal" || !strings.Contains(g.Detail, "baris kedua") {
		t.Fatalf("pesan=%q detail=%q", g.Pesan, g.Detail)
	}
}

func TestPotongDanBersihkan(t *testing.T) {
	s := strings.Repeat("é", 5000)
	e := siapkan(Entri{Level: Error, Pesan: s, Detail: strings.Repeat("x", 20000), Sumber: strings.Repeat("S", 300)})
	if n := len([]rune(e.Pesan)); n != maksPesan {
		t.Fatalf("pesan %d rune", n)
	}
	if n := len([]rune(e.Detail)); n != maksDetail {
		t.Fatalf("detail %d rune", n)
	}
	if n := len([]rune(e.Sumber)); n != maksSumber {
		t.Fatalf("sumber %d rune", n)
	}
	if e2 := siapkan(Entri{Level: Error, Pesan: "a\x00b\xffc"}); strings.ContainsAny(e2.Pesan, "\x00\xff") {
		t.Fatalf("karakter tidak sah tersisa: %q", e2.Pesan)
	}
	if siapkan(Entri{Level: Error}).Sumber != "SERVER" {
		t.Fatal("sumber kosong harus jadi SERVER")
	}
}

// ---------------------------------------------------------------- middleware HTTP

func pasangUji(t *testing.T) (*simpanan, *Perekam) {
	t.Helper()
	lama := std.Load()
	p := NewPencatat(&penampung{}, Warn, SimpanGalatDanPeringatan)
	s := &simpanan{}
	r := perekamCepat(s, 100)
	r.Mulai()
	p.PasangPerekam(r)
	std.Store(p)
	t.Cleanup(func() {
		std.Store(lama)
		r.Berhenti(context.Background())
	})
	return s, r
}

func routerUji(lambat time.Duration) *gin.Engine {
	r := gin.New()
	r.Use(Middleware(lambat), Pemulih())
	r.GET("/ok", func(c *gin.Context) { c.String(200, "ok") })
	r.GET("/tidak-ada-akses", func(c *gin.Context) { c.String(403, "x") })
	r.GET("/salah-klien", func(c *gin.Context) { c.String(400, "x") })
	r.GET("/rusak", func(c *gin.Context) { c.String(500, "x") })
	r.GET("/panik", func(c *gin.Context) { panic("sesuatu yang tidak terduga") })
	r.GET("/lambat", func(c *gin.Context) { time.Sleep(40 * time.Millisecond); c.String(200, "x") })
	r.GET("/berat", func(c *gin.Context) { c.Set(KunciBerat, true); time.Sleep(40 * time.Millisecond); c.String(200, "x") })
	r.GET("/health", func(c *gin.Context) { c.String(500, "x") })
	return r
}

func panggil(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	return w
}

// Tidak ada log akses: permintaan sukses dan galat klien (4xx) tidak dicatat sama sekali.
func TestMiddlewareTidakMencatatPermintaanNormal(t *testing.T) {
	s, r := pasangUji(t)
	rt := routerUji(time.Second)
	for i := 0; i < 50; i++ {
		panggil(rt, "/ok")
		panggil(rt, "/salah-klien")
		panggil(rt, "/tidak-ada-akses")
		panggil(rt, "/tidak-ada-rute")
	}
	r.Berhenti(context.Background())
	if n := len(s.salinan()); n != 0 {
		t.Fatalf("permintaan normal/4xx tidak boleh dicatat, tercatat %d: %+v", n, s.salinan())
	}
}

func TestMiddlewareMencatat5xxSebagaiError(t *testing.T) {
	s, r := pasangUji(t)
	rt := routerUji(time.Second)
	for i := 0; i < 10; i++ {
		panggil(rt, "/rusak")
	}
	r.Berhenti(context.Background())
	gs := s.salinan()
	if len(gs) != 1 || gs[0].Jumlah != 10 {
		t.Fatalf("10 respons 500 yang sama harus jadi satu kelompok berjumlah 10: %+v", gs)
	}
	g := gs[0]
	if g.Level != Error || g.Sumber != "HTTP" || g.Rute != "/rusak" || g.Metode != "GET" || g.Status != 500 {
		t.Fatalf("entri tidak sesuai: %+v", g.Entri)
	}
}

func TestMiddlewareTidakMencatatHealth(t *testing.T) {
	s, r := pasangUji(t)
	panggil(routerUji(time.Second), "/health")
	r.Berhenti(context.Background())
	if len(s.salinan()) != 0 {
		t.Fatal("/health tidak boleh dicatat")
	}
}

func TestMiddlewarePanikDitangkap(t *testing.T) {
	s, r := pasangUji(t)
	rt := routerUji(time.Second)
	w := panggil(rt, "/panik")
	if w.Code != 500 || !strings.Contains(w.Body.String(), "Terjadi kesalahan pada server") {
		t.Fatalf("klien harus menerima 500 yang rapi: %d %s", w.Code, w.Body.String())
	}
	// Server tetap hidup dan melayani permintaan berikutnya.
	if w := panggil(rt, "/ok"); w.Code != 200 {
		t.Fatalf("server harus tetap melayani: %d", w.Code)
	}
	r.Berhenti(context.Background())
	gs := s.salinan()
	if len(gs) != 1 {
		t.Fatalf("panik harus tercatat satu kali (tanpa entri HTTP ganda): %+v", gs)
	}
	g := gs[0]
	if g.Level != Error || g.Sumber != "PANIK" || !strings.Contains(g.Pesan, "sesuatu yang tidak terduga") || g.Status != 500 || g.Rute != "/panik" {
		t.Fatalf("entri panik: %+v", g.Entri)
	}
	if !strings.Contains(g.Detail, "serverlog_test.go") {
		t.Fatalf("jejak tumpukan harus ada:\n%s", g.Detail)
	}
}

func TestMiddlewareLambatDanRuteBerat(t *testing.T) {
	s, r := pasangUji(t)
	rt := routerUji(10 * time.Millisecond)
	panggil(rt, "/lambat")
	panggil(rt, "/berat") // rute berat tidak dianggap lambat
	panggil(rt, "/ok")
	r.Berhenti(context.Background())
	gs := s.salinan()
	if len(gs) != 1 || gs[0].Level != Warn || gs[0].Rute != "/lambat" || gs[0].DurasiMS < 10 {
		t.Fatalf("hanya /lambat yang dicatat sebagai warning: %+v", gs)
	}
	// Batas 0 mematikan catatan lambat.
	s2, r2 := pasangUji(t)
	panggil(routerUji(0), "/lambat")
	r2.Berhenti(context.Background())
	if len(s2.salinan()) != 0 {
		t.Fatal("lambat=0 harus mematikan catatan permintaan lambat")
	}
}

func TestPemulihHormatiSudahDicatat(t *testing.T) {
	s, r := pasangUji(t)
	rt := gin.New()
	rt.Use(Middleware(time.Second))
	rt.GET("/beban", func(c *gin.Context) { c.Set(KunciSudahDicatat, true); c.String(503, "sibuk") })
	panggil(rt, "/beban")
	r.Berhenti(context.Background())
	if len(s.salinan()) != 0 {
		t.Fatal("permintaan yang sudah dicatat penangannya tidak boleh dicatat ulang")
	}
}

// Banyak goroutine mencatat bersamaan (dijalankan dengan -race di Linux): tidak ada kehilangan hitungan.
func TestCatatBersamaan(t *testing.T) {
	s := &simpanan{}
	r := perekamCepat(s, 20000)
	r.Mulai()
	p := NewPencatat(&penampung{}, Warn, SimpanGalatDanPeringatan)
	p.PasangPerekam(r)
	var wg sync.WaitGroup
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				p.Catat(Entri{Level: Error, Sumber: "RACE", Pesan: fmt.Sprintf("galat %d", i%3)})
				log.New(p, "", 0).Println("[RACE WARN] peringatan bersamaan")
			}
		}(g)
	}
	wg.Wait()
	r.Berhenti(context.Background())
	var total int
	for _, g := range s.salinan() {
		total += g.Jumlah
	}
	if total != 20*200*2 {
		t.Fatalf("kejadian tersimpan %d, mau %d (dijatuhkan %d)", total, 20*200*2, r.Statistik().Dijatuhkan)
	}
}

func TestPutusDeteksi(t *testing.T) {
	if putus("bukan error") || putus(errors.New("lain")) {
		t.Fatal("bukan pemutusan klien")
	}
	if !putus(&net.OpError{Op: "write", Err: syscall.EPIPE}) {
		t.Fatal("broken pipe harus terdeteksi")
	}
	if !putus(fmt.Errorf("tulis respons: %w", &net.OpError{Op: "write", Err: errors.New("connection reset by peer")})) {
		t.Fatal("connection reset harus terdeteksi")
	}
}

func TestPembatalanKlienTidakDicatat(t *testing.T) {
	kon := &penampung{}
	p := NewPencatat(kon, Warn, SimpanGalatDanPeringatan)
	s := &simpanan{}
	r := perekamCepat(s, 10)
	r.Mulai()
	p.PasangPerekam(r)
	log.New(p, "", 0).Println("[USER ERROR] gagal membaca daftar pengguna: context canceled")
	log.New(p, "", 0).Println("[USER ERROR] gagal membaca daftar pengguna: context deadline exceeded")
	r.Berhenti(context.Background())
	if strings.Contains(kon.String(), "context canceled") {
		t.Fatalf("pembatalan klien tidak boleh dicetak: %s", kon.String())
	}
	gs := s.salinan()
	if len(gs) != 1 || !strings.Contains(gs[0].Pesan, "deadline exceeded") {
		t.Fatalf("hanya batas waktu (masalah sungguhan) yang dicatat: %+v", gs)
	}
	if p.Statistik().DibatalkanKlien != 1 {
		t.Fatalf("pembatalan harus dihitung: %+v", p.Statistik())
	}
}
