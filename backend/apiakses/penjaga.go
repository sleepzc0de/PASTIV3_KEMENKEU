package apiakses

import (
	"sync"
	"time"
)

// PenjagaGagal membatasi percobaan kunci yang salah per alamat IP: setelah Ambang kegagalan dalam satu Jendela, alamat itu ditolak (429) sampai jendela berakhir, tanpa
// memeriksa kunci lagi. Yang dihitung hanya kegagalan; permintaan yang sah tidak pernah mengurangi hitungan (penyerang tidak bisa "mencuci" hitungan dengan kunci
// miliknya sendiri). Rahasia kunci 256 bit tidak dapat ditebak; penjaga ini melindungi dari beban dan dari pemindaian prefix.
type PenjagaGagal struct {
	mu       sync.Mutex
	m        map[string]*catatanGagal
	Ambang   int
	Jendela  time.Duration
	Maks     int
	sekarang func() time.Time
}

type catatanGagal struct {
	mulai  time.Time
	jumlah int
}

// NewPenjagaGagal membuat penjaga: ambang kegagalan per jendela dan jumlah alamat maksimal yang dilacak.
func NewPenjagaGagal(ambang int, jendela time.Duration, maks int) *PenjagaGagal {
	if ambang < 1 {
		ambang = 20
	}
	if jendela <= 0 {
		jendela = 10 * time.Minute
	}
	if maks < 1 {
		maks = 50000
	}
	return &PenjagaGagal{m: make(map[string]*catatanGagal), Ambang: ambang, Jendela: jendela, Maks: maks, sekarang: time.Now}
}

// Terkunci: apakah alamat sedang ditolak, dan sampai kapan (lama tunggu).
func (g *PenjagaGagal) Terkunci(ip string) (bool, time.Duration) {
	now := g.sekarang()
	g.mu.Lock()
	defer g.mu.Unlock()
	c := g.m[ip]
	if c == nil {
		return false, 0
	}
	if now.Sub(c.mulai) >= g.Jendela {
		delete(g.m, ip)
		return false, 0
	}
	if c.jumlah >= g.Ambang {
		return true, c.mulai.Add(g.Jendela).Sub(now)
	}
	return false, 0
}

// Gagal mencatat satu kegagalan dari alamat ini.
func (g *PenjagaGagal) Gagal(ip string) {
	now := g.sekarang()
	g.mu.Lock()
	defer g.mu.Unlock()
	c := g.m[ip]
	if c == nil || now.Sub(c.mulai) >= g.Jendela {
		if c == nil && len(g.m) >= g.Maks {
			for k, v := range g.m { // tabel penuh: buang yang sudah lewat jendela
				if now.Sub(v.mulai) >= g.Jendela {
					delete(g.m, k)
				}
			}
			if len(g.m) >= g.Maks {
				return // penuh oleh alamat yang masih aktif: tidak dilacak (lebih baik tidak menolak orang sah daripada kehabisan memori)
			}
		}
		g.m[ip] = &catatanGagal{mulai: now, jumlah: 1}
		return
	}
	c.jumlah++
}

// Ukuran: jumlah alamat yang dilacak (untuk tes).
func (g *PenjagaGagal) Ukuran() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.m)
}
