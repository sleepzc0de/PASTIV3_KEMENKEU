package middleware

import (
	"slices"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
)

// allowedOrigins: FRONTEND_URL dari .env, supaya frontend di server (IP atau domain berapa pun)
// bisa memanggil API tanpa mengubah kode. Origin pengembangan (localhost dan server dev) hanya
// diizinkan di luar production, karena CORS di sini memakai AllowCredentials.
func allowedOrigins() []string {
	var origins []string
	if config.Cfg.AppEnv != "production" {
		origins = append(origins, "http://localhost:3000", "http://10.216.78.48:3001")
	}
	if u := strings.TrimRight(strings.TrimSpace(config.Cfg.FrontendURL), "/"); u != "" && !slices.Contains(origins, u) {
		origins = append(origins, u)
	}
	return origins
}

func CORSMiddleware() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:     allowedOrigins(),
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}
