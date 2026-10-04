package handlers

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/internal/fakesql"
)

// Tes untuk endpoint Tender berbentuk datar (endpointDatar): non-tender-selesai, pencatatan-non-tender,
// pencatatan-non-tender-realisasi, pencatatan-swakelola, dan pencatatan-swakelola-realisasi. Contoh baris di bawah disalin
// dari dokumentasi API Inaproc.

const contohNonTenderSelesai = `{
  "hps": 1234567890,
  "jenis_klpd": null,
  "jenis_pengadaan": "Pengadaan Barang",
  "kd_klpd": null,
  "kd_lpse": 12345,
  "kd_nontender": 123456,
  "kd_penyedia": 123456,
  "kd_pkt_dce": 123456,
  "kd_rup": "12345",
  "kd_satker": "12345",
  "kd_satker_str": null,
  "kontrak_pembayaran": "Lumsum",
  "kualifikasi_paket": "Kecil",
  "lpse_id": 1234567,
  "mak": "AA.12345.6789.10.11.12",
  "mtd_pemilihan": "Pengadaan Langsung",
  "nama_klpd": null,
  "nama_lpse": "Contoh Nama LPSE",
  "nama_paket": "Contoh Nama Paket",
  "nama_penyedia": "Contoh Nama Penyedia",
  "nama_satker": "Contoh Nama Satker",
  "nilai_kontrak": null,
  "nilai_negosiasi": 12345,
  "nilai_pdn_kontrak": null,
  "nilai_penawaran": 12345,
  "nilai_terkoreksi": null,
  "nilai_umk_kontrak": null,
  "npwp16_penyedia": "123456789012345",
  "npwp_penyedia": "12.345.678.9-098.765",
  "pagu": 12345,
  "status_nontender": "Selesai",
  "sumber_dana": "APBD",
  "tahun_anggaran": 2024,
  "tgl_penarikan": "2024-01-05T09:07:06.5Z",
  "tgl_pengumuman_nontender": "2024-01-10 13:30:00.000000Z",
  "tgl_selesai_nontender": "2024-01-15 15:45:00.000000Z",
  "url_lpse": "https://12345.go.id/lpse"
}`

const contohPencatatanNonTender = `{
  "alasan_pembatalan": null,
  "bukti_pembayaran": "Contoh Bukti Pembayaran",
  "informasi_lainnya": null,
  "jenis_klpd": "KOTA",
  "kategori_pengadaan": "Contoh Kategori Pengadaan",
  "kd_klpd": "DX",
  "kd_lpse": 12345,
  "kd_nontender_pct": 2024,
  "kd_pkt_dce": 12345,
  "kd_rup": "12345",
  "kd_satker": "12345",
  "kd_satker_str": "1.23.45.67",
  "mtd_pemilihan": "Pengadaan Langsung",
  "nama_klpd": "Kota Jakarta",
  "nama_paket": "Belanja Jasa Konsultansi Contoh",
  "nama_ppk": "Contoh Nama PPK",
  "nama_satker": "SEKRETARIAT DAERAH",
  "nilai_pdn_pct": 0,
  "nilai_umk_pct": 0,
  "nip_ppk": "12345",
  "pagu": 12345,
  "status_nontender_pct": "Aktif",
  "status_nontender_pct_ket": "Paket Selesai",
  "sumber_dana": "APBD",
  "tahun_anggaran": 2024,
  "tgl_buat_paket": "2024-01-10T11:30:33.100000Z",
  "tgl_mulai_paket": "2024-01-10T11:31:02.656Z",
  "tgl_selesai_paket": "2024-01-15T00:00:00Z",
  "total_realisasi": 12345,
  "uraian_pekerjaan": "Contoh Uraian Pekerjaan"
}`

const contohPencatatanNonTenderRealisasi = `{
  "dok_realisasi": null,
  "jenis_klpd": "KEMENTRIAN",
  "jenis_realisasi": "Contoh Jenis Realisasi",
  "kd_klpd": "DX",
  "kd_lpse": 12345,
  "kd_nontender_pct": 12345,
  "kd_paket_dce": 12345,
  "kd_rup_paket": "12345",
  "kd_satker": "12345",
  "kd_satker_str": "0.12.3.45.6.78.90.9876",
  "ket_realisasi": "Contoh Keterangan Realisasi",
  "nama_klpd": "Provinsi Jawa Barat",
  "nama_lpse": "LPSE Provinsi Jawa Barat",
  "nama_paket": "Contoh Nama Paket",
  "nama_penyedia": "Contoh Nama Penyedia",
  "nama_ppk": "Contoh Nama PPK",
  "nama_satker": "Badan Penanggulangan Bencana Daerah",
  "nilai_realisasi": 12345,
  "nip_ppk": "12345",
  "no_realisasi": "ABC/ABC/I/2024",
  "npwp_penyedia": "01.234.567.8-909.876",
  "pagu": 12345,
  "tahun_anggaran": 2024,
  "tgl_realisasi": "2024-01-01T00:00:00.000000Z"
}`

const contohPencatatanSwakelola = `{
  "alasan_pembatalan": null,
  "informasi_lainnya": null,
  "jenis_klpd": "KOTA",
  "kd_klpd": "DX",
  "kd_lpse": 12345,
  "kd_pkt_dce": 12345,
  "kd_rup": "12345",
  "kd_satker": "12345",
  "kd_satker_str": "1.23.45.67",
  "kd_swakelola_pct": 1234567890,
  "nama_klpd": "Kota Jakarta",
  "nama_paket": "Belanja Jasa Konsultansi Contoh",
  "nama_ppk": "Contoh Nama PPK",
  "nama_satker": "SEKRETARIAT DAERAH",
  "nilai_pdn_pct": 0,
  "nilai_umk_pct": 0,
  "nip_ppk": "12345",
  "pagu": 12345,
  "status_swakelola_pct": "Aktif",
  "status_swakelola_pct_ket": "Paket Selesai",
  "sumber_dana": "APBD",
  "tahun_anggaran": 2024,
  "tgl_buat_paket": "2024-01-10T11:30:33.100000Z",
  "tgl_mulai_paket": "2024-01-10T11:31:02.656Z",
  "tgl_selesai_paket": "2024-01-15T00:00:00Z",
  "tipe_swakelola": 1,
  "tipe_swakelola_nama": "Tipe X",
  "total_realisasi": 12345,
  "uraian_pekerjaan": "Contoh Uraian Pekerjaan"
}`

const contohPencatatanSwakelolaRealisasi = `{
  "dok_realisasi": "SPKS",
  "jenis_realisasi": "Dokumen Lainnya",
  "kd_klpd": "DX",
  "kd_lpse": 12345,
  "kd_satker": "12345",
  "kd_swakelola_pct": 12345,
  "ket_realisasi": "Contoh Keterangan Realisasi",
  "nama_pelaksana": "Contoh Nama Pelaksana",
  "nama_ppk": "Contoh Nama PPK",
  "nilai_realisasi": 12345,
  "nip_ppk": 12345,
  "no_realisasi": "123/AA/123",
  "npwp_pelaksana": "01.234.567.8-909.876",
  "rsk_id": 12345,
  "tahun_anggaran": 2024,
  "tgl_realisasi": "2024-01-20T00:00:00Z"
}`

const contohTenderPengumuman = `{
  "hps": 1234567890,
  "jenis_klpd": "KEMENTERIAN",
  "jenis_pengadaan": "Pengadaan Barang",
  "kd_klpd": "KX",
  "kd_lpse": 12345,
  "kd_pkt_dce": 123456,
  "kd_rup": "12345",
  "kd_satker": "12345",
  "kd_satker_str": "12345",
  "kd_tender": 12345,
  "ket_ditutup": null,
  "ket_diulang": null,
  "kontrak_pembayaran": "Lumsum",
  "kualifikasi_paket": "Kecil",
  "list_tahun_anggaran": "2024",
  "lokasi_pekerjaan": "Rumah Contoh - Jawa Barat",
  "mtd_evaluasi": "Pagu Anggaran",
  "mtd_kualifikasi": "Pra Dua File Kualitas Biaya",
  "mtd_pemilihan": "Pengadaan Langsung",
  "nama_klpd": "Contoh Nama KLPD",
  "nama_lpse": "Contoh Nama LPSE",
  "nama_paket": "Contoh Nama Paket",
  "nama_pokja": "Contoh Nama Pokja",
  "nama_ppk": "Contoh Nama PPK",
  "nama_satker": "Contoh Nama Satker",
  "nip_pokja": "123456789",
  "nip_ppk": "123456789",
  "pagu": 12345,
  "status_tender": "Selesai",
  "sumber_dana": "APBD",
  "tahun_anggaran": 2024,
  "tanggal_status": "2024-01-05T00:00:00Z",
  "tgl_buat_paket": "2024-01-05T09:07:06.5Z",
  "tgl_kolektif_kolegial": "2024-01-10T13:21:52.557Z",
  "tgl_pengumuman_tender": "2024-01-10 13:30:00.000000Z",
  "url_lpse": "https://12345.go.id/lpse",
  "versi_tender": 1
}`

const contohPesertaTender = `{
  "alasan": null,
  "kd_klpd": "KX",
  "kd_lpse": 12345,
  "kd_penyedia": 12345,
  "kd_peserta": 12345,
  "kd_pkt_dce": 12345,
  "kd_satker": "12345",
  "kd_satker_str": "12345",
  "kd_tender": 12345,
  "nama_penyedia": "Contoh Nama Penyedia",
  "nilai_penawaran": null,
  "nilai_terkoreksi": null,
  "npwp_penyedia": "",
  "npwp_penyedia_16": "12345",
  "pemenang": 0,
  "pemenang_terverifikasi": 0,
  "tahun_anggaran": 2024
}`

// Baris pertama contoh dokumentasi tender-ekontrak. penilaian_kinerja_penyedia tidak ada di contohnya, tetapi dokumentasi
// menyebutnya selalu dikembalikan sebagai larik, jadi disertakan di sini (bentuk isinya sama dengan non-tender-ekontrak).
const contohTenderEkontrak = `{
  "alamat_satker": "Jalan Contoh Satker",
  "alasan_addendum": null,
  "alasan_nilai_kontrak_10_persen": null,
  "alasan_penetapan_status_kontrak": null,
  "alasan_ubah_nilai_kontrak": "",
  "anggota_kso": null,
  "apakah_addendum": "Tidak",
  "bentuk_usaha_penyedia": "Perseroan Terbatas (PT)",
  "bapbast_history_json": [
    {
      "besar_pembayaran": 898975000,
      "jabatan_penandatangan_sk": "Menteri",
      "jabatan_wakil_penyedia": "Direktur",
      "no_bap": "05.3/BAP/DATA/XII/2021",
      "no_bast": "05.3/BAST/DATA/XII/2021",
      "progres_pekerjaan": null,
      "tgl_bap": "2021-12-06T00:00:00.000000Z",
      "tgl_bast": "2021-12-06T00:00:00.000000Z",
      "wakil_sah_penyedia": "N****** H*****"
    }
  ],
  "informasi_lainnya": "",
  "jabatan_ppk": "Pejabat Pembuat Komitmen (PPK)",
  "jabatan_wakil_penyedia": "Direktur",
  "jenis_klpd": "PROVINSI",
  "jenis_kontrak": "Harga Satuan",
  "kd_klpd": "DX",
  "kd_lpse": 12345,
  "kd_penyedia": 12345,
  "kd_satker": "12345",
  "kd_satker_str": "0.12.3.45.6.78.90.9876",
  "kd_tender": 12345,
  "kota_kontrak": "Jakarta",
  "lingkup_pekerjaan": "Contoh Lingkup Pekerjaan",
  "nama_klpd": "Provinsi DKI Jakarta",
  "nama_paket": "Contoh Nama Paket",
  "nama_pemilik_rek_bank": "Contoh Nama Pemilik Rekening Bank",
  "nama_penyedia": "Contoh Nama Penyedia",
  "nama_ppk": "Contoh Nama PPK",
  "nama_rek_bank": "Contoh Nama Rekening Bank",
  "nama_satker": "Contoh Nama Satker",
  "nilai_kontrak": 12345.67,
  "nilai_pdn_kontrak": 12345.67,
  "nilai_umk_kontrak": 12345.67,
  "nip_ppk": "1234567890",
  "no_kontrak": "123/AA/123",
  "no_rek_bank": "123456",
  "no_sk_ppk": "123/AA/123",
  "no_sppbj": "123/AA/123",
  "npwp_16_penyedia": "1234567890",
  "npwp_penyedia": "1234567890",
  "penilaian_kinerja_penyedia": [{"indikator_penilaian": "Kualitas", "nilai_indikator": 4}],
  "status_kontrak": "Kontrak Sedang Berjalan",
  "spmkspp_history_json": [
    {
      "alamat_pengiriman": null,
      "jabatan_wakil_penyedia": "Direktur",
      "kota_spmk_spp": "Jakarta",
      "no_spmk_spp": "04/SPMK/PPK1/DATA/V/2021",
      "tgl_mulai_pekerjaan": "May 7, 2021 12:00:00 AM",
      "tgl_selesai_pekerjaan": "Dec 31, 2021 12:00:00 AM",
      "tgl_spmk_spp": "2021-05-07T00:00:00.000000Z",
      "wakil_sah_penyedia": "N****** H*****",
      "waktu_penyelesaian": "240 hari kalender"
    }
  ],
  "tahun_anggaran": 2024,
  "tgl_kontrak": "2024-09-15T00:00:00.000000Z",
  "tgl_kontrak_akhir": "2024-09-15T00:00:00.000000Z",
  "tgl_kontrak_awal": "2024-09-15T00:00:00.000000Z",
  "tgl_penetapan_status_kontrak": null,
  "tipe_penyedia": "Penyedia Badan Usaha Non KSO",
  "versi_addendum": 0,
  "wakil_sah_penyedia": "Contoh Nama Wakil Sah Penyedia"
}`

// Baris kedua contoh tender-ekontrak: hanya sebagian field terisi dan kedua riwayat berupa larik kosong.
const contohTenderEkontrakMinimal = `{
  "alamat_satker": "Jalan Contoh Satker 2",
  "kd_klpd": "DX",
  "kd_tender": 67890,
  "tahun_anggaran": 2024,
  "nama_paket": "Contoh Paket Tanpa History",
  "bapbast_history_json": [],
  "spmkspp_history_json": []
}`

const contohTenderEkontrakKontrak = `{
  "alamat_satker": "Jalan Contoh Satker",
  "alasan_addendum": null,
  "alasan_nilai_kontrak_10_persen": null,
  "alasan_penetapan_status_kontrak": null,
  "alasan_ubah_nilai_kontrak": "",
  "anggota_kso": null,
  "apakah_addendum": "Tidak",
  "bentuk_usaha_penyedia": "Perseroan Terbatas (PT)",
  "informasi_lainnya": "",
  "jabatan_ppk": "Pejabat Pembuat Komitmen (PPK)",
  "jabatan_wakil_penyedia": "Direktur",
  "jenis_klpd": "PROVINSI",
  "jenis_kontrak": "Harga Satuan",
  "kd_klpd": "DX",
  "kd_lpse": 12345,
  "kd_penyedia": 12345,
  "kd_satker": "12345",
  "kd_satker_str": "0.12.3.45.6.78.90.9876",
  "kd_tender": 12345,
  "kota_kontrak": "Jakarta",
  "lingkup_pekerjaan": "Contoh Lingkup Pekerjaan",
  "nama_klpd": "Provinsi DKI Jakarta",
  "nama_paket": "Contoh Nama Paket",
  "nama_pemilik_rek_bank": "Contoh Nama Pemilik Rekening Bank",
  "nama_penyedia": "Contoh Nama Penyedia",
  "nama_ppk": "Contoh Nama PPK",
  "nama_rek_bank": "Contoh Nama Rekening Bank",
  "nama_satker": "Contoh Nama Satker",
  "nilai_kontrak": 12345.67,
  "nilai_pdn_kontrak": 12345.67,
  "nilai_umk_kontrak": 12345.67,
  "nip_ppk": "1234567890",
  "no_kontrak": "123/AA/123",
  "no_rek_bank": "123456",
  "no_sk_ppk": "123/AA/123",
  "no_sppbj": "123/AA/123",
  "npwp_16_penyedia": "1234567890",
  "npwp_penyedia": "1234567890",
  "status_kontrak": "Kontrak Sedang Berjalan",
  "tahun_anggaran": 2024,
  "tgl_kontrak": "2024-09-15T00:00:00.000000Z",
  "tgl_kontrak_akhir": "2024-09-15T00:00:00.000000Z",
  "tgl_kontrak_awal": "2024-09-15T00:00:00.000000Z",
  "tgl_penetapan_status_kontrak": null,
  "tipe_penyedia": "Penyedia Badan Usaha Non KSO",
  "versi_addendum": 0,
  "wakil_sah_penyedia": "Contoh Nama Wakil Sah Penyedia"
}`

// Nama KLPD/satker/LPSE pada contoh dokumentasi aslinya berakhir dengan karakter lebar-nol; di sini ditulis bersih.
const contohTenderSelesai = `{
  "jenis_pengadaan": "Pekerjaan Konstruksi",
  "pagu": 1234567890,
  "tgl_penetapan_pemenang": "2025-04-17T09:00:00Z",
  "kontrak_pembayaran": "Harga Satuan",
  "tgl_pengumuman_tender": "2025-02-25T13:00:00Z",
  "kd_lpse": 12345,
  "status_tender": "Selesai",
  "sumber_dana": "APBN",
  "kd_satker": "123456",
  "mtd_kualifikasi": "Pasca Satu File",
  "kd_tender": 123456,
  "nama_paket": "Contoh Nama Paket",
  "tahun_anggaran": 2025,
  "jenis_klpd": "KEMENTERIAN",
  "kd_klpd": "KX",
  "nama_klpd": "Contoh Nama KLPD",
  "kd_rup": "12345",
  "mak": "AA.12345.6789.10.11.12",
  "nama_satker": "Contoh Nama Satker",
  "kualifikasi_paket": "Menengah",
  "mtd_pemilihan": "Tender",
  "nama_lpse": "Contoh Nama LPSE",
  "kd_satker_str": "12345",
  "hps": 1234567890,
  "url_lpse": "https://lpse.pu.go.id",
  "last_update_ref": "randomgeneratedstring"
}`

const contohTenderSelesaiNilai = `{
  "hps": 1234567890,
  "jenis_klpd": "KEMENTERIAN",
  "kd_klpd": "KX",
  "kd_lpse": 12345,
  "kd_paket": 123456,
  "kd_penyedia": 123456,
  "kd_rup_paket": "12345",
  "kd_satker": "0.12.3.45.6.78.90.9876",
  "kd_tender": 12345,
  "nama_klpd": "Contoh Nama KLPD",
  "nama_penyedia": "Contoh Nama Penyedia",
  "nama_satker": "Contoh Nama Satker",
  "nilai_kontrak": 1234567890,
  "nilai_negosiasi": 1234567890,
  "nilai_pdn_kontrak": 1234567890,
  "nilai_penawaran": 1234567890,
  "nilai_terkoreksi": 1234567890,
  "nilai_umk_kontrak": 1234567890,
  "npwp_16_penyedia": "01234567890",
  "npwp_penyedia": "01.234.567.8-909.876",
  "pagu": 12345,
  "psr_id": 12345,
  "tahun_anggaran": 2024,
  "tgl_penetapan_pemenang": "2024-01-10T13:21:52.557Z",
  "tgl_pengumuman_tender": "2024-01-10 13:30:00.000000Z"
}`

const contohEkatalogInstansiSatker = `{
  "jenis_klpd": "KEMENTERIAN",
  "kd_klpd": "KX",
  "kd_satker": 12345,
  "kd_satker_str": "12345",
  "nama_klpd": "Kementerian Contoh",
  "nama_satker": "SATKER CONTOH 01"
}`

const contohEkatalogKomoditas = `{
  "Jenis_Katalog": "KATALOG NASIONAL",
  "kd_instansi_katalog": null,
  "kd_komoditas": 999,
  "nama_instansi_katalog": null,
  "nama_komoditas": "Layanan Internet Contoh"
}`

const contohEkatalogPaket = `{
  "alamat_satker": "Jl. Contoh No. 123",
  "catatan_produk": "catatan produk contoh",
  "deskripsi": "deskripsi paket contoh",
  "email_user_pokja": "pokja@example.com",
  "harga_satuan": 5000000,
  "jabatan_ppk": "PPK",
  "jml_jenis_produk": 2,
  "kd_kabupaten_wilayah_harga": 10,
  "kd_klpd": "KX",
  "kd_komoditas": 100,
  "kd_paket": 1234567,
  "kd_paket_produk": 7654321,
  "kd_penyedia": 111111,
  "kd_penyedia_distributor": 222222,
  "kd_produk": 333333,
  "kd_provinsi_wilayah_harga": 5,
  "kd_rup": 9876543,
  "kd_user_pokja": 12345,
  "kd_user_ppk": 54321,
  "kode_anggaran": "ANGGARAN.CONTOH.001",
  "kuantitas": 4,
  "nama_paket": "Pengadaan Perangkat Contoh",
  "nama_satker": "Satker Contoh",
  "nama_sumber_dana": "APBN",
  "no_paket": "PKT-2024-0001",
  "no_telp_user_pokja": "62081234567890",
  "npwp_satker": "001234567890000",
  "ongkos_kirim": 0,
  "paket_status_str": "Paket Selesai",
  "ppk_nip": "198201012000010001",
  "satker_id": 9999,
  "status_paket": "paket_selesai",
  "tahun_anggaran": 2024,
  "tanggal_buat_paket": "2024-01-02",
  "tanggal_edit_paket": "2024-01-03",
  "total_harga": 20000000
}`

const contohEkatalogPenyedia = `{
  "alamat_penyedia": "Jl. Raya Contoh No. 9",
  "email_penyedia": "info@penyediacontoh.id",
  "kbli2020_penyedia": "C12345;G67890",
  "kd_penyedia": 42,
  "kode_penyedia_sikap": 555001,
  "nama_penyedia": "PT Penyedia Contoh",
  "no_telp_penyedia": "0211234567",
  "npwp_16": null,
  "npwp_penyedia": "001234567890000",
  "penyedia_ukm": "Usaha Kecil"
}`

const contohEkatalogDistributor = `{
  "alamat_distributor": "Kompleks Niaga Contoh Blok A1",
  "email_distributor": "distributor@example.com",
  "kd_penyedia_distributor": 8888,
  "nama_distributor": "PT Distributor Contoh",
  "no_telp_distributor": "081212345678",
  "npwp_distributor": "01.234.567.8-901.000"
}`

type jenisUji struct {
	nama      string
	ep        *endpointDatar
	migrasi   string
	contoh    string
	jumlahFld int // jumlah field pada contoh dokumentasi
	// nilai yang harus muncul pada kolom (setelah pemetaan) untuk contoh dengan kd_klpd = "K10" diminta
	harap map[string]interface{}
}

func semuaJenisUji() []jenisUji {
	return []jenisUji{
		{
			nama: "non-tender-selesai", ep: nonTenderSelesai, migrasi: "024_create_inaproc_non_tender_selesai.sql", contoh: contohNonTenderSelesai, jumlahFld: 37,
			harap: map[string]interface{}{
				"kd_nontender": "123456", "kd_penyedia": "123456", "lpse_id": "1234567", "kd_lpse": "12345", "tahun_anggaran": "2024",
				"npwp_penyedia": "12.345.678.9-098.765", "status_nontender": "Selesai", "jenis_klpd": nil, "kd_satker_str": nil,
				"kd_klpd": "K10", // null di API: diisi kode KLPD yang diminta
				"pagu":    "12345", "hps": "1234567890", "nilai_kontrak": nil, "nilai_negosiasi": "12345",
			},
		},
		{
			nama: "pencatatan-non-tender", ep: pencatatanNonTender, migrasi: "025_create_inaproc_pencatatan_non_tender.sql", contoh: contohPencatatanNonTender, jumlahFld: 30,
			harap: map[string]interface{}{
				"kd_nontender_pct": "2024", "kd_klpd": "DX", "status_nontender_pct": "Aktif", "status_nontender_pct_ket": "Paket Selesai",
				"alasan_pembatalan": nil, "informasi_lainnya": nil, "bukti_pembayaran": "Contoh Bukti Pembayaran",
				"pagu": "12345", "nilai_pdn_pct": "0", "nilai_umk_pct": "0", "total_realisasi": "12345",
			},
		},
		{
			nama: "pencatatan-non-tender-realisasi", ep: pencatatanNonTenderRealisasi, migrasi: "026_create_inaproc_pencatatan_non_tender_realisasi.sql", contoh: contohPencatatanNonTenderRealisasi, jumlahFld: 24,
			harap: map[string]interface{}{
				"kd_rup_paket": "12345", "kd_paket_dce": "12345", "kd_nontender_pct": "12345", "no_realisasi": "ABC/ABC/I/2024", "dok_realisasi": nil,
				"kd_klpd": "DX", "nilai_realisasi": "12345", "pagu": "12345", "npwp_penyedia": "01.234.567.8-909.876",
			},
		},
		{
			nama: "pencatatan-swakelola", ep: pencatatanSwakelola, migrasi: "027_create_inaproc_pencatatan_swakelola.sql", contoh: contohPencatatanSwakelola, jumlahFld: 29,
			harap: map[string]interface{}{
				"kd_swakelola_pct": "1234567890", "kd_lpse": "12345", "kd_pkt_dce": "12345", "tahun_anggaran": "2024", "kd_klpd": "DX",
				"tipe_swakelola": "1", "tipe_swakelola_nama": "Tipe X", "nip_ppk": "12345",
				"status_swakelola_pct": "Aktif", "status_swakelola_pct_ket": "Paket Selesai", "alasan_pembatalan": nil, "informasi_lainnya": nil,
				"pagu": "12345", "nilai_pdn_pct": "0", "nilai_umk_pct": "0", "total_realisasi": "12345",
			},
		},
		{
			nama: "pencatatan-swakelola-realisasi", ep: pencatatanSwakelolaRealisasi, migrasi: "028_create_inaproc_pencatatan_swakelola_realisasi.sql", contoh: contohPencatatanSwakelolaRealisasi, jumlahFld: 16,
			harap: map[string]interface{}{
				"kd_swakelola_pct": "12345", "rsk_id": "12345", "kd_lpse": "12345", "nip_ppk": "12345", "kd_klpd": "DX", "tahun_anggaran": "2024",
				"no_realisasi": "123/AA/123", "dok_realisasi": "SPKS", "nama_pelaksana": "Contoh Nama Pelaksana", "npwp_pelaksana": "01.234.567.8-909.876",
				"nilai_realisasi": "12345",
			},
		},
		{
			nama: "pengumuman", ep: tenderPengumuman, migrasi: "029_create_inaproc_tender_pengumuman.sql", contoh: contohTenderPengumuman, jumlahFld: 37,
			harap: map[string]interface{}{
				"kd_tender": "12345", "kd_lpse": "12345", "kd_pkt_dce": "123456", "kd_klpd": "KX", "tahun_anggaran": "2024", "list_tahun_anggaran": "2024",
				"versi_tender": int64(1), "ket_ditutup": nil, "ket_diulang": nil, "nip_pokja": "123456789", "status_tender": "Selesai",
				"pagu": "12345", "hps": "1234567890",
			},
		},
		{
			nama: "peserta-tender", ep: tenderPeserta, migrasi: "030_create_inaproc_peserta_tender.sql", contoh: contohPesertaTender, jumlahFld: 17,
			harap: map[string]interface{}{
				"kd_tender": "12345", "kd_peserta": "12345", "kd_penyedia": "12345", "kd_klpd": "KX", "tahun_anggaran": "2024",
				"npwp_penyedia_16": "12345", "alasan": nil, "pemenang": int64(0), "pemenang_terverifikasi": int64(0),
				"npwp_penyedia":   nil, // "" di API menjadi NULL
				"nilai_penawaran": nil, "nilai_terkoreksi": nil,
			},
		},
		{
			nama: "tender-ekontrak", ep: tenderEkontrak, migrasi: "031_create_inaproc_tender_ekontrak.sql", contoh: contohTenderEkontrak, jumlahFld: 50,
			harap: map[string]interface{}{
				"kd_tender": "12345", "kd_klpd": "DX", "no_kontrak": "123/AA/123", "no_rek_bank": "123456", "npwp_16_penyedia": "1234567890",
				"nilai_kontrak": "12345.67", "nilai_pdn_kontrak": "12345.67", "versi_addendum": int64(0),
				"alasan_ubah_nilai_kontrak": nil, "informasi_lainnya": nil, "anggota_kso": nil,
				"tgl_penetapan_status_kontrak": nil, // null di contoh
			},
		},
		{
			nama: "tender-ekontrak-kontrak", ep: tenderEkontrakKontrak, migrasi: "032_create_inaproc_tender_ekontrak_kontrak.sql", contoh: contohTenderEkontrakKontrak, jumlahFld: 47,
			harap: map[string]interface{}{
				"kd_tender": "12345", "kd_klpd": "DX", "no_kontrak": "123/AA/123", "no_rek_bank": "123456", "npwp_16_penyedia": "1234567890",
				"nilai_kontrak": "12345.67", "nilai_umk_kontrak": "12345.67", "versi_addendum": int64(0),
				"alasan_ubah_nilai_kontrak": nil, "informasi_lainnya": nil, "anggota_kso": nil,
				"tgl_penetapan_status_kontrak": nil,
			},
		},
		{
			nama: "tender-selesai", ep: tenderSelesai, migrasi: "033_create_inaproc_tender_selesai.sql", contoh: contohTenderSelesai, jumlahFld: 26,
			harap: map[string]interface{}{
				"kd_tender": "123456", "kd_lpse": "12345", "kd_klpd": "KX", "tahun_anggaran": "2025", "status_tender": "Selesai",
				"last_update_ref": "randomgeneratedstring", "pagu": "1234567890", "hps": "1234567890",
			},
		},
		{
			nama: "tender-selesai-nilai", ep: tenderSelesaiNilai, migrasi: "034_create_inaproc_tender_selesai_nilai.sql", contoh: contohTenderSelesaiNilai, jumlahFld: 25,
			harap: map[string]interface{}{
				"kd_tender": "12345", "kd_paket": "123456", "kd_penyedia": "123456", "kd_rup_paket": "12345", "psr_id": "12345",
				"kd_satker": "0.12.3.45.6.78.90.9876", "npwp_16_penyedia": "01234567890", "tahun_anggaran": "2024",
				"pagu": "12345", "hps": "1234567890", "nilai_penawaran": "1234567890", "nilai_kontrak": "1234567890",
			},
		},
		{
			nama: "instansi-satker", ep: ekatalogInstansiSatker, migrasi: "035_create_inaproc_ekatalog_instansi_satker.sql", contoh: contohEkatalogInstansiSatker, jumlahFld: 6,
			harap: map[string]interface{}{"kd_klpd": "KX", "kd_satker": "12345", "kd_satker_str": "12345", "nama_satker": "SATKER CONTOH 01"},
		},
		{
			nama: "komoditas-detail", ep: ekatalogKomoditas, migrasi: "036_create_inaproc_ekatalog_komoditas.sql", contoh: contohEkatalogKomoditas, jumlahFld: 5,
			harap: map[string]interface{}{
				"Jenis_Katalog": "KATALOG NASIONAL", "kd_komoditas": "999", "nama_komoditas": "Layanan Internet Contoh",
				"kd_instansi_katalog": nil, "nama_instansi_katalog": nil,
			},
		},
		{
			nama: "paket-e-purchasing", ep: ekatalogPaket, migrasi: "037_create_inaproc_ekatalog_paket_epurchasing.sql", contoh: contohEkatalogPaket, jumlahFld: 36,
			harap: map[string]interface{}{
				"kd_paket": "1234567", "kd_paket_produk": "7654321", "kd_rup": "9876543", "kd_user_pokja": "12345", "satker_id": "9999",
				"no_telp_user_pokja": "62081234567890", "ppk_nip": "198201012000010001", "tahun_anggaran": "2024", "kd_klpd": "KX",
				"status_paket": "paket_selesai", "paket_status_str": "Paket Selesai", "jml_jenis_produk": int64(2),
				"harga_satuan": "5000000", "kuantitas": "4", "ongkos_kirim": "0", "total_harga": "20000000",
			},
		},
		{
			nama: "penyedia-detail", ep: ekatalogPenyedia, migrasi: "038_create_inaproc_ekatalog_penyedia.sql", contoh: contohEkatalogPenyedia, jumlahFld: 10,
			harap: map[string]interface{}{
				"kd_penyedia": "42", "kode_penyedia_sikap": "555001", "npwp_16": nil, "npwp_penyedia": "001234567890000",
				"kbli2020_penyedia": "C12345;G67890", "penyedia_ukm": "Usaha Kecil",
			},
		},
		{
			nama: "penyedia-distributor-detail", ep: ekatalogDistributor, migrasi: "039_create_inaproc_ekatalog_distributor.sql", contoh: contohEkatalogDistributor, jumlahFld: 6,
			harap: map[string]interface{}{"kd_penyedia_distributor": "8888", "npwp_distributor": "01.234.567.8-901.000", "no_telp_distributor": "081212345678"},
		},
	}
}

// Bentuk saringan tiap endpoint (bawaan, hanya KLPD, atau satu kode) menentukan parameter yang sah dan kolom yang diganti
// sinkronisasi. Pembantu di bawah menjaga agar tes yang berlaku untuk semua endpoint tetap satu.
const kodeUji = "KODE123"

func (j jenisUji) modeBawaan() bool { return j.ep.Saring == (saringan{}) }

func (j jenisUji) jalur() string { return "/inaproc/" + j.ep.awalan() + "/" + j.nama }

// query: GET yang sah untuk endpoint ini, ditambah pasangan lain (mis. limit).
func (j jenisUji) query(tambahan ...string) string {
	var p []string
	switch {
	case j.ep.Saring.Param != "":
		p = []string{j.ep.Saring.Param + "=" + kodeUji}
	case j.ep.Saring.TanpaTahun:
	default:
		p = []string{"tahun=2024"}
	}
	p = append(p, tambahan...)
	if len(p) == 0 {
		return ""
	}
	return "?" + strings.Join(p, "&")
}

// paramHulu: pasangan yang harus ada pada permintaan ke Inaproc untuk query() di atas.
func (j jenisUji) paramHulu() []string {
	switch {
	case j.ep.Saring.Param != "":
		return []string{j.ep.Saring.Param + "=" + kodeUji}
	case j.ep.Saring.TanpaTahun:
		return []string{"kode_klpd=K10"}
	}
	return []string{"kode_klpd=K10", "tahun=2024"}
}

func (j jenisUji) bodySync() string {
	switch {
	case j.ep.Saring.Param != "":
		return `{"kode":"` + kodeUji + `"}`
	case j.ep.Saring.TanpaTahun:
		return `{}`
	}
	return `{"tahun":"2024"}`
}

// argHapus: argumen DELETE yang diharapkan pada sinkronisasi.
func (j jenisUji) argHapus() []string {
	switch {
	case j.ep.Saring.Param != "":
		return []string{kodeUji}
	case j.ep.Saring.TanpaTahun:
		return []string{"K10"}
	}
	return []string{"K10", "2024"}
}

// wajibAdaSyarat: GET tanpa parameter apa pun harus ditolak (saringan hanya-KLPD punya bawaan K10, jadi tidak).
func (j jenisUji) wajibAdaSyarat() bool { return j.ep.Saring.Param != "" || !j.ep.Saring.TanpaTahun }

func (j jenisUji) kolomWajib() []string {
	switch {
	case j.ep.Saring.Param != "":
		return []string{j.ep.Saring.Kolom}
	case j.ep.Saring.TanpaTahun:
		return []string{"kd_klpd"}
	}
	return []string{"kd_klpd", "tahun_anggaran"}
}

// wherelokal: potongan SQL daftar lokal yang diharapkan untuk query() dengan limit.
func (j jenisUji) whereLokal() string {
	switch {
	case j.ep.Saring.Param != "":
		return "FROM " + j.ep.Tabel + " WHERE " + j.ep.Saring.Kolom + " = @p1"
	case j.ep.Saring.TanpaTahun:
		return "FROM " + j.ep.Tabel + " WHERE kd_klpd = @p1"
	}
	return "FROM " + j.ep.Tabel + " WHERE kd_klpd = @p1 AND tahun_anggaran = @p2"
}

// varianBaris mengubah satu field teks pada contoh supaya baris yang dikirim tiap halaman berbeda (row_key berbeda).
func varianBaris(contoh, label string) string {
	for _, kunci := range []string{`"nama_paket": "`, `"no_realisasi": "`, `"nama_penyedia": "`, `"nama_satker": "`, `"nama_komoditas": "`, `"nama_distributor": "`} {
		if strings.Contains(contoh, kunci) {
			return strings.Replace(contoh, kunci, kunci+label+" ", 1)
		}
	}
	return contoh
}

func contohBaris(t *testing.T, js string) map[string]interface{} {
	t.Helper()
	var row map[string]interface{}
	if err := json.Unmarshal([]byte(js), &row); err != nil {
		t.Fatal(err)
	}
	return row
}

// Huruf besar/kecil tidak dibedakan pada pencocokan, supaya nama seperti "Jenis_Katalog" (huruf besar di API) ikut terbaca;
// perbandingan dengan daftar field tetap persis.
var reKolomMigrasi = regexp.MustCompile(`(?mi)^\s+([a-z0-9_]+)\s+(?:NVARCHAR|DECIMAL|DATETIME2|INT|BIGINT)\b`)

// Kolom di migrasi harus sama dengan daftar field di deklarasi: selisihnya baru ketahuan saat INSERT gagal di server.
func TestEndpointDatarKolomSamaDenganMigrasi(t *testing.T) {
	for _, j := range semuaJenisUji() {
		t.Run(j.nama, func(t *testing.T) {
			b, err := os.ReadFile("../migrations/" + j.migrasi)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b), "CREATE TABLE "+j.ep.Tabel+" (") {
				t.Errorf("migrasi %s tidak membuat tabel %s", j.migrasi, j.ep.Tabel)
			}
			var kolom []string
			for _, m := range reKolomMigrasi.FindAllStringSubmatch(string(b), -1) {
				switch m[1] {
				case "row_key", "extra_json", "synced_at", "updated_at":
					continue
				}
				kolom = append(kolom, m[1])
			}
			handler := j.ep.KnownFields()
			sort.Strings(kolom)
			sort.Strings(handler)
			if strings.Join(kolom, ",") != strings.Join(handler, ",") {
				t.Fatalf("kolom migrasi dan daftar field berbeda.\nmigrasi: %v\nhandler: %v", kolom, handler)
			}
			seen := map[string]bool{}
			for _, f := range handler {
				if seen[f] {
					t.Errorf("field %q terdaftar lebih dari sekali", f)
				}
				seen[f] = true
			}
			// Kolom penyaring sinkronisasi dan daftar lokal harus ada.
			for _, wajib := range j.kolomWajib() {
				if !seen[wajib] {
					t.Errorf("kolom %q wajib ada (dipakai hapus-sebelum-tarik dan daftar lokal)", wajib)
				}
			}
			if !strings.Contains(j.ep.KolomDaftar, "synced_at") {
				t.Error("KolomDaftar harus memuat synced_at (dipakai untuk mengurutkan)")
			}
			// Salah ketik di KolomDaftar baru ketahuan saat daftar lokal dibuka di server sungguhan.
			ada := map[string]bool{"row_key": true, "synced_at": true}
			for _, f := range handler {
				ada[f] = true
			}
			for _, k := range strings.Split(j.ep.KolomDaftar, ",") {
				if k = strings.TrimSpace(k); !ada[k] {
					t.Errorf("KolomDaftar memuat %q yang bukan kolom tabel", k)
				}
			}
		})
	}
}

func TestEndpointDatarSemuaFieldDokumentasiDipetakan(t *testing.T) {
	for _, j := range semuaJenisUji() {
		t.Run(j.nama, func(t *testing.T) {
			row := contohBaris(t, j.contoh)
			if len(row) != j.jumlahFld {
				t.Fatalf("contoh dokumentasi memuat %d field, want %d", len(row), j.jumlahFld)
			}
			if extra := extraFieldsColumn(row, j.ep.KnownFields()); extra != nil {
				t.Errorf("ada field dokumentasi yang belum punya kolom (masuk extra_json): %v", extra)
			}
			// Field baru dari API tidak boleh hilang diam-diam.
			row["field_baru"] = "x"
			extra, _ := extraFieldsColumn(row, j.ep.KnownFields()).(string)
			if !strings.Contains(extra, `"field_baru":"x"`) {
				t.Errorf("extra_json = %q", extra)
			}
		})
	}
}

func TestEndpointDatarInsertSQLDanArgumen(t *testing.T) {
	for _, j := range semuaJenisUji() {
		t.Run(j.nama, func(t *testing.T) {
			cols := regexp.MustCompile(`\(([^)]*)\) VALUES`).FindStringSubmatch(j.ep.insertSQL)
			if cols == nil || !strings.HasPrefix(j.ep.insertSQL, "INSERT INTO "+j.ep.Tabel+" (") {
				t.Fatalf("INSERT = %s", j.ep.insertSQL)
			}
			nama := strings.Split(strings.ReplaceAll(cols[1], " ", ""), ",")
			args := j.ep.Args(contohBaris(t, j.contoh), "K10")
			if len(nama) != len(args) || strings.Count(j.ep.insertSQL, "@p") != len(args) {
				t.Fatalf("kolom = %d, placeholder = %d, argumen = %d", len(nama), strings.Count(j.ep.insertSQL, "@p"), len(args))
			}
			if nama[0] != "row_key" || nama[len(nama)-1] != "extra_json" || len(nama) != j.jumlahFld+2 {
				t.Errorf("kolom = %v", nama)
			}
			nilai := map[string]interface{}{}
			for i, n := range nama {
				nilai[n] = args[i]
			}
			for k, want := range j.harap {
				if nilai[k] != want {
					t.Errorf("%s = %#v, want %#v", k, nilai[k], want)
				}
			}
			for _, f := range j.ep.Tanggal {
				if want, terdaftar := j.harap[f]; terdaftar && want == nil {
					continue // null di contoh dokumentasi: sudah diperiksa lewat harap
				}
				if _, ok := nilai[f].(time.Time); !ok {
					t.Errorf("%s = %#v, harus terbaca sebagai waktu", f, nilai[f])
				}
			}
			if nilai["extra_json"] != nil {
				t.Errorf("extra_json = %#v, want nil", nilai["extra_json"])
			}
			a, b := j.ep.Args(contohBaris(t, j.contoh), "K10")[0], j.ep.Args(contohBaris(t, j.contoh), "K10")[0]
			if a != b || a == "" {
				t.Errorf("row_key tidak stabil: %v vs %v", a, b)
			}
		})
	}
}

func TestEndpointDatarFormatTanggalInaproc(t *testing.T) {
	// Dua bentuk yang dipakai Inaproc: RFC3339 (dengan pecahan detik) dan "yyyy-MM-dd HH:mm:ss.ffffffZ".
	a := nonTenderSelesai.Args(contohBaris(t, contohNonTenderSelesai), "K10")
	pos := map[string]int{}
	for i, n := range append([]string{"row_key"}, nonTenderSelesai.KnownFields()...) {
		pos[n] = i
	}
	for k, want := range map[string]time.Time{
		"tgl_penarikan":            time.Date(2024, 1, 5, 9, 7, 6, 500_000_000, time.UTC),
		"tgl_pengumuman_nontender": time.Date(2024, 1, 10, 13, 30, 0, 0, time.UTC),
		"tgl_selesai_nontender":    time.Date(2024, 1, 15, 15, 45, 0, 0, time.UTC),
	} {
		got, ok := a[pos[k]].(time.Time)
		if !ok || !got.Equal(want) {
			t.Errorf("%s = %#v, want %v", k, a[pos[k]], want)
		}
	}
}

func TestEndpointDatarObjekDanLarikDisimpanSebagaiJSON(t *testing.T) {
	// dok_realisasi belum terdokumentasi (null di contoh); bila API mengirim objek/larik, isinya tidak boleh hilang.
	row := contohBaris(t, contohPencatatanNonTenderRealisasi)
	row["dok_realisasi"] = []interface{}{map[string]interface{}{"nama": "bukti.pdf", "url": "https://x.invalid/bukti.pdf"}}
	args := pencatatanNonTenderRealisasi.Args(row, "K10")
	pos := -1
	for i, n := range append([]string{"row_key"}, pencatatanNonTenderRealisasi.KnownFields()...) {
		if n == "dok_realisasi" {
			pos = i
		}
	}
	s, _ := args[pos].(string)
	if !strings.Contains(s, `"nama":"bukti.pdf"`) {
		t.Errorf("dok_realisasi = %#v, want JSON berisi bukti.pdf", args[pos])
	}
}

// argKolom mengambil argumen INSERT milik kolom bernama `nama` dari hasil Args.
func argKolom(ep *endpointDatar, args []interface{}, nama string) interface{} {
	for i, n := range append([]string{"row_key"}, ep.KnownFields()...) {
		if n == nama {
			return args[i]
		}
	}
	panic("kolom tidak ada: " + nama)
}

func TestTenderEkontrakRiwayatDisimpanSebagaiJSON(t *testing.T) {
	args := tenderEkontrak.Args(contohBaris(t, contohTenderEkontrak), "K10")

	var bap, spmk, nilai []map[string]interface{}
	for nama, tujuan := range map[string]*[]map[string]interface{}{
		"bapbast_history_json": &bap, "spmkspp_history_json": &spmk, "penilaian_kinerja_penyedia": &nilai,
	} {
		s, ok := argKolom(tenderEkontrak, args, nama).(string)
		if !ok {
			t.Fatalf("%s = %#v, harus teks JSON", nama, argKolom(tenderEkontrak, args, nama))
		}
		if err := json.Unmarshal([]byte(s), tujuan); err != nil || len(*tujuan) != 1 {
			t.Fatalf("%s = %q: %v", nama, s, err)
		}
	}
	if bap[0]["no_bap"] != "05.3/BAP/DATA/XII/2021" || bap[0]["besar_pembayaran"] != float64(898975000) {
		t.Errorf("bapbast = %v", bap[0])
	}
	// Format tanggal di dalam riwayat tidak diubah; frontend yang menampilkannya.
	if spmk[0]["tgl_mulai_pekerjaan"] != "May 7, 2021 12:00:00 AM" || spmk[0]["waktu_penyelesaian"] != "240 hari kalender" {
		t.Errorf("spmkspp = %v", spmk[0])
	}
	if nilai[0]["indikator_penilaian"] != "Kualitas" || nilai[0]["nilai_indikator"] != float64(4) {
		t.Errorf("penilaian = %v", nilai[0])
	}

	// Baris minimal (contoh kedua di dokumentasi): larik kosong tersimpan sebagai "[]", field yang tidak dikirim menjadi NULL.
	min := tenderEkontrak.Args(contohBaris(t, contohTenderEkontrakMinimal), "K10")
	for nama, want := range map[string]interface{}{
		"bapbast_history_json": "[]", "spmkspp_history_json": "[]", "penilaian_kinerja_penyedia": nil,
		"kd_tender": "67890", "nama_paket": "Contoh Paket Tanpa History", "nama_penyedia": nil, "nilai_kontrak": nil, "versi_addendum": nil,
	} {
		if got := argKolom(tenderEkontrak, min, nama); got != want {
			t.Errorf("baris minimal: %s = %#v, want %#v", nama, got, want)
		}
	}
	if extra := min[len(min)-1]; extra != nil {
		t.Errorf("extra_json = %#v, want nil", extra)
	}
}

// Contoh dokumentasi tender-selesai memuat karakter lebar-nol di akhir nama. Nilai disimpan apa adanya (tidak dibuang diam-diam,
// tidak membuat baris gagal); kolom nama di migrasi 033 dibuat cukup lebar untuk itu.
func TestEndpointDatarKarakterLebarNolDisimpanApaAdanya(t *testing.T) {
	row := contohBaris(t, contohTenderSelesai)
	nama := "Contoh Nama Satker⁣‌⁠⁠‌‌⁠‌⁤"
	row["nama_satker"] = nama
	args := tenderSelesai.Args(row, "K10")
	if got := argKolom(tenderSelesai, args, "nama_satker"); got != nama {
		t.Errorf("nama_satker = %q, want %q", got, nama)
	}
	b, err := os.ReadFile("../migrations/033_create_inaproc_tender_selesai.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`nama_satker NVARCHAR\((\d+)\)`).FindStringSubmatch(string(b))
	if m == nil {
		t.Fatal("kolom nama_satker tidak ditemukan di migrasi 033")
	}
	if lebar, _ := strconv.Atoi(m[1]); lebar < 1000 {
		t.Errorf("nama_satker NVARCHAR(%d) terlalu sempit untuk nama bersisipan karakter tak terlihat (minimal 1000)", lebar)
	}
}

// ---- endpoint HTTP dengan server Inaproc tiruan ----

type inaprocPalsu struct {
	*httptest.Server
	mu        sync.Mutex
	diminta   []string // path + query tiap permintaan
	otorisasi []string
	balas     func(r *http.Request) (int, string)
}

func newInaprocPalsu(t *testing.T, balas func(r *http.Request) (int, string)) *inaprocPalsu {
	t.Helper()
	p := &inaprocPalsu{balas: balas}
	p.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.diminta = append(p.diminta, r.URL.Path+"?"+r.URL.RawQuery)
		p.otorisasi = append(p.otorisasi, r.Header.Get("Authorization"))
		p.mu.Unlock()
		kode, isi := p.balas(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(kode)
		_, _ = w.Write([]byte(isi))
	}))
	t.Cleanup(p.Close)
	return p
}

func pasangKonfigInaproc(t *testing.T, base, token string) {
	t.Helper()
	lama := config.Cfg
	config.Cfg = &config.Config{InaprocBaseURL: base, InaprocToken: token, HTTPClientTimeoutSeconds: 5}
	t.Cleanup(func() { config.Cfg = lama })
}

func pasangDBPalsu(t *testing.T) *fakesql.DB {
	t.Helper()
	db, f := fakesql.New(t)
	lama := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = lama })
	return f
}

func routerDatar() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/inaproc", func(c *gin.Context) { c.Set("user_id", "admin-uji"); c.Next() })
	for _, j := range semuaJenisUji() {
		dasar := "/" + j.ep.awalan() + "/" + j.nama
		g.GET(dasar, j.ep.Get)
		g.GET(dasar+"/local", j.ep.ListLocal)
		g.POST(dasar+"/sync", j.ep.Sync)
	}
	return r
}

func panggil(r *gin.Engine, method, path, body string) (int, map[string]interface{}) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var m map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w.Code, m
}

func TestEndpointDatarGetMeneruskanParameterDanBatasLimit(t *testing.T) {
	for _, j := range semuaJenisUji() {
		t.Run(j.nama, func(t *testing.T) {
			p := newInaprocPalsu(t, func(r *http.Request) (int, string) {
				return 200, `{"success":true,"data":[` + j.contoh + `],"meta":{"limit":1000,"has_more":true,"cursor":"abc"}}`
			})
			pasangKonfigInaproc(t, p.URL, "token-uji")

			code, body := panggil(routerDatar(), "GET", j.jalur()+j.query("limit=5000", "cursor=c0"), "")
			if code != 200 || len(body["data"].([]interface{})) != 1 || body["meta"].(map[string]interface{})["cursor"] != "abc" {
				t.Fatalf("status = %d, body = %v", code, body)
			}
			if len(p.diminta) != 1 {
				t.Fatalf("permintaan ke Inaproc = %v", p.diminta)
			}
			want := append([]string{"/api/v1/" + j.ep.awalan() + "/" + j.nama + "?", "limit=1000", "cursor=c0"}, j.paramHulu()...)
			for _, w := range want {
				if !strings.Contains(p.diminta[0], w) {
					t.Errorf("permintaan %q tidak memuat %q", p.diminta[0], w)
				}
			}
			// Pada saringan kode tunggal, kode_klpd/tahun tidak ikut dikirim (API-nya tidak mengenalnya).
			if j.ep.Saring.Param != "" && (strings.Contains(p.diminta[0], "kode_klpd=") || strings.Contains(p.diminta[0], "tahun=")) {
				t.Errorf("permintaan %q tidak boleh memuat kode_klpd/tahun", p.diminta[0])
			}
			if p.otorisasi[0] != "Bearer token-uji" {
				t.Errorf("Authorization = %q", p.otorisasi[0])
			}
		})
	}
}

func TestEndpointDatarGetValidasiDanGalatHulu(t *testing.T) {
	p := newInaprocPalsu(t, func(r *http.Request) (int, string) {
		return 429, `{"success":false,"error":{"code":"Too Many Requests","message":"Rate limit exceeded","details":"Please retry later"},"meta":{"request_id":"req-1"}}`
	})
	pasangKonfigInaproc(t, p.URL, "token-uji")
	r := routerDatar()

	for _, j := range semuaJenisUji() {
		sebelum := len(p.diminta)
		if j.wajibAdaSyarat() {
			if code, _ := panggil(r, "GET", j.jalur(), ""); code != 400 {
				t.Errorf("%s tanpa parameter wajib: %d, want 400", j.nama, code)
			}
			if len(p.diminta) != sebelum {
				t.Errorf("%s tanpa parameter wajib tidak boleh menghubungi Inaproc: %v", j.nama, p.diminta[sebelum:])
			}
		}
		// Galat dari Inaproc diteruskan apa adanya (status dan badan).
		code, body := panggil(r, "GET", j.jalur()+j.query(), "")
		if code != 429 || body["error"].(map[string]interface{})["code"] != "Too Many Requests" {
			t.Errorf("%s: 429 harus diteruskan: %d %v", j.nama, code, body)
		}
	}
	// Token belum dikonfigurasi.
	pasangKonfigInaproc(t, p.URL, "")
	if code, _ := panggil(r, "GET", "/inaproc/tender/pencatatan-non-tender?tahun=2024", ""); code != 503 {
		t.Errorf("tanpa token: %d, want 503", code)
	}
	// Data null dari Inaproc menjadi larik kosong untuk frontend.
	q := newInaprocPalsu(t, func(r *http.Request) (int, string) {
		return 200, `{"success":true,"data":null,"meta":{"limit":50,"has_more":false,"cursor":""}}`
	})
	pasangKonfigInaproc(t, q.URL, "token-uji")
	_, body := panggil(r, "GET", "/inaproc/tender/pencatatan-non-tender-realisasi?tahun=2024", "")
	if d, ok := body["data"].([]interface{}); !ok || len(d) != 0 {
		t.Errorf("data = %#v, want []", body["data"])
	}
}

// tender/pengumuman punya dua skenario di Inaproc: (tahun + kode_klpd) atau kd_tender saja. Endpoint lain tetap wajib tahun.
func TestEndpointDatarGetKdTenderMenggantikanTahunDanKlpd(t *testing.T) {
	p := newInaprocPalsu(t, func(r *http.Request) (int, string) {
		return 200, `{"success":true,"data":[` + contohTenderPengumuman + `],"meta":{"limit":10,"has_more":false,"cursor":""}}`
	})
	pasangKonfigInaproc(t, p.URL, "token-uji")
	r := routerDatar()

	// kd_tender saja, atau bersama tahun/kode_klpd: kd_tender yang menang dan hanya itu (dan limit/cursor) yang diteruskan.
	for _, q := range []string{"?kd_tender=12345", "?kd_tender=12345&tahun=2024&kode_klpd=K10", "?kd_tender=%2012345%20&cursor=c1"} {
		sebelum := len(p.diminta)
		if code, body := panggil(r, "GET", "/inaproc/tender/pengumuman"+q, ""); code != 200 {
			t.Fatalf("%s: status = %d, body = %v", q, code, body)
		}
		diminta := p.diminta[sebelum:]
		if len(diminta) != 1 || !strings.Contains(diminta[0], "kd_tender=12345") || !strings.Contains(diminta[0], "limit=50") {
			t.Fatalf("%s: permintaan = %v", q, diminta)
		}
		for _, dilarang := range []string{"tahun=", "kode_klpd="} {
			if strings.Contains(diminta[0], dilarang) {
				t.Errorf("%s: permintaan %q tidak boleh memuat %q", q, diminta[0], dilarang)
			}
		}
	}

	// Ditolak sebelum menghubungi Inaproc: tanpa kd_tender maupun tahun, atau kd_tender bukan angka.
	for _, q := range []string{"", "?kode_klpd=K10", "?kd_tender=abc", "?kd_tender=12.5", "?kd_tender=99999999999999999999"} {
		sebelum := len(p.diminta)
		if code, _ := panggil(r, "GET", "/inaproc/tender/pengumuman"+q, ""); code != 400 {
			t.Errorf("%q: status = %d, want 400", q, code)
		}
		if len(p.diminta) != sebelum {
			t.Errorf("%q tidak boleh menghubungi Inaproc: %v", q, p.diminta[sebelum:])
		}
	}

	// Skenario (tahun + kode_klpd) tetap seperti biasa.
	sebelum := len(p.diminta)
	if code, _ := panggil(r, "GET", "/inaproc/tender/pengumuman?tahun=2024", ""); code != 200 {
		t.Fatalf("dengan tahun: %d", code)
	}
	if d := p.diminta[sebelum]; !strings.Contains(d, "tahun=2024") || !strings.Contains(d, "kode_klpd=K10") || strings.Contains(d, "kd_tender") {
		t.Errorf("permintaan = %q", d)
	}

	// Endpoint lain tidak mengenal skenario kd_tender: tahun tetap wajib dan kd_tender tidak diteruskan.
	for _, j := range semuaJenisUji() {
		if j.ep.MenerimaKdTender || !j.modeBawaan() {
			continue // saringan lain (hanya KLPD, atau kode tunggal) tidak mewajibkan tahun
		}
		sebelum := len(p.diminta)
		if code, _ := panggil(r, "GET", j.jalur()+"?kd_tender=12345", ""); code != 400 {
			t.Errorf("%s dengan kd_tender saja: %d, want 400", j.nama, code)
		}
		if code, _ := panggil(r, "GET", j.jalur()+"?kd_tender=12345&tahun=2024", ""); code != 200 {
			t.Errorf("%s dengan tahun: %d, want 200", j.nama, code)
		}
		for _, d := range p.diminta[sebelum:] {
			if strings.Contains(d, "kd_tender") {
				t.Errorf("%s: kd_tender tidak boleh diteruskan: %q", j.nama, d)
			}
		}
	}
}

// Endpoint pencarian rujukan per satu kode (komoditas, penyedia, distributor E-Katalog): kode wajib, panjangnya dibatasi, dan
// nilainya hanya pernah menjadi nilai parameter/argumen, tidak pernah menyisipkan parameter lain atau teks SQL.
func TestEndpointDatarSaringanKodeTunggal(t *testing.T) {
	var diminta []string
	p := newInaprocPalsu(t, func(r *http.Request) (int, string) {
		diminta = append(diminta, r.URL.RawQuery)
		return 200, `{"success":true,"data":[` + contohEkatalogPenyedia + `],"meta":{"limit":1000,"has_more":false,"cursor":""}}`
	})
	pasangKonfigInaproc(t, p.URL, "token-uji")
	r := routerDatar()
	jalur := "/inaproc/ekatalog-archive/penyedia-detail"

	// Kode wajib, tidak boleh hanya spasi, dan tidak boleh terlalu panjang: ditolak tanpa menghubungi Inaproc.
	for _, q := range []string{"", "?kode_penyedia=", "?kode_penyedia=%20%20", "?kode_klpd=K10&tahun=2024", "?kode_penyedia=" + strings.Repeat("9", panjangKodeMaks+1)} {
		if code, _ := panggil(r, "GET", jalur+q, ""); code != 400 {
			t.Errorf("GET %q: %d, want 400", q, code)
		}
	}
	if len(diminta) != 0 {
		t.Fatalf("masukan tidak sah tidak boleh menghubungi Inaproc: %v", diminta)
	}

	// Karakter khusus dikodekan sebagai nilai satu parameter; tidak ada parameter tambahan yang bisa diselipkan.
	if code, body := panggil(r, "GET", jalur+"?kode_penyedia=%2042%20%26%20x%3D1%20", ""); code != 200 {
		t.Fatalf("status = %d, body = %v", code, body)
	}
	hulu, err := url.ParseQuery(diminta[0])
	if err != nil {
		t.Fatal(err)
	}
	if hulu.Get("kode_penyedia") != "42 & x=1" || hulu.Has("x") || hulu.Has("kode_klpd") || hulu.Has("tahun") {
		t.Errorf("parameter ke Inaproc = %v", hulu)
	}

	// Sinkronisasi: kode wajib; kode ngawur hanya menjadi argumen DELETE (tidak menyisipkan SQL) dan nilai parameter ke Inaproc.
	f := pasangDBPalsu(t)
	sebelum := len(diminta)
	for _, b := range []string{`{}`, `{"kode":"  "}`, `{"kode":"` + strings.Repeat("9", panjangKodeMaks+1) + `"}`, `{"tahun":"2024"}`, `bukan json`} {
		if code, _ := panggil(r, "POST", jalur+"/sync", b); code != 400 {
			t.Errorf("sync %q: %d, want 400", b, code)
		}
	}
	if len(diminta) != sebelum || len(f.Execs()) != 0 {
		t.Fatalf("masukan tidak sah tidak boleh menyentuh Inaproc/database: %v, %v", diminta[sebelum:], f.Execs())
	}
	const nakal = `4' OR '1'='1`
	if code, body := panggil(r, "POST", jalur+"/sync", `{"kode":" `+nakal+` "}`); code != 200 {
		t.Fatalf("sync: %d %v", code, body)
	}
	hulu, _ = url.ParseQuery(diminta[len(diminta)-1])
	if hulu.Get("kode_penyedia") != nakal {
		t.Errorf("kode ke Inaproc = %q, want %q (dipangkas spasinya)", hulu.Get("kode_penyedia"), nakal)
	}
	for _, ex := range f.Execs() {
		if strings.HasPrefix(ex.Query, "DELETE FROM inaproc_ekatalog_penyedia") {
			if ex.Query != "DELETE FROM inaproc_ekatalog_penyedia WHERE kd_penyedia = @p1" || len(ex.Args) != 1 || ex.Args[0].Value != nakal {
				t.Errorf("DELETE = %q %+v", ex.Query, ex.Args)
			}
		}
	}

	// Daftar lokal tanpa kode = semua baris tabel; dengan kode = disaring pada kolom kodenya.
	g := pasangDBPalsu(t)
	var query string
	var args []driver.NamedValue
	g.OnQuery = func(ctx context.Context, q string, a []driver.NamedValue) ([]string, [][]driver.Value, error) {
		query, args = q, a
		return []string{"row_key"}, [][]driver.Value{{"k1"}}, nil
	}
	if code, _ := panggil(r, "GET", jalur+"/local", ""); code != 200 || strings.Contains(query, "WHERE") || len(args) != 0 {
		t.Errorf("daftar lokal tanpa kode: %d %q %+v", code, query, args)
	}
	if code, _ := panggil(r, "GET", jalur+"/local?kode_penyedia=42", ""); code != 200 || !strings.Contains(query, "WHERE kd_penyedia = @p1") || len(args) != 1 || args[0].Value != "42" {
		t.Errorf("daftar lokal dengan kode: %d %q %+v", code, query, args)
	}
}

// Saringan hanya-KLPD (instansi-satker): tanpa tahun; tahun yang ikut dikirim diabaikan, dan badan sinkronisasi boleh kosong.
func TestEndpointDatarSaringanHanyaKlpd(t *testing.T) {
	var diminta []string
	p := newInaprocPalsu(t, func(r *http.Request) (int, string) {
		diminta = append(diminta, r.URL.Path+"?"+r.URL.RawQuery)
		return 200, `{"success":true,"data":[` + contohEkatalogInstansiSatker + `],"meta":{"limit":1000,"has_more":false,"cursor":""}}`
	})
	pasangKonfigInaproc(t, p.URL, "token-uji")
	r := routerDatar()
	jalur := "/inaproc/ekatalog-archive/instansi-satker"

	for _, q := range []string{"", "?tahun=2024", "?kode_klpd=X99&tahun=2024"} {
		diminta = nil
		if code, body := panggil(r, "GET", jalur+q, ""); code != 200 {
			t.Fatalf("GET %q: %d %v", q, code, body)
		}
		want := "kode_klpd=K10"
		if strings.Contains(q, "X99") {
			want = "kode_klpd=X99"
		}
		if len(diminta) != 1 || !strings.HasPrefix(diminta[0], "/api/v1/ekatalog-archive/instansi-satker?") || !strings.Contains(diminta[0], want) || strings.Contains(diminta[0], "tahun") {
			t.Errorf("GET %q: permintaan = %v, want %s tanpa tahun", q, diminta, want)
		}
	}

	// Badan kosong, {} dan kode_klpd khusus semuanya sah; JSON rusak tidak.
	f := pasangDBPalsu(t)
	for _, b := range []string{"", `{}`, `{"kode_klpd":"X99"}`} {
		if code, body := panggil(r, "POST", jalur+"/sync", b); code != 200 {
			t.Errorf("sync %q: %d %v", b, code, body)
		}
	}
	if code, _ := panggil(r, "POST", jalur+"/sync", `{rusak`); code != 400 {
		t.Errorf("sync JSON rusak: %d, want 400", code)
	}
	var hapus []string
	for _, ex := range f.Execs() {
		if strings.HasPrefix(ex.Query, "DELETE FROM inaproc_ekatalog_instansi_satker") {
			hapus = append(hapus, ex.Query+" "+ex.Args[0].Value.(string))
		}
	}
	if strings.Join(hapus, "|") != strings.Repeat("DELETE FROM inaproc_ekatalog_instansi_satker WHERE kd_klpd = @p1 K10|", 2)+"DELETE FROM inaproc_ekatalog_instansi_satker WHERE kd_klpd = @p1 X99" {
		t.Errorf("DELETE = %v", hapus)
	}
}

// Nama yang dicatat di inaproc_sync_log hanya memakai tanda hubung (kartu aktivitas dashboard memecah nama pada "-"); endpoint
// Tender tetap memakai nama lamanya.
func TestEndpointDatarNamaLog(t *testing.T) {
	for _, tc := range []struct {
		ep   *endpointDatar
		want string
	}{
		{ekatalogInstansiSatker, "ekatalog-archive-instansi-satker"},
		{ekatalogDistributor, "ekatalog-archive-penyedia-distributor-detail"},
		{tenderPengumuman, "pengumuman"},
		{pencatatanNonTender, "pencatatan-non-tender"},
	} {
		if got := tc.ep.namaLog(); got != tc.want || len(got) > 100 || strings.Contains(got, "/") {
			t.Errorf("namaLog = %q, want %q (maks. 100 karakter, tanpa '/')", got, tc.want)
		}
	}
}

func TestEndpointDatarSyncMenelusuriHalamanDanMenyimpan(t *testing.T) {
	for _, j := range semuaJenisUji() {
		t.Run(j.nama, func(t *testing.T) {
			halaman2 := varianBaris(j.contoh, "Halaman2")
			halaman1b := varianBaris(j.contoh, "Lain")
			if halaman2 == j.contoh || halaman1b == j.contoh {
				t.Fatal("contoh tidak punya field teks untuk dibedakan antarbaris")
			}
			p := newInaprocPalsu(t, func(r *http.Request) (int, string) {
				if r.URL.Query().Get("cursor") == "" {
					return 200, `{"success":true,"data":[` + j.contoh + `,` + halaman1b + `],"meta":{"limit":1000,"has_more":true,"cursor":"c1"}}`
				}
				return 200, `{"success":true,"data":[` + halaman2 + `],"meta":{"limit":1000,"has_more":false,"cursor":""}}`
			})
			pasangKonfigInaproc(t, p.URL, "token-uji")
			f := pasangDBPalsu(t)

			code, body := panggil(routerDatar(), "POST", j.jalur()+"/sync", j.bodySync())
			if code != 200 {
				t.Fatalf("status = %d, body = %v", code, body)
			}
			data := body["data"].(map[string]interface{})
			if data["total_synced"] != float64(3) || data["pages_fetched"] != float64(2) || data["total_failed"] != float64(0) {
				t.Errorf("hasil = %v", data)
			}
			// Halaman kedua meminta cursor dari halaman pertama, dan limit sinkronisasi selalu 1000.
			if len(p.diminta) != 2 || !strings.Contains(p.diminta[1], "cursor=c1") || !strings.Contains(p.diminta[0], "limit=1000") {
				t.Errorf("permintaan = %v", p.diminta)
			}
			for _, w := range append([]string{"/api/v1/" + j.ep.awalan() + "/" + j.nama + "?"}, j.paramHulu()...) {
				if !strings.Contains(p.diminta[0], w) {
					t.Errorf("permintaan sinkronisasi %q tidak memuat %q", p.diminta[0], w)
				}
			}

			var hapus, sisip, catat int
			for _, ex := range f.Execs() {
				switch {
				case strings.HasPrefix(ex.Query, "DELETE FROM "+j.ep.Tabel+" "):
					hapus++
					if ex.Query != "DELETE "+j.whereLokal() {
						t.Errorf("DELETE = %q, want %q", ex.Query, "DELETE "+j.whereLokal())
					}
					want := j.argHapus()
					if len(ex.Args) != len(want) {
						t.Fatalf("DELETE args = %+v, want %v", ex.Args, want)
					}
					for i, w := range want {
						if ex.Args[i].Value != w {
							t.Errorf("DELETE args = %+v, want %v", ex.Args, want)
						}
					}
				case strings.HasPrefix(ex.Query, "INSERT INTO "+j.ep.Tabel+" "):
					sisip++
					if len(ex.Args) != j.jumlahFld+2 {
						t.Errorf("INSERT memakai %d argumen, want %d", len(ex.Args), j.jumlahFld+2)
					}
				case strings.Contains(ex.Query, "INTO inaproc_sync_log"):
					catat++
					if ex.Args[0].Value != j.ep.namaLog() || ex.Args[5].Value != "success" || ex.Args[4].Value != int64(3) && ex.Args[4].Value != 3 {
						t.Errorf("sync_log args = %+v", ex.Args)
					}
					// Saringan kode: kodenya dicatat di kolom jenis_paket, klpd dan tahun kosong.
					if j.ep.Saring.Param != "" && (ex.Args[3].Value != kodeUji || ex.Args[1].Value != "" || ex.Args[2].Value != "") {
						t.Errorf("sync_log (saringan kode) args = %+v", ex.Args)
					}
				}
			}
			if hapus != 1 || sisip != 3 || catat != 1 {
				t.Errorf("DELETE=%d INSERT=%d sync_log=%d, want 1/3/1", hapus, sisip, catat)
			}
		})
	}
}

func TestEndpointDatarSyncMelaporkanBarisYangGagalDisimpan(t *testing.T) {
	p := newInaprocPalsu(t, func(r *http.Request) (int, string) {
		b := strings.Replace(contohNonTenderSelesai, `"nama_paket": "`, `"nama_paket": "Lain `, 1)
		return 200, `{"success":true,"data":[` + contohNonTenderSelesai + `,` + b + `],"meta":{"limit":1000,"has_more":false,"cursor":""}}`
	})
	pasangKonfigInaproc(t, p.URL, "token-uji")
	f := pasangDBPalsu(t)
	sisipan := 0
	f.OnExec = func(q string, args []driver.NamedValue) error {
		if strings.HasPrefix(q, "INSERT INTO inaproc_non_tender_selesai") {
			sisipan++
			if sisipan == 2 {
				return errors.New("Violation of PRIMARY KEY constraint")
			}
		}
		return nil
	}

	code, body := panggil(routerDatar(), "POST", "/inaproc/tender/non-tender-selesai/sync", `{"tahun":"2024"}`)
	data, _ := body["data"].(map[string]interface{})
	if code != 200 || data["total_synced"] != float64(1) || data["total_failed"] != float64(1) {
		t.Fatalf("baris yang gagal harus dihitung dan dilaporkan, bukan diam-diam: %d %v", code, body)
	}
	var pesan interface{}
	for _, ex := range f.Execs() {
		if strings.Contains(ex.Query, "INTO inaproc_sync_log") {
			pesan = ex.Args[6].Value
		}
	}
	if s, _ := pesan.(string); !strings.Contains(s, "1 baris gagal disimpan") {
		t.Errorf("catatan sync_log = %#v", pesan)
	}
}

// pencatatan-swakelola-realisasi mengirim nip_ppk sebagai angka JSON. NIP 18 digit melebihi 2^53, jadi bila dibaca lewat
// float64 digit belakangnya berubah diam-diam (198505152010011001 menjadi 198505152010011000).
func TestEndpointDatarSyncMenjagaDigitAngkaBesar(t *testing.T) {
	const nip = "198505152010011001"
	const rsk = "9007199254740993" // 2^53 + 1: tidak terwakili float64
	baris := contohPencatatanSwakelolaRealisasi
	for lama, baru := range map[string]string{
		`"nip_ppk": 12345`:         `"nip_ppk": ` + nip,
		`"rsk_id": 12345`:          `"rsk_id": ` + rsk,
		`"nilai_realisasi": 12345`: `"nilai_realisasi": 1234567890123.45`,
	} {
		if !strings.Contains(baris, lama) {
			t.Fatalf("contoh tidak memuat %s", lama)
		}
		baris = strings.Replace(baris, lama, baru, 1)
	}
	// Field yang belum dikenal juga tidak boleh berubah lewat extra_json.
	baris = strings.Replace(baris, `"rsk_id"`, `"id_besar": `+nip+`, "rsk_id"`, 1)

	p := newInaprocPalsu(t, func(r *http.Request) (int, string) {
		return 200, `{"success":true,"data":[` + baris + `],"meta":{"limit":1000,"has_more":false,"cursor":""}}`
	})
	pasangKonfigInaproc(t, p.URL, "token-uji")
	f := pasangDBPalsu(t)

	if code, body := panggil(routerDatar(), "POST", "/inaproc/tender/pencatatan-swakelola-realisasi/sync", `{"tahun":"2024"}`); code != 200 {
		t.Fatalf("%d %v", code, body)
	}
	cols := append([]string{"row_key"}, pencatatanSwakelolaRealisasi.KnownFields()...)
	cols = append(cols, "extra_json")
	for _, ex := range f.Execs() {
		if !strings.HasPrefix(ex.Query, "INSERT INTO inaproc_pencatatan_swakelola_realisasi ") {
			continue
		}
		nilai := map[string]interface{}{}
		for i, c := range cols {
			nilai[c] = ex.Args[i].Value
		}
		if nilai["nip_ppk"] != nip || nilai["rsk_id"] != rsk || nilai["nilai_realisasi"] != "1234567890123.45" {
			t.Errorf("nip_ppk = %v, rsk_id = %v, nilai_realisasi = %v", nilai["nip_ppk"], nilai["rsk_id"], nilai["nilai_realisasi"])
		}
		if s, _ := nilai["extra_json"].(string); !strings.Contains(s, `"id_besar":`+nip) {
			t.Errorf("extra_json = %q, id_besar harus utuh", nilai["extra_json"])
		}
		return
	}
	t.Error("INSERT tidak ditemukan")
}

func TestPembacaAngkaMenerimaJSONNumber(t *testing.T) {
	row := map[string]interface{}{"a": json.Number("198505152010011001"), "b": json.Number("1234.50"), "c": json.Number("42"), "d": json.Number("7.0")}
	if got := getStr(row, "a"); got != "198505152010011001" {
		t.Errorf("getStr = %q", got)
	}
	if got := getDecimalString(row, "b"); got != "1234.50" {
		t.Errorf("getDecimalString = %v", got)
	}
	if got := getInt64FromAny(row, "c"); got != int64(42) {
		t.Errorf("getInt64FromAny = %v", got)
	}
	if got := getInt64FromAny(row, "d"); got != int64(7) {
		t.Errorf("getInt64FromAny(7.0) = %v", got)
	}
}

func TestEndpointDatarSyncKdKlpdKosongDiisiKodeYangDiminta(t *testing.T) {
	// kd_klpd null pada respons non-tender-selesai: tanpa pengisian, hapus-sebelum-tarik dan daftar lokal tidak menemukan barisnya.
	p := newInaprocPalsu(t, func(r *http.Request) (int, string) {
		return 200, `{"success":true,"data":[` + contohNonTenderSelesai + `],"meta":{"limit":1000,"has_more":false,"cursor":""}}`
	})
	pasangKonfigInaproc(t, p.URL, "token-uji")
	f := pasangDBPalsu(t)
	if code, body := panggil(routerDatar(), "POST", "/inaproc/tender/non-tender-selesai/sync", `{"kode_klpd":"X99","tahun":"2024"}`); code != 200 {
		t.Fatalf("%d %v", code, body)
	}
	cols := append([]string{"row_key"}, nonTenderSelesai.KnownFields()...)
	for _, ex := range f.Execs() {
		if strings.HasPrefix(ex.Query, "INSERT INTO inaproc_non_tender_selesai") {
			for i, c := range cols {
				if c == "kd_klpd" && ex.Args[i].Value != "X99" {
					t.Errorf("kd_klpd = %#v, want X99", ex.Args[i].Value)
				}
			}
			return
		}
	}
	t.Error("INSERT tidak ditemukan")
}

func TestEndpointDatarSyncGalatHuluDicatatDanTidakMenyimpan(t *testing.T) {
	p := newInaprocPalsu(t, func(r *http.Request) (int, string) {
		return 401, `{"success":false,"error":{"code":"Unauthorized","message":"Unauthorized","details":"Invalid or missing API key"},"meta":{"request_id":"r"}}`
	})
	pasangKonfigInaproc(t, p.URL, "token-salah")
	f := pasangDBPalsu(t)
	r := routerDatar()

	code, body := panggil(r, "POST", "/inaproc/tender/pencatatan-non-tender/sync", `{"tahun":"2024"}`)
	if code != 401 || !strings.Contains(body["message"].(string), "Invalid or missing API key") {
		t.Errorf("galat harus diteruskan dengan pesannya: %d %v", code, body)
	}
	var gagal bool
	for _, ex := range f.Execs() {
		if strings.HasPrefix(ex.Query, "INSERT INTO inaproc_pencatatan_non_tender ") {
			t.Error("tidak boleh menyimpan apa pun saat Inaproc menolak")
		}
		if strings.Contains(ex.Query, "INTO inaproc_sync_log") && ex.Args[5].Value == "failed" {
			gagal = true
		}
	}
	if !gagal {
		t.Error("kegagalan harus dicatat di inaproc_sync_log")
	}
	// Tanpa tahun: ditolak sebelum menyentuh database atau Inaproc.
	g2 := pasangDBPalsu(t)
	before := len(p.diminta)
	if code, _ := panggil(r, "POST", "/inaproc/tender/pencatatan-non-tender-realisasi/sync", `{}`); code != 400 {
		t.Errorf("tanpa tahun: %d, want 400", code)
	}
	if len(p.diminta) != before || len(g2.Execs()) != 0 {
		t.Errorf("tanpa tahun tidak boleh menyentuh Inaproc/database: %d, %v", len(p.diminta)-before, g2.Execs())
	}
}

func TestEndpointDatarListLocal(t *testing.T) {
	for _, j := range semuaJenisUji() {
		t.Run(j.nama, func(t *testing.T) {
			f := pasangDBPalsu(t)
			var query string
			var args []driver.NamedValue
			f.OnQuery = func(ctx context.Context, q string, a []driver.NamedValue) ([]string, [][]driver.Value, error) {
				query, args = q, a
				return []string{"row_key", "nama_paket"}, [][]driver.Value{{"k1", "Contoh Nama Paket"}}, nil
			}
			code, body := panggil(routerDatar(), "GET", j.jalur()+"/local"+j.query("limit=10"), "")
			if code != 200 {
				t.Fatalf("status = %d, body = %v", code, body)
			}
			if body["data"].(map[string]interface{})["count"] != float64(1) {
				t.Errorf("data = %v", body["data"])
			}
			if !strings.Contains(query, j.whereLokal()) || !strings.Contains(query, "SELECT TOP (10)") {
				t.Errorf("query = %s", query)
			}
			want := j.argHapus() // sama dengan argumen penyaring daftar lokal
			if len(args) != len(want) {
				t.Fatalf("args = %+v, want %v", args, want)
			}
			for i, w := range want {
				if args[i].Value != w {
					t.Errorf("args = %+v, want %v", args, want)
				}
			}
			// Nilai penyaring dikirim sebagai argumen, tidak pernah masuk ke teks SQL.
			if strings.Contains(query, "2024") || strings.Contains(query, kodeUji) {
				t.Error("nilai penyaring tidak boleh masuk ke teks SQL")
			}
		})
	}
}
