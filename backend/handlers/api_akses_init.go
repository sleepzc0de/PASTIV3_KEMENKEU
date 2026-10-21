package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/apiakses"
	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/utils"
)

// API Akses Data PASTI: aplikasi lain mendaftar, disetujui superadmin, lalu menarik data lewat kunci API. Pendaftaran publik (api_akses_publik.go), pengelolaan oleh
// superadmin (api_akses_admin.go), pemantauan (api_akses_pantau.go), dan API untuk klien (api_akses_data.go) berbagi satu layanan per proses.

var apiAkses *apiakses.Layanan

// InitApiAkses menyiapkan layanan (penyimpanan, pembatas laju/kuota, pencatat pemakaian) dan menjalankan pekerjaan latar belakangnya. Dipanggil dari main setelah
// database tersambung.
func InitApiAkses() {
	if database.DB == nil || config.Cfg == nil {
		return
	}
	pepper := config.Cfg.APIAksesPepper
	if pepper == "" {
		pepper = config.Cfg.PasswordPepper
	}
	st := &apiakses.Store{DB: database.DB}
	apiAkses = apiakses.NewLayanan(st, st, apiakses.NewPencatat(st, 8192), pepper)
	apiAkses.Mulai()
	p := apiAkses.Pengaturan(context.Background())
	log.Printf("[API-AKSES] API Akses Data aktif (bawaan: %d permintaan/%d detik, kuota %d/%d detik)", p.LimitPermintaan, p.LimitJendelaDetik, p.KuotaPermintaan, p.KuotaPeriodeDetik)
}

// BerhentiApiAkses menuliskan sisa penghitung dan antrean ke database; dipanggil saat server dimatikan.
func BerhentiApiAkses(ctx context.Context) {
	if apiAkses != nil {
		apiAkses.Berhenti(ctx)
	}
}

// GunakanApiAkses mengganti layanan yang dipakai handler (untuk tes). Mengembalikan fungsi pemulih.
func GunakanApiAkses(l *apiakses.Layanan) func() {
	lama := apiAkses
	apiAkses = l
	return func() { apiAkses = lama }
}

// ApiAksesGerbang: middleware autentikasi API klien untuk routes. pakaiKuota=false untuk rute yang tidak menghabiskan kuota. Layanan dibaca saat permintaan datang,
// bukan saat rute dibuat (rute dibuat sebelum/tanpa layanan pada tes).
func ApiAksesGerbang(pakaiKuota bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiAkses == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "API Akses Data belum aktif", "code": apiakses.KodeLayananGagal})
			return
		}
		apiAkses.Gerbang(pakaiKuota)(c)
	}
}

func aksesSiap(c *gin.Context) bool {
	if apiAkses == nil || apiAkses.Store == nil {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "API Akses Data belum aktif")
		return false
	}
	return true
}

func aksesCtx(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), 25*time.Second)
}

// aksesGagal mencatat galat internal dan menjawab 500 tanpa rincian.
func aksesGagal(c *gin.Context, konteks string, err error) {
	if c.Request.Context().Err() != nil { // klien sudah pergi
		c.Abort()
		return
	}
	log.Printf("[API-AKSES ERROR] %s: %v", konteks, err)
	utils.ErrorResponse(c, http.StatusInternalServerError, "Terjadi kesalahan pada server")
}

func aksesID(c *gin.Context, nama string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(nama), 10, 64)
	if err != nil || id <= 0 {
		utils.ErrorResponse(c, http.StatusBadRequest, "ID tidak valid")
		return 0, false
	}
	return id, true
}

// aksesOleh: nama superadmin yang bertindak, untuk kolom "oleh".
func aksesOleh(c *gin.Context) string { return strings.TrimSpace(c.GetString("username")) }
