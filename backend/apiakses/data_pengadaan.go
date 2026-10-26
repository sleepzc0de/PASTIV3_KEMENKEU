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

	"pasti-v3-backend/peran"
)

// Pembacaan data pengadaan untuk klien. Nama tabel dan kolom hanya dari katalog (kode), nilai penyaring selalu parameter SQL, dan hasilnya hanya kolom yang ada di daftar
// keluar (katalog_pengadaan.go).
//
// Beda dengan dataset aset: tabel Inaproc tidak punya id bilangan, jadi id baris dan kursor berupa kunci hash baris (teks buram), dan paginasi memakai kursor keyset pada
// kunci itu. Kode satker Inaproc hanya 6 digit tanpa UE1, jadi pembatasan dan penyaring UE1/satker diserahkan pada BatasSatker (aturan yang sama dengan halaman Pengadaan Terpadu).

// BatasSatker menyusun potongan WHERE yang membatasi baris tabel pengadaan ke cakupan satker/UE1 (handlers.kondisiSatkerInaproc). ok=false bila tabel tidak dapat dibatasi.
// Potongan memakai nama tabel apa adanya sebagai awalan kolom, jadi query harus memakai "FROM [tabel]" tanpa alias.
type BatasSatker func(tabel string, cak peran.Cakupan) (kondisi string, ok bool)

var (
	reKodeKLPDPengadaan = regexp.MustCompile(`^[0-9A-Za-z_.\-]{1,20}$`)
	reTahunPengadaan    = regexp.MustCompile(`^(19|20)[0-9]{2}$`)
	reKunciBaris        = regexp.MustCompile(`^[0-9A-Za-z_.:\-]{1,255}$`)
)

// FilterPengadaan: penyaring dan paginasi daftar data pengadaan.
type FilterPengadaan struct {
	Q          string
	KodeKLPD   string // kode KLPD Inaproc (mis. K10)
	Tahun      string // tahun anggaran, empat digit
	KodeSatker string // kode satker 6 digit
	KodeUE1    string // 5 digit
	Kursor     string // id baris terakhir yang sudah diterima
	PerHalaman int
	Hitung     bool

	UE1Klien []string    // pembatasan milik klien (bukan masukan klien)
	Batas    BatasSatker // aturan pembatasan per satker; nil = penyaring dan pembatasan UE1/satker menutup semua baris
}

// Validasi memeriksa bentuk masukan; pesan tidak memuat isian pemanggil.
func (f FilterPengadaan) Validasi() error {
	if f.Q != "" {
		n := len([]rune(f.Q))
		if n < MinPencarian {
			return fmt.Errorf("Kata pencarian (q) minimal %d karakter", MinPencarian)
		}
		if n > MaksPencarian {
			return fmt.Errorf("Kata pencarian (q) maksimal %d karakter", MaksPencarian)
		}
	}
	if f.KodeKLPD != "" && !reKodeKLPDPengadaan.MatchString(f.KodeKLPD) {
		return errors.New("kode_klpd tidak valid")
	}
	if f.Tahun != "" && !reTahunPengadaan.MatchString(f.Tahun) {
		return errors.New("tahun harus empat digit (mis. 2025)")
	}
	if f.KodeSatker != "" && !(len(f.KodeSatker) == peran.PanjangSatker && angka(f.KodeSatker)) {
		return errors.New("kode_satker untuk dataset pengadaan harus 6 digit angka")
	}
	if f.KodeUE1 != "" && !(len(f.KodeUE1) == peran.PanjangUE1 && angka(f.KodeUE1)) {
		return errors.New("kode_ue1 harus 5 digit angka")
	}
	if f.Kursor != "" && !reKunciBaris.MatchString(f.Kursor) {
		return errors.New("kursor tidak valid")
	}
	return nil
}

// Periksa memastikan penyaring yang diminta ada pada dataset ini: penyaring yang tidak dikenal dataset ditolak, bukan diabaikan diam-diam (hasil tanpa penyaring yang
// diharapkan pemanggil lebih berbahaya daripada galat).
func (d DatasetPengadaan) Periksa(f FilterPengadaan) error {
	if err := f.Validasi(); err != nil {
		return err
	}
	if f.Tahun != "" && d.kolomTahun == "" {
		return errors.New("Dataset ini tidak punya penyaring tahun")
	}
	if f.KodeKLPD != "" && d.kolomKLPD == "" {
		return errors.New("Dataset ini tidak punya penyaring kode_klpd")
	}
	if f.Q != "" && len(d.kolomCari) == 0 {
		return errors.New("Dataset ini tidak mendukung pencarian (q)")
	}
	return nil
}

// kondisiCakupan: pembatasan baris tabel ke satu cakupan; menutup semua baris bila aturan tidak ada atau tabel tidak dapat dibatasi.
func (d DatasetPengadaan) kondisiCakupan(batas BatasSatker, cak peran.Cakupan) string {
	if batas == nil {
		return "1 = 0"
	}
	k, ok := batas(d.tabel, cak)
	if !ok || strings.TrimSpace(k) == "" {
		return "1 = 0"
	}
	return k
}

// where menyusun klausa WHERE (tanpa kata WHERE) dan argumennya. denganKursor: sertakan syarat kunci > kursor.
func (d DatasetPengadaan) where(f FilterPengadaan, denganKursor bool) (string, []interface{}) {
	var w []string
	var a []interface{}
	tambah := func(cond string, v interface{}) {
		a = append(a, v)
		w = append(w, strings.ReplaceAll(cond, "?", "@p"+strconv.Itoa(len(a))))
	}
	if denganKursor && f.Kursor != "" {
		tambah(kutip(d.kunciBaris)+" > ?", f.Kursor)
	}
	if len(f.UE1Klien) > 0 {
		var atau []string
		for _, k := range f.UE1Klien {
			atau = append(atau, "("+d.kondisiCakupan(f.Batas, peran.Cakupan{Tingkat: peran.TingkatUE1, Kode: k})+")")
		}
		w = append(w, "("+strings.Join(atau, " OR ")+")")
	}
	if f.KodeUE1 != "" {
		w = append(w, "("+d.kondisiCakupan(f.Batas, peran.Cakupan{Tingkat: peran.TingkatUE1, Kode: f.KodeUE1})+")")
	}
	if f.KodeSatker != "" {
		w = append(w, "("+d.kondisiCakupan(f.Batas, peran.Cakupan{Tingkat: peran.TingkatSatk, Kode: f.KodeSatker})+")")
	}
	if f.KodeKLPD != "" && d.kolomKLPD != "" {
		tambah(kutip(d.kolomKLPD)+" = ?", f.KodeKLPD)
	}
	if f.Tahun != "" && d.kolomTahun != "" {
		tambah(kutip(d.kolomTahun)+" = ?", f.Tahun)
	}
	if f.Q != "" && len(d.kolomCari) > 0 {
		a = append(a, "%"+escapeLike(f.Q)+"%")
		ph := "@p" + strconv.Itoa(len(a))
		var atau []string
		for _, k := range d.kolomCari {
			atau = append(atau, kutip(k)+" LIKE "+ph+` ESCAPE '\'`)
		}
		w = append(w, "("+strings.Join(atau, " OR ")+")")
	}
	return strings.Join(w, " AND "), a
}

// daftarKolom: daftar kolom SELECT: kunci baris, kolom yang keluar, dan waktu pembaruan tabel.
func (d DatasetPengadaan) daftarKolom() string {
	c := []string{kutip(d.kunciBaris)}
	for _, k := range d.Kolom {
		c = append(c, kutip(k.kolom))
	}
	c = append(c, "[updated_at]")
	return strings.Join(c, ", ")
}

// HasilPengadaan: satu halaman data pengadaan.
type HasilPengadaan struct {
	Baris            []map[string]interface{}
	KursorBerikutnya string
	AdaBerikutnya    bool
	Total            *int64
}

// BacaDaftar membaca satu halaman data dataset d menurut penyaring. Urutan menurut kunci baris, jadi kursor stabil walau data berubah di tengah penarikan.
func (d DatasetPengadaan) BacaDaftar(ctx context.Context, db *sql.DB, f FilterPengadaan, halamanMaks int) (HasilPengadaan, error) {
	if err := d.Periksa(f); err != nil {
		return HasilPengadaan{}, err
	}
	per := f.PerHalaman
	if per < 1 {
		per = HalamanBawaan
	}
	if halamanMaks < 1 {
		halamanMaks = 500
	}
	if per > halamanMaks {
		per = halamanMaks
	}
	var hasil HasilPengadaan
	if f.Hitung {
		w, a := d.where(f, false)
		q := "SELECT COUNT_BIG(*) FROM " + kutip(d.tabel)
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
	q := fmt.Sprintf("SELECT TOP (%d) %s FROM %s", per+1, d.daftarKolom(), kutip(d.tabel))
	if w != "" {
		q += " WHERE " + w
	}
	q += " ORDER BY " + kutip(d.kunciBaris)
	rows, err := db.QueryContext(ctx, q, a...)
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
		hasil.KursorBerikutnya = ""
	}
	return hasil, nil
}

// BacaDetail membaca satu baris menurut id (kunci baris); ErrTidakAda bila tidak ada atau di luar pembatasan UE1 klien. id harus berbentuk kunci baris yang sah.
func (d DatasetPengadaan) BacaDetail(ctx context.Context, db *sql.DB, id string, ue1Klien []string, batas BatasSatker) (map[string]interface{}, error) {
	if !reKunciBaris.MatchString(id) {
		return nil, ErrTidakAda
	}
	w, a := d.where(FilterPengadaan{UE1Klien: ue1Klien, Batas: batas}, false)
	a = append(a, id)
	cond := kutip(d.kunciBaris) + " = @p" + strconv.Itoa(len(a))
	if w != "" {
		cond = w + " AND " + cond
	}
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE %s", d.daftarKolom(), kutip(d.tabel), cond), a...)
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

// pindai membaca satu baris hasil menjadi peta nama JSON -> nilai, beserta kunci barisnya.
func (d DatasetPengadaan) pindai(r *sql.Rows) (map[string]interface{}, string, error) {
	n := len(d.Kolom) + 2
	nilai := make([]interface{}, n)
	ptr := make([]interface{}, n)
	for i := range nilai {
		ptr[i] = &nilai[i]
	}
	if err := r.Scan(ptr...); err != nil {
		return nil, "", err
	}
	var id string
	switch x := nilai[0].(type) {
	case string:
		id = x
	case []byte:
		id = string(x)
	}
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

// TerakhirSinkron: waktu pembaruan terakhir tabel (nil bila kosong). Pembaruan tiap penarikan menyentuh updated_at, termasuk baris yang tidak berubah pada dataset RUP lama.
func (d DatasetPengadaan) TerakhirSinkron(ctx context.Context, db *sql.DB) (*time.Time, error) {
	var t sql.NullTime
	if err := db.QueryRowContext(ctx, "SELECT MAX([updated_at]) FROM "+kutip(d.tabel)).Scan(&t); err != nil {
		return nil, err
	}
	return waktuPtr(t), nil
}
