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
	"strings"
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

// parseInaprocTime mencoba beberapa format tanggal karena API Inaproc tidak
// konsisten — sebagian field full RFC3339 (2025-08-01T00:00:00Z), sebagian
// lain hanya date-only (2025-08-01) tanpa jam.
func parseInaprocTime(s string) interface{} {
	if s == "" {
		return nil
	}
	formats := []string{time.RFC3339, "2006-01-02"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return nil
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
	b, err := json.Marshal(row)
	if err != nil {
		// Fallback yang sangat tidak mungkin terjadi, tapi tetap sediakan
		// nilai unik agar tidak collision total.
		b = []byte(fmt.Sprintf("%v", row))
	}
	sum := sha256.Sum256(b)
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

// getBoolStr mengonversi field boolean yang dikirim API sebagai string
// ("true"/"false") menjadi *bool untuk disimpan sebagai BIT di database.
func getBoolStr(m map[string]interface{}, keys ...string) interface{} {
	s := getStr(m, keys...)
	switch strings.ToLower(s) {
	case "true":
		return true
	case "false":
		return false
	default:
		return nil
	}
}

// getBoolAny mengonversi field boolean yang bisa dikirim API sebagai
// boolean JSON asli (true/false) ATAU string ("true"/"false") — inkonsistensi
// ini sudah terbukti terjadi antar endpoint Inaproc yang berbeda.
func getBoolAny(m map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			switch val := v.(type) {
			case bool:
				return val
			case string:
				switch strings.ToLower(val) {
				case "true":
					return true
				case "false":
					return false
				}
			}
		}
	}
	return nil
}

// ============================================================
// Endpoint 3: Paket Penyedia
// ============================================================

func GetPaketPenyedia(c *gin.Context) {
	cfg := config.Cfg
	if cfg.InaprocToken == "" {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Integrasi Inaproc belum dikonfigurasi (token kosong)")
		return
	}

	kodeKLPD := c.DefaultQuery("kode_klpd", kemenkeuKLPDCode)
	tahun := c.Query("tahun")
	status := c.Query("status")
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
	if status != "" {
		params.Set("status", status)
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}

	body, statusCode, err := callInaprocEndpoint("/api/v1/rup/paket-penyedia", params)
	if err != nil {
		log.Println("[INAPROC ERROR] gagal request paket-penyedia:", err)
		utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc (timeout/jaringan)")
		return
	}
	forwardInaprocResponse(c, body, statusCode)
}

type syncPaketPenyediaRequest struct {
	KodeKLPD string `json:"kode_klpd"`
	Tahun    string `json:"tahun" binding:"required"`
	Status   string `json:"status"`
}

func SyncPaketPenyedia(c *gin.Context) {
	var req syncPaketPenyediaRequest
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
	const maxPages = 500 // paket penyedia biasanya lebih banyak baris dari 2 endpoint sebelumnya

	for {
		pageCount++
		if pageCount > maxPages {
			logInaprocSync("paket-penyedia", req.KodeKLPD, req.Tahun, req.Status, "failed", totalSynced, "Melebihi batas maksimum halaman", adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Sinkronisasi dihentikan: terlalu banyak halaman")
			return
		}

		params := url.Values{}
		params.Set("kode_klpd", req.KodeKLPD)
		params.Set("tahun", req.Tahun)
		params.Set("limit", "1000")
		if req.Status != "" {
			params.Set("status", req.Status)
		}
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		body, statusCode, err := callInaprocEndpoint("/api/v1/rup/paket-penyedia", params)
		if err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal request paket-penyedia:", err)
			logInaprocSync("paket-penyedia", req.KodeKLPD, req.Tahun, req.Status, "failed", totalSynced, err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc saat sinkronisasi")
			return
		}

		if statusCode != http.StatusOK {
			errMsg := extractInaprocErrorMessage(body, statusCode)
			logInaprocSync("paket-penyedia", req.KodeKLPD, req.Tahun, req.Status, "failed", totalSynced, errMsg, adminUserID, startedAt)
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
			log.Println("[INAPROC SYNC ERROR] gagal parse paket-penyedia:", err, "| body:", string(body))
			logInaprocSync("paket-penyedia", req.KodeKLPD, req.Tahun, req.Status, "failed", totalSynced, "gagal parse: "+err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca respons Inaproc saat sinkronisasi")
			return
		}

		if pageCount == 1 && len(envelope.Data) > 0 {
			log.Printf("[INAPROC SYNC DEBUG] Contoh baris paket-penyedia: %+v", envelope.Data[0])
		}

		for _, row := range envelope.Data {
			if err := upsertPaketPenyedia(row); err != nil {
				log.Println("[INAPROC SYNC WARN] gagal simpan baris paket-penyedia:", err)
				continue
			}
			totalSynced++
		}

		if !envelope.Meta.HasMore || envelope.Meta.Cursor == "" {
			break
		}
		cursor = envelope.Meta.Cursor
	}

	logInaprocSync("paket-penyedia", req.KodeKLPD, req.Tahun, req.Status, "success", totalSynced, "", adminUserID, startedAt)
	utils.SuccessResponse(c, http.StatusOK, "Sinkronisasi berhasil", gin.H{"total_synced": totalSynced, "pages_fetched": pageCount})
}

// generatePaketPenyediaRowHash: hash SELURUH isi baris (bukan kombinasi
// field pilihan), pelajaran dari 2 endpoint sebelumnya di mana ID/kombinasi
// field yang terlihat unik ternyata tidak selalu benar-benar unik.
func generatePaketPenyediaRowHash(row map[string]interface{}) string {
	b, err := json.Marshal(row)
	if err != nil {
		b = []byte(fmt.Sprintf("%v", row))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func upsertPaketPenyedia(row map[string]interface{}) error {
	rowKey := generatePaketPenyediaRowHash(row)

	tahunAnggaran := getStr(row, "tahun_anggaran")
	kdKLPD := getStr(row, "kd_klpd")
	namaKLPD := getStr(row, "nama_klpd")
	jenisKLPD := getStr(row, "jenis_klpd")
	kdSatker := getStr(row, "kd_satker")
	kdSatkerStr := getStr(row, "kd_satker_str")
	namaSatker := getStr(row, "nama_satker")
	kdRup := getStr(row, "kd_rup")
	namaPaket := getStr(row, "nama_paket")
	pagu := getInt64FromAny(row, "pagu")
	kdMetodePengadaan := getStr(row, "kd_metode_pengadaan")
	metodePengadaan := getStr(row, "metode_pengadaan")
	kdJenisPengadaan := getStr(row, "kd_jenis_pengadaan")
	jenisPengadaan := getStr(row, "jenis_pengadaan")
	statusPradipa := getStr(row, "status_pradipa")
	statusPdn := getStr(row, "status_pdn")
	statusUkm := getStr(row, "status_ukm")
	alasanNonUkm := getStr(row, "alasan_non_ukm")
	statusKonsolidasi := getStr(row, "status_konsolidasi")
	tipePaket := getStr(row, "tipe_paket")
	kdRupSwakelola := getStr(row, "kd_rup_swakelola")
	kdRupLokal := getStr(row, "kd_rup_lokal")
	volumePekerjaan := getStr(row, "volume_pekerjaan")
	uraianPekerjaan := getStr(row, "urarian_pekerjaan", "uraian_pekerjaan")
	spesifikasiPekerjaan := getStr(row, "spesifikasi_pekerjaan")
	tglAwalPemilihan := parseInaprocTime(getStr(row, "tgl_awal_pemilihan"))
	tglAkhirPemilihan := parseInaprocTime(getStr(row, "tgl_akhir_pemilihan"))
	tglAwalKontrak := parseInaprocTime(getStr(row, "tgl_awal_kontrak"))
	tglAkhirKontrak := parseInaprocTime(getStr(row, "tgl_akhir_kontrak"))
	tglAwalPemanfaatan := parseInaprocTime(getStr(row, "tgl_awal_pemanfaatan"))
	tglAkhirPemanfaatan := parseInaprocTime(getStr(row, "tgl_akhir_pemanfaatan"))
	tglBuatPaket := parseInaprocTime(getStr(row, "tgl_buat_paket"))
	tglPengumumanPaket := parseInaprocTime(getStr(row, "tgl_pengumuman_paket"))
	nipPpk := getStr(row, "nip_ppk")
	namaPpk := getStr(row, "nama_ppk")
	usernamePpk := getStr(row, "username_ppk")
	statusAktifRup := getBoolStr(row, "status_aktif_rup")
	statusDeleteRup := getBoolStr(row, "status_delete_rup")
	statusUmumkanRup := getStr(row, "status_umumkan_rup")
	statusDikecualikan := getBoolStr(row, "status_dikecualikan")
	alasanDikecualikan := getStr(row, "alasan_dikecualikan")
	tahunPertama := getStr(row, "tahun_pertama")
	kodeRupTahunPertama := getStr(row, "kode_rup_tahun_pertama")
	nomorKontrak := getStr(row, "nomor_kontrak")
	sppAspekEkonomi := getBoolStr(row, "spp_aspek_ekonomi")
	sppAspekSosial := getBoolStr(row, "spp_aspek_sosial")
	sppAspekLingkungan := getBoolStr(row, "spp_aspek_lingkungan")
	detailLokasi := getStr(row, "detail_lokasi")
	eventDate := parseInaprocTime(getStr(row, "_event_date"))
	insertedDate := parseInaprocTime(getStr(row, "_inserted_date"))

	var exists int
	database.DB.QueryRow(`SELECT COUNT(1) FROM inaproc_paket_penyedia WHERE row_key = @p1`, rowKey).Scan(&exists)

	if exists > 0 {
		// Baris identik persis (row_key sama) tidak perlu di-UPDATE ulang,
		// cukup perbarui timestamp sinkronisasi.
		_, err := database.DB.Exec(`UPDATE inaproc_paket_penyedia SET updated_at=SYSUTCDATETIME() WHERE row_key=@p1`, rowKey)
		return err
	}

	_, err := database.DB.Exec(`
		INSERT INTO inaproc_paket_penyedia (
			row_key, tahun_anggaran, kd_klpd, nama_klpd, jenis_klpd,
			kd_satker, kd_satker_str, nama_satker, kd_rup, nama_paket, pagu,
			kd_metode_pengadaan, metode_pengadaan, kd_jenis_pengadaan, jenis_pengadaan,
			status_pradipa, status_pdn, status_ukm, alasan_non_ukm, status_konsolidasi,
			tipe_paket, kd_rup_swakelola, kd_rup_lokal, volume_pekerjaan,
			uraian_pekerjaan, spesifikasi_pekerjaan,
			tgl_awal_pemilihan, tgl_akhir_pemilihan, tgl_awal_kontrak, tgl_akhir_kontrak,
			tgl_awal_pemanfaatan, tgl_akhir_pemanfaatan, tgl_buat_paket, tgl_pengumuman_paket,
			nip_ppk, nama_ppk, username_ppk,
			status_aktif_rup, status_delete_rup, status_umumkan_rup,
			status_dikecualikan, alasan_dikecualikan, tahun_pertama, kode_rup_tahun_pertama,
			nomor_kontrak, spp_aspek_ekonomi, spp_aspek_sosial, spp_aspek_lingkungan,
			detail_lokasi, event_date, inserted_date_src
		) VALUES (
			@p1, @p2, @p3, @p4, @p5,
			@p6, @p7, @p8, @p9, @p10, @p11,
			@p12, @p13, @p14, @p15,
			@p16, @p17, @p18, @p19, @p20,
			@p21, @p22, @p23, @p24,
			@p25, @p26,
			@p27, @p28, @p29, @p30,
			@p31, @p32, @p33, @p34,
			@p35, @p36, @p37,
			@p38, @p39, @p40,
			@p41, @p42, @p43, @p44,
			@p45, @p46, @p47, @p48,
			@p49, @p50, @p51
		)`,
		rowKey, tahunAnggaran, kdKLPD, namaKLPD, jenisKLPD,
		kdSatker, kdSatkerStr, namaSatker, kdRup, namaPaket, pagu,
		kdMetodePengadaan, metodePengadaan, kdJenisPengadaan, jenisPengadaan,
		statusPradipa, statusPdn, statusUkm, alasanNonUkm, statusKonsolidasi,
		tipePaket, kdRupSwakelola, kdRupLokal, volumePekerjaan,
		uraianPekerjaan, spesifikasiPekerjaan,
		tglAwalPemilihan, tglAkhirPemilihan, tglAwalKontrak, tglAkhirKontrak,
		tglAwalPemanfaatan, tglAkhirPemanfaatan, tglBuatPaket, tglPengumumanPaket,
		nipPpk, namaPpk, usernamePpk,
		statusAktifRup, statusDeleteRup, statusUmumkanRup,
		statusDikecualikan, alasanDikecualikan, tahunPertama, kodeRupTahunPertama,
		nomorKontrak, sppAspekEkonomi, sppAspekSosial, sppAspekLingkungan,
		detailLokasi, eventDate, insertedDate,
	)
	return err
}

func ListLocalPaketPenyedia(c *gin.Context) {
	kodeKLPD := c.DefaultQuery("kode_klpd", kemenkeuKLPDCode)
	tahun := c.Query("tahun")
	status := c.Query("status")
	limit := clampLimit(c.DefaultQuery("limit", "50"))

	query := `SELECT row_key, kd_klpd, nama_klpd, kd_satker, nama_satker, kd_rup, nama_paket,
		pagu, metode_pengadaan, jenis_pengadaan, status_umumkan_rup, nama_ppk,
		tgl_awal_pemilihan, tgl_akhir_pemilihan, tahun_anggaran, synced_at
		FROM inaproc_paket_penyedia WHERE kd_klpd = @p1`
	args := []interface{}{kodeKLPD}
	argIdx := 2

	if tahun != "" {
		query += fmt.Sprintf(" AND tahun_anggaran = @p%d", argIdx)
		args = append(args, tahun)
		argIdx++
	}
	if status != "" {
		query += fmt.Sprintf(" AND status_umumkan_rup = @p%d", argIdx)
		args = append(args, status)
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

// getInt64FromAny mengonversi field numerik yang mungkin dikirim sebagai
// string ATAU float64 oleh API (tidak konsisten antar endpoint Inaproc).
func getInt64FromAny(m map[string]interface{}, key string) interface{} {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	switch val := v.(type) {
	case float64:
		return int64(val)
	case string:
		if n, err := strconv.ParseInt(val, 10, 64); err == nil {
			return n
		}
	}
	return nil
}

// ============================================================
// Endpoint 4: Paket Swakelola
// ============================================================

func GetPaketSwakelola(c *gin.Context) {
	cfg := config.Cfg
	if cfg.InaprocToken == "" {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Integrasi Inaproc belum dikonfigurasi (token kosong)")
		return
	}

	kodeKLPD := c.DefaultQuery("kode_klpd", kemenkeuKLPDCode)
	tahun := c.Query("tahun")
	status := c.Query("status")
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
	if status != "" {
		params.Set("status", status)
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}

	body, statusCode, err := callInaprocEndpoint("/api/v1/rup/paket-swakelola", params)
	if err != nil {
		log.Println("[INAPROC ERROR] gagal request paket-swakelola:", err)
		utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc (timeout/jaringan)")
		return
	}
	forwardInaprocResponse(c, body, statusCode)
}

type syncPaketSwakelolaRequest struct {
	KodeKLPD string `json:"kode_klpd"`
	Tahun    string `json:"tahun" binding:"required"`
	Status   string `json:"status"`
}

func SyncPaketSwakelola(c *gin.Context) {
	var req syncPaketSwakelolaRequest
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
			logInaprocSync("paket-swakelola", req.KodeKLPD, req.Tahun, req.Status, "failed", totalSynced, "Melebihi batas maksimum halaman", adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Sinkronisasi dihentikan: terlalu banyak halaman")
			return
		}

		params := url.Values{}
		params.Set("kode_klpd", req.KodeKLPD)
		params.Set("tahun", req.Tahun)
		params.Set("limit", "1000")
		if req.Status != "" {
			params.Set("status", req.Status)
		}
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		body, statusCode, err := callInaprocEndpoint("/api/v1/rup/paket-swakelola", params)
		if err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal request paket-swakelola:", err)
			logInaprocSync("paket-swakelola", req.KodeKLPD, req.Tahun, req.Status, "failed", totalSynced, err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc saat sinkronisasi")
			return
		}

		if statusCode != http.StatusOK {
			errMsg := extractInaprocErrorMessage(body, statusCode)
			logInaprocSync("paket-swakelola", req.KodeKLPD, req.Tahun, req.Status, "failed", totalSynced, errMsg, adminUserID, startedAt)
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
			log.Println("[INAPROC SYNC ERROR] gagal parse paket-swakelola:", err, "| body:", string(body))
			logInaprocSync("paket-swakelola", req.KodeKLPD, req.Tahun, req.Status, "failed", totalSynced, "gagal parse: "+err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca respons Inaproc saat sinkronisasi")
			return
		}

		if pageCount == 1 && len(envelope.Data) > 0 {
			log.Printf("[INAPROC SYNC DEBUG] Contoh baris paket-swakelola: %+v", envelope.Data[0])
		}

		for _, row := range envelope.Data {
			if err := upsertPaketSwakelola(row); err != nil {
				log.Println("[INAPROC SYNC WARN] gagal simpan baris paket-swakelola:", err)
				continue
			}
			totalSynced++
		}

		if !envelope.Meta.HasMore || envelope.Meta.Cursor == "" {
			break
		}
		cursor = envelope.Meta.Cursor
	}

	logInaprocSync("paket-swakelola", req.KodeKLPD, req.Tahun, req.Status, "success", totalSynced, "", adminUserID, startedAt)
	utils.SuccessResponse(c, http.StatusOK, "Sinkronisasi berhasil", gin.H{"total_synced": totalSynced, "pages_fetched": pageCount})
}

// generatePaketSwakelolaRowHash: hash seluruh isi baris, konsisten dengan
// pola 3 endpoint sebelumnya (mengantisipasi field yang terlihat unik
// ternyata tidak selalu benar-benar unik di sisi API).
func generatePaketSwakelolaRowHash(row map[string]interface{}) string {
	b, err := json.Marshal(row)
	if err != nil {
		b = []byte(fmt.Sprintf("%v", row))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func upsertPaketSwakelola(row map[string]interface{}) error {
	rowKey := generatePaketSwakelolaRowHash(row)

	kdKLPD := getStr(row, "kd_klpd")
	kdSatker := getStr(row, "kd_satker")
	kdRup := getStr(row, "kd_rup")
	namaKLPD := getStr(row, "nama_klpd")
	namaSatker := getStr(row, "nama_satker")
	namaPaket := getStr(row, "nama_paket")
	tahunAnggaran := getStr(row, "tahun_anggaran")
	status := getStr(row, "status")

	var exists int
	database.DB.QueryRow(`SELECT COUNT(1) FROM inaproc_paket_swakelola WHERE row_key = @p1`, rowKey).Scan(&exists)

	if exists > 0 {
		_, err := database.DB.Exec(`UPDATE inaproc_paket_swakelola SET updated_at=SYSUTCDATETIME() WHERE row_key=@p1`, rowKey)
		return err
	}

	_, err := database.DB.Exec(`
		INSERT INTO inaproc_paket_swakelola (
			row_key, kd_klpd, kd_satker, kd_rup, nama_klpd, nama_satker, nama_paket, tahun_anggaran, status
		) VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9)`,
		rowKey, kdKLPD, kdSatker, kdRup, namaKLPD, namaSatker, namaPaket, tahunAnggaran, status,
	)
	return err
}

func ListLocalPaketSwakelola(c *gin.Context) {
	kodeKLPD := c.DefaultQuery("kode_klpd", kemenkeuKLPDCode)
	tahun := c.Query("tahun")
	status := c.Query("status")
	limit := clampLimit(c.DefaultQuery("limit", "50"))

	query := `SELECT row_key, kd_klpd, kd_satker, kd_rup, nama_klpd, nama_satker, nama_paket,
		tahun_anggaran, status, synced_at
		FROM inaproc_paket_swakelola WHERE kd_klpd = @p1`
	args := []interface{}{kodeKLPD}
	argIdx := 2

	if tahun != "" {
		query += fmt.Sprintf(" AND tahun_anggaran = @p%d", argIdx)
		args = append(args, tahun)
		argIdx++
	}
	if status != "" {
		query += fmt.Sprintf(" AND status = @p%d", argIdx)
		args = append(args, status)
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
// Endpoint 5: Program Master
// ============================================================

func GetProgramMaster(c *gin.Context) {
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

	body, statusCode, err := callInaprocEndpoint("/api/v1/rup/program-master", params)
	if err != nil {
		log.Println("[INAPROC ERROR] gagal request program-master:", err)
		utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc (timeout/jaringan)")
		return
	}
	forwardInaprocResponse(c, body, statusCode)
}

type syncProgramMasterRequest struct {
	KodeKLPD string `json:"kode_klpd"`
	Tahun    string `json:"tahun" binding:"required"`
}

func SyncProgramMaster(c *gin.Context) {
	var req syncProgramMasterRequest
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
			logInaprocSync("program-master", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "Melebihi batas maksimum halaman", adminUserID, startedAt)
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

		body, statusCode, err := callInaprocEndpoint("/api/v1/rup/program-master", params)
		if err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal request program-master:", err)
			logInaprocSync("program-master", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc saat sinkronisasi")
			return
		}

		if statusCode != http.StatusOK {
			errMsg := extractInaprocErrorMessage(body, statusCode)
			logInaprocSync("program-master", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, errMsg, adminUserID, startedAt)
			c.JSON(statusCode, gin.H{"success": false, "message": "Sinkronisasi gagal: " + errMsg, "partial_synced": totalSynced})
			return
		}

		// Catatan: contoh dokumentasi endpoint ini TIDAK menyertakan field
		// "meta" sama sekali (berbeda dari 4 endpoint Inaproc lainnya).
		// Kode ini tetap mencoba membaca meta kalau ada (jaga-jaga API
		// sebenarnya konsisten dan dokumentasi saja yang keliru), tapi kalau
		// tidak ada, sinkronisasi otomatis dianggap selesai setelah 1 halaman.
		var envelope struct {
			Data []map[string]interface{} `json:"data"`
			Meta *struct {
				HasMore bool   `json:"has_more"`
				Cursor  string `json:"cursor"`
			} `json:"meta"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal parse program-master:", err, "| body:", string(body))
			logInaprocSync("program-master", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "gagal parse: "+err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca respons Inaproc saat sinkronisasi")
			return
		}

		if pageCount == 1 && len(envelope.Data) > 0 {
			log.Printf("[INAPROC SYNC DEBUG] Contoh baris program-master: %+v", envelope.Data[0])
		}

		for _, row := range envelope.Data {
			if err := upsertProgramMaster(row); err != nil {
				log.Println("[INAPROC SYNC WARN] gagal simpan baris program-master:", err)
				continue
			}
			totalSynced++
		}

		if envelope.Meta == nil || !envelope.Meta.HasMore || envelope.Meta.Cursor == "" {
			break
		}
		cursor = envelope.Meta.Cursor
	}

	logInaprocSync("program-master", req.KodeKLPD, req.Tahun, "", "success", totalSynced, "", adminUserID, startedAt)
	utils.SuccessResponse(c, http.StatusOK, "Sinkronisasi berhasil", gin.H{"total_synced": totalSynced, "pages_fetched": pageCount})
}

// generateProgramMasterRowHash: hash seluruh isi baris, konsisten dengan
// pola endpoint Inaproc lainnya.
func generateProgramMasterRowHash(row map[string]interface{}) string {
	b, err := json.Marshal(row)
	if err != nil {
		b = []byte(fmt.Sprintf("%v", row))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func upsertProgramMaster(row map[string]interface{}) error {
	rowKey := generateProgramMasterRowHash(row)

	isDeleted := getBool(row, "is_deleted")
	jenisKLPD := getStr(row, "jenis_klpd")
	kdKLPD := getStr(row, "kd_klpd")
	kdProgram := getStr(row, "kd_program")
	kdProgramLokal := getStr(row, "kd_program_lokal")
	kdProgramStr := getStr(row, "kd_program_str")
	kdSatker := getStr(row, "kd_satker")
	namaKLPD := getStr(row, "nama_klpd")
	namaProgram := getStr(row, "nama_program")
	paguProgram := getInt64FromAny(row, "pagu_program")
	tahunAnggaran := getStr(row, "tahun_anggaran")

	var exists int
	database.DB.QueryRow(`SELECT COUNT(1) FROM inaproc_program_master WHERE row_key = @p1`, rowKey).Scan(&exists)

	if exists > 0 {
		_, err := database.DB.Exec(`UPDATE inaproc_program_master SET updated_at=SYSUTCDATETIME() WHERE row_key=@p1`, rowKey)
		return err
	}

	_, err := database.DB.Exec(`
		INSERT INTO inaproc_program_master (
			row_key, is_deleted, jenis_klpd, kd_klpd, kd_program, kd_program_lokal,
			kd_program_str, kd_satker, nama_klpd, nama_program, pagu_program, tahun_anggaran
		) VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9, @p10, @p11, @p12)`,
		rowKey, isDeleted, jenisKLPD, kdKLPD, kdProgram, kdProgramLokal,
		kdProgramStr, kdSatker, namaKLPD, namaProgram, paguProgram, tahunAnggaran,
	)
	return err
}

func ListLocalProgramMaster(c *gin.Context) {
	kodeKLPD := c.DefaultQuery("kode_klpd", kemenkeuKLPDCode)
	tahun := c.Query("tahun")
	limit := clampLimit(c.DefaultQuery("limit", "50"))

	query := `SELECT row_key, kd_klpd, nama_klpd, kd_satker, kd_program, kd_program_str,
		nama_program, pagu_program, tahun_anggaran, is_deleted, synced_at
		FROM inaproc_program_master WHERE kd_klpd = @p1`
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
// Endpoint 6: Paket Swakelola Terumumkan
// ============================================================

func GetPaketSwakelolaTerumumkan(c *gin.Context) {
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

	body, statusCode, err := callInaprocEndpoint("/api/v1/rup/paket-swakelola-terumumkan", params)
	if err != nil {
		log.Println("[INAPROC ERROR] gagal request paket-swakelola-terumumkan:", err)
		utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc (timeout/jaringan)")
		return
	}
	forwardInaprocResponse(c, body, statusCode)
}

type syncPaketSwakelolaTerumumkanRequest struct {
	KodeKLPD string `json:"kode_klpd"`
	Tahun    string `json:"tahun" binding:"required"`
}

func SyncPaketSwakelolaTerumumkan(c *gin.Context) {
	var req syncPaketSwakelolaTerumumkanRequest
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
			logInaprocSync("paket-swakelola-terumumkan", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "Melebihi batas maksimum halaman", adminUserID, startedAt)
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

		body, statusCode, err := callInaprocEndpoint("/api/v1/rup/paket-swakelola-terumumkan", params)
		if err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal request paket-swakelola-terumumkan:", err)
			logInaprocSync("paket-swakelola-terumumkan", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc saat sinkronisasi")
			return
		}

		if statusCode != http.StatusOK {
			errMsg := extractInaprocErrorMessage(body, statusCode)
			logInaprocSync("paket-swakelola-terumumkan", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, errMsg, adminUserID, startedAt)
			c.JSON(statusCode, gin.H{"success": false, "message": "Sinkronisasi gagal: " + errMsg, "partial_synced": totalSynced})
			return
		}

		// Sama seperti Program Master, dokumentasi contoh endpoint ini juga
		// tidak menampilkan "meta" — kode tetap mencoba membacanya kalau ada,
		// fallback berhenti setelah 1 halaman kalau tidak ada.
		var envelope struct {
			Data []map[string]interface{} `json:"data"`
			Meta *struct {
				HasMore bool   `json:"has_more"`
				Cursor  string `json:"cursor"`
			} `json:"meta"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal parse paket-swakelola-terumumkan:", err, "| body:", string(body))
			logInaprocSync("paket-swakelola-terumumkan", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "gagal parse: "+err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca respons Inaproc saat sinkronisasi")
			return
		}

		if pageCount == 1 && len(envelope.Data) > 0 {
			log.Printf("[INAPROC SYNC DEBUG] Contoh baris paket-swakelola-terumumkan: %+v", envelope.Data[0])
		}

		for _, row := range envelope.Data {
			if err := upsertPaketSwakelolaTerumumkan(row); err != nil {
				log.Println("[INAPROC SYNC WARN] gagal simpan baris paket-swakelola-terumumkan:", err)
				continue
			}
			totalSynced++
		}

		if envelope.Meta == nil || !envelope.Meta.HasMore || envelope.Meta.Cursor == "" {
			break
		}
		cursor = envelope.Meta.Cursor
	}

	logInaprocSync("paket-swakelola-terumumkan", req.KodeKLPD, req.Tahun, "", "success", totalSynced, "", adminUserID, startedAt)
	utils.SuccessResponse(c, http.StatusOK, "Sinkronisasi berhasil", gin.H{"total_synced": totalSynced, "pages_fetched": pageCount})
}

func generatePaketSwakelolaTerumumkanRowHash(row map[string]interface{}) string {
	b, err := json.Marshal(row)
	if err != nil {
		b = []byte(fmt.Sprintf("%v", row))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func upsertPaketSwakelolaTerumumkan(row map[string]interface{}) error {
	rowKey := generatePaketSwakelolaTerumumkanRowHash(row)

	jenisKLPD := getStr(row, "jenis_klpd")
	kdKLPD := getStr(row, "kd_klpd")
	kdKLPDPenyelenggara := getStr(row, "kd_klpd_penyelenggara")
	kdRup := getStr(row, "kd_rup")
	kdRupLokal := getStr(row, "kd_rup_lokal")
	kdSatker := getStr(row, "kd_satker")
	kdSatkerStr := getStr(row, "kd_satker_str")
	namaKLPD := getStr(row, "nama_klpd")
	namaKLPDPenyelenggara := getStr(row, "nama_klpd_penyelenggara")
	namaPaket := getStr(row, "nama_paket")
	namaPpk := getStr(row, "nama_ppk")
	namaSatker := getStr(row, "nama_satker")
	namaSatkerPenyelenggara := getStr(row, "nama_satker_penyelenggara")
	nipPpk := getStr(row, "nip_ppk")
	pagu := getInt64FromAny(row, "pagu")
	statusAktifRup := getBoolAny(row, "status_aktif_rup")
	statusDeleteRup := getBoolAny(row, "status_delete_rup")
	statusUmumkanRup := getStr(row, "status_umumkan_rup")
	tahunAnggaran := getStr(row, "tahun_anggaran")
	tglAkhirPelaksanaanKontrak := parseInaprocTime(getStr(row, "tgl_akhir_pelaksanaan_kontrak"))
	tglAwalPelaksanaanKontrak := parseInaprocTime(getStr(row, "tgl_awal_pelaksanaan_kontrak"))
	tglBuatPaket := parseInaprocTime(getStr(row, "tgl_buat_paket"))
	tglPengumumanPaket := parseInaprocTime(getStr(row, "tgl_pengumuman_paket"))
	tipeSwakelola := getStr(row, "tipe_swakelola")
	uraianPekerjaan := getStr(row, "uraian_pekerjaan")
	usernamePpk := getStr(row, "username_ppk")
	volumePekerjaan := getStr(row, "volume_pekerjaan")

	var exists int
	database.DB.QueryRow(`SELECT COUNT(1) FROM inaproc_paket_swakelola_terumumkan WHERE row_key = @p1`, rowKey).Scan(&exists)

	if exists > 0 {
		_, err := database.DB.Exec(`UPDATE inaproc_paket_swakelola_terumumkan SET updated_at=SYSUTCDATETIME() WHERE row_key=@p1`, rowKey)
		return err
	}

	_, err := database.DB.Exec(`
		INSERT INTO inaproc_paket_swakelola_terumumkan (
			row_key, jenis_klpd, kd_klpd, kd_klpd_penyelenggara, kd_rup, kd_rup_lokal,
			kd_satker, kd_satker_str, nama_klpd, nama_klpd_penyelenggara, nama_paket,
			nama_ppk, nama_satker, nama_satker_penyelenggara, nip_ppk, pagu,
			status_aktif_rup, status_delete_rup, status_umumkan_rup, tahun_anggaran,
			tgl_akhir_pelaksanaan_kontrak, tgl_awal_pelaksanaan_kontrak,
			tgl_buat_paket, tgl_pengumuman_paket, tipe_swakelola,
			uraian_pekerjaan, username_ppk, volume_pekerjaan
		) VALUES (
			@p1, @p2, @p3, @p4, @p5, @p6,
			@p7, @p8, @p9, @p10, @p11,
			@p12, @p13, @p14, @p15, @p16,
			@p17, @p18, @p19, @p20,
			@p21, @p22,
			@p23, @p24, @p25,
			@p26, @p27, @p28
		)`,
		rowKey, jenisKLPD, kdKLPD, kdKLPDPenyelenggara, kdRup, kdRupLokal,
		kdSatker, kdSatkerStr, namaKLPD, namaKLPDPenyelenggara, namaPaket,
		namaPpk, namaSatker, namaSatkerPenyelenggara, nipPpk, pagu,
		statusAktifRup, statusDeleteRup, statusUmumkanRup, tahunAnggaran,
		tglAkhirPelaksanaanKontrak, tglAwalPelaksanaanKontrak,
		tglBuatPaket, tglPengumumanPaket, tipeSwakelola,
		uraianPekerjaan, usernamePpk, volumePekerjaan,
	)
	return err
}

func ListLocalPaketSwakelolaTerumumkan(c *gin.Context) {
	kodeKLPD := c.DefaultQuery("kode_klpd", kemenkeuKLPDCode)
	tahun := c.Query("tahun")
	limit := clampLimit(c.DefaultQuery("limit", "50"))

	query := `SELECT row_key, kd_klpd, nama_klpd, kd_satker, nama_satker, kd_rup, nama_paket,
		pagu, nama_ppk, status_umumkan_rup, tgl_awal_pelaksanaan_kontrak, tgl_akhir_pelaksanaan_kontrak,
		tahun_anggaran, synced_at
		FROM inaproc_paket_swakelola_terumumkan WHERE kd_klpd = @p1`
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
// Endpoint 7: Paket Penyedia Terumumkan
// ============================================================

func GetPaketPenyediaTerumumkan(c *gin.Context) {
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

	body, statusCode, err := callInaprocEndpoint("/api/v1/rup/paket-penyedia-terumumkan", params)
	if err != nil {
		log.Println("[INAPROC ERROR] gagal request paket-penyedia-terumumkan:", err)
		utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc (timeout/jaringan)")
		return
	}
	forwardInaprocResponse(c, body, statusCode)
}

type syncPaketPenyediaTerumumkanRequest struct {
	KodeKLPD string `json:"kode_klpd"`
	Tahun    string `json:"tahun" binding:"required"`
}

func SyncPaketPenyediaTerumumkan(c *gin.Context) {
	var req syncPaketPenyediaTerumumkanRequest
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
	const maxPages = 500

	for {
		pageCount++
		if pageCount > maxPages {
			logInaprocSync("paket-penyedia-terumumkan", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "Melebihi batas maksimum halaman", adminUserID, startedAt)
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

		body, statusCode, err := callInaprocEndpoint("/api/v1/rup/paket-penyedia-terumumkan", params)
		if err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal request paket-penyedia-terumumkan:", err)
			logInaprocSync("paket-penyedia-terumumkan", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc saat sinkronisasi")
			return
		}

		if statusCode != http.StatusOK {
			errMsg := extractInaprocErrorMessage(body, statusCode)
			logInaprocSync("paket-penyedia-terumumkan", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, errMsg, adminUserID, startedAt)
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
			log.Println("[INAPROC SYNC ERROR] gagal parse paket-penyedia-terumumkan:", err, "| body:", string(body))
			logInaprocSync("paket-penyedia-terumumkan", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "gagal parse: "+err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca respons Inaproc saat sinkronisasi")
			return
		}

		if pageCount == 1 && len(envelope.Data) > 0 {
			log.Printf("[INAPROC SYNC DEBUG] Contoh baris paket-penyedia-terumumkan: %+v", envelope.Data[0])
		}

		for _, row := range envelope.Data {
			if err := upsertPaketPenyediaTerumumkan(row); err != nil {
				log.Println("[INAPROC SYNC WARN] gagal simpan baris paket-penyedia-terumumkan:", err)
				continue
			}
			totalSynced++
		}

		if !envelope.Meta.HasMore || envelope.Meta.Cursor == "" {
			break
		}
		cursor = envelope.Meta.Cursor
	}

	logInaprocSync("paket-penyedia-terumumkan", req.KodeKLPD, req.Tahun, "", "success", totalSynced, "", adminUserID, startedAt)
	utils.SuccessResponse(c, http.StatusOK, "Sinkronisasi berhasil", gin.H{"total_synced": totalSynced, "pages_fetched": pageCount})
}

func generatePaketPenyediaTerumumkanRowHash(row map[string]interface{}) string {
	b, err := json.Marshal(row)
	if err != nil {
		b = []byte(fmt.Sprintf("%v", row))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func upsertPaketPenyediaTerumumkan(row map[string]interface{}) error {
	rowKey := generatePaketPenyediaTerumumkanRowHash(row)

	alasanDikecualikan := getStr(row, "alasan_dikecualikan")
	alasanNonUkm := getStr(row, "alasan_non_ukm")
	jenisKLPD := getStr(row, "jenis_klpd")
	jenisPengadaan := getStr(row, "jenis_pengadaan")
	kdJenisPengadaan := getStr(row, "kd_jenis_pengadaan")
	kdKLPD := getStr(row, "kd_klpd")
	kdMetodePengadaan := getStr(row, "kd_metode_pengadaan")
	kdRup := getStr(row, "kd_rup")
	kdRupLokal := getStr(row, "kd_rup_lokal")
	kdRupSwakelola := getStr(row, "kd_rup_swakelola")
	kdSatker := getStr(row, "kd_satker")
	kdSatkerStr := getStr(row, "kd_satker_str")
	kodeRupTahunPertama := getStr(row, "kode_rup_tahun_pertama")
	metodePengadaan := getStr(row, "metode_pengadaan")
	namaKLPD := getStr(row, "nama_klpd")
	namaPaket := getStr(row, "nama_paket")
	namaPpk := getStr(row, "nama_ppk")
	namaSatker := getStr(row, "nama_satker")
	nipPpk := getStr(row, "nip_ppk")
	nomorKontrak := getStr(row, "nomor_kontrak")
	pagu := getInt64FromAny(row, "pagu")
	spesifikasiPekerjaan := getStr(row, "spesifikasi_pekerjaan")
	sppAspekEkonomi := getBoolAny(row, "spp_aspek_ekonomi")
	sppAspekLingkungan := getBoolAny(row, "spp_aspek_lingkungan")
	sppAspekSosial := getBoolAny(row, "spp_aspek_sosial")
	statusAktifRup := getBoolAny(row, "status_aktif_rup")
	statusDeleteRup := getBoolAny(row, "status_delete_rup")
	statusDikecualikan := getBoolAny(row, "status_dikecualikan")
	statusKonsolidasi := getStr(row, "status_konsolidasi")
	statusPdn := getStr(row, "status_pdn")
	statusPradipa := getStr(row, "status_pradipa")
	statusUkm := getStr(row, "status_ukm")
	statusUmumkanRup := getStr(row, "status_umumkan_rup")
	tahunAnggaran := getStr(row, "tahun_anggaran")
	tahunPertama := getStr(row, "tahun_pertama")
	tglAkhirKontrak := parseInaprocTime(getStr(row, "tgl_akhir_kontrak"))
	tglAkhirPemanfaatan := parseInaprocTime(getStr(row, "tgl_akhir_pemanfaatan"))
	tglAkhirPemilihan := parseInaprocTime(getStr(row, "tgl_akhir_pemilihan"))
	tglAwalKontrak := parseInaprocTime(getStr(row, "tgl_awal_kontrak"))
	tglAwalPemanfaatan := parseInaprocTime(getStr(row, "tgl_awal_pemanfaatan"))
	tglAwalPemilihan := parseInaprocTime(getStr(row, "tgl_awal_pemilihan"))
	tglBuatPaket := parseInaprocTime(getStr(row, "tgl_buat_paket"))
	tglPengumumanPaket := parseInaprocTime(getStr(row, "tgl_pengumuman_paket"))
	tipePaket := getStr(row, "tipe_paket")
	uraianPekerjaan := getStr(row, "urarian_pekerjaan", "uraian_pekerjaan")
	usernamePpk := getStr(row, "username_ppk")
	volumePekerjaan := getStr(row, "volume_pekerjaan")

	var exists int
	database.DB.QueryRow(`SELECT COUNT(1) FROM inaproc_paket_penyedia_terumumkan WHERE row_key = @p1`, rowKey).Scan(&exists)

	if exists > 0 {
		_, err := database.DB.Exec(`UPDATE inaproc_paket_penyedia_terumumkan SET updated_at=SYSUTCDATETIME() WHERE row_key=@p1`, rowKey)
		return err
	}

	_, err := database.DB.Exec(`
		INSERT INTO inaproc_paket_penyedia_terumumkan (
			row_key, alasan_dikecualikan, alasan_non_ukm, jenis_klpd, jenis_pengadaan,
			kd_jenis_pengadaan, kd_klpd, kd_metode_pengadaan, kd_rup, kd_rup_lokal,
			kd_rup_swakelola, kd_satker, kd_satker_str, kode_rup_tahun_pertama, metode_pengadaan,
			nama_klpd, nama_paket, nama_ppk, nama_satker, nip_ppk,
			nomor_kontrak, pagu, spesifikasi_pekerjaan, spp_aspek_ekonomi, spp_aspek_lingkungan,
			spp_aspek_sosial, status_aktif_rup, status_delete_rup, status_dikecualikan, status_konsolidasi,
			status_pdn, status_pradipa, status_ukm, status_umumkan_rup, tahun_anggaran,
			tahun_pertama, tgl_akhir_kontrak, tgl_akhir_pemanfaatan, tgl_akhir_pemilihan, tgl_awal_kontrak,
			tgl_awal_pemanfaatan, tgl_awal_pemilihan, tgl_buat_paket, tgl_pengumuman_paket, tipe_paket,
			uraian_pekerjaan, username_ppk, volume_pekerjaan
		) VALUES (
			@p1, @p2, @p3, @p4, @p5,
			@p6, @p7, @p8, @p9, @p10,
			@p11, @p12, @p13, @p14, @p15,
			@p16, @p17, @p18, @p19, @p20,
			@p21, @p22, @p23, @p24, @p25,
			@p26, @p27, @p28, @p29, @p30,
			@p31, @p32, @p33, @p34, @p35,
			@p36, @p37, @p38, @p39, @p40,
			@p41, @p42, @p43, @p44, @p45,
			@p46, @p47, @p48
		)`,
		rowKey, alasanDikecualikan, alasanNonUkm, jenisKLPD, jenisPengadaan,
		kdJenisPengadaan, kdKLPD, kdMetodePengadaan, kdRup, kdRupLokal,
		kdRupSwakelola, kdSatker, kdSatkerStr, kodeRupTahunPertama, metodePengadaan,
		namaKLPD, namaPaket, namaPpk, namaSatker, nipPpk,
		nomorKontrak, pagu, spesifikasiPekerjaan, sppAspekEkonomi, sppAspekLingkungan,
		sppAspekSosial, statusAktifRup, statusDeleteRup, statusDikecualikan, statusKonsolidasi,
		statusPdn, statusPradipa, statusUkm, statusUmumkanRup, tahunAnggaran,
		tahunPertama, tglAkhirKontrak, tglAkhirPemanfaatan, tglAkhirPemilihan, tglAwalKontrak,
		tglAwalPemanfaatan, tglAwalPemilihan, tglBuatPaket, tglPengumumanPaket, tipePaket,
		uraianPekerjaan, usernamePpk, volumePekerjaan,
	)
	return err
}

func ListLocalPaketPenyediaTerumumkan(c *gin.Context) {
	kodeKLPD := c.DefaultQuery("kode_klpd", kemenkeuKLPDCode)
	tahun := c.Query("tahun")
	limit := clampLimit(c.DefaultQuery("limit", "50"))

	query := `SELECT row_key, kd_klpd, nama_klpd, kd_satker, nama_satker, kd_rup, nama_paket,
		pagu, metode_pengadaan, status_umumkan_rup, nama_ppk,
		tgl_awal_pemilihan, tgl_akhir_pemilihan, tahun_anggaran, synced_at
		FROM inaproc_paket_penyedia_terumumkan WHERE kd_klpd = @p1`
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
