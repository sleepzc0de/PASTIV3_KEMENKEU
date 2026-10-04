-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/tender/peserta-tender). Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_tender_proses.go (ada tes yang menjaganya).
-- Satu tender (kd_tender) punya banyak peserta (kd_peserta). pemenang dan pemenang_terverifikasi berupa penanda 0/1.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_peserta_tender')
BEGIN
    CREATE TABLE inaproc_peserta_tender (
        row_key NVARCHAR(255) PRIMARY KEY,

        -- Satker & KLPD
        kd_klpd NVARCHAR(20) NULL,
        kd_satker NVARCHAR(50) NULL,
        kd_satker_str NVARCHAR(50) NULL,
        kd_lpse NVARCHAR(50) NULL,

        -- Tender
        kd_pkt_dce NVARCHAR(50) NULL,
        kd_tender NVARCHAR(50) NULL,
        kd_peserta NVARCHAR(50) NULL,
        tahun_anggaran NVARCHAR(10) NULL,

        -- Penyedia
        kd_penyedia NVARCHAR(50) NULL,
        nama_penyedia NVARCHAR(500) NULL,
        npwp_penyedia NVARCHAR(50) NULL,
        npwp_penyedia_16 NVARCHAR(50) NULL,

        -- Hasil
        pemenang INT NULL,
        pemenang_terverifikasi INT NULL,
        alasan NVARCHAR(MAX) NULL,

        -- Nilai (DECIMAL, bukan BIGINT, supaya pecahan rupiah tidak terpotong)
        nilai_penawaran DECIMAL(24, 2) NULL,
        nilai_terkoreksi DECIMAL(24, 2) NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_ipt_kd_klpd_tahun ON inaproc_peserta_tender(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_ipt_kd_tender ON inaproc_peserta_tender(kd_tender);
    CREATE INDEX idx_ipt_kd_penyedia ON inaproc_peserta_tender(kd_penyedia);
END;
