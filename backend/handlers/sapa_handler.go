package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/database"
	"pasti-v3-backend/sapa"
	"pasti-v3-backend/utils"
)

// SAPA (Sistem Administrasi Pengelolaan Aset): alur Penjualan. Aturan bisnis ada di paket sapa (Layanan); file ini hanya
// menerjemahkan HTTP <-> Layanan: membaca identitas dari token, membatasi ukuran masukan, dan memetakan galat ke kode status.

const (
	sapaTimeout    = 30 * time.Second
	sapaDocxType   = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	sapaMaksUnggah = sapa.MaksTemplate + (1 << 20) // berkas template + sisa bidang formulir multipart
)

// sapaLayanan membuat Layanan di atas database aplikasi. Tes menggantinya dengan Layanan berpenyimpanan memori.
var sapaLayanan = func() *sapa.Layanan { return &sapa.Layanan{Repo: sapa.NewStore(database.DB)} }

// GunakanSapaLayanan mengganti Layanan yang dipakai handler (untuk tes) dan mengembalikan fungsi pemulihnya.
func GunakanSapaLayanan(l *sapa.Layanan) (pulihkan func()) {
	lama := sapaLayanan
	sapaLayanan = func() *sapa.Layanan { return l }
	return func() { sapaLayanan = lama }
}

func sapaCtx(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), sapaTimeout)
}

func sapaAdmin(c *gin.Context) bool {
	r := c.GetString("role")
	return r == "admin" || r == "superadmin"
}

// sapaIdentitas membaca identitas pengguna beserta peran SAPA-nya. Nama pengguna dipakai bila nama lengkap tidak ada.
func sapaIdentitas(c *gin.Context, ctx context.Context) (sapa.Identitas, bool) {
	id, err := sapaLayanan().IdentitasDari(ctx, c.GetString("user_id"), c.GetString("username"), sapaAdmin(c))
	if err != nil {
		sapaGagal(c, "identitas", err)
		return id, false
	}
	return id, true
}

// sapaMasuk memastikan pengguna boleh memakai SAPA (admin, atau sudah ditetapkan perannya).
func sapaMasuk(c *gin.Context, ctx context.Context) (sapa.Identitas, bool) {
	id, ok := sapaIdentitas(c, ctx)
	if !ok {
		return id, false
	}
	if err := sapa.Akses(id); err != nil {
		utils.ErrorResponseWithCode(c, http.StatusForbidden, capitalFirst(err.Error()), "sapa_tanpa_akses")
		return id, false
	}
	return id, true
}

func capitalFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// sapaGagal memetakan galat Layanan ke respons HTTP. Galat tak dikenal dicatat di log dan dijawab umum.
func sapaGagal(c *gin.Context, apa string, err error) {
	var kf *sapa.ErrKonflik
	var ev *sapa.ErrValidasi
	switch {
	case errors.As(err, &ev):
		pesan := "Data belum lengkap atau tidak valid"
		if len(ev.Rincian) == 1 {
			pesan = ev.Rincian[0]
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": pesan, "errors": ev.Rincian})
	case errors.As(err, &kf):
		utils.ErrorResponse(c, http.StatusConflict, kf.Pesan)
	case errors.Is(err, sapa.ErrTemplateTidakAda):
		utils.ErrorResponse(c, http.StatusConflict, "Template dokumen ini belum tersedia; minta admin mengunggahnya di Pengaturan SAPA")
	case errors.Is(err, sapa.ErrTanpaPeran):
		utils.ErrorResponseWithCode(c, http.StatusForbidden, capitalFirst(err.Error()), "sapa_tanpa_akses")
	case errors.Is(err, sapa.ErrTidakBerhak):
		utils.ErrorResponse(c, http.StatusForbidden, capitalFirst(err.Error()))
	case errors.Is(err, sapa.ErrTidakDitemukan):
		utils.ErrorResponse(c, http.StatusNotFound, "Data tidak ditemukan")
	default:
		log.Println("[SAPA ERROR]", apa+":", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Terjadi kesalahan pada server")
	}
}

// sapaUsulanID membaca UUID usulan dari alamat (:id) dan mengubahnya menjadi id internal. UUID yang tidak sah, usulan yang
// tidak ada, dan usulan yang tidak boleh dilihat pengguna dijawab sama: 404.
func sapaUsulanID(c *gin.Context, ctx context.Context, id sapa.Identitas) (int64, bool) {
	pid, err := sapaLayanan().IDUsulan(ctx, id, c.Param("id"))
	if err != nil {
		sapaGagal(c, "cari usulan", err)
		return 0, false
	}
	return pid, true
}

// sapaIDParam membaca id angka (dokumen) dari alamat.
func sapaIDParam(c *gin.Context, nama string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(nama), 10, 64)
	if err != nil || id < 1 {
		utils.ErrorResponse(c, http.StatusNotFound, "Data tidak ditemukan")
		return 0, false
	}
	return id, true
}

// sapaBody membaca badan permintaan apa adanya (JSON isian formulir) dengan batas ukuran.
func sapaBody(c *gin.Context) ([]byte, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, sapa.MaksDataTahap+1024)
	b, err := io.ReadAll(c.Request.Body)
	if err != nil {
		utils.ErrorResponse(c, http.StatusRequestEntityTooLarge, "Data formulir terlalu besar")
		return nil, false
	}
	return b, true
}

// ---------------------------------------------------------------- pengguna

// GET /sapa/saya: peran SAPA pengguna, hak membuat usulan, dan definisi tahap alur Penjualan.
func GetSapaSaya(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaIdentitas(c, ctx)
	if !ok {
		return
	}
	out := gin.H{
		"nama": id.Nama, "admin": id.Admin, "peran": id.Peran, "peran_label": sapa.PeranLabel(id.Peran),
		"kode_satker": id.KodeSatker, "kode_ue1": id.KodeUE1,
		"punya_akses": true, "boleh_membuat": false, "tahap": sapa.TahapPenjualan,
		// Pilihan isian formulir; dari satu sumber dengan validasi di backend supaya tidak pernah berbeda.
		"jenis_tim": sapa.JenisTimValid, "bentuk": sapa.BentukValid, "item_dokumen": sapa.DaftarItemDokumen,
	}
	if err := sapa.Akses(id); err != nil {
		out["punya_akses"], out["alasan"] = false, capitalFirst(err.Error())
	} else {
		out["boleh_membuat"] = id.Admin || id.Peran == sapa.PeranSatker
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", out)
}

// GET /sapa/referensi/satker?kode=...: nama, kabupaten/kota, dan UE1 satker untuk mengisi formulir usulan baru.
func GetSapaSatker(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaMasuk(c, ctx)
	if !ok {
		return
	}
	s, ref, err := sapaLayanan().CariSatker(ctx, id, c.Query("kode"))
	if err != nil {
		sapaGagal(c, "cari satker", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", gin.H{"satker": s, "ue1": ref, "kode_ue1": sapa.KodeUE1Dari(c.Query("kode"))})
}

// ---------------------------------------------------------------- usulan penjualan

func ListSapaPenjualan(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaMasuk(c, ctx)
	if !ok {
		return
	}
	perHal, _ := strconv.Atoi(c.DefaultQuery("per_halaman", "20"))
	hal, _ := strconv.Atoi(c.DefaultQuery("halaman", "1"))
	if perHal < 1 || perHal > 100 {
		perHal = 20
	}
	if hal < 1 {
		hal = 1
	}
	h, err := sapaLayanan().DaftarUsulan(ctx, id, sapa.FilterDaftar{Q: c.Query("q"), Status: c.Query("status"), Offset: (hal - 1) * perHal, Limit: perHal})
	if err != nil {
		sapaGagal(c, "daftar usulan", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", h)
}

func CreateSapaPenjualan(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaMasuk(c, ctx)
	if !ok {
		return
	}
	var in struct {
		KodeSatker string `json:"kode_satker"`
		NamaSatker string `json:"nama_satker"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Permintaan tidak valid")
		return
	}
	k, err := sapaLayanan().BuatUsulan(ctx, id, in.KodeSatker, in.NamaSatker)
	if err != nil {
		sapaGagal(c, "buat usulan", err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Usulan penjualan dibuat", k)
}

func GetSapaPenjualan(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaMasuk(c, ctx)
	if !ok {
		return
	}
	pid, ok := sapaUsulanID(c, ctx, id)
	if !ok {
		return
	}
	d, err := sapaLayanan().DetailUsulan(ctx, id, pid)
	if err != nil {
		sapaGagal(c, "detail usulan", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", d)
}

// PUT /sapa/penjualan/:id/tahap/:kunci: simpan draf isian formulir (belum divalidasi kelengkapannya).
func SaveSapaTahap(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaMasuk(c, ctx)
	if !ok {
		return
	}
	pid, ok := sapaUsulanID(c, ctx, id)
	if !ok {
		return
	}
	body, ok := sapaBody(c)
	if !ok {
		return
	}
	if err := sapaLayanan().SimpanDraf(ctx, id, pid, c.Param("kunci"), body); err != nil {
		sapaGagal(c, "simpan draf", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Draf tersimpan", nil)
}

// POST /sapa/penjualan/:id/tahap/:kunci/dokumen: validasi isian, isi template Word, dan tandai tahap selesai.
func GenerateSapaDokumen(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaMasuk(c, ctx)
	if !ok {
		return
	}
	pid, ok := sapaUsulanID(c, ctx, id)
	if !ok {
		return
	}
	body, ok := sapaBody(c)
	if !ok {
		return
	}
	h, err := sapaLayanan().Hasilkan(ctx, id, pid, c.Param("kunci"), body)
	if err != nil {
		sapaGagal(c, "hasilkan dokumen", err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Dokumen berhasil dibuat", h)
}

// POST /sapa/penjualan/:id/tahap/:kunci/selesai: catat tahap eksternal (Nadine/SIMAN) selesai.
func CompleteSapaTahap(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaMasuk(c, ctx)
	if !ok {
		return
	}
	pid, ok := sapaUsulanID(c, ctx, id)
	if !ok {
		return
	}
	var in struct {
		Nomor   string `json:"nomor"`
		Tanggal string `json:"tanggal"`
		Catatan string `json:"catatan"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Permintaan tidak valid")
		return
	}
	if err := sapaLayanan().Selesaikan(ctx, id, pid, c.Param("kunci"), in.Nomor, in.Tanggal, in.Catatan); err != nil {
		sapaGagal(c, "selesaikan tahap", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Tahap ditandai selesai", nil)
}

func SkipSapaTahap(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaMasuk(c, ctx)
	if !ok {
		return
	}
	pid, ok := sapaUsulanID(c, ctx, id)
	if !ok {
		return
	}
	var in struct {
		Catatan string `json:"catatan"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Permintaan tidak valid")
		return
	}
	if err := sapaLayanan().Lewati(ctx, id, pid, c.Param("kunci"), in.Catatan); err != nil {
		sapaGagal(c, "lewati tahap", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Tahap dilewati", nil)
}

func ReopenSapaTahap(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaMasuk(c, ctx)
	if !ok {
		return
	}
	pid, ok := sapaUsulanID(c, ctx, id)
	if !ok {
		return
	}
	if err := sapaLayanan().BukaUlang(ctx, id, pid, c.Param("kunci")); err != nil {
		sapaGagal(c, "buka ulang tahap", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Tahap dibuka ulang", nil)
}

// sapaKirimBerkas mengirim berkas Word sebagai unduhan. Nama berkas dikodekan agar aman (termasuk huruf non-ASCII).
func sapaKirimBerkas(c *gin.Context, nama string, berkas []byte) {
	sapaKirimUnduhan(c, nama, sapaDocxType, "dokumen.docx", berkas)
}

// sapaKirimUnduhan: kirim berkas apa pun sebagai unduhan; cadangan dipakai bila nama berkas tak bisa dikodekan.
func sapaKirimUnduhan(c *gin.Context, nama, tipe, cadangan string, berkas []byte) {
	cd := mime.FormatMediaType("attachment", map[string]string{"filename": nama})
	if cd == "" {
		cd = `attachment; filename="` + cadangan + `"`
	}
	c.Header("Content-Disposition", cd)
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, tipe, berkas)
}

func DownloadSapaDokumen(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaMasuk(c, ctx)
	if !ok {
		return
	}
	did, ok := sapaIDParam(c, "id")
	if !ok {
		return
	}
	info, berkas, err := sapaLayanan().UnduhDokumen(ctx, id, did)
	if err != nil {
		sapaGagal(c, "unduh dokumen", err)
		return
	}
	sapaKirimBerkas(c, info.NamaFile, berkas)
}

// ---------------------------------------------------------------- administrasi SAPA (admin)

func ListSapaTemplate(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaIdentitas(c, ctx)
	if !ok {
		return
	}
	list, err := sapaLayanan().DaftarTemplate(ctx, id)
	if err != nil {
		sapaGagal(c, "daftar template", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", list)
}

// POST /sapa/template/:kunci (multipart: berkas, catatan)
func UploadSapaTemplate(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaIdentitas(c, ctx)
	if !ok {
		return
	}
	if !id.Admin {
		sapaGagal(c, "unggah template", sapa.ErrTidakBerhak)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, sapaMaksUnggah)
	fh, err := c.FormFile("berkas")
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			utils.ErrorResponse(c, http.StatusRequestEntityTooLarge, fmt.Sprintf("Berkas template terlalu besar (maksimal %d MB)", sapa.MaksTemplate>>20))
			return
		}
		utils.ErrorResponse(c, http.StatusBadRequest, "Pilih berkas template (.docx)")
		return
	}
	if fh.Size > sapa.MaksTemplate {
		utils.ErrorResponse(c, http.StatusRequestEntityTooLarge, fmt.Sprintf("Berkas template terlalu besar (maksimal %d MB)", sapa.MaksTemplate>>20))
		return
	}
	f, err := fh.Open()
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Berkas tidak dapat dibaca")
		return
	}
	defer f.Close()
	berkas, err := io.ReadAll(io.LimitReader(f, sapa.MaksTemplate+1))
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Berkas tidak dapat dibaca")
		return
	}
	res, err := sapaLayanan().UnggahTemplate(ctx, id, c.Param("kunci"), fh.Filename, berkas, c.PostForm("catatan"))
	if err != nil {
		sapaGagal(c, "unggah template", err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Template tersimpan sebagai versi baru", res)
}

func DownloadSapaTemplate(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaIdentitas(c, ctx)
	if !ok {
		return
	}
	nama, berkas, err := sapaLayanan().UnduhTemplate(ctx, id, c.Param("kunci"))
	if err != nil {
		sapaGagal(c, "unduh template", err)
		return
	}
	sapaKirimBerkas(c, nama, berkas)
}

func ListSapaPeran(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaIdentitas(c, ctx)
	if !ok {
		return
	}
	list, err := sapaLayanan().DaftarPeran(ctx, id, c.Query("q"))
	if err != nil {
		sapaGagal(c, "daftar peran", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", list)
}

func SetSapaPeran(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaIdentitas(c, ctx)
	if !ok {
		return
	}
	var in struct {
		Peran      string `json:"peran"`
		KodeSatker string `json:"kode_satker"`
		KodeUE1    string `json:"kode_ue1"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Permintaan tidak valid")
		return
	}
	if err := sapaLayanan().AturPeran(ctx, id, c.Param("userId"), in.Peran, in.KodeSatker, in.KodeUE1); err != nil {
		sapaGagal(c, "atur peran", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Peran SAPA disimpan", nil)
}

func ListSapaRefUE1(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaIdentitas(c, ctx)
	if !ok {
		return
	}
	list, err := sapaLayanan().DaftarRefUE1(ctx, id)
	if err != nil {
		sapaGagal(c, "daftar UE1", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", list)
}

func SaveSapaRefUE1(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaIdentitas(c, ctx)
	if !ok {
		return
	}
	var in struct {
		Nama       string `json:"nama"`
		Sekretaris string `json:"sekretaris"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Permintaan tidak valid")
		return
	}
	if err := sapaLayanan().SimpanRefUE1(ctx, id, sapa.RefUE1{Kode: c.Param("kode"), Nama: in.Nama, Sekretaris: in.Sekretaris}); err != nil {
		sapaGagal(c, "simpan UE1", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Referensi UE1 disimpan", nil)
}

func DeleteSapaRefUE1(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaIdentitas(c, ctx)
	if !ok {
		return
	}
	if err := sapaLayanan().HapusRefUE1(ctx, id, c.Param("kode")); err != nil {
		sapaGagal(c, "hapus UE1", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Referensi UE1 dihapus", nil)
}
