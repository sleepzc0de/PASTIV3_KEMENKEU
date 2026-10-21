package apiakses

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	mssql "github.com/microsoft/go-mssqldb"
)

// Store membaca dan menulis tabel api_akses_* (migrasi 060). Semua nilai masuk sebagai parameter; nama tabel/kolom hanya dari kode.
type Store struct{ DB *sql.DB }

// Galat yang dikenali penanganan permintaan.
var (
	ErrTidakAda           = errors.New("data tidak ditemukan")
	ErrStatusTidakSesuai  = errors.New("status klien tidak memungkinkan tindakan ini")
	ErrTerlaluBanyakKunci = errors.New("jumlah kunci aktif sudah mencapai batas")
)

// MaksKunciAktif: jumlah kunci aktif per klien. Dua dibutuhkan untuk rotasi tanpa putus layanan; sisanya untuk lingkungan terpisah (mis. uji dan produksi).
const MaksKunciAktif = 5

func potong(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func nullJikaKosong(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func gabung(daftar []string) interface{} {
	if len(daftar) == 0 {
		return nil
	}
	return strings.Join(daftar, ",")
}

func pisah(s string) []string {
	out := []string{}
	for _, e := range strings.Split(s, ",") {
		if e = strings.TrimSpace(e); e != "" {
			out = append(out, e)
		}
	}
	return out
}

// adaGalatUnik: pelanggaran indeks/kendala unik SQL Server (2601, 2627).
func adaGalatUnik(err error) bool {
	var e mssql.Error
	if errors.As(err, &e) {
		return e.Number == 2601 || e.Number == 2627
	}
	return false
}

func intPtr(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func waktuPtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

// ---------------------------------------------------------------- pengaturan

// MuatPengaturan membaca bawaan global. Tabel yang belum ada (migrasi 060 belum dijalankan) atau kosong menghasilkan nilai pabrik.
func (s *Store) MuatPengaturan(ctx context.Context) (Pengaturan, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT kunci, nilai FROM api_akses_pengaturan`)
	if err != nil {
		return PengaturanBawaan(), err
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return PengaturanBawaan(), err
		}
		m[k] = v
	}
	if err := rows.Err(); err != nil {
		return PengaturanBawaan(), err
	}
	return DariBaris(m), nil
}

const sqlSimpanPengaturan = `UPDATE api_akses_pengaturan SET nilai = @p2, diubah_at = SYSUTCDATETIME(), diubah_oleh = @p3 WHERE kunci = @p1;
IF @@ROWCOUNT = 0 INSERT INTO api_akses_pengaturan (kunci, nilai, diubah_oleh) VALUES (@p1, @p2, @p3);`

// SimpanPengaturan menyimpan semua nilai dalam satu transaksi.
func (s *Store) SimpanPengaturan(ctx context.Context, p Pengaturan, oleh string) error {
	if err := p.Validasi(); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for k, v := range p.KeBaris() {
		if _, err := tx.ExecContext(ctx, sqlSimpanPengaturan, k, v, nullJikaKosong(potong(oleh, 100))); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---------------------------------------------------------------- klien

const kolomKlien = `k.id, k.kode, k.nama_aplikasi, k.instansi, ISNULL(k.unit, N''), k.pic_nama, ISNULL(k.pic_jabatan, N''), k.pic_email, ISNULL(k.pic_telepon, N''),
	k.tujuan, k.dataset_diminta, k.status, ISNULL(k.catatan, N''), ISNULL(k.ip_diizinkan, N''), ISNULL(k.ue1_diizinkan, N''),
	k.limit_permintaan, k.limit_jendela_detik, k.kuota_permintaan, k.kuota_periode_detik,
	k.didaftarkan_at, ISNULL(k.didaftarkan_ip, N''), k.diputuskan_at, ISNULL(k.diputuskan_oleh, N''), k.diubah_at,
	(SELECT COUNT(*) FROM api_akses_kunci x WHERE x.klien_id = k.id AND x.dicabut_at IS NULL AND (x.kedaluwarsa_at IS NULL OR x.kedaluwarsa_at > SYSUTCDATETIME())),
	(SELECT MAX(x.terakhir_dipakai_at) FROM api_akses_kunci x WHERE x.klien_id = k.id)`

type pemindai interface {
	Scan(dest ...interface{}) error
}

func pindaiKlien(r pemindai) (Klien, error) {
	var k Klien
	var diminta, ip, ue1 string
	var lp, lj, kp, kd sql.NullInt64
	var diputus, diubah, dipakai sql.NullTime
	if err := r.Scan(&k.ID, &k.Kode, &k.NamaAplikasi, &k.Instansi, &k.Unit, &k.PICNama, &k.PICJabatan, &k.PICEmail, &k.PICTelepon,
		&k.Tujuan, &diminta, &k.Status, &k.Catatan, &ip, &ue1, &lp, &lj, &kp, &kd,
		&k.DidaftarkanAt, &k.DidaftarkanIP, &diputus, &k.DiputuskanOleh, &diubah, &k.KunciAktif, &dipakai); err != nil {
		return k, err
	}
	k.DatasetDiminta, k.IPDiizinkan, k.UE1Diizinkan = pisah(diminta), pisah(ip), pisah(ue1)
	k.DatasetDiberikan = []string{}
	k.LimitPermintaan, k.LimitJendelaDetik, k.KuotaPermintaan, k.KuotaPeriodeDetik = intPtr(lp), intPtr(lj), intPtr(kp), intPtr(kd)
	k.DidaftarkanAt = k.DidaftarkanAt.UTC()
	k.DiputuskanAt, k.DiubahAt, k.TerakhirDipakai = waktuPtr(diputus), waktuPtr(diubah), waktuPtr(dipakai)
	return k, nil
}

// Override mengembalikan override batas klien.
func (k Klien) Override() Override {
	return Override{LimitPermintaan: k.LimitPermintaan, LimitJendelaDetik: k.LimitJendelaDetik, KuotaPermintaan: k.KuotaPermintaan, KuotaPeriodeDetik: k.KuotaPeriodeDetik}
}

// BuatKlien menyimpan klien yang didaftarkan SUPERADMIN: langsung berstatus disetujui, lengkap dengan scope, allowlist IP, pembatasan UE1, dan batas khusus, dalam satu
// transaksi (oleh dan waktu pencatatannya masuk kolom diputuskan_*; ip = alamat superadmin yang mendaftarkan). Tidak ada jalur pendaftaran mandiri: klien baru hanya dapat
// dibuat lewat sini. Isian harus sudah lolos Pendaftaran.Bersihkan dan cakupan sudah divalidasi (DaftarIP, DaftarUE1, NormalisasiDataset, Override.Validasi).
func (s *Store) BuatKlien(ctx context.Context, p Pendaftaran, c UbahCakupan, oleh, ip string) (int64, string, error) {
	var lastErr error
	for i := 0; i < 5; i++ {
		kode, err := BuatKodeKlien()
		if err != nil {
			return 0, "", err
		}
		id, err := s.buatKlienSekali(ctx, kode, p, c, oleh, ip)
		if err == nil {
			return id, kode, nil
		}
		if !adaGalatUnik(err) {
			return 0, "", err
		}
		lastErr = err
	}
	return 0, "", lastErr
}

func (s *Store) buatKlienSekali(ctx context.Context, kode string, p Pendaftaran, c UbahCakupan, oleh, ip string) (int64, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO api_akses_klien (kode, nama_aplikasi, instansi, unit, pic_nama, pic_jabatan, pic_email, pic_telepon, tujuan, dataset_diminta, status,
			catatan, ip_diizinkan, ue1_diizinkan, limit_permintaan, limit_jendela_detik, kuota_permintaan, kuota_periode_detik, didaftarkan_ip, diputuskan_at, diputuskan_oleh)
		OUTPUT inserted.id
		VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9, @p10, N'disetujui', @p11, @p12, @p13, @p14, @p15, @p16, @p17, @p18, SYSUTCDATETIME(), @p19)`,
		kode, p.NamaAplikasi, p.Instansi, nullJikaKosong(p.Unit), p.PICNama, nullJikaKosong(p.PICJabatan), p.PICEmail, nullJikaKosong(p.PICTelepon), p.Tujuan,
		strings.Join(c.DatasetDiberikan, ","), nullJikaKosong(potong(c.Catatan, maksCatatan)), gabung(c.IPDiizinkan), gabung(c.UE1Diizinkan),
		c.Override.LimitPermintaan, c.Override.LimitJendelaDetik, c.Override.KuotaPermintaan, c.Override.KuotaPeriodeDetik, nullJikaKosong(potong(ip, 64)),
		nullJikaKosong(potong(oleh, 100))).Scan(&id)
	if err != nil {
		return 0, err
	}
	if err := gantiDataset(ctx, tx, id, c.DatasetDiberikan); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// PenyaringKlien: syarat daftar klien.
type PenyaringKlien struct {
	Status     string
	Q          string
	Halaman    int
	PerHalaman int
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`, `[`, `\[`).Replace(s)
}

func (p PenyaringKlien) offsetLimit() (int, int) {
	limit := p.PerHalaman
	if limit < 1 || limit > 100 {
		limit = 25
	}
	h := p.Halaman
	if h < 1 {
		h = 1
	}
	return (h - 1) * limit, limit
}

// DaftarKlien: satu halaman klien (menunggu lebih dulu, lalu yang terbaru), beserta total yang cocok. Dataset yang diberikan ikut dimuat.
func (s *Store) DaftarKlien(ctx context.Context, p PenyaringKlien) ([]Klien, int64, error) {
	var w []string
	var args []interface{}
	tambah := func(cond string, v interface{}) {
		args = append(args, v)
		w = append(w, strings.ReplaceAll(cond, "?", fmt.Sprintf("@p%d", len(args))))
	}
	if StatusSah(p.Status) {
		tambah("k.status = ?", p.Status)
	}
	if q := strings.TrimSpace(p.Q); q != "" {
		args = append(args, "%"+escapeLike(potong(q, 100))+"%")
		ph := fmt.Sprintf("@p%d", len(args))
		w = append(w, fmt.Sprintf(`(k.nama_aplikasi LIKE %[1]s ESCAPE '\' OR k.instansi LIKE %[1]s ESCAPE '\' OR k.kode LIKE %[1]s ESCAPE '\' OR k.pic_nama LIKE %[1]s ESCAPE '\' OR k.pic_email LIKE %[1]s ESCAPE '\')`, ph))
	}
	where := ""
	if len(w) > 0 {
		where = " WHERE " + strings.Join(w, " AND ")
	}
	var total int64
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT_BIG(*) FROM api_akses_klien k"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset, limit := p.offsetLimit()
	args = append(args, offset, limit)
	rows, err := s.DB.QueryContext(ctx, fmt.Sprintf(`SELECT %s FROM api_akses_klien k%s
		ORDER BY CASE WHEN k.status = N'menunggu' THEN 0 ELSE 1 END, k.didaftarkan_at DESC, k.id DESC
		OFFSET @p%d ROWS FETCH NEXT @p%d ROWS ONLY`, kolomKlien, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Klien{}
	for rows.Next() {
		k, err := pindaiKlien(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	rows.Close()
	if err := s.muatDataset(ctx, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// muatDataset mengisi DatasetDiberikan semua klien dalam daftar dengan satu query.
func (s *Store) muatDataset(ctx context.Context, ks []Klien) error {
	if len(ks) == 0 {
		return nil
	}
	ph := make([]string, len(ks))
	args := make([]interface{}, len(ks))
	idx := make(map[int64]int, len(ks))
	for i, k := range ks {
		ph[i], args[i], idx[k.ID] = fmt.Sprintf("@p%d", i+1), k.ID, i
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT klien_id, dataset FROM api_akses_klien_dataset WHERE klien_id IN ("+strings.Join(ph, ",")+") ORDER BY dataset", args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var ds string
		if err := rows.Scan(&id, &ds); err != nil {
			return err
		}
		if i, ok := idx[id]; ok {
			ks[i].DatasetDiberikan = append(ks[i].DatasetDiberikan, ds)
		}
	}
	return rows.Err()
}

// HitungStatus: jumlah klien per status (untuk tab daftar).
func (s *Store) HitungStatus(ctx context.Context) (map[string]int64, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT status, COUNT_BIG(*) FROM api_akses_klien GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{StatusMenunggu: 0, StatusDisetujui: 0, StatusDitolak: 0, StatusDicabut: 0}
	for rows.Next() {
		var st string
		var n int64
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		out[st] = n
	}
	return out, rows.Err()
}

// AmbilKlien membaca satu klien lengkap dengan dataset dan kuncinya.
func (s *Store) AmbilKlien(ctx context.Context, id int64) (Klien, error) {
	k, err := pindaiKlien(s.DB.QueryRowContext(ctx, "SELECT "+kolomKlien+" FROM api_akses_klien k WHERE k.id = @p1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Klien{}, ErrTidakAda
	}
	if err != nil {
		return Klien{}, err
	}
	ks := []Klien{k}
	if err := s.muatDataset(ctx, ks); err != nil {
		return Klien{}, err
	}
	k = ks[0]
	if k.Kunci, err = s.DaftarKunci(ctx, id); err != nil {
		return Klien{}, err
	}
	return k, nil
}

// UbahCakupan: scope dan batas yang dapat diubah superadmin pada klien.
type UbahCakupan struct {
	DatasetDiberikan []string
	IPDiizinkan      []string
	UE1Diizinkan     []string
	Override         Override
	Catatan          string
}

// tulisCakupan menyimpan scope, allowlist, pembatasan UE1, dan override batas (tanpa catatan) di dalam transaksi tx. Dataset diganti seluruhnya.
func tulisCakupan(ctx context.Context, tx *sql.Tx, id int64, u UbahCakupan) error {
	res, err := tx.ExecContext(ctx, `UPDATE api_akses_klien SET ip_diizinkan = @p2, ue1_diizinkan = @p3, limit_permintaan = @p4, limit_jendela_detik = @p5,
		kuota_permintaan = @p6, kuota_periode_detik = @p7, diubah_at = SYSUTCDATETIME() WHERE id = @p1`,
		id, gabung(u.IPDiizinkan), gabung(u.UE1Diizinkan), u.Override.LimitPermintaan, u.Override.LimitJendelaDetik, u.Override.KuotaPermintaan, u.Override.KuotaPeriodeDetik)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrTidakAda
	}
	return gantiDataset(ctx, tx, id, u.DatasetDiberikan)
}

// UbahCakupan menyimpan scope, allowlist, pembatasan UE1, override batas, dan catatan klien yang statusnya bukan ditolak (ditolak bersifat final).
func (s *Store) UbahCakupan(ctx context.Context, id int64, u UbahCakupan) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM api_akses_klien WITH (UPDLOCK) WHERE id = @p1`, id).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTidakAda
		}
		return err
	}
	if status == StatusDitolak {
		return ErrStatusTidakSesuai
	}
	if err := tulisCakupan(ctx, tx, id, u); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE api_akses_klien SET catatan = @p2 WHERE id = @p1`, id, nullJikaKosong(potong(u.Catatan, maksCatatan))); err != nil {
		return err
	}
	return tx.Commit()
}

func gantiDataset(ctx context.Context, tx *sql.Tx, id int64, ds []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM api_akses_klien_dataset WHERE klien_id = @p1`, id); err != nil {
		return err
	}
	for _, d := range ds {
		if _, err := tx.ExecContext(ctx, `INSERT INTO api_akses_klien_dataset (klien_id, dataset) VALUES (@p1, @p2)`, id, d); err != nil {
			return err
		}
	}
	return nil
}

// UbahStatus memindahkan klien ke status ke bila statusnya sekarang salah satu dari dari. cak (bila bukan nil) menggantikan scope, allowlist, dan override batas dalam
// transaksi yang sama. Mencabut atau menolak klien ikut mencabut semua kuncinya: mengaktifkan kembali berarti menerbitkan kunci baru, bukan menghidupkan kunci lama.
func (s *Store) UbahStatus(ctx context.Context, id int64, dari []string, ke, oleh, catatan string, cak *UbahCakupan) error {
	if !StatusSah(ke) || len(dari) == 0 {
		return ErrStatusTidakSesuai
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ph := make([]string, len(dari))
	args := []interface{}{id, ke, nullJikaKosong(potong(oleh, 100)), nullJikaKosong(potong(catatan, maksCatatan))}
	for i, d := range dari {
		args = append(args, d)
		ph[i] = fmt.Sprintf("@p%d", len(args))
	}
	res, err := tx.ExecContext(ctx, `UPDATE api_akses_klien SET status = @p2, diputuskan_at = SYSUTCDATETIME(), diputuskan_oleh = @p3, catatan = @p4, diubah_at = SYSUTCDATETIME()
		WHERE id = @p1 AND status IN (`+strings.Join(ph, ",")+`)`, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var ada int
		if e := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_akses_klien WHERE id = @p1`, id).Scan(&ada); e == nil && ada == 0 {
			return ErrTidakAda
		}
		return ErrStatusTidakSesuai
	}
	if ke == StatusDicabut || ke == StatusDitolak {
		if _, err := tx.ExecContext(ctx, `UPDATE api_akses_kunci SET dicabut_at = SYSUTCDATETIME(), dicabut_oleh = @p2, alasan_cabut = N'klien ' + @p3
			WHERE klien_id = @p1 AND dicabut_at IS NULL`, id, nullJikaKosong(potong(oleh, 100)), ke); err != nil {
			return err
		}
	}
	if cak != nil {
		if err := tulisCakupan(ctx, tx, id, *cak); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// OpsiKlien: klien yang pernah aktif (disetujui atau dicabut) untuk pilihan penyaring pemantauan.
type OpsiKlien struct {
	ID     int64  `json:"id"`
	Kode   string `json:"kode"`
	Nama   string `json:"nama"`
	Status string `json:"status"`
}

// DaftarOpsiKlien membaca pilihan klien untuk penyaring.
func (s *Store) DaftarOpsiKlien(ctx context.Context) ([]OpsiKlien, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, kode, nama_aplikasi, status FROM api_akses_klien WHERE status IN (N'disetujui', N'dicabut') ORDER BY nama_aplikasi`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OpsiKlien{}
	for rows.Next() {
		var o OpsiKlien
		if err := rows.Scan(&o.ID, &o.Kode, &o.Nama, &o.Status); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- kunci

// DaftarKunci: semua kunci klien (tanpa rahasia), terbaru dulu.
func (s *Store) DaftarKunci(ctx context.Context, klienID int64) ([]KunciInfo, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, prefix, ISNULL(label, N''), dibuat_at, ISNULL(dibuat_oleh, N''), kedaluwarsa_at, dicabut_at, ISNULL(dicabut_oleh, N''),
		ISNULL(alasan_cabut, N''), terakhir_dipakai_at, ISNULL(terakhir_ip, N'') FROM api_akses_kunci WHERE klien_id = @p1 ORDER BY id DESC`, klienID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := time.Now()
	out := []KunciInfo{}
	for rows.Next() {
		var k KunciInfo
		var kad, cab, pakai sql.NullTime
		if err := rows.Scan(&k.ID, &k.Prefix, &k.Label, &k.DibuatAt, &k.DibuatOleh, &kad, &cab, &k.DicabutOleh, &k.AlasanCabut, &pakai, &k.TerakhirIP); err != nil {
			return nil, err
		}
		k.DibuatAt = k.DibuatAt.UTC()
		k.KedaluwarsaAt, k.DicabutAt, k.TerakhirDipakaiAt = waktuPtr(kad), waktuPtr(cab), waktuPtr(pakai)
		k.Aktif = k.AktifPada(now)
		out = append(out, k)
	}
	return out, rows.Err()
}

// BuatKunciBaru: parameter penerbitan kunci.
type BuatKunciBaru struct {
	KlienID       int64
	Prefix        string
	Hash          string
	Label         string
	Oleh          string
	KedaluwarsaAt *time.Time
	// CabutLama: kunci aktif lain milik klien ini dihentikan (rotasi). Tenggang > 0: kunci lama tetap berlaku sampai sekian lama lalu kedaluwarsa; 0: langsung dicabut.
	CabutLama bool
	Tenggang  time.Duration
}

// TerbitkanKunci menyimpan kunci baru (hanya hash) milik klien yang berstatus disetujui, dan bila rotasi, menghentikan kunci lama. Pelanggaran prefix unik dikembalikan
// apa adanya (pemanggil mencoba dengan kunci lain).
func (s *Store) TerbitkanKunci(ctx context.Context, b BuatKunciBaru) (int64, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM api_akses_klien WITH (UPDLOCK) WHERE id = @p1`, b.KlienID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrTidakAda
		}
		return 0, err
	}
	if status != StatusDisetujui {
		return 0, ErrStatusTidakSesuai
	}
	if !b.CabutLama {
		var aktif int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_akses_kunci WHERE klien_id = @p1 AND dicabut_at IS NULL AND (kedaluwarsa_at IS NULL OR kedaluwarsa_at > SYSUTCDATETIME())`, b.KlienID).Scan(&aktif); err != nil {
			return 0, err
		}
		if aktif >= MaksKunciAktif {
			return 0, ErrTerlaluBanyakKunci
		}
	}
	var kad interface{}
	if b.KedaluwarsaAt != nil {
		kad = b.KedaluwarsaAt.UTC()
	}
	var id int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO api_akses_kunci (klien_id, prefix, hash, label, dibuat_oleh, kedaluwarsa_at)
		OUTPUT inserted.id VALUES (@p1, @p2, @p3, @p4, @p5, CAST(@p6 AS DATETIME2(0)))`,
		b.KlienID, b.Prefix, b.Hash, nullJikaKosong(potong(b.Label, 100)), nullJikaKosong(potong(b.Oleh, 100)), kad).Scan(&id); err != nil {
		return 0, err
	}
	if b.CabutLama {
		if b.Tenggang <= 0 {
			_, err = tx.ExecContext(ctx, `UPDATE api_akses_kunci SET dicabut_at = SYSUTCDATETIME(), dicabut_oleh = @p3, alasan_cabut = N'diganti (rotasi)'
				WHERE klien_id = @p1 AND id <> @p2 AND dicabut_at IS NULL`, b.KlienID, id, nullJikaKosong(potong(b.Oleh, 100)))
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE api_akses_kunci SET kedaluwarsa_at = CASE WHEN kedaluwarsa_at IS NULL OR kedaluwarsa_at > @p3 THEN @p3 ELSE kedaluwarsa_at END
				WHERE klien_id = @p1 AND id <> @p2 AND dicabut_at IS NULL`, b.KlienID, id, time.Now().Add(b.Tenggang).UTC())
		}
		if err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// CabutKunci mencabut satu kunci klien. Kunci yang sudah dicabut atau bukan milik klien itu dianggap tidak ada.
func (s *Store) CabutKunci(ctx context.Context, klienID, kunciID int64, oleh, alasan string) (prefix string, err error) {
	err = s.DB.QueryRowContext(ctx, `UPDATE api_akses_kunci SET dicabut_at = SYSUTCDATETIME(), dicabut_oleh = @p3, alasan_cabut = @p4
		OUTPUT inserted.prefix WHERE id = @p1 AND klien_id = @p2 AND dicabut_at IS NULL`,
		kunciID, klienID, nullJikaKosong(potong(oleh, 100)), nullJikaKosong(potong(alasan, 200))).Scan(&prefix)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrTidakAda
	}
	return prefix, err
}

// Kredensial: semua yang dibutuhkan untuk menilai satu kunci yang masuk.
type Kredensial struct {
	KunciID       int64
	KlienID       int64
	Prefix        string
	Hash          string
	KedaluwarsaAt *time.Time
	DicabutAt     *time.Time
	KlienKode     string
	KlienNama     string
	KlienStatus   string
	IP            []string
	UE1           []string
	Dataset       []string
	Override      Override
}

// CariKredensial mencari kunci menurut prefix. nil, nil bila tidak ada.
func (s *Store) CariKredensial(ctx context.Context, prefix string) (*Kredensial, error) {
	var c Kredensial
	var kad, cab sql.NullTime
	var ip, ue1 string
	var lp, lj, kp, kd sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `SELECT k.id, k.klien_id, k.prefix, k.hash, k.kedaluwarsa_at, k.dicabut_at, c.kode, c.nama_aplikasi, c.status,
		ISNULL(c.ip_diizinkan, N''), ISNULL(c.ue1_diizinkan, N''), c.limit_permintaan, c.limit_jendela_detik, c.kuota_permintaan, c.kuota_periode_detik
		FROM api_akses_kunci k JOIN api_akses_klien c ON c.id = k.klien_id WHERE k.prefix = @p1`, prefix).
		Scan(&c.KunciID, &c.KlienID, &c.Prefix, &c.Hash, &kad, &cab, &c.KlienKode, &c.KlienNama, &c.KlienStatus, &ip, &ue1, &lp, &lj, &kp, &kd)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.KedaluwarsaAt, c.DicabutAt = waktuPtr(kad), waktuPtr(cab)
	c.IP, c.UE1 = pisah(ip), pisah(ue1)
	c.Override = Override{LimitPermintaan: intPtr(lp), LimitJendelaDetik: intPtr(lj), KuotaPermintaan: intPtr(kp), KuotaPeriodeDetik: intPtr(kd)}
	rows, err := s.DB.QueryContext(ctx, `SELECT dataset FROM api_akses_klien_dataset WHERE klien_id = @p1`, c.KlienID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	c.Dataset = []string{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		c.Dataset = append(c.Dataset, d)
	}
	return &c, rows.Err()
}

// Pakai: kunci yang dipakai (untuk "terakhir dipakai").
type Pakai struct {
	KunciID int64
	Waktu   time.Time
	IP      string
}

// TandaiDipakai memperbarui waktu dan IP terakhir pemakaian kunci (dikelompokkan, tidak per permintaan).
func (s *Store) TandaiDipakai(ctx context.Context, ps []Pakai) error {
	if len(ps) == 0 {
		return nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, p := range ps {
		if _, err := tx.ExecContext(ctx, `UPDATE api_akses_kunci SET terakhir_dipakai_at = CAST(@p2 AS DATETIME2(0)), terakhir_ip = @p3 WHERE id = @p1`,
			p.KunciID, p.Waktu.UTC(), nullJikaKosong(potong(p.IP, 64))); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DaftarAktif: klien yang disetujui beserta override batasnya (untuk tampilan pemakaian kuota saat ini).
func (s *Store) DaftarAktif(ctx context.Context) ([]Klien, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT "+kolomKlien+" FROM api_akses_klien k WHERE k.status = N'disetujui' ORDER BY k.nama_aplikasi")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Klien{}
	for rows.Next() {
		k, err := pindaiKlien(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- kuota

// MuatKuota membaca penghitung kuota yang tersimpan (dipakai saat aplikasi mulai).
func (s *Store) MuatKuota(ctx context.Context) ([]KuotaSnapshot, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT klien_id, periode_mulai, jumlah FROM api_akses_kuota`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []KuotaSnapshot
	for rows.Next() {
		var k KuotaSnapshot
		if err := rows.Scan(&k.KlienID, &k.Mulai, &k.Jumlah); err != nil {
			return nil, err
		}
		k.Mulai = k.Mulai.UTC()
		out = append(out, k)
	}
	return out, rows.Err()
}

const sqlSimpanKuota = `UPDATE api_akses_kuota SET periode_mulai = CAST(@p2 AS DATETIME2(0)), jumlah = @p3, diubah_at = SYSUTCDATETIME() WHERE klien_id = @p1;
IF @@ROWCOUNT = 0 INSERT INTO api_akses_kuota (klien_id, periode_mulai, jumlah) VALUES (@p1, CAST(@p2 AS DATETIME2(0)), @p3);`

// SimpanKuota menyalin penghitung kuota ke database.
func (s *Store) SimpanKuota(ctx context.Context, ss []KuotaSnapshot) error {
	if len(ss) == 0 {
		return nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, k := range ss {
		if _, err := tx.ExecContext(ctx, sqlSimpanKuota, k.KlienID, k.Mulai.UTC(), k.Jumlah); err != nil {
			return err
		}
	}
	return tx.Commit()
}
