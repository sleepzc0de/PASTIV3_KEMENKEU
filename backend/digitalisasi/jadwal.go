package digitalisasi

import (
	"context"
	"errors"
	"log"
	"time"
)

// Sinkronisasi otomatis berkala. Penjadwal berjalan di dalam proses server (tanpa cron di luar): ia memeriksa riwayat di
// digitalisasi_sync_log, sehingga keadaannya bertahan saat server dimulai ulang dan sinkronisasi manual ikut dihitung.
//
// Aturan:
//   - Sebuah dataset jatuh tempo bila sinkronisasi suksesnya yang terakhir sudah lebih tua dari Interval (bawaan 7 hari) atau belum
//     pernah ada, dan percobaan terakhirnya (apa pun hasilnya) sudah lebih lama dari jedaCoba. Jeda itu mencegah percobaan
//     berulang ke SLDK saat sinkronisasi gagal: percobaan berikutnya baru pada malam berikutnya.
//   - Sinkronisasi hanya DIMULAI di dalam jendela jam (bawaan 01.00-05.00 WIB) supaya query berat ke SLDK tidak berjalan di jam kerja.
//     Yang sudah berjalan boleh melewati jendela itu sampai selesai (tanpa batas waktu).
//   - Tidak pernah berjalan bersamaan dengan sinkronisasi lain (Manager hanya mengizinkan satu antrean).

// zonaWIB: waktu Indonesia Barat tanpa bergantung pada basis data zona waktu di server/container.
var zonaWIB = time.FixedZone("WIB", 7*60*60)

const (
	// Bawaan penjadwal.
	BawaanIntervalHari = 7
	BawaanJamMulai     = 1
	BawaanJamAkhir     = 5

	// PengirimOtomatis dicatat di kolom dijalankan_oleh untuk sinkronisasi yang dimulai penjadwal.
	PengirimOtomatis = "otomatis (mingguan)"

	jedaCoba = 20 * time.Hour // jarak minimal antar percobaan sebuah dataset
)

// Variabel (bukan konstanta) hanya supaya tes bisa mempercepatnya.
var (
	periodeCek = 15 * time.Minute // seberapa sering penjadwal memeriksa
	tundaAwal  = 2 * time.Minute  // penundaan pemeriksaan pertama setelah server mulai
)

// Jadwal: pengaturan sinkronisasi otomatis.
type Jadwal struct {
	Aktif    bool
	Interval time.Duration
	JamMulai int // jam (WIB, 0-23) paling awal sinkronisasi otomatis boleh dimulai
	JamAkhir int // jam (WIB, 0-23) batas akhir; sinkronisasi dimulai pada [JamMulai, JamAkhir). Sama = sepanjang hari
}

// NewJadwal membuat Jadwal dari nilai konfigurasi; nilai di luar rentang diganti bawaannya.
func NewJadwal(aktif bool, intervalHari, jamMulai, jamAkhir int) Jadwal {
	if intervalHari < 1 || intervalHari > 365 {
		intervalHari = BawaanIntervalHari
	}
	if jamMulai < 0 || jamMulai > 23 || jamAkhir < 0 || jamAkhir > 23 {
		jamMulai, jamAkhir = BawaanJamMulai, BawaanJamAkhir
	}
	return Jadwal{Aktif: aktif, Interval: time.Duration(intervalHari) * 24 * time.Hour, JamMulai: jamMulai, JamAkhir: jamAkhir}
}

// DalamJendela: apakah t berada di jendela jam yang memperbolehkan sinkronisasi otomatis dimulai (jendela boleh melewati tengah malam).
func (j Jadwal) DalamJendela(t time.Time) bool {
	h := t.In(zonaWIB).Hour()
	switch {
	case j.JamMulai == j.JamAkhir:
		return true
	case j.JamMulai < j.JamAkhir:
		return h >= j.JamMulai && h < j.JamAkhir
	default: // melewati tengah malam, mis. 22-04
		return h >= j.JamMulai || h < j.JamAkhir
	}
}

// saatJatuhTempo: kapan dataset ini paling cepat boleh disinkronkan otomatis (tanpa memperhitungkan jendela jam).
func (j Jadwal) saatJatuhTempo(now time.Time, key string, latest, lastOK map[string]LogEntry) time.Time {
	t := now
	if ok, has := lastOK[key]; has && ok.Selesai != nil {
		t = ok.Selesai.Add(j.Interval)
	}
	if l, has := latest[key]; has {
		if tunggu := l.Dibuat.Add(jedaCoba); tunggu.After(t) {
			t = tunggu
		}
	}
	return t
}

// JatuhTempo: kunci dataset yang harus disinkronkan sekarang, menurut urutan Datasets.
func (j Jadwal) JatuhTempo(now time.Time, latest, lastOK map[string]LogEntry) []string {
	var keys []string
	for _, ds := range Datasets {
		if !j.saatJatuhTempo(now, ds.Key, latest, lastOK).After(now) {
			keys = append(keys, ds.Key)
		}
	}
	return keys
}

// Berikutnya: perkiraan kapan sinkronisasi otomatis berikutnya dimulai (jatuh tempo paling awal, diselaraskan ke jendela jam).
// nil bila otomatis tidak aktif.
func (j Jadwal) Berikutnya(now time.Time, latest, lastOK map[string]LogEntry) *time.Time {
	if !j.Aktif || len(Datasets) == 0 {
		return nil
	}
	awal := j.saatJatuhTempo(now, Datasets[0].Key, latest, lastOK)
	for _, ds := range Datasets[1:] {
		if t := j.saatJatuhTempo(now, ds.Key, latest, lastOK); t.Before(awal) {
			awal = t
		}
	}
	if awal.Before(now) {
		awal = now
	}
	if !j.DalamJendela(awal) {
		w := awal.In(zonaWIB)
		mulai := time.Date(w.Year(), w.Month(), w.Day(), j.JamMulai, 0, 0, 0, zonaWIB)
		if !mulai.After(awal) {
			mulai = mulai.AddDate(0, 0, 1)
		}
		awal = mulai
	}
	return &awal
}

// InfoOtomatis: keadaan sinkronisasi otomatis untuk ditampilkan di halaman Sinkronisasi.
type InfoOtomatis struct {
	Aktif        bool       `json:"aktif"`
	IntervalHari int        `json:"interval_hari"`
	JamMulai     int        `json:"jam_mulai"`
	JamAkhir     int        `json:"jam_akhir"`
	Zona         string     `json:"zona"`
	Berikutnya   *time.Time `json:"berikutnya"`
}

// Jadwal: pengaturan sinkronisasi otomatis yang berlaku.
func (m *Manager) Jadwal() Jadwal {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.jadwal
}

// InfoOtomatis dihitung dari riwayat yang sudah dibaca pemanggil (tanpa query tambahan). Tanpa koneksi SLDK otomatis tidak berjalan.
func (m *Manager) InfoOtomatis(now time.Time, latest, lastOK map[string]LogEntry) InfoOtomatis {
	j := m.Jadwal()
	aktif := j.Aktif && m.sldk != nil
	info := InfoOtomatis{Aktif: aktif, IntervalHari: int(j.Interval / (24 * time.Hour)), JamMulai: j.JamMulai, JamAkhir: j.JamAkhir, Zona: "WIB"}
	if aktif {
		info.Berikutnya = j.Berikutnya(now, latest, lastOK)
	}
	return info
}

// MulaiPenjadwal menetapkan Jadwal dan, bila aktif dan SLDK tersedia, menjalankan penjadwal di latar belakang sampai ctx berakhir.
func (m *Manager) MulaiPenjadwal(ctx context.Context, j Jadwal) {
	m.mu.Lock()
	m.jadwal = j
	m.mu.Unlock()
	switch {
	case !j.Aktif:
		log.Println("[DIGITALISASI] sinkronisasi otomatis dinonaktifkan (DIGITALISASI_AUTO_SYNC)")
		return
	case m.sldk == nil:
		log.Println("[DIGITALISASI] sinkronisasi otomatis tidak berjalan: koneksi SLDK tidak tersedia")
		return
	}
	log.Printf("[DIGITALISASI] sinkronisasi otomatis aktif: tiap %d hari, dimulai pukul %02d.00-%02d.00 WIB",
		int(j.Interval/(24*time.Hour)), j.JamMulai, j.JamAkhir)
	go func() {
		timer := time.NewTimer(tundaAwal)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
			m.cekOtomatis(ctx, time.Now())
			timer.Reset(periodeCek)
		}
	}()
}

// cekOtomatis: satu putaran penjadwal. Galat dicatat dan tidak menghentikan penjadwal.
func (m *Manager) cekOtomatis(ctx context.Context, now time.Time) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[DIGITALISASI ERROR] panic pada penjadwal: %v", r)
		}
	}()
	j := m.Jadwal()
	if !j.Aktif || m.sldk == nil || !j.DalamJendela(now) || m.Active() != nil {
		return
	}
	qctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	latest, lastOK, err := m.LatestPerDataset(qctx)
	if err != nil {
		log.Println("[DIGITALISASI WARN] penjadwal gagal membaca riwayat:", err)
		return
	}
	keys := j.JatuhTempo(now, latest, lastOK)
	if len(keys) == 0 {
		return
	}
	info, err := m.Start(keys, PengirimOtomatis)
	switch {
	case err == nil:
		log.Printf("[DIGITALISASI] sinkronisasi otomatis dimulai: %v", info.Datasets)
	case errors.Is(err, ErrBusy):
		// Sinkronisasi manual baru saja dimulai; putaran berikutnya memeriksa lagi.
	default:
		log.Println("[DIGITALISASI ERROR] sinkronisasi otomatis gagal dimulai:", err)
	}
}
