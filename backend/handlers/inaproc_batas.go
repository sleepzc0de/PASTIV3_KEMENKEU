package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"pasti-v3-backend/config"
)

// Pembatas permintaan ke Inaproc, dipakai SEMUA panggilan (antrean penarikan dan penjadwal).
//
// Batas Inaproc: 1.000 permintaan per 60 detik, dan kuota 5.000 permintaan yang direset tiap 1 jam. Pembatas ini menjaga:
//   - laju per menit dengan jendela geser 60 detik (daftar waktu permintaan);
//   - kuota per jam dengan jendela geser 60 menit (hitungan per menit). Jendela geser selalu lebih ketat daripada jendela tetap atau jendela yang
//     dihitung sejak permintaan pertama, jadi aman bagaimanapun Inaproc mereset kuotanya;
//   - jeda bersama (cooldown) setiap kali Inaproc menjawab 429, dengan Retry-After bila ada dan eskalasi bila berulang.
//
// Hitungan per menit disimpan di database (inaproc_kuota_menit) supaya kuota yang sudah terpakai tidak "terlupa" saat server dimulai ulang
// atau saat dua server berjalan bersamaan sebentar ketika deploy.

// ErrKuotaHabis: kuota tidak tersedia dalam waktu tunggu yang diizinkan (untuk permintaan interaktif).
var ErrKuotaHabis = errors.New("kuota permintaan Inaproc sedang habis")

const (
	jendelaMenit = 60 * time.Second
	jendelaJam   = 60 * time.Minute
	// Potongan tidur saat menunggu: pesan kemajuan diperbarui dan hitungan dihitung ulang (pemakai lain bisa menambah pemakaian).
	potonganTunggu = 30 * time.Second
	// Batas atas cooldown 429.
	maksCooldown = 70 * time.Minute
)

// StatusKuota: keadaan pembatas untuk ditampilkan.
type StatusKuota struct {
	BatasPerMenit int        `json:"batas_per_menit"`
	BatasPerJam   int        `json:"batas_per_jam"`
	TerpakaiMenit int        `json:"terpakai_menit"`
	TerpakaiJam   int        `json:"terpakai_jam"`
	SisaJam       int        `json:"sisa_jam"`
	TahanSampai   *time.Time `json:"tahan_sampai"`  // jeda bersama setelah 429; nil bila tidak ada
	PulihSekitar  *time.Time `json:"pulih_sekitar"` // saat jatah mulai longgar lagi; nil bila masih ada jatah
}

type BatasInaproc struct {
	perMenit, perJam int
	sekarang         func() time.Time
	tidurFn          func(ctx context.Context, d time.Duration) bool

	mu          sync.Mutex
	burst       []time.Time   // waktu permintaan dalam 60 detik terakhir
	menit       map[int64]int // permintaan per menit (menit unix) dalam 60 menit terakhir
	tersimpan   map[int64]int // bagian yang sudah ada di database, untuk menghitung selisih saat menyimpan
	tahanSampai time.Time
	beruntun429 int
	terakhir429 time.Time
}

// NewBatasInaproc membuat pembatas; batas <= 0 memakai bawaan 800 per menit dan 4.500 per jam.
func NewBatasInaproc(perMenit, perJam int) *BatasInaproc {
	if perMenit <= 0 {
		perMenit = 800
	}
	if perJam <= 0 {
		perJam = 4500
	}
	return &BatasInaproc{
		perMenit: perMenit, perJam: perJam, sekarang: time.Now, tidurFn: tidur,
		menit: map[int64]int{}, tersimpan: map[int64]int{},
	}
}

var (
	batasInaprocMu sync.Mutex
	batasInaproc   *BatasInaproc
)

// batas mengembalikan pembatas bersama proses ini, dibuat dari konfigurasi pada pemakaian pertama.
func batas() *BatasInaproc {
	batasInaprocMu.Lock()
	defer batasInaprocMu.Unlock()
	if batasInaproc == nil {
		var menit, jam int
		if config.Cfg != nil {
			menit, jam = config.Cfg.InaprocBatasPerMenit, config.Cfg.InaprocBatasPerJam
		}
		batasInaproc = NewBatasInaproc(menit, jam)
	}
	return batasInaproc
}

// aturBatas mengganti pembatas bersama (dipakai InitInaprocPenarikan dan tes).
func aturBatas(b *BatasInaproc) {
	batasInaprocMu.Lock()
	batasInaproc = b
	batasInaprocMu.Unlock()
}

func menitUnix(t time.Time) int64 { return t.Unix() / 60 }

// kedaluwarsaMenit: saat semua permintaan pada menit m pasti sudah lebih tua dari satu jam.
func kedaluwarsaMenit(m int64) time.Time { return time.Unix((m+1)*60, 0).Add(jendelaJam) }

// pangkasLocked membuang catatan yang sudah di luar jendela. Pemanggil memegang b.mu.
func (b *BatasInaproc) pangkasLocked(now time.Time) {
	i := 0
	for i < len(b.burst) && !b.burst[i].After(now.Add(-jendelaMenit)) {
		i++
	}
	b.burst = b.burst[i:]
	for m := range b.menit {
		if !kedaluwarsaMenit(m).After(now) {
			delete(b.menit, m)
			delete(b.tersimpan, m)
		}
	}
}

func (b *BatasInaproc) totalJamLocked() int {
	n := 0
	for _, c := range b.menit {
		n += c
	}
	return n
}

// waktuTungguLocked: berapa lama lagi sampai n permintaan baru muat dalam batas per menit dan per jam (0 = muat sekarang). n dijepit ke
// batas jam supaya permintaan besar tidak menunggu tanpa akhir.
func (b *BatasInaproc) waktuTungguLocked(now time.Time, n int) time.Duration {
	if n < 1 {
		n = 1
	}
	var tunggu time.Duration
	if d := b.tahanSampai.Sub(now); d > 0 {
		tunggu = d
	}

	// Per menit.
	nm := n
	if nm > b.perMenit {
		nm = b.perMenit
	}
	if lebih := len(b.burst) + nm - b.perMenit; lebih > 0 {
		if d := b.burst[lebih-1].Add(jendelaMenit).Sub(now); d > tunggu {
			tunggu = d
		}
	}

	// Per jam.
	nj := n
	if nj > b.perJam {
		nj = b.perJam
	}
	if total := b.totalJamLocked(); total+nj > b.perJam {
		kunci := make([]int64, 0, len(b.menit))
		for m := range b.menit {
			kunci = append(kunci, m)
		}
		sort.Slice(kunci, func(i, j int) bool { return kunci[i] < kunci[j] })
		sisa := total
		for _, m := range kunci {
			sisa -= b.menit[m]
			if sisa+nj <= b.perJam {
				if d := kedaluwarsaMenit(m).Sub(now); d > tunggu {
					tunggu = d
				}
				break
			}
		}
	}
	return tunggu
}

func (b *BatasInaproc) catatLocked(now time.Time) {
	b.burst = append(b.burst, now)
	m := menitUnix(now)
	b.menit[m]++
}

// kunciKabar: kunci context untuk fungsi pesan kemajuan tugas; pembatas memakainya untuk memberi tahu bahwa ia sedang menunggu.
type kunciKabar struct{}

func denganKabar(ctx context.Context, f func(string)) context.Context {
	return context.WithValue(ctx, kunciKabar{}, f)
}

func kabarDariCtx(ctx context.Context) func(string) {
	f, _ := ctx.Value(kunciKabar{}).(func(string))
	return f
}

func bulatkan(d time.Duration) time.Duration {
	if d < time.Second {
		return d
	}
	return d.Round(time.Second)
}

// Tunggu menahan pemanggil sampai satu permintaan boleh dikirim, lalu mencatatnya. maks > 0 membatasi lama menunggu: bila perlu menunggu
// lebih lama, langsung mengembalikan ErrKuotaHabis (permintaan interaktif tidak boleh menggantung). Dibatalkan lewat ctx.
func (b *BatasInaproc) Tunggu(ctx context.Context, maks time.Duration) error {
	var sudah time.Duration
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		b.mu.Lock()
		now := b.sekarang()
		b.pangkasLocked(now)
		tunggu := b.waktuTungguLocked(now, 1)
		if tunggu <= 0 {
			b.catatLocked(now)
			b.mu.Unlock()
			return nil
		}
		b.mu.Unlock()

		if maks > 0 && sudah+tunggu > maks {
			return ErrKuotaHabis
		}
		if f := kabarDariCtx(ctx); f != nil {
			f(fmt.Sprintf("Menunggu batas permintaan Inaproc (lanjut sekitar %s lagi)", bulatkan(tunggu)))
		}
		potong := tunggu
		if potong > potonganTunggu {
			potong = potonganTunggu
		}
		if !b.tidurFn(ctx, potong) {
			return ctx.Err()
		}
		sudah += potong
	}
}

// Pastikan menunggu sampai n permintaan akan muat dalam batas (tanpa mencatatnya). Dipakai sebelum memulai tugas besar supaya tidak
// berhenti di tengah jalan karena kuota habis, khususnya untuk dataset yang menghapus data lama lebih dulu.
func (b *BatasInaproc) Pastikan(ctx context.Context, n int) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		b.mu.Lock()
		now := b.sekarang()
		b.pangkasLocked(now)
		tunggu := b.waktuTungguLocked(now, n)
		b.mu.Unlock()
		if tunggu <= 0 {
			return nil
		}
		if f := kabarDariCtx(ctx); f != nil {
			f(fmt.Sprintf("Menunggu kuota Inaproc untuk sekitar %d permintaan (lanjut sekitar %s lagi)", n, bulatkan(tunggu)))
		}
		potong := tunggu
		if potong > potonganTunggu {
			potong = potonganTunggu
		}
		if !b.tidurFn(ctx, potong) {
			return ctx.Err()
		}
	}
}

// Laporkan429 dipanggil saat Inaproc menjawab 429: semua pemanggil ikut menunggu. retryAfter dari header bila ada; bila tidak, jeda naik
// bertahap untuk 429 yang berulang dalam 30 menit (65 detik, 5 menit, 15 menit, 60 menit). Mengembalikan lama jedanya.
func (b *BatasInaproc) Laporkan429(retryAfter time.Duration) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.sekarang()
	if now.Sub(b.terakhir429) > 30*time.Minute {
		b.beruntun429 = 0
	}
	b.beruntun429++
	b.terakhir429 = now
	d := retryAfter
	if d <= 0 {
		switch b.beruntun429 {
		case 1:
			d = 65 * time.Second
		case 2:
			d = 5 * time.Minute
		case 3:
			d = 15 * time.Minute
		default:
			d = 60 * time.Minute
		}
	}
	if d > maksCooldown {
		d = maksCooldown
	}
	if t := now.Add(d); t.After(b.tahanSampai) {
		b.tahanSampai = t
	}
	return d
}

// LaporkanBerhasil: satu permintaan berhasil, eskalasi 429 diulang dari awal.
func (b *BatasInaproc) LaporkanBerhasil() {
	b.mu.Lock()
	b.beruntun429 = 0
	b.mu.Unlock()
}

// Status: keadaan saat ini untuk ditampilkan.
func (b *BatasInaproc) Status() StatusKuota {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.sekarang()
	b.pangkasLocked(now)
	s := StatusKuota{BatasPerMenit: b.perMenit, BatasPerJam: b.perJam, TerpakaiMenit: len(b.burst), TerpakaiJam: b.totalJamLocked()}
	s.SisaJam = b.perJam - s.TerpakaiJam
	if s.SisaJam < 0 {
		s.SisaJam = 0
	}
	if b.tahanSampai.After(now) {
		t := b.tahanSampai
		s.TahanSampai = &t
	}
	if d := b.waktuTungguLocked(now, 1); d > 0 {
		t := now.Add(d)
		s.PulihSekitar = &t
	}
	return s
}

// ---- penyimpanan hitungan per menit ----

// Muat membaca hitungan satu jam terakhir dari database (dipanggil sekali saat server dimulai).
func (b *BatasInaproc) Muat(ctx context.Context, db *sql.DB) error {
	b.mu.Lock()
	dari := menitUnix(b.sekarang().Add(-jendelaJam)) - 1
	b.mu.Unlock()
	hasil, err := bacaKuotaMenit(ctx, db, dari)
	if err != nil {
		return err
	}
	b.gabungkan(hasil)
	return nil
}

func bacaKuotaMenit(ctx context.Context, db *sql.DB, dari int64) (map[int64]int, error) {
	rows, err := db.QueryContext(ctx, `SELECT menit, jumlah FROM inaproc_kuota_menit WHERE menit >= @p1`, dari)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int{}
	for rows.Next() {
		var m int64
		var n int
		if err := rows.Scan(&m, &n); err != nil {
			return nil, err
		}
		out[m] = n
	}
	return out, rows.Err()
}

// gabungkan memasukkan hitungan dari database: yang lebih besar menang (server lain mungkin ikut memakai kuota yang sama), dan hitungan itu
// dianggap sudah tersimpan.
func (b *BatasInaproc) gabungkan(db map[int64]int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.sekarang()
	for m, n := range db {
		if !kedaluwarsaMenit(m).After(now) {
			continue
		}
		if n > b.menit[m] {
			b.menit[m] = n
		}
		b.tersimpan[m] = n
	}
}

// Simpan menulis selisih hitungan ke database (ditambahkan ke angka yang ada, supaya dua server tidak saling menimpa), lalu membaca
// hitungan terbaru. Dipanggil berkala oleh Mulai.
func (b *BatasInaproc) Simpan(ctx context.Context, db *sql.DB) error {
	b.mu.Lock()
	now := b.sekarang()
	b.pangkasLocked(now)
	type delta struct {
		m int64
		n int
	}
	var tulis []delta
	for m, n := range b.menit {
		if d := n - b.tersimpan[m]; d > 0 {
			tulis = append(tulis, delta{m, d})
		}
	}
	b.mu.Unlock()

	var galat error
	for _, d := range tulis {
		_, err := db.ExecContext(ctx, `
			UPDATE inaproc_kuota_menit SET jumlah = jumlah + @p2, diubah = SYSUTCDATETIME() WHERE menit = @p1;
			IF @@ROWCOUNT = 0 INSERT INTO inaproc_kuota_menit (menit, jumlah) VALUES (@p1, @p2);`, d.m, d.n)
		if err != nil {
			galat = err
			continue
		}
		b.mu.Lock()
		b.tersimpan[d.m] += d.n
		b.mu.Unlock()
	}
	if hasil, err := bacaKuotaMenit(ctx, db, menitUnix(now.Add(-jendelaJam))-1); err == nil {
		b.gabungkan(hasil)
	} else if galat == nil {
		galat = err
	}
	// Catatan lama dibuang sesekali.
	if now.Minute()%10 == 0 {
		_, _ = db.ExecContext(ctx, `DELETE FROM inaproc_kuota_menit WHERE menit < @p1`, menitUnix(now.Add(-3*time.Hour)))
	}
	return galat
}

// Mulai memuat hitungan lama lalu menyimpan berkala sampai ctx berakhir.
func (b *BatasInaproc) Mulai(ctx context.Context, db *sql.DB, periode time.Duration) {
	cx, batal := context.WithTimeout(ctx, 30*time.Second)
	if err := b.Muat(cx, db); err != nil {
		log.Println("[INAPROC KUOTA WARN] gagal memuat hitungan kuota (dimulai dari nol):", err)
	}
	batal()
	go func() {
		t := time.NewTicker(periode)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				cx, batal := context.WithTimeout(ctx, 20*time.Second)
				if err := b.Simpan(cx, db); err != nil {
					log.Println("[INAPROC KUOTA WARN] gagal menyimpan hitungan kuota:", err)
				}
				batal()
			}
		}
	}()
}
