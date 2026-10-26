package datamart

import (
	"regexp"
	"strings"
	"testing"
)

// Pembersihan membuang SEMUA rutin yang dibuat script, dan hanya itu: daftar rutin yang dibuat harus sama dengan yang dibuang, supaya bawaan (@Bersihkan = 1) benar-benar menyisakan
// hanya tabel dan tidak ada rutin yang tertinggal atau terbuang tanpa dibuat script ini.
func TestPembersihanMembuangSemuaRutinSementara(t *testing.T) {
	skrip := bacaSkripDigitalisasi(t)
	i := strings.Index(skrip, "-- Pembersihan:")
	if i < 0 {
		t.Fatal("batch pembersihan tidak ada")
	}
	dibuat := map[string]bool{}
	for _, m := range regexp.MustCompile(`CREATE OR ALTER (?:FUNCTION|PROCEDURE) dbo\.(\w+)`).FindAllStringSubmatch(skrip[:i], -1) {
		dibuat[m[1]] = true
	}
	bersih := skrip[i:]
	dibuang := map[string]bool{}
	for _, m := range regexp.MustCompile(`DROP (?:FUNCTION|PROCEDURE) IF EXISTS dbo\.(\w+);`).FindAllStringSubmatch(bersih, -1) {
		dibuang[m[1]] = true
	}
	if len(dibuat) < 17 {
		t.Fatalf("hanya %d rutin terbaca dari script", len(dibuat))
	}
	for nama := range dibuat {
		if !dibuang[nama] {
			t.Errorf("%s dibuat script tetapi tidak dibuang saat pembersihan", nama)
		}
	}
	for nama := range dibuang {
		if !dibuat[nama] {
			t.Errorf("%s dibuang saat pembersihan padahal tidak dibuat script", nama)
		}
	}
	// pembersihan hanya bila opsi 1 DAN penyegaran terakhir sukses; tabel tidak pernah dibuang
	for _, perlu := range []string{"bersihkan = 1", "= N'sukses'", "DROP TABLE IF EXISTS #opsi"} {
		if !strings.Contains(bersih, perlu) {
			t.Errorf("batch pembersihan tidak memuat %q", perlu)
		}
	}
	for _, m := range regexp.MustCompile(`(?i)DROP TABLE [^;]*;`).FindAllString(bersih, -1) {
		if m != "DROP TABLE IF EXISTS #opsi;" {
			t.Errorf("pembersihan tidak boleh membuang tabel: %q", m)
		}
	}
}
