package apiakses

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDaftarIP(t *testing.T) {
	ok := []struct{ in, want string }{
		{"10.1.2.3", "10.1.2.3/32"},
		{"10.1.2.3/24", "10.1.2.0/24"},
		{"10.1.2.3, 192.168.0.0/16\n203.0.113.9", "10.1.2.3/32,192.168.0.0/16,203.0.113.9/32"},
		{"10.1.2.3 10.1.2.3", "10.1.2.3/32"},
		{"::1", "::1/128"},
		{"2001:db8::/32", "2001:db8::/32"},
		{"::ffff:10.1.2.3", "10.1.2.3/32"},
		{"", ""},
	}
	for _, c := range ok {
		got, err := DaftarIP(c.in)
		if err != nil {
			t.Errorf("DaftarIP(%q): %v", c.in, err)
			continue
		}
		if strings.Join(got, ",") != c.want {
			t.Errorf("DaftarIP(%q) = %v, want %s", c.in, got, c.want)
		}
	}
	buruk := []string{"abc", "300.1.1.1", "10.1.2.3/33", "0.0.0.0/0", "::/0", "10.0.0.1; DROP TABLE x", "10.1.2/24", "10.1.2.3/", strings.Repeat("10.0.0.1 ", 3) + strings.Repeat("a ", 3)}
	for _, s := range buruk {
		if _, err := DaftarIP(s); err == nil {
			t.Errorf("DaftarIP(%q) harus ditolak", s)
		}
	}
	var banyak []string
	for i := 0; i < 21; i++ {
		banyak = append(banyak, "10.0.0."+string(rune('0'+i%10))+"/32")
	}
	banyak = append(banyak, "10.9.9.9", "10.9.9.8", "10.9.9.7", "10.9.9.6", "10.9.9.5", "10.9.9.4", "10.9.9.3", "10.9.9.2", "10.9.9.1", "10.9.9.0", "10.9.8.9", "10.9.8.8", "10.9.8.7", "10.9.8.6", "10.9.8.5", "10.9.8.4", "10.9.8.3", "10.9.8.2", "10.9.8.1", "10.9.8.0", "10.9.7.1")
	if _, err := DaftarIP(strings.Join(banyak, ",")); err == nil {
		t.Error("lebih dari 20 entri harus ditolak")
	}
}

func TestIPDiizinkan(t *testing.T) {
	daftar, _ := DaftarIP("10.1.2.0/24, 203.0.113.9, 2001:db8::/32")
	cases := []struct {
		ip   string
		want bool
	}{
		{"10.1.2.77", true}, {"10.1.3.1", false}, {"203.0.113.9", true}, {"203.0.113.10", false}, {"2001:db8::1", true}, {"2001:db9::1", false},
		{"::ffff:10.1.2.5", true}, {"::ffff:10.1.3.5", false}, {"", false}, {"bukan-ip", false}, {" 10.1.2.1 ", true},
	}
	for _, c := range cases {
		if got := IPDiizinkan(daftar, c.ip); got != c.want {
			t.Errorf("IPDiizinkan(%q) = %v, want %v", c.ip, got, c.want)
		}
	}
	for _, ip := range []string{"1.2.3.4", "", "bukan-ip"} {
		if !IPDiizinkan(nil, ip) {
			t.Errorf("daftar kosong = tanpa pembatasan, IP %q harus lolos", ip)
		}
	}
}

func TestDaftarUE1(t *testing.T) {
	got, err := DaftarUE1("01504, 01505\n01504")
	if err != nil || strings.Join(got, ",") != "01504,01505" {
		t.Errorf("DaftarUE1 = %v, %v", got, err)
	}
	if got, err := DaftarUE1(""); err != nil || len(got) != 0 {
		t.Errorf("kosong = tanpa pembatasan: %v, %v", got, err)
	}
	for _, s := range []string{"1504", "015041", "0150a", "01504' OR '1'='1", "-1504"} {
		if _, err := DaftarUE1(s); err == nil {
			t.Errorf("DaftarUE1(%q) harus ditolak", s)
		}
	}
}

func TestPengaturanBawaanSesuaiPermintaan(t *testing.T) {
	p := PengaturanBawaan()
	if p.LimitPermintaan != 1000 || p.LimitJendelaDetik != 60 || p.KuotaPermintaan != 5000 || p.KuotaPeriodeDetik != 3600 {
		t.Errorf("bawaan = %+v, want 1000 permintaan / 60 detik dan kuota 5000 / 3600 detik", p)
	}
	if err := p.Validasi(); err != nil {
		t.Errorf("bawaan harus sah: %v", err)
	}
}

func TestPengaturanValidasi(t *testing.T) {
	ubah := func(f func(*Pengaturan)) Pengaturan {
		p := PengaturanBawaan()
		f(&p)
		return p
	}
	buruk := []Pengaturan{
		ubah(func(p *Pengaturan) { p.LimitPermintaan = 0 }),
		ubah(func(p *Pengaturan) { p.LimitPermintaan = LimitMaks + 1 }),
		ubah(func(p *Pengaturan) { p.LimitJendelaDetik = 0 }),
		ubah(func(p *Pengaturan) { p.LimitJendelaDetik = JendelaMaks + 1 }),
		ubah(func(p *Pengaturan) { p.KuotaPermintaan = -1 }),
		ubah(func(p *Pengaturan) { p.KuotaPeriodeDetik = 59 }),
		ubah(func(p *Pengaturan) { p.KuotaPeriodeDetik = PeriodeMaks + 1 }),
		ubah(func(p *Pengaturan) { p.HalamanMaks = 5 }),
		ubah(func(p *Pengaturan) { p.HalamanMaks = 100000 }),
		ubah(func(p *Pengaturan) { p.KunciBerlakuHari = -1 }),
		ubah(func(p *Pengaturan) { p.RotasiTenggangJam = TenggangMaksJam + 1 }),
		ubah(func(p *Pengaturan) { p.LogRetensiHari = 0 }),
		ubah(func(p *Pengaturan) { p.AgregatRetensiHari = 0 }),
	}
	for i, p := range buruk {
		if err := p.Validasi(); err == nil {
			t.Errorf("kasus %d harus ditolak: %+v", i, p)
		}
	}
	sah := ubah(func(p *Pengaturan) {
		p.LimitPermintaan, p.LimitJendelaDetik, p.KuotaPermintaan, p.KuotaPeriodeDetik, p.KunciBerlakuHari, p.RotasiTenggangJam = LimitMaks, JendelaMaks, KuotaMaks, PeriodeMaks, 0, 0
	})
	if err := sah.Validasi(); err != nil {
		t.Errorf("batas atas harus sah: %v", err)
	}
}

func TestPengaturanBolakBalikBaris(t *testing.T) {
	p := PengaturanBawaan()
	p.LimitPermintaan, p.KuotaPermintaan, p.KunciBerlakuHari = 250, 9000, 90
	if got := DariBaris(p.KeBaris()); got != p {
		t.Errorf("bolak-balik: %+v != %+v", got, p)
	}
	// Baris rusak/hilang -> nilai pabrik, tidak panik.
	if got := DariBaris(map[string]string{"limit_permintaan": "banyak"}); got != PengaturanBawaan() {
		t.Errorf("nilai tidak terbaca harus memakai pabrik: %+v", got)
	}
	if got := DariBaris(nil); got != PengaturanBawaan() {
		t.Errorf("tabel kosong harus memakai pabrik: %+v", got)
	}
	if got := DariBaris(map[string]string{"limit_permintaan": "0", "kuota_permintaan": "7"}); got != PengaturanBawaan() {
		t.Errorf("hasil gabungan yang tidak sah kembali ke pabrik seluruhnya: %+v", got)
	}
}

// Pendaftaran publik ditiadakan: tidak ada pengaturan "pendaftaran_dibuka" lagi; barisnya (bila masih ada di database lama) diabaikan dan tidak pernah ditulis.
func TestPengaturanTidakMemuatPendaftaranPublik(t *testing.T) {
	if _, ada := PengaturanBawaan().KeBaris()["pendaftaran_dibuka"]; ada {
		t.Error("pengaturan pendaftaran_dibuka tidak boleh ditulis lagi")
	}
	if got := DariBaris(map[string]string{"pendaftaran_dibuka": "0", "limit_permintaan": "321"}); got.LimitPermintaan != 321 {
		t.Errorf("baris lama harus diabaikan tanpa mengganggu yang lain: %+v", got)
	}
	b, _ := json.Marshal(PengaturanBawaan())
	if strings.Contains(string(b), "pendaftaran") {
		t.Errorf("JSON pengaturan memuat pendaftaran: %s", b)
	}
}

func TestOverrideGalatKolom(t *testing.T) {
	n := func(v int) *int { return &v }
	g := (Override{LimitPermintaan: n(0), LimitJendelaDetik: n(99999), KuotaPermintaan: n(5), KuotaPeriodeDetik: n(10)}).GalatKolom()
	for _, k := range []string{"limit", "jendela", "periode"} {
		if g[k] == "" {
			t.Errorf("kolom %s harus bergalat: %v", k, g)
		}
	}
	if g["kuota"] != "" {
		t.Errorf("kuota 5 sah: %v", g)
	}
	if g := (Override{}).GalatKolom(); len(g) != 0 {
		t.Errorf("tanpa override tidak ada galat: %v", g)
	}
}

func TestOverrideBatasPerKlien(t *testing.T) {
	g := PengaturanBawaan()
	g.LimitPermintaan, g.KuotaPermintaan = 100, 200
	kosong := g.BatasUntuk(Override{})
	if kosong.LimitPermintaan != 100 || kosong.LimitJendelaDetik != 60 || kosong.KuotaPermintaan != 200 || kosong.KuotaPeriodeDetik != 3600 {
		t.Errorf("tanpa override ikut global: %+v", kosong)
	}
	n := func(v int) *int { return &v }
	b := g.BatasUntuk(Override{LimitPermintaan: n(5000), KuotaPeriodeDetik: n(86400)})
	if b.LimitPermintaan != 5000 || b.LimitJendelaDetik != 60 || b.KuotaPermintaan != 200 || b.KuotaPeriodeDetik != 86400 {
		t.Errorf("override sebagian: %+v", b)
	}
	if bt := b.KeBatas(); bt.LimitJendela.Seconds() != 60 || bt.KuotaPeriode.Hours() != 24 || bt.Kuota != 200 {
		t.Errorf("KeBatas = %+v", bt)
	}
	if err := (Override{LimitPermintaan: n(0)}).Validasi(); err == nil {
		t.Error("override 0 harus ditolak")
	}
	if err := (Override{KuotaPeriodeDetik: n(10)}).Validasi(); err == nil {
		t.Error("periode 10 detik harus ditolak")
	}
	if err := (Override{LimitPermintaan: n(1), KuotaPermintaan: n(KuotaMaks)}).Validasi(); err != nil {
		t.Errorf("override sah ditolak: %v", err)
	}
}
