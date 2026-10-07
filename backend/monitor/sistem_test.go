package monitor

import (
	"testing"
	"time"
)

// Pengukuran nyata di mesin tempat tes berjalan: nilainya tidak bisa ditebak, jadi yang diperiksa adalah kewajaran dan konsistensinya.

func TestUkurSistemWajar(t *testing.T) {
	var p pengukurCPU
	s := ukurSistem(&p, time.Now())
	if s.OS == "" || s.Arsitektur == "" || s.CPUJumlah < 1 {
		t.Errorf("identitas = %+v", s)
	}
	if s.CPUPersen != nil {
		t.Errorf("pengukuran CPU pertama tidak punya pembanding, harus nil: %v", *s.CPUPersen)
	}
	if s.MemTotal == 0 {
		t.Skipf("platform %s tidak menyediakan memori server: %v", s.OS, s.Catatan)
	}
	if s.MemTerpakai > s.MemTotal || s.MemTersedia > s.MemTotal || s.MemTerpakai+s.MemTersedia != s.MemTotal || s.MemPersen < 0 || s.MemPersen > 100 {
		t.Errorf("memori tidak konsisten: %+v", s)
	}
	if s.SwapTerpakai > s.SwapTotal {
		t.Errorf("swap tidak konsisten: %d dari %d", s.SwapTerpakai, s.SwapTotal)
	}
	if s.DiskTotal == 0 || s.DiskTerpakai > s.DiskTotal || s.DiskBebas > s.DiskTotal || s.DiskPersen < 0 || s.DiskPersen > 100 || s.DiskJalur == "" {
		t.Errorf("disk tidak konsisten: %+v", s)
	}
	if s.UptimeDetik != nil && *s.UptimeDetik < 0 {
		t.Errorf("uptime negatif: %d", *s.UptimeDetik)
	}
}

func TestPengukurCPUMenghitungSelisihAntarPengukuran(t *testing.T) {
	var p pengukurCPU
	if _, _, ok := bacaCPUMentah(); !ok {
		t.Skip("platform ini tidak menyediakan penghitung CPU")
	}
	awal := time.Now()
	p.ukur(awal)
	// beri CPU sedikit pekerjaan supaya selisih penghitung pasti bertambah
	sampai := time.Now().Add(300 * time.Millisecond)
	x := 0
	for time.Now().Before(sampai) {
		x++
	}
	_ = x
	server, proses := p.ukur(time.Now())
	if server == nil || *server < 0 || *server > 100 {
		t.Errorf("CPU server = %v, want 0..100", server)
	}
	if proses != nil && (*proses < 0 || *proses > 100*float64(ukurSistem(nil, time.Now()).CPUJumlah)+1) {
		t.Errorf("CPU proses = %v di luar kewajaran", *proses)
	}
}

func TestUkurProsesWajar(t *testing.T) {
	pr := ukurProses()
	if pr.VersiGo == "" || pr.Goroutine < 1 || pr.HeapDipakai == 0 || pr.MemDariOS == 0 || pr.UptimeDetik < 0 {
		t.Errorf("proses = %+v", pr)
	}
	if pr.HeapDipakai > pr.MemDariOS {
		t.Errorf("heap %d melebihi memori dari OS %d", pr.HeapDipakai, pr.MemDariOS)
	}
}
