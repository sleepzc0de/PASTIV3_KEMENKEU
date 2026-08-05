package middleware

import (
	"net/http"

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
