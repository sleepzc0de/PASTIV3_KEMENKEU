package middleware

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/peran"
	"pasti-v3-backend/utils"

	"github.com/gin-gonic/gin"
)

// userIsActive membaca status aktif akun dari database. Berupa variabel agar bisa diganti di tes
// tanpa database.
var userIsActive = func(userID string) (bool, error) {
	var isActive bool
	err := database.DB.QueryRow(`SELECT is_active FROM users WHERE id = @p1`, userID).Scan(&isActive)
	return isActive, err
}

// muatPeran menentukan peran data yang berlaku (peran aktif dan cakupan datanya). Berupa variabel agar bisa diganti di tes tanpa database.
var muatPeran = func(ctx context.Context, userID, akunRole string) (peran.Efektif, error) {
	return peran.Muat(ctx, userID, akunRole, config.Cfg != nil && config.Cfg.PeranDataWajib)
}

func AuthRequired() gin.HandlerFunc {
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

		// Token yang masih berlaku belum cukup: akun bisa dinonaktifkan atau dihapus setelah token
		// terbit, dan itu harus langsung berlaku, bukan menunggu token kedaluwarsa.
		active, err := userIsActive(claims.UserID)
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
		case !active:
			utils.ErrorResponseWithCode(c, http.StatusUnauthorized, "Akun Anda telah dinonaktifkan, hubungi administrator", utils.CodeAccountInactive)
			c.Abort()
			return
		}

		// Peran data yang berlaku: akun admin/superadmin memakai peran akunnya kecuali sedang bertindak sebagai peran data; peran data menentukan
		// cakupan data (peran.DariGin) dan, selama aktif, menurunkan hak administrasi menjadi pengguna biasa. Tabel peran yang belum ada (migrasi 053
		// belum dijalankan) diperlakukan seperti belum ada peran, bukan galat, supaya penerapan kode sebelum migrasi tidak mengunci semua pengguna.
		ef, err := muatPeran(c.Request.Context(), claims.UserID, claims.Role)
		if peran.TabelBelumAda(err) {
			ef, err = peran.Selesaikan(claims.Role, nil, false), nil
		}
		if err != nil {
			log.Println("[AUTH ERROR] gagal menentukan peran:", err)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Terjadi kesalahan pada server")
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		peran.Pasang(c, ef)
		c.Next()
	}
}
