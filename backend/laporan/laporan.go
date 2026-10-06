// Package laporan menulis hasil query ke berkas ekspor (CSV, Excel, PDF) secara mengalir: baris dibaca satu per satu dari Sumber
// dan langsung ditulis, jadi tabel besar tidak dimuat seluruhnya ke memori (kecuali PDF, yang dibatasi jumlah barisnya).
package laporan

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

// Sumber: baris data yang akan ditulis. Nilai() memuat satu nilai per kolom: nil, string, []byte, bool, bilangan, atau time.Time.
type Sumber interface {
	Next() bool
	Nilai() []interface{}
	Err() error
}

// Opsi: keterangan berkas.
type Opsi struct {
	Judul     string   // judul di sheet/PDF
	Subjudul  []string // baris keterangan (penyaring, waktu cetak) di PDF
	NamaSheet string   // nama sheet Excel
	Total     int64    // jumlah seluruh baris yang cocok (untuk keterangan PDF yang dipotong); negatif = tidak diketahui
	// FormatKolom: format angka Excel khusus per kolom (indeks kolom mulai 0 → mis. "0.0000000") untuk nilai desimal; kolom lain memakai #,##0.00.
	// Dipakai untuk koordinat, yang kehilangan ketelitian tampilannya bila hanya dua desimal.
	FormatKolom map[int]string
}

// MaksBarisExcel: batas baris data satu sheet Excel (1.048.576 baris dikurangi baris judul kolom).
const MaksBarisExcel = 1048575

// batasBarisExcel adalah variabel hanya supaya tes bisa menurunkannya.
var batasBarisExcel = MaksBarisExcel

// ErrMelebihiBatas: jumlah baris melebihi batas format.
var ErrMelebihiBatas = errors.New("jumlah baris melebihi batas format")

// Teks mengubah satu nilai menjadi teks untuk CSV dan PDF. Waktu tanpa jam ditulis sebagai tanggal saja.
func Teks(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	case bool:
		if x {
			return "Ya"
		}
		return "Tidak"
	case int:
		return strconv.Itoa(x)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case time.Time:
		if x.Hour() == 0 && x.Minute() == 0 && x.Second() == 0 {
			return x.Format("2006-01-02")
		}
		return x.Format("2006-01-02 15:04:05")
	}
	return fmt.Sprint(v)
}

// amanCSV mencegah teks dari sumber luar dibaca sebagai rumus oleh Excel (CSV injection): sel yang diawali = + - @ atau tab/CR diberi
// apostrof di depannya, kecuali isinya bilangan biasa.
func amanCSV(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return "'" + s
		}
	}
	return s
}

// CSV menulis berkas CSV UTF-8 (dengan BOM supaya Excel membaca huruf dengan benar). Mengembalikan jumlah baris data yang ditulis.
func CSV(w io.Writer, kolom []string, src Sumber, pemisah rune) (int, error) {
	bw := bufio.NewWriterSize(w, 64*1024)
	if _, err := bw.WriteString("\xEF\xBB\xBF"); err != nil {
		return 0, err
	}
	cw := csv.NewWriter(bw)
	cw.Comma = pemisah
	cw.UseCRLF = true
	if err := cw.Write(kolom); err != nil {
		return 0, err
	}
	rec := make([]string, len(kolom))
	n := 0
	for src.Next() {
		nilai := src.Nilai()
		for i := range rec {
			rec[i] = ""
			if i < len(nilai) {
				switch v := nilai[i].(type) {
				case string, []byte:
					rec[i] = amanCSV(Teks(v))
				default:
					rec[i] = Teks(v)
				}
			}
		}
		if err := cw.Write(rec); err != nil {
			return n, err
		}
		n++
	}
	if err := src.Err(); err != nil {
		return n, err
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return n, err
	}
	return n, bw.Flush()
}

// namaSheetAman: nama sheet Excel maksimal 31 karakter tanpa [ ] : * ? / \ .
func namaSheetAman(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '[', ']', ':', '*', '?', '/', '\\':
			return ' '
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if s == "" {
		s = "Data"
	}
	if utf8.RuneCountInString(s) > 31 {
		s = string([]rune(s)[:31])
	}
	return s
}

// lebarKolom memperkirakan lebar kolom Excel dari panjang judul dan contoh isi.
func lebarKolom(judul string, contoh []string) float64 {
	n := utf8.RuneCountInString(judul)
	for _, c := range contoh {
		if l := utf8.RuneCountInString(c); l > n {
			n = l
		}
	}
	w := float64(n) + 3
	switch {
	case w < 10:
		return 10
	case w > 60:
		return 60
	}
	return w
}

const contohLebar = 100 // baris pertama yang dipakai menentukan lebar kolom

// XLSX menulis berkas Excel satu sheet secara mengalir: judul kolom (dibekukan, dengan filter), lalu data. Bilangan ditulis sebagai
// bilangan dan waktu sebagai tanggal sungguhan. Mengembalikan jumlah baris data; ErrMelebihiBatas bila melebihi MaksBarisExcel.
func XLSX(w io.Writer, o Opsi, kolom []string, src Sumber) (int, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := namaSheetAman(o.NamaSheet)
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return 0, err
	}
	styleTanggal, err := f.NewStyle(&excelize.Style{CustomNumFmt: strPtr("yyyy-mm-dd")})
	if err != nil {
		return 0, err
	}
	styleWaktu, err := f.NewStyle(&excelize.Style{CustomNumFmt: strPtr("yyyy-mm-dd hh:mm:ss")})
	if err != nil {
		return 0, err
	}
	styleDesimal, err := f.NewStyle(&excelize.Style{NumFmt: 4}) // #,##0.00
	if err != nil {
		return 0, err
	}
	styleKolom := map[int]int{} // indeks kolom → gaya angka khusus (Opsi.FormatKolom)
	styleFormat := map[string]int{}
	for i, format := range o.FormatKolom {
		st, ada := styleFormat[format]
		if !ada {
			if st, err = f.NewStyle(&excelize.Style{CustomNumFmt: strPtr(format)}); err != nil {
				return 0, err
			}
			styleFormat[format] = st
		}
		styleKolom[i] = st
	}
	styleJudul, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"1F3A8A"}},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	if err != nil {
		return 0, err
	}

	// Lebar kolom harus ditetapkan sebelum baris pertama ditulis, jadi sebagian baris dibaca dulu.
	var contoh [][]interface{}
	for len(contoh) < contohLebar && src.Next() {
		contoh = append(contoh, append([]interface{}(nil), src.Nilai()...))
	}
	if err := src.Err(); err != nil {
		return 0, err
	}

	sw, err := f.NewStreamWriter(sheet)
	if err != nil {
		return 0, err
	}
	for i, k := range kolom {
		var teks []string
		for _, b := range contoh {
			if i < len(b) {
				teks = append(teks, Teks(b[i]))
			}
		}
		if err := sw.SetColWidth(i+1, i+1, lebarKolom(k, teks)); err != nil {
			return 0, err
		}
	}
	if err := sw.SetPanes(&excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return 0, err
	}

	judul := make([]interface{}, len(kolom))
	for i, k := range kolom {
		judul[i] = excelize.Cell{Value: k, StyleID: styleJudul}
	}
	if err := sw.SetRow("A1", judul, excelize.RowOpts{Height: 20}); err != nil {
		return 0, err
	}

	baris := 0
	tulis := func(nilai []interface{}) error {
		if baris >= batasBarisExcel {
			return ErrMelebihiBatas
		}
		sel := make([]interface{}, len(kolom))
		for i := range sel {
			if i >= len(nilai) {
				continue
			}
			switch v := nilai[i].(type) {
			case time.Time:
				st := styleWaktu
				if v.Hour() == 0 && v.Minute() == 0 && v.Second() == 0 {
					st = styleTanggal
				}
				sel[i] = excelize.Cell{Value: v, StyleID: st}
			case float64:
				st := styleDesimal
				if khusus, ada := styleKolom[i]; ada {
					st = khusus
				}
				sel[i] = excelize.Cell{Value: v, StyleID: st}
			default:
				sel[i] = v
			}
		}
		baris++
		cell, _ := excelize.CoordinatesToCellName(1, baris+1)
		return sw.SetRow(cell, sel)
	}
	for _, b := range contoh {
		if err := tulis(b); err != nil {
			return baris, err
		}
	}
	for src.Next() {
		if err := tulis(src.Nilai()); err != nil {
			return baris, err
		}
	}
	if err := src.Err(); err != nil {
		return baris, err
	}

	if baris > 0 {
		akhir, _ := excelize.CoordinatesToCellName(len(kolom), baris+1)
		if err := sw.AddTable(&excelize.Table{Range: "A1:" + akhir, Name: "Data", StyleName: "TableStyleLight9"}); err != nil {
			return baris, err
		}
	}
	if err := sw.Flush(); err != nil {
		return baris, err
	}
	return baris, f.Write(w)
}

func strPtr(s string) *string { return &s }
