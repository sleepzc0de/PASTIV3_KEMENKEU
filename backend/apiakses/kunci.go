package apiakses

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"strings"
)

// Bentuk kunci API: "pasti_" + prefix 10 karakter (pengenal, bukan rahasia) + "_" + rahasia 43 karakter (256 bit acak, base64url).
//
//	pasti_k3x9p2m7qa_u8Qm3Zr0...(43 karakter)
//
// Yang disimpan hanya prefix (untuk mencari barisnya) dan HMAC-SHA256 dari SELURUH kunci. Rahasia 256 bit sudah tak mungkin ditebak, jadi hash cepat (bukan bcrypt)
// cukup dan tidak membebani setiap permintaan; HMAC dengan kunci rahasia server (pepper) membuat hash yang bocor dari database tidak dapat dipakai menguji tebakan
// tanpa pepper itu. Dibandingkan dengan waktu konstan.

const (
	awalanKunci    = "pasti_"
	panjangPrefix  = 10
	panjangRahasia = 43
	panjangKunci   = len(awalanKunci) + panjangPrefix + 1 + panjangRahasia
)

var reKunci = regexp.MustCompile(`^pasti_([a-z2-7]{10})_([A-Za-z0-9_-]{43})$`)

// BuatKunci menghasilkan kunci baru: kunci lengkap (ditampilkan sekali), dan prefix-nya.
func BuatKunci() (kunci, prefix string, err error) {
	pb := make([]byte, 6) // 48 bit -> 10 karakter base32
	rb := make([]byte, 32)
	if _, err = rand.Read(pb); err != nil {
		return "", "", err
	}
	if _, err = rand.Read(rb); err != nil {
		return "", "", err
	}
	prefix = strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(pb))
	kunci = awalanKunci + prefix + "_" + base64.RawURLEncoding.EncodeToString(rb)
	if len(kunci) != panjangKunci || !reKunci.MatchString(kunci) {
		return "", "", errors.New("bentuk kunci yang dihasilkan tidak sesuai")
	}
	return kunci, prefix, nil
}

// BuatKodeKlien menghasilkan kode klien yang tampil ("KL-" + 8 karakter acak A-Z/2-7). Bukan rahasia dan bukan kredensial.
func BuatKodeKlien() (string, error) {
	b := make([]byte, 5) // 40 bit -> 8 karakter base32
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "KL-" + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// PrefixKunci memeriksa bentuk kunci yang diterima dan mengembalikan prefix-nya. Bentuk yang salah ditolak sebelum menyentuh database.
func PrefixKunci(kunci string) (string, bool) {
	if len(kunci) != panjangKunci {
		return "", false
	}
	m := reKunci.FindStringSubmatch(kunci)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// KunciHash: kunci rahasia server untuk HMAC hash kunci API. Diturunkan dari rahasia yang sudah ada (pepper) dengan pemisah domain, jadi tidak butuh pengaturan baru dan
// tidak sama dengan nilai yang dipakai untuk kata sandi.
func KunciHash(pepper string) []byte {
	m := hmac.New(sha256.New, []byte(pepper))
	m.Write([]byte("pasti-api-akses-kunci-v1"))
	return m.Sum(nil)
}

// HashKunci menghitung hash (hex, 64 karakter) dari kunci lengkap.
func HashKunci(kunciHash []byte, kunci string) string {
	m := hmac.New(sha256.New, kunciHash)
	m.Write([]byte(kunci))
	return hex.EncodeToString(m.Sum(nil))
}

// SamaHash membandingkan dua hash dengan waktu konstan (tidak membocorkan seberapa mirip lewat waktu). Panjang berbeda langsung dianggap beda.
func SamaHash(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// KunciDariHeader membaca kunci dari "Authorization: Bearer <kunci>" atau "X-API-Key: <kunci>". Kunci TIDAK dibaca dari alamat (query string): alamat tercatat di log
// proxy dan riwayat peramban. Bila kedua header ada, keduanya harus sama (menghindari ambiguitas yang bisa dimanfaatkan melewati pemeriksaan di proxy).
func KunciDariHeader(h http.Header) (string, bool) {
	var dari []string
	if a := strings.TrimSpace(h.Get("Authorization")); a != "" {
		skema, nilai, ok := strings.Cut(a, " ")
		if !ok || !strings.EqualFold(skema, "Bearer") {
			return "", false
		}
		dari = append(dari, strings.TrimSpace(nilai))
	}
	if x := strings.TrimSpace(h.Get("X-API-Key")); x != "" {
		dari = append(dari, x)
	}
	if len(dari) == 0 || len(dari) > 2 {
		return "", false
	}
	if len(dari) == 2 && dari[0] != dari[1] {
		return "", false
	}
	return dari[0], dari[0] != ""
}
