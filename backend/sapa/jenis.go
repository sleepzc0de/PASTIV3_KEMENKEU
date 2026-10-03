package sapa

import (
	"embed"
	"errors"
)

// Kunci jenis dokumen (juga kunci template).
const (
	DokSKTim    = "sk_tim"
	DokBA       = "ba_penelitian"
	DokNDSatker = "nd_satker"
	DokNDUE1    = "nd_ue1"
)

//go:embed templates/*.docx
var templatesBawaan embed.FS

// Penanda menjelaskan satu penanda <<...>> yang dikenali pada template.
type Penanda struct {
	Nama       string `json:"nama"`
	Keterangan string `json:"keterangan"`
	Ulang      bool   `json:"ulang,omitempty"` // penanda pada baris tabel yang diulang per item
}

// JenisDokumen: dokumen yang dihasilkan SAPA dari template Word.
type JenisDokumen struct {
	Kunci   string    `json:"kunci"`
	Label   string    `json:"label"`
	Tahap   string    `json:"tahap"`
	Bawaan  bool      `json:"bawaan"` // ada template bawaan di dalam aplikasi
	Catatan string    `json:"catatan,omitempty"`
	Penanda []Penanda `json:"penanda"`
}

var penandaDasar = []Penanda{
	{Nama: "nama satker", Keterangan: "Nama satuan kerja pada usulan"},
	{Nama: "nomor tiket", Keterangan: "Nomor registrasi usulan di aplikasi (Noreg)"},
}

var penandaND = []Penanda{
	{Nama: "sekretaris ue1", Keterangan: "Sebutan Sekretaris Unit Eselon I (dari referensi UE1)"},
	{Nama: "nama satker", Keterangan: "Nama satuan kerja"},
	{Nama: "nama satker singkat", Keterangan: "Singkatan satker; bila kosong, kurung di sekitarnya ikut dihapus"},
	{Nama: "kota", Keterangan: "Kota/kabupaten lokasi satker"},
	{Nama: "jenis bmn", Keterangan: "Jenis BMN yang diusulkan"},
	{Nama: "jumlah bmn", Keterangan: "Jumlah BMN (dihitung dari daftar barang)"},
	{Nama: "terbilang jumlah bmn", Keterangan: "Jumlah BMN terbilang beserta satuan yang dipilih untuk jenis BMN-nya, mis. (Tiga) bidang"},
	{Nama: "total nilai perolehan", Keterangan: "Jumlah nilai perolehan, mis. Rp1.500.000,00"},
	{Nama: "terbilang nilai perolehan", Keterangan: "Terbilang nilai perolehan, mis. (Satu Juta Lima Ratus Ribu Rupiah); juga tersedia: terbilang total nilai perolehan"},
	{Nama: "total nilai limit", Keterangan: "Jumlah nilai limit"},
	{Nama: "terbilang nilai limit", Keterangan: "Terbilang nilai limit; juga tersedia: terbilang total nilai limit"},
	{Nama: "alasan", Keterangan: "Alasan/pertimbangan penjualan"},
	{Nama: "tiket siman", Keterangan: "Nomor tiket SIMAN"},
	{Nama: "nomor tiket", Keterangan: "Sama dengan tiket siman (pada kalimat \"sesuai tiket pada Aplikasi SIMAN Nomor ...\")"},
	{Nama: "nomor ba", Keterangan: "Nomor Berita Acara Penelitian (dari checklist dokumen)"},
	{Nama: "tanggal ba", Keterangan: "Tanggal Berita Acara Penelitian"},
	{Nama: "kepala kantor wilayah", Keterangan: "Tembusan: pimpinan Kantor Wilayah"},
	{Nama: "hasil checklist", Keterangan: "Pada tabel checklist: diisi \"Ada\"/\"Tidak ada\" (beserta nomor dan tanggal) menurut nama dokumen di barisnya"},
	{Nama: "noreg aplikasi", Keterangan: "Nomor registrasi usulan di aplikasi"},
}

var penandaPenandatanganSatker = []Penanda{
	{Nama: "nama pejabat penandatangan", Keterangan: "Nama pejabat penandatangan"},
	{Nama: "nip pejabat penandatangan", Keterangan: "NIP pejabat penandatangan"},
	{Nama: "jabatan pejabat penandatangan", Keterangan: "Jabatan pejabat penandatangan"},
}

// DaftarJenis adalah semua jenis dokumen Penjualan.
var DaftarJenis = []JenisDokumen{
	{
		Kunci: DokSKTim, Label: "SK Tim Pemindahtanganan", Tahap: TahapTim, Bawaan: false,
		Catatan: "Daftar anggota dibuat dalam tabel: baris yang memuat <<nama anggota>> diulang untuk setiap anggota.",
		Penanda: append(append([]Penanda{}, penandaDasar...),
			Penanda{Nama: "kota", Keterangan: "Kota lokasi satker"},
			Penanda{Nama: "jabatan pimpinan", Keterangan: "Jabatan pimpinan unit organisasi/satuan kerja"},
			Penanda{Nama: "jenis tim", Keterangan: "Tim Internal Penjualan / Tim Persiapan Hibah / Tim Persiapan Tukar Menukar / Tim Pelaksanaan Tukar Menukar"},
			Penanda{Nama: "masa awal tugas", Keterangan: "Tanggal awal masa tugas tim, mis. 3 Oktober 2026"},
			Penanda{Nama: "masa akhir tugas", Keterangan: "Tanggal akhir masa tugas tim"},
			Penanda{Nama: "jumlah anggota", Keterangan: "Banyak anggota tim"},
			Penanda{Nama: "daftar anggota", Keterangan: "Alternatif tanpa tabel: seluruh anggota sebagai teks bernomor, satu per baris"},
			Penanda{Nama: "no", Keterangan: "Nomor urut anggota", Ulang: true},
			Penanda{Nama: "nama anggota", Keterangan: "Nama anggota", Ulang: true},
			Penanda{Nama: "nip anggota", Keterangan: "NIP anggota (terisi bila anggota dipilih dari HRIS2 atau NIP diketik; kosong bila tidak ada)", Ulang: true},
			Penanda{Nama: "jabatan anggota", Keterangan: "Jabatan anggota", Ulang: true},
			Penanda{Nama: "kedudukan", Keterangan: "Kedudukan dalam tim (Ketua, Sekretaris, Anggota)", Ulang: true},
		),
	},
	{
		Kunci: DokBA, Label: "Berita Acara Penelitian", Tahap: TahapBA, Bawaan: false,
		Penanda: append(append([]Penanda{}, penandaDasar...),
			Penanda{Nama: "bentuk pemindahtanganan", Keterangan: "Penjualan / Tukar Menukar / Hibah / Penyertaan Modal Pemerintah Pusat"},
			Penanda{Nama: "hari penelitian", Keterangan: "Nama hari, mis. Sabtu"},
			Penanda{Nama: "tanggal penelitian", Keterangan: "Angka tanggal, mis. 3"},
			Penanda{Nama: "bulan penelitian", Keterangan: "Nama bulan, mis. Oktober"},
			Penanda{Nama: "tahun penelitian", Keterangan: "Tahun, mis. 2026"},
			Penanda{Nama: "tanggal penelitian lengkap", Keterangan: "Tanggal lengkap, mis. 3 Oktober 2026"},
			Penanda{Nama: "nama tim", Keterangan: "Nama tim penelitian"},
		),
	},
	{
		Kunci: DokNDSatker, Label: "Nota Dinas Usulan Penjualan Satker", Tahap: TahapNDSatker, Bawaan: true,
		Catatan: "Satu berkas memuat Nota Dinas, Lampiran Daftar Barang (tabel berjudul \"Nama Barang\"), checklist kelengkapan " +
			"(tabel berjudul \"Jenis Data/Dokumen\") dan surat-surat pernyataan. Penanda gaya [@NomorND] dan [@TanggalND] dibiarkan untuk Nadine.",
		Penanda: append(append([]Penanda{}, penandaND...), penandaPenandatanganSatker...),
	},
	{
		Kunci: DokNDUE1, Label: "Nota Dinas Usulan Penjualan UE1", Tahap: TahapNDUE1, Bawaan: true,
		Catatan: "Struktur tabel Daftar Barang dan checklist sama dengan Nota Dinas Satker.",
		Penanda: append(append([]Penanda{}, penandaND...),
			Penanda{Nama: "nomor nd usulan satker", Keterangan: "Nomor Nota Dinas usulan Satker"},
			Penanda{Nama: "tanggal nd usulan", Keterangan: "Tanggal Nota Dinas usulan Satker; juga tersedia: tanggal usulan satker"},
			Penanda{Nama: "hal nd usulan", Keterangan: "Hal Nota Dinas usulan Satker; juga tersedia: hal usulan satker"},
			Penanda{Nama: "pejabat pengelola", Keterangan: "Tembusan: pejabat pengelola"},
			Penanda{Nama: "nama pejabat penandatangan", Keterangan: "Pada Nota Dinas UE1: sama dengan nama pejabat UE1 penandatangan"},
			Penanda{Nama: "jabatan pejabat penandatangan", Keterangan: "Pada Nota Dinas UE1: sama dengan jabatan pejabat UE1 penandatangan"},
			Penanda{Nama: "nama pejabat ue1 penandatangan", Keterangan: "Nama pejabat UE1 penandatangan"},
			Penanda{Nama: "jabatan pejabat ue1 penandatangan", Keterangan: "Jabatan pejabat UE1 penandatangan"},
		),
	},
}

// labelJenis: nama jenis dokumen untuk ditampilkan; kunci yang tidak dikenal dikembalikan apa adanya.
func labelJenis(kunci string) string {
	if j, ok := JenisByKunci(kunci); ok {
		return j.Label
	}
	return kunci
}

func JenisByKunci(kunci string) (JenisDokumen, bool) {
	for _, j := range DaftarJenis {
		if j.Kunci == kunci {
			return j, true
		}
	}
	return JenisDokumen{}, false
}

var ErrTemplateTidakAda = errors.New("template dokumen belum tersedia; minta admin mengunggahnya di Pengaturan SAPA")

// TemplateBawaan mengembalikan template bawaan aplikasi untuk jenis dokumen (bila ada).
func TemplateBawaan(kunci string) ([]byte, bool) {
	b, err := templatesBawaan.ReadFile("templates/" + kunci + ".docx")
	if err != nil {
		return nil, false
	}
	return b, true
}
