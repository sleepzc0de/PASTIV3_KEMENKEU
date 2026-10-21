package apiakses

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Pencatatan dan pembacaan pemakaian API: riwayat permintaan (api_akses_log) dan hitungan per jam (api_akses_agregat).

// Hasil permintaan (nilai kolom hasil).
const (
	HasilOK           = "ok"
	HasilGalatKlien   = "galat_klien"   // 4xx dari penanganan (parameter salah, tidak ditemukan, ...)
	HasilGalatServer  = "galat_server"  // 5xx
	HasilBatasLaju    = "batas_laju"    // 429: batas laju terlampaui
	HasilKuotaHabis   = "kuota_habis"   // 429: kuota habis
	HasilDitolakIP    = "ditolak_ip"    // 403: alamat tidak ada dalam allowlist
	HasilTidakSah     = "tidak_sah"     // 401: kunci tidak dikenal, salah, kedaluwarsa, dicabut, atau klien tidak aktif (sengaja tidak dibedakan keluar)
	HasilDitolakScope = "ditolak_scope" // 403: dataset di luar scope klien
)

// Opsi: pasangan kode dan label untuk pilihan penyaring di halaman pemantauan.
type Opsi struct {
	Kode  string `json:"kode"`
	Label string `json:"label"`
}

var urutanHasil = []Opsi{
	{HasilOK, "Berhasil"}, {HasilGalatKlien, "Galat permintaan (4xx)"}, {HasilGalatServer, "Galat server (5xx)"}, {HasilBatasLaju, "Batas laju terlampaui (429)"},
	{HasilKuotaHabis, "Kuota habis (429)"}, {HasilTidakSah, "Kunci tidak sah (401)"}, {HasilDitolakIP, "Alamat IP ditolak (403)"}, {HasilDitolakScope, "Dataset di luar scope (403)"},
}

// DaftarHasil: semua nilai hasil beserta labelnya.
func DaftarHasil() []Opsi { return append([]Opsi(nil), urutanHasil...) }

// HasilSah: apakah h salah satu nilai hasil.
func HasilSah(h string) bool {
	for _, o := range urutanHasil {
		if o.Kode == h {
			return true
		}
	}
	return false
}

// DaftarEndpoint: endpoint API klien (pola rute tanpa awalan) beserta uraiannya.
func DaftarEndpoint() []Opsi {
	return []Opsi{
		{"/katalog", "Katalog dataset"}, {"/aset/:dataset", "Daftar data aset"}, {"/aset/:dataset/:id", "Detail data aset"},
		{"/saya", "Profil dan batas klien"}, {"/saya/penggunaan", "Pemakaian klien"},
	}
}

// HasilDariStatus: hasil bawaan menurut kode HTTP.
func HasilDariStatus(status int) string {
	switch {
	case status >= 500:
		return HasilGalatServer
	case status >= 400:
		return HasilGalatKlien
	}
	return HasilOK
}

// Penggunaan: satu permintaan klien.
type Penggunaan struct {
	ID          int64     `json:"id,omitempty"`
	Waktu       time.Time `json:"waktu"`
	KlienID     int64     `json:"klien_id,omitempty"` // 0: kunci tidak dikenal
	KlienKode   string    `json:"klien_kode,omitempty"`
	KlienNama   string    `json:"klien_nama,omitempty"`
	KunciPrefix string    `json:"kunci_prefix,omitempty"`
	RequestID   string    `json:"request_id,omitempty"`
	Metode      string    `json:"metode"`
	Endpoint    string    `json:"endpoint"`
	Dataset     string    `json:"dataset,omitempty"`
	Status      int       `json:"status_http"`
	Hasil       string    `json:"hasil"`
	DurasiMS    int       `json:"durasi_ms"`
	Baris       *int      `json:"baris,omitempty"`
	IP          string    `json:"ip,omitempty"`
	UserAgent   string    `json:"user_agent,omitempty"`
}

// Batas atas (ms) tiap ember histogram latensi; ember terakhir = di atas yang terakhir.
var batasLatensi = [6]int{50, 100, 250, 500, 1000, 2500}

func emberLatensi(ms int) int {
	for i, b := range batasLatensi {
		if ms <= b {
			return i
		}
	}
	return len(batasLatensi)
}

// Agregat: hitungan satu (jam, klien, endpoint, status, hasil).
type Agregat struct {
	Jam      time.Time
	KlienID  int64
	Endpoint string
	Status   int
	Hasil    string
	Jumlah   int64
	Baris    int64
	LatTotal int64
	LatMaks  int
	Ember    [7]int
}

type kunciAgregat struct {
	jam      int64
	klien    int64
	endpoint string
	status   int
	hasil    string
}

func (a *Agregat) tambah(p Penggunaan) {
	a.Jumlah++
	if p.Baris != nil {
		a.Baris += int64(*p.Baris)
	}
	a.LatTotal += int64(p.DurasiMS)
	if p.DurasiMS > a.LatMaks {
		a.LatMaks = p.DurasiMS
	}
	a.Ember[emberLatensi(p.DurasiMS)]++
}

func (a *Agregat) gabung(b *Agregat) {
	a.Jumlah += b.Jumlah
	a.Baris += b.Baris
	a.LatTotal += b.LatTotal
	if b.LatMaks > a.LatMaks {
		a.LatMaks = b.LatMaks
	}
	for i := range a.Ember {
		a.Ember[i] += b.Ember[i]
	}
}

// ---------------------------------------------------------------- penulisan

const sqlSimpanLog = `INSERT INTO api_akses_log (waktu, klien_id, kunci_prefix, request_id, metode, endpoint, dataset, status_http, hasil, durasi_ms, baris, ip, user_agent)
	VALUES (CAST(@p1 AS DATETIME2(3)), @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9, @p10, @p11, @p12, @p13)`

// SimpanLog menulis sekelompok riwayat permintaan dalam satu transaksi.
func (s *Store) SimpanLog(ctx context.Context, ps []Penggunaan) error {
	if len(ps) == 0 {
		return nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, p := range ps {
		var klien interface{}
		if p.KlienID > 0 {
			klien = p.KlienID
		}
		var baris interface{}
		if p.Baris != nil {
			baris = *p.Baris
		}
		waktu := p.Waktu
		if waktu.IsZero() {
			waktu = time.Now()
		}
		if _, err := tx.ExecContext(ctx, sqlSimpanLog, waktu.UTC(), klien, nullJikaKosong(potong(p.KunciPrefix, 16)), nullJikaKosong(potong(p.RequestID, 40)), potong(p.Metode, 8),
			potong(p.Endpoint, 100), nullJikaKosong(potong(p.Dataset, 60)), p.Status, potong(p.Hasil, 20), p.DurasiMS, baris, nullJikaKosong(potong(p.IP, 64)),
			nullJikaKosong(potong(p.UserAgent, 200))); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const sqlSimpanAgregat = `UPDATE api_akses_agregat SET jumlah = jumlah + @p6, baris = baris + @p7, lat_total_ms = lat_total_ms + @p8,
	lat_maks_ms = CASE WHEN lat_maks_ms < @p9 THEN @p9 ELSE lat_maks_ms END,
	b50 = b50 + @p10, b100 = b100 + @p11, b250 = b250 + @p12, b500 = b500 + @p13, b1000 = b1000 + @p14, b2500 = b2500 + @p15, blebih = blebih + @p16
	WHERE jam = CAST(@p1 AS DATETIME2(0)) AND klien_id = @p2 AND endpoint = @p3 AND status_http = @p4 AND hasil = @p5;
IF @@ROWCOUNT = 0 INSERT INTO api_akses_agregat (jam, klien_id, endpoint, status_http, hasil, jumlah, baris, lat_total_ms, lat_maks_ms, b50, b100, b250, b500, b1000, b2500, blebih)
	VALUES (CAST(@p1 AS DATETIME2(0)), @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9, @p10, @p11, @p12, @p13, @p14, @p15, @p16);`

// SimpanAgregat menambahkan hitungan ke baris (jam, klien, endpoint, status, hasil), membuat barisnya bila belum ada.
func (s *Store) SimpanAgregat(ctx context.Context, as []Agregat) error {
	if len(as) == 0 {
		return nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, a := range as {
		if _, err := tx.ExecContext(ctx, sqlSimpanAgregat, a.Jam.UTC(), a.KlienID, potong(a.Endpoint, 100), a.Status, potong(a.Hasil, 20), a.Jumlah, a.Baris, a.LatTotal, a.LatMaks,
			a.Ember[0], a.Ember[1], a.Ember[2], a.Ember[3], a.Ember[4], a.Ember[5], a.Ember[6]); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// HapusLogSebelum menghapus riwayat permintaan yang lebih lama dari t, berkelompok supaya tidak mengunci tabel lama. Mengembalikan jumlah baris terhapus.
func (s *Store) HapusLogSebelum(ctx context.Context, t time.Time) (int64, error) {
	return s.hapusKelompok(ctx, `DELETE TOP (5000) FROM api_akses_log WHERE waktu < CAST(@p1 AS DATETIME2(3))`, t)
}

// HapusAgregatSebelum menghapus hitungan per jam yang lebih lama dari t.
func (s *Store) HapusAgregatSebelum(ctx context.Context, t time.Time) (int64, error) {
	return s.hapusKelompok(ctx, `DELETE TOP (5000) FROM api_akses_agregat WHERE jam < CAST(@p1 AS DATETIME2(0))`, t)
}

func (s *Store) hapusKelompok(ctx context.Context, q string, t time.Time) (int64, error) {
	var total int64
	for {
		res, err := s.DB.ExecContext(ctx, q, t.UTC())
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += n
		if n < 5000 {
			return total, nil
		}
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		default:
		}
	}
}

// ---------------------------------------------------------------- pembacaan: ringkasan dan agregat

// PenyaringPantau: rentang waktu (batas jam; dari termasuk, sampai tidak) dan penyaring opsional.
type PenyaringPantau struct {
	Dari     time.Time
	Sampai   time.Time
	KlienID  int64 // 0 = semua klien
	Endpoint string
}

func (p PenyaringPantau) where() (string, []interface{}) {
	var w []string
	var a []interface{}
	tambah := func(cond string, v interface{}) {
		a = append(a, v)
		w = append(w, strings.ReplaceAll(cond, "?", fmt.Sprintf("@p%d", len(a))))
	}
	tambah("a.jam >= CAST(? AS DATETIME2(0))", p.Dari.UTC().Truncate(time.Hour))
	tambah("a.jam < CAST(? AS DATETIME2(0))", p.Sampai.UTC())
	if p.KlienID > 0 {
		tambah("a.klien_id = ?", p.KlienID)
	}
	if e := strings.TrimSpace(p.Endpoint); e != "" {
		tambah("a.endpoint = ?", potong(e, 100))
	}
	return " WHERE " + strings.Join(w, " AND "), a
}

// Metrik: angka ringkas satu kelompok permintaan.
type Metrik struct {
	Jumlah      int64   `json:"jumlah"`
	Sukses      int64   `json:"sukses"`
	GalatKlien  int64   `json:"galat_klien"`  // 4xx dari penanganan
	Ditolak     int64   `json:"ditolak"`      // 401/403: kunci tidak sah, IP, atau scope
	BatasLaju   int64   `json:"batas_laju"`   // 429
	KuotaHabis  int64   `json:"kuota_habis"`  // 429
	GalatServer int64   `json:"galat_server"` // 5xx
	Baris       int64   `json:"baris"`
	LatRataMS   float64 `json:"lat_rata_ms"`
	LatMaksMS   int     `json:"lat_maks_ms"`
	LatP50MS    int     `json:"lat_p50_ms"`
	LatP95MS    int     `json:"lat_p95_ms"`
}

const kolomMetrik = `SUM(a.jumlah),
	SUM(CASE WHEN a.hasil = N'ok' THEN a.jumlah ELSE 0 END),
	SUM(CASE WHEN a.hasil = N'galat_klien' THEN a.jumlah ELSE 0 END),
	SUM(CASE WHEN a.hasil IN (N'tidak_sah', N'ditolak_ip', N'ditolak_scope') THEN a.jumlah ELSE 0 END),
	SUM(CASE WHEN a.hasil = N'batas_laju' THEN a.jumlah ELSE 0 END),
	SUM(CASE WHEN a.hasil = N'kuota_habis' THEN a.jumlah ELSE 0 END),
	SUM(CASE WHEN a.hasil = N'galat_server' THEN a.jumlah ELSE 0 END),
	SUM(a.baris), SUM(a.lat_total_ms), MAX(a.lat_maks_ms),
	SUM(a.b50), SUM(a.b100), SUM(a.b250), SUM(a.b500), SUM(a.b1000), SUM(a.b2500), SUM(a.blebih)`

// persentil memperkirakan latensi persentil q dari histogram: batas atas ember tempat jumlah kumulatif mencapai q (ember terakhir: latensi maksimum teramati).
func persentil(ember [7]int64, total int64, q float64, maks int) int {
	if total <= 0 {
		return 0
	}
	target := int64(float64(total)*q + 0.999999)
	if target < 1 {
		target = 1
	}
	var kum int64
	for i, n := range ember {
		kum += n
		if kum >= target {
			if i < len(batasLatensi) {
				if batasLatensi[i] > maks && maks > 0 {
					return maks
				}
				return batasLatensi[i]
			}
			return maks
		}
	}
	return maks
}

func pindaiMetrik(r pemindai, tujuan ...interface{}) (Metrik, error) {
	var m Metrik
	var jumlah, sukses, gk, ditolak, laju, kuota, gs, baris, latTotal sql.NullInt64
	var maks sql.NullInt64
	var e [7]sql.NullInt64
	dest := append(tujuan, &jumlah, &sukses, &gk, &ditolak, &laju, &kuota, &gs, &baris, &latTotal, &maks, &e[0], &e[1], &e[2], &e[3], &e[4], &e[5], &e[6])
	if err := r.Scan(dest...); err != nil {
		return m, err
	}
	m.Jumlah, m.Sukses, m.GalatKlien, m.Ditolak, m.BatasLaju, m.KuotaHabis, m.GalatServer = jumlah.Int64, sukses.Int64, gk.Int64, ditolak.Int64, laju.Int64, kuota.Int64, gs.Int64
	m.Baris, m.LatMaksMS = baris.Int64, int(maks.Int64)
	if m.Jumlah > 0 {
		m.LatRataMS = float64(latTotal.Int64) / float64(m.Jumlah)
	}
	var em [7]int64
	for i := range e {
		em[i] = e[i].Int64
	}
	m.LatP50MS, m.LatP95MS = persentil(em, m.Jumlah, 0.50, m.LatMaksMS), persentil(em, m.Jumlah, 0.95, m.LatMaksMS)
	return m, nil
}

// Ringkas menjumlahkan semua permintaan pada rentang beserta jumlah klien yang aktif.
func (s *Store) Ringkas(ctx context.Context, p PenyaringPantau) (Metrik, int64, error) {
	w, a := p.where()
	var aktif sql.NullInt64
	m, err := pindaiMetrik(s.DB.QueryRowContext(ctx, "SELECT COUNT(DISTINCT CASE WHEN a.klien_id <> 0 THEN a.klien_id END), "+kolomMetrik+" FROM api_akses_agregat a"+w, a...), &aktif)
	return m, aktif.Int64, err
}

// TitikDeret: satu titik waktu pada grafik.
type TitikDeret struct {
	Waktu     time.Time `json:"waktu"`
	Jumlah    int64     `json:"jumlah"`
	Gagal     int64     `json:"gagal"`    // semua hasil selain ok
	Dibatasi  int64     `json:"dibatasi"` // 429 (laju + kuota)
	LatRataMS float64   `json:"lat_rata_ms"`
}

// Deret mengelompokkan permintaan per jam, atau per hari (hari WIB) bila perHari. Titik tanpa permintaan diisi nol supaya sumbu waktu grafik utuh.
func (s *Store) Deret(ctx context.Context, p PenyaringPantau, perHari bool) ([]TitikDeret, error) {
	w, a := p.where()
	ekspresi := "a.jam"
	if perHari {
		ekspresi = "DATEADD(HOUR, -7, CAST(CAST(DATEADD(HOUR, 7, a.jam) AS DATE) AS DATETIME2(0)))"
	}
	rows, err := s.DB.QueryContext(ctx, fmt.Sprintf(`SELECT %[1]s, SUM(a.jumlah), SUM(CASE WHEN a.hasil <> N'ok' THEN a.jumlah ELSE 0 END),
		SUM(CASE WHEN a.hasil IN (N'batas_laju', N'kuota_habis') THEN a.jumlah ELSE 0 END), SUM(a.lat_total_ms)
		FROM api_akses_agregat a%[2]s GROUP BY %[1]s ORDER BY %[1]s`, ekspresi, w), a...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ada := map[int64]TitikDeret{}
	for rows.Next() {
		var t TitikDeret
		var jml, gagal, batas, lat sql.NullInt64
		if err := rows.Scan(&t.Waktu, &jml, &gagal, &batas, &lat); err != nil {
			return nil, err
		}
		t.Waktu = t.Waktu.UTC()
		t.Jumlah, t.Gagal, t.Dibatasi = jml.Int64, gagal.Int64, batas.Int64
		if t.Jumlah > 0 {
			t.LatRataMS = float64(lat.Int64) / float64(t.Jumlah)
		}
		ada[t.Waktu.Unix()] = t
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return IsiCelah(ada, p.Dari, p.Sampai, perHari), nil
}

// IsiCelah membentuk deret utuh dari awal sampai akhir (per jam atau per hari WIB) dengan nol pada titik tanpa data. Dibatasi 800 titik (jika rentang lebih panjang,
// titik tertua dibuang) agar respons tetap kecil.
func IsiCelah(ada map[int64]TitikDeret, dari, sampai time.Time, perHari bool) []TitikDeret {
	wib := time.FixedZone("WIB", 7*3600)
	awal := dari.UTC().Truncate(time.Hour)
	langkah := time.Hour
	if perHari {
		d := dari.In(wib)
		awal = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, wib).UTC()
	}
	var out []TitikDeret
	for t := awal; t.Before(sampai.UTC()); {
		if v, ok := ada[t.Unix()]; ok {
			out = append(out, v)
		} else {
			out = append(out, TitikDeret{Waktu: t})
		}
		if perHari {
			j := t.In(wib)
			t = time.Date(j.Year(), j.Month(), j.Day()+1, 0, 0, 0, 0, wib).UTC()
		} else {
			t = t.Add(langkah)
		}
	}
	if len(out) > 800 {
		out = out[len(out)-800:]
	}
	return out
}

// Kelompok: metrik satu klien atau satu endpoint.
type Kelompok struct {
	Kunci   string `json:"kunci"` // endpoint, atau id klien sebagai teks
	KlienID int64  `json:"klien_id,omitempty"`
	Kode    string `json:"kode,omitempty"`
	Nama    string `json:"nama,omitempty"`
	Metrik
}

// PerKlien: metrik per klien pada rentang (terbanyak dulu). Klien dengan kunci tidak dikenal muncul sebagai satu baris tanpa nama (klien_id 0).
func (s *Store) PerKlien(ctx context.Context, p PenyaringPantau) ([]Kelompok, error) {
	w, a := p.where()
	rows, err := s.DB.QueryContext(ctx, `SELECT a.klien_id, ISNULL(k.kode, N''), ISNULL(k.nama_aplikasi, N''), `+kolomMetrik+`
		FROM api_akses_agregat a LEFT JOIN api_akses_klien k ON k.id = a.klien_id`+w+`
		GROUP BY a.klien_id, k.kode, k.nama_aplikasi ORDER BY SUM(a.jumlah) DESC`, a...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Kelompok{}
	for rows.Next() {
		var g Kelompok
		m, err := pindaiMetrik(rows, &g.KlienID, &g.Kode, &g.Nama)
		if err != nil {
			return nil, err
		}
		g.Metrik, g.Kunci = m, fmt.Sprint(g.KlienID)
		out = append(out, g)
	}
	return out, rows.Err()
}

// PerEndpoint: metrik per endpoint pada rentang (terbanyak dulu).
func (s *Store) PerEndpoint(ctx context.Context, p PenyaringPantau) ([]Kelompok, error) {
	w, a := p.where()
	rows, err := s.DB.QueryContext(ctx, `SELECT a.endpoint, `+kolomMetrik+` FROM api_akses_agregat a`+w+` GROUP BY a.endpoint ORDER BY SUM(a.jumlah) DESC`, a...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Kelompok{}
	for rows.Next() {
		var g Kelompok
		m, err := pindaiMetrik(rows, &g.Kunci)
		if err != nil {
			return nil, err
		}
		g.Metrik = m
		out = append(out, g)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- pembacaan: riwayat permintaan

// PenyaringRiwayat: syarat pencarian riwayat permintaan.
type PenyaringRiwayat struct {
	Dari       time.Time
	Sampai     time.Time
	KlienID    int64
	Endpoint   string
	Dataset    string
	Hasil      string
	Status     int
	IP         string
	Q          string
	Halaman    int
	PerHalaman int
}

// Batas halaman riwayat.
const (
	RiwayatPerHalamanBawaan = 50
	RiwayatPerHalamanMaks   = 200
)

// Riwayat membaca satu halaman riwayat permintaan (terbaru dulu) beserta total yang cocok.
func (s *Store) Riwayat(ctx context.Context, p PenyaringRiwayat) ([]Penggunaan, int64, error) {
	var w []string
	var args []interface{}
	tambah := func(cond string, v interface{}) {
		args = append(args, v)
		w = append(w, strings.ReplaceAll(cond, "?", fmt.Sprintf("@p%d", len(args))))
	}
	if !p.Dari.IsZero() {
		tambah("l.waktu >= CAST(? AS DATETIME2(3))", p.Dari.UTC())
	}
	if !p.Sampai.IsZero() {
		tambah("l.waktu < CAST(? AS DATETIME2(3))", p.Sampai.UTC())
	}
	if p.KlienID > 0 {
		tambah("l.klien_id = ?", p.KlienID)
	}
	if v := strings.TrimSpace(p.Endpoint); v != "" {
		tambah("l.endpoint = ?", potong(v, 100))
	}
	if v := strings.TrimSpace(p.Dataset); v != "" {
		tambah("l.dataset = ?", potong(v, 60))
	}
	if v := strings.TrimSpace(p.Hasil); v != "" {
		tambah("l.hasil = ?", potong(v, 20))
	}
	if p.Status > 0 {
		tambah("l.status_http = ?", p.Status)
	}
	if v := strings.TrimSpace(p.IP); v != "" {
		tambah(`l.ip LIKE ? ESCAPE '\'`, "%"+escapeLike(potong(v, 64))+"%")
	}
	if v := strings.TrimSpace(p.Q); v != "" {
		args = append(args, "%"+escapeLike(potong(v, 100))+"%")
		ph := fmt.Sprintf("@p%d", len(args))
		w = append(w, fmt.Sprintf(`(l.endpoint LIKE %[1]s ESCAPE '\' OR l.dataset LIKE %[1]s ESCAPE '\' OR l.request_id LIKE %[1]s ESCAPE '\' OR l.kunci_prefix LIKE %[1]s ESCAPE '\' OR l.ip LIKE %[1]s ESCAPE '\')`, ph))
	}
	where := ""
	if len(w) > 0 {
		where = " WHERE " + strings.Join(w, " AND ")
	}
	var total int64
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT_BIG(*) FROM api_akses_log l"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := p.PerHalaman
	if limit < 1 {
		limit = RiwayatPerHalamanBawaan
	}
	if limit > RiwayatPerHalamanMaks {
		limit = RiwayatPerHalamanMaks
	}
	h := p.Halaman
	if h < 1 {
		h = 1
	}
	args = append(args, (h-1)*limit, limit)
	rows, err := s.DB.QueryContext(ctx, fmt.Sprintf(`SELECT l.id, l.waktu, ISNULL(l.klien_id, 0), ISNULL(k.kode, N''), ISNULL(k.nama_aplikasi, N''), ISNULL(l.kunci_prefix, N''),
		ISNULL(l.request_id, N''), l.metode, l.endpoint, ISNULL(l.dataset, N''), l.status_http, l.hasil, l.durasi_ms, l.baris, ISNULL(l.ip, N''), ISNULL(l.user_agent, N'')
		FROM api_akses_log l LEFT JOIN api_akses_klien k ON k.id = l.klien_id%s ORDER BY l.waktu DESC, l.id DESC OFFSET @p%d ROWS FETCH NEXT @p%d ROWS ONLY`,
		where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Penggunaan{}
	for rows.Next() {
		var u Penggunaan
		var baris sql.NullInt64
		if err := rows.Scan(&u.ID, &u.Waktu, &u.KlienID, &u.KlienKode, &u.KlienNama, &u.KunciPrefix, &u.RequestID, &u.Metode, &u.Endpoint, &u.Dataset, &u.Status, &u.Hasil,
			&u.DurasiMS, &baris, &u.IP, &u.UserAgent); err != nil {
			return nil, 0, err
		}
		u.Waktu = u.Waktu.UTC()
		u.Baris = intPtr(baris)
		out = append(out, u)
	}
	return out, total, rows.Err()
}
