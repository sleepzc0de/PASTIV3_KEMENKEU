package sapa

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/xuri/excelize/v2"
)

// Template Excel untuk daftar barang: diunduh, diisi di Excel, lalu diunggah kembali (impor); daftar yang sedang dikerjakan juga
// bisa diunduh dalam format yang sama (ekspor) untuk diedit di luar aplikasi.

const (
	NamaSheetBarang   = "Daftar Barang"
	NamaSheetPetunjuk = "Petunjuk"
	MaksUkuranXLSX    = 2 << 20 // berkas yang diunggah
	maksBarisXLSX     = MaksBarang
	// Batas bongkar zip: berkas .xlsx kecil bisa berisi XML raksasa (zip bomb). Berkas sah untuk 500 baris jauh di bawah ini.
	batasZipXLSX = 20 << 20
	batasXMLXLSX = 10 << 20
)

// MaksBarang: banyak barang maksimal pada satu Nota Dinas (sama dengan validasi daftar barang).
const MaksBarang = 500

type kolomBarang struct {
	judul  string
	lebar  float64
	teks   bool     // format teks (kode, NUP): angka nol di depan dan panjang tidak dirusak Excel
	uang   bool     // format rupiah dua desimal
	alias  []string // judul kolom yang dikenali pada impor (setelah dinormalkan)
	petunj string
	contoh string
}

var kolomDaftarBarang = []kolomBarang{
	{judul: "Nama Barang", lebar: 34, alias: []string{"namabarang", "nama", "uraian", "uraianbarang"}, petunj: "Wajib. Nama/uraian barang.", contoh: "Gedung Kantor Lama"},
	{judul: "Kode Barang", lebar: 16, teks: true, alias: []string{"kodebarang", "kode"}, petunj: "Kode barang BMN (boleh kosong).", contoh: "4010101001"},
	{judul: "NUP", lebar: 10, teks: true, alias: []string{"nup", "nomorurutpendaftaran"}, petunj: "Nomor Urut Pendaftaran (boleh kosong).", contoh: "12"},
	{judul: "Lokasi/Merk/Tipe", lebar: 30, alias: []string{"lokasimerktipe", "lokasimerktipeidentitas", "lokasi", "merktipe", "merk"}, petunj: "Lokasi, merk, tipe, atau identitas barang.", contoh: "Jl. Merdeka No. 1, Jakarta"},
	{judul: "Kondisi", lebar: 14, alias: []string{"kondisi"}, petunj: "Pilih dari daftar: Baik, Rusak Ringan, Rusak Berat.", contoh: "Rusak Berat"},
	{judul: "Tahun Perolehan", lebar: 12, alias: []string{"tahunperolehan", "tahun"}, petunj: "Empat digit, mis. 1999.", contoh: "1999"},
	{judul: "Nilai Perolehan (Rp)", lebar: 20, uang: true, alias: []string{"nilaiperolehan", "nilaiperolehanrp", "nilaiperolehanrupiah", "hargaperolehan"}, petunj: "Wajib. Angka rupiah, boleh dua desimal.", contoh: "2500000000"},
	{judul: "Nilai Limit (Rp)", lebar: 20, uang: true, alias: []string{"nilailimit", "nilailimitrp", "nilailimitrupiah", "limit"}, petunj: "Wajib, lebih dari nol. Angka rupiah, boleh dua desimal.", contoh: "1500000000"},
	{judul: "Keterangan", lebar: 28, alias: []string{"keterangan", "ket"}, petunj: "Catatan bebas (boleh kosong).", contoh: "-"},
}

var kondisiBarang = []string{"Baik", "Rusak Ringan", "Rusak Berat"}

const (
	idxNama = iota
	idxKode
	idxNUP
	idxLokasi
	idxKondisi
	idxTahun
	idxPerolehan
	idxLimit
	idxKet
)

func namaKolomExcel(i int) string { n, _ := excelize.ColumnNumberToName(i + 1); return n }

// normalJudul: huruf kecil dan hanya huruf/angka, supaya "Nilai Perolehan (Rp)" dan "nilai_perolehan" dikenali sama.
func normalJudul(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func gayaBarang(f *excelize.File) (header, teks, uang, umum int, err error) {
	if header, err = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"3358E0"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    []excelize.Border{{Type: "left", Style: 1, Color: "C9D9FF"}, {Type: "right", Style: 1, Color: "C9D9FF"}, {Type: "top", Style: 1, Color: "C9D9FF"}, {Type: "bottom", Style: 1, Color: "C9D9FF"}},
	}); err != nil {
		return
	}
	text := "@"
	if teks, err = f.NewStyle(&excelize.Style{CustomNumFmt: &text, Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}}); err != nil {
		return
	}
	money := "#,##0.00"
	if uang, err = f.NewStyle(&excelize.Style{CustomNumFmt: &money, Alignment: &excelize.Alignment{Vertical: "top"}}); err != nil {
		return
	}
	umum, err = f.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}})
	return
}

// siapkanSheetBarang membuat sheet daftar barang: judul, lebar kolom, format sel, panel beku, dan validasi isian (Kondisi, Tahun,
// nilai). Baris data diisi pemanggil mulai baris 2.
func siapkanSheetBarang(f *excelize.File, sheet string) error {
	header, teks, uang, umum, err := gayaBarang(f)
	if err != nil {
		return err
	}
	for i, k := range kolomDaftarBarang {
		kol := namaKolomExcel(i)
		if err := f.SetCellStr(sheet, kol+"1", k.judul); err != nil {
			return err
		}
		if err := f.SetColWidth(sheet, kol, kol, k.lebar); err != nil {
			return err
		}
		gaya := umum
		if k.teks {
			gaya = teks
		} else if k.uang {
			gaya = uang
		}
		if err := f.SetColStyle(sheet, kol, gaya); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, kol+"1", kol+"1", header); err != nil {
			return err
		}
	}
	if err := f.SetRowHeight(sheet, 1, 34); err != nil {
		return err
	}
	if err := f.SetPanes(sheet, &excelize.Panes{Freeze: true, Split: false, XSplit: 0, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return err
	}
	akhir := strconv.Itoa(maksBarisXLSX + 1)
	rentang := func(i int) string { return namaKolomExcel(i) + "2:" + namaKolomExcel(i) + akhir }

	dvKondisi := excelize.NewDataValidation(true)
	dvKondisi.SetSqref(rentang(idxKondisi))
	if err := dvKondisi.SetDropList(kondisiBarang); err != nil {
		return err
	}
	dvKondisi.SetError(excelize.DataValidationErrorStyleWarning, "Kondisi tidak dikenal", "Pilih Baik, Rusak Ringan, atau Rusak Berat.")
	if err := f.AddDataValidation(sheet, dvKondisi); err != nil {
		return err
	}

	dvTahun := excelize.NewDataValidation(true)
	dvTahun.SetSqref(rentang(idxTahun))
	dvTahun.SetRange(1900, time.Now().Year()+1, excelize.DataValidationTypeWhole, excelize.DataValidationOperatorBetween)
	dvTahun.SetError(excelize.DataValidationErrorStyleStop, "Tahun tidak valid", "Isi tahun empat digit, mis. 1999.")
	if err := f.AddDataValidation(sheet, dvTahun); err != nil {
		return err
	}

	for _, i := range []int{idxPerolehan, idxLimit} {
		dv := excelize.NewDataValidation(true)
		dv.SetSqref(rentang(i))
		dv.SetRange(0, 9e15, excelize.DataValidationTypeDecimal, excelize.DataValidationOperatorBetween)
		dv.SetError(excelize.DataValidationErrorStyleStop, "Nilai tidak valid", "Isi angka rupiah (tanpa Rp dan tanpa teks), mis. 1500000.")
		if err := f.AddDataValidation(sheet, dv); err != nil {
			return err
		}
	}
	return nil
}

func sheetPetunjuk(f *excelize.File) error {
	if _, err := f.NewSheet(NamaSheetPetunjuk); err != nil {
		return err
	}
	judul, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14, Color: "253779"}})
	tebal, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E3EBFF"}}, Alignment: &excelize.Alignment{Vertical: "top"}})
	bungkus, _ := f.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}})
	teks := []string{
		"Petunjuk pengisian: Daftar Barang usulan penjualan BMN",
		"",
		"1. Isi data pada sheet \"" + NamaSheetBarang + "\" mulai baris 2 (satu barang per baris). Baris 1 adalah judul kolom: jangan diubah namanya.",
		"2. Urutan kolom boleh berubah dan kolom tambahan akan diabaikan, asalkan judul kolom wajib (Nama Barang, Nilai Perolehan, Nilai Limit) tetap ada.",
		"3. Nilai ditulis sebagai angka rupiah tanpa \"Rp\" (mis. 2500000000 atau 2500000,50). Jumlah barang dan total nilai dihitung otomatis oleh aplikasi.",
		fmt.Sprintf("4. Maksimal %d barang. Baris kosong dilewati.", MaksBarang),
		"5. Simpan berkas sebagai .xlsx lalu unggah kembali di formulir Nota Dinas (tombol Unggah Excel).",
		"6. Jenis BMN dan satuan jumlah dipilih di formulir, bukan di berkas ini.",
	}
	for i, t := range teks {
		sel := "A" + strconv.Itoa(i+1)
		if err := f.SetCellStr(NamaSheetPetunjuk, sel, t); err != nil {
			return err
		}
	}
	_ = f.SetCellStyle(NamaSheetPetunjuk, "A1", "A1", judul)
	baris := len(teks) + 2
	for j, h := range []string{"Kolom", "Keterangan", "Contoh"} {
		sel := namaKolomExcel(j) + strconv.Itoa(baris)
		_ = f.SetCellStr(NamaSheetPetunjuk, sel, h)
		_ = f.SetCellStyle(NamaSheetPetunjuk, sel, sel, tebal)
	}
	for i, k := range kolomDaftarBarang {
		r := strconv.Itoa(baris + 1 + i)
		_ = f.SetCellStr(NamaSheetPetunjuk, "A"+r, k.judul)
		_ = f.SetCellStr(NamaSheetPetunjuk, "B"+r, k.petunj)
		_ = f.SetCellStr(NamaSheetPetunjuk, "C"+r, k.contoh)
		_ = f.SetCellStyle(NamaSheetPetunjuk, "A"+r, "C"+r, bungkus)
	}
	_ = f.SetColWidth(NamaSheetPetunjuk, "A", "A", 24)
	_ = f.SetColWidth(NamaSheetPetunjuk, "B", "B", 58)
	_ = f.SetColWidth(NamaSheetPetunjuk, "C", "C", 30)
	return nil
}

func bukuBaru() (*excelize.File, error) {
	f := excelize.NewFile()
	if err := f.SetSheetName("Sheet1", NamaSheetBarang); err != nil {
		return nil, err
	}
	if err := siapkanSheetBarang(f, NamaSheetBarang); err != nil {
		return nil, err
	}
	if err := sheetPetunjuk(f); err != nil {
		return nil, err
	}
	idx, err := f.GetSheetIndex(NamaSheetBarang)
	if err == nil {
		f.SetActiveSheet(idx)
	}
	return f, nil
}

// TemplateBarangXLSX: berkas template kosong (judul kolom + validasi + petunjuk).
func TemplateBarangXLSX() ([]byte, error) {
	f, err := bukuBaru()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// EksporBarangXLSX: daftar barang yang sedang dikerjakan dalam format template yang sama, supaya bisa diedit di Excel lalu
// diunggah kembali. Nilai uang ditulis sebagai angka bila terbaca; semua teks ditulis sebagai teks (bukan rumus), sehingga isi
// yang diawali "=" atau "+" tidak dieksekusi Excel.
func EksporBarangXLSX(barang []Barang) ([]byte, error) {
	if len(barang) > MaksBarang {
		return nil, validasi("Daftar barang terlalu banyak untuk diekspor (maksimal %d)", MaksBarang)
	}
	f, err := bukuBaru()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	for i, b := range barang {
		r := strconv.Itoa(i + 2)
		nilai := []string{b.Nama, b.Kode, b.NUP, b.Lokasi, b.Kondisi, b.TahunPerolehan, b.NilaiPerolehan, b.NilaiLimit, b.Keterangan}
		for j, v := range nilai {
			sel := namaKolomExcel(j) + r
			if kolomDaftarBarang[j].uang {
				if sen, err := ParseUang(v); err == nil {
					if err := f.SetCellFloat(NamaSheetBarang, sel, float64(sen)/100, 2, 64); err != nil {
						return nil, err
					}
					continue
				}
			}
			if j == idxTahun {
				if y, err := strconv.Atoi(v); err == nil && len(v) == 4 {
					if err := f.SetCellInt(NamaSheetBarang, sel, int64(y)); err != nil {
						return nil, err
					}
					continue
				}
			}
			if err := f.SetCellStr(NamaSheetBarang, sel, v); err != nil {
				return nil, err
			}
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// HasilImpor: isi berkas yang diunggah. Galat per baris tidak menghentikan impor (baris tetap dimuat supaya bisa diperbaiki di
// formulir); hanya masalah pada berkas itu sendiri yang menggagalkannya.
type HasilImpor struct {
	Barang     []Barang `json:"barang"`
	Galat      []string `json:"galat"`
	Peringatan []string `json:"peringatan"`
	JumlahBaca int      `json:"jumlah_baca"` // baris data yang terbaca (tidak kosong)
}

var (
	reE      = regexp.MustCompile(`^[+-]?\d+(\.\d+)?[eE][+-]?\d+$`)
	reNoise  = regexp.MustCompile(`^-?\d+\.\d{6,}$`)
	reBulat0 = regexp.MustCompile(`^(-?\d+)\.0+$`)
)

// angkaUang merapikan nilai uang dari sel: notasi ilmiah diubah ke desimal, dan sisa kesalahan pembulatan Excel
// (1500000.1000000001) dibulatkan ke dua desimal. Teks lain (mis. "1.500.000,50") dibiarkan untuk ParseUang.
func angkaUang(raw string) string {
	s := strings.TrimSpace(raw)
	if reE.MatchString(s) {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			s = strconv.FormatFloat(v, 'f', -1, 64)
		}
	}
	if reNoise.MatchString(s) {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			s = strconv.FormatFloat(math.Round(v*100)/100, 'f', 2, 64)
		}
	}
	return s
}

// angkaBulat untuk kode, NUP, dan tahun: "2001.0" -> "2001", "2.010101001E9" -> "2010101001".
func angkaBulat(raw string) string {
	s := strings.TrimSpace(raw)
	if reE.MatchString(s) {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			s = strconv.FormatFloat(v, 'f', 0, 64)
		}
	}
	if m := reBulat0.FindStringSubmatch(s); m != nil {
		s = m[1]
	}
	return s
}

func sel(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// bukaBerkasXLSX memeriksa ukuran dan membuka berkas .xlsx yang diunggah dengan batas bongkar zip. petunjuk dipakai pada pesan bila berkasnya bukan .xlsx yang sah.
// Pemanggil memasang pemulih panic (recover) lebih dulu karena pustaka pembaca menerima berkas dari pengguna.
func bukaBerkasXLSX(data []byte, petunjuk string) (*excelize.File, error) {
	if len(data) == 0 {
		return nil, validasi("Berkas kosong")
	}
	if len(data) > MaksUkuranXLSX {
		return nil, validasi("Berkas terlalu besar (maksimal %d MB)", MaksUkuranXLSX>>20)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{UnzipSizeLimit: batasZipXLSX, UnzipXMLSizeLimit: batasXMLXLSX})
	if err != nil {
		return nil, validasi("Berkas tidak dapat dibaca sebagai Excel (.xlsx). %s", petunjuk)
	}
	return f, nil
}

// ImporBarangXLSX membaca berkas Excel (template atau hasil ekspor) menjadi daftar barang. Sheet "Daftar Barang" dipakai bila
// ada, selain itu sheet pertama. Kolom dikenali dari judulnya (urutan bebas, kolom tambahan diabaikan).
func ImporBarangXLSX(data []byte) (hasil *HasilImpor, err error) {
	// Pustaka pembaca .xlsx diberi berkas dari pengguna: galat tak terduga (panic) tidak boleh menjatuhkan server.
	defer func() {
		if r := recover(); r != nil {
			hasil, err = nil, validasi("Berkas tidak dapat dibaca sebagai Excel (.xlsx)")
		}
	}()
	f, err := bukaBerkasXLSX(data, "Gunakan template dari tombol Unduh template.")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sheet := ""
	list := f.GetSheetList()
	for _, s := range list {
		if strings.EqualFold(s, NamaSheetBarang) {
			sheet = s
		}
	}
	if sheet == "" && len(list) > 0 {
		sheet = list[0]
	}
	if sheet == "" {
		return nil, validasi("Berkas tidak memuat sheet")
	}
	rows, err := f.GetRows(sheet, excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, validasi("Isi sheet tidak dapat dibaca")
	}

	// Baris judul: baris pertama (dari 10 baris awal) yang memuat ketiga judul wajib.
	col := make([]int, len(kolomDaftarBarang))
	judulBaris := -1
	for r := 0; r < len(rows) && r < 10; r++ {
		peta := map[int]int{}
		for ci, cell := range rows[r] {
			n := normalJudul(cell)
			if n == "" {
				continue
			}
			for ki, k := range kolomDaftarBarang {
				if _, dipakai := peta[ki]; dipakai {
					continue
				}
				for _, a := range k.alias {
					if n == a {
						peta[ki] = ci
					}
				}
			}
		}
		if _, a := peta[idxNama]; a {
			if _, b := peta[idxPerolehan]; b {
				if _, c := peta[idxLimit]; c {
					for ki := range col {
						if ci, ok := peta[ki]; ok {
							col[ki] = ci
						} else {
							col[ki] = -1
						}
					}
					judulBaris = r
					break
				}
			}
		}
	}
	if judulBaris < 0 {
		if adaJudulDiSheetMana(f, "nilaipermohonan") {
			return nil, validasi("Berkas ini tampaknya hasil ekspor data aset SIMAN (ada kolom Nilai Permohonan). Gunakan tombol Unggah Data SIMAN.")
		}
		return nil, validasi("Judul kolom tidak ditemukan. Kolom wajib: Nama Barang, Nilai Perolehan (Rp), Nilai Limit (Rp). Gunakan template dari tombol Unduh template.")
	}

	out := &HasilImpor{Barang: []Barang{}, Galat: []string{}, Peringatan: []string{}}
	for r := judulBaris + 1; r < len(rows); r++ {
		row := rows[r]
		kosong := true
		for _, c := range row {
			if strings.TrimSpace(c) != "" {
				kosong = false
				break
			}
		}
		if kosong {
			continue
		}
		out.JumlahBaca++
		if len(out.Barang) >= MaksBarang {
			continue
		}
		b := Barang{
			Nama:           sel(row, col[idxNama]),
			Kode:           angkaBulat(sel(row, col[idxKode])),
			NUP:            angkaBulat(sel(row, col[idxNUP])),
			Lokasi:         sel(row, col[idxLokasi]),
			Kondisi:        sel(row, col[idxKondisi]),
			TahunPerolehan: angkaBulat(sel(row, col[idxTahun])),
			NilaiPerolehan: angkaUang(sel(row, col[idxPerolehan])),
			NilaiLimit:     angkaUang(sel(row, col[idxLimit])),
			Keterangan:     sel(row, col[idxKet]),
		}
		// Kondisi disamakan dengan pilihan yang dikenal (huruf besar/kecil), selain itu dibiarkan apa adanya.
		for _, k := range kondisiBarang {
			if strings.EqualFold(b.Kondisi, k) {
				b.Kondisi = k
			}
		}
		out.Barang = append(out.Barang, b)
		for _, g := range validasiBarang(fmt.Sprintf(" (baris %d)", r+1), b) {
			out.Galat = append(out.Galat, g)
		}
	}
	if out.JumlahBaca > MaksBarang {
		out.Peringatan = append(out.Peringatan, fmt.Sprintf("Berkas memuat %d baris data; hanya %d pertama yang dimuat (batas satu Nota Dinas)", out.JumlahBaca, MaksBarang))
	}
	if len(out.Barang) == 0 {
		return nil, validasi("Tidak ada baris data pada berkas. Isi data mulai baris 2 pada sheet \"%s\".", sheet)
	}
	if len(out.Galat) > 50 {
		n := len(out.Galat) - 50
		out.Galat = append(out.Galat[:50], fmt.Sprintf("… dan %d masalah lain", n))
	}
	return out, nil
}
