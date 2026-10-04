-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/ekatalog-archive/penyedia-distributor-detail). Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_ekatalog.go (ada tes yang menjaganya).
-- Data rujukan yang dicari per kd_distributor (disimpan di kd_penyedia_distributor): sinkronisasi mengganti baris untuk kode itu.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_ekatalog_distributor')
BEGIN
    CREATE TABLE inaproc_ekatalog_distributor (
        row_key NVARCHAR(255) PRIMARY KEY,

        kd_penyedia_distributor NVARCHAR(50) NULL,
        nama_distributor NVARCHAR(500) NULL,
        npwp_distributor NVARCHAR(50) NULL,
        alamat_distributor NVARCHAR(1000) NULL,
        email_distributor NVARCHAR(255) NULL,
        no_telp_distributor NVARCHAR(100) NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_iekd_kd_distributor ON inaproc_ekatalog_distributor(kd_penyedia_distributor);
END;
