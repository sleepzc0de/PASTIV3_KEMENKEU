package sapa

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// bukuUji membuat berkas .xlsx dari baris-baris sel (nilai bertipe: string ditulis sebagai teks, angka sebagai angka).
func bukuUji(t *testing.T, sheet string, baris [][]interface{}) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	if sheet != "Sheet1" {
		if err := f.SetSheetName("Sheet1", sheet); err != nil {
			t.Fatal(err)
		}
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

func harapValidasi(t *testing.T, err error, mengandung string) {
	t.Helper()
	var ev *ErrValidasi
	if !errors.As(err, &ev) {
		t.Fatalf("err = %v, want ErrValidasi", err)
	}
	if !strings.Contains(strings.Join(ev.Rincian, "; "), mengandung) {
		t.Errorf("pesan %v tidak memuat %q", ev.Rincian, mengandung)
	}
}

func TestTemplateBarangXLSXBisaDibukaDanBerisiJudulDanPetunjuk(t *testing.T) {
	b, err := TemplateBarangXLSX()
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("template bukan xlsx sah: %v", err)
	}
	defer f.Close()
	if got := f.GetSheetList(); len(got) != 2 || got[0] != NamaSheetBarang || got[1] != NamaSheetPetunjuk {
		t.Fatalf("sheet = %v", got)
	}
	for i, k := range kolomDaftarBarang {
		sel, _ := excelize.CoordinatesToCellName(i+1, 1)
		if v, _ := f.GetCellValue(NamaSheetBarang, sel); v != k.judul {
			t.Errorf("judul %s = %q, want %q", sel, v, k.judul)
		}
	}
	dv, err := f.GetDataValidations(NamaSheetBarang)
	if err != nil || len(dv) < 4 {
		t.Errorf("validasi isian = %d (%v), want >= 4 (kondisi, tahun, dua nilai uang)", len(dv), err)
	}
	// Template kosong tidak boleh dianggap berkas berisi: pengguna harus mengisinya dulu.
	_, err = ImporBarangXLSX(b)
	harapValidasi(t, err, "Tidak ada baris data")
}

func TestEksporLaluImporMengembalikanDaftarYangSama(t *testing.T) {
	asal := []Barang{
		{Nama: "Gedung Kantor Lama", Kode: "4010101001", NUP: "0012", Lokasi: "Jl. Merdeka No. 1", Kondisi: "Rusak Berat", TahunPerolehan: "1999", NilaiPerolehan: "2500000000", NilaiLimit: "1500000000.50", Keterangan: "usulan"},
		{Nama: "Mobil Dinas", Kode: "3020101002", NUP: "7", Kondisi: "Baik", NilaiPerolehan: "300000000", NilaiLimit: "100000000"},
	}
	b, err := EksporBarangXLSX(asal)
	if err != nil {
		t.Fatal(err)
	}
	h, err := ImporBarangXLSX(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Galat) != 0 || len(h.Peringatan) != 0 {
		t.Fatalf("galat = %v, peringatan = %v", h.Galat, h.Peringatan)
	}
	if len(h.Barang) != len(asal) || h.JumlahBaca != len(asal) {
		t.Fatalf("barang = %d, dibaca = %d", len(h.Barang), h.JumlahBaca)
	}
	for i, want := range asal {
		got := h.Barang[i]
		// Nilai uang boleh berubah bentuk (1500000000.50 vs "1500000000.5"), jadi dibandingkan dalam sen.
		gp, _ := ParseUang(got.NilaiPerolehan)
		wp, _ := ParseUang(want.NilaiPerolehan)
		gl, _ := ParseUang(got.NilaiLimit)
		wl, _ := ParseUang(want.NilaiLimit)
		if gp != wp || gl != wl {
			t.Errorf("barang %d: nilai = %q/%q, want %q/%q", i, got.NilaiPerolehan, got.NilaiLimit, want.NilaiPerolehan, want.NilaiLimit)
		}
		got.NilaiPerolehan, got.NilaiLimit, want.NilaiPerolehan, want.NilaiLimit = "", "", "", ""
		if got != want {
			t.Errorf("barang %d = %+v, want %+v", i, got, want)
		}
	}
}

func TestEksporTidakMenjadikanIsiSebagaiRumus(t *testing.T) {
	// Isian yang diawali = + - @ akan dieksekusi Excel bila ditulis sebagai rumus (injeksi rumus).
	b, err := EksporBarangXLSX([]Barang{{Nama: `=HYPERLINK("http://contoh.invalid","klik")`, Lokasi: "+1+1", NilaiPerolehan: "1", NilaiLimit: "1"}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, sel := range []string{"A2", "D2"} {
		if rumus, _ := f.GetCellFormula(NamaSheetBarang, sel); rumus != "" {
			t.Errorf("%s ditulis sebagai rumus %q", sel, rumus)
		}
	}
	if v, _ := f.GetCellValue(NamaSheetBarang, "A2"); !strings.HasPrefix(v, "=HYPERLINK") {
		t.Errorf("A2 = %q, teks asli harus utuh", v)
	}
}

func TestImporUrutanKolomBebasDanKolomTambahanDiabaikan(t *testing.T) {
	b := bukuUji(t, NamaSheetBarang, [][]interface{}{
		{"Nomor", "Nilai Limit", "nama_barang", "Nilai Perolehan (Rp)", "KONDISI", "Catatan lain"},
		{1, 1500000, "Meja Rapat", 2500000, "rusak ringan", "abaikan"},
		{2, "1.200.000,50", "Kursi", "3.000.000", "Baik", "abaikan"},
	})
	h, err := ImporBarangXLSX(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Galat) != 0 {
		t.Fatalf("galat = %v", h.Galat)
	}
	if h.Barang[0].Nama != "Meja Rapat" || h.Barang[0].NilaiLimit != "1500000" || h.Barang[0].NilaiPerolehan != "2500000" {
		t.Errorf("baris 1 = %+v", h.Barang[0])
	}
	if h.Barang[0].Kondisi != "Rusak Ringan" {
		t.Errorf("kondisi = %q, harus disamakan dengan pilihan yang dikenal", h.Barang[0].Kondisi)
	}
	if l, _ := ParseUang(h.Barang[1].NilaiLimit); l != 120000050 {
		t.Errorf("limit teks format Indonesia = %q (%d sen)", h.Barang[1].NilaiLimit, l)
	}
}

func TestImporNilaiAngkaExcel(t *testing.T) {
	// Excel menyimpan angka sebagai float64: kode panjang bisa tampil dalam notasi ilmiah dan angka uang membawa sisa pembulatan.
	cases := []struct{ masuk, want string }{
		{"2.010101001E9", "2010101001"},
		{"2001.0", "2001"},
		{"2001", "2001"},
		{"0012", "0012"}, // nol di depan pada sel teks tidak boleh hilang
	}
	for _, c := range cases {
		if got := angkaBulat(c.masuk); got != c.want {
			t.Errorf("angkaBulat(%q) = %q, want %q", c.masuk, got, c.want)
		}
	}
	for _, c := range []struct{ masuk, want string }{
		{"1500000.1000000001", "1500000.10"},
		{"2.5E9", "2500000000"},
		{"1500000", "1500000"},
		{"1.500.000,50", "1.500.000,50"}, // teks Indonesia diserahkan ke ParseUang
	} {
		if got := angkaUang(c.masuk); got != c.want {
			t.Errorf("angkaUang(%q) = %q, want %q", c.masuk, got, c.want)
		}
	}
}

func TestImporMelaporkanGalatPerBarisTanpaMembuangBaris(t *testing.T) {
	b := bukuUji(t, NamaSheetBarang, [][]interface{}{
		{"Nama Barang", "Nilai Perolehan (Rp)", "Nilai Limit (Rp)", "Tahun Perolehan"},
		{"Barang Sah", 1000, 500, 2001},
		{"", 1000, 500, 2001},         // baris 3: tanpa nama
		{"Barang Limit Nol", 1000, 0}, // baris 4: limit harus > 0
		{"Barang Tahun Salah", 1000, 500, "abcd"},
		{},
		{"Barang Setelah Baris Kosong", 10, 5},
	})
	h, err := ImporBarangXLSX(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Barang) != 5 || h.JumlahBaca != 5 {
		t.Fatalf("barang = %d, dibaca = %d (baris kosong harus dilewati, baris bermasalah tetap dimuat)", len(h.Barang), h.JumlahBaca)
	}
	semua := strings.Join(h.Galat, "\n")
	for _, want := range []string{"(baris 3)", "(baris 4)", "(baris 5)"} {
		if !strings.Contains(semua, want) {
			t.Errorf("galat tidak menyebut %s:\n%s", want, semua)
		}
	}
	if strings.Contains(semua, "(baris 2)") || strings.Contains(semua, "(baris 7)") {
		t.Errorf("baris sah ikut dilaporkan:\n%s", semua)
	}
}

func TestImporMenolakBerkasYangBukanDaftarBarang(t *testing.T) {
	_, err := ImporBarangXLSX(nil)
	harapValidasi(t, err, "kosong")

	_, err = ImporBarangXLSX([]byte("bukan zip sama sekali"))
	harapValidasi(t, err, "tidak dapat dibaca")

	_, err = ImporBarangXLSX(bytes.Repeat([]byte{'x'}, MaksUkuranXLSX+1))
	harapValidasi(t, err, "terlalu besar")

	// Judul wajib tidak lengkap: Nilai Limit tidak ada.
	_, err = ImporBarangXLSX(bukuUji(t, NamaSheetBarang, [][]interface{}{{"Nama Barang", "Nilai Perolehan"}, {"A", 1}}))
	harapValidasi(t, err, "Judul kolom tidak ditemukan")

	// Hanya judul, tanpa data.
	_, err = ImporBarangXLSX(bukuUji(t, NamaSheetBarang, [][]interface{}{{"Nama Barang", "Nilai Perolehan", "Nilai Limit"}}))
	harapValidasi(t, err, "Tidak ada baris data")
}

func TestImporSheetLainDipakaiBilaNamaTidakPersis(t *testing.T) {
	// Pengguna menyalin data ke sheet bernama lain: sheet pertama dipakai.
	b := bukuUji(t, "Data Saya", [][]interface{}{
		{"Nama Barang", "Nilai Perolehan", "Nilai Limit"},
		{"Lemari", 100, 50},
	})
	h, err := ImporBarangXLSX(b)
	if err != nil || len(h.Barang) != 1 || h.Barang[0].Nama != "Lemari" {
		t.Fatalf("hasil = %+v, err = %v", h, err)
	}
}

func TestImporMembatasiJumlahBaris(t *testing.T) {
	baris := [][]interface{}{{"Nama Barang", "Nilai Perolehan", "Nilai Limit"}}
	for i := 0; i < MaksBarang+25; i++ {
		baris = append(baris, []interface{}{fmt.Sprintf("Barang %d", i+1), 100, 50})
	}
	h, err := ImporBarangXLSX(bukuUji(t, NamaSheetBarang, baris))
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Barang) != MaksBarang || h.JumlahBaca != MaksBarang+25 {
		t.Errorf("barang = %d, dibaca = %d", len(h.Barang), h.JumlahBaca)
	}
	if len(h.Peringatan) != 1 || !strings.Contains(h.Peringatan[0], "hanya 500 pertama") {
		t.Errorf("peringatan = %v", h.Peringatan)
	}
	if _, err := EksporBarangXLSX(make([]Barang, MaksBarang+1)); err == nil {
		t.Error("ekspor lebih dari batas harus ditolak")
	}
}

func TestImporMembatasiBanyakGalatYangDilaporkan(t *testing.T) {
	baris := [][]interface{}{{"Nama Barang", "Nilai Perolehan", "Nilai Limit"}}
	for i := 0; i < 80; i++ {
		baris = append(baris, []interface{}{"", 1, 1}) // tiap baris tanpa nama
	}
	h, err := ImporBarangXLSX(bukuUji(t, NamaSheetBarang, baris))
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Galat) != 51 || !strings.Contains(h.Galat[50], "30 masalah lain") {
		t.Errorf("galat = %d, terakhir = %q", len(h.Galat), h.Galat[len(h.Galat)-1])
	}
}
