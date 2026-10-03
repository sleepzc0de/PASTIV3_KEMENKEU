package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"sort"
	"strings"

	"pasti-v3-backend/sldk"
)

// queryer: *sql.DB dan *sql.Conn sama-sama bisa menjalankan query.
type queryer interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

type probeOptions struct {
	AssetTable string
	Scope      string
	Out        io.Writer
}

type indexInfo struct {
	name     string
	typeDesc string
	primary  bool
	keys     []string
	included []string
}

// runProbe memeriksa SLDK TANPA memindai tabel besar: katalog sistem, tabel referensi yang kecil, dan satu sampel
// halaman (TABLESAMPLE 0,01%) yang dipakai untuk menguji query agregat sungguhan. Tidak mengubah apa pun.
// Setiap bagian berdiri sendiri: yang gagal dilaporkan, yang lain tetap jalan.
func runProbe(ctx context.Context, db *sql.DB, opt probeOptions) {
	out := opt.Out
	schema, assetName := sldk.SplitSchemaTable(opt.AssetTable)
	section := func(title string) { fmt.Fprintf(out, "\n== %s ==\n", title) }
	line := func(format string, a ...interface{}) { fmt.Fprintf(out, "  "+format+"\n", a...) }
	warn := func(format string, a ...interface{}) { fmt.Fprintf(out, "  [PERHATIAN] "+format+"\n", a...) }

	var notes []string
	note := func(s string) { notes = append(notes, s) }

	// ---- 1. Koneksi ----
	section("1. Koneksi")
	var version, dbName string
	if err := db.QueryRowContext(ctx, `SELECT CAST(@@VERSION AS nvarchar(300)), DB_NAME()`).Scan(&version, &dbName); err != nil {
		warn("gagal membaca versi server: %v", err)
	} else {
		line("Database: %s", dbName)
		line("Server  : %s", strings.SplitN(strings.ReplaceAll(version, "\r", ""), "\n", 2)[0])
	}

	// ---- 2. Skema ----
	section("2. Skema (kolom yang dibaca kode ini)")
	have := map[string]map[string]bool{}
	if rows, err := db.QueryContext(ctx, `SELECT TABLE_NAME, COLUMN_NAME FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = @p1`, schema); err != nil {
		warn("gagal membaca INFORMATION_SCHEMA: %v", err)
	} else {
		for rows.Next() {
			var t, c string
			if rows.Scan(&t, &c) == nil {
				if have[strings.ToLower(t)] == nil {
					have[strings.ToLower(t)] = map[string]bool{}
				}
				have[strings.ToLower(t)][strings.ToLower(c)] = true
			}
		}
		_ = rows.Close()

		expected := sldk.ExpectedColumns(opt.AssetTable)
		tables := make([]string, 0, len(expected))
		for t := range expected {
			tables = append(tables, t)
		}
		sort.Strings(tables)
		problems, totalCols := 0, 0
		for _, t := range tables {
			totalCols += len(expected[t])
			cols := have[strings.ToLower(t)]
			if cols == nil {
				line("TABEL TIDAK ADA: %s.%s", schema, t)
				problems++
				continue
			}
			var missing []string
			for _, c := range expected[t] {
				if !cols[strings.ToLower(c)] {
					missing = append(missing, c)
				}
			}
			if len(missing) > 0 {
				line("%s: kolom tidak ada: %s", t, strings.Join(missing, ", "))
				problems++
			}
		}
		if problems == 0 {
			line("OK: %d tabel dan %d kolom yang dibutuhkan semuanya ada.", len(tables), totalCols)
		} else {
			note("Ada tabel/kolom yang tidak ditemukan (bagian 2): fitur terkait akan gagal atau kosong sampai nama disesuaikan.")
		}
	}

	// ---- 3. Ukuran & index tabel aset ----
	section("3. Tabel aset: ukuran dan index")
	var rowsEst, pages sql.NullInt64
	if err := db.QueryRowContext(ctx,
		`SELECT SUM(row_count), SUM(used_page_count) FROM sys.dm_db_partition_stats WHERE object_id = OBJECT_ID(@p1) AND index_id IN (0, 1)`,
		opt.AssetTable).Scan(&rowsEst, &pages); err != nil {
		warn("gagal membaca ukuran tabel (butuh izin VIEW DATABASE STATE): %v", err)
	} else if rowsEst.Valid {
		line("Perkiraan: %s baris, %.0f GB terpakai", grouped(rowsEst.Int64), float64(pages.Int64)*8/1024/1024)
	}

	indexes, err := loadIndexes(ctx, db, opt.AssetTable)
	switch {
	case err != nil:
		warn("gagal membaca index (butuh izin VIEW DEFINITION): %v", err)
	case len(indexes) == 0:
		warn("tabel %s tidak ditemukan di katalog", assetName)
	default:
		onlyHeap := true
		leading := map[string]bool{}
		for _, ix := range indexes {
			label := ix.typeDesc
			if ix.primary {
				label += ", PRIMARY KEY"
			}
			if ix.typeDesc == "HEAP" {
				line("- HEAP (tanpa index berkluster)")
				continue
			}
			onlyHeap = false
			line("- %s [%s]: kunci (%s)%s", orDash(ix.name), label, strings.Join(ix.keys, ", "), includedNote(ix.included))
			if len(ix.keys) > 0 {
				leading[strings.ToLower(ix.keys[0])] = true
			}
		}
		if onlyHeap {
			warn("tabel TANPA index. Pencarian kode register/KIB/nopol memindai seluruh tabel (bisa sampai batas 25 detik).")
			note("Tabel aset tanpa index: pencarian kode lambat. Minta DBA index pada kode_register (dan id_satker bila perlu) atau gunakan salinan lokal.")
		} else {
			for _, c := range []string{"kode_register", "no_kib", "no_polisi", "serial_number", "id_satker", "kd_kondisi"} {
				line("  kolom %-14s diindeks sebagai kunci pertama: %s", c, yesNo(leading[c]))
			}
		}
	}

	// ---- 4. Referensi ----
	section("4. Tabel referensi (kode -> nama)")
	for _, def := range sldk.RefDefs {
		rows, err := db.QueryContext(ctx, def.Query(sldk.SiblingTable(opt.AssetTable, def.Table)))
		if err != nil {
			warn("%s: gagal dibaca: %v", def.Key, err)
			continue
		}
		line("%s:", def.Key)
		n := 0
		for rows.Next() {
			var code, name sql.NullString
			if rows.Scan(&code, &name) == nil && n < 20 {
				line("    %-6s = %s", code.String, name.String)
				n++
			}
		}
		_ = rows.Close()
	}

	// ---- 5. Cakupan K/L ----
	section("5. Cakupan K/L")
	scoped := opt.Scope != ""
	if !scoped {
		line("SLDK_KL_KODE kosong: seluruh K/L dihitung dan boleh dicari.")
		var n int64
		if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", sldk.SiblingTable(opt.AssetTable, sldk.TableKL))).Scan(&n); err == nil {
			line("Ada %d K/L di tabel referensi. Kementerian Keuangan berkode 015; isi SLDK_KL_KODE=015 untuk membatasi.", n)
		}
	} else {
		var idKL sql.NullInt64
		var kdKL, urKL sql.NullString
		err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT TOP 1 [id_kl], [kd_kl], [ur_kl] FROM %s WHERE [kd_kl] = @p1", sldk.SiblingTable(opt.AssetTable, sldk.TableKL)), opt.Scope).Scan(&idKL, &kdKL, &urKL)
		if err != nil {
			warn("K/L dengan kode %q tidak ditemukan: %v", opt.Scope, err)
			note("SLDK_KL_KODE tidak cocok dengan K/L mana pun: sinkronisasi akan menghasilkan 0 aset.")
		} else {
			var satkers int64
			_ = db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", sldk.SiblingTable(opt.AssetTable, sldk.TableSatker), sldk.SatkerScopePredicate(opt.AssetTable, "@p1")), opt.Scope).Scan(&satkers)
			line("K/L %s = %s (id_kl %d), %s satker", kdKL.String, urKL.String, idKL.Int64, grouped(satkers))
		}
	}

	// ---- 6. Sampel ----
	section("6. Sampel halaman tabel aset (0,01%)")
	probeSample(ctx, db, opt, out, line, warn, note)

	// ---- Kesimpulan ----
	section("Kesimpulan")
	if len(notes) == 0 {
		line("Tidak ada masalah yang terdeteksi.")
	}
	for _, n := range notes {
		line("- %s", n)
	}

	section("Langkah berikutnya")
	if len(notes) > 0 {
		line("(setelah masalah di atas beres)")
	}
	line("1. pasti-sldk-sync ringkasan -dry-run   lihat rencana dan SQL-nya")
	line("2. pasti-sldk-sync ringkasan            jalankan di luar jam kerja; memindai tabel aset satu kali")
	line("3. Di halaman Ringkasan, admin memilih kunci penanda mana yang dihitung \"aset aktif\".")
}

func probeSample(ctx context.Context, db *sql.DB, opt probeOptions, out io.Writer,
	line, warn func(string, ...interface{}), note func(string)) {

	conn, err := db.Conn(ctx)
	if err != nil {
		warn("tidak bisa membuka koneksi untuk sampel: %v", err)
		return
	}
	defer conn.Close()

	// Tabel sementara (#) hanya hidup di satu koneksi, jadi semua langkah di bawah memakai koneksi yang sama.
	if _, err := conn.ExecContext(ctx, `SET TRANSACTION ISOLATION LEVEL READ UNCOMMITTED`); err != nil {
		warn("gagal mengatur isolasi baca: %v", err)
	}
	cols := make([]string, len(sldk.AggregateColumns))
	for i, c := range sldk.AggregateColumns {
		cols[i] = sldk.QuoteIdent(c)
	}
	if _, err := conn.ExecContext(ctx, fmt.Sprintf(
		"SELECT TOP (50000) %s INTO #sldk_sample FROM %s TABLESAMPLE (0.01 PERCENT)", strings.Join(cols, ", "), sldk.QuoteTableRef(opt.AssetTable))); err != nil {
		warn("gagal mengambil sampel: %v", err)
		note("Sampel tidak bisa diambil, jadi query agregat belum teruji (bagian 6).")
		return
	}
	var n int64
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM #sldk_sample`).Scan(&n); err != nil || n == 0 {
		warn("sampel kosong (%v). Tabel mungkin sangat kecil atau sedang dimuat ulang.", err)
		return
	}
	line("Sampel: %s baris (hanya gambaran kasar sebaran nilai, bukan angka resmi).", grouped(n))

	pct := func(x int64) string { return fmt.Sprintf("%.1f%%", float64(x)*100/float64(n)) }
	dist := func(label, expr string) {
		rows, err := conn.QueryContext(ctx, fmt.Sprintf(
			"SELECT TOP (12) CAST(%s AS nvarchar(60)), COUNT(*) FROM #sldk_sample GROUP BY %s ORDER BY COUNT(*) DESC", expr, expr))
		if err != nil {
			warn("%s: gagal: %v", label, err)
			return
		}
		defer rows.Close()
		var parts []string
		for rows.Next() {
			var v sql.NullString
			var c int64
			if rows.Scan(&v, &c) == nil {
				name := v.String
				if !v.Valid {
					name = "(kosong)"
				}
				parts = append(parts, fmt.Sprintf("%s=%s", name, pct(c)))
			}
		}
		line("%-18s %s", label+":", strings.Join(parts, "  "))
	}
	fmt.Fprintln(out, "  Sebaran nilai (persen sampel):")
	dist("status_data", "[status_data]")
	dist("sts_his", "[sts_his]")
	dist("sts_ast", "[sts_ast]")
	dist("tgl_hapus", "CASE WHEN [tgl_hapus] IS NULL THEN 'kosong' ELSE 'terisi' END")
	dist("kd_kondisi", "[kd_kondisi]")
	dist("kd_status", "[kd_status]")
	dist("kd_jns_bmn", "[kd_jns_bmn]")
	dist("status_bmn_idle", "[status_bmn_idle]")
	dist("brg_hilang_yn", "[brg_hilang_yn]")
	dist("dihentikan_yn", "[dihentikan_yn]")

	fmt.Fprintln(out, "  Aturan pemantauan (persen sampel yang terkena):")
	for _, r := range sldk.Rules {
		var hit sql.NullInt64
		if err := conn.QueryRowContext(ctx, fmt.Sprintf("SELECT SUM(CASE WHEN %s THEN 1 ELSE 0 END) FROM #sldk_sample", r.Predicate)).Scan(&hit); err != nil {
			warn("%s: predikat gagal dijalankan: %v", r.Key, err)
			continue
		}
		line("%-12s %s", r.Key+":", pct(hit.Int64))
	}

	// Keterkaitan id_satker M_ASET -> R_SATKER (asumsi cakupan K/L dan nama satker).
	var orphan int64
	satkerTbl := sldk.SiblingTable(opt.AssetTable, sldk.TableSatker)
	if err := conn.QueryRowContext(ctx, fmt.Sprintf(
		"SELECT COUNT(*) FROM #sldk_sample s WHERE s.[id_satker] IS NOT NULL AND NOT EXISTS (SELECT 1 FROM %s r WHERE r.[id_satker] = s.[id_satker])", satkerTbl)).Scan(&orphan); err != nil {
		warn("keterkaitan id_satker gagal diperiksa: %v", err)
	} else {
		line("id_satker aset yang tidak ada di R_SATKER: %s", pct(orphan))
		if float64(orphan)/float64(n) > 0.05 {
			warn("lebih dari 5%% aset tidak punya satker di R_SATKER: asumsi id_satker = id_satker mungkin salah.")
			note("id_satker di M_ASET tidak cocok dengan R_SATKER (bagian 6): nama satker dan cakupan K/L tidak bisa diandalkan.")
		}
	}

	var where string
	var args []interface{}
	if opt.Scope != "" {
		where = sldk.ScopePredicate(opt.AssetTable, "@p1")
		args = []interface{}{opt.Scope}
		var inScope int64
		if err := conn.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM #sldk_sample WHERE %s", where), opt.Scope).Scan(&inScope); err != nil {
			warn("cakupan K/L gagal diuji pada sampel: %v", err)
		} else {
			line("Aset dalam cakupan K/L %s: %s dari sampel", opt.Scope, pct(inScope))
			if inScope == 0 {
				warn("tidak ada aset sampel yang masuk cakupan K/L %s.", opt.Scope)
				note("Cakupan K/L tidak menangkap aset apa pun pada sampel: periksa SLDK_KL_KODE dan keterkaitan id_satker.")
			}
		}
	}

	// Query agregat yang SAMA dengan sinkronisasi, dijalankan pada sampel: membuktikan SQL-nya valid.
	rows, err := readAggregate(ctx, conn, sldk.AggregateSQL("[#sldk_sample]", where), args)
	if err != nil {
		warn("query agregat gagal pada sampel: %v", err)
		note("Query agregat gagal pada sampel (bagian 6): jangan jalankan `ringkasan` sebelum ini beres.")
		return
	}
	var flags []string
	for _, r := range rows {
		if r.dim == "total" {
			flags = append(flags, fmt.Sprintf("%s=%d", r.flagKey, r.jumlah))
		}
	}
	sort.Strings(flags)
	line("Query agregat valid: %d baris agregat dari sampel.", len(rows))
	line("Kunci penanda (status_data|sts_his|sts_ast|A=tgl_hapus kosong/H=terisi): %s", strings.Join(flags, "  "))
}

func loadIndexes(ctx context.Context, db *sql.DB, assetTable string) ([]indexInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT i.index_id, i.name, i.type_desc, i.is_primary_key, c.name, ic.key_ordinal, ic.is_included_column
		FROM sys.indexes i
		LEFT JOIN sys.index_columns ic ON ic.object_id = i.object_id AND ic.index_id = i.index_id
		LEFT JOIN sys.columns c ON c.object_id = ic.object_id AND c.column_id = ic.column_id
		WHERE i.object_id = OBJECT_ID(@p1)
		ORDER BY i.index_id, ic.key_ordinal`, assetTable)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byID := map[int64]*indexInfo{}
	var order []int64
	for rows.Next() {
		var id int64
		var name, typeDesc, col sql.NullString
		var primary sql.NullBool
		var ord sql.NullInt64
		var included sql.NullBool
		if err := rows.Scan(&id, &name, &typeDesc, &primary, &col, &ord, &included); err != nil {
			return nil, err
		}
		ix := byID[id]
		if ix == nil {
			ix = &indexInfo{name: name.String, typeDesc: typeDesc.String, primary: primary.Bool}
			byID[id] = ix
			order = append(order, id)
		}
		if col.Valid {
			if included.Bool {
				ix.included = append(ix.included, col.String)
			} else if ord.Int64 > 0 {
				ix.keys = append(ix.keys, col.String)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]indexInfo, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}

func includedNote(cols []string) string {
	if len(cols) == 0 {
		return ""
	}
	return fmt.Sprintf(", include (%d kolom)", len(cols))
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func yesNo(b bool) string {
	if b {
		return "ya"
	}
	return "tidak"
}

// grouped: 1234567 -> "1.234.567".
func grouped(n int64) string {
	s := fmt.Sprintf("%d", n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := strings.Join(parts, ".")
	if neg {
		return "-" + out
	}
	return out
}
