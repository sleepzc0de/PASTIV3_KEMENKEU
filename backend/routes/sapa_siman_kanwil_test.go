package routes

import (
	"net/http"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"pasti-v3-backend/sapa"
)

// Berkas seperti ekspor data aset SIMAN: sheet pertama kosong, daftar di sheet "Permohonan Pengelolaan" (baris 1 judul laporan, baris 2 judul kolom, data mulai baris 3).
func bukuEksporSIMAN(t *testing.T) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	const sheet = sapa.NamaSheetSIMAN
	if _, err := f.NewSheet(sheet); err != nil {
		t.Fatal(err)
	}
	baris := [][]interface{}{
		{"Lampiran Data Aset"},
		{"No", "Kode Barang", "NUP", "Nama Barang", "Kode Register", "Kode Satker", "Nama Satker", "Merk", "Tipe", "Tanggal Perolehan", "Nilai Perolehan", "Nilai Buku", "Nilai Permohonan",
			"Luas Aset", "Informasi Tanah", "Luas Manfaat", "Luas Yang Disewakan", "Peruntukkan", "Alamat/Lokasi", "Kondisi", "Keterangan"},
		{1, "3100102001", 246, "P.C Unit", "BEC22D41A417094EE0531561F20ACDA0", "015042000635837000KD", "KPP Pratama Contoh", "HP Prodesk 400 G2", "HP Prodesk 400 G2", "2015-12-30T00:00:00Z",
			"Rp 9.035.434,00", "Rp 0,00", "Rp 300.000,00", 1, "-", 1, 1, "", "", "Rusak Berat", "sudah rusak berat"},
	}
	for i, r := range baris {
		for j, v := range r {
			sel, _ := excelize.CoordinatesToCellName(j+1, i+1)
			if err := f.SetCellValue(sheet, sel, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSapaImporDataSIMAN(t *testing.T) {
	e := siapkan(t, nil)

	// Hanya pengguna SAPA yang dilayani.
	if j := e.kirim("tanpa", "POST", "/barang/impor-siman", []byte("{}"), ""); j.kode != http.StatusForbidden {
		t.Errorf("tanpa peran: status %d, want 403", j.kode)
	}

	// Pemetaan kolom ekspor SIMAN ke daftar barang SAPA.
	body, tipe := formBerkas(t, "berkas", "data_export_aset.xlsx", bukuEksporSIMAN(t), "")
	imp := e.harap(e.kirim("satkA", "POST", "/barang/impor-siman", body, tipe), 200).data()
	barang, _ := imp["barang"].([]interface{})
	if len(barang) != 1 {
		t.Fatalf("barang hasil impor = %v", imp["barang"])
	}
	b := barang[0].(map[string]interface{})
	want := map[string]string{
		"nama": "P.C Unit", "kode": "3100102001", "nup": "246", "lokasi": "HP Prodesk 400 G2", "kondisi": "Rusak Berat", "tahun_perolehan": "2015",
		"nilai_perolehan": "9035434", "nilai_limit": "300000", "keterangan": "sudah rusak berat",
	}
	for k, v := range want {
		if b[k] != v {
			t.Errorf("%s = %v, want %q", k, b[k], v)
		}
	}
	if g, _ := imp["galat"].([]interface{}); len(g) != 0 {
		t.Errorf("galat = %v", g)
	}

	// Berkas yang salah dijawab dengan pesan yang bisa ditindaklanjuti; template SAPA diarahkan ke tombol Unggah Excel.
	tpl := e.harap(e.kirim("satkA", "GET", "/barang/template", nil, ""), 200)
	for _, c := range []struct {
		nama  string
		isi   []byte
		pesan string
	}{
		{"daftar.csv", []byte("a,b"), ".xlsx"},
		{"rusak.xlsx", []byte("bukan zip"), "tidak dapat dibaca"},
		{"template.xlsx", tpl.w.Body.Bytes(), "Unggah Excel"},
	} {
		b, tp := formBerkas(t, "berkas", c.nama, c.isi, "")
		j := e.harap(e.kirim("satkA", "POST", "/barang/impor-siman", b, tp), http.StatusBadRequest)
		if msg, _ := j.m["message"].(string); !strings.Contains(msg, c.pesan) {
			t.Errorf("%s: pesan = %q, want memuat %q", c.nama, msg, c.pesan)
		}
	}

	// Sebaliknya, ekspor SIMAN pada tombol Unggah Excel diarahkan ke Unggah Data SIMAN.
	body, tipe = formBerkas(t, "berkas", "data_export_aset.xlsx", bukuEksporSIMAN(t), "")
	j := e.harap(e.kirim("satkA", "POST", "/barang/impor", body, tipe), http.StatusBadRequest)
	if msg, _ := j.m["message"].(string); !strings.Contains(msg, "Unggah Data SIMAN") {
		t.Errorf("pesan = %q, want memuat petunjuk Unggah Data SIMAN", msg)
	}
}

// Pemilih tembusan Kanwil membaca daftar Kanwil aktif dari Referensi Kanwil, beserta saran teks tembusannya.
func TestSapaReferensiKanwilUntukPemilih(t *testing.T) {
	e := siapkan(t, nil)
	e.repo.Kanwil["015040199"] = sapa.RefKanwil{Kode: "015040199", Nama: "KANTOR WILAYAH DJBC JAWA TIMUR I", Singkatan: "KW DJBC JATIM I", Aktif: true}
	e.repo.Kanwil["015010199"] = sapa.RefKanwil{Kode: "015010199", Nama: "Kantor Wilayah DJP Jakarta Pusat", Aktif: true}
	e.repo.Kanwil["015020199"] = sapa.RefKanwil{Kode: "015020199", Nama: "KANTOR WILAYAH NONAKTIF", Aktif: false}

	if j := e.kirim("tanpa", "GET", "/referensi/kanwil", nil, ""); j.kode != http.StatusForbidden {
		t.Errorf("tanpa peran: status %d, want 403", j.kode)
	}
	// Satker, Kanwil, dan UE1 sama-sama boleh membaca daftarnya.
	for _, u := range []string{"satkA", "kanwil", "ue1"} {
		daftar, _ := e.harap(e.kirim(u, "GET", "/referensi/kanwil", nil, ""), 200).data()["daftar"].([]interface{})
		if len(daftar) != 2 {
			t.Fatalf("%s: daftar = %v, want 2 Kanwil aktif", u, daftar)
		}
		a, c := daftar[0].(map[string]interface{}), daftar[1].(map[string]interface{})
		if a["kode"] != "015010199" || a["tembusan"] != "Kepala Kantor Wilayah DJP Jakarta Pusat" {
			t.Errorf("%s: pertama = %v", u, a)
		}
		if c["kode"] != "015040199" || c["singkatan"] != "KW DJBC JATIM I" || c["tembusan"] != "Kepala Kantor Wilayah DJBC Jawa Timur I" {
			t.Errorf("%s: kedua = %v", u, c)
		}
	}
}
