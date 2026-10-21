package apiakses

import (
	"context"
	"testing"
	"time"
)

func TestPenjagaGagalMengunciSetelahAmbang(t *testing.T) {
	j := &jam{t: time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)}
	g := NewPenjagaGagal(3, 10*time.Minute, 100)
	g.sekarang = j.now
	for i := 0; i < 2; i++ {
		g.Gagal("1.1.1.1")
		if k, _ := g.Terkunci("1.1.1.1"); k {
			t.Fatalf("belum boleh terkunci setelah %d kegagalan", i+1)
		}
	}
	g.Gagal("1.1.1.1")
	k, tunggu := g.Terkunci("1.1.1.1")
	if !k || tunggu != 10*time.Minute {
		t.Fatalf("harus terkunci 10 menit: %v %v", k, tunggu)
	}
	if k, _ := g.Terkunci("2.2.2.2"); k {
		t.Error("alamat lain tidak terpengaruh")
	}
	j.maju(4 * time.Minute)
	if _, tunggu := g.Terkunci("1.1.1.1"); tunggu != 6*time.Minute {
		t.Errorf("sisa tunggu = %v, want 6m", tunggu)
	}
	j.maju(7 * time.Minute)
	if k, _ := g.Terkunci("1.1.1.1"); k {
		t.Error("setelah jendela lewat tidak terkunci lagi")
	}
	if g.Ukuran() != 0 {
		t.Errorf("catatan lama dibuang: %d", g.Ukuran())
	}
}

func TestPenjagaGagalMemoriTerbatas(t *testing.T) {
	j := &jam{t: time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)}
	g := NewPenjagaGagal(5, time.Minute, 10)
	g.sekarang = j.now
	for i := 0; i < 1000; i++ {
		g.Gagal("10.0." + string(rune('a'+i%26)) + "." + string(rune('a'+i/26%26)))
	}
	if g.Ukuran() > 10 {
		t.Errorf("ukuran = %d, tidak boleh melebihi batas 10", g.Ukuran())
	}
	j.maju(2 * time.Minute)
	g.Gagal("baru")
	if g.Ukuran() != 1 {
		t.Errorf("setelah jendela lewat tabel dibersihkan: %d", g.Ukuran())
	}
}

func pnc(waktu time.Time, hasil string, status, ms int, baris *int) Penggunaan {
	return Penggunaan{Waktu: waktu, KlienID: 5, Metode: "GET", Endpoint: "/aset/:dataset", Status: status, Hasil: hasil, DurasiMS: ms, Baris: baris}
}

func TestPencatatMenjumlahkanPerJamDanMenulis(t *testing.T) {
	p := &penyimpanTiruan{}
	c := NewPencatat(p, 100)
	c.Mulai()
	t0 := time.Date(2026, 10, 7, 10, 15, 0, 0, time.UTC)
	n := 100
	c.Catat(pnc(t0, HasilOK, 200, 30, &n))
	c.Catat(pnc(t0.Add(10*time.Minute), HasilOK, 200, 300, &n))
	c.Catat(pnc(t0.Add(20*time.Minute), HasilOK, 200, 5000, nil))
	c.Catat(pnc(t0.Add(50*time.Minute), HasilBatasLaju, 429, 1, nil)) // jam 11
	c.Berhenti(context.Background())

	if len(p.salinanLog()) != 4 {
		t.Fatalf("riwayat = %d, want 4", len(p.salinanLog()))
	}
	var jam10 *Agregat
	for i := range p.agg {
		if p.agg[i].Hasil == HasilOK && p.agg[i].Jam.Hour() == 10 {
			jam10 = &p.agg[i]
		}
	}
	if jam10 == nil {
		t.Fatalf("agregat jam 10 tidak ada: %+v", p.agg)
	}
	if jam10.Jumlah != 3 || jam10.Baris != 200 || jam10.LatTotal != 5330 || jam10.LatMaks != 5000 {
		t.Errorf("agregat = %+v", *jam10)
	}
	// Histogram: 30 ms -> <=50, 300 ms -> <=500, 5000 ms -> lebih.
	if jam10.Ember[0] != 1 || jam10.Ember[3] != 1 || jam10.Ember[6] != 1 || jam10.Ember[1]+jam10.Ember[2]+jam10.Ember[4]+jam10.Ember[5] != 0 {
		t.Errorf("histogram = %v", jam10.Ember)
	}
	if st := c.Statistik(); st.Diterima != 4 || st.Ditulis != 4 || st.Dijatuhkan != 0 || st.Gagal != 0 {
		t.Errorf("statistik = %+v", st)
	}
	// Setelah berhenti, catatan baru diabaikan (tidak panik).
	c.Catat(pnc(t0, HasilOK, 200, 1, nil))
}

func TestPencatatAntreanPenuhMenjatuhkanRiwayatTetapMenghitung(t *testing.T) {
	p := &penyimpanTiruan{}
	c := NewPencatat(p, 5) // penulis belum dijalankan: antrean tidak terkuras
	t0 := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 20; i++ {
		c.Catat(pnc(t0, HasilOK, 200, 10, nil))
	}
	st := c.Statistik()
	if st.Diterima != 20 || st.Dijatuhkan != 15 || st.Antrean != 5 {
		t.Errorf("statistik = %+v", st)
	}
	c.Berhenti(context.Background())
	var total int64
	for _, a := range p.agg {
		total += a.Jumlah
	}
	if total != 20 {
		t.Errorf("hitungan per jam tetap lengkap walau riwayat dijatuhkan: %d", total)
	}
	if len(p.salinanLog()) != 5 {
		t.Errorf("riwayat tersimpan = %d, want 5", len(p.salinanLog()))
	}
}

func TestPencatatGagalMenulisAgregatDicobaLagi(t *testing.T) {
	p := &penyimpanTiruan{gagal: true}
	c := NewPencatat(p, 100)
	t0 := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
	c.Catat(pnc(t0, HasilOK, 200, 10, nil))
	c.Catat(pnc(t0, HasilOK, 200, 10, nil))
	c.tulisAgregat() // gagal: hitungan dikembalikan ke memori
	if len(p.agg) != 0 {
		t.Fatal("penyimpan gagal tidak boleh menerima apa pun")
	}
	c.Catat(pnc(t0, HasilOK, 200, 10, nil))
	p.mu.Lock()
	p.gagal = false
	p.mu.Unlock()
	c.tulisAgregat()
	if len(p.agg) != 1 || p.agg[0].Jumlah != 3 {
		t.Errorf("hitungan harus utuh setelah dicoba lagi: %+v", p.agg)
	}
}

func TestPersentilDariHistogram(t *testing.T) {
	// 90 permintaan <=50 ms, 9 <=250 ms, 1 > 2500 ms (maksimum 4000 ms)
	e := [7]int64{90, 0, 9, 0, 0, 0, 1}
	if got := persentil(e, 100, 0.50, 4000); got != 50 {
		t.Errorf("p50 = %d, want 50", got)
	}
	if got := persentil(e, 100, 0.95, 4000); got != 250 {
		t.Errorf("p95 = %d, want 250", got)
	}
	if got := persentil(e, 100, 0.995, 4000); got != 4000 {
		t.Errorf("p99.5 = %d, want 4000 (maksimum teramati)", got)
	}
	if got := persentil([7]int64{}, 0, 0.95, 0); got != 0 {
		t.Errorf("tanpa data = %d", got)
	}
	// Batas atas ember tidak boleh melebihi maksimum teramati.
	if got := persentil([7]int64{5, 0, 0, 0, 0, 0, 0}, 5, 0.95, 12); got != 12 {
		t.Errorf("p95 = %d, want 12 (maksimum teramati lebih kecil dari batas ember)", got)
	}
}

func TestIsiCelahDeret(t *testing.T) {
	dari := time.Date(2026, 10, 7, 10, 20, 0, 0, time.UTC)
	sampai := time.Date(2026, 10, 7, 13, 5, 0, 0, time.UTC)
	ada := map[int64]TitikDeret{
		time.Date(2026, 10, 7, 11, 0, 0, 0, time.UTC).Unix(): {Waktu: time.Date(2026, 10, 7, 11, 0, 0, 0, time.UTC), Jumlah: 7},
	}
	d := IsiCelah(ada, dari, sampai, false)
	if len(d) != 4 || d[1].Jumlah != 7 || d[0].Jumlah != 0 || !d[0].Waktu.Equal(time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("per jam: %+v", d)
	}
	hari := IsiCelah(map[int64]TitikDeret{}, time.Date(2026, 10, 5, 3, 0, 0, 0, time.UTC), time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC), true)
	// Awal hari WIB: 5 Okt 03:00 UTC = 5 Okt 10:00 WIB -> awal hari 5 Okt 00:00 WIB = 4 Okt 17:00 UTC; 3 hari WIB (5, 6, 7 Okt).
	if len(hari) != 3 || !hari[0].Waktu.Equal(time.Date(2026, 10, 4, 17, 0, 0, 0, time.UTC)) {
		t.Errorf("per hari WIB: %+v", hari)
	}
	if n := len(IsiCelah(nil, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), false)); n != 800 {
		t.Errorf("deret dibatasi 800 titik: %d", n)
	}
}
