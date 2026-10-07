package utils

import "github.com/gin-gonic/gin"

func SuccessResponse(c *gin.Context, code int, message string, data interface{}) {
	c.JSON(code, gin.H{
		"success": true,
		"message": message,
		"data":    data,
	})
}

func ErrorResponse(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{
		"success": false,
		"message": message,
	})
}

// Kode galat yang bisa dibaca mesin (field "code" pada respons galat).
const (
	// CodeSSOSessionExpired: 401 yang berasal dari sesi SSO Kemenkeu (token SSO habis atau ditolak),
	// bukan dari sesi PASTI. Frontend memakainya untuk menjelaskan alasan pengguna diarahkan ke login.
	CodeSSOSessionExpired = "sso_session_expired"

	// CodeAccountInactive: akun dinonaktifkan administrator. Berlaku juga untuk sesi yang sedang
	// berjalan (diperiksa di middleware AuthRequired), bukan hanya untuk login baru.
	CodeAccountInactive = "account_inactive"

	// CodePersetujuan: 403 karena pengguna belum menyetujui pernyataan penggunaan aplikasi (versi terbaru).
	CodePersetujuan = "persetujuan_diperlukan"

	// CodeTamu: 403 karena pengguna belum diberi peran (tamu), jadi belum boleh membuka fitur apa pun.
	CodeTamu = "tamu"
)

// ErrorResponseWithCode seperti ErrorResponse, ditambah "code" supaya frontend bisa membedakan
// jenis galat tanpa mencocokkan teks pesan.
func ErrorResponseWithCode(c *gin.Context, status int, message, code string) {
	c.JSON(status, gin.H{
		"success": false,
		"message": message,
		"code":    code,
	})
}
