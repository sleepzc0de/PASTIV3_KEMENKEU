package laporan

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

type sumberUji struct {
	baris [][]interface{}
	i     int
	galat error
}

func (s *sumberUji) Next() bool {
	if s.i >= len(s.baris) {
		return false
	}
	s.i++
	return true
}
func (s *sumberUji) Nilai() []interface{} { return s.baris[s.i-1] }
func (s *sumberUji) Err() error           { return s.galat }

func contoh() *sumberUji {
	return &sumberUji{baris: [][]interface{}{
		{"Pengadaan Laptop", int64(3), 1500000.5, time.Date(2025, 3, 4, 0, 0, 0, 0, time.UTC), nil},
		{"Jasa \"Konsultan\", Tahap 1\nbaris dua", int64(-2), 0.0, time.Date(2025, 3, 4, 10, 30, 5, 0, time.UTC), []byte("teks byte")},
	}}
}

var kolomUji = []string{"nama_paket", "jumlah", "nilai", "tanggal", "catatan"}

func TestTeks(t *testing.T) {
	kasus := map[string]struct {
		in   interface{}
		want string
	}{
		"nil":           {nil, ""},
		"string":        {"abc", "abc"},
		"byte":          {[]byte("x"), "x"},
		"bool ya":       {true, "Ya"},
		"bool tidak":    {false, "Tidak"},
		"int64":         {int64(-7), "-7"},
		"float tanpa e": {float64(12345678901.5), "12345678901.5"},
		"float bulat":   {float64(2), "2"},
		"tanggal":       {time.Date(2025, 3, 4, 0, 0, 0, 0, time.UTC), "2025-03-04"},
		"tanggal + jam": {time.Date(2025, 3, 4, 1, 2, 3, 0, time.UTC), "2025-03-04 01:02:03"},
	}
	for nama, k := range kasus {
		if got := Teks(k.in); got != k.want {
			t.Errorf("%s: Teks = %q, want %q", nama, got, k.want)
		}
	}
}

func TestCSVStrukturDanPengamanan(t *testing.T) {
	src := &sumberUji{baris: [][]interface{}{
		{"=HYPERLINK(\"http://x\")", int64(-5), "-5", "+62812", "@cmd", "-1+1"},
		{"biasa", 1.5, "", nil, "\tTab", "normal"},
	}}
	var buf bytes.Buffer
	n, err := CSV(&buf, []string{"a", "b", "c", "d", "e", "f"}, src, ';')
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "\xEF\xBB\xBF") {
		t.Error("BOM UTF-8 hilang")
	}
	baris := strings.Split(strings.TrimPrefix(out, "\xEF\xBB\xBF"), "\r\n")
	if baris[0] != "a;b;c;d;e;f" {
		t.Errorf("judul = %q", baris[0])
	}
	// Teks yang bisa dibaca Excel sebagai rumus diberi apostrof; bilangan (juga negatif) dan teks bilangan dibiarkan.
	if want := `"'=HYPERLINK(""http://x"")";-5;-5;+62812;'@cmd;'-1+1`; baris[1] != want {
		t.Errorf("baris 1 = %q, want %q", baris[1], want)
	}
	if want := "biasa;1.5;;;'\tTab;normal"; baris[2] != want {
		t.Errorf("baris 2 = %q, want %q", baris[2], want)
	}
}
func TestCSVPemisahDanKutip(t *testing.T) {
	var buf bytes.Buffer
	if _, err := CSV(&buf, kolomUji, contoh(), ','); err != nil {
		t.Fatal(err)
	}
	out := strings.TrimPrefix(buf.String(), "\xEF\xBB\xBF")
	if !strings.HasPrefix(out, "nama_paket,jumlah,nilai,tanggal,catatan\r\n") {
		t.Errorf("judul = %q", out)
	}
	// Koma, kutip, dan baris baru di dalam sel harus dikutip dengan benar.
	// Baris baru di dalam sel ikut menjadi CRLF (perilaku encoding/csv dengan UseCRLF); sel tetap satu sel karena dikutip.
	if !strings.Contains(out, "\"Jasa \"\"Konsultan\"\", Tahap 1\r\nbaris dua\"") {
		t.Errorf("kutipan sel salah: %q", out)
	}
	if !strings.Contains(out, "Pengadaan Laptop,3,1500000.5,2025-03-04,\r\n") {
		t.Errorf("baris 1 = %q", out)
	}
}

func TestCSVMeneruskanGalatSumber(t *testing.T) {
	src := contoh()
	src.galat = errors.New("koneksi putus")
	if _, err := CSV(&bytes.Buffer{}, kolomUji, src, ','); err == nil || err.Error() != "koneksi putus" {
		t.Errorf("err = %v, want galat sumber", err)
	}
}

func TestXLSXIsiDanTipeSel(t *testing.T) {
	var buf bytes.Buffer
	n, err := XLSX(&buf, Opsi{NamaSheet: "Paket [2025]/Q1: Uji?"}, kolomUji, contoh())
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	f, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) != 1 || sheets[0] != "Paket  2025  Q1  Uji" {
		t.Fatalf("sheet = %v", sheets)
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || strings.Join(rows[0], "|") != "nama_paket|jumlah|nilai|tanggal|catatan" {
		t.Fatalf("baris = %v", rows)
	}
	if rows[1][0] != "Pengadaan Laptop" || rows[2][0] != "Jasa \"Konsultan\", Tahap 1\nbaris dua" || rows[2][4] != "teks byte" {
		t.Errorf("isi teks = %v", rows)
	}
	// Bilangan tersimpan sebagai bilangan (bisa dijumlahkan), bukan teks.
	for _, sel := range []string{"B2", "C2", "B3"} {
		tipe, err := f.GetCellType(sheets[0], sel)
		if err != nil || tipe != excelize.CellTypeNumber && tipe != excelize.CellTypeUnset {
			t.Errorf("%s: tipe %v (err %v), want bilangan", sel, tipe, err)
		}
	}
	if v, _ := f.GetCellValue(sheets[0], "B3", excelize.Options{RawCellValue: true}); v != "-2" {
		t.Errorf("B3 mentah = %q, want -2", v)
	}
	if v, _ := f.GetCellValue(sheets[0], "C2", excelize.Options{RawCellValue: true}); v != "1500000.5" {
		t.Errorf("C2 mentah = %q", v)
	}
	// Tanggal tersimpan sebagai tanggal sungguhan (bilangan seri), bukan teks.
	if v, _ := f.GetCellValue(sheets[0], "D2", excelize.Options{RawCellValue: true}); v != "45720" {
		t.Errorf("D2 mentah = %q, want 45720 (4 Mar 2025)", v)
	}
	// Judul kolom dibekukan dan ada tabel dengan filter.
	if tabel, err := f.GetTables(sheets[0]); err != nil || len(tabel) != 1 || tabel[0].Range != "A1:E3" {
		t.Errorf("tabel = %+v err=%v", tabel, err)
	}
	if p, err := f.GetPanes(sheets[0]); err != nil || !p.Freeze || p.YSplit != 1 {
		t.Errorf("panes = %+v err=%v", p, err)
	}
}

// Teks yang diawali "=" harus tetap teks di Excel (bukan rumus yang dihitung).
func TestXLSXTeksRumusTidakMenjadiRumus(t *testing.T) {
	src := &sumberUji{baris: [][]interface{}{{"=1+1", "@SUM(A1)"}}}
	var buf bytes.Buffer
	if _, err := XLSX(&buf, Opsi{}, []string{"a", "b"}, src); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if rumus, _ := f.GetCellFormula("Data", "A2"); rumus != "" {
		t.Errorf("A2 berisi rumus %q", rumus)
	}
	if v, _ := f.GetCellValue("Data", "A2"); v != "=1+1" {
		t.Errorf("A2 = %q, want teks '=1+1'", v)
	}
}

func TestXLSXTanpaBarisTetapBerkasSah(t *testing.T) {
	var buf bytes.Buffer
	n, err := XLSX(&buf, Opsi{NamaSheet: "Kosong"}, kolomUji, &sumberUji{})
	if err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	f, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatalf("berkas kosong tidak sah: %v", err)
	}
	defer f.Close()
	if rows, _ := f.GetRows("Kosong"); len(rows) != 1 {
		t.Errorf("baris = %v, want hanya judul", rows)
	}
}

func TestXLSXMenolakMelebihiBatasBaris(t *testing.T) {
	lama := batasBarisExcel
	batasBarisExcel = 2
	defer func() { batasBarisExcel = lama }()
	src := &sumberUji{baris: [][]interface{}{{"a"}, {"b"}, {"c"}}}
	if _, err := XLSX(&bytes.Buffer{}, Opsi{}, []string{"x"}, src); !errors.Is(err, ErrMelebihiBatas) {
		t.Errorf("err = %v, want ErrMelebihiBatas", err)
	}
}

func TestXLSXBanyakBarisMelewatiSampelLebar(t *testing.T) {
	var baris [][]interface{}
	for i := 0; i < 450; i++ {
		baris = append(baris, []interface{}{fmt.Sprintf("paket %d", i), int64(i)})
	}
	var buf bytes.Buffer
	n, err := XLSX(&buf, Opsi{}, []string{"nama", "no"}, &sumberUji{baris: baris})
	if err != nil || n != 450 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	f, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, _ := f.GetRows("Data")
	if len(rows) != 451 || rows[450][0] != "paket 449" {
		t.Errorf("jumlah baris = %d, terakhir = %v", len(rows), rows[len(rows)-1])
	}
}

func TestPDFMenghasilkanBerkasDanMemotongBaris(t *testing.T) {
	var baris [][]interface{}
	for i := 0; i < 120; i++ {
		baris = append(baris, []interface{}{
			fmt.Sprintf("Paket nomor %d dengan nama yang sangat sangat panjang sekali sehingga harus dipotong agar muat dalam satu baris", i),
			int64(i), float64(i) * 1234567.89, time.Date(2025, 1, 1+i%28, 0, 0, 0, 0, time.UTC), "ok",
		})
	}
	var buf bytes.Buffer
	n, terpotong, err := PDF(&buf, Opsi{Judul: "Pengumuman Tender", Subjudul: []string{"KLPD: K10", "Tahun: 2025"}, Total: 120}, kolomUji, &sumberUji{baris: baris}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if n != 100 || !terpotong {
		t.Errorf("n=%d terpotong=%v, want 100 dan true", n, terpotong)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Fatalf("bukan PDF: %q", buf.Bytes()[:10])
	}
	// 100 baris tidak muat di satu halaman lanskap: judul kolom diulang di halaman berikutnya.
	if jml := bytes.Count(buf.Bytes(), []byte("/Type /Page\n")) + bytes.Count(buf.Bytes(), []byte("/Type /Page ")); jml < 2 {
		t.Errorf("jumlah halaman terdeteksi = %d, want >= 2", jml)
	}

	// Tepat sebanyak batas = tidak terpotong.
	buf.Reset()
	n, terpotong, err = PDF(&buf, Opsi{Judul: "x", Total: -1}, kolomUji, &sumberUji{baris: baris[:100]}, 100)
	if err != nil || n != 100 || terpotong {
		t.Errorf("n=%d terpotong=%v err=%v, want 100, false", n, terpotong, err)
	}
}

func TestPDFTeksNonLatinDanDataKosongTidakPanik(t *testing.T) {
	src := &sumberUji{baris: [][]interface{}{
		{"Ünïcode — “kutip” … ‘x’ 日本語 € ½", int64(1)},
		{strings.Repeat("W", 500), nil},
	}}
	var buf bytes.Buffer
	if _, _, err := PDF(&buf, Opsi{Judul: "Uji “Unicode” 日本"}, []string{"a_b", "c"}, src, 0); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if n, _, err := PDF(&buf, Opsi{Judul: "Kosong"}, []string{"a"}, &sumberUji{}, 0); err != nil || n != 0 || !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Errorf("PDF kosong: n=%d err=%v", n, err)
	}
}

func TestPDFMeneruskanGalatSumber(t *testing.T) {
	src := contoh()
	src.galat = errors.New("koneksi putus")
	if _, _, err := PDF(&bytes.Buffer{}, Opsi{Judul: "x"}, kolomUji, src, 0); err == nil {
		t.Error("galat sumber harus diteruskan")
	}
}

func TestLabelDanAngkaID(t *testing.T) {
	for in, want := range map[string]string{
		"nama_paket": "Nama Paket", "kd_klpd": "Kode KLPD", "tgl_awal_pemilihan": "Tanggal Awal Pemilihan", "mtd_pemilihan": "Metode Pemilihan",
		"kd_satker_str": "Kode Satker", "pagu": "Pagu", "no_kontrak": "No. Kontrak", "x": "X", "": "",
	} {
		if got := Label(in); got != want {
			t.Errorf("Label(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[float64]string{
		0: "0", 12: "12", 999: "999", 1000: "1.000", 1234567.89: "1.234.567,89", -2500.5: "-2.500,5", 1500000.5: "1.500.000,5", 100000: "100.000",
	} {
		if got := AngkaID(in); got != want && !(in == -2500.5 && got == "-2.500,50") && !(in == 1500000.5 && got == "1.500.000,50") {
			t.Errorf("AngkaID(%v) = %q, want %q", in, got, want)
		}
	}
}
