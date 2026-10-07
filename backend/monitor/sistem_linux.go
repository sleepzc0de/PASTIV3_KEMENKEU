//go:build linux

package monitor

import (
	"os"
	"strings"
	"syscall"
)

// Pembacaan Linux: /proc untuk server dan proses, cgroup untuk batas container, statfs untuk disk. Di container, /proc menunjukkan keadaan server induknya (bukan hanya
// container), yang memang menentukan apakah server perlu ditambah.

var (
	jalurProc   = "/proc"
	jalurCgroup = "/sys/fs/cgroup"
)

func bacaBerkas(jalur string) (string, bool) {
	b, err := os.ReadFile(jalur)
	if err != nil {
		return "", false
	}
	return string(b), true
}

func bacaCPUMentah() (idle, total uint64, ok bool) {
	s, ada := bacaBerkas(jalurProc + "/stat")
	if !ada {
		return 0, 0, false
	}
	return ParseProcStatCPU(s)
}

func bacaMemoriOS() (Meminfo, bool) {
	s, ada := bacaBerkas(jalurProc + "/meminfo")
	if !ada {
		return Meminfo{}, false
	}
	return ParseMeminfo(s)
}

func bacaLoadOS() (float64, float64, float64, bool) {
	s, ada := bacaBerkas(jalurProc + "/loadavg")
	if !ada {
		return 0, 0, 0, false
	}
	return ParseLoadavg(s)
}

func jalurDisk() string { return "/" }

func bacaDiskOS(jalur string) (total, bebas uint64, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(jalur, &st); err != nil {
		return 0, 0, false
	}
	ukuran := uint64(st.Bsize)
	return uint64(st.Blocks) * ukuran, uint64(st.Bavail) * ukuran, true
}

func bacaUptimeOS() (int64, bool) {
	s, ada := bacaBerkas(jalurProc + "/uptime")
	if !ada {
		return 0, false
	}
	v, ok := ParseUptime(s)
	return int64(v), ok
}

// tikPerDetik: satuan waktu CPU pada /proc/self/stat (USER_HZ), 100 pada hampir semua Linux.
const tikPerDetik = 100.0

func bacaProsesOS() ProsesOS {
	var p ProsesOS
	if s, ok := bacaBerkas(jalurProc + "/self/status"); ok {
		if v, ok := ParseStatusKB(s, "VmRSS"); ok {
			p.RSS = &v
		}
	}
	if s, ok := bacaBerkas(jalurProc + "/self/stat"); ok {
		if u, st, ok := ParseProcSelfStat(s); ok {
			d := float64(u+st) / tikPerDetik
			p.CPUDetik = &d
		}
	}
	if entri, err := os.ReadDir(jalurProc + "/self/fd"); err == nil {
		n := len(entri)
		p.FD = &n
	}
	return p
}

// bacaKontainerOS membaca batas cgroup (v2 lalu v1). Nil bila tidak ada batas apa pun yang dipasang.
func bacaKontainerOS() *Kontainer {
	k := &Kontainer{}
	ada := false
	if s, ok := bacaBerkas(jalurCgroup + "/memory.max"); ok { // cgroup v2
		if v, tanpa, ok := ParseCgroupBytes(s); ok && !tanpa {
			k.MemBatas, ada = &v, true
		}
		if s, ok := bacaBerkas(jalurCgroup + "/memory.current"); ok {
			if v, _, ok := ParseCgroupBytes(strings.TrimSpace(s)); ok {
				k.MemPakai = &v
			}
		}
	} else if s, ok := bacaBerkas(jalurCgroup + "/memory/memory.limit_in_bytes"); ok { // cgroup v1
		if v, tanpa, ok := ParseCgroupBytes(s); ok && !tanpa {
			k.MemBatas, ada = &v, true
		}
		if s, ok := bacaBerkas(jalurCgroup + "/memory/memory.usage_in_bytes"); ok {
			if v, _, ok := ParseCgroupBytes(s); ok {
				k.MemPakai = &v
			}
		}
	}
	if s, ok := bacaBerkas(jalurCgroup + "/cpu.max"); ok {
		if c, tanpa, ok := ParseCPUMax(s); ok && !tanpa {
			k.CPUBatas, ada = &c, true
		}
	}
	if !ada {
		return nil
	}
	return k
}
