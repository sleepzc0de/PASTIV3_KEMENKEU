-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/tender/pencatatan-swakelola-realisasi). Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_tender_swakelola.go (ada tes yang menjaganya).
-- Satu pencatatan (kd_swakelola_pct) bisa punya beberapa baris realisasi. Respons ini tidak memuat
-- nama paket, nama satker, maupun pagu: untuk itu hubungkan ke inaproc_pencatatan_swakelola lewat
-- kd_swakelola_pct. nip_ppk dikirim sebagai angka oleh API; disimpan sebagai teks supaya digitnya utuh.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_pencatatan_swakelola_realisasi')
BEGIN
    CREATE TABLE inaproc_pencatatan_swakelola_realisasi (
        row_key NVARCHAR(255) PRIMARY KEY,

        -- KLPD, satker, dan paket
        kd_klpd NVARCHAR(20) NULL,
        kd_satker NVARCHAR(50) NULL,
        kd_lpse NVARCHAR(50) NULL,
        kd_swakelola_pct NVARCHAR(50) NULL,
        rsk_id NVARCHAR(50) NULL,
        tahun_anggaran NVARCHAR(10) NULL,

        -- PPK dan pelaksana
        nama_ppk NVARCHAR(500) NULL,
        nip_ppk NVARCHAR(50) NULL,
        nama_pelaksana NVARCHAR(500) NULL,
        npwp_pelaksana NVARCHAR(50) NULL,

        -- Realisasi
        no_realisasi NVARCHAR(255) NULL,
        jenis_realisasi NVARCHAR(255) NULL,
        ket_realisasi NVARCHAR(MAX) NULL,
        dok_realisasi NVARCHAR(MAX) NULL,

        -- Nilai (DECIMAL, bukan BIGINT, supaya pecahan rupiah tidak terpotong)
        nilai_realisasi DECIMAL(24, 2) NULL,

        -- Tanggal
        tgl_realisasi DATETIME2 NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_ipsr_kd_klpd_tahun ON inaproc_pencatatan_swakelola_realisasi(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_ipsr_kd_swakelola_pct ON inaproc_pencatatan_swakelola_realisasi(kd_swakelola_pct);
    CREATE INDEX idx_ipsr_rsk_id ON inaproc_pencatatan_swakelola_realisasi(rsk_id);
END;
