package digitalisasi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Sinkronisasi menyalin satu tabel DIGITALISASI_* dari DATAMART ke database PASTI. Aplikasi tidak pernah mengakses SLDK: datamart-lah yang membaca SLDK, dengan script
// backend/datamart/buat_datamart_digitalisasi.sql (docs/datamart-digitalisasi.md), yang menyaring dan mengonversi data persis seperti yang dahulu dilakukan sinkronisasi dari SLDK.
// Karena itu nilai di datamart sudah bertipe dan berpanjang sama dengan tabel PASTI, dan disalin APA ADANYA (tanpa melewati float64).

// DefaultMaxRows membatasi jumlah baris yang ditahan di memori per dataset. Seluruh hasil dibaca dulu dari datamart, baru ditulis ke PASTI dalam satu transaksi singkat.
const DefaultMaxRows = 500_000

// Result: ringkasan satu sinkronisasi dataset.
type Result struct {
	Rows       int
	WithCoords int
	Duration   time.Duration
	// DataPer: awal penyegaran datamart yang menghasilkan data ini (UTC).
	DataPer time.Time
	// SamaDenganSebelumnya: DataPer sama dengan sinkronisasi sukses terakhir, artinya datamart belum disegarkan lagi sejak itu.
	SamaDenganSebelumnya bool
}

// Summary: kalimat untuk kolom pesan di riwayat.
func (r Result) Summary() string {
	parts := []string{fmt.Sprintf("%d baris", r.Rows)}
	if r.WithCoords > 0 || r.Rows > 0 {
		parts = append(parts, fmt.Sprintf("%d berkoordinat", r.WithCoords))
	}
	return strings.Join(parts, ", ") + fmt.Sprintf(" (%s)", r.Duration.Round(time.Second))
}

// Options: pengaturan satu kali sinkronisasi.
type Options struct {
	MaxRows  int
	RunID    int64            // dicatat di kolom id_sinkron
	Progress func(msg string) // dipanggil berkala; boleh nil
	// Sebelumnya: awal penyegaran datamart (UTC) dari sinkronisasi sukses terakhir dataset ini; nil bila belum pernah. Datamart yang LEBIH LAMA daripada ini ditolak supaya data
	// PASTI tidak mundur (mis. datamart dipulihkan dari cadangan lama).
	Sebelumnya *time.Time
}

func quoteIdent(name string) string { return "[" + strings.ReplaceAll(name, "]", "]]") + "]" }

func (o Options) progress(msg string) {
	if o.Progress != nil {
		o.Progress(msg)
	}
}

// IndeksKoordinat: posisi kolom Latitude/Longitude pada Columns (-1 bila dataset tanpa koordinat).
func (d Dataset) IndeksKoordinat() (lat, lng int) {
	lat, lng = -1, -1
	for i, c := range d.Columns {
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

// Sync membaca satu tabel DIGITALISASI_* dari datamart lalu mengganti isi tabel tujuannya di PASTI. Data lama baru dihapus setelah seluruh hasil terbaca dan siap ditulis; bila
// ada yang gagal, tabel tidak berubah.
func Sync(ctx context.Context, datamartDB, pastiDB *sql.DB, ds Dataset, opt Options) (Result, error) {
	start := time.Now()
	if opt.MaxRows <= 0 {
		opt.MaxRows = DefaultMaxRows
	}

	opt.progress("Membaca dari datamart...")
	data, runDatamart, err := bacaDatamart(ctx, datamartDB, ds, opt)
	if err != nil {
		return Result{}, err
	}
	dataPer, err := dataPerDatamart(ctx, datamartDB, runDatamart)
	if err != nil {
		return Result{}, err
	}

	res := Result{Rows: len(data), DataPer: dataPer}
	if opt.Sebelumnya != nil {
		sebelum := opt.Sebelumnya.UTC()
		if dataPer.Before(sebelum) {
			return Result{}, fmt.Errorf("datamart tidak dipakai: datamart (data per %s) lebih lama daripada data PASTI saat ini (data per %s); segarkan datamart dulu", waktuWIB(dataPer), waktuWIB(sebelum))
		}
		res.SamaDenganSebelumnya = dataPer.Equal(sebelum)
	}
	latIdx, lngIdx := ds.IndeksKoordinat()
	if latIdx >= 0 {
		for _, row := range data {
			if row[latIdx] != nil && row[lngIdx] != nil {
				res.WithCoords++
			}
		}
	}

	// Hasil kosong tidak boleh menimpa data yang ada: itu hampir pasti kegagalan di hulu (datamart baru dibuat dan belum terisi, akses, filter), bukan kenyataan bahwa semua aset hilang.
	if len(data) == 0 {
		var existing int64
		if err := pastiDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteIdent(ds.Table)).Scan(&existing); err != nil {
			return Result{}, fmt.Errorf("gagal memeriksa isi tabel %s: %w", ds.Table, err)
		}
		if existing > 0 {
			return Result{}, errors.New("datamart mengembalikan 0 baris sedangkan tabel sudah berisi data; data lama TIDAK diganti. Periksa penyegaran datamart dan aksesnya")
		}
	}

	opt.progress(fmt.Sprintf("Menyimpan %d baris ke database PASTI...", len(data)))
	if err := GantiIsiTabel(ctx, pastiDB, ds, data, opt.RunID); err != nil {
		return Result{}, err
	}
	res.Duration = time.Since(start)
	return res, nil
}

// nilaiDatamart menormalkan nilai yang dibaca dari kolom datamart agar dapat ditulis apa adanya ke kolom PASTI yang bertipe sama: teks dan bilangan bulat tetap, DECIMAL (datang
// dari driver sebagai []byte) menjadi teks angka yang dikonversi SQL Server tanpa melalui float. Tipe lain berarti struktur datamart tidak sama dengan yang diharapkan.
func nilaiDatamart(c Column, v interface{}) (interface{}, error) {
	switch x := v.(type) {
	case nil:
		return nil, nil
	case string:
		return x, nil
	case []byte:
		return string(x), nil
	case int64:
		return x, nil
	}
	return nil, fmt.Errorf("kolom %s bertipe tak terduga (%T); struktur tabel datamart tidak sama dengan migrasi 020", c.Name, v)
}

// bacaDatamart membaca seluruh isi tabel DIGITALISASI_* datamart menurut urutan id. Mengembalikan baris (kolom data saja, urutan ds.Columns) dan nomor proses penyegaran
// datamart yang menulisnya (id_sinkron terbesar; 0 bila kosong).
func bacaDatamart(ctx context.Context, db *sql.DB, ds Dataset, opt Options) ([][]interface{}, int64, error) {
	names := make([]string, 0, len(ds.Columns)+1)
	for _, c := range ds.Columns {
		names = append(names, quoteIdent(c.Name))
	}
	names = append(names, quoteIdent("id_sinkron"))

	rows, err := db.QueryContext(ctx, "SELECT "+strings.Join(names, ", ")+" FROM dbo."+quoteIdent(ds.Table)+" ORDER BY [id]")
	if err != nil {
		return nil, 0, fmt.Errorf("gagal membaca tabel %s di datamart: %w", ds.Table, err)
	}
	defer rows.Close()

	raw := make([]interface{}, len(names))
	ptr := make([]interface{}, len(names))
	for i := range raw {
		ptr[i] = &raw[i]
	}
	var data [][]interface{}
	var run int64
	for rows.Next() {
		if len(data) >= opt.MaxRows {
			return nil, 0, fmt.Errorf("tabel %s di datamart melebihi %d baris; sinkronisasi dihentikan", ds.Table, opt.MaxRows)
		}
		if err := rows.Scan(ptr...); err != nil {
			return nil, 0, fmt.Errorf("gagal membaca baris %s: %w", ds.Table, err)
		}
		row := make([]interface{}, len(ds.Columns))
		for i, c := range ds.Columns {
			if row[i], err = nilaiDatamart(c, raw[i]); err != nil {
				return nil, 0, err
			}
		}
		if id, ok := raw[len(ds.Columns)].(int64); ok && id > run {
			run = id
		}
		data = append(data, row)
		if len(data)%20000 == 0 {
			opt.progress(fmt.Sprintf("Membaca dari datamart... %d baris", len(data)))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("pembacaan %s dari datamart terputus: %w", ds.Table, err)
	}
	return data, run, nil
}

// dataPerDatamart: awal penyegaran datamart (UTC) yang menulis baris-baris yang baru dibaca (run = id_sinkron). Bila tabel kosong (run = 0), penyegaran sukses terakhir.
func dataPerDatamart(ctx context.Context, db *sql.DB, run int64) (time.Time, error) {
	var t sql.NullTime
	var err error
	if run > 0 {
		err = db.QueryRowContext(ctx, `SELECT mulai FROM dbo.datamart_digitalisasi_run WHERE id = @p1`, run).Scan(&t)
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, fmt.Errorf("jejak penyegaran datamart #%d tidak ditemukan di dbo.datamart_digitalisasi_run", run)
		}
	} else {
		err = db.QueryRowContext(ctx, `SELECT MAX(mulai) FROM dbo.datamart_digitalisasi_run WHERE status = N'sukses'`).Scan(&t)
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("tidak dapat memeriksa kesegaran datamart: %w", err)
	}
	if !t.Valid {
		return time.Time{}, errors.New("datamart belum pernah berhasil disegarkan (belum ada penyegaran sukses di dbo.datamart_digitalisasi_run)")
	}
	return t.Time.UTC(), nil
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

// GantiIsiTabel mengosongkan tabel tujuan di PASTI lalu mengisinya dengan data (urutan kolom ds.Columns) dalam satu transaksi. Kolom yang diisi manual di PASTI (ds.Keep)
// dipertahankan per ds.KeyColumn bila sumber mengirim NULL. Diekspor untuk paket acuan, yang membentuk "tabel yang seharusnya" bagi tes perbandingan dengan datamart.
func GantiIsiTabel(ctx context.Context, db *sql.DB, ds Dataset, data [][]interface{}, runID int64) error {
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
