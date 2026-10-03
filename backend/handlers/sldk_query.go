package handlers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"pasti-v3-backend/sldk"
)

// Pencarian aset SLDK memakai tabel DJKN.SIMAN2_M_ASET: ~132 juta baris, ~206 GB, 145 kolom. Karena itu:
//   - hanya kolom yang dibutuhkan UI yang diambil (sldk.AssetColumns, bukan SELECT *);
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

// resolveTextColumns memilih kolom pencarian teks: dari konfigurasi bila valid, jika tidak kolom bawaan.
func resolveTextColumns(configured []string) []string {
	var cols []string
	for _, c := range configured {
		c = strings.ToLower(strings.TrimSpace(c))
		if sldk.TextColumns[c] {
			cols = append(cols, c)
		}
	}
	if len(cols) == 0 {
		return sldk.DefaultTextColumns
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
	Anomali  string // kunci aturan pemantauan (sldk.Rules), mis. "idle"
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

// buildAssetSearch menyusun query pencarian aset. Semua nilai dari pengguna masuk lewat parameter; nama
// kolom hanya berasal dari daftar tetap di paket sldk. assetTable adalah nama mentah dari konfigurasi
// (mis. DJKN.SIMAN2_M_ASET). scopeKL (dari konfigurasi, bukan dari pengguna) membatasi hasil ke satu K/L
// bila tidak kosong.
func buildAssetSearch(p assetSearchParams, assetTable string, textCols []string, scopeKL string) (string, []interface{}, error) {
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
				sldk.QuoteIdent("kode_register"), ph, sldk.QuoteIdent("no_kib"), ph, sldk.QuoteIdent("no_polisi"), ph, sldk.QuoteIdent("serial_number"), ph,
			))
		} else {
			ph := param("%" + escapeLike(q) + "%")
			parts := make([]string, 0, len(textCols))
			for _, c := range textCols {
				parts = append(parts, fmt.Sprintf("%s LIKE %s ESCAPE '\\'", sldk.QuoteIdent(c), ph))
			}
			where = append(where, "("+strings.Join(parts, " OR ")+")")
		}
	}

	if s := strings.TrimSpace(p.IDSatker); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil || id <= 0 {
			return "", nil, queryError("Satker tidak valid")
		}
		where = append(where, sldk.QuoteIdent("id_satker")+" = "+param(id))
	}
	if s := strings.TrimSpace(p.JnsBMN); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n < 0 {
			return "", nil, queryError("Jenis BMN tidak valid")
		}
		where = append(where, sldk.QuoteIdent("kd_jns_bmn")+" = "+param(n))
	}
	if s := strings.TrimSpace(p.Kondisi); s != "" {
		if !codePattern.MatchString(s) {
			return "", nil, queryError("Kondisi tidak valid")
		}
		where = append(where, sldk.QuoteIdent("kd_kondisi")+" = "+param(s))
	}
	if s := strings.TrimSpace(p.Status); s != "" {
		if !codePattern.MatchString(s) {
			return "", nil, queryError("Status penggunaan tidak valid")
		}
		where = append(where, sldk.QuoteIdent("kd_status")+" = "+param(s))
	}
	if s := strings.TrimSpace(p.Tahun); s != "" {
		y, err := strconv.Atoi(s)
		if err != nil || y < 1900 || y > 2100 {
			return "", nil, queryError("Tahun perolehan tidak valid")
		}
		from := time.Date(y, time.January, 1, 0, 0, 0, 0, time.UTC)
		to := from.AddDate(1, 0, 0)
		where = append(where, fmt.Sprintf("%s >= %s AND %s < %s", sldk.QuoteIdent("tgl_perlh"), param(from), sldk.QuoteIdent("tgl_perlh"), param(to)))
	}
	if s := strings.TrimSpace(p.Anomali); s != "" {
		rule, ok := sldk.RuleByKey(s)
		if !ok {
			return "", nil, queryError("Penanda pemantauan tidak dikenal")
		}
		where = append(where, "("+rule.Predicate+")")
	}

	// Kriteria dari pengguna wajib ada; cakupan K/L saja tidak cukup (itu hanya pembatas).
	if len(where) == 0 {
		return "", nil, queryError("Isi kata kunci atau pilih minimal satu filter")
	}
	if scopeKL != "" {
		where = append(where, sldk.ScopePredicate(assetTable, param(scopeKL)))
	}

	cols := make([]string, len(sldk.AssetColumns))
	for i, c := range sldk.AssetColumns {
		cols[i] = sldk.QuoteIdent(c)
	}
	query := fmt.Sprintf("SELECT TOP (%d) %s FROM %s WHERE %s", assetLimit(p.Limit), strings.Join(cols, ", "), sldk.QuoteTableRef(assetTable), strings.Join(where, " AND "))
	return query, args, nil
}
