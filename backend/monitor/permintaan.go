package monitor

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// Pengukur kinerja permintaan HTTP: jumlah, galat 4xx/5xx, dan sebaran waktu respons (histogram), per jendela waktu dan per rute. Hanya angka dan pola rute yang disimpan;
// tidak ada isi permintaan, alamat lengkap, atau identitas pengguna.

// Batas atas tiap ember histogram (milidetik); ember terakhir menampung yang lebih lambat dari batas terakhir.
var batasEmber = [...]float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}

const jumlahEmber = len(batasEmber) + 1

type histogram [jumlahEmber]int64

func (h *histogram) tambah(ms float64) {
	for i, b := range batasEmber {
		if ms <= b {
			h[i]++
			return
		}
	}
	h[jumlahEmber-1]++
}

// persentil memperkirakan waktu respons pada kuantil q (0-1) dengan interpolasi linear di dalam ember; di ember terakhir dikembalikan batas terakhir (">10 detik").
func persentil(h histogram, q float64) float64 {
	var total int64
	for _, n := range h {
		total += n
	}
	if total == 0 {
		return 0
	}
	target := q * float64(total)
	var kum float64
	bawah := 0.0
	for i := 0; i < len(batasEmber); i++ {
		n := float64(h[i])
		if kum+n >= target && n > 0 {
			return bawah + (batasEmber[i]-bawah)*((target-kum)/n)
		}
		kum += n
		bawah = batasEmber[i]
	}
	return batasEmber[len(batasEmber)-1]
}

// hitungan: penjumlahan untuk satu jendela. Total/4xx/5xx mencakup semua permintaan; waktu respons (NR, SumR, HistR) hanya permintaan "ringan", yaitu bukan ekspor,
// impor, sinkronisasi, atau penarikan yang memang lama, supaya p95 mencerminkan pengalaman pengguna di halaman biasa.
type hitungan struct {
	Total, C4xx, C5xx int64
	NR                int64
	SumR              float64
	HistR             histogram
}

func (a hitungan) kurang(b hitungan) hitungan {
	d := hitungan{Total: a.Total - b.Total, C4xx: a.C4xx - b.C4xx, C5xx: a.C5xx - b.C5xx, NR: a.NR - b.NR, SumR: a.SumR - b.SumR}
	for i := range d.HistR {
		d.HistR[i] = a.HistR[i] - b.HistR[i]
	}
	return d
}

func (a *hitungan) tambahkan(b hitungan) {
	a.Total += b.Total
	a.C4xx += b.C4xx
	a.C5xx += b.C5xx
	a.NR += b.NR
	a.SumR += b.SumR
	for i := range a.HistR {
		a.HistR[i] += b.HistR[i]
	}
}

func (a hitungan) jendela(nama string, detik float64) JendelaHTTP {
	j := JendelaHTTP{Nama: nama, Total: a.Total, Galat4xx: a.C4xx, Galat5xx: a.C5xx}
	if a.NR > 0 {
		j.RataMS = a.SumR / float64(a.NR)
		j.P50MS, j.P95MS, j.P99MS = persentil(a.HistR, 0.50), persentil(a.HistR, 0.95), persentil(a.HistR, 0.99)
	}
	if a.Total > 0 {
		j.PersenGalat = float64(a.C5xx) / float64(a.Total) * 100
	}
	if detik > 0 {
		j.PerDetik = float64(a.Total) / detik
	}
	return j
}

type ember struct {
	menit int64 // menit unix
	h     hitungan
}

type statRute struct {
	metode, rute string
	n, c5xx      int64
	sum, maks    float64
	hist         histogram
	berat        bool
}

// MetrikHTTP menampung pengukuran permintaan. Aman dipakai bersamaan.
type MetrikHTTP struct {
	mu       sync.Mutex
	kum      hitungan // sejak proses mulai
	menit    [60]ember
	rute     map[string]*statRute
	mulai    time.Time
	berjalan atomic.Int64
	puncak   atomic.Int64 // puncak permintaan serentak sejak pengambilan jendela terakhir
	terakhir hitungan     // kum pada pengambilan jendela sampler sebelumnya
}

// Metrik: pengukur proses ini (dipasang middleware Middleware).
var Metrik = NewMetrikHTTP()

func NewMetrikHTTP() *MetrikHTTP {
	return &MetrikHTTP{rute: map[string]*statRute{}, mulai: time.Now()}
}

// ruteBerat: rute yang memang lama (ekspor, impor, sinkronisasi, penarikan, unduhan); tidak dihitung pada waktu respons keseluruhan.
func ruteBerat(rute string) bool {
	for _, k := range []string{"/ekspor", "/impor", "/sinkronisasi", "/penarikan", "/unduh", "/dokumen", "/template", "/tarik-sldk", "/dari-satker"} {
		if strings.Contains(rute, k) {
			return true
		}
	}
	return false
}

const maksRuteDilacak = 400 // batas pengaman bila ada rute tak terduga

// Catat mencatat satu permintaan yang selesai.
func (m *MetrikHTTP) Catat(metode, rute string, status int, d time.Duration, sekarang time.Time) {
	ms := float64(d.Microseconds()) / 1000
	berat := ruteBerat(rute)
	var h hitungan
	h.Total = 1
	switch {
	case status >= 500:
		h.C5xx = 1
	case status >= 400:
		h.C4xx = 1
	}
	if !berat {
		h.NR, h.SumR = 1, ms
		h.HistR.tambah(ms)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.kum.tambahkan(h)
	mn := sekarang.Unix() / 60
	e := &m.menit[mn%60]
	if e.menit != mn {
		*e = ember{menit: mn}
	}
	e.h.tambahkan(h)

	kunci := metode + " " + rute
	s := m.rute[kunci]
	if s == nil {
		if len(m.rute) >= maksRuteDilacak {
			return
		}
		s = &statRute{metode: metode, rute: rute, berat: berat}
		m.rute[kunci] = s
	}
	s.n++
	s.sum += ms
	if ms > s.maks {
		s.maks = ms
	}
	if status >= 500 {
		s.c5xx++
	}
	s.hist.tambah(ms)
}

// JendelaMenit menjumlahkan n menit terakhir (termasuk menit berjalan).
func (m *MetrikHTTP) JendelaMenit(nama string, n int, sekarang time.Time) JendelaHTTP {
	m.mu.Lock()
	defer m.mu.Unlock()
	var h hitungan
	mn := sekarang.Unix() / 60
	for i := range m.menit {
		e := m.menit[i]
		if e.menit > mn-int64(n) && e.menit <= mn {
			h.tambahkan(e.h)
		}
	}
	return h.jendela(nama, float64(n*60))
}

// SejakMulai: ringkasan sejak proses mulai.
func (m *MetrikHTTP) SejakMulai(sekarang time.Time) JendelaHTTP {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.kum.jendela("Sejak proses mulai", sekarang.Sub(m.mulai).Seconds())
}

// Berjalan: jumlah permintaan yang sedang diproses.
func (m *MetrikHTTP) Berjalan() int64 { return m.berjalan.Load() }

// AmbilJendela mengembalikan penjumlahan sejak pemanggilan sebelumnya dan puncak permintaan serentak, lalu mengatur ulang; dipakai pengukur berkala.
func (m *MetrikHTTP) AmbilJendela() (hitungan, int64) {
	m.mu.Lock()
	d := m.kum.kurang(m.terakhir)
	m.terakhir = m.kum
	m.mu.Unlock()
	return d, m.puncak.Swap(m.berjalan.Load())
}

func (s *statRute) ringkas() RuteLambat {
	r := RuteLambat{Rute: s.rute, Metode: s.metode, Jumlah: s.n, Galat5xx: s.c5xx, MaksMS: s.maks, P95MS: persentil(s.hist, 0.95), Berat: s.berat}
	if s.n > 0 {
		r.RataMS = s.sum / float64(s.n)
	}
	return r
}

// RuteTeratas mengembalikan rute paling lambat (menurut p95, minimal minJumlah permintaan) dan rute dengan galat 5xx terbanyak.
func (m *MetrikHTTP) RuteTeratas(n, minJumlah int) (lambat, galat []RuteLambat) {
	m.mu.Lock()
	semua := make([]RuteLambat, 0, len(m.rute))
	for _, s := range m.rute {
		semua = append(semua, s.ringkas())
	}
	m.mu.Unlock()

	lambat, galat = []RuteLambat{}, []RuteLambat{}
	for _, r := range semua {
		if r.Jumlah >= int64(minJumlah) {
			lambat = append(lambat, r)
		}
		if r.Galat5xx > 0 {
			galat = append(galat, r)
		}
	}
	sort.Slice(lambat, func(i, j int) bool {
		if lambat[i].P95MS != lambat[j].P95MS {
			return lambat[i].P95MS > lambat[j].P95MS
		}
		return lambat[i].Rute < lambat[j].Rute
	})
	sort.Slice(galat, func(i, j int) bool {
		if galat[i].Galat5xx != galat[j].Galat5xx {
			return galat[i].Galat5xx > galat[j].Galat5xx
		}
		return galat[i].Rute < galat[j].Rute
	})
	if len(lambat) > n {
		lambat = lambat[:n]
	}
	if len(galat) > n {
		galat = galat[:n]
	}
	return lambat, galat
}

// Middleware mengukur setiap permintaan (kecuali /health). Dipasang di luar middleware lain agar waktu responsnya mencakup semuanya.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}
		mulai := time.Now()
		n := Metrik.berjalan.Add(1)
		for {
			p := Metrik.puncak.Load()
			if n <= p || Metrik.puncak.CompareAndSwap(p, n) {
				break
			}
		}
		catat := func(status int) {
			Metrik.berjalan.Add(-1)
			rute := c.FullPath()
			if rute == "" {
				rute = "(rute tidak ada)"
			}
			Metrik.Catat(c.Request.Method, rute, status, time.Since(mulai), mulai)
		}
		defer func() {
			if r := recover(); r != nil { // panic dihitung sebagai galat 500, lalu diteruskan ke penangkap panic gin
				catat(500)
				panic(r)
			}
		}()
		c.Next()
		catat(c.Writer.Status())
	}
}
