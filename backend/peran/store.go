package peran

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"pasti-v3-backend/database"
)

// ErrTidakDitemukan: peran yang dimaksud tidak ada atau bukan milik pengguna itu.
var ErrTidakDitemukan = errors.New("peran tidak ditemukan")

// TabelBelumAda: galat SQL Server "Invalid object name" untuk tabel user_roles, yaitu migrasi 053 belum dijalankan. Pemanggil memperlakukannya sebagai
// "fitur peran belum aktif" (perilaku lama), bukan galat server, supaya penerapan kode sebelum migrasi tidak mengunci semua pengguna.
func TabelBelumAda(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Invalid object name") && strings.Contains(err.Error(), "user_roles")
}

func baris(id int64, peran, kode string, aktif bool, oleh sql.NullString, pada time.Time) Baris {
	b := Baris{ID: id, Peran: peran, Kode: kode, Aktif: aktif, Label: Label(peran), Oleh: oleh.String}
	if !pada.IsZero() {
		b.Pada = pada.UTC().Format(time.RFC3339)
	}
	return b
}

// Daftar: semua peran yang dipegang pengguna, urut menurut id (urutan pemberian).
func Daftar(ctx context.Context, userID string) ([]Baris, error) {
	rows, err := database.DB.QueryContext(ctx, `SELECT id, role, kode, aktif, dibuat_oleh, dibuat_pada FROM user_roles WHERE user_id = @p1 ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Baris{}
	for rows.Next() {
		var id int64
		var p, kode string
		var aktif bool
		var oleh sql.NullString
		var pada time.Time
		if err := rows.Scan(&id, &p, &kode, &aktif, &oleh, &pada); err != nil {
			return nil, err
		}
		out = append(out, baris(id, p, kode, aktif, oleh, pada))
	}
	return out, rows.Err()
}

// Muat menentukan peran yang berlaku bagi pengguna saat ini (satu query). wajib = config.Cfg.PeranDataWajib.
func Muat(ctx context.Context, userID, akunRole string, wajib bool) (Efektif, error) {
	daftar, err := Daftar(ctx, userID)
	if err != nil {
		return Efektif{}, err
	}
	return Selesaikan(akunRole, daftar, wajib), nil
}

// Tambah memberikan peran (admin). Peran yang sama persis (peran + kode) tidak digandakan: baris yang ada dikembalikan, dengan dibuat=false.
func Tambah(ctx context.Context, userID, peran, kode, oleh string) (b Baris, dibuat bool, err error) {
	kode, err = ValidasiPeran(peran, kode)
	if err != nil {
		return Baris{}, false, err
	}
	var oleNull sql.NullString
	if oleh != "" {
		oleNull = sql.NullString{String: oleh, Valid: true}
	}
	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		return Baris{}, false, err
	}
	defer tx.Rollback() // tidak berpengaruh setelah Commit

	var id int64
	var pada time.Time
	var aktif bool
	var olehAda sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT id, aktif, dibuat_oleh, dibuat_pada FROM user_roles WITH (UPDLOCK, HOLDLOCK) WHERE user_id = @p1 AND role = @p2 AND kode = @p3`,
		userID, peran, kode).Scan(&id, &aktif, &olehAda, &pada)
	switch {
	case err == nil:
		return baris(id, peran, kode, aktif, olehAda, pada), false, tx.Commit()
	case !errors.Is(err, sql.ErrNoRows):
		return Baris{}, false, err
	}
	if err := tx.QueryRowContext(ctx, `INSERT INTO user_roles (user_id, role, kode, dibuat_oleh) OUTPUT INSERTED.id, INSERTED.dibuat_pada VALUES (@p1, @p2, @p3, @p4)`,
		userID, peran, kode, oleNull).Scan(&id, &pada); err != nil {
		return Baris{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Baris{}, false, err
	}
	return baris(id, peran, kode, false, oleNull, pada), true, nil
}

// Hapus mencabut satu peran milik pengguna (admin). Bila itu peran aktif, pengguna kembali ke peran bawaannya.
func Hapus(ctx context.Context, userID string, id int64) error {
	res, err := database.DB.ExecContext(ctx, `DELETE FROM user_roles WHERE id = @p1 AND user_id = @p2`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrTidakDitemukan
	}
	return nil
}

// Aktifkan memilih peran aktif milik pengguna sendiri; id nil = kembali ke peran bawaan (admin/superadmin: akunnya; pengguna biasa: peran pertamanya).
func Aktifkan(ctx context.Context, userID string, id *int64) error {
	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE user_roles SET aktif = 0 WHERE user_id = @p1 AND aktif = 1`, userID); err != nil {
		return err
	}
	if id != nil {
		res, err := tx.ExecContext(ctx, `UPDATE user_roles SET aktif = 1 WHERE id = @p1 AND user_id = @p2`, *id, userID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrTidakDitemukan // bukan peran milik pengguna ini: tidak mengubah apa pun
		}
	}
	return tx.Commit()
}

// KodeSatkerSSO: kode satker pegawai dari data SSO (employees.kode_satker) untuk pengguna ini; kosong bila tidak ada.
func KodeSatkerSSO(ctx context.Context, userID string) (string, error) {
	var kode sql.NullString
	err := database.DB.QueryRowContext(ctx, `SELECT e.kode_satker FROM users u JOIN employees e ON e.id = u.employee_id WHERE u.id = @p1`, userID).Scan(&kode)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("baca kode satker SSO: %w", err)
	}
	return strings.TrimSpace(kode.String), nil
}
