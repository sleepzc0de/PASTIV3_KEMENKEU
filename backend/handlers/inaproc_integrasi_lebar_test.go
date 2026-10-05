package handlers

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// Tes integrasi (SQL Server sungguhan, lihat inaproc_integrasi_mssql_test.go) untuk lebar kolom teks: query skema dan migrasi 047.

// Pengumuman dengan banyak anggota pokja harus tersimpan utuh di skema migrasi yang asli (nip_pokja dan nama_pokja NVARCHAR(MAX) sejak 047).
func TestPengumumanDenganPokjaBanyakDenganSQLServer(t *testing.T) {
	db := dbIntegrasi(t)
	bersihkanUji(t, db)
	t.Cleanup(func() { bersihkanUji(t, db) })

	var lebarNip, lebarNama int
	if err := db.QueryRow("SELECT COL_LENGTH('inaproc_tender_pengumuman', 'nip_pokja'), COL_LENGTH('inaproc_tender_pengumuman', 'nama_pokja')").Scan(&lebarNip, &lebarNama); err != nil {
		t.Fatal(err)
	}
	if lebarNip != -1 || lebarNama != -1 {
		t.Fatalf("COL_LENGTH nip_pokja=%d nama_pokja=%d, want -1 (NVARCHAR(MAX)): terapkan migrasi 047 ke database uji", lebarNip, lebarNama)
	}

	nip := strings.TrimSuffix(strings.Repeat("198505152010011001, ", 20), ", ") // 20 anggota pokja, 398 karakter
	nama := strings.TrimSuffix(strings.Repeat("Nama Anggota Pokja, ", 30), ", ") // melebihi 255
	baris := strings.Replace(contohTenderPengumuman, `"kd_klpd": "KX"`, `"kd_klpd": "`+klpdUji+`"`, 1)
	baris = strings.Replace(baris, `"nip_pokja": "123456789"`, `"nip_pokja": "`+nip+`"`, 1)
	baris = strings.Replace(baris, `"nama_pokja": "Contoh Nama Pokja"`, `"nama_pokja": "`+nama+`"`, 1)
	p := newInaprocPalsu(t, func(*http.Request) (int, string) {
		return 200, `{"success":true,"data":[` + baris + `],"meta":{"limit":1000,"has_more":false,"cursor":""}}`
	})
	pasangKonfigInaproc(t, p.URL, "token-uji")

	r, pesan := tenderPengumuman.rencanaDari(syncDatarRequest{KodeKLPD: klpdUji, Tahun: "2024"})
	if pesan != "" {
		t.Fatal(pesan)
	}
	hasil, err := tenderPengumuman.Jalankan(context.Background(), r, "")
	if err != nil {
		t.Fatal(err)
	}
	if hasil.TotalSinkron != 1 || hasil.TotalGagal != 0 || hasil.TotalDipotong != 0 {
		t.Fatalf("hasil = %+v, want 1 tersimpan utuh", hasil)
	}
	var nipDb, namaDb string
	if err := db.QueryRow("SELECT nip_pokja, nama_pokja FROM inaproc_tender_pengumuman WHERE kd_klpd = @p1", klpdUji).Scan(&nipDb, &namaDb); err != nil {
		t.Fatal(err)
	}
	if nipDb != nip || namaDb != nama {
		t.Errorf("tersimpan nip_pokja %d karakter, nama_pokja %d karakter; want %d dan %d (utuh)", len(nipDb), len(namaDb), len(nip), len(nama))
	}
}

// Kolom yang memang sempit (seperti nip_pokja NVARCHAR(50) sebelum migrasi 047): lebarnya dibaca dari skema asli, nilai dipotong, dan baris
// tetap tersimpan. Memakai tabel sementara sendiri supaya skema tabel aplikasi tidak disentuh.
func TestLebarKolomTeksDanPemotonganDenganSQLServer(t *testing.T) {
	db := dbIntegrasi(t)
	const tabel = "zz_uji_lebar_kolom"
	if _, err := db.Exec("DROP TABLE IF EXISTS " + tabel); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE ` + tabel + ` (
		row_key NVARCHAR(255) PRIMARY KEY, kd_klpd NVARCHAR(20) NULL, tahun_anggaran NVARCHAR(10) NULL,
		nip_pokja NVARCHAR(50) NULL, catatan NVARCHAR(MAX) NULL, nilai DECIMAL(24,2) NULL, extra_json NVARCHAR(MAX) NULL)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + tabel) })

	lebar := lebarKolomTeks(context.Background(), tabel)
	if lebar["nip_pokja"] != 50 || lebar["kd_klpd"] != 20 || lebar["row_key"] != 255 {
		t.Errorf("lebar = %v, want nip_pokja=50 kd_klpd=20 row_key=255", lebar)
	}
	if _, ada := lebar["catatan"]; ada {
		t.Errorf("kolom NVARCHAR(MAX) tidak boleh ikut: %v", lebar)
	}
	if _, ada := lebar["nilai"]; ada {
		t.Errorf("kolom bukan teks tidak boleh ikut: %v", lebar)
	}

	e := newEndpointDatar(endpointDatar{Nama: "uji-lebar", Tabel: tabel, Teks: []string{"kd_klpd", "nip_pokja", "catatan"}, Desimal: []string{"nilai"}})
	panjang := strings.Repeat("9", 120)
	dipotong, err := e.insert(db, map[string]interface{}{"kd_klpd": klpdUji, "nip_pokja": panjang, "catatan": panjang, "nilai": 12.5}, map[string]string{}, lebar)
	if err != nil {
		t.Fatalf("baris dengan nilai terlalu panjang harus tersimpan: %v", err)
	}
	if len(dipotong) != 1 || dipotong[0] != "nip_pokja" {
		t.Errorf("dipotong = %v, want [nip_pokja]", dipotong)
	}
	var nip, catatan string
	if err := db.QueryRow("SELECT nip_pokja, catatan FROM "+tabel).Scan(&nip, &catatan); err != nil {
		t.Fatal(err)
	}
	if nip != panjang[:50] || catatan != panjang {
		t.Errorf("nip_pokja %d karakter (want 50), catatan %d karakter (want 120, kolom MAX tidak dipotong)", len(nip), len(catatan))
	}

	// Tanpa pemotongan, kolom yang sama menggugurkan baris: inilah galat yang muncul di penarikan sebelum perbaikan.
	e2 := newEndpointDatar(endpointDatar{Nama: "uji-lebar", Tabel: tabel, Teks: []string{"kd_klpd", "nip_pokja", "catatan"}, Desimal: []string{"nilai"}})
	if _, err := e2.insert(db, map[string]interface{}{"kd_klpd": klpdUji, "nip_pokja": panjang, "catatan": "lain", "nilai": 1}, map[string]string{}, nil); err == nil || !strings.Contains(err.Error(), "truncated") {
		t.Errorf("tanpa lebar harus gagal karena terpotong, err = %v", err)
	}
}
