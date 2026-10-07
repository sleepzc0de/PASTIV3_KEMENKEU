package audit

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

// Tes integrasi SQL Server: hanya berjalan bila PASTI_UJI_MSSQL_DSN diisi (database uji yang sudah dimigrasi). Semua entri uji memakai tanggal tahun 2001 dan nama
// pengguna berawalan "ujiaudit_" supaya tidak bercampur dengan data lain, dan dibersihkan sesudahnya.

const (
	penandaUji = "ujiaudit_"
	idUjiA     = "0A0A0A0A-0000-4000-8000-0000000000A1"
	idUjiB     = "0A0A0A0A-0000-4000-8000-0000000000B2"
)

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

func bersihkanUji(db *sql.DB) {
	db.Exec(`DELETE FROM audit_log WHERE username LIKE N'` + penandaUji + `%'`)
	db.Exec(`DELETE FROM users WHERE username LIKE N'` + penandaUji + `%'`)
}

func TestStoreAuditDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	bersihkanUji(db)
	if os.Getenv("PASTI_UJI_BIARKAN_DATA") != "1" {
		t.Cleanup(func() { bersihkanUji(db) })
	}
	ctx := context.Background()
	s := &Store{DB: db}

	// Akun uji agar nama lengkap dan status aktif ikut terbaca.
	if _, err := db.Exec(`INSERT INTO users (id, username, email, full_name, role, is_active) VALUES (@p1, @p2, @p3, N'Budi Uji', 'user', 1)`, idUjiA, penandaUji+"budi", penandaUji+"budi@uji.test"); err != nil {
		t.Fatal(err)
	}

	hari := func(d, jam int) time.Time { return time.Date(2001, 2, d, jam, 0, 0, 0, time.UTC) }
	ya, tidak := true, false
	es := []Entri{
		{Waktu: hari(3, 8), UserID: idUjiA, Username: penandaUji + "budi", Peran: "satker", KodePeran: "409294", Kategori: KatSapa, Aksi: "sapa.usulan.buat", Label: "Membuat usulan penjualan",
			Metode: "POST", Rute: "/api/v1/sapa/penjualan", Status: 201, Sukses: true, DurasiMS: 12, IP: "10.1.1.1", UserAgent: "UA", RequestID: "r1"},
		{Waktu: hari(3, 9), UserID: idUjiA, Username: penandaUji + "budi", Peran: "satker", Kategori: KatAuth, Aksi: AksiLoginBerhasil, Label: "Login dengan kata sandi",
			Metode: "POST", Rute: "/api/v1/auth/login", Status: 200, Sukses: true, IP: "10.1.1.1", Detail: map[string]interface{}{"metode": "password"}},
		{Waktu: hari(3, 10), Username: penandaUji + "tamu", Kategori: KatAuth, Aksi: AksiLoginGagal, Label: "Login gagal", Metode: "POST", Rute: "/api/v1/auth/login", Status: 401, Sukses: false,
			IP: "203.0.113.9", Detail: map[string]interface{}{"alasan": "kata_sandi_salah"}},
		{Waktu: hari(3, 11), Username: penandaUji + "tamu", Kategori: KatAuth, Aksi: AksiLoginGagal, Label: "Login gagal", Metode: "POST", Rute: "/api/v1/auth/login", Status: 401, Sukses: false, IP: "203.0.113.9"},
		{Waktu: hari(4, 8), UserID: idUjiB, Username: penandaUji + "sari", Peran: "pengguna_barang", Kategori: KatEkspor, Aksi: "ekspor.digitalisasi", Label: "Mengekspor data aset (tanah)",
			ObjekTipe: "dataset", ObjekID: "tanah", Metode: "GET", Rute: "/api/v1/digitalisasi/ekspor/:dataset", Status: 200, Sukses: true, IP: "10.2.2.2", Detail: map[string]interface{}{"kueri": "format=xlsx"}},
		{Waktu: hari(4, 9), UserID: idUjiA, Username: penandaUji + "budi", Peran: "satker", Kategori: KatPengguna, Aksi: AksiDitolak, Label: "Akses ditolak: Menghapus pengguna 9",
			ObjekTipe: "pengguna", ObjekID: "9", Metode: "DELETE", Rute: "/api/v1/users/:id", Status: 403, Sukses: false, IP: "10.1.1.1"},
		// teks yang melebihi lebar kolom dipotong, bukan membuat penyimpanan gagal; user_id yang bukan GUID menjadi NULL
		{Waktu: hari(5, 7), UserID: "bukan-guid", Username: penandaUji + strings.Repeat("x", 300), Kategori: KatLainnya, Aksi: "uji.panjang", Label: strings.Repeat("L", 500),
			Metode: "POST", Rute: "/api/v1/" + strings.Repeat("r", 400), Status: 200, Sukses: true, UserAgent: strings.Repeat("u", 600), IP: strings.Repeat("9", 100)},
	}
	if err := s.Simpan(ctx, es); err != nil {
		t.Fatalf("Simpan: %v", err)
	}
	if err := s.Simpan(ctx, nil); err != nil {
		t.Errorf("Simpan kosong: %v", err)
	}

	dari, sampai := hari(1, 0), hari(28, 0)
	base := Penyaring{Dari: dari, Sampai: sampai, Username: penandaUji}
	cari := func(p Penyaring) *Daftar {
		t.Helper()
		d, err := s.Cari(ctx, p)
		if err != nil {
			t.Fatalf("Cari(%+v): %v", p, err)
		}
		return d
	}

	// semua, terbaru lebih dulu
	d := cari(base)
	if d.Total != 7 || len(d.Entri) != 7 || d.Halaman != 1 || d.PerHalaman != 50 {
		t.Fatalf("semua: total=%d entri=%d halaman=%d/%d", d.Total, len(d.Entri), d.Halaman, d.PerHalaman)
	}
	if d.Entri[0].Aksi != "uji.panjang" || d.Entri[6].Aksi != "sapa.usulan.buat" {
		t.Errorf("urutan: %s ... %s", d.Entri[0].Aksi, d.Entri[6].Aksi)
	}
	// pemotongan dan NULL
	p := d.Entri[0]
	if len(p.Username) > 100 || len([]rune(p.Label)) != 300 || len(p.Rute) > 250 || len(p.UserAgent) != 300 || len(p.IP) != 64 || p.UserID != "" {
		t.Errorf("entri panjang tidak dipotong: user=%d label=%d rute=%d ua=%d ip=%d uid=%q", len(p.Username), len([]rune(p.Label)), len(p.Rute), len(p.UserAgent), len(p.IP), p.UserID)
	}
	// isi yang kembali utuh: detail, nama lengkap dari users, user_id huruf kecil, waktu UTC
	var login Entri
	for _, e := range d.Entri {
		if e.Aksi == AksiLoginBerhasil {
			login = e
		}
	}
	if login.Detail["metode"] != "password" || login.NamaLengkap != "Budi Uji" || login.UserID != strings.ToLower(idUjiA) || login.Peran != "satker" || !login.Waktu.Equal(hari(3, 9)) || login.Waktu.Location() != time.UTC {
		t.Errorf("login = %+v", login)
	}

	// setiap filter
	satu := func(nama string, p Penyaring, want int64) {
		t.Helper()
		p.Dari, p.Sampai = dari, sampai
		if got := cari(p).Total; got != want {
			t.Errorf("filter %s: total=%d, want %d", nama, got, want)
		}
	}
	satu("kategori auth", Penyaring{Username: penandaUji, Kategori: KatAuth}, 3)
	satu("aksi login gagal", Penyaring{Username: penandaUji, Aksi: AksiLoginGagal}, 2)
	satu("gagal", Penyaring{Username: penandaUji, Sukses: &tidak}, 3)
	satu("berhasil", Penyaring{Username: penandaUji, Sukses: &ya}, 4)
	satu("user id", Penyaring{UserID: idUjiA}, 3)
	satu("user id bukan guid", Penyaring{UserID: "bukan-guid"}, 0)
	satu("username", Penyaring{Username: "ujiaudit_sa"}, 1)
	satu("ip", Penyaring{Username: penandaUji, IP: "203.0.113"}, 2)
	satu("q label", Penyaring{Username: penandaUji, Q: "usulan penjualan"}, 1)
	satu("q rute", Penyaring{Username: penandaUji, Q: "ekspor/:dataset"}, 1)
	satu("q objek", Penyaring{Username: penandaUji, Q: "tanah"}, 1)
	satu("q detail", Penyaring{Username: penandaUji, Q: "kata_sandi_salah"}, 1)
	satu("q dengan karakter khusus", Penyaring{Username: penandaUji, Q: "100%_[x]'; DROP TABLE audit_log;--"}, 0)
	d = cari(Penyaring{Dari: hari(4, 0), Sampai: hari(5, 0), Username: penandaUji})
	if d.Total != 2 {
		t.Errorf("rentang waktu: total=%d, want 2", d.Total)
	}
	d = cari(Penyaring{Dari: hari(3, 9), Sampai: hari(3, 10), Username: penandaUji}) // [dari, sampai): hanya jam 9
	if d.Total != 1 || d.Entri[0].Aksi != AksiLoginBerhasil {
		t.Errorf("batas rentang: %+v", d.Total)
	}

	// halaman
	h1 := cari(Penyaring{Dari: dari, Sampai: sampai, Username: penandaUji, Halaman: 1, PerHalaman: 3})
	h3 := cari(Penyaring{Dari: dari, Sampai: sampai, Username: penandaUji, Halaman: 3, PerHalaman: 3})
	h9 := cari(Penyaring{Dari: dari, Sampai: sampai, Username: penandaUji, Halaman: 9, PerHalaman: 3})
	if len(h1.Entri) != 3 || len(h3.Entri) != 1 || len(h9.Entri) != 0 || h1.Total != 7 || h9.Halaman != 9 {
		t.Errorf("halaman: %d %d %d total=%d", len(h1.Entri), len(h3.Entri), len(h9.Entri), h1.Total)
	}
	if h1.Entri[0].ID == h3.Entri[0].ID {
		t.Error("halaman 1 dan 3 memuat entri yang sama")
	}

	// alir (ekspor)
	var terbaca []string
	if err := s.Alir(ctx, base, 5, func(e Entri) error { terbaca = append(terbaca, e.Aksi); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(terbaca) != 5 {
		t.Errorf("Alir maks 5: %d entri", len(terbaca))
	}

	// ringkasan
	r, err := s.Ringkas(ctx, dari, sampai)
	if err != nil {
		t.Fatalf("Ringkas: %v", err)
	}
	if r.Total < 7 || r.LoginGagal < 2 || r.LoginBerhasil < 1 || r.Ditolak < 1 || r.Ekspor < 1 || r.PenggunaAktif < 2 || r.Gagal < 3 || r.Berhasil < 4 || r.PerJam {
		t.Errorf("ringkasan = %+v", r)
	}
	if len(r.PerKategori) == 0 || len(r.AksiTeratas) == 0 || len(r.Deret) < 3 {
		t.Errorf("ringkasan kosong: %+v", r)
	}
	var adaIP bool
	for _, j := range r.LoginGagalIP {
		if j.Kode == "203.0.113.9" && j.Jumlah >= 2 {
			adaIP = true
		}
	}
	if !adaIP {
		t.Errorf("login gagal per IP: %+v", r.LoginGagalIP)
	}
	rj, err := s.Ringkas(ctx, hari(3, 0), hari(4, 0)) // satu hari: deret per jam
	if err != nil || !rj.PerJam || len(rj.Deret) < 4 {
		t.Errorf("ringkasan per jam: %+v err=%v", rj, err)
	}

	// per pengguna
	pp, err := s.PerPengguna(ctx, dari, sampai, penandaUji, 1, 10)
	if err != nil {
		t.Fatalf("PerPengguna: %v", err)
	}
	if pp.Total != 2 || len(pp.Pengguna) != 2 {
		t.Fatalf("per pengguna: total=%d n=%d", pp.Total, len(pp.Pengguna))
	}
	// Sari terakhir aktif 4 Feb jam 8; Budi 4 Feb jam 9 → Budi lebih dulu
	b, sr := pp.Pengguna[0], pp.Pengguna[1]
	if b.Username != penandaUji+"budi" || b.Jumlah != 3 || b.Gagal != 1 || b.NamaLengkap != "Budi Uji" || b.Aktif == nil || !*b.Aktif || b.IPTerakhir != "10.1.1.1" ||
		b.LoginTerakhir == nil || !b.LoginTerakhir.Equal(hari(3, 9)) || !b.AktifTerakhir.Equal(hari(4, 9)) || b.Email == "" {
		t.Errorf("budi = %+v", b)
	}
	if sr.Username != penandaUji+"sari" || sr.Jumlah != 1 || sr.Aktif != nil || sr.LoginTerakhir != nil || sr.PeranTerakhir != "pengguna_barang" {
		t.Errorf("sari (akun tidak ada di users) = %+v", sr)
	}
	if pp, _ := s.PerPengguna(ctx, dari, sampai, "tidak-ada-ujiaudit", 1, 10); pp == nil || pp.Total != 0 || len(pp.Pengguna) != 0 {
		t.Errorf("per pengguna tanpa hasil: %+v", pp)
	}

	// retensi: hapus yang lebih lama dari batas, yang lebih baru tetap
	n, err := s.HapusSebelum(ctx, hari(4, 0))
	if err != nil {
		t.Fatal(err)
	}
	if n < 4 {
		t.Errorf("dihapus %d, want >= 4", n)
	}
	if sisa := cari(base).Total; sisa != 3 {
		t.Errorf("sisa = %d, want 3 (entri 4 dan 5 Februari)", sisa)
	}
}
