package monitor

import (
	"testing"
	"time"
)

func sampelUji(t0 time.Time, i int, cpu float64, mem uint64) sampel {
	c := cpu
	return sampel{Waktu: t0.Add(time.Duration(i) * 30 * time.Second), CPU: &c, Load1: pf(1.5), MemTotal: 8 * gb, MemTerpakai: mem, SwapTerpakai: 0, DiskTotal: 100 * gb, DiskTerpakai: 40 * gb,
		RSS: 100 << 20, Heap: 40 << 20, Goroutine: 100 + i, DBBuka: 5, DBDipakai: 2 + i%3, DBBatas: 25, DBMenunggu: int64(i), BerjalanMaks: int64(1 + i%2)}
}

func TestBangunSnapshotMeringkasJendela(t *testing.T) {
	t0 := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
	var ss []sampel
	for i := 0; i < 10; i++ {
		s := sampelUji(t0, i, float64(10*(i+1)), uint64(2+i)*gb/2)
		s.Req = hitungan{Total: 10, C4xx: 1, C5xx: 0, NR: 8, SumR: 8 * 40}
		s.Req.HistR.tambah(40)
		ss = append(ss, s)
	}
	n := bangunSnapshot(ss, t0.Add(5*time.Minute), 3, pf(5000), pf(4000))
	if n.CPURata == nil || *n.CPURata != 55 || n.CPUMaks == nil || *n.CPUMaks != 100 {
		t.Errorf("CPU rata/maks = %v/%v, want 55/100", n.CPURata, n.CPUMaks)
	}
	if n.MemTotal != 8*gb || n.MemMaks != 11*gb/2 || n.MemRata == 0 || n.MemRata >= n.MemMaks {
		t.Errorf("memori: total %d rata %d maks %d", n.MemTotal, n.MemRata, n.MemMaks)
	}
	if n.GoroutineMaks != 109 || n.DBDipakaiMaks != 4 || n.DBBatas != 25 || n.ReqBerjalanMaks != 2 {
		t.Errorf("proses/db: goroutine %d db %d batas %d berjalan %d", n.GoroutineMaks, n.DBDipakaiMaks, n.DBBatas, n.ReqBerjalanMaks)
	}
	if n.DBTungguDelta != 6 { // kumulatif terakhir 9 dikurangi nilai awal 3
		t.Errorf("tunggu delta = %d, want 6", n.DBTungguDelta)
	}
	if n.ReqTotal != 100 || n.Req4xx != 10 || n.LatRata == nil || *n.LatRata != 40 || n.LatP95 == nil || *n.LatP95 <= 25 || *n.LatP95 > 50 {
		t.Errorf("permintaan: total %d 4xx %d rata %v p95 %v", n.ReqTotal, n.Req4xx, n.LatRata, n.LatP95)
	}
	if n.Jendela != 9*30+30 || n.DBUkuranMB == nil || *n.DBUkuranMB != 5000 || n.Load1Maks == nil || *n.Load1Maks != 1.5 {
		t.Errorf("jendela %d ukuran %v load %v", n.Jendela, n.DBUkuranMB, n.Load1Maks)
	}

	// CPU tidak tersedia (pengukuran pertama): tetap menghasilkan snapshot tanpa CPU
	s := sampelUji(t0, 0, 0, gb)
	s.CPU = nil
	if k := bangunSnapshot([]sampel{s}, t0, 0, nil, nil); k.CPURata != nil || k.CPUMaks != nil || k.MemMaks != gb {
		t.Errorf("tanpa CPU = %+v", k)
	}
	if k := bangunSnapshot(nil, t0, 0, nil, nil); k.MemTotal != 0 || k.Waktu.IsZero() {
		t.Errorf("kosong = %+v", k)
	}
	// penghitung kumulatif yang turun (aplikasi dimulai ulang) tidak menghasilkan selisih negatif
	if k := bangunSnapshot([]sampel{sampelUji(t0, 0, 5, gb)}, t0, 99, nil, nil); k.DBTungguDelta != 0 {
		t.Errorf("selisih negatif: %d", k.DBTungguDelta)
	}
}

func TestAgregatDariMenghitungRataDanPuncak(t *testing.T) {
	t0 := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	ss := []Snapshot{
		{Waktu: t0, CPURata: pf(10), CPUMaks: pf(40), Load1Maks: pf(0.5), MemTotal: 8 * gb, MemMaks: 2 * gb, SwapMaks: 0, DiskTotal: 100 * gb, DiskTerpakai: 50 * gb, RSSMaks: 100 << 20,
			GoroutineMaks: 90, DBDipakaiMaks: 3, DBBatas: 25, DBTungguDelta: 1, LatP95: pf(120), ReqTotal: 100, Req5xx: 1},
		{Waktu: t0.Add(5 * time.Minute), CPURata: pf(30), CPUMaks: pf(95), Load1Maks: pf(2.5), MemTotal: 8 * gb, MemMaks: 6 * gb, SwapMaks: gb, DiskTotal: 100 * gb, DiskTerpakai: 52 * gb, RSSMaks: 300 << 20,
			GoroutineMaks: 150, DBDipakaiMaks: 9, DBBatas: 25, DBTungguDelta: 4, LatP95: pf(800), ReqTotal: 300, Req5xx: 2},
	}
	a := AgregatDari(ss, t0)
	if a.Jumlah != 2 || a.CPURata == nil || *a.CPURata != 20 || *a.CPUMaks != 95 || *a.Load1Maks != 2.5 {
		t.Errorf("cpu/load = %+v", a)
	}
	if a.MemPersenMaks == nil || *a.MemPersenMaks != 75 || *a.MemMaks != 6*gb || *a.SwapMaks != gb {
		t.Errorf("memori = %v %v %v", a.MemPersenMaks, a.MemMaks, a.SwapMaks)
	}
	if a.DiskPersenMaks == nil || *a.DiskPersenMaks != 52 || *a.RSSMaks != 300<<20 || *a.GoroutineMaks != 150 || *a.DBDipakaiMaks != 9 || *a.DBBatas != 25 {
		t.Errorf("disk/proses/db = %+v", a)
	}
	if a.DBTungguTotal != 5 || a.ReqTotal != 400 || a.Req5xx != 3 || a.LatP95Maks == nil || *a.LatP95Maks != 800 {
		t.Errorf("jumlah = tunggu %d req %d 5xx %d p95 %v", a.DBTungguTotal, a.ReqTotal, a.Req5xx, a.LatP95Maks)
	}
	if e := AgregatDari(nil, t0); e.Jumlah != 0 || e.CPURata != nil || e.MemPersenMaks != nil {
		t.Errorf("kosong = %+v", e)
	}
}

func TestLajuPertumbuhanPerHari(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	var w []time.Time
	var v []float64
	for i := 0; i < 30; i++ { // 30 titik tiap 6 jam = 7,25 hari; tumbuh 10 per hari
		w = append(w, t0.Add(time.Duration(i)*6*time.Hour))
		v = append(v, 100+float64(i)*6/24*10)
	}
	laju, ok := LajuPertumbuhanPerHari(w, v, 12*time.Hour)
	if !ok || laju < 9.5 || laju > 10.5 {
		t.Errorf("laju = %v ok=%v, want sekitar 10", laju, ok)
	}
	// satu titik yang meloncat tidak menentukan hasil
	v2 := append([]float64(nil), v...)
	v2[15] += 5000
	if l2, _ := LajuPertumbuhanPerHari(w, v2, 12*time.Hour); l2 < 9 || l2 > 11 {
		t.Errorf("laju dengan pencilan = %v", l2)
	}
	// menyusut -> negatif; datar -> 0
	for i := range v {
		v[i] = 500 - float64(i)
	}
	if l, ok := LajuPertumbuhanPerHari(w, v, 12*time.Hour); !ok || l >= 0 {
		t.Errorf("menyusut = %v %v", l, ok)
	}
	// terlalu sedikit titik atau terlalu pendek
	if _, ok := LajuPertumbuhanPerHari(w[:4], v[:4], time.Hour); ok {
		t.Error("kurang dari enam titik")
	}
	if _, ok := LajuPertumbuhanPerHari(w[:8], v[:8], 100*time.Hour); ok {
		t.Error("rentang lebih pendek dari minimal")
	}
	if _, ok := LajuPertumbuhanPerHari(w, v[:5], time.Hour); ok {
		t.Error("panjang tidak sama")
	}
}

func TestTurunkanMengelompokkanDenganRataDanPuncak(t *testing.T) {
	t0 := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC) // kelipatan 30 menit
	var ss []Snapshot
	for i := 0; i < 12; i++ { // 12 snapshot tiap 5 menit = 1 jam
		ss = append(ss, Snapshot{Waktu: t0.Add(time.Duration(i) * 5 * time.Minute), CPURata: pf(float64(10 * i)), CPUMaks: pf(float64(10*i + 5)), MemTotal: 10 * gb, MemMaks: uint64(i+1) * gb,
			DiskTotal: 100 * gb, DiskTerpakai: uint64(40+i) * gb, GoroutineMaks: 100 + i, DBDipakaiMaks: i, DBTungguDelta: 1, ReqTotal: 100, Req5xx: 1, LatRata: pf(50), LatP95: pf(float64(100 + i)),
			DBUkuranMB: pf(float64(1000 + i))})
	}
	titik := Turunkan(ss, 30)
	if len(titik) != 2 {
		t.Fatalf("titik = %d, want 2", len(titik))
	}
	a, b := titik[0], titik[1]
	if *a.CPURata != 25 || *a.CPUMaks != 55 || *b.CPURata != 85 || *b.CPUMaks != 115 { // rata [0..50]=25, [60..110]=85
		t.Errorf("cpu = %v/%v dan %v/%v", *a.CPURata, *a.CPUMaks, *b.CPURata, *b.CPUMaks)
	}
	if *a.MemPersen != 60 || *b.MemPersen != 120 {
		t.Errorf("memori puncak = %v dan %v", *a.MemPersen, *b.MemPersen)
	}
	if *a.Goroutine != 105 || *a.DBDipakai != 5 || *a.DBTunggu != 6 || *a.ReqTotal != 600 || *a.Req5xx != 6 {
		t.Errorf("proses/db/req = %v %v %v %v %v", *a.Goroutine, *a.DBDipakai, *a.DBTunggu, *a.ReqTotal, *a.Req5xx)
	}
	if *a.LatRataMS != 50 || *a.LatP95MS != 105 || *a.DBUkuranMB != 1005 || *a.DiskPersen != 45 {
		t.Errorf("latensi/ukuran/disk = %v %v %v %v", *a.LatRataMS, *a.LatP95MS, *a.DBUkuranMB, *a.DiskPersen)
	}
	if !a.Waktu.Before(b.Waktu) {
		t.Error("titik harus berurutan menurut waktu")
	}
	if got := Turunkan(ss, 5); len(got) != 12 {
		t.Errorf("tanpa penurunan resolusi = %d titik", len(got))
	}
	if got := Turunkan(nil, 30); len(got) != 0 {
		t.Errorf("kosong = %d titik", len(got))
	}
}

func TestRingkasRiwayatMembagiRentang24JamDan7Hari(t *testing.T) {
	sekarang := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	var ss []Snapshot
	for i := 0; i < 7*24*4; i++ { // tiap 15 menit selama 7 hari; hari-hari lama CPU puncak 90, 24 jam terakhir 40
		w := sekarang.Add(-time.Duration(7*24*4-i) * 15 * time.Minute)
		cpu := 90.0
		if sekarang.Sub(w) <= 24*time.Hour {
			cpu = 40
		}
		ss = append(ss, Snapshot{Waktu: w, CPURata: pf(cpu / 2), CPUMaks: pf(cpu), DiskTotal: 100 * gb, DiskTerpakai: 50*gb + uint64(i)*(gb/100), DBUkuranMB: pf(1000 + float64(i)/4)})
	}
	h24, h7, lajuDisk, lajuDB := ringkasRiwayat(ss, sekarang)
	if *h24.CPUMaks != 40 || *h7.CPUMaks != 90 {
		t.Errorf("puncak 24 jam %v dan 7 hari %v, want 40 dan 90", *h24.CPUMaks, *h7.CPUMaks)
	}
	if h24.Jumlah != 96 || h7.Jumlah != len(ss) {
		t.Errorf("jumlah snapshot 24 jam %d, 7 hari %d", h24.Jumlah, h7.Jumlah)
	}
	if lajuDisk == nil || *lajuDisk <= 0 {
		t.Errorf("laju disk = %v", lajuDisk)
	}
	if lajuDB == nil || *lajuDB < 23 || *lajuDB > 25 { // +0,25 MB tiap 15 menit = 24 MB per hari
		t.Errorf("laju DB = %v, want sekitar 24 MB per hari", lajuDB)
	}
}
