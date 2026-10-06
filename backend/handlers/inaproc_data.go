package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/database"
	"pasti-v3-backend/laporan"
	"pasti-v3-backend/peran"
	"pasti-v3-backend/utils"
)

// Tampilan dan ekspor data lokal semua dataset Pengadaan/Tender/E-Katalog (tabel inaproc_*). Satu pembaca generik menyusun query dari
// katalog (nama tabel dan kolom berasal dari kode, bukan dari masukan pengguna; nilai penyaring selalu parameter).

const (
	dataHalamanMaks = 200
	dataTimeout     = 30 * time.Second
	eksporTimeout   = 10 * time.Minute
)

// Kolom teknis yang tidak ikut ekspor: kunci hash baris dan sisa field API yang belum dipetakan (JSON mentah).
var kolomTeknis = map[string]bool{"row_key": true, "extra_json": true}

// PenyaringData: penyaring tampilan dan ekspor. Kolom yang tidak dimiliki dataset diabaikan. Cakupan membatasi baris ke satker peran pengguna (nilai nol =
// semua data, mis. untuk hitungan kelengkapan dasbor yang hanya menanyakan ada-tidaknya data).
type PenyaringData struct {
	KodeKLPD string
	Tahun    string
	Cari     string
	Cakupan  peran.Cakupan
}

func penyaringDariQuery(c *gin.Context) PenyaringData {
	return PenyaringData{
		KodeKLPD: strings.TrimSpace(c.Query("kode_klpd")),
		Tahun:    strings.TrimSpace(c.Query("tahun")),
		Cari:     strings.TrimSpace(c.Query("cari")),
		Cakupan:  peran.DariGin(c),
	}
}

// Validasi: pesan tidak kosong = ditolak.
func (p PenyaringData) Validasi() string {
	if !tidakPanjang(p.KodeKLPD, p.Tahun) || len(p.Cari) > 200 {
		return "Penyaring terlalu panjang"
	}
	return ""
}

// kutip memberi tanda kurung siku pada nama kolom/tabel (nama berasal dari katalog atau metadata database).
func kutip(nama string) string { return "[" + strings.ReplaceAll(nama, "]", "]]") + "]" }

// likeAman: karakter khusus LIKE dibuat literal.
func likeAman(s string) string {
	return strings.NewReplacer("[", "[[]", "%", "[%]", "_", "[_]").Replace(s)
}

// where menyusun klausa WHERE (tanpa kata WHERE) dan argumennya (@p1, @p2, ... berurutan); kosong bila tanpa penyaring.
func (d *DatasetPenarikan) where(p PenyaringData) (string, []interface{}) {
	var bagian []string
	var args []interface{}
	tambah := func(format string, nilai interface{}) {
		args = append(args, nilai)
		bagian = append(bagian, fmt.Sprintf(format, "@p"+strconv.Itoa(len(args))))
	}
	if d.KolomKLPD != "" && p.KodeKLPD != "" {
		tambah(kutip(d.KolomKLPD)+" = %s", p.KodeKLPD)
	}
	if d.KolomTahun != "" && p.Tahun != "" {
		tambah(kutip(d.KolomTahun)+" = %s", p.Tahun)
	}
	if !p.Cakupan.SemuaData() {
		// Dataset yang tidak dapat dibatasi tidak pernah sampai sini (ditolak pemanggil); bila sampai, kondisinya menutup semua baris.
		kond, _ := kondisiSatkerInaproc(d.Tabel, p.Cakupan)
		bagian = append(bagian, "("+kond+")")
	}
	if p.Cari != "" && len(d.KolomRingkas) > 0 {
		args = append(args, "%"+likeAman(p.Cari)+"%")
		ph := "@p" + strconv.Itoa(len(args))
		var atau []string
		for _, k := range d.KolomRingkas {
			atau = append(atau, "CAST("+kutip(k)+" AS NVARCHAR(4000)) LIKE "+ph)
		}
		bagian = append(bagian, "("+strings.Join(atau, " OR ")+")")
	}
	return strings.Join(bagian, " AND "), args
}

func (d *DatasetPenarikan) hitung(ctx context.Context, db *sql.DB, p PenyaringData) (int64, error) {
	w, args := d.where(p)
	q := "SELECT COUNT_BIG(*) FROM " + kutip(d.Tabel)
	if w != "" {
		q += " WHERE " + w
	}
	var n int64
	err := db.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}

// semuaKolom: kolom data tabel menurut urutan di database, tanpa kolom teknis.
func (d *DatasetPenarikan) semuaKolom(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT TOP 0 * FROM "+kutip(d.Tabel))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nama, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, n := range nama {
		if !kolomTeknis[n] {
			out = append(out, n)
		}
	}
	return out, nil
}

// pilihan kolom hasil: ringkas (kolom tampilan) atau semua (seluruh kolom tabel).
func (d *DatasetPenarikan) kolomHasil(ctx context.Context, db *sql.DB, semua bool) ([]string, error) {
	if !semua {
		return d.KolomRingkas, nil
	}
	return d.semuaKolom(ctx, db)
}

// bacaBaris menjalankan SELECT kolom FROM tabel [WHERE] ORDER BY kunci dengan halaman opsional (limit <= 0 = semua baris).
func (d *DatasetPenarikan) bacaBaris(ctx context.Context, db *sql.DB, kolom []string, p PenyaringData, offset, limit int) (*sumberSQL, error) {
	daftar := make([]string, len(kolom))
	for i, k := range kolom {
		daftar[i] = kutip(k)
	}
	w, args := d.where(p)
	q := "SELECT " + strings.Join(daftar, ", ") + " FROM " + kutip(d.Tabel)
	if w != "" {
		q += " WHERE " + w
	}
	q += " ORDER BY " + kutip(d.KolomKunci)
	if limit > 0 {
		q += fmt.Sprintf(" OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, limit)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return newSumberSQL(rows, len(kolom))
}

// sumberSQL membungkus *sql.Rows sebagai laporan.Sumber; DECIMAL (dikirim driver sebagai teks) diubah menjadi bilangan.
type sumberSQL struct {
	rows    *sql.Rows
	tipe    []string // nama tipe database tiap kolom
	desimal []bool
	buf     []interface{}
	nilai   []interface{}
	err     error
}

func newSumberSQL(rows *sql.Rows, n int) (*sumberSQL, error) {
	s := &sumberSQL{rows: rows, tipe: make([]string, n), desimal: make([]bool, n), buf: make([]interface{}, n), nilai: make([]interface{}, n)}
	if cts, err := rows.ColumnTypes(); err == nil {
		for i, ct := range cts {
			if i >= n {
				break
			}
			s.tipe[i] = strings.ToUpper(ct.DatabaseTypeName())
			s.desimal[i] = s.tipe[i] == "DECIMAL" || s.tipe[i] == "NUMERIC" || s.tipe[i] == "MONEY" || s.tipe[i] == "SMALLMONEY"
		}
	}
	return s, nil
}

func (s *sumberSQL) Next() bool {
	if !s.rows.Next() {
		s.err = s.rows.Err()
		return false
	}
	ptr := make([]interface{}, len(s.buf))
	for i := range ptr {
		ptr[i] = &s.buf[i]
	}
	if err := s.rows.Scan(ptr...); err != nil {
		s.err = err
		return false
	}
	for i, v := range s.buf {
		if b, ok := v.([]byte); ok && s.desimal[i] {
			if f, err := strconv.ParseFloat(string(b), 64); err == nil {
				v = f
			}
		}
		s.nilai[i] = v
	}
	return true
}

func (s *sumberSQL) Nilai() []interface{} { return s.nilai }
func (s *sumberSQL) Err() error           { return s.err }
func (s *sumberSQL) Close()               { s.rows.Close() }

// jenisKolom: "angka", "tanggal", atau "teks" menurut tipe database (untuk perataan dan format di halaman).
func (s *sumberSQL) jenisKolom(i int) string {
	switch s.tipe[i] {
	case "DECIMAL", "NUMERIC", "MONEY", "SMALLMONEY", "INT", "BIGINT", "SMALLINT", "TINYINT", "FLOAT", "REAL":
		return "angka"
	case "DATETIME2", "DATETIME", "DATE", "SMALLDATETIME":
		return "tanggal"
	}
	return "teks"
}

// nilaiJSON: nilai untuk dikirim ke halaman; waktu menjadi teks ISO, byte menjadi teks.
func nilaiJSON(v interface{}) interface{} {
	switch x := v.(type) {
	case []byte:
		return string(x)
	case time.Time:
		return laporan.Teks(x)
	}
	return v
}

// ---- handler ----

// BolehDilihat: apakah data dataset ini boleh dibuka bagi cakupan peran. Peran yang melihat seluruh data boleh semuanya; peran terbatas hanya dataset yang
// dapat dibatasi per satker (lihat inaproc_cakupan.go).
func (d *DatasetPenarikan) BolehDilihat(cak peran.Cakupan) bool {
	return cak.SemuaData() || DapatDibatasi(d.Tabel)
}

func datasetDariRute(c *gin.Context) (*DatasetPenarikan, bool) {
	d, ok := DatasetByID(c.Param("awalan") + "/" + c.Param("nama"))
	if !ok {
		utils.ErrorResponse(c, http.StatusNotFound, "Dataset tidak dikenal")
		return nil, false
	}
	if !d.BolehDilihat(peran.DariGin(c)) {
		utils.ErrorResponse(c, http.StatusForbidden, "Dataset ini belum dapat dibatasi per satker, jadi tidak tersedia untuk peran Anda")
		return nil, false
	}
	return d, true
}

type kolomInfo struct {
	Nama  string `json:"nama"`
	Label string `json:"label"`
	Jenis string `json:"jenis"`
}

// ringkas: jumlah baris dan waktu penarikan terakhir data dataset ini dalam cakupan.
func (d *DatasetPenarikan) ringkas(ctx context.Context, db *sql.DB, cak peran.Cakupan) (int64, *time.Time, error) {
	q := "SELECT COUNT_BIG(*), MAX(synced_at) FROM " + kutip(d.Tabel)
	if !cak.SemuaData() {
		kond, _ := kondisiSatkerInaproc(d.Tabel, cak)
		q += " WHERE " + kond
	}
	var n int64
	var t sql.NullTime
	if err := db.QueryRowContext(ctx, q).Scan(&n, &t); err != nil {
		return 0, nil, err
	}
	if !t.Valid {
		return n, nil, nil
	}
	x := t.Time
	return n, &x, nil
}

// GetInaprocDataset: GET /inaproc/dataset. Daftar dataset yang boleh dibuka peran pengguna beserta jumlah barisnya dalam cakupan, untuk halaman Data & Ekspor
// bagi peran yang dibatasi per satker (peran itu tidak memakai /inaproc/penarikan yang memuat keadaan penarikan seluruh data).
func GetInaprocDataset(c *gin.Context) {
	cak := peran.DariGin(c)
	ctx, cancel := context.WithTimeout(c.Request.Context(), dataTimeout)
	defer cancel()

	var boleh []*DatasetPenarikan
	for _, d := range DaftarDataset {
		if d.BolehDilihat(cak) {
			boleh = append(boleh, d)
		}
	}
	type hitungan struct {
		n int64
		t *time.Time
	}
	hasil := make([]hitungan, len(boleh))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, d := range boleh {
		wg.Add(1)
		go func(i int, d *DatasetPenarikan) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			n, t, err := d.ringkas(ctx, database.DB, cak)
			if err != nil {
				log.Println("[INAPROC DATA WARN] gagal menghitung", d.ID+":", err)
				return
			}
			hasil[i] = hitungan{n, t}
		}(i, d)
	}
	wg.Wait()

	datasets := make([]infoDataset, 0, len(boleh))
	for i, d := range boleh {
		datasets = append(datasets, infoDataset{ID: d.ID, Kelompok: d.Kelompok, Subkelompok: d.Subkelompok, Nama: d.Nama, Deskripsi: d.Deskripsi, Mode: d.Mode, Otomatis: d.Otomatis,
			PunyaKLPD: d.KolomKLPD != "", PunyaTahun: d.KolomTahun != "", Baris: hasil[i].n, DisinkronAt: hasil[i].t})
	}
	kelompok := make([]infoKelompok, 0, len(UrutanKelompok))
	for _, k := range UrutanKelompok {
		for _, d := range boleh {
			if d.Kelompok == k {
				kelompok = append(kelompok, infoKelompok{ID: k, Nama: NamaKelompok(k)})
				break
			}
		}
	}
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil daftar dataset", gin.H{
		"kelompok": kelompok, "datasets": datasets, "kode_klpd": kemenkeuKLPDCode, "cakupan": cak,
	})
}

// GetInaprocData: halaman data lokal satu dataset (kolom ringkasan) dengan penyaring kode_klpd, tahun, dan cari.
func GetInaprocData(c *gin.Context) {
	d, ok := datasetDariRute(c)
	if !ok {
		return
	}
	p := penyaringDariQuery(c)
	if pesan := p.Validasi(); pesan != "" {
		utils.ErrorResponse(c, http.StatusBadRequest, pesan)
		return
	}
	halaman, _ := strconv.Atoi(c.DefaultQuery("halaman", "1"))
	perHalaman, _ := strconv.Atoi(c.DefaultQuery("per_halaman", "50"))
	if halaman < 1 {
		halaman = 1
	}
	if perHalaman < 1 || perHalaman > dataHalamanMaks {
		perHalaman = 50
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), dataTimeout)
	defer cancel()
	total, err := d.hitung(ctx, database.DB, p)
	if err != nil {
		log.Println("[INAPROC DATA ERROR] hitung", d.ID+":", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca data")
		return
	}
	src, err := d.bacaBaris(ctx, database.DB, d.KolomRingkas, p, (halaman-1)*perHalaman, perHalaman)
	if err != nil {
		log.Println("[INAPROC DATA ERROR] baca", d.ID+":", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca data")
		return
	}
	defer src.Close()
	kolom := make([]kolomInfo, len(d.KolomRingkas))
	for i, k := range d.KolomRingkas {
		kolom[i] = kolomInfo{Nama: k, Label: laporan.Label(k), Jenis: src.jenisKolom(i)}
	}
	baris := [][]interface{}{}
	for src.Next() {
		b := make([]interface{}, len(src.Nilai()))
		for i, v := range src.Nilai() {
			b[i] = nilaiJSON(v)
		}
		baris = append(baris, b)
	}
	if src.Err() != nil {
		log.Println("[INAPROC DATA ERROR] baca baris", d.ID+":", src.Err())
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca data")
		return
	}

	data := gin.H{"dataset": d.ID, "kolom": kolom, "baris": baris, "total": total, "halaman": halaman, "per_halaman": perHalaman}
	if d.KolomTahun != "" {
		data["tahun_tersedia"] = d.tahunTersedia(ctx, database.DB, p.Cakupan)
	}
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil data", data)
}

// tahunTersedia: tahun yang ada di tabel (dalam cakupan), terbaru dulu (kosong bila gagal dibaca).
func (d *DatasetPenarikan) tahunTersedia(ctx context.Context, db *sql.DB, cak peran.Cakupan) []string {
	batas := ""
	if !cak.SemuaData() {
		kond, _ := kondisiSatkerInaproc(d.Tabel, cak)
		batas = " AND (" + kond + ")"
	}
	rows, err := db.QueryContext(ctx, "SELECT DISTINCT "+kutip(d.KolomTahun)+" FROM "+kutip(d.Tabel)+" WHERE "+kutip(d.KolomTahun)+" IS NOT NULL"+batas+" ORDER BY 1 DESC")
	if err != nil {
		return []string{}
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v interface{}
		if rows.Scan(&v) == nil {
			out = append(out, laporan.Teks(nilaiJSON(v)))
		}
	}
	return out
}

// penulisBerkas menunda pengiriman header sampai ada byte pertama yang ditulis, supaya kegagalan sebelum itu masih bisa dijawab
// dengan galat JSON biasa.
type penulisBerkas struct {
	c     *gin.Context
	tipe  string
	nama  string
	mulai bool
	tulis int64
}

func (p *penulisBerkas) Write(b []byte) (int, error) {
	if !p.mulai {
		p.mulai = true
		h := p.c.Writer.Header()
		h.Set("Content-Type", p.tipe)
		h.Set("Content-Disposition", `attachment; filename="`+p.nama+`"`)
		h.Set("Cache-Control", "no-store")
		h.Set("X-Content-Type-Options", "nosniff")
		p.c.Status(http.StatusOK)
	}
	n, err := p.c.Writer.Write(b)
	p.tulis += int64(n)
	return n, err
}

// namaBerkasEkspor: nama ASCII aman, mis. tender-pengumuman_K10_2025_20261004-0930.xlsx
func namaBerkasEkspor(d *DatasetPenarikan, p PenyaringData, ekstensi string, sekarang time.Time) string {
	bersih := func(s string) string {
		return strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
				return r
			}
			return '-'
		}, s)
	}
	bagian := []string{bersih(strings.ReplaceAll(d.ID, "/", "-"))}
	if d.KolomKLPD != "" && p.KodeKLPD != "" {
		bagian = append(bagian, bersih(p.KodeKLPD))
	}
	if d.KolomTahun != "" && p.Tahun != "" {
		bagian = append(bagian, bersih(p.Tahun))
	}
	bagian = append(bagian, sekarang.In(zonaWIB).Format("20060102-1504"))
	return strings.Join(bagian, "_") + "." + ekstensi
}

// keteranganPenyaring: baris keterangan penyaring untuk PDF.
func keteranganPenyaring(d *DatasetPenarikan, p PenyaringData) []string {
	var s []string
	if d.KolomKLPD != "" && p.KodeKLPD != "" {
		s = append(s, "KLPD: "+p.KodeKLPD)
	}
	if d.KolomTahun != "" && p.Tahun != "" {
		s = append(s, "Tahun: "+p.Tahun)
	}
	if p.Cari != "" {
		s = append(s, "Pencarian: "+p.Cari)
	}
	if len(s) == 0 {
		return []string{"Tanpa penyaring (seluruh data)"}
	}
	return []string{strings.Join(s, "  |  ")}
}

// EksporInaprocData: mengunduh data lokal satu dataset sebagai xlsx, csv, atau pdf. Excel dan CSV memuat semua kolom; PDF memuat kolom
// ringkasan dan dibatasi 5.000 baris. Query: format, kode_klpd, tahun, cari, pemisah (csv: titik-koma | koma | tab).
func EksporInaprocData(c *gin.Context) {
	d, ok := datasetDariRute(c)
	if !ok {
		return
	}
	p := penyaringDariQuery(c)
	if pesan := p.Validasi(); pesan != "" {
		utils.ErrorResponse(c, http.StatusBadRequest, pesan)
		return
	}
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "xlsx")))
	if format != "xlsx" && format != "csv" && format != "pdf" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Format harus xlsx, csv, atau pdf")
		return
	}
	pemisah := ';'
	switch c.DefaultQuery("pemisah", "titik-koma") {
	case "titik-koma":
	case "koma":
		pemisah = ','
	case "tab":
		pemisah = '\t'
	default:
		utils.ErrorResponse(c, http.StatusBadRequest, "Pemisah harus titik-koma, koma, atau tab")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), eksporTimeout)
	defer cancel()
	total, err := d.hitung(ctx, database.DB, p)
	if err != nil {
		log.Println("[INAPROC EKSPOR ERROR] hitung", d.ID+":", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menyiapkan ekspor")
		return
	}
	if format == "xlsx" && total > laporan.MaksBarisExcel {
		utils.ErrorResponse(c, http.StatusUnprocessableEntity,
			fmt.Sprintf("Data (%d baris) melebihi batas satu sheet Excel. Persempit penyaring atau pilih CSV.", total))
		return
	}

	kolom, err := d.kolomHasil(ctx, database.DB, format != "pdf")
	if err != nil {
		log.Println("[INAPROC EKSPOR ERROR] kolom", d.ID+":", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menyiapkan ekspor")
		return
	}
	src, err := d.bacaBaris(ctx, database.DB, kolom, p, 0, 0)
	if err != nil {
		log.Println("[INAPROC EKSPOR ERROR] baca", d.ID+":", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca data")
		return
	}
	defer src.Close()

	now := time.Now()
	opsi := laporan.Opsi{
		Judul:     d.Nama,
		Subjudul:  append(keteranganPenyaring(d, p), "Sumber: Inaproc (data.inaproc.id), disimpan di PASTI V3"),
		NamaSheet: d.Nama,
		Total:     total,
	}
	w := &penulisBerkas{c: c}
	var baris int
	switch format {
	case "csv":
		w.tipe, w.nama = "text/csv; charset=utf-8", namaBerkasEkspor(d, p, "csv", now)
		baris, err = laporan.CSV(w, kolom, src, pemisah)
	case "pdf":
		w.tipe, w.nama = "application/pdf", namaBerkasEkspor(d, p, "pdf", now)
		baris, _, err = laporan.PDF(w, opsi, kolom, src, laporan.MaksBarisPDF)
	default:
		w.tipe, w.nama = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", namaBerkasEkspor(d, p, "xlsx", now)
		baris, err = laporan.XLSX(w, opsi, kolom, src)
	}
	if err != nil {
		log.Printf("[INAPROC EKSPOR ERROR] %s (%s): %v", d.ID, format, err)
		if !w.mulai { // belum ada byte terkirim: masih bisa menjawab dengan galat biasa
			switch {
			case errors.Is(err, laporan.ErrMelebihiBatas):
				utils.ErrorResponse(c, http.StatusUnprocessableEntity, "Data melebihi batas format. Persempit penyaring atau pilih CSV.")
			default:
				utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membuat berkas ekspor")
			}
		}
		return
	}
	log.Printf("[INAPROC EKSPOR] %s diekspor %s (%d baris) oleh %s", d.ID, format, baris, c.GetString("username"))
}
