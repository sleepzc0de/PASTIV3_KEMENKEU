package handlers

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/database"
	"pasti-v3-backend/persetujuan"
	"pasti-v3-backend/utils"
)

// Persetujuan pernyataan penggunaan aplikasi: setiap pengguna (kecuali superadmin) membacanya dan menyetujuinya sekali (mengetik "SAYA SETUJU" dan mencentang).
// Akun non-SSO sekaligus mengisi nama lengkap, NIP, dan email; akun SSO identitasnya sudah dari SSO Kemenkeu. Endpoint ini boleh dibuka tamu (AuthTamuBoleh).

const persetujuanTimeout = 10 * time.Second

// ringkasanPersetujuan: keadaan persetujuan dan profil satu pengguna.
type ringkasanPersetujuan struct {
	Sudah            bool
	LokalPerluDiisi  bool // akun non-SSO: nama, NIP, dan email diisi bersamaan dengan persetujuan
	AuthProvider     string
	Nama, NIP, Email string
}

func bacaPersetujuan(ctx context.Context, userID string) (ringkasanPersetujuan, error) {
	var r ringkasanPersetujuan
	var nip, nipPegawai, versi sql.NullString
	var at sql.NullTime
	err := database.DB.QueryRowContext(ctx, `
		SELECT u.auth_provider, u.full_name, u.email, u.nip, e.nip, u.persetujuan_at, u.persetujuan_versi
		FROM users u LEFT JOIN employees e ON e.id = u.employee_id
		WHERE u.id = @p1`, userID).Scan(&r.AuthProvider, &r.Nama, &r.Email, &nip, &nipPegawai, &at, &versi)
	if err != nil {
		return r, err
	}
	r.NIP = nip.String
	if r.NIP == "" {
		r.NIP = nipPegawai.String
	}
	r.Sudah = at.Valid && versi.String == persetujuan.Versi
	// Akun non-SSO yang belum punya NIP (mis. dibuat admin sebelum aturan ini) wajib melengkapi profil saat menyetujui.
	r.LokalPerluDiisi = r.AuthProvider != "sso" && !r.Sudah
	return r, nil
}

// GetPersetujuan: GET /auth/persetujuan. Teks pernyataan, frasa yang harus diketik, dan keadaan persetujuan pengguna (beserta profil awal untuk akun non-SSO).
func GetPersetujuan(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), persetujuanTimeout)
	defer cancel()
	r, err := bacaPersetujuan(ctx, c.GetString("user_id"))
	if err != nil {
		log.Println("[PERSETUJUAN ERROR] baca:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca pernyataan")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil pernyataan", gin.H{
		"versi": persetujuan.Versi, "judul": persetujuan.Judul, "paragraf": persetujuan.Paragraf, "frasa": persetujuan.Frasa,
		"sudah": r.Sudah, "auth_provider": r.AuthProvider, "isi_profil": r.LokalPerluDiisi,
		"profil": gin.H{"nama": r.Nama, "nip": r.NIP, "email": r.Email},
	})
}

type persetujuanMasukan struct {
	Frasa  string `json:"frasa"`
	Setuju bool   `json:"setuju"`
	Nama   string `json:"nama"`
	NIP    string `json:"nip"`
	Email  string `json:"email"`
}

// PostPersetujuan: POST /auth/persetujuan. Menyetujui pernyataan: frasa harus diketik dan kotak persetujuan dicentang. Akun non-SSO juga wajib mengisi nama lengkap,
// NIP, dan email yang sah; email tidak boleh dipakai akun lain (email maupun username) dan tidak boleh identitas superadmin yang dicadangkan di .env. Menyetujui
// lagi versi yang sama tidak mengubah apa pun.
func PostPersetujuan(c *gin.Context) {
	var m persetujuanMasukan
	if err := c.ShouldBindJSON(&m); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid")
		return
	}
	if !m.Setuju || !persetujuan.FrasaSah(m.Frasa) {
		utils.ErrorResponse(c, http.StatusBadRequest, "Ketik \""+persetujuan.Frasa+"\" dan centang kotak persetujuan untuk melanjutkan")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), persetujuanTimeout)
	defer cancel()
	userID := c.GetString("user_id")
	r, err := bacaPersetujuan(ctx, userID)
	if err != nil {
		log.Println("[PERSETUJUAN ERROR] baca:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca data pengguna")
		return
	}
	if r.Sudah {
		utils.SuccessResponse(c, http.StatusOK, "Pernyataan sudah disetujui", gin.H{"sudah": true})
		return
	}

	if r.AuthProvider == "sso" {
		if _, err := database.DB.ExecContext(ctx, `UPDATE users SET persetujuan_at = SYSUTCDATETIME(), persetujuan_versi = @p1, updated_at = SYSUTCDATETIME() WHERE id = @p2`, persetujuan.Versi, userID); err != nil {
			log.Println("[PERSETUJUAN ERROR] simpan:", err)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menyimpan persetujuan")
			return
		}
		utils.SuccessResponse(c, http.StatusOK, "Pernyataan disetujui", gin.H{"sudah": true})
		return
	}

	p := persetujuan.Profil{Nama: m.Nama, NIP: m.NIP, Email: m.Email}.Rapikan()
	if g := p.Validasi(); len(g) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": pesanProfil(g), "galat": g})
		return
	}
	// Identitas superadmin hanya berlaku untuk akun SSO: akun non-SSO tidak boleh memakainya (tidak untuk menyamar, tidak untuk memicu penaikan).
	if utils.IsProtectedIdentity(p.Email, p.NIP) {
		utils.ErrorResponse(c, http.StatusBadRequest, "Email atau NIP ini dicadangkan untuk akun lain. Gunakan email yang berbeda atau login dengan SSO Kemenkeu")
		return
	}
	// Email harus unik di antara email dan username akun lain (login menerima keduanya).
	var bentrok int
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id <> @p1 AND (LOWER(email) = @p2 OR LOWER(username) = @p2)`, userID, p.Email).Scan(&bentrok); err != nil {
		log.Println("[PERSETUJUAN ERROR] cek email:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal memeriksa email")
		return
	}
	if bentrok > 0 {
		utils.ErrorResponse(c, http.StatusConflict, "Email sudah digunakan oleh akun lain")
		return
	}
	if _, err := database.DB.ExecContext(ctx, `
		UPDATE users SET full_name = @p1, nip = @p2, email = @p3, persetujuan_at = SYSUTCDATETIME(), persetujuan_versi = @p4, updated_at = SYSUTCDATETIME()
		WHERE id = @p5`, p.Nama, p.NIP, p.Email, persetujuan.Versi, userID); err != nil {
		log.Println("[PERSETUJUAN ERROR] simpan:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menyimpan persetujuan")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Pernyataan disetujui", gin.H{"sudah": true})
}

// pesanProfil menggabungkan pesan galat isian menurut urutan tetap (nama, NIP, email).
func pesanProfil(g map[string]string) string {
	var out []string
	for _, k := range []string{"nama", "nip", "email"} {
		if m, ada := g[k]; ada {
			out = append(out, m)
		}
	}
	return strings.Join(out, ". ")
}
