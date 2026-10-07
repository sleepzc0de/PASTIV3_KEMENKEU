// Paket audit mencatat aktivitas pengguna (siapa melakukan apa, kapan, dari mana, dan hasilnya) ke tabel audit_log (migrasi 058) tanpa memperlambat permintaan:
// middleware menyusun satu entri per permintaan yang perlu dicatat, perekam menuliskannya ke database secara berkelompok di latar belakang, dan pembersih menghapus
// entri yang melewati masa retensi. Hanya superadmin yang dapat membacanya (lihat handlers/audit_handler.go); tidak ada rute untuk mengubah atau menghapus entri.
//
// Yang TIDAK pernah dicatat: badan permintaan, kata sandi, token, dan kode OAuth. Rincian (detail) hanya berisi parameter rute, alasan gagal, dan kata kunci pencarian
// yang sudah dibersihkan.
package audit

import "time"

// Entri: satu baris audit_log.
type Entri struct {
	ID          int64                  `json:"id"`
	Waktu       time.Time              `json:"waktu"` // UTC
	RequestID   string                 `json:"request_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	Username    string                 `json:"username,omitempty"`
	NamaLengkap string                 `json:"nama_lengkap,omitempty"` // dari tabel users saat dibaca; tidak disimpan di audit_log
	Peran       string                 `json:"peran,omitempty"`
	KodePeran   string                 `json:"kode_peran,omitempty"`
	Kategori    string                 `json:"kategori"`
	Aksi        string                 `json:"aksi"`
	Label       string                 `json:"label"`
	Metode      string                 `json:"metode"`
	Rute        string                 `json:"rute"`
	ObjekTipe   string                 `json:"objek_tipe,omitempty"`
	ObjekID     string                 `json:"objek_id,omitempty"`
	Status      int                    `json:"status_http"`
	Sukses      bool                   `json:"sukses"`
	DurasiMS    int                    `json:"durasi_ms"`
	IP          string                 `json:"ip,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	Detail      map[string]interface{} `json:"detail,omitempty"`
}

// Kategori aktivitas (nilai kolom kategori).
const (
	KatAuth         = "auth"
	KatPengguna     = "pengguna"
	KatPeran        = "peran"
	KatSapa         = "sapa"
	KatDigitalisasi = "digitalisasi"
	KatPengadaan    = "pengadaan"
	KatReferensi    = "referensi"
	KatEkspor       = "ekspor"
	KatHRIS2        = "hris2"
	KatAudit        = "audit"
	KatLainnya      = "lainnya"
)

// Kode aksi bawaan yang dikenali di luar katalog rute.
const (
	AksiLoginBerhasil = "auth.login.berhasil"
	AksiLoginGagal    = "auth.login.gagal"
	AksiDitolak       = "akses.ditolak"
)

// Penyaring: syarat pencarian log. Nilai kosong berarti tanpa syarat itu.
type Penyaring struct {
	Dari       time.Time // termasuk
	Sampai     time.Time // tidak termasuk
	UserID     string
	Username   string // mengandung (tanpa membedakan huruf besar/kecil)
	Kategori   string
	Aksi       string
	Sukses     *bool
	IP         string // mengandung
	Q          string // kata kunci pada uraian, aksi, rute, objek, pengguna, dan IP
	Halaman    int    // mulai dari 1
	PerHalaman int    // bawaan 50, maksimal 200
}

// Batas halaman.
const (
	PerHalamanBawaan = 50
	PerHalamanMaks   = 200
	MaksEkspor       = 50000
)

func (p Penyaring) halamanAman() (offset, limit int) {
	limit = p.PerHalaman
	if limit <= 0 {
		limit = PerHalamanBawaan
	}
	if limit > PerHalamanMaks {
		limit = PerHalamanMaks
	}
	h := p.Halaman
	if h < 1 {
		h = 1
	}
	return (h - 1) * limit, limit
}

// Daftar: satu halaman hasil pencarian.
type Daftar struct {
	Entri      []Entri `json:"entri"`
	Total      int64   `json:"total"`
	Halaman    int     `json:"halaman"`
	PerHalaman int     `json:"per_halaman"`
}

// TitikWaktu: jumlah aktivitas pada satu jam atau hari.
type TitikWaktu struct {
	Waktu  time.Time `json:"waktu"`
	Jumlah int64     `json:"jumlah"`
	Gagal  int64     `json:"gagal"`
}

// JumlahPer: hitungan per kode (kategori, aksi, ip).
type JumlahPer struct {
	Kode   string `json:"kode"`
	Label  string `json:"label,omitempty"`
	Jumlah int64  `json:"jumlah"`
}

// Ringkasan: angka ringkas aktivitas pada rentang waktu tertentu.
type Ringkasan struct {
	Dari          time.Time    `json:"dari"`
	Sampai        time.Time    `json:"sampai"`
	Total         int64        `json:"total"`
	Berhasil      int64        `json:"berhasil"`
	Gagal         int64        `json:"gagal"`
	PenggunaAktif int64        `json:"pengguna_aktif"`
	LoginBerhasil int64        `json:"login_berhasil"`
	LoginGagal    int64        `json:"login_gagal"`
	Ditolak       int64        `json:"ditolak"`
	Ekspor        int64        `json:"ekspor"`
	PerKategori   []JumlahPer  `json:"per_kategori"`
	AksiTeratas   []JumlahPer  `json:"aksi_teratas"`
	LoginGagalIP  []JumlahPer  `json:"login_gagal_ip"` // sumber (IP) dengan login gagal terbanyak
	Deret         []TitikWaktu `json:"deret"`
	PerJam        bool         `json:"per_jam"` // deret per jam (rentang sampai 3 hari) atau per hari
}

// RingkasanPengguna: aktivitas satu pengguna pada rentang waktu tertentu.
type RingkasanPengguna struct {
	UserID        string     `json:"user_id"`
	Username      string     `json:"username"`
	NamaLengkap   string     `json:"nama_lengkap,omitempty"`
	Email         string     `json:"email,omitempty"`
	Aktif         *bool      `json:"aktif,omitempty"` // nil bila akunnya sudah dihapus
	Jumlah        int64      `json:"jumlah"`
	Gagal         int64      `json:"gagal"`
	AktifTerakhir time.Time  `json:"aktif_terakhir"`
	LoginTerakhir *time.Time `json:"login_terakhir,omitempty"`
	IPTerakhir    string     `json:"ip_terakhir,omitempty"`
	PeranTerakhir string     `json:"peran_terakhir,omitempty"`
}

// DaftarPengguna: satu halaman ringkasan per pengguna.
type DaftarPengguna struct {
	Pengguna   []RingkasanPengguna `json:"pengguna"`
	Total      int64               `json:"total"`
	Halaman    int                 `json:"halaman"`
	PerHalaman int                 `json:"per_halaman"`
}

// Statistik perekam (untuk pemantauan): bila Dijatuhkan atau Gagal bertambah, ada aktivitas yang tidak tercatat.
type Statistik struct {
	Diterima   int64 `json:"diterima"`
	Ditulis    int64 `json:"ditulis"`
	Dijatuhkan int64 `json:"dijatuhkan"` // antrean penuh
	Gagal      int64 `json:"gagal"`      // gagal menulis ke database
	Antrean    int   `json:"antrean"`
	Kapasitas  int   `json:"kapasitas"`
}
