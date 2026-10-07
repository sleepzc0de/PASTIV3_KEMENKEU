package routes

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"pasti-v3-backend/sapa"
)

// Tes integrasi (router asli + SQL Server sungguhan; hanya berjalan bila PASTI_UJI_MSSQL_DSN diisi, lihat bukaDBUji): tab Sinkronisasi Digitalisasi Aset tertutup bagi
// UE1, Kanwil, dan Satker, serta kunci usulan SAPA yang sudah selesai (buka kunci oleh Pengguna Barang atau superadmin, hapus hanya selama belum selesai).

func TestSinkronisasiDigitalisasiTertutupBagiPeranTerbatasDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	bersihkanUjiPeran(db)
	if os.Getenv("PASTI_UJI_BIARKAN_DATA") != "1" {
		t.Cleanup(func() { bersihkanUjiPeran(db) })
	}
	admin := buatPenggunaUji(t, db, "admin", "superadmin")

	for _, c := range []struct {
		nama, peran, kode string
		boleh             bool
	}{
		{"barang", "pengguna_barang", "", true},
		{"ue1", "ue1", "09971", false},
		{"kanwil", "kanwil", "099710199", false},
		{"satker", "satker", "971001", false},
	} {
		p := buatPenggunaUji(t, db, "sinkron-"+c.nama, "user")
		beriPeran(t, admin, p, c.peran, c.kode)
		code, body, _ := panggil(t, "GET", "/digitalisasi/sinkronisasi", p.token, "")
		want := http.StatusForbidden
		if c.boleh {
			want = http.StatusOK
		}
		if code != want {
			t.Errorf("%s: GET /digitalisasi/sinkronisasi = %d %v, want %d", c.nama, code, body["message"], want)
		}
		if !c.boleh {
			if msg, _ := body["message"].(string); !strings.Contains(msg, "Pengguna Barang dan superadmin") {
				t.Errorf("%s: pesan penolakan = %q", c.nama, msg)
			}
		}
		// Bagian lain Digitalisasi Aset tetap terbuka, dan menjalankan atau membatalkan sinkronisasi tetap khusus superadmin.
		for _, path := range []string{"/digitalisasi/ringkasan", "/digitalisasi/peta?dataset=tanah", "/digitalisasi/data/tanah"} {
			if code, _, _ := panggil(t, "GET", path, p.token, ""); code != http.StatusOK {
				t.Errorf("%s: GET %s = %d, want 200", c.nama, path, code)
			}
		}
		for _, path := range []string{"/digitalisasi/sinkronisasi", "/digitalisasi/sinkronisasi/batal"} {
			if code, _, _ := panggil(t, "POST", path, p.token, `{"semua":true}`); code != http.StatusForbidden {
				t.Errorf("%s: POST %s = %d, want 403", c.nama, path, code)
			}
		}
	}

	// Tamu (belum punya peran) ditolak di pintu.
	tamu := buatPenggunaUji(t, db, "sinkron-tamu", "user")
	if code, body, _ := panggil(t, "GET", "/digitalisasi/sinkronisasi", tamu.token, ""); code != http.StatusForbidden || body["code"] != "tamu" {
		t.Errorf("tamu: %d %v", code, body)
	}

	// Superadmin melihat status sinkronisasi; yang menentukan adalah peran aktif, jadi superadmin yang bertindak sebagai Satker tidak melihatnya.
	if code, body, _ := panggil(t, "GET", "/digitalisasi/sinkronisasi", admin.token, ""); code != http.StatusOK {
		t.Errorf("superadmin: %d %v, want 200", code, body)
	}
	id := beriPeran(t, admin, admin, "satker", "972001")
	pilihPeran(t, admin, id)
	if code, _, _ := panggil(t, "GET", "/digitalisasi/sinkronisasi", admin.token, ""); code != http.StatusForbidden {
		t.Errorf("superadmin sebagai Satker: %d, want 403", code)
	}
	pilihPeran(t, admin, nil)
	if code, _, _ := panggil(t, "GET", "/digitalisasi/sinkronisasi", admin.token, ""); code != http.StatusOK {
		t.Errorf("superadmin kembali ke peran bawaan: %d, want 200", code)
	}
}

func TestSapaKunciUsulanDanHapusDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	bersihkanUjiPeran(db)
	hapusUsulanUji := func() {
		db.Exec(`DELETE FROM sapa_penjualan WHERE kode_satker LIKE N'09971%' OR kode_satker LIKE N'09972%'`)
	}
	hapusUsulanUji()
	if os.Getenv("PASTI_UJI_BIARKAN_DATA") != "1" {
		t.Cleanup(func() {
			hapusUsulanUji()
			bersihkanUjiPeran(db)
		})
	}
	ctx := context.Background()
	s := sapa.NewStore(db)
	const kodeUji = "099710199971001000" // UE1 09971, Kanwil 099710199, satker 971001

	buat := func() sapa.KasusInfo {
		t.Helper()
		k, err := s.BuatPenjualan(ctx, sapa.BuatInput{KodeSatker: kodeUji, NamaSatker: "SATKER UJI KUNCI", KodeUE1: sapa.KodeUE1Dari(kodeUji), Oleh: "uji"})
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	// n tahap pertama selesai (tahap kedua dilewati supaya kedua status terhitung).
	selesaikan := func(k sapa.KasusInfo, n int) {
		t.Helper()
		for i, td := range sapa.TahapPenjualan[:n] {
			status := sapa.StatusSelesai
			if i == 1 {
				status = sapa.StatusDilewati
			}
			if err := s.SimpanTahap(ctx, k.ID, sapa.TahapRow{Kunci: td.Kunci, Status: status, DiperbaruiOleh: "uji"}); err != nil {
				t.Fatal(err)
			}
		}
	}
	hitung := func(tabel, kolom string, id int64) int {
		t.Helper()
		var n int
		if err := db.QueryRow(fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE %s = @p1`, tabel, kolom), id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	t.Run("SQL penghapusan hanya untuk usulan yang belum selesai dan ikut menghapus tahap serta dokumen", func(t *testing.T) {
		selesai := buat()
		selesaikan(selesai, len(sapa.TahapPenjualan))
		if ok, err := s.HapusPenjualan(ctx, selesai.ID); err != nil || ok {
			t.Fatalf("usulan selesai: ok=%v err=%v, want false tanpa galat", ok, err)
		}
		if hitung("sapa_penjualan", "id", selesai.ID) != 1 || hitung("sapa_penjualan_tahap", "penjualan_id", selesai.ID) != len(sapa.TahapPenjualan) {
			t.Fatal("usulan selesai dan tahapnya harus tetap ada")
		}

		berjalan := buat()
		selesaikan(berjalan, len(sapa.TahapPenjualan)-1) // satu tahap lagi baru selesai
		if _, err := s.SimpanDokumen(ctx, sapa.DokumenBaru{PenjualanID: berjalan.ID, Tahap: sapa.TahapNDSatker, Jenis: sapa.DokNDSatker, NamaFile: "uji.docx", Berkas: []byte("PK-uji"), Oleh: "uji"}); err != nil {
			t.Fatal(err)
		}
		if hitung("sapa_dokumen", "penjualan_id", berjalan.ID) != 1 {
			t.Fatal("prasyarat: dokumen harus ada")
		}
		if ok, err := s.HapusPenjualan(ctx, berjalan.ID); err != nil || !ok {
			t.Fatalf("usulan berjalan: ok=%v err=%v, want true", ok, err)
		}
		for _, c := range [][2]string{{"sapa_penjualan", "id"}, {"sapa_penjualan_tahap", "penjualan_id"}, {"sapa_dokumen", "penjualan_id"}} {
			if n := hitung(c[0], c[1], berjalan.ID); n != 0 {
				t.Errorf("%s masih memuat %d baris usulan yang dihapus", c[0], n)
			}
		}
		if ok, err := s.HapusPenjualan(ctx, berjalan.ID); err != nil || ok {
			t.Errorf("hapus dua kali: ok=%v err=%v, want false", ok, err)
		}
		if ok, err := s.HapusPenjualan(ctx, 2147483000); err != nil || ok {
			t.Errorf("usulan tak ada: ok=%v err=%v, want false", ok, err)
		}
		// Usulan tanpa tahap sama sekali dapat dihapus.
		kosong := buat()
		if ok, err := s.HapusPenjualan(ctx, kosong.ID); err != nil || !ok {
			t.Errorf("usulan tanpa tahap: ok=%v err=%v, want true", ok, err)
		}
	})

	t.Run("alur HTTP: terkunci setelah selesai, buka kunci oleh Pengguna Barang atau superadmin, lalu dapat dihapus", func(t *testing.T) {
		admin := buatPenggunaUji(t, db, "kunci-admin", "superadmin")
		pengguna := func(nama, peran, kode string) penggunaUji {
			p := buatPenggunaUji(t, db, "kunci-"+nama, "user")
			beriPeran(t, admin, p, peran, kode)
			return p
		}
		satker := pengguna("satker", "satker", "971001")
		satkerLain := pengguna("satkerlain", "satker", "972001")
		kanwil := pengguna("kanwil", "kanwil", "099710199")
		ue1 := pengguna("ue1", "ue1", "09971")
		barang := pengguna("barang", "pengguna_barang", "")

		k := buat()
		selesaikan(k, len(sapa.TahapPenjualan))
		base := "/sapa/penjualan/" + k.UUID

		detail := func(p penggunaUji) map[string]interface{} {
			t.Helper()
			code, body, _ := panggil(t, "GET", base, p.token, "")
			if code != http.StatusOK {
				t.Fatalf("detail %s = %d %v", p.username, code, body)
			}
			return data(t, body)
		}
		// Terkunci: penanda pada detail menurut peran.
		for p, bukaKunci := range map[*penggunaUji]bool{&satker: false, &kanwil: false, &ue1: false, &barang: true, &admin: true} {
			d := detail(*p)
			if d["selesai"] != true || d["boleh_hapus"] != false || d["boleh_buka_kunci"] != bukaKunci {
				t.Errorf("%s: selesai=%v boleh_hapus=%v boleh_buka_kunci=%v", p.username, d["selesai"], d["boleh_hapus"], d["boleh_buka_kunci"])
			}
		}
		// Tahap terakhir tidak bisa lagi dicatat ulang oleh pemiliknya (UE1), dan usulan tidak bisa dihapus.
		code, body, _ := panggil(t, "POST", base+"/tahap/"+sapa.TahapSimanUE1+"/selesai", ue1.token, `{"catatan":"ubah"}`)
		if msg, _ := body["message"].(string); code != http.StatusConflict || !strings.Contains(msg, "terkunci") {
			t.Errorf("ubah tahap terakhir = %d %q, want 409 terkunci", code, msg)
		}
		if code, _, _ := panggil(t, "DELETE", base, satker.token, ""); code != http.StatusConflict {
			t.Errorf("hapus usulan selesai oleh satker = %d, want 409", code)
		}
		// Yang tidak berhak membuka kunci.
		for _, p := range []penggunaUji{satker, kanwil, ue1} {
			if code, _, _ := panggil(t, "POST", base+"/tahap/"+sapa.TahapSimanUE1+"/buka-ulang", p.token, ""); code != http.StatusForbidden {
				t.Errorf("%s membuka kunci = %d, want 403", p.username, code)
			}
		}
		if code, _, _ := panggil(t, "POST", base+"/tahap/"+sapa.TahapSimanUE1+"/buka-ulang", satkerLain.token, ""); code != http.StatusNotFound {
			t.Errorf("satker lain membuka kunci = %d, want 404 (usulan tak terlihat)", code)
		}
		if detail(admin)["selesai"] != true {
			t.Fatal("usulan harus tetap terkunci setelah percobaan yang ditolak")
		}

		// Pengguna Barang membuka kunci: usulan berjalan lagi; Satker pemilik boleh menghapus, Kanwil dan UE1 tidak.
		if code, body, _ := panggil(t, "POST", base+"/tahap/"+sapa.TahapSimanUE1+"/buka-ulang", barang.token, ""); code != http.StatusOK {
			t.Fatalf("pengguna barang membuka kunci = %d %v", code, body)
		}
		if d := detail(satker); d["selesai"] != false || d["tahap_saat_ini"] != sapa.TahapSimanUE1 || d["boleh_hapus"] != true {
			t.Errorf("setelah dibuka kuncinya: selesai=%v tahap_saat_ini=%v boleh_hapus=%v", d["selesai"], d["tahap_saat_ini"], d["boleh_hapus"])
		}
		for _, p := range []penggunaUji{kanwil, ue1} {
			if code, _, _ := panggil(t, "DELETE", base, p.token, ""); code != http.StatusForbidden {
				t.Errorf("%s menghapus = %d, want 403", p.username, code)
			}
		}
		if code, _, _ := panggil(t, "DELETE", base, satkerLain.token, ""); code != http.StatusNotFound {
			t.Errorf("satker lain menghapus = %d, want 404", code)
		}
		if hitung("sapa_penjualan", "id", k.ID) != 1 {
			t.Fatal("usulan tidak boleh hilang oleh percobaan yang ditolak")
		}
		if code, body, _ := panggil(t, "DELETE", base, satker.token, ""); code != http.StatusOK {
			t.Fatalf("satker pemilik menghapus = %d %v", code, body)
		}
		if code, _, _ := panggil(t, "GET", base, admin.token, ""); code != http.StatusNotFound {
			t.Errorf("usulan yang dihapus = %d, want 404", code)
		}
		for _, c := range [][2]string{{"sapa_penjualan", "id"}, {"sapa_penjualan_tahap", "penjualan_id"}} {
			if n := hitung(c[0], c[1], k.ID); n != 0 {
				t.Errorf("%s masih memuat %d baris", c[0], n)
			}
		}

		// Superadmin juga dapat membuka kunci, dan Pengguna Barang dapat menghapus usulan berjalan milik satker mana pun.
		k2 := buat()
		selesaikan(k2, len(sapa.TahapPenjualan))
		base2 := "/sapa/penjualan/" + k2.UUID
		if code, _, _ := panggil(t, "POST", base2+"/tahap/"+sapa.TahapSimanUE1+"/buka-ulang", admin.token, ""); code != http.StatusOK {
			t.Errorf("superadmin membuka kunci = %d, want 200", code)
		}
		if code, _, _ := panggil(t, "DELETE", base2, barang.token, ""); code != http.StatusOK {
			t.Errorf("pengguna barang menghapus usulan berjalan = %d, want 200", code)
		}
	})
}
