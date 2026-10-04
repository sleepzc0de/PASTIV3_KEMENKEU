-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/ekatalog/list-produk-penyedia, E-Katalog V6), ditambah kolom kode_penyedia. Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_ekatalog6.go (ada tes yang menjaganya).
-- Respons API hanya memuat produknya (tanpa kode penyedia), jadi kode_penyedia BUKAN field API: diisi dari kode yang diminta saat
-- sinkronisasi, dan dipakai mengganti baris lama untuk penyedia yang sama. status_produk_tayang berupa true/false di API (1/0 di sini).
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_ekatalog6_produk_penyedia')
BEGIN
    CREATE TABLE inaproc_ekatalog6_produk_penyedia (
        row_key NVARCHAR(255) PRIMARY KEY,

        kode_penyedia NVARCHAR(50) NULL,
        kd_produk NVARCHAR(100) NULL,
        nama_produk NVARCHAR(1000) NULL,
        status_produk NVARCHAR(50) NULL,
        status_produk_tayang INT NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_iek6pp_kode_penyedia ON inaproc_ekatalog6_produk_penyedia(kode_penyedia);
    CREATE INDEX idx_iek6pp_kd_produk ON inaproc_ekatalog6_produk_penyedia(kd_produk);
END;
