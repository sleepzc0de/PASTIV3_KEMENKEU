package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

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

	totalSynced, totalFailed := 0, 0
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

	logInaprocSync("jadwal-tahapan-non-tender", req.KodeKLPD, req.Tahun, "", "success", totalSynced, catatanHasil(HasilSinkron{TotalGagal: totalFailed}), adminUserID, startedAt)
	utils.SuccessResponse(c, http.StatusOK, "Sinkronisasi berhasil", gin.H{"total_synced": totalSynced, "total_failed": totalFailed, "pages_fetched": pageCount})
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

// ============================================================
// TENDER Endpoint 2: Jadwal Tahapan Tender
// ============================================================

type syncJadwalTahapanTenderRequest struct {
	KodeKLPD string `json:"kode_klpd"`
	Tahun    string `json:"tahun" binding:"required"`
}

func SyncJadwalTahapanTender(c *gin.Context) {
	var req syncJadwalTahapanTenderRequest
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
		`DELETE FROM inaproc_jadwal_tahapan_tender WHERE kd_klpd = @p1 AND tahun_anggaran = @p2`,
		req.KodeKLPD, req.Tahun,
	); err != nil {
		log.Println("[INAPROC SYNC WARN] gagal hapus data lama jadwal-tahapan-tender:", err)
	}

	totalSynced, totalFailed := 0, 0
	cursor := ""
	pageCount := 0
	const maxPages = 200

	for {
		pageCount++
		if pageCount > maxPages {
			logInaprocSync("jadwal-tahapan-tender", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "Melebihi batas maksimum halaman", adminUserID, startedAt)
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

		body, statusCode, err := callInaprocEndpoint("/api/v1/tender/jadwal-tahapan-tender", params)
		if err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal request jadwal-tahapan-tender:", err)
			logInaprocSync("jadwal-tahapan-tender", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc saat sinkronisasi")
			return
		}

		if statusCode != http.StatusOK {
			errMsg := extractInaprocErrorMessage(body, statusCode)
			logInaprocSync("jadwal-tahapan-tender", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, errMsg, adminUserID, startedAt)
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
			log.Println("[INAPROC SYNC ERROR] gagal parse jadwal-tahapan-tender:", err, "| body:", string(body))
			logInaprocSync("jadwal-tahapan-tender", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "gagal parse: "+err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca respons Inaproc saat sinkronisasi")
			return
		}

		if pageCount == 1 && len(envelope.Data) > 0 {
			log.Printf("[INAPROC SYNC DEBUG] Contoh baris jadwal-tahapan-tender: %+v", envelope.Data[0])
		}

		for _, row := range envelope.Data {
			if err := insertJadwalTahapanTender(row); err != nil {
				log.Println("[INAPROC SYNC WARN] gagal simpan baris jadwal-tahapan-tender:", err)
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

	logInaprocSync("jadwal-tahapan-tender", req.KodeKLPD, req.Tahun, "", "success", totalSynced, catatanHasil(HasilSinkron{TotalGagal: totalFailed}), adminUserID, startedAt)
	utils.SuccessResponse(c, http.StatusOK, "Sinkronisasi berhasil", gin.H{"total_synced": totalSynced, "total_failed": totalFailed, "pages_fetched": pageCount})
}

func insertJadwalTahapanTender(row map[string]interface{}) error {
	rowKey := generateInaprocRowHash(row)

	kdAkt := getStr(row, "kd_akt")
	kdKLPD := getStr(row, "kd_klpd")
	kdLpse := getStr(row, "kd_lpse")
	kdSatker := getStr(row, "kd_satker")
	kdSatkerStr := getStr(row, "kd_satker_str")
	kdTahapan := getStr(row, "kd_tahapan")
	kdTender := getStr(row, "kd_tender")
	namaAkt := getStr(row, "nama_akt")
	namaTahapan := getStr(row, "nama_tahapan")
	tahunAnggaran := getStr(row, "tahun_anggaran")
	tglAkhir := parseInaprocTime(getStr(row, "tgl_akhir"))
	tglAwal := parseInaprocTime(getStr(row, "tgl_awal"))

	_, err := database.DB.Exec(`
		INSERT INTO inaproc_jadwal_tahapan_tender (
			row_key, kd_akt, kd_klpd, kd_lpse, kd_satker, kd_satker_str,
			kd_tahapan, kd_tender, nama_akt, nama_tahapan, tahun_anggaran, tgl_akhir, tgl_awal
		) VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9, @p10, @p11, @p12, @p13)`,
		rowKey, kdAkt, kdKLPD, kdLpse, kdSatker, kdSatkerStr,
		kdTahapan, kdTender, namaAkt, namaTahapan, tahunAnggaran, tglAkhir, tglAwal,
	)
	return err
}

// ============================================================
// TENDER Endpoint 3: Non Tender E-Kontrak
// ============================================================
//
// Setiap baris memuat tiga array bersarang (bapbast_history_json,
// spmkspp_history_json, penilaian_kinerja_penyedia) yang dikembalikan API
// SELALU sebagai array ([] kalau kosong). Ketiganya disimpan apa adanya sebagai
// teks JSON di kolom NVARCHAR(MAX), bukan dinormalisasi ke tabel terpisah.

// Field yang sudah dipetakan ke kolom tersendiri. Field lain dari API masuk ke
// kolom extra_json supaya tidak hilang diam-diam kalau API menambah field baru.
var nonTenderEkontrakKnownFields = []string{
	"kd_klpd", "kd_tender", "tahun_anggaran", "nama_paket", "alamat_satker",
	"bapbast_history_json", "spmkspp_history_json", "penilaian_kinerja_penyedia",
}

type syncNonTenderEkontrakRequest struct {
	KodeKLPD string `json:"kode_klpd"`
	Tahun    string `json:"tahun" binding:"required"`
}

func SyncNonTenderEkontrak(c *gin.Context) {
	var req syncNonTenderEkontrakRequest
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
		`DELETE FROM inaproc_non_tender_ekontrak WHERE kd_klpd = @p1 AND tahun_anggaran = @p2`,
		req.KodeKLPD, req.Tahun,
	); err != nil {
		log.Println("[INAPROC SYNC WARN] gagal hapus data lama non-tender-ekontrak:", err)
	}

	totalSynced, totalFailed := 0, 0
	cursor := ""
	pageCount := 0
	const maxPages = 200

	for {
		pageCount++
		if pageCount > maxPages {
			logInaprocSync("non-tender-ekontrak", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "Melebihi batas maksimum halaman", adminUserID, startedAt)
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

		body, statusCode, err := callInaprocEndpoint("/api/v1/tender/non-tender-ekontrak", params)
		if err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal request non-tender-ekontrak:", err)
			logInaprocSync("non-tender-ekontrak", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc saat sinkronisasi")
			return
		}

		if statusCode != http.StatusOK {
			errMsg := extractInaprocErrorMessage(body, statusCode)
			logInaprocSync("non-tender-ekontrak", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, errMsg, adminUserID, startedAt)
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
			log.Println("[INAPROC SYNC ERROR] gagal parse non-tender-ekontrak:", err, "| body:", string(body))
			logInaprocSync("non-tender-ekontrak", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "gagal parse: "+err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca respons Inaproc saat sinkronisasi")
			return
		}

		if pageCount == 1 && len(envelope.Data) > 0 {
			log.Printf("[INAPROC SYNC DEBUG] Contoh baris non-tender-ekontrak: %+v", envelope.Data[0])
		}

		for _, row := range envelope.Data {
			if err := insertNonTenderEkontrak(row); err != nil {
				log.Println("[INAPROC SYNC WARN] gagal simpan baris non-tender-ekontrak:", err)
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

	logInaprocSync("non-tender-ekontrak", req.KodeKLPD, req.Tahun, "", "success", totalSynced, catatanHasil(HasilSinkron{TotalGagal: totalFailed}), adminUserID, startedAt)
	utils.SuccessResponse(c, http.StatusOK, "Sinkronisasi berhasil", gin.H{"total_synced": totalSynced, "total_failed": totalFailed, "pages_fetched": pageCount})
}

// jsonArrayColumn mengubah field array dari respons Inaproc menjadi teks JSON
// untuk kolom NVARCHAR(MAX). Field yang hilang/null disimpan sebagai "[]"
// supaya konsisten dengan kontrak API ("selalu array").
func jsonArrayColumn(row map[string]interface{}, key string) string {
	v, ok := row[key]
	if !ok || v == nil {
		return "[]"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// extraFieldsColumn mengumpulkan field di luar knownFields menjadi satu objek
// JSON. Mengembalikan nil (NULL di database) kalau tidak ada field tambahan.
func extraFieldsColumn(row map[string]interface{}, knownFields []string) interface{} {
	extra := make(map[string]interface{}, len(row))
	for k, v := range row {
		extra[k] = v
	}
	for _, k := range knownFields {
		delete(extra, k)
	}
	if len(extra) == 0 {
		return nil
	}
	b, err := json.Marshal(extra)
	if err != nil {
		return nil
	}
	return string(b)
}

func insertNonTenderEkontrak(row map[string]interface{}) error {
	rowKey := generateInaprocRowHash(row)

	_, err := database.DB.Exec(`
		INSERT INTO inaproc_non_tender_ekontrak (
			row_key, kd_klpd, kd_tender, tahun_anggaran, nama_paket, alamat_satker,
			bapbast_history_json, spmkspp_history_json, penilaian_kinerja_penyedia, extra_json
		) VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9, @p10)`,
		rowKey,
		getStr(row, "kd_klpd"),
		getStr(row, "kd_tender"),
		getStr(row, "tahun_anggaran"),
		getStr(row, "nama_paket"),
		getStr(row, "alamat_satker"),
		jsonArrayColumn(row, "bapbast_history_json"),
		jsonArrayColumn(row, "spmkspp_history_json"),
		jsonArrayColumn(row, "penilaian_kinerja_penyedia"),
		extraFieldsColumn(row, nonTenderEkontrakKnownFields),
	)
	return err
}

// ============================================================
// TENDER Endpoint 4: Non Tender E-Kontrak Kontrak
// ============================================================
//
// Respons berbentuk datar (±47 field). Nama kolom di tabel
// inaproc_non_tender_ekontrak_kontrak sama persis dengan nama field API, jadi
// INSERT dibangun dari daftar nama di bawah (bukan satu variabel per field).
// Daftar ini HARUS sinkron dengan migrasi 017.
var (
	// Disimpan sebagai NVARCHAR; nilai kosong/null disimpan sebagai NULL.
	nonTenderEkontrakKontrakTextFields = []string{
		"kd_klpd", "jenis_klpd", "nama_klpd", "kd_lpse", "kd_satker", "kd_satker_str", "nama_satker", "alamat_satker",
		"kd_nontender", "tahun_anggaran", "nama_paket", "mtd_pengadaan", "lingkup_pekerjaan", "informasi_lainnya",
		"no_kontrak", "no_sppbj", "jenis_kontrak", "status_kontrak", "kota_kontrak",
		"alasan_penetapan_status_kontrak", "apakah_addendum", "alasan_addendum",
		"alasan_ubah_nilai_kontrak", "alasan_nilai_kontrak_10_persen",
		"nama_ppk", "nip_ppk", "jabatan_ppk", "no_sk_ppk",
		"nama_penyedia", "bentuk_usaha_penyedia", "tipe_penyedia", "npwp_penyedia", "npwp16_penyedia",
		"wakil_sah_penyedia", "jabatan_wakil_penyedia", "anggota_kso",
		"nama_rek_bank", "no_rek_bank", "nama_pemilik_rek_bank",
	}
	// DECIMAL(24,2): nilai rupiah bisa berpecahan, jadi tidak dipotong ke BIGINT.
	nonTenderEkontrakKontrakDecimalFields = []string{"nilai_kontrak", "nilai_pdn_kontrak", "nilai_umk_kontrak"}
	nonTenderEkontrakKontrakIntFields     = []string{"versi_addendum"}
	nonTenderEkontrakKontrakDateFields    = []string{
		"tgl_kontrak", "tgl_kontrak_awal", "tgl_kontrak_akhir", "tgl_penetapan_status_kontrak",
	}
)

// Semua field yang sudah punya kolom sendiri; sisanya masuk ke extra_json.
func nonTenderEkontrakKontrakKnownFields() []string {
	var all []string
	all = append(all, nonTenderEkontrakKontrakTextFields...)
	all = append(all, nonTenderEkontrakKontrakDecimalFields...)
	all = append(all, nonTenderEkontrakKontrakIntFields...)
	all = append(all, nonTenderEkontrakKontrakDateFields...)
	return all
}

// Urutan kolom di sini harus sama dengan urutan argumen di
// insertNonTenderEkontrakKontrak. Nama kolom berasal dari konstanta di atas,
// bukan dari input pengguna.
var nonTenderEkontrakKontrakInsertSQL = buildNonTenderEkontrakKontrakInsertSQL()

func buildNonTenderEkontrakKontrakInsertSQL() string {
	cols := []string{"row_key"}
	cols = append(cols, nonTenderEkontrakKontrakKnownFields()...)
	cols = append(cols, "extra_json")

	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = "@p" + strconv.Itoa(i+1)
	}
	return "INSERT INTO inaproc_non_tender_ekontrak_kontrak (" + strings.Join(cols, ", ") +
		") VALUES (" + strings.Join(placeholders, ", ") + ")"
}

type syncNonTenderEkontrakKontrakRequest struct {
	KodeKLPD string `json:"kode_klpd"`
	Tahun    string `json:"tahun" binding:"required"`
}

func SyncNonTenderEkontrakKontrak(c *gin.Context) {
	var req syncNonTenderEkontrakKontrakRequest
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
		`DELETE FROM inaproc_non_tender_ekontrak_kontrak WHERE kd_klpd = @p1 AND tahun_anggaran = @p2`,
		req.KodeKLPD, req.Tahun,
	); err != nil {
		log.Println("[INAPROC SYNC WARN] gagal hapus data lama non-tender-ekontrak-kontrak:", err)
	}

	totalSynced, totalFailed := 0, 0
	cursor := ""
	pageCount := 0
	const maxPages = 200

	for {
		pageCount++
		if pageCount > maxPages {
			logInaprocSync("non-tender-ekontrak-kontrak", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "Melebihi batas maksimum halaman", adminUserID, startedAt)
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

		body, statusCode, err := callInaprocEndpoint("/api/v1/tender/non-tender-ekontrak-kontrak", params)
		if err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal request non-tender-ekontrak-kontrak:", err)
			logInaprocSync("non-tender-ekontrak-kontrak", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc saat sinkronisasi")
			return
		}

		if statusCode != http.StatusOK {
			errMsg := extractInaprocErrorMessage(body, statusCode)
			logInaprocSync("non-tender-ekontrak-kontrak", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, errMsg, adminUserID, startedAt)
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
			log.Println("[INAPROC SYNC ERROR] gagal parse non-tender-ekontrak-kontrak:", err, "| body:", string(body))
			logInaprocSync("non-tender-ekontrak-kontrak", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "gagal parse: "+err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca respons Inaproc saat sinkronisasi")
			return
		}

		if pageCount == 1 && len(envelope.Data) > 0 {
			log.Printf("[INAPROC SYNC DEBUG] Contoh baris non-tender-ekontrak-kontrak: %+v", envelope.Data[0])
		}

		for _, row := range envelope.Data {
			if err := insertNonTenderEkontrakKontrak(row); err != nil {
				log.Println("[INAPROC SYNC WARN] gagal simpan baris non-tender-ekontrak-kontrak:", err)
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

	logInaprocSync("non-tender-ekontrak-kontrak", req.KodeKLPD, req.Tahun, "", "success", totalSynced, catatanHasil(HasilSinkron{TotalGagal: totalFailed}), adminUserID, startedAt)
	utils.SuccessResponse(c, http.StatusOK, "Sinkronisasi berhasil", gin.H{"total_synced": totalSynced, "total_failed": totalFailed, "pages_fetched": pageCount})
}

// getDecimalString membaca field numerik (angka JSON atau string angka) dan
// mengembalikannya sebagai string desimal untuk kolom DECIMAL, tanpa memotong
// pecahan. Mengembalikan nil (NULL) kalau kosong atau bukan angka.
func getDecimalString(m map[string]interface{}, key string) interface{} {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	switch val := v.(type) {
	case json.Number:
		return val.String()
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case string:
		if _, err := strconv.ParseFloat(val, 64); err == nil {
			return val
		}
	}
	return nil
}

// Urutan argumen harus sama dengan urutan kolom di nonTenderEkontrakKontrakInsertSQL.
func nonTenderEkontrakKontrakArgs(row map[string]interface{}) []interface{} {
	args := []interface{}{generateInaprocRowHash(row)}
	for _, f := range nonTenderEkontrakKontrakTextFields {
		args = append(args, nullIfEmpty(getStr(row, f)))
	}
	for _, f := range nonTenderEkontrakKontrakDecimalFields {
		args = append(args, getDecimalString(row, f))
	}
	for _, f := range nonTenderEkontrakKontrakIntFields {
		args = append(args, getInt64FromAny(row, f))
	}
	for _, f := range nonTenderEkontrakKontrakDateFields {
		args = append(args, parseInaprocTime(getStr(row, f)))
	}
	args = append(args, extraFieldsColumn(row, nonTenderEkontrakKontrakKnownFields()))
	return args
}

func insertNonTenderEkontrakKontrak(row map[string]interface{}) error {
	_, err := database.DB.Exec(nonTenderEkontrakKontrakInsertSQL, nonTenderEkontrakKontrakArgs(row)...)
	return err
}

// ============================================================
// TENDER Endpoint 5: Non Tender Pengumuman
// ============================================================
//
// Sama seperti endpoint 4: respons datar, nama kolom di tabel
// inaproc_non_tender_pengumuman sama persis dengan nama field API, dan INSERT
// dibangun dari daftar nama di bawah. Daftar ini HARUS sinkron dengan migrasi 018.
var (
	// Disimpan sebagai NVARCHAR; nilai kosong/null disimpan sebagai NULL.
	nonTenderPengumumanTextFields = []string{
		"kd_klpd", "jenis_klpd", "nama_klpd", "kd_satker", "kd_satker_str", "nama_satker",
		"kd_lpse", "nama_lpse", "url_lpse",
		"kd_nontender", "kd_pkt_dce", "lls_id", "kd_rup", "tahun_anggaran", "nama_paket",
		"jenis_pengadaan", "kualifikasi_paket", "kontrak_pembayaran", "mtd_pemilihan",
		"sumber_dana", "mak", "repeat_order",
		"status_nontender", "ket_ditutup", "ket_diulang",
		"nip_nama_ppk", "nip_nama_pp", "nip_nama_pokja",
	}
	// DECIMAL(24,2): nilai rupiah bisa berpecahan, jadi tidak dipotong ke BIGINT.
	nonTenderPengumumanDecimalFields = []string{"pagu", "hps"}
	nonTenderPengumumanIntFields     = []string{"versi_nontender"}
	nonTenderPengumumanDateFields    = []string{"tgl_buat_paket", "tgl_kolektif_kolegial", "tgl_pengumuman_nontender"}
)

// Semua field yang sudah punya kolom sendiri; sisanya masuk ke extra_json.
func nonTenderPengumumanKnownFields() []string {
	var all []string
	all = append(all, nonTenderPengumumanTextFields...)
	all = append(all, nonTenderPengumumanDecimalFields...)
	all = append(all, nonTenderPengumumanIntFields...)
	all = append(all, nonTenderPengumumanDateFields...)
	return all
}

// Urutan kolom di sini harus sama dengan urutan argumen di
// nonTenderPengumumanArgs. Nama kolom berasal dari konstanta di atas, bukan
// dari input pengguna.
var nonTenderPengumumanInsertSQL = buildNonTenderPengumumanInsertSQL()

func buildNonTenderPengumumanInsertSQL() string {
	cols := []string{"row_key"}
	cols = append(cols, nonTenderPengumumanKnownFields()...)
	cols = append(cols, "extra_json")

	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = "@p" + strconv.Itoa(i+1)
	}
	return "INSERT INTO inaproc_non_tender_pengumuman (" + strings.Join(cols, ", ") +
		") VALUES (" + strings.Join(placeholders, ", ") + ")"
}

type syncNonTenderPengumumanRequest struct {
	KodeKLPD string `json:"kode_klpd"`
	Tahun    string `json:"tahun" binding:"required"`
}

func SyncNonTenderPengumuman(c *gin.Context) {
	var req syncNonTenderPengumumanRequest
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
		`DELETE FROM inaproc_non_tender_pengumuman WHERE kd_klpd = @p1 AND tahun_anggaran = @p2`,
		req.KodeKLPD, req.Tahun,
	); err != nil {
		log.Println("[INAPROC SYNC WARN] gagal hapus data lama non-tender-pengumuman:", err)
	}

	totalSynced, totalFailed := 0, 0
	cursor := ""
	pageCount := 0
	const maxPages = 200

	for {
		pageCount++
		if pageCount > maxPages {
			logInaprocSync("non-tender-pengumuman", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "Melebihi batas maksimum halaman", adminUserID, startedAt)
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

		body, statusCode, err := callInaprocEndpoint("/api/v1/tender/non-tender-pengumuman", params)
		if err != nil {
			log.Println("[INAPROC SYNC ERROR] gagal request non-tender-pengumuman:", err)
			logInaprocSync("non-tender-pengumuman", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusBadGateway, "Gagal menghubungi API Inaproc saat sinkronisasi")
			return
		}

		if statusCode != http.StatusOK {
			errMsg := extractInaprocErrorMessage(body, statusCode)
			logInaprocSync("non-tender-pengumuman", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, errMsg, adminUserID, startedAt)
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
			log.Println("[INAPROC SYNC ERROR] gagal parse non-tender-pengumuman:", err, "| body:", string(body))
			logInaprocSync("non-tender-pengumuman", req.KodeKLPD, req.Tahun, "", "failed", totalSynced, "gagal parse: "+err.Error(), adminUserID, startedAt)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca respons Inaproc saat sinkronisasi")
			return
		}

		if pageCount == 1 && len(envelope.Data) > 0 {
			log.Printf("[INAPROC SYNC DEBUG] Contoh baris non-tender-pengumuman: %+v", envelope.Data[0])
		}

		for _, row := range envelope.Data {
			if err := insertNonTenderPengumuman(row); err != nil {
				log.Println("[INAPROC SYNC WARN] gagal simpan baris non-tender-pengumuman:", err)
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

	logInaprocSync("non-tender-pengumuman", req.KodeKLPD, req.Tahun, "", "success", totalSynced, catatanHasil(HasilSinkron{TotalGagal: totalFailed}), adminUserID, startedAt)
	utils.SuccessResponse(c, http.StatusOK, "Sinkronisasi berhasil", gin.H{"total_synced": totalSynced, "total_failed": totalFailed, "pages_fetched": pageCount})
}

// Urutan argumen harus sama dengan urutan kolom di nonTenderPengumumanInsertSQL.
func nonTenderPengumumanArgs(row map[string]interface{}) []interface{} {
	args := []interface{}{generateInaprocRowHash(row)}
	for _, f := range nonTenderPengumumanTextFields {
		args = append(args, nullIfEmpty(getStr(row, f)))
	}
	for _, f := range nonTenderPengumumanDecimalFields {
		args = append(args, getDecimalString(row, f))
	}
	for _, f := range nonTenderPengumumanIntFields {
		args = append(args, getInt64FromAny(row, f))
	}
	for _, f := range nonTenderPengumumanDateFields {
		args = append(args, parseInaprocTime(getStr(row, f)))
	}
	args = append(args, extraFieldsColumn(row, nonTenderPengumumanKnownFields()))
	return args
}

func insertNonTenderPengumuman(row map[string]interface{}) error {
	_, err := database.DB.Exec(nonTenderPengumumanInsertSQL, nonTenderPengumumanArgs(row)...)
	return err
}

