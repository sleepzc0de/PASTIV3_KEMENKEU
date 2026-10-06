package handlers

import "github.com/gin-gonic/gin"

// Endpoint E-Katalog V6 (/api/v1/ekatalog/...): respons datar berhalaman cursor, ditangani oleh endpointDatar
// (inaproc_tender_datar.go) seperti endpoint Tender dan E-Katalog archive. Tiga endpoint memakai bentuk saringan bawaan:
//   - penyedia-detail: pencarian rujukan per kode_penyedia.
//   - list-produk-penyedia: pencarian per kode_penyedia; respons tidak memuat kode penyedia, jadi kolom kode_penyedia diisi dari
//     permintaan (KolomKonteks).
//   - paket-e-purchasing: kode_klpd + tahun, tetapi kolomnya bernama kode_klpd dan fiscal_year; kode_klpd "swasta" menampilkan
//     paket swasta.
//
// Dua endpoint punya aturan penyaring sendiri dan dipasang lewat GetDengan/SyncDengan/ListLocalDengan di
// inaproc_ekatalog6_saring.go: list-kategori-produk (drill-down berjenjang) dan e-purchasing-by-produk.
//
// Catatan: pembungkus Get*/Sync*/ListLocal* di bawah sudah TIDAK terpasang di rute (halaman per-dataset dihapus; semuanya lewat penarikan terpadu,
// /inaproc/data, dan /inaproc/ekspor). Pembungkus itu dipertahankan hanya sebagai pintu masuk tes untuk mesin endpointDatar dan aturan
// penyaring yang masih dipakai penarikan.
//
// Daftar field di tiap deklarasi HARUS sinkron dengan migrasinya (040-044); tes menjaganya. Tabel dinamai inaproc_ekatalog6_* supaya
// tidak bertabrakan dengan tabel E-Katalog archive (inaproc_ekatalog_*).

const awalanEkatalog6 = "ekatalog"

// ============================================================
// E-Katalog V6 Endpoint 1: Penyedia Detail
// ============================================================
//
// Migrasi 040. Dicari dengan kode_penyedia (kode teks seperti "01ABCXYZ123"), yang disimpan di kolom kode_penyedia. status_umkk
// berupa penanda 0/1.
var ekatalog6Penyedia = newEndpointDatar(endpointDatar{
	Nama:   "penyedia-detail",
	Awalan: awalanEkatalog6,
	Tabel:  "inaproc_ekatalog6_penyedia",
	Saring: saringan{Param: "kode_penyedia", Kolom: "kode_penyedia"},
	Teks: []string{
		"kode_penyedia", "nama_penyedia", "nib", "npwp_penyedia", "bentuk_usaha", "jenis_perusahaan", "status_aktif", "rekan_id",
		"alamat_penyedia", "email", "telepon", "kbli_id", "kbli_name",
	},
	Bulat: []string{"status_umkk"},
	KolomDaftar: `row_key, kode_penyedia, nama_penyedia, nib, npwp_penyedia, bentuk_usaha, jenis_perusahaan, status_aktif, status_umkk,
		telepon, email, synced_at`,
})

func GetEkatalog6Penyedia(c *gin.Context)       { ekatalog6Penyedia.Get(c) }
func SyncEkatalog6Penyedia(c *gin.Context)      { ekatalog6Penyedia.Sync(c) }
// ============================================================
// E-Katalog V6 Endpoint 4: List Produk Penyedia
// ============================================================
//
// Migrasi 041. Dicari dengan kode_penyedia. Respons hanya memuat produknya (tanpa kode penyedia), jadi kolom kode_penyedia di tabel
// diisi dari permintaan; itu juga yang dipakai menghapus data lama untuk penyedia yang sama. status_produk_tayang berupa
// true/false di API, disimpan sebagai 1/0.
var ekatalog6ProdukPenyedia = newEndpointDatar(endpointDatar{
	Nama:         "list-produk-penyedia",
	Awalan:       awalanEkatalog6,
	Tabel:        "inaproc_ekatalog6_produk_penyedia",
	Saring:       saringan{Param: "kode_penyedia", Kolom: "kode_penyedia"},
	Teks:         []string{"kd_produk", "nama_produk", "status_produk"},
	Bulat:        []string{"status_produk_tayang"},
	KolomKonteks: map[string]string{"kode_penyedia": "kode"},
	KolomDaftar:  `row_key, kode_penyedia, kd_produk, nama_produk, status_produk, status_produk_tayang, synced_at`,
})

func GetEkatalog6ProdukPenyedia(c *gin.Context)       { ekatalog6ProdukPenyedia.Get(c) }
func SyncEkatalog6ProdukPenyedia(c *gin.Context)      { ekatalog6ProdukPenyedia.Sync(c) }
// ============================================================
// E-Katalog V6 Endpoint 3: Paket E-Purchasing
// ============================================================
//
// Migrasi 043. Kolom KLPD dan tahun bernama kode_klpd dan fiscal_year. kode_klpd "swasta" (nilai literal) meminta paket swasta;
// itu diteruskan apa adanya dan bila respons mengosongkan kode_klpd, kolom diisi "swasta" supaya hapus-sebelum-tarik menemukannya.
// is_swasta berupa true/false di API, disimpan sebagai 1/0.
var ekatalog6Paket = newEndpointDatar(endpointDatar{
	Nama:   "paket-e-purchasing",
	Awalan: awalanEkatalog6,
	Tabel:  "inaproc_ekatalog6_paket_epurchasing",
	Saring: saringan{KolomKLPD: "kode_klpd", KolomTahun: "fiscal_year"},
	Teks: []string{
		"kode_klpd", "fiscal_year", "order_id", "product_id", "kode_penyedia", "rekan_id", "kode_satker", "nama_satker",
		"rup_code", "rup_name", "rup_desc", "mak", "funding_source", "status", "shipment_status",
	},
	Desimal: []string{"shipping_fee", "total", "total_qty"},
	Bulat:   []string{"count_product", "is_swasta"},
	Tanggal: []string{"order_date", "last_update_date"},
	KolomDaftar: `row_key, kode_klpd, fiscal_year, order_id, rup_code, rup_name, nama_satker, kode_penyedia, status, shipment_status,
		total_qty, total, is_swasta, order_date, synced_at`,
})

func GetEkatalog6Paket(c *gin.Context)       { ekatalog6Paket.Get(c) }
func SyncEkatalog6Paket(c *gin.Context)      { ekatalog6Paket.Sync(c) }
// ============================================================
// E-Katalog V6 Endpoint 2: List Kategori Produk
// ============================================================
//
// Migrasi 042. Drill-down L1 → L2 → L3: tanpa filter berisi level 1; dengan kd_kategori_1 level 2; dengan kd_kategori_1 dan
// kd_kategori_2 level 3. Respons level 2 dan 3 tidak memuat kode induknya, jadi kd_kategori_1/2 yang kosong diisi dari permintaan
// (IsiBilaKosong) dan kolom tingkat (1-3) dihitung dari permintaan. Satu sinkronisasi menarik satu tingkat (untuk satu induk).
// Aturannya ada di ekatalog6KategoriParam/Rencana/Where (inaproc_ekatalog6_saring.go).
var ekatalog6Kategori = newEndpointDatar(endpointDatar{
	Nama:   "list-kategori-produk",
	Awalan: awalanEkatalog6,
	Tabel:  "inaproc_ekatalog6_kategori",
	Teks: []string{
		"datamart_id", "kd_kategori_1", "nama_kategori_1", "kd_kategori_2", "nama_kategori_2", "kd_kategori_3", "nama_kategori_3",
	},
	KolomKonteks:  map[string]string{"tingkat": "tingkat"},
	IsiBilaKosong: map[string]string{"kd_kategori_1": "kd_kategori_1", "kd_kategori_2": "kd_kategori_2"},
	KolomDaftar: `row_key, tingkat, datamart_id, kd_kategori_1, nama_kategori_1, kd_kategori_2, nama_kategori_2, kd_kategori_3,
		nama_kategori_3, synced_at`,
})

func GetEkatalog6Kategori(c *gin.Context)  { ekatalog6Kategori.GetDengan(c, ekatalog6KategoriParam) }
func SyncEkatalog6Kategori(c *gin.Context) { ekatalog6Kategori.SyncDengan(c, ekatalog6KategoriRencana) }
func ListLocalEkatalog6Kategori(c *gin.Context) {
	ekatalog6Kategori.ListLocalDengan(c, ekatalog6KategoriWhere)
}

// ============================================================
// E-Katalog V6 Endpoint 5: Nilai Transaksi E-Purchasing per Produk
// ============================================================
//
// Migrasi 044. Respons tidak memuat tahun, jadi kolom tahun diisi dari permintaan. Aturan penyaring (tahun wajib; kode_klpd dan/atau
// kd_kategori_1 minimal salah satu; status berdaftar nilai) ada di inaproc_ekatalog6_saring.go.
var ekatalog6Transaksi = newEndpointDatar(endpointDatar{
	Nama:   "e-purchasing-by-produk",
	Awalan: awalanEkatalog6,
	Tabel:  "inaproc_ekatalog6_epurchasing_produk",
	Saring: saringan{KolomKLPD: "kode_klpd"},
	Teks: []string{
		"order_id", "kd_kategori_1", "kategori_1", "kd_kategori_2", "kategori_2", "kd_kategori_3", "kategori_3",
		"product_id", "nama_produk", "kode_klpd", "nama_group_klpd", "nama_klpd", "kode_satker", "nama_satker", "status",
	},
	Desimal:      []string{"nilai_transaksi"},
	KolomKonteks: map[string]string{"tahun": "tahun"},
	KolomDaftar: `row_key, tahun, order_id, product_id, nama_produk, kd_kategori_1, kategori_1, kode_klpd, nama_klpd, nama_satker, status,
		nilai_transaksi, synced_at`,
})

func GetEkatalog6Transaksi(c *gin.Context) { ekatalog6Transaksi.GetDengan(c, ekatalog6TransaksiParam) }
func SyncEkatalog6Transaksi(c *gin.Context) {
	ekatalog6Transaksi.SyncDengan(c, ekatalog6TransaksiRencana)
}
func ListLocalEkatalog6Transaksi(c *gin.Context) {
	ekatalog6Transaksi.ListLocalDengan(c, ekatalog6TransaksiWhere)
}
