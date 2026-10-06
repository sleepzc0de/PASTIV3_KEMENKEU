package handlers

import (
	"database/sql"
	"testing"

	"github.com/gin-gonic/gin"
)

func routerSatker() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/satker/keterhubungan", GetSatkerKeterhubungan)
	return r
}

func TestGabungSatker(t *testing.T) {
	ref := map[string]RefUE1Baris{"01504": {Kode: "01504", Nama: "DIREKTORAT JENDERAL PAJAK", Singkatan: "DJP"}}
	aset := map[string]*asetSatker{
		"119091": {Nama: "KPP A", KodeUE1: "01504", Jenis: "INDUK SATKER", KDJ: 3, KDO: 1, JumlahKode: 2, PerDataset: map[string]int64{"tanah": 3, "gedung_lainnya": 1}},
		"200200": {Nama: "KANTOR B", KodeUE1: "01599", Jenis: "INDUK SATKER", JumlahKode: 1, PerDataset: map[string]int64{"tanah": 1}},
		"300300": {PerDataset: map[string]int64{"rusunara": 2}}, // hanya muncul di tabel aset, tanpa baris di daftar satker
	}
	peng := map[string]*pengadaanSatker{
		"119091": {Nama: "Kantor Pelayanan Pajak A ", RUPPaket: 2, RUPPagu: 1500000, Tender: 1, TenderPagu: 900000},
		"999999": {Nama: "SATKER TANPA ASET", RUPPaket: 1, RUPPagu: 100},
		"200200": {Nama: "KANTOR B"}, // kunci ada tetapi tanpa angka pengadaan: tidak dihitung sebagai punya pengadaan
	}
	satker, r := gabungSatker(aset, peng, ref)

	if len(satker) != 4 {
		t.Fatalf("jumlah satker = %d, want 4 (119091, 200200, 300300, 999999)", len(satker))
	}
	for i := 1; i < len(satker); i++ {
		if satker[i-1].Kode >= satker[i].Kode {
			t.Errorf("hasil harus terurut menurut kode: %s sebelum %s", satker[i-1].Kode, satker[i].Kode)
		}
	}
	by := map[string]SatkerTerhubung{}
	for _, s := range satker {
		by[s.Kode] = s
	}
	a := by["119091"]
	if a.Status != statusTerhubung || a.JumlahAset != 4 || a.UE1 != "01504 · DJP" || a.KDJ != 3 || a.Pengadaan.RUPPaket != 2 || a.Pengadaan.TenderPagu != 900000 || a.JumlahKode != 2 {
		t.Errorf("119091 = %+v", a)
	}
	if a.NamaPengadaan != "Kantor Pelayanan Pajak A " {
		t.Errorf("nama menurut Inaproc yang berbeda harus ikut dicatat: %q", a.NamaPengadaan)
	}
	if b := by["200200"]; b.Status != statusHanyaAset || b.UE1 != "UE1 01599" || b.NamaPengadaan != "" {
		t.Errorf("200200 = %+v, want hanya_aset, UE1 belum terdaftar, tanpa nama pengadaan (namanya sama)", b)
	}
	if c := by["300300"]; c.Status != statusHanyaAset || c.Nama != "" || c.JumlahAset != 2 || c.UE1 != "" {
		t.Errorf("300300 = %+v, want hanya_aset tanpa nama/UE1", c)
	}
	if d := by["999999"]; d.Status != statusHanyaPengadaan || d.Nama != "SATKER TANPA ASET" || d.JumlahAset != 0 || d.Aset == nil {
		t.Errorf("999999 = %+v, want hanya_pengadaan dengan nama dari Inaproc dan peta aset tidak nil", d)
	}
	if r.SatkerAset != 3 || r.SatkerPengadaan != 2 || r.Terhubung != 1 || r.HanyaAset != 2 || r.HanyaPengadaan != 1 {
		t.Errorf("ringkasan = %+v", r)
	}
	if r.SatkerAset != r.Terhubung+r.HanyaAset || r.SatkerPengadaan != r.Terhubung+r.HanyaPengadaan {
		t.Errorf("ringkasan tidak konsisten: %+v", r)
	}

	// data kosong tidak membuat galat dan menghasilkan daftar kosong (bukan nil, supaya JSON-nya [])
	kosong, rk := gabungSatker(nil, nil, nil)
	if kosong == nil || len(kosong) != 0 || rk != (RingkasanKeterhubungan{}) {
		t.Errorf("kosong = %v %+v", kosong, rk)
	}
}

// ---- integrasi SQL Server ----

var kunciSatkerUji = []string{"911001", "911002", "012345", "911003"}

func bersihkanSatkerUji(db *sql.DB) {
	for _, tabel := range []string{"DIGITALISASI_SATKER", "DIGITALISASI_TANAH", "DIGITALISASI_GEDUNG_LAINNYA"} {
		db.Exec(`DELETE FROM ` + tabel + ` WHERE Kode_Satker LIKE N'015%' AND SUBSTRING(Kode_Satker, 10, 6) IN ('911001','911002','012345','911003') OR Kode_Satker = N'PENDEK-UJI'`)
	}
	db.Exec(`DELETE FROM ref_ue1 WHERE kode = '09994'`)
	db.Exec("DELETE FROM inaproc_paket_penyedia WHERE kd_klpd = 'LAIN' AND row_key LIKE 'UJI-uji-%'") // baris KLPD lain milik tes ini
}

func TestSatkerKeterhubunganDenganSQLServer(t *testing.T) {
	db := dbIntegrasi(t)
	bersihkanUji(t, db)
	bersihkanSatkerUji(db)
	if !biarkanDataUji() {
		t.Cleanup(func() { bersihkanUji(t, db); bersihkanSatkerUji(db) })
	}

	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
	}
	exec(`INSERT INTO ref_ue1 (kode, nama, singkatan) VALUES ('09994', N'UNIT UJI', N'UU')`)

	// Sisi aset. 911001: induk + anak (kode 6 digit sama), dua baris tanah dan satu gedung; 911002: satu tanah; 012345: kode berawalan nol.
	exec(`INSERT INTO DIGITALISASI_SATKER (Kode_UE1, Kode_Satker, Jenis_Satker, Nama_Satker, Jumlah_KDJ, Jumlah_KDO) VALUES
		(N'09994', N'015099991911001001KP', N'ANAK SATKER', N'ANAK SATKER UJI', 1, 0),
		(N'09994', N'015099991911001000KP', N'INDUK SATKER', N'INDUK SATKER UJI', 2, 1),
		(N'09994', N'015099991911002000KP', N'INDUK SATKER', N'SATKER UJI B', 0, 0),
		(N'01504', N'015040199012345000KP', N'INDUK SATKER', N'SATKER UJI NOL DEPAN', 0, 0),
		(N'01504', N'PENDEK-UJI', N'INDUK SATKER', N'KODE TERLALU PENDEK', 0, 0)`)
	exec(`INSERT INTO DIGITALISASI_TANAH (Kode_UE1, Kode_Satker) VALUES (N'09994', N'015099991911001000KP'), (N'09994', N'015099991911001000KP'),
		(N'09994', N'015099991911002000KP'), (N'01504', N'PENDEK-UJI')`)
	exec(`INSERT INTO DIGITALISASI_GEDUNG_LAINNYA (Kode_UE1, Kode_Satker) VALUES (N'09994', N'015099991911001001KP')`)

	// Sisi pengadaan untuk KLPD uji dan 2025. 911001: 2 RUP aktif (satu lagi dihapus, satu tahun lain, satu KLPD lain), tender dua versi, 1 non-tender.
	th := "2025"
	rup := func(klpd, tahun, satker string, pagu int64, hapus int) {
		sisip(t, db, "inaproc_paket_penyedia", map[string]interface{}{"kd_klpd": klpd, "tahun_anggaran": tahun, "kd_satker_str": satker, "nama_satker": "SATKER " + satker,
			"pagu": pagu, "status_delete_rup": hapus, "status_aktif_rup": 1})
	}
	rup(klpdUji, th, "911001", 1000000, 0)
	rup(klpdUji, th, "911001", 500000, 0)
	rup(klpdUji, th, "911001", 9999999, 1) // dihapus
	rup(klpdUji, "2024", "911001", 777, 0) // tahun lain
	rup("LAIN", th, "911001", 888, 0)      // KLPD lain
	rup(klpdUji, th, "12345", 300, 0)      // 5 digit: dilengkapi nol di depan -> 012345
	rup(klpdUji, th, "911003", 100, 0)     // tidak ada di data aset
	rup(klpdUji, th, "  ", 5, 0)           // kode kosong tidak dihitung
	for _, v := range []int{1, 2} {
		sisip(t, db, "inaproc_tender_pengumuman", map[string]interface{}{"kd_klpd": klpdUji, "tahun_anggaran": th, "kd_tender": "UJI-T1", "versi_tender": v,
			"kd_satker_str": "911001", "nama_satker": "SATKER 911001", "pagu": 2000 * v})
	}
	sisip(t, db, "inaproc_non_tender_pengumuman", map[string]interface{}{"kd_klpd": klpdUji, "tahun_anggaran": th, "kd_nontender": "UJI-N1", "versi_nontender": 1,
		"kd_satker_str": "911001", "nama_satker": "SATKER 911001", "pagu": 50})

	code, body := call(routerSatker(), "GET", "/satker/keterhubungan?kode_klpd="+klpdUji+"&tahun="+th, "")
	if code != 200 {
		t.Fatalf("status %d: %v", code, body)
	}
	d := dataOf(t, body)
	if d["tahun"] != th || d["kode_klpd"] != klpdUji {
		t.Errorf("tahun/klpd = %v/%v", d["tahun"], d["kode_klpd"])
	}
	by := map[string]map[string]interface{}{}
	for _, x := range d["satker"].([]interface{}) {
		m := x.(map[string]interface{})
		by[m["kode"].(string)] = m
	}

	a := by["911001"]
	if a == nil {
		t.Fatalf("911001 tidak ada. satker: %v", d["satker"])
	}
	aset := a["aset"].(map[string]interface{})
	peng := a["pengadaan"].(map[string]interface{})
	if a["status"] != statusTerhubung || a["nama"] != "INDUK SATKER UJI" || a["kode_ue1"] != "09994" || a["ue1"] != "09994 · UU" || a["jumlah_kode_aset"] != float64(2) {
		t.Errorf("911001 = %v, want terhubung, nama dan UE1 dari induk, 2 kode (induk + anak)", a)
	}
	if aset["tanah"] != float64(2) || aset["gedung_lainnya"] != float64(1) || a["jumlah_aset"] != float64(3) || a["kdj"] != float64(3) || a["kdo"] != float64(1) {
		t.Errorf("aset 911001 = %v jumlah=%v kdj=%v kdo=%v", aset, a["jumlah_aset"], a["kdj"], a["kdo"])
	}
	if peng["rup_paket"] != float64(2) || peng["rup_pagu"] != 1500000.0 {
		t.Errorf("RUP 911001 = %v, want 2 paket aktif Rp 1.500.000 (yang dihapus, tahun lain, KLPD lain tidak dihitung)", peng)
	}
	if peng["tender"] != float64(1) || peng["tender_pagu"] != 4000.0 {
		t.Errorf("tender 911001 = %v, want 1 tender (versi terbaru, pagu 4000)", peng)
	}
	if peng["non_tender"] != float64(1) || peng["non_tender_pagu"] != 50.0 {
		t.Errorf("non-tender 911001 = %v", peng)
	}

	if b := by["911002"]; b == nil || b["status"] != statusHanyaAset || b["jumlah_aset"] != float64(1) {
		t.Errorf("911002 = %v, want hanya_aset", b)
	}
	if n := by["012345"]; n == nil || n["status"] != statusTerhubung || n["pengadaan"].(map[string]interface{})["rup_paket"] != float64(1) {
		t.Errorf("012345 = %v, want terhubung lewat kd_satker_str 12345 yang dilengkapi nol di depan", n)
	}
	if p := by["911003"]; p == nil || p["status"] != statusHanyaPengadaan || p["nama"] != "SATKER 911003" || p["jumlah_aset"] != float64(0) {
		t.Errorf("911003 = %v, want hanya_pengadaan dengan nama dari Inaproc", p)
	}
	if by[""] != nil || by["      "] != nil || by["000000"] != nil {
		t.Error("kode Inaproc kosong tidak boleh menjadi satker")
	}

	ring := d["ringkasan"].(map[string]interface{})
	if ring["aset_tanpa_kode"].(float64) < 2 { // satker PENDEK-UJI dan tanah PENDEK-UJI
		t.Errorf("aset_tanpa_kode = %v, want >= 2", ring["aset_tanpa_kode"])
	}
	if ring["terhubung"].(float64) < 2 || ring["hanya_aset"].(float64) < 1 || ring["hanya_pengadaan"].(float64) < 1 {
		t.Errorf("ringkasan = %v", ring)
	}

	// tahun tanpa data pengadaan: semua satker uji menjadi hanya_aset (tidak galat)
	_, body = call(routerSatker(), "GET", "/satker/keterhubungan?kode_klpd="+klpdUji+"&tahun=2019", "")
	for _, x := range dataOf(t, body)["satker"].([]interface{}) {
		m := x.(map[string]interface{})
		if m["kode"] == "911001" && m["status"] != statusHanyaAset {
			t.Errorf("911001 pada 2019 = %v, want hanya_aset", m["status"])
		}
	}

	// masukan tidak sah
	for _, url := range []string{"/satker/keterhubungan?tahun=20x5", "/satker/keterhubungan?kode_klpd=ABCDEFGHIJKLMNOPQRSTUVWXYZ"} {
		if code, _ := call(routerSatker(), "GET", url, ""); code != 400 {
			t.Errorf("GET %s = %d, want 400", url, code)
		}
	}
}
