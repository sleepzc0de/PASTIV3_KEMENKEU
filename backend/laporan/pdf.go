package laporan

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-pdf/fpdf"
)

// Ukuran halaman: A4 lanskap.
const (
	halamanLebar  = 297.0
	halamanTinggi = 210.0
	marginKiri    = 10.0
	marginKanan   = 10.0
	marginAtas    = 12.0
	marginBawah   = 14.0
	tinggiBaris   = 5.2
	tinggiJudul   = 6.2
	fontIsi       = 7.0
)

// MaksBarisPDF: batas bawaan baris data satu berkas PDF. PDF cocok untuk dibaca, bukan untuk data lengkap: Excel dan CSV tidak dibatasi.
const MaksBarisPDF = 5000

// Singkatan yang ditulis huruf besar semua pada judul kolom PDF.
var singkatan = map[string]string{
	"kd": "Kode", "klpd": "KLPD", "rup": "RUP", "hps": "HPS", "ppk": "PPK", "npwp": "NPWP", "mak": "MAK", "nib": "NIB", "id": "ID",
	"tgl": "Tanggal", "mtd": "Metode", "no": "No.", "nip": "NIP", "lpse": "LPSE", "ukm": "UKM", "bap": "BAP", "bast": "BAST",
	"spmk": "SPMK", "spp": "SPP", "kbli": "KBLI", "ekontrak": "E-Kontrak", "epurchasing": "E-Purchasing", "str": "",
}

// Label mengubah nama kolom (nama_paket) menjadi judul kolom yang enak dibaca (Nama Paket).
func Label(kolom string) string {
	var bagian []string
	for _, p := range strings.Split(kolom, "_") {
		if p == "" {
			continue
		}
		if s, ok := singkatan[strings.ToLower(p)]; ok {
			if s != "" {
				bagian = append(bagian, s)
			}
			continue
		}
		bagian = append(bagian, strings.ToUpper(p[:1])+p[1:])
	}
	if len(bagian) == 0 {
		return kolom
	}
	return strings.Join(bagian, " ")
}

// AngkaID menulis bilangan dengan pemisah ribuan titik dan desimal koma (1.234.567,89); bilangan bulat tanpa desimal.
func AngkaID(f float64) string {
	neg := f < 0
	if neg {
		f = -f
	}
	s := strconv.FormatFloat(f, 'f', 2, 64)
	utuh, desimal := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		utuh, desimal = s[:i], s[i+1:]
	}
	var b strings.Builder
	for i, r := range utuh {
		if i > 0 && (len(utuh)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	out := b.String()
	if desimal != "" && desimal != "00" {
		out += "," + desimal
	}
	if neg {
		out = "-" + out
	}
	return out
}

func teksPDF(v interface{}) (string, bool) {
	switch x := v.(type) {
	case float64:
		return AngkaID(x), true
	case float32:
		return AngkaID(float64(x)), true
	case int, int32, int64:
		return Teks(x), true
	}
	return Teks(v), false
}

// PDF menulis tabel PDF (A4 lanskap) dengan judul kolom yang diulang di tiap halaman. Hanya maks baris pertama yang ditulis;
// terpotong = masih ada baris yang tidak ditampilkan. Teks panjang dipotong dengan "..." agar baris tetap satu garis.
func PDF(w io.Writer, o Opsi, kolom []string, src Sumber, maks int) (baris int, terpotong bool, err error) {
	if maks <= 0 {
		maks = MaksBarisPDF
	}
	// Dibaca dulu (dibatasi maks) karena lebar kolom ditentukan dari isinya.
	var data [][]string
	var angka [][]bool
	for len(data) < maks && src.Next() {
		nilai := src.Nilai()
		t := make([]string, len(kolom))
		a := make([]bool, len(kolom))
		for i := range t {
			if i < len(nilai) {
				t[i], a[i] = teksPDF(nilai[i])
			}
		}
		data = append(data, t)
		angka = append(angka, a)
	}
	if err := src.Err(); err != nil {
		return 0, false, err
	}
	terpotong = len(data) >= maks && src.Next()
	if err := src.Err(); err != nil {
		return 0, false, err
	}

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(marginKiri, marginAtas, marginKanan)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetTitle(o.Judul, true)
	pdf.SetCreator("PASTI V3", true)
	pdf.AliasNbPages("{nb}")
	tr := pdf.UnicodeTranslatorFromDescriptor("") // teks UTF-8 -> cp1252 (huruf Latin dan tanda baca umum)
	dibuat := time.Now().In(time.FixedZone("WIB", 7*3600)).Format("02-01-2006 15:04") + " WIB"
	pdf.SetFooterFunc(func() {
		pdf.SetY(-9)
		pdf.SetFont("Helvetica", "", 7)
		pdf.SetTextColor(110, 110, 110)
		pdf.CellFormat(0, 4, tr(fmt.Sprintf("PASTI V3 - %s - dicetak %s - halaman %d dari {nb}", o.Judul, dibuat, pdf.PageNo())), "", 0, "C", false, 0, "")
	})

	lebar := lebarKolomPDF(kolom, data)
	judulKolom := make([]string, len(kolom))
	for i, k := range kolom {
		judulKolom[i] = Label(k)
	}

	muat := func(teks string, lebarSel float64) string {
		teks = tr(strings.Join(strings.Fields(teks), " "))
		maksLebar := lebarSel - 2
		if pdf.GetStringWidth(teks) <= maksLebar {
			return teks
		}
		r := []rune(teks)
		for len(r) > 1 {
			r = r[:len(r)-1]
			if pdf.GetStringWidth(string(r)+"...") <= maksLebar {
				return string(r) + "..."
			}
		}
		return "..."
	}
	barisJudul := func() {
		pdf.SetFont("Helvetica", "B", fontIsi)
		pdf.SetFillColor(31, 58, 138)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetDrawColor(200, 200, 200)
		for i, j := range judulKolom {
			pdf.CellFormat(lebar[i], tinggiBaris+0.8, muat(j, lebar[i]), "1", 0, "L", true, 0, "")
		}
		pdf.Ln(-1)
		pdf.SetTextColor(30, 30, 30)
		pdf.SetFont("Helvetica", "", fontIsi)
	}

	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 13)
	pdf.SetTextColor(15, 23, 42)
	pdf.CellFormat(0, tinggiJudul, tr(o.Judul), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(90, 90, 90)
	for _, s := range o.Subjudul {
		pdf.CellFormat(0, 4.2, tr(s), "", 1, "L", false, 0, "")
	}
	ringkasan := fmt.Sprintf("%s baris ditampilkan", AngkaID(float64(len(data))))
	if o.Total >= 0 && terpotong {
		ringkasan = fmt.Sprintf("%s baris pertama dari %s baris (data lengkap: gunakan Excel atau CSV)", AngkaID(float64(len(data))), AngkaID(float64(o.Total)))
	} else if terpotong {
		ringkasan += " (masih ada baris lain; data lengkap: gunakan Excel atau CSV)"
	}
	pdf.CellFormat(0, 4.2, tr(ringkasan), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	barisJudul()
	batasBawah := halamanTinggi - marginBawah
	for r, sel := range data {
		if pdf.GetY()+tinggiBaris > batasBawah {
			pdf.AddPage()
			barisJudul()
		}
		if r%2 == 1 {
			pdf.SetFillColor(243, 245, 250)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		for i, teks := range sel {
			rata := "L"
			if angka[r][i] {
				rata = "R"
			}
			pdf.CellFormat(lebar[i], tinggiBaris, muat(teks, lebar[i]), "1", 0, rata, true, 0, "")
		}
		pdf.Ln(-1)
	}
	if len(data) == 0 {
		pdf.SetFont("Helvetica", "I", 9)
		pdf.CellFormat(0, 8, "Tidak ada data yang cocok dengan penyaring ini.", "", 1, "L", false, 0, "")
	}

	if err := pdf.Error(); err != nil {
		return 0, false, err
	}
	if err := pdf.Output(w); err != nil {
		return 0, false, err
	}
	return len(data), terpotong, nil
}

// lebarKolomPDF membagi lebar halaman menurut panjang judul dan isi (dibatasi), dengan lebar minimum per kolom.
func lebarKolomPDF(kolom []string, data [][]string) []float64 {
	n := len(kolom)
	bobot := make([]float64, n)
	for i, k := range kolom {
		m := utf8.RuneCountInString(Label(k))
		for _, b := range data {
			if l := utf8.RuneCountInString(b[i]); l > m {
				m = l
			}
		}
		switch {
		case m < 6:
			m = 6
		case m > 38:
			m = 38
		}
		bobot[i] = float64(m)
	}
	pakai := halamanLebar - marginKiri - marginKanan
	var jumlah float64
	for _, b := range bobot {
		jumlah += b
	}
	out := make([]float64, n)
	for i, b := range bobot {
		out[i] = b / jumlah * pakai
		if out[i] < 11 {
			out[i] = 11
		}
	}
	// Pembulatan ke atas ke lebar minimum bisa melampaui halaman: skala ulang.
	var total float64
	for _, w := range out {
		total += w
	}
	if total > pakai {
		for i := range out {
			out[i] = out[i] / total * pakai
		}
	}
	return out
}
