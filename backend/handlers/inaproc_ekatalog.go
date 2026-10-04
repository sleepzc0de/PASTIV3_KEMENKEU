package handlers

import "github.com/gin-gonic/gin"

// Endpoint E-Katalog V5 archive (/api/v1/ekatalog-archive/...): respons datar berhalaman cursor, ditangani oleh endpointDatar
// (inaproc_tender_datar.go) seperti endpoint Tender. Bedanya ada pada penyaringnya:
//   - instansi-satker: kode_klpd saja (tanpa tahun).
//   - paket-e-purchasing: kode_klpd + tahun, seperti Tender.
//   - komoditas-detail, penyedia-detail, penyedia-distributor-detail: pencarian rujukan per satu kode. Tidak ada daftar "semua"
//     di API-nya, jadi sinkronisasi menyimpan hasil pencarian untuk kode yang dimasukkan admin (data lama untuk kode itu diganti).
//
// Daftar field di tiap deklarasi HARUS sinkron dengan migrasinya (035-039); tes menjaganya.

const awalanEkatalog = "ekatalog-archive"

// ============================================================
// E-Katalog Endpoint 1: Instansi Satker
// ============================================================
//
// Migrasi 035. kd_satker berupa angka di API; disimpan sebagai teks seperti kode lainnya.
var ekatalogInstansiSatker = newEndpointDatar(endpointDatar{
	Nama:        "instansi-satker",
	Awalan:      awalanEkatalog,
	Tabel:       "inaproc_ekatalog_instansi_satker",
	Saring:      saringan{TanpaTahun: true},
	Teks:        []string{"kd_klpd", "jenis_klpd", "nama_klpd", "kd_satker", "kd_satker_str", "nama_satker"},
	KolomDaftar: `row_key, kd_klpd, nama_klpd, jenis_klpd, kd_satker, kd_satker_str, nama_satker, synced_at`,
})

func GetEkatalogInstansiSatker(c *gin.Context)       { ekatalogInstansiSatker.Get(c) }
func SyncEkatalogInstansiSatker(c *gin.Context)      { ekatalogInstansiSatker.Sync(c) }
func ListLocalEkatalogInstansiSatker(c *gin.Context) { ekatalogInstansiSatker.ListLocal(c) }

// ============================================================
// E-Katalog Endpoint 2: Komoditas Detail
// ============================================================
//
// Migrasi 036. Nama field pertama di API memakai huruf besar ("Jenis_Katalog") dan kolomnya pun begitu, sama persis. Dicari
// dengan kode_komoditas, yang disimpan di kolom kd_komoditas.
var ekatalogKomoditas = newEndpointDatar(endpointDatar{
	Nama:        "komoditas-detail",
	Awalan:      awalanEkatalog,
	Tabel:       "inaproc_ekatalog_komoditas",
	Saring:      saringan{Param: "kode_komoditas", Kolom: "kd_komoditas"},
	Teks:        []string{"kd_komoditas", "nama_komoditas", "Jenis_Katalog", "kd_instansi_katalog", "nama_instansi_katalog"},
	KolomDaftar: `row_key, kd_komoditas, nama_komoditas, Jenis_Katalog, kd_instansi_katalog, nama_instansi_katalog, synced_at`,
})

func GetEkatalogKomoditas(c *gin.Context)       { ekatalogKomoditas.Get(c) }
func SyncEkatalogKomoditas(c *gin.Context)      { ekatalogKomoditas.Sync(c) }
func ListLocalEkatalogKomoditas(c *gin.Context) { ekatalogKomoditas.ListLocal(c) }

// ============================================================
// E-Katalog Endpoint 3: Paket E-Purchasing
// ============================================================
//
// Migrasi 037. Satu paket (kd_paket) bisa punya beberapa baris produk (kd_paket_produk). kuantitas bisa berpecahan (DECIMAL);
// jml_jenis_produk bilangan bulat. Kontak Pokja (email, telepon) ikut disimpan apa adanya dari API.
var ekatalogPaket = newEndpointDatar(endpointDatar{
	Nama:   "paket-e-purchasing",
	Awalan: awalanEkatalog,
	Tabel:  "inaproc_ekatalog_paket_epurchasing",
	Teks: []string{
		"kd_klpd", "kd_rup", "tahun_anggaran", "kd_paket", "no_paket", "nama_paket", "deskripsi", "catatan_produk",
		"status_paket", "paket_status_str", "kode_anggaran", "nama_sumber_dana",
		"kd_komoditas", "kd_produk", "kd_paket_produk", "kd_penyedia", "kd_penyedia_distributor",
		"satker_id", "nama_satker", "alamat_satker", "npwp_satker",
		"kd_user_ppk", "ppk_nip", "jabatan_ppk", "kd_user_pokja", "email_user_pokja", "no_telp_user_pokja",
		"kd_provinsi_wilayah_harga", "kd_kabupaten_wilayah_harga",
	},
	Desimal: []string{"harga_satuan", "kuantitas", "ongkos_kirim", "total_harga"},
	Bulat:   []string{"jml_jenis_produk"},
	Tanggal: []string{"tanggal_buat_paket", "tanggal_edit_paket"},
	KolomDaftar: `row_key, kd_klpd, tahun_anggaran, kd_paket, no_paket, nama_paket, nama_satker, kd_komoditas, kd_penyedia,
		kuantitas, total_harga, status_paket, paket_status_str, tanggal_buat_paket, synced_at`,
})

func GetEkatalogPaket(c *gin.Context)       { ekatalogPaket.Get(c) }
func SyncEkatalogPaket(c *gin.Context)      { ekatalogPaket.Sync(c) }
func ListLocalEkatalogPaket(c *gin.Context) { ekatalogPaket.ListLocal(c) }

// ============================================================
// E-Katalog Endpoint 4: Penyedia Detail
// ============================================================
//
// Migrasi 038. Dicari dengan kode_penyedia, yang diasumsikan sama dengan kd_penyedia (bukan kode_penyedia_sikap): dokumentasi
// tidak menyebutnya.
var ekatalogPenyedia = newEndpointDatar(endpointDatar{
	Nama:   "penyedia-detail",
	Awalan: awalanEkatalog,
	Tabel:  "inaproc_ekatalog_penyedia",
	Saring: saringan{Param: "kode_penyedia", Kolom: "kd_penyedia"},
	Teks: []string{
		"kd_penyedia", "kode_penyedia_sikap", "nama_penyedia", "npwp_penyedia", "npwp_16", "penyedia_ukm", "kbli2020_penyedia",
		"alamat_penyedia", "email_penyedia", "no_telp_penyedia",
	},
	KolomDaftar: `row_key, kd_penyedia, kode_penyedia_sikap, nama_penyedia, npwp_penyedia, npwp_16, penyedia_ukm,
		no_telp_penyedia, email_penyedia, synced_at`,
})

func GetEkatalogPenyedia(c *gin.Context)       { ekatalogPenyedia.Get(c) }
func SyncEkatalogPenyedia(c *gin.Context)      { ekatalogPenyedia.Sync(c) }
func ListLocalEkatalogPenyedia(c *gin.Context) { ekatalogPenyedia.ListLocal(c) }

// ============================================================
// E-Katalog Endpoint 5: Penyedia Distributor Detail
// ============================================================
//
// Migrasi 039. Dicari dengan kd_distributor, yang disimpan di kolom kd_penyedia_distributor.
var ekatalogDistributor = newEndpointDatar(endpointDatar{
	Nama:   "penyedia-distributor-detail",
	Awalan: awalanEkatalog,
	Tabel:  "inaproc_ekatalog_distributor",
	Saring: saringan{Param: "kd_distributor", Kolom: "kd_penyedia_distributor"},
	Teks: []string{
		"kd_penyedia_distributor", "nama_distributor", "npwp_distributor", "alamat_distributor", "email_distributor", "no_telp_distributor",
	},
	KolomDaftar: `row_key, kd_penyedia_distributor, nama_distributor, npwp_distributor, no_telp_distributor, email_distributor, synced_at`,
})

func GetEkatalogDistributor(c *gin.Context)       { ekatalogDistributor.Get(c) }
func SyncEkatalogDistributor(c *gin.Context)      { ekatalogDistributor.Sync(c) }
func ListLocalEkatalogDistributor(c *gin.Context) { ekatalogDistributor.ListLocal(c) }
