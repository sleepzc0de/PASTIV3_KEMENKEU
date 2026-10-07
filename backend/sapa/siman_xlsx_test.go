package sapa

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

// Judul kolom ekspor data aset SIMAN ("Lampiran Data Aset"), sama dengan berkas aslinya.
var judulSIMAN = []interface{}{"No", "Kode Barang", "NUP", "Nama Barang", "Kode Register", "Kode Satker", "Nama Satker", "Merk", "Tipe", "Tanggal Perolehan", "Nilai Perolehan", "Nilai Buku",
	"Nilai Permohonan", "Luas Aset", "Informasi Tanah", "Luas Manfaat", "Luas Yang Disewakan", "Peruntukkan", "Alamat/Lokasi", "Kondisi", "Keterangan", "Status KIB", "Cara Penjualan",
	"Pilihan Penjualan", "Nama Pembeli", "Jabatan Pembeli"}

// barisSIMAN menyusun satu baris data seperti ekspor SIMAN: nilai uang dan tanggal berupa teks, NUP berupa angka.
func barisSIMAN(no int, kode string, nup int, nama, satker, merk string, tanggal interface{}, perolehan, permohonan, kondisi, ket string) []interface{} {
	return []interface{}{no, kode, nup, nama, "BEC22D41A417094EE0531561F20ACDA0", satker, "KPP Pratama Contoh", merk, merk, tanggal, perolehan, "Rp 0,00", permohonan, 1, "-", 1, 1, "", "", kondisi, ket, "Belum Dicek", "Lelang", "Normal/Tidak Scrap", "", ""}
}

// bukuSIMAN membuat berkas seperti ekspor SIMAN: "Sheet1" kosong, lalu sheet "Permohonan Pengelolaan" dengan baris judul "Lampiran Data Aset", baris kolom, dan data.
func bukuSIMAN(t *testing.T, data ...[]interface{}) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	const sheet = NamaSheetSIMAN
	if _, err := f.NewSheet(sheet); err != nil {
		t.Fatal(err)
	}
	baris := append([][]interface{}{{"Lampiran Data Aset"}, judulSIMAN}, data...)
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

func TestImporSIMANMemetakanKolomSesuaiPermintaan(t *testing.T) {
	b := bukuSIMAN(t,
		barisSIMAN(1, "3100102001", 246, "P.C Unit", "015042000635837000KD", "HP Prodesk 400 G2", "2015-12-30T00:00:00Z", "Rp 9.035.434,00", "Rp 300.000,00", "Rusak Berat", "sudah rusak berat"),
		barisSIMAN(2, "3100102001", 249, "P.C Unit", "015042000635837000KD", "HP Dekstop Pro MT i5", "2018-12-10T00:00:00Z", "Rp 12.090.000,00", "Rp 425.000,00", "Baik", ""),
	)
	h, err := ImporSIMANXLSX(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Galat) != 0 || len(h.Peringatan) != 0 || h.JumlahBaca != 2 || len(h.Barang) != 2 {
		t.Fatalf("galat=%v peringatan=%v baca=%d barang=%d", h.Galat, h.Peringatan, h.JumlahBaca, len(h.Barang))
	}
	want := Barang{Nama: "P.C Unit", Kode: "3100102001", NUP: "246", Lokasi: "HP Prodesk 400 G2", Kondisi: "Rusak Berat", TahunPerolehan: "2015", NilaiPerolehan: "9035434", NilaiLimit: "300000", Keterangan: "sudah rusak berat"}
	if h.Barang[0] != want {
		t.Errorf("barang[0] = %+v\nwant      %+v", h.Barang[0], want)
	}
	want2 := Barang{Nama: "P.C Unit", Kode: "3100102001", NUP: "249", Lokasi: "HP Dekstop Pro MT i5", Kondisi: "Baik", TahunPerolehan: "2018", NilaiPerolehan: "12090000", NilaiLimit: "425000"}
	if h.Barang[1] != want2 {
		t.Errorf("barang[1] = %+v\nwant      %+v", h.Barang[1], want2)
	}
}

func TestImporSIMANJudulDiBarisPertamaPadaSheetBebas(t *testing.T) {
	// Judul di baris pertama (tanpa baris judul "Lampiran Data Aset") pada sheet bernama lain juga dikenali.
	f := excelize.NewFile()
	defer f.Close()
	for i, v := range judulSIMAN {
		sel, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("Sheet1", sel, v)
	}
	for j, v := range barisSIMAN(1, "3100102001", 5, "Meja", "015042000635837000KD", "Olympic", "2010-01-02T00:00:00Z", "Rp 1.000.000,00", "Rp 100.000,00", "Rusak Ringan", "") {
		sel, _ := excelize.CoordinatesToCellName(j+1, 2)
		f.SetCellValue("Sheet1", sel, v)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	h, err := ImporSIMANXLSX(buf.Bytes())
	if err != nil || len(h.Barang) != 1 || h.Barang[0].Nama != "Meja" || h.Barang[0].Kondisi != "Rusak Ringan" {
		t.Fatalf("hasil=%+v err=%v", h, err)
	}
}

func TestTahunDariTanggal(t *testing.T) {
	for _, c := range []struct {
		raw   string
		tahun string
		ok    bool
	}{
		{"2015-12-30T00:00:00Z", "2015", true},
		{"2015-1-3", "2015", true},
		{"30-12-2015", "2015", true},
		{"30/12/2015", "2015", true},
		{"30.12.2015", "2015", true},
		{"2018", "2018", true},
		{"12 Maret 2020", "2020", true},
		{"43444", "2018", true},
		{"  ", "", true},
		{"", "", true},
		{"tanpa tanggal", "", false},
		{"0", "", false},
	} {
		tahun, ok := tahunDariTanggal(c.raw)
		if tahun != c.tahun || ok != c.ok {
			t.Errorf("tahunDariTanggal(%q) = %q,%v, want %q,%v", c.raw, tahun, ok, c.tahun, c.ok)
		}
	}
}

func TestUangSIMAN(t *testing.T) {
	for _, c := range []struct{ raw, want string }{
		{"Rp 9.035.434,00", "9035434"},
		{"Rp 300.000,00", "300000"},
		{"Rp 1.234,50", "1234.50"},
		{"Rp 1.234,05", "1234.05"},
		{"Rp. 2.500.000,00", "2500000"},
		{"RP 12.090.000,00", "12090000"},
		{"IDR 5.000,00", "5000"},
		{"Rp 0,00", "0"},
		{"Rp\u00a07.000,00", "7000"},
		{"1500000", "1500000"},
		{"1500000.5", "1500000.50"},
		{"1.5E+07", "15000000"},
		{"1500000.1000000001", "1500000.10"},
		{"", ""},
		{"Rp", ""},
		{"abc", "abc"},
		{"Rp abc", "abc"},
	} {
		if got := uangSIMAN(c.raw); got != c.want {
			t.Errorf("uangSIMAN(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestImporSIMANTanggalAsliExcelDanKondisiHurufKecil(t *testing.T) {
	// Tanggal yang disimpan Excel sebagai tanggal sungguhan (nomor seri) dan kondisi berhuruf kecil.
	tgl := time.Date(2019, 6, 15, 0, 0, 0, 0, time.UTC)
	b := bukuSIMAN(t, barisSIMAN(1, "3100102001", 7, "Laptop", "015042000635837000KD", "Lenovo", tgl, "Rp 10.000.000,00", "Rp 500.000,00", "rusak berat", ""))
	h, err := ImporSIMANXLSX(b)
	if err != nil || len(h.Barang) != 1 {
		t.Fatalf("hasil=%+v err=%v", h, err)
	}
	if h.Barang[0].TahunPerolehan != "2019" || h.Barang[0].Kondisi != "Rusak Berat" || len(h.Galat) != 0 {
		t.Errorf("barang = %+v galat = %v", h.Barang[0], h.Galat)
	}
}

func TestImporSIMANMelaporkanMasalahTanpaMembuangBaris(t *testing.T) {
	b := bukuSIMAN(t,
		barisSIMAN(1, "3100102001", 1, "A", "015042000635837000KD", "m", "2015-12-30T00:00:00Z", "Rp 1.000,00", "Rp 0,00", "Baik", ""),              // Nilai Permohonan belum diisi
		barisSIMAN(2, "3100102001", 2, "B", "015042000635837000KD", "m", "tidak jelas", "Rp 1.000,00", "Rp 100,00", "Baik", ""),                     // tanggal tidak terbaca
		barisSIMAN(3, "3100102001", 3, "C", "015099900000000000KD", "m", "2015-12-30T00:00:00Z", "Rp 1.000,00", "Rp 100,00", "Baik", ""),            // satker lain
		barisSIMAN(4, "3100102001", 4, "", "015042000635837000KD", "m", "2015-12-30T00:00:00Z", "Rp 1.000,00", "Rp 100,00", "Baik", ""),             // nama kosong
		barisSIMAN(5, "3100102001", 5, "E", "015042000635837000KD", "m", "2015-12-30T00:00:00Z", "bukan angka", "Rp 100,00", "Baik", ""),            // perolehan tidak terbaca
		barisSIMAN(6, "3100102001", 6, "F", "015042000635837000KD", "m", "2015-12-30T00:00:00Z", "Rp 1.000,00", "Rp 100,00", "Baik", "baris sehat"), // sehat
	)
	h, err := ImporSIMANXLSX(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Barang) != 6 || h.JumlahBaca != 6 {
		t.Fatalf("barang=%d baca=%d, want 6 (baris bermasalah tetap dimuat)", len(h.Barang), h.JumlahBaca)
	}
	galat := strings.Join(h.Galat, "\n")
	for _, w := range []string{
		"Nilai limit barang (baris 3) harus lebih dari nol",
		"Tanggal Perolehan barang (baris 4) tidak dapat dibaca",
		"Nama barang (baris 6)",
		"Nilai perolehan barang (baris 7)",
	} {
		if !strings.Contains(galat, w) {
			t.Errorf("galat tidak memuat %q:\n%s", w, galat)
		}
	}
	// Nomor baris merujuk ke baris di Excel (judul "Lampiran Data Aset" di baris 1, kolom di baris 2, data mulai baris 3).
	peringatan := strings.Join(h.Peringatan, "\n")
	if !strings.Contains(peringatan, "1 barang belum punya Nilai Permohonan") {
		t.Errorf("peringatan nilai permohonan hilang: %s", peringatan)
	}
	if !strings.Contains(peringatan, "2 satker berbeda") || !strings.Contains(peringatan, "015042000635837000KD") || !strings.Contains(peringatan, "015099900000000000KD") {
		t.Errorf("peringatan satker berbeda hilang: %s", peringatan)
	}
}

func TestImporSIMANMembatasiJumlahBaris(t *testing.T) {
	var data [][]interface{}
	for i := 0; i < MaksBarang+5; i++ {
		data = append(data, barisSIMAN(i+1, "3100102001", i+1, fmt.Sprintf("Barang %d", i+1), "015042000635837000KD", "m", "2015-12-30T00:00:00Z", "Rp 1.000,00", "Rp 100,00", "Baik", ""))
	}
	h, err := ImporSIMANXLSX(bukuSIMAN(t, data...))
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Barang) != MaksBarang || h.JumlahBaca != MaksBarang+5 || len(h.Peringatan) != 1 || !strings.Contains(h.Peringatan[0], "hanya 500 pertama") {
		t.Errorf("barang=%d baca=%d peringatan=%v", len(h.Barang), h.JumlahBaca, h.Peringatan)
	}
}

func TestImporSIMANMenolakBerkasYangSalah(t *testing.T) {
	// Berkas rusak atau kosong.
	_, err := ImporSIMANXLSX([]byte("bukan zip"))
	harapValidasi(t, err, "tidak dapat dibaca")
	_, err = ImporSIMANXLSX(nil)
	harapValidasi(t, err, "Berkas kosong")

	// Berkas tanpa judul kolom SIMAN.
	_, err = ImporSIMANXLSX(bukuUji(t, "Sheet1", [][]interface{}{{"a", "b"}, {1, 2}}))
	harapValidasi(t, err, "Judul kolom data SIMAN tidak ditemukan")

	// Judul ada tetapi tidak ada data di bawahnya.
	_, err = ImporSIMANXLSX(bukuSIMAN(t))
	harapValidasi(t, err, "Tidak ada baris data")

	// Template SAPA diunggah lewat tombol SIMAN: diarahkan ke tombol yang benar, dan sebaliknya.
	tpl, err := TemplateBarangXLSX()
	if err != nil {
		t.Fatal(err)
	}
	_, err = ImporSIMANXLSX(tpl)
	harapValidasi(t, err, "Unggah Excel")

	_, err = ImporBarangXLSX(bukuSIMAN(t, barisSIMAN(1, "3100102001", 1, "A", "015042000635837000KD", "m", "2015-12-30T00:00:00Z", "Rp 1.000,00", "Rp 100,00", "Baik", "")))
	harapValidasi(t, err, "Unggah Data SIMAN")
}

func TestImporSIMANTidakMenjalankanRumus(t *testing.T) {
	// Isi sel yang diawali "=" dibaca sebagai teks biasa dan hanya lewat validasi barang; tidak dievaluasi.
	b := bukuSIMAN(t, barisSIMAN(1, "3100102001", 1, "=HYPERLINK(\"x\")", "015042000635837000KD", "+cmd", "2015-12-30T00:00:00Z", "Rp 1.000,00", "Rp 100,00", "Baik", "-1+1"))
	h, err := ImporSIMANXLSX(b)
	if err != nil || len(h.Barang) != 1 {
		t.Fatalf("hasil=%+v err=%v", h, err)
	}
	if h.Barang[0].Lokasi != "+cmd" || h.Barang[0].Keterangan != "-1+1" {
		t.Errorf("barang = %+v", h.Barang[0])
	}
}
