package monitor

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestPersentilHistogram(t *testing.T) {
	var h histogram
	if persentil(h, 0.95) != 0 {
		t.Error("histogram kosong harus 0")
	}
	// 100 permintaan: 90 di ember <=10 ms, 10 di ember 500-1000 ms
	for i := 0; i < 90; i++ {
		h.tambah(8)
	}
	for i := 0; i < 10; i++ {
		h.tambah(800)
	}
	p50, p95, p99 := persentil(h, 0.50), persentil(h, 0.95), persentil(h, 0.99)
	if p50 <= 0 || p50 > 10 {
		t.Errorf("p50 = %v, want di ember 5-10 ms", p50)
	}
	if p95 <= 500 || p95 > 1000 || p99 <= p95 {
		t.Errorf("p95 = %v p99 = %v, want di ember 500-1000 ms dan p99 >= p95", p95, p99)
	}
	// lebih lambat dari batas terakhir: dilaporkan batas terakhir (">10 detik")
	var lambat histogram
	lambat.tambah(60000)
	if got := persentil(lambat, 0.95); got != 10000 {
		t.Errorf("ember terakhir = %v, want 10000", got)
	}
}

func TestRuteBerat(t *testing.T) {
	for rute, want := range map[string]bool{
		"/api/v1/digitalisasi/ekspor/:dataset": true, "/api/v1/sapa/barang/impor-siman": true, "/api/v1/inaproc/penarikan": true, "/api/v1/sapa/dokumen/:id/unduh": true,
		"/api/v1/digitalisasi/sinkronisasi": true, "/api/v1/users": false, "/api/v1/digitalisasi/data/:dataset": false, "/api/v1/auth/me": false,
	} {
		if got := ruteBerat(rute); got != want {
			t.Errorf("ruteBerat(%q) = %v, want %v", rute, got, want)
		}
	}
}

func TestMetrikMenghitungJendelaDanMengecualikanRuteBeratDariWaktuRespons(t *testing.T) {
	m := NewMetrikHTTP()
	t0 := time.Date(2026, 10, 7, 10, 0, 30, 0, time.UTC)
	for i := 0; i < 20; i++ {
		m.Catat("GET", "/api/v1/users", 200, 40*time.Millisecond, t0)
	}
	m.Catat("GET", "/api/v1/users", 500, 40*time.Millisecond, t0)
	m.Catat("GET", "/api/v1/users", 404, 40*time.Millisecond, t0)
	// ekspor memang lama: dihitung sebagai permintaan tetapi tidak memengaruhi waktu respons keseluruhan
	m.Catat("GET", "/api/v1/digitalisasi/ekspor/:dataset", 200, 9*time.Second, t0)

	j := m.JendelaMenit("x", 5, t0)
	if j.Total != 23 || j.Galat5xx != 1 || j.Galat4xx != 1 {
		t.Errorf("jendela = %+v", j)
	}
	if j.P95MS <= 0 || j.P95MS > 50 || j.RataMS < 39 || j.RataMS > 41 {
		t.Errorf("waktu respons ringan: rata %v p95 %v (ekspor 9 detik tidak boleh ikut)", j.RataMS, j.P95MS)
	}
	if j.PersenGalat < 4 || j.PersenGalat > 5 { // 1 dari 23
		t.Errorf("persen galat = %v", j.PersenGalat)
	}

	// Jendela bergeser: 10 menit kemudian jendela 5 menit kosong, tetapi sejak mulai tetap ada.
	nanti := t0.Add(10 * time.Minute)
	if j := m.JendelaMenit("x", 5, nanti); j.Total != 0 {
		t.Errorf("jendela 5 menit setelah 10 menit = %+v", j)
	}
	if j := m.JendelaMenit("x", 60, nanti); j.Total != 23 {
		t.Errorf("jendela 1 jam = %+v", j)
	}
	if j := m.SejakMulai(nanti); j.Total != 23 {
		t.Errorf("sejak mulai = %+v", j)
	}

	// Ember menit yang dipakai ulang (60 menit kemudian) tidak boleh membawa hitungan lama.
	m.Catat("GET", "/api/v1/users", 200, 10*time.Millisecond, t0.Add(60*time.Minute))
	if j := m.JendelaMenit("x", 5, t0.Add(60*time.Minute)); j.Total != 1 {
		t.Errorf("ember bekas harus bersih: %+v", j)
	}
}

func TestMetrikAmbilJendelaMengembalikanSelisih(t *testing.T) {
	m := NewMetrikHTTP()
	now := time.Now()
	m.Catat("GET", "/a", 200, time.Millisecond, now)
	m.Catat("GET", "/a", 500, time.Millisecond, now)
	d, _ := m.AmbilJendela()
	if d.Total != 2 || d.C5xx != 1 {
		t.Errorf("jendela pertama = %+v", d)
	}
	m.Catat("GET", "/a", 200, time.Millisecond, now)
	d, _ = m.AmbilJendela()
	if d.Total != 1 || d.C5xx != 0 {
		t.Errorf("jendela kedua hanya selisih: %+v", d)
	}
	if d, _ = m.AmbilJendela(); d.Total != 0 {
		t.Errorf("tanpa permintaan baru: %+v", d)
	}
}

func TestRuteTeratasUrutDanBatas(t *testing.T) {
	m := NewMetrikHTTP()
	now := time.Now()
	for i := 0; i < 10; i++ {
		m.Catat("GET", "/cepat", 200, 5*time.Millisecond, now)
		m.Catat("GET", "/lambat", 200, 2*time.Second, now)
		m.Catat("GET", "/galat", 500, 100*time.Millisecond, now)
	}
	m.Catat("GET", "/jarang", 200, 9*time.Second, now) // di bawah minimal permintaan
	lambat, galat := m.RuteTeratas(5, 5)
	if len(lambat) != 3 || lambat[0].Rute != "/lambat" || lambat[2].Rute != "/cepat" {
		t.Errorf("rute lambat = %+v", lambat)
	}
	if len(galat) != 1 || galat[0].Rute != "/galat" || galat[0].Galat5xx != 10 {
		t.Errorf("rute galat = %+v", galat)
	}
	if l, _ := m.RuteTeratas(1, 5); len(l) != 1 {
		t.Errorf("batas n = %d", len(l))
	}
}

func TestMiddlewareMencatatStatusDanMengabaikanHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lama := Metrik
	Metrik = NewMetrikHTTP()
	defer func() { Metrik = lama }()

	r := gin.New()
	r.Use(Middleware())
	r.GET("/health", func(c *gin.Context) { c.Status(200) })
	r.GET("/api/v1/ok/:id", func(c *gin.Context) { c.Status(200) })
	r.GET("/api/v1/rusak", func(c *gin.Context) { c.Status(500) })
	r.GET("/api/v1/panik", func(c *gin.Context) { panic("meledak") })
	r.Use(gin.Recovery())

	kirim := func(path string) int {
		w := httptest.NewRecorder()
		func() {
			defer func() { _ = recover() }()
			r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		}()
		return w.Code
	}
	kirim("/health")
	kirim("/api/v1/ok/1")
	kirim("/api/v1/ok/2")
	kirim("/api/v1/rusak")
	kirim("/tidak-ada")
	kirim("/api/v1/panik")

	j := Metrik.SejakMulai(time.Now())
	if j.Total != 5 { // health tidak dihitung
		t.Errorf("total = %d, want 5 (tanpa /health)", j.Total)
	}
	if j.Galat5xx != 2 { // 500 biasa dan panic
		t.Errorf("5xx = %d, want 2 (galat biasa dan panic)", j.Galat5xx)
	}
	if j.Galat4xx != 1 {
		t.Errorf("4xx = %d, want 1 (rute tidak ada)", j.Galat4xx)
	}
	if Metrik.Berjalan() != 0 {
		t.Errorf("permintaan berjalan = %d, want 0 setelah semuanya selesai", Metrik.Berjalan())
	}
	lambat, _ := Metrik.RuteTeratas(10, 1)
	var adaPola, adaTakAda bool
	for _, r := range lambat {
		adaPola = adaPola || r.Rute == "/api/v1/ok/:id" && r.Jumlah == 2 // pola rute, bukan alamat dengan nilai parameter
		adaTakAda = adaTakAda || r.Rute == "(rute tidak ada)"
	}
	if !adaPola || !adaTakAda {
		t.Errorf("rute dicatat per pola: %+v", lambat)
	}
	_ = http.StatusOK
}

func TestRuteDilacakDibatasi(t *testing.T) {
	m := NewMetrikHTTP()
	now := time.Now()
	for i := 0; i < maksRuteDilacak+50; i++ {
		m.Catat("GET", "/r/"+time.Duration(i).String(), 200, time.Millisecond, now)
	}
	if len(m.rute) > maksRuteDilacak {
		t.Errorf("rute dilacak = %d, batas %d", len(m.rute), maksRuteDilacak)
	}
	if j := m.SejakMulai(now); j.Total != int64(maksRuteDilacak+50) {
		t.Errorf("hitungan total tetap lengkap: %d", j.Total)
	}
}
