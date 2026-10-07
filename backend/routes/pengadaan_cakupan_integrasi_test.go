package routes

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
)

// Tes integrasi pembatasan data Pengadaan per satker (kd_satker_str) lewat router asli dan SQL Server sungguhan. Memakai database uji yang sama dengan
// peran_integrasi_test.go (aset dan pengadaan paket penyedia KLPD "UJIP" tahun 2025 dari seedAsetUji), ditambah baris tender, swakelola, dan E-Katalog V6.
//
// Data uji (satker 6 digit -> UE1 / Kanwil menurut data aset): 971001 (09971 / 099710199), 971002 (09971 / 099710199), 971003 (09971 / 099710299),
// 972001 (09972 / 099720199), dan 999999 yang tidak ada di data aset.

// seedPengadaanUji menambah data Pengadaan uji. Kode RUP paket penyedia: RUP-UJIP-0 .. RUP-UJIP-4 (urutan sama dengan seedAsetUji: 971001, 971001, 971003, 972001, 999999).
func seedPengadaanUji(t *testing.T, db *sql.DB) {
	t.Helper()
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
	}
	for i := 0; i < 5; i++ {
		exec(`UPDATE inaproc_paket_penyedia SET kd_rup = @p1 WHERE row_key = @p2`, fmt.Sprintf("RUP-UJIP-%d", i), fmt.Sprintf("UJIP-peran-%d", i))
	}
	// tender: T1 971001, T2 972001, T3 971003
	for _, x := range []struct{ kunci, satker, rup string }{{"T1", "971001", "RUP-UJIP-0"}, {"T2", "972001", "RUP-UJIP-3"}, {"T3", "971003", "RUP-UJIP-2"}} {
		exec(`INSERT INTO inaproc_tender_pengumuman (row_key, kd_klpd, tahun_anggaran, kd_satker_str, nama_satker, kd_tender, kd_rup, pagu, hps, versi_tender) VALUES (@p1, N'UJIP', N'2025', @p2, @p3, @p4, @p5, 1000, 900, 1)`,
			"UJIP-tender-"+x.kunci, x.satker, "SATKER "+x.satker, "UJIP-"+x.kunci, x.rup)
	}
	// nilai tender selesai: tanpa kolom kode satker (dihubungkan lewat kode tender); N4 yatim (tendernya tidak ada di pengumuman maupun tender selesai)
	for _, x := range []struct{ kunci, tender, penyedia string }{{"N1", "UJIP-T1", "PT A"}, {"N2", "UJIP-T2", "PT B"}, {"N3", "UJIP-T3", "PT C"}, {"N4", "UJIP-TX", "PT D"}} {
		exec(`INSERT INTO inaproc_tender_selesai_nilai (row_key, kd_klpd, tahun_anggaran, kd_satker, kd_tender, nama_penyedia, hps, nilai_kontrak) VALUES (@p1, N'UJIP', N'2025', N'123.456', @p2, @p3, 1000, 900)`,
			"UJIP-nilai-"+x.kunci, x.tender, x.penyedia)
	}
	// swakelola: SW1 punya padanan di swakelola terumumkan (satker 971001), SW2 yatim
	exec(`INSERT INTO inaproc_paket_swakelola_terumumkan (row_key, kd_klpd, tahun_anggaran, kd_satker_str, nama_satker, kd_rup, pagu, status_delete_rup, status_aktif_rup) VALUES (N'UJIP-swt-1', N'UJIP', N'2025', N'971001', N'SATKER 971001', N'UJIP-SW1', 500, 0, 1)`)
	for _, k := range []string{"SW1", "SW2"} {
		exec(`INSERT INTO inaproc_paket_swakelola (row_key, kd_klpd, tahun_anggaran, kd_satker, kd_rup, nama_paket) VALUES (@p1, N'UJIP', N'2025', N'777', @p2, @p3)`, "UJIP-sw-"+k, "UJIP-"+k, "Swakelola "+k)
	}
	// program master: tak dapat dibatasi per satker
	exec(`INSERT INTO inaproc_program_master (row_key, kd_klpd, tahun_anggaran, kd_satker, pagu_program, is_deleted) VALUES (N'UJIP-prog-1', N'UJIP', N'2025', N'777', 5000, 0)`)
	// E-Katalog V6: E1 -> RUP 1 (971001), E2 -> RUP 0 (971001) dan RUP 3 (972001), E3 tanpa kode RUP (swasta), E4 -> RUP 2 (971003) di antara kode lain
	for _, x := range []struct{ kunci, rup string }{{"E1", "RUP-UJIP-1"}, {"E2", "RUP-UJIP-0; RUP-UJIP-3"}, {"E3", ""}, {"E4", "XX-LAIN; RUP-UJIP-2"}} {
		var rup interface{} = x.rup
		if x.rup == "" {
			rup = nil
		}
		exec(`INSERT INTO inaproc_ekatalog6_paket_epurchasing (row_key, kode_klpd, fiscal_year, order_id, kode_satker, rup_code, status, total, is_swasta) VALUES (@p1, N'UJIP', N'2025', @p2, N'xyz', @p3, N'selesai', 100, 0)`,
			"UJIP-ek6-"+x.kunci, "UJIP-O-"+x.kunci, rup)
	}
}

func bersihkanPengadaanUji(db *sql.DB) {
	for _, tabel := range []string{
		"inaproc_paket_penyedia", "inaproc_tender_pengumuman", "inaproc_tender_selesai_nilai", "inaproc_paket_swakelola_terumumkan", "inaproc_paket_swakelola",
		"inaproc_program_master",
	} {
		db.Exec(`DELETE FROM ` + tabel + ` WHERE kd_klpd = N'UJIP'`)
	}
	db.Exec(`DELETE FROM inaproc_ekatalog6_paket_epurchasing WHERE kode_klpd = N'UJIP'`)
}

// jumlah baris dataset (query kode_klpd=UJIP&tahun=2025) menurut peran pengguna; -1 bila ditolak.
func totalDataPengadaan(t *testing.T, p penggunaUji, dataset string) int {
	t.Helper()
	code, body, _ := panggil(t, "GET", "/inaproc/data/"+dataset+"?kode_klpd=UJIP&tahun=2025&per_halaman=100", p.token, "")
	if code == 403 {
		return -1
	}
	if code != 200 {
		t.Fatalf("data %s: %d %v", dataset, code, body)
	}
	return int(data(t, body)["total"].(float64))
}

func analitikPengadaan(t *testing.T, p penggunaUji) map[string]interface{} {
	t.Helper()
	code, body, _ := panggil(t, "GET", "/inaproc/analitik?kode_klpd=UJIP&tahun=2025&segarkan=1", p.token, "")
	if code != 200 {
		t.Fatalf("analitik: %d %v", code, body)
	}
	return data(t, body)
}

func angka(t *testing.T, m map[string]interface{}, jalur ...string) float64 {
	t.Helper()
	var cur interface{} = m
	for _, k := range jalur {
		mm, ok := cur.(map[string]interface{})
		if !ok {
			t.Fatalf("jalur %v terputus di %q: %v", jalur, k, cur)
		}
		cur = mm[k]
	}
	f, ok := cur.(float64)
	if !ok {
		t.Fatalf("jalur %v bukan angka: %v", jalur, cur)
	}
	return f
}

func TestPengadaanDibatasiPerSatkerDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	bersihkanUjiPeran(db)
	bersihkanPengadaanUji(db)
	if os.Getenv("PASTI_UJI_BIARKAN_DATA") != "1" {
		t.Cleanup(func() { bersihkanUjiPeran(db); bersihkanPengadaanUji(db) })
	}
	seedAsetUji(t, db)
	seedPengadaanUji(t, db)

	admin := buatPenggunaUji(t, db, "admin", "superadmin")
	sebagai := func(nama, role, kode string) penggunaUji {
		p := buatPenggunaUji(t, db, nama, "user")
		pilihPeran(t, p, beriPeran(t, admin, p, role, kode))
		return p
	}
	barang := sebagai("pgd-barang", "pengguna_barang", "")
	satker1 := sebagai("pgd-satker1", "satker", "971001")
	satker2 := sebagai("pgd-satker2", "satker", "971002")
	satkerLuar := sebagai("pgd-satker-luar", "satker", "999999")
	kanwil1 := sebagai("pgd-kanwil1", "kanwil", "099710199")
	kanwil2 := sebagai("pgd-kanwil2", "kanwil", "099710299")
	ue1a := sebagai("pgd-ue1a", "ue1", ue1A)
	ue1b := sebagai("pgd-ue1b", "ue1", ue1B)

	t.Run("paket penyedia (kd_satker_str langsung)", func(t *testing.T) {
		for _, c := range []struct {
			nama string
			p    penggunaUji
			want int
		}{
			{"admin", admin, 5}, {"pengguna barang", barang, 5},
			{"satker 971001", satker1, 2}, {"satker 971002 (tanpa pengadaan)", satker2, 0},
			{"satker 999999 (tidak ada di data aset)", satkerLuar, 1},
			{"kanwil 099710199", kanwil1, 2}, {"kanwil 099710299", kanwil2, 1},
			{"ue1 09971", ue1a, 3}, {"ue1 09972", ue1b, 1},
		} {
			if got := totalDataPengadaan(t, c.p, "rup/paket-penyedia"); got != c.want {
				t.Errorf("%s: %d paket penyedia, want %d", c.nama, got, c.want)
			}
		}
	})

	t.Run("dataset yang dihubungkan lewat kode paket", func(t *testing.T) {
		for _, c := range []struct {
			dataset string
			want    map[string]int // nama peran -> jumlah
		}{
			// nilai tender selesai: lewat kode tender; N4 yatim hanya terlihat oleh yang melihat semua
			{"tender/tender-selesai-nilai", map[string]int{"admin": 4, "satker1": 1, "kanwil1": 1, "kanwil2": 1, "ue1a": 2, "ue1b": 1, "satker2": 0}},
			// swakelola: lewat kode RUP swakelola terumumkan; SW2 yatim
			{"rup/paket-swakelola", map[string]int{"admin": 2, "satker1": 1, "kanwil1": 1, "ue1a": 1, "ue1b": 0, "kanwil2": 0}},
			// E-Katalog V6: lewat kode RUP paket penyedia (bisa banyak kode); E3 tanpa kode RUP tidak ikut bagi peran terbatas
			{"ekatalog/paket-e-purchasing", map[string]int{"admin": 4, "satker1": 2, "kanwil1": 2, "kanwil2": 1, "ue1a": 3, "ue1b": 1, "satker2": 0}},
		} {
			peranUji := map[string]penggunaUji{"admin": admin, "satker1": satker1, "satker2": satker2, "kanwil1": kanwil1, "kanwil2": kanwil2, "ue1a": ue1a, "ue1b": ue1b}
			for nama, want := range c.want {
				if got := totalDataPengadaan(t, peranUji[nama], c.dataset); got != want {
					t.Errorf("%s oleh %s: %d baris, want %d", c.dataset, nama, got, want)
				}
			}
		}
	})

	t.Run("dataset yang tidak dapat dibatasi tertutup bagi peran terbatas", func(t *testing.T) {
		for _, ds := range []string{"rup/program-master", "ekatalog/e-purchasing-by-produk", "ekatalog-archive/komoditas-detail", "tender/non-tender-ekontrak"} {
			for nama, p := range map[string]penggunaUji{"satker1": satker1, "kanwil1": kanwil1, "ue1a": ue1a} {
				if code, _, _ := panggil(t, "GET", "/inaproc/data/"+ds, p.token, ""); code != 403 {
					t.Errorf("data %s oleh %s = %d, want 403", ds, nama, code)
				}
				if code, _, _ := panggil(t, "GET", "/inaproc/ekspor/"+ds+"?format=csv", p.token, ""); code != 403 {
					t.Errorf("ekspor %s oleh %s = %d, want 403", ds, nama, code)
				}
			}
			for nama, p := range map[string]penggunaUji{"admin": admin, "barang": barang} {
				if code, _, _ := panggil(t, "GET", "/inaproc/data/"+ds, p.token, ""); code != 200 {
					t.Errorf("data %s oleh %s = %d, want 200", ds, nama, code)
				}
			}
		}
		if totalDataPengadaan(t, admin, "rup/program-master") != 1 {
			t.Error("admin harus melihat program master uji")
		}
	})

	t.Run("daftar dataset dan tahun tersedia mengikuti cakupan", func(t *testing.T) {
		ids := func(p penggunaUji) (map[string]float64, map[string]interface{}) {
			code, body, _ := panggil(t, "GET", "/inaproc/dataset", p.token, "")
			if code != 200 {
				t.Fatalf("daftar dataset: %d %v", code, body)
			}
			d := data(t, body)
			out := map[string]float64{}
			for _, x := range d["datasets"].([]interface{}) {
				m := x.(map[string]interface{})
				out[m["id"].(string)] = m["baris"].(float64)
			}
			return out, d
		}
		semua, _ := ids(admin)
		if _, ada := semua["rup/program-master"]; !ada {
			t.Error("admin harus melihat program master di daftar dataset")
		}
		terbatas, d := ids(satker1)
		if _, ada := terbatas["rup/program-master"]; ada {
			t.Error("satker tidak boleh melihat program master di daftar dataset")
		}
		if _, ada := terbatas["ekatalog-archive/komoditas-detail"]; ada {
			t.Error("satker tidak boleh melihat rujukan komoditas di daftar dataset")
		}
		if len(terbatas) >= len(semua) || len(terbatas) < 10 {
			t.Errorf("daftar dataset satker = %d, semua = %d: harus lebih sedikit tetapi tetap memuat dataset yang dapat dibatasi", len(terbatas), len(semua))
		}
		if cak, _ := d["cakupan"].(map[string]interface{}); cak["tingkat"] != "satker" || cak["kode"] != "971001" {
			t.Errorf("cakupan di daftar dataset = %v", d["cakupan"])
		}
		// jumlah baris per dataset memakai cakupan peran (bukan seluruh tabel)
		if terbatas["rup/paket-penyedia"] != 2 || semua["rup/paket-penyedia"] < 5 {
			t.Errorf("baris paket penyedia: satker = %v (want 2), semua = %v (want >= 5)", terbatas["rup/paket-penyedia"], semua["rup/paket-penyedia"])
		}
		// tahun tersedia di tampilan data hanya dari baris dalam cakupan
		_, body, _ := panggil(t, "GET", "/inaproc/data/rup/paket-penyedia", satker2.token, "")
		if th, _ := data(t, body)["tahun_tersedia"].([]interface{}); len(th) != 0 {
			t.Errorf("tahun tersedia satker tanpa data = %v, want kosong", th)
		}
		_, body, _ = panggil(t, "GET", "/inaproc/data/rup/paket-penyedia", satker1.token, "")
		if th, _ := data(t, body)["tahun_tersedia"].([]interface{}); len(th) != 1 || th[0] != "2025" {
			t.Errorf("tahun tersedia satker 971001 = %v, want [2025]", th)
		}
	})

	t.Run("dasbor dibatasi dan tidak bocor lewat cache", func(t *testing.T) {
		// admin lebih dulu (mengisi cache), lalu peran terbatas dengan permintaan tanpa `segarkan` agar cache diuji
		hAdmin := analitikPengadaan(t, admin)
		if hAdmin["hasil"].(map[string]interface{})["batas"] != nil {
			t.Error("admin tidak boleh memuat batas cakupan")
		}
		if n := angka(t, hAdmin, "hasil", "rup", "total_paket"); n != 5 {
			t.Errorf("admin: total paket RUP %v, want 5", n)
		}
		if n := angka(t, hAdmin, "hasil", "rup", "pagu_program"); n != 5000 {
			t.Errorf("admin: pagu program %v, want 5000", n)
		}
		code, body, _ := panggil(t, "GET", "/inaproc/analitik?kode_klpd=UJIP&tahun=2025", satker1.token, "")
		if code != 200 {
			t.Fatalf("analitik satker (setelah admin): %d", code)
		}
		if n := angka(t, data(t, body), "hasil", "rup", "total_paket"); n != 2 {
			t.Errorf("satker 971001 setelah admin: total paket RUP %v, want 2 (cache tidak boleh dibagi antar cakupan)", n)
		}

		for _, c := range []struct {
			nama                           string
			p                              penggunaUji
			tingkat, kode                  string
			rup, swakelola, tender, corong float64
			kontrakNilai                   float64
			tahunKosong                    bool
		}{
			{"satker 971001", satker1, "satker", "971001", 2, 1, 1, 2, 900, false},
			{"satker 971002", satker2, "satker", "971002", 0, 0, 0, 0, 0, true},
			{"kanwil 099710199", kanwil1, "kanwil", "099710199", 2, 1, 1, 2, 900, false},
			{"ue1 09971", ue1a, "ue1", ue1A, 3, 1, 2, 3, 1800, false},
			{"ue1 09972", ue1b, "ue1", ue1B, 1, 0, 1, 1, 900, false},
		} {
			h := analitikPengadaan(t, c.p)
			hasil := h["hasil"].(map[string]interface{})
			// semua bagian harus berjalan: SQL berbatas harus sah di SQL Server sungguhan
			if g := hasil["galat"]; g != nil {
				t.Errorf("%s: bagian dasbor gagal: %v", c.nama, g)
			}
			batas, _ := hasil["batas"].(map[string]interface{})
			if batas["tingkat"] != c.tingkat || batas["kode"] != c.kode {
				t.Errorf("%s: batas = %v, want %s %s", c.nama, batas, c.tingkat, c.kode)
			}
			if ts, _ := hasil["tidak_tersedia"].([]interface{}); len(ts) == 0 {
				t.Errorf("%s: tidak_tersedia kosong, want bagian yang tak dapat dibatasi disebut", c.nama)
			}
			if n := angka(t, h, "hasil", "rup", "total_paket"); n != c.rup {
				t.Errorf("%s: total paket RUP %v, want %v", c.nama, n, c.rup)
			}
			if n := angka(t, h, "hasil", "rup", "paket_swakelola"); n != c.swakelola {
				t.Errorf("%s: paket swakelola %v, want %v", c.nama, n, c.swakelola)
			}
			if n := angka(t, h, "hasil", "rup", "pagu_program"); n != 0 {
				t.Errorf("%s: pagu program %v, want 0 (tak dapat dibatasi)", c.nama, n)
			}
			if n := angka(t, h, "hasil", "pemilihan", "tender_jumlah"); n != c.tender {
				t.Errorf("%s: tender %v, want %v", c.nama, n, c.tender)
			}
			if n := angka(t, h, "hasil", "pemilihan", "nilai_kontrak"); n != c.kontrakNilai {
				t.Errorf("%s: nilai kontrak %v, want %v", c.nama, n, c.kontrakNilai)
			}
			if n := angka(t, h, "hasil", "corong", "total_paket"); n != c.corong {
				t.Errorf("%s: corong %v paket, want %v", c.nama, n, c.corong)
			}
			if n := angka(t, h, "hasil", "ekatalog", "v6", "transaksi_baris"); n != 0 {
				t.Errorf("%s: transaksi per produk %v, want 0 (tak dapat dibatasi)", c.nama, n)
			}
			tahun, _ := h["tahun_tersedia"].([]interface{})
			if c.tahunKosong && len(tahun) != 0 || !c.tahunKosong && (len(tahun) != 1 || tahun[0] != "2025") {
				t.Errorf("%s: tahun tersedia = %v", c.nama, tahun)
			}
		}
	})

	t.Run("ekspor dibatasi ke satker", func(t *testing.T) {
		baris := func(p penggunaUji) int {
			code, _, raw := panggil(t, "GET", "/inaproc/ekspor/rup/paket-penyedia?format=csv&kode_klpd=UJIP&tahun=2025", p.token, "")
			if code != 200 {
				t.Fatalf("ekspor: %d", code)
			}
			return len(strings.Split(strings.TrimSpace(string(raw)), "\n")) - 1 // tanpa baris judul
		}
		if n := baris(satker1); n != 2 {
			t.Errorf("ekspor satker 971001 = %d baris, want 2", n)
		}
		if n := baris(ue1a); n != 3 {
			t.Errorf("ekspor ue1 09971 = %d baris, want 3", n)
		}
		if n := baris(admin); n != 5 {
			t.Errorf("ekspor admin = %d baris, want 5", n)
		}
	})

	t.Run("keadaan penarikan tetap hanya untuk peran semua data", func(t *testing.T) {
		for nama, p := range map[string]penggunaUji{"satker1": satker1, "kanwil1": kanwil1, "ue1a": ue1a} {
			for _, path := range []string{"/inaproc/penarikan", "/inaproc/penarikan/aktif", "/inaproc/penarikan/riwayat"} {
				if code, _, _ := panggil(t, "GET", path, p.token, ""); code != 403 {
					t.Errorf("GET %s oleh %s = %d, want 403", path, nama, code)
				}
			}
			if code, _, _ := panggil(t, "POST", "/inaproc/penarikan", p.token, `{}`); code != 403 {
				t.Errorf("memulai penarikan oleh %s = %d, want 403", nama, code)
			}
		}
		for nama, p := range map[string]penggunaUji{"admin": admin, "barang": barang} {
			if code, _, _ := panggil(t, "GET", "/inaproc/penarikan", p.token, ""); code == 403 || code == 401 {
				t.Errorf("GET /inaproc/penarikan oleh %s = %d, want tidak ditolak", nama, code)
			}
		}
	})
}
