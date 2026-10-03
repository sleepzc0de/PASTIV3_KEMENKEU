package sapa

import (
	"testing"
	"time"
)

func TestTerbilang(t *testing.T) {
	cases := map[int64]string{
		0:             "nol",
		1:             "satu",
		10:            "sepuluh",
		11:            "sebelas",
		12:            "dua belas",
		19:            "sembilan belas",
		20:            "dua puluh",
		21:            "dua puluh satu",
		99:            "sembilan puluh sembilan",
		100:           "seratus",
		101:           "seratus satu",
		111:           "seratus sebelas",
		200:           "dua ratus",
		999:           "sembilan ratus sembilan puluh sembilan",
		1000:          "seribu",
		1001:          "seribu satu",
		1100:          "seribu seratus",
		2000:          "dua ribu",
		11000:         "sebelas ribu",
		100000:        "seratus ribu",
		101000:        "seratus satu ribu",
		1000000:       "satu juta",
		1500000:       "satu juta lima ratus ribu",
		100000000:     "seratus juta",
		1000000000:    "satu miliar",
		2147483647:    "dua miliar seratus empat puluh tujuh juta empat ratus delapan puluh tiga ribu enam ratus empat puluh tujuh",
		1000000000000: "satu triliun",
		1001001001:    "satu miliar satu juta seribu satu",
		5000000000001: "lima triliun satu",
	}
	for n, want := range cases {
		if got := Terbilang(n); got != want {
			t.Errorf("Terbilang(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestRupiahDanTerbilang(t *testing.T) {
	cases := []struct {
		sen            int64
		rupiah, tulisa string
	}{
		{0, "Rp0,00", "(Nol Rupiah)"},
		{10000000000, "Rp100.000.000,00", "(Seratus Juta Rupiah)"}, // contoh dari ketentuan: Rp100.000.000,00 (Seratus Juta Rupiah)
		{150000000, "Rp1.500.000,00", "(Satu Juta Lima Ratus Ribu Rupiah)"},
		{100, "Rp1,00", "(Satu Rupiah)"},
		{100050, "Rp1.000,50", "(Seribu Rupiah Lima Puluh Sen)"},
		{5, "Rp0,05", "(Nol Rupiah Lima Sen)"},
	}
	for _, c := range cases {
		if got := Rupiah(c.sen); got != c.rupiah {
			t.Errorf("Rupiah(%d) = %q, want %q", c.sen, got, c.rupiah)
		}
		if got := RupiahTerbilang(c.sen); got != c.tulisa {
			t.Errorf("RupiahTerbilang(%d) = %q, want %q", c.sen, got, c.tulisa)
		}
	}
}

func TestJumlahTerbilang(t *testing.T) {
	if got := JumlahTerbilang(3, ""); got != "(Tiga)" {
		t.Errorf("got %q", got)
	}
	if got := JumlahTerbilang(21, " bidang "); got != "(Dua Puluh Satu) bidang" {
		t.Errorf("got %q", got)
	}
}

func TestParseUang(t *testing.T) {
	ok := map[string]int64{
		"1500000":        150000000,
		"1500000.5":      150000050,
		"1500000.50":     150000050,
		"1.500.000":      150000000,
		"1.500.000,50":   150000050,
		"Rp 1.500.000,5": 150000050,
		"1,5":            150,
		"0":              0,
		"1.500":          150000, // tepat tiga angka di belakang satu titik = ribuan
		"12.5":           1250,
		"250.75":         25075,
		" 100 ":          10000,
	}
	for in, want := range ok {
		got, err := ParseUang(in)
		if err != nil || got != want {
			t.Errorf("ParseUang(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "  ", "abc", "-5", "1e6", "1.234,567", "1.2.3,4,5", "99999999999999999999", ",", "Rp"} {
		if _, err := ParseUang(bad); err == nil {
			t.Errorf("ParseUang(%q) seharusnya galat", bad)
		}
	}
}

func TestTanggal(t *testing.T) {
	d, err := ParseTanggal("2026-10-03")
	if err != nil {
		t.Fatal(err)
	}
	if got := TanggalPanjang(d); got != "3 Oktober 2026" {
		t.Errorf("TanggalPanjang = %q", got)
	}
	if NamaHari(d) != "Sabtu" || NamaBulan(d) != "Oktober" {
		t.Errorf("hari/bulan = %s/%s", NamaHari(d), NamaBulan(d))
	}
	if z, err := ParseTanggal(""); err != nil || !z.IsZero() || TanggalPanjang(z) != "" {
		t.Errorf("tanggal kosong: %v %v", z, err)
	}
	for _, bad := range []string{"03-10-2026", "2026-13-01", "2026-02-30", "abc", "1800-01-01"} {
		if _, err := ParseTanggal(bad); err == nil {
			t.Errorf("ParseTanggal(%q) seharusnya galat", bad)
		}
	}
	_ = time.Now
}
