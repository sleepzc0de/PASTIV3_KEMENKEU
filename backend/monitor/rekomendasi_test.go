package monitor

import (
	"strings"
	"testing"

	"pasti-v3-backend/audit"
)

const gb = uint64(1) << 30

func u64(v uint64) *uint64 { return &v }
func ip(v int) *int        { return &v }

// sehat: keadaan awal yang semuanya cukup; tiap tes mengubah satu bagian.
func sehat() Masukan {
	return Masukan{
		Sistem: Sistem{CPUJumlah: 4, CPUPersen: pf(20), MemTotal: 8 * gb, MemTerpakai: 3 * gb, MemTersedia: 5 * gb, MemPersen: 37.5, SwapTotal: 2 * gb, SwapTerpakai: 0,
			DiskJalur: "/", DiskTotal: 100 * gb, DiskTerpakai: 40 * gb, DiskBebas: 60 * gb, DiskPersen: 40},
		Proses: Proses{Goroutine: 80, HeapDipakai: 50 << 20, RSS: u64(120 << 20)},
		DB: Database{Utama: &PoolDB{Nama: "Database aplikasi", Tersambung: true, PingMS: 3, Batas: 25, Terbuka: 4, Dipakai: 2, Menganggur: 2},
			SQL: &InfoSQL{UkuranMB: 5000, TerpakaiMB: pf(4000), Volume: []VolumeDB{{Titik: "D:\\", Total: 500 * gb, Bebas: 400 * gb, Persen: 20}}, PLE: pf(8000), BufferHitRatio: pf(99.5)}},
		JamTerakhir: JendelaHTTP{Total: 500, P95MS: 120, RataMS: 40, P50MS: 30, P99MS: 300, Galat5xx: 0},
		H24: &Agregat{Jumlah: 288, CPURata: pf(18), CPUMaks: pf(60), Load1Maks: pf(0.8), MemPersenMaks: pf(45), DiskPersenMaks: pf(40), GoroutineMaks: ip(120), DBDipakaiMaks: ip(6), DBBatas: ip(25),
			LatP95Maks: pf(300)},
		H7: &Agregat{Jumlah: 2016, CPUMaks: pf(75), MemPersenMaks: pf(50), DiskPersenMaks: pf(40), GoroutineMaks: ip(150), DBDipakaiMaks: ip(8)},
	}
}

func cari(ks []Kapasitas, kunci string) Kapasitas {
	for _, k := range ks {
		if k.Kunci == kunci {
			return k
		}
	}
	return Kapasitas{}
}

func harap(t *testing.T, m Masukan, kunci, status string) Kapasitas {
	t.Helper()
	k := cari(Evaluasi(m), kunci)
	if k.Kunci == "" {
		t.Fatalf("kapasitas %q tidak ada", kunci)
	}
	if k.Status != status {
		t.Errorf("%s: status = %q, want %q\n%s", kunci, k.Status, status, strings.Join(k.Rincian, "\n"))
	}
	if k.Tindakan == "" || len(k.Rincian) == 0 {
		t.Errorf("%s: tindakan atau rincian kosong: %+v", kunci, k)
	}
	return k
}

func TestEvaluasiKeadaanSehatSemuaCukup(t *testing.T) {
	ks := Evaluasi(sehat())
	if len(ks) != 11 {
		t.Fatalf("jumlah penilaian = %d, want 11", len(ks))
	}
	for _, k := range ks {
		if k.Status != StatusCukup {
			t.Errorf("%s: status = %q, want cukup\n%s", k.Kunci, k.Status, strings.Join(k.Rincian, "\n"))
		}
		if k.Kelompok != KelompokResource && k.Kelompok != KelompokKinerja {
			t.Errorf("%s: kelompok = %q", k.Kunci, k.Kelompok)
		}
	}
	status, tambah := Gabungkan(ks)
	if status != StatusCukup || len(tambah) != 0 {
		t.Errorf("gabungan = %s %v", status, tambah)
	}
	if k := cari(ks, "cpu"); k.Tindakan != "Tidak perlu ditambah" {
		t.Errorf("tindakan = %q", k.Tindakan)
	}
}

func TestEvaluasiCPU(t *testing.T) {
	for _, c := range []struct {
		nama   string
		ubah   func(*Masukan)
		status string
	}{
		{"rata-rata 24 jam tinggi", func(m *Masukan) { m.H24.CPURata = pf(85) }, StatusKritis},
		{"rata-rata 24 jam sedang", func(m *Masukan) { m.H24.CPURata = pf(65) }, StatusPerhatian},
		{"puncak 98% dengan rata-rata 40%", func(m *Masukan) { m.H24.CPURata, m.H24.CPUMaks = pf(40), pf(98) }, StatusPerhatian},
		{"puncak 98% tetapi rata-rata rendah (lonjakan sesaat)", func(m *Masukan) { m.H24.CPURata, m.H24.CPUMaks = pf(10), pf(98) }, StatusCukup},
		{"beban 4 per core", func(m *Masukan) { m.H24.Load1Maks = pf(16) }, StatusKritis},
		{"beban 2 per core", func(m *Masukan) { m.H24.Load1Maks = pf(8) }, StatusPerhatian},
		{"tanpa riwayat memakai nilai sekarang", func(m *Masukan) { m.H24, m.H7 = nil, nil; m.Sistem.CPUPersen = pf(90) }, StatusKritis},
		{"tanpa data sama sekali", func(m *Masukan) { m.H24, m.H7 = nil, nil; m.Sistem.CPUPersen = nil }, StatusTakAdaData},
	} {
		m := sehat()
		c.ubah(&m)
		k := cari(Evaluasi(m), "cpu")
		if k.Status != c.status {
			t.Errorf("%s: status = %q, want %q\n%s", c.nama, k.Status, c.status, strings.Join(k.Rincian, "\n"))
		}
	}
	m := sehat()
	m.H24.CPURata = pf(90)
	if k := cari(Evaluasi(m), "cpu"); !strings.HasPrefix(k.Tindakan, "Perlu ditambah: vCPU") || !strings.Contains(k.Tindakan, "4 core") {
		t.Errorf("tindakan = %q", k.Tindakan)
	}
}

func TestEvaluasiMemori(t *testing.T) {
	m := sehat()
	m.H24.MemPersenMaks = pf(95)
	if k := harap(t, m, "memori", StatusKritis); !strings.Contains(k.Tindakan, "RAM") || !strings.Contains(k.Tindakan, "8,0 GB") {
		t.Errorf("tindakan = %q", k.Tindakan)
	}
	m = sehat()
	m.H24.MemPersenMaks = pf(88)
	harap(t, m, "memori", StatusPerhatian)

	// swap aktif saat memori hampir penuh = kuat memori kurang
	m = sehat()
	m.H24.MemPersenMaks, m.Sistem.SwapTerpakai = pf(82), gb
	k := harap(t, m, "memori", StatusKritis)
	if !strings.Contains(strings.Join(k.Rincian, " "), "swap") {
		t.Errorf("rincian harus menyebut swap: %v", k.Rincian)
	}
	// swap terpakai tetapi memori longgar: tidak menaikkan status
	m = sehat()
	m.Sistem.SwapTerpakai = gb
	harap(t, m, "memori", StatusCukup)

	m = sehat()
	m.Sistem.MemTotal, m.H24, m.H7 = 0, nil, nil
	harap(t, m, "memori", StatusTakAdaData)
}

func TestEvaluasiDisk(t *testing.T) {
	m := sehat()
	m.Sistem.DiskPersen = 92
	harap(t, m, "disk", StatusKritis)
	m.Sistem.DiskPersen = 85
	harap(t, m, "disk", StatusPerhatian)

	// masih longgar tetapi tumbuh cepat: penuh dalam 5 hari -> kritis; 20 hari -> perhatian; tumbuh lambat -> cukup
	m = sehat() // sisa 60 GB
	laju := float64(12 * gb)
	m.LajuDiskHari = &laju // 5 hari
	k := harap(t, m, "disk", StatusKritis)
	if !strings.Contains(strings.Join(k.Rincian, " "), "penuh dalam kira-kira 5 hari") {
		t.Errorf("rincian = %v", k.Rincian)
	}
	laju = float64(3 * gb) // 20 hari
	harap(t, m, "disk", StatusPerhatian)
	laju = float64(gb) / 10 // 600 hari
	harap(t, m, "disk", StatusCukup)
	laju = -float64(gb) // menyusut
	harap(t, m, "disk", StatusCukup)

	m = sehat()
	m.Sistem.DiskTotal = 0
	harap(t, m, "disk", StatusTakAdaData)
}

func TestEvaluasiPenyimpananDatabase(t *testing.T) {
	m := sehat()
	m.DB.SQL.Volume = []VolumeDB{{Titik: "D:\\", Total: 500 * gb, Bebas: 25 * gb, Persen: 95}}
	harap(t, m, "db_penyimpanan", StatusKritis)

	// volume longgar, tetapi database tumbuh 50 GB per hari sedangkan sisa 40 GB -> kritis
	m = sehat()
	m.DB.SQL.Volume = []VolumeDB{{Titik: "D:\\", Total: 500 * gb, Bebas: 40 * gb, Persen: 55}}
	laju := 50.0 * 1024
	m.LajuDBHari = &laju
	harap(t, m, "db_penyimpanan", StatusKritis)

	// tanpa info volume: dinilai terhadap batas ukuran file, atau dinyatakan tidak dapat dinilai bila tidak dibatasi
	m = sehat()
	m.DB.SQL.Volume = nil
	k := harap(t, m, "db_penyimpanan", StatusCukup)
	if !strings.Contains(strings.Join(k.Rincian, " "), "tidak dapat dibaca") {
		t.Errorf("harus menjelaskan mengapa sisa ruang tidak dinilai: %v", k.Rincian)
	}
	m.DB.SQL.File = []FileDB{{Nama: "data", Jenis: "ROWS", UkuranMB: 9500, MaksMB: pf(10000)}}
	harap(t, m, "db_penyimpanan", StatusKritis)

	m = sehat()
	m.DB.SQL = nil
	harap(t, m, "db_penyimpanan", StatusTakAdaData)
}

func TestEvaluasiKoneksiDatabase(t *testing.T) {
	m := sehat()
	m.H24.DBDipakaiMaks, m.H24.DBTungguTotal = ip(24), 7
	k := harap(t, m, "db_koneksi", StatusKritis)
	if !strings.Contains(k.Tindakan, "SetMaxOpenConns") || !strings.Contains(k.Tindakan, "25") {
		t.Errorf("tindakan = %q", k.Tindakan)
	}
	m = sehat()
	m.H24.DBDipakaiMaks = ip(20) // 80% dari 25, tidak ada yang menunggu
	harap(t, m, "db_koneksi", StatusPerhatian)
	m = sehat()
	m.H24.DBTungguTotal = 3 // sempat menunggu walau puncaknya rendah
	harap(t, m, "db_koneksi", StatusPerhatian)
	m = sehat()
	m.DB.Utama.Batas = 0
	harap(t, m, "db_koneksi", StatusTakAdaData)
}

func TestEvaluasiMemoriSQLServer(t *testing.T) {
	m := sehat()
	m.DB.SQL.PLE = pf(50)
	harap(t, m, "db_memori", StatusKritis)
	m.DB.SQL.PLE = pf(200)
	harap(t, m, "db_memori", StatusPerhatian)
	m = sehat()
	m.DB.SQL.BufferHitRatio = pf(85)
	harap(t, m, "db_memori", StatusPerhatian)
	m = sehat()
	m.DB.SQL.PLE, m.DB.SQL.BufferHitRatio = nil, nil
	k := harap(t, m, "db_memori", StatusTakAdaData)
	if !strings.Contains(k.Rincian[0], "VIEW SERVER STATE") {
		t.Errorf("harus menyebut izin yang dibutuhkan: %v", k.Rincian)
	}
}

func TestEvaluasiResponsDanGalat(t *testing.T) {
	m := sehat()
	m.JamTerakhir.P95MS = 4000
	k := harap(t, m, "respons", StatusKritis)
	if !strings.HasPrefix(k.Tindakan, "Lambat:") {
		t.Errorf("respons lambat bukan otomatis 'tambah resource': %q", k.Tindakan)
	}
	m.JamTerakhir.P95MS = 1500
	harap(t, m, "respons", StatusPerhatian)
	m = sehat()
	m.JamTerakhir.Total = 10 // terlalu sedikit untuk dinilai
	harap(t, m, "respons", StatusTakAdaData)
	harap(t, m, "galat", StatusTakAdaData)

	m = sehat()
	m.JamTerakhir.Galat5xx, m.JamTerakhir.PersenGalat = 100, 20
	if k := harap(t, m, "galat", StatusKritis); !strings.HasPrefix(k.Tindakan, "Periksa segera") {
		t.Errorf("tindakan = %q", k.Tindakan)
	}
	m.JamTerakhir.PersenGalat = 2
	harap(t, m, "galat", StatusPerhatian)
	m.JamTerakhir.PersenGalat = 0.2
	harap(t, m, "galat", StatusCukup)
}

func TestEvaluasiProsesAplikasi(t *testing.T) {
	m := sehat()
	m.Proses.Goroutine = 60000
	harap(t, m, "proses", StatusKritis)
	m.Proses.Goroutine = 20000
	harap(t, m, "proses", StatusPerhatian)

	// memori proses dibandingkan dengan batas container bila ada, selain itu dengan memori server
	m = sehat()
	batas := 128 * uint64(1<<20)
	m.Sistem.Kontainer = &Kontainer{MemBatas: &batas}
	m.Proses.RSS = u64(125 << 20)
	k := harap(t, m, "proses", StatusKritis)
	if !strings.Contains(strings.Join(k.Rincian, " "), "batas memori container") {
		t.Errorf("rincian = %v", k.Rincian)
	}
	m = sehat()
	m.Proses.RSS = u64(7 * gb)
	harap(t, m, "proses", StatusPerhatian)
	// puncak 24 jam ikut dinilai
	m = sehat()
	m.H24.RSSMaks = u64(6500 << 20) // 79% dari 8 GB
	harap(t, m, "proses", StatusPerhatian)
	m = sehat()
	m.Proses.RSS = nil
	harap(t, m, "proses", StatusCukup)
}

func TestEvaluasiKetersambunganDanAudit(t *testing.T) {
	m := sehat()
	m.DB.Utama.Tersambung, m.DB.Utama.Galat = false, "connection refused"
	k := harap(t, m, "db_tersambung", StatusKritis)
	if !strings.Contains(k.Rincian[0], "connection refused") {
		t.Errorf("rincian = %v", k.Rincian)
	}
	m = sehat()
	m.DB.Utama.PingMS = 300
	harap(t, m, "db_tersambung", StatusPerhatian)
	m = sehat()
	m.DB.SLDK = &PoolDB{Nama: "Database SLDK", Tersambung: false, Galat: "timeout"}
	harap(t, m, "db_tersambung", StatusPerhatian)
	m.DB.SLDK = &PoolDB{Nama: "Database SLDK", Tersambung: true, PingMS: 12}
	harap(t, m, "db_tersambung", StatusCukup)

	m = sehat()
	m.Audit = audit.Statistik{Diterima: 100, Ditulis: 95, Dijatuhkan: 3, Gagal: 2}
	k = harap(t, m, "audit", StatusPerhatian)
	if !strings.Contains(strings.Join(k.Rincian, " "), "5 entri audit tidak tercatat") {
		t.Errorf("rincian = %v", k.Rincian)
	}
}

func TestGabungkanHanyaResourceKritisYangPerluDitambah(t *testing.T) {
	m := sehat()
	m.H24.CPURata = pf(90)       // resource kritis
	m.H24.MemPersenMaks = pf(88) // resource perhatian
	m.Proses.Goroutine = 60000   // kinerja kritis: bukan "perlu ditambah"
	status, tambah := Gabungkan(Evaluasi(m))
	if status != StatusKritis {
		t.Errorf("status = %q", status)
	}
	if len(tambah) != 1 || tambah[0] != "CPU server" {
		t.Errorf("perlu ditambah = %v, want hanya CPU server", tambah)
	}
	if s, _ := Gabungkan(nil); s != StatusTakAdaData {
		t.Errorf("tanpa penilaian = %q", s)
	}
}

func TestFormatAngkaDanUkuran(t *testing.T) {
	for _, c := range []struct {
		v    float64
		d    int
		want string
	}{{0, 0, "0"}, {12.345, 1, "12,3"}, {1234567.891, 2, "1.234.567,89"}, {999, 0, "999"}, {1000, 0, "1.000"}, {-4321.5, 1, "-4.321,5"}} {
		if got := fmtAngka(c.v, c.d); got != c.want {
			t.Errorf("fmtAngka(%v,%d) = %q, want %q", c.v, c.d, got, c.want)
		}
	}
	for _, c := range []struct {
		b    uint64
		want string
	}{{0, "0 B"}, {512, "512 B"}, {1536, "1,5 KB"}, {5 << 20, "5,0 MB"}, {3 * gb, "3,0 GB"}, {uint64(2.5 * float64(gb) * 1024), "2,5 TB"}} {
		if got := FmtBytes(c.b); got != c.want {
			t.Errorf("FmtBytes(%d) = %q, want %q", c.b, got, c.want)
		}
	}
}
