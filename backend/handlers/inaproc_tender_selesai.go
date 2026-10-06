package handlers

// ============================================================
// TENDER Endpoint 6: Non Tender Selesai
// ============================================================
//
// Respons datar; logikanya ada di endpointDatar (inaproc_tender_datar.go). Daftar field di bawah HARUS sinkron dengan
// migrasi 024.
var nonTenderSelesai = newEndpointDatar(endpointDatar{
	Nama:  "non-tender-selesai",
	Tabel: "inaproc_non_tender_selesai",
	Teks: []string{
		"kd_klpd", "jenis_klpd", "nama_klpd", "kd_satker", "kd_satker_str", "nama_satker",
		"kd_lpse", "lpse_id", "nama_lpse", "url_lpse",
		"kd_nontender", "kd_pkt_dce", "kd_rup", "tahun_anggaran", "nama_paket",
		"jenis_pengadaan", "kualifikasi_paket", "kontrak_pembayaran", "mtd_pemilihan",
		"sumber_dana", "mak",
		"kd_penyedia", "nama_penyedia", "npwp_penyedia", "npwp16_penyedia",
		"status_nontender",
	},
	Desimal: []string{
		"pagu", "hps", "nilai_penawaran", "nilai_negosiasi", "nilai_terkoreksi",
		"nilai_kontrak", "nilai_pdn_kontrak", "nilai_umk_kontrak",
	},
	Tanggal: []string{"tgl_pengumuman_nontender", "tgl_selesai_nontender", "tgl_penarikan"},
	KolomDaftar: `row_key, kd_klpd, kd_nontender, tahun_anggaran, kd_rup, nama_paket, nama_satker,
		mtd_pemilihan, nama_penyedia, pagu, hps, nilai_kontrak, status_nontender, tgl_selesai_nontender, synced_at`,
})

