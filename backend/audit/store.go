package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Store membaca dan menulis tabel audit_log (migrasi 058).
type Store struct{ DB *sql.DB }

// escapeLike menonaktifkan karakter khusus LIKE pada masukan pengguna (dipakai bersama ESCAPE '\').
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`, `[`, `\[`).Replace(s)
}

// potong membatasi teks pada n karakter supaya tidak melebihi lebar kolom (SQL Server menolak teks yang terlalu panjang).
func potong(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
}

func nullJikaKosong(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

const sqlSimpan = `INSERT INTO audit_log (waktu, request_id, user_id, username, peran, kode_peran, kategori, aksi, label, metode, rute, objek_tipe, objek_id,
		status_http, sukses, durasi_ms, ip, user_agent, detail)
	VALUES (CAST(@p1 AS DATETIME2(3)), @p2, TRY_CAST(@p3 AS UNIQUEIDENTIFIER), @p4, @p5, @p6, @p7, @p8, @p9, @p10, @p11, @p12, @p13, @p14, @p15, @p16, @p17, @p18, @p19)`

// Simpan menulis beberapa entri dalam satu transaksi.
func (s *Store) Simpan(ctx context.Context, es []Entri) error {
	if len(es) == 0 {
		return nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, sqlSimpan)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, e := range es {
		var detail interface{}
		if len(e.Detail) > 0 {
			b, err := json.Marshal(e.Detail)
			if err == nil {
				detail = string(b)
			}
		}
		waktu := e.Waktu
		if waktu.IsZero() {
			waktu = time.Now()
		}
		if _, err := stmt.ExecContext(ctx,
			waktu.UTC(), nullJikaKosong(potong(e.RequestID, 40)), nullJikaKosong(e.UserID), nullJikaKosong(potong(e.Username, 100)),
			nullJikaKosong(potong(e.Peran, 30)), nullJikaKosong(potong(e.KodePeran, 20)), potong(e.Kategori, 30), potong(e.Aksi, 100), potong(e.Label, 300),
			potong(e.Metode, 8), potong(e.Rute, 250), nullJikaKosong(potong(e.ObjekTipe, 40)), nullJikaKosong(potong(e.ObjekID, 120)),
			e.Status, e.Sukses, e.DurasiMS, nullJikaKosong(potong(e.IP, 64)), nullJikaKosong(potong(e.UserAgent, 300)), detail,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// bangunKondisi menyusun klausa WHERE (kolom diawali alias "a.") dan argumennya dari penyaring.
func bangunKondisi(p Penyaring) (string, []interface{}) {
	var w []string
	var a []interface{}
	tambah := func(cond string, v interface{}) {
		a = append(a, v)
		w = append(w, strings.ReplaceAll(cond, "?", "@p"+strconv.Itoa(len(a))))
	}
	if !p.Dari.IsZero() {
		tambah("a.waktu >= CAST(? AS DATETIME2(3))", p.Dari.UTC())
	}
	if !p.Sampai.IsZero() {
		tambah("a.waktu < CAST(? AS DATETIME2(3))", p.Sampai.UTC())
	}
	if v := strings.TrimSpace(p.UserID); v != "" {
		tambah("a.user_id = TRY_CAST(? AS UNIQUEIDENTIFIER)", v)
	}
	if v := strings.TrimSpace(p.Username); v != "" {
		tambah(`a.username LIKE ? ESCAPE '\'`, "%"+escapeLike(v)+"%")
	}
	if v := strings.TrimSpace(p.Kategori); v != "" {
		tambah("a.kategori = ?", v)
	}
	if v := strings.TrimSpace(p.Aksi); v != "" {
		tambah("a.aksi = ?", v)
	}
	if p.Sukses != nil {
		tambah("a.sukses = ?", *p.Sukses)
	}
	if v := strings.TrimSpace(p.IP); v != "" {
		tambah(`a.ip LIKE ? ESCAPE '\'`, "%"+escapeLike(v)+"%")
	}
	if v := strings.TrimSpace(p.Q); v != "" {
		tambah(`(a.label LIKE ? ESCAPE '\' OR a.aksi LIKE ? ESCAPE '\' OR a.rute LIKE ? ESCAPE '\' OR a.objek_id LIKE ? ESCAPE '\' OR a.username LIKE ? ESCAPE '\'
			OR a.ip LIKE ? ESCAPE '\' OR a.detail LIKE ? ESCAPE '\')`, "%"+escapeLike(v)+"%")
	}
	if len(w) == 0 {
		return "", a
	}
	return " WHERE " + strings.Join(w, " AND "), a
}

const kolomBaca = `a.id, a.waktu, ISNULL(a.request_id, N''), ISNULL(LOWER(CONVERT(NVARCHAR(36), a.user_id)), N''), ISNULL(a.username, N''), ISNULL(u.full_name, N''),
	ISNULL(a.peran, N''), ISNULL(a.kode_peran, N''), a.kategori, a.aksi, a.label, a.metode, a.rute, ISNULL(a.objek_tipe, N''), ISNULL(a.objek_id, N''),
	a.status_http, a.sukses, a.durasi_ms, ISNULL(a.ip, N''), ISNULL(a.user_agent, N''), a.detail`

type pemindai interface {
	Scan(dest ...interface{}) error
}

func pindaiEntri(r pemindai) (Entri, error) {
	var e Entri
	var detail sql.NullString
	if err := r.Scan(&e.ID, &e.Waktu, &e.RequestID, &e.UserID, &e.Username, &e.NamaLengkap, &e.Peran, &e.KodePeran, &e.Kategori, &e.Aksi, &e.Label, &e.Metode, &e.Rute,
		&e.ObjekTipe, &e.ObjekID, &e.Status, &e.Sukses, &e.DurasiMS, &e.IP, &e.UserAgent, &detail); err != nil {
		return e, err
	}
	e.Waktu = e.Waktu.UTC()
	if detail.Valid && detail.String != "" {
		var m map[string]interface{}
		if json.Unmarshal([]byte(detail.String), &m) == nil {
			e.Detail = m
		} else {
			e.Detail = map[string]interface{}{"teks": detail.String}
		}
	}
	return e, nil
}

// Cari mengembalikan satu halaman entri (terbaru lebih dulu) beserta jumlah seluruh entri yang cocok.
func (s *Store) Cari(ctx context.Context, p Penyaring) (*Daftar, error) {
	where, args := bangunKondisi(p)
	offset, limit := p.halamanAman()

	var total int64
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT_BIG(*) FROM audit_log a"+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	out := &Daftar{Entri: []Entri{}, Total: total, PerHalaman: limit, Halaman: offset/limit + 1}
	if total == 0 || int64(offset) >= total {
		return out, nil
	}
	n := len(args)
	q := "SELECT " + kolomBaca + " FROM audit_log a LEFT JOIN users u ON u.id = a.user_id" + where +
		fmt.Sprintf(" ORDER BY a.waktu DESC, a.id DESC OFFSET @p%d ROWS FETCH NEXT @p%d ROWS ONLY", n+1, n+2)
	rows, err := s.DB.QueryContext(ctx, q, append(args, offset, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		e, err := pindaiEntri(rows)
		if err != nil {
			return nil, err
		}
		out.Entri = append(out.Entri, e)
	}
	return out, rows.Err()
}

// Alir memanggil fn untuk setiap entri yang cocok (terbaru lebih dulu), paling banyak maks entri; dipakai ekspor tanpa memuat semuanya ke memori.
func (s *Store) Alir(ctx context.Context, p Penyaring, maks int, fn func(Entri) error) error {
	if maks <= 0 || maks > MaksEkspor {
		maks = MaksEkspor
	}
	where, args := bangunKondisi(p)
	q := fmt.Sprintf("SELECT TOP (%d) %s FROM audit_log a LEFT JOIN users u ON u.id = a.user_id%s ORDER BY a.waktu DESC, a.id DESC", maks, kolomBaca, where)
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		e, err := pindaiEntri(rows)
		if err != nil {
			return err
		}
		if err := fn(e); err != nil {
			return err
		}
	}
	return rows.Err()
}

// Ringkas menghitung angka ringkas aktivitas pada [dari, sampai). Tanpa dari: 24 jam terakhir; tanpa sampai: sekarang.
func (s *Store) Ringkas(ctx context.Context, dari, sampai time.Time) (*Ringkasan, error) {
	if sampai.IsZero() {
		sampai = time.Now()
	}
	if dari.IsZero() {
		dari = sampai.Add(-24 * time.Hour)
	}
	dari, sampai = dari.UTC(), sampai.UTC()
	r := &Ringkasan{Dari: dari, Sampai: sampai, PerKategori: []JumlahPer{}, AksiTeratas: []JumlahPer{}, LoginGagalIP: []JumlahPer{}, Deret: []TitikWaktu{}}
	r.PerJam = sampai.Sub(dari) <= 72*time.Hour
	rentang := "a.waktu >= CAST(@p1 AS DATETIME2(3)) AND a.waktu < CAST(@p2 AS DATETIME2(3))"

	err := s.DB.QueryRowContext(ctx, `SELECT COUNT_BIG(*), ISNULL(SUM(CASE WHEN a.sukses = 1 THEN 1 ELSE 0 END), 0), COUNT(DISTINCT a.user_id),
			ISNULL(SUM(CASE WHEN a.aksi = @p3 THEN 1 ELSE 0 END), 0), ISNULL(SUM(CASE WHEN a.aksi = @p4 THEN 1 ELSE 0 END), 0),
			ISNULL(SUM(CASE WHEN a.aksi = @p5 THEN 1 ELSE 0 END), 0), ISNULL(SUM(CASE WHEN a.kategori = @p6 THEN 1 ELSE 0 END), 0)
		FROM audit_log a WHERE `+rentang, dari, sampai, AksiLoginBerhasil, AksiLoginGagal, AksiDitolak, KatEkspor,
	).Scan(&r.Total, &r.Berhasil, &r.PenggunaAktif, &r.LoginBerhasil, &r.LoginGagal, &r.Ditolak, &r.Ekspor)
	if err != nil {
		return nil, err
	}
	r.Gagal = r.Total - r.Berhasil

	baca := func(q string, args []interface{}, fn func(*sql.Rows) error) error {
		rows, err := s.DB.QueryContext(ctx, q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			if err := fn(rows); err != nil {
				return err
			}
		}
		return rows.Err()
	}
	rg := []interface{}{dari, sampai}

	if err := baca("SELECT a.kategori, COUNT_BIG(*) FROM audit_log a WHERE "+rentang+" GROUP BY a.kategori ORDER BY 2 DESC", rg, func(rs *sql.Rows) error {
		var j JumlahPer
		if err := rs.Scan(&j.Kode, &j.Jumlah); err != nil {
			return err
		}
		j.Label = labelKategori[j.Kode]
		r.PerKategori = append(r.PerKategori, j)
		return nil
	}); err != nil {
		return nil, err
	}

	if err := baca("SELECT TOP (8) a.aksi, MAX(a.label), COUNT_BIG(*) FROM audit_log a WHERE "+rentang+" GROUP BY a.aksi ORDER BY 3 DESC, a.aksi", rg, func(rs *sql.Rows) error {
		var j JumlahPer
		if err := rs.Scan(&j.Kode, &j.Label, &j.Jumlah); err != nil {
			return err
		}
		r.AksiTeratas = append(r.AksiTeratas, j)
		return nil
	}); err != nil {
		return nil, err
	}

	if err := baca("SELECT TOP (5) ISNULL(a.ip, N'(tidak diketahui)'), COUNT_BIG(*) FROM audit_log a WHERE "+rentang+" AND a.aksi = @p3 GROUP BY a.ip ORDER BY 2 DESC",
		[]interface{}{dari, sampai, AksiLoginGagal}, func(rs *sql.Rows) error {
			var j JumlahPer
			if err := rs.Scan(&j.Kode, &j.Jumlah); err != nil {
				return err
			}
			r.LoginGagalIP = append(r.LoginGagalIP, j)
			return nil
		}); err != nil {
		return nil, err
	}

	bagian := "day"
	if r.PerJam {
		bagian = "hour"
	}
	// DATEADD(bagian, DATEDIFF(bagian, 0, waktu), 0): awal jam atau hari tempat entri berada.
	if err := baca(fmt.Sprintf(`SELECT DATEADD(%[1]s, DATEDIFF(%[1]s, 0, a.waktu), 0) AS t, COUNT_BIG(*), ISNULL(SUM(CASE WHEN a.sukses = 0 THEN 1 ELSE 0 END), 0)
		FROM audit_log a WHERE %[2]s GROUP BY DATEADD(%[1]s, DATEDIFF(%[1]s, 0, a.waktu), 0) ORDER BY t`, bagian, rentang), rg, func(rs *sql.Rows) error {
		var t TitikWaktu
		if err := rs.Scan(&t.Waktu, &t.Jumlah, &t.Gagal); err != nil {
			return err
		}
		t.Waktu = t.Waktu.UTC()
		r.Deret = append(r.Deret, t)
		return nil
	}); err != nil {
		return nil, err
	}
	return r, nil
}

// PerPengguna meringkas aktivitas tiap pengguna pada rentang waktu (yang paling baru aktif lebih dulu). q menyaring menurut username (mengandung).
func (s *Store) PerPengguna(ctx context.Context, dari, sampai time.Time, q string, halaman, perHalaman int) (*DaftarPengguna, error) {
	if perHalaman <= 0 {
		perHalaman = PerHalamanBawaan
	}
	if perHalaman > PerHalamanMaks {
		perHalaman = PerHalamanMaks
	}
	if halaman < 1 {
		halaman = 1
	}
	where := "a.user_id IS NOT NULL"
	var args []interface{}
	if !dari.IsZero() {
		args = append(args, dari.UTC())
		where += " AND a.waktu >= CAST(@p" + strconv.Itoa(len(args)) + " AS DATETIME2(3))"
	}
	if !sampai.IsZero() {
		args = append(args, sampai.UTC())
		where += " AND a.waktu < CAST(@p" + strconv.Itoa(len(args)) + " AS DATETIME2(3))"
	}
	if v := strings.TrimSpace(q); v != "" {
		args = append(args, "%"+escapeLike(v)+"%")
		where += ` AND a.username LIKE @p` + strconv.Itoa(len(args)) + ` ESCAPE '\'`
	}
	out := &DaftarPengguna{Pengguna: []RingkasanPengguna{}, Halaman: halaman, PerHalaman: perHalaman}
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(DISTINCT a.user_id) FROM audit_log a WHERE "+where, args...).Scan(&out.Total); err != nil {
		return nil, err
	}
	if out.Total == 0 {
		return out, nil
	}
	n := len(args)
	query := fmt.Sprintf(`SELECT LOWER(CONVERT(NVARCHAR(36), g.user_id)), g.username, ISNULL(u.full_name, N''), ISNULL(u.email, N''), u.is_active, g.jumlah, g.gagal, g.terakhir, g.login_terakhir,
			ISNULL(t.ip, N''), ISNULL(t.peran, N'')
		FROM (SELECT a.user_id, MAX(a.username) AS username, COUNT_BIG(*) AS jumlah, SUM(CASE WHEN a.sukses = 0 THEN 1 ELSE 0 END) AS gagal, MAX(a.waktu) AS terakhir,
				MAX(CASE WHEN a.aksi = @p%d THEN a.waktu END) AS login_terakhir
			FROM audit_log a WHERE %s GROUP BY a.user_id) g
		LEFT JOIN users u ON u.id = g.user_id
		OUTER APPLY (SELECT TOP (1) x.ip, x.peran FROM audit_log x WHERE x.user_id = g.user_id ORDER BY x.waktu DESC, x.id DESC) t
		ORDER BY g.terakhir DESC, g.user_id OFFSET @p%d ROWS FETCH NEXT @p%d ROWS ONLY`, n+1, where, n+2, n+3)
	rows, err := s.DB.QueryContext(ctx, query, append(args, AksiLoginBerhasil, (halaman-1)*perHalaman, perHalaman)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p RingkasanPengguna
		var aktif sql.NullBool
		var login sql.NullTime
		if err := rows.Scan(&p.UserID, &p.Username, &p.NamaLengkap, &p.Email, &aktif, &p.Jumlah, &p.Gagal, &p.AktifTerakhir, &login, &p.IPTerakhir, &p.PeranTerakhir); err != nil {
			return nil, err
		}
		p.AktifTerakhir = p.AktifTerakhir.UTC()
		if aktif.Valid {
			v := aktif.Bool
			p.Aktif = &v
		}
		if login.Valid {
			t := login.Time.UTC()
			p.LoginTerakhir = &t
		}
		out.Pengguna = append(out.Pengguna, p)
	}
	return out, rows.Err()
}

// HapusSebelum menghapus entri yang lebih lama dari batas, berkelompok supaya tidak mengunci tabel lama; mengembalikan jumlah yang dihapus.
func (s *Store) HapusSebelum(ctx context.Context, batas time.Time) (int64, error) {
	var total int64
	for {
		res, err := s.DB.ExecContext(ctx, "DELETE TOP (5000) FROM audit_log WHERE waktu < CAST(@p1 AS DATETIME2(3))", batas.UTC())
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
		case <-time.After(100 * time.Millisecond):
		}
	}
}
