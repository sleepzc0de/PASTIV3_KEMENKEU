package monitor

import "testing"

const meminfoContoh = `MemTotal:        8143496 kB
MemFree:          812340 kB
MemAvailable:    5021600 kB
Buffers:          210000 kB
Cached:          3500000 kB
SwapCached:            0 kB
SwapTotal:       2097148 kB
SwapFree:        2000000 kB
Dirty:               120 kB
`

func TestParseMeminfo(t *testing.T) {
	m, ok := ParseMeminfo(meminfoContoh)
	if !ok {
		t.Fatal("meminfo tidak terbaca")
	}
	if m.Total != 8143496*1024 || m.Tersedia != 5021600*1024 || m.Bebas != 812340*1024 || m.SwapTotal != 2097148*1024 || m.SwapBebas != 2000000*1024 {
		t.Errorf("meminfo = %+v", m)
	}
	// Kernel lama tanpa MemAvailable: diperkirakan dari MemFree + Buffers + Cached.
	lama := "MemTotal: 1000 kB\nMemFree: 100 kB\nBuffers: 50 kB\nCached: 150 kB\n"
	m, ok = ParseMeminfo(lama)
	if !ok || m.Tersedia != 300*1024 {
		t.Errorf("perkiraan MemAvailable = %+v ok=%v", m, ok)
	}
	// MemAvailable tidak boleh melebihi total
	m, _ = ParseMeminfo("MemTotal: 100 kB\nMemAvailable: 900 kB\n")
	if m.Tersedia != m.Total {
		t.Errorf("tersedia > total: %+v", m)
	}
	for _, buruk := range []string{"", "bukan meminfo", "MemTotal: abc kB\n"} {
		if _, ok := ParseMeminfo(buruk); ok {
			t.Errorf("%q seharusnya tidak terbaca", buruk)
		}
	}
}

func TestParseLoadavg(t *testing.T) {
	l1, l5, l15, ok := ParseLoadavg("0.52 1.58 2.59 1/467 12345\n")
	if !ok || l1 != 0.52 || l5 != 1.58 || l15 != 2.59 {
		t.Errorf("loadavg = %v %v %v %v", l1, l5, l15, ok)
	}
	for _, buruk := range []string{"", "0.5 0.4", "a b c d"} {
		if _, _, _, ok := ParseLoadavg(buruk); ok {
			t.Errorf("%q seharusnya tidak terbaca", buruk)
		}
	}
}

func TestParseProcStatCPU(t *testing.T) {
	s := "cpu  100 20 30 400 50 6 7 8 9 10\ncpu0 50 10 15 200 25 3 3 4 0 0\nintr 12345\n"
	idle, total, ok := ParseProcStatCPU(s)
	// idle = idle(400) + iowait(50); total = 100+20+30+400+50+6+7+8 (guest tidak dihitung dua kali)
	if !ok || idle != 450 || total != 621 {
		t.Errorf("idle=%d total=%d ok=%v, want 450 621", idle, total, ok)
	}
	for _, buruk := range []string{"", "cpu0 1 2 3 4 5 6 7 8\n", "cpu  1 2 3\n", "cpu  a b c d e\n"} {
		if _, _, ok := ParseProcStatCPU(buruk); ok {
			t.Errorf("%q seharusnya tidak terbaca", buruk)
		}
	}
	// kernel lama dengan kurang dari 8 kolom tetap terbaca
	if idle, total, ok := ParseProcStatCPU("cpu  10 0 5 80 5\n"); !ok || idle != 85 || total != 100 {
		t.Errorf("kolom pendek: %d %d %v", idle, total, ok)
	}
}

func TestParseUptimeDanStatus(t *testing.T) {
	if v, ok := ParseUptime("350735.47 234388.90\n"); !ok || v != 350735.47 {
		t.Errorf("uptime = %v %v", v, ok)
	}
	if _, ok := ParseUptime(""); ok {
		t.Error("uptime kosong")
	}
	status := "Name:\tpasti\nVmPeak:\t  900000 kB\nVmRSS:\t   45678 kB\nThreads:\t12\n"
	if v, ok := ParseStatusKB(status, "VmRSS"); !ok || v != 45678*1024 {
		t.Errorf("VmRSS = %v %v", v, ok)
	}
	if _, ok := ParseStatusKB(status, "VmSwap"); ok {
		t.Error("kunci yang tidak ada")
	}
}

func TestParseProcSelfStat(t *testing.T) {
	// nama proses memuat spasi dan tanda kurung; utime dan stime = kolom ke-14 dan ke-15
	s := "1234 (pasti (backend) x) S 1 1234 1234 0 -1 4194560 100 0 0 0 250 75 0 0 20 0 8 0 100 1000000 500 18446744073709551615"
	u, st, ok := ParseProcSelfStat(s)
	if !ok || u != 250 || st != 75 {
		t.Errorf("utime=%d stime=%d ok=%v", u, st, ok)
	}
	for _, buruk := range []string{"", "1234 pasti S 1", "1 (x) S 1 2 3"} {
		if _, _, ok := ParseProcSelfStat(buruk); ok {
			t.Errorf("%q seharusnya tidak terbaca", buruk)
		}
	}
}

func TestParseCgroup(t *testing.T) {
	if v, tanpa, ok := ParseCgroupBytes("536870912\n"); !ok || tanpa || v != 536870912 {
		t.Errorf("batas 512MB = %v %v %v", v, tanpa, ok)
	}
	if _, tanpa, ok := ParseCgroupBytes("max\n"); !ok || !tanpa {
		t.Error("max = tanpa batas")
	}
	// cgroup v1: angka sangat besar berarti tanpa batas
	if _, tanpa, ok := ParseCgroupBytes("9223372036854771712"); !ok || !tanpa {
		t.Error("angka besar cgroup v1 = tanpa batas")
	}
	if _, _, ok := ParseCgroupBytes("abc"); ok {
		t.Error("teks bukan angka")
	}

	if c, tanpa, ok := ParseCPUMax("200000 100000\n"); !ok || tanpa || c != 2 {
		t.Errorf("cpu.max 2 core = %v %v %v", c, tanpa, ok)
	}
	if c, tanpa, ok := ParseCPUMax("50000 100000"); !ok || tanpa || c != 0.5 {
		t.Errorf("cpu.max setengah core = %v %v %v", c, tanpa, ok)
	}
	if _, tanpa, ok := ParseCPUMax("max 100000"); !ok || !tanpa {
		t.Error("cpu.max max = tanpa batas")
	}
	for _, buruk := range []string{"", "max", "100000 0", "a b"} {
		if _, _, ok := ParseCPUMax(buruk); ok {
			t.Errorf("%q seharusnya tidak terbaca", buruk)
		}
	}
}
