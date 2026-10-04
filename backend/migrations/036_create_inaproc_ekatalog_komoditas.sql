-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/ekatalog-archive/komoditas-detail), termasuk "Jenis_Katalog" yang memakai huruf besar di API-nya.
-- Daftar yang sama dipakai handler Go untuk membangun INSERT, jadi jangan menambah/mengubah kolom di sini
-- tanpa menyesuaikan daftar field di backend/handlers/inaproc_ekatalog.go (ada tes yang menjaganya).
-- Data rujukan yang dicari per kode_komoditas (disimpan di kd_komoditas): sinkronisasi mengganti baris untuk kode itu.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_ekatalog_komoditas')
BEGIN
    CREATE TABLE inaproc_ekatalog_komoditas (
        row_key NVARCHAR(255) PRIMARY KEY,

        kd_komoditas NVARCHAR(50) NULL,
        nama_komoditas NVARCHAR(500) NULL,
        Jenis_Katalog NVARCHAR(100) NULL,
        kd_instansi_katalog NVARCHAR(50) NULL,
        nama_instansi_katalog NVARCHAR(500) NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_iekk_kd_komoditas ON inaproc_ekatalog_komoditas(kd_komoditas);
END;
