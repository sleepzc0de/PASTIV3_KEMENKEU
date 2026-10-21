package apiakses

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"pasti-v3-backend/serverlog"
)

// Penyimpan: tempat riwayat dan hitungan ditulis (Store, atau tiruan pada tes).
type Penyimpan interface {
	SimpanLog(ctx context.Context, ps []Penggunaan) error
	SimpanAgregat(ctx context.Context, as []Agregat) error
}

// StatistikPencatat: keadaan pencatat. Dijatuhkan atau Gagal yang bertambah berarti ada permintaan yang tidak masuk riwayat (hitungan per jam tetap benar bila hanya
// riwayatnya yang dijatuhkan).
type StatistikPencatat struct {
	Diterima   int64 `json:"diterima"`
	Ditulis    int64 `json:"ditulis"`
	Dijatuhkan int64 `json:"dijatuhkan"` // antrean riwayat penuh
	Gagal      int64 `json:"gagal"`      // gagal menulis ke database
	Antrean    int   `json:"antrean"`
	Kapasitas  int   `json:"kapasitas"`
}

// Pencatat mencatat pemakaian tanpa menambah waktu respons: hitungan per jam dijumlahkan di memori (murah) dan ditulis tiap beberapa detik; riwayat per permintaan
// masuk antrean dan ditulis berkelompok. Antrean penuh (database macet) menjatuhkan riwayat, bukan menahan permintaan.
type Pencatat struct {
	simpan Penyimpan
	ch     chan Penggunaan

	mu  sync.Mutex
	agg map[kunciAgregat]*Agregat

	mtutup   sync.RWMutex // melindungi penutupan antrean dari pengiriman yang bersamaan
	tutup    bool
	selesai  chan struct{}
	mulai    sync.Once
	interval time.Duration

	diterima, ditulis, dijatuhkan, gagal atomic.Int64
}

// NewPencatat membuat pencatat dengan antrean riwayat berkapasitas kapasitas. Mulai harus dipanggil agar mulai menulis.
func NewPencatat(simpan Penyimpan, kapasitas int) *Pencatat {
	if kapasitas < 1 {
		kapasitas = 8192
	}
	return &Pencatat{simpan: simpan, ch: make(chan Penggunaan, kapasitas), agg: make(map[kunciAgregat]*Agregat), selesai: make(chan struct{}), interval: 2 * time.Second}
}

// Mulai menjalankan penulis latar belakang (sekali).
func (p *Pencatat) Mulai() { p.mulai.Do(func() { go p.jalankan() }) }

// Catat menjumlahkan ke hitungan per jam dan memasukkan riwayat ke antrean (tanpa menunggu).
func (p *Pencatat) Catat(u Penggunaan) {
	p.mtutup.RLock()
	defer p.mtutup.RUnlock()
	if p.tutup {
		return
	}
	p.diterima.Add(1)
	jam := u.Waktu.UTC().Truncate(time.Hour)
	k := kunciAgregat{jam: jam.Unix(), klien: u.KlienID, endpoint: u.Endpoint, status: u.Status, hasil: u.Hasil}
	p.mu.Lock()
	a := p.agg[k]
	if a == nil {
		a = &Agregat{Jam: jam, KlienID: u.KlienID, Endpoint: u.Endpoint, Status: u.Status, Hasil: u.Hasil}
		p.agg[k] = a
	}
	a.tambah(u)
	p.mu.Unlock()

	select {
	case p.ch <- u:
	default:
		p.dijatuhkan.Add(1)
	}
}

// Statistik membaca keadaan saat ini.
func (p *Pencatat) Statistik() StatistikPencatat {
	return StatistikPencatat{Diterima: p.diterima.Load(), Ditulis: p.ditulis.Load(), Dijatuhkan: p.dijatuhkan.Load(), Gagal: p.gagal.Load(), Antrean: len(p.ch), Kapasitas: cap(p.ch)}
}

func (p *Pencatat) jalankan() {
	defer close(p.selesai)
	tik := time.NewTicker(p.interval)
	defer tik.Stop()
	var kelompok []Penggunaan
	siramLog := func() {
		if len(kelompok) == 0 {
			return
		}
		p.tulisLog(kelompok)
		kelompok = kelompok[:0]
	}
	for {
		select {
		case u, ok := <-p.ch:
			if !ok {
				siramLog()
				p.tulisAgregat()
				return
			}
			kelompok = append(kelompok, u)
			if len(kelompok) >= 200 {
				siramLog()
			}
		case <-tik.C:
			siramLog()
			p.tulisAgregat()
		}
	}
}

func (p *Pencatat) tulisLog(ps []Penggunaan) {
	ctx, batal := context.WithTimeout(context.Background(), 20*time.Second)
	defer batal()
	salin := append([]Penggunaan(nil), ps...)
	coba := func() (err error) {
		if !serverlog.Aman("API-AKSES", func() { err = p.simpan.SimpanLog(ctx, salin) }) {
			err = errors.New("panik saat menulis riwayat")
		}
		return err
	}
	err := coba()
	if err != nil {
		time.Sleep(500 * time.Millisecond)
		err = coba()
	}
	if err != nil {
		p.gagal.Add(int64(len(ps)))
		log.Printf("[API-AKSES ERROR] gagal menulis %d riwayat permintaan: %v", len(ps), err)
		return
	}
	p.ditulis.Add(int64(len(ps)))
}

// tulisAgregat menukar hitungan di memori dengan peta kosong lalu menuliskannya; bila gagal, hitungannya dikembalikan supaya dicoba lagi (tidak hilang).
func (p *Pencatat) tulisAgregat() {
	p.mu.Lock()
	if len(p.agg) == 0 {
		p.mu.Unlock()
		return
	}
	ambil := p.agg
	p.agg = make(map[kunciAgregat]*Agregat)
	p.mu.Unlock()

	daftar := make([]Agregat, 0, len(ambil))
	for _, a := range ambil {
		daftar = append(daftar, *a)
	}
	ctx, batal := context.WithTimeout(context.Background(), 20*time.Second)
	defer batal()
	var err error
	if !serverlog.Aman("API-AKSES", func() { err = p.simpan.SimpanAgregat(ctx, daftar) }) {
		err = errors.New("panik saat menulis hitungan")
	}
	if err == nil {
		return
	}
	log.Printf("[API-AKSES ERROR] gagal menulis hitungan per jam (%d baris), akan dicoba lagi: %v", len(daftar), err)
	p.mu.Lock()
	for k, a := range ambil {
		if ada := p.agg[k]; ada != nil {
			ada.gabung(a)
		} else {
			p.agg[k] = a
		}
	}
	p.mu.Unlock()
}

// Berhenti berhenti menerima catatan, menuliskan sisanya, lalu selesai (atau menyerah bila ctx habis).
func (p *Pencatat) Berhenti(ctx context.Context) {
	p.mtutup.Lock()
	if !p.tutup {
		p.tutup = true
		p.Mulai() // pastikan penulis ada supaya antrean terkuras
		close(p.ch)
	}
	p.mtutup.Unlock()
	select {
	case <-p.selesai:
	case <-ctx.Done():
	}
}
