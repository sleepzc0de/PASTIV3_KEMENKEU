// Package analitik memuat bentuk data dasbor Pengadaan terpadu dan penyusun wawasan (penjelasan berbasis angka) dari data itu.
// Pengambilan data dari database ada di package handlers; di sini hanya logika murni, supaya bisa diuji tanpa database.
package analitik

import "time"

// Pasangan: satu kelompok (label) dengan jumlah baris dan nilai rupiah.
type Pasangan struct {
	Label  string  `json:"label"`
	Jumlah int64   `json:"jumlah"`
	Nilai  float64 `json:"nilai"`
}

// TitikBulan: satu bulan (1-12) dengan jumlah dan nilai.
type TitikBulan struct {
	Bulan  int     `json:"bulan"`
	Jumlah int64   `json:"jumlah"`
	Nilai  float64 `json:"nilai"`
}

// RUP: perencanaan (paket penyedia, swakelola, program).
type RUP struct {
	TotalPaket        int64        `json:"total_paket"`
	TotalPagu         float64      `json:"total_pagu"`
	PaketSwakelola    int64        `json:"paket_swakelola"`
	PaguSwakelola     float64      `json:"pagu_swakelola"` // dari swakelola terumumkan (daftar swakelola tidak memuat pagu)
	PaguProgram       float64      `json:"pagu_program"`
	StatusUmumkan     []Pasangan   `json:"status_umumkan"`
	PerMetode         []Pasangan   `json:"per_metode"`
	PerJenis          []Pasangan   `json:"per_jenis"`
	TopSatker         []Pasangan   `json:"top_satker"`
	PerBulanPemilihan []TitikBulan `json:"per_bulan_pemilihan"` // menurut tanggal awal pemilihan yang direncanakan, 12 bulan
	StatusUKM         []Pasangan   `json:"status_ukm"`
	StatusPDN         []Pasangan   `json:"status_pdn"`
}

// Efisiensi: selisih HPS dan nilai kontrak pada paket yang sudah selesai.
type Efisiensi struct {
	Sampel       int64      `json:"sampel"`
	TotalHPS     float64    `json:"total_hps"`
	TotalKontrak float64    `json:"total_kontrak"`
	Persen       float64    `json:"persen"`  // (HPS - kontrak) / HPS x 100 atas seluruh sampel
	Median       float64    `json:"median"`  // median efisiensi per paket (%)
	Sebaran      []Pasangan `json:"sebaran"` // jumlah paket per rentang efisiensi
}

// Persaingan: jumlah peserta per tender.
type Persaingan struct {
	TenderBerpeserta int64      `json:"tender_berpeserta"`
	SatuPeserta      int64      `json:"satu_peserta"`
	RataPeserta      float64    `json:"rata_peserta"`
	Sebaran          []Pasangan `json:"sebaran"`
}

// WaktuProses: hari dari pengumuman sampai penetapan pemenang.
type WaktuProses struct {
	Sampel int64   `json:"sampel"`
	Median float64 `json:"median"`
	Rata   float64 `json:"rata"`
}

// PasarPenyedia: sebaran nilai kontrak antar penyedia (tender dan non-tender).
type PasarPenyedia struct {
	JumlahPenyedia int64      `json:"jumlah_penyedia"`
	TotalNilai     float64    `json:"total_nilai"`
	HHI            float64    `json:"hhi"` // Herfindahl-Hirschman, 0-10.000
	Top            []Pasangan `json:"top"`
}

// Pemilihan: tender dan non-tender.
type Pemilihan struct {
	TenderJumlah      int64         `json:"tender_jumlah"`
	TenderPagu        float64       `json:"tender_pagu"`
	TenderHPS         float64       `json:"tender_hps"`
	NonTenderJumlah   int64         `json:"non_tender_jumlah"`
	NonTenderPagu     float64       `json:"non_tender_pagu"`
	NonTenderHPS      float64       `json:"non_tender_hps"`
	TenderSelesai     int64         `json:"tender_selesai"`
	NilaiKontrak      float64       `json:"nilai_kontrak"` // tender selesai + non-tender selesai
	StatusTender      []Pasangan    `json:"status_tender"`
	MetodeTender      []Pasangan    `json:"metode_tender"`
	MetodeNonTender   []Pasangan    `json:"metode_non_tender"`
	JenisTender       []Pasangan    `json:"jenis_tender"`
	PerBulanTender    []TitikBulan  `json:"per_bulan_tender"`
	PerBulanNonTender []TitikBulan  `json:"per_bulan_non_tender"`
	Efisiensi         Efisiensi     `json:"efisiensi"`
	Persaingan        Persaingan    `json:"persaingan"`
	WaktuProses       WaktuProses   `json:"waktu_proses"`
	Pasar             PasarPenyedia `json:"pasar"`
}

// KontrakBerakhir: kontrak yang segera berakhir.
type KontrakBerakhir struct {
	NoKontrak string    `json:"no_kontrak"`
	NamaPaket string    `json:"nama_paket"`
	Penyedia  string    `json:"penyedia"`
	Nilai     float64   `json:"nilai"`
	Berakhir  time.Time `json:"berakhir"`
	SisaHari  int       `json:"sisa_hari"`
	Jenis     string    `json:"jenis"` // "Tender" atau "Non-tender"
}

// Kontrak: kontrak hasil tender dan non-tender (e-kontrak).
type Kontrak struct {
	TenderJumlah    int64             `json:"tender_jumlah"`
	TenderNilai     float64           `json:"tender_nilai"`
	NonTenderJumlah int64             `json:"non_tender_jumlah"`
	NonTenderNilai  float64           `json:"non_tender_nilai"`
	Status          []Pasangan        `json:"status"`
	Addendum        int64             `json:"addendum"`
	PerBulan        []TitikBulan      `json:"per_bulan"` // menurut tanggal kontrak
	BerakhirDalam   int64             `json:"berakhir_dalam"`
	NilaiBerakhir   float64           `json:"nilai_berakhir"`
	AkanBerakhir    []KontrakBerakhir `json:"akan_berakhir"`
	HariPeringatan  int               `json:"hari_peringatan"`
}

// EkatalogV5 dan EkatalogV6: e-purchasing.
type EkatalogV5 struct {
	Paket        int64        `json:"paket"`
	Nilai        float64      `json:"nilai"`
	PerBulan     []TitikBulan `json:"per_bulan"`
	TopKomoditas []Pasangan   `json:"top_komoditas"`
	TopPenyedia  []Pasangan   `json:"top_penyedia"`
	Status       []Pasangan   `json:"status"`
}

type EkatalogV6 struct {
	Order          int64        `json:"order"`
	Nilai          float64      `json:"nilai"`
	OrderSwasta    int64        `json:"order_swasta"`
	NilaiSwasta    float64      `json:"nilai_swasta"`
	PerBulan       []TitikBulan `json:"per_bulan"`
	TopPenyedia    []Pasangan   `json:"top_penyedia"`
	Status         []Pasangan   `json:"status"`
	TransaksiNilai float64      `json:"transaksi_nilai"`
	TransaksiBaris int64        `json:"transaksi_baris"`
	TopKategori    []Pasangan   `json:"top_kategori"`
}

type Ekatalog struct {
	V5 EkatalogV5 `json:"v5"`
	V6 EkatalogV6 `json:"v6"`
}

// Corong: dari RUP sampai proses pengadaan, menurut kecocokan kode RUP.
type Corong struct {
	TotalPaket int64      `json:"total_paket"`
	TotalPagu  float64    `json:"total_pagu"`
	Tahap      []Pasangan `json:"tahap"` // Tender, Non-tender, E-Purchasing, Belum diproses
}

// Pembanding: angka tahun sebelumnya (hanya yang ada datanya).
type Pembanding struct {
	Tahun        string  `json:"tahun"`
	RUPPaket     int64   `json:"rup_paket"`
	RUPPagu      float64 `json:"rup_pagu"`
	TenderJumlah int64   `json:"tender_jumlah"`
	NilaiKontrak float64 `json:"nilai_kontrak"`
}

// Hasil: seluruh dasbor untuk satu KLPD dan tahun. Bagian bernilai nil bila gagal dibaca (alasannya di Galat) atau tidak diminta.
type Hasil struct {
	Tahun    string `json:"tahun"`
	KodeKLPD string `json:"kode_klpd"`

	RUP        *RUP        `json:"rup"`
	Pemilihan  *Pemilihan  `json:"pemilihan"`
	Kontrak    *Kontrak    `json:"kontrak"`
	Ekatalog   *Ekatalog   `json:"ekatalog"`
	Corong     *Corong     `json:"corong"`
	Pembanding *Pembanding `json:"pembanding"`

	// Dataset (nama tampil) yang belum punya baris untuk KLPD dan tahun ini, dan waktu penarikan terakhir yang sukses.
	DatasetKosong []string   `json:"dataset_kosong"`
	TerakhirTarik *time.Time `json:"terakhir_tarik"`

	Galat map[string]string `json:"galat"`

	// Sekarang: patokan waktu aturan yang bergantung bulan/hari (diisi pemanggil; tes menetapkannya).
	Sekarang time.Time `json:"-"`
}

// Tingkat wawasan, dari yang paling mendesak.
const (
	TingkatPenting   = "penting"
	TingkatPerhatian = "perhatian"
	TingkatInfo      = "info"
	TingkatBaik      = "baik"
)

// Bagian dasbor yang dirujuk wawasan.
const (
	BagianRingkasan = "ringkasan"
	BagianRUP       = "rup"
	BagianPemilihan = "pemilihan"
	BagianKontrak   = "kontrak"
	BagianEkatalog  = "ekatalog"
	BagianCorong    = "corong"
	BagianData      = "data"
)

// Wawasan: satu penjelasan singkat berbasis angka.
type Wawasan struct {
	Bagian  string `json:"bagian"`
	Tingkat string `json:"tingkat"`
	Judul   string `json:"judul"`
	Isi     string `json:"isi"`
}
