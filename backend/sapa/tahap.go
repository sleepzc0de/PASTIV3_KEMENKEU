package sapa

import (
	"errors"
	"strings"
)

// Peran SAPA (berbeda dari peran aplikasi user/admin/superadmin): menentukan tahap mana yang boleh dikerjakan.
const (
	PeranSatker = "satker"
	PeranKanwil = "kanwil"
	PeranUE1    = "ue1"
)

var PeranValid = []string{PeranSatker, PeranKanwil, PeranUE1}

func PeranLabel(p string) string {
	switch p {
	case PeranSatker:
		return "Satuan Kerja"
	case PeranKanwil:
		return "Kantor Wilayah"
	case PeranUE1:
		return "Unit Eselon I"
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
		Keterangan: "Nota Dinas usulan Satker kepada Unit Eselon I, beserta daftar barang, checklist kelengkapan, dan surat-surat pernyataan."},
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
			return errors.New("tahap \"" + t.Label + "\" sudah selesai; tahap ini tidak bisa diubah lagi (minta admin membuka ulang tahap sesudahnya)")
		}
	}
	return nil
}

// ---------------------------------------------------------------- hak akses

// Identitas adalah pengguna yang sedang bekerja di SAPA.
type Identitas struct {
	UserID     string
	Nama       string
	Admin      bool   // peran aplikasi admin atau superadmin
	Peran      string // peran SAPA; kosong bila belum ditetapkan
	KodeSatker string // untuk peran satker
	KodeUE1    string // untuk peran ue1
}

// Kode18 mengambil 18 digit pertama kode satker ("015010199409294002KP" -> "015010199409294002").
func Kode18(kode string) string {
	k := strings.ToUpper(strings.TrimSpace(kode))
	if len(k) > 18 {
		k = k[:18]
	}
	return k
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

var ErrTanpaPeran = errors.New("akun Anda belum ditetapkan perannya di SAPA; hubungi admin")

func (i Identitas) punyaAkses() error {
	if i.Admin {
		return nil
	}
	switch i.Peran {
	case PeranSatker:
		if Kode18(i.KodeSatker) == "" {
			return errors.New("peran Satuan Kerja Anda belum memiliki kode satker; hubungi admin")
		}
	case PeranUE1:
		if strings.TrimSpace(i.KodeUE1) == "" {
			return errors.New("peran Unit Eselon I Anda belum memiliki kode UE1; hubungi admin")
		}
	case PeranKanwil:
	default:
		return ErrTanpaPeran
	}
	return nil
}

// Akses memeriksa apakah pengguna boleh masuk ke SAPA.
func Akses(i Identitas) error { return i.punyaAkses() }

// Terlihat: apakah usulan boleh dilihat pengguna. Admin melihat semuanya; satker hanya satkernya sendiri; UE1 hanya
// usulan di bawah UE1-nya; Kantor Wilayah melihat semua usulan (kode wilayah belum ada di data).
func Terlihat(i Identitas, k Kasus) bool {
	if i.punyaAkses() != nil {
		return false
	}
	if i.Admin {
		return true
	}
	switch i.Peran {
	case PeranSatker:
		return Kode18(i.KodeSatker) == Kode18(k.KodeSatker)
	case PeranUE1:
		return strings.TrimSpace(i.KodeUE1) == k.KodeUE1
	case PeranKanwil:
		return true
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
	if Kode18(i.KodeSatker) != Kode18(kodeSatker) {
		return errors.New("Anda hanya dapat membuat usulan untuk satker Anda sendiri")
	}
	return nil
}
