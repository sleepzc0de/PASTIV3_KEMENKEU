package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/database"
	"pasti-v3-backend/sldk"
	"pasti-v3-backend/utils"
)

// Halaman Ringkasan dan Pemantauan membaca tabel sldk_agregat di database PASTI, yang diisi oleh
// `pasti-sldk-sync ringkasan` (backend/cmd/sldk-sync). Aplikasi tidak pernah menghitung langsung ke tabel
// aset SLDK yang berukuran ratusan GB.

const (
	overviewTimeout    = 20 * time.Second
	settingActiveFlags = "aktif_flag_keys"
	maxActiveFlagKeys  = 50
)

var flagKeyPattern = regexp.MustCompile(`^[0-9A-Za-z\-|]{1,60}$`)

type overviewGroup struct {
	K1             *string `json:"k1"`
	K2             *string `json:"k2,omitempty"`
	Jumlah         int64   `json:"jumlah"`
	NilaiPerolehan float64 `json:"nilai_perolehan"`
	NilaiBuku      float64 `json:"nilai_buku"`
	NilaiSusut     float64 `json:"nilai_susut"`
}

type overviewTotal struct {
	Jumlah         int64   `json:"jumlah"`
	NilaiPerolehan float64 `json:"nilai_perolehan"`
	NilaiBuku      float64 `json:"nilai_buku"`
	NilaiSusut     float64 `json:"nilai_susut"`
}

type overviewAnomaly struct {
	Key           string           `json:"key"`
	Label         string           `json:"label"`
	Keterangan    string           `json:"keterangan"`
	Jumlah        int64            `json:"jumlah"`
	SatkerTeratas []overviewSatker `json:"satker_teratas"`
}

type overviewSatker struct {
	ID     string `json:"id"`
	Jumlah int64  `json:"jumlah"`
}

type syncInfo struct {
	ID          int64      `json:"id"`
	Status      string     `json:"status"`
	Mulai       time.Time  `json:"mulai"`
	Selesai     *time.Time `json:"selesai"`
	Cakupan     *string    `json:"cakupan"`
	JumlahBaris *int64     `json:"jumlah_baris"`
	TotalAset   *int64     `json:"total_aset"`
	DataPer     *time.Time `json:"data_per"`
	Pesan       *string    `json:"pesan"`
}

type flagKeyInfo struct {
	Key    string `json:"key"`
	Jumlah int64  `json:"jumlah"`
}

func scanMoney(ns sql.NullString) float64 {
	if !ns.Valid {
		return 0
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(ns.String), 64)
	if err != nil {
		return 0
	}
	return f
}

func optStr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

func latestSync(ctx context.Context, onlySuccess bool) (*syncInfo, error) {
	q := `SELECT TOP 1 id, status, mulai, selesai, cakupan, jumlah_baris, total_aset, data_per, pesan
	      FROM sldk_sync_log WHERE jenis = 'ringkasan'`
	if onlySuccess {
		q += ` AND status = 'sukses'`
	}
	q += ` ORDER BY id DESC`

	var s syncInfo
	var selesai, dataPer sql.NullTime
	var cakupan, pesan sql.NullString
	var baris, total sql.NullInt64
	err := database.DB.QueryRowContext(ctx, q).Scan(&s.ID, &s.Status, &s.Mulai, &selesai, &cakupan, &baris, &total, &dataPer, &pesan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if selesai.Valid {
		s.Selesai = &selesai.Time
	}
	if dataPer.Valid {
		s.DataPer = &dataPer.Time
	}
	s.Cakupan, s.Pesan = optStr(cakupan), optStr(pesan)
	if baris.Valid {
		s.JumlahBaris = &baris.Int64
	}
	if total.Valid {
		s.TotalAset = &total.Int64
	}
	return &s, nil
}

// activeFlagKeys membaca pengaturan "kunci penanda yang dihitung aktif". Kosong berarti semua baris dihitung.
func activeFlagKeys(ctx context.Context) ([]string, error) {
	var raw sql.NullString
	err := database.DB.QueryRowContext(ctx, `SELECT nilai FROM sldk_pengaturan WHERE kunci = @p1`, settingActiveFlags).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !raw.Valid) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var keys []string
	if err := json.Unmarshal([]byte(raw.String), &keys); err != nil {
		log.Println("[SLDK WARN] pengaturan aktif_flag_keys tidak valid, diabaikan:", err)
		return nil, nil
	}
	return keys, nil
}

func allFlagKeys(ctx context.Context) ([]flagKeyInfo, error) {
	rows, err := database.DB.QueryContext(ctx,
		`SELECT flag_key, SUM(jumlah) FROM sldk_agregat WHERE dim = 'total' GROUP BY flag_key ORDER BY SUM(jumlah) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []flagKeyInfo{}
	for rows.Next() {
		var f flagKeyInfo
		if err := rows.Scan(&f.Key, &f.Jumlah); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// flagClause menyusun " AND flag_key IN (...)" untuk kunci aktif; kosong bila semua baris dihitung.
func flagClause(keys []string, firstParam int) (string, []interface{}) {
	if len(keys) == 0 {
		return "", nil
	}
	ph := make([]string, len(keys))
	args := make([]interface{}, len(keys))
	for i, k := range keys {
		ph[i] = fmt.Sprintf("@p%d", firstParam+i)
		args[i] = k
	}
	return " AND flag_key IN (" + strings.Join(ph, ",") + ")", args
}

func queryGroups(ctx context.Context, dim string, top int, orderBy string, flagSQL string, flagArgs []interface{}) ([]overviewGroup, error) {
	topSQL := ""
	if top > 0 {
		topSQL = fmt.Sprintf("TOP (%d) ", top)
	}
	// dim dan orderBy berasal dari konstanta di kode ini, bukan dari pengguna.
	q := fmt.Sprintf(`SELECT %sk1, k2, SUM(jumlah), SUM(nilai_perolehan), SUM(nilai_buku), SUM(nilai_susut)
		FROM sldk_agregat WHERE dim = @p1%s GROUP BY k1, k2 ORDER BY %s`, topSQL, flagSQL, orderBy)
	rows, err := database.DB.QueryContext(ctx, q, append([]interface{}{dim}, flagArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []overviewGroup{}
	for rows.Next() {
		var g overviewGroup
		var k1, k2, aset, buku, susut sql.NullString
		if err := rows.Scan(&k1, &k2, &g.Jumlah, &aset, &buku, &susut); err != nil {
			return nil, err
		}
		g.K1, g.K2 = optStr(k1), optStr(k2)
		g.NilaiPerolehan, g.NilaiBuku, g.NilaiSusut = scanMoney(aset), scanMoney(buku), scanMoney(susut)
		out = append(out, g)
	}
	return out, rows.Err()
}

func topSatkerByRule(ctx context.Context, rule sldk.Rule, flagSQL string, flagArgs []interface{}) ([]overviewSatker, error) {
	col := rule.CountColumn() // nama kolom dari daftar tetap sldk.Rules
	q := fmt.Sprintf(`SELECT TOP (10) k1, SUM(%s) FROM sldk_agregat WHERE dim = 'satker'%s GROUP BY k1
		HAVING SUM(%s) > 0 ORDER BY SUM(%s) DESC`, col, flagSQL, col, col)
	rows, err := database.DB.QueryContext(ctx, q, flagArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []overviewSatker{}
	for rows.Next() {
		var id sql.NullString
		var n int64
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		if id.Valid {
			out = append(out, overviewSatker{ID: id.String, Jumlah: n})
		}
	}
	return out, rows.Err()
}

// GetAssetOverview mengembalikan data halaman Ringkasan dan Pemantauan dari hasil sinkronisasi terakhir.
func GetAssetOverview(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), overviewTimeout)
	defer cancel()

	fail := func(what string, err error) {
		if errors.Is(err, context.Canceled) {
			return
		}
		log.Println("[SLDK ERROR]", what+":", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca ringkasan aset")
	}

	latest, err := latestSync(ctx, false)
	if err != nil {
		fail("riwayat sinkronisasi", err)
		return
	}
	lastOK, err := latestSync(ctx, true)
	if err != nil {
		fail("sinkronisasi sukses terakhir", err)
		return
	}
	// Belum pernah berhasil: tidak ada data untuk ditampilkan. UI menjelaskan cara menjalankan sinkronisasi.
	if lastOK == nil {
		utils.SuccessResponse(c, http.StatusOK, "Belum ada data ringkasan", gin.H{"tersedia": false, "sinkron_terakhir": latest, "sinkron_sukses": nil})
		return
	}

	keys, err := allFlagKeys(ctx)
	if err != nil {
		fail("kunci penanda", err)
		return
	}
	configured, err := activeFlagKeys(ctx)
	if err != nil {
		fail("pengaturan", err)
		return
	}
	// Hanya kunci yang masih ada di data terakhir yang dipakai (data bisa berubah antar sinkronisasi).
	exists := map[string]bool{}
	for _, k := range keys {
		exists[k.Key] = true
	}
	var active []string
	for _, k := range configured {
		if exists[k] {
			active = append(active, k)
		}
	}
	flagSQL, flagArgs := flagClause(active, 2)

	var total overviewTotal
	counts := make([]int64, len(sldk.Rules))
	{
		var aset, buku, susut sql.NullString
		sums := make([]string, len(sldk.Rules))
		for i, r := range sldk.Rules {
			sums[i] = "ISNULL(SUM(" + r.CountColumn() + "), 0)"
		}
		// Query ini tidak punya parameter dim, jadi placeholder kunci dimulai dari @p1.
		totalFlagSQL, totalFlagArgs := flagClause(active, 1)
		q := "SELECT ISNULL(SUM(jumlah), 0), SUM(nilai_perolehan), SUM(nilai_buku), SUM(nilai_susut), " + strings.Join(sums, ", ") +
			" FROM sldk_agregat WHERE dim = 'total'" + totalFlagSQL
		dest := []interface{}{&total.Jumlah, &aset, &buku, &susut}
		for i := range counts {
			dest = append(dest, &counts[i])
		}
		if err := database.DB.QueryRowContext(ctx, q, totalFlagArgs...).Scan(dest...); err != nil {
			fail("total", err)
			return
		}
		total.NilaiPerolehan, total.NilaiBuku, total.NilaiSusut = scanMoney(aset), scanMoney(buku), scanMoney(susut)
	}

	byValue := "SUM(nilai_buku) DESC"
	dims := []struct {
		name    string
		top     int
		orderBy string
	}{
		{"jenis", 0, byValue}, {"kondisi", 0, byValue}, {"status", 0, byValue}, {"jenis_kondisi", 0, "k1, k2"},
		{"provinsi", 20, byValue}, {"tahun", 0, "k1"}, {"satker", 10, byValue},
	}
	groups := map[string][]overviewGroup{}
	for _, d := range dims {
		g, err := queryGroups(ctx, d.name, d.top, d.orderBy, flagSQL, flagArgs)
		if err != nil {
			fail("dimensi "+d.name, err)
			return
		}
		groups[d.name] = g
	}

	anomalies := make([]overviewAnomaly, 0, len(sldk.Rules))
	satkerIDs := map[int64]bool{}
	for _, g := range groups["satker"] {
		if g.K1 != nil {
			if id, ok := toInt64(*g.K1); ok {
				satkerIDs[id] = true
			}
		}
	}
	for i, r := range sldk.Rules {
		// Query top satker per aturan memakai placeholder kunci mulai @p1 (tanpa parameter dim).
		fSQL, fArgs := flagClause(active, 1)
		top, err := topSatkerByRule(ctx, r, fSQL, fArgs)
		if err != nil {
			fail("aturan "+r.Key, err)
			return
		}
		for _, s := range top {
			if id, ok := toInt64(s.ID); ok {
				satkerIDs[id] = true
			}
		}
		anomalies = append(anomalies, overviewAnomaly{Key: r.Key, Label: r.Label, Keterangan: r.Keterangan, Jumlah: counts[i], SatkerTeratas: top})
	}

	// Nama satker diambil dari SLDK (tabel kecil, ber-cache). Bila SLDK tidak tersedia, UI menampilkan id saja.
	satker := map[string]satkerInfo{}
	if database.SLDKDB != nil && len(satkerIDs) > 0 {
		ids := make([]int64, 0, len(satkerIDs))
		for id := range satkerIDs {
			ids = append(ids, id)
		}
		lctx, lcancel := context.WithTimeout(ctx, sldkLookupTimeout)
		for id, info := range lookupSatker(lctx, ids) {
			satker[strconv.FormatInt(id, 10)] = info
		}
		lcancel()
	}

	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil ringkasan aset", gin.H{
		"tersedia":         true,
		"sinkron_terakhir": latest,
		"sinkron_sukses":   lastOK,
		"definisi": gin.H{
			"flag_keys_aktif": active,
			"terkonfirmasi":   len(active) > 0,
		},
		"flag_keys":      keys,
		"total":          total,
		"jenis":          groups["jenis"],
		"kondisi":        groups["kondisi"],
		"status":         groups["status"],
		"jenis_kondisi":  groups["jenis_kondisi"],
		"provinsi":       groups["provinsi"],
		"tahun":          groups["tahun"],
		"satker_teratas": groups["satker"],
		"anomali":        anomalies,
		"satker":         satker,
	})
}

// UpdateOverviewSettings menyimpan kunci penanda yang dihitung "aset aktif" (khusus admin). Daftar kosong
// berarti semua baris dihitung. Arti nilai status_data/sts_his/sts_ast belum dikonfirmasi di kode, jadi
// keputusannya ada pada admin yang memahami datanya.
func UpdateOverviewSettings(c *gin.Context) {
	var body struct {
		FlagKeysAktif []string `json:"flag_keys_aktif"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid")
		return
	}
	if len(body.FlagKeysAktif) > maxActiveFlagKeys {
		utils.ErrorResponse(c, http.StatusBadRequest, "Terlalu banyak kunci penanda")
		return
	}
	seen := map[string]bool{}
	clean := make([]string, 0, len(body.FlagKeysAktif))
	for _, k := range body.FlagKeysAktif {
		k = strings.TrimSpace(k)
		if !flagKeyPattern.MatchString(k) {
			utils.ErrorResponse(c, http.StatusBadRequest, "Kunci penanda tidak valid")
			return
		}
		if !seen[k] {
			seen[k] = true
			clean = append(clean, k)
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), overviewTimeout)
	defer cancel()

	if len(clean) > 0 {
		keys, err := allFlagKeys(ctx)
		if err != nil {
			log.Println("[SLDK ERROR] kunci penanda:", err)
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal memeriksa kunci penanda")
			return
		}
		exists := map[string]bool{}
		for _, k := range keys {
			exists[k.Key] = true
		}
		for _, k := range clean {
			if !exists[k] {
				utils.ErrorResponse(c, http.StatusBadRequest, "Kunci penanda tidak ada di data ringkasan terakhir: "+k)
				return
			}
		}
	}

	var err error
	if len(clean) == 0 {
		_, err = database.DB.ExecContext(ctx, `DELETE FROM sldk_pengaturan WHERE kunci = @p1`, settingActiveFlags)
	} else {
		encoded, _ := json.Marshal(clean)
		_, err = database.DB.ExecContext(ctx,
			`UPDATE sldk_pengaturan SET nilai = @p2, diubah_oleh = @p3, diubah_pada = SYSUTCDATETIME() WHERE kunci = @p1;
			 IF @@ROWCOUNT = 0 INSERT INTO sldk_pengaturan (kunci, nilai, diubah_oleh) VALUES (@p1, @p2, @p3)`,
			settingActiveFlags, string(encoded), c.GetString("username"))
	}
	if err != nil {
		log.Println("[SLDK ERROR] gagal menyimpan pengaturan:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menyimpan pengaturan")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Pengaturan disimpan", gin.H{"flag_keys_aktif": clean})
}
