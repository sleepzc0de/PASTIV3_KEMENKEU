package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/database"
	"pasti-v3-backend/digitalisasi"
	"pasti-v3-backend/laporan"
	"pasti-v3-backend/utils"
)

// Unduh data Digitalisasi Aset (tab Data) sebagai Excel, CSV, atau PDF dengan pencarian dan filter yang sama dengan daftar di layar. Excel dan CSV
// memuat semua kolom tabel; PDF memuat kolom tabel daftar dan dibatasi laporan.MaksBarisPDF baris. Kolom pribadi (Sensitive) hanya ikut bagi admin,
// seperti pada daftar dan detail. Baris dibaca dan ditulis mengalir, jadi tabel besar tidak dimuat seluruhnya ke memori.

// dgFilter menyusun klausa WHERE (diawali " WHERE ", atau kosong), argumennya (@p1, @p2, ...), dan keterangan penyaring yang aktif dari query:
// q, ue1, provinsi, kondisi, jenis_satker, dan tanpa_koordinat. Dipakai daftar dan ekspor supaya keduanya selalu menyaring sama. Pesan tidak
// kosong = permintaan ditolak (400).
func dgFilter(c *gin.Context, ds digitalisasi.Dataset) (whereSQL string, args []interface{}, keterangan []string, pesan string) {
	var where []string
	param := func(v interface{}) string {
		args = append(args, v)
		return fmt.Sprintf("@p%d", len(args))
	}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		if len([]rune(q)) > 100 {
			return "", nil, nil, "Kata kunci terlalu panjang"
		}
		ph := param("%" + escapeLike(q) + "%")
		parts := make([]string, len(ds.SearchColumns))
		for i, col := range ds.SearchColumns {
			parts[i] = qc(col) + " LIKE " + ph + " ESCAPE '\\'"
		}
		where = append(where, "("+strings.Join(parts, " OR ")+")")
		keterangan = append(keterangan, "Pencarian: "+q)
	}
	for _, f := range []struct{ param, col, label string }{
		{"ue1", ds.Roles.UE1, "Unit eselon I"}, {"provinsi", ds.Roles.Provinsi, "Provinsi"}, {"kondisi", ds.Roles.Kondisi, "Kondisi"},
	} {
		if v := strings.TrimSpace(c.Query(f.param)); v != "" && f.col != "" {
			where = append(where, qc(f.col)+" = "+param(v))
			keterangan = append(keterangan, f.label+": "+v)
		}
	}
	if v := strings.TrimSpace(c.Query("jenis_satker")); v != "" && ds.Key == "satker" {
		where = append(where, "[Jenis_Satker] = "+param(v))
		keterangan = append(keterangan, "Jenis satker: "+v)
	}
	if c.Query("tanpa_koordinat") == "1" && ds.Geo {
		where = append(where, "[Latitude] IS NULL")
		keterangan = append(keterangan, "Hanya yang belum punya koordinat")
	}
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}
	return whereSQL, args, keterangan, ""
}

// Label judul kolom di berkas ekspor. Aturannya sama dengan columnLabel di frontend/components/digitalisasi/digitalisasi.ts supaya judul di
// berkas sama dengan yang terlihat di layar; kolom luas dan nilai diberi satuannya.
var dgTokenLabel = map[string]string{
	"KelurahanDesa": "Kelurahan/Desa", "KabKota": "Kab/Kota", "RTRW": "RT/RW", "UE1": "UE1", "KDJ": "KDJ", "KDO": "KDO", "NUP": "NUP",
	"RN": "Rumah Negara", "id": "ID",
}

func labelKolomDigitalisasi(nama string) string {
	switch nama {
	case "Latitude":
		return "Lintang"
	case "Longitude":
		return "Bujur"
	case "synced_at":
		return "Disinkronkan"
	}
	bagian := strings.Split(nama, "_")
	for i, p := range bagian {
		if l, ada := dgTokenLabel[p]; ada {
			bagian[i] = l
		}
	}
	label := strings.Join(bagian, " ")
	if r, n := utf8.DecodeRuneInString(label); n > 0 {
		label = string(unicode.ToUpper(r)) + label[n:]
	}
	switch {
	case strings.HasPrefix(nama, "Luas_"):
		label += " (m²)"
	case strings.HasPrefix(nama, "Nilai_"):
		label += " (Rp)"
	}
	return label
}

// dgKolomPDF: kolom ringkasan untuk PDF (satu halaman lanskap tidak muat puluhan kolom): sama dengan yang tampil di tabel layar, yaitu satker,
// uraian, kab/kota, provinsi, luas, (status penghuni), kondisi, dan nilai menurut peran kolom tiap dataset; satker memakai daftar tetapnya.
// Semuanya kolom biasa (bukan kolom pribadi).
func dgKolomPDF(ds digitalisasi.Dataset) []string {
	if ds.Key == "satker" {
		return []string{"Kode_Satker", "Nama_Satker", "Jenis_Satker", "KabKota_Satker", "Provinsi_Satker", "Jumlah_KDJ", "Jumlah_KDO"}
	}
	var out []string
	for _, n := range []string{ds.Roles.NamaSatker, ds.Roles.Uraian, ds.Roles.KabKota, ds.Roles.Provinsi, ds.Roles.Luas, ds.Roles.StatusPenghuni, ds.Roles.Kondisi, ds.Roles.Nilai} {
		if n != "" {
			out = append(out, n)
		}
	}
	return out
}

// Kolom turunan pada berkas ekspor: singkatan dan uraian UE1 dari referensi UE1 (migrasi 052), disisipkan tepat setelah kolom kode UE1 supaya
// berkas tidak hanya memuat kode. Bukan kolom tabel; nilainya dibaca lewat subquery ke ref_ue1 (kosong bila kodenya belum terdaftar).
const (
	kolomSingkatanUE1 = "Singkatan_UE1"
	kolomUraianUE1    = "Uraian_UE1"
)

// dgEkspresiKolom: ekspresi SELECT untuk satu kolom berkas. Kolom tabel biasa dikutip apa adanya; kolom turunan UE1 berupa subquery yang merujuk
// kolom kode UE1 tabel dataset (tanpa alias, jadi terbaca dari tabel luar).
func dgEkspresiKolom(ds digitalisasi.Dataset, nama string) string {
	switch nama {
	case kolomSingkatanUE1:
		return "(SELECT TOP (1) r.singkatan FROM ref_ue1 r WHERE r.kode = " + qc(ds.Roles.UE1) + ") AS " + qc(nama)
	case kolomUraianUE1:
		return "(SELECT TOP (1) r.nama FROM ref_ue1 r WHERE r.kode = " + qc(ds.Roles.UE1) + ") AS " + qc(nama)
	}
	return qc(nama)
}

// dgKolomEkspor: kolom berkas menurut urutan tabel. ringkas (PDF) = dgKolomPDF; selain itu semua kolom ditambah waktu sinkronisasi, dengan
// singkatan dan uraian UE1 setelah kolom kode UE1. Kolom pribadi hanya untuk admin.
func dgKolomEkspor(ds digitalisasi.Dataset, admin, ringkas bool) (nama []string, koordinat []bool) {
	if ringkas {
		nama = dgKolomPDF(ds)
		return nama, make([]bool, len(nama))
	}
	for _, c := range ds.Columns {
		if c.Sensitive && !admin {
			continue
		}
		nama = append(nama, c.Name)
		koordinat = append(koordinat, c.Kind == digitalisasi.Coord)
		if ds.Roles.UE1 != "" && c.Name == ds.Roles.UE1 {
			nama = append(nama, kolomSingkatanUE1, kolomUraianUE1)
			koordinat = append(koordinat, false, false)
		}
	}
	return append(nama, "synced_at"), append(koordinat, false)
}

// sumberWaktuWIB mengubah nilai waktu pada kolom tertentu dari UTC (synced_at disimpan SYSUTCDATETIME) menjadi waktu dinding WIB, supaya angka di
// berkas sama dengan jam di Indonesia. Hasilnya time.Time berzona UTC yang berisi jam WIB: baik CSV maupun Excel menuliskan jam dindingnya apa
// adanya, tanpa konversi zona lagi.
type sumberWaktuWIB struct {
	*sumberSQL
	kolom []int
}

func (s *sumberWaktuWIB) Nilai() []interface{} {
	v := s.sumberSQL.Nilai()
	for _, i := range s.kolom {
		if t, ok := v[i].(time.Time); ok {
			w := t.In(zonaWIB)
			v[i] = time.Date(w.Year(), w.Month(), w.Day(), w.Hour(), w.Minute(), w.Second(), 0, time.UTC)
		}
	}
	return v
}

// namaBerkasDigitalisasi: nama ASCII aman, mis. digitalisasi-tanah_20261006-1130.xlsx (disaring: digitalisasi-tanah-disaring_...).
func namaBerkasDigitalisasi(kunci, ekstensi string, disaring bool, sekarang time.Time) string {
	bersih := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			return r
		}
		return '-'
	}, kunci)
	nama := "digitalisasi-" + bersih
	if disaring {
		nama += "-disaring"
	}
	return nama + "_" + sekarang.In(zonaWIB).Format("20060102-1504") + "." + ekstensi
}

// EksporDigitalisasiData: GET /digitalisasi/ekspor/:dataset. Query: format (xlsx | csv | pdf; bawaan xlsx), pemisah (csv: titik-koma | koma | tab),
// dan penyaring yang sama dengan daftar (q, ue1, provinsi, kondisi, jenis_satker, tanpa_koordinat).
func EksporDigitalisasiData(c *gin.Context) {
	ds, ok := dgDataset(c)
	if !ok {
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
	whereSQL, args, keterangan, pesan := dgFilter(c, ds)
	if pesan != "" {
		utils.ErrorResponse(c, http.StatusBadRequest, pesan)
		return
	}
	admin := dgIsAdmin(c)

	ctx, cancel := dgKonteks(c, eksporTimeout)
	defer cancel()
	var total int64
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT_BIG(*) FROM "+dgSumber(ctx, ds)+whereSQL, args...).Scan(&total); err != nil {
		log.Println("[DIGITALISASI EKSPOR ERROR] hitung", ds.Key+":", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menyiapkan ekspor")
		return
	}
	if format == "xlsx" && total > laporan.MaksBarisExcel {
		utils.ErrorResponse(c, http.StatusUnprocessableEntity,
			fmt.Sprintf("Data (%d baris) melebihi batas satu sheet Excel. Persempit penyaring atau pilih CSV.", total))
		return
	}

	nama, koordinat := dgKolomEkspor(ds, admin, format == "pdf")
	judul := make([]string, len(nama))
	pilih := make([]string, len(nama))
	formatKolom := map[int]string{}
	var kolomWaktu []int
	for i, n := range nama {
		judul[i] = labelKolomDigitalisasi(n)
		pilih[i] = dgEkspresiKolom(ds, n)
		if koordinat[i] {
			formatKolom[i] = "0.0000000" // lintang/bujur disimpan DECIMAL(10, 7)
		}
		if n == "synced_at" {
			judul[i] = "Disinkronkan (WIB)"
			kolomWaktu = append(kolomWaktu, i)
		}
	}
	urut := "id"
	if ds.Roles.Satker != "" {
		urut = qc(ds.Roles.Satker) + ", id"
	}
	rows, err := database.DB.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s%s ORDER BY %s", strings.Join(pilih, ", "), dgSumber(ctx, ds), whereSQL, urut), args...)
	if err != nil {
		log.Println("[DIGITALISASI EKSPOR ERROR] baca", ds.Key+":", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca data")
		return
	}
	sumber, _ := newSumberSQL(rows, len(nama))
	defer sumber.Close()
	src := &sumberWaktuWIB{sumberSQL: sumber, kolom: kolomWaktu}

	now := time.Now()
	ket := keterangan
	if len(ket) == 0 {
		ket = []string{"Tanpa penyaring (seluruh data)"}
	}
	opsi := laporan.Opsi{
		Judul:       ds.Label,
		Subjudul:    []string{strings.Join(ket, "  |  "), "Sumber: SLDK, disalin ke PASTI V3 lewat sinkronisasi"},
		NamaSheet:   ds.Label,
		Total:       total,
		FormatKolom: formatKolom,
	}
	w := &penulisBerkas{c: c}
	disaring := len(keterangan) > 0
	var baris int
	switch format {
	case "csv":
		w.tipe, w.nama = "text/csv; charset=utf-8", namaBerkasDigitalisasi(ds.Key, "csv", disaring, now)
		baris, err = laporan.CSV(w, judul, src, pemisah)
	case "pdf":
		w.tipe, w.nama = "application/pdf", namaBerkasDigitalisasi(ds.Key, "pdf", disaring, now)
		baris, _, err = laporan.PDF(w, opsi, judul, src, laporan.MaksBarisPDF)
	default:
		w.tipe, w.nama = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", namaBerkasDigitalisasi(ds.Key, "xlsx", disaring, now)
		baris, err = laporan.XLSX(w, opsi, judul, src)
	}
	if err != nil {
		log.Printf("[DIGITALISASI EKSPOR ERROR] %s (%s): %v", ds.Key, format, err)
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
	log.Printf("[DIGITALISASI EKSPOR] %s diekspor %s (%d baris) oleh %s", ds.Key, format, baris, c.GetString("username"))
}
