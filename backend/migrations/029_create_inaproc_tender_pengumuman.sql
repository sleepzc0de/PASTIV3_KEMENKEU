-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/tender/pengumuman). Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_tender_proses.go (ada tes yang menjaganya).
-- Satu tender (kd_tender) bisa punya beberapa pengumuman (versi_tender), jadi kd_tender tidak unik.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_tender_pengumuman')
BEGIN
    CREATE TABLE inaproc_tender_pengumuman (
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
        kd_pkt_dce NVARCHAR(50) NULL,
        kd_rup NVARCHAR(50) NULL,
        kd_tender NVARCHAR(50) NULL,
        tahun_anggaran NVARCHAR(10) NULL,
        list_tahun_anggaran NVARCHAR(100) NULL,
        nama_paket NVARCHAR(1000) NULL,
        jenis_pengadaan NVARCHAR(100) NULL,
        kualifikasi_paket NVARCHAR(100) NULL,
        kontrak_pembayaran NVARCHAR(100) NULL,
        lokasi_pekerjaan NVARCHAR(1000) NULL,
        sumber_dana NVARCHAR(100) NULL,
        versi_tender INT NULL,

        -- Metode
        mtd_pemilihan NVARCHAR(100) NULL,
        mtd_kualifikasi NVARCHAR(255) NULL,
        mtd_evaluasi NVARCHAR(255) NULL,

        -- PPK dan Pokja
        nama_ppk NVARCHAR(255) NULL,
        nip_ppk NVARCHAR(50) NULL,
        nama_pokja NVARCHAR(255) NULL,
        nip_pokja NVARCHAR(50) NULL,

        -- Status
        status_tender NVARCHAR(100) NULL,
        ket_ditutup NVARCHAR(MAX) NULL,
        ket_diulang NVARCHAR(MAX) NULL,

        -- Nilai (DECIMAL, bukan BIGINT, supaya pecahan rupiah tidak terpotong)
        pagu DECIMAL(24, 2) NULL,
        hps DECIMAL(24, 2) NULL,

        -- Tanggal
        tanggal_status DATETIME2 NULL,
        tgl_buat_paket DATETIME2 NULL,
        tgl_kolektif_kolegial DATETIME2 NULL,
        tgl_pengumuman_tender DATETIME2 NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_itp_kd_klpd_tahun ON inaproc_tender_pengumuman(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_itp_kd_tender ON inaproc_tender_pengumuman(kd_tender);
    CREATE INDEX idx_itp_kd_rup ON inaproc_tender_pengumuman(kd_rup);
END;
