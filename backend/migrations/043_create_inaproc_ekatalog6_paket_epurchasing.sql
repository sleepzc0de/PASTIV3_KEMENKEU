-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/ekatalog/paket-e-purchasing, E-Katalog V6). Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_ekatalog6.go (ada tes yang menjaganya).
-- Kolom KLPD dan tahun bernama kode_klpd dan fiscal_year (bukan kd_klpd / tahun_anggaran). kode_klpd "swasta" = paket swasta; bila
-- respons mengosongkannya, kolom diisi "swasta" supaya hapus-sebelum-tarik menemukannya. is_swasta berupa true/false di API (1/0 di
-- sini). Satu order (order_id) bisa muncul untuk beberapa produk (product_id).
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_ekatalog6_paket_epurchasing')
BEGIN
    CREATE TABLE inaproc_ekatalog6_paket_epurchasing (
        row_key NVARCHAR(255) PRIMARY KEY,

        -- Order
        kode_klpd NVARCHAR(50) NULL,
        fiscal_year NVARCHAR(10) NULL,
        order_id NVARCHAR(100) NULL,
        product_id NVARCHAR(100) NULL,
        kode_penyedia NVARCHAR(50) NULL,
        rekan_id NVARCHAR(50) NULL,
        status NVARCHAR(100) NULL,
        shipment_status NVARCHAR(100) NULL,
        is_swasta INT NULL,

        -- Satker dan RUP
        kode_satker NVARCHAR(50) NULL,
        nama_satker NVARCHAR(500) NULL,
        rup_code NVARCHAR(50) NULL,
        rup_name NVARCHAR(1000) NULL,
        rup_desc NVARCHAR(MAX) NULL,
        mak NVARCHAR(255) NULL,
        funding_source NVARCHAR(100) NULL,

        -- Jumlah dan nilai (DECIMAL, bukan BIGINT, supaya pecahan tidak terpotong; total_qty bisa berpecahan)
        count_product INT NULL,
        total_qty DECIMAL(24, 4) NULL,
        shipping_fee DECIMAL(24, 2) NULL,
        total DECIMAL(24, 2) NULL,

        -- Tanggal
        order_date DATETIME2 NULL,
        last_update_date DATETIME2 NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_iek6pe_klpd_tahun ON inaproc_ekatalog6_paket_epurchasing(kode_klpd, fiscal_year);
    CREATE INDEX idx_iek6pe_order_id ON inaproc_ekatalog6_paket_epurchasing(order_id);
    CREATE INDEX idx_iek6pe_kode_penyedia ON inaproc_ekatalog6_paket_epurchasing(kode_penyedia);
END;
