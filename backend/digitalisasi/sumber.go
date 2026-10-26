package digitalisasi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"
)

// Sumber data sinkronisasi: SATU, yaitu datamart (tabel DIGITALISASI_* hasil script backend/datamart/buat_datamart_digitalisasi.sql; lihat docs/datamart-digitalisasi.md).
// Aplikasi tidak pernah mengakses SLDK untuk dataset ini; datamart-lah yang membaca SLDK, di luar aplikasi, dijadwalkan administrator database.

// SumberDatamart dicatat di digitalisasi_sync_log.sumber untuk setiap sinkronisasi. Riwayat lama (sebelum aplikasi hanya memakai datamart) bernilai NULL atau "sldk".
const SumberDatamart = "datamart"

const masaCacheInfoSumber = time.Minute

func waktuWIB(t time.Time) string { return t.In(zonaWIB).Format("02/01/2006 15.04") + " WIB" }

// AdaSumber: datamart dikonfigurasi (DATAMART_DB_HOST terisi), jadi sinkronisasi dapat dijalankan.
func (m *Manager) AdaSumber() bool { return m.datamart != nil }

// InfoSumber: keadaan datamart untuk halaman Sinkronisasi.
type InfoSumber struct {
	Tersedia   bool `json:"tersedia"`   // datamart dikonfigurasi
	Terjangkau bool `json:"terjangkau"` // jejak penyegarannya dapat dibaca
	// DataPer: awal penyegaran datamart terakhir yang berhasil (UTC); nil bila belum pernah atau tidak terbaca.
	DataPer *time.Time `json:"data_per"`
	// PenyegaranTerakhir: status proses penyegaran datamart terbaru ("sukses" | "gagal" | "berjalan"; "" bila belum ada). "gagal" artinya data yang dipakai berasal dari penyegaran
	// sukses sebelumnya.
	PenyegaranTerakhir string `json:"penyegaran_terakhir"`
}

type cacheInfoSumber struct {
	dibaca time.Time
	info   InfoSumber
}

// bacaInfoDatamart membaca jejak penyegaran datamart (dbo.datamart_digitalisasi_run).
func (m *Manager) bacaInfoDatamart(ctx context.Context) (InfoSumber, error) {
	info := InfoSumber{Tersedia: m.datamart != nil}
	if m.datamart == nil {
		return info, errors.New("datamart tidak dikonfigurasi")
	}
	var sukses sql.NullTime
	var status sql.NullString
	err := m.datamart.QueryRowContext(ctx, `SELECT
		(SELECT MAX(mulai) FROM dbo.datamart_digitalisasi_run WHERE status = N'sukses'),
		(SELECT TOP (1) status FROM dbo.datamart_digitalisasi_run ORDER BY id DESC)`).Scan(&sukses, &status)
	if err != nil {
		return info, fmt.Errorf("tidak dapat memeriksa kesegaran datamart: %w", err)
	}
	info.Terjangkau = true
	if sukses.Valid {
		t := sukses.Time.UTC()
		info.DataPer = &t
	}
	if status.Valid {
		info.PenyegaranTerakhir = status.String
	}
	return info, nil
}

// InfoSumber membaca kesegaran datamart (bila ada) dengan cache semenit supaya polling halaman tidak membebani datamart.
func (m *Manager) InfoSumber(ctx context.Context) InfoSumber {
	if m.datamart == nil {
		return InfoSumber{}
	}
	m.mu.Lock()
	c := m.cacheSumber
	m.mu.Unlock()
	if !c.dibaca.IsZero() && time.Since(c.dibaca) < masaCacheInfoSumber {
		return c.info
	}
	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	info, err := m.bacaInfoDatamart(cctx)
	if err != nil {
		log.Println("[DIGITALISASI WARN]", err)
	}
	m.mu.Lock()
	m.cacheSumber = cacheInfoSumber{dibaca: time.Now(), info: info}
	m.mu.Unlock()
	return info
}

// setSumber mencatat sumber yang dibaca pada baris riwayat. Pernyataan terpisah dari pembaruan status supaya urutan dan bentuk pembaruan lainnya tidak berubah.
func (m *Manager) setSumber(id int64, sumber string) {
	ctx, cancel := bgCtx()
	defer cancel()
	if _, err := m.pasti.ExecContext(ctx, `UPDATE digitalisasi_sync_log SET sumber = @p1 WHERE id = @p2`, sumber, id); err != nil {
		log.Println("[DIGITALISASI WARN] gagal mencatat sumber data:", err)
	}
}

// dataPerTerakhir: awal penyegaran datamart (UTC) dari sinkronisasi sukses terakhir dataset key; nil bila belum pernah atau tidak tercatat (riwayat sebelum kolom data_per ada).
func (m *Manager) dataPerTerakhir(ctx context.Context, key string) (*time.Time, error) {
	var t sql.NullTime
	err := m.pasti.QueryRowContext(ctx,
		`SELECT TOP (1) data_per FROM digitalisasi_sync_log WHERE dataset = @p1 AND status = @p2 AND data_per IS NOT NULL ORDER BY id DESC`,
		key, StatusSukses).Scan(&t)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("tidak dapat memeriksa sinkronisasi terakhir: %w", err)
	}
	if !t.Valid {
		return nil, nil
	}
	u := t.Time.UTC()
	return &u, nil
}

// sinkronDataset menyalin satu dataset dari datamart ke PASTI. Galat dari sini adalah alasan sinkronisasi gagal; tidak ada sumber lain yang dicoba.
func (m *Manager) sinkronDataset(ctx context.Context, ds Dataset, logID int64, progress func(string)) (Result, error) {
	if m.datamart == nil {
		return Result{}, ErrNoSumber
	}
	sebelumnya, err := m.dataPerTerakhir(ctx, ds.Key)
	if err != nil {
		return Result{}, err
	}
	m.setSumber(logID, SumberDatamart)
	return m.run(ctx, m.datamart, m.pasti, ds, Options{RunID: logID, Progress: progress, Sebelumnya: sebelumnya})
}
