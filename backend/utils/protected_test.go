package utils

import (
	"testing"

	"pasti-v3-backend/config"
)

func TestIsProtectedIdentityDanDaftarNilai(t *testing.T) {
	lama := config.Cfg
	t.Cleanup(func() { config.Cfg = lama })

	config.Cfg = &config.Config{SuperadminProtectedEmail: " Budi@Kemenkeu.go.id ; ani@kemenkeu.go.id,", SuperadminProtectedNIP: "199609102018011005 198001012005011001"}
	cases := []struct {
		nama, email, nip string
		want             bool
	}{
		{"email pertama (huruf besar/kecil tidak dibedakan)", "budi@kemenkeu.GO.id", "", true},
		{"email kedua", "ani@kemenkeu.go.id", "", true},
		{"NIP pertama", "", "199609102018011005", true},
		{"NIP kedua", "tidak@dikenal.id", " 198001012005011001 ", true},
		{"tidak terdaftar", "orang@kemenkeu.go.id", "111111111111111111", false},
		{"kosong", "", "", false},
		{"sebagian email tidak cocok", "budi@kemenkeu.go.id.palsu", "", false},
		{"awalan NIP tidak cocok", "", "1996091020180110", false},
	}
	for _, c := range cases {
		if got := IsProtectedIdentity(c.email, c.nip); got != c.want {
			t.Errorf("%s: IsProtectedIdentity(%q, %q) = %v, want %v", c.nama, c.email, c.nip, got, c.want)
		}
	}
	if !SuperadminTerkonfigurasi() {
		t.Error("SuperadminTerkonfigurasi harus true bila ada entri")
	}

	// tanpa konfigurasi: tidak ada superadmin, email/NIP kosong tidak pernah cocok dengan entri kosong
	config.Cfg = &config.Config{}
	if IsProtectedIdentity("", "") || IsProtectedIdentity("a@b.co", "123") || SuperadminTerkonfigurasi() {
		t.Error("tanpa konfigurasi tidak boleh ada superadmin")
	}
	config.Cfg = &config.Config{SuperadminProtectedEmail: " , ; "}
	if SuperadminTerkonfigurasi() || IsProtectedIdentity("", "") {
		t.Error("entri kosong tidak boleh dihitung sebagai konfigurasi")
	}
	config.Cfg = nil
	if IsProtectedIdentity("a@b.co", "1") || SuperadminTerkonfigurasi() {
		t.Error("tanpa config tidak boleh ada superadmin")
	}
}
