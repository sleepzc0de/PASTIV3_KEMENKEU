package apiakses

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"pasti-v3-backend/serverlog"
)

// Sumber: data yang dibutuhkan gerbang untuk menilai permintaan (Store, atau tiruan pada tes).
type Sumber interface {
	CariKredensial(ctx context.Context, prefix string) (*Kredensial, error)
	MuatPengaturan(ctx context.Context) (Pengaturan, error)
}

// Waktu tunggu cache. Pencabutan kunci atau klien berlaku seketika di proses ini (BuangCache dipanggil setelah setiap perubahan lewat aplikasi) dan paling lambat
// sebanyak ttlKredensial detik bila diubah langsung di database atau instance lain.
const (
	ttlKredensial       = 10 * time.Second
	ttlNegatif          = 5 * time.Second
	ttlPengaturan       = 5 * time.Second
	maksCacheNegatif    = 10000
	maksCacheKredensial = 5000
)

type tersimpan struct {
	k           *Kredensial
	kedaluwarsa time.Time
}

// Layanan menggabungkan penyimpanan, pembatas, pencatat, dan cache gerbang API Akses Data. Satu per proses.
type Layanan struct {
	Store    *Store
	Sumber   Sumber
	Pembatas *Pembatas
	Pencatat *Pencatat
	Gagal    *PenjagaGagal

	kunciHash []byte
	hashDummy string // pembanding untuk kunci yang tidak dikenal, supaya pekerjaan per permintaan sama
	sekarang  func() time.Time

	mu            sync.Mutex
	pengaturan    Pengaturan
	muatTerakhir  time.Time
	pengaturanAda bool
	cache         map[string]tersimpan
	negatif       map[string]time.Time
	dipakai       map[int64]Pakai

	henti       chan struct{}
	selesai     chan struct{}
	mulai       sync.Once
	hentiSekali sync.Once
}

// NewLayanan membuat layanan. store boleh nil pada tes (Sumber dan Pencatat diisi sendiri). pepper: rahasia server untuk hash kunci (jangan kosong).
func NewLayanan(store *Store, sumber Sumber, pencatat *Pencatat, pepper string) *Layanan {
	kh := KunciHash(pepper)
	return &Layanan{
		Store: store, Sumber: sumber, Pembatas: NewPembatas(), Pencatat: pencatat, Gagal: NewPenjagaGagal(30, 10*time.Minute, 50000),
		kunciHash: kh, hashDummy: HashKunci(kh, "pasti_dummy"), sekarang: time.Now,
		pengaturan: PengaturanBawaan(), cache: make(map[string]tersimpan), negatif: make(map[string]time.Time), dipakai: make(map[int64]Pakai),
		henti: make(chan struct{}), selesai: make(chan struct{}),
	}
}

// HashKunci menghitung hash kunci dengan rahasia server layanan ini.
func (l *Layanan) HashKunci(kunci string) string { return HashKunci(l.kunciHash, kunci) }

// Pengaturan mengembalikan bawaan global yang berlaku, dibaca ulang dari database paling cepat tiap ttlPengaturan. Galat baca mempertahankan nilai terakhir yang baik.
func (l *Layanan) Pengaturan(ctx context.Context) Pengaturan {
	now := l.sekarang()
	l.mu.Lock()
	if l.pengaturanAda && now.Sub(l.muatTerakhir) < ttlPengaturan {
		p := l.pengaturan
		l.mu.Unlock()
		return p
	}
	l.mu.Unlock()
	if l.Sumber == nil {
		return l.pengaturan
	}
	p, err := l.Sumber.MuatPengaturan(ctx)
	l.mu.Lock()
	defer l.mu.Unlock()
	if err != nil {
		if !l.pengaturanAda {
			log.Println("[API-AKSES WARN] pengaturan tidak terbaca, memakai nilai pabrik:", err)
			l.pengaturanAda = true // jangan mencoba lagi pada setiap permintaan; coba lagi setelah ttlPengaturan
			l.muatTerakhir = now
		}
		return l.pengaturan
	}
	l.pengaturan, l.muatTerakhir, l.pengaturanAda = p, now, true
	return p
}

// TerapkanPengaturan menyimpan pengaturan baru dan langsung memakainya.
func (l *Layanan) TerapkanPengaturan(ctx context.Context, p Pengaturan, oleh string) error {
	if l.Store == nil {
		return errors.New("penyimpanan tidak tersedia")
	}
	if err := l.Store.SimpanPengaturan(ctx, p, oleh); err != nil {
		return err
	}
	l.mu.Lock()
	l.pengaturan, l.muatTerakhir, l.pengaturanAda = p, l.sekarang(), true
	l.mu.Unlock()
	return nil
}

// BuangCache membuang cache kredensial: dipanggil setelah klien atau kunci diubah supaya perubahan (terutama pencabutan) berlaku seketika.
func (l *Layanan) BuangCache() {
	l.mu.Lock()
	l.cache = make(map[string]tersimpan)
	l.negatif = make(map[string]time.Time)
	l.mu.Unlock()
}

// cari mencari kredensial menurut prefix: cache dulu, lalu database (bila bolehDB). nil bila tidak ada.
func (l *Layanan) cari(ctx context.Context, prefix string, bolehDB bool) (*Kredensial, error) {
	now := l.sekarang()
	l.mu.Lock()
	if t, ok := l.cache[prefix]; ok && now.Before(t.kedaluwarsa) {
		l.mu.Unlock()
		return t.k, nil
	}
	if t, ok := l.negatif[prefix]; ok && now.Before(t) {
		l.mu.Unlock()
		return nil, nil
	}
	l.mu.Unlock()
	if !bolehDB || l.Sumber == nil {
		return nil, nil
	}
	k, err := l.Sumber.CariKredensial(ctx, prefix)
	if err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if k == nil {
		if len(l.negatif) >= maksCacheNegatif {
			l.negatif = make(map[string]time.Time)
		}
		l.negatif[prefix] = now.Add(ttlNegatif)
		return nil, nil
	}
	if len(l.cache) >= maksCacheKredensial {
		l.cache = make(map[string]tersimpan)
	}
	l.cache[prefix] = tersimpan{k: k, kedaluwarsa: now.Add(ttlKredensial)}
	return k, nil
}

// catatDipakai menyimpan "terakhir dipakai" sampai ditulis berkala.
func (l *Layanan) catatDipakai(kunciID int64, ip string, t time.Time) {
	l.mu.Lock()
	l.dipakai[kunciID] = Pakai{KunciID: kunciID, Waktu: t, IP: ip}
	l.mu.Unlock()
}

// BatasKlien: batas yang berlaku bagi klien (override klien atau bawaan global saat ini).
func (l *Layanan) BatasKlien(ctx context.Context, o Override) BatasEfektif {
	return l.Pengaturan(ctx).BatasUntuk(o)
}

// PemakaianKlien: keadaan batas laju dan kuota satu klien sekarang.
type PemakaianKlien struct {
	KlienID       int64        `json:"klien_id"`
	Kode          string       `json:"kode"`
	Nama          string       `json:"nama"`
	Batas         BatasEfektif `json:"batas"`
	LimitSisa     int          `json:"limit_sisa"`
	KuotaTerpakai int          `json:"kuota_terpakai"`
	KuotaSisa     int          `json:"kuota_sisa"`
	KuotaReset    time.Time    `json:"kuota_reset"`
	KuotaPersen   float64      `json:"kuota_persen"`
	Override      bool         `json:"override"` // memakai batas khusus klien
}

// PemakaianSaatIni menghitung pemakaian batas laju dan kuota semua klien yang disetujui (tanpa memakai jatah).
func (l *Layanan) PemakaianSaatIni(ctx context.Context) ([]PemakaianKlien, error) {
	if l.Store == nil {
		return nil, errors.New("penyimpanan tidak tersedia")
	}
	ks, err := l.Store.DaftarAktif(ctx)
	if err != nil {
		return nil, err
	}
	p := l.Pengaturan(ctx)
	out := make([]PemakaianKlien, 0, len(ks))
	for _, k := range ks {
		o := k.Override()
		out = append(out, l.pemakaian(k.ID, k.Kode, k.NamaAplikasi, p.BatasUntuk(o), o != Override{}))
	}
	return out, nil
}

func (l *Layanan) pemakaian(id int64, kode, nama string, b BatasEfektif, override bool) PemakaianKlien {
	d := l.Pembatas.Lihat(id, b.KeBatas())
	pu := PemakaianKlien{KlienID: id, Kode: kode, Nama: nama, Batas: b, LimitSisa: d.LimitSisa, KuotaTerpakai: d.KuotaTerpakai, KuotaSisa: d.KuotaSisa, KuotaReset: d.KuotaReset, Override: override}
	if b.KuotaPermintaan > 0 {
		pu.KuotaPersen = float64(d.KuotaTerpakai) / float64(b.KuotaPermintaan) * 100
	}
	return pu
}

// PemakaianSatu: pemakaian satu klien (untuk pemilik klien dan rincian di halaman superadmin).
func (l *Layanan) PemakaianSatu(ctx context.Context, id int64, kode, nama string, o Override) PemakaianKlien {
	return l.pemakaian(id, kode, nama, l.Pengaturan(ctx).BatasUntuk(o), o != Override{})
}

// TerbitkanKunci membuat kunci baru untuk klien yang disetujui dan mengembalikan kunci lengkap (SATU-SATUNYA kesempatan melihatnya), info kuncinya, dan galat.
// rotasi: kunci aktif lain dihentikan (langsung atau setelah masa tenggang menurut pengaturan). label opsional.
func (l *Layanan) TerbitkanKunci(ctx context.Context, klienID int64, label, oleh string, rotasi bool) (string, KunciInfo, error) {
	if l.Store == nil {
		return "", KunciInfo{}, errors.New("penyimpanan tidak tersedia")
	}
	p := l.Pengaturan(ctx)
	var kad *time.Time
	if p.KunciBerlakuHari > 0 {
		t := l.sekarang().Add(time.Duration(p.KunciBerlakuHari) * 24 * time.Hour).UTC().Truncate(time.Second)
		kad = &t
	}
	var lastErr error
	for i := 0; i < 5; i++ {
		kunci, prefix, err := BuatKunci()
		if err != nil {
			return "", KunciInfo{}, err
		}
		id, err := l.Store.TerbitkanKunci(ctx, BuatKunciBaru{
			KlienID: klienID, Prefix: prefix, Hash: l.HashKunci(kunci), Label: label, Oleh: oleh, KedaluwarsaAt: kad,
			CabutLama: rotasi, Tenggang: time.Duration(p.RotasiTenggangJam) * time.Hour,
		})
		if err == nil {
			l.BuangCache()
			return kunci, KunciInfo{ID: id, Prefix: prefix, Label: label, DibuatAt: l.sekarang().UTC(), DibuatOleh: oleh, KedaluwarsaAt: kad, Aktif: true}, nil
		}
		if !adaGalatUnik(err) {
			return "", KunciInfo{}, err
		}
		lastErr = err
	}
	return "", KunciInfo{}, lastErr
}

// ---------------------------------------------------------------- latar belakang

// Mulai memulihkan kuota yang tersimpan lalu menjalankan pekerjaan berkala (penyalinan kuota dan "terakhir dipakai" tiap 5 detik, pembersihan memori tiap menit,
// pembersihan data lama tiap hari). Hanya sekali.
func (l *Layanan) Mulai() {
	l.mulai.Do(func() {
		if l.Store != nil {
			ctx, batal := context.WithTimeout(context.Background(), 15*time.Second)
			ks, err := l.Store.MuatKuota(ctx)
			batal()
			if err != nil {
				log.Println("[API-AKSES WARN] gagal memulihkan kuota tersimpan:", err)
			}
			for _, k := range ks {
				l.Pembatas.Pulihkan(k)
			}
		}
		if l.Pencatat != nil {
			l.Pencatat.Mulai()
		}
		go l.jalankan()
	})
}

func (l *Layanan) jalankan() {
	defer close(l.selesai)
	cepat := time.NewTicker(5 * time.Second)
	menit := time.NewTicker(time.Minute)
	harian := time.NewTicker(24 * time.Hour)
	awal := time.NewTimer(3 * time.Minute)
	defer cepat.Stop()
	defer menit.Stop()
	defer harian.Stop()
	defer awal.Stop()
	for {
		select {
		case <-l.henti:
			return
		case <-cepat.C:
			serverlog.Aman("API-AKSES", l.simpanBerkala)
		case <-menit.C:
			l.Pembatas.Bersihkan()
		case <-awal.C:
			serverlog.Aman("API-AKSES", l.bersihkanLama)
		case <-harian.C:
			serverlog.Aman("API-AKSES", l.bersihkanLama)
		}
	}
}

// simpanBerkala menyalin penghitung kuota yang berubah dan waktu terakhir pemakaian kunci ke database.
func (l *Layanan) simpanBerkala() {
	if l.Store == nil {
		return
	}
	ctx, batal := context.WithTimeout(context.Background(), 15*time.Second)
	defer batal()
	if ss := l.Pembatas.SnapshotKotor(); len(ss) > 0 {
		if err := l.Store.SimpanKuota(ctx, ss); err != nil {
			l.Pembatas.TandaiKotor(ss)
			log.Println("[API-AKSES ERROR] gagal menyimpan penghitung kuota:", err)
		}
	}
	l.mu.Lock()
	ps := make([]Pakai, 0, len(l.dipakai))
	for _, p := range l.dipakai {
		ps = append(ps, p)
	}
	l.dipakai = make(map[int64]Pakai)
	l.mu.Unlock()
	if len(ps) > 0 {
		if err := l.Store.TandaiDipakai(ctx, ps); err != nil {
			log.Println("[API-AKSES ERROR] gagal menyimpan waktu terakhir pemakaian kunci:", err)
		}
	}
}

// bersihkanLama menghapus riwayat dan hitungan yang melewati masa simpan menurut pengaturan.
func (l *Layanan) bersihkanLama() {
	if l.Store == nil {
		return
	}
	ctx, batal := context.WithTimeout(context.Background(), 10*time.Minute)
	defer batal()
	p := l.Pengaturan(ctx)
	now := l.sekarang()
	if n, err := l.Store.HapusLogSebelum(ctx, now.AddDate(0, 0, -p.LogRetensiHari)); err != nil {
		log.Println("[API-AKSES ERROR] gagal membersihkan riwayat permintaan lama:", err)
	} else if n > 0 {
		log.Printf("[API-AKSES] %d riwayat permintaan lebih dari %d hari dihapus", n, p.LogRetensiHari)
	}
	if n, err := l.Store.HapusAgregatSebelum(ctx, now.AddDate(0, 0, -p.AgregatRetensiHari)); err != nil {
		log.Println("[API-AKSES ERROR] gagal membersihkan hitungan per jam lama:", err)
	} else if n > 0 {
		log.Printf("[API-AKSES] %d baris hitungan per jam lebih dari %d hari dihapus", n, p.AgregatRetensiHari)
	}
}

// Berhenti menghentikan pekerjaan berkala dan menuliskan sisa kuota, waktu pemakaian, serta antrean pencatat (dipanggil saat server dimatikan).
func (l *Layanan) Berhenti(ctx context.Context) {
	l.mulai.Do(func() { close(l.selesai) }) // Mulai tidak pernah dipanggil: tidak ada penjalan yang perlu ditunggu
	l.hentiSekali.Do(func() { close(l.henti) })
	select {
	case <-l.selesai:
	case <-time.After(2 * time.Second):
	case <-ctx.Done():
	}
	l.simpanBerkala()
	if l.Pencatat != nil {
		l.Pencatat.Berhenti(ctx)
	}
}
