-- Tabel digitalisasi aset (KL 015) yang disalin dari SLDK, plus riwayat sinkronisasinya.
--
-- FILE INI DIHASILKAN dari backend/digitalisasi/datasets.go. Jangan disunting manual: ubah datasets.go lalu
-- jalankan  go test ./digitalisasi -run TestMigrationFile -update  (tes biasa gagal bila file ini tidak sejalan).
--
-- Tabel DIGITALISASI_* diisi oleh fitur Sinkronisasi di halaman Digitalisasi Aset (isinya diganti penuh tiap
-- sinkronisasi). Kolom id_sinkron menunjuk ke digitalisasi_sync_log.id.

-- Riwayat sinkronisasi per dataset: antri -> berjalan -> sukses | gagal | dibatalkan.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'digitalisasi_sync_log')
BEGIN
    CREATE TABLE digitalisasi_sync_log (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        dataset NVARCHAR(40) NOT NULL,       -- kunci dataset (lihat datasets.go)
        status NVARCHAR(20) NOT NULL,        -- antri | berjalan | sukses | gagal | dibatalkan
        dibuat DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        mulai DATETIME2 NULL,
        selesai DATETIME2 NULL,
        jumlah_baris BIGINT NULL,
        jumlah_koordinat BIGINT NULL,        -- baris yang punya koordinat valid
        pesan NVARCHAR(1000) NULL,           -- kemajuan saat berjalan; ringkasan atau galat setelah selesai
        dijalankan_oleh NVARCHAR(100) NULL
    );

    CREATE INDEX idx_digitalisasi_sync_log_dataset ON digitalisasi_sync_log(dataset, id);
END;

-- DIGITALISASI_SATKER: Satker aktif KL 015 (anak satker pada UE1 tertentu digabung ke induk) beserta jumlah kendaraan dinas
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'DIGITALISASI_SATKER')
BEGIN
    CREATE TABLE DIGITALISASI_SATKER (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        Kode_UE1 NVARCHAR(10) NULL,
        Kode_Satker NVARCHAR(40) NULL,
        Jenis_Satker NVARCHAR(20) NULL,
        Nama_Satker NVARCHAR(500) NULL,
        Alamat_Satker NVARCHAR(MAX) NULL,
        KelurahanDesa_Satker NVARCHAR(200) NULL,
        Kecamatan_Satker NVARCHAR(200) NULL,
        KabKota_Satker NVARCHAR(200) NULL,
        Provinsi_Satker NVARCHAR(200) NULL,
        Status_Gedung_Kantor NVARCHAR(200) NULL,
        Foto NVARCHAR(900) NULL,
        Jumlah_KDJ INT NULL,
        Kondisi_KDJ NVARCHAR(MAX) NULL,
        Jumlah_KDO INT NULL,
        Kondisi_KDO NVARCHAR(MAX) NULL,
        id_sinkron BIGINT NULL,
        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_digitalisasi_satker_kode_ue1 ON DIGITALISASI_SATKER(Kode_UE1);
    CREATE INDEX idx_digitalisasi_satker_kode_satker ON DIGITALISASI_SATKER(Kode_Satker);
    CREATE INDEX idx_digitalisasi_satker_provinsi_satker ON DIGITALISASI_SATKER(Provinsi_Satker);
END;

-- DIGITALISASI_TANAH: Aset tanah (kd_brg 2%) KL 015 beserta status hukum, jumlah bangunan di atasnya, dan koordinat
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'DIGITALISASI_TANAH')
BEGIN
    CREATE TABLE DIGITALISASI_TANAH (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        id_aset_tanah BIGINT NULL,
        kode_register_tanah NVARCHAR(60) NULL,
        Kode_UE1 NVARCHAR(10) NULL,
        Kode_Satker NVARCHAR(40) NULL,
        Nama_Satker NVARCHAR(500) NULL,
        Kode_tanah NVARCHAR(30) NULL,
        Uraian_tanah NVARCHAR(500) NULL,
        No_aset NVARCHAR(30) NULL,
        Alamat_tanah NVARCHAR(MAX) NULL,
        RTRW_tanah NVARCHAR(50) NULL,
        KelurahanDesa_tanah NVARCHAR(200) NULL,
        Kecamatan_tanah NVARCHAR(200) NULL,
        KabKota_tanah NVARCHAR(200) NULL,
        Provinsi_tanah NVARCHAR(200) NULL,
        Luas_Tanah DECIMAL(28, 4) NULL,
        Luas_Bangunan DECIMAL(28, 4) NULL,
        Jumlah_Bangunan INT NULL,
        Status_hukum NVARCHAR(MAX) NULL,
        Foto NVARCHAR(MAX) NULL,
        Kondisi_Tanah NVARCHAR(100) NULL,
        Nilai_Tanah DECIMAL(28, 2) NULL,
        Latitude DECIMAL(10, 7) NULL,
        Longitude DECIMAL(10, 7) NULL,
        id_sinkron BIGINT NULL,
        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_digitalisasi_tanah_kode_ue1 ON DIGITALISASI_TANAH(Kode_UE1);
    CREATE INDEX idx_digitalisasi_tanah_kode_satker ON DIGITALISASI_TANAH(Kode_Satker);
    CREATE INDEX idx_digitalisasi_tanah_provinsi_tanah ON DIGITALISASI_TANAH(Provinsi_tanah);
END;

-- DIGITALISASI_GEDUNG_KANTOR_UTAMA: Gedung kantor utama (kd_brg 4010101001) KL 015 beserta tanah di bawahnya, status asuransi, dan koordinat
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'DIGITALISASI_GEDUNG_KANTOR_UTAMA')
BEGIN
    CREATE TABLE DIGITALISASI_GEDUNG_KANTOR_UTAMA (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        Kode_UE1 NVARCHAR(10) NULL,
        Kode_Satker NVARCHAR(40) NULL,
        Nama_Satker NVARCHAR(500) NULL,
        Kode_Register_bangunan NVARCHAR(60) NULL,
        Kode_bangunan NVARCHAR(30) NULL,
        Uraian_bangunan NVARCHAR(500) NULL,
        NUP_bangunan NVARCHAR(30) NULL,
        Alamat_bangunan NVARCHAR(MAX) NULL,
        RTRW_bangunan NVARCHAR(50) NULL,
        KelurahanDesa_bangunan NVARCHAR(200) NULL,
        Kecamatan_bangunan NVARCHAR(200) NULL,
        KabKota_bangunan NVARCHAR(200) NULL,
        Provinsi_bangunan NVARCHAR(200) NULL,
        Luas_Bangunan DECIMAL(28, 4) NULL,
        Foto_bangunan NVARCHAR(MAX) NULL,
        Kondisi_Bangunan NVARCHAR(100) NULL,
        Nilai_Bangunan DECIMAL(28, 2) NULL,
        Status_Asuransi NVARCHAR(20) NULL,
        Kode_tanah NVARCHAR(30) NULL,
        Uraian_tanah NVARCHAR(500) NULL,
        No_aset_tanah NVARCHAR(30) NULL,
        Latitude DECIMAL(10, 7) NULL,
        Longitude DECIMAL(10, 7) NULL,
        id_sinkron BIGINT NULL,
        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_digitalisasi_gedung_kantor_utama_kode_ue1 ON DIGITALISASI_GEDUNG_KANTOR_UTAMA(Kode_UE1);
    CREATE INDEX idx_digitalisasi_gedung_kantor_utama_kode_satker ON DIGITALISASI_GEDUNG_KANTOR_UTAMA(Kode_Satker);
    CREATE INDEX idx_digitalisasi_gedung_kantor_utama_provinsi_bangunan ON DIGITALISASI_GEDUNG_KANTOR_UTAMA(Provinsi_bangunan);
END;

-- DIGITALISASI_GEDUNG_LAINNYA: Gedung dan bangunan lain (kd_brg 40101%, selain gedung kantor utama) KL 015 beserta fungsi, status hukum, dan asuransi
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'DIGITALISASI_GEDUNG_LAINNYA')
BEGIN
    CREATE TABLE DIGITALISASI_GEDUNG_LAINNYA (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        Kode_UE1 NVARCHAR(10) NULL,
        Kode_Satker NVARCHAR(40) NULL,
        Nama_Satker NVARCHAR(500) NULL,
        Kode_Register_bangunan NVARCHAR(60) NULL,
        Kode_bangunan NVARCHAR(30) NULL,
        Uraian_bangunan NVARCHAR(500) NULL,
        NUP_bangunan NVARCHAR(30) NULL,
        Alamat_bangunan NVARCHAR(MAX) NULL,
        RTRW_bangunan NVARCHAR(50) NULL,
        KelurahanDesa_bangunan NVARCHAR(200) NULL,
        Kecamatan_bangunan NVARCHAR(200) NULL,
        KabKota_bangunan NVARCHAR(200) NULL,
        Provinsi_bangunan NVARCHAR(200) NULL,
        Luas_Bangunan DECIMAL(28, 4) NULL,
        Foto_bangunan NVARCHAR(MAX) NULL,
        Fungsi NVARCHAR(500) NULL,
        Status_Hukum NVARCHAR(MAX) NULL,
        Nama_Pengguna NVARCHAR(500) NULL,
        Kondisi_Bangunan NVARCHAR(100) NULL,
        Nilai_Bangunan DECIMAL(28, 2) NULL,
        Status_Asuransi NVARCHAR(20) NULL,
        Latitude DECIMAL(10, 7) NULL,
        Longitude DECIMAL(10, 7) NULL,
        id_sinkron BIGINT NULL,
        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_digitalisasi_gedung_lainnya_kode_ue1 ON DIGITALISASI_GEDUNG_LAINNYA(Kode_UE1);
    CREATE INDEX idx_digitalisasi_gedung_lainnya_kode_satker ON DIGITALISASI_GEDUNG_LAINNYA(Kode_Satker);
    CREATE INDEX idx_digitalisasi_gedung_lainnya_provinsi_bangunan ON DIGITALISASI_GEDUNG_LAINNYA(Provinsi_bangunan);
END;

-- DIGITALISASI_RUSUNARA: Rumah susun negara (kd_brg 4010208%) KL 015 beserta jumlah kamar tidur per tipe (E, D, C)
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'DIGITALISASI_RUSUNARA')
BEGIN
    CREATE TABLE DIGITALISASI_RUSUNARA (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        Kode_UE1 NVARCHAR(10) NULL,
        Kode_Satker NVARCHAR(40) NULL,
        Nama_Satker NVARCHAR(500) NULL,
        Kode_Rusun NVARCHAR(30) NULL,
        Uraian_Rusun NVARCHAR(500) NULL,
        NUP_Rusun NVARCHAR(30) NULL,
        Alamat_Rusun NVARCHAR(MAX) NULL,
        RTRW_Rusun NVARCHAR(50) NULL,
        KelurahanDesa_Rusun NVARCHAR(200) NULL,
        Kecamatan_Rusun NVARCHAR(200) NULL,
        KabKota_Rusun NVARCHAR(200) NULL,
        Provinsi_Rusun NVARCHAR(200) NULL,
        Luas_Rusun DECIMAL(28, 4) NULL,
        Foto_Rusun NVARCHAR(MAX) NULL,
        Kondisi_Rusun NVARCHAR(100) NULL,
        Nilai_Rusun DECIMAL(28, 2) NULL,
        Kamar_tipe_E INT NULL,
        Kamar_tipe_D INT NULL,
        Kamar_tipe_C INT NULL,
        Latitude DECIMAL(10, 7) NULL,
        Longitude DECIMAL(10, 7) NULL,
        id_sinkron BIGINT NULL,
        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_digitalisasi_rusunara_kode_ue1 ON DIGITALISASI_RUSUNARA(Kode_UE1);
    CREATE INDEX idx_digitalisasi_rusunara_kode_satker ON DIGITALISASI_RUSUNARA(Kode_Satker);
    CREATE INDEX idx_digitalisasi_rusunara_provinsi_rusun ON DIGITALISASI_RUSUNARA(Provinsi_Rusun);
END;

-- DIGITALISASI_RUMAH_NEGARA: Rumah negara (kd_brg 4010201%, 4010202%, 4010209%; tanpa mess) KL 015 beserta status penghuni dan status hukum
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'DIGITALISASI_RUMAH_NEGARA')
BEGIN
    CREATE TABLE DIGITALISASI_RUMAH_NEGARA (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        Kode_UE1 NVARCHAR(10) NULL,
        Kode_Satker NVARCHAR(40) NULL,
        Nama_Satker NVARCHAR(500) NULL,
        Kode_Register_RN NVARCHAR(60) NULL,
        Kode_RN NVARCHAR(30) NULL,
        Uraian_RN NVARCHAR(500) NULL,
        NUP_RN NVARCHAR(30) NULL,
        Alamat_RN NVARCHAR(MAX) NULL,
        RTRW_RN NVARCHAR(50) NULL,
        KelurahanDesa_RN NVARCHAR(200) NULL,
        Kecamatan_RN NVARCHAR(200) NULL,
        KabKota_RN NVARCHAR(200) NULL,
        Provinsi_RN NVARCHAR(200) NULL,
        Luas_RN DECIMAL(28, 4) NULL,
        Foto_RN NVARCHAR(MAX) NULL,
        Status_Penghuni NVARCHAR(100) NULL,
        Nama_Penghuni NVARCHAR(500) NULL,  -- data pribadi: hanya tampil untuk admin
        Kondisi_RN NVARCHAR(100) NULL,
        Status_Hukum NVARCHAR(MAX) NULL,
        Latitude DECIMAL(10, 7) NULL,
        Longitude DECIMAL(10, 7) NULL,
        id_sinkron BIGINT NULL,
        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_digitalisasi_rumah_negara_kode_ue1 ON DIGITALISASI_RUMAH_NEGARA(Kode_UE1);
    CREATE INDEX idx_digitalisasi_rumah_negara_kode_satker ON DIGITALISASI_RUMAH_NEGARA(Kode_Satker);
    CREATE INDEX idx_digitalisasi_rumah_negara_provinsi_rn ON DIGITALISASI_RUMAH_NEGARA(Provinsi_RN);
END;

-- DIGITALISASI_MESS_RUMAH_NEGARA: Mess rumah negara (kd_brg 4010202016) KL 015 beserta status hukum dan jumlah kamar tidur tipe E dan D
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'DIGITALISASI_MESS_RUMAH_NEGARA')
BEGIN
    CREATE TABLE DIGITALISASI_MESS_RUMAH_NEGARA (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        Kode_UE1 NVARCHAR(10) NULL,
        Kode_Satker NVARCHAR(40) NULL,
        Nama_Satker NVARCHAR(500) NULL,
        Kode_mess NVARCHAR(30) NULL,
        Uraian_mess NVARCHAR(500) NULL,
        NUP_mess NVARCHAR(30) NULL,
        Alamat_mess NVARCHAR(MAX) NULL,
        RTRW_mess NVARCHAR(50) NULL,
        KelurahanDesa_mess NVARCHAR(200) NULL,
        Kecamatan_mess NVARCHAR(200) NULL,
        KabKota_mess NVARCHAR(200) NULL,
        Provinsi_mess NVARCHAR(200) NULL,
        Luas_mess DECIMAL(28, 4) NULL,
        Foto_mess NVARCHAR(MAX) NULL,
        Kondisi_mess NVARCHAR(100) NULL,
        Nilai_mess DECIMAL(28, 2) NULL,
        Status_Hukum NVARCHAR(MAX) NULL,
        Kamar_tipe_E INT NULL,
        Kamar_tipe_D INT NULL,
        Latitude DECIMAL(10, 7) NULL,
        Longitude DECIMAL(10, 7) NULL,
        id_sinkron BIGINT NULL,
        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_digitalisasi_mess_rumah_negara_kode_ue1 ON DIGITALISASI_MESS_RUMAH_NEGARA(Kode_UE1);
    CREATE INDEX idx_digitalisasi_mess_rumah_negara_kode_satker ON DIGITALISASI_MESS_RUMAH_NEGARA(Kode_Satker);
    CREATE INDEX idx_digitalisasi_mess_rumah_negara_provinsi_mess ON DIGITALISASI_MESS_RUMAH_NEGARA(Provinsi_mess);
END;
