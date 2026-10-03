package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"pasti-v3-backend/sldk"
)

const (
	jenisRingkasan  = "ringkasan"
	maxAggregateRow = 2_000_000 // pagar: hasil agregat sebesar ini berarti cakupan terlalu lebar
	insertChunk     = 100       // 14 parameter per baris x 100 = 1400, di bawah batas 2100 parameter SQL Server
	staleAfterHours = 6
)

type aggRow struct {
	dim     string
	k1, k2  sql.NullString
	flagKey string
	jumlah  int64
	// Nilai uang dibaca sebagai teks (decimal(28,2)): float64 akan kehilangan ketelitian di angka triliunan.
	aset, buku, susut sql.NullString
	n                 []int64 // satu penghitung per aturan di sldk.Rules
	dataPer           sql.NullTime
}

func nullStr(s sql.NullString) interface{} {
	if !s.Valid {
		return nil
	}
	return s.String
}

func (r aggRow) values() []interface{} {
	v := []interface{}{r.dim, nullStr(r.k1), nullStr(r.k2), r.flagKey, r.jumlah, nullStr(r.aset), nullStr(r.buku), nullStr(r.susut)}
	for _, n := range r.n {
		v = append(v, n)
	}
	if r.dataPer.Valid {
		return append(v, r.dataPer.Time)
	}
	return append(v, nil)
}

type syncOptions struct {
	AssetTable string // nama mentah, mis. DJKN.SIMAN2_M_ASET
	Scope      string // kode K/L; kosong = seluruh K/L
	Out        io.Writer
}

// aggregatePlan menyusun query agregat beserta argumennya.
func aggregatePlan(assetTable, scope string) (string, []interface{}) {
	if scope == "" {
		return sldk.AggregateSQL(sldk.QuoteTableRef(assetTable), ""), nil
	}
	return sldk.AggregateSQL(sldk.QuoteTableRef(assetTable), sldk.ScopePredicate(assetTable, "@p1")), []interface{}{scope}
}

// startLog mencatat sinkronisasi dimulai. Menolak bila masih ada yang berjalan; yang berjalan lebih dari
// staleAfterHours dianggap gagal (mis. proses terbunuh) supaya tidak mengunci selamanya.
func startLog(ctx context.Context, db *sql.DB, scope string) (int64, error) {
	if _, err := db.ExecContext(ctx,
		`UPDATE sldk_sync_log SET status = 'gagal', selesai = SYSUTCDATETIME(), pesan = @p2
		 WHERE jenis = @p1 AND status = 'berjalan' AND mulai <= DATEADD(HOUR, -6, SYSUTCDATETIME())`,
		jenisRingkasan, fmt.Sprintf("Tidak selesai: dianggap gagal setelah %d jam", staleAfterHours)); err != nil {
		return 0, fmt.Errorf("gagal memeriksa sinkronisasi yang tertinggal: %w", err)
	}

	var runID int64
	var mulai time.Time
	err := db.QueryRowContext(ctx,
		`SELECT TOP 1 id, mulai FROM sldk_sync_log WHERE jenis = @p1 AND status = 'berjalan' ORDER BY id DESC`,
		jenisRingkasan).Scan(&runID, &mulai)
	if err == nil {
		return 0, fmt.Errorf("sudah ada sinkronisasi yang berjalan (id %d, mulai %s UTC); tunggu sampai selesai", runID, mulai.Format("2006-01-02 15:04"))
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("gagal memeriksa sinkronisasi yang berjalan: %w", err)
	}

	var cakupan interface{}
	if scope != "" {
		cakupan = scope
	}
	var id int64
	if err := db.QueryRowContext(ctx,
		`INSERT INTO sldk_sync_log (jenis, status, cakupan) OUTPUT INSERTED.id VALUES (@p1, 'berjalan', @p2)`,
		jenisRingkasan, cakupan).Scan(&id); err != nil {
		return 0, fmt.Errorf("gagal mencatat awal sinkronisasi: %w", err)
	}
	return id, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

// finishLog mencatat hasil. Memakai konteks sendiri karena konteks sinkronisasi bisa saja sudah habis.
func finishLog(db *sql.DB, id int64, status string, rows int, total int64, dataPer sql.NullTime, msg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var dp, jml, tot interface{}
	if dataPer.Valid {
		dp = dataPer.Time
	}
	if status == "sukses" {
		jml, tot = rows, total
	}
	_, err := db.ExecContext(ctx,
		`UPDATE sldk_sync_log SET status = @p2, selesai = SYSUTCDATETIME(), jumlah_baris = @p3, total_aset = @p4, data_per = @p5, pesan = @p6 WHERE id = @p1`,
		id, status, jml, tot, dp, truncate(msg, 900))
	return err
}

func readAggregate(ctx context.Context, q queryer, query string, args []interface{}) ([]aggRow, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []aggRow
	for rows.Next() {
		r := aggRow{n: make([]int64, len(sldk.Rules))}
		dest := []interface{}{&r.dim, &r.k1, &r.k2, &r.flagKey, &r.jumlah, &r.aset, &r.buku, &r.susut}
		for i := range r.n {
			dest = append(dest, &r.n[i])
		}
		dest = append(dest, &r.dataPer)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		out = append(out, r)
		if len(out) > maxAggregateRow {
			return nil, fmt.Errorf("hasil agregat melebihi %d baris; persempit cakupan dengan SLDK_KL_KODE", maxAggregateRow)
		}
	}
	return out, rows.Err()
}

func aggregateColumns() []string {
	cols := []string{"dim", "k1", "k2", "flag_key", "jumlah", "nilai_perolehan", "nilai_buku", "nilai_susut"}
	for _, r := range sldk.Rules {
		cols = append(cols, r.CountColumn())
	}
	return append(cols, "data_per")
}

// writeAggregate mengganti isi sldk_agregat dengan hasil baru dalam SATU transaksi: gagal di tengah jalan
// berarti data lama tetap utuh.
func writeAggregate(ctx context.Context, db *sql.DB, rows []aggRow) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() // tidak berefek bila sudah di-commit

	if _, err := tx.ExecContext(ctx, `DELETE FROM sldk_agregat`); err != nil {
		return fmt.Errorf("gagal mengosongkan sldk_agregat: %w", err)
	}

	cols := aggregateColumns()
	perRow := len(cols)
	for start := 0; start < len(rows); start += insertChunk {
		end := min(start+insertChunk, len(rows))
		var b strings.Builder
		args := make([]interface{}, 0, (end-start)*perRow)
		fmt.Fprintf(&b, "INSERT INTO sldk_agregat (%s) VALUES ", strings.Join(cols, ", "))
		for i, r := range rows[start:end] {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("(")
			for j := 0; j < perRow; j++ {
				if j > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "@p%d", len(args)+j+1)
			}
			b.WriteString(")")
			args = append(args, r.values()...)
		}
		if _, err := tx.ExecContext(ctx, b.String(), args...); err != nil {
			return fmt.Errorf("gagal menulis baris %d-%d: %w", start+1, end, err)
		}
	}
	return tx.Commit()
}

// runRingkasan memindai tabel aset SLDK sekali, lalu menyimpan hasil agregasinya di database PASTI.
func runRingkasan(ctx context.Context, sldkDB, pastiDB *sql.DB, opt syncOptions) (err error) {
	out := opt.Out
	query, args := aggregatePlan(opt.AssetTable, opt.Scope)

	id, err := startLog(ctx, pastiDB, opt.Scope)
	if err != nil {
		return err
	}
	// Apa pun yang terjadi sesudah ini, hasilnya dicatat di log.
	defer func() {
		if err != nil {
			if logErr := finishLog(pastiDB, id, "gagal", 0, 0, sql.NullTime{}, err.Error()); logErr != nil {
				fmt.Fprintln(out, "[PERINGATAN] gagal mencatat kegagalan ke sldk_sync_log:", logErr)
			}
		}
	}()

	started := time.Now()
	fmt.Fprintf(out, "[1/3] Memindai %s di SLDK (satu kali pemindaian; bisa memakan waktu lama)...\n", opt.AssetTable)
	rows, err := readAggregate(ctx, sldkDB, query, args)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("pemindaian melebihi batas waktu: %w", err)
		}
		return fmt.Errorf("query agregat gagal: %w", err)
	}
	fmt.Fprintf(out, "      selesai dalam %s, %d baris agregat\n", time.Since(started).Round(time.Second), len(rows))

	// Hasil kosong hampir pasti berarti cakupan salah (mis. kode K/L tidak ada atau id_satker tidak
	// berhubungan): jangan menimpa data lama dengan kekosongan.
	var total int64
	var dataPer sql.NullTime
	for _, r := range rows {
		if r.dim == "total" {
			total += r.jumlah
		}
		if r.dataPer.Valid && (!dataPer.Valid || r.dataPer.Time.After(dataPer.Time)) {
			dataPer = r.dataPer
		}
	}
	if len(rows) == 0 || total == 0 {
		return errors.New("hasil agregat kosong (0 aset); data lama TIDAK diganti. Periksa SLDK_KL_KODE dan jalankan `probe`")
	}

	fmt.Fprintf(out, "[2/3] Menulis %d baris ke sldk_agregat (data lama diganti dalam satu transaksi)...\n", len(rows))
	if err := writeAggregate(ctx, pastiDB, rows); err != nil {
		return err
	}

	fmt.Fprintln(out, "[3/3] Mencatat hasil...")
	if err := finishLog(pastiDB, id, "sukses", len(rows), total, dataPer, ""); err != nil {
		return fmt.Errorf("data sudah tersimpan, tetapi gagal mencatat log: %w", err)
	}
	fmt.Fprintf(out, "Selesai: %d aset dihitung dalam %d baris agregat (total %s).\n", total, len(rows), time.Since(started).Round(time.Second))
	return nil
}
