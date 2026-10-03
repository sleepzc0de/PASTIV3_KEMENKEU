package handlers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Pencarian aset SLDK memakai tabel DJKN.SIMAN2_M_ASET: ~132 juta baris, ~206 GB, 145 kolom. Karena itu:
//   - hanya kolom yang dibutuhkan UI yang diambil (bukan SELECT *);
//   - tidak ada ORDER BY (pada tabel tanpa index itu memaksa pemindaian penuh);
//   - pencarian kode memakai kesamaan persis, teks bebas harus eksplisit dan minimal 3 karakter;
//   - harus ada minimal satu kriteria, jadi tidak ada daftar "semua aset".

// queryError: kesalahan masukan pengguna (400), dibedakan dari kegagalan server.
type queryError string

func (e queryError) Error() string { return string(e) }

const (
	assetDefaultLimit = 50
	assetMaxLimit     = 100
	assetMinTextLen   = 3
	assetMaxTextLen   = 100
)

// assetColumns: kolom M_ASET yang dikirim ke frontend. Data pribadi (mis. nm_penghuni) sengaja tidak ikut.
var assetColumns = []string{
	// identitas & klasifikasi
	"id_aset", "kode_register", "kd_satker", "id_satker", "kd_brg", "ur_sskel", "no_aset", "no_kib", "kd_jns_bmn",
	"jns_aset", "merk", "tipe", "serial_number", "no_polisi", "kuantitas", "intra_extra", "status_bmn_yn", "tercatat",
	// nilai
	"rph_aset", "rph_mutasi", "rph_susut", "rph_buku", "umur_sisa", "kd_dsr_hrg", "cara_perlh", "asl_perlh",
	"kd_sumber_dana", "ur_sumber_dana", "no_dok_perolehan",
	// tanggal
	"tgl_perlh", "tgl_rekam", "tgl_rekam_pertama", "tgl_buku_pertama", "tgl_guna", "tgl_renov", "tgl_hapus",
	// status & kondisi
	"kd_kondisi", "kd_status", "status_pengelolaan", "status_bmn_idle", "kd_jns_idle", "brg_hilang_yn", "brg_rusak_yn",
	"dihentikan_yn", "hapus_lainnya_yn", "rencana_hibah_yn", "kemitraan_yn", "properti_investasi_yn", "status_sbsn",
	"kmk_sbsn", "no_dana", "tgl_dana", "tgl_akhir_sbsn", "status_sanksi",
	// lokasi
	"alamat", "alamat_lain", "vc_alamat_lengkap", "komplek", "kd_rtrw", "ur_kel", "ur_kec", "ur_kab", "ur_prov", "kd_pos",
	"gps_latitude", "gps_longitude", "lokasi_ruang", "negara", "bts_utara", "bts_selatan", "bts_barat", "bts_timur",
	// tanah & bangunan
	"luas", "luas_tapak", "luas_tnhl", "luas_tnhk", "luas_tnhb", "luas_pemanfaatan", "jml_lantai", "jml_bdg", "jml_bidang",
	"bentuk", "peruntukan", "peruntukan_tnh", "topografi_kontur", "topografi_elevasi", "aksesibilitas", "panjang", "lebar",
	"optimalisasi", "kapasitas", "sbsk",
	// dokumen & hukum
	"kd_status_hukum", "no_perkara_hukum", "jns_dok_bukti_kepemilikan", "no_dok_bukti_kepemilikan",
	"stat_dok_bukti_kepemilikan", "tgl_dok_bukti_kepemilikan", "jns_sertifikat", "no_psp", "tgl_psp",
	// pengguna
	"jns_pengguna", "kd_unit_pengguna", "nm_unit_pengguna", "ket_pengguna",
	// lain-lain & kualitas data
	"catatan", "jml_photo", "status_data", "sts_his", "sts_ast", "dq_tgl_invalid_cnt", "_ingestion_date", "updated_at",
}

// assetTextColumns: kolom teks yang boleh dipakai pencarian teks bebas. SLDK_ASSET_SEARCH_COLUMNS di .env
// dipersempit ke himpunan ini supaya nilai di .env tidak bisa menyisipkan kolom sembarang.
var assetTextColumns = map[string]bool{
	"ur_sskel": true, "merk": true, "tipe": true, "alamat": true, "alamat_lain": true, "vc_alamat_lengkap": true,
	"nm_unit_pengguna": true, "ur_kel": true, "ur_kec": true, "ur_kab": true, "ur_prov": true, "catatan": true,
	"serial_number": true, "no_polisi": true, "kode_register": true, "no_kib": true, "kd_brg": true,
}

var defaultAssetTextColumns = []string{"ur_sskel", "merk", "tipe", "alamat", "nm_unit_pengguna"}

// resolveTextColumns memilih kolom pencarian teks: dari konfigurasi bila valid, jika tidak kolom bawaan.
func resolveTextColumns(configured []string) []string {
	var cols []string
	for _, c := range configured {
		c = strings.ToLower(strings.TrimSpace(c))
		if assetTextColumns[c] {
			cols = append(cols, c)
		}
	}
	if len(cols) == 0 {
		return defaultAssetTextColumns
	}
	return cols
}

type assetSearchParams struct {
	Q        string // kata kunci
	By       string // "id" (kode persis, bawaan) atau "teks" (mengandung kata)
	IDSatker string
	JnsBMN   string
	Kondisi  string
	Status   string
	Tahun    string // tahun perolehan
	Limit    string
}

var codePattern = regexp.MustCompile(`^[0-9A-Za-z]{1,8}$`)

// escapeLike menonaktifkan karakter khusus LIKE pada masukan pengguna (dipakai bersama ESCAPE '\').
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`, `[`, `\[`).Replace(s)
}

func assetLimit(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return assetDefaultLimit
	}
	if n > assetMaxLimit {
		return assetMaxLimit
	}
	return n
}

// buildAssetSearch menyusun query pencarian aset. Semua nilai dari pengguna masuk lewat parameter;
// nama kolom hanya berasal dari daftar tetap di atas.
func buildAssetSearch(p assetSearchParams, tableRef string, textCols []string) (string, []interface{}, error) {
	var where []string
	var args []interface{}
	param := func(v interface{}) string {
		args = append(args, v)
		return fmt.Sprintf("@p%d", len(args))
	}

	by := p.By
	if by == "" {
		by = "id"
	}
	if by != "id" && by != "teks" {
		return "", nil, queryError("Jenis pencarian tidak dikenal")
	}

	q := strings.TrimSpace(p.Q)
	if q != "" {
		n := utf8.RuneCountInString(q)
		if n < assetMinTextLen {
			return "", nil, queryError(fmt.Sprintf("Kata kunci minimal %d karakter", assetMinTextLen))
		}
		if n > assetMaxTextLen {
			return "", nil, queryError("Kata kunci terlalu panjang")
		}
		if by == "id" {
			ph := param(q)
			where = append(where, fmt.Sprintf(
				"(%s = %s OR %s = %s OR %s = %s OR %s = %s)",
				quoteIdent("kode_register"), ph, quoteIdent("no_kib"), ph, quoteIdent("no_polisi"), ph, quoteIdent("serial_number"), ph,
			))
		} else {
			ph := param("%" + escapeLike(q) + "%")
			parts := make([]string, 0, len(textCols))
			for _, c := range textCols {
				parts = append(parts, fmt.Sprintf("%s LIKE %s ESCAPE '\\'", quoteIdent(c), ph))
			}
			where = append(where, "("+strings.Join(parts, " OR ")+")")
		}
	}

	if s := strings.TrimSpace(p.IDSatker); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil || id <= 0 {
			return "", nil, queryError("Satker tidak valid")
		}
		where = append(where, quoteIdent("id_satker")+" = "+param(id))
	}
	if s := strings.TrimSpace(p.JnsBMN); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n < 0 {
			return "", nil, queryError("Jenis BMN tidak valid")
		}
		where = append(where, quoteIdent("kd_jns_bmn")+" = "+param(n))
	}
	if s := strings.TrimSpace(p.Kondisi); s != "" {
		if !codePattern.MatchString(s) {
			return "", nil, queryError("Kondisi tidak valid")
		}
		where = append(where, quoteIdent("kd_kondisi")+" = "+param(s))
	}
	if s := strings.TrimSpace(p.Status); s != "" {
		if !codePattern.MatchString(s) {
			return "", nil, queryError("Status penggunaan tidak valid")
		}
		where = append(where, quoteIdent("kd_status")+" = "+param(s))
	}
	if s := strings.TrimSpace(p.Tahun); s != "" {
		y, err := strconv.Atoi(s)
		if err != nil || y < 1900 || y > 2100 {
			return "", nil, queryError("Tahun perolehan tidak valid")
		}
		from := time.Date(y, time.January, 1, 0, 0, 0, 0, time.UTC)
		to := from.AddDate(1, 0, 0)
		where = append(where, fmt.Sprintf("%s >= %s AND %s < %s", quoteIdent("tgl_perlh"), param(from), quoteIdent("tgl_perlh"), param(to)))
	}

	if len(where) == 0 {
		return "", nil, queryError("Isi kata kunci atau pilih minimal satu filter")
	}

	cols := make([]string, len(assetColumns))
	for i, c := range assetColumns {
		cols[i] = quoteIdent(c)
	}
	query := fmt.Sprintf("SELECT TOP (%d) %s FROM %s WHERE %s", assetLimit(p.Limit), strings.Join(cols, ", "), tableRef, strings.Join(where, " AND "))
	return query, args, nil
}
