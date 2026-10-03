package digitalisasi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// Status riwayat sinkronisasi (kolom digitalisasi_sync_log.status).
const (
	StatusAntri      = "antri"
	StatusBerjalan   = "berjalan"
	StatusSukses     = "sukses"
	StatusGagal      = "gagal"
	StatusDibatalkan = "dibatalkan"
)

var (
	ErrBusy           = errors.New("sinkronisasi lain sedang berjalan")
	ErrNoSLDK         = errors.New("koneksi ke SLDK tidak tersedia")
	ErrUnknownDataset = errors.New("dataset tidak dikenal")
	ErrNothingToRun   = errors.New("tidak ada dataset yang dipilih")
)

// DefaultTimeout: batas waktu per dataset (query SLDK ke tabel aset berukuran ratusan GB bisa lama).
const DefaultTimeout = 2 * time.Hour

// syncFunc dipisah supaya pengelola antrean bisa diuji tanpa SLDK.
type syncFunc func(ctx context.Context, sldkDB, pastiDB *sql.DB, ds Dataset, opt Options) (Result, error)

// RunInfo: sinkronisasi yang sedang berjalan (satu antrean berisi satu atau lebih dataset).
type RunInfo struct {
	Datasets []string  `json:"datasets"`
	Saat     string    `json:"saat_ini"`
	Mulai    time.Time `json:"mulai"`
	Oleh     string    `json:"oleh"`
}

type activeRun struct {
	info   RunInfo
	logIDs map[string]int64
	cancel context.CancelFunc
}

// Manager menjalankan sinkronisasi di latar belakang (di luar permintaan HTTP) dan menjaga agar hanya satu
// antrean berjalan pada satu waktu, karena tiap query membebani server SLDK.
type Manager struct {
	pasti, sldk *sql.DB
	Timeout     time.Duration
	run         syncFunc

	mu     sync.Mutex
	active *activeRun
}

func NewManager(pasti, sldk *sql.DB) *Manager {
	return &Manager{pasti: pasti, sldk: sldk, Timeout: DefaultTimeout, run: Sync}
}

// Default dipakai handler HTTP; diisi oleh Init saat server dimulai.
var Default *Manager

func Init(pasti, sldk *sql.DB) {
	Default = NewManager(pasti, sldk)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if n, err := Default.RecoverOrphans(ctx); err != nil {
		log.Println("[DIGITALISASI WARN] gagal membersihkan riwayat yang menggantung:", err)
	} else if n > 0 {
		log.Printf("[DIGITALISASI] %d riwayat sinkronisasi yang menggantung ditandai gagal (server dimulai ulang)", n)
	}
}

// RecoverOrphans: sinkronisasi berjalan di dalam proses server, jadi saat server (re)start tidak mungkin ada
// yang benar-benar berjalan. Baris yang masih "antri"/"berjalan" adalah sisa proses sebelumnya.
func (m *Manager) RecoverOrphans(ctx context.Context) (int64, error) {
	res, err := m.pasti.ExecContext(ctx,
		`UPDATE digitalisasi_sync_log SET status = @p1, selesai = SYSUTCDATETIME(), pesan = @p2 WHERE status IN (@p3, @p4)`,
		StatusGagal, "Dihentikan karena server dimulai ulang", StatusAntri, StatusBerjalan)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Active: salinan keadaan antrean yang sedang berjalan, atau nil.
func (m *Manager) Active() *RunInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active == nil {
		return nil
	}
	cp := m.active.info
	cp.Datasets = append([]string(nil), cp.Datasets...)
	return &cp
}

// Cancel menghentikan antrean yang sedang berjalan (query SLDK yang sedang dibaca ikut dibatalkan).
func (m *Manager) Cancel() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active == nil {
		return false
	}
	m.active.cancel()
	return true
}

// Start memasukkan dataset terpilih ke antrean dan langsung kembali; pekerjaannya berjalan di goroutine.
// Urutan antrean selalu mengikuti urutan Datasets, apa pun urutan masukan.
func (m *Manager) Start(keys []string, user string) (RunInfo, error) {
	if m.sldk == nil {
		return RunInfo{}, ErrNoSLDK
	}
	want := map[string]bool{}
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if _, ok := ByKey(k); !ok {
			return RunInfo{}, fmt.Errorf("%w: %q", ErrUnknownDataset, k)
		}
		want[k] = true
	}
	var queue []Dataset
	for _, ds := range Datasets {
		if want[ds.Key] {
			queue = append(queue, ds)
		}
	}
	if len(queue) == 0 {
		return RunInfo{}, ErrNothingToRun
	}

	m.mu.Lock()
	if m.active != nil {
		m.mu.Unlock()
		return RunInfo{}, ErrBusy
	}
	ctx, cancel := context.WithCancel(context.Background())
	run := &activeRun{
		info:   RunInfo{Mulai: time.Now().UTC(), Oleh: user},
		logIDs: map[string]int64{},
		cancel: cancel,
	}
	// Dicatat sebelum kunci dilepas supaya permintaan kedua yang datang bersamaan ditolak.
	m.active = run
	m.mu.Unlock()

	for _, ds := range queue {
		id, err := m.enqueue(ds.Key, user)
		if err != nil {
			cancel()
			m.finishRun(run)
			// Baris yang sudah terlanjur dibuat dibatalkan agar tidak menggantung.
			for key, logID := range run.logIDs {
				m.finishLog(logID, StatusDibatalkan, nil, "Antrean gagal dibuat ("+key+")")
			}
			return RunInfo{}, fmt.Errorf("gagal mencatat antrean: %w", err)
		}
		run.logIDs[ds.Key] = id
		m.mu.Lock()
		run.info.Datasets = append(run.info.Datasets, ds.Key)
		m.mu.Unlock()
	}

	go m.execute(ctx, run, queue)
	return m.snapshot(run), nil
}

func (m *Manager) snapshot(run *activeRun) RunInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := run.info
	cp.Datasets = append([]string(nil), cp.Datasets...)
	return cp
}

func (m *Manager) finishRun(run *activeRun) {
	m.mu.Lock()
	if m.active == run {
		m.active = nil
	}
	m.mu.Unlock()
}

func (m *Manager) setCurrent(run *activeRun, key string) {
	m.mu.Lock()
	run.info.Saat = key
	m.mu.Unlock()
}

func (m *Manager) execute(ctx context.Context, run *activeRun, queue []Dataset) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[DIGITALISASI ERROR] panic saat sinkronisasi: %v", r)
			for _, ds := range queue {
				m.finishLog(run.logIDs[ds.Key], StatusGagal, nil, "Terjadi kesalahan internal")
			}
		}
		run.cancel()
		m.finishRun(run)
	}()

	for i, ds := range queue {
		logID := run.logIDs[ds.Key]
		if ctx.Err() != nil {
			for _, rest := range queue[i:] {
				m.finishLog(run.logIDs[rest.Key], StatusDibatalkan, nil, "Dibatalkan sebelum dimulai")
			}
			return
		}
		m.setCurrent(run, ds.Key)
		m.markRunning(logID)

		dctx, dcancel := context.WithTimeout(ctx, m.Timeout)
		lastWrite := time.Time{}
		res, err := m.run(dctx, m.sldk, m.pasti, ds, Options{
			RunID: logID,
			Progress: func(msg string) {
				// Pesan kemajuan ditulis paling cepat tiap 3 detik.
				if time.Since(lastWrite) < 3*time.Second {
					return
				}
				lastWrite = time.Now()
				m.setMessage(logID, msg)
			},
		})
		timedOut := errors.Is(dctx.Err(), context.DeadlineExceeded)
		dcancel()

		switch {
		case err == nil:
			rows, coords := int64(res.Rows), int64(res.WithCoords)
			m.finishLog(logID, StatusSukses, &logCounts{Rows: rows, Coords: coords}, res.Summary())
			log.Printf("[DIGITALISASI] %s: %s", ds.Key, res.Summary())
		case ctx.Err() != nil:
			m.finishLog(logID, StatusDibatalkan, nil, "Dibatalkan oleh pengguna")
			log.Printf("[DIGITALISASI] %s dibatalkan", ds.Key)
		case timedOut:
			m.finishLog(logID, StatusGagal, nil, fmt.Sprintf("Melebihi batas waktu %s", m.Timeout))
			log.Printf("[DIGITALISASI ERROR] %s melebihi batas waktu", ds.Key)
		default:
			m.finishLog(logID, StatusGagal, nil, err.Error())
			log.Printf("[DIGITALISASI ERROR] %s: %v", ds.Key, err)
		}
	}
}

// ---- riwayat (digitalisasi_sync_log) ----

type logCounts struct{ Rows, Coords int64 }

func bgCtx() (context.Context, context.CancelFunc) {
	// Pencatatan hasil harus tetap berhasil walau konteks sinkronisasi sudah dibatalkan.
	return context.WithTimeout(context.Background(), 30*time.Second)
}

func clip(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

func (m *Manager) enqueue(dataset, user string) (int64, error) {
	ctx, cancel := bgCtx()
	defer cancel()
	var id int64
	err := m.pasti.QueryRowContext(ctx,
		`INSERT INTO digitalisasi_sync_log (dataset, status, dijalankan_oleh) OUTPUT INSERTED.id VALUES (@p1, @p2, @p3)`,
		dataset, StatusAntri, clip(user, 100)).Scan(&id)
	return id, err
}

func (m *Manager) markRunning(id int64) {
	ctx, cancel := bgCtx()
	defer cancel()
	if _, err := m.pasti.ExecContext(ctx,
		`UPDATE digitalisasi_sync_log SET status = @p1, mulai = SYSUTCDATETIME(), pesan = @p2 WHERE id = @p3`,
		StatusBerjalan, "Membaca dari SLDK...", id); err != nil {
		log.Println("[DIGITALISASI WARN] gagal mencatat status berjalan:", err)
	}
}

func (m *Manager) setMessage(id int64, msg string) {
	ctx, cancel := bgCtx()
	defer cancel()
	if _, err := m.pasti.ExecContext(ctx, `UPDATE digitalisasi_sync_log SET pesan = @p1 WHERE id = @p2 AND status = @p3`,
		clip(msg, 1000), id, StatusBerjalan); err != nil {
		log.Println("[DIGITALISASI WARN] gagal mencatat kemajuan:", err)
	}
}

func (m *Manager) finishLog(id int64, status string, counts *logCounts, msg string) {
	ctx, cancel := bgCtx()
	defer cancel()
	var rows, coords interface{}
	if counts != nil {
		rows, coords = counts.Rows, counts.Coords
	}
	if _, err := m.pasti.ExecContext(ctx,
		`UPDATE digitalisasi_sync_log SET status = @p1, selesai = SYSUTCDATETIME(), jumlah_baris = @p2, jumlah_koordinat = @p3, pesan = @p4 WHERE id = @p5`,
		status, rows, coords, clip(msg, 1000), id); err != nil {
		log.Println("[DIGITALISASI WARN] gagal mencatat hasil sinkronisasi:", err)
	}
}

// LogEntry: satu baris riwayat.
type LogEntry struct {
	ID              int64      `json:"id"`
	Dataset         string     `json:"dataset"`
	Status          string     `json:"status"`
	Dibuat          time.Time  `json:"dibuat"`
	Mulai           *time.Time `json:"mulai"`
	Selesai         *time.Time `json:"selesai"`
	JumlahBaris     *int64     `json:"jumlah_baris"`
	JumlahKoordinat *int64     `json:"jumlah_koordinat"`
	Pesan           *string    `json:"pesan"`
	DijalankanOleh  *string    `json:"dijalankan_oleh"`
}

const logColumns = `id, dataset, status, dibuat, mulai, selesai, jumlah_baris, jumlah_koordinat, pesan, dijalankan_oleh`

func scanLog(rows *sql.Rows) (LogEntry, error) {
	var e LogEntry
	var mulai, selesai sql.NullTime
	var baris, koord sql.NullInt64
	var pesan, oleh sql.NullString
	if err := rows.Scan(&e.ID, &e.Dataset, &e.Status, &e.Dibuat, &mulai, &selesai, &baris, &koord, &pesan, &oleh); err != nil {
		return e, err
	}
	if mulai.Valid {
		e.Mulai = &mulai.Time
	}
	if selesai.Valid {
		e.Selesai = &selesai.Time
	}
	if baris.Valid {
		e.JumlahBaris = &baris.Int64
	}
	if koord.Valid {
		e.JumlahKoordinat = &koord.Int64
	}
	if pesan.Valid {
		e.Pesan = &pesan.String
	}
	if oleh.Valid {
		e.DijalankanOleh = &oleh.String
	}
	return e, nil
}

func queryLogs(ctx context.Context, db *sql.DB, q string, args ...interface{}) ([]LogEntry, error) {
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LogEntry{}
	for rows.Next() {
		e, err := scanLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// History: riwayat terbaru (semua dataset), terbaru dulu.
func (m *Manager) History(ctx context.Context, limit int) ([]LogEntry, error) {
	if limit < 1 || limit > 200 {
		limit = 30
	}
	return queryLogs(ctx, m.pasti, fmt.Sprintf(`SELECT TOP (%d) %s FROM digitalisasi_sync_log ORDER BY id DESC`, limit, logColumns))
}

// LatestPerDataset: riwayat terakhir tiap dataset, dan sinkronisasi sukses terakhirnya.
func (m *Manager) LatestPerDataset(ctx context.Context) (latest, lastOK map[string]LogEntry, err error) {
	pick := func(where string) (map[string]LogEntry, error) {
		q := fmt.Sprintf(`SELECT %s FROM (
			SELECT %s, ROW_NUMBER() OVER (PARTITION BY dataset ORDER BY id DESC) AS rn
			FROM digitalisasi_sync_log %s
		) x WHERE rn = 1`, logColumns, logColumns, where)
		list, err := queryLogs(ctx, m.pasti, q)
		if err != nil {
			return nil, err
		}
		out := map[string]LogEntry{}
		for _, e := range list {
			out[e.Dataset] = e
		}
		return out, nil
	}
	if latest, err = pick(""); err != nil {
		return nil, nil, err
	}
	if lastOK, err = pick("WHERE status = '" + StatusSukses + "'"); err != nil {
		return nil, nil, err
	}
	return latest, lastOK, nil
}
