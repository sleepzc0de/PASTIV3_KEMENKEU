// Paket monitor mengukur pemakaian resource (server, proses aplikasi, kinerja permintaan HTTP, dan database), menyimpan riwayatnya (tabel monitor_snapshot, migrasi
// 058), dan menilai resource mana yang cukup, perlu diperhatikan, atau perlu ditambah. Khusus dibaca superadmin (lihat handlers/monitor_handler.go).
package monitor

import (
	"strconv"
	"strings"
)

// Pembaca format berkas /proc dan cgroup Linux. Dipisah dari pembacaan berkasnya supaya bisa diuji di OS mana pun.

// Meminfo: isi /proc/meminfo dalam byte.
type Meminfo struct {
	Total, Tersedia, Bebas, Buffers, Cache, SwapTotal, SwapBebas uint64
}

func kbKeByte(s string) (uint64, bool) {
	f := strings.Fields(s)
	if len(f) == 0 {
		return 0, false
	}
	n, err := strconv.ParseUint(f[0], 10, 64)
	if err != nil {
		return 0, false
	}
	return n * 1024, true
}

// ParseMeminfo membaca /proc/meminfo; ok=false bila MemTotal tidak ada. Bila MemAvailable tidak ada (kernel lama), diperkirakan dari MemFree + Buffers + Cached.
func ParseMeminfo(s string) (Meminfo, bool) {
	var m Meminfo
	var adaTersedia bool
	for _, baris := range strings.Split(s, "\n") {
		i := strings.IndexByte(baris, ':')
		if i < 0 {
			continue
		}
		nilai, ok := kbKeByte(baris[i+1:])
		if !ok {
			continue
		}
		switch baris[:i] {
		case "MemTotal":
			m.Total = nilai
		case "MemAvailable":
			m.Tersedia, adaTersedia = nilai, true
		case "MemFree":
			m.Bebas = nilai
		case "Buffers":
			m.Buffers = nilai
		case "Cached":
			m.Cache = nilai
		case "SwapTotal":
			m.SwapTotal = nilai
		case "SwapFree":
			m.SwapBebas = nilai
		}
	}
	if m.Total == 0 {
		return m, false
	}
	if !adaTersedia {
		m.Tersedia = m.Bebas + m.Buffers + m.Cache
	}
	if m.Tersedia > m.Total {
		m.Tersedia = m.Total
	}
	return m, true
}

// ParseLoadavg membaca /proc/loadavg ("0.52 0.58 0.59 1/467 12345").
func ParseLoadavg(s string) (l1, l5, l15 float64, ok bool) {
	f := strings.Fields(s)
	if len(f) < 3 {
		return 0, 0, 0, false
	}
	var err error
	if l1, err = strconv.ParseFloat(f[0], 64); err != nil {
		return 0, 0, 0, false
	}
	if l5, err = strconv.ParseFloat(f[1], 64); err != nil {
		return 0, 0, 0, false
	}
	if l15, err = strconv.ParseFloat(f[2], 64); err != nil {
		return 0, 0, 0, false
	}
	return l1, l5, l15, true
}

// ParseProcStatCPU membaca baris "cpu" pertama pada /proc/stat: idle (idle + iowait) dan total (user nice system idle iowait irq softirq steal; guest sudah termasuk di user).
func ParseProcStatCPU(s string) (idle, total uint64, ok bool) {
	for _, baris := range strings.Split(s, "\n") {
		f := strings.Fields(baris)
		if len(f) < 5 || f[0] != "cpu" {
			continue
		}
		var n [8]uint64
		for i := 0; i < 8 && i+1 < len(f); i++ {
			v, err := strconv.ParseUint(f[i+1], 10, 64)
			if err != nil {
				return 0, 0, false
			}
			n[i] = v
		}
		for _, v := range n {
			total += v
		}
		return n[3] + n[4], total, true
	}
	return 0, 0, false
}

// ParseUptime membaca /proc/uptime (detik sejak server menyala).
func ParseUptime(s string) (float64, bool) {
	f := strings.Fields(s)
	if len(f) < 1 {
		return 0, false
	}
	v, err := strconv.ParseFloat(f[0], 64)
	return v, err == nil
}

// ParseStatusKB membaca satu baris "Kunci:   1234 kB" dari /proc/self/status dalam byte.
func ParseStatusKB(s, kunci string) (uint64, bool) {
	for _, baris := range strings.Split(s, "\n") {
		if strings.HasPrefix(baris, kunci+":") {
			return kbKeByte(baris[len(kunci)+1:])
		}
	}
	return 0, false
}

// ParseProcSelfStat membaca utime dan stime (satuan tik jam, biasanya 100 per detik) dari /proc/self/stat. Nama proses di dalam tanda kurung boleh memuat spasi.
func ParseProcSelfStat(s string) (utime, stime uint64, ok bool) {
	i := strings.LastIndexByte(s, ')')
	if i < 0 {
		return 0, 0, false
	}
	f := strings.Fields(s[i+1:])
	// setelah ")" : state(0) ppid(1) ... utime(11) stime(12)
	if len(f) < 13 {
		return 0, 0, false
	}
	u, err1 := strconv.ParseUint(f[11], 10, 64)
	st, err2 := strconv.ParseUint(f[12], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return u, st, true
}

// ParseCgroupBytes membaca nilai cgroup berupa angka byte atau "max" (tanpa batas); ok=false bila isinya tidak terbaca.
func ParseCgroupBytes(s string) (nilai uint64, tanpaBatas bool, ok bool) {
	t := strings.TrimSpace(s)
	if t == "max" {
		return 0, true, true
	}
	v, err := strconv.ParseUint(t, 10, 64)
	if err != nil {
		return 0, false, false
	}
	// cgroup v1 memakai angka sangat besar (mendekati 2^63) untuk "tanpa batas"
	if v >= 1<<60 {
		return 0, true, true
	}
	return v, false, true
}

// ParseCPUMax membaca cgroup v2 cpu.max ("max 100000" atau "200000 100000") menjadi jumlah core yang boleh dipakai; tanpaBatas bila "max".
func ParseCPUMax(s string) (core float64, tanpaBatas bool, ok bool) {
	f := strings.Fields(s)
	if len(f) < 2 {
		return 0, false, false
	}
	if f[0] == "max" {
		return 0, true, true
	}
	kuota, err1 := strconv.ParseFloat(f[0], 64)
	periode, err2 := strconv.ParseFloat(f[1], 64)
	if err1 != nil || err2 != nil || periode <= 0 {
		return 0, false, false
	}
	return kuota / periode, false, true
}
