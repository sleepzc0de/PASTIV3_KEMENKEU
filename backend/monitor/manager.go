package monitor

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"pasti-v3-backend/audit"
)

const (
	intervalUkur     = 30 * time.Second
	intervalSimpan   = 5 * time.Minute
	kapasitasRing    = 720 // 6 jam pada interval 30 detik
	usiaCacheSQL     = 60 * time.Second
	usiaCacheTabel   = 10 * time.Minute
	usiaCacheRiwayat = 60 * time.Second
)

// Manager menjalankan pengukur berkala dan merakit ringkasan untuk halaman Monitor Resource.
type Manager struct {
	DBUtama func() *sql.DB // database aplikasi
	DBSLDK  func() *sql.DB // database SLDK (nil bila tidak dikonfigurasi)
	Store   *Store
	// TugasLatar menyebut tugas besar yang sedang berjalan (sinkronisasi, penarikan); diisi pemanggil agar paket ini tidak bergantung pada paket lain.
	TugasLatar  func() []TugasLatar
	RetensiHari int

	cpu   pengukurCPU
	mulai time.Time

	mu             sync.Mutex
	ring           []sampel
	akum           []sampel
	tungguAwal     int64
	terakhirSimp   time.Time
	terakhirBersih time.Time

	infoSQL      *InfoSQL
	infoSQLWaktu time.Time
	tabel        []TabelDB
	tabelWaktu   time.Time
	tabelGalat   string

	riwayat riwayatCache
}

type riwayatCache struct {
	waktu            time.Time
	h24, h7          *Agregat
	lajuDisk, lajuDB *float64
}

// Default: pengelola proses ini (nil sebelum Mulai).
var (
	muDefault sync.RWMutex
	defaultM  *Manager
)

// Pasang menetapkan pengelola proses ini.
func Pasang(m *Manager) {
	muDefault.Lock()
	defaultM = m
	muDefault.Unlock()
}

// Default mengembalikan pengelola proses ini atau nil.
func Default() *Manager {
	muDefault.RLock()
	defer muDefault.RUnlock()
	return defaultM
}

// NewManager membuat pengelola. Mulai harus dipanggil agar pengukuran berkala berjalan.
func NewManager(utama, sldk func() *sql.DB, store *Store, retensiHari int) *Manager {
	return &Manager{DBUtama: utama, DBSLDK: sldk, Store: store, RetensiHari: retensiHari, mulai: time.Now()}
}

func (m *Manager) pool(ctx context.Context, nama string, f func() *sql.DB, ping bool) *PoolDB {
	if f == nil {
		return nil
	}
	return poolDari(ctx, nama, f(), ping)
}

// Mulai menjalankan pengukur berkala sampai ctx dibatalkan.
func (m *Manager) Mulai(ctx context.Context) {
	go func() {
		m.ukurSatu(ctx, time.Now()) // pengukuran pertama hanya mengisi penghitung awal CPU
		if p := m.pool(ctx, "", m.DBUtama, false); p != nil {
			m.mu.Lock()
			m.tungguAwal = p.Menunggu
			m.terakhirSimp = time.Now()
			m.mu.Unlock()
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second): // supaya CPU sudah terbaca beberapa detik setelah aplikasi menyala
		}
		m.ukurSatu(ctx, time.Now())
		tik := time.NewTicker(intervalUkur)
		defer tik.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-tik.C:
				m.ukurSatu(ctx, t)
			}
		}
	}()
}

// ukurSatu mengambil satu sampel, memasukkannya ke ring, dan menyimpan snapshot bila sudah waktunya.
func (m *Manager) ukurSatu(ctx context.Context, sekarang time.Time) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("[MONITOR ERROR] pengukuran gagal:", r)
		}
	}()
	sis := ukurSistem(&m.cpu, sekarang)
	pr := ukurProses()
	d, puncak := Metrik.AmbilJendela()
	s := sampel{Waktu: sekarang, CPU: sis.CPUPersen, CPUProses: sis.CPUProsesPersen, Load1: sis.Load1, MemTotal: sis.MemTotal, MemTerpakai: sis.MemTerpakai,
		SwapTerpakai: sis.SwapTerpakai, DiskTotal: sis.DiskTotal, DiskTerpakai: sis.DiskTerpakai, Heap: pr.HeapDipakai, Goroutine: pr.Goroutine, Req: d, BerjalanMaks: puncak}
	if pr.RSS != nil {
		s.RSS = *pr.RSS
	}
	if p := m.pool(ctx, "", m.DBUtama, false); p != nil {
		s.DBBuka, s.DBDipakai, s.DBBatas, s.DBMenunggu = p.Terbuka, p.Dipakai, p.Batas, p.Menunggu
	}

	m.mu.Lock()
	m.ring = append(m.ring, s)
	if len(m.ring) > kapasitasRing {
		m.ring = m.ring[len(m.ring)-kapasitasRing:]
	}
	m.akum = append(m.akum, s)
	siap := m.Store != nil && sekarang.Sub(m.terakhirSimp) >= intervalSimpan
	var ss []sampel
	var tungguAwal int64
	if siap {
		ss, tungguAwal = m.akum, m.tungguAwal
		m.akum, m.terakhirSimp, m.tungguAwal = nil, sekarang, s.DBMenunggu
	} else if m.Store == nil && len(m.akum) > kapasitasRing {
		m.akum = m.akum[len(m.akum)-kapasitasRing:]
	}
	bersih := m.Store != nil && sekarang.Sub(m.terakhirBersih) >= time.Hour
	if bersih {
		m.terakhirBersih = sekarang
	}
	m.mu.Unlock()

	if siap {
		c, batal := context.WithTimeout(ctx, 20*time.Second)
		defer batal()
		var ukuran, terpakai *float64
		if info := m.ambilInfoSQL(c, 10*time.Minute); info != nil && info.UkuranMB > 0 {
			ukuran = ptr(info.UkuranMB)
			terpakai = info.TerpakaiMB
		}
		if err := m.Store.Simpan(c, bangunSnapshot(ss, sekarang, tungguAwal, ukuran, terpakai)); err != nil {
			log.Println("[MONITOR ERROR] gagal menyimpan snapshot resource:", err)
		}
	}
	if bersih && m.RetensiHari > 0 {
		c, batal := context.WithTimeout(ctx, 30*time.Second)
		defer batal()
		if n, err := m.Store.HapusSebelum(c, sekarang.AddDate(0, 0, -m.RetensiHari)); err != nil {
			log.Println("[MONITOR ERROR] gagal membersihkan riwayat resource lama:", err)
		} else if n > 0 {
			log.Printf("[MONITOR] %d snapshot resource lebih dari %d hari dihapus", n, m.RetensiHari)
		}
	}
}

func (m *Manager) sampelTerakhir() (sampel, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.ring) == 0 {
		return sampel{}, false
	}
	return m.ring[len(m.ring)-1], true
}

// ambilInfoSQL membaca keadaan SQL Server dengan cache selama usia detik.
func (m *Manager) ambilInfoSQL(ctx context.Context, usia time.Duration) *InfoSQL {
	m.mu.Lock()
	if m.infoSQL != nil && time.Since(m.infoSQLWaktu) < usia {
		v := m.infoSQL
		m.mu.Unlock()
		return v
	}
	m.mu.Unlock()
	if m.DBUtama == nil {
		return nil
	}
	info := bacaInfoSQL(ctx, m.DBUtama())
	m.mu.Lock()
	m.infoSQL, m.infoSQLWaktu = info, time.Now()
	m.mu.Unlock()
	return info
}

// TabelTeratas membaca tabel terbesar dengan cache; galat (mis. izin) dikembalikan sebagai teks.
func (m *Manager) TabelTeratas(ctx context.Context) ([]TabelDB, string) {
	m.mu.Lock()
	if !m.tabelWaktu.IsZero() && time.Since(m.tabelWaktu) < usiaCacheTabel {
		t, g := m.tabel, m.tabelGalat
		m.mu.Unlock()
		return t, g
	}
	m.mu.Unlock()
	if m.DBUtama == nil {
		return nil, "database belum tersambung"
	}
	t, err := bacaTabelTeratas(ctx, m.DBUtama(), 15)
	g := ""
	if err != nil {
		g = "Ukuran tabel belum dapat dibaca (butuh izin VIEW DATABASE STATE): " + potongPesan(err.Error(), 160)
		t = nil
	}
	m.mu.Lock()
	m.tabel, m.tabelWaktu, m.tabelGalat = t, time.Now(), g
	m.mu.Unlock()
	return t, g
}

// ringkasRiwayat meringkas snapshot 7 hari menjadi agregat 24 jam dan 7 hari serta laju pertumbuhan disk dan database per hari.
func ringkasRiwayat(ss []Snapshot, sekarang time.Time) (h24, h7 *Agregat, lajuDisk, lajuDB *float64) {
	batas24 := sekarang.Add(-24 * time.Hour)
	var s24 []Snapshot
	var wD, wB []time.Time
	var vD, vB []float64
	for _, s := range ss {
		if !s.Waktu.Before(batas24) {
			s24 = append(s24, s)
		}
		if s.DiskTotal > 0 {
			wD, vD = append(wD, s.Waktu), append(vD, float64(s.DiskTerpakai))
		}
		if s.DBUkuranMB != nil {
			wB, vB = append(wB, s.Waktu), append(vB, *s.DBUkuranMB)
		}
	}
	h24, h7 = AgregatDari(s24, batas24), AgregatDari(ss, sekarang.Add(-7*24*time.Hour))
	if v, ok := LajuPertumbuhanPerHari(wD, vD, 12*time.Hour); ok {
		lajuDisk = ptr(v)
	}
	if v, ok := LajuPertumbuhanPerHari(wB, vB, 12*time.Hour); ok {
		lajuDB = ptr(v)
	}
	return
}

func (m *Manager) agregat(ctx context.Context, sekarang time.Time) (h24, h7 *Agregat, lajuDisk, lajuDB *float64) {
	if m.Store == nil {
		return nil, nil, nil, nil
	}
	m.mu.Lock()
	c := m.riwayat
	m.mu.Unlock()
	if !c.waktu.IsZero() && time.Since(c.waktu) < usiaCacheRiwayat {
		return c.h24, c.h7, c.lajuDisk, c.lajuDB
	}
	ss, err := m.Store.Baca(ctx, sekarang.Add(-7*24*time.Hour), sekarang.Add(time.Minute))
	if err != nil {
		log.Println("[MONITOR ERROR] gagal membaca riwayat resource:", err)
		return c.h24, c.h7, c.lajuDisk, c.lajuDB // pakai cache lama bila ada
	}
	h24, h7, lajuDisk, lajuDB = ringkasRiwayat(ss, sekarang)
	m.mu.Lock()
	m.riwayat = riwayatCache{waktu: time.Now(), h24: h24, h7: h7, lajuDisk: lajuDisk, lajuDB: lajuDB}
	m.mu.Unlock()
	return
}

// Ringkasan merakit seluruh keadaan dan penilaian untuk halaman Monitor Resource.
func (m *Manager) Ringkasan(ctx context.Context) Ringkasan {
	sekarang := time.Now()
	r := Ringkasan{Waktu: sekarang.UTC()}
	r.Sistem = ukurSistem(nil, sekarang)
	if s, ok := m.sampelTerakhir(); ok {
		r.Sistem.CPUPersen, r.Sistem.CPUProsesPersen = s.CPU, s.CPUProses
	}
	r.Proses = ukurProses()
	r.Database.Utama = m.pool(ctx, "Database aplikasi", m.DBUtama, true)
	r.Database.SLDK = m.pool(ctx, "Database SLDK", m.DBSLDK, true)
	if r.Database.Utama != nil && r.Database.Utama.Tersambung {
		r.Database.SQL = m.ambilInfoSQL(ctx, usiaCacheSQL)
	}

	menit5 := Metrik.JendelaMenit("5 menit terakhir", 5, sekarang)
	jam1 := Metrik.JendelaMenit("1 jam terakhir", 60, sekarang)
	r.HTTP = HTTP{Berjalan: Metrik.Berjalan(), Jendela: []JendelaHTTP{menit5, jam1, Metrik.SejakMulai(sekarang)}}
	r.HTTP.RuteLambat, r.HTTP.RuteGalat = Metrik.RuteTeratas(8, 5)

	h24, h7, lajuDisk, lajuDB := m.agregat(ctx, sekarang)
	r.Riwayat24j, r.Riwayat7h = h24, h7
	r.Kapasitas = Evaluasi(Masukan{Sistem: r.Sistem, Proses: r.Proses, DB: r.Database, JamTerakhir: jam1, H24: h24, H7: h7, LajuDiskHari: lajuDisk, LajuDBHari: lajuDB, Audit: audit.StatistikDefault()})
	r.Status, r.PerluDitambah = Gabungkan(r.Kapasitas)

	r.Tugas = []TugasLatar{}
	if m.TugasLatar != nil {
		r.Tugas = m.TugasLatar()
	}
	r.Pengukuran.Aktif = true
	r.Pengukuran.IntervalDtk = int(intervalUkur.Seconds())
	r.Pengukuran.MulaiPada = m.mulai.UTC()
	r.Pengukuran.RetensiHari = m.RetensiHari
	return r
}

// RentangRiwayat: pilihan rentang grafik dan lebar titiknya.
var RentangRiwayat = map[string]struct {
	Durasi     time.Duration
	LebarMenit int
}{
	"1j":  {time.Hour, 0}, // dari ring di memori (resolusi 30 detik)
	"24j": {24 * time.Hour, 5},
	"7h":  {7 * 24 * time.Hour, 30},
	"30h": {30 * 24 * time.Hour, 120},
}

// Riwayat mengembalikan deret waktu untuk grafik. "1j" memakai ring di memori; yang lain membaca monitor_snapshot lalu menurunkan resolusinya.
func (m *Manager) Riwayat(ctx context.Context, rentang string) (*Riwayat, bool) {
	cfg, ok := RentangRiwayat[rentang]
	if !ok {
		return nil, false
	}
	sekarang := time.Now()
	out := &Riwayat{Rentang: rentang, Dari: sekarang.Add(-cfg.Durasi).UTC(), Sampai: sekarang.UTC(), LebarTitik: cfg.LebarMenit, Titik: []TitikRiwayat{}}
	if cfg.LebarMenit == 0 {
		out.LebarTitik = 1
		m.mu.Lock()
		ring := append([]sampel(nil), m.ring...)
		m.mu.Unlock()
		for _, s := range ring {
			if s.Waktu.Before(sekarang.Add(-cfg.Durasi)) {
				continue
			}
			out.Titik = append(out.Titik, titikDariSampel(s))
		}
		return out, true
	}
	if m.Store == nil {
		return out, true
	}
	ss, err := m.Store.Baca(ctx, sekarang.Add(-cfg.Durasi), sekarang.Add(time.Minute))
	if err != nil {
		log.Println("[MONITOR ERROR] gagal membaca riwayat untuk grafik:", err)
		return out, true
	}
	out.Titik = Turunkan(ss, cfg.LebarMenit)
	return out, true
}

func titikDariSampel(s sampel) TitikRiwayat {
	t := TitikRiwayat{Waktu: s.Waktu.UTC(), CPURata: s.CPU, CPUMaks: s.CPU, Load1: s.Load1, Goroutine: ptr(float64(s.Goroutine)), DBDipakai: ptr(float64(s.DBDipakai)),
		ReqTotal: ptr(float64(s.Req.Total)), Req5xx: ptr(float64(s.Req.C5xx))}
	if s.MemTotal > 0 {
		t.MemPersen = ptr(float64(s.MemTerpakai) / float64(s.MemTotal) * 100)
	}
	if s.DiskTotal > 0 {
		t.DiskPersen = ptr(float64(s.DiskTerpakai) / float64(s.DiskTotal) * 100)
	}
	if s.RSS > 0 {
		t.RSS = ptr(float64(s.RSS))
	}
	if s.Heap > 0 {
		t.Heap = ptr(float64(s.Heap))
	}
	if s.Req.NR > 0 {
		t.LatRataMS = ptr(s.Req.SumR / float64(s.Req.NR))
		t.LatP95MS = ptr(persentil(s.Req.HistR, 0.95))
	}
	return t
}
