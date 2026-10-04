-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/tender/pencatatan-non-tender). Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_tender_pencatatan.go (ada tes yang menjaganya).
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_pencatatan_non_tender')
BEGIN
    CREATE TABLE inaproc_pencatatan_non_tender (
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
        kd_nontender_pct NVARCHAR(50) NULL,
        kd_pkt_dce NVARCHAR(50) NULL,
        kd_rup NVARCHAR(50) NULL,
        tahun_anggaran NVARCHAR(10) NULL,
        nama_paket NVARCHAR(1000) NULL,
        kategori_pengadaan NVARCHAR(255) NULL,
        mtd_pemilihan NVARCHAR(100) NULL,
        sumber_dana NVARCHAR(100) NULL,
        uraian_pekerjaan NVARCHAR(MAX) NULL,

        -- PPK
        nama_ppk NVARCHAR(500) NULL,
        nip_ppk NVARCHAR(50) NULL,

        -- Status
        status_nontender_pct NVARCHAR(100) NULL,
        status_nontender_pct_ket NVARCHAR(500) NULL,
        alasan_pembatalan NVARCHAR(MAX) NULL,

        -- Lainnya
        bukti_pembayaran NVARCHAR(1000) NULL,
        informasi_lainnya NVARCHAR(MAX) NULL,

        -- Nilai (DECIMAL, bukan BIGINT, supaya pecahan rupiah tidak terpotong)
        pagu DECIMAL(24, 2) NULL,
        nilai_pdn_pct DECIMAL(24, 2) NULL,
        nilai_umk_pct DECIMAL(24, 2) NULL,
        total_realisasi DECIMAL(24, 2) NULL,

        -- Tanggal
        tgl_buat_paket DATETIME2 NULL,
        tgl_mulai_paket DATETIME2 NULL,
        tgl_selesai_paket DATETIME2 NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_ipnt_kd_klpd_tahun ON inaproc_pencatatan_non_tender(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_ipnt_kd_nontender_pct ON inaproc_pencatatan_non_tender(kd_nontender_pct);
    CREATE INDEX idx_ipnt_kd_rup ON inaproc_pencatatan_non_tender(kd_rup);
END;
