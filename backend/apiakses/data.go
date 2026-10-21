package apiakses

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"pasti-v3-backend/digitalisasi"
	"pasti-v3-backend/peran"
)

// Pembacaan data untuk klien. Nama tabel dan kolom hanya dari katalog (kode), nilai penyaring selalu parameter, dan hasilnya hanya kolom yang lolos kolomTertutup.

// Batas masukan penyaring.
const (
	MaksPencarian = 100
	MinPencarian  = 3
	maksWilayah   = 200
)

var reKodeSatker = regexp.MustCompile(`^[0-9A-Za-z.\-]{1,40}$`)

// FilterData: penyaring dan paginasi daftar data. Paginasi memakai kursor (id baris terakhir yang sudah diterima), bukan nomor halaman: stabil dan tetap cepat pada
// tabel besar, dan tidak melewatkan atau menggandakan baris bila data berubah di tengah penarikan.
type FilterData struct {
	Q          string
	KodeSatker string // 6 digit: satu satker; selain itu awalan kode satker lengkap
	KodeUE1    string // 5 digit
	Provinsi   string
	KabKota    string
	Kursor     int64
	PerHalaman int
	Hitung     bool
	UE1Klien   []string // pembatasan milik klien (bukan masukan klien)
}

// Validasi memeriksa masukan penyaring; pesan tidak memuat isian pemanggil.
func (f FilterData) Validasi() error {
	if f.Q != "" {
		n := len([]rune(f.Q))
		if n < MinPencarian {
			return fmt.Errorf("Kata pencarian (q) minimal %d karakter", MinPencarian)
		}
		if n > MaksPencarian {
			return fmt.Errorf("Kata pencarian (q) maksimal %d karakter", MaksPencarian)
		}
	}
	if f.KodeSatker != "" && !reKodeSatker.MatchString(f.KodeSatker) {
		return errors.New("kode_satker tidak valid")
	}
	if f.KodeUE1 != "" && !(len(f.KodeUE1) == peran.PanjangUE1 && angka(f.KodeUE1)) {
		return errors.New("kode_ue1 harus 5 digit angka")
	}
	if len([]rune(f.Provinsi)) > maksWilayah || len([]rune(f.KabKota)) > maksWilayah {
		return errors.New("Nama wilayah terlalu panjang")
	}
	if f.Kursor < 0 {
		return errors.New("kursor tidak valid")
	}
	return nil
}

func angka(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func kutip(nama string) string { return "[" + strings.ReplaceAll(nama, "]", "]]") + "]" }

// kondisiUE1 menyusun pembatasan UE1: salah satu kode UE1 pada daftar. Kode yang bukan 5 digit angka menutup semua baris (aman bila data konfigurasi rusak).
func kondisiUE1(kolomSatker string, kode []string) string {
	if len(kode) == 0 {
		return ""
	}
	var atau []string
	for _, k := range kode {
		atau = append(atau, "("+peran.Cakupan{Tingkat: peran.TingkatUE1, Kode: k}.KondisiSQL(kutip(kolomSatker))+")")
	}
	return "(" + strings.Join(atau, " OR ") + ")"
}

// where menyusun klausa WHERE (tanpa kata WHERE) dan argumennya. denganKursor: sertakan syarat id > kursor.
func (d DatasetAPI) where(f FilterData, denganKursor bool) (string, []interface{}) {
	var w []string
	var a []interface{}
	tambah := func(cond string, v interface{}) {
		a = append(a, v)
		w = append(w, strings.ReplaceAll(cond, "?", "@p"+strconv.Itoa(len(a))))
	}
	if denganKursor {
		tambah("id > ?", f.Kursor)
	}
	satker := d.KolomSatker()
	if c := kondisiUE1(satker, f.UE1Klien); c != "" {
		w = append(w, c)
	}
	if f.KodeUE1 != "" {
		w = append(w, "("+peran.Cakupan{Tingkat: peran.TingkatUE1, Kode: f.KodeUE1}.KondisiSQL(kutip(satker))+")")
	}
	if f.KodeSatker != "" {
		if len(f.KodeSatker) == peran.PanjangSatker && angka(f.KodeSatker) {
			w = append(w, "("+peran.Cakupan{Tingkat: peran.TingkatSatk, Kode: f.KodeSatker}.KondisiSQL(kutip(satker))+")")
		} else {
			tambah(kutip(satker)+` LIKE ? ESCAPE '\'`, escapeLike(f.KodeSatker)+"%")
		}
	}
	if f.Provinsi != "" && d.KolomProvinsi() != "" {
		tambah(kutip(d.KolomProvinsi())+" = ?", f.Provinsi)
	}
	if f.KabKota != "" && d.KolomKabKota() != "" {
		tambah(kutip(d.KolomKabKota())+" = ?", f.KabKota)
	}
	if f.Q != "" {
		if kol := d.KolomPencarian(); len(kol) > 0 {
			a = append(a, "%"+escapeLike(f.Q)+"%")
			ph := "@p" + strconv.Itoa(len(a))
			var atau []string
			for _, k := range kol {
				atau = append(atau, kutip(k)+` LIKE `+ph+` ESCAPE '\'`)
			}
			w = append(w, "("+strings.Join(atau, " OR ")+")")
		}
	}
	return strings.Join(w, " AND "), a
}

// daftarKolom: daftar kolom SELECT: id, kolom yang keluar, dan waktu sinkronisasi.
func (d DatasetAPI) daftarKolom() string {
	c := []string{"id"}
	for _, k := range d.Kolom {
		c = append(c, kutip(k.kolom))
	}
	c = append(c, "[synced_at]")
	return strings.Join(c, ", ")
}

// HasilData: satu halaman data.
type HasilData struct {
	Baris            []map[string]interface{}
	KursorBerikutnya int64
	AdaBerikutnya    bool
	Total            *int64
}

// BacaDaftar membaca satu halaman data dataset d menurut penyaring.
func (d DatasetAPI) BacaDaftar(ctx context.Context, db *sql.DB, f FilterData, halamanMaks int) (HasilData, error) {
	if err := f.Validasi(); err != nil {
		return HasilData{}, err
	}
	per := f.PerHalaman
	if per < 1 {
		per = 100
	}
	if halamanMaks < 1 {
		halamanMaks = 500
	}
	if per > halamanMaks {
		per = halamanMaks
	}
	var hasil HasilData
	if f.Hitung {
		w, a := d.where(f, false)
		q := "SELECT COUNT_BIG(*) FROM " + kutip(d.Tabel())
		if w != "" {
			q += " WHERE " + w
		}
		var n int64
		if err := db.QueryRowContext(ctx, q, a...).Scan(&n); err != nil {
			return hasil, err
		}
		hasil.Total = &n
	}
	w, a := d.where(f, true)
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT TOP (%d) %s FROM %s WHERE %s ORDER BY id", per+1, d.daftarKolom(), kutip(d.Tabel()), w), a...)
	if err != nil {
		return hasil, err
	}
	defer rows.Close()
	hasil.Baris = []map[string]interface{}{}
	for rows.Next() {
		b, id, err := d.pindai(rows)
		if err != nil {
			return hasil, err
		}
		if len(hasil.Baris) == per {
			hasil.AdaBerikutnya = true
			break
		}
		hasil.Baris = append(hasil.Baris, b)
		hasil.KursorBerikutnya = id
	}
	if err := rows.Err(); err != nil {
		return hasil, err
	}
	if !hasil.AdaBerikutnya {
		hasil.KursorBerikutnya = 0
	}
	return hasil, nil
}

// BacaDetail membaca satu baris menurut id; ErrTidakAda bila tidak ada atau di luar pembatasan UE1 klien.
func (d DatasetAPI) BacaDetail(ctx context.Context, db *sql.DB, id int64, ue1Klien []string) (map[string]interface{}, error) {
	w, a := d.where(FilterData{UE1Klien: ue1Klien}, false)
	a = append(a, id)
	cond := "id = @p" + strconv.Itoa(len(a))
	if w != "" {
		cond = w + " AND " + cond
	}
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE %s", d.daftarKolom(), kutip(d.Tabel()), cond), a...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, ErrTidakAda
	}
	b, _, err := d.pindai(rows)
	return b, err
}

// pindai membaca satu baris hasil menjadi peta nama JSON -> nilai.
func (d DatasetAPI) pindai(r *sql.Rows) (map[string]interface{}, int64, error) {
	n := len(d.Kolom) + 2
	nilai := make([]interface{}, n)
	ptr := make([]interface{}, n)
	for i := range nilai {
		ptr[i] = &nilai[i]
	}
	if err := r.Scan(ptr...); err != nil {
		return nil, 0, err
	}
	id, _ := nilai[0].(int64)
	b := make(map[string]interface{}, n)
	b["id"] = id
	for i, k := range d.Kolom {
		b[k.Nama] = nilaiKolom(k.jenis, nilai[i+1])
	}
	if t, ok := nilai[n-1].(time.Time); ok {
		b["disinkronkan_pada"] = t.UTC().Format(time.RFC3339)
	}
	return b, id, nil
}

// nilaiKolom mengubah nilai driver menjadi nilai JSON: teks dari byte, desimal sebagai bilangan, NULL tetap null.
func nilaiKolom(jenis digitalisasi.Kind, v interface{}) interface{} {
	switch x := v.(type) {
	case nil:
		return nil
	case []byte:
		s := string(x)
		if jenis == digitalisasi.Decimal || jenis == digitalisasi.Coord {
			if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
				return f
			}
		}
		return s
	case string:
		if jenis == digitalisasi.Decimal || jenis == digitalisasi.Coord {
			if f, err := strconv.ParseFloat(strings.TrimSpace(x), 64); err == nil {
				return f
			}
		}
		return x
	case time.Time:
		return x.UTC().Format(time.RFC3339)
	}
	return v
}

// TerakhirSinkron: waktu sinkronisasi sukses terakhir dataset (nil bila belum pernah), dari riwayat sinkronisasi Digitalisasi Aset.
func (d DatasetAPI) TerakhirSinkron(ctx context.Context, db *sql.DB) (*time.Time, error) {
	var t sql.NullTime
	err := db.QueryRowContext(ctx, `SELECT TOP 1 selesai FROM digitalisasi_sync_log WHERE dataset = @p1 AND status = N'sukses' ORDER BY id DESC`, d.Kunci()).Scan(&t)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return waktuPtr(t), nil
}
