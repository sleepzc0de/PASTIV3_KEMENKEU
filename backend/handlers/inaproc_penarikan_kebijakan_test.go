package handlers

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"


	"pasti-v3-backend/config"
	"pasti-v3-backend/internal/fakesql"
)

// Tes kebijakan penarikan yang stabil: percobaan ulang 3 kali sehari lalu istirahat 8 jam (dibaca dari riwayat), tanpa tabrakan antar
// penarikan (antrean tunggal, kunci database), dan pemesanan kuota sebelum tugas dimulai.

// penarikUjiDB: pengelola dengan database palsu yang jawabannya bisa diatur per jenis query lewat peta potongan-query -> jawaban.
type jawabanUji struct {
	kolom []string
	baris [][]driver.Value
}

func penarikDenganJawaban(t *testing.T, stub pelaksanaTugas, jawab map[string]jawabanUji) (*PenarikInaproc, *fakesql.DB) {
	t.Helper()
	m, f := penarikUji(t, stub)
	dasar := f.OnQuery
	f.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		for potongan, j := range jawab {
			if strings.Contains(q, potongan) {
				return j.kolom, j.baris, nil
			}
		}
		return dasar(ctx, q, args)
	}
	return m, f
}

func barisGagal(dataset, parameter, perm, pesan string, waktu ...time.Time) jawabanUji {
	var baris [][]driver.Value
	for _, w := range waktu {
		baris = append(baris, []driver.Value{dataset, parameter, w, perm, pesan})
	}
	return jawabanUji{kolom: []string{"dataset", "parameter", "waktu", "permintaan", "pesan"}, baris: baris}
}

func TestKegagalanBelumPulihMembacaRiwayatPerTugas(t *testing.T) {
	now := wib(2026, 10, 4, 12)
	m, f := penarikDenganJawaban(t, sukses(0), map[string]jawabanUji{"WITH r AS": {
		kolom: []string{"dataset", "parameter", "waktu", "permintaan", "pesan"},
		baris: [][]driver.Value{
			{"tender/pengumuman", "K10/2026", now.Add(-time.Hour), `{"kode_klpd":"K10","tahun":"2026"}`, "timeout"},
			{"tender/pengumuman", "K10/2026", now.Add(-20 * time.Minute), nil, "502"},
			{"ekatalog/penyedia-detail", "01ABC", now.Add(-30 * time.Minute), `{"kode":"01ABC"}`, nil},
		}}})
	got, err := m.KegagalanBelumPulih(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	a, b := got["tender/pengumuman|K10/2026"], got["ekatalog/penyedia-detail|01ABC"]
	if len(got) != 2 || a == nil || b == nil || len(a.Waktu) != 2 || a.Pesan != "502" || a.Permintaan == "" || len(b.Waktu) != 1 || b.Pesan != "" {
		t.Fatalf("hasil = %+v / %+v", a, b)
	}
	// Hanya kegagalan sesudah sukses terakhir yang dibaca, dalam jendela 48 jam; sukses (manual atau otomatis) menutup siklus.
	q, ada := kueriPertama(f, "WITH r AS")
	if !ada || !strings.Contains(q.Query, "id_sukses IS NULL OR id > id_sukses") || !strings.Contains(q.Query, "status IN (@p2, @p3)") {
		t.Fatalf("query = %s", q.Query)
	}
	if q.Args[0].Value.(time.Time).Before(now.Add(-49*time.Hour).UTC()) || q.Args[1].Value != PenarikanSukses || q.Args[2].Value != PenarikanGagal {
		t.Errorf("args = %+v", q.Args)
	}
}

func kueriPertama(f *fakesql.DB, mengandung string) (fakesql.Stmt, bool) {
	for _, q := range f.Queries() {
		if strings.Contains(q.Query, mengandung) {
			return q, true
		}
	}
	return fakesql.Stmt{}, false
}

// Percobaan ulang: tugas yang gagal dua kali dan sudah melewati jarak antar percobaan ditarik lagi sebagai percobaan ke-3, di luar jendela jam.
func TestCekOtomatisMencobaUlangTugasGagalDiLuarJendelaJam(t *testing.T) {
	now := wib(2026, 10, 4, 12) // di luar jendela 01-05
	var dipanggil []string
	stub := func(_ context.Context, tg Tugas, _ string, _ func(string)) (HasilSinkron, error) {
		dipanggil = append(dipanggil, tg.Kunci()+"#"+string(rune('0'+percobaanKe(tg))))
		return HasilSinkron{TotalSinkron: 1}, nil
	}
	m, f := penarikDenganJawaban(t, stub, map[string]jawabanUji{"WITH r AS": barisGagal("tender/pengumuman", "K10/2026", `{"kode_klpd":"K10","tahun":"2026"}`, "timeout",
		now.Add(-time.Hour), now.Add(-20*time.Minute))})
	penjadwalConfig(t)
	m.cekOtomatis(context.Background(), now)
	tungguSelesai(t, m)

	if len(dipanggil) != 1 || dipanggil[0] != "tender/pengumuman|K10/2026#3" {
		t.Fatalf("dipanggil = %v, want hanya percobaan ke-3 tender/pengumuman K10/2026 (tugas reguler menunggu jendela jam)", dipanggil)
	}
	// Riwayat mencatat percobaan ke-3 dan isian tugas (untuk percobaan berikutnya).
	var ada bool
	for _, q := range f.Queries() {
		if strings.Contains(q.Query, "INSERT INTO inaproc_penarikan (") {
			ada = true
			var perm PermintaanTarik
			if q.Args[3].Value != PemicuOtomatis || q.Args[6].Value != int64(3) || json.Unmarshal([]byte(q.Args[7].Value.(string)), &perm) != nil || perm.Tahun != "2026" || perm.KodeKLPD != "K10" {
				t.Errorf("INSERT riwayat = %+v", q.Args)
			}
		}
	}
	if !ada {
		t.Error("riwayat tidak dibuat")
	}
}

func TestCekOtomatisTidakMenarikSelamaIstirahatAtauDitahan(t *testing.T) {
	now := wib(2026, 10, 4, 2) // di dalam jendela: tugas reguler lain pun akan jalan bila tidak ditahan
	var n int32
	stub := func(context.Context, Tugas, string, func(string)) (HasilSinkron, error) {
		atomic.AddInt32(&n, 1)
		return HasilSinkron{}, nil
	}
	// Semua tugas rencana otomatis sudah segar kecuali satu yang gagal tiga kali (istirahat).
	p := PengaturanBawaan()
	p.Dataset = []string{"tender/pengumuman"}
	p.JumlahTahun = 1
	jawab := map[string]jawabanUji{
		"WITH r AS": barisGagal("tender/pengumuman", "K10/2026", `{"kode_klpd":"K10","tahun":"2026"}`, "x", now.Add(-2*time.Hour), now.Add(-90*time.Minute), now.Add(-time.Hour)),
		"inaproc_penarikan_pengaturan": {kolom: []string{"aktif", "interval_hari", "jam_mulai", "jam_akhir", "kode_klpd", "jumlah_tahun", "jeda_detik", "dataset", "diubah", "diubah_oleh"},
			baris: [][]driver.Value{{true, int64(2), int64(1), int64(5), "K10", int64(1), int64(0), `["tender/pengumuman"]`, now.UTC(), "uji"}}},
	}
	m, _ := penarikDenganJawaban(t, stub, jawab)
	penjadwalConfig(t)
	m.cekOtomatis(context.Background(), now)
	if m.Aktif() != nil || n != 0 {
		t.Fatalf("istirahat 8 jam: tidak boleh menarik (aktif=%v, panggilan=%d)", m.Aktif() != nil, n)
	}

	// Istirahat sudah lewat (gagal terakhir 9 jam lalu): siklus baru, ditarik.
	jawab["WITH r AS"] = barisGagal("tender/pengumuman", "K10/2026", `{"kode_klpd":"K10","tahun":"2026"}`, "x", now.Add(-11*time.Hour), now.Add(-10*time.Hour), now.Add(-9*time.Hour))
	m2, _ := penarikDenganJawaban(t, stub, jawab)
	m2.cekOtomatis(context.Background(), now)
	tungguSelesai(t, m2)
	if n != 1 {
		t.Fatalf("setelah istirahat: panggilan = %d, want 1", n)
	}

	// Ditahan pemutus beruntun: tidak menarik walau semuanya jatuh tempo.
	jawab["WITH r AS"] = barisGagal("", "", "", "")
	m3, _ := penarikDenganJawaban(t, stub, jawab)
	m3.TahanOtomatis(now.Add(20 * time.Minute))
	m3.cekOtomatis(context.Background(), now)
	if m3.Aktif() != nil || n != 1 {
		t.Errorf("ditahan: aktif=%v panggilan=%d", m3.Aktif() != nil, n)
	}
	m3.cekOtomatis(context.Background(), now.Add(25*time.Minute)) // setelah masa tahan
	tungguSelesai(t, m3)
	if n != 2 {
		t.Errorf("setelah masa tahan: panggilan = %d, want 2", n)
	}
}

// penjadwalConfig mengisi konfigurasi Inaproc yang dibutuhkan penjadwal (token ada, otomatis aktif).
func penjadwalConfig(t *testing.T) bool {
	t.Helper()
	pasangKonfigInaproc(t, "http://tidak-dipakai.invalid", "token-uji")
	config.Cfg.InaprocAutoSync, config.Cfg.InaprocAutoIntervalHari, config.Cfg.InaprocAutoJamMulai, config.Cfg.InaprocAutoJamAkhir, config.Cfg.InaprocAutoJumlahTahun = true, 2, 1, 5, 2
	return true
}

// ---- tidak saling tabrakan ----

func TestKunciDatabaseMenolakAntreanBilaDipegangSalinanLain(t *testing.T) {
	var ditolak atomic.Bool
	ditolak.Store(true)
	m, f := penarikDenganJawaban(t, sukses(1), nil)
	dasar := f.OnQuery
	f.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "sp_getapplock") {
			kode := int64(0)
			if ditolak.Load() {
				kode = -1 // kunci dipegang sesi lain (mis. salinan backend lama saat deploy)
			}
			return []string{"r"}, [][]driver.Value{{kode}}, nil
		}
		return dasar(ctx, q, args)
	}
	tg := tugasUji(t, "tender/pengumuman", PermintaanTarik{Tahun: "2025"})
	if _, err := m.Start([]Tugas{tg}, PemicuOtomatis, Oleh{}, 0); err != ErrPenarikanSibuk {
		t.Fatalf("err = %v, want ErrPenarikanSibuk", err)
	}
	if m.Aktif() != nil {
		t.Error("antrean tidak boleh tersisa setelah kunci ditolak")
	}
	for _, q := range f.Queries() {
		if strings.Contains(q.Query, "INSERT INTO inaproc_penarikan (") {
			t.Error("riwayat tidak boleh dibuat bila kunci ditolak")
		}
	}

	// Kunci dilepas pihak lain: Start berhasil, dan kunci dilepas lagi saat antrean selesai.
	ditolak.Store(false)
	if _, err := m.Start([]Tugas{tg}, PemicuOtomatis, Oleh{}, 0); err != nil {
		t.Fatal(err)
	}
	tungguSelesai(t, m)
	var dilepas bool
	for _, ex := range f.Execs() {
		if strings.Contains(ex.Query, "sp_releaseapplock") {
			dilepas = true
		}
	}
	if !dilepas {
		t.Error("kunci database harus dilepas setelah antrean selesai")
	}
}


// ---- kuota ----

func TestTugasMenungguKuotaSebelumMulai(t *testing.T) {
	var dimulai time.Time
	var jam *jamUji
	m, _ := penarikUji(t, func(context.Context, Tugas, string, func(string)) (HasilSinkron, error) {
		dimulai = jam.Now()
		return HasilSinkron{TotalSinkron: 1}, nil
	})
	b, j := batasUji(t)
	jam = j
	b.perJam = 10
	mulai := j.Now()
	for i := 0; i < 10; i++ { // kuota jam habis
		_ = b.Tunggu(context.Background(), 0)
	}
	if _, err := m.Start([]Tugas{tugasUji(t, "tender/pengumuman", PermintaanTarik{Tahun: "2025"})}, PemicuManual, Oleh{}, 0); err != nil {
		t.Fatal(err)
	}
	tungguSelesai(t, m)
	if dimulai.IsZero() || dimulai.Sub(mulai) < 59*time.Minute {
		t.Errorf("tugas dimulai +%s, want setelah jatah jam longgar (sekitar satu jam)", dimulai.Sub(mulai))
	}
}

func TestPerkiraanPermintaanDariHalamanPenarikanSebelumnya(t *testing.T) {
	tg := tugasUji(t, "tender/pengumuman", PermintaanTarik{Tahun: "2025"})
	m, f := penarikDenganJawaban(t, sukses(0), map[string]jawabanUji{"SELECT TOP 1 halaman": {kolom: []string{"halaman"}, baris: [][]driver.Value{{int64(100)}}}})
	if got := m.perkiraanPermintaan(context.Background(), tg); got != 132 {
		t.Errorf("perkiraan = %d, want 132 (100 halaman, cadangan 30 persen, dan 2 permintaan)", got)
	}
	if q, ok := kueriPertama(f, "SELECT TOP 1 halaman"); !ok || q.Args[0].Value != "tender/pengumuman" || q.Args[1].Value != "K10/2025" || q.Args[2].Value != PenarikanSukses {
		t.Errorf("query perkiraan = %+v", q)
	}
	m2, _ := penarikUji(t, sukses(0))
	if got := m2.perkiraanPermintaan(context.Background(), tg); got != 30 {
		t.Errorf("tanpa riwayat = %d, want 30", got)
	}
}
