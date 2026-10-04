package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/utils"
)

// Implementasi bersama untuk endpoint Tender Inaproc yang respons datarnya (satu objek per baris, tanpa array bersarang) dan
// berhalaman dengan cursor: non-tender-selesai, pencatatan-non-tender, pencatatan-non-tender-realisasi, dan seterusnya.
// Tiap endpoint cukup mendeklarasikan nama jalur API, nama tabel, dan daftar field menurut tipenya; proxy ke Inaproc, sinkronisasi
// ke database, dan daftar lokal ditangani di sini.
//
// Nama kolom di tabel sama persis dengan nama field API. Daftar field di deklarasi HARUS sinkron dengan migrasi tabelnya; tes
// (TestEndpointDatarKolomSamaDenganMigrasi) menjaganya. Field API yang belum dipetakan ke kolom disimpan di extra_json supaya
// tidak hilang diam-diam.
type endpointDatar struct {
	Nama  string // jalur setelah /api/v1/tender/ (juga dicatat di inaproc_sync_log)
	Tabel string

	// Disimpan sebagai NVARCHAR; nilai kosong/null menjadi NULL. Objek/larik disimpan sebagai teks JSON.
	Teks []string
	// DECIMAL(24,2): nilai rupiah bisa berpecahan, jadi tidak dipotong ke BIGINT.
	Desimal []string
	Bulat   []string
	Tanggal []string

	// Kolom yang dipilih untuk daftar lokal (GET .../local); harus memuat synced_at.
	KolomDaftar string

	// MenerimaKdTender: GET boleh memakai kd_tender saja (tanpa tahun dan kode_klpd), seperti tender/pengumuman yang memang
	// punya dua skenario: (tahun + kode_klpd) atau kd_tender. Bila kd_tender diisi, hanya kd_tender yang diteruskan ke Inaproc.
	// Sinkronisasi tetap per tahun dan kode_klpd.
	MenerimaKdTender bool

	insertSQL string
}

func newEndpointDatar(e endpointDatar) *endpointDatar {
	e.insertSQL = e.buildInsertSQL()
	return &e
}

// KnownFields: semua field yang sudah punya kolom sendiri, menurut urutan kolom di INSERT (setelah row_key).
func (e *endpointDatar) KnownFields() []string {
	var all []string
	all = append(all, e.Teks...)
	all = append(all, e.Desimal...)
	all = append(all, e.Bulat...)
	all = append(all, e.Tanggal...)
	return all
}

// Nama kolom berasal dari deklarasi di kode, bukan dari input pengguna.
func (e *endpointDatar) buildInsertSQL() string {
	cols := []string{"row_key"}
	cols = append(cols, e.KnownFields()...)
	cols = append(cols, "extra_json")

	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = "@p" + strconv.Itoa(i+1)
	}
	return "INSERT INTO " + e.Tabel + " (" + strings.Join(cols, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"
}

// teksKolom membaca field sebagai teks: angka/boolean/teks apa adanya, objek dan larik sebagai JSON, null sebagai "".
func teksKolom(row map[string]interface{}, key string) string {
	if s := getStr(row, key); s != "" {
		return s
	}
	switch v := row[key].(type) {
	case map[string]interface{}, []interface{}:
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
	}
	return ""
}

// Args menyusun argumen INSERT; urutannya sama dengan kolom di insertSQL. kd_klpd yang kosong di respons diisi kode KLPD yang
// diminta, supaya hapus-sebelum-tarik (WHERE kd_klpd = ...) dan daftar lokal tetap menemukan barisnya. row_key dihitung dari
// baris asli dari API.
func (e *endpointDatar) Args(row map[string]interface{}, kodeKLPD string) []interface{} {
	args := []interface{}{generateInaprocRowHash(row)}
	for _, f := range e.Teks {
		v := teksKolom(row, f)
		if f == "kd_klpd" && v == "" {
			v = kodeKLPD
		}
		args = append(args, nullIfEmpty(v))
	}
	for _, f := range e.Desimal {
		args = append(args, getDecimalString(row, f))
	}
	for _, f := range e.Bulat {
		args = append(args, getInt64FromAny(row, f))
	}
	for _, f := range e.Tanggal {
		args = append(args, parseInaprocTime(getStr(row, f)))
	}
	args = append(args, extraFieldsColumn(row, e.KnownFields()))
	return args
}

func (e *endpointDatar) insert(row map[string]interface{}, kodeKLPD string) error {
	_, err := database.DB.Exec(e.insertSQL, e.Args(row, kodeKLPD)...)
	return err
}

// Get meneruskan permintaan ke Inaproc (data langsung, tanpa database).
func (e *endpointDatar) Get(c *gin.Context) {
	if config.Cfg.InaprocToken == "" {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Integrasi Inaproc belum dikonfigurasi (token kosong)")
		return
	}

	kodeKLPD := c.DefaultQuery("kode_klpd", kemenkeuKLPDCodeTender)
	tahun := c.Query("tahun")
	cursor := c.Query("cursor")
	kdTender := ""
	if e.MenerimaKdTender {
		kdTender = strings.TrimSpace(c.Query("kd_tender"))
	}

	params := url.Values{}
	switch {
	case kdTender != "":
		if _, err := strconv.ParseInt(kdTender, 10, 64); err != nil {
			utils.ErrorResponse(c, http.StatusBadRequest, "Parameter 'kd_tender' harus berupa angka")
			return
		}
		params.Set("kd_tender", kdTender)
	case tahun == "" && e.MenerimaKdTender:
		utils.ErrorResponse(c, http.StatusBadRequest, "Isi 'tahun' atau 'kd_tender'")
		return
	case tahun == "":
		utils.ErrorResponse(c, http.StatusBadRequest, "Parameter 'tahun' wajib diisi")
		return
	default:
		params.Set("kode_klpd", kodeKLPD)
		params.Set("tahun", tahun)
	}
	params.Set("limit", strconv.Itoa(clampLimit(c.DefaultQuery("limit", "50"))))
	if cursor != "" {
		params.Set("cursor", cursor)
	}

	body, statusCode, err := callInaprocEndpoint("/api/v1/tender/"+e.Nama, params)
	if err != nil {
		log.Println("[INAPROC ERROR] gagal request "+e.Nama+":", err)
		utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc (timeout/jaringan)")
		return
	}
	forwardInaprocResponse(c, body, statusCode)
}

type syncDatarRequest struct {
	KodeKLPD string `json:"kode_klpd"`
	Tahun    string `json:"tahun" binding:"required"`
}

// Sync (admin) menarik seluruh halaman dari Inaproc ke tabel lokal. Data lama untuk klpd dan tahun yang sama dihapus lebih dulu
// supaya tidak menumpuk. Baris yang gagal disimpan dilewati, dihitung, dan dilaporkan (tidak menggagalkan seluruh sinkronisasi).
func (e *endpointDatar) Sync(c *gin.Context) {
	var req syncDatarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "tahun wajib diisi")
		return
	}
	if req.KodeKLPD == "" {
		req.KodeKLPD = kemenkeuKLPDCodeTender
	}

	adminUserID := c.GetString("user_id")
	startedAt := time.Now()

	if _, err := database.DB.Exec(
		"DELETE FROM "+e.Tabel+" WHERE kd_klpd = @p1 AND tahun_anggaran = @p2",
		req.KodeKLPD, req.Tahun,
	); err != nil {
		log.Println("[INAPROC SYNC WARN] gagal hapus data lama "+e.Nama+":", err)
	}

	totalSynced, totalFailed := 0, 0
	cursor := ""
	pageCount := 0
	const maxPages = 200

	for {
		pageCount++
		if pageCount > maxPages {
			logInaprocSync(e.Nama, req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "Melebihi batas maksimum halaman", adminUserID, startedAt)
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

		body, statusCode, err := callInaprocEndpoint("/api/v1/tender/"+e.Nama, params)
		if err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal request "+e.Nama+":", err)
			logInaprocSync(e.Nama, req.KodeKLPD, req.Tahun, "", "failed", totalSynced, err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc saat sinkronisasi")
			return
		}

		if statusCode != http.StatusOK {
			errMsg := extractInaprocErrorMessage(body, statusCode)
			logInaprocSync(e.Nama, req.KodeKLPD, req.Tahun, "", "failed", totalSynced, errMsg, adminUserID, startedAt)
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
		// UseNumber: angka dibaca sebagai teks aslinya, bukan float64. Identitas seperti NIP 18 digit (nip_ppk dikirim sebagai
		// angka di pencatatan-swakelola-realisasi) melebihi 2^53 dan akan berubah digit belakangnya bila lewat float64.
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber()
		if err := dec.Decode(&envelope); err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal parse "+e.Nama+":", err, "| body:", string(body))
			logInaprocSync(e.Nama, req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "gagal parse: "+err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca respons Inaproc saat sinkronisasi")
			return
		}

		if pageCount == 1 && len(envelope.Data) > 0 {
			log.Printf("[INAPROC SYNC DEBUG] Contoh baris %s: %+v", e.Nama, envelope.Data[0])
		}

		for _, row := range envelope.Data {
			if err := e.insert(row, req.KodeKLPD); err != nil {
				log.Println("[INAPROC SYNC WARN] gagal simpan baris "+e.Nama+":", err)
				totalFailed++
				continue
			}
			totalSynced++
		}

		if !envelope.Meta.HasMore || envelope.Meta.Cursor == "" {
			break
		}
		cursor = envelope.Meta.Cursor
	}

	catatan := ""
	if totalFailed > 0 {
		catatan = fmt.Sprintf("%d baris gagal disimpan", totalFailed)
	}
	logInaprocSync(e.Nama, req.KodeKLPD, req.Tahun, "", "success", totalSynced, catatan, adminUserID, startedAt)
	utils.SuccessResponse(c, http.StatusOK, "Sinkronisasi berhasil", gin.H{"total_synced": totalSynced, "total_failed": totalFailed, "pages_fetched": pageCount})
}

// ListLocal membaca data yang sudah disinkronkan dari database PASTI.
func (e *endpointDatar) ListLocal(c *gin.Context) {
	kodeKLPD := c.DefaultQuery("kode_klpd", kemenkeuKLPDCodeTender)
	tahun := c.Query("tahun")
	limit := clampLimit(c.DefaultQuery("limit", "50"))

	query := "SELECT " + e.KolomDaftar + " FROM " + e.Tabel + " WHERE kd_klpd = @p1"
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
