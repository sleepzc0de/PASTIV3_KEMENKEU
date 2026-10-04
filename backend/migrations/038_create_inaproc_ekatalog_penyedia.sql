-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/ekatalog-archive/penyedia-detail). Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_ekatalog.go (ada tes yang menjaganya).
-- Data rujukan yang dicari per kode_penyedia (diasumsikan = kd_penyedia): sinkronisasi mengganti baris untuk kode itu.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_ekatalog_penyedia')
BEGIN
    CREATE TABLE inaproc_ekatalog_penyedia (
        row_key NVARCHAR(255) PRIMARY KEY,

        kd_penyedia NVARCHAR(50) NULL,
        kode_penyedia_sikap NVARCHAR(50) NULL,
        nama_penyedia NVARCHAR(500) NULL,
        npwp_penyedia NVARCHAR(50) NULL,
        npwp_16 NVARCHAR(50) NULL,
        penyedia_ukm NVARCHAR(100) NULL,
        kbli2020_penyedia NVARCHAR(MAX) NULL, -- daftar kode KBLI dipisah titik koma
        alamat_penyedia NVARCHAR(1000) NULL,
        email_penyedia NVARCHAR(255) NULL,
        no_telp_penyedia NVARCHAR(100) NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_iekp_kd_penyedia ON inaproc_ekatalog_penyedia(kd_penyedia);
    CREATE INDEX idx_iekp_sikap ON inaproc_ekatalog_penyedia(kode_penyedia_sikap);
END;
