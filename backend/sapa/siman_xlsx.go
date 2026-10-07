package sapa

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Impor daftar barang dari hasil ekspor data aset SIMAN ("Lampiran Data Aset", sheet "Permohonan Pengelolaan"). Berkas ini punya satu baris judul di atas daftar
// dan kolom yang berbeda dari template SAPA, jadi dibaca dengan pemetaan sendiri:
//
//	Nama Barang        <- Nama Barang
//	Kode Barang        <- Kode Barang
//	NUP                <- NUP
//	Lokasi/Merk/Tipe   <- Merk
//	Kondisi            <- Kondisi
//	Tahun Perolehan    <- tahun dari Tanggal Perolehan
//	Nilai Perolehan    <- Nilai Perolehan
//	Nilai Limit        <- Nilai Permohonan
//	Keterangan         <- Keterangan
//
// Kolom lain pada berkas SIMAN (Kode Register, Kode Satker, Nilai Buku, luas, status KIB, dan seterusnya) tidak dipakai.

// NamaSheetSIMAN: sheet berisi daftar pada ekspor SIMAN; sheet lain dicoba bila sheet ini tidak ada.
const NamaSheetSIMAN = "Permohonan Pengelolaan"

const (
	sNama = iota
	sKode
	sNUP
	sMerk
	sTanggal
	sPerolehan
	sPermohonan
	sKondisi
	sKet
	sSatker
	jumlahKolomSIMAN
)

// Judul kolom ekspor SIMAN (setelah dinormalkan: huruf kecil, hanya huruf dan angka).
var aliasSIMAN = [jumlahKolomSIMAN][]string{
	sNama:       {"namabarang"},
	sKode:       {"kodebarang"},
	sNUP:        {"nup"},
	sMerk:       {"merk"},
	sTanggal:    {"tanggalperolehan", "tglperolehan"},
	sPerolehan:  {"nilaiperolehan"},
	sPermohonan: {"nilaipermohonan"},
	sKondisi:    {"kondisi"},
	sKet:        {"keterangan"},
	sSatker:     {"kodesatker"},
}

var (
	reTglISO   = regexp.MustCompile(`^(\d{4})-\d{1,2}-\d{1,2}`)
	reTglID    = regexp.MustCompile(`^\d{1,2}[-/.]\d{1,2}[-/.](\d{4})`)
	reTahun4   = regexp.MustCompile(`^\d{4}$`)
	reTahunDi  = regexp.MustCompile(`(?:^|\D)((?:19|20)\d{2})(?:\D|$)`)
	reAwalanRp = regexp.MustCompile(`(?i)^(rp\.?|idr)\s*`)
)

// tahunDariTanggal mengambil tahun dari sel Tanggal Perolehan: teks ISO ("2015-12-30T00:00:00Z"), teks Indonesia ("30-12-2015", "30/12/2015", "30 Desember 2015"), tahun
// saja, atau tanggal Excel (nomor seri). Kosong dianggap sah (tahun dibiarkan kosong); ok=false bila ada isinya tetapi tidak terbaca.
func tahunDariTanggal(raw string) (tahun string, ok bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", true
	}
	if m := reTglISO.FindStringSubmatch(s); m != nil {
		return m[1], true
	}
	if m := reTglID.FindStringSubmatch(s); m != nil {
		return m[1], true
	}
	if reTahun4.MatchString(s) {
		return s, true
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil && v >= 1 && v < 2958466 {
		if t, err := excelize.ExcelDateToTime(v, false); err == nil {
			return strconv.Itoa(t.Year()), true
		}
	}
	if m := reTahunDi.FindStringSubmatch(s); m != nil {
		return m[1], true
	}
	return "", false
}

// uangPolos menulis nilai dalam sen sebagai angka rupiah polos: "9035434", atau "9035434.50" / "9035434.55" bila ada sen.
func uangPolos(sen int64) string {
	if sen%100 == 0 {
		return strconv.FormatInt(sen/100, 10)
	}
	return fmt.Sprintf("%d.%02d", sen/100, sen%100)
}

// uangSIMAN membaca nilai uang pada ekspor SIMAN, yang berupa teks berformat Indonesia ("Rp 9.035.434,00") atau angka biasa. Hasilnya angka polos; bila tidak
// terbaca, teksnya dikembalikan apa adanya (tanpa awalan Rp) supaya validasi barang melaporkannya di baris yang bersangkutan.
func uangSIMAN(raw string) string {
	s := strings.TrimSpace(strings.ReplaceAll(raw, " ", " "))
	if reAwalanRp.MatchString(s) || strings.Contains(s, ",") {
		s = strings.TrimSpace(reAwalanRp.ReplaceAllString(s, ""))
	} else {
		s = angkaUang(s)
	}
	if s == "" {
		return ""
	}
	if sen, err := ParseUang(s); err == nil {
		return uangPolos(sen)
	}
	return s
}

// petaKolom mencocokkan judul pada satu baris dengan alias kolom; hasilnya indeks kolom per jenis kolom (-1 bila tidak ada).
func petaKolom(row []string, alias [jumlahKolomSIMAN][]string) [jumlahKolomSIMAN]int {
	var col [jumlahKolomSIMAN]int
	for i := range col {
		col[i] = -1
	}
	for ci, cell := range row {
		n := normalJudul(cell)
		if n == "" {
			continue
		}
		for ki := range alias {
			if col[ki] >= 0 {
				continue
			}
			for _, a := range alias[ki] {
				if n == a {
					col[ki] = ci
				}
			}
		}
	}
	return col
}

// adaJudul: apakah salah satu dari baris-baris awal memuat salah satu judul kolom bernama-normal itu (untuk petunjuk salah pilih tombol).
func adaJudul(rows [][]string, nama ...string) bool {
	for r := 0; r < len(rows) && r < 10; r++ {
		for _, cell := range rows[r] {
			n := normalJudul(cell)
			for _, w := range nama {
				if n == w {
					return true
				}
			}
		}
	}
	return false
}

// adaJudulDiSheetMana memeriksa semua sheet, karena ekspor SIMAN menaruh daftarnya di sheet kedua (sheet pertama kosong).
func adaJudulDiSheetMana(f *excelize.File, nama ...string) bool {
	for _, s := range f.GetSheetList() {
		if rows, err := f.GetRows(s, excelize.Options{RawCellValue: true}); err == nil && adaJudul(rows, nama...) {
			return true
		}
	}
	return false
}

// ImporSIMANXLSX membaca berkas ekspor data aset SIMAN menjadi daftar barang SAPA. Sheet "Permohonan Pengelolaan" dipakai bila ada, selain itu sheet pertama yang
// memuat judul kolom SIMAN (Nama Barang, Nilai Perolehan, Nilai Permohonan). Seperti impor template: galat per baris tidak menghentikan impor.
func ImporSIMANXLSX(data []byte) (hasil *HasilImpor, err error) {
	defer func() {
		if r := recover(); r != nil {
			hasil, err = nil, validasi("Berkas tidak dapat dibaca sebagai Excel (.xlsx)")
		}
	}()
	f, err := bukaBerkasXLSX(data, "Gunakan berkas hasil ekspor data aset dari SIMAN.")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Sheet yang bernama "Permohonan Pengelolaan" diperiksa lebih dulu; sheet lain (mis. "Sheet1" yang kosong) sesudahnya.
	urut := []string{}
	for _, s := range f.GetSheetList() {
		if strings.EqualFold(s, NamaSheetSIMAN) {
			urut = append([]string{s}, urut...)
		} else {
			urut = append(urut, s)
		}
	}
	if len(urut) == 0 {
		return nil, validasi("Berkas tidak memuat sheet")
	}

	templateSAPA := false
	for _, sheet := range urut {
		rows, err := f.GetRows(sheet, excelize.Options{RawCellValue: true})
		if err != nil {
			continue
		}
		judulBaris := -1
		var col [jumlahKolomSIMAN]int
		for r := 0; r < len(rows) && r < 10; r++ {
			c := petaKolom(rows[r], aliasSIMAN)
			if c[sNama] >= 0 && c[sPerolehan] >= 0 && c[sPermohonan] >= 0 {
				judulBaris, col = r, c
				break
			}
		}
		if judulBaris < 0 {
			if adaJudul(rows, "nilailimit", "nilailimitrp") {
				templateSAPA = true
			}
			continue
		}
		return bacaBarisSIMAN(rows, judulBaris, col, sheet)
	}
	if templateSAPA {
		return nil, validasi("Berkas ini memakai judul kolom template SAPA (Nilai Limit), bukan ekspor SIMAN. Gunakan tombol Unggah Excel untuk template SAPA.")
	}
	return nil, validasi("Judul kolom data SIMAN tidak ditemukan. Kolom yang dibutuhkan: Nama Barang, Nilai Perolehan, dan Nilai Permohonan (sheet \"%s\" pada hasil ekspor data aset SIMAN).", NamaSheetSIMAN)
}

func bacaBarisSIMAN(rows [][]string, judulBaris int, col [jumlahKolomSIMAN]int, sheet string) (*HasilImpor, error) {
	out := &HasilImpor{Barang: []Barang{}, Galat: []string{}, Peringatan: []string{}}
	satker := map[string]bool{}
	tanpaLimit := 0
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
		if k := sel(row, col[sSatker]); k != "" {
			satker[k] = true
		}
		if len(out.Barang) >= MaksBarang {
			continue
		}
		tahun, tahunOK := tahunDariTanggal(sel(row, col[sTanggal]))
		b := Barang{
			Nama:           sel(row, col[sNama]),
			Kode:           angkaBulat(sel(row, col[sKode])),
			NUP:            angkaBulat(sel(row, col[sNUP])),
			Lokasi:         sel(row, col[sMerk]),
			Kondisi:        sel(row, col[sKondisi]),
			TahunPerolehan: tahun,
			NilaiPerolehan: uangSIMAN(sel(row, col[sPerolehan])),
			NilaiLimit:     uangSIMAN(sel(row, col[sPermohonan])),
			Keterangan:     sel(row, col[sKet]),
		}
		for _, k := range kondisiBarang {
			if strings.EqualFold(b.Kondisi, k) {
				b.Kondisi = k
			}
		}
		out.Barang = append(out.Barang, b)
		if !tahunOK {
			out.Galat = append(out.Galat, fmt.Sprintf("Tanggal Perolehan barang (baris %d) tidak dapat dibaca: \"%s\"; isi Tahun perolehan secara manual", r+1, sel(row, col[sTanggal])))
		}
		if l, e := ParseUang(b.NilaiLimit); e == nil && l == 0 {
			tanpaLimit++
		}
		out.Galat = append(out.Galat, validasiBarang(fmt.Sprintf(" (baris %d)", r+1), b)...)
	}
	if len(out.Barang) == 0 {
		return nil, validasi("Tidak ada baris data di bawah judul kolom pada sheet \"%s\".", sheet)
	}
	if out.JumlahBaca > MaksBarang {
		out.Peringatan = append(out.Peringatan, fmt.Sprintf("Berkas memuat %d baris data; hanya %d pertama yang dimuat (batas satu Nota Dinas)", out.JumlahBaca, MaksBarang))
	}
	if tanpaLimit > 0 {
		out.Peringatan = append(out.Peringatan, fmt.Sprintf("%d barang belum punya Nilai Permohonan di SIMAN (nilainya nol); isi Nilai limit barang itu di daftar di bawah", tanpaLimit))
	}
	if len(satker) > 1 {
		kode := make([]string, 0, len(satker))
		for k := range satker {
			kode = append(kode, k)
		}
		sort.Strings(kode)
		tampil := kode
		if len(tampil) > 3 {
			tampil = tampil[:3]
		}
		sisa := ""
		if len(kode) > len(tampil) {
			sisa = fmt.Sprintf(" dan %d lainnya", len(kode)-len(tampil))
		}
		out.Peringatan = append(out.Peringatan, fmt.Sprintf("Berkas memuat barang dari %d satker berbeda (%s%s); pastikan semuanya memang milik usulan ini", len(kode), strings.Join(tampil, ", "), sisa))
	}
	if len(out.Galat) > 50 {
		n := len(out.Galat) - 50
		out.Galat = append(out.Galat[:50], fmt.Sprintf("… dan %d masalah lain", n))
	}
	return out, nil
}
