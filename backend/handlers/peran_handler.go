package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"pasti-v3-backend/audit"
	"pasti-v3-backend/database"
	"pasti-v3-backend/peran"
	"pasti-v3-backend/utils"
)

// Peran data pengguna (migrasi 053): daftar peran milik sendiri, berpindah peran aktif, dan pengelolaan peran oleh admin. Penjelasan peran, kode, dan cakupan
// ada di paket peran.

const peranTimeout = 10 * time.Second

// ringkasPeran: peran yang berlaku dan yang tersedia bagi satu pengguna, dipakai /auth/me dan /auth/peran.
//
//	akun_role : users.role (hak administrasi akun)
//	role      : peran untuk hak administrasi saat ini (turun menjadi "user" selama bertindak sebagai peran data)
//	peran     : peran yang berlaku (superadmin, pengguna_barang, ue1, kanwil, satker, atau "" bila belum punya peran = tamu)
//	cakupan   : data yang boleh dilihat peran itu
//	tersedia  : peran data yang dipegang; peran_id = yang berlaku (0 = peran bawaan akun)
//	bawaan    : superadmin bisa kembali ke peran akunnya lewat pemilih peran
//	tamu      : belum punya peran apa pun, jadi belum boleh membuka fitur apa pun
func ringkasPeran(ctx context.Context, userID, akunRole string) (gin.H, error) {
	daftar, err := peran.Daftar(ctx, userID)
	if err != nil {
		if !peran.TabelBelumAda(err) {
			return nil, err
		}
		daftar = []peran.Baris{} // migrasi 053 belum dijalankan: belum ada peran
	}
	ef := peran.Selesaikan(akunRole, daftar)
	return gin.H{
		"akun_role":   akunRole,
		"role":        ef.Role,
		"peran":       ef.Peran,
		"peran_label": peran.Label(ef.Peran),
		"peran_id":    ef.PeranID,
		"kode":        ef.Kode,
		"cakupan":     ef.Cakupan,
		"tersedia":    daftar,
		"bawaan":      akunRole == peran.AkunSuperadmin,
		"tamu":        ef.Tamu(),
	}, nil
}

// GetPeranSaya: GET /auth/peran.
func GetPeranSaya(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), peranTimeout)
	defer cancel()
	h, err := ringkasPeran(ctx, c.GetString("user_id"), c.GetString(peran.KunciGinAkun))
	if err != nil {
		log.Println("[PERAN ERROR] baca peran sendiri:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca peran")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil peran", h)
}

type peranAktifMasukan struct {
	ID *int64 `json:"id"` // null = kembali ke peran bawaan
}

// PostPeranAktif: POST /auth/peran/aktif. Berpindah peran aktif ke salah satu peran milik sendiri (atau kembali ke peran bawaan). Berlaku di
// permintaan berikutnya, tanpa token baru: peran dibaca dari database pada setiap permintaan.
func PostPeranAktif(c *gin.Context) {
	var m peranAktifMasukan
	if err := c.ShouldBindJSON(&m); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid")
		return
	}
	userID, akun := c.GetString("user_id"), c.GetString(peran.KunciGinAkun)
	ctx, cancel := context.WithTimeout(c.Request.Context(), peranTimeout)
	defer cancel()
	if err := peran.Aktifkan(ctx, userID, m.ID); err != nil {
		if errors.Is(err, peran.ErrTidakDitemukan) {
			utils.ErrorResponse(c, http.StatusNotFound, "Peran tidak ditemukan")
			return
		}
		log.Println("[PERAN ERROR] pindah peran:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal berpindah peran")
		return
	}
	h, err := ringkasPeran(ctx, userID, akun)
	if err != nil {
		log.Println("[PERAN ERROR] baca peran setelah pindah:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Peran berpindah, tetapi gagal dibaca ulang")
		return
	}
	audit.Tandai(c, audit.Tanda{Detail: map[string]interface{}{"peran_id": m.ID}})
	utils.SuccessResponse(c, http.StatusOK, "Peran aktif diubah", h)
}

// penggunaDariParam memeriksa :id (UUID) dan keberadaan penggunanya; menjawab sendiri bila tidak sah.
func penggunaDariParam(c *gin.Context, ctx context.Context) (string, bool) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "ID pengguna tidak valid")
		return "", false
	}
	var ada int
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id = @p1`, id).Scan(&ada); err != nil {
		log.Println("[PERAN ERROR] cek pengguna:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca pengguna")
		return "", false
	}
	if ada == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "Pengguna tidak ditemukan")
		return "", false
	}
	return id, true
}

// GetPeranPengguna: GET /users/:id/peran. Peran yang dipegang pengguna, beserta saran yang diturunkan dari kode satker di data SSO-nya. Hanya pengguna yang
// boleh dilihat pemanggil (lihat kondisiPengguna): UE1/Kanwil/Satker hanya dalam cakupan kode satkernya, selain itu 404.
func GetPeranPengguna(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), peranTimeout)
	defer cancel()
	id, ok := penggunaDariParam(c, ctx)
	if !ok {
		return
	}
	if !penggunaTerlihat(c, ctx, id) {
		return
	}
	daftar, err := peran.Daftar(ctx, id)
	if err != nil {
		log.Println("[PERAN ERROR] baca peran pengguna:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca peran")
		return
	}
	kode, err := peran.KodeSatkerSSO(ctx, id)
	if err != nil {
		log.Println("[PERAN WARN]", err)
	}
	saran := peran.SaranDariKodeSatker(kode)
	if saran == nil {
		saran = []peran.Saran{}
	}
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil peran pengguna", gin.H{"peran": daftar, "saran": saran, "kode_satker_sso": kode})
}

type peranMasukan struct {
	Peran string `json:"role"`
	Kode  string `json:"kode"`
}

// PostPeranPengguna: POST /users/:id/peran (superadmin, Pengguna Barang). Memberikan satu peran; peran yang sama persis tidak digandakan. Pengguna Barang tidak dapat
// menyentuh superadmin (dijawab 404).
func PostPeranPengguna(c *gin.Context) {
	var m peranMasukan
	if err := c.ShouldBindJSON(&m); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid")
		return
	}
	if _, err := peran.ValidasiPeran(m.Peran, m.Kode); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), peranTimeout)
	defer cancel()
	id, ok := penggunaDariParam(c, ctx)
	if !ok {
		return
	}
	if !penggunaTerlihat(c, ctx, id) {
		return
	}
	b, dibuat, err := peran.Tambah(ctx, id, m.Peran, m.Kode, c.GetString("username"))
	if err != nil {
		log.Println("[PERAN ERROR] beri peran:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal memberi peran")
		return
	}
	audit.Tandai(c, audit.Tanda{Detail: map[string]interface{}{"peran": m.Peran, "kode": m.Kode, "baru": dibuat}})
	status, pesan := http.StatusOK, "Peran sudah dimiliki pengguna ini"
	if dibuat {
		status, pesan = http.StatusCreated, "Peran diberikan"
	}
	utils.SuccessResponse(c, status, pesan, b)
}

// DeletePeranPengguna: DELETE /users/:id/peran/:peranId (superadmin, Pengguna Barang). Mencabut satu peran; bila itu peran aktifnya, pengguna kembali ke peran bawaan.
func DeletePeranPengguna(c *gin.Context) {
	peranID, err := strconv.ParseInt(c.Param("peranId"), 10, 64)
	if err != nil || peranID <= 0 {
		utils.ErrorResponse(c, http.StatusBadRequest, "ID peran tidak valid")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), peranTimeout)
	defer cancel()
	id, ok := penggunaDariParam(c, ctx)
	if !ok {
		return
	}
	if !penggunaTerlihat(c, ctx, id) {
		return
	}
	if err := peran.Hapus(ctx, id, peranID); err != nil {
		if errors.Is(err, peran.ErrTidakDitemukan) {
			utils.ErrorResponse(c, http.StatusNotFound, "Peran tidak ditemukan")
			return
		}
		log.Println("[PERAN ERROR] cabut peran:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal mencabut peran")
		return
	}
	audit.Tandai(c, audit.Tanda{Detail: map[string]interface{}{"peran_id": peranID}})
	utils.SuccessResponse(c, http.StatusOK, "Peran dicabut", gin.H{"id": peranID})
}
