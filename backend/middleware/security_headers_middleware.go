package middleware

import "github.com/gin-gonic/gin"

// SecurityHeadersMiddleware menambahkan header keamanan standar ke setiap
// respons, sebagai defense-in-depth terhadap clickjacking, MIME sniffing,
// dan XSS — berlaku terlepas dari apakah request melewati Nginx atau tidak.
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Content-Security-Policy", "frame-ancestors 'self';")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	}
}
