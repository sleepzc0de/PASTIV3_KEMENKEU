package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/serverlog"
)

func TestPembatasLajuLonjakanLaluTolak(t *testing.T) {
	p := NewPembatasLaju(10, 5, 100)
	t0 := time.Now()
	for i := 0; i < 5; i++ {
		if ok, _ := p.Izin("1.1.1.1", t0); !ok {
			t.Fatalf("permintaan %d dalam lonjakan harus diizinkan", i+1)
		}
	}
	ok, tunggu := p.Izin("1.1.1.1", t0)
	if ok {
		t.Fatal("permintaan ke-6 harus ditolak")
	}
	if tunggu <= 0 || tunggu > 200*time.Millisecond {
		t.Fatalf("waktu tunggu tidak masuk akal: %v", tunggu)
	}
	// Kunci lain tidak terpengaruh.
	if ok, _ := p.Izin("2.2.2.2", t0); !ok {
		t.Fatal("IP lain harus tetap boleh")
	}
	// Setelah 100 ms terisi ulang 1 token (10/detik).
	if ok, _ := p.Izin("1.1.1.1", t0.Add(100*time.Millisecond)); !ok {
		t.Fatal("setelah terisi ulang harus boleh lagi")
	}
	if ok, _ := p.Izin("1.1.1.1", t0.Add(100*time.Millisecond)); ok {
		t.Fatal("hanya satu token yang terisi")
	}
}

func TestPembatasLajuMemoriBerbatas(t *testing.T) {
	p := NewPembatasLaju(1, 2, 100)
	t0 := time.Now()
	// 10 ribu alamat berbeda (serangan memenuhi memori): tabel tidak melewati batas, dan pengguna sah tidak ditolak.
	for i := 0; i < 10000; i++ {
		ip := "10." + string(rune('0'+i%10)) + "." + strings.Repeat("x", i%7) + "." + time.Duration(i).String()
		if ok, _ := p.Izin(ip, t0); !ok {
			t.Fatalf("alamat baru tidak boleh ditolak (i=%d)", i)
		}
		if p.Ukuran() > 100 {
			t.Fatalf("ukuran tabel %d melewati batas 100", p.Ukuran())
		}
	}
	// Ember yang sudah lama penuh kembali dibersihkan sehingga tabel bisa dipakai lagi.
	later := t0.Add(time.Minute)
	if ok, _ := p.Izin("alamat-sah", later); !ok {
		t.Fatal("harus boleh")
	}
	if p.Ukuran() > 100 {
		t.Fatalf("ukuran %d", p.Ukuran())
	}
}

func TestPembatasLajuBersamaan(t *testing.T) {
	p := NewPembatasLaju(1, 50, 100)
	var diizinkan atomic.Int64
	var wg sync.WaitGroup
	t0 := time.Now()
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				if ok, _ := p.Izin("sama", t0); ok {
					diizinkan.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	if n := diizinkan.Load(); n != 50 {
		t.Fatalf("dari 2000 permintaan serentak harus tepat 50 (lonjakan) yang lolos, lolos %d", n)
	}
}

func routerUjiKetahanan(mw ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mw...)
	r.GET("/x", func(c *gin.Context) { c.String(200, "ok") })
	r.GET("/health", func(c *gin.Context) { c.String(200, "ok") })
	r.POST("/unggah", func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			c.String(400, "terlalu besar")
			return
		}
		c.String(200, "ok")
	})
	return r
}

func TestBatasLajuMiddleware429(t *testing.T) {
	rt := routerUjiKetahanan(BatasLaju(NewPembatasLaju(1, 3, 10), "pelan-pelan"))
	kode := func(path string) (int, string, string) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		req.RemoteAddr = "9.9.9.9:1234"
		rt.ServeHTTP(w, req)
		return w.Code, w.Header().Get("Retry-After"), w.Body.String()
	}
	for i := 0; i < 3; i++ {
		if c, _, _ := kode("/x"); c != 200 {
			t.Fatalf("permintaan %d: %d", i+1, c)
		}
	}
	c, ra, body := kode("/x")
	if c != 429 || ra == "" || !strings.Contains(body, "pelan-pelan") || !strings.Contains(body, KodeTerlaluBanyak) {
		t.Fatalf("harus 429 dengan Retry-After dan pesan: %d %q %s", c, ra, body)
	}
	if c, _, _ := kode("/health"); c != 200 {
		t.Fatalf("/health tidak boleh dibatasi: %d", c)
	}
	// Pembatas nil = tidak membatasi.
	rt2 := routerUjiKetahanan(BatasLaju(nil, ""))
	for i := 0; i < 50; i++ {
		w := httptest.NewRecorder()
		rt2.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
		if w.Code != 200 {
			t.Fatal("tanpa pembatas harus selalu lolos")
		}
	}
}

// Dua permintaan memegang slot; permintaan ketiga menunggu lalu ditolak 503 setelah batas tunggu; yang keempat setelah slot kosong lolos.
func TestBatasSerentakMenolakBilaPenuh(t *testing.T) {
	lepas := make(chan struct{})
	masuk := make(chan struct{}, 10)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BatasSerentak(2, 80*time.Millisecond))
	r.GET("/lama", func(c *gin.Context) { masuk <- struct{}{}; <-lepas; c.String(200, "ok") })
	r.GET("/cepat", func(c *gin.Context) { c.String(200, "ok") })
	r.GET("/health", func(c *gin.Context) { c.String(200, "ok") })

	hasil := make(chan int, 10)
	for i := 0; i < 2; i++ {
		go func() {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/lama", nil))
			hasil <- w.Code
		}()
	}
	<-masuk
	<-masuk
	// Slot penuh.
	w := httptest.NewRecorder()
	mulai := time.Now()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/cepat", nil))
	if w.Code != 503 || w.Header().Get("Retry-After") == "" || !strings.Contains(w.Body.String(), KodeSibuk) {
		t.Fatalf("harus 503: %d %s", w.Code, w.Body.String())
	}
	if d := time.Since(mulai); d < 70*time.Millisecond {
		t.Fatalf("seharusnya menunggu dulu sebelum menolak: %v", d)
	}
	// /health tetap menjawab saat penuh.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))
	if w.Code != 200 {
		t.Fatalf("/health harus tetap menjawab saat penuh: %d", w.Code)
	}
	close(lepas)
	<-hasil
	<-hasil
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/cepat", nil))
	if w.Code != 200 {
		t.Fatalf("setelah slot kosong harus lolos: %d", w.Code)
	}
	if StatistikKetahanan().DitolakBeban < 1 {
		t.Fatal("penolakan harus dihitung")
	}
}

// Lonjakan singkat tidak ditolak: permintaan yang menunggu mendapat slot begitu ada yang selesai.
func TestBatasSerentakAntreSebentar(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BatasSerentak(1, 2*time.Second))
	r.GET("/x", func(c *gin.Context) { time.Sleep(30 * time.Millisecond); c.String(200, "ok") })
	var wg sync.WaitGroup
	var tolak atomic.Int64
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
			if w.Code != 200 {
				tolak.Add(1)
			}
		}()
	}
	wg.Wait()
	if tolak.Load() != 0 {
		t.Fatalf("%d permintaan ditolak padahal hanya lonjakan singkat", tolak.Load())
	}
}

func TestBeratBerbagiBatasDanMenandai(t *testing.T) {
	berat := Berat(1, 60*time.Millisecond)
	lepas := make(chan struct{})
	masuk := make(chan struct{}, 2)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var ditandai atomic.Bool
	r.GET("/ekspor-a", berat, func(c *gin.Context) {
		ditandai.Store(c.GetBool(serverlog.KunciBerat))
		masuk <- struct{}{}
		<-lepas
		c.String(200, "ok")
	})
	r.GET("/ekspor-b", berat, func(c *gin.Context) { c.String(200, "ok") })
	selesai := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/ekspor-a", nil))
		selesai <- w.Code
	}()
	<-masuk
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/ekspor-b", nil))
	if w.Code != 503 {
		t.Fatalf("rute berat lain harus ditolak saat batas bersama penuh: %d", w.Code)
	}
	close(lepas)
	if <-selesai != 200 {
		t.Fatal("yang berjalan harus selesai")
	}
	if !ditandai.Load() {
		t.Fatal("rute berat harus ditandai KunciBerat")
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/ekspor-b", nil))
	if w.Code != 200 {
		t.Fatalf("setelah selesai harus boleh: %d", w.Code)
	}
}

func TestBatasBadan(t *testing.T) {
	rt := routerUjiKetahanan(BatasBadan(100))
	kecil := httptest.NewRecorder()
	rt.ServeHTTP(kecil, httptest.NewRequest("POST", "/unggah", strings.NewReader(strings.Repeat("a", 50))))
	if kecil.Code != 200 {
		t.Fatalf("badan kecil harus lolos: %d", kecil.Code)
	}
	besar := httptest.NewRecorder()
	rt.ServeHTTP(besar, httptest.NewRequest("POST", "/unggah", strings.NewReader(strings.Repeat("a", 500))))
	if besar.Code != http.StatusBadRequest {
		t.Fatalf("badan melebihi batas harus gagal dibaca: %d", besar.Code)
	}
}
