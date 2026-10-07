package monitor

import (
	"context"
	"database/sql"
	"sort"
	"time"
)

// Riwayat pengukuran: pengukur berkala (tiap 30 detik) menyimpan satu Snapshot tiap 5 menit ke tabel monitor_snapshot (rata-rata dan puncak pada jendela itu).

// sampel: satu pengukuran sesaat oleh pengukur berkala.
type sampel struct {
	Waktu                               time.Time
	CPU, CPUProses, Load1               *float64
	MemTotal, MemTerpakai, SwapTerpakai uint64
	DiskTotal, DiskTerpakai, RSS, Heap  uint64
	Goroutine, DBBuka, DBDipakai        int
	DBBatas                             int
	DBMenunggu                          int64 // kumulatif sejak proses mulai
	Req                                 hitungan
	BerjalanMaks                        int64
}

// Snapshot: ringkasan satu jendela (baris monitor_snapshot).
type Snapshot struct {
	Waktu                                time.Time
	Jendela                              int // detik
	CPURata, CPUMaks, Load1Maks          *float64
	MemTotal, MemRata, MemMaks, SwapMaks uint64
	DiskTotal, DiskTerpakai              uint64
	RSSMaks, HeapMaks                    uint64
	GoroutineMaks                        int
	DBBukaMaks, DBDipakaiMaks, DBBatas   int
	DBTungguDelta                        int64
	DBUkuranMB, DBTerpakaiMB             *float64
	ReqTotal, Req4xx, Req5xx             int64
	LatRata, LatP95                      *float64
	ReqBerjalanMaks                      int
}

func ptr(v float64) *float64 { return &v }

func maksU(a, b uint64) uint64 {
	if b > a {
		return b
	}
	return a
}

func maksI(a, b int) int {
	if b > a {
		return b
	}
	return a
}

// bangunSnapshot meringkas sampel-sampel sebuah jendela. tungguAwal: nilai kumulatif DBMenunggu sebelum sampel pertama.
func bangunSnapshot(ss []sampel, waktu time.Time, tungguAwal int64, ukuranMB, terpakaiMB *float64) Snapshot {
	snap := Snapshot{Waktu: waktu.UTC(), DBUkuranMB: ukuranMB, DBTerpakaiMB: terpakaiMB}
	if len(ss) == 0 {
		return snap
	}
	snap.Jendela = int(ss[len(ss)-1].Waktu.Sub(ss[0].Waktu).Seconds()) + 30
	var sumCPU, sumMem float64
	var nCPU int
	var req hitungan
	for _, s := range ss {
		if s.CPU != nil {
			sumCPU += *s.CPU
			nCPU++
			if snap.CPUMaks == nil || *s.CPU > *snap.CPUMaks {
				snap.CPUMaks = ptr(*s.CPU)
			}
		}
		if s.Load1 != nil && (snap.Load1Maks == nil || *s.Load1 > *snap.Load1Maks) {
			snap.Load1Maks = ptr(*s.Load1)
		}
		if s.MemTotal > 0 {
			snap.MemTotal = s.MemTotal
		}
		sumMem += float64(s.MemTerpakai)
		snap.MemMaks = maksU(snap.MemMaks, s.MemTerpakai)
		snap.SwapMaks = maksU(snap.SwapMaks, s.SwapTerpakai)
		snap.DiskTotal, snap.DiskTerpakai = s.DiskTotal, s.DiskTerpakai
		snap.RSSMaks = maksU(snap.RSSMaks, s.RSS)
		snap.HeapMaks = maksU(snap.HeapMaks, s.Heap)
		snap.GoroutineMaks = maksI(snap.GoroutineMaks, s.Goroutine)
		snap.DBBukaMaks = maksI(snap.DBBukaMaks, s.DBBuka)
		snap.DBDipakaiMaks = maksI(snap.DBDipakaiMaks, s.DBDipakai)
		snap.DBBatas = s.DBBatas
		snap.ReqBerjalanMaks = maksI(snap.ReqBerjalanMaks, int(s.BerjalanMaks))
		req.tambahkan(s.Req)
	}
	if nCPU > 0 {
		snap.CPURata = ptr(sumCPU / float64(nCPU))
	}
	snap.MemRata = uint64(sumMem / float64(len(ss)))
	if last := ss[len(ss)-1].DBMenunggu; last >= tungguAwal {
		snap.DBTungguDelta = last - tungguAwal
	}
	snap.ReqTotal, snap.Req4xx, snap.Req5xx = req.Total, req.C4xx, req.C5xx
	if req.NR > 0 {
		snap.LatRata = ptr(req.SumR / float64(req.NR))
		snap.LatP95 = ptr(persentil(req.HistR, 0.95))
	}
	return snap
}

// ---------------------------------------------------------------- penyimpanan

// Store membaca dan menulis tabel monitor_snapshot.
type Store struct{ DB *sql.DB }

func nilaiNol(v uint64) interface{} {
	if v == 0 {
		return nil
	}
	return int64(v)
}

func nilaiPtr(p *float64) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

// Simpan menulis satu snapshot.
func (s *Store) Simpan(ctx context.Context, n Snapshot) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO monitor_snapshot (waktu, jendela_detik, cpu_rata, cpu_maks, load1_maks, mem_total, mem_terpakai_rata, mem_terpakai_maks, swap_terpakai_maks,
			disk_total, disk_terpakai, proses_rss_maks, heap_maks, goroutine_maks, db_buka_maks, db_dipakai_maks, db_batas_koneksi, db_tunggu_delta, db_ukuran_mb, db_terpakai_mb,
			req_total, req_4xx, req_5xx, lat_rata_ms, lat_p95_ms, req_berjalan_maks)
		VALUES (CAST(@p1 AS DATETIME2(0)), @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9, @p10, @p11, @p12, @p13, @p14, @p15, @p16, @p17, @p18, @p19, @p20, @p21, @p22, @p23, @p24, @p25, @p26)`,
		n.Waktu.UTC(), n.Jendela, nilaiPtr(n.CPURata), nilaiPtr(n.CPUMaks), nilaiPtr(n.Load1Maks), nilaiNol(n.MemTotal), nilaiNol(n.MemRata), nilaiNol(n.MemMaks), int64(n.SwapMaks),
		nilaiNol(n.DiskTotal), nilaiNol(n.DiskTerpakai), nilaiNol(n.RSSMaks), nilaiNol(n.HeapMaks), n.GoroutineMaks, n.DBBukaMaks, n.DBDipakaiMaks, n.DBBatas, n.DBTungguDelta,
		nilaiPtr(n.DBUkuranMB), nilaiPtr(n.DBTerpakaiMB), n.ReqTotal, n.Req4xx, n.Req5xx, nilaiPtr(n.LatRata), nilaiPtr(n.LatP95), n.ReqBerjalanMaks)
	return err
}

// Baca membaca snapshot pada [dari, sampai) menurut urutan waktu.
func (s *Store) Baca(ctx context.Context, dari, sampai time.Time) ([]Snapshot, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT waktu, jendela_detik, cpu_rata, cpu_maks, load1_maks, mem_total, mem_terpakai_rata, mem_terpakai_maks, swap_terpakai_maks, disk_total, disk_terpakai,
			proses_rss_maks, heap_maks, goroutine_maks, db_buka_maks, db_dipakai_maks, db_batas_koneksi, db_tunggu_delta, db_ukuran_mb, db_terpakai_mb, req_total, req_4xx, req_5xx,
			lat_rata_ms, lat_p95_ms, req_berjalan_maks
		FROM monitor_snapshot WHERE waktu >= CAST(@p1 AS DATETIME2(0)) AND waktu < CAST(@p2 AS DATETIME2(0)) ORDER BY waktu`, dari.UTC(), sampai.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Snapshot
	for rows.Next() {
		var n Snapshot
		var cpuR, cpuM, load, dbU, dbT, latR, latP sql.NullFloat64
		var memT, memR, memM, swap, dT, dP, rss, heap sql.NullInt64
		var gor, dbB, dbD, dbBt, rb sql.NullInt64
		var tunggu, rt, r4, r5 sql.NullInt64
		if err := rows.Scan(&n.Waktu, &n.Jendela, &cpuR, &cpuM, &load, &memT, &memR, &memM, &swap, &dT, &dP, &rss, &heap, &gor, &dbB, &dbD, &dbBt, &tunggu, &dbU, &dbT, &rt, &r4, &r5,
			&latR, &latP, &rb); err != nil {
			return nil, err
		}
		n.Waktu = n.Waktu.UTC()
		f := func(v sql.NullFloat64) *float64 {
			if v.Valid {
				return ptr(v.Float64)
			}
			return nil
		}
		u := func(v sql.NullInt64) uint64 {
			if v.Valid && v.Int64 > 0 {
				return uint64(v.Int64)
			}
			return 0
		}
		n.CPURata, n.CPUMaks, n.Load1Maks, n.DBUkuranMB, n.DBTerpakaiMB, n.LatRata, n.LatP95 = f(cpuR), f(cpuM), f(load), f(dbU), f(dbT), f(latR), f(latP)
		n.MemTotal, n.MemRata, n.MemMaks, n.SwapMaks, n.DiskTotal, n.DiskTerpakai, n.RSSMaks, n.HeapMaks = u(memT), u(memR), u(memM), u(swap), u(dT), u(dP), u(rss), u(heap)
		n.GoroutineMaks, n.DBBukaMaks, n.DBDipakaiMaks, n.DBBatas, n.ReqBerjalanMaks = int(gor.Int64), int(dbB.Int64), int(dbD.Int64), int(dbBt.Int64), int(rb.Int64)
		n.DBTungguDelta, n.ReqTotal, n.Req4xx, n.Req5xx = tunggu.Int64, rt.Int64, r4.Int64, r5.Int64
		out = append(out, n)
	}
	return out, rows.Err()
}

// HapusSebelum menghapus snapshot yang lebih lama dari batas.
func (s *Store) HapusSebelum(ctx context.Context, batas time.Time) (int64, error) {
	res, err := s.DB.ExecContext(ctx, "DELETE FROM monitor_snapshot WHERE waktu < CAST(@p1 AS DATETIME2(0))", batas.UTC())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ---------------------------------------------------------------- agregasi dan penurunan resolusi

// AgregatDari meringkas sekumpulan snapshot menjadi rata-rata dan puncak.
func AgregatDari(ss []Snapshot, dari time.Time) *Agregat {
	a := &Agregat{Jumlah: len(ss), Dari: dari.UTC()}
	if len(ss) == 0 {
		return a
	}
	var sumCPU float64
	var nCPU int
	for _, s := range ss {
		if s.CPURata != nil {
			sumCPU += *s.CPURata
			nCPU++
		}
		if s.CPUMaks != nil && (a.CPUMaks == nil || *s.CPUMaks > *a.CPUMaks) {
			a.CPUMaks = ptr(*s.CPUMaks)
		}
		if s.Load1Maks != nil && (a.Load1Maks == nil || *s.Load1Maks > *a.Load1Maks) {
			a.Load1Maks = ptr(*s.Load1Maks)
		}
		if s.MemTotal > 0 {
			t := s.MemTotal
			a.MemTotal = &t
			p := float64(s.MemMaks) / float64(s.MemTotal) * 100
			if a.MemPersenMaks == nil || p > *a.MemPersenMaks {
				a.MemPersenMaks = ptr(p)
			}
			if a.MemMaks == nil || s.MemMaks > *a.MemMaks {
				m := s.MemMaks
				a.MemMaks = &m
			}
		}
		if a.SwapMaks == nil || s.SwapMaks > *a.SwapMaks {
			m := s.SwapMaks
			a.SwapMaks = &m
		}
		if s.DiskTotal > 0 {
			p := float64(s.DiskTerpakai) / float64(s.DiskTotal) * 100
			if a.DiskPersenMaks == nil || p > *a.DiskPersenMaks {
				a.DiskPersenMaks = ptr(p)
			}
		}
		if s.RSSMaks > 0 && (a.RSSMaks == nil || s.RSSMaks > *a.RSSMaks) {
			m := s.RSSMaks
			a.RSSMaks = &m
		}
		if a.GoroutineMaks == nil || s.GoroutineMaks > *a.GoroutineMaks {
			m := s.GoroutineMaks
			a.GoroutineMaks = &m
		}
		if a.DBDipakaiMaks == nil || s.DBDipakaiMaks > *a.DBDipakaiMaks {
			m := s.DBDipakaiMaks
			a.DBDipakaiMaks = &m
		}
		if s.DBBatas > 0 {
			b := s.DBBatas
			a.DBBatas = &b
		}
		a.DBTungguTotal += s.DBTungguDelta
		if s.LatP95 != nil && (a.LatP95Maks == nil || *s.LatP95 > *a.LatP95Maks) {
			a.LatP95Maks = ptr(*s.LatP95)
		}
		a.ReqTotal += s.ReqTotal
		a.Req5xx += s.Req5xx
	}
	if nCPU > 0 {
		a.CPURata = ptr(sumCPU / float64(nCPU))
	}
	return a
}

// LajuPertumbuhanPerHari memperkirakan pertambahan nilai per hari: rata-rata sepertiga akhir deret dikurangi rata-rata sepertiga awal, dibagi jarak waktu antara pusat
// keduanya (rata-rata dipakai agar satu titik yang melonjak tidak menentukan hasil). ok=false bila datanya kurang dari enam titik atau rentangnya lebih pendek dari minSpan.
func LajuPertumbuhanPerHari(waktu []time.Time, nilai []float64, minSpan time.Duration) (float64, bool) {
	n := len(nilai)
	if n < 6 || len(waktu) != n || waktu[n-1].Sub(waktu[0]) < minSpan {
		return 0, false
	}
	rata := func(a []float64) float64 {
		var s float64
		for _, v := range a {
			s += v
		}
		return s / float64(len(a))
	}
	pusat := func(a []time.Time) time.Time {
		var s int64
		for _, t := range a {
			s += t.Unix()
		}
		return time.Unix(s/int64(len(a)), 0)
	}
	k := n / 3
	dt := pusat(waktu[n-k:]).Sub(pusat(waktu[:k])).Hours() / 24
	if dt <= 0 {
		return 0, false
	}
	return (rata(nilai[n-k:]) - rata(nilai[:k])) / dt, true
}

// Turunkan mengelompokkan snapshot menjadi titik grafik selebar lebarMenit menit: rata-rata untuk nilai rata-rata, puncak untuk nilai puncak.
func Turunkan(ss []Snapshot, lebarMenit int) []TitikRiwayat {
	if lebarMenit < 1 {
		lebarMenit = 1
	}
	lebar := int64(lebarMenit) * 60
	kelompok := map[int64][]Snapshot{}
	var kunci []int64
	for _, s := range ss {
		k := s.Waktu.Unix() / lebar
		if _, ada := kelompok[k]; !ada {
			kunci = append(kunci, k)
		}
		kelompok[k] = append(kelompok[k], s)
	}
	sort.Slice(kunci, func(i, j int) bool { return kunci[i] < kunci[j] })
	out := make([]TitikRiwayat, 0, len(kunci))
	for _, k := range kunci {
		g := kelompok[k]
		t := TitikRiwayat{Waktu: g[len(g)-1].Waktu}
		var sCPU, sLat, sLatBobot float64
		var nCPU int
		var memMaks, diskMaks, load, rss, heap, gor, dbPakai, dbTunggu, req, r5, p95, dbUkuran float64
		var adaMem, adaDisk, adaLoad, adaRSS, adaHeap, adaDB, adaLat, adaUkuran bool
		var cpuMaks float64
		var adaCPUMaks bool
		for _, s := range g {
			if s.CPURata != nil {
				sCPU += *s.CPURata
				nCPU++
			}
			if s.CPUMaks != nil && (!adaCPUMaks || *s.CPUMaks > cpuMaks) {
				cpuMaks, adaCPUMaks = *s.CPUMaks, true
			}
			if s.MemTotal > 0 {
				if p := float64(s.MemMaks) / float64(s.MemTotal) * 100; !adaMem || p > memMaks {
					memMaks, adaMem = p, true
				}
			}
			if s.DiskTotal > 0 {
				if p := float64(s.DiskTerpakai) / float64(s.DiskTotal) * 100; !adaDisk || p > diskMaks {
					diskMaks, adaDisk = p, true
				}
			}
			if s.Load1Maks != nil && (!adaLoad || *s.Load1Maks > load) {
				load, adaLoad = *s.Load1Maks, true
			}
			if s.RSSMaks > 0 && (!adaRSS || float64(s.RSSMaks) > rss) {
				rss, adaRSS = float64(s.RSSMaks), true
			}
			if s.HeapMaks > 0 && (!adaHeap || float64(s.HeapMaks) > heap) {
				heap, adaHeap = float64(s.HeapMaks), true
			}
			gor = maxF(gor, float64(s.GoroutineMaks))
			dbPakai, adaDB = maxF(dbPakai, float64(s.DBDipakaiMaks)), true
			dbTunggu += float64(s.DBTungguDelta)
			req += float64(s.ReqTotal)
			r5 += float64(s.Req5xx)
			if s.LatRata != nil {
				sLat += *s.LatRata * float64(s.ReqTotal)
				sLatBobot += float64(s.ReqTotal)
			}
			if s.LatP95 != nil && (!adaLat || *s.LatP95 > p95) {
				p95, adaLat = *s.LatP95, true
			}
			if s.DBUkuranMB != nil {
				dbUkuran, adaUkuran = *s.DBUkuranMB, true
			}
		}
		if nCPU > 0 {
			t.CPURata = ptr(sCPU / float64(nCPU))
		}
		if adaCPUMaks {
			t.CPUMaks = ptr(cpuMaks)
		}
		if adaMem {
			t.MemPersen = ptr(memMaks)
		}
		if adaDisk {
			t.DiskPersen = ptr(diskMaks)
		}
		if adaLoad {
			t.Load1 = ptr(load)
		}
		if adaRSS {
			t.RSS = ptr(rss)
		}
		if adaHeap {
			t.Heap = ptr(heap)
		}
		t.Goroutine = ptr(gor)
		if adaDB {
			t.DBDipakai = ptr(dbPakai)
		}
		t.DBTunggu, t.ReqTotal, t.Req5xx = ptr(dbTunggu), ptr(req), ptr(r5)
		if sLatBobot > 0 {
			t.LatRataMS = ptr(sLat / sLatBobot)
		}
		if adaLat {
			t.LatP95MS = ptr(p95)
		}
		if adaUkuran {
			t.DBUkuranMB = ptr(dbUkuran)
		}
		out = append(out, t)
	}
	return out
}

func maxF(a, b float64) float64 {
	if b > a {
		return b
	}
	return a
}
