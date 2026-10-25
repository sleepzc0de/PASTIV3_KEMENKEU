// Paket apiakses menyediakan "API Akses Data PASTI": aplikasi lain mendaftar, disetujui superadmin, lalu menarik data lewat kunci API yang diterbitkan PASTI.
//
// Garis besar (rinciannya ada di docs/api-akses-data.md):
//   - Kunci API (bukan JWT/SSO pengguna): kunci acak 256 bit, hanya hash HMAC-SHA256-nya yang disimpan, dibandingkan dengan waktu konstan. Terpisah penuh dari
//     autentikasi pengguna; kunci tidak pernah membuka rute pengguna dan token pengguna tidak membuka rute klien.
//   - Scope per klien (dataset yang diizinkan, opsional allowlist IP dan pembatasan UE1), kolom pribadi tidak pernah keluar (katalog.go).
//   - Batas laju (jendela geser, akurat) dan kuota (jendela tetap) per klien, nilainya dari database dan dapat diubah tanpa deploy ulang (batas.go, pengaturan.go).
//   - Setiap permintaan dicatat (log + agregat per jam) untuk halaman pemantauan; tindakan pengelolaan dan kejadian keamanan masuk Log Audit.
package apiakses

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Status siklus hidup klien.
const (
	StatusMenunggu  = "menunggu"
	StatusDisetujui = "disetujui"
	StatusDitolak   = "ditolak"
	StatusDicabut   = "dicabut"
)

// StatusSah: apakah s salah satu status klien.
func StatusSah(s string) bool {
	switch s {
	case StatusMenunggu, StatusDisetujui, StatusDitolak, StatusDicabut:
		return true
	}
	return false
}

// BolehPindah: perpindahan status yang dibolehkan. Ditolak bersifat final (pemohon mendaftar ulang); dicabut dapat diaktifkan kembali oleh superadmin.
func BolehPindah(dari, ke string) bool {
	switch dari {
	case StatusMenunggu:
		return ke == StatusDisetujui || ke == StatusDitolak
	case StatusDisetujui:
		return ke == StatusDicabut
	case StatusDicabut:
		return ke == StatusDisetujui
	}
	return false
}

// Batas isian pendaftaran.
const (
	maksNamaAplikasi = 150
	maksInstansi     = 200
	maksUnit         = 200
	maksPICNama      = 150
	maksPICJabatan   = 150
	maksPICEmail     = 200
	maksPICTelepon   = 40
	minTujuan        = 20
	maksTujuan       = 2000
	maksDatasetMinta = 64
	maksCatatan      = 1000
)

// Pendaftaran: data aplikasi klien yang dimasukkan SUPERADMIN saat mendaftarkannya (POST /api-akses/klien). Tidak ada formulir publik: pihak luar menghubungi administrator PASTI.
type Pendaftaran struct {
	NamaAplikasi   string   `json:"nama_aplikasi"`
	Instansi       string   `json:"instansi"`
	Unit           string   `json:"unit"`
	PICNama        string   `json:"pic_nama"`
	PICJabatan     string   `json:"pic_jabatan"`
	PICEmail       string   `json:"pic_email"`
	PICTelepon     string   `json:"pic_telepon"`
	Tujuan         string   `json:"tujuan"`
	DatasetDiminta []string `json:"dataset_diminta"`
}

var (
	reEmail   = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9\-]+(\.[A-Za-z0-9\-]+)*\.[A-Za-z]{2,}$`)
	reTelepon = regexp.MustCompile(`^[0-9+()\-. ]{6,40}$`)
)

// adaKarakterKendali: karakter kendali (selain tab dan baris baru bila diizinkan), termasuk NUL; teks dengan itu ditolak, bukan diam-diam dipotong.
func adaKarakterKendali(s string, izinBarisBaru bool) bool {
	for _, r := range s {
		if r == '\t' || (izinBarisBaru && (r == '\n' || r == '\r')) {
			continue
		}
		if unicode.IsControl(r) || r == ' ' || r == ' ' {
			return true
		}
	}
	return false
}

// rapikan memangkas spasi tepi dan memadatkan spasi beruntun menjadi satu (kecuali pada teks panjang yang boleh berbaris-baris).
func rapikan(s string) string { return strings.Join(strings.Fields(s), " ") }

// Bersihkan merapikan isian dan memeriksanya. Hasilnya peta kolom -> pesan galat (kosong bila sah); pesan sengaja tidak memuat isian pemohon. Daftar dataset diperiksa
// terhadap katalog (hanya kode yang dikenal), dihilangkan duplikatnya, dan diurutkan menurut katalog.
func (p *Pendaftaran) Bersihkan() map[string]string {
	g := map[string]string{}
	p.NamaAplikasi, p.Instansi, p.Unit = rapikan(p.NamaAplikasi), rapikan(p.Instansi), rapikan(p.Unit)
	p.PICNama, p.PICJabatan = rapikan(p.PICNama), rapikan(p.PICJabatan)
	p.PICEmail, p.PICTelepon = strings.TrimSpace(p.PICEmail), strings.TrimSpace(p.PICTelepon)
	p.Tujuan = strings.TrimSpace(strings.ReplaceAll(p.Tujuan, "\r\n", "\n"))

	teks := func(kolom, nilai, nama string, min, maks int, wajib bool) {
		n := utf8.RuneCountInString(nilai)
		switch {
		case nilai == "" && wajib:
			g[kolom] = nama + " wajib diisi"
		case nilai != "" && n < min:
			g[kolom] = nama + " terlalu pendek"
		case n > maks:
			g[kolom] = nama + " terlalu panjang (maksimal " + strconv.Itoa(maks) + " karakter)"
		case adaKarakterKendali(nilai, false):
			g[kolom] = nama + " memuat karakter yang tidak diizinkan"
		}
	}
	teks("nama_aplikasi", p.NamaAplikasi, "Nama aplikasi", 3, maksNamaAplikasi, true)
	teks("instansi", p.Instansi, "Instansi", 2, maksInstansi, true)
	teks("unit", p.Unit, "Unit", 2, maksUnit, false)
	teks("pic_nama", p.PICNama, "Nama PIC", 3, maksPICNama, true)
	teks("pic_jabatan", p.PICJabatan, "Jabatan PIC", 2, maksPICJabatan, false)

	switch {
	case p.PICEmail == "":
		g["pic_email"] = "Email PIC wajib diisi"
	case len(p.PICEmail) > maksPICEmail || !reEmail.MatchString(p.PICEmail):
		g["pic_email"] = "Email PIC tidak valid"
	}
	if p.PICTelepon != "" && !reTelepon.MatchString(p.PICTelepon) {
		g["pic_telepon"] = "Nomor telepon tidak valid"
	}

	n := utf8.RuneCountInString(p.Tujuan)
	switch {
	case p.Tujuan == "":
		g["tujuan"] = "Tujuan penggunaan wajib diisi"
	case n < minTujuan:
		g["tujuan"] = "Jelaskan tujuan penggunaan dengan lebih rinci (minimal " + strconv.Itoa(minTujuan) + " karakter)"
	case n > maksTujuan:
		g["tujuan"] = "Tujuan penggunaan terlalu panjang (maksimal " + strconv.Itoa(maksTujuan) + " karakter)"
	case adaKarakterKendali(p.Tujuan, true):
		g["tujuan"] = "Tujuan penggunaan memuat karakter yang tidak diizinkan"
	}

	ds, err := NormalisasiDataset(p.DatasetDiminta, maksDatasetMinta)
	switch {
	case err != nil:
		g["dataset_diminta"] = err.Error()
	case len(ds) == 0:
		g["dataset_diminta"] = "Pilih sedikitnya satu dataset"
	}
	p.DatasetDiminta = ds
	return g
}

// NormalisasiDataset memeriksa daftar kode dataset terhadap katalog gabungan (aset dan pengadaan): hanya kode yang dikenal, tanpa duplikat, urut menurut katalog. Kode yang
// tidak dikenal ditolak (bukan diabaikan) supaya salah ketik atau percobaan memasukkan kode lain tidak lolos diam-diam.
func NormalisasiDataset(kode []string, maks int) ([]string, error) {
	if len(kode) > maks {
		return nil, errors.New("Terlalu banyak dataset dipilih")
	}
	ada := map[string]bool{}
	for _, k := range kode {
		k = strings.TrimSpace(k)
		if !DatasetDikenal(k) {
			return nil, errors.New("Ada dataset yang tidak dikenal")
		}
		ada[k] = true
	}
	var out []string
	for _, d := range SemuaDataset() {
		if ada[d.Kode] {
			out = append(out, d.Kode)
		}
	}
	return out, nil
}

// Klien: satu aplikasi klien (tampilan untuk superadmin).
type Klien struct {
	ID               int64      `json:"id"`
	Kode             string     `json:"kode"`
	NamaAplikasi     string     `json:"nama_aplikasi"`
	Instansi         string     `json:"instansi"`
	Unit             string     `json:"unit,omitempty"`
	PICNama          string     `json:"pic_nama"`
	PICJabatan       string     `json:"pic_jabatan,omitempty"`
	PICEmail         string     `json:"pic_email"`
	PICTelepon       string     `json:"pic_telepon,omitempty"`
	Tujuan           string     `json:"tujuan"`
	DatasetDiminta   []string   `json:"dataset_diminta"`
	DatasetDiberikan []string   `json:"dataset_diberikan"`
	Status           string     `json:"status"`
	Catatan          string     `json:"catatan,omitempty"`
	IPDiizinkan      []string   `json:"ip_diizinkan"`
	UE1Diizinkan     []string   `json:"ue1_diizinkan"`
	DidaftarkanAt    time.Time  `json:"didaftarkan_at"`
	DidaftarkanIP    string     `json:"didaftarkan_ip,omitempty"`
	DiputuskanAt     *time.Time `json:"diputuskan_at,omitempty"`
	DiputuskanOleh   string     `json:"diputuskan_oleh,omitempty"`
	DiubahAt         *time.Time `json:"diubah_at,omitempty"`

	// Override batas per klien (nil = ikut bawaan global) dan batas yang berlaku sekarang.
	LimitPermintaan   *int          `json:"limit_permintaan"`
	LimitJendelaDetik *int          `json:"limit_jendela_detik"`
	KuotaPermintaan   *int          `json:"kuota_permintaan"`
	KuotaPeriodeDetik *int          `json:"kuota_periode_detik"`
	Berlaku           *BatasEfektif `json:"batas_berlaku,omitempty"`

	KunciAktif      int         `json:"kunci_aktif"`
	TerakhirDipakai *time.Time  `json:"terakhir_dipakai,omitempty"`
	Kunci           []KunciInfo `json:"kunci,omitempty"`
}

// KunciInfo: satu kunci API klien (tanpa rahasianya; hanya pengenal/prefix yang boleh terlihat).
type KunciInfo struct {
	ID                int64      `json:"id"`
	Prefix            string     `json:"prefix"`
	Label             string     `json:"label,omitempty"`
	DibuatAt          time.Time  `json:"dibuat_at"`
	DibuatOleh        string     `json:"dibuat_oleh,omitempty"`
	KedaluwarsaAt     *time.Time `json:"kedaluwarsa_at,omitempty"`
	DicabutAt         *time.Time `json:"dicabut_at,omitempty"`
	DicabutOleh       string     `json:"dicabut_oleh,omitempty"`
	AlasanCabut       string     `json:"alasan_cabut,omitempty"`
	TerakhirDipakaiAt *time.Time `json:"terakhir_dipakai_at,omitempty"`
	TerakhirIP        string     `json:"terakhir_ip,omitempty"`
	Aktif             bool       `json:"aktif"`
}

// AktifPada: kunci masih bisa dipakai pada waktu t (belum dicabut dan belum kedaluwarsa).
func (k KunciInfo) AktifPada(t time.Time) bool {
	if k.DicabutAt != nil {
		return false
	}
	return k.KedaluwarsaAt == nil || t.Before(*k.KedaluwarsaAt)
}
