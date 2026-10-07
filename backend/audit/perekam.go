package audit

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// SimpanFunc menulis sekelompok entri ke penyimpanan.
type SimpanFunc func(ctx context.Context, es []Entri) error

// Perekam menampung entri di antrean dan menuliskannya ke penyimpanan secara berkelompok di latar belakang, sehingga mencatat audit tidak menambah waktu respons.
// Catat tidak pernah menunggu: bila antrean penuh (database macet), entri dijatuhkan dan dihitung (Statistik.Dijatuhkan) supaya aplikasi tetap melayani pengguna.
type Perekam struct {
	simpan  SimpanFunc
	ch      chan Entri
	selesai chan struct{}
	mu      sync.RWMutex // melindungi penutupan antrean dari pengiriman yang bersamaan
	tutup   bool

	diterima, ditulis, dijatuhkan, gagal atomic.Int64
	interval                             time.Duration
	maksKelompok                         int
}

// NewPerekam membuat perekam dengan antrean berkapasitas kapasitas entri. Mulai harus dipanggil agar entri mulai ditulis.
func NewPerekam(simpan SimpanFunc, kapasitas int) *Perekam {
	if kapasitas < 1 {
		kapasitas = 2048
	}
	return &Perekam{simpan: simpan, ch: make(chan Entri, kapasitas), selesai: make(chan struct{}), interval: time.Second, maksKelompok: 100}
}

// Mulai menjalankan penulis latar belakang.
func (p *Perekam) Mulai() { go p.jalankan() }

// Catat memasukkan entri ke antrean tanpa menunggu. Mengembalikan false bila entri dijatuhkan.
func (p *Perekam) Catat(e Entri) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.tutup {
		return false
	}
	p.diterima.Add(1)
	select {
	case p.ch <- e:
		return true
	default:
		p.dijatuhkan.Add(1)
		return false
	}
}

// Statistik: keadaan perekam saat ini.
func (p *Perekam) Statistik() Statistik {
	return Statistik{Diterima: p.diterima.Load(), Ditulis: p.ditulis.Load(), Dijatuhkan: p.dijatuhkan.Load(), Gagal: p.gagal.Load(), Antrean: len(p.ch), Kapasitas: cap(p.ch)}
}

func (p *Perekam) jalankan() {
	defer close(p.selesai)
	tik := time.NewTicker(p.interval)
	defer tik.Stop()
	var kelompok []Entri
	siram := func() {
		if len(kelompok) == 0 {
			return
		}
		p.tulis(kelompok)
		kelompok = kelompok[:0]
	}
	for {
		select {
		case e, ok := <-p.ch:
			if !ok {
				siram()
				return
			}
			kelompok = append(kelompok, e)
			if len(kelompok) >= p.maksKelompok {
				siram()
			}
		case <-tik.C:
			siram()
		}
	}
}

// tulis menulis satu kelompok; bila gagal dicoba sekali lagi, selebihnya kelompok itu dibuang dan dihitung sebagai gagal.
func (p *Perekam) tulis(es []Entri) {
	ctx, batal := context.WithTimeout(context.Background(), 20*time.Second)
	defer batal()
	err := p.simpan(ctx, es)
	if err != nil {
		time.Sleep(500 * time.Millisecond)
		err = p.simpan(ctx, es)
	}
	if err != nil {
		p.gagal.Add(int64(len(es)))
		log.Printf("[AUDIT ERROR] gagal menulis %d entri audit: %v", len(es), err)
		return
	}
	p.ditulis.Add(int64(len(es)))
}

// Berhenti berhenti menerima entri baru, menuliskan sisa antrean, lalu selesai (atau menyerah bila ctx habis).
func (p *Perekam) Berhenti(ctx context.Context) {
	p.mu.Lock()
	if !p.tutup {
		p.tutup = true
		close(p.ch)
	}
	p.mu.Unlock()
	select {
	case <-p.selesai:
	case <-ctx.Done():
	}
}

// Default: perekam proses ini. Nil (mis. pada tes) berarti middleware tidak mencatat apa pun.
var (
	muDefault sync.RWMutex
	defaultP  *Perekam
)

// Pasang menetapkan perekam proses ini (nil untuk mematikan).
func Pasang(p *Perekam) {
	muDefault.Lock()
	defaultP = p
	muDefault.Unlock()
}

func perekam() *Perekam {
	muDefault.RLock()
	defer muDefault.RUnlock()
	return defaultP
}

// StatistikDefault: statistik perekam proses ini (nol bila belum dipasang).
func StatistikDefault() Statistik {
	if p := perekam(); p != nil {
		return p.Statistik()
	}
	return Statistik{}
}

// Mulai memasang dan menjalankan perekam proses ini yang menulis ke database, dan mengembalikan penyimpanannya (untuk pembacaan oleh handler).
func Mulai(db *sql.DB) *Store {
	st := &Store{DB: db}
	p := NewPerekam(st.Simpan, 4096)
	p.Mulai()
	Pasang(p)
	return st
}

// Berhenti menyiram sisa antrean perekam proses ini (dipanggil saat server dimatikan).
func Berhenti(ctx context.Context) {
	if p := perekam(); p != nil {
		p.Berhenti(ctx)
	}
}

// MulaiPembersihan menghapus entri yang lebih lama dari retensiHari hari sekali sehari (dan sekali saat mulai) sampai ctx dibatalkan. retensiHari <= 0: tidak dibersihkan.
func MulaiPembersihan(ctx context.Context, st *Store, retensiHari int) {
	if retensiHari <= 0 || st == nil {
		return
	}
	go func() {
		bersihkan := func() {
			c, batal := context.WithTimeout(ctx, 10*time.Minute)
			defer batal()
			n, err := st.HapusSebelum(c, time.Now().AddDate(0, 0, -retensiHari))
			if err != nil {
				log.Println("[AUDIT ERROR] gagal membersihkan log audit lama:", err)
				return
			}
			if n > 0 {
				log.Printf("[AUDIT] %d entri lebih dari %d hari dihapus", n, retensiHari)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Minute): // beri waktu aplikasi selesai menyala
		}
		bersihkan()
		tik := time.NewTicker(24 * time.Hour)
		defer tik.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tik.C:
				bersihkan()
			}
		}
	}()
}
