package peran

import (
	"context"

	"github.com/gin-gonic/gin"
)

// Kunci konteks Gin yang diisi middleware autentikasi.
const (
	KunciGinCakupan = "cakupan"   // Cakupan peran aktif
	KunciGinPeran   = "peran"     // nama peran aktif (lihat Efektif.Peran)
	KunciGinAkun    = "akun_role" // users.role akun, tidak terpengaruh peran aktif
)

type kunciKonteks struct{}

// DenganCakupan menempelkan cakupan pada context.Context supaya fungsi pembaca data yang hanya menerima ctx ikut membatasi hasilnya.
func DenganCakupan(ctx context.Context, c Cakupan) context.Context {
	return context.WithValue(ctx, kunciKonteks{}, c)
}

// CakupanDari membaca cakupan dari context.Context. Tanpa cakupan terpasang (mis. tugas latar belakang atau tes) hasilnya semua data.
func CakupanDari(ctx context.Context) Cakupan {
	if c, ok := ctx.Value(kunciKonteks{}).(Cakupan); ok {
		return c
	}
	return CakupanSemua
}

// DariGin membaca cakupan yang dipasang middleware. Tanpa middleware (mis. tes yang memasang rute langsung) hasilnya semua data.
func DariGin(c *gin.Context) Cakupan {
	if v, ok := c.Get(KunciGinCakupan); ok {
		if cak, ok := v.(Cakupan); ok {
			return cak
		}
	}
	return CakupanSemua
}

// Pasang menyimpan peran yang berlaku pada konteks Gin; "role" diganti dengan peran efektif untuk pemeriksaan hak (RequireRole dan sejenisnya).
func Pasang(c *gin.Context, e Efektif) {
	c.Set("role", e.Role)
	c.Set(KunciGinAkun, e.AkunRole)
	c.Set(KunciGinPeran, e.Peran)
	c.Set(KunciGinCakupan, e.Cakupan)
}
