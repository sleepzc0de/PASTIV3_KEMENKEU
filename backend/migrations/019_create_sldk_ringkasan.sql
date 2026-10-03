-- Ringkasan aset SLDK untuk halaman Ringkasan dan Pemantauan (Data Aset).
--
-- Tabel SLDK sumber (DJKN.SIMAN2_M_ASET) berisi ~132 juta baris, jadi dashboard tidak menghitung langsung
-- ke sana. Perintah `pasti-sldk-sync ringkasan` (backend/cmd/sldk-sync) memindainya SEKALI dan menyimpan
-- hasil agregasinya di sini. Aplikasi hanya membaca; tabel ini diisi oleh perintah tersebut.
--
-- Nama kolom sldk_agregat harus sejalan dengan hasil query agregat di backend/sldk/agregat.go
-- (AggregateResultColumns). Menambah aturan pemantauan baru berarti menambah kolom n_<kunci> di sini.

-- Riwayat sinkronisasi: kapan, cakupan, hasil. Dipakai juga sebagai penanda "sedang berjalan".
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'sldk_sync_log')
BEGIN
    CREATE TABLE sldk_sync_log (
        id INT IDENTITY(1,1) PRIMARY KEY,
        jenis NVARCHAR(30) NOT NULL,
        status NVARCHAR(20) NOT NULL,        -- berjalan | sukses | gagal
        mulai DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        selesai DATETIME2 NULL,
        cakupan NVARCHAR(100) NULL,          -- kode K/L, atau kosong = seluruh K/L
        jumlah_baris INT NULL,               -- jumlah baris agregat yang ditulis
        total_aset BIGINT NULL,              -- jumlah seluruh baris M_ASET yang dihitung (dim total)
        data_per DATE NULL,                  -- tanggal data terbaru di SLDK (_ingestion_date)
        pesan NVARCHAR(1000) NULL
    );

    CREATE INDEX idx_sldk_sync_log_jenis ON sldk_sync_log(jenis, id);
END;

-- Hasil agregasi per dimensi. flag_key = kombinasi status_data|sts_his|sts_ast|A/H dari tiap baris sumber;
-- arti nilainya belum dikonfirmasi, jadi disimpan sebagai dimensi dan admin memilih yang dihitung "aktif".
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'sldk_agregat')
BEGIN
    CREATE TABLE sldk_agregat (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        dim NVARCHAR(20) NOT NULL,           -- total | jenis | kondisi | status | jenis_kondisi | satker | provinsi | tahun
        k1 NVARCHAR(200) NULL,               -- nilai dimensi (NULL = data sumber kosong)
        k2 NVARCHAR(200) NULL,               -- hanya untuk jenis_kondisi (kondisi)
        flag_key NVARCHAR(60) NOT NULL,
        jumlah BIGINT NOT NULL,
        nilai_perolehan DECIMAL(28, 2) NULL,
        nilai_buku DECIMAL(28, 2) NULL,
        nilai_susut DECIMAL(28, 2) NULL,

        -- Penghitung aturan pemantauan (backend/sldk/rules.go)
        n_idle BIGINT NOT NULL DEFAULT 0,
        n_hilang BIGINT NOT NULL DEFAULT 0,
        n_dihentikan BIGINT NOT NULL DEFAULT 0,
        n_nilai_nol BIGINT NOT NULL DEFAULT 0,
        n_dq_tanggal BIGINT NOT NULL DEFAULT 0,

        data_per DATE NULL
    );

    CREATE INDEX idx_sldk_agregat_dim ON sldk_agregat(dim, flag_key);
END;

-- Pengaturan dashboard (mis. kunci penanda mana yang dihitung "aset aktif"), diubah admin dari UI.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'sldk_pengaturan')
BEGIN
    CREATE TABLE sldk_pengaturan (
        kunci NVARCHAR(100) NOT NULL PRIMARY KEY,
        nilai NVARCHAR(MAX) NULL,
        diubah_oleh NVARCHAR(100) NULL,
        diubah_pada DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );
END;
