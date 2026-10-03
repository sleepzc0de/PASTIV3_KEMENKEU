package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/database"
	"pasti-v3-backend/digitalisasi"
	"pasti-v3-backend/utils"
)

// Halaman Digitalisasi Aset membaca tabel DIGITALISASI_* di database PASTI (diisi lewat sinkronisasi dari SLDK,
// lihat digitalisasi_sync.go). Semua nama tabel/kolom di sini berasal dari daftar tetap digitalisasi.Datasets;
// masukan pengguna hanya masuk sebagai parameter.

const (
	dgTimeout   = 20 * time.Second
	dgMapMax    = 100000 // titik per dataset pada peta
	dgPageMax   = 100
	dgOptionMax = 200
)

// Kotak batas Indonesia (termasuk Pulau Rote di selatan dan Miangas di utara). Koordinat di luar kotak ini hampir
// pasti salah isi (lintang/bujur tertukar atau tanda minus hilang) dan tidak digambar di peta.
const (
	idLatMin, idLatMax = -11.5, 6.5
	idLngMin, idLngMax = 94.5, 141.5
)

func inIndonesiaSQL(lat, lng string) string {
	return fmt.Sprintf("%s BETWEEN %v AND %v AND %s BETWEEN %v AND %v", lat, idLatMin, idLatMax, lng, idLngMin, idLngMax)
}

// Nama UE1 yang diketahui pasti (dari komentar pada query satker). Kode lain ditampilkan sebagai "UE1 <kode>".
var ue1Names = map[string]string{"01504": "DJP", "01505": "DJBC", "01508": "DJPb", "01515": "BATII"}

func ue1Label(kode string) string {
	if kode == "" || kode == "(kosong)" {
		return "(kosong)"
	}
	if n, ok := ue1Names[kode]; ok {
		return kode + " · " + n
	}
	return "UE1 " + kode
}

func dgIsAdmin(c *gin.Context) bool {
	role := c.GetString("role")
	return role == "admin" || role == "superadmin"
}

func dgFail(c *gin.Context, what string, err error) {
	log.Println("[DIGITALISASI ERROR]", what+":", err)
	utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca data digitalisasi aset")
}

func dgDataset(c *gin.Context) (digitalisasi.Dataset, bool) {
	ds, ok := digitalisasi.ByKey(c.Param("dataset"))
	if !ok {
		utils.ErrorResponse(c, http.StatusNotFound, "Dataset tidak dikenal")
	}
	return ds, ok
}

func qc(name string) string { return "[" + strings.ReplaceAll(name, "]", "]]") + "]" }

// assetDatasets: dataset selain satker (aset fisik yang punya luas/nilai/kondisi).
func assetDatasets() []digitalisasi.Dataset {
	var out []digitalisasi.Dataset
	for _, ds := range digitalisasi.Datasets {
		if ds.Key != "satker" {
			out = append(out, ds)
		}
	}
	return out
}

func sumDec(col string) string {
	if col == "" {
		return "CAST(0 AS DECIMAL(38, 4))"
	}
	return "CAST(ISNULL(SUM(" + qc(col) + "), 0) AS DECIMAL(38, 4))"
}

func nullCount(col string) string {
	if col == "" {
		return "0"
	}
	return "ISNULL(SUM(CASE WHEN " + qc(col) + " IS NULL THEN 1 ELSE 0 END), 0)"
}

// ---------------------------------------------------------------- jumlah baris dan riwayat

func dgTableCounts(ctx context.Context) (map[string]int64, error) {
	parts := make([]string, len(digitalisasi.Datasets))
	for i, ds := range digitalisasi.Datasets {
		parts[i] = fmt.Sprintf("SELECT '%s', COUNT(*) FROM %s", ds.Key, qc(ds.Table))
	}
	rows, err := database.DB.QueryContext(ctx, strings.Join(parts, " UNION ALL "))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var k string
		var n int64
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- GET /digitalisasi/ringkasan

type dgAgg struct {
	Jumlah int64   `json:"jumlah"`
	Luas   float64 `json:"luas"`
	Nilai  float64 `json:"nilai"`
}

type dgAssetStat struct {
	Key            string  `json:"key"`
	Label          string  `json:"label"`
	Geo            bool    `json:"geo"`
	PunyaLuas      bool    `json:"punya_luas"`
	PunyaNilai     bool    `json:"punya_nilai"`
	Jumlah         int64   `json:"jumlah"`
	Luas           float64 `json:"luas"`
	Nilai          float64 `json:"nilai"`
	Bertitik       int64   `json:"bertitik"`
	TanpaKoordinat int64   `json:"tanpa_koordinat"`
	DiLuar         int64   `json:"di_luar_indonesia"`
	TanpaFoto      int64   `json:"tanpa_foto"`
	TanpaKondisi   int64   `json:"tanpa_kondisi"`
}

type dgUE1 struct {
	Kode   string           `json:"kode"`
	Label  string           `json:"label"`
	Satker int64            `json:"satker"`
	KDJ    int64            `json:"kdj"`
	KDO    int64            `json:"kdo"`
	Per    map[string]dgAgg `json:"per"`
}

type dgProvinsi struct {
	Nama string           `json:"nama"`
	Per  map[string]dgAgg `json:"per"`
}

type dgCount struct {
	K      *string `json:"k"`
	Jumlah int64   `json:"jumlah"`
	Nilai  float64 `json:"nilai"`
}

func dgAssetStats(ctx context.Context) ([]dgAssetStat, error) {
	var out []dgAssetStat
	for _, ds := range assetDatasets() {
		r := ds.Roles
		sel := []string{"COUNT(*)", sumDec(r.Luas), sumDec(r.Nilai)}
		if ds.Geo {
			sel = append(sel,
				"ISNULL(SUM(CASE WHEN "+inIndonesiaSQL("[Latitude]", "[Longitude]")+" THEN 1 ELSE 0 END), 0)",
				nullCount("Latitude"))
		} else {
			sel = append(sel, "0", "0")
		}
		sel = append(sel, nullCount(r.Foto), nullCount(r.Kondisi))

		var s dgAssetStat
		var luas, nilai sql.NullString
		err := database.DB.QueryRowContext(ctx, "SELECT "+strings.Join(sel, ", ")+" FROM "+qc(ds.Table)).
			Scan(&s.Jumlah, &luas, &nilai, &s.Bertitik, &s.TanpaKoordinat, &s.TanpaFoto, &s.TanpaKondisi)
		if err != nil {
			return nil, fmt.Errorf("statistik %s: %w", ds.Key, err)
		}
		s.Key, s.Label, s.Geo = ds.Key, ds.Label, ds.Geo
		s.PunyaLuas, s.PunyaNilai = r.Luas != "", r.Nilai != ""
		s.Luas, s.Nilai = scanMoney(luas), scanMoney(nilai)
		if ds.Geo {
			s.DiLuar = s.Jumlah - s.Bertitik - s.TanpaKoordinat
		}
		out = append(out, s)
	}
	return out, nil
}

// dgBreakdown menjumlahkan jumlah/luas/nilai per kunci pengelompokan (UE1 atau provinsi) untuk semua dataset aset.
func dgBreakdown(ctx context.Context, keyExpr func(ds digitalisasi.Dataset) string) (map[string]map[string]dgAgg, error) {
	var parts []string
	for _, ds := range assetDatasets() {
		r := ds.Roles
		k := keyExpr(ds)
		parts = append(parts, fmt.Sprintf("SELECT '%s' AS ds, %s AS k, COUNT(*) AS n, %s AS luas, %s AS nilai FROM %s GROUP BY %s",
			ds.Key, k, sumDec(r.Luas), sumDec(r.Nilai), qc(ds.Table), k))
	}
	rows, err := database.DB.QueryContext(ctx, strings.Join(parts, " UNION ALL "))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]map[string]dgAgg{}
	for rows.Next() {
		var dsKey, k string
		var n int64
		var luas, nilai sql.NullString
		if err := rows.Scan(&dsKey, &k, &n, &luas, &nilai); err != nil {
			return nil, err
		}
		if out[k] == nil {
			out[k] = map[string]dgAgg{}
		}
		out[k][dsKey] = dgAgg{Jumlah: n, Luas: scanMoney(luas), Nilai: scanMoney(nilai)}
	}
	return out, rows.Err()
}

func dgGroupCounts(ctx context.Context, table, col, valueCol string, top int) ([]dgCount, error) {
	nilai := "CAST(0 AS DECIMAL(38, 2))"
	if valueCol != "" {
		nilai = "CAST(ISNULL(SUM(" + qc(valueCol) + "), 0) AS DECIMAL(38, 2))"
	}
	topSQL := ""
	if top > 0 {
		topSQL = fmt.Sprintf("TOP (%d) ", top)
	}
	// Teks status hukum bisa panjang (gabungan beberapa status): dikelompokkan menurut 300 karakter pertama.
	key := "CAST(" + qc(col) + " AS NVARCHAR(300))"
	rows, err := database.DB.QueryContext(ctx, fmt.Sprintf(
		"SELECT %s%s AS k, COUNT(*) AS n, %s FROM %s GROUP BY %s ORDER BY COUNT(*) DESC", topSQL, key, nilai, qc(table), key))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []dgCount{}
	for rows.Next() {
		var k sql.NullString
		var n int64
		var nilai sql.NullString
		if err := rows.Scan(&k, &n, &nilai); err != nil {
			return nil, err
		}
		out = append(out, dgCount{K: optStr(k), Jumlah: n, Nilai: scanMoney(nilai)})
	}
	return out, rows.Err()
}

func dgScalar(ctx context.Context, q string) (int64, error) {
	var n sql.NullInt64
	if err := database.DB.QueryRowContext(ctx, q).Scan(&n); err != nil {
		return 0, err
	}
	return n.Int64, nil
}

// GetDigitalisasiRingkasan: data analitik untuk tab Ringkasan.
func GetDigitalisasiRingkasan(c *gin.Context) {
	if digitalisasi.Default == nil {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Fitur digitalisasi aset belum siap")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), dgTimeout)
	defer cancel()

	counts, err := dgTableCounts(ctx)
	if err != nil {
		dgFail(c, "jumlah baris", err)
		return
	}
	var anyData bool
	for _, n := range counts {
		if n > 0 {
			anyData = true
		}
	}
	if !anyData {
		utils.SuccessResponse(c, http.StatusOK, "Belum ada data digitalisasi aset", gin.H{"tersedia": false})
		return
	}

	stats, err := dgAssetStats(ctx)
	if err != nil {
		dgFail(c, "statistik aset", err)
		return
	}

	// Satker dan kendaraan.
	var induk, anak, kdj, kdo int64
	{
		var a, b, k1, k2 sql.NullInt64
		err := database.DB.QueryRowContext(ctx, `SELECT
			SUM(CASE WHEN Jenis_Satker = N'INDUK SATKER' THEN 1 ELSE 0 END),
			SUM(CASE WHEN Jenis_Satker = N'ANAK SATKER' THEN 1 ELSE 0 END),
			SUM(Jumlah_KDJ), SUM(Jumlah_KDO) FROM DIGITALISASI_SATKER`).Scan(&a, &b, &k1, &k2)
		if err != nil {
			dgFail(c, "satker", err)
			return
		}
		induk, anak, kdj, kdo = a.Int64, b.Int64, k1.Int64, k2.Int64
	}

	// Rincian per UE1.
	ue1Rows, err := dgBreakdown(ctx, func(ds digitalisasi.Dataset) string {
		return "ISNULL(" + qc(ds.Roles.UE1) + ", N'(kosong)')"
	})
	if err != nil {
		dgFail(c, "per UE1", err)
		return
	}
	ue1 := map[string]*dgUE1{}
	for kode, per := range ue1Rows {
		ue1[kode] = &dgUE1{Kode: kode, Label: ue1Label(kode), Per: per}
	}
	{
		rows, err := database.DB.QueryContext(ctx, `SELECT ISNULL(Kode_UE1, N'(kosong)'), COUNT(*), ISNULL(SUM(Jumlah_KDJ), 0), ISNULL(SUM(Jumlah_KDO), 0)
			FROM DIGITALISASI_SATKER GROUP BY ISNULL(Kode_UE1, N'(kosong)')`)
		if err != nil {
			dgFail(c, "satker per UE1", err)
			return
		}
		for rows.Next() {
			var kode string
			var n, a, b int64
			if err := rows.Scan(&kode, &n, &a, &b); err != nil {
				rows.Close()
				dgFail(c, "satker per UE1", err)
				return
			}
			u := ue1[kode]
			if u == nil {
				u = &dgUE1{Kode: kode, Label: ue1Label(kode), Per: map[string]dgAgg{}}
				ue1[kode] = u
			}
			u.Satker, u.KDJ, u.KDO = n, a, b
		}
		rows.Close()
	}
	ue1List := make([]dgUE1, 0, len(ue1))
	for _, u := range ue1 {
		ue1List = append(ue1List, *u)
	}
	sort.Slice(ue1List, func(i, j int) bool {
		if ue1List[i].Satker != ue1List[j].Satker {
			return ue1List[i].Satker > ue1List[j].Satker
		}
		return ue1List[i].Kode < ue1List[j].Kode
	})

	// Rincian per provinsi.
	provRows, err := dgBreakdown(ctx, func(ds digitalisasi.Dataset) string {
		return "ISNULL(NULLIF(UPPER(LTRIM(RTRIM(" + qc(ds.Roles.Provinsi) + "))), N''), N'(kosong)')"
	})
	if err != nil {
		dgFail(c, "per provinsi", err)
		return
	}
	provList := make([]dgProvinsi, 0, len(provRows))
	for nama, per := range provRows {
		provList = append(provList, dgProvinsi{Nama: nama, Per: per})
	}
	totalOf := func(p dgProvinsi) int64 {
		var t int64
		for _, a := range p.Per {
			t += a.Jumlah
		}
		return t
	}
	sort.Slice(provList, func(i, j int) bool {
		a, b := totalOf(provList[i]), totalOf(provList[j])
		if a != b {
			return a > b
		}
		return provList[i].Nama < provList[j].Nama
	})

	// Kondisi, status hukum, asuransi, status penghuni.
	kondisi := map[string][]dgCount{}
	statusHukum := map[string][]dgCount{}
	asuransi := map[string][]dgCount{}
	for _, ds := range assetDatasets() {
		r := ds.Roles
		if r.Kondisi != "" {
			if kondisi[ds.Key], err = dgGroupCounts(ctx, ds.Table, r.Kondisi, r.Nilai, 0); err != nil {
				dgFail(c, "kondisi "+ds.Key, err)
				return
			}
		}
		if r.StatusHukum != "" {
			if statusHukum[ds.Key], err = dgGroupCounts(ctx, ds.Table, r.StatusHukum, "", 10); err != nil {
				dgFail(c, "status hukum "+ds.Key, err)
				return
			}
		}
		if r.Asuransi != "" {
			if asuransi[ds.Key], err = dgGroupCounts(ctx, ds.Table, r.Asuransi, "", 0); err != nil {
				dgFail(c, "asuransi "+ds.Key, err)
				return
			}
		}
	}
	rn, _ := digitalisasi.ByKey("rumah_negara")
	statusPenghuni, err := dgGroupCounts(ctx, rn.Table, rn.Roles.StatusPenghuni, "", 0)
	if err != nil {
		dgFail(c, "status penghuni", err)
		return
	}

	// Hunian (kamar) dan tanah-bangunan.
	sumInt := func(table, col string) (int64, error) {
		return dgScalar(ctx, fmt.Sprintf("SELECT ISNULL(SUM(%s), 0) FROM %s", qc(col), qc(table)))
	}
	hunian := gin.H{}
	for _, spec := range []struct{ key, ds, col string }{
		{"rusun_kamar_e", "rusunara", "Kamar_tipe_E"}, {"rusun_kamar_d", "rusunara", "Kamar_tipe_D"}, {"rusun_kamar_c", "rusunara", "Kamar_tipe_C"},
		{"mess_kamar_e", "mess_rumah_negara", "Kamar_tipe_E"}, {"mess_kamar_d", "mess_rumah_negara", "Kamar_tipe_D"},
		{"tanah_jumlah_bangunan", "tanah", "Jumlah_Bangunan"},
	} {
		d, _ := digitalisasi.ByKey(spec.ds)
		n, err := sumInt(d.Table, spec.col)
		if err != nil {
			dgFail(c, "hunian "+spec.key, err)
			return
		}
		hunian[spec.key] = n
	}

	// Kelengkapan data tingkat satker.
	kelengkapan := gin.H{}
	for key, q := range map[string]string{
		"satker_induk": `SELECT COUNT(*) FROM DIGITALISASI_SATKER WHERE Jenis_Satker = N'INDUK SATKER'`,
		"induk_tanpa_kantor_utama": `SELECT COUNT(*) FROM DIGITALISASI_SATKER s WHERE s.Jenis_Satker = N'INDUK SATKER'
			AND NOT EXISTS (SELECT 1 FROM DIGITALISASI_GEDUNG_KANTOR_UTAMA g WHERE g.Kode_Satker = s.Kode_Satker)`,
		"induk_tanpa_tanah": `SELECT COUNT(*) FROM DIGITALISASI_SATKER s WHERE s.Jenis_Satker = N'INDUK SATKER'
			AND NOT EXISTS (SELECT 1 FROM DIGITALISASI_TANAH t WHERE t.Kode_Satker = s.Kode_Satker)`,
	} {
		n, err := dgScalar(ctx, q)
		if err != nil {
			dgFail(c, "kelengkapan "+key, err)
			return
		}
		kelengkapan[key] = n
	}

	_, lastOK, err := digitalisasi.Default.LatestPerDataset(ctx)
	if err != nil {
		dgFail(c, "riwayat", err)
		return
	}
	sinkron := make([]gin.H, 0, len(digitalisasi.Datasets))
	for _, ds := range digitalisasi.Datasets {
		var ok interface{}
		if e, found := lastOK[ds.Key]; found {
			ok = e.Selesai
		}
		sinkron = append(sinkron, gin.H{"dataset": ds.Key, "label": ds.Label, "jumlah_baris": counts[ds.Key], "terakhir_sukses": ok})
	}

	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil ringkasan digitalisasi aset", gin.H{
		"tersedia":        true,
		"sinkron":         sinkron,
		"satker":          gin.H{"total": induk + anak, "induk": induk, "anak": anak, "kdj": kdj, "kdo": kdo},
		"aset":            stats,
		"hunian":          hunian,
		"ue1":             ue1List,
		"provinsi":        provList,
		"kondisi":         kondisi,
		"status_hukum":    statusHukum,
		"asuransi":        asuransi,
		"status_penghuni": statusPenghuni,
		"kelengkapan":     kelengkapan,
	})
}

// ---------------------------------------------------------------- GET /digitalisasi/peta

type dgMapSet struct {
	Key            string       `json:"key"`
	Label          string       `json:"label"`
	Total          int64        `json:"total"`
	Bertitik       int64        `json:"bertitik"`
	TanpaKoordinat int64        `json:"tanpa_koordinat"`
	DiLuar         int64        `json:"di_luar_indonesia"`
	Terpotong      bool         `json:"terpotong"`
	Titik          [][3]float64 `json:"titik"` // [id, lintang, bujur]
}

func round6(f float64) float64 { return math.Round(f*1e6) / 1e6 }

// GetDigitalisasiPeta: titik koordinat per dataset dalam bentuk ringkas. Rincian satu titik diambil terpisah
// saat diklik (GetDigitalisasiDetail).
func GetDigitalisasiPeta(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), dgTimeout)
	defer cancel()

	want := map[string]bool{}
	if v := strings.TrimSpace(c.Query("dataset")); v != "" {
		for _, k := range strings.Split(v, ",") {
			want[strings.TrimSpace(k)] = true
		}
	}
	ue1 := strings.TrimSpace(c.Query("ue1"))

	sets := []dgMapSet{}
	for _, ds := range digitalisasi.Datasets {
		if !ds.Geo || (len(want) > 0 && !want[ds.Key]) {
			continue
		}
		where, args := "", []interface{}{}
		if ue1 != "" {
			where, args = " WHERE "+qc(ds.Roles.UE1)+" = @p1", append(args, ue1)
		}
		set := dgMapSet{Key: ds.Key, Label: ds.Label, Titik: [][3]float64{}}

		var total, inside, none sql.NullInt64
		err := database.DB.QueryRowContext(ctx, fmt.Sprintf(
			`SELECT COUNT(*), SUM(CASE WHEN %s THEN 1 ELSE 0 END), SUM(CASE WHEN Latitude IS NULL THEN 1 ELSE 0 END) FROM %s%s`,
			inIndonesiaSQL("[Latitude]", "[Longitude]"), qc(ds.Table), where), args...).Scan(&total, &inside, &none)
		if err != nil {
			dgFail(c, "hitung titik "+ds.Key, err)
			return
		}
		set.Total, set.Bertitik, set.TanpaKoordinat = total.Int64, inside.Int64, none.Int64
		set.DiLuar = set.Total - set.Bertitik - set.TanpaKoordinat

		cond := inIndonesiaSQL("[Latitude]", "[Longitude]")
		if where != "" {
			cond += " AND " + qc(ds.Roles.UE1) + " = @p1"
		}
		rows, err := database.DB.QueryContext(ctx, fmt.Sprintf(
			"SELECT TOP (%d) id, Latitude, Longitude FROM %s WHERE %s", dgMapMax+1, qc(ds.Table), cond), args...)
		if err != nil {
			dgFail(c, "titik "+ds.Key, err)
			return
		}
		for rows.Next() {
			var id int64
			var lat, lng float64
			if err := rows.Scan(&id, &lat, &lng); err != nil {
				rows.Close()
				dgFail(c, "titik "+ds.Key, err)
				return
			}
			if len(set.Titik) >= dgMapMax {
				set.Terpotong = true
				break
			}
			set.Titik = append(set.Titik, [3]float64{float64(id), round6(lat), round6(lng)})
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			dgFail(c, "titik "+ds.Key, err)
			return
		}
		sets = append(sets, set)
	}
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil titik peta", gin.H{"datasets": sets})
}

// ---------------------------------------------------------------- GET /digitalisasi/data/:dataset

type dgColumnInfo struct {
	Nama     string `json:"nama"`
	Tipe     string `json:"tipe"` // teks | bilangan | desimal
	Sensitif bool   `json:"sensitif,omitempty"`
}

func columnKind(c digitalisasi.Column) string {
	switch c.Kind {
	case digitalisasi.Int, digitalisasi.BigInt:
		return "bilangan"
	case digitalisasi.Decimal, digitalisasi.Coord:
		return "desimal"
	}
	return "teks"
}

// normalizeRow mengubah kolom desimal (yang datang sebagai teks dari driver) menjadi angka JSON.
func normalizeRow(ds digitalisasi.Dataset, row map[string]interface{}) {
	for _, col := range ds.Columns {
		if col.Kind != digitalisasi.Decimal && col.Kind != digitalisasi.Coord {
			continue
		}
		if s, ok := row[col.Name].(string); ok {
			if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
				row[col.Name] = f
			}
		}
	}
}

func dgDistinct(ctx context.Context, table, col string) ([]string, error) {
	rows, err := database.DB.QueryContext(ctx, fmt.Sprintf(
		"SELECT DISTINCT TOP (%d) %s FROM %s WHERE %s IS NOT NULL ORDER BY %s", dgOptionMax, qc(col), qc(table), qc(col), qc(col)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListDigitalisasiData: daftar baris satu dataset dengan pencarian, filter, dan halaman.
func ListDigitalisasiData(c *gin.Context) {
	ds, ok := dgDataset(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), dgTimeout)
	defer cancel()
	admin := dgIsAdmin(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	per, _ := strconv.Atoi(c.DefaultQuery("per_page", "25"))
	if per < 1 {
		per = 25
	}
	if per > dgPageMax {
		per = dgPageMax
	}

	var where []string
	var args []interface{}
	param := func(v interface{}) string {
		args = append(args, v)
		return fmt.Sprintf("@p%d", len(args))
	}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		if len([]rune(q)) > 100 {
			utils.ErrorResponse(c, http.StatusBadRequest, "Kata kunci terlalu panjang")
			return
		}
		ph := param("%" + escapeLike(q) + "%")
		parts := make([]string, len(ds.SearchColumns))
		for i, col := range ds.SearchColumns {
			parts[i] = qc(col) + " LIKE " + ph + " ESCAPE '\\'"
		}
		where = append(where, "("+strings.Join(parts, " OR ")+")")
	}
	for _, f := range []struct{ param, col string }{
		{"ue1", ds.Roles.UE1}, {"provinsi", ds.Roles.Provinsi}, {"kondisi", ds.Roles.Kondisi},
	} {
		if v := strings.TrimSpace(c.Query(f.param)); v != "" && f.col != "" {
			where = append(where, qc(f.col)+" = "+param(v))
		}
	}
	if v := strings.TrimSpace(c.Query("jenis_satker")); v != "" && ds.Key == "satker" {
		where = append(where, "[Jenis_Satker] = "+param(v))
	}
	if c.Query("tanpa_koordinat") == "1" && ds.Geo {
		where = append(where, "[Latitude] IS NULL")
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+qc(ds.Table)+whereSQL, args...).Scan(&total); err != nil {
		dgFail(c, "hitung "+ds.Key, err)
		return
	}

	cols := ds.DisplayColumns(admin)
	sel := []string{"id"}
	infos := make([]dgColumnInfo, 0, len(cols))
	for _, col := range cols {
		sel = append(sel, qc(col.Name))
		infos = append(infos, dgColumnInfo{Nama: col.Name, Tipe: columnKind(col), Sensitif: col.Sensitive})
	}
	if ds.Geo {
		sel = append(sel, "[Latitude]", "[Longitude]")
	}
	order := "id"
	if ds.Roles.Satker != "" {
		order = qc(ds.Roles.Satker) + ", id"
	}
	offset, limit := param((page-1)*per), param(per)
	rows, err := database.DB.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s%s ORDER BY %s OFFSET %s ROWS FETCH NEXT %s ROWS ONLY",
		strings.Join(sel, ", "), qc(ds.Table), whereSQL, order, offset, limit), args...)
	if err != nil {
		dgFail(c, "daftar "+ds.Key, err)
		return
	}
	defer rows.Close()
	list, err := rowsToMaps(rows)
	if err != nil {
		dgFail(c, "baca daftar "+ds.Key, err)
		return
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	for _, row := range list {
		normalizeRow(ds, row)
	}

	filters := gin.H{}
	for name, col := range map[string]string{"ue1": ds.Roles.UE1, "provinsi": ds.Roles.Provinsi, "kondisi": ds.Roles.Kondisi} {
		if col == "" {
			continue
		}
		opts, err := dgDistinct(ctx, ds.Table, col)
		if err != nil {
			dgFail(c, "opsi filter "+name, err)
			return
		}
		filters[name] = opts
	}

	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil data", gin.H{
		"dataset": ds.Key, "label": ds.Label, "geo": ds.Geo,
		"kolom": infos, "rows": list, "total": total, "page": page, "per_page": per, "filter": filters,
	})
}

// GetDigitalisasiDetail: seluruh kolom satu baris (kecuali kolom pribadi bagi non-admin).
func GetDigitalisasiDetail(c *gin.Context) {
	ds, ok := dgDataset(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.ErrorResponse(c, http.StatusBadRequest, "ID tidak valid")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), dgTimeout)
	defer cancel()
	admin := dgIsAdmin(c)

	sel := []string{"id"}
	var infos []dgColumnInfo
	for _, col := range ds.Columns {
		if col.Sensitive && !admin {
			continue
		}
		sel = append(sel, qc(col.Name))
		infos = append(infos, dgColumnInfo{Nama: col.Name, Tipe: columnKind(col), Sensitif: col.Sensitive})
	}
	sel = append(sel, "[synced_at]")
	rows, err := database.DB.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE id = @p1", strings.Join(sel, ", "), qc(ds.Table)), id)
	if err != nil {
		dgFail(c, "detail "+ds.Key, err)
		return
	}
	defer rows.Close()
	list, err := rowsToMaps(rows)
	if err != nil {
		dgFail(c, "baca detail "+ds.Key, err)
		return
	}
	if len(list) == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "Data tidak ditemukan")
		return
	}
	normalizeRow(ds, list[0])
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil detail", gin.H{
		"dataset": ds.Key, "label": ds.Label, "kolom": infos, "row": list[0],
	})
}
