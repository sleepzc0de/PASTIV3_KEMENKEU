package handlers

import (
	"context"
	"database/sql/driver"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/database"
	"pasti-v3-backend/internal/fakesql"
)

func routerRefKanwil(role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", role)
		c.Set("username", "penguji")
		c.Next()
	})
	khusus := func(h gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			if c.GetString("role") != "superadmin" { // meniru RequireSuperadmin pada rute asli
				c.AbortWithStatus(403)
				return
			}
			h(c)
		}
	}
	r.GET("/kanwil", GetRefKanwil)
	r.PUT("/kanwil/:kode", khusus(PutRefKanwil))
	r.DELETE("/kanwil/:kode", khusus(DeleteRefKanwil))
	r.POST("/kanwil/dari-satker", khusus(PostRefKanwilDariSatker))
	r.POST("/kanwil/tarik-sldk", khusus(PostRefKanwilTarikSLDK))
	return r
}

func TestValidasiRefKanwil(t *testing.T) {
	ok := func(kode string, m refKanwilMasukan) {
		t.Helper()
		if p := validasiRefKanwil(kode, &m); p != "" {
			t.Errorf("validasiRefKanwil(%q, %+v) = %q, want sah", kode, m, p)
		}
	}
	gagal := func(kode string, m refKanwilMasukan, mengandung string) {
		t.Helper()
		if p := validasiRefKanwil(kode, &m); !strings.Contains(p, mengandung) {
			t.Errorf("validasiRefKanwil(%q, %+v) = %q, want memuat %q", kode, m, p, mengandung)
		}
	}
	satu, besar, min := 1, 10000, -1
	ok("015040199", refKanwilMasukan{Nama: "KANTOR WILAYAH DJP JAKARTA PUSAT", Singkatan: "KANWIL DJP JKT PUSAT"})
	ok("015040199", refKanwilMasukan{Nama: "TANPA SINGKATAN"})
	ok("015040199", refKanwilMasukan{Nama: "X", Urutan: &satu})
	// kode UE1 (5 digit), kode satker 6 digit, kode terlalu panjang, huruf, spasi, titik, dan angka non-ASCII bukan kode Kanwil
	for _, kode := range []string{"", "01504", "119091", "0150401991", "01504019A", "0150 0199", "015.04.01", "٠١٥٠٤٠١٩٩"} {
		gagal(kode, refKanwilMasukan{Nama: "X"}, "9 digit")
	}
	gagal("015040199", refKanwilMasukan{Nama: "  "}, "wajib")
	gagal("015040199", refKanwilMasukan{Nama: strings.Repeat("A", 201)}, "terlalu panjang")
	gagal("015040199", refKanwilMasukan{Nama: "X", Singkatan: strings.Repeat("B", 31)}, "Singkatan")
	gagal("015040199", refKanwilMasukan{Nama: "X", Urutan: &besar}, "Urutan")
	gagal("015040199", refKanwilMasukan{Nama: "X", Urutan: &min}, "Urutan")

	m := refKanwilMasukan{Nama: "  KANTOR   WILAYAH  DJP ", Singkatan: " K W "}
	if p := validasiRefKanwil("015040199", &m); p != "" || m.Nama != "KANTOR WILAYAH DJP" || m.Singkatan != "K W" {
		t.Errorf("rapikan: %q %+v", p, m)
	}
}

func TestPotongRune(t *testing.T) {
	if s, p := potongRune("abc", 3); s != "abc" || p {
		t.Errorf("tepat batas: %q %v", s, p)
	}
	if s, p := potongRune("ÀÉÎÕÜ", 3); s != "ÀÉÎ" || !p {
		t.Errorf("huruf non-ASCII dipotong per karakter, bukan per byte: %q %v", s, p)
	}
}

// terapkanKanwilSLDK: kode baru ditambahkan, baris "satker"/"sldk" yang berbeda diperbarui, yang sama dilewati, dan baris "manual" TIDAK pernah disentuh.
func TestTerapkanKanwilSLDKTidakMenimpaBarisManual(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "FROM ref_kanwil") {
			return []string{"kode", "sumber", "nama", "status"}, [][]driver.Value{
				{"015040199", "sldk", "NAMA LAMA", "AKTIF"},      // berbeda -> diperbarui
				{"015040299", "manual", "DIUBAH SUPERADMIN", ""}, // manual -> dilewati
				{"015040399", "sldk", "SAMA PERSIS", "AKTIF"},    // sama -> tanpa perubahan
				{"015050199", "satker", "DARI DATA SATKER", ""},  // sumber satker -> SLDK lebih otoritatif, diperbarui
				{"015050299", "sldk", "STATUS BERUBAH", "AKTIF"}, // hanya status berbeda -> diperbarui
			}, nil
		}
		return nil, nil, fmt.Errorf("query tak terduga: %s", q)
	}
	panjang := strings.Repeat("N", 250)
	baris := []kanwilSLDK{
		{Kode: "015040199", Nama: "NAMA BARU", Status: "AKTIF"},
		{Kode: "015040299", Nama: "NAMA DARI SLDK", Status: "AKTIF"},
		{Kode: " 015040399 ", Nama: " SAMA   PERSIS ", Status: "AKTIF"},
		{Kode: "015050199", Nama: "NAMA SLDK", Status: ""},
		{Kode: "015050299", Nama: "STATUS BERUBAH", Status: "TIDAK AKTIF"},
		{Kode: "015060199", Nama: panjang, Status: ""}, // baru, nama dipotong
		{Kode: "01506", Nama: "KODE UE1 BUKAN KANWIL"}, // tidak sah
		{Kode: "015060299", Nama: "   "},               // uraian kosong -> tidak sah
		{Kode: "015060199", Nama: "KODE GANDA"},        // ganda -> tidak sah
		{Kode: "01506019X", Nama: "HURUF PADA KODE"},   // tidak sah
	}
	h, err := terapkanKanwilSLDK(context.Background(), db, baris, "penguji")
	if err != nil {
		t.Fatal(err)
	}
	want := HasilTarikKanwil{Dibaca: 10, Ditambahkan: 1, Diperbarui: 3, TanpaPerubahan: 1, DilewatiManual: 1, TidakSah: 4, Dipotong: 1}
	if h != want {
		t.Errorf("hasil = %+v, want %+v", h, want)
	}
	// Tidak ada perintah tulis yang menyebut baris manual, dan semua dalam satu transaksi.
	ev := f.Events()
	if ev[0] != "BEGIN" || ev[len(ev)-1] != "COMMIT" {
		t.Errorf("harus satu transaksi: %v", ev)
	}
	var tambah, ubah int
	for _, e := range f.Execs() {
		if strings.Contains(strings.Join(argTeks(e.Args), " "), "015040299") {
			t.Errorf("baris manual 015040299 disentuh: %s %v", e.Query, argTeks(e.Args))
		}
		switch {
		case strings.HasPrefix(strings.TrimSpace(e.Query), "INSERT INTO ref_kanwil"):
			tambah++
			if a := argTeks(e.Args); a[0] != "015060199" || len(a[1]) != 200 {
				t.Errorf("INSERT args = %v (nama harus 200 karakter)", a)
			}
		case strings.HasPrefix(strings.TrimSpace(e.Query), "UPDATE ref_kanwil"):
			ubah++
		}
	}
	if tambah != 1 || ubah != 3 {
		t.Errorf("INSERT=%d UPDATE=%d, want 1 dan 3", tambah, ubah)
	}
}

func argTeks(a []driver.NamedValue) []string {
	out := make([]string, len(a))
	for i, v := range a {
		out[i] = fmt.Sprint(v.Value)
	}
	return out
}

// Kesalahan menulis membatalkan seluruh penarikan (tidak ada yang separuh jadi).
func TestTerapkanKanwilSLDKBatalBilaGagal(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(context.Context, string, []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return []string{"kode", "sumber", "nama", "status"}, nil, nil
	}
	f.OnExec = func(q string, _ []driver.NamedValue) error {
		if strings.Contains(q, "INSERT") {
			return fmt.Errorf("mssql: gagal")
		}
		return nil
	}
	if _, err := terapkanKanwilSLDK(context.Background(), db, []kanwilSLDK{{Kode: "015040199", Nama: "A"}}, "x"); err == nil {
		t.Fatal("seharusnya galat")
	}
	ev := f.Events()
	if ev[len(ev)-1] != "ROLLBACK" {
		t.Errorf("harus ROLLBACK: %v", ev)
	}
}

// Pembaca SLDK: membaca hanya kolom yang dibutuhkan (tanpa kolom pribadi) dan menolak hasil yang melebihi batas pengaman.
func TestBacaKanwilSLDK(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		for _, terlarang := range []string{"nip", "nama", "jabatan", "email", "no_telp", "no_fax"} {
			if regexp.MustCompile(`(?i)\bK\.` + terlarang).MatchString(q) {
				t.Errorf("query membaca kolom pribadi %q: %s", terlarang, q)
			}
		}
		if !strings.Contains(q, "DJKN.SIMAN2_R_KORWIL") || !strings.Contains(q, "'015%'") || !strings.Contains(q, "ROW_NUMBER()") {
			t.Errorf("query SLDK = %s", q)
		}
		return []string{"kode", "nama", "status"}, [][]driver.Value{{"015040199 ", "KANWIL A", "AKTIF"}, {"015040299", nil, nil}}, nil
	}
	got, err := bacaKanwilSLDK(context.Background(), db)
	if err != nil || len(got) != 2 || got[0] != (kanwilSLDK{"015040199", "KANWIL A", "AKTIF"}) || got[1] != (kanwilSLDK{Kode: "015040299"}) {
		t.Fatalf("hasil = %+v (%v)", got, err)
	}

	f.OnQuery = func(context.Context, string, []driver.NamedValue) ([]string, [][]driver.Value, error) {
		rows := make([][]driver.Value, refKanwilBarisSLDKMaks+1)
		for i := range rows {
			rows[i] = []driver.Value{fmt.Sprintf("%09d", i), "X", nil}
		}
		return []string{"kode", "nama", "status"}, rows, nil
	}
	if _, err := bacaKanwilSLDK(context.Background(), db); err == nil || !strings.Contains(err.Error(), "lebih dari") {
		t.Errorf("batas baris: %v", err)
	}
}

// Penarikan tanpa koneksi SLDK dijawab 503 dan SLDK yang kosong tidak mengubah apa pun; hasil bacaan SLDK tidak bocor ke klien.
func TestPostRefKanwilTarikSLDKTanpaKoneksiDanGalat(t *testing.T) {
	lama := database.SLDKDB
	t.Cleanup(func() { database.SLDKDB = lama })

	database.SLDKDB = nil
	if code, _ := call(routerRefKanwil("superadmin"), "POST", "/kanwil/tarik-sldk", ""); code != 503 {
		t.Errorf("tanpa SLDK = %d, want 503", code)
	}
	if code, _ := call(routerRefKanwil("user"), "POST", "/kanwil/tarik-sldk", ""); code != 403 {
		t.Errorf("oleh user = %d, want 403", code)
	}

	db, f := fakesql.New(t)
	database.SLDKDB = db
	f.OnQuery = func(context.Context, string, []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return nil, nil, fmt.Errorf("mssql: login failed for user 'USERROMADAN' password=RAHASIA host=10.216.121.1")
	}
	code, body := call(routerRefKanwil("superadmin"), "POST", "/kanwil/tarik-sldk", "")
	if code != 502 || strings.Contains(fmt.Sprint(body), "RAHASIA") || strings.Contains(fmt.Sprint(body), "10.216") {
		t.Errorf("galat SLDK = %d %v (tidak boleh membocorkan detail koneksi)", code, body)
	}

	f.OnQuery = func(context.Context, string, []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return []string{"kode", "nama", "status"}, nil, nil
	}
	if code, body := call(routerRefKanwil("superadmin"), "POST", "/kanwil/tarik-sldk", ""); code != 409 || !strings.Contains(fmt.Sprint(body["message"]), "tidak diubah") {
		t.Errorf("SLDK kosong = %d %v, want 409 tanpa perubahan", code, body)
	}
}

// ---- integrasi SQL Server ----

// Seluruh data uji berkode 01599xxxx: "01599" bukan kode UE1 yang ada, tetapi tetap berawalan KL 015 sehingga ikut terbaca oleh query SLDK (filter KL 015).
const awalanUjiKanwil = "01599"

func TestRefKanwilDenganSQLServer(t *testing.T) {
	db := dbIntegrasi(t)
	bersih := func() {
		db.Exec(`DELETE FROM ref_kanwil WHERE kode LIKE '` + awalanUjiKanwil + `%'`)
		db.Exec(`DELETE FROM DIGITALISASI_SATKER WHERE Kode_Satker LIKE '` + awalanUjiKanwil + `%'`)
		db.Exec(`IF OBJECT_ID('DJKN.SIMAN2_R_KORWIL', 'U') IS NOT NULL DELETE FROM DJKN.SIMAN2_R_KORWIL WHERE kd_wileselon LIKE '` + awalanUjiKanwil + `%' OR kd_wileselon LIKE '5999%'`)
	}
	bersih()
	if !biarkanDataUji() {
		t.Cleanup(bersih)
	}
	superadmin, user := routerRefKanwil("superadmin"), routerRefKanwil("user")
	const k0, k1, k2, k3, k4 = "015990099", "015990199", "015990299", "015990399", "015990499"

	// ---- pengguna biasa boleh membaca tetapi tidak mengubah
	if code, body := call(user, "GET", "/kanwil", ""); code != 200 {
		t.Fatalf("GET: %d %v", code, body)
	}
	if code, _ := call(user, "PUT", "/kanwil/"+k0, `{"nama":"X"}`); code != 403 {
		t.Errorf("PUT oleh user = %d, want 403", code)
	}

	// ---- tambah manual: urutan bawaan 100, aktif, sumber manual, kode UE1 diturunkan, spasi dirapikan
	code, body := call(superadmin, "PUT", "/kanwil/"+k0, `{"nama":"  KANTOR   WILAYAH   MANUAL  ","singkatan":"KW MANUAL"}`)
	if code != 200 {
		t.Fatalf("PUT baru: %d %v", code, body)
	}
	d := dataOf(t, body)
	if d["kode"] != k0 || d["kode_ue1"] != "01599" || d["nama"] != "KANTOR WILAYAH MANUAL" || d["singkatan"] != "KW MANUAL" || d["urutan"] != float64(100) ||
		d["aktif"] != true || d["sumber"] != "manual" || d["diubah_oleh"] != "penguji" {
		t.Errorf("hasil PUT baru = %v", d)
	}
	// urutan/aktif yang tidak dikirim tidak berubah; nonaktifkan; masukan tidak sah ditolak tanpa mengubah apa pun
	if code, body = call(superadmin, "PUT", "/kanwil/"+k0, `{"nama":"KANTOR WILAYAH MANUAL"}`); code != 200 || dataOf(t, body)["urutan"] != float64(100) || dataOf(t, body)["aktif"] != true {
		t.Errorf("ubah tanpa urutan/aktif: %d %v", code, body)
	}
	if code, body = call(superadmin, "PUT", "/kanwil/"+k0, `{"nama":"KANTOR WILAYAH MANUAL","aktif":false,"urutan":7}`); code != 200 || dataOf(t, body)["aktif"] != false || dataOf(t, body)["urutan"] != float64(7) {
		t.Errorf("nonaktifkan: %d %v", code, body)
	}
	for _, c := range []struct{ kode, badan string }{
		{"01599009", `{"nama":"X"}`}, {"0159900990", `{"nama":"X"}`}, {k0, `{"nama":""}`}, {k0, `{"nama":"X","urutan":99999}`}, {k0, `bukan json`},
	} {
		if code, _ := call(superadmin, "PUT", "/kanwil/"+c.kode, c.badan); code != 400 {
			t.Errorf("PUT %s %s = %d, want 400", c.kode, c.badan, code)
		}
	}
	if got := hitungSSO(t, db, `SELECT COUNT(*) FROM ref_kanwil WHERE kode LIKE '`+awalanUjiKanwil+`%'`); got != 1 {
		t.Errorf("jumlah baris setelah masukan tidak sah = %d, want 1", got)
	}

	// ---- kode di data satker yang belum terdaftar dilaporkan, lengkap dengan saran uraian dari nama satker
	if _, err := db.Exec(`INSERT INTO DIGITALISASI_SATKER (Kode_UE1, Kode_Satker, Jenis_Satker, Nama_Satker) VALUES
		(N'01599', N'015990199111111000KP', N'INDUK SATKER', N'KANTOR PELAYANAN PAJAK UJI'),
		(N'01599', N'015990199222222000KP', N'INDUK SATKER', N'KANTOR WILAYAH DJP UJI RAYA'),
		(N'01599', N'015990199333333000KP', N'INDUK SATKER', N'KANWIL UJI'),
		(N'01599', N'015990299444444000KP', N'INDUK SATKER', N'SATKER BIASA UJI'),
		(N'01599', N'015990399555555000KP', N'INDUK SATKER', N'KANTOR PUSAT UJI'),
		(N'01599', N'015990099666666000KP', N'INDUK SATKER', N'SUDAH TERDAFTAR'),
		(N'01599', N'01599KP', N'INDUK SATKER', N'KODE TERLALU PENDEK'),
		(N'01599', N'01599X199777777000KP', N'INDUK SATKER', N'KODE BERHURUF')`); err != nil {
		t.Fatal(err)
	}
	_, body = call(superadmin, "GET", "/kanwil", "")
	belum := map[string]map[string]interface{}{}
	for _, x := range dataOf(t, body)["belum_terdaftar"].([]interface{}) {
		m := x.(map[string]interface{})
		belum[m["kode"].(string)] = m
	}
	// k1: 3 satker; saran = nama TERPENDEK di antara yang menyebut kantor wilayah ("KANWIL UJI"); k2: tanpa saran; k3: kantor pusat
	if m := belum[k1]; m == nil || m["satker"] != float64(3) || m["saran"] != "KANWIL UJI" || m["kode_ue1"] != "01599" {
		t.Errorf("%s = %v, want 3 satker, saran KANWIL UJI", k1, m)
	}
	if m := belum[k2]; m == nil || m["satker"] != float64(1) || m["saran"] != "" {
		t.Errorf("%s = %v, want 1 satker tanpa saran", k2, m)
	}
	if m := belum[k3]; m == nil || m["saran"] != "KANTOR PUSAT UJI" {
		t.Errorf("%s = %v, want saran KANTOR PUSAT UJI", k3, m)
	}
	for _, kode := range []string{k0, "01599KP", "01599X199"} {
		if belum[kode] != nil {
			t.Errorf("%s tidak boleh dilaporkan: sudah terdaftar atau bukan kode Kanwil sah", kode)
		}
	}
	if d := dataOf(t, body); d["sldk_tersedia"] != (database.SLDKDB != nil) {
		t.Errorf("sldk_tersedia = %v", d["sldk_tersedia"])
	}

	// ---- tambah dari satker: hanya yang punya saran, pengguna biasa ditolak, berulang aman, baris yang sudah ada tidak berubah
	if code, _ := call(user, "POST", "/kanwil/dari-satker", ""); code != 403 {
		t.Errorf("dari-satker oleh user = %d, want 403", code)
	}
	code, body = call(superadmin, "POST", "/kanwil/dari-satker", "")
	if d := dataOf(t, body); code != 200 || d["ditambahkan"].(float64) < 2 {
		t.Fatalf("dari-satker = %d %v, want minimal 2 ditambahkan", code, body)
	}
	var nama, sumber string
	db.QueryRow(`SELECT nama, sumber FROM ref_kanwil WHERE kode = '`+k1+`'`).Scan(&nama, &sumber)
	if nama != "KANWIL UJI" || sumber != "satker" {
		t.Errorf("%s = %q / %q, want KANWIL UJI / satker", k1, nama, sumber)
	}
	if _, body = call(superadmin, "POST", "/kanwil/dari-satker", ""); dataOf(t, body)["ditambahkan"] != float64(0) {
		t.Errorf("dari-satker ulang = %v, want 0 ditambahkan", body)
	}
	if got := hitungSSO(t, db, `SELECT COUNT(*) FROM ref_kanwil WHERE kode = '`+k2+`'`); got != 0 {
		t.Error("kode tanpa saran tidak boleh ditambahkan")
	}
	db.QueryRow(`SELECT nama, sumber FROM ref_kanwil WHERE kode = '`+k0+`'`).Scan(&nama, &sumber)
	if nama != "KANTOR WILAYAH MANUAL" || sumber != "manual" {
		t.Errorf("baris manual berubah: %q / %q", nama, sumber)
	}

	// ---- tarik dari SLDK: tabel tiruan DJKN.SIMAN2_R_KORWIL di database uji ini (menguji sintaks T-SQL pembacanya terhadap SQL Server sungguhan)
	if _, err := db.Exec(`IF SCHEMA_ID('DJKN') IS NULL EXEC('CREATE SCHEMA DJKN')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`IF OBJECT_ID('DJKN.SIMAN2_R_KORWIL', 'U') IS NULL CREATE TABLE DJKN.SIMAN2_R_KORWIL (
		id_korwil INT NULL, kd_wileselon NVARCHAR(9) NULL, kd_korwil NVARCHAR(4) NULL, ur_korwil NVARCHAR(100) NULL, status_korwil NVARCHAR(50) NULL,
		nip NVARCHAR(50) NULL, nama NVARCHAR(60) NULL, updated_at DATETIME2 NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO DJKN.SIMAN2_R_KORWIL (id_korwil, kd_wileselon, kd_korwil, ur_korwil, status_korwil, nip, nama, updated_at) VALUES
		(1, N'015990199', N'0199', N'KANTOR WILAYAH DJP UJI RAYA (SLDK LAMA)', N'AKTIF', N'199001012015011001', N'PEJABAT RAHASIA', '2024-01-01'),
		(2, N'015990199', N'0199', N'KANTOR WILAYAH DJP UJI RAYA (SLDK BARU)', N'AKTIF', N'199001012015011001', N'PEJABAT RAHASIA', '2025-01-01'),
		(3, N'015990099', N'0099', N'NAMA SLDK UNTUK BARIS MANUAL', N'AKTIF', NULL, NULL, '2025-01-01'),
		(4, N'015990499', N'0499', N'KANWIL BARU DARI SLDK', NULL, NULL, NULL, '2025-01-01'),
		(5, N'0159905', N'0199', N'KODE PENDEK TIDAK SAH', N'AKTIF', NULL, NULL, '2025-01-01'),
		(6, N'015990599', N'0599', NULL, N'AKTIF', NULL, NULL, '2025-01-01'),
		(7, N'01599A699', N'A699', N'HURUF PADA KODE', N'AKTIF', NULL, NULL, '2025-01-01'),
		(8, N'599900199', N'0199', N'KL LAIN DIABAIKAN', N'AKTIF', NULL, NULL, '2025-01-01')`); err != nil {
		t.Fatal(err)
	}
	lama := database.SLDKDB
	t.Cleanup(func() { database.SLDKDB = lama })
	database.SLDKDB = db

	if code, _ := call(user, "POST", "/kanwil/tarik-sldk", ""); code != 403 {
		t.Errorf("tarik oleh user = %d, want 403", code)
	}
	tarik := func() map[string]interface{} {
		t.Helper()
		code, body := call(superadmin, "POST", "/kanwil/tarik-sldk", "")
		if code != 200 {
			t.Fatalf("tarik: %d %v", code, body)
		}
		return dataOf(t, body)
	}
	cocok := func(got map[string]interface{}, want map[string]float64) {
		t.Helper()
		for k, v := range want {
			if got[k] != v {
				t.Errorf("hasil tarik %s = %v, want %v (semua: %v)", k, got[k], v, got)
			}
		}
	}
	// KL lain tidak dibaca; k1 dedup ke baris terbaru; k4 baru; k0 manual dilewati; kode pendek, uraian kosong, dan kode berhuruf tidak sah
	cocok(tarik(), map[string]float64{"dibaca": 6, "ditambahkan": 1, "diperbarui": 1, "tanpa_perubahan": 0, "dilewati_manual": 1, "tidak_sah": 3, "dipotong": 0})
	var status *string
	db.QueryRow(`SELECT nama, sumber, status_sldk FROM ref_kanwil WHERE kode = '`+k1+`'`).Scan(&nama, &sumber, &status)
	if nama != "KANTOR WILAYAH DJP UJI RAYA (SLDK BARU)" || sumber != "sldk" || status == nil || *status != "AKTIF" {
		t.Errorf("%s = %q / %q / %v, want baris SLDK terbaru", k1, nama, sumber, status)
	}
	db.QueryRow(`SELECT nama, sumber, status_sldk FROM ref_kanwil WHERE kode = '`+k4+`'`).Scan(&nama, &sumber, &status)
	if nama != "KANWIL BARU DARI SLDK" || sumber != "sldk" || status != nil {
		t.Errorf("%s = %q / %q / %v, want baru dari SLDK tanpa status", k4, nama, sumber, status)
	}
	db.QueryRow(`SELECT nama, sumber FROM ref_kanwil WHERE kode = '`+k0+`'`).Scan(&nama, &sumber)
	if nama != "KANTOR WILAYAH MANUAL" || sumber != "manual" {
		t.Errorf("baris manual ditimpa SLDK: %q / %q", nama, sumber)
	}
	db.QueryRow(`SELECT nama, sumber FROM ref_kanwil WHERE kode = '`+k3+`'`).Scan(&nama, &sumber)
	if nama != "KANTOR PUSAT UJI" || sumber != "satker" {
		t.Errorf("%s tidak ada di SLDK: harus tetap KANTOR PUSAT UJI / satker, dapat %q / %q", k3, nama, sumber)
	}
	if got := hitungSSO(t, db, `SELECT COUNT(*) FROM ref_kanwil WHERE kode IN ('015990599', '0159905', '01599A699', '599900199')`); got != 0 {
		t.Errorf("kode tidak sah atau KL lain masuk referensi: %d baris", got)
	}

	// diulang: tidak ada yang berubah; lalu uraian k4 di SLDK berubah -> hanya itu yang diperbarui
	cocok(tarik(), map[string]float64{"dibaca": 6, "ditambahkan": 0, "diperbarui": 0, "tanpa_perubahan": 2, "dilewati_manual": 1, "tidak_sah": 3})
	if _, err := db.Exec(`UPDATE DJKN.SIMAN2_R_KORWIL SET ur_korwil = N'KANWIL BARU DIGANTI', status_korwil = N'AKTIF' WHERE kd_wileselon = N'015990499'`); err != nil {
		t.Fatal(err)
	}
	cocok(tarik(), map[string]float64{"ditambahkan": 0, "diperbarui": 1, "tanpa_perubahan": 1})

	// ---- baris yang diedit superadmin sesudahnya menjadi manual dan tidak ditimpa penarikan; hapus: user ditolak, superadmin menghapus, ulang 404, kode tidak sah 400
	if code, _ := call(superadmin, "PUT", "/kanwil/"+k4, `{"nama":"DIPERBAIKI MANUAL"}`); code != 200 {
		t.Fatalf("PUT k4: %d", code)
	}
	cocok(tarik(), map[string]float64{"diperbarui": 0, "dilewati_manual": 2})
	db.QueryRow(`SELECT nama, sumber FROM ref_kanwil WHERE kode = '`+k4+`'`).Scan(&nama, &sumber)
	if nama != "DIPERBAIKI MANUAL" || sumber != "manual" {
		t.Errorf("%s = %q / %q, want diedit manual dan tidak ditimpa", k4, nama, sumber)
	}
	if code, _ := call(user, "DELETE", "/kanwil/"+k4, ""); code != 403 {
		t.Errorf("DELETE oleh user = %d, want 403", code)
	}
	if code, body := call(superadmin, "DELETE", "/kanwil/"+k4, ""); code != 200 {
		t.Errorf("DELETE = %d %v", code, body)
	}
	if code, _ := call(superadmin, "DELETE", "/kanwil/"+k4, ""); code != 404 {
		t.Errorf("DELETE ulang = %d, want 404", code)
	}
	if code, _ := call(superadmin, "DELETE", "/kanwil/abc", ""); code != 400 {
		t.Errorf("DELETE kode tidak sah = %d, want 400", code)
	}
}

// Migrasi 056 dapat dijalankan ulang tanpa mengubah isi, dan batasan tabel menolak kode dan sumber yang tidak sah.
func TestMigrasi056AmanDijalankanUlang(t *testing.T) {
	db := dbIntegrasi(t)
	bersih := func() { db.Exec(`DELETE FROM ref_kanwil WHERE kode LIKE '` + awalanUjiKanwil + `%'`) }
	bersih()
	if !biarkanDataUji() {
		t.Cleanup(bersih)
	}
	if _, err := db.Exec(`INSERT INTO ref_kanwil (kode, nama, sumber) VALUES (N'015990099', N'ISI TETAP', N'manual')`); err != nil {
		t.Fatal(err)
	}
	skrip, err := os.ReadFile("../migrations/056_create_ref_kanwil.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i, batch := range regexp.MustCompile(`(?im)^[ \t]*GO[ \t]*\r?$`).Split(string(skrip), -1) {
		if strings.TrimSpace(batch) == "" {
			continue
		}
		if _, err := db.Exec(batch); err != nil {
			t.Fatalf("jalankan ulang batch %d: %v", i+1, err)
		}
	}
	var nama string
	if err := db.QueryRow(`SELECT nama FROM ref_kanwil WHERE kode = '015990099'`).Scan(&nama); err != nil || nama != "ISI TETAP" {
		t.Errorf("isi setelah migrasi diulang = %q (%v)", nama, err)
	}
	if _, err := db.Exec(`INSERT INTO ref_kanwil (kode, nama) VALUES (N'01599009', N'X')`); err == nil {
		t.Error("kode 8 digit harus ditolak oleh batasan tabel")
	}
	if _, err := db.Exec(`INSERT INTO ref_kanwil (kode, nama, sumber) VALUES (N'015990098', N'X', N'lain')`); err == nil {
		t.Error("sumber tidak dikenal harus ditolak oleh batasan tabel")
	}
}
