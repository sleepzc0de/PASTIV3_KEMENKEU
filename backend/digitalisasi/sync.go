package digitalisasi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// DefaultMaxRows membatasi jumlah baris yang ditahan di memori per dataset. Seluruh hasil dibaca dulu dari
// SLDK, baru ditulis ke PASTI dalam satu transaksi singkat, supaya halaman tidak terkunci selama query SLDK
// yang bisa memakan puluhan menit.
const DefaultMaxRows = 500_000

// Result: ringkasan satu sinkronisasi dataset.
type Result struct {
	Rows       int
	WithCoords int
	Stats      convStats
	Duration   time.Duration
}

// Summary: kalimat untuk kolom pesan di riwayat.
func (r Result) Summary() string {
	parts := []string{fmt.Sprintf("%d baris", r.Rows)}
	if r.WithCoords > 0 || r.Rows > 0 {
		parts = append(parts, fmt.Sprintf("%d berkoordinat", r.WithCoords))
	}
	if r.Stats.BadCoord > 0 {
		parts = append(parts, fmt.Sprintf("%d koordinat tidak valid dibuang", r.Stats.BadCoord))
	}
	if r.Stats.Truncated > 0 {
		parts = append(parts, fmt.Sprintf("%d teks dipotong", r.Stats.Truncated))
	}
	if r.Stats.BadNumber > 0 {
		parts = append(parts, fmt.Sprintf("%d angka tidak terbaca", r.Stats.BadNumber))
	}
	return strings.Join(parts, ", ") + fmt.Sprintf(" (%s)", r.Duration.Round(time.Second))
}

// Options: pengaturan satu kali sinkronisasi.
type Options struct {
	MaxRows  int
	RunID    int64            // dicatat di kolom id_sinkron
	Progress func(msg string) // dipanggil berkala; boleh nil
}

func quoteIdent(name string) string { return "[" + strings.ReplaceAll(name, "]", "]]") + "]" }

func (o Options) progress(msg string) {
	if o.Progress != nil {
		o.Progress(msg)
	}
}

// Sync membaca satu dataset dari SLDK lalu mengganti isi tabel tujuannya. Data lama baru dihapus setelah
// seluruh hasil terbaca dan siap ditulis; bila ada yang gagal, tabel tidak berubah.
func Sync(ctx context.Context, sldkDB, pastiDB *sql.DB, ds Dataset, opt Options) (Result, error) {
	start := time.Now()
	if opt.MaxRows <= 0 {
		opt.MaxRows = DefaultMaxRows
	}
	query, err := ds.Query()
	if err != nil {
		return Result{}, fmt.Errorf("query %s tidak ditemukan: %w", ds.QueryFile, err)
	}

	opt.progress("Membaca dari SLDK...")
	data, st, err := readSource(ctx, sldkDB, ds, query, opt)
	if err != nil {
		return Result{}, err
	}

	res := Result{Rows: len(data), Stats: st}
	latIdx, lngIdx := coordIndexes(ds)
	if latIdx >= 0 {
		for _, row := range data {
			if row[latIdx] != nil && row[lngIdx] != nil {
				res.WithCoords++
			}
		}
	}

	// Hasil kosong tidak boleh menimpa data yang ada: itu hampir pasti kegagalan di hulu (akses, filter), bukan
	// kenyataan bahwa semua aset hilang.
	if len(data) == 0 {
		var existing int64
		if err := pastiDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteIdent(ds.Table)).Scan(&existing); err != nil {
			return Result{}, fmt.Errorf("gagal memeriksa isi tabel %s: %w", ds.Table, err)
		}
		if existing > 0 {
			return Result{}, errors.New("SLDK mengembalikan 0 baris sedangkan tabel sudah berisi data; data lama TIDAK diganti. Periksa akses ke SLDK dan filter query")
		}
	}

	opt.progress(fmt.Sprintf("Menyimpan %d baris ke database PASTI...", len(data)))
	if err := replaceTable(ctx, pastiDB, ds, data, opt.RunID); err != nil {
		return Result{}, err
	}
	res.Duration = time.Since(start)
	return res, nil
}

// coordIndexes: posisi kolom Latitude/Longitude pada Columns (-1 bila dataset tanpa koordinat).
func coordIndexes(ds Dataset) (lat, lng int) {
	lat, lng = -1, -1
	for i, c := range ds.Columns {
		switch {
		case strings.EqualFold(c.Name, "Latitude"):
			lat = i
		case strings.EqualFold(c.Name, "Longitude"):
			lng = i
		}
	}
	if lat < 0 || lng < 0 {
		return -1, -1
	}
	return lat, lng
}

// readSource menjalankan query pada SATU koneksi (query memakai tabel # sementara yang hanya hidup per sesi) dan
// memetakan kolom hasilnya ke kolom tujuan menurut nama.
func readSource(ctx context.Context, db *sql.DB, ds Dataset, query string, opt Options) ([][]interface{}, convStats, error) {
	var st convStats
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, st, fmt.Errorf("tidak bisa terhubung ke SLDK: %w", err)
	}
	defer conn.Close()

	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, st, fmt.Errorf("query SLDK gagal: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, st, err
	}
	byName := make(map[string]int, len(cols))
	for i, c := range cols {
		byName[strings.ToLower(c)] = i
	}
	src := make([]int, len(ds.Columns))
	var missing []string
	for i, c := range ds.Columns {
		j, ok := byName[strings.ToLower(c.SourceName())]
		if !ok {
			missing = append(missing, c.SourceName())
			continue
		}
		src[i] = j
	}
	if len(missing) > 0 {
		return nil, st, fmt.Errorf("hasil query tidak memuat kolom: %s (hasil %q memuat %d kolom)", strings.Join(missing, ", "), ds.QueryFile, len(cols))
	}

	latIdx, lngIdx := coordIndexes(ds)
	raw := make([]interface{}, len(cols))
	ptr := make([]interface{}, len(cols))
	for i := range raw {
		ptr[i] = &raw[i]
	}

	var data [][]interface{}
	for rows.Next() {
		if len(data) >= opt.MaxRows {
			return nil, st, fmt.Errorf("hasil query melebihi %d baris; sinkronisasi dihentikan", opt.MaxRows)
		}
		if err := rows.Scan(ptr...); err != nil {
			return nil, st, fmt.Errorf("gagal membaca baris hasil: %w", err)
		}
		row := make([]interface{}, len(ds.Columns))
		for i, c := range ds.Columns {
			row[i] = convertValue(c, raw[src[i]], &st)
		}
		fixCoordinates(row, latIdx, lngIdx, &st)
		data = append(data, row)
		if len(data)%20000 == 0 {
			opt.progress(fmt.Sprintf("Membaca dari SLDK... %d baris", len(data)))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, st, fmt.Errorf("query SLDK terputus: %w", err)
	}
	// Skrip bertahap (tabel #) bisa mengirim hasil tambahan; menelusurinya memunculkan galat di langkah penutup.
	for rows.NextResultSet() {
	}
	if err := rows.Err(); err != nil {
		return nil, st, fmt.Errorf("query SLDK terputus: %w", err)
	}
	return data, st, nil
}

// loadKeep membaca nilai kolom manual (ds.Keep) per kunci sebelum tabel dikosongkan.
func loadKeep(ctx context.Context, tx *sql.Tx, ds Dataset) (map[string][]interface{}, error) {
	cols := make([]string, 0, len(ds.Keep)+1)
	cols = append(cols, quoteIdent(ds.KeyColumn))
	for _, k := range ds.Keep {
		cols = append(cols, quoteIdent(k))
	}
	rows, err := tx.QueryContext(ctx, "SELECT "+strings.Join(cols, ", ")+" FROM "+quoteIdent(ds.Table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]interface{}{}
	for rows.Next() {
		key := sql.NullString{}
		vals := make([]sql.NullString, len(ds.Keep))
		dest := []interface{}{&key}
		for i := range vals {
			dest = append(dest, &vals[i])
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		if !key.Valid {
			continue
		}
		kept := make([]interface{}, len(vals))
		for i, v := range vals {
			if v.Valid {
				kept[i] = v.String
			}
		}
		out[key.String] = kept
	}
	return out, rows.Err()
}

func columnIndex(ds Dataset, name string) int {
	for i, c := range ds.Columns {
		if strings.EqualFold(c.Name, name) {
			return i
		}
	}
	return -1
}

// replaceTable mengosongkan tabel tujuan lalu mengisinya dengan data baru dalam satu transaksi.
func replaceTable(ctx context.Context, db *sql.DB, ds Dataset, data [][]interface{}, runID int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("gagal memulai transaksi: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if len(ds.Keep) > 0 {
		old, err := loadKeep(ctx, tx, ds)
		if err != nil {
			return fmt.Errorf("gagal membaca kolom manual %s: %w", ds.Table, err)
		}
		keyIdx := columnIndex(ds, ds.KeyColumn)
		keepIdx := make([]int, len(ds.Keep))
		for i, k := range ds.Keep {
			keepIdx[i] = columnIndex(ds, k)
		}
		for _, row := range data {
			key, _ := row[keyIdx].(string)
			kept, ok := old[key]
			if !ok {
				continue
			}
			for i, idx := range keepIdx {
				if row[idx] == nil {
					row[idx] = kept[i]
				}
			}
		}
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM "+quoteIdent(ds.Table)); err != nil {
		return fmt.Errorf("gagal mengosongkan %s: %w", ds.Table, err)
	}

	names := make([]string, 0, len(ds.Columns)+1)
	for _, c := range ds.Columns {
		names = append(names, quoteIdent(c.Name))
	}
	names = append(names, quoteIdent("id_sinkron"))
	width := len(names)
	// Batas parameter SQL Server 2100 per perintah, dan 1000 baris per VALUES.
	perBatch := 2000 / width
	if perBatch > 1000 {
		perBatch = 1000
	}
	if perBatch < 1 {
		perBatch = 1
	}
	head := "INSERT INTO " + quoteIdent(ds.Table) + " (" + strings.Join(names, ", ") + ") VALUES "

	for from := 0; from < len(data); from += perBatch {
		to := from + perBatch
		if to > len(data) {
			to = len(data)
		}
		var sb strings.Builder
		sb.WriteString(head)
		args := make([]interface{}, 0, (to-from)*width)
		for i := from; i < to; i++ {
			if i > from {
				sb.WriteString(", ")
			}
			sb.WriteString("(")
			for j := 0; j < width; j++ {
				if j > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(&sb, "@p%d", len(args)+1)
				if j < width-1 {
					args = append(args, data[i][j])
				} else {
					args = append(args, runID)
				}
			}
			sb.WriteString(")")
		}
		if _, err := tx.ExecContext(ctx, sb.String(), args...); err != nil {
			return fmt.Errorf("gagal menyimpan ke %s (baris %d-%d): %w", ds.Table, from+1, to, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("gagal menyimpan perubahan %s: %w", ds.Table, err)
	}
	committed = true
	return nil
}
