package digitalisasi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"
)

// Sumber data sinkronisasi: SLDK, dan datamart sebagai cadangan (salinan data SLDK yang dibutuhkan query ini, dengan nama skema, tabel, dan kolom yang sama; lihat
// docs/datamart-sldk.md dan backend/datamart). Query yang sama berjalan tanpa diubah di keduanya.
//
// Aturan peralihan (mode "otomatis"):
//   - SLDK selalu dicoba lebih dulu.
//   - Datamart hanya dipakai bila MEMBACA dari SLDK gagal (SumberError: tidak terhubung, query ditolak atau terputus). Galat data, galat menulis ke PASTI, pembatalan pengguna,
//     dan batas waktu tidak memicu peralihan.
//   - Sebagai cadangan, datamart hanya dipakai bila datamart lebih BARU daripada sinkronisasi sukses terakhir dataset itu di PASTI. Datamart yang lebih tua tidak boleh
//     memundurkan data PASTI yang sudah lebih baru; sinkronisasi dicatat gagal dengan alasannya.
//   - Bila sambungan ke SLDK gagal (bukan sekadar satu query), dataset berikutnya pada antrean yang sama langsung ke datamart tanpa menunggu SLDK lagi.
//   - Mode "sldk" hanya membaca SLDK; mode "datamart" hanya datamart (pilihan sengaja operator, tanpa pemeriksaan "lebih baru").

// Sumber data yang tercatat di digitalisasi_sync_log.sumber.
const (
	SumberSLDK     = "sldk"
	SumberDatamart = "datamart"
)

// Mode sumber (DIGITALISASI_SUMBER).
const (
	ModeOtomatis = "otomatis"
	ModeSLDK     = "sldk"
	ModeDatamart = "datamart"
)

const masaCacheInfoSumber = time.Minute

// NormalisasiMode: nilai selain "sldk" dan "datamart" menjadi "otomatis".
func NormalisasiMode(m string) string {
	switch m {
	case ModeSLDK, ModeDatamart:
		return m
	}
	return ModeOtomatis
}

type sumberBaca struct {
	nama string // SumberSLDK | SumberDatamart
	db   *sql.DB
}

// label: nama untuk pesan kepada pengguna.
func (s sumberBaca) label() string {
	if s.nama == SumberDatamart {
		return "datamart"
	}
	return "SLDK"
}

// PasangDatamart memasang datamart (boleh nil) dan mode sumber. Dipanggil sekali saat server dimulai, sebelum penjadwal.
func (m *Manager) PasangDatamart(db *sql.DB, mode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.datamart = db
	m.mode = NormalisasiMode(mode)
}

func (m *Manager) modeSumber() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return NormalisasiMode(m.mode)
}

// sumberUrut: sumber yang dicoba menurut mode, berurutan. Sumber yang tidak dikonfigurasi (nil) dilewati.
func (m *Manager) sumberUrut() []sumberBaca {
	m.mu.Lock()
	sldk, datamart, mode := m.sldk, m.datamart, NormalisasiMode(m.mode)
	m.mu.Unlock()
	var out []sumberBaca
	if sldk != nil && mode != ModeDatamart {
		out = append(out, sumberBaca{SumberSLDK, sldk})
	}
	if datamart != nil && mode != ModeSLDK {
		out = append(out, sumberBaca{SumberDatamart, datamart})
	}
	return out
}

// AdaSumber: ada sumber yang dapat dipakai sinkronisasi menurut mode dan konfigurasi.
func (m *Manager) AdaSumber() bool { return len(m.sumberUrut()) > 0 }

// ulangDariSLDK: sinkronisasi yang berhasil dari datamart bersifat sementara (datamart bisa tertinggal), jadi penjadwal mencobanya lagi lebih cepat agar kembali ke SLDK begitu
// SLDK pulih. Tidak berlaku bila SLDK memang tidak dipakai (mode datamart atau SLDK tidak dikonfigurasi).
func (m *Manager) ulangDariSLDK() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sldk != nil && NormalisasiMode(m.mode) == ModeOtomatis
}

func waktuWIB(t time.Time) string { return t.In(zonaWIB).Format("02/01/2006 15.04") + " WIB" }

// datamartDisegarkan: awal penyegaran datamart terakhir yang berhasil (UTC), dari jejak yang ditulis script datamart (dbo.datamart_sldk_run). Awal proses dipakai, bukan
// akhirnya, karena itulah saat data SLDK dibaca (konservatif).
func (m *Manager) datamartDisegarkan(ctx context.Context) (time.Time, error) {
	m.mu.Lock()
	db := m.datamart
	m.mu.Unlock()
	if db == nil {
		return time.Time{}, errors.New("datamart tidak dikonfigurasi")
	}
	var t sql.NullTime
	if err := db.QueryRowContext(ctx, `SELECT MAX(mulai) FROM dbo.datamart_sldk_run WHERE status = N'sukses'`).Scan(&t); err != nil {
		return time.Time{}, fmt.Errorf("tidak dapat memeriksa kesegaran datamart: %w", err)
	}
	if !t.Valid {
		return time.Time{}, errors.New("datamart belum pernah berhasil disegarkan")
	}
	return t.Time.UTC(), nil
}

// layakCadangan memutuskan apakah datamart boleh dipakai sebagai cadangan untuk dataset key: harus lebih baru daripada sinkronisasi sukses terakhir dataset itu di PASTI
// (tanpa sinkronisasi sukses sebelumnya, datamart yang mana pun lebih baik daripada tidak ada data). Mengembalikan waktu datamart dan alasan penolakan ("" = layak).
func (m *Manager) layakCadangan(ctx context.Context, key string) (time.Time, string) {
	segar, err := m.datamartDisegarkan(ctx)
	if err != nil {
		return time.Time{}, err.Error()
	}
	var terakhir sql.NullTime
	err = m.pasti.QueryRowContext(ctx,
		`SELECT TOP (1) mulai FROM digitalisasi_sync_log WHERE dataset = @p1 AND status = @p2 AND mulai IS NOT NULL ORDER BY id DESC`,
		key, StatusSukses).Scan(&terakhir)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return segar, "tidak dapat memeriksa sinkronisasi terakhir: " + err.Error()
	}
	// Dibandingkan pada presisi detik: jejak datamart (DATETIME2(0)) tidak lebih halus, jadi penyegaran di detik yang sama dengan sinkronisasi PASTI tidak dianggap lebih lama.
	if terakhir.Valid && segar.Before(terakhir.Time.UTC().Truncate(time.Second)) {
		return segar, fmt.Sprintf("datamart (data per %s) lebih lama daripada data PASTI saat ini (sinkronisasi %s)", waktuWIB(segar), waktuWIB(terakhir.Time))
	}
	return segar, ""
}

// InfoSumber: keadaan sumber data untuk halaman Sinkronisasi.
type InfoSumber struct {
	Mode     string `json:"mode"`     // otomatis | sldk | datamart
	SLDK     bool   `json:"sldk"`     // SLDK dikonfigurasi
	Datamart bool   `json:"datamart"` // datamart dikonfigurasi
	Tersedia bool   `json:"tersedia"` // ada sumber yang dapat dipakai sinkronisasi (menurut mode)
	Cadangan bool   `json:"cadangan"` // datamart dipakai otomatis bila SLDK gagal dibaca
	// DatamartDisegarkan: awal penyegaran datamart terakhir yang berhasil (UTC); nil bila tidak diketahui.
	DatamartDisegarkan *time.Time `json:"datamart_disegarkan"`
	DatamartTerjangkau bool       `json:"datamart_terjangkau"`
}

// InfoSumber membaca kesegaran datamart (bila ada) dengan cache semenit supaya polling halaman tidak membebani datamart.
func (m *Manager) InfoSumber(ctx context.Context) InfoSumber {
	m.mu.Lock()
	info := InfoSumber{Mode: NormalisasiMode(m.mode), SLDK: m.sldk != nil, Datamart: m.datamart != nil}
	m.mu.Unlock()
	info.Tersedia = m.AdaSumber()
	info.Cadangan = info.Mode == ModeOtomatis && info.SLDK && info.Datamart
	if !info.Datamart {
		return info
	}
	m.mu.Lock()
	c := m.cacheSumber
	m.mu.Unlock()
	if time.Since(c.dibaca) < masaCacheInfoSumber && !c.dibaca.IsZero() {
		info.DatamartDisegarkan, info.DatamartTerjangkau = c.disegarkan, c.terjangkau
		return info
	}
	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	t, err := m.datamartDisegarkan(cctx)
	c = cacheInfoSumber{dibaca: time.Now(), terjangkau: err == nil}
	if err == nil {
		c.disegarkan = &t
	}
	m.mu.Lock()
	m.cacheSumber = c
	m.mu.Unlock()
	info.DatamartDisegarkan, info.DatamartTerjangkau = c.disegarkan, c.terjangkau
	return info
}

type cacheInfoSumber struct {
	dibaca     time.Time
	disegarkan *time.Time
	terjangkau bool
}

// hasilBaca: hasil membaca satu dataset lewat sumber yang berhasil.
type hasilBaca struct {
	Result
	Sumber string // SumberSLDK | SumberDatamart ("" bila gagal)
	// Catatan: keterangan tambahan untuk pesan riwayat (mis. dibaca dari datamart beserta kesegarannya); tanpa teks galat mentah (bisa memuat nama server).
	Catatan string
}

// bacaDataset menyinkronkan satu dataset dengan sumber yang berlaku: mencoba tiap sumber menurut urutan dan beralih ke berikutnya hanya pada SumberError dari SLDK. Galat
// yang dikembalikan sudah memuat alasan semua sumber yang dicoba.
func (m *Manager) bacaDataset(ctx context.Context, run *activeRun, ds Dataset, logID int64, progress func(string)) (hasilBaca, error) {
	urut := m.sumberUrut()
	if len(urut) == 0 {
		return hasilBaca{}, ErrNoSLDK
	}
	var galatSLDK error
	for i, s := range urut {
		var segar time.Time
		catatan := ""
		switch {
		case s.nama == SumberSLDK && run.sldkMati:
			galatSLDK = &SumberError{Err: errors.New("koneksi ke SLDK sudah gagal pada antrean ini"), Koneksi: true}
			continue
		case s.nama == SumberDatamart && i > 0:
			// Cadangan: hanya setelah SLDK gagal DIBACA, dan hanya bila datamart tidak memundurkan data.
			var se *SumberError
			if galatSLDK == nil || !errors.As(galatSLDK, &se) || ctx.Err() != nil {
				return hasilBaca{}, galatSLDK
			}
			var alasan string
			if segar, alasan = m.layakCadangan(ctx, ds.Key); alasan != "" {
				return hasilBaca{}, fmt.Errorf("%w; datamart tidak dipakai: %s", galatSLDK, alasan)
			}
			m.setMessage(logID, "SLDK tidak dapat dibaca; beralih ke datamart...")
			catatan = fmt.Sprintf("dari datamart, data per %s; SLDK %s", waktuWIB(segar), ringkasGalatSLDK(galatSLDK))
		case s.nama == SumberDatamart:
			// Dipilih sengaja (mode datamart atau SLDK tidak dikonfigurasi): tanpa pemeriksaan "lebih baru"; kesegarannya tetap dicatat.
			if t, err := m.datamartDisegarkan(ctx); err == nil {
				segar = t
				catatan = fmt.Sprintf("dari datamart, data per %s", waktuWIB(t))
			} else {
				catatan = "dari datamart"
			}
		}

		m.setSumber(logID, s.nama)
		res, err := m.run(ctx, s.db, m.pasti, ds, Options{RunID: logID, Progress: progress, NamaSumber: s.label()})
		if err == nil {
			if s.nama == SumberDatamart && i > 0 {
				log.Printf("[DIGITALISASI WARN] %s dibaca dari datamart karena SLDK gagal: %v", ds.Key, galatSLDK)
			}
			return hasilBaca{Result: res, Sumber: s.nama, Catatan: catatan}, nil
		}
		var se *SumberError
		if s.nama == SumberSLDK && errors.As(err, &se) && i+1 < len(urut) {
			if se.Koneksi {
				run.sldkMati = true
			}
			galatSLDK = err
			continue
		}
		if s.nama == SumberDatamart && i > 0 {
			return hasilBaca{}, fmt.Errorf("SLDK: %v; datamart: %w", galatSLDK, err)
		}
		return hasilBaca{}, err
	}
	return hasilBaca{}, galatSLDK
}

// ringkasGalatSLDK: kalimat pendek tentang kegagalan SLDK tanpa teks galat mentah (yang bisa memuat nama server atau tabel).
func ringkasGalatSLDK(err error) string {
	var se *SumberError
	if errors.As(err, &se) && !se.Koneksi {
		return "gagal dibaca"
	}
	return "tidak dapat dihubungi"
}

// setSumber mencatat sumber yang sedang dibaca pada baris riwayat. Pernyataan terpisah dari pembaruan status supaya urutan dan bentuk pembaruan lainnya tidak berubah.
func (m *Manager) setSumber(id int64, sumber string) {
	ctx, cancel := bgCtx()
	defer cancel()
	if _, err := m.pasti.ExecContext(ctx, `UPDATE digitalisasi_sync_log SET sumber = @p1 WHERE id = @p2`, sumber, id); err != nil {
		log.Println("[DIGITALISASI WARN] gagal mencatat sumber data:", err)
	}
}
