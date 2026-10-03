package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/database"
	"pasti-v3-backend/digitalisasi"
	"pasti-v3-backend/utils"
)

// InitDigitalisasi menyiapkan pengelola sinkronisasi; dipanggil sekali saat server dimulai, setelah koneksi
// database tersedia.
func InitDigitalisasi() {
	digitalisasi.Init(database.DB, database.SLDKDB)
}

func dgReady(c *gin.Context) bool {
	if digitalisasi.Default == nil {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Fitur digitalisasi aset belum siap")
		return false
	}
	return true
}

// maskLog menyembunyikan isi galat dari non-admin: pesan galat SLDK/SQL bisa memuat nama server atau tabel.
func maskLog(e digitalisasi.LogEntry, admin bool) digitalisasi.LogEntry {
	if !admin {
		if e.Status == digitalisasi.StatusGagal {
			msg := "Sinkronisasi gagal"
			e.Pesan = &msg
		}
		e.DijalankanOleh = nil
	}
	return e
}

type dgDatasetStatus struct {
	Key            string                 `json:"key"`
	Label          string                 `json:"label"`
	Deskripsi      string                 `json:"deskripsi"`
	Tabel          string                 `json:"tabel"`
	Peta           bool                   `json:"peta"`
	JumlahBaris    int64                  `json:"jumlah_baris"`
	Terakhir       *digitalisasi.LogEntry `json:"terakhir"`
	TerakhirSukses *digitalisasi.LogEntry `json:"terakhir_sukses"`
}

// GetDigitalisasiSinkronisasi: status tiap dataset, antrean yang sedang berjalan, dan riwayat terbaru.
func GetDigitalisasiSinkronisasi(c *gin.Context) {
	if !dgReady(c) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), dgTimeout)
	defer cancel()
	admin := dgIsAdmin(c)
	m := digitalisasi.Default

	counts, err := dgTableCounts(ctx)
	if err != nil {
		dgFail(c, "jumlah baris", err)
		return
	}
	latest, lastOK, err := m.LatestPerDataset(ctx)
	if err != nil {
		dgFail(c, "riwayat terakhir", err)
		return
	}
	history, err := m.History(ctx, 30)
	if err != nil {
		dgFail(c, "riwayat", err)
		return
	}
	for i := range history {
		history[i] = maskLog(history[i], admin)
	}

	list := make([]dgDatasetStatus, 0, len(digitalisasi.Datasets))
	for _, ds := range digitalisasi.Datasets {
		s := dgDatasetStatus{Key: ds.Key, Label: ds.Label, Deskripsi: ds.Description, Tabel: ds.Table, Peta: ds.Geo, JumlahBaris: counts[ds.Key]}
		if e, ok := latest[ds.Key]; ok {
			e = maskLog(e, admin)
			s.Terakhir = &e
		}
		if e, ok := lastOK[ds.Key]; ok {
			e = maskLog(e, admin)
			s.TerakhirSukses = &e
		}
		list = append(list, s)
	}

	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil status sinkronisasi", gin.H{
		"sldk_tersedia": database.SLDKDB != nil,
		"aktif":         m.Active(),
		"datasets":      list,
		"riwayat":       history,
	})
}

// StartDigitalisasiSync (admin): memasukkan dataset terpilih ke antrean dan langsung kembali; sinkronisasi
// berjalan di latar belakang karena query ke SLDK bisa memakan waktu lama.
func StartDigitalisasiSync(c *gin.Context) {
	if !dgReady(c) {
		return
	}
	var body struct {
		Datasets []string `json:"datasets"`
		Semua    bool     `json:"semua"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid")
		return
	}
	keys := body.Datasets
	if body.Semua {
		keys = digitalisasi.Keys()
	}

	user := c.GetString("username")
	info, err := digitalisasi.Default.Start(keys, user)
	switch {
	case err == nil:
		log.Printf("[DIGITALISASI] sinkronisasi %v dimulai oleh %s", info.Datasets, user)
		c.JSON(http.StatusAccepted, gin.H{"success": true, "message": "Sinkronisasi dimulai", "data": gin.H{"aktif": info}})
	case errors.Is(err, digitalisasi.ErrBusy):
		utils.ErrorResponse(c, http.StatusConflict, "Sinkronisasi lain sedang berjalan. Tunggu sampai selesai atau batalkan dulu.")
	case errors.Is(err, digitalisasi.ErrNoSLDK):
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Koneksi ke SLDK belum tersedia. Periksa pengaturan SLDK_DB_* di server.")
	case errors.Is(err, digitalisasi.ErrUnknownDataset), errors.Is(err, digitalisasi.ErrNothingToRun):
		utils.ErrorResponse(c, http.StatusBadRequest, "Pilih minimal satu dataset yang valid")
	default:
		log.Println("[DIGITALISASI ERROR] gagal memulai sinkronisasi:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal memulai sinkronisasi")
	}
}

// CancelDigitalisasiSync (admin): membatalkan antrean yang sedang berjalan.
func CancelDigitalisasiSync(c *gin.Context) {
	if !dgReady(c) {
		return
	}
	cancelled := digitalisasi.Default.Cancel()
	if cancelled {
		log.Printf("[DIGITALISASI] sinkronisasi dibatalkan oleh %s", c.GetString("username"))
	}
	utils.SuccessResponse(c, http.StatusOK, "Permintaan pembatalan diterima", gin.H{"dibatalkan": cancelled})
}
