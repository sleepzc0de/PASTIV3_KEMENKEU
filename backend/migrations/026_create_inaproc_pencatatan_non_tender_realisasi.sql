-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/tender/pencatatan-non-tender-realisasi). Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_tender_pencatatan.go (ada tes yang menjaganya).
-- Satu pencatatan (kd_nontender_pct) bisa punya beberapa baris realisasi.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_pencatatan_non_tender_realisasi')
BEGIN
    CREATE TABLE inaproc_pencatatan_non_tender_realisasi (
        row_key NVARCHAR(255) PRIMARY KEY,

        -- Satker & KLPD
        kd_klpd NVARCHAR(20) NULL,
        jenis_klpd NVARCHAR(100) NULL,
        nama_klpd NVARCHAR(255) NULL,
        kd_satker NVARCHAR(50) NULL,
        kd_satker_str NVARCHAR(50) NULL,
        nama_satker NVARCHAR(500) NULL,

        -- Paket
        kd_lpse NVARCHAR(50) NULL,
        nama_lpse NVARCHAR(255) NULL,
        kd_nontender_pct NVARCHAR(50) NULL,
        kd_paket_dce NVARCHAR(50) NULL,
        kd_rup_paket NVARCHAR(50) NULL,
        tahun_anggaran NVARCHAR(10) NULL,
        nama_paket NVARCHAR(1000) NULL,

        -- PPK dan penyedia
        nama_ppk NVARCHAR(500) NULL,
        nip_ppk NVARCHAR(50) NULL,
        nama_penyedia NVARCHAR(500) NULL,
        npwp_penyedia NVARCHAR(50) NULL,

        -- Realisasi
        no_realisasi NVARCHAR(255) NULL,
        jenis_realisasi NVARCHAR(255) NULL,
        ket_realisasi NVARCHAR(MAX) NULL,
        dok_realisasi NVARCHAR(MAX) NULL,

        -- Nilai (DECIMAL, bukan BIGINT, supaya pecahan rupiah tidak terpotong)
        pagu DECIMAL(24, 2) NULL,
        nilai_realisasi DECIMAL(24, 2) NULL,

        -- Tanggal
        tgl_realisasi DATETIME2 NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_ipntr_kd_klpd_tahun ON inaproc_pencatatan_non_tender_realisasi(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_ipntr_kd_nontender_pct ON inaproc_pencatatan_non_tender_realisasi(kd_nontender_pct);
    CREATE INDEX idx_ipntr_kd_rup_paket ON inaproc_pencatatan_non_tender_realisasi(kd_rup_paket);
END;
