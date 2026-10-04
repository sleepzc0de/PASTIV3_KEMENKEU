-- Penarikan data Pengadaan/Tender/E-Katalog terpadu: riwayat tiap tugas penarikan (manual dan otomatis) dan pengaturan penarikan otomatis.

-- Satu baris = satu tugas penarikan (satu dataset dengan satu isian, mis. "K10/2025"). Tugas yang dimulai bersamaan
-- (satu antrean) berbagi batch_id. Penarikan otomatis dikenali dari pemicu = 'otomatis' dan dijadwalkan per (dataset, parameter).
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_penarikan')
BEGIN
    CREATE TABLE inaproc_penarikan (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        batch_id NVARCHAR(36) NOT NULL,
        dataset NVARCHAR(100) NOT NULL,
        parameter NVARCHAR(200) NOT NULL DEFAULT '',
        pemicu NVARCHAR(10) NOT NULL,
        status NVARCHAR(20) NOT NULL,
        jumlah_baris INT NULL,
        baris_gagal INT NULL,
        halaman INT NULL,
        percobaan INT NOT NULL DEFAULT 1,
        pesan NVARCHAR(1000) NULL,
        dijalankan_oleh NVARCHAR(100) NULL,
        dibuat DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        mulai DATETIME2 NULL,
        selesai DATETIME2 NULL
    );

    CREATE INDEX idx_inaproc_penarikan_dataset ON inaproc_penarikan(dataset, parameter, id DESC);
    CREATE INDEX idx_inaproc_penarikan_batch ON inaproc_penarikan(batch_id);
    CREATE INDEX idx_inaproc_penarikan_pemicu ON inaproc_penarikan(pemicu, status, id DESC);
END;

-- Satu baris saja (id = 1). Tanpa baris = pakai nilai bawaan dari konfigurasi server (INAPROC_AUTO_*).
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_penarikan_pengaturan')
BEGIN
    CREATE TABLE inaproc_penarikan_pengaturan (
        id INT NOT NULL PRIMARY KEY CHECK (id = 1),
        aktif BIT NOT NULL,
        interval_hari INT NOT NULL,
        jam_mulai INT NOT NULL,
        jam_akhir INT NOT NULL,
        kode_klpd NVARCHAR(20) NOT NULL,
        jumlah_tahun INT NOT NULL,
        jeda_detik INT NOT NULL,
        dataset NVARCHAR(MAX) NULL,
        diubah DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        diubah_oleh NVARCHAR(100) NULL
    );
END;
