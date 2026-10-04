package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
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
	"pasti-v3-backend/utils"
)

// Implementasi bersama untuk endpoint Inaproc (Tender dan E-Katalog archive) yang respons datarnya (satu objek per baris, tanpa
// array bersarang) dan berhalaman dengan cursor: non-tender-selesai, pencatatan-non-tender, ekatalog-archive/penyedia-detail, dst.
// Tiap endpoint cukup mendeklarasikan nama jalur API, nama tabel, dan daftar field menurut tipenya; proxy ke Inaproc, sinkronisasi
// ke database, dan daftar lokal ditangani di sini.
//
// Nama kolom di tabel sama persis dengan nama field API. Daftar field di deklarasi HARUS sinkron dengan migrasi tabelnya; tes
// (TestEndpointDatarKolomSamaDenganMigrasi) menjaganya. Field API yang belum dipetakan ke kolom disimpan di extra_json supaya
// tidak hilang diam-diam.
type endpointDatar struct {
	Nama string // jalur setelah /api/v1/<awalan>/
	// Awalan: kelompok API setelah /api/v1/. Kosong = "tender". Untuk kelompok lain (mis. "ekatalog-archive"), nama yang dicatat
	// di inaproc_sync_log menjadi "<awalan>-<nama>".
	Awalan string
	Tabel  string

	// Saring: cara endpoint ini disaring. Nol = bawaan Tender: kode_klpd (bawaan "K10") + tahun.
	Saring saringan

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

// saringan menentukan parameter yang wajib pada GET dan sinkronisasi serta kolom penyaring di database. Tiga bentuk:
//   - nol (bawaan): kode_klpd + tahun; kolom kd_klpd dan tahun_anggaran.
//   - TanpaTahun: kode_klpd saja (data rujukan per KLPD); kolom kd_klpd.
//   - Param terisi: satu kode tunggal (pencarian rujukan per kode, mis. kode_penyedia); kolom `Kolom`. Pada sinkronisasi kode
//     dikirim di badan JSON sebagai "kode".
type saringan struct {
	Param      string // nama parameter kode tunggal di API Inaproc (mis. "kode_penyedia"); kosong = berdasarkan kode_klpd
	Kolom      string // kolom tabel yang menyimpan kode itu (mis. "kd_penyedia"); wajib bila Param terisi
	TanpaTahun bool   // hanya bila Param kosong: tidak ada parameter tahun
}

const (
	awalanTender = "tender"
	// Kode tunggal dikirim sebagai parameter query dan disimpan di kolom bertipe teks pendek; batas ini menolak masukan ngawur
	// sebelum menghubungi Inaproc.
	panjangKodeMaks = 100
)

func (e *endpointDatar) awalan() string {
	if e.Awalan == "" {
		return awalanTender
	}
	return e.Awalan
}

func (e *endpointDatar) jalurAPI() string { return "/api/v1/" + e.awalan() + "/" + e.Nama }

// namaLog: nama yang dicatat di inaproc_sync_log. Hanya tanda hubung, karena kartu aktivitas di dashboard memecah nama pada "-".
func (e *endpointDatar) namaLog() string {
	if e.awalan() == awalanTender {
		return e.Nama
	}
	return e.awalan() + "-" + e.Nama
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
	case e.Saring.Param != "":
		kode := strings.TrimSpace(c.Query(e.Saring.Param))
		if kode == "" {
			utils.ErrorResponse(c, http.StatusBadRequest, "Parameter '"+e.Saring.Param+"' wajib diisi")
			return
		}
		if len(kode) > panjangKodeMaks {
			utils.ErrorResponse(c, http.StatusBadRequest, "Parameter '"+e.Saring.Param+"' terlalu panjang")
			return
		}
		params.Set(e.Saring.Param, kode)
	case e.Saring.TanpaTahun:
		params.Set("kode_klpd", kodeKLPD)
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

	body, statusCode, err := callInaprocEndpoint(e.jalurAPI(), params)
	if err != nil {
		log.Println("[INAPROC ERROR] gagal request "+e.namaLog()+":", err)
		utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc (timeout/jaringan)")
		return
	}
	forwardInaprocResponse(c, body, statusCode)
}

// syncDatarRequest: tahun wajib pada saringan bawaan, kode wajib pada saringan kode tunggal; sisanya diabaikan.
type syncDatarRequest struct {
	KodeKLPD string `json:"kode_klpd"`
	Tahun    string `json:"tahun"`
	Kode     string `json:"kode"`
}

// paramDasar: parameter penyaring ke Inaproc (di luar limit dan cursor) menurut bentuk saringan endpoint ini.
func (e *endpointDatar) paramDasar(req syncDatarRequest) url.Values {
	p := url.Values{}
	switch {
	case e.Saring.Param != "":
		p.Set(e.Saring.Param, req.Kode)
	case e.Saring.TanpaTahun:
		p.Set("kode_klpd", req.KodeKLPD)
	default:
		p.Set("kode_klpd", req.KodeKLPD)
		p.Set("tahun", req.Tahun)
	}
	return p
}

// hapusLama: DELETE untuk data yang akan diganti sinkronisasi, menurut bentuk saringan endpoint ini.
func (e *endpointDatar) hapusLama(req syncDatarRequest) (string, []interface{}) {
	switch {
	case e.Saring.Param != "":
		return "DELETE FROM " + e.Tabel + " WHERE " + e.Saring.Kolom + " = @p1", []interface{}{req.Kode}
	case e.Saring.TanpaTahun:
		return "DELETE FROM " + e.Tabel + " WHERE kd_klpd = @p1", []interface{}{req.KodeKLPD}
	default:
		return "DELETE FROM " + e.Tabel + " WHERE kd_klpd = @p1 AND tahun_anggaran = @p2", []interface{}{req.KodeKLPD, req.Tahun}
	}
}

// potong memotong teks ke n karakter (bukan byte), untuk kolom log yang sempit.
func potong(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// Sync (admin) menarik seluruh halaman dari Inaproc ke tabel lokal. Data lama untuk penyaring yang sama (klpd dan tahun, klpd saja,
// atau satu kode) dihapus lebih dulu supaya tidak menumpuk. Baris yang gagal disimpan dilewati, dihitung, dan dilaporkan (tidak
// menggagalkan seluruh sinkronisasi).
func (e *endpointDatar) Sync(c *gin.Context) {
	var req syncDatarRequest
	// Badan kosong dibolehkan (saringan klpd saja tidak butuh isian); JSON rusak ditolak.
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		utils.ErrorResponse(c, http.StatusBadRequest, "Permintaan tidak valid")
		return
	}
	req.Tahun = strings.TrimSpace(req.Tahun)
	req.Kode = strings.TrimSpace(req.Kode)
	if req.KodeKLPD == "" {
		req.KodeKLPD = kemenkeuKLPDCodeTender
	}
	switch {
	case e.Saring.Param != "" && req.Kode == "":
		utils.ErrorResponse(c, http.StatusBadRequest, "kode wajib diisi")
		return
	case e.Saring.Param != "" && len(req.Kode) > panjangKodeMaks:
		utils.ErrorResponse(c, http.StatusBadRequest, "kode terlalu panjang")
		return
	case e.Saring.Param == "" && !e.Saring.TanpaTahun && req.Tahun == "":
		utils.ErrorResponse(c, http.StatusBadRequest, "tahun wajib diisi")
		return
	}

	// Yang dicatat di inaproc_sync_log: klpd dan tahun, atau (untuk saringan kode) kode itu di kolom jenis_paket.
	nama := e.namaLog()
	logKLPD, logTahun, logJenis := req.KodeKLPD, req.Tahun, ""
	if e.Saring.Param != "" {
		logKLPD, logTahun, logJenis = "", "", potong(req.Kode, 50)
	}

	adminUserID := c.GetString("user_id")
	startedAt := time.Now()

	hapusSQL, hapusArgs := e.hapusLama(req)
	if _, err := database.DB.Exec(hapusSQL, hapusArgs...); err != nil {
		log.Println("[INAPROC SYNC WARN] gagal hapus data lama "+nama+":", err)
	}

	totalSynced, totalFailed := 0, 0
	cursor := ""
	pageCount := 0
	const maxPages = 200

	for {
		pageCount++
		if pageCount > maxPages {
			logInaprocSync(nama, logKLPD, logTahun, logJenis, "failed", totalSynced, "Melebihi batas maksimum halaman", adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Sinkronisasi dihentikan: terlalu banyak halaman")
			return
		}

		params := e.paramDasar(req)
		params.Set("limit", "1000")
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		body, statusCode, err := callInaprocEndpoint(e.jalurAPI(), params)
		if err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal request "+nama+":", err)
			logInaprocSync(nama, logKLPD, logTahun, logJenis, "failed", totalSynced, err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc saat sinkronisasi")
			return
		}

		if statusCode != http.StatusOK {
			errMsg := extractInaprocErrorMessage(body, statusCode)
			logInaprocSync(nama, logKLPD, logTahun, logJenis, "failed", totalSynced, errMsg, adminUserID, startedAt)
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
			log.Println("[INAPROC SYNC ERROR] gagal parse "+nama+":", err, "| body:", string(body))
			logInaprocSync(nama, logKLPD, logTahun, logJenis, "failed", totalSynced, "gagal parse: "+err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca respons Inaproc saat sinkronisasi")
			return
		}

		if pageCount == 1 && len(envelope.Data) > 0 {
			log.Printf("[INAPROC SYNC DEBUG] Contoh baris %s: %+v", nama, envelope.Data[0])
		}

		for _, row := range envelope.Data {
			if err := e.insert(row, req.KodeKLPD); err != nil {
				log.Println("[INAPROC SYNC WARN] gagal simpan baris "+nama+":", err)
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
	logInaprocSync(nama, logKLPD, logTahun, logJenis, "success", totalSynced, catatan, adminUserID, startedAt)
	utils.SuccessResponse(c, http.StatusOK, "Sinkronisasi berhasil", gin.H{"total_synced": totalSynced, "total_failed": totalFailed, "pages_fetched": pageCount})
}

// ListLocal membaca data yang sudah disinkronkan dari database PASTI. Penyaringnya mengikuti bentuk saringan endpoint:
// kd_klpd (+ tahun_anggaran bila ada) atau, untuk saringan kode tunggal, kolom kodenya (tanpa kode = semua baris).
func (e *endpointDatar) ListLocal(c *gin.Context) {
	limit := clampLimit(c.DefaultQuery("limit", "50"))

	query := "SELECT " + e.KolomDaftar + " FROM " + e.Tabel
	var args []interface{}

	if e.Saring.Param != "" {
		if kode := strings.TrimSpace(c.Query(e.Saring.Param)); kode != "" {
			query += " WHERE " + e.Saring.Kolom + " = @p1"
			args = append(args, kode)
		}
	} else {
		query += " WHERE kd_klpd = @p1"
		args = append(args, c.DefaultQuery("kode_klpd", kemenkeuKLPDCodeTender))
		if tahun := c.Query("tahun"); tahun != "" && !e.Saring.TanpaTahun {
			query += " AND tahun_anggaran = @p2"
			args = append(args, tahun)
		}
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
