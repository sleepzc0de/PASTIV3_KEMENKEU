package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"pasti-v3-backend/config"
)

// Penarikan otomatis berkala. Penjadwal berjalan di dalam proses server (tanpa cron di luar): ia memeriksa riwayat di
// inaproc_penarikan, sehingga keadaannya bertahan saat server dimulai ulang.
//
// Aturan penarikan teratur:
//   - Yang dijadwalkan adalah tugas (dataset + isian), mis. "Pengumuman Tender untuk K10 tahun 2025": tiap dataset otomatis dikali
//     tahun berjalan dan (JumlahTahun-1) tahun sebelumnya. Dataset per kode (rujukan) tidak ikut: Inaproc tidak punya daftar "semua".
//   - Sebuah tugas jatuh tempo bila penarikan OTOMATIS suksesnya yang terakhir sudah lebih tua dari IntervalHari (bawaan 2 hari)
//     atau belum pernah ada. Penarikan manual tidak menggeser jadwal ini.
//   - Penarikan teratur hanya DIMULAI di dalam jendela jam (bawaan 01.00-05.00 WIB); yang sudah berjalan boleh melewatinya sampai selesai.
//
// Kebijakan gagal tarik (berlaku untuk penarikan manual maupun otomatis):
//   - Tugas yang gagal dicoba ulang otomatis, paling banyak maksPercobaan kali (bawaan 3) dalam satu siklus, dengan jarak minimal
//     jedaAntarPercobaan antar percobaan (jadi ketiganya jatuh di hari yang sama). Percobaan ulang tidak menunggu jendela jam.
//   - Setelah maksPercobaan kali gagal, tugas ISTIRAHAT selama lamaIstirahat (bawaan 8 jam). Setelah itu ia boleh ditarik lagi dan
//     siklus baru dimulai (percobaan dihitung dari awal). Tugas di rencana otomatis ditarik lagi otomatis; tugas manual di luar rencana
//     (mis. per kode) tidak dicoba otomatis lagi setelah istirahat, dan bisa ditarik manual kapan saja.
//   - Yang dihitung percobaan hanya yang berstatus gagal. Dibatalkan, dilewati, dan dihentikan karena server dimulai ulang tidak dihitung.
//     Satu penarikan sukses (manual atau otomatis) menutup siklus.
//
// Kestabilan lain: pemutus beruntun (manager) menghentikan antrean bila banyak tugas gagal berturut-turut dan menahan penjadwal sebentar;
// antrean tunggal dan kunci database mencegah dua penarikan berjalan bersamaan; pembatas permintaan menjaga batas Inaproc.

// zonaWIB: waktu Indonesia Barat tanpa bergantung pada basis data zona waktu di server/container.
var zonaWIB = time.FixedZone("WIB", 7*60*60)

// Variabel (bukan konstanta) hanya supaya tes bisa mempercepatnya atau konfigurasi mengubahnya.
var (
	periodeCek         = 5 * time.Minute  // seberapa sering penjadwal memeriksa
	tundaAwal          = 2 * time.Minute  // penundaan pemeriksaan pertama setelah server mulai
	maksPercobaan      = 3                // percobaan gagal dalam satu siklus sebelum istirahat
	lamaIstirahat      = 8 * time.Hour    // istirahat setelah maksPercobaan kali gagal
	jedaAntarPercobaan = 10 * time.Minute // jarak minimal antar percobaan gagal sebuah tugas
	// Kegagalan lebih tua dari ini tidak dibaca lagi (siklus sudah lama lewat).
	jendelaKegagalan = 48 * time.Hour
)

// terapkanKebijakan membaca kebijakan dari konfigurasi (nilai di luar rentang diganti bawaan).
func terapkanKebijakan() {
	if c := config.Cfg; c != nil {
		if c.InaprocMaksPercobaan >= 1 && c.InaprocMaksPercobaan <= 10 {
			maksPercobaan = c.InaprocMaksPercobaan
		}
		if c.InaprocIstirahatJam >= 1 && c.InaprocIstirahatJam <= 72 {
			lamaIstirahat = time.Duration(c.InaprocIstirahatJam) * time.Hour
		}
	}
}

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

// ---- kebijakan gagal tarik ----

// KondisiTugas: keadaan siklus percobaan sebuah tugas.
type KondisiTugas struct {
	Terpakai        int       // percobaan gagal pada siklus berjalan (0..maksPercobaan)
	Istirahat       bool      // sedang istirahat setelah maksPercobaan kali gagal
	IstirahatSampai time.Time // akhir istirahat (bermakna bila Istirahat)
	GagalTerakhir   time.Time
	BelumPulih      bool // ada kegagalan sejak penarikan sukses terakhir
}

// Kondisi menghitung siklus percobaan dari waktu-waktu kegagalan (urut naik, semuanya sesudah penarikan sukses terakhir). Tiap maksPercobaan
// kegagalan beruntun menutup satu siklus dan memulai istirahat lamaIstirahat; kegagalan sesudah istirahat memulai siklus baru.
func Kondisi(gagal []time.Time, now time.Time) KondisiTugas {
	k := KondisiTugas{BelumPulih: len(gagal) > 0}
	if len(gagal) == 0 {
		return k
	}
	k.GagalTerakhir = gagal[len(gagal)-1]
	n := 0
	var istirahat time.Time
	for _, t := range gagal {
		if !istirahat.IsZero() && t.Before(istirahat) {
			continue // gagal yang tercatat selama istirahat (mis. penarikan manual) tidak membuka siklus baru
		}
		n++
		if n >= maksPercobaan {
			istirahat = t.Add(lamaIstirahat)
			n = 0
		}
	}
	if istirahat.After(now) {
		k.Istirahat, k.IstirahatSampai, k.Terpakai = true, istirahat, maksPercobaan
		return k
	}
	k.Terpakai = n
	return k
}

// SaatCobaLagi: kapan percobaan otomatis berikutnya paling cepat boleh dimulai (nol = sekarang).
func (k KondisiTugas) SaatCobaLagi() time.Time {
	switch {
	case k.Istirahat:
		return k.IstirahatSampai
	case k.Terpakai > 0:
		return k.GagalTerakhir.Add(jedaAntarPercobaan)
	}
	return time.Time{}
}

// KegagalanTugas: kegagalan sebuah tugas sejak penarikan sukses terakhir (dari riwayat).
type KegagalanTugas struct {
	Dataset    string
	Parameter  string
	Waktu      []time.Time // urut naik
	Permintaan string      // isian tugas (JSON); kosong untuk baris lama
	Pesan      string      // pesan kegagalan terakhir
}

func (k *KegagalanTugas) kunci() string { return k.Dataset + "|" + k.Parameter }

// KegagalanBelumPulih membaca tugas yang gagal sejak penarikan suksesnya yang terakhir (manual atau otomatis), dalam jendelaKegagalan.
func (m *PenarikInaproc) KegagalanBelumPulih(ctx context.Context, now time.Time) (map[string]*KegagalanTugas, error) {
	rows, err := m.db.QueryContext(ctx, `
		WITH r AS (
			SELECT id, dataset, parameter, status, COALESCE(selesai, dibuat) AS waktu, permintaan, pesan,
				MAX(CASE WHEN status = @p2 THEN id END) OVER (PARTITION BY dataset, parameter) AS id_sukses
			FROM inaproc_penarikan WHERE dibuat >= @p1 AND status IN (@p2, @p3))
		SELECT dataset, parameter, waktu, permintaan, pesan FROM r
		WHERE status = @p3 AND (id_sukses IS NULL OR id > id_sukses) ORDER BY dataset, parameter, id`,
		now.Add(-jendelaKegagalan).UTC(), PenarikanSukses, PenarikanGagal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]*KegagalanTugas{}
	for rows.Next() {
		var ds, par string
		var waktu time.Time
		var perm, pesan sql.NullString
		if err := rows.Scan(&ds, &par, &waktu, &perm, &pesan); err != nil {
			return nil, err
		}
		k := &KegagalanTugas{Dataset: ds, Parameter: par}
		if lama, ada := out[k.kunci()]; ada {
			k = lama
		} else {
			out[k.kunci()] = k
		}
		k.Waktu = append(k.Waktu, waktu)
		if perm.Valid && perm.String != "" {
			k.Permintaan = perm.String
		}
		k.Pesan = pesan.String
	}
	return out, rows.Err()
}

// tugasDariKegagalan menyusun ulang tugas dari isian yang tersimpan; false bila tidak bisa (baris lama, dataset tidak ada, isian tidak sah).
func tugasDariKegagalan(k *KegagalanTugas) (Tugas, bool) {
	d, ok := DatasetByID(k.Dataset)
	if !ok || k.Permintaan == "" {
		return Tugas{}, false
	}
	var perm PermintaanTarik
	if err := json.Unmarshal([]byte(k.Permintaan), &perm); err != nil {
		return Tugas{}, false
	}
	perm = d.Normalisasi(perm)
	if d.Validasi(perm) != "" {
		return Tugas{}, false
	}
	t := Tugas{Dataset: d, Perm: perm}
	if t.Kunci() != k.kunci() {
		return Tugas{}, false
	}
	return t, true
}

// saatJatuhTempo: kapan tugas ini paling cepat boleh ditarik menurut interval (tanpa memperhitungkan jendela jam dan kegagalan).
func (p Pengaturan) saatJatuhTempo(now time.Time, t Tugas, riwayat map[string]RiwayatOtomatis) time.Time {
	saat := now
	if r, ada := riwayat[t.Kunci()]; ada && r.SuksesTerakhir != nil {
		saat = r.SuksesTerakhir.Add(time.Duration(p.IntervalHari) * 24 * time.Hour)
	}
	return saat
}

// TugasTerjadwal memisahkan tugas yang harus ditarik sekarang menjadi:
//   - lanjutan: percobaan ulang (siklus berjalan, jarak antar percobaan sudah lewat) dan tugas yang istirahatnya sudah selesai. Tidak
//     menunggu jendela jam dan tidak menunggu interval: data yang gagal harus segera pulih.
//   - reguler: tugas yang jatuh tempo menurut interval; hanya boleh dimulai di dalam jendela jam.
//
// Tugas yang sedang istirahat atau masih dalam jarak antar percobaan tidak masuk keduanya.
func (p Pengaturan) TugasTerjadwal(now time.Time, riwayat map[string]RiwayatOtomatis, gagal map[string]*KegagalanTugas) (lanjutan, reguler []Tugas) {
	dalamRencana := map[string]bool{}
	for _, t := range p.RencanaOtomatis(now) {
		k := t.Kunci()
		dalamRencana[k] = true
		if g := gagal[k]; g != nil && len(g.Waktu) > 0 {
			kond := Kondisi(g.Waktu, now)
			if now.Before(kond.SaatCobaLagi()) {
				continue
			}
			t.Percobaan = kond.Terpakai + 1
			lanjutan = append(lanjutan, t)
			continue
		}
		if !p.saatJatuhTempo(now, t, riwayat).After(now) {
			reguler = append(reguler, t)
		}
	}

	// Tugas di luar rencana (penarikan manual yang gagal): hanya dilanjutkan selama siklus percobaannya berjalan.
	var luar []string
	for k := range gagal {
		if !dalamRencana[k] {
			luar = append(luar, k)
		}
	}
	sort.Strings(luar)
	for _, k := range luar {
		g := gagal[k]
		kond := Kondisi(g.Waktu, now)
		if kond.Terpakai == 0 || kond.Istirahat || now.Before(kond.SaatCobaLagi()) {
			continue
		}
		if t, ok := tugasDariKegagalan(g); ok {
			t.Percobaan = kond.Terpakai + 1
			lanjutan = append(lanjutan, t)
		}
	}
	return lanjutan, reguler
}

// JatuhTempo: semua tugas otomatis yang harus ditarik sekarang tanpa memperhitungkan jendela jam (lanjutan dan reguler).
func (p Pengaturan) JatuhTempo(now time.Time, riwayat map[string]RiwayatOtomatis, gagal map[string]*KegagalanTugas) []Tugas {
	l, r := p.TugasTerjadwal(now, riwayat, gagal)
	return append(l, r...)
}

// Berikutnya: perkiraan kapan penarikan otomatis berikutnya dimulai: yang paling awal dari percobaan ulang/akhir istirahat (tanpa jendela
// jam) dan jatuh tempo reguler (diselaraskan ke jendela jam). nil bila otomatis tidak aktif atau tidak ada tugas.
func (p Pengaturan) Berikutnya(now time.Time, riwayat map[string]RiwayatOtomatis, gagal map[string]*KegagalanTugas) *time.Time {
	tugas := p.RencanaOtomatis(now)
	if !p.Aktif || len(tugas) == 0 {
		return nil
	}
	var awal time.Time
	ambil := func(t time.Time) {
		if awal.IsZero() || t.Before(awal) {
			awal = t
		}
	}
	for _, t := range tugas {
		if g := gagal[t.Kunci()]; g != nil && len(g.Waktu) > 0 {
			ambil(Kondisi(g.Waktu, now).SaatCobaLagi())
			continue
		}
		ambil(p.selaraskan(maxWaktu(p.saatJatuhTempo(now, t, riwayat), now)))
	}
	if awal.Before(now) {
		awal = now
	}
	return &awal
}

func maxWaktu(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// TugasBermasalah: tugas yang gagal dan belum pulih, untuk ditampilkan di halaman.
type TugasBermasalah struct {
	Dataset           string     `json:"dataset"`
	Nama              string     `json:"nama"`
	Parameter         string     `json:"parameter"`
	Gagal             int        `json:"gagal"` // percobaan gagal pada siklus berjalan
	MaksPercobaan     int        `json:"maks_percobaan"`
	Istirahat         bool       `json:"istirahat"`
	BerikutnyaSekitar *time.Time `json:"berikutnya_sekitar"` // percobaan otomatis berikutnya (atau akhir istirahat); nil bila tidak ada
	TerakhirGagal     time.Time  `json:"terakhir_gagal"`
	Pesan             string     `json:"pesan"`
	DalamRencana      bool       `json:"dalam_rencana"` // ikut penarikan otomatis; yang di luar rencana tidak ditarik otomatis lagi setelah istirahat
}

// Bermasalah menyusun daftar tugas yang sedang gagal beserta keadaan siklus percobaannya, urut menurut katalog lalu isian.
func (p Pengaturan) Bermasalah(now time.Time, gagal map[string]*KegagalanTugas) []TugasBermasalah {
	dalamRencana := map[string]bool{}
	for _, t := range p.RencanaOtomatis(now) {
		dalamRencana[t.Kunci()] = true
	}
	urutan := map[string]int{}
	for i, d := range DaftarDataset {
		urutan[d.ID] = i
	}
	var out []TugasBermasalah
	for k, g := range gagal {
		if len(g.Waktu) == 0 {
			continue
		}
		d, ok := DatasetByID(g.Dataset)
		if !ok {
			continue
		}
		kond := Kondisi(g.Waktu, now)
		x := TugasBermasalah{Dataset: g.Dataset, Nama: d.Nama, Parameter: g.Parameter, Gagal: kond.Terpakai, MaksPercobaan: maksPercobaan,
			Istirahat: kond.Istirahat, TerakhirGagal: kond.GagalTerakhir, Pesan: g.Pesan, DalamRencana: dalamRencana[k]}
		if p.Aktif && (dalamRencana[k] || (kond.Terpakai > 0 && !kond.Istirahat)) {
			t := maxWaktu(kond.SaatCobaLagi(), now)
			if !dalamRencana[k] && kond.Istirahat {
				x.BerikutnyaSekitar = nil
			} else {
				x.BerikutnyaSekitar = &t
			}
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool {
		if urutan[out[i].Dataset] != urutan[out[j].Dataset] {
			return urutan[out[i].Dataset] < urutan[out[j].Dataset]
		}
		return out[i].Parameter < out[j].Parameter
	})
	return out
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
	JatuhTempo       int        `json:"jatuh_tempo"`       // tugas yang jatuh tempo sekarang (percobaan ulang dan reguler)
	Istirahat        int        `json:"istirahat"`         // tugas yang sedang istirahat setelah gagal berulang
	TerakhirOtomatis *time.Time `json:"terakhir_otomatis"` // penarikan otomatis sukses terakhir (mana pun)
	DitahanSampai    *time.Time `json:"ditahan_sampai"`    // penarikan otomatis ditahan sementara (gangguan Inaproc/jaringan)
	MaksPercobaan    int        `json:"maks_percobaan"`
	IstirahatJam     float64    `json:"istirahat_jam"`
	Zona             string     `json:"zona"`
}

// InfoOtomatis dihitung dari pengaturan, riwayat otomatis, dan kegagalan yang belum pulih.
func (m *PenarikInaproc) InfoOtomatis(ctx context.Context, p Pengaturan, now time.Time, gagal map[string]*KegagalanTugas) (InfoOtomatis, error) {
	riwayat, err := m.TerakhirOtomatis(ctx)
	if err != nil {
		return InfoOtomatis{}, err
	}
	tokenAda := config.Cfg != nil && config.Cfg.InaprocToken != ""
	info := InfoOtomatis{Aktif: p.Aktif && tokenAda, TokenAda: tokenAda, Zona: "WIB", MaksPercobaan: maksPercobaan, IstirahatJam: lamaIstirahat.Hours()}
	info.JumlahTugas = len(p.RencanaOtomatis(now))
	info.JatuhTempo = len(p.JatuhTempo(now, riwayat, gagal))
	for _, g := range gagal {
		if Kondisi(g.Waktu, now).Istirahat {
			info.Istirahat++
		}
	}
	for _, r := range riwayat {
		if r.SuksesTerakhir != nil && (info.TerakhirOtomatis == nil || r.SuksesTerakhir.After(*info.TerakhirOtomatis)) {
			t := *r.SuksesTerakhir
			info.TerakhirOtomatis = &t
		}
	}
	if t := m.otomatisDitahanSampai(); t.After(now) {
		info.DitahanSampai = &t
	}
	if info.Aktif {
		b := p.Berikutnya(now, riwayat, gagal)
		if b != nil && info.DitahanSampai != nil && b.Before(*info.DitahanSampai) {
			t := *info.DitahanSampai
			b = &t
		}
		info.Berikutnya = b
	}
	return info, nil
}

// MulaiPenjadwal menjalankan penjadwal di latar belakang sampai ctx berakhir. Penjadwal selalu berjalan; ia sendiri yang
// memeriksa pengaturan (aktif atau tidak) dan token tiap putaran.
func (m *PenarikInaproc) MulaiPenjadwal(ctx context.Context) {
	p := PengaturanBawaan()
	log.Printf("[INAPROC PENARIKAN] penjadwal dimulai (bawaan server: aktif=%v, tiap %d hari, mulai pukul %02d.00-%02d.00 WIB; gagal: maks %d percobaan lalu istirahat %s)",
		p.Aktif, p.IntervalHari, p.JamMulai, p.JamAkhir, maksPercobaan, lamaIstirahat)
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
	if config.Cfg == nil || config.Cfg.InaprocToken == "" || m.Aktif() != nil || now.Before(m.otomatisDitahanSampai()) {
		return
	}
	qctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	p, err := m.MuatPengaturan(qctx)
	if err != nil {
		log.Println("[INAPROC PENARIKAN WARN] penjadwal gagal membaca pengaturan:", err)
		return
	}
	if !p.Aktif {
		return
	}
	riwayat, err := m.TerakhirOtomatis(qctx)
	if err != nil {
		log.Println("[INAPROC PENARIKAN WARN] penjadwal gagal membaca riwayat:", err)
		return
	}
	gagal, err := m.KegagalanBelumPulih(qctx, now)
	if err != nil {
		log.Println("[INAPROC PENARIKAN WARN] penjadwal gagal membaca kegagalan:", err)
		return
	}
	lanjutan, reguler := p.TugasTerjadwal(now, riwayat, gagal)
	tugas := lanjutan
	if p.DalamJendela(now) {
		tugas = append(tugas, reguler...)
	}
	if len(tugas) == 0 {
		return
	}
	info, err := m.Start(tugas, PemicuOtomatis, Oleh{Nama: namaPengirimOto}, time.Duration(p.JedaDetik)*time.Second)
	switch {
	case err == nil:
		log.Printf("[INAPROC PENARIKAN] penarikan otomatis dimulai: %d tugas (%d percobaan ulang/lanjutan, %d reguler)", info.Total, len(lanjutan), len(tugas)-len(lanjutan))
	case errors.Is(err, ErrPenarikanSibuk):
		// Penarikan lain sedang berjalan; putaran berikutnya memeriksa lagi.
	default:
		log.Println("[INAPROC PENARIKAN ERROR] penarikan otomatis gagal dimulai:", err)
	}
}
