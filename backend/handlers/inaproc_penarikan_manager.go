package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"pasti-v3-backend/database"
)

// Pengelola penarikan data Pengadaan/Tender/E-Katalog. Penarikan berjalan di latar belakang (di luar permintaan HTTP) dan hanya
// satu antrean berjalan pada satu waktu: Inaproc membatasi laju permintaan (429), dan penggantian data memakai transaksi yang
// berat. Antrean dibuat dari halaman Penarikan Data (manual) atau penjadwal (otomatis); keduanya lewat Start.

// Status tugas penarikan (kolom inaproc_penarikan.status).
const (
	PenarikanAntri      = "antri"
	PenarikanBerjalan   = "berjalan"
	PenarikanSukses     = "sukses"
	PenarikanGagal      = "gagal"
	PenarikanDibatalkan = "dibatalkan"
	PenarikanDilewati   = "dilewati"
)

// Pemicu penarikan (kolom inaproc_penarikan.pemicu).
const (
	PemicuManual    = "manual"
	PemicuOtomatis  = "otomatis"
	namaPengirimOto = "otomatis"

	// Batas tugas dalam satu antrean: menolak permintaan yang tidak masuk akal.
	maksTugasPerAntrean = 500
)

var (
	ErrPenarikanSibuk = errors.New("penarikan lain sedang berjalan")
	ErrTidakAdaTugas  = errors.New("tidak ada tugas yang dipilih")
)

// Variabel (bukan konstanta) hanya supaya tes bisa mempercepatnya.
var (
	// Jeda bawaan antar tugas (dapat diganti pengaturan).
	jedaAntarTugasBawaan = 2 * time.Second
	// Pemutus beruntun: sekian tugas gagal berturut-turut menghentikan antrean (sisanya dilewati, bukan dihitung gagal) dan menahan
	// penarikan otomatis selama jedaPemutus. Mencegah semua tugas menghabiskan kesempatannya saat Inaproc atau jaringan sedang bermasalah.
	ambangPemutus = 5
	jedaPemutus   = 30 * time.Minute
)

// kunciAppLock: nama kunci aplikasi SQL Server yang menjaga hanya satu penarikan berjalan di seluruh server (juga saat dua salinan backend hidup
// bersamaan sebentar ketika deploy).
const kunciAppLock = "pasti_inaproc_penarikan"

// Tugas: satu dataset dengan satu isian penarikan (sudah dinormalisasi dan divalidasi).
type Tugas struct {
	Dataset *DatasetPenarikan
	Perm    PermintaanTarik
	// Percobaan: urutan percobaan dalam siklus gagal-tarik (1 = pertama). 0 dianggap 1.
	Percobaan int
}

// Kunci: pengenal tugas untuk jatuh tempo penarikan otomatis dan pencegahan ganda.
func (t Tugas) Kunci() string { return t.Dataset.ID + "|" + t.Dataset.Ringkas(t.Perm) }

// Oleh: pemicu penarikan. ID = id pengguna (kolom synced_by sync_log); kosong untuk penarikan otomatis.
type Oleh struct {
	ID   string
	Nama string
}

// pelaksanaTugas dipisah supaya pengelola bisa diuji tanpa Inaproc.
type pelaksanaTugas func(ctx context.Context, t Tugas, oleh string, kabar func(string)) (HasilSinkron, error)

func pelaksanaBawaan(ctx context.Context, t Tugas, oleh string, kabar func(string)) (HasilSinkron, error) {
	return t.Dataset.Jalankan(ctx, t.Perm, oleh, kabar)
}

// TugasInfo: keadaan satu tugas dalam antrean yang sedang berjalan (dikirim ke halaman).
type TugasInfo struct {
	ID          int64      `json:"id"`
	Dataset     string     `json:"dataset"`
	Nama        string     `json:"nama"`
	Parameter   string     `json:"parameter"`
	Status      string     `json:"status"`
	Pesan       string     `json:"pesan"`
	JumlahBaris int        `json:"jumlah_baris"`
	BarisGagal  int        `json:"baris_gagal"`
	Percobaan   int        `json:"percobaan"`
	Mulai       *time.Time `json:"mulai"`
	Selesai     *time.Time `json:"selesai"`
}

// InfoAktif: antrean yang sedang berjalan.
type InfoAktif struct {
	BatchID    string      `json:"batch_id"`
	Pemicu     string      `json:"pemicu"`
	Oleh       string      `json:"oleh"`
	Mulai      time.Time   `json:"mulai"`
	Dibatalkan bool        `json:"dibatalkan"`
	Total      int         `json:"total"`
	Selesai    int         `json:"selesai"`
	Tugas      []TugasInfo `json:"tugas"`
}

type tugasAktif struct {
	Tugas
	info TugasInfo
}

type antreanAktif struct {
	info   InfoAktif
	tugas  []*tugasAktif
	oleh   Oleh
	jeda   time.Duration
	batal  context.CancelFunc
	galat  string // diisi bila antrean dihentikan karena galat yang membuat sisanya sia-sia (token ditolak)
	// ditolak: dataset yang dijawab 403 oleh Inaproc (id dataset → alasan); sisa tugas dataset itu dilewati, dataset lain tetap jalan.
	ditolak map[string]string
	dbatal  bool
	kunci  *sql.Conn // koneksi yang memegang kunci aplikasi SQL Server selama antrean berjalan (nil bila tidak bisa dipastikan)
}

// PenarikInaproc mengelola antrean, riwayat, dan penjadwal.
type PenarikInaproc struct {
	db       *sql.DB
	jalankan pelaksanaTugas

	mu    sync.Mutex
	aktif *antreanAktif
	// tahanOtomatis: penarikan otomatis ditahan sampai saat ini (setelah pemutus beruntun).
	tahanOtomatis time.Time
}

// Penarik dipakai handler HTTP; diisi InitInaprocPenarikan saat server dimulai.
var Penarik *PenarikInaproc

func NewPenarikInaproc(db *sql.DB) *PenarikInaproc {
	return &PenarikInaproc{db: db, jalankan: pelaksanaBawaan}
}

// RecoverOrphans: penarikan berjalan di dalam proses server, jadi saat server (re)start tidak mungkin ada yang benar-benar
// berjalan. Baris yang masih "antri"/"berjalan" adalah sisa proses sebelumnya. Ditandai dibatalkan, bukan gagal: server dimulai ulang bukan
// kesalahan sumber data, jadi tidak boleh menghabiskan kesempatan percobaan tugasnya.
func (m *PenarikInaproc) RecoverOrphans(ctx context.Context) (int64, error) {
	res, err := m.db.ExecContext(ctx,
		`UPDATE inaproc_penarikan SET status = @p1, selesai = SYSUTCDATETIME(), pesan = @p2 WHERE status IN (@p3, @p4)`,
		PenarikanDibatalkan, "Dihentikan karena server dimulai ulang", PenarikanAntri, PenarikanBerjalan)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Aktif: salinan keadaan antrean yang sedang berjalan, atau nil.
func (m *PenarikInaproc) Aktif() *InfoAktif {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.aktif == nil {
		return nil
	}
	return m.salinInfo(m.aktif)
}

// salinInfo: pemanggil memegang m.mu.
func (m *PenarikInaproc) salinInfo(a *antreanAktif) *InfoAktif {
	cp := a.info
	cp.Dibatalkan = a.dbatal
	cp.Tugas = make([]TugasInfo, len(a.tugas))
	cp.Selesai = 0
	for i, t := range a.tugas {
		cp.Tugas[i] = t.info
		switch t.info.Status {
		case PenarikanSukses, PenarikanGagal, PenarikanDibatalkan, PenarikanDilewati:
			cp.Selesai++
		}
	}
	return &cp
}

// Batalkan menghentikan antrean yang sedang berjalan: tugas yang sedang berjalan dibatalkan (data lama tetap utuh untuk dataset
// generik; dataset RUP/non-tender menyelesaikan dulu tugas itu) dan sisanya tidak dimulai.
func (m *PenarikInaproc) Batalkan() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.aktif == nil {
		return false
	}
	m.aktif.dbatal = true
	m.aktif.batal()
	return true
}

// Start memasukkan tugas ke antrean dan langsung kembali; pekerjaannya berjalan di goroutine. Tugas ganda (kunci sama) digabung
// dan urutannya mengikuti urutan masukan.
func (m *PenarikInaproc) Start(tugas []Tugas, pemicu string, oleh Oleh, jeda time.Duration) (InfoAktif, error) {
	var uniq []Tugas
	sudah := map[string]bool{}
	for _, t := range tugas {
		if k := t.Kunci(); !sudah[k] {
			sudah[k] = true
			uniq = append(uniq, t)
		}
	}
	if len(uniq) == 0 {
		return InfoAktif{}, ErrTidakAdaTugas
	}
	if len(uniq) > maksTugasPerAntrean {
		return InfoAktif{}, fmt.Errorf("terlalu banyak tugas dalam satu antrean (maksimal %d)", maksTugasPerAntrean)
	}
	if jeda < 0 {
		jeda = 0
	}

	m.mu.Lock()
	if m.aktif != nil {
		m.mu.Unlock()
		return InfoAktif{}, ErrPenarikanSibuk
	}
	ctx, batal := context.WithCancel(context.Background())
	run := &antreanAktif{
		info: InfoAktif{BatchID: uuid.NewString(), Pemicu: pemicu, Oleh: oleh.Nama, Mulai: time.Now().UTC(), Total: len(uniq)},
		oleh: oleh, jeda: jeda, batal: batal,
	}
	// Dicatat sebelum kunci dilepas supaya permintaan kedua yang datang bersamaan ditolak.
	m.aktif = run
	m.mu.Unlock()

	// Kunci di database: menutup celah bila ada salinan backend lain yang sedang menarik (mis. saat deploy).
	kunci, err := m.ambilKunciDB()
	if err != nil {
		batal()
		m.lepas(run)
		return InfoAktif{}, err
	}
	run.kunci = kunci

	for _, t := range uniq {
		id, err := m.enqueue(run.info.BatchID, t, pemicu, oleh.Nama)
		if err != nil {
			batal()
			m.lepas(run)
			// Baris yang sudah terlanjur dibuat dibatalkan agar tidak menggantung.
			for _, ta := range run.tugas {
				m.selesaiLog(ta.info.ID, PenarikanDibatalkan, HasilSinkron{}, "Antrean gagal dibuat", ta.info.Percobaan)
			}
			return InfoAktif{}, fmt.Errorf("gagal mencatat antrean: %w", err)
		}
		ta := &tugasAktif{Tugas: t, info: TugasInfo{
			ID: id, Dataset: t.Dataset.ID, Nama: t.Dataset.Nama, Parameter: t.Dataset.Ringkas(t.Perm), Status: PenarikanAntri, Percobaan: percobaanKe(t),
		}}
		m.mu.Lock()
		run.tugas = append(run.tugas, ta)
		m.mu.Unlock()
	}

	go m.eksekusi(ctx, run)
	m.mu.Lock()
	defer m.mu.Unlock()
	return *m.salinInfo(run), nil
}

func (m *PenarikInaproc) lepas(run *antreanAktif) {
	m.lepasKunciDB(run.kunci)
	run.kunci = nil
	m.mu.Lock()
	if m.aktif == run {
		m.aktif = nil
	}
	m.mu.Unlock()
	kosongkanCacheAnalitik() // data berubah: dasbor harus dihitung ulang
}

func percobaanKe(t Tugas) int {
	if t.Percobaan < 1 {
		return 1
	}
	return t.Percobaan
}

// ambilKunciDB mengambil kunci aplikasi SQL Server (sp_getapplock, milik sesi, tanpa menunggu) pada satu koneksi yang ditahan selama
// penarikan. Mengembalikan ErrPenarikanSibuk bila kunci dipegang pihak lain. Bila kunci tidak bisa dipastikan (database bermasalah), penarikan
// tetap berjalan dengan penjagaan dalam proses saja dan masalahnya dicatat; kunci ini penjaga tambahan, bukan syarat.
func (m *PenarikInaproc) ambilKunciDB() (*sql.Conn, error) {
	ctx, cancel := bgCtx()
	defer cancel()
	conn, err := m.db.Conn(ctx)
	if err != nil {
		log.Println("[INAPROC PENARIKAN WARN] tidak bisa mengambil koneksi untuk kunci penarikan:", err)
		return nil, nil
	}
	var kode sql.NullInt64
	err = conn.QueryRowContext(ctx,
		`DECLARE @r INT; EXEC @r = sp_getapplock @Resource = N'`+kunciAppLock+`', @LockMode = N'Exclusive', @LockOwner = N'Session', @LockTimeout = 0; SELECT @r`).Scan(&kode)
	if err != nil {
		_ = conn.Close()
		log.Println("[INAPROC PENARIKAN WARN] tidak bisa memastikan kunci penarikan di database:", err)
		return nil, nil
	}
	if kode.Valid && kode.Int64 < 0 {
		_ = conn.Close()
		return nil, ErrPenarikanSibuk
	}
	return conn, nil
}

func (m *PenarikInaproc) lepasKunciDB(conn *sql.Conn) {
	if conn == nil {
		return
	}
	ctx, cancel := bgCtx()
	defer cancel()
	if _, err := conn.ExecContext(ctx, `EXEC sp_releaseapplock @Resource = N'`+kunciAppLock+`', @LockOwner = N'Session'`); err != nil {
		log.Println("[INAPROC PENARIKAN WARN] gagal melepas kunci penarikan:", err)
	}
	_ = conn.Close() // menutup koneksi juga melepas kunci sesi
}

// TahanOtomatis menahan penarikan otomatis sampai t (pemutus beruntun atau gangguan Inaproc).
func (m *PenarikInaproc) TahanOtomatis(sampai time.Time) {
	m.mu.Lock()
	if sampai.After(m.tahanOtomatis) {
		m.tahanOtomatis = sampai
	}
	m.mu.Unlock()
}

// otomatisDitahanSampai: sampai kapan penarikan otomatis ditahan (nol = tidak).
func (m *PenarikInaproc) otomatisDitahanSampai() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tahanOtomatis
}

// ubah menerapkan perubahan pada info tugas di bawah kunci.
func (m *PenarikInaproc) ubah(t *tugasAktif, f func(*TugasInfo)) {
	m.mu.Lock()
	f(&t.info)
	m.mu.Unlock()
}

// tidur menunggu d atau sampai ctx berakhir; false bila dibatalkan.
func tidur(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (m *PenarikInaproc) eksekusi(ctx context.Context, run *antreanAktif) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[INAPROC PENARIKAN ERROR] panic saat penarikan: %v", r)
			for _, t := range run.tugas {
				if m.statusTugas(t) == PenarikanAntri || m.statusTugas(t) == PenarikanBerjalan {
					m.akhiri(t, PenarikanGagal, HasilSinkron{}, "Terjadi kesalahan internal")
				}
			}
		}
		run.batal()
		m.lepas(run)
	}()

	gagalBeruntun := 0
	for i, t := range run.tugas {
		switch {
		case ctx.Err() != nil:
			m.akhiri(t, PenarikanDibatalkan, HasilSinkron{}, "Dibatalkan sebelum dimulai")
			continue
		case run.galat != "":
			m.akhiri(t, PenarikanDilewati, HasilSinkron{}, run.galat)
			continue
		case run.ditolak[t.Dataset.ID] != "":
			m.akhiri(t, PenarikanDilewati, HasilSinkron{}, run.ditolak[t.Dataset.ID])
			continue
		}

		m.ubah(t, func(i *TugasInfo) {
			now := time.Now().UTC()
			i.Status, i.Mulai, i.Pesan = PenarikanBerjalan, &now, "Memulai"
		})
		m.tandaiBerjalan(t.info.ID)

		hasil, err := m.jalankanTugas(ctx, run, t)
		switch {
		case err == nil:
			gagalBeruntun = 0
			m.akhiri(t, PenarikanSukses, hasil, catatanHasil(hasil))
		case ctx.Err() != nil:
			m.akhiri(t, PenarikanDibatalkan, HasilSinkron{}, "Dibatalkan oleh pengguna")
		default:
			var g *GalatSinkron
			pesan := err.Error()
			if errors.As(err, &g) {
				pesan = g.Pesan
				switch {
				// Token salah atau tidak ada: sisanya pasti ikut ditolak, jadi tidak dicoba satu per satu. 503 dari Inaproc sendiri
				// (layanan tidak tersedia) bukan soal token; itu ditangani pemutus beruntun di bawah.
				case g.Status == http.StatusUnauthorized || (g.Status == http.StatusServiceUnavailable && !g.Hulu):
					run.galat = "Dilewati: Inaproc menolak akses (" + pesan + ")"
				// 403 berlaku per dataset: izin token di Inaproc bisa berbeda tiap endpoint, dan dataset sebelumnya mungkin sudah berhasil
				// ditarik dengan token yang sama. Sisa tugas dataset ini dilewati; dataset lain tetap dicoba (bila semuanya ditolak,
				// pemutus beruntun yang menghentikan antrean).
				case g.Status == http.StatusForbidden:
					if run.ditolak == nil {
						run.ditolak = map[string]string{}
					}
					run.ditolak[t.Dataset.ID] = "Dilewati: Inaproc menolak akses ke dataset ini (" + pesan + ")"
				}
			}
			m.akhiri(t, PenarikanGagal, HasilSinkron{}, pesan)
			log.Printf("[INAPROC PENARIKAN] %s (%s) gagal: %s", t.info.Dataset, t.info.Parameter, pesan)
			gagalBeruntun++
			if run.galat == "" && ambangPemutus > 0 && gagalBeruntun >= ambangPemutus {
				run.galat = fmt.Sprintf("Dilewati: %d tugas gagal berturut-turut, Inaproc atau jaringan sedang bermasalah. Penarikan dilanjutkan otomatis setelah jeda %s", gagalBeruntun, jedaPemutus)
				m.TahanOtomatis(time.Now().Add(jedaPemutus))
				log.Printf("[INAPROC PENARIKAN] pemutus beruntun: %d kegagalan berturut-turut, antrean dihentikan dan penarikan otomatis ditahan %s", gagalBeruntun, jedaPemutus)
			}
		}

		if i < len(run.tugas)-1 && ctx.Err() == nil && run.galat == "" {
			tidur(ctx, run.jeda)
		}
	}
}

func (m *PenarikInaproc) statusTugas(t *tugasAktif) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return t.info.Status
}

// jalankanTugas menjalankan satu tugas. Sebelum mulai, ditunggu sampai kuota per jam cukup untuk perkiraan jumlah permintaannya (dari
// penarikan sukses terakhir tugas yang sama), supaya penarikan tidak berhenti di tengah karena kuota habis, khususnya dataset RUP/non-tender
// lama yang menghapus data lama lebih dulu. Gangguan sementara per halaman (429, 5xx, jaringan) dicoba ulang di klien Inaproc
// (inaproc_klien.go), jadi penarikan dilanjutkan dari halaman yang gagal; di sini tidak ada pengulangan seluruh tugas. Tugas yang tetap
// gagal dicatat dan dicoba ulang oleh kebijakan percobaan (inaproc_penarikan_jadwal.go).
func (m *PenarikInaproc) jalankanTugas(ctx context.Context, run *antreanAktif, t *tugasAktif) (HasilSinkron, error) {
	terakhirTulis := time.Time{}
	kabar := func(pesan string) {
		m.ubah(t, func(i *TugasInfo) { i.Pesan = pesan })
		// Pesan kemajuan ditulis ke database paling cepat tiap 3 detik.
		if time.Since(terakhirTulis) < 3*time.Second {
			return
		}
		terakhirTulis = time.Now()
		m.tulisPesan(t.info.ID, pesan)
	}
	ctx = denganKabar(ctx, kabar)

	if err := batas().Pastikan(ctx, m.perkiraanPermintaan(ctx, t.Tugas)); err != nil {
		return HasilSinkron{}, &GalatSinkron{Status: statusDibatalkan, Pesan: "Dibatalkan"}
	}
	return m.jalankan(ctx, t.Tugas, run.oleh.ID, kabar)
}

// perkiraanPermintaan: jumlah permintaan yang dibutuhkan tugas ini, dari jumlah halaman penarikan suksesnya yang terakhir (dengan
// cadangan 30% dan satu permintaan uji); tanpa riwayat dipakai 30.
func (m *PenarikInaproc) perkiraanPermintaan(ctx context.Context, t Tugas) int {
	const bawaan = 30
	var halaman sql.NullInt64
	err := m.db.QueryRowContext(ctx,
		`SELECT TOP 1 halaman FROM inaproc_penarikan WHERE dataset = @p1 AND parameter = @p2 AND status = @p3 AND halaman IS NOT NULL ORDER BY id DESC`,
		t.Dataset.ID, potong(t.Dataset.Ringkas(t.Perm), 200), PenarikanSukses).Scan(&halaman)
	if err != nil || !halaman.Valid || halaman.Int64 < 1 {
		return bawaan
	}
	return int(halaman.Int64)*13/10 + 2
}

// akhiri menandai tugas selesai di memori dan di riwayat.
func (m *PenarikInaproc) akhiri(t *tugasAktif, status string, hasil HasilSinkron, pesan string) {
	var percobaan int
	m.ubah(t, func(i *TugasInfo) {
		now := time.Now().UTC()
		i.Status, i.Selesai, i.Pesan = status, &now, pesan
		if status == PenarikanSukses {
			i.JumlahBaris, i.BarisGagal = hasil.TotalSinkron, hasil.TotalGagal
		}
		percobaan = i.Percobaan
	})
	m.selesaiLog(t.info.ID, status, hasil, pesan, percobaan)
}

// ---- riwayat (inaproc_penarikan) ----

func bgCtx() (context.Context, context.CancelFunc) {
	// Pencatatan hasil harus tetap berhasil walau konteks penarikan sudah dibatalkan.
	return context.WithTimeout(context.Background(), 30*time.Second)
}

func (m *PenarikInaproc) enqueue(batchID string, t Tugas, pemicu, oleh string) (int64, error) {
	ctx, cancel := bgCtx()
	defer cancel()
	var id int64
	// permintaan: isian tugas sebagai JSON, supaya tugas yang gagal bisa dicoba ulang persis sama (termasuk yang bukan bagian rencana otomatis).
	perm, _ := json.Marshal(t.Perm)
	err := m.db.QueryRowContext(ctx,
		`INSERT INTO inaproc_penarikan (batch_id, dataset, parameter, pemicu, status, dijalankan_oleh, percobaan, permintaan)
		 OUTPUT INSERTED.id VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8)`,
		batchID, t.Dataset.ID, potong(t.Dataset.Ringkas(t.Perm), 200), pemicu, PenarikanAntri, nullIfEmpty(potong(oleh, 100)),
		percobaanKe(t), string(perm)).Scan(&id)
	return id, err
}

func (m *PenarikInaproc) tandaiBerjalan(id int64) {
	ctx, cancel := bgCtx()
	defer cancel()
	if _, err := m.db.ExecContext(ctx,
		`UPDATE inaproc_penarikan SET status = @p1, mulai = SYSUTCDATETIME(), pesan = @p2 WHERE id = @p3`,
		PenarikanBerjalan, "Memulai", id); err != nil {
		log.Println("[INAPROC PENARIKAN WARN] gagal mencatat status berjalan:", err)
	}
}

func (m *PenarikInaproc) tulisPesan(id int64, pesan string) {
	ctx, cancel := bgCtx()
	defer cancel()
	if _, err := m.db.ExecContext(ctx, `UPDATE inaproc_penarikan SET pesan = @p1 WHERE id = @p2 AND status = @p3`,
		potong(pesan, 1000), id, PenarikanBerjalan); err != nil {
		log.Println("[INAPROC PENARIKAN WARN] gagal mencatat kemajuan:", err)
	}
}

func (m *PenarikInaproc) selesaiLog(id int64, status string, hasil HasilSinkron, pesan string, percobaan int) {
	ctx, cancel := bgCtx()
	defer cancel()
	var baris, gagal, halaman interface{}
	if status == PenarikanSukses {
		baris, gagal, halaman = hasil.TotalSinkron, hasil.TotalGagal, hasil.Halaman
	}
	if percobaan < 1 {
		percobaan = 1
	}
	if _, err := m.db.ExecContext(ctx,
		`UPDATE inaproc_penarikan SET status = @p1, selesai = SYSUTCDATETIME(), jumlah_baris = @p2, baris_gagal = @p3, halaman = @p4,
		   percobaan = @p5, pesan = @p6 WHERE id = @p7`,
		status, baris, gagal, halaman, percobaan, nullIfEmpty(potong(pesan, 1000)), id); err != nil {
		log.Println("[INAPROC PENARIKAN WARN] gagal mencatat hasil penarikan:", err)
	}
}

// RiwayatPenarikan: satu baris riwayat.
type RiwayatPenarikan struct {
	ID             int64      `json:"id"`
	BatchID        string     `json:"batch_id"`
	Dataset        string     `json:"dataset"`
	Parameter      string     `json:"parameter"`
	Pemicu         string     `json:"pemicu"`
	Status         string     `json:"status"`
	JumlahBaris    *int64     `json:"jumlah_baris"`
	BarisGagal     *int64     `json:"baris_gagal"`
	Halaman        *int64     `json:"halaman"`
	Percobaan      int        `json:"percobaan"`
	Pesan          *string    `json:"pesan"`
	DijalankanOleh *string    `json:"dijalankan_oleh"`
	Dibuat         time.Time  `json:"dibuat"`
	Mulai          *time.Time `json:"mulai"`
	Selesai        *time.Time `json:"selesai"`
}

const kolomRiwayat = `id, batch_id, dataset, parameter, pemicu, status, jumlah_baris, baris_gagal, halaman, percobaan, pesan, dijalankan_oleh, dibuat, mulai, selesai`

func pindaiRiwayat(rows *sql.Rows) (RiwayatPenarikan, error) {
	var e RiwayatPenarikan
	var baris, gagal, halaman sql.NullInt64
	var pesan, oleh sql.NullString
	var mulai, selesai sql.NullTime
	if err := rows.Scan(&e.ID, &e.BatchID, &e.Dataset, &e.Parameter, &e.Pemicu, &e.Status, &baris, &gagal, &halaman, &e.Percobaan,
		&pesan, &oleh, &e.Dibuat, &mulai, &selesai); err != nil {
		return e, err
	}
	if baris.Valid {
		e.JumlahBaris = &baris.Int64
	}
	if gagal.Valid {
		e.BarisGagal = &gagal.Int64
	}
	if halaman.Valid {
		e.Halaman = &halaman.Int64
	}
	if pesan.Valid {
		e.Pesan = &pesan.String
	}
	if oleh.Valid {
		e.DijalankanOleh = &oleh.String
	}
	if mulai.Valid {
		e.Mulai = &mulai.Time
	}
	if selesai.Valid {
		e.Selesai = &selesai.Time
	}
	return e, nil
}

func (m *PenarikInaproc) queryRiwayat(ctx context.Context, q string, args ...interface{}) ([]RiwayatPenarikan, error) {
	rows, err := m.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RiwayatPenarikan{}
	for rows.Next() {
		e, err := pindaiRiwayat(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Riwayat: riwayat terbaru, terbaru dulu; dataset kosong = semua dataset.
func (m *PenarikInaproc) Riwayat(ctx context.Context, dataset string, limit int) ([]RiwayatPenarikan, error) {
	if limit < 1 || limit > 500 {
		limit = 50
	}
	q := fmt.Sprintf(`SELECT TOP (%d) %s FROM inaproc_penarikan`, limit, kolomRiwayat)
	var args []interface{}
	if dataset != "" {
		q += ` WHERE dataset = @p1`
		args = append(args, dataset)
	}
	return m.queryRiwayat(ctx, q+` ORDER BY id DESC`, args...)
}

// TerakhirPerDataset: riwayat terakhir tiap dataset dan penarikan sukses terakhirnya (semua pemicu dan isian).
func (m *PenarikInaproc) TerakhirPerDataset(ctx context.Context) (latest, lastOK map[string]RiwayatPenarikan, err error) {
	pilih := func(where string) (map[string]RiwayatPenarikan, error) {
		q := fmt.Sprintf(`SELECT %s FROM (
			SELECT %s, ROW_NUMBER() OVER (PARTITION BY dataset ORDER BY id DESC) AS rn FROM inaproc_penarikan %s
		) x WHERE rn = 1`, kolomRiwayat, kolomRiwayat, where)
		list, err := m.queryRiwayat(ctx, q)
		if err != nil {
			return nil, err
		}
		out := map[string]RiwayatPenarikan{}
		for _, e := range list {
			out[e.Dataset] = e
		}
		return out, nil
	}
	if latest, err = pilih(""); err != nil {
		return nil, nil, err
	}
	if lastOK, err = pilih("WHERE status = '" + PenarikanSukses + "'"); err != nil {
		return nil, nil, err
	}
	return latest, lastOK, nil
}

// RiwayatOtomatis: untuk tiap (dataset, isian) penarikan otomatis, saat sukses terakhir dan saat percobaan terakhir. Dasar jatuh tempo.
type RiwayatOtomatis struct {
	SuksesTerakhir *time.Time
	CobaTerakhir   time.Time
}

func (m *PenarikInaproc) TerakhirOtomatis(ctx context.Context) (map[string]RiwayatOtomatis, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT dataset, parameter, MAX(CASE WHEN status = @p1 THEN selesai END), MAX(dibuat)
		FROM inaproc_penarikan WHERE pemicu = @p2 GROUP BY dataset, parameter`, PenarikanSukses, PemicuOtomatis)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]RiwayatOtomatis{}
	for rows.Next() {
		var ds, par string
		var ok sql.NullTime
		var coba time.Time
		if err := rows.Scan(&ds, &par, &ok, &coba); err != nil {
			return nil, err
		}
		r := RiwayatOtomatis{CobaTerakhir: coba}
		if ok.Valid {
			t := ok.Time
			r.SuksesTerakhir = &t
		}
		out[ds+"|"+par] = r
	}
	return out, rows.Err()
}

// RingkasTabel: jumlah baris dan waktu sinkronisasi terakhir tiap tabel dataset; tabel yang gagal dibaca diabaikan (nilai nol).
type RingkasanTabel struct {
	Baris       int64
	DisinkronAt *time.Time
}

func (m *PenarikInaproc) RingkasTabel(ctx context.Context) map[string]RingkasanTabel {
	out := map[string]RingkasanTabel{}
	for _, d := range DaftarDataset {
		var n int64
		var t sql.NullTime
		// Nama tabel berasal dari katalog di kode, bukan dari masukan pengguna.
		if err := m.db.QueryRowContext(ctx, "SELECT COUNT_BIG(*), MAX(synced_at) FROM "+d.Tabel).Scan(&n, &t); err != nil {
			log.Println("[INAPROC PENARIKAN WARN] gagal menghitung baris", d.Tabel+":", err)
			continue
		}
		r := RingkasanTabel{Baris: n}
		if t.Valid {
			x := t.Time
			r.DisinkronAt = &x
		}
		out[d.ID] = r
	}
	return out
}

// InitInaprocPenarikan menyiapkan pengelola dan penjadwal penarikan otomatis; dipanggil sekali saat server dimulai, setelah koneksi
// database tersedia.
func InitInaprocPenarikan() {
	terapkanKebijakan()
	// Pembatas permintaan bersama: hitungan kuota satu jam terakhir dimuat dari database (dan disimpan berkala), jadi kuota yang sudah
	// terpakai sebelum server dimulai ulang tetap diperhitungkan.
	aturBatas(nil)
	batas().Mulai(context.Background(), database.DB, 10*time.Second)
	Penarik = NewPenarikInaproc(database.DB)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if n, err := Penarik.RecoverOrphans(ctx); err != nil {
		log.Println("[INAPROC PENARIKAN WARN] gagal membersihkan riwayat yang menggantung:", err)
	} else if n > 0 {
		log.Printf("[INAPROC PENARIKAN] %d riwayat penarikan yang menggantung ditandai gagal (server dimulai ulang)", n)
	}
	Penarik.MulaiPenjadwal(context.Background())
}
