package handlers

import (
	"database/sql"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func routerRefUE1(role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", role)
		c.Set("username", "penguji")
		c.Next()
	})
	r.GET("/ue1", GetRefUE1)
	r.PUT("/ue1/:kode", func(c *gin.Context) {
		if c.GetString("role") != "superadmin" { // meniru RequireRole pada rute asli
			c.AbortWithStatus(403)
			return
		}
		PutRefUE1(c)
	})
	r.DELETE("/ue1/:kode", func(c *gin.Context) {
		if c.GetString("role") != "superadmin" {
			c.AbortWithStatus(403)
			return
		}
		DeleteRefUE1(c)
	})
	return r
}

func TestValidasiRefUE1(t *testing.T) {
	ok := func(kode string, m refUE1Masukan) {
		t.Helper()
		if p := validasiRefUE1(kode, &m); p != "" {
			t.Errorf("validasiRefUE1(%q, %+v) = %q, want sah", kode, m, p)
		}
	}
	gagal := func(kode string, m refUE1Masukan, mengandung string) {
		t.Helper()
		if p := validasiRefUE1(kode, &m); !strings.Contains(p, mengandung) {
			t.Errorf("validasiRefUE1(%q, %+v) = %q, want memuat %q", kode, m, p, mengandung)
		}
	}
	satu, besar := 1, 10000
	ok("01504", refUE1Masukan{Nama: "DIREKTORAT JENDERAL PAJAK", Singkatan: "DJP"})
	ok("01599", refUE1Masukan{Nama: "UNIT BARU"}) // singkatan boleh kosong
	ok("01599", refUE1Masukan{Nama: "UNIT BARU", Urutan: &satu})
	for _, kode := range []string{"", "1504", "015041", "0150A", "01 04", "01.04", "٠١٥٠٤"} {
		gagal(kode, refUE1Masukan{Nama: "X"}, "5 digit")
	}
	gagal("01504", refUE1Masukan{Nama: "  "}, "wajib")
	gagal("01504", refUE1Masukan{Nama: strings.Repeat("A", 201)}, "terlalu panjang")
	gagal("01504", refUE1Masukan{Nama: "X", Singkatan: strings.Repeat("B", 31)}, "Singkatan")
	gagal("01504", refUE1Masukan{Nama: "X", Urutan: &besar}, "Urutan")
	min := -1
	gagal("01504", refUE1Masukan{Nama: "X", Urutan: &min}, "Urutan")

	// spasi ganda dirapikan
	m := refUE1Masukan{Nama: "  DIREKTORAT   JENDERAL  PAJAK ", Singkatan: " D J P "}
	if p := validasiRefUE1("01504", &m); p != "" || m.Nama != "DIREKTORAT JENDERAL PAJAK" || m.Singkatan != "D J P" {
		t.Errorf("rapikan: %q %+v", p, m)
	}
}

func TestLabelRefUE1(t *testing.T) {
	ref := map[string]RefUE1Baris{
		"01504": {Kode: "01504", Nama: "DIREKTORAT JENDERAL PAJAK", Singkatan: "DJP"},
		"01599": {Kode: "01599", Nama: "UNIT TANPA SINGKATAN"},
	}
	for kode, want := range map[string]string{
		"01504": "01504 · DJP", "01599": "01599 · UNIT TANPA SINGKATAN", "01500": "UE1 01500", "(kosong)": "(kosong)", "": "(kosong)",
	} {
		if got := labelUE1(kode, ref); got != want {
			t.Errorf("labelUE1(%q) = %q, want %q", kode, got, want)
		}
	}
	if got := labelUE1("01504", nil); got != "UE1 01504" {
		t.Errorf("peta nil (referensi gagal dibaca) harus jatuh ke kode: %q", got)
	}
}

// ---- integrasi SQL Server ----

// Data awal migrasi 052 sama dengan yang diberikan pengguna (14 UE1). Tes ini memakai database KHUSUS TES yang baru dimigrasi.
var ue1DataAwal = []struct{ kode, nama, singkatan string }{
	{"01501", "SEKRETARIAT JENDERAL", "SETJEN"},
	{"01502", "INSPEKTORAT JENDERAL", "ITJEN"},
	{"01503", "DIREKTORAT JENDERAL ANGGARAN", "DJA"},
	{"01504", "DIREKTORAT JENDERAL PAJAK", "DJP"},
	{"01505", "DIREKTORAT JENDERAL BEA DAN CUKAI", "DJBC"},
	{"01506", "DIREKTORAT JENDERAL PERIMBANGAN KEUANGAN", "DJPK"},
	{"01507", "DIREKTORAT JENDERAL PEMBIAYAAN DAN RISIKO", "DJPPR"},
	{"01508", "DIREKTORAT JENDERAL PERBENDAHARAAN", "DJPB"},
	{"01509", "DIREKTORAT JENDERAL KEKAYAAN NEGARA", "DJKN"},
	{"01511", "BADAN PENDIDIKAN DAN PELATIHAN KEUANGAN", "BPPK"},
	{"01512", "DIREKTORAT JENDERAL STRATEGI EKONOMI DAN FISKAL", "DJSEF"},
	{"01513", "LEMBAGA NATIONAL SINGLE WINDOW", "LNSW"},
	{"01514", "DIREKTORAT JENDERAL STABILITAS DAN PENGEMBANGAN SEKTOR KEUANGAN", "DJSPSK"},
	{"01515", "BADAN TEKNOLOGI, INFORMASI DAN INTELIJEN KEUANGAN", "BTIIK"},
}

func TestRefUE1DenganSQLServer(t *testing.T) {
	db := dbIntegrasi(t)
	bersih := func() {
		db.Exec(`DELETE FROM ref_ue1 WHERE kode LIKE '0999%'`)
		db.Exec(`DELETE FROM DIGITALISASI_SATKER WHERE Kode_UE1 LIKE '0999%'`)
	}
	bersih()
	if !biarkanDataUji() {
		t.Cleanup(bersih)
	}

	// ---- data awal dari migrasi
	code, body := call(routerRefUE1("user"), "GET", "/ue1", "")
	if code != 200 {
		t.Fatalf("GET: %d %v", code, body)
	}
	daftar := dataOf(t, body)["daftar"].([]interface{})
	byKode := map[string]map[string]interface{}{}
	for _, x := range daftar {
		m := x.(map[string]interface{})
		byKode[m["kode"].(string)] = m
	}
	for _, w := range ue1DataAwal {
		m := byKode[w.kode]
		if m == nil || m["nama"] != w.nama || m["singkatan"] != w.singkatan || m["aktif"] != true {
			t.Errorf("data awal %s = %v, want %s / %s", w.kode, m, w.nama, w.singkatan)
		}
	}
	if byKode["01510"] != nil {
		t.Error("01510 tidak ada pada data awal (kode itu memang tidak dipakai)")
	}
	if first := daftar[0].(map[string]interface{}); first["kode"] != "01501" {
		t.Errorf("urutan pertama = %v, want 01501 (SETJEN)", first["kode"])
	}

	// ---- pengguna biasa tidak boleh mengubah
	if code, _ := call(routerRefUE1("user"), "PUT", "/ue1/09991", `{"nama":"X"}`); code != 403 {
		t.Errorf("PUT oleh user = %d, want 403", code)
	}

	// ---- admin: tambah kode baru (urutan bawaan 100, aktif), singkatan boleh kosong
	admin := routerRefUE1("superadmin")
	code, body = call(admin, "PUT", "/ue1/09991", `{"nama":"  UNIT   BARU  ","singkatan":"UB"}`)
	if code != 200 {
		t.Fatalf("PUT baru: %d %v", code, body)
	}
	d := dataOf(t, body)
	if d["kode"] != "09991" || d["nama"] != "UNIT BARU" || d["singkatan"] != "UB" || d["urutan"] != float64(100) || d["aktif"] != true || d["diubah_oleh"] != "penguji" {
		t.Errorf("hasil PUT baru = %v", d)
	}

	// ---- ubah: urutan dan aktif tidak dikirim -> tidak berubah; kirim aktif=false lalu urutan=5
	code, body = call(admin, "PUT", "/ue1/09991", `{"nama":"UNIT BARU DIUBAH","singkatan":""}`)
	if code != 200 {
		t.Fatalf("PUT ubah: %d %v", code, body)
	}
	d = dataOf(t, body)
	if d["nama"] != "UNIT BARU DIUBAH" || d["singkatan"] != "" || d["urutan"] != float64(100) || d["aktif"] != true {
		t.Errorf("ubah tanpa urutan/aktif: %v", d)
	}
	code, body = call(admin, "PUT", "/ue1/09991", `{"nama":"UNIT BARU DIUBAH","aktif":false,"urutan":5}`)
	d = dataOf(t, body)
	if code != 200 || d["aktif"] != false || d["urutan"] != float64(5) {
		t.Errorf("nonaktifkan: %d %v", code, d)
	}

	// ---- masukan tidak sah ditolak dan tidak mengubah apa pun
	for _, c := range []struct{ kode, badan string }{
		{"9991", `{"nama":"X"}`}, {"09991", `{"nama":""}`}, {"09991", `{"nama":"X","urutan":99999}`}, {"09991", `bukan json`},
	} {
		if code, _ := call(admin, "PUT", "/ue1/"+c.kode, c.badan); code != 400 {
			t.Errorf("PUT %s %s = %d, want 400", c.kode, c.badan, code)
		}
	}
	var nama string
	db.QueryRow(`SELECT nama FROM ref_ue1 WHERE kode = '09991'`).Scan(&nama)
	if nama != "UNIT BARU DIUBAH" {
		t.Errorf("nama setelah penolakan = %q", nama)
	}

	// ---- kode di data satker yang belum terdaftar dilaporkan
	if _, err := db.Exec(`INSERT INTO DIGITALISASI_SATKER (Kode_UE1, Kode_Satker, Nama_Satker) VALUES (N'09992', N'015099999999999000KP', N'SATKER UJI A'), (N'09992', N'015099999999998000KP', N'SATKER UJI B'), (N'09991', N'015099999999997000KP', N'SATKER UJI C')`); err != nil {
		t.Fatal(err)
	}
	code, body = call(admin, "GET", "/ue1", "")
	belum := dataOf(t, body)["belum_terdaftar"].([]interface{})
	var ketemu09992, ketemu09991 bool
	for _, x := range belum {
		m := x.(map[string]interface{})
		if m["kode"] == "09992" && m["satker"] == float64(2) {
			ketemu09992 = true
		}
		if m["kode"] == "09991" {
			ketemu09991 = true
		}
	}
	if !ketemu09992 || ketemu09991 {
		t.Errorf("belum_terdaftar = %v, want 09992 (2 satker) dan bukan 09991 yang sudah terdaftar", belum)
	}

	// ---- hapus: pengguna biasa ditolak; admin menghapus; kode yang sudah tidak ada -> 404; kode tidak sah -> 400
	if code, _ := call(routerRefUE1("user"), "DELETE", "/ue1/09991", ""); code != 403 {
		t.Errorf("DELETE oleh user = %d, want 403", code)
	}
	if code, body := call(admin, "DELETE", "/ue1/09991", ""); code != 200 {
		t.Errorf("DELETE = %d %v", code, body)
	}
	if code, _ := call(admin, "DELETE", "/ue1/09991", ""); code != 404 {
		t.Errorf("DELETE ulang = %d, want 404", code)
	}
	if code, _ := call(admin, "DELETE", "/ue1/abc", ""); code != 400 {
		t.Errorf("DELETE kode tidak sah = %d, want 400", code)
	}
	if got := hitungSSO(t, db, `SELECT COUNT(*) FROM ref_ue1 WHERE kode = '09991'`); got != 0 {
		t.Errorf("09991 masih ada setelah dihapus")
	}
}

// Migrasi 052 dijalankan ulang tidak menimpa perubahan admin pada data awal, dan menyalin kode yang sudah terlanjur diisi di referensi SAPA.
func TestMigrasi052AmanDijalankanUlang(t *testing.T) {
	db := dbIntegrasi(t)
	bersih := func() {
		db.Exec(`DELETE FROM sapa_ref_ue1 WHERE kode = '09993'`)
		db.Exec(`DELETE FROM ref_ue1 WHERE kode = '09993'`)
		db.Exec(`UPDATE ref_ue1 SET singkatan = N'DJP', nama = N'DIREKTORAT JENDERAL PAJAK' WHERE kode = '01504'`)
	}
	bersih()
	if !biarkanDataUji() {
		t.Cleanup(bersih)
	}
	if _, err := db.Exec(`UPDATE ref_ue1 SET singkatan = N'DJP-UBAH', nama = N'NAMA DIUBAH ADMIN' WHERE kode = '01504'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sapa_ref_ue1 (kode, nama, sebutan_sekretaris) VALUES ('09993', N'DARI SAPA', N'Sekretaris Contoh')`); err != nil {
		t.Fatal(err)
	}

	skrip, err := os.ReadFile("../migrations/052_create_ref_ue1.sql")
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

	var nama, singkatan string
	db.QueryRow(`SELECT nama, ISNULL(singkatan, N'') FROM ref_ue1 WHERE kode = '01504'`).Scan(&nama, &singkatan)
	if nama != "NAMA DIUBAH ADMIN" || singkatan != "DJP-UBAH" {
		t.Errorf("perubahan admin tertimpa: %q / %q", nama, singkatan)
	}
	var sing sql.NullString
	if err := db.QueryRow(`SELECT nama, singkatan FROM ref_ue1 WHERE kode = '09993'`).Scan(&nama, &sing); err != nil || nama != "DARI SAPA" || sing.Valid {
		t.Errorf("kode dari SAPA = %q singkatan=%v err=%v, want disalin tanpa singkatan", nama, sing, err)
	}
	if got := hitungSSO(t, db, `SELECT COUNT(*) FROM ref_ue1 WHERE kode LIKE '015[0-1][0-9]'`); got != 14 {
		t.Errorf("jumlah kode data awal = %d, want 14 (tidak berganda)", got)
	}
}
