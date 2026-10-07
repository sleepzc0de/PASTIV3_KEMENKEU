package sapa

import (
	"errors"
	"strings"
)

// Peran di SAPA diturunkan dari peran data aplikasi (paket peran), bukan ditetapkan di SAPA: Satker (kode satker 6 digit), Kanwil (kode 9 digit), UE1
// (kode 5 digit), Pengguna Barang, atau admin/superadmin. Tiga yang pertama mengerjakan tahap milik perannya; Pengguna Barang hanya melihat semua usulan.
const (
	PeranSatker         = "satker"
	PeranKanwil         = "kanwil"
	PeranUE1            = "ue1"
	PeranPenggunaBarang = "pengguna_barang"
)

// PeranValid: peran yang mengerjakan tahap (Pengguna Barang tidak punya tahap).
var PeranValid = []string{PeranSatker, PeranKanwil, PeranUE1}

func PeranLabel(p string) string {
	switch p {
	case PeranSatker:
		return "Satuan Kerja"
	case PeranKanwil:
		return "Kantor Wilayah"
	case PeranUE1:
		return "Unit Eselon I"
	case PeranPenggunaBarang:
		return "Pengguna Barang"
	}
	return p
}

// Jenis tahap: form = dikerjakan di aplikasi dan menghasilkan dokumen; eksternal = dikerjakan di aplikasi lain
// (Nadine, SIMAN); aplikasi hanya mencatat bahwa tahap itu sudah selesai.
const (
	JenisForm      = "form"
	JenisEksternal = "eksternal"
)

// Status tahap pada satu usulan. Tahap yang belum punya catatan berstatus "belum".
const (
	StatusBelum    = "belum"
	StatusDraft    = "draft"
	StatusSelesai  = "selesai"
	StatusDilewati = "dilewati"
)

// Kunci tahap Penjualan.
const (
	TahapTim          = "tim"
	TahapBA           = "ba"
	TahapNDSatker     = "nd_satker"
	TahapNadineSatker = "nadine_satker"
	TahapSimanSatker  = "siman_satker"
	TahapSimanKanwil  = "siman_kanwil"
	TahapUE1Terima    = "ue1_terima"
	TahapNDUE1        = "nd_ue1"
	TahapNadineUE1    = "nadine_ue1"
	TahapSimanUE1     = "siman_ue1"
)

// Tahap mendefinisikan satu langkah alur kerja.
type Tahap struct {
	Kunci         string   `json:"kunci"`
	Peran         string   `json:"peran"`
	Label         string   `json:"label"`
	Keterangan    string   `json:"keterangan"`
	Jenis         string   `json:"jenis"`
	Kanal         string   `json:"kanal,omitempty"` // aplikasi tempat tahap eksternal dikerjakan
	Dokumen       []string `json:"dokumen,omitempty"`
	BolehDilewati bool     `json:"boleh_dilewati"`
	// WajibNomorTanggal: tahap eksternal yang nomor dan tanggalnya dipakai tahap berikutnya.
	WajibNomorTanggal bool `json:"wajib_nomor_tanggal,omitempty"`
}

// TahapPenjualan adalah alur Penjualan, berurutan. Tahap hanya boleh dikerjakan setelah semua tahap sebelumnya
// selesai atau dilewati.
var TahapPenjualan = []Tahap{
	{Kunci: TahapTim, Peran: PeranSatker, Label: "Pembentukan Tim", Jenis: JenisForm, Dokumen: []string{DokSKTim}, BolehDilewati: true,
		Keterangan: "Membuat konsep Surat Keputusan Tim Pemindahtanganan. Lewati bila SK Tim dibuat di luar aplikasi."},
	{Kunci: TahapBA, Peran: PeranSatker, Label: "Penyusunan Berita Acara Penelitian", Jenis: JenisForm, Dokumen: []string{DokBA}, BolehDilewati: true,
		Keterangan: "Membuat konsep Berita Acara Penelitian. Lewati bila dibuat di luar aplikasi."},
	{Kunci: TahapNDSatker, Peran: PeranSatker, Label: "Penyusunan konsep Nota Dinas Usulan Penjualan Satker", Jenis: JenisForm, Dokumen: []string{DokNDSatker},
		Keterangan: "Nota Dinas usulan Satker kepada Unit Eselon I, beserta daftar barang, checklist kelengkapan, dan surat-surat pernyataan. Buat tiket di SIMAN lebih dulu: nomor tiketnya dicantumkan pada Nota Dinas."},
	{Kunci: TahapNadineSatker, Peran: PeranSatker, Label: "Penetapan Nota Dinas Usulan Satker", Jenis: JenisEksternal, Kanal: "Nadine", WajibNomorTanggal: true,
		Keterangan: "Nota Dinas ditetapkan melalui Nadine. Catat nomor dan tanggalnya."},
	{Kunci: TahapSimanSatker, Peran: PeranSatker, Label: "Pembuatan tiket SIMAN dan upload dokumen", Jenis: JenisEksternal, Kanal: "SIMAN",
		Keterangan: "Satker membuat tiket di SIMAN dan mengunggah dokumen pendukung."},
	{Kunci: TahapSimanKanwil, Peran: PeranKanwil, Label: "Penelitian tiket SIMAN", Jenis: JenisEksternal, Kanal: "SIMAN",
		Keterangan: "Kantor Wilayah meneliti tiket di SIMAN."},
	{Kunci: TahapUE1Terima, Peran: PeranUE1, Label: "Menerima usulan Satker dan melakukan penelitian", Jenis: JenisEksternal, Kanal: "Nadine dan SIMAN",
		Keterangan: "Unit Eselon I menerima usulan dan meneliti melalui Nadine dan SIMAN."},
	{Kunci: TahapNDUE1, Peran: PeranUE1, Label: "Penyusunan konsep Nota Dinas Usulan Penjualan UE1", Jenis: JenisForm, Dokumen: []string{DokNDUE1},
		Keterangan: "Nota Dinas usulan Unit Eselon I, disusun dari data usulan Satker."},
	{Kunci: TahapNadineUE1, Peran: PeranUE1, Label: "Penetapan Nota Dinas Usulan UE1", Jenis: JenisEksternal, Kanal: "Nadine",
		Keterangan: "Nota Dinas UE1 ditetapkan melalui Nadine."},
	{Kunci: TahapSimanUE1, Peran: PeranUE1, Label: "Penerusan tiket SIMAN", Jenis: JenisEksternal, Kanal: "SIMAN",
		Keterangan: "Unit Eselon I meneruskan tiket di SIMAN."},
}

func TahapByKunci(kunci string) (Tahap, int, bool) {
	for i, t := range TahapPenjualan {
		if t.Kunci == kunci {
			return t, i, true
		}
	}
	return Tahap{}, -1, false
}

func selesaiAtauDilewati(status string) bool {
	return status == StatusSelesai || status == StatusDilewati
}

// Status menyajikan status tiap tahap (kunci -> status). Kunci yang tidak ada dianggap "belum".
type StatusTahap map[string]string

func (s StatusTahap) get(kunci string) string {
	if v, ok := s[kunci]; ok && v != "" {
		return v
	}
	return StatusBelum
}

// TahapSaatIni: tahap pertama yang belum selesai atau dilewati; kosong bila semua tahap sudah selesai.
func TahapSaatIni(s StatusTahap) string {
	for _, t := range TahapPenjualan {
		if !selesaiAtauDilewati(s.get(t.Kunci)) {
			return t.Kunci
		}
	}
	return ""
}

// Selesai: seluruh tahap selesai atau dilewati.
func Selesai(s StatusTahap) bool { return TahapSaatIni(s) == "" }

// BolehDikerjakan memeriksa urutan: semua tahap sebelumnya sudah selesai/dilewati.
func BolehDikerjakan(s StatusTahap, kunci string) error {
	_, idx, ok := TahapByKunci(kunci)
	if !ok {
		return errors.New("tahap tidak dikenal")
	}
	for _, t := range TahapPenjualan[:idx] {
		if !selesaiAtauDilewati(s.get(t.Kunci)) {
			return errors.New("tahap \"" + t.Label + "\" harus diselesaikan lebih dulu")
		}
	}
	return nil
}

// BolehDiubah: tahap yang sudah selesai hanya boleh diubah (dibuat ulang) bila belum ada tahap sesudahnya yang selesai,
// supaya dokumen tahap berikutnya tidak berselisih dengan data yang baru.
func BolehDiubah(s StatusTahap, kunci string) error {
	_, idx, ok := TahapByKunci(kunci)
	if !ok {
		return errors.New("tahap tidak dikenal")
	}
	for _, t := range TahapPenjualan[idx+1:] {
		if selesaiAtauDilewati(s.get(t.Kunci)) {
			return errors.New("tahap \"" + t.Label + "\" sudah selesai; tahap ini tidak bisa diubah lagi (minta Pengguna Barang atau superadmin membuka ulang tahap sesudahnya)")
		}
	}
	return nil
}

// ---------------------------------------------------------------- hak akses

// Identitas adalah pengguna yang sedang bekerja di SAPA. Peran dan kodenya berasal dari peran data aplikasi yang sedang aktif.
type Identitas struct {
	UserID     string
	Nama       string
	Admin      bool   // bertindak sebagai admin atau superadmin (peran bawaan akun)
	Peran      string // satker | kanwil | ue1 | pengguna_barang; kosong bila akun belum diberi peran
	KodeSatker string // untuk peran satker: kode satker 6 digit (karakter ke-10 sampai ke-15 kode satker lengkap)
	KodeKanwil string // untuk peran kanwil: kode 9 digit (9 karakter pertama kode satker lengkap)
	KodeUE1    string // untuk peran ue1: kode 5 digit
}

// PeranAplikasi: peran data aplikasi yang sedang berlaku bagi pengguna (dari middleware autentikasi), bahan Identitas.
type PeranAplikasi struct {
	Admin bool   // peran efektif admin atau superadmin
	Peran string // peran data aktif: satker, kanwil, ue1, pengguna_barang, atau kosong
	Kode  string // kode peran (6, 9, atau 5 digit); kosong untuk Pengguna Barang
}

// Kode18 mengambil 18 digit pertama kode satker ("015010199409294002KP" -> "015010199409294002").
func Kode18(kode string) string {
	k := strings.ToUpper(strings.TrimSpace(kode))
	if len(k) > 18 {
		k = k[:18]
	}
	return k
}

// Kode6Dari: kode satker 6 digit (karakter ke-10 sampai ke-15) dari kode satker lengkap; kosong bila kodenya terlalu pendek.
func Kode6Dari(kode string) string {
	k := Kode18(kode)
	if len(k) < 15 {
		return ""
	}
	return k[9:15]
}

// Kanwil9Dari: kode Kanwil (9 karakter pertama) dari kode satker lengkap; kosong bila kodenya terlalu pendek.
func Kanwil9Dari(kode string) string {
	k := Kode18(kode)
	if len(k) < 9 {
		return ""
	}
	return k[:9]
}

// KodeUE1Dari: lima digit pertama kode satker.
func KodeUE1Dari(kode string) string {
	k := Kode18(kode)
	if len(k) < 5 {
		return ""
	}
	return k[:5]
}

// Kasus adalah data usulan yang dibutuhkan untuk menilai hak akses dan mengisi dokumen.
type Kasus struct {
	ID         int64
	Noreg      string
	KodeSatker string
	NamaSatker string
	KodeUE1    string
}

var ErrTanpaPeran = errors.New("akun Anda belum diberi peran (Satker, Kanwil, UE1, atau Pengguna Barang); hubungi admin")

func (i Identitas) punyaAkses() error {
	if i.Admin {
		return nil
	}
	switch i.Peran {
	case PeranSatker:
		if strings.TrimSpace(i.KodeSatker) == "" {
			return errors.New("peran Satker Anda belum memiliki kode satker; hubungi admin")
		}
	case PeranUE1:
		if strings.TrimSpace(i.KodeUE1) == "" {
			return errors.New("peran UE1 Anda belum memiliki kode UE1; hubungi admin")
		}
	case PeranKanwil:
		if strings.TrimSpace(i.KodeKanwil) == "" {
			return errors.New("peran Kanwil Anda belum memiliki kode Kanwil; hubungi admin")
		}
	case PeranPenggunaBarang:
	default:
		return ErrTanpaPeran
	}
	return nil
}

// Akses memeriksa apakah pengguna boleh masuk ke SAPA.
func Akses(i Identitas) error { return i.punyaAkses() }

// Terlihat: apakah usulan boleh dilihat pengguna. Admin dan Pengguna Barang melihat semuanya; Satker hanya satkernya sendiri (kode 6 digit); Kanwil hanya
// usulan di bawah Kanwil-nya (9 karakter pertama kode satker); UE1 hanya usulan di bawah UE1-nya.
func Terlihat(i Identitas, k Kasus) bool {
	if i.punyaAkses() != nil {
		return false
	}
	if i.Admin {
		return true
	}
	switch i.Peran {
	case PeranPenggunaBarang:
		return true
	case PeranSatker:
		return strings.TrimSpace(i.KodeSatker) != "" && Kode6Dari(k.KodeSatker) == strings.TrimSpace(i.KodeSatker)
	case PeranUE1:
		return strings.TrimSpace(i.KodeUE1) == k.KodeUE1
	case PeranKanwil:
		return strings.TrimSpace(i.KodeKanwil) != "" && Kanwil9Dari(k.KodeSatker) == strings.TrimSpace(i.KodeKanwil)
	}
	return false
}

// BolehBertindak: apakah pengguna boleh mengerjakan tahap ini pada usulan ini (harus terlihat dan perannya sesuai).
func BolehBertindak(i Identitas, k Kasus, t Tahap) bool {
	if !Terlihat(i, k) {
		return false
	}
	return i.Admin || i.Peran == t.Peran
}

// PesanTerkunci: usulan yang seluruh tahapnya sudah selesai atau dilewati terkunci total (tidak ada tahap yang dapat diubah, termasuk tahap terakhir, dan usulan tidak
// dapat dihapus) sampai kuncinya dibuka.
const PesanTerkunci = "Usulan ini sudah selesai dan terkunci. Hanya Pengguna Barang atau superadmin yang dapat membuka kuncinya."

// BolehBukaKunci: apakah pengguna boleh membuka kunci usulan, yaitu membuka ulang tahap yang sudah selesai atau dilewati. Hanya superadmin dan Pengguna Barang
// (yang memang harus bisa melihat usulannya).
func BolehBukaKunci(i Identitas, k Kasus) bool {
	return Terlihat(i, k) && (i.Admin || i.Peran == PeranPenggunaBarang)
}

// BolehMenghapus: apakah pengguna boleh menghapus usulan yang belum selesai. Satker pemilik usulan (kode satker 6 digitnya sama), Pengguna Barang, dan superadmin;
// Kanwil dan UE1 hanya memproses usulan, jadi tidak menghapusnya. Usulan yang sudah selesai tidak dapat dihapus selama masih terkunci (diperiksa pemanggil).
func BolehMenghapus(i Identitas, k Kasus) bool {
	return Terlihat(i, k) && (i.Admin || i.Peran == PeranPenggunaBarang || i.Peran == PeranSatker)
}

// BolehMembuat: pembuat usulan adalah satker untuk satkernya sendiri; admin untuk satker mana pun.
func BolehMembuat(i Identitas, kodeSatker string) error {
	if err := i.punyaAkses(); err != nil {
		return err
	}
	if i.Admin {
		return nil
	}
	if i.Peran != PeranSatker {
		return errors.New("hanya Satuan Kerja yang dapat membuat usulan penjualan")
	}
	if strings.TrimSpace(i.KodeSatker) == "" || Kode6Dari(kodeSatker) != strings.TrimSpace(i.KodeSatker) {
		return errors.New("Anda hanya dapat membuat usulan untuk satker Anda sendiri")
	}
	return nil
}
