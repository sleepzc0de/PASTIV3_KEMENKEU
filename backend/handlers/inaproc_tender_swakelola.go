package handlers

import "github.com/gin-gonic/gin"

// ============================================================
// TENDER Endpoint 9: Pencatatan Swakelola
// ============================================================
//
// Respons datar; logikanya ada di endpointDatar (inaproc_tender_datar.go). Daftar field di bawah HARUS sinkron dengan
// migrasi 027. "pct" pada kd_swakelola_pct/status_swakelola_pct/nilai_pdn_pct/nilai_umk_pct berarti pencatatan (bukan persen);
// nilai_pdn_pct dan nilai_umk_pct adalah nilai rupiah. tipe_swakelola berbentuk angka (kode tipe) tetapi disimpan sebagai teks.
var pencatatanSwakelola = newEndpointDatar(endpointDatar{
	Nama:  "pencatatan-swakelola",
	Tabel: "inaproc_pencatatan_swakelola",
	Teks: []string{
		"kd_klpd", "jenis_klpd", "nama_klpd", "kd_satker", "kd_satker_str", "nama_satker",
		"kd_lpse", "kd_swakelola_pct", "kd_pkt_dce", "kd_rup", "tahun_anggaran", "nama_paket",
		"sumber_dana", "uraian_pekerjaan", "tipe_swakelola", "tipe_swakelola_nama",
		"nama_ppk", "nip_ppk",
		"status_swakelola_pct", "status_swakelola_pct_ket", "alasan_pembatalan", "informasi_lainnya",
	},
	Desimal: []string{"pagu", "nilai_pdn_pct", "nilai_umk_pct", "total_realisasi"},
	Tanggal: []string{"tgl_buat_paket", "tgl_mulai_paket", "tgl_selesai_paket"},
	KolomDaftar: `row_key, kd_klpd, kd_swakelola_pct, tahun_anggaran, kd_rup, nama_paket, nama_satker, tipe_swakelola_nama,
		pagu, total_realisasi, status_swakelola_pct, status_swakelola_pct_ket, tgl_selesai_paket, synced_at`,
})

func GetPencatatanSwakelola(c *gin.Context)       { pencatatanSwakelola.Get(c) }
func SyncPencatatanSwakelola(c *gin.Context)      { pencatatanSwakelola.Sync(c) }
func ListLocalPencatatanSwakelola(c *gin.Context) { pencatatanSwakelola.ListLocal(c) }

// ============================================================
// TENDER Endpoint 10: Pencatatan Swakelola Realisasi
// ============================================================
//
// Satu pencatatan bisa punya beberapa baris realisasi. Respons hanya memuat kode (tanpa nama paket/satker/pagu); untuk itu
// hubungkan ke pencatatan-swakelola lewat kd_swakelola_pct. nip_ppk dan rsk_id dikirim sebagai angka; keduanya dibaca sebagai
// teks asli (Sync memakai UseNumber) supaya NIP 18 digit tidak berubah. Daftar field HARUS sinkron dengan migrasi 028.
var pencatatanSwakelolaRealisasi = newEndpointDatar(endpointDatar{
	Nama:  "pencatatan-swakelola-realisasi",
	Tabel: "inaproc_pencatatan_swakelola_realisasi",
	Teks: []string{
		"kd_klpd", "kd_satker", "kd_lpse", "kd_swakelola_pct", "rsk_id", "tahun_anggaran",
		"nama_ppk", "nip_ppk", "nama_pelaksana", "npwp_pelaksana",
		"no_realisasi", "jenis_realisasi", "ket_realisasi", "dok_realisasi",
	},
	Desimal: []string{"nilai_realisasi"},
	Tanggal: []string{"tgl_realisasi"},
	KolomDaftar: `row_key, kd_klpd, kd_swakelola_pct, rsk_id, tahun_anggaran, nama_pelaksana, no_realisasi, jenis_realisasi,
		nilai_realisasi, tgl_realisasi, synced_at`,
})

func GetPencatatanSwakelolaRealisasi(c *gin.Context)       { pencatatanSwakelolaRealisasi.Get(c) }
func SyncPencatatanSwakelolaRealisasi(c *gin.Context)      { pencatatanSwakelolaRealisasi.Sync(c) }
func ListLocalPencatatanSwakelolaRealisasi(c *gin.Context) { pencatatanSwakelolaRealisasi.ListLocal(c) }
