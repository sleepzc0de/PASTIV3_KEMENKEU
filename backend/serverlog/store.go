package serverlog

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Store membaca dan menulis tabel server_log (migrasi 059). Satu baris = satu kelompok kejadian yang sama (sidik jari sama) dalam jendela gabung.
type Store struct{ DB *sql.DB }

// jendelaGabung: kejadian yang sama dalam rentang ini sejak kejadian pertama sebuah baris digabung ke baris itu (hitungannya bertambah). Lewat dari ini dibuat baris
// baru, supaya galat yang terus berulang berhari-hari tetap terlihat per jam, bukan satu baris raksasa.
const jendelaGabung = time.Hour

// maksJumlahBaris: hitungan pada satu baris tidak melewati ini, supaya kolom INT tidak pernah meluap (yang akan menggagalkan penulisan berulang-ulang).
const maksJumlahBaris = 2_000_000_000

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`, `[`, `\[`).Replace(s)
}

func potong(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
}

const sqlGabung = `SET NOCOUNT ON;
DECLARE @id BIGINT = (SELECT TOP 1 id FROM server_log WHERE sidik = @p1 AND waktu >= CAST(@p2 AS DATETIME2(3)) ORDER BY id DESC);
IF @id IS NOT NULL
    UPDATE server_log
       SET jumlah = CASE WHEN CAST(jumlah AS BIGINT) + @p3 > 2000000000 THEN 2000000000 ELSE jumlah + @p3 END,
           waktu_akhir = CASE WHEN waktu_akhir < CAST(@p4 AS DATETIME2(3)) THEN CAST(@p4 AS DATETIME2(3)) ELSE waktu_akhir END,
           request_id = COALESCE(NULLIF(@p5, N''), request_id),
           user_id = COALESCE(NULLIF(@p6, N''), user_id)
     WHERE id = @id;
ELSE
    INSERT INTO server_log (waktu, waktu_akhir, level, sumber, pesan, detail, jumlah, sidik, rute, metode, status_http, durasi_ms, request_id, user_id)
    VALUES (CAST(@p7 AS DATETIME2(3)), CAST(@p4 AS DATETIME2(3)), @p8, @p9, @p10, NULLIF(@p11, N''), @p3, @p1, NULLIF(@p12, N''), NULLIF(@p13, N''),
            NULLIF(@p14, 0), NULLIF(@p15, 0), NULLIF(@p5, N''), NULLIF(@p6, N''));`

// Simpan menulis beberapa kelompok dalam satu transaksi; kelompok yang sidik jarinya sudah ada pada jendela gabung menambah hitungan baris itu.
func (s *Store) Simpan(ctx context.Context, gs []Gabungan) error {
	if len(gs) == 0 {
		return nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, g := range gs {
		akhir := g.Akhir
		if akhir.IsZero() {
			akhir = g.Waktu
		}
		if akhir.IsZero() {
			akhir = time.Now()
		}
		awal := g.Waktu
		if awal.IsZero() || awal.After(akhir) {
			awal = akhir
		}
		if g.Jumlah > maksJumlahBaris {
			g.Jumlah = maksJumlahBaris
		}
		if _, err := tx.ExecContext(ctx, sqlGabung,
			g.Sidik, awal.Add(-jendelaGabung).UTC(), g.Jumlah, akhir.UTC(), potong(g.RequestID, 64), potong(g.UserID, 64),
			awal.UTC(), string(g.Level), potong(g.Sumber, maksSumber), potong(g.Pesan, maksPesan), potong(g.Detail, maksDetail),
			potong(g.Rute, maksRute), potong(g.Metode, 10), g.Status, g.DurasiMS,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Penyaring: syarat pembacaan daftar.
type Penyaring struct {
	Level        string // "" (semua), "error", atau "warn"
	Sumber       string
	Q            string // dicari pada pesan, rute, dan request id
	Dari, Sampai time.Time
	Halaman      int
	PerHalaman   int
}

func bangunKondisi(p Penyaring) (string, []interface{}) {
	var w []string
	var a []interface{}
	tambah := func(cond string, v interface{}) {
		a = append(a, v)
		w = append(w, strings.ReplaceAll(cond, "?", "@p"+strconv.Itoa(len(a))))
	}
	if p.Level == string(Error) || p.Level == string(Warn) {
		tambah("level = ?", p.Level)
	}
	if v := strings.TrimSpace(p.Sumber); v != "" {
		tambah("sumber = ?", v)
	}
	if !p.Dari.IsZero() {
		tambah("waktu_akhir >= CAST(? AS DATETIME2(3))", p.Dari.UTC())
	}
	if !p.Sampai.IsZero() {
		tambah("waktu < CAST(? AS DATETIME2(3))", p.Sampai.UTC())
	}
	if v := strings.TrimSpace(p.Q); v != "" {
		tambah(`(pesan LIKE ? ESCAPE '\' OR rute LIKE ? ESCAPE '\' OR request_id LIKE ? ESCAPE '\' OR sumber LIKE ? ESCAPE '\')`, "%"+escapeLike(v)+"%")
	}
	if len(w) == 0 {
		return "", a
	}
	return " WHERE " + strings.Join(w, " AND "), a
}

// kolomBaca tanpa detail (jejak panjang) supaya daftar tetap ringan; detail dibaca lewat Detail.
const kolomBaca = `id, waktu, waktu_akhir, level, sumber, pesan, jumlah, ISNULL(rute, N''), ISNULL(metode, N''), ISNULL(status_http, 0), ISNULL(durasi_ms, 0),
	ISNULL(request_id, N''), ISNULL(user_id, N''), sidik`

type pemindai interface {
	Scan(dest ...interface{}) error
}

func pindai(r pemindai) (Baris, error) {
	var b Baris
	var level string
	if err := r.Scan(&b.ID, &b.Waktu, &b.WaktuAkhir, &level, &b.Sumber, &b.Pesan, &b.Jumlah, &b.Rute, &b.Metode, &b.Status, &b.DurasiMS, &b.RequestID, &b.UserID, &b.Sidik); err != nil {
		return b, err
	}
	b.Level = Level(level)
	b.Waktu, b.WaktuAkhir = b.Waktu.UTC(), b.WaktuAkhir.UTC()
	b.Sidik = strings.TrimSpace(b.Sidik)
	return b, nil
}

// Hasil: satu halaman baris beserta jumlah seluruh baris yang cocok.
type Hasil struct {
	Baris      []Baris `json:"baris"`
	Total      int64   `json:"total"`
	Halaman    int     `json:"halaman"`
	PerHalaman int     `json:"per_halaman"`
}

// Cari membaca satu halaman baris (terbaru dulu menurut waktu kejadian terakhir) beserta jumlah seluruh baris yang cocok.
func (s *Store) Cari(ctx context.Context, p Penyaring) (*Hasil, error) {
	if p.PerHalaman < 1 || p.PerHalaman > 200 {
		p.PerHalaman = 50
	}
	if p.Halaman < 1 {
		p.Halaman = 1
	}
	where, args := bangunKondisi(p)
	out := &Hasil{Baris: []Baris{}, Halaman: p.Halaman, PerHalaman: p.PerHalaman}
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT_BIG(*) FROM server_log"+where, args...).Scan(&out.Total); err != nil {
		return nil, err
	}
	offset := (p.Halaman - 1) * p.PerHalaman
	if out.Total == 0 || int64(offset) >= out.Total {
		return out, nil
	}
	args = append(args, offset, p.PerHalaman)
	q := fmt.Sprintf("SELECT %s FROM server_log%s ORDER BY waktu_akhir DESC, id DESC OFFSET @p%d ROWS FETCH NEXT @p%d ROWS ONLY", kolomBaca, where, len(args)-1, len(args))
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		b, err := pindai(rows)
		if err != nil {
			return nil, err
		}
		out.Baris = append(out.Baris, b)
	}
	return out, rows.Err()
}

// Detail membaca satu baris lengkap (termasuk jejak). sql.ErrNoRows bila tidak ada.
func (s *Store) Detail(ctx context.Context, id int64) (Baris, error) {
	b, err := pindai(s.DB.QueryRowContext(ctx, "SELECT "+kolomBaca+" FROM server_log WHERE id = @p1", id))
	if err != nil {
		return b, err
	}
	var detail sql.NullString
	if err := s.DB.QueryRowContext(ctx, "SELECT detail FROM server_log WHERE id = @p1", id).Scan(&detail); err == nil && detail.Valid {
		b.Detail = detail.String
	}
	return b, nil
}

// Opsi: daftar sumber yang pernah tercatat (untuk pilihan penyaring).
func (s *Store) Opsi(ctx context.Context) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT TOP 100 sumber FROM server_log GROUP BY sumber ORDER BY MAX(waktu_akhir) DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// TitikJam: jumlah kejadian pada satu jam (menurut waktu kejadian terakhir pada barisnya).
type TitikJam struct {
	Jam   time.Time `json:"jam"`
	Error int64     `json:"error"`
	Warn  int64     `json:"warn"`
}

// JumlahSumber: kejadian per sumber dan level.
type JumlahSumber struct {
	Sumber string `json:"sumber"`
	Level  Level  `json:"level"`
	Jumlah int64  `json:"jumlah"`
}

// Ringkasan: angka ikhtisar untuk halaman Log Server.
type Ringkasan struct {
	Error24Jam     int64          `json:"error_24_jam"`
	Warn24Jam      int64          `json:"warn_24_jam"`
	Error7Hari     int64          `json:"error_7_hari"`
	Warn7Hari      int64          `json:"warn_7_hari"`
	Kelompok24Jam  int            `json:"kelompok_24_jam"` // jumlah jenis kejadian berbeda dalam 24 jam
	TotalBaris     int64          `json:"total_baris"`
	Terakhir       *time.Time     `json:"terakhir,omitempty"`
	PerJam         []TitikJam     `json:"per_jam"` // 24 jam terakhir, jam paling lama dulu
	Sumber         []JumlahSumber `json:"sumber"`  // paling sering dalam 24 jam
	PalingBerulang []Baris        `json:"paling_berulang"`
}

// Ringkasan menghitung ikhtisar relatif terhadap sekarang.
func (s *Store) Ringkasan(ctx context.Context, sekarang time.Time) (Ringkasan, error) {
	r := Ringkasan{PerJam: []TitikJam{}, Sumber: []JumlahSumber{}, PalingBerulang: []Baris{}}
	sekarang = sekarang.UTC()
	d24, d7 := sekarang.Add(-24*time.Hour), sekarang.Add(-7*24*time.Hour)

	if err := s.DB.QueryRowContext(ctx, `SELECT
			ISNULL(SUM(CASE WHEN level = 'error' AND waktu_akhir >= CAST(@p1 AS DATETIME2(3)) THEN CAST(jumlah AS BIGINT) END), 0),
			ISNULL(SUM(CASE WHEN level = 'warn'  AND waktu_akhir >= CAST(@p1 AS DATETIME2(3)) THEN CAST(jumlah AS BIGINT) END), 0),
			ISNULL(SUM(CASE WHEN level = 'error' THEN CAST(jumlah AS BIGINT) END), 0),
			ISNULL(SUM(CASE WHEN level = 'warn'  THEN CAST(jumlah AS BIGINT) END), 0),
			ISNULL(SUM(CASE WHEN waktu_akhir >= CAST(@p1 AS DATETIME2(3)) THEN 1 ELSE 0 END), 0)
		FROM server_log WHERE waktu_akhir >= CAST(@p2 AS DATETIME2(3))`, d24, d7).
		Scan(&r.Error24Jam, &r.Warn24Jam, &r.Error7Hari, &r.Warn7Hari, &r.Kelompok24Jam); err != nil {
		return r, err
	}
	// Ukuran tabel dan kejadian terakhir (seluruh waktu, bukan hanya 7 hari).
	var terakhir sql.NullTime
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT_BIG(*), MAX(waktu_akhir) FROM server_log").Scan(&r.TotalBaris, &terakhir); err != nil {
		return r, err
	}
	if terakhir.Valid {
		t := terakhir.Time.UTC()
		r.Terakhir = &t
	}

	// Per jam: jam tanpa kejadian tetap ada (nol) supaya grafik utuh.
	awal := d24.Truncate(time.Hour).Add(time.Hour)
	for i := 0; i < 24; i++ {
		r.PerJam = append(r.PerJam, TitikJam{Jam: awal.Add(time.Duration(i) * time.Hour)})
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT DATEADD(HOUR, DATEDIFF(HOUR, 0, waktu_akhir), 0), level, SUM(CAST(jumlah AS BIGINT))
		FROM server_log WHERE waktu_akhir >= CAST(@p1 AS DATETIME2(3))
		GROUP BY DATEADD(HOUR, DATEDIFF(HOUR, 0, waktu_akhir), 0), level`, awal)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var jam time.Time
		var level string
		var n int64
		if err := rows.Scan(&jam, &level, &n); err != nil {
			rows.Close()
			return r, err
		}
		idx := int(jam.UTC().Sub(awal) / time.Hour)
		if idx < 0 || idx >= len(r.PerJam) {
			continue
		}
		if level == string(Error) {
			r.PerJam[idx].Error += n
		} else {
			r.PerJam[idx].Warn += n
		}
	}
	rows.Close()

	rows, err = s.DB.QueryContext(ctx, `SELECT TOP 8 sumber, level, SUM(CAST(jumlah AS BIGINT)) AS n FROM server_log
		WHERE waktu_akhir >= CAST(@p1 AS DATETIME2(3)) GROUP BY sumber, level ORDER BY n DESC`, d24)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var j JumlahSumber
		var level string
		if err := rows.Scan(&j.Sumber, &level, &j.Jumlah); err != nil {
			rows.Close()
			return r, err
		}
		j.Level = Level(level)
		r.Sumber = append(r.Sumber, j)
	}
	rows.Close()

	rows, err = s.DB.QueryContext(ctx, "SELECT TOP 5 "+kolomBaca+" FROM server_log WHERE waktu_akhir >= CAST(@p1 AS DATETIME2(3)) AND jumlah > 1 ORDER BY jumlah DESC, id DESC", d24)
	if err != nil {
		return r, err
	}
	defer rows.Close()
	for rows.Next() {
		b, err := pindai(rows)
		if err != nil {
			return r, err
		}
		r.PalingBerulang = append(r.PalingBerulang, b)
	}
	return r, rows.Err()
}

// Pembersihan menghapus baris lama menurut retensi per level dan membatasi jumlah baris. Dihapus per kelompok kecil supaya tidak mengunci tabel lama-lama.
type Pembersihan struct {
	ErrorSebelum time.Time // baris error dengan kejadian terakhir sebelum ini dihapus (zero = tidak dihapus berdasarkan umur)
	WarnSebelum  time.Time
	MaksBaris    int // jumlah baris tertinggi yang dipertahankan (yang tertua dihapus lebih dulu); <= 0 = tanpa batas
}

const ukuranKelompokHapus = 2000

// Bersihkan menjalankan pembersihan dan mengembalikan jumlah baris yang dihapus.
func (s *Store) Bersihkan(ctx context.Context, p Pembersihan) (int64, error) {
	var total int64
	hapus := func(query string, args ...interface{}) error {
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			res, err := s.DB.ExecContext(ctx, query, args...)
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			total += n
			if n < ukuranKelompokHapus {
				return nil
			}
		}
	}
	if !p.ErrorSebelum.IsZero() {
		if err := hapus(fmt.Sprintf("DELETE TOP (%d) FROM server_log WHERE level = 'error' AND waktu_akhir < CAST(@p1 AS DATETIME2(3))", ukuranKelompokHapus), p.ErrorSebelum.UTC()); err != nil {
			return total, err
		}
	}
	if !p.WarnSebelum.IsZero() {
		if err := hapus(fmt.Sprintf("DELETE TOP (%d) FROM server_log WHERE level = 'warn' AND waktu_akhir < CAST(@p1 AS DATETIME2(3))", ukuranKelompokHapus), p.WarnSebelum.UTC()); err != nil {
			return total, err
		}
	}
	if p.MaksBaris > 0 {
		var ada int64
		if err := s.DB.QueryRowContext(ctx, "SELECT COUNT_BIG(*) FROM server_log").Scan(&ada); err != nil {
			return total, err
		}
		for lebih := ada - int64(p.MaksBaris); lebih > 0; lebih -= ukuranKelompokHapus {
			n := lebih
			if n > ukuranKelompokHapus {
				n = ukuranKelompokHapus
			}
			res, err := s.DB.ExecContext(ctx, "DELETE FROM server_log WHERE id IN (SELECT TOP (@p1) id FROM server_log ORDER BY id ASC)", n)
			if err != nil {
				return total, err
			}
			d, _ := res.RowsAffected()
			total += d
			if d == 0 {
				break
			}
		}
	}
	return total, nil
}

// MulaiPembersihan menjalankan Bersihkan tiap jam (dan sekali beberapa menit setelah menyala) sampai ctx dibatalkan. Hasilnya dilaporkan lewat laporkan bila ada yang dihapus
// atau gagal.
func MulaiPembersihan(ctx context.Context, st *Store, retensiError, retensiWarn time.Duration, maksBaris int, laporkan func(dihapus int64, err error)) {
	if st == nil {
		return
	}
	go func() {
		bersihkan := func() {
			c, batal := context.WithTimeout(ctx, 10*time.Minute)
			defer batal()
			p := Pembersihan{MaksBaris: maksBaris}
			now := time.Now()
			if retensiError > 0 {
				p.ErrorSebelum = now.Add(-retensiError)
			}
			if retensiWarn > 0 {
				p.WarnSebelum = now.Add(-retensiWarn)
			}
			n, err := st.Bersihkan(c, p)
			if laporkan != nil && (n > 0 || err != nil) {
				laporkan(n, err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Minute): // beri waktu aplikasi selesai menyala
		}
		Aman("SERVERLOG", bersihkan)
		tik := time.NewTicker(time.Hour)
		defer tik.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tik.C:
				Aman("SERVERLOG", bersihkan)
			}
		}
	}()
}
