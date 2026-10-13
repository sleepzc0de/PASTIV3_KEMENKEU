package middleware

import (
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/peran"
)

// Setiap permintaan terautentikasi membaca status akun dan peran pengguna dari database (dua query) supaya penonaktifan dan perubahan peran langsung berlaku. Saat banyak
// pengguna membuka halaman yang memanggil belasan endpoint sekaligus, itulah yang paling cepat menghabiskan kolam koneksi. Hasilnya disimpan sebentar di memori:
//
//   - perubahan yang dibuat lewat aplikasi (kelola pengguna/peran, persetujuan, login SSO) langsung membuang cache (InvalidasiAkun), jadi tetap langsung berlaku;
//   - perubahan yang dibuat langsung di database berlaku paling lambat setelah AUTH_CACHE_DETIK (bawaan 10 detik);
//   - AUTH_CACHE_DETIK=0 mematikan cache.
//
// Tidak ada hasil galat yang disimpan, dan jumlah entri dibatasi.

const maksEntriCache = 20000

type akunTersimpan struct {
	status      StatusAkun
	efektif     peran.Efektif
	kedaluwarsa time.Time
}

type cacheAkun struct {
	mu  sync.RWMutex
	m   map[string]akunTersimpan
	gen uint64 // bertambah setiap kali cache dibuang; hasil yang dibaca sebelum pembuangan tidak boleh disimpan sesudahnya
}

var cacheAuth = &cacheAkun{m: make(map[string]akunTersimpan)}

func ttlCacheAuth() time.Duration {
	if config.Cfg == nil || config.Cfg.AuthCacheDetik <= 0 {
		return 0
	}
	return time.Duration(config.Cfg.AuthCacheDetik) * time.Second
}

func (c *cacheAkun) ambil(id string, sekarang time.Time) (akunTersimpan, bool) {
	c.mu.RLock()
	a, ok := c.m[id]
	c.mu.RUnlock()
	if !ok || !sekarang.Before(a.kedaluwarsa) {
		return akunTersimpan{}, false
	}
	return a, true
}

func (c *cacheAkun) generasi() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.gen
}

// simpan menyimpan hasil yang dibaca pada generasi gen; dilewati bila cache sudah dibuang sejak itu (hasilnya mungkin sudah usang).
func (c *cacheAkun) simpan(id string, a akunTersimpan, gen uint64, ttl time.Duration, sekarang time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if gen != c.gen {
		return
	}
	if len(c.m) >= maksEntriCache {
		for k, v := range c.m {
			if !sekarang.Before(v.kedaluwarsa) {
				delete(c.m, k)
			}
		}
		if len(c.m) >= maksEntriCache {
			c.m = make(map[string]akunTersimpan)
		}
	}
	a.kedaluwarsa = sekarang.Add(ttl)
	c.m[id] = a
}

func (c *cacheAkun) buang() {
	c.mu.Lock()
	c.gen++
	if len(c.m) > 0 {
		c.m = make(map[string]akunTersimpan)
	}
	c.mu.Unlock()
}

// InvalidasiAkun membuang seluruh cache status akun dan peran. Dipanggil setelah akun atau peran diubah.
func InvalidasiAkun() { cacheAuth.buang() }

// InvalidasiAkunMiddleware membuang cache setelah permintaan yang bisa mengubah akun atau peran: perubahan (POST/PUT/PATCH/DELETE) di bawah /api/v1/users dan /api/v1/auth/
// (peran aktif, persetujuan), dan callback login SSO (yang membuat/menyinkronkan akun). Dibuang juga bila permintaannya gagal, karena perubahan sebagian mungkin sudah terjadi.
func InvalidasiAkunMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		p := c.Request.URL.Path
		switch c.Request.Method {
		case "POST", "PUT", "PATCH", "DELETE":
			if strings.HasPrefix(p, "/api/v1/users") || strings.HasPrefix(p, "/api/v1/auth/") {
				InvalidasiAkun()
			}
		default:
			if p == "/sso/callback/login" {
				InvalidasiAkun()
			}
		}
	}
}
