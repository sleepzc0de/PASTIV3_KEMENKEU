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

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/sldk"
	"pasti-v3-backend/utils"
)

// Batas yang melindungi server SLDK: tabel aset sangat besar, jadi satu pencarian bisa memindai ratusan GB.
const sldkMaxConcurrent = 4 // permintaan berat yang boleh berjalan bersamaan

var (
	sldkSearchTimeout = 25 * time.Second // pencarian & kartu aset
	sldkLookupTimeout = 15 * time.Second // tabel referensi/satker yang kecil
	sldkSlotWait      = 3 * time.Second  // lama menunggu giliran sebelum ditolak (429)
)

var sldkSlots = make(chan struct{}, sldkMaxConcurrent)

// acquireSLDKSlot mengambil satu giliran permintaan berat; ok=false bila terlalu ramai atau klien sudah pergi.
func acquireSLDKSlot(ctx context.Context) (release func(), ok bool) {
	timer := time.NewTimer(sldkSlotWait)
	defer timer.Stop()
	select {
	case sldkSlots <- struct{}{}:
		return func() { <-sldkSlots }, true
	case <-timer.C:
		return nil, false
	case <-ctx.Done():
		return nil, false
	}
}

func sldkReady(c *gin.Context) bool {
	if database.SLDKDB == nil {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Koneksi ke SLDK sedang tidak tersedia")
		return false
	}
	return true
}

// sldkTable memberi nama tabel SLDK lain dengan skema yang sama dengan tabel aset (mis. DJKN.SIMAN2_R_SATKER).
func sldkTable(name string) string {
	return sldk.SiblingTable(config.Cfg.SLDKAssetTable, name)
}

// queryMaps menjalankan query dan mengembalikan baris sebagai map. Galat yang muncul saat iterasi
// (mis. batas waktu di tengah pemindaian) dikembalikan, bukan diabaikan.
func queryMaps(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := database.SLDKDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results, err := rowsToMaps(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []map[string]interface{}{}
	}
	return results, nil
}

// respondSLDKError membalas kegagalan query SLDK tanpa membocorkan detail database ke klien.
func respondSLDKError(c *gin.Context, ctx context.Context, err error, what, timeoutMsg string) {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		utils.ErrorResponse(c, http.StatusGatewayTimeout, timeoutMsg)
		return
	}
	if errors.Is(err, context.Canceled) {
		return // klien sudah menutup koneksi
	}
	log.Println("[SLDK ERROR]", what+":", err)
	utils.ErrorResponse(c, http.StatusBadGateway, what)
}

func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int32:
		return int64(n), true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		return i, err == nil
	}
	return 0, false
}

// ============ Tabel referensi (kode -> nama) ============

type refItem struct {
	Kode string `json:"kode"`
	Nama string `json:"nama"`
}

// ruleInfo: aturan pemantauan (statis, bukan dari database) untuk filter pencarian dan dashboard.
type ruleInfo struct {
	Key        string `json:"key"`
	Label      string `json:"label"`
	Keterangan string `json:"keterangan"`
}

func ruleInfos() []ruleInfo {
	out := make([]ruleInfo, 0, len(sldk.Rules))
	for _, r := range sldk.Rules {
		out = append(out, ruleInfo{Key: r.Key, Label: r.Label, Keterangan: r.Keterangan})
	}
	return out
}

type refSet struct {
	JenisBMN         []refItem  `json:"jenis_bmn"`
	Kondisi          []refItem  `json:"kondisi"`
	StatusPenggunaan []refItem  `json:"status_penggunaan"`
	StatusHukum      []refItem  `json:"status_hukum"`
	Anomali          []ruleInfo `json:"anomali"`
}

var refCache struct {
	sync.Mutex
	data *refSet
	at   time.Time
}

const refTTL = time.Hour

func loadRefItems(ctx context.Context, query string) ([]refItem, error) {
	rows, err := database.SLDKDB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []refItem{}
	for rows.Next() {
		var kode, nama sql.NullString
		if err := rows.Scan(&kode, &nama); err != nil {
			return nil, err
		}
		if kode.Valid && strings.TrimSpace(kode.String) != "" {
			items = append(items, refItem{Kode: strings.TrimSpace(kode.String), Nama: strings.TrimSpace(nama.String)})
		}
	}
	return items, rows.Err()
}

// GetAssetReferences mengembalikan kamus kode -> nama (jenis BMN, kondisi, status penggunaan, status hukum).
// Tabelnya kecil (belasan baris), jadi aman dimuat langsung lalu disimpan di memori selama refTTL. Tabel yang
// gagal dimuat dikembalikan kosong (UI menampilkan kodenya) dan tidak membuat seluruh respons gagal.
func GetAssetReferences(c *gin.Context) {
	if !sldkReady(c) {
		return
	}
	if config.Cfg.SLDKAssetTable == "" {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "SLDK_ASSET_TABLE belum dikonfigurasi di .env")
		return
	}

	refCache.Lock()
	defer refCache.Unlock()
	if refCache.data != nil && time.Since(refCache.at) < refTTL {
		utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil referensi", refCache.data)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), sldkLookupTimeout)
	defer cancel()

	set := &refSet{Anomali: ruleInfos()}
	allOK := true
	for _, def := range sldk.RefDefs {
		items, err := loadRefItems(ctx, def.Query(sldkTable(def.Table)))
		if err != nil {
			allOK = false
			log.Println("[SLDK WARN] gagal memuat referensi", def.Key+":", err)
			items = []refItem{}
		}
		switch def.Key {
		case "jenis_bmn":
			set.JenisBMN = items
		case "kondisi":
			set.Kondisi = items
		case "status_penggunaan":
			set.StatusPenggunaan = items
		case "status_hukum":
			set.StatusHukum = items
		}
	}

	if allOK {
		refCache.data = set
		refCache.at = time.Now()
	}
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil referensi", set)
}

// ============ Satker ============

type satkerInfo struct {
	ID    int64  `json:"id"`
	Kode  string `json:"kode"`
	Nama  string `json:"nama"`
	found bool
	at    time.Time
}

var satkerCache = struct {
	sync.Mutex
	m map[int64]satkerInfo
}{m: map[int64]satkerInfo{}}

const (
	satkerTTL      = 6 * time.Hour
	satkerCacheMax = 20000
)

// lookupSatker mencari nama satker untuk sekumpulan id_satker. Yang sudah ada di cache tidak ditanyakan
// lagi; hasil "tidak ditemukan" juga disimpan supaya id yang sama tidak memicu pemindaian berulang.
func lookupSatker(ctx context.Context, ids []int64) map[int64]satkerInfo {
	out := map[int64]satkerInfo{}
	var missing []int64

	satkerCache.Lock()
	for _, id := range ids {
		if e, ok := satkerCache.m[id]; ok && time.Since(e.at) < satkerTTL {
			if e.found {
				out[id] = e
			}
			continue
		}
		missing = append(missing, id)
	}
	satkerCache.Unlock()

	if len(missing) == 0 {
		return out
	}

	placeholders := make([]string, len(missing))
	args := make([]interface{}, len(missing))
	for i, id := range missing {
		placeholders[i] = fmt.Sprintf("@p%d", i+1)
		args[i] = id
	}
	rows, err := queryMaps(ctx, fmt.Sprintf(
		"SELECT id_satker, kd_satker, ur_satker FROM %s WHERE id_satker IN (%s)",
		sldkTable("SIMAN2_R_SATKER"), strings.Join(placeholders, ",")), args...)
	if err != nil {
		log.Println("[SLDK WARN] gagal mencari nama satker:", err)
		return out // nama gagal dimuat: UI menampilkan id/kode saja, hasil tidak disimpan di cache
	}

	now := time.Now()
	found := map[int64]satkerInfo{}
	for _, r := range rows {
		id, ok := toInt64(r["id_satker"])
		if !ok {
			continue
		}
		kode, _ := r["kd_satker"].(string)
		nama, _ := r["ur_satker"].(string)
		found[id] = satkerInfo{ID: id, Kode: strings.TrimSpace(kode), Nama: strings.TrimSpace(nama), found: true, at: now}
	}

	satkerCache.Lock()
	if len(satkerCache.m) > satkerCacheMax {
		satkerCache.m = map[int64]satkerInfo{}
	}
	for _, id := range missing {
		if info, ok := found[id]; ok {
			satkerCache.m[id] = info
			out[id] = info
		} else {
			satkerCache.m[id] = satkerInfo{ID: id, at: now}
		}
	}
	satkerCache.Unlock()
	return out
}

// SearchSatker dipakai pemilih satker pada filter: mencari menurut kode (awalan) atau nama (mengandung kata).
func SearchSatker(c *gin.Context) {
	if !sldkReady(c) {
		return
	}
	q := strings.TrimSpace(c.Query("q"))
	if len([]rune(q)) < assetMinTextLen {
		utils.ErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Kata kunci minimal %d karakter", assetMinTextLen))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), sldkLookupTimeout)
	defer cancel()

	query := fmt.Sprintf(
		"SELECT TOP (20) id_satker, kd_satker, ur_satker FROM %s WHERE (kd_satker LIKE @p1 ESCAPE '\\' OR ur_satker LIKE @p2 ESCAPE '\\')",
		sldkTable(sldk.TableSatker))
	args := []interface{}{escapeLike(q) + "%", "%" + escapeLike(q) + "%"}
	if kl := config.Cfg.SLDKKLKode; kl != "" {
		query += " AND " + sldk.SatkerScopePredicate(config.Cfg.SLDKAssetTable, "@p3")
		args = append(args, kl)
	}
	query += " ORDER BY ur_satker"
	rows, err := queryMaps(ctx, query, args...)
	if err != nil {
		respondSLDKError(c, ctx, err, "Gagal mencari satker di SLDK", "Pencarian satker melebihi batas waktu")
		return
	}

	items := make([]satkerInfo, 0, len(rows))
	for _, r := range rows {
		id, ok := toInt64(r["id_satker"])
		if !ok {
			continue
		}
		kode, _ := r["kd_satker"].(string)
		nama, _ := r["ur_satker"].(string)
		items = append(items, satkerInfo{ID: id, Kode: strings.TrimSpace(kode), Nama: strings.TrimSpace(nama)})
	}
	utils.SuccessResponse(c, http.StatusOK, "Pencarian satker berhasil", gin.H{"items": items})
}

// ============ Pencarian aset ============

// SearchAssets mencari aset. Lihat sldk_query.go untuk aturan query dan alasannya.
func SearchAssets(c *gin.Context) {
	if !sldkReady(c) {
		return
	}
	if config.Cfg.SLDKAssetTable == "" {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "SLDK_ASSET_TABLE belum dikonfigurasi di .env")
		return
	}

	query, args, err := buildAssetSearch(assetSearchParams{
		Q:        c.Query("q"),
		By:       c.Query("by"),
		IDSatker: c.Query("id_satker"),
		JnsBMN:   c.Query("kd_jns_bmn"),
		Kondisi:  c.Query("kd_kondisi"),
		Status:   c.Query("kd_status"),
		Tahun:    c.Query("tahun"),
		Anomali:  c.Query("anomali"),
		Limit:    c.Query("limit"),
	}, config.Cfg.SLDKAssetTable, resolveTextColumns(config.Cfg.SLDKAssetSearchCols), config.Cfg.SLDKKLKode)
	if err != nil {
		var qe queryError
		if errors.As(err, &qe) {
			utils.ErrorResponse(c, http.StatusBadRequest, qe.Error())
		} else {
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menyusun pencarian")
		}
		return
	}

	release, ok := acquireSLDKSlot(c.Request.Context())
	if !ok {
		utils.ErrorResponse(c, http.StatusTooManyRequests, "Server SLDK sedang menangani banyak pencarian, coba lagi beberapa saat lagi")
		return
	}
	defer release()

	ctx, cancel := context.WithTimeout(c.Request.Context(), sldkSearchTimeout)
	defer cancel()

	started := time.Now()
	results, err := queryMaps(ctx, query, args...)
	if err != nil {
		respondSLDKError(c, ctx, err, "Gagal menjalankan pencarian di SLDK", fmt.Sprintf(
			"Pencarian melebihi batas %d detik karena tabel aset sangat besar. Persempit dengan satker, jenis BMN, atau cari memakai kode register.",
			int(sldkSearchTimeout/time.Second)))
		return
	}

	// Nama satker untuk baris yang ditemukan.
	idSet := map[int64]bool{}
	var ids []int64
	for _, r := range results {
		if id, ok := toInt64(r["id_satker"]); ok && !idSet[id] {
			idSet[id] = true
			ids = append(ids, id)
		}
	}
	satker := map[string]satkerInfo{}
	if len(ids) > 0 {
		for id, info := range lookupSatker(ctx, ids) {
			satker[strconv.FormatInt(id, 10)] = info
		}
	}

	utils.SuccessResponse(c, http.StatusOK, "Pencarian berhasil", gin.H{
		"results":    results,
		"count":      len(results),
		"limit":      assetLimit(c.Query("limit")),
		"satker":     satker,
		"elapsed_ms": time.Since(started).Milliseconds(),
	})
}

// ============ Kartu aset: bagian per jenis dari tabel anak yang kecil ============

type detailSection struct {
	Rows  []map[string]interface{} `json:"rows"`
	Error string                   `json:"error,omitempty"`
}

// GetAssetDetail mengambil bagian-bagian kartu aset secara paralel. Setiap bagian berdiri sendiri: yang gagal
// ditandai error, yang lain tetap tampil.
func GetAssetDetail(c *gin.Context) {
	if !sldkReady(c) {
		return
	}
	if config.Cfg.SLDKAssetTable == "" {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "SLDK_ASSET_TABLE belum dikonfigurasi di .env")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.ErrorResponse(c, http.StatusBadRequest, "ID aset tidak valid")
		return
	}

	release, ok := acquireSLDKSlot(c.Request.Context())
	if !ok {
		utils.ErrorResponse(c, http.StatusTooManyRequests, "Server SLDK sedang menangani banyak permintaan, coba lagi beberapa saat lagi")
		return
	}
	defer release()

	ctx, cancel := context.WithTimeout(c.Request.Context(), sldkSearchTimeout)
	defer cancel()

	sections := make(map[string]detailSection, len(sldk.DetailSections))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, def := range sldk.DetailSections {
		wg.Add(1)
		go func(def sldk.DetailSection) {
			defer wg.Done()
			key := def.Key
			rows, err := queryMaps(ctx, def.Query(sldkTable(def.Table)), id)
			section := detailSection{Rows: rows}
			if err != nil {
				log.Println("[SLDK WARN] bagian kartu aset", key, "gagal:", err)
				section = detailSection{Rows: []map[string]interface{}{}, Error: "Bagian ini gagal dimuat"}
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					section.Error = "Bagian ini melebihi batas waktu"
				}
			}
			mu.Lock()
			sections[key] = section
			mu.Unlock()
		}(def)
	}
	wg.Wait()

	if errors.Is(ctx.Err(), context.Canceled) {
		return // klien sudah menutup koneksi
	}
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil detail aset", gin.H{"id_aset": id, "sections": sections})
}

// ============ Bantuan umum ============

// rowsToMaps juga dipakai handler Inaproc.
func rowsToMaps(rows *sql.Rows) ([]map[string]interface{}, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, col := range cols {
			val := values[i]
			switch v := val.(type) {
			case []byte:
				rowMap[col] = string(v)
			default:
				rowMap[col] = v
			}
		}
		results = append(results, rowMap)
	}

	return results, nil
}

