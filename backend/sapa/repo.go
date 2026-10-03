package sapa

import (
	"context"
	"time"
)

// Repo adalah penyimpanan data SAPA. Implementasi SQL ada di store.go; tes memakai implementasi dalam memori.
type Repo interface {
	// pengguna dan hak akses
	PeranPengguna(ctx context.Context, userID string) (*PeranInfo, error) // nil bila pengguna tidak dikenal
	DaftarPeran(ctx context.Context, q string, limit int) ([]PeranRow, error)
	SimpanPeran(ctx context.Context, userID, peran, kodeSatker, kodeUE1, oleh string) error // peran kosong = hapus

	// usulan penjualan
	BuatPenjualan(ctx context.Context, in BuatInput) (KasusInfo, error)
	AmbilPenjualan(ctx context.Context, id int64) (*KasusInfo, error) // nil bila tidak ada
	DaftarPenjualan(ctx context.Context, scope Scope, f FilterDaftar) ([]KasusInfo, int, error)
	StatusTahapBanyak(ctx context.Context, ids []int64) (map[int64]StatusTahap, error)
	TahapPenjualan(ctx context.Context, id int64) (map[string]TahapRow, error)
	SimpanTahap(ctx context.Context, id int64, t TahapRow) error

	// dokumen hasil
	SimpanDokumen(ctx context.Context, d DokumenBaru) (DokumenInfo, error)
	DaftarDokumen(ctx context.Context, penjualanID int64) ([]DokumenInfo, error)
	AmbilDokumen(ctx context.Context, id int64) (*DokumenInfo, []byte, error) // nil bila tidak ada

	// template
	TemplateAktif(ctx context.Context, kunci string) ([]byte, *TemplateInfo, error) // nil bila belum ada unggahan
	SimpanTemplate(ctx context.Context, in TemplateBaru) (TemplateInfo, error)
	RiwayatTemplate(ctx context.Context, kunci string) ([]TemplateInfo, error)

	// referensi
	AmbilRefUE1(ctx context.Context, kode string) (*RefUE1, error)
	DaftarRefUE1(ctx context.Context) ([]RefUE1, error)
	SimpanRefUE1(ctx context.Context, r RefUE1, oleh string) error
	HapusRefUE1(ctx context.Context, kode string) error
	CariSatker(ctx context.Context, kode18 string) (*SatkerInfo, error) // dari data Digitalisasi Aset; nil bila tidak ada
}

// PeranInfo: peran SAPA seorang pengguna beserta cakupannya. Nama diisi dari data pengguna bila tersedia; Peran kosong
// berarti pengguna ada tetapi belum ditetapkan perannya.
type PeranInfo struct {
	Nama       string `json:"nama,omitempty"`
	Peran      string `json:"peran"`
	KodeSatker string `json:"kode_satker,omitempty"`
	KodeUE1    string `json:"kode_ue1,omitempty"`
}

// PeranRow: satu baris daftar pengguna untuk penetapan peran (admin).
type PeranRow struct {
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	Nama       string `json:"nama"`
	Email      string `json:"email"`
	PeranApp   string `json:"peran_app"`
	Peran      string `json:"peran"`
	KodeSatker string `json:"kode_satker"`
	KodeUE1    string `json:"kode_ue1"`
}

// Scope membatasi daftar usulan yang boleh dilihat.
type Scope struct {
	Semua    bool
	Kode18   string // usulan milik satker ini
	KodeUE1  string // usulan di bawah UE1 ini
	TidakAda bool   // tidak boleh melihat apa pun
}

func ScopeDari(i Identitas) Scope {
	if i.punyaAkses() != nil {
		return Scope{TidakAda: true}
	}
	if i.Admin {
		return Scope{Semua: true}
	}
	switch i.Peran {
	case PeranSatker:
		return Scope{Kode18: Kode18(i.KodeSatker)}
	case PeranUE1:
		return Scope{KodeUE1: i.KodeUE1}
	case PeranKanwil:
		return Scope{Semua: true}
	}
	return Scope{TidakAda: true}
}

type BuatInput struct {
	KodeSatker string
	NamaSatker string
	KodeUE1    string
	UserID     string
	Oleh       string
}

// KasusInfo adalah satu usulan penjualan.
type KasusInfo struct {
	ID             int64     `json:"id"`
	Noreg          string    `json:"noreg"`
	KodeSatker     string    `json:"kode_satker"`
	NamaSatker     string    `json:"nama_satker"`
	KodeUE1        string    `json:"kode_ue1"`
	DibuatOleh     string    `json:"dibuat_oleh"`
	DibuatPada     time.Time `json:"dibuat_pada"`
	DiperbaruiPada time.Time `json:"diperbarui_pada"`
}

func (k KasusInfo) Kasus() Kasus {
	return Kasus{ID: k.ID, Noreg: k.Noreg, KodeSatker: k.KodeSatker, NamaSatker: k.NamaSatker, KodeUE1: k.KodeUE1}
}

type FilterDaftar struct {
	Q      string
	Status string // "", "berjalan", "selesai"
	Offset int
	Limit  int
}

// TahapRow: catatan satu tahap pada satu usulan.
type TahapRow struct {
	Kunci          string
	Status         string
	Data           []byte // JSON isian formulir; kosong untuk tahap eksternal
	Nomor          string
	Tanggal        string // YYYY-MM-DD atau kosong
	Catatan        string
	DiperbaruiOleh string
	DiperbaruiPada time.Time
}

type DokumenBaru struct {
	PenjualanID int64
	Tahap       string
	Jenis       string
	NamaFile    string
	Berkas      []byte
	Peringatan  []string
	Oleh        string
}

type DokumenInfo struct {
	ID          int64     `json:"id"`
	PenjualanID int64     `json:"penjualan_id"`
	Tahap       string    `json:"tahap"`
	Jenis       string    `json:"jenis"`
	JenisLabel  string    `json:"jenis_label"`
	NamaFile    string    `json:"nama_file"`
	Ukuran      int       `json:"ukuran"`
	Peringatan  []string  `json:"peringatan,omitempty"`
	DibuatOleh  string    `json:"dibuat_oleh"`
	DibuatPada  time.Time `json:"dibuat_pada"`
}

type TemplateBaru struct {
	Kunci    string
	NamaFile string
	Berkas   []byte
	Catatan  string
	Oleh     string
}

type TemplateInfo struct {
	ID           int64     `json:"id"`
	Kunci        string    `json:"kunci"`
	Versi        int       `json:"versi"`
	NamaFile     string    `json:"nama_file"`
	Ukuran       int       `json:"ukuran"`
	Aktif        bool      `json:"aktif"`
	Catatan      string    `json:"catatan,omitempty"`
	DiunggahOleh string    `json:"diunggah_oleh"`
	DiunggahPada time.Time `json:"diunggah_pada"`
}

type RefUE1 struct {
	Kode       string `json:"kode"`
	Nama       string `json:"nama"`
	Sekretaris string `json:"sekretaris"` // sebutan lengkap, mis. "Sekretaris Direktorat Jenderal ..."
}

// SatkerInfo: satker menurut data Digitalisasi Aset (hasil sinkronisasi SLDK).
type SatkerInfo struct {
	Kode     string `json:"kode"`
	Nama     string `json:"nama"`
	KabKota  string `json:"kab_kota"`
	Provinsi string `json:"provinsi"`
	KodeUE1  string `json:"kode_ue1"`
}
