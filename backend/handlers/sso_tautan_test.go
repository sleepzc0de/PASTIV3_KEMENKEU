package handlers

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func TestPutusanTautan(t *testing.T) {
	sso := akunSama{ID: "a", AuthProvider: "sso"}
	sso2 := akunSama{ID: "b", AuthProvider: "sso"}
	lokal := akunSama{ID: "c", AuthProvider: "local"}

	for _, c := range []struct {
		nama string
		sama []akunSama
		mau  aksiTautan
	}{
		{"tidak ada akun dengan email itu: tambah baru", nil, tautanBuatBaru},
		{"tepat satu akun SSO: tautkan", []akunSama{sso}, tautanTautkan},
		{"penyedia SSO huruf besar tetap SSO", []akunSama{{ID: "a", AuthProvider: "SSO"}}, tautanTautkan},
		{"satu akun lokal: jangan diambil alih", []akunSama{lokal}, tautanBentrok},
		{"penyedia kosong dianggap bukan SSO", []akunSama{{ID: "a"}}, tautanBentrok},
		{"dua akun SSO: ambigu", []akunSama{sso, sso2}, tautanBentrok},
		{"akun SSO dan akun lokal sekaligus", []akunSama{sso, lokal}, tautanBentrok},
	} {
		if got := putusanTautan(c.sama); got != c.mau {
			t.Errorf("%s: putusan = %d, mau %d", c.nama, got, c.mau)
		}
	}
}

// Galat basis data mentah (nama tabel, constraint, email) tidak boleh sampai ke layar login; hanya pesan umum atau pesan bentrok yang ramah.
func TestPesanGalatSesiAkunTidakMembocorkanGalatBasisData(t *testing.T) {
	mentah := fmt.Errorf("insert user gagal: %w", errors.New("mssql: Violation of UNIQUE KEY constraint 'UQ__users__AB6E6164216A5249'. "+
		"Cannot insert duplicate key in object 'dbo.users'. The duplicate key value is (budi@kemenkeu.go.id)."))

	umum := pesanGalatSesiAkun(mentah)
	bentrok := pesanGalatSesiAkun(fmt.Errorf("x: %w", errAkunBentrok))

	for nama, pesan := range map[string]string{"umum": umum, "bentrok": bentrok} {
		if pesan == "" {
			t.Errorf("%s: pesan kosong", nama)
		}
		for _, terlarang := range []string{"mssql", "unique", "uq__", "dbo.", "constraint", "duplicate", "kemenkeu.go.id", "insert"} {
			if strings.Contains(strings.ToLower(pesan), terlarang) {
				t.Errorf("%s: pesan memuat %q: %q", nama, terlarang, pesan)
			}
		}
	}
	if !strings.Contains(umum, "Gagal membuat sesi akun") {
		t.Errorf("pesan umum = %q", umum)
	}
	if !strings.Contains(bentrok, "tidak dapat ditautkan otomatis") || !strings.Contains(bentrok, "administrator") {
		t.Errorf("pesan bentrok = %q", bentrok)
	}
	if umum == bentrok {
		t.Error("pesan umum dan pesan bentrok seharusnya berbeda")
	}
}

func TestKodeRujukan(t *testing.T) {
	pola := regexp.MustCompile(`^[0-9A-F]{8}$`)
	a, b := kodeRujukan(), kodeRujukan()
	if !pola.MatchString(a) || !pola.MatchString(b) {
		t.Errorf("format kode salah: %q %q", a, b)
	}
	if a == b {
		t.Errorf("dua kode berurutan sama: %q", a)
	}
}
