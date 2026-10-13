package serverlog

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

// Tes integrasi SQL Server: hanya berjalan bila PASTI_UJI_MSSQL_DSN diisi (database uji yang sudah dimigrasi sampai 059). Semua baris uji memakai sumber berawalan "UJISL"
// supaya tidak bercampur dengan data lain, dan dibersihkan sesudahnya.

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

func bersihkanUji(db *sql.DB) { db.Exec(`DELETE FROM server_log WHERE sumber LIKE N'UJISL%'`) }

func gab(level Level, sumber, pesan string, n int, waktu time.Time) Gabungan {
	e := siapkan(Entri{Waktu: waktu, Level: level, Sumber: sumber, Pesan: pesan})
	return Gabungan{Entri: e, Sidik: Sidik(level, e.Sumber, e.Rute, e.Pesan), Jumlah: n, Akhir: waktu}
}

func TestStoreDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	bersihkanUji(db)
	if os.Getenv("PASTI_UJI_BIARKAN_DATA") != "1" {
		t.Cleanup(func() { bersihkanUji(db) })
	}
	ctx := context.Background()
	s := &Store{DB: db}
	sekarang := time.Now().UTC().Truncate(time.Second)

	t.Run("menggabung kejadian yang sama dalam jendela satu jam", func(t *testing.T) {
		g := gab(Error, "UJISL GABUNG", "koneksi putus ke 10.0.0.5", 3, sekarang.Add(-30*time.Minute))
		if err := s.Simpan(ctx, []Gabungan{g}); err != nil {
			t.Fatal(err)
		}
		// Kejadian yang sama lagi 10 menit kemudian (angka berbeda, sidik jari sama): menambah hitungan baris yang ada, tidak membuat baris baru.
		g2 := gab(Error, "UJISL GABUNG", "koneksi putus ke 10.0.0.9", 4, sekarang.Add(-10*time.Minute))
		g2.RequestID, g2.UserID = "req-terbaru", "user-terbaru"
		if err := s.Simpan(ctx, []Gabungan{g2}); err != nil {
			t.Fatal(err)
		}
		d, err := s.Cari(ctx, Penyaring{Sumber: "UJISL GABUNG"})
		if err != nil {
			t.Fatal(err)
		}
		if d.Total != 1 || len(d.Baris) != 1 {
			t.Fatalf("harus satu baris: %+v", d)
		}
		b := d.Baris[0]
		if b.Jumlah != 7 {
			t.Fatalf("jumlah harus 3+4=7, dapat %d", b.Jumlah)
		}
		if !b.Waktu.Equal(sekarang.Add(-30*time.Minute)) || !b.WaktuAkhir.Equal(sekarang.Add(-10*time.Minute)) {
			t.Fatalf("waktu pertama/terakhir salah: %v .. %v", b.Waktu, b.WaktuAkhir)
		}
		if b.RequestID != "req-terbaru" || b.UserID != "user-terbaru" {
			t.Fatalf("sampel terbaru harus tersimpan: %+v", b)
		}
		if b.Level != Error || b.Sidik != g.Sidik {
			t.Fatalf("level/sidik: %+v", b)
		}
	})

	t.Run("kejadian lewat jendela satu jam menjadi baris baru", func(t *testing.T) {
		lama := gab(Warn, "UJISL JENDELA", "lambat", 2, sekarang.Add(-5*time.Hour))
		baru := gab(Warn, "UJISL JENDELA", "lambat", 1, sekarang.Add(-1*time.Minute))
		if err := s.Simpan(ctx, []Gabungan{lama}); err != nil {
			t.Fatal(err)
		}
		if err := s.Simpan(ctx, []Gabungan{baru}); err != nil {
			t.Fatal(err)
		}
		d, _ := s.Cari(ctx, Penyaring{Sumber: "UJISL JENDELA"})
		if d.Total != 2 {
			t.Fatalf("dua baris (jam berbeda), dapat %d", d.Total)
		}
	})

	t.Run("dua kelompok pada satu panggilan, rincian dan detail tersimpan", func(t *testing.T) {
		e := gab(Error, "UJISL RINCIAN", "panik: nil pointer", 1, sekarang)
		e.Detail, e.Rute, e.Metode, e.Status, e.DurasiMS = "goroutine 1 [running]:\nmain.go:10", "/api/v1/uji/:id", "GET", 500, 123
		w := gab(Warn, "UJISL RINCIAN", "peringatan lain", 1, sekarang)
		if err := s.Simpan(ctx, []Gabungan{e, w}); err != nil {
			t.Fatal(err)
		}
		d, _ := s.Cari(ctx, Penyaring{Sumber: "UJISL RINCIAN", Level: "error"})
		if d.Total != 1 {
			t.Fatalf("filter level error: %d", d.Total)
		}
		b, err := s.Detail(ctx, d.Baris[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		if b.Detail != e.Detail || b.Rute != "/api/v1/uji/:id" || b.Metode != "GET" || b.Status != 500 || b.DurasiMS != 123 {
			t.Fatalf("rincian tidak lengkap: %+v", b)
		}
		if _, err := s.Detail(ctx, 999999999); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("id tidak ada harus ErrNoRows: %v", err)
		}
	})

	t.Run("teks sangat panjang dan karakter khusus tidak membuat penulisan gagal", func(t *testing.T) {
		g := gab(Error, "UJISL PANJANG", strings.Repeat("é", 5000)+" '; DROP TABLE users;--", 1, sekarang)
		g.Detail = strings.Repeat("x", 30000)
		if err := s.Simpan(ctx, []Gabungan{g}); err != nil {
			t.Fatalf("penulisan teks panjang gagal: %v", err)
		}
		var n int
		db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n) // tabel masih ada
	})

	t.Run("filter pencarian, sumber, rentang waktu, dan halaman", func(t *testing.T) {
		bersihkanUji(db)
		var semua []Gabungan
		for i := 0; i < 7; i++ {
			semua = append(semua, gab(Error, "UJISL FILTER", "galat unik nomor "+strings.Repeat("a", i+1), 1, sekarang.Add(-time.Duration(i)*time.Minute)))
		}
		w := gab(Warn, "UJISL FILTER B", "peringatan 100%_[x]", 1, sekarang)
		w.Rute, w.RequestID = "/api/v1/cari", "req-cari-1"
		semua = append(semua, w)
		if err := s.Simpan(ctx, semua); err != nil {
			t.Fatal(err)
		}
		d, _ := s.Cari(ctx, Penyaring{Sumber: "UJISL FILTER", PerHalaman: 3})
		if d.Total != 7 || len(d.Baris) != 3 || d.PerHalaman != 3 || d.Halaman != 1 {
			t.Fatalf("halaman 1: %+v", d)
		}
		d2, _ := s.Cari(ctx, Penyaring{Sumber: "UJISL FILTER", PerHalaman: 3, Halaman: 3})
		if len(d2.Baris) != 1 {
			t.Fatalf("halaman 3 harus berisi sisa 1: %d", len(d2.Baris))
		}
		// terbaru dulu
		if !d.Baris[0].WaktuAkhir.After(d.Baris[2].WaktuAkhir) {
			t.Fatal("urutan harus terbaru dulu")
		}
		for _, q := range []struct {
			nama string
			p    Penyaring
			mau  int
		}{
			{"kata kunci di pesan", Penyaring{Sumber: "UJISL FILTER B", Q: "100%_[x]"}, 1}, // karakter khusus LIKE dianggap harfiah
			{"kata kunci di rute", Penyaring{Q: "/api/v1/cari", Sumber: "UJISL FILTER B"}, 1},
			{"kata kunci request id", Penyaring{Q: "req-cari-1"}, 1},
			{"tanpa hasil", Penyaring{Sumber: "UJISL FILTER", Q: "tidak-ada-sama-sekali"}, 0},
			{"level warn", Penyaring{Level: "warn", Sumber: "UJISL FILTER B"}, 1},
			{"level warn tanpa sumber itu", Penyaring{Level: "warn", Sumber: "UJISL FILTER"}, 0},
			{"rentang waktu", Penyaring{Sumber: "UJISL FILTER", Dari: sekarang.Add(-150 * time.Second), Sampai: sekarang.Add(time.Second)}, 3},
		} {
			d, err := s.Cari(ctx, q.p)
			if err != nil {
				t.Fatalf("%s: %v", q.nama, err)
			}
			if int(d.Total) != q.mau {
				t.Errorf("%s: total %d, mau %d", q.nama, d.Total, q.mau)
			}
		}
		o, err := s.Opsi(ctx)
		if err != nil {
			t.Fatal(err)
		}
		ada := map[string]bool{}
		for _, v := range o {
			ada[v] = true
		}
		if !ada["UJISL FILTER"] || !ada["UJISL FILTER B"] {
			t.Fatalf("opsi sumber: %v", o)
		}
	})

	t.Run("ringkasan menghitung kejadian per level, per jam, dan sumber", func(t *testing.T) {
		bersihkanUji(db)
		// Hitungan sengaja sangat besar supaya baris ini pasti masuk "paling berulang" dan "sumber teratas" walau DB uji sudah berisi data lain.
		a := gab(Error, "UJISL RINGKAS", "galat a", 1_500_000_000, sekarang.Add(-2*time.Hour))
		b := gab(Error, "UJISL RINGKAS", "galat b", 1, sekarang.Add(-30*time.Minute))
		c := gab(Warn, "UJISL RINGKAS", "peringatan c", 4, sekarang.Add(-10*time.Minute))
		d := gab(Error, "UJISL RINGKAS", "galat lama", 100, sekarang.Add(-3*24*time.Hour)) // di luar 24 jam, di dalam 7 hari
		if err := s.Simpan(ctx, []Gabungan{a, b, c, d}); err != nil {
			t.Fatal(err)
		}
		r, err := s.Ringkasan(ctx, sekarang)
		if err != nil {
			t.Fatal(err)
		}
		// Basis data uji bisa memuat baris lain; yang pasti: sedikitnya angka dari baris uji ini.
		if r.Error24Jam < 1_500_000_001 || r.Warn24Jam < 4 || r.Error7Hari < 1_500_000_101 || r.Kelompok24Jam < 3 {
			t.Fatalf("ringkasan terlalu kecil: %+v", r)
		}
		if len(r.PerJam) != 24 {
			t.Fatalf("grafik per jam harus 24 titik, dapat %d", len(r.PerJam))
		}
		var totE, totW int64
		for i, p := range r.PerJam {
			if i > 0 && !p.Jam.After(r.PerJam[i-1].Jam) {
				t.Fatal("jam harus menaik")
			}
			totE += p.Error
			totW += p.Warn
		}
		if totE < 1_500_000_001 || totW < 4 {
			t.Fatalf("jumlah per jam tidak cocok: error=%d warn=%d", totE, totW)
		}
		if r.Terakhir == nil || r.TotalBaris < 4 {
			t.Fatalf("terakhir/total: %+v", r)
		}
		adaSumber, adaBerulang := false, false
		for _, j := range r.Sumber {
			if j.Sumber == "UJISL RINGKAS" {
				adaSumber = true
			}
		}
		for _, b := range r.PalingBerulang {
			if b.Pesan == "galat a" && b.Jumlah == 1_500_000_000 {
				adaBerulang = true
			}
		}
		if !adaSumber || !adaBerulang {
			t.Fatalf("sumber teratas/paling berulang tidak lengkap: %+v", r)
		}
	})

	t.Run("hitungan baris tidak meluap melewati batas kolom INT", func(t *testing.T) {
		bersihkanUji(db)
		g := gab(Error, "UJISL LUAP", "galat besar", maksJumlahBaris-5, sekarang.Add(-10*time.Minute))
		if err := s.Simpan(ctx, []Gabungan{g}); err != nil {
			t.Fatal(err)
		}
		// Menambah 100 lagi akan melewati 2 miliar: harus dijepit, bukan gagal dengan galat luapan aritmetika.
		if err := s.Simpan(ctx, []Gabungan{gab(Error, "UJISL LUAP", "galat besar", 100, sekarang)}); err != nil {
			t.Fatalf("penulisan yang akan meluap harus dijepit, bukan gagal: %v", err)
		}
		d, _ := s.Cari(ctx, Penyaring{Sumber: "UJISL LUAP"})
		if d.Total != 1 || d.Baris[0].Jumlah != maksJumlahBaris {
			t.Fatalf("hitungan harus dijepit di %d: %+v", maksJumlahBaris, d.Baris)
		}
	})

	t.Run("pembersihan retensi per level dan batas jumlah baris", func(t *testing.T) {
		bersihkanUji(db)
		var gs []Gabungan
		// error: 3 baris lama (50 hari), 2 baru; warn: 3 baris 10 hari, 2 baru
		for i := 0; i < 3; i++ {
			gs = append(gs, gab(Error, "UJISL BERSIH", "error lama "+strings.Repeat("e", i+1), 1, sekarang.Add(-50*24*time.Hour)))
			gs = append(gs, gab(Warn, "UJISL BERSIH", "warn lama "+strings.Repeat("w", i+1), 1, sekarang.Add(-10*24*time.Hour)))
		}
		for i := 0; i < 2; i++ {
			gs = append(gs, gab(Error, "UJISL BERSIH", "error baru "+strings.Repeat("e", i+1), 1, sekarang))
			gs = append(gs, gab(Warn, "UJISL BERSIH", "warn baru "+strings.Repeat("w", i+1), 1, sekarang))
		}
		if err := s.Simpan(ctx, gs); err != nil {
			t.Fatal(err)
		}
		// error disimpan 30 hari, warn 7 hari: error lama (50 hari) dan warn lama (10 hari) dihapus; yang baru tinggal.
		n, err := s.Bersihkan(ctx, Pembersihan{ErrorSebelum: sekarang.Add(-30 * 24 * time.Hour), WarnSebelum: sekarang.Add(-7 * 24 * time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		if n < 6 {
			t.Fatalf("seharusnya sedikitnya 6 baris lama dihapus, dihapus %d", n)
		}
		d, _ := s.Cari(ctx, Penyaring{Sumber: "UJISL BERSIH"})
		if d.Total != 4 {
			t.Fatalf("tinggal 4 baris baru, dapat %d", d.Total)
		}
		for _, b := range d.Baris {
			if !strings.Contains(b.Pesan, "baru") {
				t.Fatalf("yang lama masih ada: %+v", b)
			}
		}
		// Warn lebih singkat retensinya daripada error: warn 10 hari dihapus sedangkan error 10 hari akan bertahan.
		bersihkanUji(db)
		s.Simpan(ctx, []Gabungan{gab(Error, "UJISL BERSIH", "error sepuluh hari", 1, sekarang.Add(-10*24*time.Hour)), gab(Warn, "UJISL BERSIH", "warn sepuluh hari", 1, sekarang.Add(-10*24*time.Hour))})
		s.Bersihkan(ctx, Pembersihan{ErrorSebelum: sekarang.Add(-30 * 24 * time.Hour), WarnSebelum: sekarang.Add(-7 * 24 * time.Hour)})
		d, _ = s.Cari(ctx, Penyaring{Sumber: "UJISL BERSIH"})
		if d.Total != 1 || d.Baris[0].Level != Error {
			t.Fatalf("hanya error 10 hari yang bertahan: %+v", d)
		}

		// Batas jumlah baris: yang tertua dihapus lebih dulu. Hitung baris non-uji dulu supaya batasnya tepat.
		bersihkanUji(db)
		var lain int
		db.QueryRow(`SELECT COUNT(*) FROM server_log`).Scan(&lain)
		var banyak []Gabungan
		for i := 0; i < 10; i++ {
			banyak = append(banyak, gab(Error, "UJISL BATAS", "baris ke "+strings.Repeat("z", i+1), 1, sekarang.Add(-time.Duration(10-i)*time.Minute)))
		}
		s.Simpan(ctx, banyak)
		if _, err := s.Bersihkan(ctx, Pembersihan{MaksBaris: lain + 4}); err != nil {
			t.Fatal(err)
		}
		var sisa int
		db.QueryRow(`SELECT COUNT(*) FROM server_log`).Scan(&sisa)
		if sisa != lain+4 {
			t.Fatalf("setelah dibatasi harus tersisa %d baris, tersisa %d", lain+4, sisa)
		}
	})
}

// Menulis banyak kelompok sekaligus lewat Perekam sungguhan ke SQL Server.
func TestPerekamKeSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	bersihkanUji(db)
	t.Cleanup(func() { bersihkanUji(db) })
	st := &Store{DB: db}
	r := NewPerekam(st.Simpan, 5000)
	r.interval = 20 * time.Millisecond
	r.Mulai()
	for i := 0; i < 3000; i++ {
		r.Catat(siapkan(Entri{Waktu: time.Now(), Level: Error, Sumber: "UJISL REKAM", Pesan: "gagal membaca baris " + string(rune('a'+i%5))}))
	}
	r.Catat(siapkan(Entri{Waktu: time.Now(), Level: Warn, Sumber: "UJISL REKAM", Pesan: "peringatan"}))
	r.Berhenti(context.Background())
	var jumlah, baris int
	db.QueryRow(`SELECT ISNULL(SUM(jumlah),0), COUNT(*) FROM server_log WHERE sumber = N'UJISL REKAM'`).Scan(&jumlah, &baris)
	if jumlah != 3001 {
		t.Fatalf("total kejadian tersimpan %d, mau 3001 (statistik %+v)", jumlah, r.Statistik())
	}
	if baris > 20 {
		t.Fatalf("3001 kejadian dengan 6 jenis pesan seharusnya menjadi sedikit baris, ada %d", baris)
	}
}
