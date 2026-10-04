-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/tender/tender-selesai). Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_tender_proses.go (ada tes yang menjaganya).
-- Contoh dokumentasi memuat karakter tak terlihat (lebar nol) di akhir nama KLPD/satker/LPSE; nilainya disimpan apa adanya,
-- jadi kolom nama dibuat lebih lebar supaya baris tidak gagal tersimpan karena terpotong.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_tender_selesai')
BEGIN
    CREATE TABLE inaproc_tender_selesai (
        row_key NVARCHAR(255) PRIMARY KEY,

        -- Satker & KLPD
        kd_klpd NVARCHAR(20) NULL,
        jenis_klpd NVARCHAR(100) NULL,
        nama_klpd NVARCHAR(500) NULL,
        kd_satker NVARCHAR(50) NULL,
        kd_satker_str NVARCHAR(50) NULL,
        nama_satker NVARCHAR(1000) NULL,

        -- LPSE
        kd_lpse NVARCHAR(50) NULL,
        nama_lpse NVARCHAR(500) NULL,
        url_lpse NVARCHAR(1000) NULL,

        -- Paket
        kd_rup NVARCHAR(50) NULL,
        kd_tender NVARCHAR(50) NULL,
        tahun_anggaran NVARCHAR(10) NULL,
        nama_paket NVARCHAR(1000) NULL,
        jenis_pengadaan NVARCHAR(100) NULL,
        kualifikasi_paket NVARCHAR(100) NULL,
        kontrak_pembayaran NVARCHAR(100) NULL,
        sumber_dana NVARCHAR(100) NULL,
        mak NVARCHAR(255) NULL,
        mtd_pemilihan NVARCHAR(100) NULL,
        mtd_kualifikasi NVARCHAR(255) NULL,

        -- Status
        status_tender NVARCHAR(100) NULL,
        last_update_ref NVARCHAR(255) NULL,

        -- Nilai (DECIMAL, bukan BIGINT, supaya pecahan rupiah tidak terpotong)
        pagu DECIMAL(24, 2) NULL,
        hps DECIMAL(24, 2) NULL,

        -- Tanggal
        tgl_pengumuman_tender DATETIME2 NULL,
        tgl_penetapan_pemenang DATETIME2 NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_its_kd_klpd_tahun ON inaproc_tender_selesai(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_its_kd_tender ON inaproc_tender_selesai(kd_tender);
    CREATE INDEX idx_its_kd_rup ON inaproc_tender_selesai(kd_rup);
END;
