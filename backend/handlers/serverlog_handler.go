package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/middleware"
	"pasti-v3-backend/serverlog"
	"pasti-v3-backend/utils"
)

// Log server: hanya galat (error) dan peringatan (warning), khusus superadmin (dipasang di routes). Hanya membaca; tidak ada rute untuk mengubah atau menghapus baris
// (baris dihapus otomatis oleh pembersihan retensi). Catatan info/debug dan log akses per permintaan memang tidak pernah disimpan.

var (
	logServerStore       *serverlog.Store
	logServerBatalBersih context.CancelFunc
)

// InitServerLog memasang perekam log server (menulis berkelompok ke database) dan pembersih retensi. Dipanggil dari main setelah database tersambung.
func InitServerLog() {
	if database.DB == nil {
		return
	}
	logServerStore = serverlog.Mulai(database.DB)
	ctx, batal := context.WithCancel(context.Background())
	logServerBatalBersih = batal
	p := pengaturanLogServer()
	serverlog.MulaiPembersihan(ctx, logServerStore,
		time.Duration(p.RetensiErrorHari)*24*time.Hour, time.Duration(p.RetensiWarnHari)*24*time.Hour, p.MaksBaris,
		func(dihapus int64, err error) {
			if err != nil {
				log.Println("[LOGSERVER WARN] gagal membersihkan log server lama:", err)
				return
			}
			log.Printf("[LOGSERVER INFO] %d baris log server lama dihapus", dihapus)
		})
	log.Printf("[INFO] Log server aktif: menyimpan %s, retensi error %d hari, warning %d hari, maks %d baris", p.Simpan, p.RetensiErrorHari, p.RetensiWarnHari, p.MaksBaris)
}

// BerhentiServerLog menyiram sisa antrean log ke database; dipanggil saat server dimatikan.
func BerhentiServerLog(ctx context.Context) {
	if logServerBatalBersih != nil {
		logServerBatalBersih()
	}
	serverlog.Berhenti(ctx)
}

// GunakanServerLogStore mengganti penyimpanan log server yang dibaca handler (untuk tes). Mengembalikan fungsi pemulih.
func GunakanServerLogStore(s *serverlog.Store) func() {
	lama := logServerStore
	logServerStore = s
	return func() { logServerStore = lama }
}

// PengaturanLogServer: nilai yang berlaku, ditampilkan di halaman supaya jelas apa yang disimpan dan berapa lama.
type PengaturanLogServer struct {
	Level            string `json:"level"`  // ambang cetak konsol
	Simpan           string `json:"simpan"` // warn (error + warning), error, atau off
	RetensiErrorHari int    `json:"retensi_error_hari"`
	RetensiWarnHari  int    `json:"retensi_warn_hari"`
	MaksBaris        int    `json:"maks_baris"`
	LambatMS         int    `json:"lambat_ms"`
}

func pengaturanLogServer() PengaturanLogServer {
	p := PengaturanLogServer{Level: string(serverlog.Warn), Simpan: string(serverlog.SimpanGalatDanPeringatan), RetensiErrorHari: 30, RetensiWarnHari: 7, MaksBaris: 50000, LambatMS: 3000}
	if c := config.Cfg; c != nil {
		p.Level = string(LevelKonsolLog())
		p.Simpan = string(serverlog.BacaModeSimpan(c.LogSimpan))
		p.RetensiErrorHari, p.RetensiWarnHari, p.MaksBaris, p.LambatMS = c.LogRetensiErrorHari, c.LogRetensiWarnHari, c.LogMaksBaris, c.LogLambatMS
	}
	return p
}

// LevelKonsolLog: ambang cetak konsol yang berlaku (LOG_LEVEL; bawaan warn di production dan info di pengembangan).
func LevelKonsolLog() serverlog.Level {
	bawaan := serverlog.Info
	if config.Cfg == nil {
		return bawaan
	}
	if config.Cfg.AppEnv == "production" {
		bawaan = serverlog.Warn
	}
	return serverlog.BacaLevel(config.Cfg.LogLevel, bawaan)
}

func logServerCtx(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), 25*time.Second)
}

func logServerSiap(c *gin.Context) bool {
	if logServerStore == nil {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Log server belum aktif")
		return false
	}
	return true
}

func logServerGagal(c *gin.Context, konteks string, err error) {
	if c.Request.Context().Err() != nil {
		return // pembaca sudah pergi
	}
	log.Printf("[LOGSERVER ERROR] %s: %v", konteks, err)
	utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca log server")
}

// penyaringLogServer menyusun penyaring dari parameter kueri: level (error|warn), sumber, q, dari, sampai, halaman, per_halaman.
func penyaringLogServer(c *gin.Context) (serverlog.Penyaring, error) {
	var p serverlog.Penyaring
	var err error
	if p.Dari, err = bacaWaktuAudit(c.Query("dari"), false); err != nil {
		return p, err
	}
	if p.Sampai, err = bacaWaktuAudit(c.Query("sampai"), true); err != nil {
		return p, err
	}
	if !p.Dari.IsZero() && !p.Sampai.IsZero() && !p.Sampai.After(p.Dari) {
		return p, errors.New("batas akhir waktu harus setelah batas awal")
	}
	switch l := c.Query("level"); l {
	case "", "semua":
	case string(serverlog.Error), string(serverlog.Warn):
		p.Level = l
	default:
		return p, fmt.Errorf("level %q tidak dikenal (pilih error atau warn)", l)
	}
	p.Sumber = c.Query("sumber")
	p.Q = c.Query("q")
	p.Halaman, p.PerHalaman = intQuery(c, "halaman"), intQuery(c, "per_halaman")
	return p, nil
}

// GET /log-server: satu halaman log (galat dan peringatan) menurut filter, terbaru dulu.
func GetServerLog(c *gin.Context) {
	if !logServerSiap(c) {
		return
	}
	p, err := penyaringLogServer(c)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := logServerCtx(c)
	defer cancel()
	d, err := logServerStore.Cari(ctx, p)
	if err != nil {
		logServerGagal(c, "cari", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", d)
}

// GET /log-server/ringkasan: ikhtisar 24 jam dan 7 hari terakhir, grafik per jam, sumber teratas, kejadian paling berulang, keadaan perekam, dan tekanan beban.
func GetServerLogRingkasan(c *gin.Context) {
	if !logServerSiap(c) {
		return
	}
	ctx, cancel := logServerCtx(c)
	defer cancel()
	r, err := logServerStore.Ringkasan(ctx, time.Now())
	if err != nil {
		logServerGagal(c, "ringkasan", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", gin.H{
		"ringkasan":  r,
		"perekam":    serverlog.StatistikDefault(),
		"pengaturan": pengaturanLogServer(),
		"ketahanan":  middleware.StatistikKetahanan(),
	})
}

// GET /log-server/opsi: sumber yang pernah tercatat, untuk filter.
func GetServerLogOpsi(c *gin.Context) {
	if !logServerSiap(c) {
		return
	}
	ctx, cancel := logServerCtx(c)
	defer cancel()
	sumber, err := logServerStore.Opsi(ctx)
	if err != nil {
		logServerGagal(c, "opsi", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", gin.H{"sumber": sumber, "level": []string{string(serverlog.Error), string(serverlog.Warn)}})
}

// GET /log-server/:id: satu baris lengkap (termasuk jejak tumpukan).
func GetServerLogDetail(c *gin.Context) {
	if !logServerSiap(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		utils.ErrorResponse(c, http.StatusBadRequest, "ID log tidak valid")
		return
	}
	ctx, cancel := logServerCtx(c)
	defer cancel()
	b, err := logServerStore.Detail(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		utils.ErrorResponse(c, http.StatusNotFound, "Log tidak ditemukan (mungkin sudah dihapus oleh pembersihan retensi)")
		return
	}
	if err != nil {
		logServerGagal(c, "detail", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", b)
}
