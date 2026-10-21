package apiakses

import (
	"strings"
	"testing"

	"pasti-v3-backend/digitalisasi"
)

func pendaftaranSah() Pendaftaran {
	return Pendaftaran{
		NamaAplikasi: "Aplikasi Contoh", Instansi: "Direktorat Jenderal Contoh", Unit: "Biro TI", PICNama: "Budi Santoso", PICJabatan: "Kepala Subbagian",
		PICEmail: "budi.santoso@contoh.go.id", PICTelepon: "+62 21 3456789", Tujuan: "Menampilkan jumlah dan sebaran aset tanah pada dasbor internal kami.",
		DatasetDiminta: []string{"aset.tanah", "aset.satker"},
	}
}

func TestPendaftaranSah(t *testing.T) {
	p := pendaftaranSah()
	if g := p.Bersihkan(); len(g) != 0 {
		t.Fatalf("pendaftaran sah ditolak: %v", g)
	}
	// Daftar dataset diurutkan menurut katalog dan tanpa duplikat.
	if strings.Join(p.DatasetDiminta, ",") != "aset.satker,aset.tanah" {
		t.Errorf("dataset = %v", p.DatasetDiminta)
	}
}

func TestPendaftaranMenolakIsianBuruk(t *testing.T) {
	cases := []struct {
		nama  string
		ubah  func(*Pendaftaran)
		kolom string
	}{
		{"nama kosong", func(p *Pendaftaran) { p.NamaAplikasi = "  " }, "nama_aplikasi"},
		{"nama terlalu panjang", func(p *Pendaftaran) { p.NamaAplikasi = strings.Repeat("a", 151) }, "nama_aplikasi"},
		{"nama berkarakter kendali", func(p *Pendaftaran) { p.NamaAplikasi = "Apl\x00ikasi" }, "nama_aplikasi"},
		{"instansi kosong", func(p *Pendaftaran) { p.Instansi = "" }, "instansi"},
		{"pic kosong", func(p *Pendaftaran) { p.PICNama = "" }, "pic_nama"},
		{"email kosong", func(p *Pendaftaran) { p.PICEmail = "" }, "pic_email"},
		{"email salah", func(p *Pendaftaran) { p.PICEmail = "bukan-email" }, "pic_email"},
		{"email dengan baris baru (header injection)", func(p *Pendaftaran) { p.PICEmail = "a@b.go.id\nBcc: x@y.com" }, "pic_email"},
		{"telepon huruf", func(p *Pendaftaran) { p.PICTelepon = "abc-def" }, "pic_telepon"},
		{"tujuan kosong", func(p *Pendaftaran) { p.Tujuan = "" }, "tujuan"},
		{"tujuan terlalu pendek", func(p *Pendaftaran) { p.Tujuan = "untuk uji" }, "tujuan"},
		{"tujuan terlalu panjang", func(p *Pendaftaran) { p.Tujuan = strings.Repeat("x", 2001) }, "tujuan"},
		{"tujuan berkarakter kendali", func(p *Pendaftaran) { p.Tujuan = strings.Repeat("x", 30) + "\x07" }, "tujuan"},
		{"tanpa dataset", func(p *Pendaftaran) { p.DatasetDiminta = nil }, "dataset_diminta"},
		{"dataset tidak dikenal", func(p *Pendaftaran) { p.DatasetDiminta = []string{"aset.tanah", "users"} }, "dataset_diminta"},
		{"dataset berupa tabel lain", func(p *Pendaftaran) { p.DatasetDiminta = []string{"aset.DIGITALISASI_TANAH; DROP TABLE users"} }, "dataset_diminta"},
		{"terlalu banyak dataset", func(p *Pendaftaran) { p.DatasetDiminta = make([]string, 31) }, "dataset_diminta"},
	}
	for _, c := range cases {
		p := pendaftaranSah()
		c.ubah(&p)
		g := p.Bersihkan()
		if g[c.kolom] == "" {
			t.Errorf("%s: kolom %s harus bergalat, got %v", c.nama, c.kolom, g)
		}
	}
}

func TestPendaftaranGalatTidakMemuatIsianPemohon(t *testing.T) {
	p := pendaftaranSah()
	p.PICEmail = "<script>alert(1)</script>"
	g := p.Bersihkan()
	for k, v := range g {
		if strings.Contains(v, "script") || strings.Contains(v, "alert") {
			t.Errorf("pesan galat %s memantulkan isian pemohon: %q", k, v)
		}
	}
}

func TestPendaftaranMemadatkanSpasi(t *testing.T) {
	p := pendaftaranSah()
	p.NamaAplikasi = "  Aplikasi   \t Contoh  "
	p.Bersihkan()
	if p.NamaAplikasi != "Aplikasi Contoh" {
		t.Errorf("nama = %q", p.NamaAplikasi)
	}
}

func TestPerpindahanStatus(t *testing.T) {
	ok := [][2]string{
		{StatusMenunggu, StatusDisetujui}, {StatusMenunggu, StatusDitolak}, {StatusDisetujui, StatusDicabut}, {StatusDicabut, StatusDisetujui},
	}
	for _, c := range ok {
		if !BolehPindah(c[0], c[1]) {
			t.Errorf("%s -> %s harus boleh", c[0], c[1])
		}
	}
	tidak := [][2]string{
		{StatusMenunggu, StatusDicabut}, {StatusMenunggu, StatusMenunggu}, {StatusDitolak, StatusDisetujui}, {StatusDitolak, StatusMenunggu}, {StatusDisetujui, StatusDitolak},
		{StatusDisetujui, StatusMenunggu}, {StatusDicabut, StatusDitolak}, {"", StatusDisetujui}, {"sembarang", StatusDisetujui},
	}
	for _, c := range tidak {
		if BolehPindah(c[0], c[1]) {
			t.Errorf("%s -> %s tidak boleh", c[0], c[1])
		}
	}
	if StatusSah("hapus") || !StatusSah(StatusDicabut) {
		t.Error("StatusSah salah")
	}
}

// Kolom yang tidak boleh keluar: kebijakan ini harus tetap berlaku untuk SETIAP dataset, termasuk dataset yang ditambahkan nanti.
func TestKolomPribadiTidakPernahKeluar(t *testing.T) {
	if len(Katalog()) != len(digitalisasi.Datasets) || len(Katalog()) == 0 {
		t.Fatal("katalog harus memuat semua dataset Digitalisasi Aset")
	}
	for _, d := range Katalog() {
		if len(d.Kolom) == 0 {
			t.Errorf("%s tidak punya kolom keluar", d.Kode)
		}
		keluar := map[string]bool{}
		for _, k := range d.Kolom {
			keluar[strings.ToLower(k.kolom)] = true
			n := strings.ToLower(k.kolom)
			if strings.HasPrefix(n, "foto") || n == "nama_pengguna" || n == "nama_penghuni" {
				t.Errorf("%s: kolom %s tidak boleh keluar", d.Kode, k.kolom)
			}
		}
		for _, c := range d.ds.Columns {
			if c.Sensitive && keluar[strings.ToLower(c.Name)] {
				t.Errorf("%s: kolom sensitif %s keluar lewat API", d.Kode, c.Name)
			}
		}
		for _, c := range d.KolomPencarian() {
			if !keluar[strings.ToLower(c)] {
				t.Errorf("%s: pencarian memakai kolom %s yang tidak keluar", d.Kode, c)
			}
		}
	}
	rn, _ := CariDataset("aset.rumah_negara")
	for _, k := range rn.Kolom {
		if strings.EqualFold(k.kolom, "Nama_Penghuni") {
			t.Error("nama penghuni rumah negara tidak boleh keluar")
		}
	}
	gl, _ := CariDataset("aset.gedung_lainnya")
	for _, k := range gl.Kolom {
		if strings.EqualFold(k.kolom, "Nama_Pengguna") {
			t.Error("Nama_Pengguna tidak boleh keluar")
		}
	}
}

func TestKolomSensitifBaruOtomatisTertutup(t *testing.T) {
	if !kolomTertutup(digitalisasi.Column{Name: "Nomor_KTP", Sensitive: true}) {
		t.Error("kolom bertanda Sensitive harus tertutup, apa pun namanya")
	}
	if !kolomTertutup(digitalisasi.Column{Name: "Foto_bangunan"}) || !kolomTertutup(digitalisasi.Column{Name: "foto"}) {
		t.Error("kolom foto harus tertutup")
	}
	if kolomTertutup(digitalisasi.Column{Name: "Uraian_tanah"}) {
		t.Error("kolom biasa tidak boleh tertutup")
	}
}

func TestCariDatasetHanyaKodeDikenal(t *testing.T) {
	if _, ok := CariDataset("aset.tanah"); !ok {
		t.Error("aset.tanah harus dikenal")
	}
	for _, k := range []string{"", "tanah", "aset.", "aset.users", "aset.tanah; DROP TABLE x", "ASET.TANAH", "../aset.tanah", "pengadaan.tender"} {
		if _, ok := CariDataset(k); ok {
			t.Errorf("CariDataset(%q) harus gagal", k)
		}
	}
	if ds, err := NormalisasiDataset([]string{"aset.tanah", "aset.tanah", " aset.satker "}, 10); err != nil || strings.Join(ds, ",") != "aset.satker,aset.tanah" {
		t.Errorf("NormalisasiDataset = %v, %v", ds, err)
	}
	if _, err := NormalisasiDataset([]string{"aset.xyz"}, 10); err == nil {
		t.Error("kode tak dikenal harus ditolak, bukan diabaikan")
	}
}
