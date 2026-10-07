package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/peran"
)

func konteksPeran(role, peranAktif string, cak peran.Cakupan) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("role", role)
	c.Set(peran.KunciGinPeran, peranAktif)
	c.Set(peran.KunciGinCakupan, cak)
	return c
}

func TestKondisiPengguna(t *testing.T) {
	const tanpaSuperadmin = "u.role <> N'superadmin' AND u.is_protected = 0"
	cases := []struct {
		nama   string
		c      *gin.Context
		want   string
		harus  []string // potongan yang harus ada
		larang []string // potongan yang tidak boleh ada
	}{
		{"superadmin melihat semua", konteksPeran("superadmin", "superadmin", peran.CakupanSemua), "1 = 1", nil, nil},
		{"pengguna barang: semua kecuali superadmin", konteksPeran("user", peran.PenggunaBarang, peran.CakupanSemua), tanpaSuperadmin, nil, []string{"kode_satker"}},
		{"ue1: prefix 5 digit", konteksPeran("user", peran.UE1, peran.Cakupan{Tingkat: peran.TingkatUE1, Kode: "01504"}), "", []string{tanpaSuperadmin, "LEFT(e.kode_satker, 5) = N'01504'"}, nil},
		{"kanwil: prefix 9 digit", konteksPeran("user", peran.Kanwil, peran.Cakupan{Tingkat: peran.TingkatKwl, Kode: "015040199"}), "", []string{tanpaSuperadmin, "LEFT(e.kode_satker, 9) = N'015040199'"}, nil},
		{"satker: karakter 10-15", konteksPeran("user", peran.Satker, peran.Cakupan{Tingkat: peran.TingkatSatk, Kode: "119091"}), "", []string{tanpaSuperadmin, "SUBSTRING(e.kode_satker, 10, 6) = N'119091'"}, nil},
		{"tamu tidak melihat siapa pun", konteksPeran("user", "", peran.Cakupan{Tingkat: peran.Kosong}), "1 = 0", nil, nil},
		{"role admin lama tanpa peran", konteksPeran("admin", "", peran.Cakupan{Tingkat: peran.Kosong}), "1 = 0", nil, nil},
		// kode cakupan yang rusak menutup, tidak membuka: kondisi cakupan menjadi 1 = 0
		{"ue1 dengan kode rusak", konteksPeran("user", peran.UE1, peran.Cakupan{Tingkat: peran.TingkatUE1, Kode: "0150' OR 1=1--"}), "", []string{tanpaSuperadmin, "1 = 0"}, []string{"OR 1=1"}},
		// superadmin yang bertindak sebagai satker memakai cakupan satker, bukan semua
		{"superadmin sebagai satker", konteksPeran("user", peran.Satker, peran.Cakupan{Tingkat: peran.TingkatSatk, Kode: "119091"}), "", []string{tanpaSuperadmin, "N'119091'"}, nil},
	}
	for _, c := range cases {
		got := kondisiPengguna(c.c)
		if c.want != "" && got != c.want {
			t.Errorf("%s: %q, want %q", c.nama, got, c.want)
		}
		for _, h := range c.harus {
			if !strings.Contains(got, h) {
				t.Errorf("%s: %q tidak memuat %q", c.nama, got, h)
			}
		}
		for _, l := range c.larang {
			if strings.Contains(got, l) {
				t.Errorf("%s: %q tidak boleh memuat %q", c.nama, got, l)
			}
		}
	}
}

func TestKeputusanSuperadmin(t *testing.T) {
	lama := config.Cfg
	t.Cleanup(func() { config.Cfg = lama })
	config.Cfg = &config.Config{SuperadminProtectedEmail: "bos@kemenkeu.go.id", SuperadminProtectedNIP: "199609102018011005"}

	cases := []struct {
		nama                string
		provider, role      string
		protected, aktif    bool
		email, nip          string
		wantNaik, wantTurun bool
	}{
		{"SSO cocok .env, masih user: dinaikkan", "sso", "user", false, true, "bos@kemenkeu.go.id", "", true, false},
		{"SSO cocok lewat NIP: dinaikkan", "sso", "user", false, true, "lain@x.id", "199609102018011005", true, false},
		{"SSO cocok, sudah superadmin tetapi nonaktif: diaktifkan", "sso", "superadmin", true, false, "bos@kemenkeu.go.id", "", true, false},
		{"SSO cocok, sudah benar: tidak berubah", "sso", "superadmin", true, true, "bos@kemenkeu.go.id", "", false, false},
		{"SSO tidak ada di .env tetapi superadmin: diturunkan", "sso", "superadmin", true, true, "mantan@kemenkeu.go.id", "123", false, true},
		{"akun protected tetapi bukan superadmin lagi: diturunkan", "sso", "user", true, true, "mantan@kemenkeu.go.id", "", false, true},
		{"akun non-SSO dengan email superadmin: tidak pernah dinaikkan", "local", "user", false, true, "bos@kemenkeu.go.id", "199609102018011005", false, false},
		{"akun non-SSO yang berrole superadmin: diturunkan", "local", "superadmin", true, true, "bos@kemenkeu.go.id", "", false, true},
		{"pengguna biasa: tidak berubah", "sso", "user", false, true, "biasa@kemenkeu.go.id", "111", false, false},
	}
	for _, c := range cases {
		naik, turun := keputusanSuperadmin(c.provider, c.role, c.protected, c.aktif, c.email, c.nip)
		if naik != c.wantNaik || turun != c.wantTurun {
			t.Errorf("%s: naik=%v turun=%v, want naik=%v turun=%v", c.nama, naik, turun, c.wantNaik, c.wantTurun)
		}
	}
}
