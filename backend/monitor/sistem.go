package monitor

import (
	"runtime"
	"sync"
	"time"
)

// Pengukuran server. Pembacaan khusus platform ada di sistem_linux.go, sistem_windows.go, dan sistem_lain.go; di sini hanya pengolahan bersama.

// ProsesOS: keadaan proses yang hanya bisa dibaca dari OS (nil bila tidak tersedia di platform ini).
type ProsesOS struct {
	RSS      *uint64  // memori fisik proses (byte)
	FD       *int     // berkas/soket yang terbuka
	CPUDetik *float64 // waktu CPU proses (user + sistem) kumulatif
}

// pengukurCPU menghitung pemakaian CPU dari selisih penghitung kumulatif antara dua pengukuran.
type pengukurCPU struct {
	mu          sync.Mutex
	idle, total uint64
	ada         bool
	waktuProses time.Time
	cpuProses   float64
	adaProses   bool
}

// ukur mengembalikan pemakaian CPU server (0-100) dan proses (persen dari satu core) sejak pemanggilan sebelumnya; nil pada pemanggilan pertama atau bila tidak dapat dibaca.
func (p *pengukurCPU) ukur(sekarang time.Time) (server *float64, proses *float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if idle, total, ok := bacaCPUMentah(); ok {
		if p.ada && total > p.total {
			dTotal, dIdle := float64(total-p.total), float64(idle-p.idle)
			if idle < p.idle {
				dIdle = 0
			}
			v := (1 - dIdle/dTotal) * 100
			if v < 0 {
				v = 0
			}
			if v > 100 {
				v = 100
			}
			server = &v
		}
		p.idle, p.total, p.ada = idle, total, true
	}
	if po := bacaProsesOS(); po.CPUDetik != nil {
		if p.adaProses && sekarang.After(p.waktuProses) {
			dt := sekarang.Sub(p.waktuProses).Seconds()
			if dt > 0 && *po.CPUDetik >= p.cpuProses {
				v := (*po.CPUDetik - p.cpuProses) / dt * 100
				proses = &v
			}
		}
		p.waktuProses, p.cpuProses, p.adaProses = sekarang, *po.CPUDetik, true
	}
	return server, proses
}

func persen(bagian, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(bagian) / float64(total) * 100
}

// ukurSistem mengukur keadaan server saat ini.
func ukurSistem(p *pengukurCPU, sekarang time.Time) Sistem {
	s := Sistem{OS: runtime.GOOS, Arsitektur: runtime.GOARCH, CPUJumlah: runtime.NumCPU()}
	if p != nil { // nil: pemanggil mengisi CPU dari pengukuran berkala terakhir
		s.CPUPersen, s.CPUProsesPersen = p.ukur(sekarang)
	}
	if _, _, ok := bacaCPUMentah(); !ok {
		s.Catatan = append(s.Catatan, "Pemakaian CPU server tidak dapat dibaca di platform ini.")
	}
	if m, ok := bacaMemoriOS(); ok {
		s.MemTotal, s.MemTersedia = m.Total, m.Tersedia
		s.MemTerpakai = m.Total - m.Tersedia
		s.MemPersen = persen(s.MemTerpakai, s.MemTotal)
		s.SwapTotal = m.SwapTotal
		if m.SwapTotal >= m.SwapBebas {
			s.SwapTerpakai = m.SwapTotal - m.SwapBebas
		}
	} else {
		s.Catatan = append(s.Catatan, "Memori server tidak dapat dibaca di platform ini.")
	}
	if l1, l5, l15, ok := bacaLoadOS(); ok {
		s.Load1, s.Load5, s.Load15 = &l1, &l5, &l15
	}
	s.DiskJalur = jalurDisk()
	if total, bebas, ok := bacaDiskOS(s.DiskJalur); ok && total > 0 {
		s.DiskTotal, s.DiskBebas = total, bebas
		if total >= bebas {
			s.DiskTerpakai = total - bebas
		}
		s.DiskPersen = persen(s.DiskTerpakai, s.DiskTotal)
	} else {
		s.Catatan = append(s.Catatan, "Ruang disk tidak dapat dibaca di platform ini.")
	}
	if u, ok := bacaUptimeOS(); ok {
		s.UptimeDetik = &u
	}
	s.Kontainer = bacaKontainerOS()
	return s
}

var waktuMulaiProses = time.Now()

// ukurProses mengukur keadaan proses aplikasi.
func ukurProses() Proses {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	po := bacaProsesOS()
	pr := Proses{
		VersiGo: runtime.Version(), UptimeDetik: int64(time.Since(waktuMulaiProses).Seconds()), Goroutine: runtime.NumGoroutine(),
		HeapDipakai: ms.HeapAlloc, HeapDariOS: ms.HeapSys, MemDariOS: ms.Sys, RSS: po.RSS, GCJumlah: ms.NumGC, FDTerbuka: po.FD,
	}
	if ms.NumGC > 0 {
		pr.GCJedaTerakhir = float64(ms.PauseNs[(ms.NumGC+255)%256]) / 1e6
	}
	return pr
}
