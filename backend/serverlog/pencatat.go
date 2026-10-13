package serverlog

import (
	"context"
	"database/sql"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// tekanJendela: kejadian yang sama (sidik jari sama) hanya dicetak sekali per jendela ini; sisanya dihitung dan dilaporkan pada cetakan berikutnya.
	// Ini yang menjaga log konsol (docker logs) tidak membengkak saat satu galat terjadi ribuan kali. Hitungan sebenarnya tetap tersimpan di database.
	tekanJendela = 10 * time.Second
	maksTekan    = 2000 // jumlah sidik jari yang diingat untuk penekanan; melewati ini, ingatan dikosongkan
	maksAwal     = 200  // kejadian sebelum database tersambung (saat menyala) yang ditahan untuk disimpan nanti
)

// Pencatat adalah pengganti keluaran log standar (log.SetOutput). Ia memilah setiap baris log menurut levelnya (dari tanda seperti "[INAPROC SYNC ERROR]"):
//
//   - error dan warning: dicetak ke konsol (dengan penekanan pengulangan), disamarkan dari rahasia, dan disimpan lewat Perekam;
//   - info dan debug: dibuang, kecuali pesan siklus hidup "[INFO]" polos (menyala/berhenti) yang selalu dicetak, dan semua info bila LOG_LEVEL=info atau debug.
//     Info/debug TIDAK pernah disimpan.
type Pencatat struct {
	konsol  io.Writer
	level   Level      // ambang cetak konsol
	mode    ModeSimpan // apa yang disimpan ke database
	perekam atomic.Pointer[Perekam]

	dibatalkan atomic.Int64 // galat "context canceled" yang diabaikan (klien membatalkan permintaan)

	mu    sync.Mutex // melindungi konsol, tekan, dan awal
	tekan map[string]*statTekan
	awal  []Entri
}

type statTekan struct {
	mulai   time.Time
	ditekan int
}

// NewPencatat membuat pencatat. konsol biasanya os.Stderr (tujuan bawaan log standar).
func NewPencatat(konsol io.Writer, level Level, mode ModeSimpan) *Pencatat {
	return &Pencatat{konsol: konsol, level: level, mode: mode, tekan: make(map[string]*statTekan)}
}

// Write memenuhi io.Writer untuk log standar. Tidak pernah mengembalikan galat supaya log.Print tidak terganggu.
func (p *Pencatat) Write(b []byte) (int, error) {
	n := len(b)
	baris := strings.TrimRight(string(b), "\r\n")
	if baris == "" {
		return n, nil
	}
	rest := lepasWaktu(baris)
	tag, pesan := pisahTag(rest)
	level, sumber, siklus := klasifikasi(tag)

	if level == Error || level == Warn {
		kepala, detail := pisahBaris(pesan)
		p.Catat(Entri{Level: level, Sumber: sumber, Pesan: kepala, Detail: detail})
		return n, nil
	}
	// Info dan debug: hanya dicetak (tidak disimpan), dan hanya bila ambangnya mengizinkan atau ini pesan siklus hidup.
	if siklus || level.peringkat() >= p.level.peringkat() {
		p.tulisKonsol(b)
	}
	return n, nil
}

// Catat mencatat sebuah kejadian berlevel Error atau Warn secara langsung (dipakai middleware HTTP dan penangkap panik). Level lain diabaikan.
func (p *Pencatat) Catat(e Entri) {
	if e.Level != Error && e.Level != Warn {
		return
	}
	// Pembatalan oleh klien (tab ditutup, halaman berpindah, permintaan dibatalkan) muncul sebagai galat "context canceled" pada query yang sedang berjalan. Itu bukan masalah
	// server dan, bila dicatat, akan membanjiri log dengan galat palsu; cukup dihitung.
	if strings.Contains(e.Pesan, "context canceled") {
		p.dibatalkan.Add(1)
		return
	}
	if e.Waktu.IsZero() {
		e.Waktu = time.Now()
	}
	e = siapkan(e)
	e.sidik = Sidik(e.Level, e.Sumber, e.Rute, e.Pesan)

	if e.Level.peringkat() >= p.level.peringkat() {
		if tampil, ditekan := p.bolehTampil(e.sidik, e.Waktu); tampil {
			p.tulisKonsol([]byte(formatKonsol(e, ditekan)))
		}
	}
	if p.mode.boleh(e.Level) {
		p.kirim(e)
	}
}

func (p *Pencatat) tulisKonsol(b []byte) {
	if p.konsol == nil {
		return
	}
	p.mu.Lock()
	p.konsol.Write(b)
	p.mu.Unlock()
}

// bolehTampil: apakah kejadian ini dicetak ke konsol. ditekan: berapa kejadian serupa sebelumnya yang tidak dicetak pada jendela yang baru berakhir.
func (p *Pencatat) bolehTampil(sidik string, sekarang time.Time) (tampil bool, ditekan int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	t, ada := p.tekan[sidik]
	if ada && sekarang.Sub(t.mulai) < tekanJendela {
		t.ditekan++
		return false, 0
	}
	if ada {
		ditekan = t.ditekan
	} else if len(p.tekan) >= maksTekan {
		p.tekan = make(map[string]*statTekan)
	}
	p.tekan[sidik] = &statTekan{mulai: sekarang}
	return true, ditekan
}

// kirim meneruskan kejadian ke perekam; sebelum perekam terpasang (aplikasi baru menyala) ditahan sebentar di memori.
func (p *Pencatat) kirim(e Entri) {
	if r := p.perekam.Load(); r != nil {
		r.Catat(e)
		return
	}
	p.mu.Lock()
	if r := p.perekam.Load(); r != nil {
		p.mu.Unlock()
		r.Catat(e)
		return
	}
	if len(p.awal) < maksAwal {
		p.awal = append(p.awal, e)
	}
	p.mu.Unlock()
}

// PasangPerekam menghubungkan perekam dan menyerahkan kejadian yang tertahan sejak aplikasi menyala.
func (p *Pencatat) PasangPerekam(r *Perekam) {
	p.mu.Lock()
	p.perekam.Store(r)
	tahan := p.awal
	p.awal = nil
	p.mu.Unlock()
	if r == nil {
		return
	}
	for _, e := range tahan {
		r.Catat(e)
	}
}

// Statistik perekam (nol bila belum terpasang).
func (p *Pencatat) Statistik() Statistik {
	var s Statistik
	if r := p.perekam.Load(); r != nil {
		s = r.Statistik()
	}
	s.DibatalkanKlien = p.dibatalkan.Load()
	return s
}

// formatKonsol: satu baris (ditambah rincian bila ada) untuk konsol, mirip format log standar supaya mudah dibaca bersama baris lain.
func formatKonsol(e Entri, ditekan int) string {
	var sb strings.Builder
	sb.WriteString(e.Waktu.Format("2006/01/02 15:04:05"))
	sb.WriteString(" [")
	sb.WriteString(e.Sumber)
	sb.WriteString(" ")
	sb.WriteString(e.Level.Label())
	sb.WriteString("] ")
	if e.Metode != "" || e.Rute != "" {
		sb.WriteString(e.Metode)
		sb.WriteString(" ")
		sb.WriteString(e.Rute)
		sb.WriteString(" ")
	}
	if e.Status != 0 {
		sb.WriteString(strconv.Itoa(e.Status))
		sb.WriteString(" ")
	}
	if e.DurasiMS > 0 {
		sb.WriteString("(")
		sb.WriteString(strconv.Itoa(e.DurasiMS))
		sb.WriteString("ms) ")
	}
	sb.WriteString(e.Pesan)
	if e.RequestID != "" {
		sb.WriteString(" req=")
		sb.WriteString(e.RequestID)
	}
	if e.UserID != "" {
		sb.WriteString(" user=")
		sb.WriteString(e.UserID)
	}
	if ditekan > 0 {
		sb.WriteString(" [")
		sb.WriteString(strconv.Itoa(ditekan))
		sb.WriteString(" kejadian serupa sebelumnya tidak dicetak]")
	}
	sb.WriteString("\n")
	if e.Detail != "" {
		sb.WriteString(e.Detail)
		sb.WriteString("\n")
	}
	return sb.String()
}

// lepasWaktu membuang awalan waktu bawaan log standar ("2006/01/02 15:04:05 " atau dengan mikrodetik) bila ada.
func lepasWaktu(s string) string {
	if len(s) >= 20 && s[4] == '/' && s[7] == '/' && s[10] == ' ' && s[13] == ':' && s[16] == ':' {
		i := 19
		if i < len(s) && s[i] == '.' { // dengan mikrodetik
			for i++; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
			}
		}
		return strings.TrimLeft(s[i:], " ")
	}
	return s
}

// pisahTag memisahkan tanda di awal baris: "[INAPROC SYNC ERROR] pesan" -> ("INAPROC SYNC ERROR", "pesan"). Tanpa tanda: ("", s).
func pisahTag(s string) (tag, pesan string) {
	if !strings.HasPrefix(s, "[") {
		return "", s
	}
	i := strings.IndexByte(s, ']')
	if i < 0 || i > 60 {
		return "", s
	}
	return s[1:i], strings.TrimSpace(s[i+1:])
}

// klasifikasi menentukan level dan sumber dari tanda. Kata terakhir yang menentukan (ERROR/FATAL/PANIC = error; WARN = warning; DEBUG; INFO/SUCCESS = info);
// tanda tanpa kata level dianggap info dengan sumber seluruh tanda. siklus: tanda "INFO" polos (pesan menyala/berhenti aplikasi).
func klasifikasi(tag string) (level Level, sumber string, siklus bool) {
	t := strings.ToUpper(strings.TrimSpace(tag))
	if t == "" {
		return Info, "", false
	}
	kata := strings.Fields(t)
	akhir := kata[len(kata)-1]
	sumber = strings.Join(kata[:len(kata)-1], " ")
	switch akhir {
	case "ERROR", "FATAL", "PANIC", "ERR":
		return Error, sumber, false
	case "WARN", "WARNING":
		return Warn, sumber, false
	case "DEBUG":
		return Debug, sumber, false
	case "INFO", "SUCCESS":
		return Info, sumber, t == "INFO"
	}
	return Info, t, false
}

// pisahBaris memisahkan baris pertama (pesan) dari sisanya (rincian, mis. jejak tumpukan).
func pisahBaris(s string) (kepala, detail string) {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
	}
	return s, ""
}

// ---------------------------------------------------------------- pencatat proses ini

// std: pencatat proses ini. Sebelum Pasang dipanggil (mis. pada tes), mencetak ke stderr dan tidak menyimpan apa pun.
var std atomic.Pointer[Pencatat]

func init() { std.Store(NewPencatat(os.Stderr, Warn, SimpanMati)) }

func pencatat() *Pencatat { return std.Load() }

// Pasang menjadikan pencatat ini keluaran log standar proses (log.SetOutput) dan pencatat bagi Catat. level: ambang cetak konsol; mode: apa yang disimpan.
func Pasang(konsol io.Writer, level Level, mode ModeSimpan) *Pencatat {
	p := NewPencatat(konsol, level, mode)
	std.Store(p)
	log.SetOutput(p)
	return p
}

// Catat mencatat kejadian Error/Warn pada pencatat proses ini.
func Catat(e Entri) { pencatat().Catat(e) }

// StatistikDefault: statistik perekam proses ini.
func StatistikDefault() Statistik { return pencatat().Statistik() }

// Mulai memasang dan menjalankan perekam proses ini yang menulis ke database, dan mengembalikan penyimpanannya (untuk dibaca handler).
func Mulai(db *sql.DB) *Store {
	st := &Store{DB: db}
	r := NewPerekam(st.Simpan, 2048)
	r.Mulai()
	pencatat().PasangPerekam(r)
	return st
}

// Berhenti menyiram sisa antrean perekam proses ini (dipanggil saat server dimatikan).
func Berhenti(ctx context.Context) {
	if r := pencatat().perekam.Load(); r != nil {
		r.Berhenti(ctx)
	}
}
