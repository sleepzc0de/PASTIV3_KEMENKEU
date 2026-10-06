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
			utils.ErrorResponse(c, http.StatusForbidden, "Data ini hanya tersedia bagi peran yang melihat seluruh data. Untuk peran Anda, gunakan tab Satker di Dashboard.")
			c.Abort()
			return
		}
		c.Next()
	}
}
