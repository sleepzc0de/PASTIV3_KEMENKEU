package apiakses

import (
	"fmt"
	"strings"
	"sync"

	"pasti-v3-backend/digitalisasi"
)

// Katalog dataset PENGADAAN (kode "pengadaan.<kunci>"): data Inaproc yang disimpan PASTI (RUP, tender, non-tender, pencatatan, dan E-Katalog). Sama seperti dataset aset,
// tidak ada pembacaan generik dari tabel mana pun: setiap dataset ditulis di sini dengan daftar kolom yang BOLEH keluar (daftar putih, bukan daftar hitam), jadi kolom baru pada
// tabelnya otomatis tertutup sampai dinilai dan ditambahkan. Tes (katalog_pengadaan_test.go) membandingkan daftar ini dengan migrasi: setiap kolom tabel harus tercatat
// sebagai keluar, tertutup, atau teknis, sehingga tidak ada kolom yang lolos tanpa dinilai.
//
// Kebijakan kolom (penilaian satu per satu, rinciannya di docs/api-akses-data.md):
//   - KELUAR: kode dan nama paket, satker, KLPD, LPSE; metode, jenis, kualifikasi, status; pagu, HPS, nilai, volume; tanggal; nomor kontrak; kode dan nama penyedia.
//   - TERTUTUP, data orang: PPK, Pokja, dan petugas lain (nama, NIP, jabatan, SK, username, kode pengguna, email, telepon), wakil sah penyedia dan anggota KSO, pelaksana swakelola.
//   - TERTUTUP, identitas pajak dan keuangan: NPWP (penyedia, pelaksana, satker), nama dan nomor rekening bank.
//   - TERTUTUP, isi bebas yang tidak dapat diperkirakan: alasan dan keterangan yang diketik petugas, tautan dokumen, riwayat BAP/BAST dan penilaian kinerja (JSON), dan sisa field API (extra_json).
//   - TEKNIS: kunci hash baris (menjadi id), synced_at, dan updated_at (yang terakhir diberikan sebagai disinkronkan_pada: waktu terakhir penarikan menyentuh baris itu).
//
// Pembatasan UE1 klien memakai aturan yang sama dengan pembatasan per satker pada halaman Pengadaan Terpadu (handlers/inaproc_cakupan.go), dipasang pemanggil lewat BatasSatker.
// Dataset yang tidak dapat dibatasi per satker (program master, e-kontrak non-tender, transaksi per produk V6, dan rujukan katalog) tidak ada di katalog ini.

const awalanPengadaan = "pengadaan."

// Kelompok dataset pada katalog gabungan.
const (
	KelompokAset      = "aset"
	KelompokPengadaan = "pengadaan"
)

// Subkelompok dataset pengadaan (untuk pengelompokan pada pilihan scope).
const (
	subRUP        = "Perencanaan (RUP)"
	subTender     = "Tender"
	subNonTender  = "Non-tender"
	subPencatatan = "Pencatatan"
	subEkatalog   = "E-Katalog"
)

// kolomTeknisPengadaan: kolom tabel yang tidak pernah dipetakan ke API (kunci hash baris menjadi id; synced_at dan updated_at tidak diberikan terpisah).
var kolomTeknisPengadaan = map[string]bool{"row_key": true, "extra_json": true, "synced_at": true, "updated_at": true}

// definisiPengadaan: definisi satu dataset dalam bentuk ringkas. keluar berisi kolom yang keluar, tiap kolom dapat diberi akhiran tipe: ":d" desimal, ":i" bilangan,
// ":t" waktu, ":b" logika (tanpa akhiran = teks). tutup berisi kolom yang SENGAJA tidak keluar (diperlukan tes untuk memastikan setiap kolom tabel sudah dinilai).
type definisiPengadaan struct {
	kunci, sub, label, deskripsi, tabel string
	kunciBaris                          string // kolom unik untuk kursor dan id
	klpd, tahun                         string // kolom penyaring kode KLPD dan tahun; kosong = tidak ada
	cari                                string // kolom pencarian (q), harus kolom yang keluar
	keluar, tutup                       string
}

// DatasetPengadaan: satu dataset pengadaan yang dapat ditarik.
type DatasetPengadaan struct {
	Kode        string
	Label       string
	Deskripsi   string
	Subkelompok string
	Kolom       []KolomAPI
	Tertutup    []string // kolom tabel yang sengaja tidak keluar (untuk dokumentasi dan tes)

	tabel      string
	kunciBaris string
	kolomKLPD  string
	kolomTahun string
	kolomCari  []string
}

// Tabel: nama tabel sumber (dari kode, bukan masukan).
func (d DatasetPengadaan) Tabel() string { return d.tabel }

// Kunci: kunci dataset tanpa awalan ("tender_pengumuman"), dipakai pada alamat.
func (d DatasetPengadaan) Kunci() string { return strings.TrimPrefix(d.Kode, awalanPengadaan) }

// KolomKunci: kolom unik tabel yang menjadi id baris dan kursor.
func (d DatasetPengadaan) KolomKunci() string { return d.kunciBaris }

// KolomKLPD dan KolomTahun: kolom penyaring kode KLPD dan tahun (kosong = dataset tidak punya penyaring itu).
func (d DatasetPengadaan) KolomKLPD() string  { return d.kolomKLPD }
func (d DatasetPengadaan) KolomTahun() string { return d.kolomTahun }

// PunyaTahun dan PunyaKLPD: apakah dataset punya kolom penyaring itu.
func (d DatasetPengadaan) PunyaTahun() bool { return d.kolomTahun != "" }
func (d DatasetPengadaan) PunyaKLPD() bool  { return d.kolomKLPD != "" }

// KolomPencarian: kolom teks untuk parameter q (semuanya kolom yang keluar).
func (d DatasetPengadaan) KolomPencarian() []string { return d.kolomCari }

// susunPengadaan menguraikan definisi; panic bila definisi tidak konsisten (kesalahan pemrograman yang harus gagal saat mulai, bukan saat ada permintaan).
func susunPengadaan(def definisiPengadaan) DatasetPengadaan {
	d := DatasetPengadaan{
		Kode: awalanPengadaan + def.kunci, Label: def.label, Deskripsi: def.deskripsi, Subkelompok: def.sub,
		Tertutup: strings.Fields(def.tutup), tabel: def.tabel, kunciBaris: def.kunciBaris, kolomKLPD: def.klpd, kolomTahun: def.tahun,
	}
	ada := map[string]bool{}
	for _, f := range strings.Fields(def.keluar) {
		nama, tipe, jenis := f, "teks", digitalisasi.Text
		if i := strings.LastIndexByte(f, ':'); i >= 0 {
			nama = f[:i]
			switch f[i+1:] {
			case "d":
				tipe, jenis = "desimal", digitalisasi.Decimal
			case "i":
				tipe, jenis = "bilangan", digitalisasi.Int
			case "t":
				tipe = "waktu"
			case "b":
				tipe = "logika"
			default:
				panic(fmt.Sprintf("katalog pengadaan %s: akhiran tipe tidak dikenal pada kolom %q", def.kunci, f))
			}
		}
		if ada[nama] || kolomTeknisPengadaan[nama] {
			panic(fmt.Sprintf("katalog pengadaan %s: kolom %q ganda atau teknis", def.kunci, nama))
		}
		ada[nama] = true
		d.Kolom = append(d.Kolom, KolomAPI{Nama: nama, Tipe: tipe, kolom: nama, jenis: jenis})
	}
	for _, c := range d.Tertutup {
		if ada[c] {
			panic(fmt.Sprintf("katalog pengadaan %s: kolom %q ada di daftar keluar dan tertutup sekaligus", def.kunci, c))
		}
	}
	for _, c := range strings.Fields(def.cari) {
		if !ada[c] {
			panic(fmt.Sprintf("katalog pengadaan %s: kolom pencarian %q tidak termasuk kolom yang keluar", def.kunci, c))
		}
		d.kolomCari = append(d.kolomCari, c)
	}
	for _, c := range []string{def.klpd, def.tahun} {
		if c != "" && !ada[c] {
			panic(fmt.Sprintf("katalog pengadaan %s: kolom penyaring %q tidak termasuk kolom yang keluar", def.kunci, c))
		}
	}
	if d.kunciBaris == "" || len(d.Kolom) == 0 || def.tabel == "" {
		panic("katalog pengadaan " + def.kunci + ": kunci baris, tabel, atau kolom kosong")
	}
	return d
}

var katalogPengadaan = sync.OnceValue(func() []DatasetPengadaan {
	out := make([]DatasetPengadaan, 0, len(definisiDataPengadaan))
	lihat := map[string]bool{}
	for _, def := range definisiDataPengadaan {
		d := susunPengadaan(def)
		if lihat[d.Kode] {
			panic("katalog pengadaan: kode ganda " + d.Kode)
		}
		lihat[d.Kode] = true
		out = append(out, d)
	}
	return out
})

// KatalogPengadaan: semua dataset pengadaan, urut seperti definisi.
func KatalogPengadaan() []DatasetPengadaan { return katalogPengadaan() }

// CariDatasetPengadaan mencari dataset pengadaan menurut kode lengkap ("pengadaan.tender_pengumuman"). Hanya kode yang ada di katalog.
func CariDatasetPengadaan(kode string) (DatasetPengadaan, bool) {
	if !strings.HasPrefix(kode, awalanPengadaan) {
		return DatasetPengadaan{}, false
	}
	for _, d := range katalogPengadaan() {
		if d.Kode == kode {
			return d, true
		}
	}
	return DatasetPengadaan{}, false
}

// InfoDataset: ringkasan satu dataset pada katalog gabungan (aset dan pengadaan), untuk pilihan scope, katalog klien, dan validasi.
type InfoDataset struct {
	Kode        string `json:"kode"`
	Label       string `json:"label"`
	Deskripsi   string `json:"deskripsi"`
	Kelompok    string `json:"kelompok"`
	Subkelompok string `json:"subkelompok,omitempty"`
}

// SemuaDataset: dataset aset lalu dataset pengadaan, urut seperti katalog masing-masing.
func SemuaDataset() []InfoDataset {
	out := make([]InfoDataset, 0, len(digitalisasi.Datasets)+len(definisiDataPengadaan))
	for _, d := range Katalog() {
		out = append(out, InfoDataset{Kode: d.Kode, Label: d.Label, Deskripsi: d.Deskripsi, Kelompok: KelompokAset})
	}
	for _, d := range katalogPengadaan() {
		out = append(out, InfoDataset{Kode: d.Kode, Label: d.Label, Deskripsi: d.Deskripsi, Kelompok: KelompokPengadaan, Subkelompok: d.Subkelompok})
	}
	return out
}

// DatasetDikenal: apakah kode ada di katalog gabungan (aset atau pengadaan).
func DatasetDikenal(kode string) bool {
	if _, ok := CariDataset(kode); ok {
		return true
	}
	_, ok := CariDatasetPengadaan(kode)
	return ok
}

// definisiDataPengadaan: daftar dataset pengadaan. Kolom dinilai dari migrasi tabel Inaproc (backend/migrations); jangan menambah kolom ke "keluar" tanpa menilai apakah
// memuat data orang, identitas pajak/keuangan, atau isi bebas.
var definisiDataPengadaan = []definisiPengadaan{
	{
		kunci: "rup_paket_penyedia", sub: subRUP, label: "Paket Penyedia (RUP)", deskripsi: "Rencana paket pengadaan melalui penyedia: pagu, metode, jenis, jadwal pemilihan, dan status pengumuman.", tabel: "inaproc_paket_penyedia",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "nama_paket kd_rup nama_satker",
		keluar: `tahun_anggaran kd_klpd nama_klpd jenis_klpd kd_satker kd_satker_str nama_satker kd_rup nama_paket pagu:i kd_metode_pengadaan metode_pengadaan
			kd_jenis_pengadaan jenis_pengadaan status_pradipa status_pdn status_ukm status_konsolidasi tipe_paket kd_rup_swakelola kd_rup_lokal volume_pekerjaan
			uraian_pekerjaan spesifikasi_pekerjaan tgl_awal_pemilihan:t tgl_akhir_pemilihan:t tgl_awal_kontrak:t tgl_akhir_kontrak:t tgl_awal_pemanfaatan:t
			tgl_akhir_pemanfaatan:t tgl_buat_paket:t tgl_pengumuman_paket:t status_aktif_rup:b status_delete_rup:b status_umumkan_rup status_dikecualikan:b
			tahun_pertama kode_rup_tahun_pertama nomor_kontrak spp_aspek_ekonomi:b spp_aspek_sosial:b spp_aspek_lingkungan:b detail_lokasi event_date:t
			inserted_date_src:t`,
		tutup: `alasan_non_ukm nip_ppk nama_ppk username_ppk alasan_dikecualikan`,
	},
	{
		kunci: "rup_paket_penyedia_terumumkan", sub: subRUP, label: "Paket Penyedia Terumumkan (RUP)", deskripsi: "Paket penyedia yang sudah diumumkan di SiRUP.", tabel: "inaproc_paket_penyedia_terumumkan",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "nama_paket kd_rup nama_satker",
		keluar: `jenis_klpd jenis_pengadaan kd_jenis_pengadaan kd_klpd kd_metode_pengadaan kd_rup kd_rup_lokal kd_rup_swakelola kd_satker kd_satker_str
			kode_rup_tahun_pertama metode_pengadaan nama_klpd nama_paket nama_satker nomor_kontrak pagu:i spesifikasi_pekerjaan spp_aspek_ekonomi:b
			spp_aspek_lingkungan:b spp_aspek_sosial:b status_aktif_rup:b status_delete_rup:b status_dikecualikan:b status_konsolidasi status_pdn status_pradipa
			status_ukm status_umumkan_rup tahun_anggaran tahun_pertama tgl_akhir_kontrak:t tgl_akhir_pemanfaatan:t tgl_akhir_pemilihan:t tgl_awal_kontrak:t
			tgl_awal_pemanfaatan:t tgl_awal_pemilihan:t tgl_buat_paket:t tgl_pengumuman_paket:t tipe_paket uraian_pekerjaan volume_pekerjaan`,
		tutup: `alasan_dikecualikan alasan_non_ukm nama_ppk nip_ppk username_ppk`,
	},
	{
		kunci: "rup_anggaran_penyedia", sub: subRUP, label: "Anggaran Paket Penyedia (RUP)", deskripsi: "Rincian sumber dana dan anggaran (MAK) per paket penyedia.", tabel: "inaproc_paket_anggaran_penyedia",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "kd_rup nama_satker mak",
		keluar: `asal_dana asal_dana_klpd asal_dana_satker jenis_klpd kd_kegiatan kd_klpd kd_komponen kd_rup kd_rup_lokal kd_satker kd_satker_str kd_subkegiatan mak
			nama_klpd nama_satker pagu:i status_aktif_rup:b status_delete_rup:b status_umumkan_rup sumber_dana tahun_anggaran tahun_anggaran_dana`,
		tutup: ``,
	},
	{
		kunci: "rup_paket_swakelola", sub: subRUP, label: "Paket Swakelola (RUP)", deskripsi: "Rencana paket yang dikerjakan secara swakelola.", tabel: "inaproc_paket_swakelola",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "nama_paket kd_rup nama_satker",
		keluar: `kd_klpd kd_satker kd_rup nama_klpd nama_satker nama_paket tahun_anggaran status`,
		tutup:  ``,
	},
	{
		kunci: "rup_paket_swakelola_terumumkan", sub: subRUP, label: "Paket Swakelola Terumumkan (RUP)", deskripsi: "Paket swakelola yang sudah diumumkan di SiRUP.", tabel: "inaproc_paket_swakelola_terumumkan",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "nama_paket kd_rup nama_satker",
		keluar: `jenis_klpd kd_klpd kd_klpd_penyelenggara kd_rup kd_rup_lokal kd_satker kd_satker_str nama_klpd nama_klpd_penyelenggara nama_paket nama_satker
			nama_satker_penyelenggara pagu:i status_aktif_rup:b status_delete_rup:b status_umumkan_rup tahun_anggaran tgl_akhir_pelaksanaan_kontrak:t
			tgl_awal_pelaksanaan_kontrak:t tgl_buat_paket:t tgl_pengumuman_paket:t tipe_swakelola uraian_pekerjaan volume_pekerjaan`,
		tutup: `nama_ppk nip_ppk username_ppk`,
	},
	{
		kunci: "rup_anggaran_swakelola", sub: subRUP, label: "Anggaran Paket Swakelola (RUP)", deskripsi: "Rincian sumber dana dan anggaran (MAK) per paket swakelola.", tabel: "inaproc_paket_anggaran_swakelola",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "kd_rup nama_satker mak",
		keluar: `asal_dana asal_dana_klpd asal_dana_satker jenis_klpd kd_kegiatan kd_klpd kd_komponen kd_rup kd_rup_lokal kd_satker kd_satker_str kd_subkegiatan mak
			nama_klpd nama_satker pagu:i status_aktif_rup:b status_delete_rup:b status_umumkan_rup sumber_dana tahun_anggaran tahun_anggaran_dana`,
		tutup: ``,
	},
	{
		kunci: "rup_riwayat_kaji_ulang", sub: subRUP, label: "Riwayat Kaji Ulang RUP", deskripsi: "Perubahan (kaji ulang) paket RUP: jenis revisi dan tanggal. Alasan kaji ulang tidak diberikan.", tabel: "inaproc_history_kaji_ulang",
		kunciBaris: "datamart_id", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "kd_rup_lama kd_rup_baru nama_satker",
		keluar: `tahun_anggaran kd_klpd nama_klpd jenis_klpd kd_satker kd_satker_str nama_satker kd_rup_lama kd_rup_baru jenis_paket jenis_revisi
			tgl_kaji_ulang:t event_date:t inserted_date_src:t`,
		tutup: `alasan_kajiulang`,
	},
	{
		kunci: "tender_pengumuman", sub: subTender, label: "Pengumuman Tender", deskripsi: "Pengumuman tender: pagu, HPS, metode, kualifikasi, dan status.", tabel: "inaproc_tender_pengumuman",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "nama_paket kd_tender kd_rup nama_satker",
		keluar: `kd_klpd jenis_klpd nama_klpd kd_satker kd_satker_str nama_satker kd_lpse nama_lpse url_lpse kd_pkt_dce kd_rup kd_tender tahun_anggaran
			list_tahun_anggaran nama_paket jenis_pengadaan kualifikasi_paket kontrak_pembayaran lokasi_pekerjaan sumber_dana versi_tender:i mtd_pemilihan
			mtd_kualifikasi mtd_evaluasi status_tender pagu:d hps:d tanggal_status:t tgl_buat_paket:t tgl_kolektif_kolegial:t tgl_pengumuman_tender:t`,
		tutup: `nama_ppk nip_ppk nama_pokja nip_pokja ket_ditutup ket_diulang`,
	},
	{
		kunci: "tender_peserta", sub: subTender, label: "Peserta Tender", deskripsi: "Peserta tender beserta penawaran dan status pemenang.", tabel: "inaproc_peserta_tender",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "kd_tender nama_penyedia",
		keluar: `kd_klpd kd_satker kd_satker_str kd_lpse kd_pkt_dce kd_tender kd_peserta tahun_anggaran kd_penyedia nama_penyedia pemenang:i pemenang_terverifikasi:i
			nilai_penawaran:d nilai_terkoreksi:d`,
		tutup: `npwp_penyedia npwp_penyedia_16 alasan`,
	},
	{
		kunci: "tender_jadwal_tahapan", sub: subTender, label: "Jadwal Tahapan Tender", deskripsi: "Jadwal tiap tahapan tender.", tabel: "inaproc_jadwal_tahapan_tender",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "kd_tender nama_tahapan",
		keluar: `kd_akt kd_klpd kd_lpse kd_satker kd_satker_str kd_tahapan kd_tender nama_akt nama_tahapan tahun_anggaran tgl_akhir:t tgl_awal:t`,
		tutup:  ``,
	},
	{
		kunci: "tender_kontrak", sub: subTender, label: "Kontrak Tender", deskripsi: "Kontrak hasil tender: nilai, penyedia, status, addendum, dan masa kontrak.", tabel: "inaproc_tender_ekontrak_kontrak",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "nama_paket kd_tender no_kontrak nama_penyedia",
		keluar: `kd_klpd jenis_klpd nama_klpd kd_lpse kd_satker kd_satker_str nama_satker alamat_satker kd_tender tahun_anggaran nama_paket lingkup_pekerjaan
			no_kontrak no_sppbj jenis_kontrak status_kontrak kota_kontrak tgl_kontrak:t tgl_kontrak_awal:t tgl_kontrak_akhir:t tgl_penetapan_status_kontrak:t
			apakah_addendum versi_addendum:i nilai_kontrak:d nilai_pdn_kontrak:d nilai_umk_kontrak:d kd_penyedia nama_penyedia bentuk_usaha_penyedia tipe_penyedia`,
		tutup: `informasi_lainnya alasan_penetapan_status_kontrak alasan_addendum alasan_ubah_nilai_kontrak alasan_nilai_kontrak_10_persen nama_ppk nip_ppk
			jabatan_ppk no_sk_ppk npwp_penyedia npwp_16_penyedia wakil_sah_penyedia jabatan_wakil_penyedia anggota_kso nama_rek_bank no_rek_bank
			nama_pemilik_rek_bank`,
	},
	{
		kunci: "tender_selesai", sub: subTender, label: "Tender Selesai", deskripsi: "Tender yang sudah selesai: pagu, HPS, dan tanggal penetapan pemenang.", tabel: "inaproc_tender_selesai",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "nama_paket kd_tender kd_rup nama_satker",
		keluar: `kd_klpd jenis_klpd nama_klpd kd_satker kd_satker_str nama_satker kd_lpse nama_lpse url_lpse kd_rup kd_tender tahun_anggaran nama_paket jenis_pengadaan
			kualifikasi_paket kontrak_pembayaran sumber_dana mak mtd_pemilihan mtd_kualifikasi status_tender last_update_ref pagu:d hps:d tgl_pengumuman_tender:t
			tgl_penetapan_pemenang:t`,
		tutup: ``,
	},
	{
		kunci: "tender_selesai_nilai", sub: subTender, label: "Nilai Tender Selesai", deskripsi: "Nilai penawaran, koreksi, negosiasi, dan kontrak tender selesai beserta penyedia pemenang.", tabel: "inaproc_tender_selesai_nilai",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "kd_tender kd_rup_paket nama_penyedia",
		keluar: `kd_klpd jenis_klpd nama_klpd kd_satker nama_satker kd_lpse kd_tender kd_paket kd_rup_paket psr_id tahun_anggaran kd_penyedia nama_penyedia pagu:d
			hps:d nilai_penawaran:d nilai_terkoreksi:d nilai_negosiasi:d nilai_kontrak:d nilai_pdn_kontrak:d nilai_umk_kontrak:d tgl_pengumuman_tender:t
			tgl_penetapan_pemenang:t`,
		tutup: `npwp_penyedia npwp_16_penyedia`,
	},
	{
		kunci: "nontender_pengumuman", sub: subNonTender, label: "Pengumuman Non-Tender", deskripsi: "Pengumuman pengadaan non-tender: pagu, HPS, metode, dan status.", tabel: "inaproc_non_tender_pengumuman",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "nama_paket kd_nontender kd_rup nama_satker",
		keluar: `kd_klpd jenis_klpd nama_klpd kd_satker kd_satker_str nama_satker kd_lpse nama_lpse url_lpse kd_nontender kd_pkt_dce lls_id kd_rup tahun_anggaran
			nama_paket jenis_pengadaan kualifikasi_paket kontrak_pembayaran mtd_pemilihan sumber_dana mak repeat_order versi_nontender:i pagu:d hps:d
			status_nontender tgl_buat_paket:t tgl_kolektif_kolegial:t tgl_pengumuman_nontender:t`,
		tutup: `ket_ditutup ket_diulang nip_nama_ppk nip_nama_pp nip_nama_pokja`,
	},
	{
		kunci: "nontender_selesai", sub: subNonTender, label: "Non-Tender Selesai", deskripsi: "Pengadaan non-tender yang sudah selesai: penyedia dan nilai.", tabel: "inaproc_non_tender_selesai",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "nama_paket kd_nontender nama_penyedia nama_satker",
		keluar: `kd_klpd jenis_klpd nama_klpd kd_satker kd_satker_str nama_satker kd_lpse lpse_id nama_lpse url_lpse kd_nontender kd_pkt_dce kd_rup tahun_anggaran
			nama_paket jenis_pengadaan kualifikasi_paket kontrak_pembayaran mtd_pemilihan sumber_dana mak kd_penyedia nama_penyedia status_nontender pagu:d hps:d
			nilai_penawaran:d nilai_negosiasi:d nilai_terkoreksi:d nilai_kontrak:d nilai_pdn_kontrak:d nilai_umk_kontrak:d tgl_pengumuman_nontender:t
			tgl_selesai_nontender:t tgl_penarikan:t`,
		tutup: `npwp_penyedia npwp16_penyedia`,
	},
	{
		kunci: "nontender_kontrak", sub: subNonTender, label: "Kontrak Non-Tender", deskripsi: "Kontrak pengadaan non-tender: nilai, penyedia, status, addendum, dan masa kontrak.", tabel: "inaproc_non_tender_ekontrak_kontrak",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "nama_paket kd_nontender no_kontrak nama_penyedia",
		keluar: `kd_klpd jenis_klpd nama_klpd kd_lpse kd_satker kd_satker_str nama_satker alamat_satker kd_nontender tahun_anggaran nama_paket mtd_pengadaan
			lingkup_pekerjaan no_kontrak no_sppbj jenis_kontrak status_kontrak kota_kontrak tgl_kontrak:t tgl_kontrak_awal:t tgl_kontrak_akhir:t
			tgl_penetapan_status_kontrak:t apakah_addendum versi_addendum:i nilai_kontrak:d nilai_pdn_kontrak:d nilai_umk_kontrak:d nama_penyedia
			bentuk_usaha_penyedia tipe_penyedia`,
		tutup: `informasi_lainnya alasan_penetapan_status_kontrak alasan_addendum alasan_ubah_nilai_kontrak alasan_nilai_kontrak_10_persen nama_ppk nip_ppk
			jabatan_ppk no_sk_ppk npwp_penyedia npwp16_penyedia wakil_sah_penyedia jabatan_wakil_penyedia anggota_kso nama_rek_bank no_rek_bank
			nama_pemilik_rek_bank`,
	},
	{
		kunci: "nontender_jadwal_tahapan", sub: subNonTender, label: "Jadwal Tahapan Non-Tender", deskripsi: "Jadwal tiap tahapan pengadaan non-tender.", tabel: "inaproc_jadwal_tahapan_non_tender",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "kd_nontender nama_tahapan",
		keluar: `kd_akt kd_klpd kd_lpse kd_nontender kd_satker kd_satker_str kd_tahapan nama_akt nama_tahapan tahun_anggaran tgl_akhir:t tgl_awal:t`,
		tutup:  ``,
	},
	{
		kunci: "pencatatan_nontender", sub: subPencatatan, label: "Pencatatan Non-Tender", deskripsi: "Pencatatan pengadaan non-tender di luar e-tendering.", tabel: "inaproc_pencatatan_non_tender",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "nama_paket kd_nontender_pct kd_rup nama_satker",
		keluar: `kd_klpd jenis_klpd nama_klpd kd_satker kd_satker_str nama_satker kd_lpse kd_nontender_pct kd_pkt_dce kd_rup tahun_anggaran nama_paket
			kategori_pengadaan mtd_pemilihan sumber_dana uraian_pekerjaan status_nontender_pct status_nontender_pct_ket pagu:d nilai_pdn_pct:d nilai_umk_pct:d
			total_realisasi:d tgl_buat_paket:t tgl_mulai_paket:t tgl_selesai_paket:t`,
		tutup: `nama_ppk nip_ppk alasan_pembatalan bukti_pembayaran informasi_lainnya`,
	},
	{
		kunci: "pencatatan_nontender_realisasi", sub: subPencatatan, label: "Realisasi Pencatatan Non-Tender", deskripsi: "Realisasi pencatatan non-tender: penyedia, nilai, dan tanggal.", tabel: "inaproc_pencatatan_non_tender_realisasi",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "nama_paket nama_penyedia kd_nontender_pct",
		keluar: `kd_klpd jenis_klpd nama_klpd kd_satker kd_satker_str nama_satker kd_lpse nama_lpse kd_nontender_pct kd_paket_dce kd_rup_paket tahun_anggaran
			nama_paket nama_penyedia no_realisasi jenis_realisasi pagu:d nilai_realisasi:d tgl_realisasi:t`,
		tutup: `nama_ppk nip_ppk npwp_penyedia ket_realisasi dok_realisasi`,
	},
	{
		kunci: "pencatatan_swakelola", sub: subPencatatan, label: "Pencatatan Swakelola", deskripsi: "Pencatatan paket swakelola.", tabel: "inaproc_pencatatan_swakelola",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "nama_paket kd_swakelola_pct kd_rup nama_satker",
		keluar: `kd_klpd jenis_klpd nama_klpd kd_satker kd_satker_str nama_satker kd_lpse kd_swakelola_pct kd_pkt_dce kd_rup tahun_anggaran nama_paket sumber_dana
			uraian_pekerjaan tipe_swakelola tipe_swakelola_nama status_swakelola_pct status_swakelola_pct_ket pagu:d nilai_pdn_pct:d nilai_umk_pct:d
			total_realisasi:d tgl_buat_paket:t tgl_mulai_paket:t tgl_selesai_paket:t`,
		tutup: `nama_ppk nip_ppk alasan_pembatalan informasi_lainnya`,
	},
	{
		kunci: "pencatatan_swakelola_realisasi", sub: subPencatatan, label: "Realisasi Pencatatan Swakelola", deskripsi: "Realisasi pencatatan swakelola: nilai dan tanggal.", tabel: "inaproc_pencatatan_swakelola_realisasi",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "kd_swakelola_pct no_realisasi",
		keluar: `kd_klpd kd_satker kd_lpse kd_swakelola_pct rsk_id tahun_anggaran no_realisasi jenis_realisasi nilai_realisasi:d tgl_realisasi:t`,
		tutup:  `nama_ppk nip_ppk nama_pelaksana npwp_pelaksana ket_realisasi dok_realisasi`,
	},
	{
		kunci: "ekatalog_v5_paket", sub: subEkatalog, label: "Paket E-Purchasing V5", deskripsi: "Paket e-purchasing arsip E-Katalog V5: komoditas, penyedia, kuantitas, dan nilai.", tabel: "inaproc_ekatalog_paket_epurchasing",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "tahun_anggaran", cari: "nama_paket no_paket kd_rup nama_satker",
		keluar: `kd_klpd kd_rup tahun_anggaran kd_paket no_paket nama_paket deskripsi status_paket paket_status_str kode_anggaran nama_sumber_dana kd_komoditas
			kd_produk kd_paket_produk kd_penyedia kd_penyedia_distributor satker_id nama_satker alamat_satker kd_provinsi_wilayah_harga kd_kabupaten_wilayah_harga
			harga_satuan:d kuantitas:d ongkos_kirim:d total_harga:d jml_jenis_produk:i tanggal_buat_paket:t tanggal_edit_paket:t`,
		tutup: `catatan_produk npwp_satker kd_user_ppk ppk_nip jabatan_ppk kd_user_pokja email_user_pokja no_telp_user_pokja`,
	},
	{
		kunci: "ekatalog_v6_paket", sub: subEkatalog, label: "Paket E-Purchasing V6", deskripsi: "Paket e-purchasing E-Katalog V6, termasuk paket swasta.", tabel: "inaproc_ekatalog6_paket_epurchasing",
		kunciBaris: "row_key", klpd: "kode_klpd", tahun: "fiscal_year", cari: "rup_name rup_code order_id nama_satker",
		keluar: `kode_klpd fiscal_year order_id product_id kode_penyedia rekan_id status shipment_status is_swasta:i kode_satker nama_satker rup_code rup_name rup_desc
			mak funding_source count_product:i total_qty:d shipping_fee:d total:d order_date:t last_update_date:t`,
		tutup: ``,
	},
	{
		kunci: "ekatalog_instansi_satker", sub: subEkatalog, label: "Instansi dan Satker E-Katalog", deskripsi: "Daftar instansi dan satuan kerja pengguna E-Katalog.", tabel: "inaproc_ekatalog_instansi_satker",
		kunciBaris: "row_key", klpd: "kd_klpd", tahun: "", cari: "nama_satker nama_klpd kd_satker",
		keluar: `kd_klpd jenis_klpd nama_klpd kd_satker kd_satker_str nama_satker`,
		tutup:  ``,
	},
}
