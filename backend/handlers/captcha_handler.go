package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"

	"pasti-v3-backend/utils"
)

// captchaStore menyimpan jawaban captcha sementara di memori (in-process).
// Catatan: kalau nanti backend di-scale jadi multi-instance/replica,
// store ini perlu diganti ke Redis atau storage terpusat lainnya,
// karena setiap instance punya memori terpisah.
var captchaStore = base64Captcha.DefaultMemStore

// GenerateCaptcha membuat gambar captcha digit baru, dikembalikan sebagai base64 PNG
func GenerateCaptcha(c *gin.Context) {
	driver := base64Captcha.NewDriverDigit(80, 240, 5, 0.7, 80)
	captcha := base64Captcha.NewCaptcha(driver, captchaStore)

	id, b64s, _, err := captcha.Generate()
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membuat captcha")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Captcha berhasil dibuat", gin.H{
		"captcha_id":    id,
		"captcha_image": b64s,
	})
}

// VerifyCaptcha memvalidasi jawaban captcha. clear=true supaya captcha
// hanya bisa dipakai sekali (mencegah replay).
func VerifyCaptcha(captchaID, answer string) bool {
	if captchaID == "" || answer == "" {
		return false
	}
	return captchaStore.Verify(captchaID, answer, true)
}
