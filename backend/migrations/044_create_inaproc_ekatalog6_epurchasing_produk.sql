-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/ekatalog/e-purchasing-by-produk, E-Katalog V6), ditambah kolom tahun. Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_ekatalog6.go (ada tes yang menjaganya).
-- Respons tidak memuat tahun, jadi tahun BUKAN field API: diisi dari tahun yang diminta saat sinkronisasi. Satu sinkronisasi menarik
-- satu kombinasi (tahun, status, kode_klpd / kd_kategori_1, kd_product) dan mengganti baris untuk kombinasi yang sama. status yang
-- tidak dikirim berarti COMPLETED saja di API; sinkronisasi kita selalu mengirim status eksplisit. kd_product di API dibandingkan
-- dengan kolom product_id (asumsi).
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_ekatalog6_epurchasing_produk')
BEGIN
    CREATE TABLE inaproc_ekatalog6_epurchasing_produk (
        row_key NVARCHAR(255) PRIMARY KEY,

        tahun NVARCHAR(10) NULL,
        order_id NVARCHAR(100) NULL,
        status NVARCHAR(100) NULL,

        -- Kategori produk (tiga tingkat)
        kd_kategori_1 NVARCHAR(100) NULL,
        kategori_1 NVARCHAR(500) NULL,
        kd_kategori_2 NVARCHAR(100) NULL,
        kategori_2 NVARCHAR(500) NULL,
        kd_kategori_3 NVARCHAR(100) NULL,
        kategori_3 NVARCHAR(500) NULL,

        -- Produk
        product_id NVARCHAR(100) NULL,
        nama_produk NVARCHAR(1000) NULL,

        -- KLPD dan satker
        kode_klpd NVARCHAR(50) NULL,
        nama_group_klpd NVARCHAR(255) NULL,
        nama_klpd NVARCHAR(255) NULL,
        kode_satker NVARCHAR(50) NULL,
        nama_satker NVARCHAR(500) NULL,

        -- Nilai (DECIMAL, bukan BIGINT, supaya pecahan tidak terpotong)
        nilai_transaksi DECIMAL(24, 2) NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_iek6ep_tahun_klpd_status ON inaproc_ekatalog6_epurchasing_produk(tahun, kode_klpd, status);
    CREATE INDEX idx_iek6ep_order_id ON inaproc_ekatalog6_epurchasing_produk(order_id);
    CREATE INDEX idx_iek6ep_product_id ON inaproc_ekatalog6_epurchasing_produk(product_id);
END;
