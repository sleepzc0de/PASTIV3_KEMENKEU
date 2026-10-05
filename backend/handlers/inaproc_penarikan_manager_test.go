package handlers

import (
	"context"
	"database/sql/driver"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"pasti-v3-backend/config"
	"pasti-v3-backend/internal/fakesql"
)

// ---- pembantu ----

// penarikUji: pengelola dengan database palsu yang menjawab INSERT ... OUTPUT dengan id berurutan.
func penarikUji(t *testing.T, stub pelaksanaTugas) (*PenarikInaproc, *fakesql.DB) {
	t.Helper()
	db, f := fakesql.New(t)
	var seq int64
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "INSERT INTO inaproc_penarikan") {
			return []string{"id"}, [][]driver.Value{{atomic.AddInt64(&seq, 1)}}, nil
		}
		return nil, nil, nil // SELECT lain: tanpa baris
	}
	m := NewPenarikInaproc(db)
	m.jalankan = stub
	batasUji(t)
	return m, f
}

func tungguSelesai(t *testing.T, m *PenarikInaproc) {
	t.Helper()
	batas := time.Now().Add(5 * time.Second)
	for m.Aktif() != nil {
		if time.Now().After(batas) {
			t.Fatal("antrean tidak selesai dalam 5 detik")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func tugasUji(t *testing.T, id string, perm PermintaanTarik) Tugas {
	t.Helper()
	d, ok := DatasetByID(id)
	if !ok {
		t.Fatalf("dataset %s tidak ada", id)
	}
	p := d.Normalisasi(perm)
	if pesan := d.Validasi(p); pesan != "" {
		t.Fatal(pesan)
	}
	return Tugas{Dataset: d, Perm: p}
}

// statusAkhir: status terakhir tiap id riwayat, dari perintah UPDATE penutup.
func statusAkhir(f *fakesql.DB) map[int64]string {
	out := map[int64]string{}
	for _, ex := range f.Execs() {
		if strings.Contains(ex.Query, "UPDATE inaproc_penarikan SET status = @p1, selesai") {
			out[ex.Args[6].Value.(int64)] = ex.Args[0].Value.(string)
		}
	}
	return out
}

func sukses(n int) pelaksanaTugas {
	return func(context.Context, Tugas, string, func(string)) (HasilSinkron, error) {
		return HasilSinkron{TotalSinkron: n, Halaman: 1}, nil
	}
}

// ---- antrean ----

func TestStartMenjalankanTugasBerurutanDanMencatatRiwayat(t *testing.T) {
	var mu sync.Mutex
	var urutan, pengguna []string
	m, f := penarikUji(t, func(_ context.Context, tg Tugas, oleh string, kabar func(string)) (HasilSinkron, error) {
		mu.Lock()
		urutan = append(urutan, tg.Dataset.ID+"|"+tg.Dataset.Ringkas(tg.Perm))
		pengguna = append(pengguna, oleh)
		mu.Unlock()
		kabar("separuh jalan")
		return HasilSinkron{TotalSinkron: 7, TotalGagal: 1, Halaman: 2}, nil
	})

	info, err := m.Start([]Tugas{
		tugasUji(t, "tender/pengumuman", PermintaanTarik{Tahun: "2025"}),
		tugasUji(t, "rup/paket-penyedia", PermintaanTarik{Tahun: "2024"}),
	}, PemicuManual, Oleh{ID: "u-1", Nama: "admin"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if info.Total != 2 || info.Pemicu != PemicuManual || info.Oleh != "admin" || len(info.Tugas) != 2 || info.BatchID == "" {
		t.Errorf("info awal = %+v", info)
	}
	tungguSelesai(t, m)

	if got := strings.Join(urutan, ","); got != "tender/pengumuman|K10/2025,rup/paket-penyedia|K10/2024" {
		t.Errorf("urutan = %s", got)
	}
	if pengguna[0] != "u-1" || pengguna[1] != "u-1" {
		t.Errorf("id pengguna diteruskan ke pelaksana = %v", pengguna)
	}
	st := statusAkhir(f)
	if len(st) != 2 || st[1] != PenarikanSukses || st[2] != PenarikanSukses {
		t.Errorf("status akhir = %v", st)
	}
	// Dua baris riwayat dibuat lebih dulu (antri), sebelum tugas pertama berjalan.
	var antri int
	for _, q := range f.Queries() {
		if strings.Contains(q.Query, "INSERT INTO inaproc_penarikan") {
			antri++
			if q.Args[3].Value != PemicuManual || q.Args[5].Value != "admin" {
				t.Errorf("INSERT riwayat = %+v", q.Args)
			}
		}
	}
	if antri != 2 {
		t.Errorf("baris riwayat = %d, want 2", antri)
	}
	// Catatan baris gagal ikut tersimpan di pesan.
	var ada bool
	for _, ex := range f.Execs() {
		if strings.Contains(ex.Query, "SET status = @p1, selesai") && ex.Args[5].Value == "1 baris gagal disimpan" {
			ada = true
		}
	}
	if !ada {
		t.Error("pesan '1 baris gagal disimpan' tidak tercatat")
	}
}

func TestStartMenolakAntreanKeduaSelamaYangPertamaBerjalan(t *testing.T) {
	lepas := make(chan struct{})
	m, _ := penarikUji(t, func(ctx context.Context, _ Tugas, _ string, _ func(string)) (HasilSinkron, error) {
		<-lepas
		return HasilSinkron{}, nil
	})
	tg := tugasUji(t, "tender/pengumuman", PermintaanTarik{Tahun: "2025"})
	if _, err := m.Start([]Tugas{tg}, PemicuManual, Oleh{Nama: "a"}, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start([]Tugas{tg}, PemicuOtomatis, Oleh{Nama: "b"}, 0); !errors.Is(err, ErrPenarikanSibuk) {
		t.Errorf("err = %v, want ErrPenarikanSibuk", err)
	}
	if m.Aktif() == nil {
		t.Error("antrean pertama harus masih aktif")
	}
	close(lepas)
	tungguSelesai(t, m)
	// Setelah selesai, antrean baru diterima lagi.
	if _, err := m.Start([]Tugas{tg}, PemicuManual, Oleh{Nama: "a"}, 0); err != nil {
		t.Errorf("antrean setelah selesai ditolak: %v", err)
	}
	tungguSelesai(t, m)
}

func TestStartMenggabungkanTugasGandaDanMenolakKosong(t *testing.T) {
	var n int32
	m, _ := penarikUji(t, func(context.Context, Tugas, string, func(string)) (HasilSinkron, error) {
		atomic.AddInt32(&n, 1)
		return HasilSinkron{}, nil
	})
	tg := tugasUji(t, "tender/pengumuman", PermintaanTarik{Tahun: "2025"})
	// Isian sama setelah dinormalisasi (K10 bawaan, spasi dipangkas) = tugas yang sama.
	ganda := tugasUji(t, "tender/pengumuman", PermintaanTarik{Tahun: " 2025 ", KodeKLPD: "K10"})
	info, err := m.Start([]Tugas{tg, ganda}, PemicuManual, Oleh{}, 0)
	if err != nil || info.Total != 1 {
		t.Fatalf("info=%+v err=%v, want 1 tugas", info, err)
	}
	tungguSelesai(t, m)
	if n != 1 {
		t.Errorf("pelaksana dipanggil %d kali, want 1", n)
	}
	if _, err := m.Start(nil, PemicuManual, Oleh{}, 0); !errors.Is(err, ErrTidakAdaTugas) {
		t.Errorf("err = %v, want ErrTidakAdaTugas", err)
	}
}

func TestBatalkanMenghentikanTugasBerjalanDanMembatalkanSisanya(t *testing.T) {
	mulai := make(chan struct{})
	m, f := penarikUji(t, func(ctx context.Context, _ Tugas, _ string, _ func(string)) (HasilSinkron, error) {
		close(mulai)
		<-ctx.Done()
		return HasilSinkron{}, &GalatSinkron{Status: statusDibatalkan, Pesan: "Sinkronisasi dibatalkan"}
	})
	if _, err := m.Start([]Tugas{
		tugasUji(t, "tender/pengumuman", PermintaanTarik{Tahun: "2025"}),
		tugasUji(t, "tender/peserta-tender", PermintaanTarik{Tahun: "2025"}),
		tugasUji(t, "tender/tender-selesai", PermintaanTarik{Tahun: "2025"}),
	}, PemicuManual, Oleh{Nama: "admin"}, 0); err != nil {
		t.Fatal(err)
	}
	<-mulai
	if !m.Batalkan() {
		t.Fatal("Batalkan harus mengembalikan true saat ada antrean")
	}
	if a := m.Aktif(); a != nil && !a.Dibatalkan {
		t.Error("info aktif harus menandai pembatalan")
	}
	tungguSelesai(t, m)
	st := statusAkhir(f)
	for id := int64(1); id <= 3; id++ {
		if st[id] != PenarikanDibatalkan {
			t.Errorf("tugas %d: status %q, want dibatalkan (semua: %v)", id, st[id], st)
		}
	}
	if m.Batalkan() {
		t.Error("Batalkan tanpa antrean harus false")
	}
}

// Kegagalan satu tugas (mis. 429 yang tidak sembuh setelah pengulangan per halaman di klien) dicatat gagal dan tidak menghentikan sisa
// antrean; pengulangan tugasnya ditangani kebijakan percobaan, bukan antrean.
func TestKegagalanSatuTugasTidakMenghentikanSisanya(t *testing.T) {
	var panggilan int32
	m, f := penarikUji(t, func(_ context.Context, tg Tugas, _ string, _ func(string)) (HasilSinkron, error) {
		atomic.AddInt32(&panggilan, 1)
		if tg.Dataset.ID == "tender/pengumuman" {
			return HasilSinkron{}, &GalatSinkron{Status: 429, Pesan: "Sinkronisasi gagal: Rate limit exceeded", Hulu: true}
		}
		return HasilSinkron{TotalSinkron: 1}, nil
	})
	if _, err := m.Start([]Tugas{
		tugasUji(t, "tender/pengumuman", PermintaanTarik{Tahun: "2025"}),
		tugasUji(t, "tender/peserta-tender", PermintaanTarik{Tahun: "2025"}),
	}, PemicuManual, Oleh{}, 0); err != nil {
		t.Fatal(err)
	}
	tungguSelesai(t, m)
	st := statusAkhir(f)
	if st[1] != PenarikanGagal || st[2] != PenarikanSukses {
		t.Errorf("status = %v, want tugas 1 gagal dan tugas 2 sukses", st)
	}
	if panggilan != 2 {
		t.Errorf("panggilan = %d, want 2 (tidak ada pengulangan tugas di tingkat antrean)", panggilan)
	}
}

// Pemutus beruntun: banyak tugas gagal berturut-turut menghentikan antrean (sisanya dilewati, tidak dihitung gagal) dan menahan penarikan otomatis.
func TestPemutusBeruntunMenghentikanAntreanDanMenahanOtomatis(t *testing.T) {
	lama, lamaJeda := ambangPemutus, jedaPemutus
	ambangPemutus, jedaPemutus = 3, 30*time.Minute
	defer func() { ambangPemutus, jedaPemutus = lama, lamaJeda }()

	var panggilan int32
	m, f := penarikUji(t, func(context.Context, Tugas, string, func(string)) (HasilSinkron, error) {
		atomic.AddInt32(&panggilan, 1)
		return HasilSinkron{}, &GalatSinkron{Status: http.StatusBadGateway, Pesan: "Gagal menghubungi API Inaproc"}
	})
	var tugas []Tugas
	for _, id := range []string{"tender/pengumuman", "tender/peserta-tender", "tender/tender-selesai", "tender/tender-selesai-nilai", "tender/tender-ekontrak"} {
		tugas = append(tugas, tugasUji(t, id, PermintaanTarik{Tahun: "2025"}))
	}
	if _, err := m.Start(tugas, PemicuOtomatis, Oleh{Nama: "otomatis"}, 0); err != nil {
		t.Fatal(err)
	}
	tungguSelesai(t, m)
	st := statusAkhir(f)
	if panggilan != 3 || st[1] != PenarikanGagal || st[2] != PenarikanGagal || st[3] != PenarikanGagal || st[4] != PenarikanDilewati || st[5] != PenarikanDilewati {
		t.Errorf("panggilan=%d status=%v, want 3 gagal lalu 2 dilewati", panggilan, st)
	}
	if t0 := m.otomatisDitahanSampai(); !t0.After(time.Now().Add(25 * time.Minute)) {
		t.Errorf("penarikan otomatis harus ditahan sekitar 30 menit, ditahan sampai %v", t0)
	}
	// 503 dari Inaproc sendiri (Hulu) tidak dianggap token ditolak: antrean lanjut sampai pemutus beruntun, bukan langsung dilewati semua.
}

// Berhasil di tengah memutus hitungan kegagalan beruntun.
func TestPemutusBeruntunDiulangBilaAdaYangBerhasil(t *testing.T) {
	lama := ambangPemutus
	ambangPemutus = 3
	defer func() { ambangPemutus = lama }()
	var i int32
	m, f := penarikUji(t, func(context.Context, Tugas, string, func(string)) (HasilSinkron, error) {
		if n := atomic.AddInt32(&i, 1); n == 3 {
			return HasilSinkron{TotalSinkron: 1}, nil // 2 gagal, 1 sukses, 2 gagal lagi: tidak sampai 3 beruntun
		}
		return HasilSinkron{}, &GalatSinkron{Status: http.StatusServiceUnavailable, Pesan: "layanan tidak tersedia", Hulu: true}
	})
	var tugas []Tugas
	for _, id := range []string{"tender/pengumuman", "tender/peserta-tender", "tender/tender-selesai", "tender/tender-selesai-nilai", "tender/tender-ekontrak"} {
		tugas = append(tugas, tugasUji(t, id, PermintaanTarik{Tahun: "2025"}))
	}
	if _, err := m.Start(tugas, PemicuManual, Oleh{}, 0); err != nil {
		t.Fatal(err)
	}
	tungguSelesai(t, m)
	for id, s := range statusAkhir(f) {
		if s == PenarikanDilewati {
			t.Errorf("tugas %d dilewati padahal hitungan beruntun terputus oleh keberhasilan", id)
		}
	}
}

// Token ditolak: sisa antrean dilewati, bukan dicoba satu per satu.
func TestTokenDitolakMelewatiSisaAntrean(t *testing.T) {
	var panggilan int32
	m, f := penarikUji(t, func(context.Context, Tugas, string, func(string)) (HasilSinkron, error) {
		atomic.AddInt32(&panggilan, 1)
		return HasilSinkron{}, &GalatSinkron{Status: http.StatusUnauthorized, Pesan: "Sinkronisasi gagal: token tidak valid", Hulu: true}
	})
	if _, err := m.Start([]Tugas{
		tugasUji(t, "tender/pengumuman", PermintaanTarik{Tahun: "2025"}),
		tugasUji(t, "tender/peserta-tender", PermintaanTarik{Tahun: "2025"}),
		tugasUji(t, "tender/tender-selesai", PermintaanTarik{Tahun: "2025"}),
	}, PemicuManual, Oleh{}, 0); err != nil {
		t.Fatal(err)
	}
	tungguSelesai(t, m)
	st := statusAkhir(f)
	if panggilan != 1 || st[1] != PenarikanGagal || st[2] != PenarikanDilewati || st[3] != PenarikanDilewati {
		t.Errorf("panggilan=%d status=%v", panggilan, st)
	}
}

func TestPanikDiPelaksanaTidakMenggantungkanAntrean(t *testing.T) {
	m, f := penarikUji(t, func(context.Context, Tugas, string, func(string)) (HasilSinkron, error) {
		panic("boom")
	})
	if _, err := m.Start([]Tugas{tugasUji(t, "tender/pengumuman", PermintaanTarik{Tahun: "2025"})}, PemicuManual, Oleh{}, 0); err != nil {
		t.Fatal(err)
	}
	tungguSelesai(t, m)
	if st := statusAkhir(f); st[1] != PenarikanGagal {
		t.Errorf("status = %v, want gagal", st)
	}
	// Setelah panik, antrean baru tetap bisa dimulai.
	m.jalankan = sukses(1)
	if _, err := m.Start([]Tugas{tugasUji(t, "tender/pengumuman", PermintaanTarik{Tahun: "2025"})}, PemicuManual, Oleh{}, 0); err != nil {
		t.Errorf("antrean setelah panik ditolak: %v", err)
	}
	tungguSelesai(t, m)
}

func TestJedaAntarTugasDapatDibatalkan(t *testing.T) {
	var panggilan int32
	m, _ := penarikUji(t, func(context.Context, Tugas, string, func(string)) (HasilSinkron, error) {
		atomic.AddInt32(&panggilan, 1)
		return HasilSinkron{}, nil
	})
	if _, err := m.Start([]Tugas{
		tugasUji(t, "tender/pengumuman", PermintaanTarik{Tahun: "2025"}),
		tugasUji(t, "tender/peserta-tender", PermintaanTarik{Tahun: "2025"}),
	}, PemicuManual, Oleh{}, time.Hour); err != nil {
		t.Fatal(err)
	}
	for atomic.LoadInt32(&panggilan) < 1 {
		time.Sleep(time.Millisecond)
	}
	m.Batalkan()
	tungguSelesai(t, m) // tanpa dibatalkan, jeda satu jam akan membuat tes ini macet
	if panggilan != 1 {
		t.Errorf("panggilan = %d, want 1", panggilan)
	}
}

// Server dimulai ulang bukan kesalahan sumber data: yang menggantung ditandai dibatalkan (tidak menghabiskan kesempatan percobaan).
func TestRecoverOrphansMenandaiAntrianDanBerjalanDibatalkan(t *testing.T) {
	m, f := penarikUji(t, sukses(0))
	if _, err := m.RecoverOrphans(context.Background()); err != nil {
		t.Fatal(err)
	}
	var ada bool
	for _, ex := range f.Execs() {
		if strings.Contains(ex.Query, "UPDATE inaproc_penarikan SET status = @p1") && strings.Contains(ex.Query, "IN (@p3, @p4)") {
			ada = true
			if ex.Args[0].Value != PenarikanDibatalkan || ex.Args[2].Value != PenarikanAntri || ex.Args[3].Value != PenarikanBerjalan {
				t.Errorf("args = %+v", ex.Args)
			}
		}
	}
	if !ada {
		t.Error("UPDATE pemulihan tidak dijalankan")
	}
}

// ---- pengaturan dan jadwal ----

func wib(th, bln, tgl, jam int) time.Time {
	return time.Date(th, time.Month(bln), tgl, jam, 0, 0, 0, zonaWIB)
}

func pengaturanUji() Pengaturan {
	p := Pengaturan{Aktif: true, IntervalHari: 2, JamMulai: 1, JamAkhir: 5, KodeKLPD: "K10", JumlahTahun: 2, JedaDetik: 2, Dataset: IDDatasetOtomatis()}
	return p
}

func TestPengaturanRapikanMenolakNilaiNgawur(t *testing.T) {
	salah := []func(*Pengaturan){
		func(p *Pengaturan) { p.IntervalHari = 0 },
		func(p *Pengaturan) { p.IntervalHari = 31 },
		func(p *Pengaturan) { p.JamMulai = 24 },
		func(p *Pengaturan) { p.JamAkhir = -1 },
		func(p *Pengaturan) { p.KodeKLPD = "  " },
		func(p *Pengaturan) { p.KodeKLPD = strings.Repeat("K", 21) },
		func(p *Pengaturan) { p.JumlahTahun = 0 },
		func(p *Pengaturan) { p.JumlahTahun = 6 },
		func(p *Pengaturan) { p.JedaDetik = 61 },
		func(p *Pengaturan) { p.Dataset = []string{"tidak/ada"} },
		func(p *Pengaturan) { p.Dataset = []string{"ekatalog/penyedia-detail"} }, // per kode: tidak bisa otomatis
	}
	for i, ubah := range salah {
		p := pengaturanUji()
		ubah(&p)
		if p.Rapikan() == "" {
			t.Errorf("kasus %d: pengaturan ngawur diterima: %+v", i, p)
		}
	}

	p := pengaturanUji()
	p.Dataset = []string{"ekatalog/paket-e-purchasing", "rup/paket-penyedia", "rup/paket-penyedia"}
	p.KodeKLPD = " K10 "
	if pesan := p.Rapikan(); pesan != "" {
		t.Fatal(pesan)
	}
	// Diurutkan menurut katalog dan tanpa duplikat.
	if strings.Join(p.Dataset, ",") != "rup/paket-penyedia,ekatalog/paket-e-purchasing" || p.KodeKLPD != "K10" {
		t.Errorf("setelah dirapikan: %+v", p)
	}
}

func TestPengaturanBawaanDariKonfigurasi(t *testing.T) {
	pasangKonfigInaproc(t, "http://x", "t")
	config.Cfg.InaprocAutoSync, config.Cfg.InaprocAutoIntervalHari, config.Cfg.InaprocAutoJamMulai, config.Cfg.InaprocAutoJamAkhir, config.Cfg.InaprocAutoJumlahTahun = true, 3, 22, 4, 1
	p := PengaturanBawaan()
	if !p.Aktif || p.IntervalHari != 3 || p.JamMulai != 22 || p.JamAkhir != 4 || p.JumlahTahun != 1 || !p.BawaanServer {
		t.Errorf("bawaan = %+v", p)
	}
	// Nilai di luar rentang diganti bawaan aman.
	config.Cfg.InaprocAutoIntervalHari, config.Cfg.InaprocAutoJumlahTahun, config.Cfg.InaprocAutoJamMulai = 999, 99, 30
	p = PengaturanBawaan()
	if p.IntervalHari != 2 || p.JumlahTahun != 2 || p.JamMulai != 1 || p.JamAkhir != 5 {
		t.Errorf("bawaan setelah nilai ngawur = %+v", p)
	}
	if err := (&p).Rapikan(); err != "" {
		t.Errorf("bawaan sendiri harus lolos Rapikan: %s", err)
	}
}

func TestRencanaOtomatisSemuaDatasetKaliTahun(t *testing.T) {
	p := pengaturanUji()
	tugas := p.RencanaOtomatis(wib(2026, 10, 4, 10))

	// 26 dataset per KLPD+tahun dan 1 transaksi dikali 2 tahun, ditambah 1 dataset per KLPD dan 1 kategori tingkat 1.
	if len(tugas) != 56 {
		t.Fatalf("jumlah tugas = %d, want 56", len(tugas))
	}
	kunci := map[string]bool{}
	for _, tg := range tugas {
		if tg.Dataset.Mode == ModeKode {
			t.Errorf("dataset per kode tidak boleh ikut otomatis: %s", tg.Dataset.ID)
		}
		if kunci[tg.Kunci()] {
			t.Errorf("tugas ganda: %s", tg.Kunci())
		}
		kunci[tg.Kunci()] = true
		if tg.Dataset.Validasi(tg.Perm) != "" {
			t.Errorf("tugas %s tidak lolos validasi: %+v", tg.Kunci(), tg.Perm)
		}
	}
	for _, mau := range []string{"tender/pengumuman|K10/2026", "tender/pengumuman|K10/2025", "ekatalog-archive/instansi-satker|K10",
		"ekatalog/list-kategori-produk|L1", "ekatalog/e-purchasing-by-produk|2026/COMPLETED/K10"} {
		if !kunci[mau] {
			t.Errorf("tugas %q tidak ada", mau)
		}
	}
	// Tahun mengikuti zona WIB: 31 Des 2026 18.00 UTC sudah 2027 di WIB.
	if th := TahunPenarikan(time.Date(2026, 12, 31, 18, 0, 0, 0, time.UTC), 2); th[0] != "2027" || th[1] != "2026" {
		t.Errorf("tahun = %v", th)
	}
	// Hanya dataset terpilih yang ikut.
	p.Dataset = []string{"tender/pengumuman"}
	p.JumlahTahun = 3
	if got := p.RencanaOtomatis(wib(2026, 10, 4, 10)); len(got) != 3 {
		t.Errorf("satu dataset x 3 tahun = %d tugas", len(got))
	}
}

func TestDalamJendelaDanSelaraskan(t *testing.T) {
	p := pengaturanUji()
	for jam, mau := range map[int]bool{0: false, 1: true, 4: true, 5: false, 12: false, 23: false} {
		if got := p.DalamJendela(wib(2026, 10, 4, jam)); got != mau {
			t.Errorf("jam %d: %v, want %v", jam, got, mau)
		}
	}
	// Jendela melewati tengah malam.
	p.JamMulai, p.JamAkhir = 22, 4
	for jam, mau := range map[int]bool{21: false, 22: true, 23: true, 0: true, 3: true, 4: false} {
		if got := p.DalamJendela(wib(2026, 10, 4, jam)); got != mau {
			t.Errorf("22-04, jam %d: %v, want %v", jam, got, mau)
		}
	}
	// Sama = sepanjang hari.
	p.JamMulai, p.JamAkhir = 3, 3
	if !p.DalamJendela(wib(2026, 10, 4, 15)) {
		t.Error("jam mulai = jam akhir harus berarti sepanjang hari")
	}

	p = pengaturanUji()
	if got := p.selaraskan(wib(2026, 10, 4, 10)); !got.Equal(wib(2026, 10, 5, 1)) {
		t.Errorf("setelah jendela: %v, want 5 Okt 01.00 WIB", got)
	}
	if got := p.selaraskan(wib(2026, 10, 4, 0)); !got.Equal(wib(2026, 10, 4, 1)) {
		t.Errorf("sebelum jendela: %v, want 4 Okt 01.00 WIB", got)
	}
	if got := p.selaraskan(wib(2026, 10, 4, 2)); !got.Equal(wib(2026, 10, 4, 2)) {
		t.Errorf("di dalam jendela tidak boleh bergeser: %v", got)
	}
}

func TestJatuhTempoMengikutiIntervalDanSiklusKegagalan(t *testing.T) {
	p := pengaturanUji()
	p.Dataset = []string{"tender/pengumuman"}
	p.JumlahTahun = 1
	now := wib(2026, 10, 4, 2)
	tg := p.RencanaOtomatis(now)[0]
	kunci := tg.Kunci()
	jam := func(j float64) time.Time { return now.Add(-time.Duration(j * float64(time.Hour))) }
	ptr := func(t time.Time) *time.Time { return &t }
	sukses := func(j float64) map[string]RiwayatOtomatis {
		return map[string]RiwayatOtomatis{kunci: {SuksesTerakhir: ptr(jam(j)), CobaTerakhir: jam(j)}}
	}
	gagal := func(jamLalu ...float64) map[string]*KegagalanTugas {
		g := &KegagalanTugas{Dataset: tg.Dataset.ID, Parameter: tg.Dataset.Ringkas(tg.Perm)}
		for _, j := range jamLalu {
			g.Waktu = append(g.Waktu, jam(j))
		}
		return map[string]*KegagalanTugas{kunci: g}
	}

	kasus := []struct {
		nama        string
		riwayat     map[string]RiwayatOtomatis
		gagal       map[string]*KegagalanTugas
		lanjutan    int
		reguler     int
		percobaanKe int
	}{
		{"belum pernah", nil, nil, 0, 1, 0},
		{"sukses 24 jam lalu (interval 2 hari)", sukses(24), nil, 0, 0, 0},
		{"sukses 49 jam lalu", sukses(49), nil, 0, 1, 0},
		{"tugas lain yang sukses tidak berpengaruh", map[string]RiwayatOtomatis{"tender/pengumuman|K10/2020": {SuksesTerakhir: ptr(jam(1)), CobaTerakhir: jam(1)}}, nil, 0, 1, 0},

		// Kegagalan pertama: dicoba ulang setelah jarak antar percobaan (10 menit), tanpa menunggu interval.
		{"gagal 5 menit lalu: masih menunggu jarak antar percobaan", sukses(100), gagal(5.0 / 60), 0, 0, 0},
		{"gagal 15 menit lalu: percobaan ke-2", sukses(100), gagal(0.25), 1, 0, 2},
		{"gagal 2 kali, terakhir 15 menit lalu: percobaan ke-3", nil, gagal(1, 0.25), 1, 0, 3},
		{"sukses baru (interval belum lewat) tetapi ada gagal sesudahnya: tetap dilanjutkan", sukses(24), gagal(0.25), 1, 0, 2},

		// Setelah 3 kali gagal: istirahat 8 jam sejak kegagalan ketiga, lalu siklus baru.
		{"3 gagal, terakhir 15 menit lalu: istirahat", nil, gagal(2, 1, 0.25), 0, 0, 0},
		{"3 gagal, terakhir 7 jam lalu: masih istirahat", nil, gagal(9, 8, 7), 0, 0, 0},
		{"3 gagal, terakhir 9 jam lalu: istirahat selesai, siklus baru percobaan ke-1", nil, gagal(11, 10, 9), 1, 0, 1},
		{"istirahat selesai lalu gagal lagi 15 menit lalu: percobaan ke-2 siklus baru", nil, gagal(20, 19, 18, 0.25), 1, 0, 2},
	}
	for _, k := range kasus {
		l, r := p.TugasTerjadwal(now, k.riwayat, k.gagal)
		if len(l) != k.lanjutan || len(r) != k.reguler {
			t.Errorf("%s: lanjutan=%d reguler=%d, want %d dan %d", k.nama, len(l), len(r), k.lanjutan, k.reguler)
			continue
		}
		if k.lanjutan == 1 && k.percobaanKe > 0 && l[0].Percobaan != k.percobaanKe {
			t.Errorf("%s: percobaan ke-%d, want ke-%d", k.nama, l[0].Percobaan, k.percobaanKe)
		}
		if got := len(p.JatuhTempo(now, k.riwayat, k.gagal)); got != k.lanjutan+k.reguler {
			t.Errorf("%s: JatuhTempo = %d", k.nama, got)
		}
	}
}

func TestKondisiSiklusPercobaan(t *testing.T) {
	now := wib(2026, 10, 4, 12)
	jam := func(j float64) time.Time { return now.Add(-time.Duration(j * float64(time.Hour))) }
	ls := func(j ...float64) []time.Time {
		var out []time.Time
		for _, x := range j {
			out = append(out, jam(x))
		}
		return out
	}

	if k := Kondisi(nil, now); k.BelumPulih || k.Terpakai != 0 || k.Istirahat {
		t.Errorf("tanpa gagal = %+v", k)
	}
	k := Kondisi(ls(1), now)
	if !k.BelumPulih || k.Terpakai != 1 || k.Istirahat || !k.SaatCobaLagi().Equal(jam(1).Add(jedaAntarPercobaan)) {
		t.Errorf("satu gagal = %+v", k)
	}
	k = Kondisi(ls(3, 2, 1), now)
	if !k.Istirahat || k.Terpakai != 3 || !k.IstirahatSampai.Equal(jam(1).Add(8*time.Hour)) || !k.SaatCobaLagi().Equal(k.IstirahatSampai) {
		t.Errorf("tiga gagal = %+v", k)
	}
	// Gagal yang tercatat selama istirahat (mis. penarikan manual) tidak memperpanjang istirahat dan tidak membuka siklus baru.
	k = Kondisi(ls(3, 2, 1, 0.5), now)
	if !k.Istirahat || !k.IstirahatSampai.Equal(jam(1).Add(8*time.Hour)) {
		t.Errorf("gagal saat istirahat = %+v", k)
	}
	// Istirahat selesai: siklus baru dimulai dari nol dan masih tercatat belum pulih.
	k = Kondisi(ls(20, 19, 18), now)
	if k.Istirahat || k.Terpakai != 0 || !k.BelumPulih || !k.SaatCobaLagi().IsZero() {
		t.Errorf("setelah istirahat = %+v", k)
	}
	// Enam gagal dengan istirahat di tengah: dua siklus penuh, istirahat kedua masih berjalan.
	k = Kondisi(ls(40, 39, 38, 6, 5, 4), now)
	if !k.Istirahat || !k.IstirahatSampai.Equal(jam(4).Add(8*time.Hour)) {
		t.Errorf("dua siklus = %+v", k)
	}
	// Kebijakan dapat diubah (env): 2 percobaan, istirahat 1 jam.
	lamaPerc, lamaIst := maksPercobaan, lamaIstirahat
	maksPercobaan, lamaIstirahat = 2, time.Hour
	defer func() { maksPercobaan, lamaIstirahat = lamaPerc, lamaIst }()
	if k := Kondisi(ls(3, 2), now); k.Istirahat || k.Terpakai != 0 {
		t.Errorf("istirahat 1 jam sudah lewat = %+v", k)
	}
	if k := Kondisi(ls(2, 0.5), now); !k.Istirahat || k.Terpakai != 2 {
		t.Errorf("2 gagal = %+v", k)
	}
}

func TestTugasDiLuarRencanaHanyaDilanjutkanSelamaSiklusBerjalan(t *testing.T) {
	p := pengaturanUji()
	p.Dataset = []string{"tender/pengumuman"}
	p.JumlahTahun = 1
	now := wib(2026, 10, 4, 12)
	d, _ := DatasetByID("ekatalog/penyedia-detail")
	manual := Tugas{Dataset: d, Perm: d.Normalisasi(PermintaanTarik{Kode: "01ABC"})}
	perm := `{"kode":"01ABC"}`
	g := func(jamLalu ...float64) map[string]*KegagalanTugas {
		k := &KegagalanTugas{Dataset: d.ID, Parameter: d.Ringkas(manual.Perm), Permintaan: perm}
		for _, j := range jamLalu {
			k.Waktu = append(k.Waktu, now.Add(-time.Duration(j*float64(time.Hour))))
		}
		return map[string]*KegagalanTugas{manual.Kunci(): k}
	}
	jml := func(gagal map[string]*KegagalanTugas) (int, int) {
		l, r := p.TugasTerjadwal(now, nil, gagal)
		var luar int
		for _, x := range l {
			if x.Dataset.ID == d.ID {
				luar++
				if x.Perm.Kode != "01ABC" {
					t.Errorf("isian tugas tidak disusun ulang dengan benar: %+v", x.Perm)
				}
			}
		}
		return luar, len(r)
	}
	if luar, _ := jml(g(0.25)); luar != 1 {
		t.Errorf("gagal sekali: %d tugas luar rencana, want 1 (percobaan ke-2)", luar)
	}
	if luar, _ := jml(g(2, 1, 0.25)); luar != 0 {
		t.Errorf("sedang istirahat: %d, want 0", luar)
	}
	if luar, _ := jml(g(11, 10, 9)); luar != 0 {
		t.Errorf("istirahat selesai: tugas manual tidak ditarik otomatis lagi, dapat %d", luar)
	}
	// Baris lama tanpa isian tersimpan tidak bisa dilanjutkan.
	lama := g(0.25)
	lama[manual.Kunci()].Permintaan = ""
	if luar, _ := jml(lama); luar != 0 {
		t.Errorf("tanpa isian tersimpan: %d, want 0", luar)
	}
	// Isian yang tidak cocok dengan kuncinya ditolak.
	salah := g(0.25)
	salah[manual.Kunci()].Permintaan = `{"kode":"LAIN"}`
	if luar, _ := jml(salah); luar != 0 {
		t.Errorf("isian tidak cocok kunci: %d, want 0", luar)
	}
}

func TestBermasalahMenjelaskanKeadaanSiklus(t *testing.T) {
	p := pengaturanUji()
	p.Dataset = []string{"tender/pengumuman"}
	p.JumlahTahun = 1
	now := wib(2026, 10, 4, 12)
	tg := p.RencanaOtomatis(now)[0]
	g := func(jamLalu ...float64) map[string]*KegagalanTugas {
		k := &KegagalanTugas{Dataset: tg.Dataset.ID, Parameter: tg.Dataset.Ringkas(tg.Perm), Pesan: "timeout"}
		for _, j := range jamLalu {
			k.Waktu = append(k.Waktu, now.Add(-time.Duration(j*float64(time.Hour))))
		}
		return map[string]*KegagalanTugas{tg.Kunci(): k}
	}
	b := p.Bermasalah(now, g(0.05))
	if len(b) != 1 || b[0].Gagal != 1 || b[0].MaksPercobaan != 3 || b[0].Istirahat || !b[0].DalamRencana || b[0].BerikutnyaSekitar == nil ||
		!b[0].BerikutnyaSekitar.Equal(now.Add(-3*time.Minute).Add(jedaAntarPercobaan)) || b[0].Pesan != "timeout" {
		t.Errorf("satu gagal = %+v", b)
	}
	b = p.Bermasalah(now, g(2, 1, 0.5))
	if len(b) != 1 || !b[0].Istirahat || b[0].Gagal != 3 || b[0].BerikutnyaSekitar == nil || !b[0].BerikutnyaSekitar.Equal(now.Add(-30*time.Minute).Add(8*time.Hour)) {
		t.Errorf("istirahat = %+v", b)
	}
	p.Aktif = false
	if b := p.Bermasalah(now, g(0.5)); len(b) != 1 || b[0].BerikutnyaSekitar != nil {
		t.Errorf("otomatis nonaktif tidak punya jadwal percobaan: %+v", b)
	}
}

func TestBerikutnya(t *testing.T) {
	p := pengaturanUji()
	p.Dataset = []string{"tender/pengumuman"}
	p.JumlahTahun = 1
	now := wib(2026, 10, 4, 10)
	tg := p.RencanaOtomatis(now)[0]

	// Belum pernah ditarik: jatuh tempo sekarang, tetapi baru boleh mulai di jendela berikutnya.
	if got := p.Berikutnya(now, nil, nil); got == nil || !got.Equal(wib(2026, 10, 5, 1)) {
		t.Errorf("tanpa riwayat: %v, want 5 Okt 01.00 WIB", got)
	}
	// Sukses 4 Okt 02.00: jatuh tempo 6 Okt 02.00 (di dalam jendela).
	riwayat := map[string]RiwayatOtomatis{tg.Kunci(): {SuksesTerakhir: func() *time.Time { x := wib(2026, 10, 4, 2); return &x }(), CobaTerakhir: wib(2026, 10, 4, 2)}}
	if got := p.Berikutnya(now, riwayat, nil); got == nil || !got.Equal(wib(2026, 10, 6, 2)) {
		t.Errorf("setelah sukses: %v, want 6 Okt 02.00 WIB", got)
	}
	// Gagal 9.55 lalu: percobaan ulang 10 menit setelahnya (09.55 + 10 menit = 10.05), tidak menunggu jendela jam.
	gagal := map[string]*KegagalanTugas{tg.Kunci(): {Waktu: []time.Time{now.Add(-5 * time.Minute)}}}
	if got := p.Berikutnya(now, riwayat, gagal); got == nil || !got.Equal(now.Add(5*time.Minute)) {
		t.Errorf("percobaan ulang: %v, want %v", got, now.Add(5*time.Minute))
	}
	// Istirahat 8 jam: berikutnya = akhir istirahat.
	istirahat := map[string]*KegagalanTugas{tg.Kunci(): {Waktu: []time.Time{now.Add(-3 * time.Hour), now.Add(-2 * time.Hour), now.Add(-time.Hour)}}}
	if got := p.Berikutnya(now, riwayat, istirahat); got == nil || !got.Equal(now.Add(-time.Hour).Add(8*time.Hour)) {
		t.Errorf("istirahat: %v", got)
	}
	p.Aktif = false
	if got := p.Berikutnya(now, nil, nil); got != nil {
		t.Errorf("nonaktif harus nil, dapat %v", got)
	}
}

// ---- penjadwal (cekOtomatis) ----

func penjadwalUji(t *testing.T, stub pelaksanaTugas) (*PenarikInaproc, *fakesql.DB) {
	t.Helper()
	pasangKonfigInaproc(t, "http://tidak-dipakai.invalid", "token-uji")
	config.Cfg.InaprocAutoSync, config.Cfg.InaprocAutoIntervalHari, config.Cfg.InaprocAutoJamMulai, config.Cfg.InaprocAutoJamAkhir, config.Cfg.InaprocAutoJumlahTahun = true, 2, 1, 5, 2
	return penarikUji(t, stub)
}

func TestCekOtomatisMemulaiSemuaTugasDiDalamJendela(t *testing.T) {
	var mu sync.Mutex
	var dipanggil []string
	var pengguna []string
	selesaiSatu := make(chan struct{}, 1)
	m, f := penjadwalUji(t, func(_ context.Context, tg Tugas, oleh string, _ func(string)) (HasilSinkron, error) {
		mu.Lock()
		dipanggil = append(dipanggil, tg.Kunci())
		pengguna = append(pengguna, oleh)
		mu.Unlock()
		select {
		case selesaiSatu <- struct{}{}:
		default:
		}
		return HasilSinkron{}, nil
	})
	m.cekOtomatis(context.Background(), wib(2026, 10, 4, 2))

	a := m.Aktif()
	if a == nil || a.Total != 56 || a.Pemicu != PemicuOtomatis || a.Oleh != "otomatis" {
		t.Fatalf("antrean otomatis = %+v", a)
	}
	<-selesaiSatu
	m.Batalkan() // jeda antar tugas 2 detik x 55 tugas: dihentikan setelah tugas pertama
	tungguSelesai(t, m)
	if len(dipanggil) != 1 || dipanggil[0] != "rup/paket-penyedia|K10/2026" {
		t.Errorf("tugas pertama = %v", dipanggil)
	}
	if pengguna[0] != "" {
		t.Errorf("penarikan otomatis tidak punya pengguna, dapat %q", pengguna[0])
	}
	var otomatis int
	for _, q := range f.Queries() {
		if strings.Contains(q.Query, "INSERT INTO inaproc_penarikan") && q.Args[3].Value == PemicuOtomatis {
			otomatis++
		}
	}
	if otomatis != 56 {
		t.Errorf("baris riwayat otomatis = %d, want 56", otomatis)
	}
}

func TestCekOtomatisTidakJalanBilaSyaratTidakTerpenuhi(t *testing.T) {
	var n int32
	stub := func(context.Context, Tugas, string, func(string)) (HasilSinkron, error) {
		atomic.AddInt32(&n, 1)
		return HasilSinkron{}, nil
	}
	m, _ := penjadwalUji(t, stub)

	// Di luar jendela jam.
	m.cekOtomatis(context.Background(), wib(2026, 10, 4, 12))
	if m.Aktif() != nil {
		t.Error("di luar jendela tidak boleh memulai")
	}
	// Token kosong.
	config.Cfg.InaprocToken = ""
	m.cekOtomatis(context.Background(), wib(2026, 10, 4, 2))
	if m.Aktif() != nil {
		t.Error("tanpa token tidak boleh memulai")
	}
	config.Cfg.InaprocToken = "token-uji"
	// Dinonaktifkan di pengaturan server.
	config.Cfg.InaprocAutoSync = false
	m.cekOtomatis(context.Background(), wib(2026, 10, 4, 2))
	if m.Aktif() != nil {
		t.Error("pengaturan nonaktif tidak boleh memulai")
	}
	if n != 0 {
		t.Errorf("pelaksana dipanggil %d kali", n)
	}
}

func TestCekOtomatisMenghormatiRiwayatDanAntreanAktif(t *testing.T) {
	lepas := make(chan struct{})
	m, f := penjadwalUji(t, func(ctx context.Context, _ Tugas, _ string, _ func(string)) (HasilSinkron, error) {
		<-lepas
		return HasilSinkron{}, nil
	})
	// Semua tugas baru sukses 1 jam lalu secara otomatis: tidak ada yang jatuh tempo.
	now := wib(2026, 10, 4, 2)
	p := PengaturanBawaan()
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "GROUP BY dataset, parameter") {
			var baris [][]driver.Value
			for _, tg := range p.RencanaOtomatis(now) {
				baris = append(baris, []driver.Value{tg.Dataset.ID, tg.Dataset.Ringkas(tg.Perm), now.Add(-time.Hour), now.Add(-time.Hour)})
			}
			return []string{"dataset", "parameter", "sukses", "coba"}, baris, nil
		}
		return nil, nil, nil
	}
	m.cekOtomatis(context.Background(), now)
	if m.Aktif() != nil {
		t.Error("semua tugas masih segar: tidak boleh memulai")
	}

	// Antrean manual sedang berjalan: penjadwal tidak menumpuk.
	f.OnQuery = nil
	f2db, f2 := fakesql.New(t)
	_ = f2
	m2 := NewPenarikInaproc(f2db)
	m2.jalankan = func(ctx context.Context, _ Tugas, _ string, _ func(string)) (HasilSinkron, error) {
		<-lepas
		return HasilSinkron{}, nil
	}
	f2.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "INSERT INTO inaproc_penarikan") {
			return []string{"id"}, [][]driver.Value{{int64(1)}}, nil
		}
		return nil, nil, nil
	}
	if _, err := m2.Start([]Tugas{tugasUji(t, "tender/pengumuman", PermintaanTarik{Tahun: "2025"})}, PemicuManual, Oleh{}, 0); err != nil {
		t.Fatal(err)
	}
	m2.cekOtomatis(context.Background(), now)
	if a := m2.Aktif(); a == nil || a.Total != 1 {
		t.Errorf("antrean manual harus tetap sendirian: %+v", a)
	}
	close(lepas)
	tungguSelesai(t, m2)
}

func TestMuatDanSimpanPengaturan(t *testing.T) {
	m, f := penarikUji(t, sukses(0))
	pasangKonfigInaproc(t, "http://x", "t")

	// Belum tersimpan: bawaan server.
	p, err := m.MuatPengaturan(context.Background())
	if err != nil || !p.BawaanServer {
		t.Fatalf("p=%+v err=%v", p, err)
	}

	// Tersimpan: dibaca dari baris; dataset yang sudah tidak ada atau tidak bisa otomatis dibuang.
	diubah := time.Date(2026, 10, 4, 1, 0, 0, 0, time.UTC)
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "FROM inaproc_penarikan_pengaturan") {
			return []string{"aktif", "interval_hari", "jam_mulai", "jam_akhir", "kode_klpd", "jumlah_tahun", "jeda_detik", "dataset", "diubah", "diubah_oleh"},
				[][]driver.Value{{true, int64(3), int64(22), int64(4), "K10", int64(1), int64(5),
					`["tender/pengumuman","sudah/dihapus","ekatalog/penyedia-detail"]`, diubah, "admin"}}, nil
		}
		return nil, nil, nil
	}
	p, err = m.MuatPengaturan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p.BawaanServer || p.IntervalHari != 3 || p.JamMulai != 22 || p.JumlahTahun != 1 || p.JedaDetik != 5 || p.DiubahOleh != "admin" ||
		strings.Join(p.Dataset, ",") != "tender/pengumuman" {
		t.Errorf("pengaturan = %+v", p)
	}

	// Simpan: UPDATE lalu INSERT bila belum ada, dalam satu perintah; dataset tersimpan sebagai JSON.
	baru := pengaturanUji()
	baru.Dataset = []string{"tender/pengumuman", "rup/paket-penyedia"}
	if pesan := baru.Rapikan(); pesan != "" {
		t.Fatal(pesan)
	}
	if err := m.SimpanPengaturan(context.Background(), baru, "admin"); err != nil {
		t.Fatal(err)
	}
	var tersimpan bool
	for _, ex := range f.Execs() {
		if strings.Contains(ex.Query, "UPDATE inaproc_penarikan_pengaturan") {
			tersimpan = true
			if !strings.Contains(ex.Query, "IF @@ROWCOUNT = 0") || ex.Args[7].Value != `["rup/paket-penyedia","tender/pengumuman"]` || ex.Args[8].Value != "admin" {
				t.Errorf("simpan: query=%s args=%+v", ex.Query, ex.Args)
			}
		}
	}
	if !tersimpan {
		t.Error("pengaturan tidak disimpan")
	}
}
