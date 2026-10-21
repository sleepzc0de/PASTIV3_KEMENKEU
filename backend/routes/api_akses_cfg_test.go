package routes

import (
	"testing"

	"pasti-v3-backend/config"
)

// pasangCfgUji memasang konfigurasi minimal (rahasia JWT) untuk tes yang memeriksa token tanpa database; dikembalikan seperti semula setelah tes.
func pasangCfgUji(t *testing.T) {
	t.Helper()
	lama := config.Cfg
	config.Cfg = &config.Config{JWTSecret: "rahasia-uji-api-akses-0123456789abcdef", JWTAccessExpireMin: 60}
	t.Cleanup(func() { config.Cfg = lama })
}
