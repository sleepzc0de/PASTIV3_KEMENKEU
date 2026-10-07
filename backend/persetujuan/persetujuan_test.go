package persetujuan

import (
	"strings"
	"testing"
)

func TestFrasaSah(t *testing.T) {
	for _, s := range []string{"SAYA SETUJU", "saya setuju", "  Saya   Setuju  ", "SAYA\tSETUJU"} {
		if !FrasaSah(s) {
			t.Errorf("FrasaSah(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "saya", "SAYA TIDAK SETUJU", "SAYASETUJU", "SAYA SETUJU.", "setuju saya"} {
		if FrasaSah(s) {
			t.Errorf("FrasaSah(%q) = true, want false", s)
		}
	}
}

func TestProfilRapikanDanValidasi(t *testing.T) {
	p := Profil{Nama: "  Budi   Santoso ", NIP: " 1996 0910 2018 0110 05 ", Email: "  Budi@Kemenkeu.GO.ID "}.Rapikan()
	if p.Nama != "Budi Santoso" || p.NIP != "199609102018011005" || p.Email != "budi@kemenkeu.go.id" {
		t.Fatalf("rapikan = %+v", p)
	}
	if g := p.Validasi(); len(g) != 0 {
		t.Errorf("profil sah ditolak: %v", g)
	}

	cases := []struct {
		nama  string
		p     Profil
		kunci string
	}{
		{"nama pendek", Profil{Nama: "Bu", NIP: "199609102018011005", Email: "a@b.co"}, "nama"},
		{"nama kosong", Profil{Nama: "", NIP: "199609102018011005", Email: "a@b.co"}, "nama"},
		{"nama kepanjangan", Profil{Nama: strings.Repeat("a", 101), NIP: "199609102018011005", Email: "a@b.co"}, "nama"},
		{"nama berkontrol", Profil{Nama: "Budi\x00Santoso", NIP: "199609102018011005", Email: "a@b.co"}, "nama"},
		{"nip huruf", Profil{Nama: "Budi", NIP: "19960910201801100A", Email: "a@b.co"}, "nip"},
		{"nip pendek", Profil{Nama: "Budi", NIP: "1996091020", Email: "a@b.co"}, "nip"},
		{"nip kosong", Profil{Nama: "Budi", NIP: "", Email: "a@b.co"}, "nip"},
		{"email tanpa at", Profil{Nama: "Budi", NIP: "199609102018011005", Email: "budi.kemenkeu.go.id"}, "email"},
		{"email tanpa domain titik", Profil{Nama: "Budi", NIP: "199609102018011005", Email: "budi@localhost"}, "email"},
		{"email dengan nama tampilan", Profil{Nama: "Budi", NIP: "199609102018011005", Email: "Budi <budi@kemenkeu.go.id>"}, "email"},
		{"email kosong", Profil{Nama: "Budi", NIP: "199609102018011005", Email: ""}, "email"},
		{"email kepanjangan", Profil{Nama: "Budi", NIP: "199609102018011005", Email: strings.Repeat("a", 96) + "@b.co"}, "email"},
	}
	for _, c := range cases {
		g := c.p.Rapikan().Validasi()
		if _, ada := g[c.kunci]; !ada {
			t.Errorf("%s: galat %q tidak muncul: %v", c.nama, c.kunci, g)
		}
	}
	// NIP 9 digit (NIP lama) sah
	if g := (Profil{Nama: "Budi", NIP: "123456789", Email: "a@b.co"}).Validasi(); len(g) != 0 {
		t.Errorf("NIP 9 digit ditolak: %v", g)
	}
}

func TestPernyataanTidakKosong(t *testing.T) {
	if Versi == "" || Frasa == "" || Judul == "" || len(Paragraf) < 3 {
		t.Fatal("pernyataan harus punya versi, frasa, judul, dan isi")
	}
	for i, p := range Paragraf {
		if strings.TrimSpace(p) == "" {
			t.Errorf("paragraf %d kosong", i)
		}
	}
	if len(Versi) > 20 {
		t.Error("versi melebihi lebar kolom persetujuan_versi (20)")
	}
}
