package handlers

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/utils"
)

// Implementasi bersama untuk endpoint Inaproc (Tender, E-Katalog archive, E-Katalog V6) yang respons datarnya (satu objek per baris,
// tanpa array bersarang) dan berhalaman dengan cursor: non-tender-selesai, pencatatan-non-tender, ekatalog/penyedia-detail, dst.
// Tiap endpoint cukup mendeklarasikan nama jalur API, nama tabel, dan daftar field menurut tipenya; proxy ke Inaproc, sinkronisasi
// ke database, dan daftar lokal ditangani di sini.
//
// Nama kolom di tabel sama persis dengan nama field API. Daftar field di deklarasi HARUS sinkron dengan migrasi tabelnya; tes
// (TestEndpointDatarKolomSamaDenganMigrasi) menjaganya. Field API yang belum dipetakan ke kolom disimpan di extra_json supaya
// tidak hilang diam-diam.
//
// Aturan penyaring yang bukan bentuk bawaan (lihat saringan) dipasang per endpoint lewat GetDengan, SyncDengan, dan ListLocalDengan:
// mesin pengambilan, penyimpanan, dan pencatatan tetap yang di sini; hanya penyusunan parameter, rencana sinkronisasi, dan klausa
// WHERE daftar lokal yang diganti.
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

	// KolomKonteks: kolom tabel yang BUKAN field API; nilainya berasal dari permintaan sinkronisasi (kolom → kunci konteks), mis.
	// kode penyedia pada daftar produk yang responsnya tidak memuatnya. Kolom ini dihitung dalam migrasi (SemuaKolom) dan disusun
	// terurut menurut nama di INSERT, setelah kolom field API.
	KolomKonteks map[string]string
	// IsiBilaKosong: field API → kunci konteks; bila nilai di baris API kosong, diisi dari konteks permintaan (mis. kode induk
	// kategori yang tidak ada di respons level 2 dan 3).
	IsiBilaKosong map[string]string

	// Kolom yang dipilih untuk daftar lokal (GET .../local); harus memuat synced_at.
	KolomDaftar string

	// MenerimaKdTender: GET boleh memakai kd_tender saja (tanpa tahun dan kode_klpd), seperti tender/pengumuman yang memang
	// punya dua skenario: (tahun + kode_klpd) atau kd_tender. Bila kd_tender diisi, hanya kd_tender yang diteruskan ke Inaproc.
	// Sinkronisasi tetap per tahun dan kode_klpd.
	MenerimaKdTender bool

	insertSQL    string
	kolomKonteks []string // kunci KolomKonteks, terurut
}

// saringan menentukan parameter yang wajib pada GET dan sinkronisasi serta kolom penyaring di database. Tiga bentuk bawaan:
//   - nol: kode_klpd + tahun; kolom kd_klpd dan tahun_anggaran (atau KolomKLPD/KolomTahun bila dinamai lain).
//   - TanpaTahun: kode_klpd saja (data rujukan per KLPD); kolom kd_klpd.
//   - Param terisi: satu kode tunggal (pencarian rujukan per kode, mis. kode_penyedia); kolom `Kolom`. Pada sinkronisasi kode
//     dikirim di badan JSON sebagai "kode".
type saringan struct {
	Param      string // nama parameter kode tunggal di API Inaproc (mis. "kode_penyedia"); kosong = berdasarkan kode_klpd
	Kolom      string // kolom tabel yang menyimpan kode itu (mis. "kd_penyedia"); wajib bila Param terisi
	TanpaTahun bool   // hanya bila Param kosong: tidak ada parameter tahun

	// Nama kolom bila bukan kd_klpd / tahun_anggaran (mis. V6: kode_klpd dan fiscal_year).
	KolomKLPD  string
	KolomTahun string
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

func (e *endpointDatar) kolomKLPD() string {
	if e.Saring.KolomKLPD != "" {
		return e.Saring.KolomKLPD
	}
	return "kd_klpd"
}

func (e *endpointDatar) kolomTahun() string {
	if e.Saring.KolomTahun != "" {
		return e.Saring.KolomTahun
	}
	return "tahun_anggaran"
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
	for k := range e.KolomKonteks {
		e.kolomKonteks = append(e.kolomKonteks, k)
	}
	sort.Strings(e.kolomKonteks)
	e.insertSQL = e.buildInsertSQL()
	return &e
}

// KnownFields: semua field API yang sudah punya kolom sendiri, menurut urutan kolom di INSERT (setelah row_key).
func (e *endpointDatar) KnownFields() []string {
	var all []string
	all = append(all, e.Teks...)
	all = append(all, e.Desimal...)
	all = append(all, e.Bulat...)
	all = append(all, e.Tanggal...)
	return all
}

// SemuaKolom: kolom data tabel (tanpa row_key, extra_json, dan penanda waktu): field API ditambah kolom konteks.
func (e *endpointDatar) SemuaKolom() []string {
	return append(e.KnownFields(), e.kolomKonteks...)
}

// Nama kolom berasal dari deklarasi di kode, bukan dari input pengguna.
func (e *endpointDatar) buildInsertSQL() string {
	cols := []string{"row_key"}
	cols = append(cols, e.SemuaKolom()...)
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

// Args menyusun argumen INSERT dengan konteks sebatas kode KLPD yang diminta; lihat ArgsKonteks.
func (e *endpointDatar) Args(row map[string]interface{}, kodeKLPD string) []interface{} {
	return e.ArgsKonteks(row, map[string]string{"kode_klpd": kodeKLPD})
}

// ArgsKonteks menyusun argumen INSERT; urutannya sama dengan kolom di insertSQL. Kolom KLPD yang kosong di respons diisi kode
// KLPD yang diminta (kunci konteks "kode_klpd"), supaya hapus-sebelum-tarik dan daftar lokal tetap menemukan barisnya; field di
// IsiBilaKosong diisi dengan cara yang sama dari kunci konteksnya; kolom konteks diisi dari konteks. row_key dihitung dari baris
// asli dari API.
func (e *endpointDatar) ArgsKonteks(row map[string]interface{}, konteks map[string]string) []interface{} {
	args := []interface{}{generateInaprocRowHash(row)}
	for _, f := range e.Teks {
		v := teksKolom(row, f)
		if v == "" {
			if kunci, ada := e.IsiBilaKosong[f]; ada {
				v = konteks[kunci]
			} else if f == e.kolomKLPD() {
				v = konteks["kode_klpd"]
			}
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
	for _, kolom := range e.kolomKonteks {
		args = append(args, nullIfEmpty(konteks[e.KolomKonteks[kolom]]))
	}
	args = append(args, extraFieldsColumn(row, e.KnownFields()))
	return args
}

func (e *endpointDatar) insert(ex pelaksanaSQL, row map[string]interface{}, konteks map[string]string) error {
	_, err := ex.Exec(e.insertSQL, e.ArgsKonteks(row, konteks)...)
	return err
}

// paramGetBawaan menyusun parameter penyaring GET menurut bentuk saringan bawaan; pesan tidak kosong = permintaan ditolak (400).
func (e *endpointDatar) paramGetBawaan(c *gin.Context) (url.Values, string) {
	kodeKLPD := c.DefaultQuery("kode_klpd", kemenkeuKLPDCodeTender)
	tahun := c.Query("tahun")
	kdTender := ""
	if e.MenerimaKdTender {
		kdTender = strings.TrimSpace(c.Query("kd_tender"))
	}

	params := url.Values{}
	switch {
	case kdTender != "":
		if _, err := strconv.ParseInt(kdTender, 10, 64); err != nil {
			return nil, "Parameter 'kd_tender' harus berupa angka"
		}
		params.Set("kd_tender", kdTender)
	case e.Saring.Param != "":
		kode := strings.TrimSpace(c.Query(e.Saring.Param))
		if kode == "" {
			return nil, "Parameter '" + e.Saring.Param + "' wajib diisi"
		}
		if len(kode) > panjangKodeMaks {
			return nil, "Parameter '" + e.Saring.Param + "' terlalu panjang"
		}
		params.Set(e.Saring.Param, kode)
	case e.Saring.TanpaTahun:
		params.Set("kode_klpd", kodeKLPD)
	case tahun == "" && e.MenerimaKdTender:
		return nil, "Isi 'tahun' atau 'kd_tender'"
	case tahun == "":
		return nil, "Parameter 'tahun' wajib diisi"
	default:
		params.Set("kode_klpd", kodeKLPD)
		params.Set("tahun", tahun)
	}
	return params, ""
}

// Get meneruskan permintaan ke Inaproc (data langsung, tanpa database).
func (e *endpointDatar) Get(c *gin.Context) { e.GetDengan(c, e.paramGetBawaan) }

// GetDengan sama dengan Get, tetapi penyusunan parameter penyaring (di luar limit dan cursor) diserahkan ke `susun`; pesan tidak
// kosong yang dikembalikannya menolak permintaan dengan 400.
func (e *endpointDatar) GetDengan(c *gin.Context, susun func(*gin.Context) (url.Values, string)) {
	if config.Cfg.InaprocToken == "" {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Integrasi Inaproc belum dikonfigurasi (token kosong)")
		return
	}

	params, pesan := susun(c)
	if pesan != "" {
		utils.ErrorResponse(c, http.StatusBadRequest, pesan)
		return
	}
	params.Set("limit", strconv.Itoa(clampLimit(c.DefaultQuery("limit", "50"))))
	if cursor := c.Query("cursor"); cursor != "" {
		params.Set("cursor", cursor)
	}

	body, statusCode, err := callInaprocEndpointInteraktif(e.jalurAPI(), params)
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

// rencanaSync: apa yang dikerjakan satu sinkronisasi.
type rencanaSync struct {
	Params    url.Values // penyaring ke Inaproc (di luar limit dan cursor)
	HapusSQL  string     // DELETE data lama yang akan diganti
	HapusArgs []interface{}
	// Konteks: nilai dari permintaan untuk kolom konteks dan IsiBilaKosong; kunci "kode_klpd" mengisi kolom KLPD yang kosong.
	Konteks map[string]string
	// Yang dicatat di inaproc_sync_log: kode_klpd, tahun, dan jenis_paket (dipakai untuk menandai penyaring lain, mis. kode).
	LogKLPD, LogTahun, LogJenis string
	// Kemajuan (opsional): dipanggil dengan pesan singkat tiap halaman terambil dan tiap sekian baris tersimpan.
	Kemajuan func(pesan string)
}

// potong memotong teks ke n karakter (bukan byte), untuk kolom log yang sempit.
func potong(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// rencanaBawaan menyusun rencana sinkronisasi menurut bentuk saringan bawaan dari badan JSON; pesan tidak kosong = ditolak (400).
func (e *endpointDatar) rencanaBawaan(c *gin.Context) (*rencanaSync, string) {
	var req syncDatarRequest
	// Badan kosong dibolehkan (saringan klpd saja tidak butuh isian); JSON rusak ditolak.
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		return nil, "Permintaan tidak valid"
	}
	return e.rencanaDari(req)
}

// rencanaDari sama dengan rencanaBawaan, tetapi dari nilai yang sudah terbaca (dipakai penarikan terjadwal dan antrean, tanpa HTTP).
func (e *endpointDatar) rencanaDari(req syncDatarRequest) (*rencanaSync, string) {
	req.Tahun = strings.TrimSpace(req.Tahun)
	req.Kode = strings.TrimSpace(req.Kode)
	if req.KodeKLPD == "" {
		req.KodeKLPD = kemenkeuKLPDCodeTender
	}
	switch {
	case e.Saring.Param != "" && req.Kode == "":
		return nil, "kode wajib diisi"
	case e.Saring.Param != "" && len(req.Kode) > panjangKodeMaks:
		return nil, "kode terlalu panjang"
	case e.Saring.Param == "" && !e.Saring.TanpaTahun && req.Tahun == "":
		return nil, "tahun wajib diisi"
	}

	r := &rencanaSync{
		Params:  url.Values{},
		Konteks: map[string]string{"kode_klpd": req.KodeKLPD, "tahun": req.Tahun, "kode": req.Kode},
		// Yang dicatat: klpd dan tahun, atau (untuk saringan kode) kode itu di kolom jenis_paket.
		LogKLPD: req.KodeKLPD, LogTahun: req.Tahun,
	}
	switch {
	case e.Saring.Param != "":
		r.Params.Set(e.Saring.Param, req.Kode)
		r.HapusSQL = "DELETE FROM " + e.Tabel + " WHERE " + e.Saring.Kolom + " = @p1"
		r.HapusArgs = []interface{}{req.Kode}
		r.LogKLPD, r.LogTahun, r.LogJenis = "", "", potong(req.Kode, 50)
	case e.Saring.TanpaTahun:
		r.Params.Set("kode_klpd", req.KodeKLPD)
		r.HapusSQL = "DELETE FROM " + e.Tabel + " WHERE " + e.kolomKLPD() + " = @p1"
		r.HapusArgs = []interface{}{req.KodeKLPD}
	default:
		r.Params.Set("kode_klpd", req.KodeKLPD)
		r.Params.Set("tahun", req.Tahun)
		r.HapusSQL = "DELETE FROM " + e.Tabel + " WHERE " + e.kolomKLPD() + " = @p1 AND " + e.kolomTahun() + " = @p2"
		r.HapusArgs = []interface{}{req.KodeKLPD, req.Tahun}
	}
	return r, ""
}

// HasilSinkron: hasil satu sinkronisasi dataset.
type HasilSinkron struct {
	TotalSinkron int // baris yang tersimpan
	TotalGagal   int // baris yang gagal disimpan (dilewati)
	Halaman      int // halaman yang diambil dari Inaproc
}

// GalatSinkron: kegagalan sinkronisasi beserta status HTTP yang cocok bila dijawabkan ke klien. Hulu = Inaproc menolak (Status
// berasal dari Inaproc, mis. 401/429), bukan kegagalan di sisi kita.
type GalatSinkron struct {
	Status   int
	Pesan    string
	Sebagian int // baris yang sudah tersimpan sebelum gagal (selalu 0: penyimpanan baru dimulai setelah semua halaman terambil)
	Hulu     bool
}

func (g *GalatSinkron) Error() string { return g.Pesan }

const (
	maksHalamanSinkron = 200
	// statusDibatalkan: status (tidak baku) untuk sinkronisasi yang dibatalkan lewat konteks.
	statusDibatalkan = 499
	// Pembatalan dan penyimpanan diperiksa tiap sekian baris.
	periksaTiapBaris = 500
)

// pelaksanaSQL: *sql.DB atau *sql.Tx.
type pelaksanaSQL interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

// Sync (admin) menarik seluruh halaman dari Inaproc ke tabel lokal. Data lama untuk penyaring yang sama (klpd dan tahun, klpd saja,
// atau satu kode) diganti, dan penggantian itu atomik: halaman diambil lebih dulu ke berkas sementara tanpa menyentuh database, baru
// data lama dihapus dan yang baru disisipkan dalam satu transaksi. Jika pengambilan gagal atau dibatalkan, data lama tetap utuh. Baris
// yang gagal disimpan dilewati, dihitung, dan dilaporkan (tidak menggagalkan seluruh sinkronisasi).
func (e *endpointDatar) Sync(c *gin.Context) { e.SyncDengan(c, e.rencanaBawaan) }

// SyncDengan sama dengan Sync, tetapi rencananya (parameter, penghapusan data lama, konteks, catatan log) disusun oleh `susun`;
// pesan tidak kosong yang dikembalikannya menolak permintaan dengan 400.
func (e *endpointDatar) SyncDengan(c *gin.Context, susun func(*gin.Context) (*rencanaSync, string)) {
	rencana, pesan := susun(c)
	if pesan != "" {
		utils.ErrorResponse(c, http.StatusBadRequest, pesan)
		return
	}

	// Tidak memakai konteks permintaan: sinkronisasi tetap selesai walau klien menutup koneksi di tengah jalan.
	hasil, err := e.Jalankan(context.Background(), rencana, c.GetString("user_id"))
	if err != nil {
		var g *GalatSinkron
		switch {
		case errors.As(err, &g) && g.Hulu:
			c.JSON(g.Status, gin.H{"success": false, "message": g.Pesan, "partial_synced": g.Sebagian})
		case errors.As(err, &g):
			utils.ErrorResponse(c, g.Status, g.Pesan)
		default:
			utils.ErrorResponse(c, http.StatusInternalServerError, "Sinkronisasi gagal")
		}
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Sinkronisasi berhasil", gin.H{"total_synced": hasil.TotalSinkron, "total_failed": hasil.TotalGagal, "pages_fetched": hasil.Halaman})
}

// Jalankan mengerjakan satu rencana sinkronisasi tanpa konteks HTTP (dipakai Sync, antrean penarikan, dan penjadwal). oleh = id
// pengguna pemicu; kosong untuk penarikan otomatis. Pembatalan lewat ctx menghentikan pengambilan (permintaan yang sedang berjalan ikut
// dibatalkan) maupun penyimpanan (transaksi dibatalkan, data lama utuh). Setiap hasil dicatat di inaproc_sync_log.
func (e *endpointDatar) Jalankan(ctx context.Context, rencana *rencanaSync, oleh string) (HasilSinkron, error) {
	nama := e.namaLog()
	mulai := time.Now()
	catat := func(status string, total int, catatan string) {
		logInaprocSync(nama, rencana.LogKLPD, rencana.LogTahun, rencana.LogJenis, status, total, catatan, oleh, mulai)
	}
	gagal := func(g *GalatSinkron, catatan string) (HasilSinkron, error) {
		catat("failed", g.Sebagian, catatan)
		return HasilSinkron{}, g
	}
	dibatalkan := func() (HasilSinkron, error) {
		return gagal(&GalatSinkron{Status: statusDibatalkan, Pesan: "Sinkronisasi dibatalkan"}, "Dibatalkan")
	}
	kabar := func(format string, a ...interface{}) {
		if rencana.Kemajuan != nil {
			rencana.Kemajuan(fmt.Sprintf(format, a...))
		}
	}

	// ---- Fase 1: ambil semua halaman ke berkas sementara (satu baris JSON per baris data) ----
	spool, err := os.CreateTemp("", "inaproc-sinkron-*.jsonl")
	if err != nil {
		log.Println("[INAPROC SYNC ERROR] gagal membuat berkas sementara "+nama+":", err)
		return gagal(&GalatSinkron{Status: http.StatusInternalServerError, Pesan: "Gagal menyiapkan penampungan sementara"}, err.Error())
	}
	defer func() {
		spool.Close()
		os.Remove(spool.Name())
	}()
	tulis := bufio.NewWriterSize(spool, 1<<20)

	jumlahBaris, halaman := 0, 0
	cursor := ""
	for {
		if ctx.Err() != nil {
			return dibatalkan()
		}
		halaman++
		if halaman > maksHalamanSinkron {
			return gagal(&GalatSinkron{Status: http.StatusInternalServerError, Pesan: "Sinkronisasi dihentikan: terlalu banyak halaman"}, "Melebihi batas maksimum halaman")
		}

		params := url.Values{}
		for k, v := range rencana.Params {
			params[k] = append([]string(nil), v...)
		}
		params.Set("limit", "1000")
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		body, statusCode, err := callInaprocEndpointCtx(ctx, e.jalurAPI(), params)
		if err != nil {
			if ctx.Err() != nil {
				return dibatalkan()
			}
			log.Println("[INAPROC SYNC ERROR] gagal request "+nama+":", err)
			return gagal(&GalatSinkron{Status: http.StatusBadGateway, Pesan: "Gagal menghubungi API Inaproc saat sinkronisasi"}, err.Error())
		}

		if statusCode != http.StatusOK {
			errMsg := extractInaprocErrorMessage(body, statusCode)
			return gagal(&GalatSinkron{Status: statusCode, Pesan: "Sinkronisasi gagal: " + errMsg, Hulu: true}, errMsg)
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
			return gagal(&GalatSinkron{Status: http.StatusInternalServerError, Pesan: "Gagal membaca respons Inaproc saat sinkronisasi"}, "gagal parse: "+err.Error())
		}

		if halaman == 1 && len(envelope.Data) > 0 {
			log.Printf("[INAPROC SYNC DEBUG] Contoh baris %s: %+v", nama, envelope.Data[0])
		}

		for _, row := range envelope.Data {
			b, err := json.Marshal(row)
			if err != nil {
				return gagal(&GalatSinkron{Status: http.StatusInternalServerError, Pesan: "Gagal memproses respons Inaproc"}, "marshal baris: "+err.Error())
			}
			if _, err := tulis.Write(append(b, '\n')); err != nil {
				return gagal(&GalatSinkron{Status: http.StatusInternalServerError, Pesan: "Gagal menyimpan ke penampungan sementara"}, err.Error())
			}
			jumlahBaris++
		}

		kabar("Mengambil dari Inaproc: halaman %d, %d baris", halaman, jumlahBaris)
		if !envelope.Meta.HasMore || envelope.Meta.Cursor == "" {
			break
		}
		cursor = envelope.Meta.Cursor
	}
	if err := tulis.Flush(); err != nil {
		return gagal(&GalatSinkron{Status: http.StatusInternalServerError, Pesan: "Gagal menyimpan ke penampungan sementara"}, err.Error())
	}
	if _, err := spool.Seek(0, io.SeekStart); err != nil {
		return gagal(&GalatSinkron{Status: http.StatusInternalServerError, Pesan: "Gagal membaca penampungan sementara"}, err.Error())
	}

	// ---- Fase 2: ganti data lama dengan yang baru dalam satu transaksi ----
	if ctx.Err() != nil {
		return dibatalkan()
	}
	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Println("[INAPROC SYNC ERROR] gagal memulai transaksi "+nama+":", err)
		return gagal(&GalatSinkron{Status: http.StatusInternalServerError, Pesan: "Gagal memulai penyimpanan data"}, err.Error())
	}
	if _, err := tx.Exec(rencana.HapusSQL, rencana.HapusArgs...); err != nil {
		_ = tx.Rollback()
		log.Println("[INAPROC SYNC ERROR] gagal hapus data lama "+nama+":", err)
		return gagal(&GalatSinkron{Status: http.StatusInternalServerError, Pesan: "Gagal mengganti data lama (data lama tetap utuh)"}, "hapus data lama: "+err.Error())
	}

	totalSynced, totalFailed := 0, 0
	dec := json.NewDecoder(bufio.NewReaderSize(spool, 1<<20))
	dec.UseNumber()
	for n := 0; dec.More(); n++ {
		if n%periksaTiapBaris == 0 && ctx.Err() != nil {
			_ = tx.Rollback()
			return dibatalkan()
		}
		if n > 0 && n%(2*periksaTiapBaris) == 0 {
			kabar("Menyimpan ke database: %d dari %d baris", n, jumlahBaris)
		}
		var row map[string]interface{}
		if err := dec.Decode(&row); err != nil {
			_ = tx.Rollback()
			return gagal(&GalatSinkron{Status: http.StatusInternalServerError, Pesan: "Gagal membaca penampungan sementara"}, err.Error())
		}
		if err := e.insert(tx, row, rencana.Konteks); err != nil {
			log.Println("[INAPROC SYNC WARN] gagal simpan baris "+nama+":", err)
			totalFailed++
			continue
		}
		totalSynced++
	}
	if err := tx.Commit(); err != nil {
		log.Println("[INAPROC SYNC ERROR] gagal commit "+nama+":", err)
		return gagal(&GalatSinkron{Status: http.StatusInternalServerError, Pesan: "Gagal menyimpan data (data lama tetap utuh)"}, "commit: "+err.Error())
	}

	catatan := ""
	if totalFailed > 0 {
		catatan = fmt.Sprintf("%d baris gagal disimpan", totalFailed)
	}
	catat("success", totalSynced, catatan)
	return HasilSinkron{TotalSinkron: totalSynced, TotalGagal: totalFailed, Halaman: halaman}, nil
}

// whereBawaan: klausa WHERE daftar lokal menurut bentuk saringan bawaan: kolom KLPD (+ kolom tahun bila ada) atau, untuk saringan
// kode tunggal, kolom kodenya (tanpa kode = semua baris). Mengembalikan "" bila tanpa penyaring.
func (e *endpointDatar) whereBawaan(c *gin.Context) (string, []interface{}) {
	if e.Saring.Param != "" {
		if kode := strings.TrimSpace(c.Query(e.Saring.Param)); kode != "" {
			return " WHERE " + e.Saring.Kolom + " = @p1", []interface{}{kode}
		}
		return "", nil
	}
	where := " WHERE " + e.kolomKLPD() + " = @p1"
	args := []interface{}{c.DefaultQuery("kode_klpd", kemenkeuKLPDCodeTender)}
	if tahun := c.Query("tahun"); tahun != "" && !e.Saring.TanpaTahun {
		where += " AND " + e.kolomTahun() + " = @p2"
		args = append(args, tahun)
	}
	return where, args
}

// ListLocal membaca data yang sudah disinkronkan dari database PASTI. Penyaringnya mengikuti bentuk saringan endpoint.
func (e *endpointDatar) ListLocal(c *gin.Context) { e.ListLocalDengan(c, e.whereBawaan) }

// ListLocalDengan sama dengan ListLocal, tetapi klausa WHERE (diawali " WHERE ", atau kosong) dan argumennya disusun oleh `susun`.
func (e *endpointDatar) ListLocalDengan(c *gin.Context, susun func(*gin.Context) (string, []interface{})) {
	limit := clampLimit(c.DefaultQuery("limit", "50"))

	where, args := susun(c)
	query := "SELECT " + e.KolomDaftar + " FROM " + e.Tabel + where
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
