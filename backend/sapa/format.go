// Package sapa memuat logika Sistem Administrasi Pengelolaan Aset (SAPA): alur kerja, data formulir, pengisian
// template dokumen Word, dan aturan hak akses. Modul pertama adalah Penjualan; modul lain (Sewa, dll.) menyusul.
package sapa

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------- terbilang

var satuan = []string{"nol", "satu", "dua", "tiga", "empat", "lima", "enam", "tujuh", "delapan", "sembilan", "sepuluh", "sebelas"}

// skala memuat kata untuk kelompok tiga digit dari kanan: (satuan), ribu, juta, miliar, triliun, kuadriliun.
var skala = []string{"", "ribu", "juta", "miliar", "triliun", "kuadriliun"}

func ratusan(n int) string { // 1..999
	var parts []string
	if h := n / 100; h > 0 {
		if h == 1 {
			parts = append(parts, "seratus")
		} else {
			parts = append(parts, satuan[h]+" ratus")
		}
	}
	r := n % 100
	switch {
	case r == 0:
	case r < 12:
		parts = append(parts, satuan[r])
	case r < 20:
		parts = append(parts, satuan[r-10]+" belas")
	default:
		s := satuan[r/10] + " puluh"
		if r%10 > 0 {
			s += " " + satuan[r%10]
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " ")
}

// Terbilang mengeja bilangan bulat tidak negatif dalam bahasa Indonesia, huruf kecil ("seribu dua ratus").
func Terbilang(n int64) string {
	if n < 0 {
		return "minus " + Terbilang(-n)
	}
	if n == 0 {
		return satuan[0]
	}
	var groups []int
	for x := n; x > 0; x /= 1000 {
		groups = append(groups, int(x%1000))
	}
	var parts []string
	for i := len(groups) - 1; i >= 0; i-- {
		g := groups[i]
		if g == 0 {
			continue
		}
		switch {
		case i == 1 && g == 1:
			parts = append(parts, "seribu") // 1.000 = "seribu", bukan "satu ribu"
		case i == 0:
			parts = append(parts, ratusan(g))
		default:
			parts = append(parts, ratusan(g)+" "+skala[i])
		}
	}
	return strings.Join(parts, " ")
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// JumlahTerbilang: "(Tiga)" atau, bila ada satuan, "(Tiga) unit".
func JumlahTerbilang(n int64, satuanBarang string) string {
	s := "(" + titleCase(Terbilang(n)) + ")"
	if t := strings.TrimSpace(satuanBarang); t != "" {
		s += " " + t
	}
	return s
}

// ---------------------------------------------------------------- uang

// MaxSen adalah batas nilai uang yang diterima (sekitar 9 kuadriliun rupiah): cukup besar, dan jauh dari batas int64.
const MaxSen = 900_000_000_000_000_000

// ParseUang membaca nilai rupiah menjadi sen. Format yang diterima: "1500000", "1500000.50" (titik desimal, bentuk
// yang dikirim formulir) atau "1.500.000,50" (format Indonesia). Paling banyak dua angka desimal; tidak negatif.
func ParseUang(s string) (int64, error) {
	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(strings.TrimPrefix(t, "Rp"), "rp")
	t = strings.ReplaceAll(strings.TrimSpace(t), " ", "")
	if t == "" {
		return 0, errors.New("nilai kosong")
	}
	switch {
	case strings.Contains(t, ","):
		t = strings.ReplaceAll(t, ".", "")
		t = strings.Replace(t, ",", ".", 1)
	case strings.Count(t, ".") >= 2:
		t = strings.ReplaceAll(t, ".", "")
	case strings.Count(t, ".") == 1:
		i := strings.Index(t, ".")
		// "1.500" (tepat tiga angka di belakang titik) dibaca sebagai pemisah ribuan.
		if len(t)-i-1 == 3 && i >= 1 && i <= 3 {
			t = strings.Replace(t, ".", "", 1)
		}
	}
	intPart, frac := t, ""
	if i := strings.Index(t, "."); i >= 0 {
		intPart, frac = t[:i], t[i+1:]
	}
	if intPart == "" && frac == "" {
		return 0, errors.New("nilai tidak valid")
	}
	if len(frac) > 2 {
		return 0, errors.New("paling banyak dua angka di belakang koma")
	}
	for _, part := range []string{intPart, frac} {
		for _, r := range part {
			if r < '0' || r > '9' {
				return 0, errors.New("nilai harus berupa angka")
			}
		}
	}
	if len(intPart) > 16 {
		return 0, errors.New("nilai terlalu besar")
	}
	rp := int64(0)
	if intPart != "" {
		v, err := strconv.ParseInt(intPart, 10, 64)
		if err != nil {
			return 0, errors.New("nilai tidak valid")
		}
		rp = v
	}
	frac += strings.Repeat("0", 2-len(frac))
	sen, _ := strconv.ParseInt(frac, 10, 64)
	total := rp*100 + sen
	if rp > MaxSen/100 || total > MaxSen {
		return 0, errors.New("nilai terlalu besar")
	}
	return total, nil
}

func groupThousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, c)
	}
	return string(out)
}

// Angka memformat sen menjadi "1.500.000,50" (selalu dua desimal).
func Angka(sen int64) string {
	return fmt.Sprintf("%s,%02d", groupThousands(sen/100), sen%100)
}

// Rupiah memformat sen menjadi "Rp1.500.000,00".
func Rupiah(sen int64) string { return "Rp" + Angka(sen) }

// RupiahTerbilang: "(Satu Juta Lima Ratus Ribu Rupiah)"; bila ada sen: "(... Rupiah Lima Puluh Sen)".
func RupiahTerbilang(sen int64) string {
	s := titleCase(Terbilang(sen/100)) + " Rupiah"
	if c := sen % 100; c > 0 {
		s += " " + titleCase(Terbilang(c)) + " Sen"
	}
	return "(" + s + ")"
}

// ---------------------------------------------------------------- tanggal

var namaBulan = []string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
var namaHari = []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}

// ParseTanggal membaca "YYYY-MM-DD" (bentuk kotak tanggal HTML). Kosong = tanggal nol, tanpa galat.
func ParseTanggal(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, errors.New("tanggal harus berformat TTTT-BB-HH")
	}
	if t.Year() < 1990 || t.Year() > 2100 {
		return time.Time{}, errors.New("tahun tanggal tidak wajar")
	}
	return t, nil
}

// TanggalPanjang: "3 Oktober 2026"; tanggal nol menghasilkan teks kosong.
func TanggalPanjang(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return fmt.Sprintf("%d %s %d", t.Day(), namaBulan[t.Month()], t.Year())
}

func NamaHari(t time.Time) string { return namaHari[t.Weekday()] }

func NamaBulan(t time.Time) string { return namaBulan[t.Month()] }
