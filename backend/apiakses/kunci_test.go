package apiakses

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

func TestBuatKunciBentukDanKeunikan(t *testing.T) {
	lihat := map[string]bool{}
	lihatPrefix := map[string]bool{}
	for i := 0; i < 500; i++ {
		k, prefix, err := BuatKunci()
		if err != nil {
			t.Fatal(err)
		}
		if len(k) != panjangKunci || !strings.HasPrefix(k, "pasti_"+prefix+"_") {
			t.Fatalf("bentuk kunci salah: %q (prefix %q)", k, prefix)
		}
		if p, ok := PrefixKunci(k); !ok || p != prefix {
			t.Fatalf("PrefixKunci(%q) = %q, %v", k, p, ok)
		}
		if lihat[k] {
			t.Fatal("kunci kembar")
		}
		lihat[k] = true
		lihatPrefix[prefix] = true
	}
	if len(lihatPrefix) < 495 { // 48 bit acak: tabrakan dalam 500 kunci praktis mustahil
		t.Errorf("prefix terlalu sering kembar: %d unik dari 500", len(lihatPrefix))
	}
}

func TestPrefixKunciMenolakBentukSalah(t *testing.T) {
	k, _, _ := BuatKunci()
	salah := []string{
		"", "pasti_", k[:len(k)-1], k + "x", strings.ToUpper(k), "PASTI_" + k[6:], "pasti_" + strings.Repeat("a", 10) + "_" + strings.Repeat("A", 42),
		"pasti_" + strings.Repeat("a", 9) + "_" + strings.Repeat("A", 43), "pasti_aaaaaaaaa1_" + strings.Repeat("A", 43), // 1 bukan huruf base32
		k[:16] + " " + k[17:], k + "\n", "' OR 1=1 --", "pasti_" + strings.Repeat("a", 10) + "_" + strings.Repeat("A", 42) + "!",
	}
	for _, s := range salah {
		if _, ok := PrefixKunci(s); ok {
			t.Errorf("PrefixKunci(%q) harus ditolak", s)
		}
	}
}

func TestHashKunci(t *testing.T) {
	h1, h2 := KunciHash("rahasia-satu"), KunciHash("rahasia-dua")
	k, _, _ := BuatKunci()
	a := HashKunci(h1, k)
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(a) {
		t.Fatalf("hash bukan 64 heks: %q", a)
	}
	if a != HashKunci(h1, k) {
		t.Error("hash harus deterministik")
	}
	if a == HashKunci(h2, k) {
		t.Error("rahasia server berbeda harus menghasilkan hash berbeda (HMAC bukan hash polos)")
	}
	k2, _, _ := BuatKunci()
	if a == HashKunci(h1, k2) {
		t.Error("kunci berbeda harus menghasilkan hash berbeda")
	}
	if strings.Contains(a, k[len(k)-20:]) {
		t.Error("hash tidak boleh memuat bagian kunci")
	}
	if string(KunciHash("x")) == "x" || len(KunciHash("x")) != 32 {
		t.Error("kunci hash harus diturunkan, bukan rahasia mentah")
	}
}

func TestSamaHash(t *testing.T) {
	h := KunciHash("p")
	k, _, _ := BuatKunci()
	a := HashKunci(h, k)
	if !SamaHash(a, a) {
		t.Error("hash sama harus cocok")
	}
	b := []byte(a)
	b[len(b)-1] ^= 1
	if SamaHash(a, string(b)) {
		t.Error("beda satu bit harus ditolak")
	}
	if SamaHash(a, a[:63]) || SamaHash(a, "") || SamaHash("", a) {
		t.Error("panjang berbeda harus ditolak")
	}
}

func TestKunciDariHeader(t *testing.T) {
	k, _, _ := BuatKunci()
	k2, _, _ := BuatKunci()
	cases := []struct {
		nama   string
		header map[string]string
		want   string
		ok     bool
	}{
		{"bearer", map[string]string{"Authorization": "Bearer " + k}, k, true},
		{"bearer huruf kecil", map[string]string{"Authorization": "bearer " + k}, k, true},
		{"x-api-key", map[string]string{"X-API-Key": k}, k, true},
		{"keduanya sama", map[string]string{"Authorization": "Bearer " + k, "X-API-Key": k}, k, true},
		{"keduanya berbeda ditolak", map[string]string{"Authorization": "Bearer " + k, "X-API-Key": k2}, "", false},
		{"skema lain ditolak", map[string]string{"Authorization": "Basic " + k}, "", false},
		{"tanpa skema ditolak", map[string]string{"Authorization": k}, "", false},
		{"kosong", map[string]string{}, "", false},
		{"bearer kosong", map[string]string{"Authorization": "Bearer "}, "", false},
	}
	for _, c := range cases {
		h := http.Header{}
		for a, b := range c.header {
			h.Set(a, b)
		}
		got, ok := KunciDariHeader(h)
		if ok != c.ok || got != c.want {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", c.nama, got, ok, c.want, c.ok)
		}
	}
}

func TestBuatKodeKlien(t *testing.T) {
	re := regexp.MustCompile(`^KL-[A-Z2-7]{8}$`)
	for i := 0; i < 50; i++ {
		k, err := BuatKodeKlien()
		if err != nil || !re.MatchString(k) {
			t.Fatalf("kode klien %q (%v)", k, err)
		}
	}
}
