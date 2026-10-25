package apiakses

import (
	"strings"

	"pasti-v3-backend/digitalisasi"
)

// Katalog dataset yang boleh ditarik klien eksternal. Hanya dataset yang strukturnya dikenal dan kolomnya sudah dinilai yang masuk: tidak ada pembacaan generik
// dari tabel mana pun. Ada dua kelompok: data aset hasil sinkronisasi SLDK (Digitalisasi Aset), kode "aset.<dataset>", di berkas ini; dan data pengadaan Inaproc, kode
// "pengadaan.<dataset>", di katalog_pengadaan.go. Menambah dataset berarti menambah entri beserta aturan kolomnya, bukan mengubah pembaca. Katalog() hanya memuat dataset aset;
// SemuaDataset() memuat keduanya.
//
// Kolom yang TIDAK pernah keluar (kolomTertutup): kolom bertanda Sensitive pada definisi dataset (data pribadi, mis. nama penghuni rumah negara), tautan foto
// (alamat berkas internal), dan nama pengguna bangunan. Kolom teknis tabel (alamat sinkronisasi, dll.) juga tidak dipetakan sama sekali.

const awalanAset = "aset."

// KolomAPI: satu kolom yang keluar lewat API.
type KolomAPI struct {
	Nama  string `json:"nama"` // nama pada JSON (huruf kecil)
	Tipe  string `json:"tipe"` // teks | bilangan | desimal
	kolom string // nama kolom pada tabel
	jenis digitalisasi.Kind
}

// DatasetAPI: satu dataset yang dapat ditarik.
type DatasetAPI struct {
	Kode      string
	Label     string
	Deskripsi string
	Kolom     []KolomAPI
	Geo       bool
	ds        digitalisasi.Dataset
}

// kolomTertutup menentukan kolom yang tidak boleh keluar walaupun ada di tabel. Sumber utamanya bendera Sensitive pada definisi dataset; sisanya aturan tambahan di sini.
func kolomTertutup(c digitalisasi.Column) bool {
	if c.Sensitive {
		return true
	}
	n := strings.ToLower(c.Name)
	return strings.HasPrefix(n, "foto") || n == "nama_pengguna"
}

func tipeKolom(c digitalisasi.Column) string {
	switch c.Kind {
	case digitalisasi.Int, digitalisasi.BigInt:
		return "bilangan"
	case digitalisasi.Decimal, digitalisasi.Coord:
		return "desimal"
	}
	return "teks"
}

func susunDataset(ds digitalisasi.Dataset) DatasetAPI {
	d := DatasetAPI{Kode: awalanAset + ds.Key, Label: ds.Label, Deskripsi: ds.Description, Geo: ds.Geo, ds: ds}
	for _, c := range ds.Columns {
		if kolomTertutup(c) {
			continue
		}
		d.Kolom = append(d.Kolom, KolomAPI{Nama: strings.ToLower(c.Name), Tipe: tipeKolom(c), kolom: c.Name, jenis: c.Kind})
	}
	return d
}

// Katalog: semua dataset yang dapat ditarik, urut seperti Digitalisasi Aset.
func Katalog() []DatasetAPI {
	out := make([]DatasetAPI, 0, len(digitalisasi.Datasets))
	for _, ds := range digitalisasi.Datasets {
		out = append(out, susunDataset(ds))
	}
	return out
}

// CariDataset mencari dataset menurut kode ("aset.tanah").
func CariDataset(kode string) (DatasetAPI, bool) {
	k, ok := strings.CutPrefix(kode, awalanAset)
	if !ok {
		return DatasetAPI{}, false
	}
	ds, ok := digitalisasi.ByKey(k)
	if !ok {
		return DatasetAPI{}, false
	}
	return susunDataset(ds), true
}

// adaKolom: apakah kolom tabel (nama asli) termasuk yang keluar.
func (d DatasetAPI) adaKolom(nama string) bool {
	for _, k := range d.Kolom {
		if strings.EqualFold(k.kolom, nama) {
			return true
		}
	}
	return false
}

// KolomPencarian: kolom teks untuk parameter q, hanya yang juga keluar lewat API (pencarian tidak boleh menjadi jalan mengintip kolom yang disembunyikan).
func (d DatasetAPI) KolomPencarian() []string {
	var out []string
	for _, c := range d.ds.SearchColumns {
		if d.adaKolom(c) {
			out = append(out, c)
		}
	}
	return out
}

// Tabel: nama tabel sumber (dari kode, bukan masukan).
func (d DatasetAPI) Tabel() string { return d.ds.Table }

// KolomSatker: kolom kode satker lengkap (untuk penyaring dan pembatasan UE1).
func (d DatasetAPI) KolomSatker() string { return d.ds.Roles.Satker }

// KolomProvinsi dan KolomKabKota: kolom wilayah untuk penyaring (kosong bila dataset tidak punya).
func (d DatasetAPI) KolomProvinsi() string { return d.ds.Roles.Provinsi }
func (d DatasetAPI) KolomKabKota() string  { return d.ds.Roles.KabKota }

// Kunci: kunci dataset pada Digitalisasi Aset ("tanah").
func (d DatasetAPI) Kunci() string { return d.ds.Key }

// DaftarKode: kode semua dataset pada katalog gabungan (aset lalu pengadaan).
func DaftarKode() []string {
	var out []string
	for _, d := range SemuaDataset() {
		out = append(out, d.Kode)
	}
	return out
}
