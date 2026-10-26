-- Riwayat bulanan Digitalisasi Aset: potret ketujuh tabel DIGITALISASI_* pada akhir tiap bulan, untuk melihat perubahan data sebagai deret waktu.
--
-- FILE INI DIHASILKAN dari backend/digitalisasi/riwayat.go. Jangan disunting manual: ubah riwayat.go lalu
-- jalankan  go test ./digitalisasi -run TestHistoryMigrationFile -update  (tes biasa gagal bila file ini tidak sejalan).
--
-- Satu baris = satu baris tabel sumber pada satu periode. periode = TANGGAL TERAKHIR bulan yang ditutup (posisi akhir Oktober 2026 = 2026-10-31). jenis_aset = kunci dataset
-- (satker, tanah, gedung_kantor_utama, gedung_lainnya, rusunara, rumah_negara, mess_rumah_negara). Kolom inti seragam dipetakan dari kolom tiap tabel sumber; kolom khusus jenis
-- aset ada di data_tambahan (JSON, kunci = nama kolom huruf kecil). Data pribadi, tautan foto, dan nama pengguna bangunan tidak disalin.
-- Aman dijalankan berulang kali (idempotent).

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'DIGITALISASI_ASET_ALL')
BEGIN
    CREATE TABLE DIGITALISASI_ASET_ALL (
        id BIGINT IDENTITY(1,1) NOT NULL,
        periode DATE NOT NULL,                -- tanggal terakhir bulan yang ditutup
        jenis_aset NVARCHAR(40) NOT NULL,     -- kunci dataset
        kode_ue1 NVARCHAR(10) NULL,
        kode_satker NVARCHAR(40) NULL,
        nama_satker NVARCHAR(500) NULL,
        kode_register NVARCHAR(60) NULL,  -- kode register aset (tanah, gedung, rumah negara)
        kode_barang NVARCHAR(30) NULL,
        nup NVARCHAR(30) NULL,
        uraian NVARCHAR(500) NULL,
        alamat NVARCHAR(MAX) NULL,
        kelurahan_desa NVARCHAR(200) NULL,
        kecamatan NVARCHAR(200) NULL,
        kab_kota NVARCHAR(200) NULL,
        provinsi NVARCHAR(200) NULL,
        luas DECIMAL(28, 4) NULL,  -- luas tanah (jenis tanah) atau luas bangunan
        nilai DECIMAL(28, 2) NULL,
        kondisi NVARCHAR(100) NULL,
        status_hukum NVARCHAR(MAX) NULL,
        status_asuransi NVARCHAR(20) NULL,
        status_penghuni NVARCHAR(100) NULL,
        latitude DECIMAL(10, 7) NULL,
        longitude DECIMAL(10, 7) NULL,
        data_tambahan NVARCHAR(MAX) NULL,     -- JSON: kolom khusus jenis aset
        data_per DATETIME2(3) NULL,           -- data sumber per kapan (awal penyegaran datamart, UTC)
        diambil_pada DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        CONSTRAINT pk_digitalisasi_aset_all PRIMARY KEY NONCLUSTERED (id),
        CONSTRAINT ck_digitalisasi_aset_all_periode CHECK (periode = EOMONTH(periode)),
        CONSTRAINT ck_digitalisasi_aset_all_json CHECK (data_tambahan IS NULL OR ISJSON(data_tambahan) = 1)
    );

    CREATE CLUSTERED INDEX cx_digitalisasi_aset_all ON DIGITALISASI_ASET_ALL(periode, jenis_aset, id);
    CREATE INDEX idx_digitalisasi_aset_all_satker ON DIGITALISASI_ASET_ALL(jenis_aset, kode_satker, periode);
    CREATE INDEX idx_digitalisasi_aset_all_ue1 ON DIGITALISASI_ASET_ALL(kode_ue1, jenis_aset, periode);
    CREATE INDEX idx_digitalisasi_aset_all_register ON DIGITALISASI_ASET_ALL(kode_register, periode);
END;
