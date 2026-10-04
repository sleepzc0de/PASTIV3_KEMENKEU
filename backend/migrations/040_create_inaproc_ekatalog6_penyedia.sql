-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/ekatalog/penyedia-detail, E-Katalog V6). Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_ekatalog6.go (ada tes yang menjaganya).
-- Data rujukan yang dicari per kode_penyedia (kode teks, mis. "01ABCXYZ123"): sinkronisasi mengganti baris untuk kode itu.
-- status_umkk berupa penanda 0/1.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_ekatalog6_penyedia')
BEGIN
    CREATE TABLE inaproc_ekatalog6_penyedia (
        row_key NVARCHAR(255) PRIMARY KEY,

        kode_penyedia NVARCHAR(50) NULL,
        nama_penyedia NVARCHAR(500) NULL,
        nib NVARCHAR(50) NULL,
        npwp_penyedia NVARCHAR(50) NULL,
        bentuk_usaha NVARCHAR(100) NULL,
        jenis_perusahaan NVARCHAR(100) NULL,
        status_aktif NVARCHAR(50) NULL,
        rekan_id NVARCHAR(50) NULL,
        alamat_penyedia NVARCHAR(1000) NULL,
        email NVARCHAR(255) NULL,
        telepon NVARCHAR(100) NULL,
        kbli_id NVARCHAR(MAX) NULL,   -- beberapa kode dipisah koma
        kbli_name NVARCHAR(MAX) NULL, -- beberapa nama dipisah koma
        status_umkk INT NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_iek6p_kode_penyedia ON inaproc_ekatalog6_penyedia(kode_penyedia);
    CREATE INDEX idx_iek6p_nib ON inaproc_ekatalog6_penyedia(nib);
END;
