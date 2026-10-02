-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/tender/non-tender-ekontrak-kontrak). Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_tender_handler.go.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_non_tender_ekontrak_kontrak')
BEGIN
    CREATE TABLE inaproc_non_tender_ekontrak_kontrak (
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
        kd_nontender NVARCHAR(50) NULL,
        tahun_anggaran NVARCHAR(10) NULL,
        nama_paket NVARCHAR(1000) NULL,
        mtd_pengadaan NVARCHAR(100) NULL,
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
        nama_penyedia NVARCHAR(500) NULL,
        bentuk_usaha_penyedia NVARCHAR(255) NULL,
        tipe_penyedia NVARCHAR(100) NULL,
        npwp_penyedia NVARCHAR(50) NULL,
        npwp16_penyedia NVARCHAR(50) NULL,
        wakil_sah_penyedia NVARCHAR(255) NULL,
        jabatan_wakil_penyedia NVARCHAR(255) NULL,
        anggota_kso NVARCHAR(MAX) NULL,
        nama_rek_bank NVARCHAR(255) NULL,
        no_rek_bank NVARCHAR(100) NULL,
        nama_pemilik_rek_bank NVARCHAR(255) NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_intek_kd_klpd_tahun ON inaproc_non_tender_ekontrak_kontrak(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_intek_kd_nontender ON inaproc_non_tender_ekontrak_kontrak(kd_nontender);
    CREATE INDEX idx_intek_no_kontrak ON inaproc_non_tender_ekontrak_kontrak(no_kontrak);
END;
