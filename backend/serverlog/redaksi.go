package serverlog

import (
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	maksPesan  = 2000
	maksDetail = 8000
	maksSumber = 80
	maksRute   = 200
)

// Pesan galat kerap memuat potongan alamat, kueri, atau string koneksi. Kata sandi, token, dan rahasia lain disamarkan sebelum dicetak atau disimpan.
var (
	reBearer   = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9\-._~+/]+=*`)
	reJWT      = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]*`)
	reKunciNil = regexp.MustCompile(`(?i)\b(password|passwd|pwd|secret|client_secret|token|access_token|refresh_token|id_token|api[_-]?key|apikey|authorization)\b("?\s*[=:]\s*)("[^"]*"|'[^']*'|[^\s&;,"']+)`)
	reKueri    = regexp.MustCompile(`(?i)([?&](?:code|state|key|signature)=)[^&\s"']+`)
)

const tersamar = "[disembunyikan]"

// Samarkan menyamarkan rahasia pada teks log.
func Samarkan(s string) string {
	if s == "" {
		return s
	}
	s = reBearer.ReplaceAllString(s, "Bearer "+tersamar)
	s = reJWT.ReplaceAllString(s, tersamar)
	s = reKunciNil.ReplaceAllString(s, "${1}${2}"+tersamar)
	s = reKueri.ReplaceAllString(s, "${1}"+tersamar)
	return s
}

// bersihkan membuang karakter yang tidak sah untuk disimpan dan memotong ke batas rune.
func bersihkan(s string, maks int) string {
	s = strings.ToValidUTF8(s, "?")
	s = strings.ReplaceAll(s, "\x00", "")
	if utf8.RuneCountInString(s) <= maks {
		return s
	}
	r := []rune(s)
	return string(r[:maks-1]) + "…"
}

var (
	reUUID  = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	reHex   = regexp.MustCompile(`\b[0-9a-fA-F]{16,}\b`)
	reAngka = regexp.MustCompile(`\d+`)
)

// Sidik menghitung sidik jari kejadian: sumber, rute, dan pesan yang dinormalkan (angka, UUID, dan nilai heksa panjang diseragamkan) sehingga kejadian yang sama
// dengan angka berbeda (mis. "gagal membaca baris 12" dan "baris 98") digabung menjadi satu.
func Sidik(level Level, sumber, rute, pesan string) string {
	n := reUUID.ReplaceAllString(pesan, "<id>")
	n = reHex.ReplaceAllString(n, "<hex>")
	n = reAngka.ReplaceAllString(n, "#")
	if len(n) > 300 {
		n = n[:300]
	}
	h := sha1.Sum([]byte(string(level) + "|" + sumber + "|" + rute + "|" + n))
	return hex.EncodeToString(h[:])
}

// siapkan menormalkan sebuah entri sebelum dicetak/disimpan: rahasia disamarkan, teks dipotong, level dan waktu diisi.
func siapkan(e Entri) Entri {
	e.Pesan = bersihkan(Samarkan(strings.TrimSpace(e.Pesan)), maksPesan)
	e.Detail = bersihkan(Samarkan(strings.TrimSpace(e.Detail)), maksDetail)
	e.Sumber = bersihkan(strings.TrimSpace(e.Sumber), maksSumber)
	if e.Sumber == "" {
		e.Sumber = "SERVER"
	}
	e.Rute = bersihkan(e.Rute, maksRute)
	e.Metode = bersihkan(e.Metode, 10)
	e.RequestID = bersihkan(e.RequestID, 64)
	e.UserID = bersihkan(e.UserID, 64)
	return e
}
