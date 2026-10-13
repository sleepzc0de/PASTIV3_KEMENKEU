package middleware

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/peran"
	"pasti-v3-backend/utils"
)

// pasangDenganCache seperti pasang, tetapi cache autentikasi hidup (ttl detik) dan pemanggilan ke "database" dihitung.
func pasangDenganCache(t *testing.T, ttl int, k *kondisiAkun) (statusDipanggil, peranDipanggil *atomic.Int64) {
	t.Helper()
	pasang(t, k)
	config.Cfg.AuthCacheDetik = ttl
	InvalidasiAkun()
	t.Cleanup(InvalidasiAkun)
	statusDipanggil, peranDipanggil = &atomic.Int64{}, &atomic.Int64{}
	var mu sync.Mutex
	muatStatusAkun = func(context.Context, string) (StatusAkun, error) {
		statusDipanggil.Add(1)
		mu.Lock()
		defer mu.Unlock()
		return k.status, k.err
	}
	muatPeran = func(_ context.Context, _ string, role string) (peran.Efektif, error) {
		peranDipanggil.Add(1)
		mu.Lock()
		defer mu.Unlock()
		return peran.Selesaikan(role, k.baris), nil
	}
	return
}

func panggilCache(t *testing.T) int {
	t.Helper()
	kode, _, _ := panggilGerbang(t, AuthRequired(), "user", false)
	return kode
}

func akunBiasa() *kondisiAkun {
	return &kondisiAkun{
		status: StatusAkun{Aktif: true, Role: "user", Setuju: true},
		baris:  []peran.Baris{{ID: 1, Peran: peran.Satker, Kode: "119091"}},
	}
}

func TestCacheAuthMenghematQueryDatabase(t *testing.T) {
	st, pr := pasangDenganCache(t, 30, akunBiasa())
	for i := 0; i < 25; i++ {
		if k := panggilCache(t); k != 200 {
			t.Fatalf("permintaan %d: %d", i+1, k)
		}
	}
	if st.Load() != 1 || pr.Load() != 1 {
		t.Fatalf("25 permintaan pengguna yang sama harus membaca database sekali (status=%d peran=%d)", st.Load(), pr.Load())
	}
}

func TestCacheAuthDimatikanBilaTTLNol(t *testing.T) {
	st, _ := pasangDenganCache(t, 0, akunBiasa())
	for i := 0; i < 5; i++ {
		panggilCache(t)
	}
	if st.Load() != 5 {
		t.Fatalf("tanpa cache setiap permintaan membaca database, dibaca %d kali", st.Load())
	}
}

// Penonaktifan akun harus langsung berlaku setelah cache dibuang (perubahan lewat aplikasi).
func TestCacheAuthInvalidasiMembuatPerubahanLangsungBerlaku(t *testing.T) {
	k := akunBiasa()
	pasangDenganCache(t, 60, k)
	if panggilCache(t) != 200 {
		t.Fatal("awal harus lolos")
	}
	k.status.Aktif = false // admin menonaktifkan akun
	if panggilCache(t) != 200 {
		t.Fatal("sebelum dibuang, hasil cache yang masih segar dipakai (batas TTL)")
	}
	InvalidasiAkun() // yang dilakukan middleware setelah PUT /users/:id/deactivate
	if kode := panggilCache(t); kode != 401 {
		t.Fatalf("setelah cache dibuang, akun nonaktif harus ditolak 401, dapat %d", kode)
	}
}

func TestCacheAuthTidakMenyimpanGalatDanAkunNonaktif(t *testing.T) {
	k := &kondisiAkun{err: sql.ErrConnDone}
	st, _ := pasangDenganCache(t, 60, k)
	if panggilCache(t) != 500 || panggilCache(t) != 500 {
		t.Fatal("galat database harus 500")
	}
	if st.Load() != 2 {
		t.Fatalf("galat tidak boleh di-cache (dibaca %d kali)", st.Load())
	}
	// Database pulih: langsung berhasil (tidak tertahan oleh galat yang lalu).
	k.err, k.status, k.baris = nil, StatusAkun{Aktif: true, Role: "user", Setuju: true}, akunBiasa().baris
	if panggilCache(t) != 200 {
		t.Fatal("setelah database pulih harus lolos")
	}
	// Akun nonaktif tidak di-cache.
	k2 := akunBiasa()
	k2.status.Aktif = false
	st2, _ := pasangDenganCache(t, 60, k2)
	panggilCache(t)
	panggilCache(t)
	if st2.Load() != 2 {
		t.Fatalf("akun nonaktif tidak boleh di-cache (dibaca %d kali)", st2.Load())
	}
}

func TestCacheAuthKedaluwarsa(t *testing.T) {
	c := &cacheAkun{m: map[string]akunTersimpan{}}
	t0 := time.Now()
	c.simpan("u1", akunTersimpan{status: StatusAkun{Aktif: true}}, c.generasi(), 10*time.Second, t0)
	if _, ok := c.ambil("u1", t0.Add(9*time.Second)); !ok {
		t.Fatal("harus masih segar")
	}
	if _, ok := c.ambil("u1", t0.Add(10*time.Second)); ok {
		t.Fatal("harus sudah kedaluwarsa")
	}
}

// Hasil yang dibaca sebelum cache dibuang tidak boleh masuk cache sesudahnya (kalau tidak, perubahan yang baru dibuat bisa hilang).
func TestCacheAuthGenerasiMenolakHasilUsang(t *testing.T) {
	c := &cacheAkun{m: map[string]akunTersimpan{}}
	gen := c.generasi()
	c.buang() // perubahan akun terjadi saat pembacaan sedang berjalan
	c.simpan("u1", akunTersimpan{status: StatusAkun{Aktif: true}}, gen, time.Minute, time.Now())
	if _, ok := c.ambil("u1", time.Now()); ok {
		t.Fatal("hasil dari generasi lama tidak boleh disimpan")
	}
}

func TestCacheAuthBerbatas(t *testing.T) {
	c := &cacheAkun{m: map[string]akunTersimpan{}}
	t0 := time.Now()
	for i := 0; i < maksEntriCache+500; i++ {
		c.simpan(time.Duration(i).String(), akunTersimpan{}, c.generasi(), time.Minute, t0)
		if len(c.m) > maksEntriCache {
			t.Fatalf("cache melewati batas: %d", len(c.m))
		}
	}
}

func TestInvalidasiAkunMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(InvalidasiAkunMiddleware())
	ok := func(c *gin.Context) { c.String(200, "ok") }
	r.GET("/api/v1/users", ok)
	r.PUT("/api/v1/users/:id/deactivate", ok)
	r.POST("/api/v1/auth/persetujuan", ok)
	r.POST("/api/v1/sapa/penjualan", ok)
	r.GET("/sso/callback/login", ok)

	// Isi cache lalu lihat apakah permintaan tertentu membuangnya.
	isi := func() {
		cacheAuth.simpan("u1", akunTersimpan{}, cacheAuth.generasi(), time.Minute, time.Now())
	}
	ada := func() bool { _, ok := cacheAuth.ambil("u1", time.Now()); return ok }
	cacheAuth.buang()
	for _, k := range []struct {
		metode, path string
		buang        bool
	}{
		{"GET", "/api/v1/users", false},             // membaca tidak mengubah apa pun
		{"PUT", "/api/v1/users/x/deactivate", true}, // mengubah akun
		{"POST", "/api/v1/auth/persetujuan", true},  // persetujuan
		{"POST", "/api/v1/sapa/penjualan", false},   // tidak menyangkut akun/peran
		{"GET", "/sso/callback/login", true},        // login SSO membuat/menyinkronkan akun
	} {
		isi()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(k.metode, k.path, nil))
		if ada() == k.buang {
			t.Errorf("%s %s: cache seharusnya dibuang=%v", k.metode, k.path, k.buang)
		}
	}
	cacheAuth.buang()
}

// Banyak permintaan serentak dari banyak pengguna: hasilnya konsisten dan tiap pengguna hanya sekali ke database.
func TestCacheAuthBersamaan(t *testing.T) {
	st, _ := pasangDenganCache(t, 30, akunBiasa())
	// Siapkan satu pengguna dulu supaya semua permintaan bersamaan melihat cache terisi.
	if panggilCache(t) != 200 {
		t.Fatal("awal")
	}
	var wg sync.WaitGroup
	var gagal atomic.Int64
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				req := httptest.NewRequest(http.MethodGet, "/x", nil)
				tok, _, _ := utils.GenerateAccessToken("11111111-1111-4111-8111-111111111111", "budi", "user")
				req.Header.Set("Authorization", "Bearer "+tok)
				r := gin.New()
				r.GET("/x", AuthRequired(), func(c *gin.Context) { c.String(200, "ok") })
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				if w.Code != 200 {
					gagal.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	if gagal.Load() != 0 {
		t.Fatalf("%d permintaan gagal", gagal.Load())
	}
	if st.Load() != 1 {
		t.Fatalf("2000 permintaan serentak harus tetap satu kali baca database, dibaca %d kali", st.Load())
	}
}
