//go:build windows

package monitor

import (
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Pembacaan Windows (lingkungan pengembangan): memori, CPU, dan disk lewat API sistem. Load average dan batas container tidak ada di Windows.

// x/sys/windows tidak menyediakan GetSystemTimes dan GlobalMemoryStatusEx, jadi dipanggil langsung dari kernel32.
var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGetSystemTimes       = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func filetimeKeUint(f windows.Filetime) uint64 {
	return uint64(f.HighDateTime)<<32 | uint64(f.LowDateTime)
}

func bacaCPUMentah() (idle, total uint64, ok bool) {
	var i, k, u windows.Filetime
	if r1, _, _ := procGetSystemTimes.Call(uintptr(unsafe.Pointer(&i)), uintptr(unsafe.Pointer(&k)), uintptr(unsafe.Pointer(&u))); r1 == 0 {
		return 0, 0, false
	}
	// kernel sudah mencakup waktu idle
	return filetimeKeUint(i), filetimeKeUint(k) + filetimeKeUint(u), true
}

func bacaMemoriOS() (Meminfo, bool) {
	var m memoryStatusEx
	m.Length = uint32(unsafe.Sizeof(m))
	if r1, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m))); r1 == 0 {
		return Meminfo{}, false
	}
	// Swap tidak dilaporkan di Windows: selisih TotalPageFile dan memori fisik adalah batas commit, bukan jumlah halaman yang benar-benar ditukar ke disk, sehingga
	// memakainya sebagai "swap terpakai" memicu peringatan palsu. Server produksi memakai Linux, yang melaporkan swap sesungguhnya.
	return Meminfo{Total: m.TotalPhys, Tersedia: m.AvailPhys, Bebas: m.AvailPhys}, m.TotalPhys > 0
}

func bacaLoadOS() (float64, float64, float64, bool) { return 0, 0, 0, false }

func jalurDisk() string {
	if wd, err := os.Getwd(); err == nil {
		if v := filepath.VolumeName(wd); v != "" {
			return v + `\`
		}
	}
	return `C:\`
}

func bacaDiskOS(jalur string) (total, bebas uint64, ok bool) {
	p, err := windows.UTF16PtrFromString(jalur)
	if err != nil {
		return 0, 0, false
	}
	var tersediaUntukPemanggil, totalByte, bebasByte uint64
	if err := windows.GetDiskFreeSpaceEx(p, &tersediaUntukPemanggil, &totalByte, &bebasByte); err != nil {
		return 0, 0, false
	}
	return totalByte, tersediaUntukPemanggil, true
}

func bacaUptimeOS() (int64, bool) {
	return int64(windows.DurationSinceBoot().Seconds()), true
}

func bacaProsesOS() ProsesOS {
	var p ProsesOS
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(windows.CurrentProcess(), &creation, &exit, &kernel, &user); err == nil {
		d := float64(filetimeKeUint(kernel)+filetimeKeUint(user)) / 1e7 // satuan 100 nanodetik
		p.CPUDetik = &d
	}
	return p
}

func bacaKontainerOS() *Kontainer { return nil }
