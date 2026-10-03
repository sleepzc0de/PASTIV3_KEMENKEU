package sapa

// Data contoh yang sah untuk setiap formulir. Dipakai untuk menguji template yang diunggah admin (uji coba
// pengisian sebelum template dipakai pengguna) dan oleh tes.

func ContohKasus() Kasus {
	return Kasus{ID: 1, Noreg: "PJ-2026-00001", KodeSatker: "015010199409294002", NamaSatker: "Kantor Pelayanan Kekayaan Negara dan Lelang Jakarta I", KodeUE1: "01501"}
}

func ContohTim() DataTim {
	return DataTim{
		JabatanPimpinan: "Kepala Kantor", JenisTim: "Tim Internal Penjualan", MasaAwal: "2026-01-02", MasaAkhir: "2026-12-31", Kota: "Jakarta",
		Anggota: []Anggota{{"Budi", "Kepala Seksi A", "Ketua"}, {"Siti", "Kepala Seksi B", "Sekretaris"}, {"Andi", "Pelaksana", "Anggota"}},
	}
}

func ContohBA() DataBA {
	return DataBA{Bentuk: "Penjualan", TanggalPenelitian: "2026-10-03", NamaTim: "Tim Internal Penjualan"}
}

func ContohNDSatker() DataNDSatker {
	return DataNDSatker{
		SudahRP4: true, TujuanSurat: "Sekretaris Direktorat Jenderal Contoh", Kota: "Jakarta", SingkatanSatker: "KPKNL Jakarta I",
		JenisBMN: "Tanah dan Bangunan", Satuan: "bidang", Alasan: "Barang rusak berat dan tidak ekonomis untuk diperbaiki.\nBiaya pemeliharaan tinggi.",
		TiketSiman: "SIMAN-2026-0099", KepalaKanwil: "Kepala Kantor Wilayah DJKN Jakarta",
		Penandatangan: Penandatangan{Nama: "Budi Santoso", NIP: "198001012005011001", Jabatan: "Kepala Kantor"},
		Barang: []Barang{
			{Nama: "Tanah Kantor", Kode: "2010101001", NUP: "1", Lokasi: "Jl. Merdeka 1", Kondisi: "Baik", TahunPerolehan: "2001", NilaiPerolehan: "1000000000", NilaiLimit: "900000000", Keterangan: "-"},
			{Nama: "Gedung Kantor", Kode: "4010101001", NUP: "2", Lokasi: "Jl. Merdeka 1\nLantai 2", Kondisi: "Rusak Berat", TahunPerolehan: "1999", NilaiPerolehan: "2500000000.50", NilaiLimit: "1500000000"},
			{Nama: "Pos Jaga", Kode: "4010101002", NUP: "3", Lokasi: "Jl. Merdeka 1", Kondisi: "Rusak Berat", TahunPerolehan: "2005", NilaiPerolehan: "50000000", NilaiLimit: "10000000"},
		},
		Dokumen: map[string]DokPendukung{
			DokBeritaAcara: {Ada: true, Nomor: "BA-12/2026", Tanggal: "2026-09-15"},
			DokPSP:         {Ada: true, Nomor: "PSP-1", Tanggal: "2020-01-02"},
			DokRP4:         {Ada: true},
			DokKIB:         {Ada: false},
		},
	}
}

func ContohNDUE1() DataNDUE1 {
	return DataNDUE1{
		NomorND: "ND-77/KPKNL.JKT1/2026", TanggalND: "2026-10-01", HalND: "Permohonan Penjualan BMN pada KPKNL Jakarta I",
		SekretarisUE1: "Sekretaris Direktorat Jenderal Contoh", KepalaKanwil: "Kepala Kantor Wilayah DJKN Jakarta",
		PejabatPengelola: "Direktur Barang Milik Negara", Penandatangan: Penandatangan{Nama: "Dewi Lestari", Jabatan: "Sekretaris Direktorat Jenderal Contoh"},
	}
}
