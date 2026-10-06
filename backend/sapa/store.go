package sapa

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Store adalah implementasi Repo di atas SQL Server (tabel sapa_* pada migrasi 021, dan DIGITALISASI_SATKER dari
// fitur Digitalisasi Aset). Semua masukan pengguna masuk sebagai parameter; nama tabel dan kolom tetap.
type Store struct{ DB *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{DB: db} }

var _ Repo = (*Store)(nil)

const (
	kunciUrutanPenjualan = "penjualan"
	tglFormat            = "2006-01-02"
)

// likeEscape membuat teks aman dipakai di LIKE: karakter khusus dianggap huruf biasa.
func likeEscape(s string) string {
	r := strings.NewReplacer("[", "[[]", "%", "[%]", "_", "[_]")
	return r.Replace(s)
}

func str(n sql.NullString) string {
	if n.Valid {
		return n.String
	}
	return ""
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// withTx menjalankan fn dalam satu transaksi; galat membatalkan semuanya.
func (s *Store) withTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// ---------------------------------------------------------------- pengguna

// NamaPengguna: nama lengkap pengguna aplikasi; kosong bila tidak dikenal. Peran dan cakupan datanya tidak dibaca di sini, melainkan dari peran
// data aplikasi yang sedang aktif (paket peran, dipasang middleware autentikasi).
func (s *Store) NamaPengguna(ctx context.Context, userID string) (string, error) {
	var nama sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT full_name FROM users WHERE id = @p1`, userID).Scan(&nama)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return str(nama), nil
}
// ---------------------------------------------------------------- usulan

// UUID dibaca sebagai teks (CONVERT) lalu dibakukan ke huruf kecil di scanKasus: driver mengembalikan UNIQUEIDENTIFIER
// sebagai 16 byte dengan urutan byte campuran, yang mudah salah dibaca.
const kolomPenjualan = `p.id, CONVERT(NVARCHAR(36), p.uuid), p.noreg, p.kode_satker, p.nama_satker, p.kode_ue1, ISNULL(p.dibuat_oleh, ''), p.dibuat_pada, p.diperbarui_pada`

type pemindai interface {
	Scan(dest ...interface{}) error
}

func scanKasus(r pemindai) (KasusInfo, error) {
	var k KasusInfo
	err := r.Scan(&k.ID, &k.UUID, &k.Noreg, &k.KodeSatker, &k.NamaSatker, &k.KodeUE1, &k.DibuatOleh, &k.DibuatPada, &k.DiperbaruiPada)
	k.UUID = strings.ToLower(k.UUID)
	return k, err
}

// BuatPenjualan memberi nomor registrasi dan menyimpan usulan dalam satu transaksi, supaya nomor yang gagal disimpan
// tidak terbuang dan dua usulan tidak pernah mendapat nomor yang sama.
func (s *Store) BuatPenjualan(ctx context.Context, in BuatInput) (KasusInfo, error) {
	var k KasusInfo
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		tahun := time.Now().Year()
		var urut int
		err := tx.QueryRowContext(ctx,
			`MERGE sapa_urutan WITH (HOLDLOCK) AS t
			  USING (SELECT @p1 AS kunci, @p2 AS tahun) AS s ON t.kunci = s.kunci AND t.tahun = s.tahun
			  WHEN MATCHED THEN UPDATE SET nilai = t.nilai + 1
			  WHEN NOT MATCHED THEN INSERT (kunci, tahun, nilai) VALUES (s.kunci, s.tahun, 1)
			  OUTPUT INSERTED.nilai;`, kunciUrutanPenjualan, tahun).Scan(&urut)
		if err != nil {
			return err
		}
		noreg := fmt.Sprintf("PJ-%d-%05d", tahun, urut)
		var userID interface{}
		if in.UserID != "" {
			userID = in.UserID
		}
		var id int64
		err = tx.QueryRowContext(ctx,
			`INSERT INTO sapa_penjualan (noreg, kode_satker, nama_satker, kode_ue1, dibuat_oleh_id, dibuat_oleh, uuid)
			 OUTPUT INSERTED.id VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7)`,
			noreg, in.KodeSatker, in.NamaSatker, in.KodeUE1, userID, in.Oleh, uuid.NewString()).Scan(&id)
		if err != nil {
			return err
		}
		k, err = scanKasus(tx.QueryRowContext(ctx, `SELECT `+kolomPenjualan+` FROM sapa_penjualan p WHERE p.id = @p1`, id))
		return err
	})
	return k, err
}

func (s *Store) AmbilPenjualan(ctx context.Context, id int64) (*KasusInfo, error) {
	k, err := scanKasus(s.DB.QueryRowContext(ctx, `SELECT `+kolomPenjualan+` FROM sapa_penjualan p WHERE p.id = @p1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// AmbilPenjualanUUID mencari usulan menurut UUID. Pemanggil sudah memastikan bentuknya sah (BakukanUUID), karena nilai
// yang bukan UUID membuat SQL Server menolak konversinya.
func (s *Store) AmbilPenjualanUUID(ctx context.Context, id string) (*KasusInfo, error) {
	k, err := scanKasus(s.DB.QueryRowContext(ctx, `SELECT `+kolomPenjualan+` FROM sapa_penjualan p WHERE p.uuid = @p1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (s *Store) DaftarPenjualan(ctx context.Context, sc Scope, f FilterDaftar) ([]KasusInfo, int, error) {
	if sc.TidakAda {
		return []KasusInfo{}, 0, nil
	}
	var where []string
	var args []interface{}
	arg := func(v interface{}) string {
		args = append(args, v)
		return fmt.Sprintf("@p%d", len(args))
	}
	switch {
	case sc.Semua:
	case sc.Kode6 != "":
		where = append(where, "SUBSTRING(p.kode_satker, 10, 6) = "+arg(sc.Kode6))
	case sc.Kanwil9 != "":
		where = append(where, "LEFT(p.kode_satker, 9) = "+arg(sc.Kanwil9))
	case sc.KodeUE1 != "":
		where = append(where, "p.kode_ue1 = "+arg(sc.KodeUE1))
	default:
		return []KasusInfo{}, 0, nil
	}
	if f.Q != "" {
		p := arg("%" + likeEscape(f.Q) + "%")
		where = append(where, fmt.Sprintf("(p.noreg LIKE %s OR p.nama_satker LIKE %s OR p.kode_satker LIKE %s)", p, p, p))
	}
	jumlahSelesai := `(SELECT COUNT(1) FROM sapa_penjualan_tahap t WHERE t.penjualan_id = p.id AND t.status IN ('selesai', 'dilewati'))`
	switch f.Status {
	case "selesai":
		where = append(where, fmt.Sprintf("%s >= %d", jumlahSelesai, len(TahapPenjualan)))
	case "berjalan":
		where = append(where, fmt.Sprintf("%s < %d", jumlahSelesai, len(TahapPenjualan)))
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM sapa_penjualan p`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	paged := append(append([]interface{}(nil), args...), f.Offset, f.Limit)
	rows, err := s.DB.QueryContext(ctx,
		`SELECT `+kolomPenjualan+` FROM sapa_penjualan p`+cond+
			fmt.Sprintf(` ORDER BY p.id DESC OFFSET @p%d ROWS FETCH NEXT @p%d ROWS ONLY`, len(args)+1, len(args)+2), paged...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []KasusInfo{}
	for rows.Next() {
		k, err := scanKasus(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, k)
	}
	return out, total, rows.Err()
}

func (s *Store) StatusTahapBanyak(ctx context.Context, ids []int64) (map[int64]StatusTahap, error) {
	out := map[int64]StatusTahap{}
	if len(ids) == 0 {
		return out, nil
	}
	ph := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		ph[i], args[i] = fmt.Sprintf("@p%d", i+1), id
		out[id] = StatusTahap{}
	}
	rows, err := s.DB.QueryContext(ctx,
		`SELECT penjualan_id, kunci, status FROM sapa_penjualan_tahap WHERE penjualan_id IN (`+strings.Join(ph, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var kunci, status string
		if err := rows.Scan(&id, &kunci, &status); err != nil {
			return nil, err
		}
		out[id][kunci] = status
	}
	return out, rows.Err()
}

func (s *Store) TahapPenjualan(ctx context.Context, id int64) (map[string]TahapRow, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT kunci, status, data, nomor, tanggal, catatan, diperbarui_oleh, diperbarui_pada
		   FROM sapa_penjualan_tahap WHERE penjualan_id = @p1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]TahapRow{}
	for rows.Next() {
		var t TahapRow
		var data, nomor, catatan, oleh sql.NullString
		var tgl sql.NullTime
		if err := rows.Scan(&t.Kunci, &t.Status, &data, &nomor, &tgl, &catatan, &oleh, &t.DiperbaruiPada); err != nil {
			return nil, err
		}
		if data.Valid {
			t.Data = []byte(data.String)
		}
		t.Nomor, t.Catatan, t.DiperbaruiOleh = str(nomor), str(catatan), str(oleh)
		if tgl.Valid {
			t.Tanggal = tgl.Time.Format(tglFormat)
		}
		out[t.Kunci] = t
	}
	return out, rows.Err()
}

func (s *Store) SimpanTahap(ctx context.Context, id int64, t TahapRow) error {
	var data interface{}
	if len(t.Data) > 0 {
		data = string(t.Data)
	}
	pada := t.DiperbaruiPada
	if pada.IsZero() {
		pada = time.Now().UTC()
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE sapa_penjualan_tahap WITH (UPDLOCK, SERIALIZABLE)
			    SET status = @p3, data = @p4, nomor = @p5, tanggal = @p6, catatan = @p7, diperbarui_oleh = @p8, diperbarui_pada = @p9
			  WHERE penjualan_id = @p1 AND kunci = @p2;
			 IF @@ROWCOUNT = 0
			     INSERT INTO sapa_penjualan_tahap (penjualan_id, kunci, status, data, nomor, tanggal, catatan, diperbarui_oleh, diperbarui_pada)
			     VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9);`,
			id, t.Kunci, t.Status, data, nullStr(t.Nomor), nullStr(t.Tanggal), nullStr(t.Catatan), nullStr(t.DiperbaruiOleh), pada)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE sapa_penjualan SET diperbarui_pada = @p2 WHERE id = @p1`, id, pada)
		return err
	})
}

// ---------------------------------------------------------------- dokumen

func (s *Store) SimpanDokumen(ctx context.Context, d DokumenBaru) (DokumenInfo, error) {
	var peringatan interface{}
	if len(d.Peringatan) > 0 {
		b, _ := json.Marshal(d.Peringatan)
		peringatan = string(b)
	}
	var id int64
	var pada time.Time
	err := s.DB.QueryRowContext(ctx,
		`INSERT INTO sapa_dokumen (penjualan_id, tahap, jenis, nama_file, ukuran, konten, peringatan, dibuat_oleh)
		 OUTPUT INSERTED.id, INSERTED.dibuat_pada
		 VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8)`,
		d.PenjualanID, d.Tahap, d.Jenis, d.NamaFile, len(d.Berkas), d.Berkas, peringatan, d.Oleh).Scan(&id, &pada)
	if err != nil {
		return DokumenInfo{}, err
	}
	return DokumenInfo{ID: id, PenjualanID: d.PenjualanID, Tahap: d.Tahap, Jenis: d.Jenis, JenisLabel: labelJenis(d.Jenis), NamaFile: d.NamaFile,
		Ukuran: len(d.Berkas), Peringatan: d.Peringatan, DibuatOleh: d.Oleh, DibuatPada: pada}, nil
}

const kolomDokumen = `id, penjualan_id, tahap, jenis, nama_file, ukuran, peringatan, ISNULL(dibuat_oleh, ''), dibuat_pada`

func scanDokumen(r pemindai) (DokumenInfo, error) {
	var d DokumenInfo
	var peringatan sql.NullString
	if err := r.Scan(&d.ID, &d.PenjualanID, &d.Tahap, &d.Jenis, &d.NamaFile, &d.Ukuran, &peringatan, &d.DibuatOleh, &d.DibuatPada); err != nil {
		return d, err
	}
	if peringatan.Valid && peringatan.String != "" {
		_ = json.Unmarshal([]byte(peringatan.String), &d.Peringatan)
	}
	d.JenisLabel = labelJenis(d.Jenis)
	return d, nil
}

func (s *Store) DaftarDokumen(ctx context.Context, penjualanID int64) ([]DokumenInfo, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+kolomDokumen+` FROM sapa_dokumen WHERE penjualan_id = @p1 ORDER BY id DESC`, penjualanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DokumenInfo{}
	for rows.Next() {
		d, err := scanDokumen(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) AmbilDokumen(ctx context.Context, id int64) (*DokumenInfo, []byte, error) {
	var berkas []byte
	var d DokumenInfo
	var peringatan sql.NullString
	err := s.DB.QueryRowContext(ctx,
		`SELECT `+kolomDokumen+`, konten FROM sapa_dokumen WHERE id = @p1`, id).
		Scan(&d.ID, &d.PenjualanID, &d.Tahap, &d.Jenis, &d.NamaFile, &d.Ukuran, &peringatan, &d.DibuatOleh, &d.DibuatPada, &berkas)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if peringatan.Valid && peringatan.String != "" {
		_ = json.Unmarshal([]byte(peringatan.String), &d.Peringatan)
	}
	d.JenisLabel = labelJenis(d.Jenis)
	return &d, berkas, nil
}

// ---------------------------------------------------------------- template

const kolomTemplate = `id, kunci, versi, nama_file, ukuran, aktif, catatan, ISNULL(diunggah_oleh, ''), diunggah_pada`

func scanTemplate(r pemindai, extra ...interface{}) (TemplateInfo, error) {
	var t TemplateInfo
	var catatan sql.NullString
	dest := append([]interface{}{&t.ID, &t.Kunci, &t.Versi, &t.NamaFile, &t.Ukuran, &t.Aktif, &catatan, &t.DiunggahOleh, &t.DiunggahPada}, extra...)
	err := r.Scan(dest...)
	t.Catatan = str(catatan)
	return t, err
}

func (s *Store) TemplateAktif(ctx context.Context, kunci string) ([]byte, *TemplateInfo, error) {
	var berkas []byte
	t, err := scanTemplate(s.DB.QueryRowContext(ctx,
		`SELECT TOP 1 `+kolomTemplate+`, konten FROM sapa_template WHERE kunci = @p1 AND aktif = 1`, kunci), &berkas)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return berkas, &t, nil
}

// SimpanTemplate menyimpan unggahan sebagai versi baru dan menjadikannya satu-satunya versi aktif.
func (s *Store) SimpanTemplate(ctx context.Context, in TemplateBaru) (TemplateInfo, error) {
	var out TemplateInfo
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var versi int
		// UPDLOCK + HOLDLOCK: dua unggahan bersamaan mengantre, bukan bentrok pada nomor versi.
		if err := tx.QueryRowContext(ctx,
			`SELECT ISNULL(MAX(versi), 0) + 1 FROM sapa_template WITH (UPDLOCK, HOLDLOCK) WHERE kunci = @p1`, in.Kunci).Scan(&versi); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE sapa_template SET aktif = 0 WHERE kunci = @p1 AND aktif = 1`, in.Kunci); err != nil {
			return err
		}
		var err error
		out, err = scanTemplate(tx.QueryRowContext(ctx,
			`INSERT INTO sapa_template (kunci, versi, nama_file, ukuran, konten, aktif, catatan, diunggah_oleh)
			 OUTPUT INSERTED.id, INSERTED.kunci, INSERTED.versi, INSERTED.nama_file, INSERTED.ukuran, INSERTED.aktif,
			        INSERTED.catatan, ISNULL(INSERTED.diunggah_oleh, ''), INSERTED.diunggah_pada
			 VALUES (@p1, @p2, @p3, @p4, @p5, 1, @p6, @p7)`,
			in.Kunci, versi, in.NamaFile, len(in.Berkas), in.Berkas, nullStr(in.Catatan), in.Oleh))
		return err
	})
	return out, err
}

func (s *Store) RiwayatTemplate(ctx context.Context, kunci string) ([]TemplateInfo, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+kolomTemplate+` FROM sapa_template WHERE kunci = @p1 ORDER BY versi DESC`, kunci)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TemplateInfo{}
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- referensi

// Nama UE1 dibaca dari referensi UE1 umum (ref_ue1, migrasi 052) bila kodenya terdaftar di sana; sapa_ref_ue1 tetap menyimpan sebutan
// Sekretaris untuk Nota Dinas dan menjadi cadangan nama.
func (s *Store) AmbilRefUE1(ctx context.Context, kode string) (*RefUE1, error) {
	var r RefUE1
	err := s.DB.QueryRowContext(ctx, `SELECT s.kode, COALESCE(m.nama, s.nama), s.sebutan_sekretaris
		FROM sapa_ref_ue1 s LEFT JOIN ref_ue1 m ON m.kode = s.kode WHERE s.kode = @p1`, kode).Scan(&r.Kode, &r.Nama, &r.Sekretaris)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// DaftarRefUE1 memuat semua UE1 yang sudah punya sebutan Sekretaris, ditambah UE1 aktif dari referensi umum yang belum punya (Sekretaris
// kosong) supaya admin melihat seluruh daftarnya dan tinggal mengisi sebutannya.
func (s *Store) DaftarRefUE1(ctx context.Context) ([]RefUE1, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT k.kode, k.nama, k.sebutan FROM (
			SELECT s.kode, COALESCE(m.nama, s.nama) AS nama, s.sebutan_sekretaris AS sebutan
			FROM sapa_ref_ue1 s LEFT JOIN ref_ue1 m ON m.kode = s.kode
			UNION ALL
			SELECT m.kode, m.nama, N'' FROM ref_ue1 m
			WHERE m.aktif = 1 AND NOT EXISTS (SELECT 1 FROM sapa_ref_ue1 s WHERE s.kode = m.kode)
		) k ORDER BY k.kode`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RefUE1{}
	for rows.Next() {
		var r RefUE1
		if err := rows.Scan(&r.Kode, &r.Nama, &r.Sekretaris); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) SimpanRefUE1(ctx context.Context, r RefUE1, oleh string) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE sapa_ref_ue1 WITH (UPDLOCK, SERIALIZABLE)
			    SET nama = @p2, sebutan_sekretaris = @p3, diubah_oleh = @p4, diubah_pada = SYSUTCDATETIME()
			  WHERE kode = @p1;
			 IF @@ROWCOUNT = 0
			     INSERT INTO sapa_ref_ue1 (kode, nama, sebutan_sekretaris, diubah_oleh) VALUES (@p1, @p2, @p3, @p4);`,
			r.Kode, r.Nama, r.Sekretaris, oleh)
		return err
	})
}

func (s *Store) HapusRefUE1(ctx context.Context, kode string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sapa_ref_ue1 WHERE kode = @p1`, kode)
	return err
}

// SatkerDenganKode6: satker pada data Digitalisasi Aset yang kode satker 6 digitnya (karakter ke-10 sampai ke-15) sama; induk (akhiran 000) lebih dulu.
// Dipakai menyiapkan kode satker lengkap pengguna berperan Satker.
func (s *Store) SatkerDenganKode6(ctx context.Context, kode6 string) ([]SatkerInfo, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT Kode_Satker, Nama_Satker, KabKota_Satker, Provinsi_Satker, Kode_UE1
		   FROM DIGITALISASI_SATKER
		  WHERE Kode_Satker IS NOT NULL AND LEN(Kode_Satker) >= 15 AND SUBSTRING(Kode_Satker, 10, 6) = @p1
		  ORDER BY CASE WHEN SUBSTRING(Kode_Satker, 16, 3) = '000' THEN 0 ELSE 1 END, Kode_Satker`, kode6)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SatkerInfo{}
	for rows.Next() {
		var si SatkerInfo
		var nama, kab, prov, ue1 sql.NullString
		if err := rows.Scan(&si.Kode, &nama, &kab, &prov, &ue1); err != nil {
			return nil, err
		}
		si.Nama, si.KabKota, si.Provinsi, si.KodeUE1 = str(nama), str(kab), str(prov), str(ue1)
		out = append(out, si)
	}
	return out, rows.Err()
}

// CariSatker mencari satker pada data Digitalisasi Aset. Kode satker di sana berakhiran "KP" (18 digit + KP),
// jadi dicocokkan menurut awalan.
func (s *Store) CariSatker(ctx context.Context, kode18 string) (*SatkerInfo, error) {
	var si SatkerInfo
	var nama, kab, prov, ue1 sql.NullString
	err := s.DB.QueryRowContext(ctx,
		`SELECT TOP 1 Kode_Satker, Nama_Satker, KabKota_Satker, Provinsi_Satker, Kode_UE1
		   FROM DIGITALISASI_SATKER WHERE Kode_Satker LIKE @p1 + '%'
		  ORDER BY id`, likeEscape(kode18)).Scan(&si.Kode, &nama, &kab, &prov, &ue1)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	si.Nama, si.KabKota, si.Provinsi, si.KodeUE1 = str(nama), str(kab), str(prov), str(ue1)
	return &si, nil
}
