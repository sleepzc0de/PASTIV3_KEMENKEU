package handlers

import "github.com/gin-gonic/gin"

// Endpoint Tender (bukan non tender) 11-16: pengumuman, peserta, e-kontrak, kontrak, selesai, dan nilai selesai. Respons datar;
// logikanya ada di endpointDatar (inaproc_tender_datar.go). Daftar field di tiap deklarasi HARUS sinkron dengan migrasinya
// (029-034); tes (TestEndpointDatarKolomSamaDenganMigrasi) menjaganya.

// ============================================================
// TENDER Endpoint 11: Pengumuman
// ============================================================
//
// Dua skenario di Inaproc: (tahun + kode_klpd) atau kd_tender saja, jadi MenerimaKdTender menyala. Migrasi 029.
var tenderPengumuman = newEndpointDatar(endpointDatar{
	Nama:  "pengumuman",
	Tabel: "inaproc_tender_pengumuman",
	Teks: []string{
		"kd_klpd", "jenis_klpd", "nama_klpd", "kd_satker", "kd_satker_str", "nama_satker",
		"kd_lpse", "nama_lpse", "url_lpse", "kd_pkt_dce", "kd_rup", "kd_tender", "tahun_anggaran", "list_tahun_anggaran",
		"nama_paket", "jenis_pengadaan", "kualifikasi_paket", "kontrak_pembayaran", "lokasi_pekerjaan", "sumber_dana",
		"mtd_pemilihan", "mtd_kualifikasi", "mtd_evaluasi",
		"nama_ppk", "nip_ppk", "nama_pokja", "nip_pokja",
		"status_tender", "ket_ditutup", "ket_diulang",
	},
	Desimal: []string{"pagu", "hps"},
	Bulat:   []string{"versi_tender"},
	Tanggal: []string{"tanggal_status", "tgl_buat_paket", "tgl_kolektif_kolegial", "tgl_pengumuman_tender"},
	KolomDaftar: `row_key, kd_klpd, kd_tender, versi_tender, tahun_anggaran, kd_rup, nama_paket, nama_satker, mtd_pemilihan,
		pagu, hps, status_tender, tgl_pengumuman_tender, synced_at`,
	MenerimaKdTender: true,
})

func GetTenderPengumuman(c *gin.Context)       { tenderPengumuman.Get(c) }
func SyncTenderPengumuman(c *gin.Context)      { tenderPengumuman.Sync(c) }
func ListLocalTenderPengumuman(c *gin.Context) { tenderPengumuman.ListLocal(c) }

// ============================================================
// TENDER Endpoint 12: Peserta Tender
// ============================================================
//
// Satu tender punya banyak peserta (kd_peserta). pemenang dan pemenang_terverifikasi berupa penanda 0/1. Migrasi 030.
var tenderPeserta = newEndpointDatar(endpointDatar{
	Nama:  "peserta-tender",
	Tabel: "inaproc_peserta_tender",
	Teks: []string{
		"kd_klpd", "kd_satker", "kd_satker_str", "kd_lpse", "kd_pkt_dce", "kd_tender", "kd_peserta", "tahun_anggaran",
		"kd_penyedia", "nama_penyedia", "npwp_penyedia", "npwp_penyedia_16", "alasan",
	},
	Desimal: []string{"nilai_penawaran", "nilai_terkoreksi"},
	Bulat:   []string{"pemenang", "pemenang_terverifikasi"},
	KolomDaftar: `row_key, kd_klpd, kd_tender, kd_peserta, tahun_anggaran, nama_penyedia, npwp_penyedia,
		nilai_penawaran, nilai_terkoreksi, pemenang, pemenang_terverifikasi, synced_at`,
})

func GetTenderPeserta(c *gin.Context)       { tenderPeserta.Get(c) }
func SyncTenderPeserta(c *gin.Context)      { tenderPeserta.Sync(c) }
func ListLocalTenderPeserta(c *gin.Context) { tenderPeserta.ListLocal(c) }

// ============================================================
// TENDER Endpoint 13 dan 14: E-Kontrak dan Kontrak
// ============================================================
//
// Kedua endpoint memuat field kontrak yang sama; tender-ekontrak (13) menambah tiga riwayat yang selalu berupa larik
// (bapbast_history_json, spmkspp_history_json, penilaian_kinerja_penyedia), disimpan sebagai teks JSON. Data rekening bank
// penyedia ikut disimpan, sama seperti kontrak non tender (migrasi 017). Migrasi 031 (ep 13) dan 032 (ep 14).
var kontrakTenderTeks = []string{
	"kd_klpd", "jenis_klpd", "nama_klpd", "kd_lpse", "kd_satker", "kd_satker_str", "nama_satker", "alamat_satker",
	"kd_tender", "tahun_anggaran", "nama_paket", "lingkup_pekerjaan", "informasi_lainnya",
	"no_kontrak", "no_sppbj", "jenis_kontrak", "status_kontrak", "kota_kontrak",
	"alasan_penetapan_status_kontrak", "apakah_addendum", "alasan_addendum",
	"alasan_ubah_nilai_kontrak", "alasan_nilai_kontrak_10_persen",
	"nama_ppk", "nip_ppk", "jabatan_ppk", "no_sk_ppk",
	"kd_penyedia", "nama_penyedia", "bentuk_usaha_penyedia", "tipe_penyedia", "npwp_penyedia", "npwp_16_penyedia",
	"wakil_sah_penyedia", "jabatan_wakil_penyedia", "anggota_kso",
	"nama_rek_bank", "no_rek_bank", "nama_pemilik_rek_bank",
}

var kontrakTenderDesimal = []string{"nilai_kontrak", "nilai_pdn_kontrak", "nilai_umk_kontrak"}
var kontrakTenderTanggal = []string{"tgl_kontrak", "tgl_kontrak_awal", "tgl_kontrak_akhir", "tgl_penetapan_status_kontrak"}

const kolomDaftarKontrakTender = `row_key, kd_klpd, kd_tender, tahun_anggaran, no_kontrak, nama_paket, nama_satker, nama_penyedia,
		nilai_kontrak, status_kontrak, tgl_kontrak, synced_at`

var tenderEkontrak = newEndpointDatar(endpointDatar{
	Nama:  "tender-ekontrak",
	Tabel: "inaproc_tender_ekontrak",
	Teks: append(append([]string{}, kontrakTenderTeks...),
		"bapbast_history_json", "spmkspp_history_json", "penilaian_kinerja_penyedia"),
	Desimal:     kontrakTenderDesimal,
	Bulat:       []string{"versi_addendum"},
	Tanggal:     kontrakTenderTanggal,
	KolomDaftar: kolomDaftarKontrakTender,
})

func GetTenderEkontrak(c *gin.Context)       { tenderEkontrak.Get(c) }
func SyncTenderEkontrak(c *gin.Context)      { tenderEkontrak.Sync(c) }
func ListLocalTenderEkontrak(c *gin.Context) { tenderEkontrak.ListLocal(c) }

var tenderEkontrakKontrak = newEndpointDatar(endpointDatar{
	Nama:        "tender-ekontrak-kontrak",
	Tabel:       "inaproc_tender_ekontrak_kontrak",
	Teks:        kontrakTenderTeks,
	Desimal:     kontrakTenderDesimal,
	Bulat:       []string{"versi_addendum"},
	Tanggal:     kontrakTenderTanggal,
	KolomDaftar: kolomDaftarKontrakTender,
})

func GetTenderEkontrakKontrak(c *gin.Context)       { tenderEkontrakKontrak.Get(c) }
func SyncTenderEkontrakKontrak(c *gin.Context)      { tenderEkontrakKontrak.Sync(c) }
func ListLocalTenderEkontrakKontrak(c *gin.Context) { tenderEkontrakKontrak.ListLocal(c) }

// ============================================================
// TENDER Endpoint 15: Tender Selesai
// ============================================================
//
// Cursor berbasis datamart_id di sisi Inaproc (last_update_ref); bagi kita tetap meta.cursor biasa. Contoh dokumentasi memuat
// karakter tak terlihat (lebar nol) di akhir nama KLPD/satker/LPSE; nilainya disimpan apa adanya, jadi kolom nama dibuat lega.
// Migrasi 033.
var tenderSelesai = newEndpointDatar(endpointDatar{
	Nama:  "tender-selesai",
	Tabel: "inaproc_tender_selesai",
	Teks: []string{
		"kd_klpd", "jenis_klpd", "nama_klpd", "kd_satker", "kd_satker_str", "nama_satker",
		"kd_lpse", "nama_lpse", "url_lpse", "kd_rup", "kd_tender", "tahun_anggaran", "nama_paket",
		"jenis_pengadaan", "kualifikasi_paket", "kontrak_pembayaran", "sumber_dana", "mak",
		"mtd_pemilihan", "mtd_kualifikasi", "status_tender", "last_update_ref",
	},
	Desimal: []string{"pagu", "hps"},
	Tanggal: []string{"tgl_pengumuman_tender", "tgl_penetapan_pemenang"},
	KolomDaftar: `row_key, kd_klpd, kd_tender, tahun_anggaran, kd_rup, nama_paket, nama_satker, mtd_pemilihan,
		pagu, hps, status_tender, tgl_penetapan_pemenang, synced_at`,
})

func GetTenderSelesai(c *gin.Context)       { tenderSelesai.Get(c) }
func SyncTenderSelesai(c *gin.Context)      { tenderSelesai.Sync(c) }
func ListLocalTenderSelesai(c *gin.Context) { tenderSelesai.ListLocal(c) }

// ============================================================
// TENDER Endpoint 16: Tender Selesai Nilai
// ============================================================
//
// Nilai penawaran/negosiasi/kontrak per penyedia pemenang. Respons tidak memuat nama paket; hubungkan ke tender-selesai
// lewat kd_tender. kd_satker di sini berbentuk kode bertitik. Migrasi 034.
var tenderSelesaiNilai = newEndpointDatar(endpointDatar{
	Nama:  "tender-selesai-nilai",
	Tabel: "inaproc_tender_selesai_nilai",
	Teks: []string{
		"kd_klpd", "jenis_klpd", "nama_klpd", "kd_satker", "nama_satker", "kd_lpse",
		"kd_tender", "kd_paket", "kd_rup_paket", "psr_id", "tahun_anggaran",
		"kd_penyedia", "nama_penyedia", "npwp_penyedia", "npwp_16_penyedia",
	},
	Desimal: []string{
		"pagu", "hps", "nilai_penawaran", "nilai_terkoreksi", "nilai_negosiasi",
		"nilai_kontrak", "nilai_pdn_kontrak", "nilai_umk_kontrak",
	},
	Tanggal: []string{"tgl_pengumuman_tender", "tgl_penetapan_pemenang"},
	KolomDaftar: `row_key, kd_klpd, kd_tender, psr_id, tahun_anggaran, nama_penyedia, nama_satker, pagu, hps,
		nilai_penawaran, nilai_negosiasi, nilai_kontrak, tgl_penetapan_pemenang, synced_at`,
})

func GetTenderSelesaiNilai(c *gin.Context)       { tenderSelesaiNilai.Get(c) }
func SyncTenderSelesaiNilai(c *gin.Context)      { tenderSelesaiNilai.Sync(c) }
func ListLocalTenderSelesaiNilai(c *gin.Context) { tenderSelesaiNilai.ListLocal(c) }
