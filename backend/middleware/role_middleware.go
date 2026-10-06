package middleware

import (
	"net/http"

	"pasti-v3-backend/peran"
	"pasti-v3-backend/utils"

	"github.com/gin-gonic/gin"
)

// RequireRole membatasi akses endpoint hanya untuk role tertentu (mis. "admin", "superadmin")
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

// RequireCakupanSemua membatasi endpoint untuk peran yang boleh melihat seluruh data (Super Admin, Admin, Pengguna Barang, dan pengguna tanpa peran
// selama pembatasan belum diwajibkan). Dipakai pada data yang belum bisa dibatasi per satker, seperti data Pengadaan lengkap.
func RequireCakupanSemua() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !peran.DariGin(c).SemuaData() {
			utils.ErrorResponse(c, http.StatusForbidden, "Keadaan dan pengaturan penarikan data hanya tersedia bagi peran yang melihat seluruh data. Data pengadaan satker Anda ada di menu Pengadaan dan Dashboard.")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireCakupanAda menolak pengguna yang belum diberi peran data sementara pembatasan diwajibkan (cakupan kosong). Dipakai pada data yang dibatasi per
// satker: peran UE1, Kanwil, dan Satker lolos (datanya dibatasi pembaca), peran yang melihat semua data juga lolos.
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
