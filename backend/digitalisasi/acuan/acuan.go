// Package acuan memuat implementasi ACUAN pembacaan SLDK untuk Digitalisasi Aset: menjalankan ketujuh query asli (digitalisasi/queries) terhadap SLDK dan mengubah hasilnya ke
// nilai kolom tabel DIGITALISASI_* (pangkas spasi, kosong menjadi NULL, potong sesuai panjang kolom, angka berkoma desimal, koordinat divalidasi dan berpasangan).
//
// Server TIDAK memakai paket ini: aplikasi membaca data hanya dari tabel DIGITALISASI_* di datamart (digitalisasi.Sync) dan tidak pernah terhubung ke SLDK untuk sinkronisasi.
// Paket ini ada sebagai definisi yang dapat dijalankan dari "data yang benar" itu: script datamart (backend/datamart/buat_datamart_digitalisasi.sql) dihasilkan agar isinya sama
// dengan keluaran paket ini, dan tes di backend/datamart membandingkannya baris demi baris. Tes di paket ini menjaga agar tidak ada kode server yang mengimpornya.
package acuan

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"pasti-v3-backend/digitalisasi"
)

// Baca menjalankan query asli dataset ds pada SATU koneksi ke SLDK (query memakai tabel # sementara yang hanya hidup per sesi) dan memetakan kolom hasilnya ke kolom tujuan
// menurut nama, lengkap dengan konversi nilai dan koordinat. maksBaris <= 0 berarti tanpa batas.
func Baca(ctx context.Context, db *sql.DB, ds digitalisasi.Dataset, maksBaris int) ([][]interface{}, Statistik, error) {
	var st Statistik
	query, err := ds.Query()
	if err != nil {
		return nil, st, fmt.Errorf("query %s tidak ditemukan: %w", ds.QueryFile, err)
	}
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

	latIdx, lngIdx := ds.IndeksKoordinat()
	raw := make([]interface{}, len(cols))
	ptr := make([]interface{}, len(cols))
	for i := range raw {
		ptr[i] = &raw[i]
	}

	var data [][]interface{}
	for rows.Next() {
		if maksBaris > 0 && len(data) >= maksBaris {
			return nil, st, fmt.Errorf("hasil query melebihi %d baris", maksBaris)
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

// Sinkron membaca dataset dari SLDK (Baca) lalu mengganti isi tabel PASTI-nya dengan cara yang sama seperti sinkronisasi dari datamart (digitalisasi.GantiIsiTabel). Hasilnya
// adalah "tabel yang seharusnya" untuk dibandingkan dengan tabel yang dibentuk script datamart. Mengembalikan jumlah baris.
func Sinkron(ctx context.Context, sldkDB, pastiDB *sql.DB, ds digitalisasi.Dataset, runID int64) (int, error) {
	data, _, err := Baca(ctx, sldkDB, ds, 0)
	if err != nil {
		return 0, err
	}
	if err := digitalisasi.GantiIsiTabel(ctx, pastiDB, ds, data, runID); err != nil {
		return 0, err
	}
	return len(data), nil
}
