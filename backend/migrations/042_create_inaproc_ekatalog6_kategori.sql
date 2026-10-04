-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/ekatalog/list-kategori-produk, E-Katalog V6), ditambah kolom tingkat. Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_ekatalog6.go (ada tes yang menjaganya).
-- Drill-down L1 -> L2 -> L3 dalam satu tabel. Respons level 2 dan 3 tidak memuat kode induknya, jadi kd_kategori_1 (level 2 dan 3)
-- dan kd_kategori_2 (level 3) yang kosong diisi dari permintaan, dan tingkat (1, 2, atau 3) dihitung dari permintaan; tingkat BUKAN
-- field API. Satu sinkronisasi menarik satu tingkat untuk satu induk dan mengganti baris tingkat dan induk yang sama.
-- nama_kategori_1 pada baris tingkat 2 dan 3 kosong (API tidak memberikannya): sambungkan ke baris tingkat 1 lewat kd_kategori_1.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_ekatalog6_kategori')
BEGIN
    CREATE TABLE inaproc_ekatalog6_kategori (
        row_key NVARCHAR(255) PRIMARY KEY,

        tingkat INT NULL,
        datamart_id NVARCHAR(50) NULL,
        kd_kategori_1 NVARCHAR(100) NULL,
        nama_kategori_1 NVARCHAR(500) NULL,
        kd_kategori_2 NVARCHAR(100) NULL,
        nama_kategori_2 NVARCHAR(500) NULL,
        kd_kategori_3 NVARCHAR(100) NULL,
        nama_kategori_3 NVARCHAR(500) NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_iek6k_tingkat ON inaproc_ekatalog6_kategori(tingkat, kd_kategori_1, kd_kategori_2);
END;
