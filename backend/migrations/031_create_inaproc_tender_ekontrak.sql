-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/tender/tender-ekontrak). Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_tender_proses.go (ada tes yang menjaganya).
-- Field kontrak sama dengan tender-ekontrak-kontrak (migrasi 032); tabel ini menambah tiga riwayat yang selalu berupa larik JSON
-- ([] bila kosong): bapbast_history_json, spmkspp_history_json, penilaian_kinerja_penyedia. Data rekening bank penyedia ikut
-- disimpan, sama seperti kontrak non tender.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_tender_ekontrak')
BEGIN
    CREATE TABLE inaproc_tender_ekontrak (
        row_key NVARCHAR(255) PRIMARY KEY,

        -- Satker & KLPD
        kd_klpd NVARCHAR(20) NULL,
        jenis_klpd NVARCHAR(100) NULL,
        nama_klpd NVARCHAR(255) NULL,
        kd_lpse NVARCHAR(50) NULL,
        kd_satker NVARCHAR(50) NULL,
        kd_satker_str NVARCHAR(50) NULL,
        nama_satker NVARCHAR(500) NULL,
        alamat_satker NVARCHAR(1000) NULL,

        -- Paket
        kd_tender NVARCHAR(50) NULL,
        tahun_anggaran NVARCHAR(10) NULL,
        nama_paket NVARCHAR(1000) NULL,
        lingkup_pekerjaan NVARCHAR(MAX) NULL,
        informasi_lainnya NVARCHAR(MAX) NULL,

        -- Kontrak
        no_kontrak NVARCHAR(255) NULL,
        no_sppbj NVARCHAR(255) NULL,
        jenis_kontrak NVARCHAR(100) NULL,
        status_kontrak NVARCHAR(100) NULL,
        kota_kontrak NVARCHAR(255) NULL,
        tgl_kontrak DATETIME2 NULL,
        tgl_kontrak_awal DATETIME2 NULL,
        tgl_kontrak_akhir DATETIME2 NULL,
        tgl_penetapan_status_kontrak DATETIME2 NULL,
        alasan_penetapan_status_kontrak NVARCHAR(MAX) NULL,
        apakah_addendum NVARCHAR(50) NULL,
        versi_addendum INT NULL,
        alasan_addendum NVARCHAR(MAX) NULL,

        -- Nilai (DECIMAL, bukan BIGINT, supaya pecahan rupiah tidak terpotong)
        nilai_kontrak DECIMAL(24, 2) NULL,
        nilai_pdn_kontrak DECIMAL(24, 2) NULL,
        nilai_umk_kontrak DECIMAL(24, 2) NULL,
        alasan_ubah_nilai_kontrak NVARCHAR(MAX) NULL,
        alasan_nilai_kontrak_10_persen NVARCHAR(MAX) NULL,

        -- PPK
        nama_ppk NVARCHAR(255) NULL,
        nip_ppk NVARCHAR(50) NULL,
        jabatan_ppk NVARCHAR(255) NULL,
        no_sk_ppk NVARCHAR(255) NULL,

        -- Penyedia
        kd_penyedia NVARCHAR(50) NULL,
        nama_penyedia NVARCHAR(500) NULL,
        bentuk_usaha_penyedia NVARCHAR(255) NULL,
        tipe_penyedia NVARCHAR(100) NULL,
        npwp_penyedia NVARCHAR(50) NULL,
        npwp_16_penyedia NVARCHAR(50) NULL,
        wakil_sah_penyedia NVARCHAR(255) NULL,
        jabatan_wakil_penyedia NVARCHAR(255) NULL,
        anggota_kso NVARCHAR(MAX) NULL,
        nama_rek_bank NVARCHAR(255) NULL,
        no_rek_bank NVARCHAR(100) NULL,
        nama_pemilik_rek_bank NVARCHAR(255) NULL,

        -- Riwayat (teks JSON berisi larik)
        bapbast_history_json NVARCHAR(MAX) NULL,
        spmkspp_history_json NVARCHAR(MAX) NULL,
        penilaian_kinerja_penyedia NVARCHAR(MAX) NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_ite_kd_klpd_tahun ON inaproc_tender_ekontrak(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_ite_kd_tender ON inaproc_tender_ekontrak(kd_tender);
    CREATE INDEX idx_ite_no_kontrak ON inaproc_tender_ekontrak(no_kontrak);
END;
