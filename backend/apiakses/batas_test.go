package apiakses

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Jam tiruan yang dapat dimajukan.
type jam struct {
	mu sync.Mutex
	t  time.Time
}

func (j *jam) now() time.Time { j.mu.Lock(); defer j.mu.Unlock(); return j.t }
func (j *jam) maju(d time.Duration) {
	j.mu.Lock()
	j.t = j.t.Add(d)
	j.mu.Unlock()
}

// Awal jam penuh supaya periode kuota (kelipatan 3600 detik) mudah dihitung.
func pembatasUji() (*Pembatas, *jam) {
	j := &jam{t: time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)}
	p := NewPembatas()
	p.sekarang = j.now
	return p, j
}

var batasBawaan = Batas{LimitPermintaan: 1000, LimitJendela: 60 * time.Second, Kuota: 5000, KuotaPeriode: time.Hour}

func TestBawaan1000Per60DetikDan5000PerJam(t *testing.T) {
	p, _ := pembatasUji()
	for i := 1; i <= 1000; i++ {
		if d := p.Coba(1, batasBawaan, true); !d.Izin {
			t.Fatalf("permintaan ke-%d harus diterima (alasan %q)", i, d.Alasan)
		}
	}
	d := p.Coba(1, batasBawaan, true)
	if d.Izin || d.Alasan != AlasanLaju {
		t.Fatalf("permintaan ke-1001 dalam 60 detik harus ditolak karena batas laju: %+v", d)
	}
	if d.TungguDetik != 60 {
		t.Errorf("Retry-After = %d, want 60 (semua permintaan terjadi pada detik yang sama)", d.TungguDetik)
	}
	if d.LimitSisa != 0 || d.Batas.LimitPermintaan != 1000 {
		t.Errorf("sisa/limit salah: %+v", d)
	}
}

func TestJendelaGeserTepatTanpaLonjakanGandaDiBatas(t *testing.T) {
	p, j := pembatasUji()
	b := Batas{LimitPermintaan: 10, LimitJendela: 60 * time.Second, Kuota: 1_000_000, KuotaPeriode: time.Hour}
	// 10 permintaan pada detik ke-59 jendela pertama...
	j.maju(59 * time.Second)
	for i := 0; i < 10; i++ {
		if !p.Coba(1, b, true).Izin {
			t.Fatal("10 permintaan pertama harus diterima")
		}
	}
	// ...lalu 2 detik kemudian (jendela tetap sudah berganti, jendela geser belum): tidak boleh ada lonjakan dua kali lipat.
	j.maju(2 * time.Second)
	d := p.Coba(1, b, true)
	if d.Izin || d.Alasan != AlasanLaju {
		t.Fatal("jendela geser harus menolak: 10 permintaan baru saja terjadi dalam 60 detik terakhir")
	}
	if d.TungguDetik != 58 {
		t.Errorf("Retry-After = %d, want 58 (permintaan tertua keluar jendela 58 detik lagi)", d.TungguDetik)
	}
	// Tepat saat permintaan tertua keluar jendela, satu slot terbuka.
	j.maju(58 * time.Second)
	for i := 0; i < 10; i++ { // semua 10 stempel berumur 60 detik tepat
		if !p.Coba(1, b, true).Izin {
			t.Fatalf("permintaan ke-%d setelah jendela lewat harus diterima", i+1)
		}
	}
	if p.Coba(1, b, true).Izin {
		t.Error("permintaan ke-11 harus ditolak lagi")
	}
}

func TestPenolakanTidakMemakaiJatah(t *testing.T) {
	p, _ := pembatasUji()
	b := Batas{LimitPermintaan: 2, LimitJendela: time.Minute, Kuota: 100, KuotaPeriode: time.Hour}
	p.Coba(1, b, true)
	p.Coba(1, b, true)
	for i := 0; i < 50; i++ {
		p.Coba(1, b, true) // ditolak: batas laju
	}
	d := p.Lihat(1, b)
	if d.KuotaTerpakai != 2 {
		t.Errorf("kuota terpakai = %d, want 2: 429 tidak boleh memakai kuota", d.KuotaTerpakai)
	}
	if d.LimitSisa != 0 {
		t.Errorf("limit sisa = %d, want 0", d.LimitSisa)
	}
}

func TestKuotaHabisDanPulihPadaPergantianPeriode(t *testing.T) {
	p, j := pembatasUji()
	b := Batas{LimitPermintaan: 100000, LimitJendela: time.Second, Kuota: 5000, KuotaPeriode: time.Hour}
	for i := 0; i < 5000; i++ {
		j.maju(10 * time.Millisecond) // menyebar supaya batas laju tidak ikut menahan
		if d := p.Coba(1, b, true); !d.Izin {
			t.Fatalf("permintaan ke-%d harus diterima (alasan %q)", i+1, d.Alasan)
		}
	}
	d := p.Coba(1, b, true)
	if d.Izin || d.Alasan != AlasanKuota {
		t.Fatalf("permintaan ke-5001 harus ditolak karena kuota: %+v", d)
	}
	if d.KuotaSisa != 0 || d.KuotaTerpakai != 5000 {
		t.Errorf("kuota sisa/terpakai salah: %+v", d)
	}
	// 50 detik terpakai di atas (5000 x 10 ms); periode berakhir pukul 11:00:00 tepat.
	wantTunggu := int((time.Hour - 50*time.Second).Seconds())
	if d.TungguDetik != wantTunggu {
		t.Errorf("Retry-After = %d, want %d", d.TungguDetik, wantTunggu)
	}
	if want := time.Date(2026, 10, 7, 11, 0, 0, 0, time.UTC); !d.KuotaReset.Equal(want) {
		t.Errorf("KuotaReset = %v, want %v (tiap jam tepat)", d.KuotaReset, want)
	}
	// Melewati pergantian jam: kuota penuh lagi.
	j.maju(time.Hour)
	d = p.Coba(1, b, true)
	if !d.Izin || d.KuotaTerpakai != 1 || d.KuotaSisa != 4999 {
		t.Fatalf("setelah periode berganti kuota harus penuh lagi: %+v", d)
	}
}

func TestKuotaDiperiksaSebelumBatasLaju(t *testing.T) {
	p, _ := pembatasUji()
	b := Batas{LimitPermintaan: 2, LimitJendela: time.Minute, Kuota: 2, KuotaPeriode: time.Hour}
	p.Coba(1, b, true)
	p.Coba(1, b, true)
	// Keduanya habis; yang dilaporkan harus kuota (masa tunggu panjang), bukan "coba lagi sebentar".
	if d := p.Coba(1, b, true); d.Alasan != AlasanKuota {
		t.Errorf("alasan = %q, want %q", d.Alasan, AlasanKuota)
	}
}

func TestTanpaKuotaTetapDikenaiBatasLaju(t *testing.T) {
	p, _ := pembatasUji()
	b := Batas{LimitPermintaan: 3, LimitJendela: time.Minute, Kuota: 1, KuotaPeriode: time.Hour}
	if !p.Coba(1, b, true).Izin {
		t.Fatal("satu permintaan memakai seluruh kuota")
	}
	// Kuota habis, tetapi permintaan yang tidak memakai kuota (mis. melihat pemakaian sendiri) tetap dilayani sampai batas laju.
	if d := p.Coba(1, b, false); !d.Izin {
		t.Fatalf("permintaan tanpa kuota harus diterima: %+v", d)
	}
	if d := p.Coba(1, b, false); !d.Izin {
		t.Fatalf("permintaan tanpa kuota harus diterima: %+v", d)
	}
	if d := p.Coba(1, b, false); d.Izin || d.Alasan != AlasanLaju {
		t.Fatalf("batas laju tetap berlaku untuk permintaan tanpa kuota: %+v", d)
	}
	if d := p.Lihat(1, b); d.KuotaTerpakai != 1 {
		t.Errorf("kuota terpakai = %d, want 1", d.KuotaTerpakai)
	}
}

func TestKlienTerpisah(t *testing.T) {
	p, _ := pembatasUji()
	b := Batas{LimitPermintaan: 1, LimitJendela: time.Minute, Kuota: 1, KuotaPeriode: time.Hour}
	if !p.Coba(1, b, true).Izin || p.Coba(1, b, true).Izin {
		t.Fatal("klien 1: satu diterima, berikutnya ditolak")
	}
	if !p.Coba(2, b, true).Izin {
		t.Error("klien 2 tidak boleh terpengaruh klien 1")
	}
}

// Inti "tahan banyak pengguna serentak": dengan ratusan goroutine yang berebut, jumlah yang diterima persis sama dengan batas (tidak lebih, tidak kurang).
func TestAkuratSaatDiaksesSerentak(t *testing.T) {
	p, _ := pembatasUji()
	b := Batas{LimitPermintaan: 1000, LimitJendela: time.Minute, Kuota: 5000, KuotaPeriode: time.Hour}
	var diterima, laju, kuota atomic.Int64
	var wg sync.WaitGroup
	for g := 0; g < 200; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ { // 4000 percobaan
				d := p.Coba(7, b, true)
				switch {
				case d.Izin:
					diterima.Add(1)
				case d.Alasan == AlasanLaju:
					laju.Add(1)
				case d.Alasan == AlasanKuota:
					kuota.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	if diterima.Load() != 1000 {
		t.Errorf("diterima = %d, want tepat 1000", diterima.Load())
	}
	if laju.Load() != 3000 || kuota.Load() != 0 {
		t.Errorf("laju = %d, kuota = %d, want 3000 dan 0", laju.Load(), kuota.Load())
	}
	if d := p.Lihat(7, b); d.KuotaTerpakai != 1000 {
		t.Errorf("kuota terpakai = %d, want 1000 (hanya yang diterima yang dihitung)", d.KuotaTerpakai)
	}
}

func TestKuotaAkuratSaatSerentak(t *testing.T) {
	p, j := pembatasUji()
	b := Batas{LimitPermintaan: 100000, LimitJendela: time.Second, Kuota: 500, KuotaPeriode: time.Hour}
	var diterima atomic.Int64
	var wg sync.WaitGroup
	for g := 0; g < 100; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				if p.Coba(9, b, true).Izin {
					diterima.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	_ = j
	if diterima.Load() != 500 {
		t.Errorf("diterima = %d, want tepat 500 (kuota)", diterima.Load())
	}
}

func TestPerubahanBatasBerlakuSegera(t *testing.T) {
	p, _ := pembatasUji()
	b := Batas{LimitPermintaan: 5, LimitJendela: time.Minute, Kuota: 100, KuotaPeriode: time.Hour}
	for i := 0; i < 5; i++ {
		p.Coba(1, b, true)
	}
	if p.Coba(1, b, true).Izin {
		t.Fatal("harus ditolak pada batas 5")
	}
	// Superadmin menaikkan batas: permintaan berikutnya langsung memakai batas baru, tanpa memulai ulang apa pun.
	b.LimitPermintaan = 8
	for i := 0; i < 3; i++ {
		if !p.Coba(1, b, true).Izin {
			t.Fatalf("batas dinaikkan ke 8: permintaan ke-%d harus diterima", 6+i)
		}
	}
	if p.Coba(1, b, true).Izin {
		t.Error("harus ditolak lagi pada 8")
	}
	// Diturunkan di bawah yang sudah terpakai: ditolak sampai cukup stempel keluar.
	b.LimitPermintaan = 3
	if d := p.Coba(1, b, true); d.Izin {
		t.Error("batas diturunkan di bawah pemakaian: harus ditolak")
	}
	// Kuota dinaikkan/diturunkan juga langsung berlaku.
	b.LimitPermintaan, b.Kuota = 100, 8
	if d := p.Coba(1, b, true); d.Izin || d.Alasan != AlasanKuota {
		t.Errorf("kuota diturunkan ke 8 (sudah terpakai 8): harus ditolak karena kuota, got %+v", d)
	}
}

func TestPeriodeKuotaDapatDiubah(t *testing.T) {
	p, j := pembatasUji()
	j.maju(30 * time.Minute) // 10:30
	b := Batas{LimitPermintaan: 1000, LimitJendela: time.Minute, Kuota: 10, KuotaPeriode: 30 * time.Minute}
	d := p.Coba(1, b, true)
	if want := time.Date(2026, 10, 7, 11, 0, 0, 0, time.UTC); !d.KuotaReset.Equal(want) {
		t.Errorf("reset = %v, want %v untuk periode 30 menit", d.KuotaReset, want)
	}
}

func TestJamMundurTidakMerusakUrutan(t *testing.T) {
	p, j := pembatasUji()
	b := Batas{LimitPermintaan: 3, LimitJendela: time.Minute, Kuota: 100, KuotaPeriode: time.Hour}
	p.Coba(1, b, true)
	j.maju(-5 * time.Second) // koreksi jam sistem
	p.Coba(1, b, true)
	p.Coba(1, b, true)
	if p.Coba(1, b, true).Izin {
		t.Error("harus ditolak: tiga permintaan dalam jendela")
	}
}

func TestSisaDanResetPadaHeader(t *testing.T) {
	p, j := pembatasUji()
	b := Batas{LimitPermintaan: 5, LimitJendela: time.Minute, Kuota: 50, KuotaPeriode: time.Hour}
	d := p.Coba(1, b, true)
	if d.LimitSisa != 4 || d.KuotaSisa != 49 {
		t.Errorf("sisa = %d/%d, want 4/49", d.LimitSisa, d.KuotaSisa)
	}
	j.maju(10 * time.Second)
	d = p.Coba(1, b, true)
	if want := j.now().Add(time.Minute); !d.LimitReset.Equal(want) {
		t.Errorf("LimitReset = %v, want %v (stempel terbaru + jendela)", d.LimitReset, want)
	}
}

func TestSnapshotDanPulihkanKuota(t *testing.T) {
	p, j := pembatasUji()
	b := Batas{LimitPermintaan: 100, LimitJendela: time.Minute, Kuota: 50, KuotaPeriode: time.Hour}
	for i := 0; i < 7; i++ {
		p.Coba(3, b, true)
	}
	ss := p.SnapshotKotor()
	if len(ss) != 1 || ss[0].KlienID != 3 || ss[0].Jumlah != 7 {
		t.Fatalf("snapshot = %+v", ss)
	}
	if len(p.SnapshotKotor()) != 0 {
		t.Error("setelah diambil, tidak ada lagi yang kotor")
	}
	p.TandaiKotor(ss) // penyimpanan gagal: dicoba lagi
	if len(p.SnapshotKotor()) != 1 {
		t.Error("TandaiKotor harus menandai ulang")
	}

	// Aplikasi dimulai ulang: pembatas baru, kuota dipulihkan dari database.
	baru := NewPembatas()
	baru.sekarang = j.now
	baru.Pulihkan(ss[0])
	if d := baru.Lihat(3, b); d.KuotaTerpakai != 7 || d.KuotaSisa != 43 {
		t.Errorf("setelah pulih: %+v", d)
	}
	if d := baru.Coba(3, b, true); !d.Izin || d.KuotaTerpakai != 8 {
		t.Errorf("hitungan harus berlanjut dari 7: %+v", d)
	}
	// Snapshot dari periode lama tidak berlaku pada periode baru.
	lama := NewPembatas()
	lama.sekarang = j.now
	lama.Pulihkan(KuotaSnapshot{KlienID: 3, Mulai: ss[0].Mulai.Add(-time.Hour), Jumlah: 49})
	if d := lama.Coba(3, b, true); d.KuotaTerpakai != 1 {
		t.Errorf("snapshot periode lama harus diabaikan: %+v", d)
	}
}

func TestBersihkanMembuangYangTidakPerlu(t *testing.T) {
	p, j := pembatasUji()
	b := Batas{LimitPermintaan: 10, LimitJendela: time.Minute, Kuota: 100, KuotaPeriode: time.Hour}
	p.Coba(1, b, false) // tanpa kuota
	p.Coba(2, b, true)
	if n := p.Bersihkan(); n != 0 {
		t.Errorf("masih dalam jendela: tidak ada yang dibuang, got %d", n)
	}
	j.maju(2 * time.Minute)
	p.Bersihkan()
	if p.Ukuran() != 1 {
		t.Errorf("klien 1 (tanpa kuota, jendela lewat) dibuang, klien 2 (kuota terpakai, periode berjalan) dipertahankan; ukuran = %d", p.Ukuran())
	}
	p.SnapshotKotor() // sudah disalin ke database
	j.maju(time.Hour)
	p.Bersihkan()
	if p.Ukuran() != 0 {
		t.Errorf("setelah periode lewat semua dibuang; ukuran = %d", p.Ukuran())
	}
}

func TestAwalPeriode(t *testing.T) {
	cases := []struct {
		t       time.Time
		periode time.Duration
		want    time.Time
	}{
		{time.Date(2026, 10, 7, 10, 59, 59, 0, time.UTC), time.Hour, time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)},
		{time.Date(2026, 10, 7, 11, 0, 0, 0, time.UTC), time.Hour, time.Date(2026, 10, 7, 11, 0, 0, 0, time.UTC)},
		{time.Date(2026, 10, 7, 11, 20, 0, 0, time.UTC), 15 * time.Minute, time.Date(2026, 10, 7, 11, 15, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		if got := AwalPeriode(c.t, c.periode); !got.Equal(c.want) {
			t.Errorf("AwalPeriode(%v, %v) = %v, want %v", c.t, c.periode, got, c.want)
		}
	}
}
