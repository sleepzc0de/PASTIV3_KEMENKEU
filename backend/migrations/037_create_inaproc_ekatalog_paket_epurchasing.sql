-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/ekatalog-archive/paket-e-purchasing). Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_ekatalog.go (ada tes yang menjaganya).
-- Satu paket (kd_paket) bisa punya beberapa baris produk (kd_paket_produk), jadi kd_paket tidak unik.
-- Kontak Pokja (email_user_pokja, no_telp_user_pokja) ikut disimpan apa adanya dari API.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_ekatalog_paket_epurchasing')
BEGIN
    CREATE TABLE inaproc_ekatalog_paket_epurchasing (
        row_key NVARCHAR(255) PRIMARY KEY,

        -- Paket
        kd_klpd NVARCHAR(20) NULL,
        kd_rup NVARCHAR(50) NULL,
        tahun_anggaran NVARCHAR(10) NULL,
        kd_paket NVARCHAR(50) NULL,
        no_paket NVARCHAR(100) NULL,
        nama_paket NVARCHAR(1000) NULL,
        deskripsi NVARCHAR(MAX) NULL,
        catatan_produk NVARCHAR(MAX) NULL,
        status_paket NVARCHAR(100) NULL,
        paket_status_str NVARCHAR(100) NULL,
        kode_anggaran NVARCHAR(255) NULL,
        nama_sumber_dana NVARCHAR(100) NULL,

        -- Produk dan penyedia
        kd_komoditas NVARCHAR(50) NULL,
        kd_produk NVARCHAR(50) NULL,
        kd_paket_produk NVARCHAR(50) NULL,
        kd_penyedia NVARCHAR(50) NULL,
        kd_penyedia_distributor NVARCHAR(50) NULL,

        -- Satker
        satker_id NVARCHAR(50) NULL,
        nama_satker NVARCHAR(500) NULL,
        alamat_satker NVARCHAR(1000) NULL,
        npwp_satker NVARCHAR(50) NULL,

        -- PPK dan Pokja
        kd_user_ppk NVARCHAR(50) NULL,
        ppk_nip NVARCHAR(50) NULL,
        jabatan_ppk NVARCHAR(255) NULL,
        kd_user_pokja NVARCHAR(50) NULL,
        email_user_pokja NVARCHAR(255) NULL,
        no_telp_user_pokja NVARCHAR(50) NULL,

        -- Wilayah harga
        kd_provinsi_wilayah_harga NVARCHAR(50) NULL,
        kd_kabupaten_wilayah_harga NVARCHAR(50) NULL,

        -- Harga (DECIMAL, bukan BIGINT, supaya pecahan tidak terpotong; kuantitas bisa berpecahan)
        harga_satuan DECIMAL(24, 2) NULL,
        kuantitas DECIMAL(24, 4) NULL,
        ongkos_kirim DECIMAL(24, 2) NULL,
        total_harga DECIMAL(24, 2) NULL,
        jml_jenis_produk INT NULL,

        -- Tanggal
        tanggal_buat_paket DATETIME2 NULL,
        tanggal_edit_paket DATETIME2 NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_iekpe_kd_klpd_tahun ON inaproc_ekatalog_paket_epurchasing(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_iekpe_kd_paket ON inaproc_ekatalog_paket_epurchasing(kd_paket);
    CREATE INDEX idx_iekpe_kd_komoditas ON inaproc_ekatalog_paket_epurchasing(kd_komoditas);
    CREATE INDEX idx_iekpe_kd_penyedia ON inaproc_ekatalog_paket_epurchasing(kd_penyedia);
END;
