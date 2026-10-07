package middleware

import (
	"net/http"

	"pasti-v3-backend/peran"
	"pasti-v3-backend/utils"

	"github.com/gin-gonic/gin"
)

// RequireRole membatasi akses endpoint hanya untuk role tertentu. Role yang dibandingkan adalah role untuk hak saat ini: "superadmin" hanya selama pengguna
// bertindak sebagai dirinya (bukan sebagai peran data), selain itu "user".
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")

		allowed := false
		for _, r := range allowedRoles {
			if r == role {
				allowed = true
				break
			}
		}

		if !allowed {
			utils.ErrorResponse(c, http.StatusForbidden, "Anda tidak memiliki akses untuk aksi ini")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireSuperadmin: fitur khusus superadmin (ditetapkan di .env), mis. penarikan data Inaproc, sinkronisasi SLDK, template SAPA, referensi UE1, dan HRIS2.
func RequireSuperadmin() gin.HandlerFunc { return RequireRole(peran.AkunSuperadmin) }

// KelolaPengguna: peran yang boleh mengelola pengguna (membuat, mengubah, menonaktifkan, menghapus, memberi dan mencabut peran): superadmin, dan Pengguna Barang
// untuk semua pengguna kecuali superadmin.
func KelolaPengguna(c *gin.Context) bool {
	return c.GetString("role") == peran.AkunSuperadmin || c.GetString(peran.KunciGinPeran) == peran.PenggunaBarang
}

// LihatPengguna: peran yang boleh melihat daftar pengguna: yang boleh mengelola, ditambah UE1, Kanwil, dan Satker (hanya melihat pengguna dalam cakupan kode satkernya).
func LihatPengguna(c *gin.Context) bool {
	if KelolaPengguna(c) {
		return true
	}
	switch c.GetString(peran.KunciGinPeran) {
	case peran.UE1, peran.Kanwil, peran.Satker:
		return true
	}
	return false
}

// RequireKelolaPengguna menolak (403) peran yang tidak boleh mengelola pengguna.
func RequireKelolaPengguna() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !KelolaPengguna(c) {
			utils.ErrorResponse(c, http.StatusForbidden, "Anda tidak memiliki akses untuk mengelola pengguna")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireLihatPengguna menolak (403) peran yang tidak boleh membuka daftar pengguna.
func RequireLihatPengguna() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !LihatPengguna(c) {
			utils.ErrorResponse(c, http.StatusForbidden, "Anda tidak memiliki akses untuk melihat pengguna")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireCakupanSemua membatasi endpoint untuk peran yang boleh melihat seluruh data (Superadmin dan Pengguna Barang). Dipakai pada keadaan yang memuat seluruh
// data, seperti status dan riwayat penarikan Pengadaan.
func RequireCakupanSemua() gin.HandlerFunc {
	return RequireCakupanSemuaPesan("Keadaan dan pengaturan penarikan data hanya tersedia bagi peran yang melihat seluruh data. Data pengadaan satker Anda ada di menu Pengadaan dan Dashboard.")
}

// RequireCakupanSemuaPesan sama seperti RequireCakupanSemua dengan pesan penolakan sendiri, untuk fitur selain penarikan Pengadaan (mis. status sinkronisasi
// Digitalisasi Aset).
func RequireCakupanSemuaPesan(pesan string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !peran.DariGin(c).SemuaData() {
			utils.ErrorResponse(c, http.StatusForbidden, pesan)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireCakupanAda menolak pengguna yang tidak punya cakupan data (tamu). AuthRequired sudah menolak tamu; ini pengaman kedua bagi rute yang dipasang tanpanya.
// Peran UE1, Kanwil, dan Satker lolos (datanya dibatasi pembaca), peran yang melihat semua data juga lolos.
func RequireCakupanAda() gin.HandlerFunc {
	return func(c *gin.Context) {
		if peran.DariGin(c).Tingkat == peran.Kosong {
			utils.ErrorResponse(c, http.StatusForbidden, "Akun Anda belum diberi peran data, sehingga belum ada data yang dapat ditampilkan. Hubungi administrator.")
			c.Abort()
			return
		}
		c.Next()
	}
}
