-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/tender/tender-selesai-nilai). Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_tender_proses.go (ada tes yang menjaganya).
-- Nilai penawaran/negosiasi/kontrak per penyedia. Respons tidak memuat nama paket: hubungkan ke inaproc_tender_selesai
-- lewat kd_tender. Di sini kd_satker berbentuk kode bertitik.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_tender_selesai_nilai')
BEGIN
    CREATE TABLE inaproc_tender_selesai_nilai (
        row_key NVARCHAR(255) PRIMARY KEY,

        -- Satker & KLPD
        kd_klpd NVARCHAR(20) NULL,
        jenis_klpd NVARCHAR(100) NULL,
        nama_klpd NVARCHAR(255) NULL,
        kd_satker NVARCHAR(50) NULL,
        nama_satker NVARCHAR(500) NULL,
        kd_lpse NVARCHAR(50) NULL,

        -- Tender
        kd_tender NVARCHAR(50) NULL,
        kd_paket NVARCHAR(50) NULL,
        kd_rup_paket NVARCHAR(50) NULL,
        psr_id NVARCHAR(50) NULL,
        tahun_anggaran NVARCHAR(10) NULL,

        -- Penyedia
        kd_penyedia NVARCHAR(50) NULL,
        nama_penyedia NVARCHAR(500) NULL,
        npwp_penyedia NVARCHAR(50) NULL,
        npwp_16_penyedia NVARCHAR(50) NULL,

        -- Nilai (DECIMAL, bukan BIGINT, supaya pecahan rupiah tidak terpotong)
        pagu DECIMAL(24, 2) NULL,
        hps DECIMAL(24, 2) NULL,
        nilai_penawaran DECIMAL(24, 2) NULL,
        nilai_terkoreksi DECIMAL(24, 2) NULL,
        nilai_negosiasi DECIMAL(24, 2) NULL,
        nilai_kontrak DECIMAL(24, 2) NULL,
        nilai_pdn_kontrak DECIMAL(24, 2) NULL,
        nilai_umk_kontrak DECIMAL(24, 2) NULL,

        -- Tanggal
        tgl_pengumuman_tender DATETIME2 NULL,
        tgl_penetapan_pemenang DATETIME2 NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_itsn_kd_klpd_tahun ON inaproc_tender_selesai_nilai(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_itsn_kd_tender ON inaproc_tender_selesai_nilai(kd_tender);
    CREATE INDEX idx_itsn_psr_id ON inaproc_tender_selesai_nilai(psr_id);
END;
