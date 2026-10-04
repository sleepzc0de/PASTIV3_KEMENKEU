-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/ekatalog-archive/instansi-satker). Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_ekatalog.go (ada tes yang menjaganya).
-- Data rujukan per KLPD (tanpa tahun): sinkronisasi mengganti seluruh baris untuk kd_klpd yang sama.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_ekatalog_instansi_satker')
BEGIN
    CREATE TABLE inaproc_ekatalog_instansi_satker (
        row_key NVARCHAR(255) PRIMARY KEY,

        kd_klpd NVARCHAR(20) NULL,
        jenis_klpd NVARCHAR(100) NULL,
        nama_klpd NVARCHAR(255) NULL,
        kd_satker NVARCHAR(50) NULL,
        kd_satker_str NVARCHAR(50) NULL,
        nama_satker NVARCHAR(500) NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_iekis_kd_klpd ON inaproc_ekatalog_instansi_satker(kd_klpd);
    CREATE INDEX idx_iekis_kd_satker ON inaproc_ekatalog_instansi_satker(kd_satker);
END;
