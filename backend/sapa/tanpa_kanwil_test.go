package sapa

import (
	"errors"
	"strings"
	"testing"
)

// Tidak semua satker punya Kanwil: dengan "tanpa Kanwil" tembusannya tidak wajib diisi dan butir tembusan Kanwil dibuang dari dokumen (bukan dibiarkan kosong), sedangkan
// butir tembusan lain tetap ada. Tanpa centang itu tembusan tetap wajib, seperti sebelumnya.
func TestNDSatkerTanpaKanwil(t *testing.T) {
	// tetap wajib bila tidak dicentang; pesannya menunjuk jalan keluarnya
	d := ndSatkerContoh()
	d.KepalaKanwil = ""
	if g := d.Validasi(); len(g) != 1 || !strings.Contains(g[0], "Tembusan Kepala Kantor Wilayah") || !strings.Contains(g[0], "tidak punya Kanwil") {
		t.Errorf("tanpa centang dan tanpa isian: %v", g)
	}

	// dicentang: sah tanpa isian, dan isian yang sempat diketik dibuang
	d.TanpaKanwil = true
	d.KepalaKanwil = "Kepala Kantor Wilayah DJKN Lama"
	d.Rapikan()
	if d.KepalaKanwil != "" || len(d.Validasi()) != 0 {
		t.Errorf("setelah dicentang: kepala=%q galat=%v", d.KepalaKanwil, d.Validasi())
	}

	// dokumen: butir Kanwil dibuang pada ND Satker, butir lain tetap
	h, err := IsiNDSatker(templateBawaan(t, DokNDSatker), kasusContoh(), d)
	if err != nil {
		t.Fatal(err)
	}
	teks := bukaHasil(t, h).Text()
	if strings.Contains(teks, "Kantor Wilayah DJKN") || strings.Contains(teks, "<<Kepala Kantor Wilayah>>") {
		t.Errorf("butir tembusan Kanwil masih ada:\n%s", teks)
	}
	if !strings.Contains(teks, "Tembusan:") || !strings.Contains(teks, "Kepala Biro Manajemen BMN dan Pengadaan") {
		t.Errorf("butir tembusan lain harus tetap ada:\n%s", teks)
	}
	if strings.Contains(strings.Join(h.Peringatan, " "), "kepala kantor wilayah") {
		t.Errorf("tidak boleh ada peringatan penanda tak terisi: %v", h.Peringatan)
	}

	// pembanding: tanpa centang, butir Kanwil terisi seperti biasa
	biasa, err := IsiNDSatker(templateBawaan(t, DokNDSatker), kasusContoh(), ndSatkerContoh())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bukaHasil(t, biasa).Text(), "Kepala Kantor Wilayah DJKN Jakarta") {
		t.Error("tanpa centang, tembusan Kanwil harus terisi")
	}
}

func TestNDUE1TanpaKanwil(t *testing.T) {
	ue := ndUE1Contoh()
	ue.KepalaKanwil = ""
	if g := ue.Validasi(); len(g) != 1 || !strings.Contains(g[0], "tidak punya Kanwil") {
		t.Errorf("tanpa centang dan tanpa isian: %v", g)
	}
	ue.TanpaKanwil = true
	ue.Rapikan()
	if g := ue.Validasi(); len(g) != 0 {
		t.Fatalf("setelah dicentang: %v", g)
	}
	h, err := IsiNDUE1(templateBawaan(t, DokNDUE1), kasusContoh(), ndSatkerContoh(), ue)
	if err != nil {
		t.Fatal(err)
	}
	teks := bukaHasil(t, h).Text()
	if strings.Contains(teks, "Kantor Wilayah DJKN") {
		t.Errorf("butir tembusan Kanwil masih ada:\n%s", teks)
	}
	if !strings.Contains(teks, "Tembusan:") || !strings.Contains(teks, ue.PejabatPengelola) {
		t.Errorf("butir tembusan pejabat pengelola harus tetap ada:\n%s", teks)
	}
}

// Saran tembusan Kanwil pada formulir ND datang dari referensi Kanwil menurut 9 karakter pertama kode satker; tanpa referensi (atau nonaktif) sarannya kosong karena tidak
// semua satker punya Kanwil. ND UE1 mengikuti usulan Satker, termasuk pilihan "tanpa Kanwil".
func TestSaranTembusanKanwilDariReferensi(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	sarNDSatker := func() DataNDSatker {
		t.Helper()
		s, ok := e.tahap(e.satkerA, k.ID, TahapNDSatker).Saran.(DataNDSatker)
		if !ok {
			t.Fatal("saran ND Satker bukan DataNDSatker")
		}
		return s
	}
	// tanpa referensi: kosong
	if s := sarNDSatker(); s.KepalaKanwil != "" || s.TanpaKanwil {
		t.Errorf("tanpa referensi: %+v", s)
	}
	// referensi aktif: "Kepala <uraian>" berhuruf judul bila uraiannya huruf besar semua
	kode9 := Kanwil9Dari(kodeA)
	e.repo.Kanwil[kode9] = RefKanwil{Kode: kode9, Nama: "KANTOR WILAYAH DJP JAKARTA PUSAT", Aktif: true}
	if s := sarNDSatker(); s.KepalaKanwil != "Kepala Kantor Wilayah DJP Jakarta Pusat" || s.KodeKanwil != kode9 {
		t.Errorf("dari referensi: %q (kode %q), want kode %q", s.KepalaKanwil, s.KodeKanwil, kode9)
	}
	// referensi nonaktif tidak disarankan
	e.repo.Kanwil[kode9] = RefKanwil{Kode: kode9, Nama: "KANTOR WILAYAH DJP JAKARTA PUSAT", Aktif: false}
	if s := sarNDSatker(); s.KepalaKanwil != "" || s.KodeKanwil != "" {
		t.Errorf("referensi nonaktif: %q (kode %q)", s.KepalaKanwil, s.KodeKanwil)
	}
	e.repo.Kanwil[kode9] = RefKanwil{Kode: kode9, Nama: "Kantor Wilayah DJP Jakarta Pusat", Aktif: true}

	// ND UE1 memakai saran dari referensi bila ND Satker belum ada, dan mengikuti ND Satker (termasuk tanpa Kanwil) setelah ada
	e.siapkanNDSatker(k)
	for _, tk := range []string{TahapNadineSatker, TahapSimanSatker} {
		if err := e.l.Selesaikan(e.ctx, e.satkerA, k.ID, tk, "ND-1/2026", "2026-10-01", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.l.Selesaikan(e.ctx, e.kanwil, k.ID, TahapSimanKanwil, "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := e.l.Selesaikan(e.ctx, e.ue1, k.ID, TahapUE1Terima, "", "", ""); err != nil {
		t.Fatal(err)
	}
	ue, ok := e.tahap(e.ue1, k.ID, TahapNDUE1).Saran.(DataNDUE1)
	if !ok || ue.KepalaKanwil != ndSatkerContoh().KepalaKanwil || ue.TanpaKanwil {
		t.Errorf("saran ND UE1 mengikuti ND Satker yang tersimpan: %+v", ue)
	}
}

// Tembusan Kanwil terhubung ke Referensi Kanwil lewat kode 9 digit: kode boleh kosong (diketik manual), selain itu harus 9 digit angka, dan dibuang bersama isian bila "tanpa Kanwil".
func TestKodeKanwilTerhubungDanDivalidasi(t *testing.T) {
	d := ndSatkerContoh()
	d.KodeKanwil = " 015010199 "
	d.Rapikan()
	if d.KodeKanwil != "015010199" || len(d.Validasi()) != 0 {
		t.Errorf("kode sah: %q galat=%v", d.KodeKanwil, d.Validasi())
	}
	for _, buruk := range []string{"01501", "01501019A", "0150101999", "kanwil"} {
		d.KodeKanwil = buruk
		if g := d.Validasi(); len(g) != 1 || !strings.Contains(g[0], "Kode Kanwil harus 9 digit angka") {
			t.Errorf("kode %q: galat=%v", buruk, g)
		}
	}
	d.KodeKanwil = ""
	if g := d.Validasi(); len(g) != 0 {
		t.Errorf("kode kosong (manual) harus sah: %v", g)
	}

	// tanpa Kanwil: kode ikut dibuang dan tidak divalidasi
	d.KodeKanwil, d.TanpaKanwil = "salah", true
	d.Rapikan()
	if d.KodeKanwil != "" || len(d.Validasi()) != 0 {
		t.Errorf("tanpa Kanwil: kode=%q galat=%v", d.KodeKanwil, d.Validasi())
	}

	ue := DataNDUE1{NomorND: "ND-1/2026", TanggalND: "2026-10-01", HalND: "Hal", SekretarisUE1: "Sekretaris", KepalaKanwil: "Kepala Kanwil X", KodeKanwil: "12345", PejabatPengelola: "Direktur",
		Penandatangan: Penandatangan{Nama: "Siti", Jabatan: "Sekretaris Ditjen"}}
	if g := ue.Validasi(); len(g) != 1 || !strings.Contains(g[0], "Kode Kanwil harus 9 digit angka") {
		t.Errorf("ND UE1 kode salah: %v", g)
	}
	ue.KodeKanwil = "015010199"
	if g := ue.Validasi(); len(g) != 0 {
		t.Errorf("ND UE1 kode sah: %v", g)
	}
	ue.TanpaKanwil, ue.KodeKanwil = true, "015010199"
	ue.Rapikan()
	if ue.KodeKanwil != "" || ue.KepalaKanwil != "" {
		t.Errorf("ND UE1 tanpa Kanwil: %+v", ue)
	}
}

// Pemilih tembusan di formulir memakai daftar Kanwil aktif (urut menurut kode) beserta saran teks tembusannya; hanya pengguna SAPA yang boleh membacanya.
func TestRefKanwilAktifUntukPemilih(t *testing.T) {
	e := baru(t)
	e.repo.Kanwil["015040199"] = RefKanwil{Kode: "015040199", Nama: "KANTOR WILAYAH DJBC JAWA TIMUR I", Singkatan: "KW DJBC JATIM I", Aktif: true}
	e.repo.Kanwil["015010199"] = RefKanwil{Kode: "015010199", Nama: "Kantor Wilayah DJP Jakarta Pusat", Aktif: true}
	e.repo.Kanwil["015020199"] = RefKanwil{Kode: "015020199", Nama: "KANTOR WILAYAH NONAKTIF", Aktif: false}

	got, err := e.l.RefKanwilAktif(e.ctx, e.satkerA)
	if err != nil {
		t.Fatal(err)
	}
	want := []PilihanKanwil{
		{Kode: "015010199", Nama: "Kantor Wilayah DJP Jakarta Pusat", Tembusan: "Kepala Kantor Wilayah DJP Jakarta Pusat"},
		{Kode: "015040199", Nama: "KANTOR WILAYAH DJBC JAWA TIMUR I", Singkatan: "KW DJBC JATIM I", Tembusan: "Kepala Kantor Wilayah DJBC Jawa Timur I"},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("daftar = %+v, want %+v", got, want)
	}
	if _, err := e.l.RefKanwilAktif(e.ctx, e.tanpa); !errors.Is(err, ErrTanpaPeran) {
		t.Errorf("tanpa peran: err = %v, want ErrTanpaPeran", err)
	}
}
