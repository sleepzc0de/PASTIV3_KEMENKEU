package sapa

import (
	"context"
	"strings"
	"time"
)

// Repo adalah penyimpanan data SAPA. Implementasi SQL ada di store.go; tes memakai implementasi dalam memori.
type Repo interface {
	// pengguna: peran dan cakupan datanya berasal dari peran data aplikasi (paket peran), bukan dari SAPA
	NamaPengguna(ctx context.Context, userID string) (string, error) // nama lengkap; kosong bila pengguna tidak dikenal

	// usulan penjualan
	BuatPenjualan(ctx context.Context, in BuatInput) (KasusInfo, error)
	AmbilPenjualan(ctx context.Context, id int64) (*KasusInfo, error)        // nil bila tidak ada
	AmbilPenjualanUUID(ctx context.Context, uuid string) (*KasusInfo, error) // nil bila tidak ada; uuid sudah dibakukan (huruf kecil)
	DaftarPenjualan(ctx context.Context, scope Scope, f FilterDaftar) ([]KasusInfo, int, error)
	StatusTahapBanyak(ctx context.Context, ids []int64) (map[int64]StatusTahap, error)
	TahapPenjualan(ctx context.Context, id int64) (map[string]TahapRow, error)
	SimpanTahap(ctx context.Context, id int64, t TahapRow) error
	// HapusPenjualan menghapus usulan beserta tahap dan dokumen hasilnya, tetapi HANYA bila belum selesai (pemeriksaan dan penghapusan dalam satu pernyataan, supaya
	// usulan yang baru saja diselesaikan tidak ikut terhapus). Mengembalikan false bila usulan tidak ada atau sudah selesai.
	HapusPenjualan(ctx context.Context, id int64) (bool, error)

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
	CariSatker(ctx context.Context, kode18 string) (*SatkerInfo, error)        // dari data Digitalisasi Aset; nil bila tidak ada
	SatkerDenganKode6(ctx context.Context, kode6 string) ([]SatkerInfo, error) // satker (induk lebih dulu) berkode 6 digit itu pada data Digitalisasi Aset

	// jenis BMN dan satuan jumlahnya (diatur admin)
	AmbilRefBMN(ctx context.Context) (RefBMN, error) // semua, termasuk yang nonaktif
	SimpanSatuanBMN(ctx context.Context, s SatuanBMN, oleh string) error
	HapusSatuanBMN(ctx context.Context, nama string) error             // juga menghapus pemetaannya ke jenis
	SimpanJenisBMN(ctx context.Context, j JenisBMN, oleh string) error // menggantikan seluruh pemetaan satuan jenis itu
	HapusJenisBMN(ctx context.Context, nama string) error
}

// Scope membatasi daftar usulan yang boleh dilihat. Kode-kodenya berasal dari kode satker lengkap pada usulan.
type Scope struct {
	Semua    bool
	Kode6    string // usulan milik satker ini (karakter ke-10 sampai ke-15 kode satker)
	Kanwil9  string // usulan di bawah Kanwil ini (9 karakter pertama kode satker)
	KodeUE1  string // usulan di bawah UE1 ini (5 karakter pertama)
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
	case PeranPenggunaBarang:
		return Scope{Semua: true}
	case PeranSatker:
		return Scope{Kode6: strings.TrimSpace(i.KodeSatker)}
	case PeranUE1:
		return Scope{KodeUE1: strings.TrimSpace(i.KodeUE1)}
	case PeranKanwil:
		return Scope{Kanwil9: strings.TrimSpace(i.KodeKanwil)}
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

// KasusInfo adalah satu usulan penjualan. Klien hanya mengenal UUID (dikirim sebagai "id"): nomor id berurutan adalah
// kunci internal dan tidak pernah dikirim, supaya alamat usulan lain tidak bisa ditebak.
type KasusInfo struct {
	ID             int64     `json:"-"`
	UUID           string    `json:"id"`
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
	PenjualanID int64     `json:"-"` // kunci internal
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
