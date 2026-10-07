package middleware

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"

	"pasti-v3-backend/database"
	"pasti-v3-backend/peran"
	"pasti-v3-backend/persetujuan"
	"pasti-v3-backend/utils"

	"github.com/gin-gonic/gin"
)

// StatusAkun: keadaan akun yang dibaca dari database pada setiap permintaan (bukan dari token), supaya penonaktifan, perubahan role, dan persetujuan langsung berlaku.
type StatusAkun struct {
	Aktif  bool
	Role   string // users.role: superadmin atau user
	Setuju bool   // sudah menyetujui pernyataan penggunaan aplikasi versi terbaru
}

// muatStatusAkun membaca keadaan akun. Berupa variabel agar bisa diganti di tes tanpa database.
var muatStatusAkun = func(userID string) (StatusAkun, error) {
	var s StatusAkun
	err := database.DB.QueryRow(
		`SELECT is_active, role, CASE WHEN persetujuan_at IS NOT NULL AND persetujuan_versi = @p2 THEN 1 ELSE 0 END FROM users WHERE id = @p1`,
		userID, persetujuan.Versi,
	).Scan(&s.Aktif, &s.Role, &s.Setuju)
	return s, err
}

// muatPeran menentukan peran data yang berlaku (peran aktif dan cakupan datanya). Berupa variabel agar bisa diganti di tes tanpa database.
var muatPeran = func(ctx context.Context, userID, akunRole string) (peran.Efektif, error) {
	return peran.Muat(ctx, userID, akunRole)
}

// AuthRequired: pengguna login yang boleh memakai aplikasi. Selain token yang sah dan akun aktif, pengguna harus sudah menyetujui pernyataan penggunaan
// aplikasi dan sudah punya peran (superadmin atau peran data); tamu dan yang belum menyetujui ditolak (403) di semua endpoint yang memakai ini.
func AuthRequired() gin.HandlerFunc { return autentikasi(false) }

// AuthTamuBoleh: seperti AuthRequired tetapi tidak menolak tamu atau yang belum menyetujui pernyataan. Hanya untuk endpoint yang dibutuhkan tamu untuk
// membaca profilnya dan menyetujui pernyataan (/auth/me, /auth/peran, /auth/persetujuan).
func AuthTamuBoleh() gin.HandlerFunc { return autentikasi(true) }

func autentikasi(bolehTamu bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.ErrorResponse(c, http.StatusUnauthorized, "Token otorisasi tidak ditemukan")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			utils.ErrorResponse(c, http.StatusUnauthorized, "Format token tidak valid")
			c.Abort()
			return
		}

		claims, err := utils.ValidateToken(parts[1])
		if err != nil {
			utils.ErrorResponse(c, http.StatusUnauthorized, "Token tidak valid atau kedaluwarsa")
			c.Abort()
			return
		}

		// Token yang masih berlaku belum cukup: akun bisa dinonaktifkan atau dihapus setelah token terbit, dan itu harus langsung berlaku, bukan menunggu token
		// kedaluwarsa. Role juga dibaca dari database (bukan dari token), jadi hak superadmin yang dicabut langsung hilang.
		st, err := muatStatusAkun(claims.UserID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			utils.ErrorResponse(c, http.StatusUnauthorized, "Akun tidak ditemukan")
			c.Abort()
			return
		case err != nil:
			log.Println("[AUTH ERROR] gagal memeriksa status akun:", err)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Terjadi kesalahan pada server")
			c.Abort()
			return
		case !st.Aktif:
			utils.ErrorResponseWithCode(c, http.StatusUnauthorized, "Akun Anda telah dinonaktifkan, hubungi administrator", utils.CodeAccountInactive)
			c.Abort()
			return
		}

		// Peran data yang berlaku: superadmin memakai peran akunnya kecuali sedang bertindak sebagai peran data; peran data menentukan cakupan data
		// (peran.DariGin) dan, selama aktif, menurunkan hak superadmin menjadi pengguna biasa. Pengguna tanpa peran adalah tamu (cakupan kosong). Tabel peran
		// yang belum ada (migrasi 053 belum dijalankan) diperlakukan seperti belum ada peran, bukan galat.
		ef, err := muatPeran(c.Request.Context(), claims.UserID, st.Role)
		if peran.TabelBelumAda(err) {
			ef, err = peran.Selesaikan(st.Role, nil), nil
		}
		if err != nil {
			log.Println("[AUTH ERROR] gagal menentukan peran:", err)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Terjadi kesalahan pada server")
			c.Abort()
			return
		}

		if !bolehTamu {
			// Superadmin dikecualikan dari pernyataan supaya tidak pernah terkunci di luar aplikasi.
			if !st.Setuju && st.Role != peran.AkunSuperadmin {
				utils.ErrorResponseWithCode(c, http.StatusForbidden, "Anda perlu membaca dan menyetujui pernyataan penggunaan aplikasi terlebih dahulu", utils.CodePersetujuan)
				c.Abort()
				return
			}
			if ef.Tamu() {
				utils.ErrorResponseWithCode(c, http.StatusForbidden, "Akun Anda belum diberi peran, sehingga belum dapat membuka fitur apa pun. Hubungi administrator", utils.CodeTamu)
				c.Abort()
				return
			}
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set(KunciGinSetuju, st.Setuju)
		peran.Pasang(c, ef)
		c.Next()
	}
}

// KunciGinSetuju: kunci konteks Gin berisi apakah pengguna sudah menyetujui pernyataan (bool).
const KunciGinSetuju = "setuju"
