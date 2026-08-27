package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/dto"
	"pasti-v3-backend/utils"
)

const kemenkeuKLPDCode = "K10"

// ============================================================
// Helper umum (dipakai kedua endpoint)
// ============================================================

func getStr(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			switch val := v.(type) {
			case string:
				if val != "" {
					return val
				}
			case float64:
				return strconv.FormatFloat(val, 'f', -1, 64)
			case bool:
				if val {
					return "1"
				}
				return "0"
			}
		}
	}
	return ""
}

func getBool(m map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			if b, ok := v.(bool); ok {
				return b
			}
		}
	}
	return nil
}

func getInt64(m map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			if f, ok := v.(float64); ok {
				return int64(f)
			}
		}
	}
	return nil
}

func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func parseInaprocTime(s string) interface{} {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return t
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func extractInaprocErrorMessage(body []byte, statusCode int) string {
	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Details string `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		if errResp.Error.Details != "" {
			return errResp.Error.Message + ": " + errResp.Error.Details
		}
		return errResp.Error.Message
	}
	return fmt.Sprintf("status %d", statusCode)
}

func logInaprocSync(endpoint, kodeKLPD, tahun, jenisPaket, status string, totalSynced int, errMsg string, adminUserID string, startedAt time.Time) {
	database.DB.Exec(`
		INSERT INTO inaproc_sync_log (endpoint, kode_klpd, tahun, jenis_paket, total_rows_synced, status, error_message, synced_by, started_at)
		VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9)`,
		endpoint, kodeKLPD, tahun, nullIfEmpty(jenisPaket), totalSynced, status,
		nullIfEmpty(errMsg), adminUserID, startedAt,
	)
}

// callInaprocEndpoint melakukan GET generik ke API Inaproc dengan Bearer
// token, dipakai oleh semua endpoint Inaproc yang kita integrasikan.
func callInaprocEndpoint(path string, params url.Values) ([]byte, int, error) {
	cfg := config.Cfg
	reqURL := fmt.Sprintf("%s%s?%s", cfg.InaprocBaseURL, path, params.Encode())

	httpClient := utils.NewSSOHTTPClient()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.InaprocToken)
	req.Header.Set("Accept", "application/json")

	resp, err := utils.DoWithRetry(httpClient, req, 2)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return body, resp.StatusCode, nil
}

// ============================================================
// Endpoint 1: History Kaji Ulang RUP
// ============================================================

func GetHistoryKajiUlang(c *gin.Context) {
	cfg := config.Cfg
	if cfg.InaprocToken == "" {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Integrasi Inaproc belum dikonfigurasi (token kosong)")
		return
	}

	kodeKLPD := c.DefaultQuery("kode_klpd", kemenkeuKLPDCode)
	tahun := c.Query("tahun")
	jenisPaket := c.Query("jenis_paket")
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
	if jenisPaket != "" {
		params.Set("jenis_paket", jenisPaket)
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}

	body, statusCode, err := callInaprocEndpoint("/api/v1/rup/history-kaji-ulang", params)
	if err != nil {
		log.Println("[INAPROC ERROR] gagal request:", err)
		utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc (timeout/jaringan)")
		return
	}
	forwardInaprocResponse(c, body, statusCode)
}

func forwardInaprocResponse(c *gin.Context, body []byte, statusCode int) {
	if statusCode != http.StatusOK {
		log.Println("[INAPROC ERROR] status non-200:", statusCode, "| body:", string(body))
		var errResp map[string]interface{}
		if jsonErr := json.Unmarshal(body, &errResp); jsonErr == nil {
			c.JSON(statusCode, errResp)
			return
		}
		utils.ErrorResponse(c, statusCode, fmt.Sprintf("Inaproc mengembalikan status %d", statusCode))
		return
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca respons Inaproc")
		return
	}
	if raw["data"] == nil {
		raw["data"] = []interface{}{}
	}
	c.JSON(http.StatusOK, raw)
}

func clampLimit(limitStr string) int {
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	return limit
}

func SyncHistoryKajiUlang(c *gin.Context) {
	var req dto.SyncKajiUlangRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "tahun wajib diisi")
		return
	}
	if req.KodeKLPD == "" {
		req.KodeKLPD = kemenkeuKLPDCode
	}

	adminUserID := c.GetString("user_id")
	startedAt := time.Now()
	totalSynced := 0
	cursor := ""
	pageCount := 0
	const maxPages = 200

	for {
		pageCount++
		if pageCount > maxPages {
			logInaprocSync("history-kaji-ulang", req.KodeKLPD, req.Tahun, req.JenisPaket, "failed", totalSynced, "Melebihi batas maksimum halaman", adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Sinkronisasi dihentikan: terlalu banyak halaman")
			return
		}

		params := url.Values{}
		params.Set("kode_klpd", req.KodeKLPD)
		params.Set("tahun", req.Tahun)
		params.Set("limit", "1000")
		if req.JenisPaket != "" {
			params.Set("jenis_paket", req.JenisPaket)
		}
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		body, statusCode, err := callInaprocEndpoint("/api/v1/rup/history-kaji-ulang", params)
		if err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal request:", err)
			logInaprocSync("history-kaji-ulang", req.KodeKLPD, req.Tahun, req.JenisPaket, "failed", totalSynced, err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc saat sinkronisasi")
			return
		}

		if statusCode != http.StatusOK {
			errMsg := extractInaprocErrorMessage(body, statusCode)
			logInaprocSync("history-kaji-ulang", req.KodeKLPD, req.Tahun, req.JenisPaket, "failed", totalSynced, errMsg, adminUserID, startedAt)
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
			logInaprocSync("history-kaji-ulang", req.KodeKLPD, req.Tahun, req.JenisPaket, "failed", totalSynced, "gagal parse: "+err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca respons Inaproc saat sinkronisasi")
			return
		}

		for _, row := range envelope.Data {
			if err := upsertKajiUlang(row); err != nil {
				log.Println("[INAPROC SYNC WARN] gagal simpan baris:", err)
				continue
			}
			totalSynced++
		}

		if !envelope.Meta.HasMore || envelope.Meta.Cursor == "" {
			break
		}
		cursor = envelope.Meta.Cursor
	}

	logInaprocSync("history-kaji-ulang", req.KodeKLPD, req.Tahun, req.JenisPaket, "success", totalSynced, "", adminUserID, startedAt)
	utils.SuccessResponse(c, http.StatusOK, "Sinkronisasi berhasil", gin.H{"total_synced": totalSynced, "pages_fetched": pageCount})
}

// generateRowHash: dipakai karena datamart_id di dokumentasi History Kaji
// Ulang tidak muncul di respons nyata.
func generateRowHash(row map[string]interface{}) string {
	kdSatker := getStr(row, "kd_satker", "kdSatker")
	kdRupLama := getStr(row, "kd_rup_lama", "kdRupLama")
	kdRupBaru := getStr(row, "kd_rup_baru", "kdRupBaru")
	jenisRevisi := getStr(row, "jenis_revisi", "jenisRevisi")
	tglKajiUlang := getStr(row, "tgl_kaji_ulang", "tglKajiUlang")
	tahunAnggaran := getStr(row, "tahun_anggaran", "tahunAnggaran")

	raw := fmt.Sprintf("kju|%s|%s|%s|%s|%s|%s", tahunAnggaran, kdSatker, kdRupLama, kdRupBaru, jenisRevisi, tglKajiUlang)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func upsertKajiUlang(row map[string]interface{}) error {
	datamartID := getStr(row, "datamart_id")
	if datamartID == "" {
		datamartID = generateRowHash(row)
	}

	tahunAnggaran := getStr(row, "tahun_anggaran", "tahunAnggaran")
	kdKLPD := getStr(row, "kd_klpd", "kdKlpd", "kdKLPD")
	namaKLPD := getStr(row, "nama_klpd", "namaKlpd", "namaKLPD")
	jenisKLPD := getStr(row, "jenis_klpd", "jenisKlpd", "jenisKLPD")
	kdSatker := getStr(row, "kd_satker", "kdSatker")
	kdSatkerStr := getStr(row, "kd_satker_str", "kdSatkerStr")
	namaSatker := getStr(row, "nama_satker", "namaSatker")
	kdRupLama := getStr(row, "kd_rup_lama", "kdRupLama")
	kdRupBaru := getStr(row, "kd_rup_baru", "kdRupBaru")
	jenisPaket := getStr(row, "jenis_paket", "jenisPaket")
	jenisRevisi := getStr(row, "jenis_revisi", "jenisRevisi")
	alasan := getStr(row, "alasan_kajiulang", "alasanKajiulang", "alasanKajiUlang")
	tglKajiUlang := parseInaprocTime(getStr(row, "tgl_kaji_ulang", "tglKajiUlang"))
	eventDate := parseInaprocTime(getStr(row, "_event_date", "eventDate"))
	insertedDate := parseInaprocTime(getStr(row, "_inserted_date", "insertedDate"))

	var exists int
	database.DB.QueryRow(`SELECT COUNT(1) FROM inaproc_history_kaji_ulang WHERE datamart_id = @p1`, datamartID).Scan(&exists)

	if exists > 0 {
		_, err := database.DB.Exec(`
			UPDATE inaproc_history_kaji_ulang SET
				tahun_anggaran=@p2, kd_klpd=@p3, nama_klpd=@p4, jenis_klpd=@p5,
				kd_satker=@p6, kd_satker_str=@p7, nama_satker=@p8,
				kd_rup_lama=@p9, kd_rup_baru=@p10, jenis_paket=@p11, jenis_revisi=@p12,
				alasan_kajiulang=@p13, tgl_kaji_ulang=@p14, event_date=@p15,
				inserted_date_src=@p16, updated_at=SYSUTCDATETIME()
			WHERE datamart_id=@p1`,
			datamartID, tahunAnggaran, kdKLPD, namaKLPD, jenisKLPD,
			kdSatker, kdSatkerStr, namaSatker,
			kdRupLama, kdRupBaru, jenisPaket, jenisRevisi,
			alasan, tglKajiUlang, eventDate, insertedDate,
		)
		return err
	}

	_, err := database.DB.Exec(`
		INSERT INTO inaproc_history_kaji_ulang (
			datamart_id, tahun_anggaran, kd_klpd, nama_klpd, jenis_klpd,
			kd_satker, kd_satker_str, nama_satker,
			kd_rup_lama, kd_rup_baru, jenis_paket, jenis_revisi,
			alasan_kajiulang, tgl_kaji_ulang, event_date, inserted_date_src
		) VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9, @p10, @p11, @p12, @p13, @p14, @p15, @p16)`,
		datamartID, tahunAnggaran, kdKLPD, namaKLPD, jenisKLPD,
		kdSatker, kdSatkerStr, namaSatker,
		kdRupLama, kdRupBaru, jenisPaket, jenisRevisi,
		alasan, tglKajiUlang, eventDate, insertedDate,
	)
	return err
}

func ListLocalKajiUlang(c *gin.Context) {
	kodeKLPD := c.DefaultQuery("kode_klpd", kemenkeuKLPDCode)
	tahun := c.Query("tahun")
	jenisPaket := c.Query("jenis_paket")
	limit := clampLimit(c.DefaultQuery("limit", "50"))

	query := `SELECT datamart_id, tahun_anggaran, kd_klpd, nama_klpd, jenis_klpd,
		kd_satker, kd_satker_str, nama_satker, kd_rup_lama, kd_rup_baru,
		jenis_paket, jenis_revisi, alasan_kajiulang, tgl_kaji_ulang, synced_at
		FROM inaproc_history_kaji_ulang WHERE kd_klpd = @p1`
	args := []interface{}{kodeKLPD}
	argIdx := 2

	if tahun != "" {
		query += fmt.Sprintf(" AND tahun_anggaran = @p%d", argIdx)
		args = append(args, tahun)
		argIdx++
	}
	if jenisPaket != "" {
		query += fmt.Sprintf(" AND jenis_paket = @p%d", argIdx)
		args = append(args, jenisPaket)
		argIdx++
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

// ============================================================
// Endpoint 2: Paket Anggaran Penyedia
// ============================================================

func GetPaketAnggaranPenyedia(c *gin.Context) {
	cfg := config.Cfg
	if cfg.InaprocToken == "" {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Integrasi Inaproc belum dikonfigurasi (token kosong)")
		return
	}

	kodeKLPD := c.DefaultQuery("kode_klpd", kemenkeuKLPDCode)
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

	body, statusCode, err := callInaprocEndpoint("/api/v1/rup/paket-anggaran-penyedia", params)
	if err != nil {
		log.Println("[INAPROC ERROR] gagal request paket-anggaran:", err)
		utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc (timeout/jaringan)")
		return
	}
	forwardInaprocResponse(c, body, statusCode)
}

type syncPaketAnggaranRequest struct {
	KodeKLPD string `json:"kode_klpd"`
	Tahun    string `json:"tahun" binding:"required"`
}

func SyncPaketAnggaranPenyedia(c *gin.Context) {
	var req syncPaketAnggaranRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "tahun wajib diisi")
		return
	}
	if req.KodeKLPD == "" {
		req.KodeKLPD = kemenkeuKLPDCode
	}

	adminUserID := c.GetString("user_id")
	startedAt := time.Now()
	totalSynced := 0
	cursor := ""
	pageCount := 0
	const maxPages = 200

	for {
		pageCount++
		if pageCount > maxPages {
			logInaprocSync("paket-anggaran-penyedia", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "Melebihi batas maksimum halaman", adminUserID, startedAt)
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

		body, statusCode, err := callInaprocEndpoint("/api/v1/rup/paket-anggaran-penyedia", params)
		if err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal request paket-anggaran:", err)
			logInaprocSync("paket-anggaran-penyedia", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc saat sinkronisasi")
			return
		}

		if statusCode != http.StatusOK {
			errMsg := extractInaprocErrorMessage(body, statusCode)
			logInaprocSync("paket-anggaran-penyedia", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, errMsg, adminUserID, startedAt)
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
			log.Println("[INAPROC SYNC ERROR] gagal parse paket-anggaran:", err, "| body:", string(body))
			logInaprocSync("paket-anggaran-penyedia", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "gagal parse: "+err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca respons Inaproc saat sinkronisasi")
			return
		}

		if pageCount == 1 && len(envelope.Data) > 0 {
			log.Printf("[INAPROC SYNC DEBUG] Contoh baris paket-anggaran: %+v", envelope.Data[0])
		}

		for _, row := range envelope.Data {
			if err := upsertPaketAnggaran(row); err != nil {
				log.Println("[INAPROC SYNC WARN] gagal simpan baris paket-anggaran:", err)
				continue
			}
			totalSynced++
		}

		if !envelope.Meta.HasMore || envelope.Meta.Cursor == "" {
			break
		}
		cursor = envelope.Meta.Cursor
	}

	logInaprocSync("paket-anggaran-penyedia", req.KodeKLPD, req.Tahun, "", "success", totalSynced, "", adminUserID, startedAt)
	utils.SuccessResponse(c, http.StatusOK, "Sinkronisasi berhasil", gin.H{"total_synced": totalSynced, "pages_fetched": pageCount})
}

// generatePaketRowHash: kombinasi field yang secara logis unik per baris
// paket anggaran (tahun + satker + kode RUP + komponen). Dipakai sebagai
// primary key deterministik karena API tidak menyediakan ID unik eksplisit
// yang terbukti andal (pelajaran dari endpoint History Kaji Ulang).
func generatePaketRowHash(row map[string]interface{}) string {
	tahunAnggaran := getStr(row, "tahun_anggaran")
	kdSatker := getStr(row, "kd_satker")
	kdRup := getStr(row, "kd_rup")
	kdRupLokal := getStr(row, "kd_rup_lokal")
	kdKomponen := getStr(row, "kd_komponen")
	kdKegiatan := getStr(row, "kd_kegiatan")

	raw := fmt.Sprintf("pap|%s|%s|%s|%s|%s|%s", tahunAnggaran, kdSatker, kdRup, kdRupLokal, kdKomponen, kdKegiatan)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func upsertPaketAnggaran(row map[string]interface{}) error {
	rowKey := generatePaketRowHash(row)

	asalDana := getStr(row, "asal_dana")
	asalDanaKLPD := getStr(row, "asal_dana_klpd")
	asalDanaSatker := getStr(row, "asal_dana_satker")
	jenisKLPD := getStr(row, "jenis_klpd")
	kdKegiatan := getStr(row, "kd_kegiatan")
	kdKLPD := getStr(row, "kd_klpd")
	kdKomponen := getStr(row, "kd_komponen")
	kdRup := getStr(row, "kd_rup")
	kdRupLokal := getStr(row, "kd_rup_lokal")
	kdSatker := getStr(row, "kd_satker")
	kdSatkerStr := getStr(row, "kd_satker_str")
	kdSubkegiatan := getStr(row, "kd_subkegiatan")
	mak := getStr(row, "mak")
	namaKLPD := getStr(row, "nama_klpd")
	namaSatker := getStr(row, "nama_satker")
	pagu := getInt64(row, "pagu")
	statusAktif := getBool(row, "status_aktif_rup")
	statusDelete := getBool(row, "status_delete_rup")
	statusUmumkan := getStr(row, "status_umumkan_rup")
	sumberDana := getStr(row, "sumber_dana")
	tahunAnggaran := getStr(row, "tahun_anggaran")
	tahunAnggaranDana := getStr(row, "tahun_anggaran_dana")

	var exists int
	database.DB.QueryRow(`SELECT COUNT(1) FROM inaproc_paket_anggaran_penyedia WHERE row_key = @p1`, rowKey).Scan(&exists)

	if exists > 0 {
		_, err := database.DB.Exec(`
			UPDATE inaproc_paket_anggaran_penyedia SET
				asal_dana=@p2, asal_dana_klpd=@p3, asal_dana_satker=@p4, jenis_klpd=@p5,
				kd_kegiatan=@p6, kd_klpd=@p7, kd_komponen=@p8, kd_rup=@p9, kd_rup_lokal=@p10,
				kd_satker=@p11, kd_satker_str=@p12, kd_subkegiatan=@p13, mak=@p14,
				nama_klpd=@p15, nama_satker=@p16, pagu=@p17, status_aktif_rup=@p18,
				status_delete_rup=@p19, status_umumkan_rup=@p20, sumber_dana=@p21,
				tahun_anggaran=@p22, tahun_anggaran_dana=@p23, updated_at=SYSUTCDATETIME()
			WHERE row_key=@p1`,
			rowKey, asalDana, asalDanaKLPD, asalDanaSatker, jenisKLPD,
			kdKegiatan, kdKLPD, kdKomponen, kdRup, kdRupLokal,
			kdSatker, kdSatkerStr, kdSubkegiatan, mak,
			namaKLPD, namaSatker, pagu, statusAktif,
			statusDelete, statusUmumkan, sumberDana,
			tahunAnggaran, tahunAnggaranDana,
		)
		return err
	}

	_, err := database.DB.Exec(`
		INSERT INTO inaproc_paket_anggaran_penyedia (
			row_key, asal_dana, asal_dana_klpd, asal_dana_satker, jenis_klpd,
			kd_kegiatan, kd_klpd, kd_komponen, kd_rup, kd_rup_lokal,
			kd_satker, kd_satker_str, kd_subkegiatan, mak,
			nama_klpd, nama_satker, pagu, status_aktif_rup,
			status_delete_rup, status_umumkan_rup, sumber_dana,
			tahun_anggaran, tahun_anggaran_dana
		) VALUES (
			@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9, @p10,
			@p11, @p12, @p13, @p14, @p15, @p16, @p17, @p18, @p19, @p20,
			@p21, @p22, @p23
		)`,
		rowKey, asalDana, asalDanaKLPD, asalDanaSatker, jenisKLPD,
		kdKegiatan, kdKLPD, kdKomponen, kdRup, kdRupLokal,
		kdSatker, kdSatkerStr, kdSubkegiatan, mak,
		namaKLPD, namaSatker, pagu, statusAktif,
		statusDelete, statusUmumkan, sumberDana,
		tahunAnggaran, tahunAnggaranDana,
	)
	return err
}

func ListLocalPaketAnggaran(c *gin.Context) {
	kodeKLPD := c.DefaultQuery("kode_klpd", kemenkeuKLPDCode)
	tahun := c.Query("tahun")
	limit := clampLimit(c.DefaultQuery("limit", "50"))

	query := `SELECT row_key, kd_klpd, nama_klpd, kd_satker, nama_satker, kd_rup, kd_rup_lokal,
		kd_kegiatan, kd_subkegiatan, kd_komponen, mak, pagu, sumber_dana, asal_dana,
		status_aktif_rup, status_delete_rup, status_umumkan_rup,
		tahun_anggaran, tahun_anggaran_dana, synced_at
		FROM inaproc_paket_anggaran_penyedia WHERE kd_klpd = @p1`
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

// ============================================================
// Riwayat sinkronisasi (dipakai semua endpoint Inaproc)
// ============================================================

func GetSyncHistory(c *gin.Context) {
	rows, err := database.DB.Query(`
		SELECT TOP 20 endpoint, kode_klpd, tahun, ISNULL(jenis_paket, ''), total_rows_synced, status,
		       ISNULL(error_message, ''), started_at, finished_at
		FROM inaproc_sync_log
		ORDER BY finished_at DESC`)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal mengambil riwayat sinkronisasi")
		return
	}
	defer rows.Close()

	results, err := rowsToMaps(rows)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal memproses riwayat sinkronisasi")
		return
	}
	if results == nil {
		results = []map[string]interface{}{}
	}
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil riwayat sinkronisasi", results)
}
