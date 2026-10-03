package sapa

import (
	"strings"
	"testing"
)

func TestDefinisiTahapKonsisten(t *testing.T) {
	seen := map[string]bool{}
	for _, tp := range TahapPenjualan {
		if seen[tp.Kunci] {
			t.Errorf("kunci tahap %q ganda", tp.Kunci)
		}
		seen[tp.Kunci] = true
		if !dalam(tp.Peran, PeranValid) {
			t.Errorf("tahap %s: peran %q tidak valid", tp.Kunci, tp.Peran)
		}
		switch tp.Jenis {
		case JenisForm:
			if len(tp.Dokumen) == 0 {
				t.Errorf("tahap form %s tidak menghasilkan dokumen", tp.Kunci)
			}
		case JenisEksternal:
			if tp.Kanal == "" {
				t.Errorf("tahap eksternal %s tanpa kanal (Nadine/SIMAN)", tp.Kunci)
			}
			if len(tp.Dokumen) != 0 {
				t.Errorf("tahap eksternal %s tidak boleh menghasilkan dokumen", tp.Kunci)
			}
		default:
			t.Errorf("tahap %s: jenis %q tidak dikenal", tp.Kunci, tp.Jenis)
		}
		if tp.BolehDilewati && tp.Jenis != JenisForm {
			t.Errorf("hanya tahap form yang boleh dilewati: %s", tp.Kunci)
		}
	}
	if len(TahapPenjualan) != 10 {
		t.Errorf("alur Penjualan harus 10 tahap, ada %d", len(TahapPenjualan))
	}
	// Urutan menurut alur kerja.
	want := []string{"tim", "ba", "nd_satker", "nadine_satker", "siman_satker", "siman_kanwil", "ue1_terima", "nd_ue1", "nadine_ue1", "siman_ue1"}
	for i, k := range want {
		if TahapPenjualan[i].Kunci != k {
			t.Errorf("tahap ke-%d = %s, want %s", i+1, TahapPenjualan[i].Kunci, k)
		}
	}
}

func TestUrutanTahap(t *testing.T) {
	s := StatusTahap{}
	if got := TahapSaatIni(s); got != TahapTim {
		t.Fatalf("tahap pertama = %q", got)
	}
	if err := BolehDikerjakan(s, TahapNDSatker); err == nil || !strings.Contains(err.Error(), "Pembentukan Tim") {
		t.Errorf("melompati tahap harus ditolak dan menyebut tahap yang harus selesai: %v", err)
	}
	if err := BolehDikerjakan(s, TahapTim); err != nil {
		t.Errorf("tahap pertama selalu boleh: %v", err)
	}

	s[TahapTim] = StatusDilewati
	s[TahapBA] = StatusSelesai
	if got := TahapSaatIni(s); got != TahapNDSatker {
		t.Errorf("tahap saat ini = %q", got)
	}
	if err := BolehDikerjakan(s, TahapNDSatker); err != nil {
		t.Errorf("setelah tim dilewati dan BA selesai, ND boleh: %v", err)
	}
	s[TahapNDSatker] = StatusDraft // draft belum dihitung selesai
	if err := BolehDikerjakan(s, TahapNadineSatker); err == nil {
		t.Error("draft tidak boleh dianggap selesai")
	}

	for _, tp := range TahapPenjualan {
		s[tp.Kunci] = StatusSelesai
	}
	if !Selesai(s) || TahapSaatIni(s) != "" {
		t.Error("semua tahap selesai harus menandai usulan selesai")
	}
	if _, _, ok := TahapByKunci("tidak_ada"); ok {
		t.Error("tahap tak dikenal")
	}
	if err := BolehDikerjakan(s, "tidak_ada"); err == nil {
		t.Error("tahap tak dikenal harus galat")
	}
}

func TestBolehDiubahHanyaBilaTahapBerikutnyaBelumSelesai(t *testing.T) {
	s := StatusTahap{TahapTim: StatusSelesai, TahapBA: StatusSelesai, TahapNDSatker: StatusSelesai}
	if err := BolehDiubah(s, TahapNDSatker); err != nil {
		t.Errorf("tahap terakhir yang selesai boleh diubah: %v", err)
	}
	if err := BolehDiubah(s, TahapBA); err == nil || !strings.Contains(err.Error(), "Nota Dinas Usulan Penjualan Satker") {
		t.Errorf("BA tidak boleh diubah setelah ND selesai: %v", err)
	}
	s[TahapNadineSatker] = StatusSelesai
	if err := BolehDiubah(s, TahapNDSatker); err == nil {
		t.Error("ND tidak boleh diubah setelah Nadine selesai")
	}
	// Tahap berikutnya yang hanya draft tidak menghalangi.
	s2 := StatusTahap{TahapTim: StatusSelesai, TahapBA: StatusDraft}
	if err := BolehDiubah(s2, TahapTim); err != nil {
		t.Errorf("tahap berikutnya masih draft: %v", err)
	}
}

func TestKodeSatker(t *testing.T) {
	if got := Kode18("015010199409294002KP"); got != "015010199409294002" {
		t.Errorf("Kode18 = %q", got)
	}
	if got := Kode18(" 015010199409294002 "); got != "015010199409294002" {
		t.Errorf("Kode18 = %q", got)
	}
	if got := KodeUE1Dari("015010199409294002"); got != "01501" {
		t.Errorf("KodeUE1Dari = %q", got)
	}
	if KodeUE1Dari("01") != "" {
		t.Error("kode terlalu pendek harus menghasilkan kosong")
	}
}

func TestHakAkses(t *testing.T) {
	kasus := Kasus{KodeSatker: "015010199409294002", KodeUE1: "01501"}
	lain := Kasus{KodeSatker: "015040199119091000", KodeUE1: "01504"}
	admin := Identitas{Admin: true}
	satker := Identitas{Peran: PeranSatker, KodeSatker: "015010199409294002KP"}
	ue1 := Identitas{Peran: PeranUE1, KodeUE1: "01501"}
	kanwil := Identitas{Peran: PeranKanwil}
	tanpaPeran := Identitas{}

	tahap := func(k string) Tahap { tp, _, _ := TahapByKunci(k); return tp }

	// Terlihat
	cases := []struct {
		nama string
		id   Identitas
		k    Kasus
		want bool
	}{
		{"admin melihat semua", admin, lain, true},
		{"satker melihat usulan satkernya (kode ber-KP)", satker, kasus, true},
		{"satker tidak melihat satker lain", satker, lain, false},
		{"UE1 melihat usulan di bawahnya", ue1, kasus, true},
		{"UE1 tidak melihat UE1 lain", ue1, lain, false},
		{"kanwil melihat semua (v1)", kanwil, lain, true},
		{"tanpa peran tidak melihat apa pun", tanpaPeran, kasus, false},
		{"satker tanpa kode satker tidak melihat", Identitas{Peran: PeranSatker}, kasus, false},
		{"UE1 tanpa kode tidak melihat", Identitas{Peran: PeranUE1}, kasus, false},
		{"peran asing tidak melihat", Identitas{Peran: "tamu"}, kasus, false},
	}
	for _, c := range cases {
		if got := Terlihat(c.id, c.k); got != c.want {
			t.Errorf("Terlihat/%s = %v, want %v", c.nama, got, c.want)
		}
	}

	// BolehBertindak: perannya harus sama dengan peran tahap.
	acts := []struct {
		nama  string
		id    Identitas
		tahap string
		want  bool
	}{
		{"satker mengerjakan ND satker", satker, TahapNDSatker, true},
		{"satker tidak boleh tahap kanwil", satker, TahapSimanKanwil, false},
		{"satker tidak boleh tahap UE1", satker, TahapNDUE1, false},
		{"UE1 mengerjakan ND UE1", ue1, TahapNDUE1, true},
		{"UE1 tidak boleh tahap satker", ue1, TahapNDSatker, false},
		{"kanwil meneliti tiket", kanwil, TahapSimanKanwil, true},
		{"admin boleh semua", admin, TahapNDUE1, true},
		{"tanpa peran tidak boleh", tanpaPeran, TahapTim, false},
	}
	for _, c := range acts {
		if got := BolehBertindak(c.id, kasus, tahap(c.tahap)); got != c.want {
			t.Errorf("BolehBertindak/%s = %v, want %v", c.nama, got, c.want)
		}
	}
	if BolehBertindak(satker, lain, tahap(TahapNDSatker)) {
		t.Error("satker tidak boleh bertindak pada usulan satker lain")
	}

	// BolehMembuat
	if err := BolehMembuat(satker, "015010199409294002"); err != nil {
		t.Errorf("satker membuat untuk satkernya: %v", err)
	}
	if err := BolehMembuat(satker, "015040199119091000"); err == nil {
		t.Error("satker tidak boleh membuat untuk satker lain")
	}
	if err := BolehMembuat(ue1, "015010199409294002"); err == nil {
		t.Error("UE1 tidak boleh membuat usulan")
	}
	if err := BolehMembuat(kanwil, "015010199409294002"); err == nil {
		t.Error("kanwil tidak boleh membuat usulan")
	}
	if err := BolehMembuat(admin, "015040199119091000"); err != nil {
		t.Errorf("admin boleh: %v", err)
	}
	if err := BolehMembuat(tanpaPeran, "015010199409294002"); err != ErrTanpaPeran {
		t.Errorf("tanpa peran: %v", err)
	}
	if err := Akses(tanpaPeran); err != ErrTanpaPeran {
		t.Errorf("Akses tanpa peran: %v", err)
	}
	if err := Akses(admin); err != nil {
		t.Errorf("Akses admin: %v", err)
	}
}
