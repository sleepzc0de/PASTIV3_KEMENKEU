package monitor

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

// Tes integrasi SQL Server: hanya berjalan bila PASTI_UJI_MSSQL_DSN diisi (database uji yang sudah dimigrasi sampai 058). Snapshot uji memakai tanggal tahun 2001 agar tidak
// bercampur dengan riwayat sungguhan, dan dibersihkan sesudahnya.

func bukaDBUji(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PASTI_UJI_MSSQL_DSN")
	if dsn == "" {
		t.Skip("PASTI_UJI_MSSQL_DSN tidak diisi: tes integrasi SQL Server dilewati")
	}
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("tidak bisa terhubung ke database uji: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestStoreSnapshotDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	ctx := context.Background()
	bersih := func() { db.Exec(`DELETE FROM monitor_snapshot WHERE waktu < '2002-01-01'`) }
	bersih()
	if os.Getenv("PASTI_UJI_BIARKAN_DATA") != "1" {
		t.Cleanup(bersih)
	}
	s := &Store{DB: db}

	t0 := time.Date(2001, 3, 4, 10, 0, 0, 0, time.UTC)
	penuh := Snapshot{Waktu: t0, Jendela: 300, CPURata: pf(35.5), CPUMaks: pf(80), Load1Maks: pf(1.25), MemTotal: 8 * gb, MemRata: 3 * gb, MemMaks: 5 * gb, SwapMaks: gb / 2,
		DiskTotal: 100 * gb, DiskTerpakai: 55 * gb, RSSMaks: 150 << 20, HeapMaks: 60 << 20, GoroutineMaks: 123, DBBukaMaks: 7, DBDipakaiMaks: 4, DBBatas: 25, DBTungguDelta: 2,
		DBUkuranMB: pf(5120.5), DBTerpakaiMB: pf(4000.25), ReqTotal: 321, Req4xx: 12, Req5xx: 3, LatRata: pf(45.5), LatP95: pf(210), ReqBerjalanMaks: 6}
	if err := s.Simpan(ctx, penuh); err != nil {
		t.Fatalf("Simpan lengkap: %v", err)
	}
	// Snapshot tanpa data opsional (platform tanpa CPU/memori/disk) tetap bisa disimpan dan dibaca.
	minim := Snapshot{Waktu: t0.Add(5 * time.Minute), Jendela: 300, GoroutineMaks: 50, DBBatas: 25}
	if err := s.Simpan(ctx, minim); err != nil {
		t.Fatalf("Simpan minim: %v", err)
	}

	got, err := s.Baca(ctx, t0.Add(-time.Hour), t0.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("terbaca %d snapshot, want 2", len(got))
	}
	a, b := got[0], got[1]
	if !a.Waktu.Equal(t0) || a.Waktu.Location() != time.UTC || a.Jendela != 300 {
		t.Errorf("waktu = %v jendela %d", a.Waktu, a.Jendela)
	}
	if a.CPURata == nil || *a.CPURata != 35.5 || *a.CPUMaks != 80 || *a.Load1Maks != 1.25 || a.MemTotal != 8*gb || a.MemMaks != 5*gb || a.SwapMaks != gb/2 || a.DiskTerpakai != 55*gb {
		t.Errorf("snapshot lengkap = %+v", a)
	}
	if a.GoroutineMaks != 123 || a.DBDipakaiMaks != 4 || a.DBBatas != 25 || a.DBTungguDelta != 2 || a.DBUkuranMB == nil || *a.DBUkuranMB != 5120.5 || a.ReqTotal != 321 || a.Req5xx != 3 ||
		a.LatP95 == nil || *a.LatP95 != 210 || a.ReqBerjalanMaks != 6 {
		t.Errorf("snapshot lengkap (2) = %+v", a)
	}
	if b.CPURata != nil || b.CPUMaks != nil || b.Load1Maks != nil || b.MemTotal != 0 || b.DiskTotal != 0 || b.DBUkuranMB != nil || b.LatP95 != nil || b.GoroutineMaks != 50 {
		t.Errorf("snapshot minim harus mempertahankan nilai kosong: %+v", b)
	}

	// batas [dari, sampai)
	if got, _ := s.Baca(ctx, t0, t0.Add(5*time.Minute)); len(got) != 1 {
		t.Errorf("rentang setengah terbuka: %d snapshot, want 1", len(got))
	}
	if n, err := s.HapusSebelum(ctx, t0.Add(time.Minute)); err != nil || n != 1 {
		t.Errorf("HapusSebelum = %d %v, want 1", n, err)
	}
	if got, _ := s.Baca(ctx, t0.Add(-time.Hour), t0.Add(time.Hour)); len(got) != 1 {
		t.Errorf("sisa %d, want 1", len(got))
	}
}

func TestInfoSQLDanPoolDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	ctx := context.Background()

	p := poolDari(ctx, "uji", db, true)
	if p == nil || !p.Tersambung || p.PingMS < 0 || p.Batas < 0 || p.Galat != "" { // ping bisa terukur 0 pada timer Windows yang kasar
		t.Errorf("pool = %+v", p)
	}
	if poolDari(ctx, "nil", nil, true) != nil {
		t.Error("pool dari db nil harus nil")
	}

	info := bacaInfoSQL(ctx, db)
	if info == nil || info.Versi == "" || info.Edisi == "" || info.NamaDB == "" {
		t.Fatalf("info = %+v", info)
	}
	if len(info.File) < 2 || info.UkuranMB <= 0 || info.TerpakaiMB == nil || *info.TerpakaiMB <= 0 || *info.TerpakaiMB > info.UkuranMB {
		t.Errorf("file/ukuran = %d file, ukuran %v terpakai %v", len(info.File), info.UkuranMB, info.TerpakaiMB)
	}
	jenis := map[string]bool{}
	for _, f := range info.File {
		jenis[f.Jenis] = true
	}
	if !jenis["ROWS"] || !jenis["LOG"] {
		t.Errorf("harus ada file data dan log: %+v", info.File)
	}
	// Akun uji (sa) punya izin penuh, jadi semua bagian terbaca; pada akun terbatas bagian ini dilewati dan dijelaskan di Catatan.
	if len(info.Catatan) != 0 {
		t.Logf("catatan (izin terbatas?): %v", info.Catatan)
	} else {
		if len(info.Volume) == 0 || info.CPUJumlah == nil || info.MemFisikMB == nil || info.SesiPengguna == nil {
			t.Errorf("tanpa catatan, semua bagian harus terisi: %+v", info)
		}
		if info.PLE == nil {
			t.Errorf("PLE kosong tanpa catatan: %+v", info)
		}
	}

	tabel, err := bacaTabelTeratas(ctx, db, 5)
	if err != nil {
		t.Logf("tabel teratas tidak terbaca (izin?): %v", err)
	} else {
		if len(tabel) == 0 || len(tabel) > 5 {
			t.Fatalf("tabel teratas = %d", len(tabel))
		}
		for i := 1; i < len(tabel); i++ {
			if tabel[i].UkuranMB > tabel[i-1].UkuranMB {
				t.Errorf("tabel tidak urut menurut ukuran: %+v", tabel)
			}
		}
	}
}

func TestManagerRingkasanDanRiwayatDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	ctx := context.Background()
	m := NewManager(func() *sql.DB { return db }, nil, &Store{DB: db}, 30)
	m.TugasLatar = func() []TugasLatar { return []TugasLatar{{Nama: "Uji", Berjalan: true, Info: "x"}} }

	// dua pengukuran supaya CPU punya pembanding
	m.ukurSatu(ctx, time.Now().Add(-30*time.Second))
	time.Sleep(50 * time.Millisecond)
	m.ukurSatu(ctx, time.Now())

	r := m.Ringkasan(ctx)
	if r.Status == "" || len(r.Kapasitas) != 11 || r.Waktu.IsZero() {
		t.Fatalf("ringkasan = status %q kapasitas %d", r.Status, len(r.Kapasitas))
	}
	if r.Database.Utama == nil || !r.Database.Utama.Tersambung || r.Database.SQL == nil || r.Database.SQL.Versi == "" || r.Database.SLDK != nil {
		t.Errorf("database = %+v", r.Database)
	}
	if len(r.HTTP.Jendela) != 3 || r.Proses.Goroutine < 1 || len(r.Tugas) != 1 || !r.Tugas[0].Berjalan || !r.Pengukuran.Aktif || r.Pengukuran.RetensiHari != 30 {
		t.Errorf("http/proses/tugas = %+v / %+v / %+v / %+v", r.HTTP.Jendela, r.Proses.Goroutine, r.Tugas, r.Pengukuran)
	}
	for _, k := range r.Kapasitas {
		switch k.Status {
		case StatusCukup, StatusPerhatian, StatusKritis, StatusTakAdaData:
		default:
			t.Errorf("%s: status tidak dikenal %q", k.Kunci, k.Status)
		}
	}
	kdb := cari(r.Kapasitas, "db_tersambung")
	if kdb.Status != StatusCukup && kdb.Status != StatusPerhatian {
		t.Errorf("database uji seharusnya tersambung: %+v", kdb)
	}

	// grafik 1 jam dari ring di memori
	h, ok := m.Riwayat(ctx, "1j")
	if !ok || len(h.Titik) != 2 {
		t.Errorf("riwayat 1j = ok %v titik %d, want 2", ok, len(h.Titik))
	}
	// grafik dari database: membaca tabel tanpa galat walau kosong
	if h, ok := m.Riwayat(ctx, "24j"); !ok || h.Titik == nil {
		t.Errorf("riwayat 24j = %+v ok=%v", h, ok)
	}
	if _, ok := m.Riwayat(ctx, "tidak-ada"); ok {
		t.Error("rentang tidak dikenal harus ditolak")
	}
	if tbl, galat := m.TabelTeratas(ctx); galat == "" && len(tbl) == 0 {
		t.Error("tabel teratas kosong tanpa galat")
	}
}
