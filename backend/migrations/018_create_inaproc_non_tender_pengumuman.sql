-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/tender/non-tender-pengumuman). Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_tender_handler.go.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_non_tender_pengumuman')
BEGIN
    CREATE TABLE inaproc_non_tender_pengumuman (
        row_key NVARCHAR(255) PRIMARY KEY,

        -- Satker & KLPD
        kd_klpd NVARCHAR(20) NULL,
        jenis_klpd NVARCHAR(100) NULL,
        nama_klpd NVARCHAR(255) NULL,
        kd_satker NVARCHAR(50) NULL,
        kd_satker_str NVARCHAR(50) NULL,
        nama_satker NVARCHAR(500) NULL,

        -- LPSE
        kd_lpse NVARCHAR(50) NULL,
        nama_lpse NVARCHAR(255) NULL,
        url_lpse NVARCHAR(1000) NULL,

        -- Paket
        kd_nontender NVARCHAR(50) NULL,
        kd_pkt_dce NVARCHAR(50) NULL,
        lls_id NVARCHAR(50) NULL,
        kd_rup NVARCHAR(50) NULL,
        tahun_anggaran NVARCHAR(10) NULL,
        nama_paket NVARCHAR(1000) NULL,
        jenis_pengadaan NVARCHAR(100) NULL,
        kualifikasi_paket NVARCHAR(100) NULL,
        kontrak_pembayaran NVARCHAR(100) NULL,
        mtd_pemilihan NVARCHAR(100) NULL,
        sumber_dana NVARCHAR(100) NULL,
        mak NVARCHAR(255) NULL,
        repeat_order NVARCHAR(50) NULL,
        versi_nontender INT NULL,

        -- Nilai (DECIMAL, bukan BIGINT, supaya pecahan rupiah tidak terpotong)
        pagu DECIMAL(24, 2) NULL,
        hps DECIMAL(24, 2) NULL,

        -- Status
        status_nontender NVARCHAR(100) NULL,
        ket_ditutup NVARCHAR(MAX) NULL,
        ket_diulang NVARCHAR(MAX) NULL,

        -- Pelaksana
        nip_nama_ppk NVARCHAR(500) NULL,
        nip_nama_pp NVARCHAR(500) NULL,
        nip_nama_pokja NVARCHAR(MAX) NULL,

        -- Tanggal
        tgl_buat_paket DATETIME2 NULL,
        tgl_kolektif_kolegial DATETIME2 NULL,
        tgl_pengumuman_nontender DATETIME2 NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_intp_kd_klpd_tahun ON inaproc_non_tender_pengumuman(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_intp_kd_nontender ON inaproc_non_tender_pengumuman(kd_nontender);
    CREATE INDEX idx_intp_kd_rup ON inaproc_non_tender_pengumuman(kd_rup);
END;
