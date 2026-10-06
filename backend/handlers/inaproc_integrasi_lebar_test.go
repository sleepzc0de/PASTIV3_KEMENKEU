package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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

// Halaman Penarikan Data membaca daftar dari status penarikan dengan .length/.map, jadi daftar yang kosong harus berupa [] di JSON, bukan null
// (irisan nil di Go menjadi null). Dulu `bermasalah` null pada server tanpa riwayat kegagalan dan halaman crash.
func TestStatusPenarikanKirimDaftarSebagaiLarikDenganSQLServer(t *testing.T) {
	db := dbIntegrasi(t)
	lama := Penarik
	Penarik = NewPenarikInaproc(db)
	t.Cleanup(func() { Penarik = lama })
	batasUji(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/penarikan", func(c *gin.Context) { c.Set("role", "superadmin"); GetInaprocPenarikan(c) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/penarikan", nil))
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var res struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	for _, kunci := range []string{"bermasalah", "riwayat", "datasets", "kelompok", "tahun_bawaan"} {
		if _, ok := res.Data[kunci].([]interface{}); !ok {
			t.Errorf("data.%s = %#v, want larik (bukan null)", kunci, res.Data[kunci])
		}
	}
}

// Kolom yang terbukti terlalu sempit untuk data asli sudah dilebarkan (migrasi 048 dan 049): kd_rup memuat banyak kode dipisah ";" tanpa batas
// yang wajar, jadi NVARCHAR(MAX) dan tanpa indeks; kolom nama/nomor pada pencatatan non tender juga NVARCHAR(MAX).
func TestKolomDilebarkanMigrasi048Dan049DenganSQLServer(t *testing.T) {
	db := dbIntegrasi(t)
	for _, c := range [][2]string{
		{"inaproc_non_tender_pengumuman", "kd_rup"}, {"inaproc_non_tender_selesai", "kd_rup"}, {"inaproc_pencatatan_non_tender", "kd_rup"},
		{"inaproc_pencatatan_swakelola", "kd_rup"}, {"inaproc_tender_pengumuman", "kd_rup"}, {"inaproc_tender_selesai", "kd_rup"},
		{"inaproc_pencatatan_non_tender_realisasi", "kd_rup_paket"}, {"inaproc_tender_selesai_nilai", "kd_rup_paket"},
		{"inaproc_pencatatan_non_tender", "nama_paket"}, {"inaproc_pencatatan_non_tender_realisasi", "nama_paket"},
		{"inaproc_pencatatan_non_tender_realisasi", "nama_penyedia"}, {"inaproc_pencatatan_non_tender_realisasi", "no_realisasi"},
	} {
		var n int
		if err := db.QueryRow("SELECT COL_LENGTH(@p1, @p2)", c[0], c[1]).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != -1 {
			t.Errorf("%s.%s = %d byte, want -1 (NVARCHAR(MAX)): terapkan migrasi 048 dan 049 ke database uji", c[0], c[1], n)
		}
	}
	var indeks int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sys.indexes WHERE name IN ('idx_intp_kd_rup','idx_intsl_kd_rup','idx_ipnt_kd_rup','idx_ips_kd_rup',
		'idx_itp_kd_rup','idx_its_kd_rup','idx_ipntr_kd_rup_paket')`).Scan(&indeks); err != nil {
		t.Fatal(err)
	}
	if indeks != 0 {
		t.Errorf("%d indeks kd_rup masih ada, want 0 (kolom MAX tidak bisa diindeks; migrasi 049)", indeks)
	}
}

// Migrasi 050: nama paket RUP (nama gabungan panjang) dan mak e-purchasing V6 tidak lagi terpotong. Handler RUP lama menyimpannya apa adanya,
// jadi nama sepanjang ribuan karakter harus tersimpan utuh di tabel asli.
func TestNamaPaketRUPPanjangTersimpanUtuhDenganSQLServer(t *testing.T) {
	db := dbIntegrasi(t)
	for _, c := range [][2]string{
		{"inaproc_paket_penyedia", "nama_paket"}, {"inaproc_paket_swakelola", "nama_paket"}, {"inaproc_paket_swakelola_terumumkan", "nama_paket"},
		{"inaproc_paket_penyedia_terumumkan", "nama_paket"}, {"inaproc_ekatalog6_paket_epurchasing", "mak"},
	} {
		var n int
		if err := db.QueryRow("SELECT COL_LENGTH(@p1, @p2)", c[0], c[1]).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != -1 {
			t.Errorf("%s.%s = %d byte, want -1 (NVARCHAR(MAX)): terapkan migrasi 050 ke database uji", c[0], c[1], n)
		}
	}

	bersihkanUji(t, db)
	t.Cleanup(func() { bersihkanUji(t, db) })
	nama := strings.Repeat("Pakaian Dinas Pegawai/Perawat (Jawa Tengah), ", 40) // ~1.800 karakter
	p := newInaprocPalsu(t, func(*http.Request) (int, string) {
		return 200, `{"success":true,"data":[{"kd_rup":7001,"kd_klpd":"` + klpdUji + `","tahun_anggaran":2031,"nama_paket":"` + nama + `"}],"meta":{"limit":1000,"has_more":false,"cursor":""}}`
	})
	pasangKonfigInaproc(t, p.URL, "token-uji")

	d, _ := DatasetByID("rup/paket-penyedia")
	hasil, err := d.Jalankan(context.Background(), d.Normalisasi(PermintaanTarik{KodeKLPD: klpdUji, Tahun: "2031"}), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if hasil.TotalSinkron != 1 || hasil.TotalGagal != 0 {
		t.Fatalf("hasil = %+v, want 1 tersimpan tanpa gagal", hasil)
	}
	var tersimpan string
	if err := db.QueryRow("SELECT nama_paket FROM inaproc_paket_penyedia WHERE kd_klpd = @p1 AND tahun_anggaran = '2031'", klpdUji).Scan(&tersimpan); err != nil {
		t.Fatal(err)
	}
	if tersimpan != nama {
		t.Errorf("nama_paket tersimpan %d karakter, want %d (utuh)", len(tersimpan), len(nama))
	}
}

// Satu paket dapat memuat puluhan kode RUP: daftar sepanjang ribuan karakter tersimpan utuh, tidak dipotong.
func TestKodeRUPSangatPanjangTersimpanUtuhDenganSQLServer(t *testing.T) {
	db := dbIntegrasi(t)
	bersihkanUji(t, db)
	t.Cleanup(func() { bersihkanUji(t, db) })

	var kode []string
	for i := 0; i < 600; i++ {
		kode = append(kode, "6626"+strings.Repeat("1", 4)+string(rune('0'+i%10)))
	}
	panjang := strings.Join(kode, ";") // ~5.400 karakter
	baris := strings.Replace(contohPencatatanNonTender, `"kd_klpd": "DX"`, `"kd_klpd": "`+klpdUji+`"`, 1)
	baris = strings.Replace(baris, `"kd_rup": "12345"`, `"kd_rup": "`+panjang+`"`, 1)
	if !strings.Contains(baris, panjang) || !strings.Contains(baris, `"kd_klpd": "`+klpdUji+`"`) {
		t.Fatal("contoh pencatatan non tender tidak punya kd_klpd/kd_rup seperti yang diasumsikan tes")
	}
	p := newInaprocPalsu(t, func(*http.Request) (int, string) {
		return 200, `{"success":true,"data":[` + baris + `],"meta":{"limit":1000,"has_more":false,"cursor":""}}`
	})
	pasangKonfigInaproc(t, p.URL, "token-uji")

	r, pesan := pencatatanNonTender.rencanaDari(syncDatarRequest{KodeKLPD: klpdUji, Tahun: "2024"})
	if pesan != "" {
		t.Fatal(pesan)
	}
	hasil, err := pencatatanNonTender.Jalankan(context.Background(), r, "")
	if err != nil {
		t.Fatal(err)
	}
	if hasil.TotalSinkron != 1 || hasil.TotalGagal != 0 || hasil.TotalDipotong != 0 {
		t.Fatalf("hasil = %+v, want 1 tersimpan utuh tanpa pemotongan", hasil)
	}
	var tersimpan string
	if err := db.QueryRow("SELECT kd_rup FROM inaproc_pencatatan_non_tender WHERE kd_klpd = @p1", klpdUji).Scan(&tersimpan); err != nil {
		t.Fatal(err)
	}
	if tersimpan != panjang {
		t.Errorf("kd_rup tersimpan %d karakter, want %d (utuh)", len(tersimpan), len(panjang))
	}
}

// Data asli: kd_rup pada pengumuman bisa memuat beberapa kode dipisah ";". Paket RUP dengan salah satu kode itu tetap dihitung sudah diproses,
// bukan "Belum diproses" (yang akan terjadi bila kolomnya dibandingkan utuh dengan satu kode).
func TestCorongKodeRUPBerlipatDenganSQLServer(t *testing.T) {
	db := dbIntegrasi(t)
	bersihkanUji(t, db)
	t.Cleanup(func() { bersihkanUji(t, db) })
	k, th := klpdUji, "2031"

	for _, kd := range []string{"RUP-A", "RUP-B", "RUP-C", "RUP-D"} {
		sisip(t, db, "inaproc_paket_penyedia", m{"kd_klpd": k, "tahun_anggaran": th, "kd_rup": kd, "pagu": int64(100_000_000), "status_aktif_rup": 1, "status_delete_rup": 0})
	}
	// Dua kode RUP dalam satu pengumuman tender (dengan spasi di sekitar pemisah), satu pada non-tender, satu tidak diproses.
	sisip(t, db, "inaproc_tender_pengumuman", m{"kd_klpd": k, "tahun_anggaran": th, "kd_tender": "TB1", "versi_tender": 1, "kd_rup": "RUP-A; RUP-B", "status_tender": "Selesai"})
	sisip(t, db, "inaproc_non_tender_pengumuman", m{"kd_klpd": k, "tahun_anggaran": th, "kd_nontender": "NB1", "versi_nontender": 1, "kd_rup": "RUP-C;", "status_nontender": "Selesai"})

	h := HitungAnalitik(context.Background(), db, klpdUji, th, time.Date(2031, 10, 4, 3, 0, 0, 0, zonaWIB))
	if len(h.Galat) != 0 {
		t.Fatalf("bagian gagal dibaca: %v", h.Galat)
	}
	peta := map[string]int64{}
	for _, x := range h.Corong.Tahap {
		peta[x.Label] = x.Jumlah
	}
	if h.Corong.TotalPaket != 4 || peta["Tender"] != 2 || peta["Non-tender"] != 1 || peta["Belum diproses"] != 1 {
		t.Errorf("corong = %+v (tahap %v), want 4 paket: 2 tender, 1 non-tender, 1 belum diproses", h.Corong, peta)
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
