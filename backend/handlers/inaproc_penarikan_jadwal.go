package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"pasti-v3-backend/config"
)

// Penarikan otomatis berkala. Penjadwal berjalan di dalam proses server (tanpa cron di luar): ia memeriksa riwayat di
// inaproc_penarikan, sehingga keadaannya bertahan saat server dimulai ulang.
//
// Aturan:
//   - Yang dijadwalkan adalah tugas (dataset + isian), mis. "Pengumuman Tender untuk K10 tahun 2025": tiap dataset otomatis dikali
//     tahun berjalan dan (JumlahTahun-1) tahun sebelumnya. Dataset per kode (rujukan) tidak ikut: Inaproc tidak punya daftar "semua".
//   - Sebuah tugas jatuh tempo bila penarikan OTOMATIS suksesnya yang terakhir sudah lebih tua dari IntervalHari (bawaan 2 hari)
//     atau belum pernah ada, dan percobaan otomatis terakhirnya (apa pun hasilnya) sudah lebih lama dari jedaCoba. Penarikan manual
//     tidak menggeser jadwal ini. Jeda itu mencegah percobaan berulang saat Inaproc menolak (mis. 429).
//   - Penarikan hanya DIMULAI di dalam jendela jam (bawaan 01.00-05.00 WIB); yang sudah berjalan boleh melewatinya sampai selesai.
//   - Tidak pernah berjalan bersamaan dengan penarikan lain (satu antrean pada satu waktu).
//   - Pengaturan dibaca dari database tiap putaran, jadi perubahan dari halaman Penarikan Data berlaku tanpa memulai ulang server.

// zonaWIB: waktu Indonesia Barat tanpa bergantung pada basis data zona waktu di server/container.
var zonaWIB = time.FixedZone("WIB", 7*60*60)

// Variabel (bukan konstanta) hanya supaya tes bisa mempercepatnya.
var (
	periodeCek = 15 * time.Minute // seberapa sering penjadwal memeriksa
	tundaAwal  = 2 * time.Minute  // penundaan pemeriksaan pertama setelah server mulai
	jedaCoba   = 6 * time.Hour    // jarak minimal antar percobaan otomatis sebuah tugas
)

// Pengaturan: pengaturan penarikan otomatis.
type Pengaturan struct {
	Aktif        bool       `json:"aktif"`
	IntervalHari int        `json:"interval_hari"`
	JamMulai     int        `json:"jam_mulai"` // jam (WIB, 0-23) paling awal penarikan otomatis boleh dimulai
	JamAkhir     int        `json:"jam_akhir"` // batas akhir; dimulai pada [JamMulai, JamAkhir). Sama = sepanjang hari
	KodeKLPD     string     `json:"kode_klpd"`
	JumlahTahun  int        `json:"jumlah_tahun"` // tahun berjalan dan (JumlahTahun-1) tahun sebelumnya
	JedaDetik    int        `json:"jeda_detik"`   // jeda antar tugas, supaya tidak menabrak batas laju Inaproc
	Dataset      []string   `json:"dataset"`      // ID dataset yang ikut
	Diubah       *time.Time `json:"diubah"`
	DiubahOleh   string     `json:"diubah_oleh"`
	BawaanServer bool       `json:"bawaan_server"` // true = belum pernah disimpan dari halaman; memakai konfigurasi server
}

// IDDatasetOtomatis: ID semua dataset yang mampu ikut penarikan otomatis, menurut urutan katalog.
func IDDatasetOtomatis() []string {
	var ids []string
	for _, d := range DaftarDataset {
		if d.Otomatis {
			ids = append(ids, d.ID)
		}
	}
	return ids
}

func jepit(n, min, max, bawaan int) int {
	if n < min || n > max {
		return bawaan
	}
	return n
}

// PengaturanBawaan dari konfigurasi server (INAPROC_AUTO_*); nilai di luar rentang diganti bawaannya.
func PengaturanBawaan() Pengaturan {
	p := Pengaturan{Aktif: true, IntervalHari: 2, JamMulai: 1, JamAkhir: 5, KodeKLPD: kemenkeuKLPDCode, JumlahTahun: 2, JedaDetik: int(jedaAntarTugasBawaan / time.Second),
		Dataset: IDDatasetOtomatis(), BawaanServer: true}
	if c := config.Cfg; c != nil {
		p.Aktif = c.InaprocAutoSync
		p.IntervalHari = jepit(c.InaprocAutoIntervalHari, 1, 30, 2)
		p.JumlahTahun = jepit(c.InaprocAutoJumlahTahun, 1, 5, 2)
		if c.InaprocAutoJamMulai >= 0 && c.InaprocAutoJamMulai <= 23 && c.InaprocAutoJamAkhir >= 0 && c.InaprocAutoJamAkhir <= 23 {
			p.JamMulai, p.JamAkhir = c.InaprocAutoJamMulai, c.InaprocAutoJamAkhir
		}
	}
	return p
}

// Rapikan memangkas isian dan mengurutkan dataset menurut katalog (tanpa duplikat); pesan tidak kosong = ditolak.
func (p *Pengaturan) Rapikan() string {
	p.KodeKLPD = strings.TrimSpace(p.KodeKLPD)
	switch {
	case p.IntervalHari < 1 || p.IntervalHari > 30:
		return "Interval harus antara 1 dan 30 hari"
	case p.JamMulai < 0 || p.JamMulai > 23 || p.JamAkhir < 0 || p.JamAkhir > 23:
		return "Jam harus antara 0 dan 23"
	case p.KodeKLPD == "" || len(p.KodeKLPD) > 20:
		return "Kode KLPD wajib diisi (maksimal 20 karakter)"
	case p.JumlahTahun < 1 || p.JumlahTahun > 5:
		return "Jumlah tahun harus antara 1 dan 5"
	case p.JedaDetik < 0 || p.JedaDetik > 60:
		return "Jeda antar tugas harus antara 0 dan 60 detik"
	}
	pilih := map[string]bool{}
	for _, id := range p.Dataset {
		d, ok := DatasetByID(id)
		if !ok {
			return "Dataset tidak dikenal: " + potong(id, 60)
		}
		if !d.Otomatis {
			return "Dataset ini tidak bisa ditarik otomatis (hanya per kode): " + d.Nama
		}
		pilih[d.ID] = true
	}
	p.Dataset = nil
	for _, d := range DaftarDataset {
		if pilih[d.ID] {
			p.Dataset = append(p.Dataset, d.ID)
		}
	}
	return ""
}

// DalamJendela: apakah t berada di jendela jam yang memperbolehkan penarikan otomatis dimulai (jendela boleh melewati tengah malam).
func (p Pengaturan) DalamJendela(t time.Time) bool {
	h := t.In(zonaWIB).Hour()
	switch {
	case p.JamMulai == p.JamAkhir:
		return true
	case p.JamMulai < p.JamAkhir:
		return h >= p.JamMulai && h < p.JamAkhir
	default: // melewati tengah malam, mis. 22-04
		return h >= p.JamMulai || h < p.JamAkhir
	}
}

// selaraskan: saat paling awal >= t yang berada di jendela jam.
func (p Pengaturan) selaraskan(t time.Time) time.Time {
	if p.DalamJendela(t) {
		return t
	}
	w := t.In(zonaWIB)
	mulai := time.Date(w.Year(), w.Month(), w.Day(), p.JamMulai, 0, 0, 0, zonaWIB)
	if !mulai.After(t) {
		mulai = mulai.AddDate(0, 0, 1)
	}
	return mulai
}

// TahunPenarikan: tahun berjalan (WIB) dan (jumlah-1) tahun sebelumnya, terbaru dulu.
func TahunPenarikan(now time.Time, jumlah int) []string {
	th := now.In(zonaWIB).Year()
	var out []string
	for i := 0; i < jumlah; i++ {
		out = append(out, strconv.Itoa(th-i))
	}
	return out
}

// RencanaOtomatis: semua tugas penarikan otomatis menurut pengaturan, menurut urutan katalog (tahun terbaru dulu).
func (p Pengaturan) RencanaOtomatis(now time.Time) []Tugas {
	var tugas []Tugas
	tahun := TahunPenarikan(now, p.JumlahTahun)
	for _, id := range p.Dataset {
		d, ok := DatasetByID(id)
		if !ok || !d.Otomatis {
			continue
		}
		tambah := func(perm PermintaanTarik) {
			tugas = append(tugas, Tugas{Dataset: d, Perm: d.Normalisasi(perm)})
		}
		switch d.Mode {
		case ModeKLPDTahun, ModeTransaksi:
			for _, th := range tahun {
				tambah(PermintaanTarik{KodeKLPD: p.KodeKLPD, Tahun: th})
			}
		case ModeKLPD:
			tambah(PermintaanTarik{KodeKLPD: p.KodeKLPD})
		case ModeKategori:
			tambah(PermintaanTarik{}) // tingkat 1
		}
	}
	return tugas
}

// saatJatuhTempo: kapan tugas ini paling cepat boleh ditarik otomatis (tanpa memperhitungkan jendela jam).
func (p Pengaturan) saatJatuhTempo(now time.Time, t Tugas, riwayat map[string]RiwayatOtomatis) time.Time {
	saat := now
	r, ada := riwayat[t.Kunci()]
	if !ada {
		return saat
	}
	if r.SuksesTerakhir != nil {
		saat = r.SuksesTerakhir.Add(time.Duration(p.IntervalHari) * 24 * time.Hour)
	}
	if tunggu := r.CobaTerakhir.Add(jedaCoba); tunggu.After(saat) {
		saat = tunggu
	}
	return saat
}

// JatuhTempo: tugas otomatis yang harus ditarik sekarang.
func (p Pengaturan) JatuhTempo(now time.Time, riwayat map[string]RiwayatOtomatis) []Tugas {
	var out []Tugas
	for _, t := range p.RencanaOtomatis(now) {
		if !p.saatJatuhTempo(now, t, riwayat).After(now) {
			out = append(out, t)
		}
	}
	return out
}

// Berikutnya: perkiraan kapan penarikan otomatis berikutnya dimulai (jatuh tempo paling awal, diselaraskan ke jendela jam).
// nil bila otomatis tidak aktif atau tidak ada tugas.
func (p Pengaturan) Berikutnya(now time.Time, riwayat map[string]RiwayatOtomatis) *time.Time {
	tugas := p.RencanaOtomatis(now)
	if !p.Aktif || len(tugas) == 0 {
		return nil
	}
	awal := p.saatJatuhTempo(now, tugas[0], riwayat)
	for _, t := range tugas[1:] {
		if s := p.saatJatuhTempo(now, t, riwayat); s.Before(awal) {
			awal = s
		}
	}
	if awal.Before(now) {
		awal = now
	}
	awal = p.selaraskan(awal)
	return &awal
}

// MuatPengaturan membaca pengaturan tersimpan; belum ada = bawaan dari konfigurasi server.
func (m *PenarikInaproc) MuatPengaturan(ctx context.Context) (Pengaturan, error) {
	var p Pengaturan
	var dataset sql.NullString
	var diubah sql.NullTime
	var oleh sql.NullString
	err := m.db.QueryRowContext(ctx, `
		SELECT aktif, interval_hari, jam_mulai, jam_akhir, kode_klpd, jumlah_tahun, jeda_detik, dataset, diubah, diubah_oleh
		FROM inaproc_penarikan_pengaturan WHERE id = 1`).
		Scan(&p.Aktif, &p.IntervalHari, &p.JamMulai, &p.JamAkhir, &p.KodeKLPD, &p.JumlahTahun, &p.JedaDetik, &dataset, &diubah, &oleh)
	if errors.Is(err, sql.ErrNoRows) {
		return PengaturanBawaan(), nil
	}
	if err != nil {
		return Pengaturan{}, err
	}
	if dataset.Valid && dataset.String != "" {
		if err := json.Unmarshal([]byte(dataset.String), &p.Dataset); err != nil {
			return Pengaturan{}, err
		}
	}
	// Dataset yang sudah tidak ada di katalog dibuang diam-diam (mis. setelah pembaruan aplikasi).
	var sah []string
	for _, id := range p.Dataset {
		if d, ok := DatasetByID(id); ok && d.Otomatis {
			sah = append(sah, id)
		}
	}
	p.Dataset = sah
	if diubah.Valid {
		t := diubah.Time
		p.Diubah = &t
	}
	p.DiubahOleh = oleh.String
	return p, nil
}

// SimpanPengaturan menyimpan pengaturan yang sudah lolos Rapikan.
func (m *PenarikInaproc) SimpanPengaturan(ctx context.Context, p Pengaturan, oleh string) error {
	ds, err := json.Marshal(p.Dataset)
	if err != nil {
		return err
	}
	_, err = m.db.ExecContext(ctx, `
		UPDATE inaproc_penarikan_pengaturan SET aktif = @p1, interval_hari = @p2, jam_mulai = @p3, jam_akhir = @p4, kode_klpd = @p5,
			jumlah_tahun = @p6, jeda_detik = @p7, dataset = @p8, diubah = SYSUTCDATETIME(), diubah_oleh = @p9 WHERE id = 1;
		IF @@ROWCOUNT = 0
			INSERT INTO inaproc_penarikan_pengaturan (id, aktif, interval_hari, jam_mulai, jam_akhir, kode_klpd, jumlah_tahun, jeda_detik, dataset, diubah_oleh)
			VALUES (1, @p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9);`,
		p.Aktif, p.IntervalHari, p.JamMulai, p.JamAkhir, p.KodeKLPD, p.JumlahTahun, p.JedaDetik, string(ds), nullIfEmpty(potong(oleh, 100)))
	return err
}

// InfoOtomatis: keadaan penarikan otomatis untuk halaman Penarikan Data.
type InfoOtomatis struct {
	Aktif            bool       `json:"aktif"` // pengaturan aktif DAN token Inaproc tersedia
	TokenAda         bool       `json:"token_ada"`
	Berikutnya       *time.Time `json:"berikutnya"`
	JumlahTugas      int        `json:"jumlah_tugas"`      // tugas dalam satu putaran penuh
	JatuhTempo       int        `json:"jatuh_tempo"`       // tugas yang jatuh tempo sekarang
	TerakhirOtomatis *time.Time `json:"terakhir_otomatis"` // penarikan otomatis sukses terakhir (mana pun)
	Zona             string     `json:"zona"`
}

// InfoOtomatis dihitung dari pengaturan dan riwayat otomatis.
func (m *PenarikInaproc) InfoOtomatis(ctx context.Context, p Pengaturan, now time.Time) (InfoOtomatis, error) {
	riwayat, err := m.TerakhirOtomatis(ctx)
	if err != nil {
		return InfoOtomatis{}, err
	}
	tokenAda := config.Cfg != nil && config.Cfg.InaprocToken != ""
	info := InfoOtomatis{Aktif: p.Aktif && tokenAda, TokenAda: tokenAda, Zona: "WIB"}
	info.JumlahTugas = len(p.RencanaOtomatis(now))
	info.JatuhTempo = len(p.JatuhTempo(now, riwayat))
	for _, r := range riwayat {
		if r.SuksesTerakhir != nil && (info.TerakhirOtomatis == nil || r.SuksesTerakhir.After(*info.TerakhirOtomatis)) {
			t := *r.SuksesTerakhir
			info.TerakhirOtomatis = &t
		}
	}
	if info.Aktif {
		info.Berikutnya = p.Berikutnya(now, riwayat)
	}
	return info, nil
}

// MulaiPenjadwal menjalankan penjadwal di latar belakang sampai ctx berakhir. Penjadwal selalu berjalan; ia sendiri yang
// memeriksa pengaturan (aktif atau tidak) dan token tiap putaran.
func (m *PenarikInaproc) MulaiPenjadwal(ctx context.Context) {
	p := PengaturanBawaan()
	log.Printf("[INAPROC PENARIKAN] penjadwal dimulai (bawaan server: aktif=%v, tiap %d hari, mulai pukul %02d.00-%02d.00 WIB)",
		p.Aktif, p.IntervalHari, p.JamMulai, p.JamAkhir)
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
func (m *PenarikInaproc) cekOtomatis(ctx context.Context, now time.Time) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[INAPROC PENARIKAN ERROR] panic pada penjadwal: %v", r)
		}
	}()
	if config.Cfg == nil || config.Cfg.InaprocToken == "" || m.Aktif() != nil {
		return
	}
	qctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	p, err := m.MuatPengaturan(qctx)
	if err != nil {
		log.Println("[INAPROC PENARIKAN WARN] penjadwal gagal membaca pengaturan:", err)
		return
	}
	if !p.Aktif || !p.DalamJendela(now) {
		return
	}
	riwayat, err := m.TerakhirOtomatis(qctx)
	if err != nil {
		log.Println("[INAPROC PENARIKAN WARN] penjadwal gagal membaca riwayat:", err)
		return
	}
	tugas := p.JatuhTempo(now, riwayat)
	if len(tugas) == 0 {
		return
	}
	info, err := m.Start(tugas, PemicuOtomatis, Oleh{Nama: namaPengirimOto}, time.Duration(p.JedaDetik)*time.Second)
	switch {
	case err == nil:
		log.Printf("[INAPROC PENARIKAN] penarikan otomatis dimulai: %d tugas", info.Total)
	case errors.Is(err, ErrPenarikanSibuk):
		// Penarikan manual baru saja dimulai; putaran berikutnya memeriksa lagi.
	default:
		log.Println("[INAPROC PENARIKAN ERROR] penarikan otomatis gagal dimulai:", err)
	}
}
