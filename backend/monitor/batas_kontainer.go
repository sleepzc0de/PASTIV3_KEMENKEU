package monitor

// BatasMemoriKontainer mengembalikan batas memori cgroup container (byte) bila container dibatasi (mem_limit di Docker); false bila tanpa batas atau bukan Linux.
// Dipakai main untuk memasang batas lunak memori Go sedikit di bawahnya.
func BatasMemoriKontainer() (uint64, bool) {
	k := bacaKontainerOS()
	if k == nil || k.MemBatas == nil || *k.MemBatas == 0 {
		return 0, false
	}
	return *k.MemBatas, true
}
