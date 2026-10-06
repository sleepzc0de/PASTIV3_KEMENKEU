package handlers

// ============================================================
// TENDER Endpoint 7: Pencatatan Non Tender
// ============================================================
//
// Respons datar; logikanya ada di endpointDatar (inaproc_tender_datar.go). Daftar field di bawah HARUS sinkron dengan
// migrasi 025. Catatan nama: "pct" pada nilai_pdn_pct/nilai_umk_pct/kd_nontender_pct/status_nontender_pct berarti pencatatan
// (bukan persen); nilai_pdn_pct dan nilai_umk_pct adalah nilai rupiah.
var pencatatanNonTender = newEndpointDatar(endpointDatar{
	Nama:  "pencatatan-non-tender",
	Tabel: "inaproc_pencatatan_non_tender",
	Teks: []string{
		"kd_klpd", "jenis_klpd", "nama_klpd", "kd_satker", "kd_satker_str", "nama_satker",
		"kd_lpse", "kd_nontender_pct", "kd_pkt_dce", "kd_rup", "tahun_anggaran", "nama_paket",
		"kategori_pengadaan", "mtd_pemilihan", "sumber_dana", "uraian_pekerjaan",
		"nama_ppk", "nip_ppk",
		"status_nontender_pct", "status_nontender_pct_ket", "alasan_pembatalan",
		"bukti_pembayaran", "informasi_lainnya",
	},
	Desimal: []string{"pagu", "nilai_pdn_pct", "nilai_umk_pct", "total_realisasi"},
	Tanggal: []string{"tgl_buat_paket", "tgl_mulai_paket", "tgl_selesai_paket"},
	KolomDaftar: `row_key, kd_klpd, kd_nontender_pct, tahun_anggaran, kd_rup, nama_paket, nama_satker, mtd_pemilihan,
		pagu, total_realisasi, status_nontender_pct, status_nontender_pct_ket, tgl_selesai_paket, synced_at`,
})

// ============================================================
// TENDER Endpoint 8: Pencatatan Non Tender Realisasi
// ============================================================
//
// Satu pencatatan bisa punya beberapa baris realisasi. Daftar field HARUS sinkron dengan migrasi 026.
var pencatatanNonTenderRealisasi = newEndpointDatar(endpointDatar{
	Nama:  "pencatatan-non-tender-realisasi",
	Tabel: "inaproc_pencatatan_non_tender_realisasi",
	Teks: []string{
		"kd_klpd", "jenis_klpd", "nama_klpd", "kd_satker", "kd_satker_str", "nama_satker",
		"kd_lpse", "nama_lpse", "kd_nontender_pct", "kd_paket_dce", "kd_rup_paket", "tahun_anggaran", "nama_paket",
		"nama_ppk", "nip_ppk", "nama_penyedia", "npwp_penyedia",
		"no_realisasi", "jenis_realisasi", "ket_realisasi", "dok_realisasi",
	},
	Desimal: []string{"pagu", "nilai_realisasi"},
	Tanggal: []string{"tgl_realisasi"},
	KolomDaftar: `row_key, kd_klpd, kd_nontender_pct, tahun_anggaran, kd_rup_paket, nama_paket, nama_satker, nama_penyedia,
		no_realisasi, jenis_realisasi, pagu, nilai_realisasi, tgl_realisasi, synced_at`,
})

