//go:build !linux && !windows

package monitor

// Platform selain Linux dan Windows (mis. macOS saat pengembangan): hanya data dari runtime Go yang tersedia.

func bacaCPUMentah() (uint64, uint64, bool)         { return 0, 0, false }
func bacaMemoriOS() (Meminfo, bool)                 { return Meminfo{}, false }
func bacaLoadOS() (float64, float64, float64, bool) { return 0, 0, 0, false }
func jalurDisk() string                             { return "/" }
func bacaDiskOS(string) (uint64, uint64, bool)      { return 0, 0, false }
func bacaUptimeOS() (int64, bool)                   { return 0, false }
func bacaProsesOS() ProsesOS                        { return ProsesOS{} }
func bacaKontainerOS() *Kontainer                   { return nil }
