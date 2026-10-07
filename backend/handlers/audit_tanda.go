package handlers

import (
	"github.com/gin-gonic/gin"

	"pasti-v3-backend/audit"
	"pasti-v3-backend/database"
)

// Penanda audit untuk kejadian yang hanya diketahui handler (siapa yang login, mengapa gagal). Middleware audit yang menuliskan entrinya; di sini hanya melengkapi
// keterangannya. Jangan mengisi kata sandi, token, atau isi permintaan.

// labelAlasanLogin: uraian alasan login gagal (kode alasan juga disimpan di rincian entri).
var labelAlasanLogin = map[string]string{
	"permintaan_tidak_valid":   "permintaan tidak lengkap",
	"captcha_salah":            "captcha salah atau kedaluwarsa",
	"pengguna_tidak_ditemukan": "pengguna tidak ditemukan",
	"akun_tidak_aktif":         "akun tidak aktif",
	"akun_terkunci":            "akun terkunci sementara",
	"akun_dikunci":             "akun dikunci setelah percobaan gagal berulang",
	"akun_sso":                 "akun SSO tidak punya kata sandi lokal",
	"kata_sandi_salah":         "kata sandi salah",
	"sso_ditolak":              "SSO gagal",
}

// auditLoginGagal menandai permintaan login ini sebagai gagal. userID dan username diisi hanya bila akunnya ada: nama yang diketik untuk akun yang tidak ada TIDAK
// dicatat karena orang kerap salah mengetik kata sandi di kolom nama pengguna.
func auditLoginGagal(c *gin.Context, alasan, userID, username string, detail map[string]interface{}) {
	gagal := false
	d := map[string]interface{}{"alasan": alasan}
	for k, v := range detail {
		d[k] = v
	}
	label := "Login gagal"
	if l, ok := labelAlasanLogin[alasan]; ok {
		label += ": " + l
	}
	audit.Tandai(c, audit.Tanda{Kategori: audit.KatAuth, Aksi: audit.AksiLoginGagal, Label: label, UserID: userID, Username: username, Sukses: &gagal, Detail: d})
}

// auditLoginBerhasil menandai login yang berhasil beserta penggunanya (belum ada di konteks karena belum lewat middleware autentikasi).
func auditLoginBerhasil(c *gin.Context, userID, username, metode string) {
	ok := true
	label := "Login dengan kata sandi"
	if metode == "sso" {
		label = "Login SSO Kemenkeu"
	}
	audit.Tandai(c, audit.Tanda{Kategori: audit.KatAuth, Aksi: audit.AksiLoginBerhasil, Label: label, UserID: userID, Username: username, Sukses: &ok, Detail: map[string]interface{}{"metode": metode}})
}

// usernameDariID membaca username sebuah akun (untuk melengkapi entri audit); kosong bila tidak ketemu atau terjadi galat.
func usernameDariID(userID string) string {
	if userID == "" || database.DB == nil {
		return ""
	}
	var u string
	if err := database.DB.QueryRow(`SELECT username FROM users WHERE id = @p1`, userID).Scan(&u); err != nil {
		return ""
	}
	return u
}
