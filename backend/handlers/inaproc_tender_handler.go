package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/utils"
)

// ============================================================
// TENDER Endpoint 1: Jadwal Tahapan Non Tender
// ============================================================
//
// CATATAN PENTING: parameter kode_klpd di modul TENDER didokumentasikan
// bertipe integer, BERBEDA dari modul RUP yang bertipe string ("K10").
// Kemungkinan modul Tender memakai skema kode KLPD yang berbeda. Konstanta
// di bawah tetap dipakai sebagai default awal — kalau API menolak dengan
// error, sesuaikan value-nya sesuai kode KLPD Kemenkeu yang valid untuk
// modul Tender (tanyakan ke tim pengelola Inaproc kalau perlu).
const kemenkeuKLPDCodeTender = "K10"

func GetJadwalTahapanNonTender(c *gin.Context) {
	cfg := config.Cfg
	if cfg.InaprocToken == "" {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Integrasi Inaproc belum dikonfigurasi (token kosong)")
		return
	}

	kodeKLPD := c.DefaultQuery("kode_klpd", kemenkeuKLPDCodeTender)
	tahun := c.Query("tahun")
	limitStr := c.DefaultQuery("limit", "50")
	cursor := c.Query("cursor")

	if tahun == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Parameter 'tahun' wajib diisi")
		return
	}

	limit := clampLimit(limitStr)

	params := url.Values{}
	params.Set("kode_klpd", kodeKLPD)
	params.Set("tahun", tahun)
	params.Set("limit", strconv.Itoa(limit))
	if cursor != "" {
		params.Set("cursor", cursor)
	}

	body, statusCode, err := callInaprocEndpoint("/api/v1/tender/jadwal-tahapan-non-tender", params)
	if err != nil {
		log.Println("[INAPROC ERROR] gagal request jadwal-tahapan-non-tender:", err)
		utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc (timeout/jaringan)")
		return
	}
	forwardInaprocResponse(c, body, statusCode)
}

type syncJadwalTahapanNonTenderRequest struct {
	KodeKLPD string `json:"kode_klpd"`
	Tahun    string `json:"tahun" binding:"required"`
}

func SyncJadwalTahapanNonTender(c *gin.Context) {
	var req syncJadwalTahapanNonTenderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "tahun wajib diisi")
		return
	}
	if req.KodeKLPD == "" {
		req.KodeKLPD = kemenkeuKLPDCodeTender
	}

	adminUserID := c.GetString("user_id")
	startedAt := time.Now()

	// Hapus data lama untuk kombinasi filter yang sama sebelum menarik ulang,
	// mencegah duplikasi (pola yang sudah diterapkan konsisten di semua
	// endpoint Inaproc sebelumnya).
	if _, err := database.DB.Exec(
		`DELETE FROM inaproc_jadwal_tahapan_non_tender WHERE kd_klpd = @p1 AND tahun_anggaran = @p2`,
		req.KodeKLPD, req.Tahun,
	); err != nil {
		log.Println("[INAPROC SYNC WARN] gagal hapus data lama jadwal-tahapan-non-tender:", err)
	}

	totalSynced := 0
	cursor := ""
	pageCount := 0
	const maxPages = 200

	for {
		pageCount++
		if pageCount > maxPages {
			logInaprocSync("jadwal-tahapan-non-tender", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "Melebihi batas maksimum halaman", adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Sinkronisasi dihentikan: terlalu banyak halaman")
			return
		}

		params := url.Values{}
		params.Set("kode_klpd", req.KodeKLPD)
		params.Set("tahun", req.Tahun)
		params.Set("limit", "1000")
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		body, statusCode, err := callInaprocEndpoint("/api/v1/tender/jadwal-tahapan-non-tender", params)
		if err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal request jadwal-tahapan-non-tender:", err)
			logInaprocSync("jadwal-tahapan-non-tender", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc saat sinkronisasi")
			return
		}

		if statusCode != http.StatusOK {
			errMsg := extractInaprocErrorMessage(body, statusCode)
			logInaprocSync("jadwal-tahapan-non-tender", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, errMsg, adminUserID, startedAt)
			c.JSON(statusCode, gin.H{"success": false, "message": "Sinkronisasi gagal: " + errMsg, "partial_synced": totalSynced})
			return
		}

		var envelope struct {
			Data []map[string]interface{} `json:"data"`
			Meta struct {
				HasMore bool   `json:"has_more"`
				Cursor  string `json:"cursor"`
			} `json:"meta"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal parse jadwal-tahapan-non-tender:", err, "| body:", string(body))
			logInaprocSync("jadwal-tahapan-non-tender", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "gagal parse: "+err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca respons Inaproc saat sinkronisasi")
			return
		}

		if pageCount == 1 && len(envelope.Data) > 0 {
			log.Printf("[INAPROC SYNC DEBUG] Contoh baris jadwal-tahapan-non-tender: %+v", envelope.Data[0])
		}

		for _, row := range envelope.Data {
			if err := insertJadwalTahapanNonTender(row); err != nil {
				log.Println("[INAPROC SYNC WARN] gagal simpan baris jadwal-tahapan-non-tender:", err)
				continue
			}
			totalSynced++
		}

		if !envelope.Meta.HasMore || envelope.Meta.Cursor == "" {
			break
		}
		cursor = envelope.Meta.Cursor
	}

	logInaprocSync("jadwal-tahapan-non-tender", req.KodeKLPD, req.Tahun, "", "success", totalSynced, "", adminUserID, startedAt)
	utils.SuccessResponse(c, http.StatusOK, "Sinkronisasi berhasil", gin.H{"total_synced": totalSynced, "pages_fetched": pageCount})
}

func insertJadwalTahapanNonTender(row map[string]interface{}) error {
	rowKey := generateInaprocRowHash(row)

	kdAkt := getStr(row, "kd_akt")
	kdKLPD := getStr(row, "kd_klpd")
	kdLpse := getStr(row, "kd_lpse")
	kdNontender := getStr(row, "kd_nontender")
	kdSatker := getStr(row, "kd_satker")
	kdSatkerStr := getStr(row, "kd_satker_str")
	kdTahapan := getStr(row, "kd_tahapan")
	namaAkt := getStr(row, "nama_akt")
	namaTahapan := getStr(row, "nama_tahapan")
	tahunAnggaran := getStr(row, "tahun_anggaran")
	tglAkhir := parseInaprocTime(getStr(row, "tgl_akhir"))
	tglAwal := parseInaprocTime(getStr(row, "tgl_awal"))

	_, err := database.DB.Exec(`
		INSERT INTO inaproc_jadwal_tahapan_non_tender (
			row_key, kd_akt, kd_klpd, kd_lpse, kd_nontender, kd_satker, kd_satker_str,
			kd_tahapan, nama_akt, nama_tahapan, tahun_anggaran, tgl_akhir, tgl_awal
		) VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9, @p10, @p11, @p12, @p13)`,
		rowKey, kdAkt, kdKLPD, kdLpse, kdNontender, kdSatker, kdSatkerStr,
		kdTahapan, namaAkt, namaTahapan, tahunAnggaran, tglAkhir, tglAwal,
	)
	return err
}

func ListLocalJadwalTahapanNonTender(c *gin.Context) {
	kodeKLPD := c.DefaultQuery("kode_klpd", kemenkeuKLPDCodeTender)
	tahun := c.Query("tahun")
	limit := clampLimit(c.DefaultQuery("limit", "50"))

	query := `SELECT row_key, kd_klpd, kd_nontender, kd_satker, kd_satker_str,
		nama_akt, nama_tahapan, tahun_anggaran, tgl_awal, tgl_akhir, synced_at
		FROM inaproc_jadwal_tahapan_non_tender WHERE kd_klpd = @p1`
	args := []interface{}{kodeKLPD}

	if tahun != "" {
		query += " AND tahun_anggaran = @p2"
		args = append(args, tahun)
	}

	query = fmt.Sprintf("SELECT TOP (%d) * FROM (%s) t ORDER BY synced_at DESC", limit, query)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal mengambil data lokal: "+err.Error())
		return
	}
	defer rows.Close()

	results, err := rowsToMaps(rows)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal memproses data lokal")
		return
	}
	if results == nil {
		results = []map[string]interface{}{}
	}
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil data lokal", gin.H{"results": results, "count": len(results)})
}
