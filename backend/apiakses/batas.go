package apiakses

import (
	"math"
	"sync"
	"time"
)

// Batas laju dan kuota per klien.
//
//   - Batas laju: JENDELA GESER yang tepat. Setiap permintaan yang diterima dicatat stempel waktunya; permintaan baru diterima bila kurang dari LimitPermintaan stempel
//     berumur kurang dari LimitJendela. Tidak ada lonjakan dua kali lipat di batas jendela seperti pada jendela tetap. Memori: 8 byte per permintaan dalam jendela
//     (paling banyak LimitMaks per klien).
//   - Kuota: JENDELA TETAP selaras kelipatan KuotaPeriode sejak epoch UTC (periode 3600 detik = tiap jam tepat). Penghitungnya disalin ke database secara berkala
//     (Layanan.simpanKuota) dan dipulihkan saat aplikasi mulai, jadi pemulaian ulang tidak mengembalikan kuota penuh.
//
// Penolakan (429) tidak memakai jatah apa pun. Kuota diperiksa lebih dulu karena masa tunggunya lebih lama: klien yang kuotanya habis harus diberi tahu itu, bukan
// disuruh mencoba lagi satu detik kemudian.
//
// Pembatas hidup di memori satu proses (satu instance aplikasi, seperti pembatas laju dan captcha lain di aplikasi ini): tanpa penyimpanan bersama, menjalankan
// beberapa instance membuat batas berlaku per instance. Alasan pilihan ini ada di docs/api-akses-data.md.

// Batas: batas yang berlaku pada satu permintaan.
type Batas struct {
	LimitPermintaan int
	LimitJendela    time.Duration
	Kuota           int
	KuotaPeriode    time.Duration
}

// Alasan penolakan.
const (
	AlasanLaju  = "batas_laju"
	AlasanKuota = "kuota_habis"
)

// Keputusan: hasil pemeriksaan satu permintaan, beserta angka untuk header respons.
type Keputusan struct {
	Izin        bool
	Alasan      string // kosong bila diizinkan
	TungguDetik int    // Retry-After saat ditolak (minimal 1)
	Batas       Batas

	LimitSisa  int       // sisa permintaan pada jendela laju (sesudah permintaan ini bila diizinkan)
	LimitReset time.Time // saat jatah jendela laju pulih penuh (stempel terbaru keluar dari jendela); sekarang bila belum ada permintaan

	KuotaTerpakai int
	KuotaSisa     int
	KuotaReset    time.Time // akhir periode kuota berjalan
}

type keadaan struct {
	mu       sync.Mutex
	stempel  []int64 // UnixNano permintaan yang diterima, urut naik; yang berindeks < awal sudah keluar dari jendela
	awal     int
	jendela  time.Duration
	kuotaMul int64 // detik Unix awal periode kuota berjalan
	kuotaAkh int64 // detik Unix akhir periode kuota berjalan
	kuotaJml int
	kotor    bool // penghitung kuota berubah sejak terakhir disalin ke database
}

// Pembatas: keadaan semua klien di memori.
type Pembatas struct {
	mu       sync.Mutex
	klien    map[int64]*keadaan
	sekarang func() time.Time
}

func NewPembatas() *Pembatas {
	return &Pembatas{klien: make(map[int64]*keadaan), sekarang: time.Now}
}

func (p *Pembatas) ambil(id int64) *keadaan {
	p.mu.Lock()
	defer p.mu.Unlock()
	k := p.klien[id]
	if k == nil {
		k = &keadaan{}
		p.klien[id] = k
	}
	return k
}

// AwalPeriode: awal periode tetap yang memuat t (kelipatan periode sejak epoch UTC).
func AwalPeriode(t time.Time, periode time.Duration) time.Time {
	d := int64(periode / time.Second)
	if d <= 0 {
		d = 1
	}
	s := t.Unix()
	return time.Unix(s-s%d, 0).UTC()
}

func detikNaik(d time.Duration) int {
	n := int(math.Ceil(d.Seconds()))
	if n < 1 {
		n = 1
	}
	return n
}

// segarkan membuang stempel yang sudah keluar dari jendela dan menyelaraskan periode kuota dengan waktu sekarang. k.mu harus dipegang.
func (k *keadaan) segarkan(now time.Time, b Batas) {
	k.jendela = b.LimitJendela
	batas := now.Add(-b.LimitJendela).UnixNano()
	for k.awal < len(k.stempel) && k.stempel[k.awal] <= batas {
		k.awal++
	}
	if k.awal > 0 && k.awal*2 >= len(k.stempel) { // pangkas awal agar memori tidak tumbuh tanpa batas
		n := copy(k.stempel, k.stempel[k.awal:])
		k.stempel = k.stempel[:n]
		k.awal = 0
	}
	mulai := AwalPeriode(now, b.KuotaPeriode)
	if k.kuotaMul != mulai.Unix() {
		k.kuotaMul, k.kuotaAkh, k.kuotaJml = mulai.Unix(), mulai.Add(b.KuotaPeriode).Unix(), 0
	} else {
		k.kuotaAkh = mulai.Add(b.KuotaPeriode).Unix()
	}
}

// isi menulis angka sisa/reset ke keputusan. k.mu harus dipegang.
func (k *keadaan) isi(d *Keputusan, now time.Time, b Batas) {
	dalam := len(k.stempel) - k.awal
	d.Batas = b
	d.LimitSisa = b.LimitPermintaan - dalam
	if d.LimitSisa < 0 {
		d.LimitSisa = 0
	}
	d.LimitReset = now
	if dalam > 0 {
		d.LimitReset = time.Unix(0, k.stempel[len(k.stempel)-1]).Add(b.LimitJendela)
	}
	d.KuotaTerpakai = k.kuotaJml
	d.KuotaSisa = b.Kuota - k.kuotaJml
	if d.KuotaSisa < 0 {
		d.KuotaSisa = 0
	}
	d.KuotaReset = time.Unix(k.kuotaAkh, 0).UTC()
}

// Coba memeriksa dan, bila diizinkan, mencatat satu permintaan klien id. pakaiKuota=false untuk permintaan yang tidak menghabiskan kuota (mis. melihat pemakaian
// sendiri): tetap dikenai batas laju tetapi tidak memeriksa maupun memakai kuota.
func (p *Pembatas) Coba(id int64, b Batas, pakaiKuota bool) Keputusan {
	now := p.sekarang()
	k := p.ambil(id)
	k.mu.Lock()
	defer k.mu.Unlock()
	k.segarkan(now, b)

	var d Keputusan
	switch {
	case pakaiKuota && k.kuotaJml >= b.Kuota:
		d.Alasan = AlasanKuota
		d.TungguDetik = detikNaik(time.Unix(k.kuotaAkh, 0).Sub(now))
	case len(k.stempel)-k.awal >= b.LimitPermintaan:
		d.Alasan = AlasanLaju
		// Perlu keluar sebanyak (isi - limit + 1) stempel tertua supaya ada tempat; biasanya hanya satu (yang tertua).
		i := k.awal + (len(k.stempel) - k.awal - b.LimitPermintaan)
		d.TungguDetik = detikNaik(time.Unix(0, k.stempel[i]).Add(b.LimitJendela).Sub(now))
	default:
		n := now.UnixNano()
		if l := len(k.stempel); l > 0 && n < k.stempel[l-1] { // jam mundur: jaga urutan stempel
			n = k.stempel[l-1]
		}
		k.stempel = append(k.stempel, n)
		if pakaiKuota {
			k.kuotaJml++
			k.kotor = true
		}
		d.Izin = true
	}
	k.isi(&d, now, b)
	return d
}

// Lihat membaca keadaan klien tanpa memakai jatah apa pun.
func (p *Pembatas) Lihat(id int64, b Batas) Keputusan {
	now := p.sekarang()
	p.mu.Lock()
	k := p.klien[id]
	p.mu.Unlock()
	var d Keputusan
	if k == nil {
		k = &keadaan{}
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.segarkan(now, b)
	d.Izin = true
	k.isi(&d, now, b)
	return d
}

// KuotaSnapshot: penghitung kuota satu klien untuk disalin ke database.
type KuotaSnapshot struct {
	KlienID int64
	Mulai   time.Time
	Jumlah  int
}

// SnapshotKotor mengambil penghitung kuota yang berubah sejak pengambilan terakhir dan menandainya bersih. Bila penyimpanan gagal, panggil TandaiKotor.
func (p *Pembatas) SnapshotKotor() []KuotaSnapshot {
	p.mu.Lock()
	daftar := make(map[int64]*keadaan, len(p.klien))
	for id, k := range p.klien {
		daftar[id] = k
	}
	p.mu.Unlock()
	var out []KuotaSnapshot
	for id, k := range daftar {
		k.mu.Lock()
		if k.kotor {
			out = append(out, KuotaSnapshot{KlienID: id, Mulai: time.Unix(k.kuotaMul, 0).UTC(), Jumlah: k.kuotaJml})
			k.kotor = false
		}
		k.mu.Unlock()
	}
	return out
}

// TandaiKotor menandai ulang penghitung yang gagal disimpan supaya dicoba lagi pada putaran berikutnya.
func (p *Pembatas) TandaiKotor(ss []KuotaSnapshot) {
	for _, s := range ss {
		k := p.ambil(s.KlienID)
		k.mu.Lock()
		if k.kuotaMul == s.Mulai.Unix() {
			k.kotor = true
		}
		k.mu.Unlock()
	}
}

// Pulihkan memasang penghitung kuota yang tersimpan (saat aplikasi mulai). Tidak menimpa penghitung yang sudah lebih besar pada periode yang sama.
func (p *Pembatas) Pulihkan(s KuotaSnapshot) {
	k := p.ambil(s.KlienID)
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.kuotaMul == s.Mulai.Unix() && k.kuotaJml >= s.Jumlah {
		return
	}
	k.kuotaMul, k.kuotaJml = s.Mulai.Unix(), s.Jumlah
}

// Bersihkan membuang keadaan klien yang tidak lagi diperlukan: tidak ada permintaan dalam jendela dan tidak ada kuota terpakai pada periode yang masih berjalan.
// Mengembalikan jumlah yang dibuang.
func (p *Pembatas) Bersihkan() int {
	now := p.sekarang()
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for id, k := range p.klien {
		k.mu.Lock()
		kosong := true
		if l := len(k.stempel); l > k.awal && k.stempel[l-1] > now.Add(-k.jendela).UnixNano() {
			kosong = false
		}
		if k.kuotaJml > 0 && k.kuotaAkh > now.Unix() {
			kosong = false
		}
		if k.kotor {
			kosong = false
		}
		k.mu.Unlock()
		if kosong {
			delete(p.klien, id)
			n++
		}
	}
	return n
}

// Ukuran: jumlah klien yang dilacak (untuk tes).
func (p *Pembatas) Ukuran() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.klien)
}
