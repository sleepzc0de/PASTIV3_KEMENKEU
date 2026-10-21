package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/apiakses"
	"pasti-v3-backend/audit"
	"pasti-v3-backend/utils"
)

// Pendaftaran aplikasi klien: formulir publik (tanpa login, karena pemohon adalah aplikasi/pihak lain yang belum punya akun PASTI). Pengamannya: captcha yang sama dengan
// login, batas 5 pendaftaran per jam per IP, batas jumlah pendaftaran yang menunggu, validasi ketat, dan persetujuan manual superadmin sebelum ada kredensial apa pun.

// maksMenunggu: pendaftaran yang menunggu keputusan tidak boleh melebihi ini (menahan banjir pendaftaran palsu dari banyak alamat).
const maksMenunggu = 300

type pendaftaranReq struct {
	apiakses.Pendaftaran
	CaptchaID     string `json:"captcha_id"`
	CaptchaAnswer string `json:"captcha_answer"`
}

type infoDatasetAPI struct {
	Kode      string `json:"kode"`
	Label     string `json:"label"`
	Deskripsi string `json:"deskripsi"`
}

func daftarInfoDataset() []infoDatasetAPI {
	var out []infoDatasetAPI
	for _, d := range apiakses.Katalog() {
		out = append(out, infoDatasetAPI{Kode: d.Kode, Label: d.Label, Deskripsi: d.Deskripsi})
	}
	return out
}

// GetApiAksesInfo: GET /api-akses/info (publik). Dataset yang dapat diminta dan apakah pendaftaran sedang dibuka, untuk formulir pendaftaran.
func GetApiAksesInfo(c *gin.Context) {
	dibuka := true
	if apiAkses != nil {
		dibuka = apiAkses.Pengaturan(c.Request.Context()).PendaftaranDibuka
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", gin.H{"dataset": daftarInfoDataset(), "pendaftaran_dibuka": dibuka && apiAkses != nil})
}

// PostApiAksesPendaftaran: POST /api-akses/pendaftaran (publik). Menyimpan pendaftaran berstatus menunggu; tidak ada kredensial yang terbit sebelum superadmin menyetujui.
func PostApiAksesPendaftaran(c *gin.Context) {
	if !aksesSiap(c) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var req pendaftaranReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Data pendaftaran tidak valid")
		return
	}
	// Captcha lebih dulu (sekali pakai), sebelum pemeriksaan apa pun, seperti pada login.
	if !VerifyCaptcha(req.CaptchaID, req.CaptchaAnswer) {
		utils.ErrorResponseWithCode(c, http.StatusBadRequest, "Kode keamanan salah atau kedaluwarsa. Muat ulang kode lalu coba lagi", "captcha_salah")
		return
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	if !apiAkses.Pengaturan(ctx).PendaftaranDibuka {
		utils.ErrorResponseWithCode(c, http.StatusForbidden, "Pendaftaran aplikasi baru sedang ditutup", "pendaftaran_ditutup")
		return
	}
	galat := req.Pendaftaran.Bersihkan()
	if len(galat) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Periksa kembali isian Anda", "code": "validasi", "errors": galat})
		return
	}
	hitung, err := apiAkses.Store.HitungStatus(ctx)
	if err != nil {
		aksesGagal(c, "hitung pendaftaran menunggu", err)
		return
	}
	if hitung[apiakses.StatusMenunggu] >= maksMenunggu {
		utils.ErrorResponseWithCode(c, http.StatusTooManyRequests, "Antrean pendaftaran sedang penuh. Coba lagi beberapa hari lagi", "antrean_penuh")
		return
	}
	_, kode, err := apiAkses.Store.TambahKlien(ctx, req.Pendaftaran, c.ClientIP())
	if err != nil {
		aksesGagal(c, "simpan pendaftaran", err)
		return
	}
	audit.Tandai(c, audit.Tanda{Kategori: audit.KatAPIAkses, Aksi: "apiakses.daftar", Label: "Pendaftaran aplikasi klien API", Username: "pendaftar-publik",
		ObjekTipe: "klien_api", ObjekID: kode,
		Detail: map[string]interface{}{"nama_aplikasi": req.NamaAplikasi, "instansi": req.Instansi, "jumlah_dataset": len(req.DatasetDiminta)}})
	utils.SuccessResponse(c, http.StatusCreated, "Pendaftaran diterima dan menunggu persetujuan administrator PASTI", gin.H{"kode": kode})
}
