package analitik

import (
	"strings"
	"testing"
	"time"
)

func tgl(th, bln, hari int) time.Time {
	return time.Date(th, time.Month(bln), hari, 9, 0, 0, 0, time.UTC)
}

func cari(w []Wawasan, potongan string) *Wawasan {
	for i := range w {
		if strings.Contains(w[i].Judul, potongan) {
			return &w[i]
		}
	}
	return nil
}

func wajibAda(t *testing.T, w []Wawasan, potongan, tingkat, bagian string) *Wawasan {
	t.Helper()
	x := cari(w, potongan)
	if x == nil {
		t.Fatalf("wawasan %q tidak ada; ada: %v", potongan, judul(w))
	}
	if x.Tingkat != tingkat || x.Bagian != bagian {
		t.Errorf("wawasan %q: tingkat=%s bagian=%s, want %s/%s", potongan, x.Tingkat, x.Bagian, tingkat, bagian)
	}
	return x
}

func wajibTidakAda(t *testing.T, w []Wawasan, potongan string) {
	t.Helper()
	if x := cari(w, potongan); x != nil {
		t.Errorf("wawasan %q seharusnya tidak muncul: %+v", potongan, *x)
	}
}

func judul(w []Wawasan) []string {
	var j []string
	for _, x := range w {
		j = append(j, x.Tingkat+":"+x.Judul)
	}
	return j
}

func TestFormatRupiahPersenBulat(t *testing.T) {
	for in, want := range map[float64]string{
		0: "Rp 0", 950000: "Rp 950.000", 1500000: "Rp 1,5 juta", 12345678: "Rp 12,3 juta", 3400000000: "Rp 3,4 miliar",
		1250000000000: "Rp 1,25 triliun", -2500000000: "-Rp 2,5 miliar", 999999: "Rp 999.999", 1e9: "Rp 1 miliar",
	} {
		if got := Rupiah(in); got != want {
			t.Errorf("Rupiah(%v) = %q, want %q", in, got, want)
		}
	}
	if Persen(12.34) != "12,3%" || Persen(0) != "0%" || Persen(100) != "100%" {
		t.Errorf("Persen: %q %q %q", Persen(12.34), Persen(0), Persen(100))
	}
	if Bulat(1234567) != "1.234.567" || Bulat(12) != "12" || Bulat(-1000) != "-1.000" {
		t.Errorf("Bulat: %q %q %q", Bulat(1234567), Bulat(12), Bulat(-1000))
	}
}

func TestSusunTanpaDataTidakPanikDanKosong(t *testing.T) {
	if Susun(nil) != nil {
		t.Error("nil harus menghasilkan nil")
	}
	if w := Susun(&Hasil{Tahun: "2025", KodeKLPD: "K10", Sekarang: tgl(2025, 5, 1)}); len(w) != 0 {
		t.Errorf("hasil kosong menghasilkan wawasan: %v", judul(w))
	}
	// Bagian yang kosong tetapi tidak nil juga aman (pembagian dengan nol).
	h := &Hasil{Tahun: "2025", KodeKLPD: "K10", Sekarang: tgl(2025, 5, 1),
		RUP: &RUP{}, Pemilihan: &Pemilihan{}, Kontrak: &Kontrak{}, Ekatalog: &Ekatalog{}, Corong: &Corong{}}
	if w := Susun(h); len(w) != 0 {
		t.Errorf("bagian kosong menghasilkan wawasan: %v", judul(w))
	}
}

func TestUrutanWawasanMengikutiTingkat(t *testing.T) {
	h := &Hasil{
		Tahun: "2025", KodeKLPD: "K10", Sekarang: tgl(2025, 10, 4),
		RUP:           &RUP{TotalPaket: 10, TotalPagu: 1e9},
		Corong:        &Corong{TotalPaket: 10, TotalPagu: 1e9, Tahap: []Pasangan{{"Tender", 2, 2e8}, {"Belum diproses", 8, 8e8}}}, // 80% belum
		Pemilihan:     &Pemilihan{Efisiensi: Efisiensi{Sampel: 10, TotalHPS: 1e9, TotalKontrak: 9.5e8, Persen: 5, Median: 4}},
		DatasetKosong: []string{"Peserta Tender"},
	}
	w := Susun(h)
	rank := map[string]int{TingkatPenting: 0, TingkatPerhatian: 1, TingkatInfo: 2, TingkatBaik: 3}
	for i := 1; i < len(w); i++ {
		if rank[w[i-1].Tingkat] > rank[w[i].Tingkat] {
			t.Fatalf("urutan salah di %d: %v", i, judul(w))
		}
	}
	if w[0].Tingkat != TingkatPenting {
		t.Errorf("yang pertama harus penting: %v", judul(w))
	}
}

func TestWawasanCorong(t *testing.T) {
	corong := func(belumPagu float64, belumJml int64) *Corong {
		return &Corong{TotalPaket: 10, TotalPagu: 100, Tahap: []Pasangan{{"Tender", 10 - belumJml, 100 - belumPagu}, {"Belum diproses", belumJml, belumPagu}}}
	}
	// >= 50% belum diproses setelah Juni pada tahun berjalan: penting.
	w := Susun(&Hasil{Tahun: "2025", Sekarang: tgl(2025, 8, 1), Corong: corong(60, 6)})
	x := wajibAda(t, w, "Sebagian besar pagu perencanaan belum diproses", TingkatPenting, BagianCorong)
	if !strings.Contains(x.Isi, "60%") || !strings.Contains(x.Isi, "6 dari 10 paket") {
		t.Errorf("isi tidak memuat angka: %s", x.Isi)
	}
	// Pada paruh pertama tahun yang sama, 60% hanya perhatian (masih wajar).
	w = Susun(&Hasil{Tahun: "2025", Sekarang: tgl(2025, 3, 1), Corong: corong(60, 6)})
	wajibAda(t, w, "masih besar", TingkatPerhatian, BagianCorong)
	// Tahun lalu tidak memakai aturan "setelah Juni".
	w = Susun(&Hasil{Tahun: "2024", Sekarang: tgl(2025, 3, 1), Corong: corong(60, 6)})
	wajibTidakAda(t, w, "Sebagian besar pagu perencanaan belum diproses")
	// < 20%: baik; 20-30%: info.
	wajibAda(t, Susun(&Hasil{Tahun: "2025", Sekarang: tgl(2025, 8, 1), Corong: corong(10, 1)}), "banyak masuk proses", TingkatBaik, BagianCorong)
	wajibAda(t, Susun(&Hasil{Tahun: "2025", Sekarang: tgl(2025, 8, 1), Corong: corong(25, 3)}), "Keterhubungan", TingkatInfo, BagianCorong)
}

func TestWawasanTriwulanIV(t *testing.T) {
	bulan := func(nilai ...float64) []TitikBulan {
		var out []TitikBulan
		for i, n := range nilai {
			out = append(out, TitikBulan{Bulan: i + 1, Nilai: n})
		}
		return out
	}
	// 12 bulan; Oktober-Desember memuat 450 dari 900 = 50%.
	w := Susun(&Hasil{RUP: &RUP{PerBulanPemilihan: bulan(50, 50, 50, 50, 50, 50, 50, 50, 50, 150, 150, 150)}})
	x := wajibAda(t, w, "menumpuk di triwulan IV", TingkatPerhatian, BagianRUP)
	if !strings.Contains(x.Isi, "50%") || !strings.Contains(x.Isi, "Oktober") {
		t.Errorf("isi = %s", x.Isi)
	}
	// Merata: tidak ada wawasan. Satu bulan puncak 25%: info.
	wajibTidakAda(t, Susun(&Hasil{RUP: &RUP{PerBulanPemilihan: bulan(80, 80, 80, 80, 80, 80, 80, 80, 80, 80, 100, 100)}}), "triwulan IV")
	wajibAda(t, Susun(&Hasil{RUP: &RUP{PerBulanPemilihan: bulan(250, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 250)}}), "Bulan puncak", TingkatInfo, BagianRUP)
}

func TestWawasanEfisiensi(t *testing.T) {
	uji := func(sampel int64, persen, median float64) []Wawasan {
		return Susun(&Hasil{Pemilihan: &Pemilihan{Efisiensi: Efisiensi{Sampel: sampel, TotalHPS: 1000e6, TotalKontrak: 1000e6 * (1 - persen/100), Persen: persen, Median: median}}})
	}
	wajibAda(t, uji(20, -3, -1), "melampaui HPS", TingkatPenting, BagianPemilihan)
	wajibAda(t, uji(20, 1, 0.5), "Efisiensi harga tipis", TingkatPerhatian, BagianPemilihan)
	wajibAda(t, uji(20, 5, 4), "Efisiensi harga dari pemilihan", TingkatInfo, BagianPemilihan)
	x := wajibAda(t, uji(20, 12, 10), "Efisiensi harga sehat", TingkatBaik, BagianPemilihan)
	if !strings.Contains(x.Isi, "12%") || !strings.Contains(x.Isi, "Median per paket 10%") || !strings.Contains(x.Isi, "20 paket") {
		t.Errorf("isi = %s", x.Isi)
	}
	// Sampel kecil tidak boleh divonis, hanya diinformasikan.
	x = wajibAda(t, uji(3, -50, -50), "baru dihitung dari sedikit paket", TingkatInfo, BagianPemilihan)
	if !strings.Contains(x.Isi, "belum bisa dianggap representatif") {
		t.Errorf("isi = %s", x.Isi)
	}
	if w := uji(0, 0, 0); len(w) != 0 {
		t.Errorf("tanpa sampel: %v", judul(w))
	}
}

func TestWawasanPersaingan(t *testing.T) {
	uji := func(total, satu int64, rata float64) []Wawasan {
		return Susun(&Hasil{Pemilihan: &Pemilihan{Persaingan: Persaingan{TenderBerpeserta: total, SatuPeserta: satu, RataPeserta: rata}}})
	}
	wajibAda(t, uji(100, 60, 2), "sangat lemah", TingkatPenting, BagianPemilihan)
	wajibAda(t, uji(100, 35, 4), "perlu diperkuat", TingkatPerhatian, BagianPemilihan)
	wajibAda(t, uji(100, 10, 2.5), "perlu diperkuat", TingkatPerhatian, BagianPemilihan) // rata-rata peserta rendah
	wajibAda(t, uji(100, 10, 5), "cukup sehat", TingkatBaik, BagianPemilihan)
	wajibAda(t, uji(100, 20, 5), "Tingkat persaingan", TingkatInfo, BagianPemilihan)
	x := uji(100, 60, 2.25)[0]
	if !strings.Contains(x.Isi, "60 dari 100 tender (60%)") || !strings.Contains(x.Isi, "2,3 peserta") && !strings.Contains(x.Isi, "2,2 peserta") {
		t.Errorf("isi = %s", x.Isi)
	}
}

func TestWawasanPasarPenyedia(t *testing.T) {
	uji := func(hhi float64, topNilai float64) []Wawasan {
		return Susun(&Hasil{Pemilihan: &Pemilihan{Pasar: PasarPenyedia{JumlahPenyedia: 12, TotalNilai: 100e6, HHI: hhi, Top: []Pasangan{{"PT Maju Jaya", 4, topNilai}}}}})
	}
	x := wajibAda(t, uji(3000, 60e6), "sangat terkonsentrasi", TingkatPenting, BagianPemilihan)
	if !strings.Contains(x.Isi, "PT Maju Jaya") || !strings.Contains(x.Isi, "60%") || !strings.Contains(x.Isi, "3.000") {
		t.Errorf("isi = %s", x.Isi)
	}
	wajibAda(t, uji(1800, 20e6), "cukup tinggi", TingkatPerhatian, BagianPemilihan)
	wajibAda(t, uji(1000, 30e6), "cukup tinggi", TingkatPerhatian, BagianPemilihan) // satu penyedia >= 25%
	wajibAda(t, uji(800, 10e6), "terdiversifikasi", TingkatBaik, BagianPemilihan)
}

func TestWawasanAddendumDanKontrakBerakhir(t *testing.T) {
	uji := func(add int64) []Wawasan {
		return Susun(&Hasil{Kontrak: &Kontrak{TenderJumlah: 60, NonTenderJumlah: 40, Addendum: add}})
	}
	wajibAda(t, uji(35), "sangat sering", TingkatPenting, BagianKontrak)
	wajibAda(t, uji(20), "Banyak kontrak beradendum", TingkatPerhatian, BagianKontrak)
	wajibAda(t, uji(5), "Kontrak dengan addendum", TingkatInfo, BagianKontrak)
	wajibTidakAda(t, uji(0), "ddendum")

	k := &Kontrak{BerakhirDalam: 3, NilaiBerakhir: 4.5e9, HariPeringatan: 60, AkanBerakhir: []KontrakBerakhir{
		{NamaPaket: "Pemeliharaan Gedung Utama", Nilai: 2e9, Berakhir: tgl(2025, 11, 3), SisaHari: 12},
	}}
	x := wajibAda(t, Susun(&Hasil{Kontrak: k}), "Kontrak segera berakhir", TingkatPerhatian, BagianKontrak)
	for _, s := range []string{"3 kontrak", "Rp 4,5 miliar", "60 hari", "Pemeliharaan Gedung Utama", "03-11-2025", "sisa 12 hari"} {
		if !strings.Contains(x.Isi, s) {
			t.Errorf("isi tidak memuat %q: %s", s, x.Isi)
		}
	}
}

func TestWawasanWaktuProsesDanStatusTender(t *testing.T) {
	wajibAda(t, Susun(&Hasil{Pemilihan: &Pemilihan{WaktuProses: WaktuProses{Sampel: 30, Median: 75, Rata: 80}}}), "relatif lama", TingkatPerhatian, BagianPemilihan)
	wajibAda(t, Susun(&Hasil{Pemilihan: &Pemilihan{WaktuProses: WaktuProses{Sampel: 30, Median: 40, Rata: 45}}}), "Lama proses", TingkatInfo, BagianPemilihan)

	status := []Pasangan{{"Selesai", 70, 0}, {"Tender Gagal", 10, 0}, {"Dibatalkan", 5, 0}, {"Ditutup", 5, 0}, {"Berjalan", 10, 0}}
	x := wajibAda(t, Susun(&Hasil{Pemilihan: &Pemilihan{StatusTender: status}}), "gagal atau diulang", TingkatPerhatian, BagianPemilihan)
	if !strings.Contains(x.Isi, "20 dari 100 tender (20%)") {
		t.Errorf("isi = %s", x.Isi)
	}
	wajibTidakAda(t, Susun(&Hasil{Pemilihan: &Pemilihan{StatusTender: []Pasangan{{"Selesai", 95, 0}, {"Gagal", 5, 0}}}}), "gagal atau diulang")
}

func TestWawasanRUPSatkerMetodeUmumkan(t *testing.T) {
	r := &RUP{
		TotalPaket: 100, TotalPagu: 1000e6,
		TopSatker:     []Pasangan{{"Direktorat Jenderal Anggaran", 30, 400e6}},
		PerMetode:     []Pasangan{{"Pengadaan Langsung", 60, 550e6}, {"Tender", 40, 450e6}},
		StatusUmumkan: []Pasangan{{"Terumumkan", 70, 700e6}, {"Belum Terumumkan", 30, 300e6}},
	}
	w := Susun(&Hasil{RUP: r})
	wajibAda(t, w, "terkonsentrasi pada satu satuan kerja", TingkatPerhatian, BagianRUP)
	x := wajibAda(t, w, "Metode Pengadaan Langsung mendominasi", TingkatInfo, BagianRUP)
	if !strings.Contains(x.Isi, "55%") {
		t.Errorf("isi = %s", x.Isi)
	}
	x = wajibAda(t, w, "belum terumumkan", TingkatPerhatian, BagianRUP)
	if !strings.Contains(x.Isi, "30 paket") || !strings.Contains(x.Isi, "30%") {
		t.Errorf("isi = %s", x.Isi)
	}
	// 20-35%: info; di bawahnya: tidak disebut.
	r.TopSatker = []Pasangan{{"Satker A", 5, 250e6}}
	wajibAda(t, Susun(&Hasil{RUP: r}), "dengan pagu terbesar", TingkatInfo, BagianRUP)
	r.TopSatker = []Pasangan{{"Satker A", 5, 100e6}}
	wajibTidakAda(t, Susun(&Hasil{RUP: r}), "satuan kerja")
	// "Terumumkan" saja (tanpa "belum") dihitung sudah diumumkan.
	r.StatusUmumkan = []Pasangan{{"Terumumkan", 100, 1000e6}}
	wajibTidakAda(t, Susun(&Hasil{RUP: r}), "belum terumumkan")
}

func TestWawasanEkatalog(t *testing.T) {
	h := &Hasil{
		Pemilihan: &Pemilihan{NilaiKontrak: 600e6},
		Ekatalog: &Ekatalog{
			V5: EkatalogV5{Nilai: 300e6, TopKomoditas: []Pasangan{{"Laptop", 10, 150e6}}},
			V6: EkatalogV6{Nilai: 100e6, NilaiSwasta: 30e6, OrderSwasta: 7},
		},
	}
	w := Susun(h)
	x := wajibAda(t, w, "saluran pengadaan utama", TingkatInfo, BagianEkatalog)
	if !strings.Contains(x.Isi, "40%") { // 400 / (400 + 600)
		t.Errorf("isi = %s", x.Isi)
	}
	wajibAda(t, w, "bergantung pada satu komoditas", TingkatPerhatian, BagianEkatalog)
	wajibAda(t, w, "paket swasta", TingkatInfo, BagianEkatalog)

	h.Pemilihan.NilaiKontrak = 9000e6
	wajibAda(t, Susun(h), "Realisasi lewat e-purchasing", TingkatInfo, BagianEkatalog)
}

func TestWawasanPembandingTahunLalu(t *testing.T) {
	h := &Hasil{
		Tahun:      "2025",
		RUP:        &RUP{TotalPaket: 120, TotalPagu: 1200e6},
		Pemilihan:  &Pemilihan{TenderJumlah: 50},
		Pembanding: &Pembanding{Tahun: "2024", RUPPaket: 100, RUPPagu: 1000e6, TenderJumlah: 40},
	}
	w := Susun(h)
	x := wajibAda(t, w, "naik dibanding 2024", TingkatInfo, BagianRingkasan)
	if !strings.Contains(x.Isi, "20%") || !strings.Contains(x.Isi, "Rp 1,2 miliar") {
		t.Errorf("isi = %s", x.Isi)
	}
	wajibAda(t, w, "Jumlah tender berubah", TingkatInfo, BagianPemilihan)

	h.RUP.TotalPagu = 800e6
	wajibAda(t, Susun(h), "turun dibanding 2024", TingkatInfo, BagianRingkasan)
	// Perubahan kecil tidak disebut.
	h.RUP.TotalPagu = 1050e6
	h.Pemilihan.TenderJumlah = 41
	w = Susun(h)
	wajibTidakAda(t, w, "dibanding 2024")
	wajibTidakAda(t, w, "Jumlah tender berubah")
}

func TestWawasanKelengkapanDataDanGalat(t *testing.T) {
	lama := tgl(2025, 9, 20)
	h := &Hasil{
		Tahun: "2025", KodeKLPD: "K10", Sekarang: tgl(2025, 10, 4),
		DatasetKosong: []string{"A", "B", "C", "D", "E", "F", "G", "H"},
		TerakhirTarik: &lama,
		Galat:         map[string]string{"pemilihan": "x", "kontrak": "y"},
	}
	w := Susun(h)
	x := wajibAda(t, w, "Sebagian data belum ditarik", TingkatPerhatian, BagianData)
	if !strings.Contains(x.Isi, "A, B, C, D, E, F dan 2 dataset lain") || !strings.Contains(x.Isi, "K10 tahun 2025") {
		t.Errorf("isi = %s", x.Isi)
	}
	x = wajibAda(t, w, "sudah lama tidak diperbarui", TingkatPerhatian, BagianData)
	if !strings.Contains(x.Isi, "14 hari") {
		t.Errorf("isi = %s", x.Isi)
	}
	// Galat per bagian muncul berurutan (bukan acak) dan tidak membocorkan isi galat.
	var galat []string
	for _, y := range w {
		if y.Judul == "Satu bagian dasbor gagal dihitung" {
			galat = append(galat, y.Isi)
			if strings.Contains(y.Isi, " x") || strings.Contains(y.Isi, "(y)") {
				t.Errorf("isi galat bocor: %s", y.Isi)
			}
		}
	}
	if len(galat) != 2 || !strings.Contains(galat[0], "kontrak") || !strings.Contains(galat[1], "pemilihan") {
		t.Errorf("galat = %v", galat)
	}
	// Data segar: tidak ada peringatan data lama.
	baru := tgl(2025, 10, 3)
	h.TerakhirTarik = &baru
	wajibTidakAda(t, Susun(h), "sudah lama tidak diperbarui")
}
