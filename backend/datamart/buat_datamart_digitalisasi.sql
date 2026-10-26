-- ============================================================================================================================================
-- DATAMART DIGITALISASI ASET: membentuk tujuh tabel DIGITALISASI_* dari tabel SLDK di data warehouse
--
-- FILE INI DIHASILKAN dari backend/digitalisasi (datasets.go dan queries/*.sql) oleh backend/datamart/digitalisasi.go. Jangan disunting manual: ubah sumbernya lalu jalankan
--   go test ./datamart -run TestSkripDigitalisasiSesuaiGenerator -update
-- (tes biasa gagal bila berkas ini tidak sejalan dengan aplikasi).
--
-- APA INI
--   Satu script yang, dijalankan di database data warehouse tempat tabel SLDK berada, (1) membaca tabel SLDK (DJKN.SIMAN2_*) dengan ketujuh query asli Digitalisasi Aset (disisipkan
--   apa adanya di bawah), (2) mengubah hasilnya persis seperti yang dilakukan aplikasi saat sinkronisasi (pangkas spasi, kosong menjadi NULL, potong sesuai panjang kolom, angka berkoma
--   desimal, koordinat divalidasi dan berpasangan), dan (3) membentuk dan mengisi tujuh tabel DIGITALISASI_* yang kolom dan tipenya sama dengan tabel aplikasi (migrasi 020).
--   Aplikasi lalu menarik dari tabel DIGITALISASI_* ini tanpa menyentuh SLDK.
--
-- CARA MEMAKAI (SSMS atau sqlcmd; tidak perlu mode SQLCMD)
--   1. Buka database data warehouse yang berisi tabel SLDK. Tabelnya dibaca dengan nama DJKN.SIMAN2_M_ASET dan seterusnya (daftar lengkap: bagian PEMETAAN SUMBER di bawah).
--      Database ini harus TERPISAH dari database aplikasi PASTI.
--   2. Jalankan seluruh script. Selesai: tabel DIGITALISASI_* terbentuk dan terisi. Menjalankannya lagi menyegarkan isinya; tabel tidak di-drop sehingga hak akses tabel tetap.
--   3. HANYA bila tabel SLDK tidak ada di database ini dengan nama DJKN.SIMAN2_*: ubah @Sumber (database lain di server ini, atau linked server) atau tulis pemetaan per tabel di bagian
--      PEMETAAN SUMBER. Script memeriksa SEMUA tabel sumber sebelum membuat apa pun, dan menyebut mana yang tidak terbaca beserta alasannya.
--   Bawaan (@Bersihkan = 1): setelah berhasil, fungsi, prosedur, dan sinonim sementara dibuang sehingga yang tersisa hanya TABEL (tujuh DIGITALISASI_* dan dua tabel jejak). Untuk
--   mempertahankannya agar penyegaran berikutnya cukup   EXEC dbo.datamart_digitalisasi_segarkan;   (langkah Transact-SQL biasa pada SQL Agent), ubah @Bersihkan menjadi 0.
--   Memberi hak baca kepada pengguna aplikasi:  GRANT SELECT ON dbo.DIGITALISASI_SATKER TO <pengguna>; ... (hak tabel tidak hilang saat penyegaran), serta SELECT pada dbo.datamart_digitalisasi_run.
--
-- JAMINAN
--   * Tabel SLDK hanya DIBACA. Tabel DIGITALISASI_* diisi lewat staging dan diganti dalam SATU transaksi setelah ketujuh dataset berhasil; bila ada yang gagal atau sumber
--     mengembalikan 0 baris sementara tabel sudah berisi, isi yang lama TIDAK berubah.
--   * Satu proses pada satu waktu. Jejak tiap proses: dbo.datamart_digitalisasi_run (mulai, selesai, status, pesan) dan dbo.datamart_digitalisasi_tabel (jumlah baris dan berkoordinat).
--     Kolom id_sinkron tiap baris = id proses penyegarannya. Waktu data: SELECT MAX(mulai) FROM dbo.datamart_digitalisasi_run WHERE status = N'sukses'.
--   * Script menolak berjalan di database yang berisi digitalisasi_sync_log (database aplikasi PASTI).
--   * Kolom Nama_Penghuni (DIGITALISASI_RUMAH_NEGARA) adalah data pribadi: batasi akses seperti ke database PASTI.
--   * Kolom Status_Gedung_Kantor dan Foto pada DIGITALISASI_SATKER diisi manual (query selalu mengirim NULL): nilainya dipertahankan per Kode_Satker, seperti pada aplikasi.
--   * Kesamaan dengan aplikasi dibuktikan tes (bandingan tabel demi tabel, kolom demi kolom). Batasnya: angka dengan lebih dari 15 digit penting (aplikasi membacanya sebagai float64,
--     script ini mempertahankan digitnya), bilangan bulat di atas 2^53, dan bentuk heksadesimal Go (0x1p3) pada kolom angka bertipe teks. Rincian: docs/datamart-digitalisasi.md.
--
-- Syarat: SQL Server 2017 atau lebih baru (STRING_AGG, TRIM), hak SELECT pada tabel SLDK, dan hak CREATE TABLE/SCHEMA/PROCEDURE/FUNCTION di database ini (SYNONYM hanya bila pemetaan memerlukannya).
-- ============================================================================================================================================

SET NOCOUNT ON;

DECLARE @Sumber NVARCHAR(300) = N'';   -- Awalan lokasi tabel SLDK. KOSONG (bawaan) = tabel DJKN.SIMAN2_* ada di database tempat script ini dijalankan. Isi [NamaDatabase] bila di database lain pada server ini, atau [LinkedServer].[NamaDatabase] bila di server lain.
DECLARE @Bersihkan BIT = 1;            -- 1 (bawaan) = buang fungsi/prosedur/sinonim sementara setelah berhasil, sisakan tabel saja. 0 = pertahankan (untuk EXEC dbo.datamart_digitalisasi_segarkan terjadwal).
DECLARE @sql NVARCHAR(MAX) = N'';

-- Pengaman: jangan pernah dijalankan di database aplikasi PASTI (tabel DIGITALISASI_* aplikasi akan tertimpa).
IF OBJECT_ID(N'dbo.digitalisasi_sync_log', N'U') IS NOT NULL
BEGIN
    RAISERROR (N'Database ini tampaknya database aplikasi PASTI (ada tabel digitalisasi_sync_log). Jalankan script ini di database data warehouse yang terpisah. TIDAK ADA yang diubah.', 16, 1);
    SET NOEXEC ON;   -- batch berikutnya hanya dikompilasi, tidak dijalankan (dimatikan lagi di akhir script)
END;

-- ============================================================================================================================================
-- PEMETAAN SUMBER: tempat tiap tabel/view SLDK yang dibaca ketujuh query. Bawaan: DJKN.<nama> di database ini (atau di bawah @Sumber).
-- ============================================================================================================================================
DECLARE @peta TABLE (nama SYSNAME NOT NULL PRIMARY KEY, sumber NVARCHAR(400) NOT NULL);
INSERT @peta (nama, sumber)
SELECT v.nama, CASE WHEN NULLIF(LTRIM(RTRIM(@Sumber)), N'') IS NULL THEN N'DJKN.' ELSE LTRIM(RTRIM(@Sumber)) + N'.DJKN.' END + v.nama
FROM (VALUES
    (N'SIMAN2_M_ASET'),
    (N'SIMAN2_M_ASET_HUKUM'),
    (N'SIMAN2_M_ASET_PEMAKAI'),
    (N'SIMAN2_M_ASET_PHOTO'),
    (N'SIMAN2_M_ASET_TANAH_BANGUNAN'),
    (N'SIMAN2_PENGELOLAAN'),
    (N'SIMAN2_R_KONDISI'),
    (N'SIMAN2_R_SATKER'),
    (N'SIMAN2_R_STATUS_HUKUM_NEW'),
    (N'SIMAN2_T_INV_BANGUNAN'),
    (N'VW_SIMAN_RUANG_GEDUNG_BANGUNAN_015')
) AS v(nama);

-- OPSIONAL: bila nama tabel di data warehouse BERBEDA dari SLDK (skema atau nama lain), tulis nama sebenarnya per tabel. Contoh (hapus tanda -- di depannya):
--   UPDATE @peta SET sumber = N'SLDK.SIMAN2_M_ASET' WHERE nama = N'SIMAN2_M_ASET';
--   UPDATE @peta SET sumber = N'[NamaDatabase].[skema].[NamaTabel]' WHERE nama = N'SIMAN2_R_SATKER';
-- (tulis penyesuaian pemetaan di bawah baris ini)

-- Pemeriksaan awal, SEBELUM ada yang dibuat: tiap tabel/view sumber harus dapat dibaca dari database ini.
DECLARE @hilang NVARCHAR(MAX) = N'', @nama SYSNAME, @src NVARCHAR(400);
DECLARE c_peta CURSOR LOCAL FAST_FORWARD FOR SELECT nama, sumber FROM @peta ORDER BY nama;
OPEN c_peta;
FETCH NEXT FROM c_peta INTO @nama, @src;
WHILE @@FETCH_STATUS = 0
BEGIN
    BEGIN TRY
        SET @sql = N'DECLARE @x INT; SELECT TOP (0) @x = 1 FROM ' + @src + N';';
        EXEC (@sql);
    END TRY
    BEGIN CATCH
        SET @hilang += N'  - ' + @src + N'  ->  ' + ERROR_MESSAGE() + NCHAR(10);
    END CATCH;
    FETCH NEXT FROM c_peta INTO @nama, @src;
END;
CLOSE c_peta;
DEALLOCATE c_peta;
IF @hilang <> N''
BEGIN
    SET @hilang = LEFT(N'Tabel/view sumber SLDK berikut tidak dapat dibaca dari database ini. Periksa lokasinya (ubah @Sumber atau tulis pemetaan di bagian PEMETAAN SUMBER) dan hak SELECT-nya. TIDAK ADA yang diubah.' + NCHAR(10) + @hilang, 2000);
    RAISERROR (N'%s', 16, 1, @hilang);
    SET NOEXEC ON;
END;

-- Sinonim hanya dibuat untuk tabel yang tidak berada di DJKN.<nama> database ini. Bila di DJKN sudah ada tabel/view sungguhan bernama sama, sinonim tidak dapat dibuat.
IF EXISTS (SELECT 1 FROM @peta AS p INNER JOIN sys.objects AS x ON x.schema_id = SCHEMA_ID(N'DJKN') AND x.name = p.nama AND x.type IN (N'U', N'V')
           WHERE REPLACE(REPLACE(p.sumber, N'[', N''), N']', N'') NOT IN (N'DJKN.' + p.nama, DB_NAME() + N'.DJKN.' + p.nama))
BEGIN
    RAISERROR (N'Skema DJKN di database ini sudah berisi tabel/view sungguhan dengan nama yang sama dengan tabel sumber yang dipetakan ke tempat lain (bukan sinonim), mis. dari script datamart lain. Samakan pemetaan atau pakai database yang bersih. TIDAK ADA yang diubah.', 16, 1);
    SET NOEXEC ON;
END;

-- Pilihan pembersihan, dibawa ke batch penutup lewat tabel sementara sesi.
DROP TABLE IF EXISTS #opsi;
CREATE TABLE #opsi (bersihkan BIT NOT NULL);
INSERT #opsi (bersihkan) VALUES (ISNULL(@Bersihkan, 1));

IF SCHEMA_ID(N'DJKN') IS NULL EXEC (N'CREATE SCHEMA DJKN');
IF SCHEMA_ID(N'DIGITALISASI_STG') IS NULL EXEC (N'CREATE SCHEMA DIGITALISASI_STG');

-- Jejak proses.
IF OBJECT_ID(N'dbo.datamart_digitalisasi_run', N'U') IS NULL
    CREATE TABLE dbo.datamart_digitalisasi_run (
        id      BIGINT        IDENTITY(1, 1) NOT NULL CONSTRAINT pk_datamart_digitalisasi_run PRIMARY KEY,
        mulai   DATETIME2(3)  NOT NULL CONSTRAINT df_datamart_digitalisasi_run_mulai DEFAULT SYSUTCDATETIME(),   -- UTC
        selesai DATETIME2(3)  NULL,
        status  NVARCHAR(10)  NOT NULL,                                                                          -- berjalan | sukses | gagal
        sumber  NVARCHAR(400) NULL,                                                                              -- tempat tabel SLDK yang dibaca (sinonim, atau langsung)
        pesan   NVARCHAR(2000) NULL
    );

IF OBJECT_ID(N'dbo.datamart_digitalisasi_tabel', N'U') IS NULL
    CREATE TABLE dbo.datamart_digitalisasi_tabel (
        run_id    BIGINT        NOT NULL,
        tabel     NVARCHAR(128) NOT NULL,
        baris     BIGINT        NOT NULL,
        koordinat BIGINT        NULL,   -- baris yang punya koordinat valid (tabel bergeo saja)
        CONSTRAINT pk_datamart_digitalisasi_tabel PRIMARY KEY (run_id, tabel),
        CONSTRAINT fk_datamart_digitalisasi_tabel_run FOREIGN KEY (run_id) REFERENCES dbo.datamart_digitalisasi_run (id)
    );

-- Tabel DIGITALISASI_* datamart (sama dengan migrasi 020 aplikasi); dibuat bila belum ada.
IF OBJECT_ID(N'dbo.DIGITALISASI_SATKER', N'U') IS NULL
BEGIN
    CREATE TABLE dbo.DIGITALISASI_SATKER (
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

    CREATE INDEX idx_digitalisasi_satker_kode_ue1 ON dbo.DIGITALISASI_SATKER(Kode_UE1);
    CREATE INDEX idx_digitalisasi_satker_kode_satker ON dbo.DIGITALISASI_SATKER(Kode_Satker);
    CREATE INDEX idx_digitalisasi_satker_provinsi_satker ON dbo.DIGITALISASI_SATKER(Provinsi_Satker);
END;

IF OBJECT_ID(N'dbo.DIGITALISASI_TANAH', N'U') IS NULL
BEGIN
    CREATE TABLE dbo.DIGITALISASI_TANAH (
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

    CREATE INDEX idx_digitalisasi_tanah_kode_ue1 ON dbo.DIGITALISASI_TANAH(Kode_UE1);
    CREATE INDEX idx_digitalisasi_tanah_kode_satker ON dbo.DIGITALISASI_TANAH(Kode_Satker);
    CREATE INDEX idx_digitalisasi_tanah_provinsi_tanah ON dbo.DIGITALISASI_TANAH(Provinsi_tanah);
END;

IF OBJECT_ID(N'dbo.DIGITALISASI_GEDUNG_KANTOR_UTAMA', N'U') IS NULL
BEGIN
    CREATE TABLE dbo.DIGITALISASI_GEDUNG_KANTOR_UTAMA (
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

    CREATE INDEX idx_digitalisasi_gedung_kantor_utama_kode_ue1 ON dbo.DIGITALISASI_GEDUNG_KANTOR_UTAMA(Kode_UE1);
    CREATE INDEX idx_digitalisasi_gedung_kantor_utama_kode_satker ON dbo.DIGITALISASI_GEDUNG_KANTOR_UTAMA(Kode_Satker);
    CREATE INDEX idx_digitalisasi_gedung_kantor_utama_provinsi_bangunan ON dbo.DIGITALISASI_GEDUNG_KANTOR_UTAMA(Provinsi_bangunan);
END;

IF OBJECT_ID(N'dbo.DIGITALISASI_GEDUNG_LAINNYA', N'U') IS NULL
BEGIN
    CREATE TABLE dbo.DIGITALISASI_GEDUNG_LAINNYA (
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

    CREATE INDEX idx_digitalisasi_gedung_lainnya_kode_ue1 ON dbo.DIGITALISASI_GEDUNG_LAINNYA(Kode_UE1);
    CREATE INDEX idx_digitalisasi_gedung_lainnya_kode_satker ON dbo.DIGITALISASI_GEDUNG_LAINNYA(Kode_Satker);
    CREATE INDEX idx_digitalisasi_gedung_lainnya_provinsi_bangunan ON dbo.DIGITALISASI_GEDUNG_LAINNYA(Provinsi_bangunan);
END;

IF OBJECT_ID(N'dbo.DIGITALISASI_RUSUNARA', N'U') IS NULL
BEGIN
    CREATE TABLE dbo.DIGITALISASI_RUSUNARA (
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

    CREATE INDEX idx_digitalisasi_rusunara_kode_ue1 ON dbo.DIGITALISASI_RUSUNARA(Kode_UE1);
    CREATE INDEX idx_digitalisasi_rusunara_kode_satker ON dbo.DIGITALISASI_RUSUNARA(Kode_Satker);
    CREATE INDEX idx_digitalisasi_rusunara_provinsi_rusun ON dbo.DIGITALISASI_RUSUNARA(Provinsi_Rusun);
END;

IF OBJECT_ID(N'dbo.DIGITALISASI_RUMAH_NEGARA', N'U') IS NULL
BEGIN
    CREATE TABLE dbo.DIGITALISASI_RUMAH_NEGARA (
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
        Nama_Penghuni NVARCHAR(500) NULL,  -- data pribadi (nama penghuni): batasi akses ke datamart
        Kondisi_RN NVARCHAR(100) NULL,
        Status_Hukum NVARCHAR(MAX) NULL,
        Latitude DECIMAL(10, 7) NULL,
        Longitude DECIMAL(10, 7) NULL,
        id_sinkron BIGINT NULL,
        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_digitalisasi_rumah_negara_kode_ue1 ON dbo.DIGITALISASI_RUMAH_NEGARA(Kode_UE1);
    CREATE INDEX idx_digitalisasi_rumah_negara_kode_satker ON dbo.DIGITALISASI_RUMAH_NEGARA(Kode_Satker);
    CREATE INDEX idx_digitalisasi_rumah_negara_provinsi_rn ON dbo.DIGITALISASI_RUMAH_NEGARA(Provinsi_RN);
END;

IF OBJECT_ID(N'dbo.DIGITALISASI_MESS_RUMAH_NEGARA', N'U') IS NULL
BEGIN
    CREATE TABLE dbo.DIGITALISASI_MESS_RUMAH_NEGARA (
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

    CREATE INDEX idx_digitalisasi_mess_rumah_negara_kode_ue1 ON dbo.DIGITALISASI_MESS_RUMAH_NEGARA(Kode_UE1);
    CREATE INDEX idx_digitalisasi_mess_rumah_negara_kode_satker ON dbo.DIGITALISASI_MESS_RUMAH_NEGARA(Kode_Satker);
    CREATE INDEX idx_digitalisasi_mess_rumah_negara_provinsi_mess ON dbo.DIGITALISASI_MESS_RUMAH_NEGARA(Provinsi_mess);
END;


-- Sinonim DJKN.<nama> ke tempat tabel SLDK yang sebenarnya, HANYA untuk tabel yang tidak berada di DJKN.<nama> database ini (lihat PEMETAAN SUMBER). Query asli di bawah memakai nama
-- DJKN.SIMAN2_* apa adanya, sehingga tidak perlu diubah satu karakter pun. Bila semua tabel sudah berada di DJKN database ini, tidak ada sinonim.
SET @sql = N'';
SELECT @sql += N'DROP SYNONYM DJKN.' + QUOTENAME(s.name) + N';' FROM sys.synonyms AS s WHERE s.schema_id = SCHEMA_ID(N'DJKN') AND s.name IN (SELECT nama FROM @peta);
EXEC (@sql);
SET @sql = N'';
SELECT @sql += N'CREATE SYNONYM DJKN.' + QUOTENAME(nama) + N' FOR ' + sumber + N';' FROM @peta
WHERE REPLACE(REPLACE(sumber, N'[', N''), N']', N'') NOT IN (N'DJKN.' + nama, DB_NAME() + N'.DJKN.' + nama);
EXEC (@sql);
GO

-- ============================================================================================================================================
-- FUNGSI KONVERSI: padanan digitalisasi.convertValue / parseNumber / fixCoordinates pada aplikasi (backend/digitalisasi/convert.go)
-- Pola LIKE/PATINDEX dan TRIM memakai COLLATE Latin1_General_BIN2 (urutan kode): collation bawaan database tidak boleh memengaruhi hasil.
-- ============================================================================================================================================

-- strings.TrimSpace (Go): U+0009-000D, U+0020, U+0085, U+00A0, U+1680, U+2000-200A, U+2028, U+2029, U+202F, U+205F, U+3000.
CREATE OR ALTER FUNCTION dbo.dm_pangkas (@v NVARCHAR(MAX))
RETURNS NVARCHAR(MAX)
AS
BEGIN
    RETURN TRIM(NCHAR(9) + NCHAR(10) + NCHAR(11) + NCHAR(12) + NCHAR(13) + NCHAR(32) + NCHAR(133) + NCHAR(160) + NCHAR(5760)
              + NCHAR(8192) + NCHAR(8193) + NCHAR(8194) + NCHAR(8195) + NCHAR(8196) + NCHAR(8197) + NCHAR(8198) + NCHAR(8199) + NCHAR(8200) + NCHAR(8201) + NCHAR(8202)
              + NCHAR(8232) + NCHAR(8233) + NCHAR(8239) + NCHAR(8287) + NCHAR(12288) FROM @v COLLATE Latin1_General_BIN2);
END;
GO

-- Kolom teks: dipangkas, kosong menjadi NULL, dipotong ke @maks unit UTF-16 (0 = tanpa batas) tanpa memotong pasangan pengganti (surrogate) di tengah:
-- bila unit terakhir hasil potongan (posisi @maks) adalah separuh pertama sebuah pasangan, ia dibuang juga.
-- Ditulis sebagai satu ekspresi supaya SQL Server 2019+ dapat menyisipkannya ke query (jauh lebih cepat daripada panggilan fungsi per baris).
CREATE OR ALTER FUNCTION dbo.dm_teks (@v NVARCHAR(MAX), @maks INT)
RETURNS NVARCHAR(MAX)
AS
BEGIN
    RETURN (SELECT CASE
                WHEN x.s IS NULL OR DATALENGTH(x.s) = 0 THEN NULL
                WHEN @maks > 0 AND DATALENGTH(x.s) > @maks * 2
                    THEN CASE WHEN UNICODE(SUBSTRING(x.s, @maks, 1)) BETWEEN 55296 AND 56319 THEN SUBSTRING(x.s, 1, @maks - 1) ELSE SUBSTRING(x.s, 1, @maks) END
                ELSE x.s
            END
            FROM (SELECT dbo.dm_pangkas(@v) AS s) AS x);
END;
GO

-- parseNumber (Go), langkah 1, jalur lengkap: bentuk "polos" sebuah angka dalam teks = tanda minus (bila negatif), digit, dan paling banyak satu titik desimal, tanpa eksponen
-- ("-1.5E+3" -> "-1500", " 0,00005 " -> "0.00005"). NULL bila bukan angka menurut strconv.ParseFloat: tanda +/-, digit, satu titik, eksponen e/E, garis bawah di antara dua digit,
-- dan (aturan aplikasi) satu koma menjadi titik desimal bila tidak ada titik. Tidak dikenali (NULL): bentuk heksadesimal (0x1p3), Inf/NaN, dan angka di atas 10^400.
CREATE OR ALTER FUNCTION dbo.dm_polos_lambat (@v NVARCHAR(MAX))
RETURNS NVARCHAR(MAX)
AS
BEGIN
    DECLARE @t NVARCHAR(MAX) = dbo.dm_pangkas(@v), @tanda NVARCHAR(1) = N'', @m NVARCHAR(MAX), @x NVARCHAR(MAX) = N'', @pe INT, @titik INT, @digit NVARCHAR(MAX), @geser INT = 0, @pos INT, @xs INT = 1;

    IF @t IS NULL OR DATALENGTH(@t) = 0 OR DATALENGTH(@t) > 800 RETURN NULL;
    IF CHARINDEX(N',', @t) > 0 AND CHARINDEX(N'.', @t) = 0 SET @t = STUFF(@t, CHARINDEX(N',', @t), 1, N'.');

    IF CHARINDEX(N'_', @t) > 0
    BEGIN
        -- garis bawah hanya sah tepat di antara dua digit (strconv.underscoreOK)
        IF @t COLLATE Latin1_General_BIN2 LIKE N'%[^0-9]\_%' ESCAPE N'\' OR @t COLLATE Latin1_General_BIN2 LIKE N'\_%' ESCAPE N'\'
           OR @t COLLATE Latin1_General_BIN2 LIKE N'%\_[^0-9]%' ESCAPE N'\' OR @t COLLATE Latin1_General_BIN2 LIKE N'%\_' ESCAPE N'\'
            RETURN NULL;
        SET @t = REPLACE(@t, N'_', N'');
    END;

    IF UNICODE(LEFT(@t, 1)) IN (43, 45)
    BEGIN
        IF UNICODE(LEFT(@t, 1)) = 45 SET @tanda = N'-';
        SET @t = SUBSTRING(@t, 2, 4000);
    END;

    SET @pe = PATINDEX(N'%[eE]%' COLLATE Latin1_General_BIN2, @t);
    IF @pe > 0
    BEGIN
        SET @m = LEFT(@t, @pe - 1);
        SET @x = SUBSTRING(@t, @pe + 1, 4000);
    END
    ELSE SET @m = @t;

    -- mantisa: hanya digit dan paling banyak satu titik, minimal satu digit
    IF PATINDEX(N'%[^0-9.]%' COLLATE Latin1_General_BIN2, @m) > 0 OR PATINDEX(N'%[0-9]%' COLLATE Latin1_General_BIN2, @m) = 0 RETURN NULL;
    SET @titik = CHARINDEX(N'.', @m);
    IF @titik > 0 AND CHARINDEX(N'.', @m, @titik + 1) > 0 RETURN NULL;

    -- eksponen: tanda opsional lalu 1-3 digit
    IF @pe > 0
    BEGIN
        IF UNICODE(LEFT(@x, 1)) IN (43, 45)
        BEGIN
            IF UNICODE(LEFT(@x, 1)) = 45 SET @xs = -1;
            SET @x = SUBSTRING(@x, 2, 4000);
        END;
        IF DATALENGTH(@x) = 0 OR DATALENGTH(@x) > 6 OR PATINDEX(N'%[^0-9]%' COLLATE Latin1_General_BIN2, @x) > 0 RETURN NULL;
        SET @geser = @xs * CAST(@x AS INT);
    END;

    -- geser titik desimal sebanyak eksponen
    SET @digit = REPLACE(@m, N'.', N'');
    SET @pos = CASE WHEN @titik > 0 THEN @titik - 1 ELSE LEN(@digit) END + @geser;
    IF @pos > 400 RETURN NULL;   -- di atas 10^400 (melampaui FLOAT)
    IF @pos < -323 RETURN N'0';   -- di bawah bilangan FLOAT terkecil (sekitar 5E-324): strconv.ParseFloat membulatkannya ke 0
    RETURN @tanda + CASE
        WHEN @pos <= 0 THEN N'0.' + REPLICATE(N'0', -@pos) + @digit
        WHEN @pos >= LEN(@digit) THEN @digit + REPLICATE(N'0', @pos - LEN(@digit))
        ELSE LEFT(@digit, @pos) + N'.' + SUBSTRING(@digit, @pos + 1, 4000)
    END;
END;
GO

-- parseNumber (Go), langkah 1: bentuk "polos" angka (hasil sama dengan dm_polos_lambat). Bentuk yang paling umum, [-]digit[.digit] atau dengan satu koma desimal, langsung dikembalikan
-- dalam satu ekspresi bertipe terbatas (dapat disisipkan ke query oleh SQL Server 2019+, sekitar 2x lebih cepat daripada jalur lengkap); bentuk lain (eksponen, tanda +, garis bawah, dan
-- sebagainya) serta teks panjang lewat dm_polos_lambat.
CREATE OR ALTER FUNCTION dbo.dm_pangkas_pendek (@v NVARCHAR(100))
RETURNS NVARCHAR(100)
AS
BEGIN
    RETURN TRIM(NCHAR(9) + NCHAR(10) + NCHAR(11) + NCHAR(12) + NCHAR(13) + NCHAR(32) + NCHAR(133) + NCHAR(160) + NCHAR(5760)
              + NCHAR(8192) + NCHAR(8193) + NCHAR(8194) + NCHAR(8195) + NCHAR(8196) + NCHAR(8197) + NCHAR(8198) + NCHAR(8199) + NCHAR(8200) + NCHAR(8201) + NCHAR(8202)
              + NCHAR(8232) + NCHAR(8233) + NCHAR(8239) + NCHAR(8287) + NCHAR(12288) FROM @v COLLATE Latin1_General_BIN2);
END;
GO

CREATE OR ALTER FUNCTION dbo.dm_polos (@v NVARCHAR(MAX))
RETURNS NVARCHAR(MAX)
AS
BEGIN
    RETURN (SELECT CASE
                WHEN @v IS NULL THEN NULL
                WHEN DATALENGTH(@v) > 200 THEN dbo.dm_polos_lambat(@v)
                WHEN x.s IS NULL OR DATALENGTH(x.s) = 0 THEN NULL
                WHEN DATALENGTH(x.w) >= 2
                     AND (x.w LIKE N'[0-9]' OR x.w LIKE N'[0-9]%[0-9]')
                     AND x.w NOT LIKE N'%[^0-9.]%'
                     AND LEN(x.w) - LEN(REPLACE(x.w, N'.', N'')) <= 1
                    THEN CAST(x.tanda + x.w AS NVARCHAR(MAX))
                ELSE dbo.dm_polos_lambat(@v)
            END
            FROM (SELECT q2.s, q2.tanda, w = CAST(CASE WHEN CHARINDEX(N'.', q2.u) = 0 THEN REPLACE(q2.u, N',', N'.') ELSE q2.u END AS NVARCHAR(100)) COLLATE Latin1_General_BIN2
                  FROM (SELECT q1.s,
                               tanda = CAST(CASE WHEN LEFT(q1.s, 1) COLLATE Latin1_General_BIN2 = N'-' THEN N'-' ELSE N'' END AS NVARCHAR(1)),
                               u = CAST(CASE WHEN LEFT(q1.s, 1) COLLATE Latin1_General_BIN2 = N'-' THEN SUBSTRING(q1.s, 2, 100) ELSE q1.s END AS NVARCHAR(100))
                        FROM (SELECT dbo.dm_pangkas_pendek(CAST(@v AS NVARCHAR(100))) AS s) AS q1) AS q2) AS x);
END;
GO

-- parseNumber (Go): angka (FLOAT) dari teks; NULL bila bukan angka.
CREATE OR ALTER FUNCTION dbo.dm_angka (@v NVARCHAR(MAX))
RETURNS FLOAT
AS
BEGIN
    RETURN TRY_CAST(dbo.dm_polos(@v) AS FLOAT);
END;
GO

-- Kolom INT/BIGINT: bilangan bulat; bukan angka atau berpecahan -> NULL.
CREATE OR ALTER FUNCTION dbo.dm_bilangan (@v NVARCHAR(MAX))
RETURNS BIGINT
AS
BEGIN
    RETURN (SELECT CASE
                WHEN x.p IS NULL THEN NULL
                WHEN x.t = 0 THEN TRY_CAST(x.p AS BIGINT)
                WHEN SUBSTRING(x.p, x.t + 1, 4000) COLLATE Latin1_General_BIN2 LIKE N'%[^0]%' THEN NULL
                ELSE TRY_CAST(LEFT(x.p, x.t - 1) AS BIGINT)
            END
            FROM (SELECT q.p, t = CHARINDEX(N'.', q.p) FROM (SELECT dbo.dm_polos(@v) AS p) AS q) AS x);
END;
GO

-- Kolom DECIMAL(28, @skala): nilai angka dibulatkan SEKALI ke @skala (0-6) dari digit aslinya, seperti aplikasi yang mengirim teks angka ke kolom DECIMAL.
CREATE OR ALTER FUNCTION dbo.dm_desimal (@v NVARCHAR(MAX), @skala INT)
RETURNS DECIMAL(38, 6)
AS
BEGIN
    RETURN (SELECT CASE
                WHEN x.p IS NULL THEN NULL
                WHEN @skala = 0 THEN TRY_CAST(TRY_CAST(x.p AS DECIMAL(38, 0)) AS DECIMAL(38, 6))
                WHEN @skala = 1 THEN TRY_CAST(TRY_CAST(x.p AS DECIMAL(38, 1)) AS DECIMAL(38, 6))
                WHEN @skala = 2 THEN TRY_CAST(TRY_CAST(x.p AS DECIMAL(38, 2)) AS DECIMAL(38, 6))
                WHEN @skala = 3 THEN TRY_CAST(TRY_CAST(x.p AS DECIMAL(38, 3)) AS DECIMAL(38, 6))
                WHEN @skala = 4 THEN TRY_CAST(TRY_CAST(x.p AS DECIMAL(38, 4)) AS DECIMAL(38, 6))
                WHEN @skala = 5 THEN TRY_CAST(TRY_CAST(x.p AS DECIMAL(38, 5)) AS DECIMAL(38, 6))
                WHEN @skala = 6 THEN TRY_CAST(x.p AS DECIMAL(38, 6))
            END
            FROM (SELECT dbo.dm_polos(@v) AS p) AS x);
END;
GO

-- Kolom sumber bertipe FLOAT/REAL dibaca aplikasi sebagai bilangan (REAL diperlebar ke FLOAT), lalu ditulis sebagai angka terpendek yang kembali ke bilangan yang sama
-- (strconv.FormatFloat 'f', -1). Padanannya: format "R" (budaya tetap en-US), yang mencoba 15 digit lalu langsung 17 digit; Go juga mencoba 16 digit, jadi 16 digit dicoba di sini.
CREATE OR ALTER FUNCTION dbo.dm_float_ke_teks (@f FLOAT)
RETURNS NVARCHAR(60)
AS
BEGIN
    DECLARE @r NVARCHAR(60) = FORMAT(@f, N'R', N'en-US'), @d NVARCHAR(60), @s2 VARCHAR(40);
    IF @r IS NULL RETURN NULL;
    -- banyaknya digit penting pada format "R"
    SET @d = @r COLLATE Latin1_General_BIN2;
    IF CHARINDEX(N'E', @d) > 0 SET @d = LEFT(@d, CHARINDEX(N'E', @d) - 1);
    SET @d = REPLACE(REPLACE(@d, N'-', N''), N'.', N'');
    SET @d = SUBSTRING(@d, PATINDEX(N'%[^0]%' COLLATE Latin1_General_BIN2, @d + N'1'), 60);
    IF LEN(@d) > 15
    BEGIN
        SET @s2 = CONVERT(VARCHAR(40), @f, 2);   -- tepat 16 digit
        IF TRY_CAST(@s2 AS FLOAT) = @f RETURN @s2;
    END;
    RETURN @r;
END;
GO

-- Koordinat: angka dengan |nilai| <= @batas (90 untuk lintang, 180 untuk bujur); selain itu NULL. Kolom FLOAT (@flt) dipakai apa adanya, selain itu teks (@teks) dibaca.
-- Kepasangan dan (0,0) diperiksa pada INSERT.
CREATE OR ALTER FUNCTION dbo.dm_koordinat (@teks NVARCHAR(MAX), @flt FLOAT, @batas FLOAT)
RETURNS FLOAT
AS
BEGIN
    RETURN (SELECT CASE WHEN x.f IS NULL OR ABS(x.f) > @batas THEN NULL ELSE x.f END
            FROM (SELECT CASE WHEN @flt IS NOT NULL THEN @flt ELSE dbo.dm_angka(@teks) END AS f) AS x);
END;
GO

-- ============================================================================================================================================
-- QUERY ASLI PER DATASET + KONVERSI
-- ============================================================================================================================================

-- DIGITALISASI_SATKER: Satker aktif KL 015 (anak satker pada UE1 tertentu digabung ke induk) beserta jumlah kendaraan dinas
CREATE OR ALTER PROCEDURE dbo.datamart_dg_muat_satker @run BIGINT
AS
BEGIN
    SET NOCOUNT ON;

-- ---------------------------------------------------------------------------------------------------
-- QUERY ASLI (queries/satker.sql). Satu-satunya tambahan: INTO #raw_satker pada SELECT akhir.
-- ---------------------------------------------------------------------------------------------------
-- ============================================================
-- REVISI 5: satker.sql (fix duplikat + kolom Jenis_Satker)
-- Tambahan:
--   Kolom Jenis_Satker berdasarkan SUBSTRING(kd_satker,16,3)
--   '000' = INDUK SATKER, selain itu = ANAK SATKER
-- ============================================================

WITH SatkerExclude AS (
    SELECT kd_satker FROM (VALUES
        (N'015010199409294002KP'),
        (N'015010199673102000KP'),
        (N'015010199409294001KP')
    ) AS X(kd_satker)
),
SatkerMergeRule AS (
    SELECT
        prefix,
        prefix + N'000KP' AS induk_kd_satker
    FROM (VALUES
        (N'015040199119091'),   -- UE1 DJP
        (N'015050199410640'),   -- UE1 DJBC
        (N'015080199527010'),   -- UE1 DJPB
        (N'015150199691273')    -- UE1 BATII
    ) AS X(prefix)
),
SatkerBase AS (
    -- Dedup baris mentah per kd_satker (jaga-jaga ada duplikat di sumber,
    -- karena tabel SIMAN2_R_SATKER adalah heap tanpa PK)
    SELECT *
    FROM (
        SELECT
            S.*,
            ROW_NUMBER() OVER (
                PARTITION BY S.kd_satker
                ORDER BY S.updated_at DESC, S.id_satker DESC
            ) AS rn
        FROM DJKN.SIMAN2_R_SATKER AS S
        WHERE S.status_uakpb = N'AKTIF'
          AND S.kd_satker LIKE N'015%'
    ) AS X
    WHERE X.rn = 1
),
SatkerAktif AS (
    -- DISTINCT wajib: tanpa ini, satu satker induk akan muncul
    -- berkali-kali (sebanyak jumlah anak satker yang di-merge)
    SELECT DISTINCT
        COALESCE(INDUK.kd_satker, S.kd_satker) AS kd_satker,
        COALESCE(INDUK.ur_satker, S.ur_satker) AS ur_satker,
        COALESCE(INDUK.alamat_satker, S.alamat_satker) AS alamat_satker,
        COALESCE(INDUK.nm_kel, S.nm_kel) AS nm_kel,
        COALESCE(INDUK.nm_kec, S.nm_kec) AS nm_kec,
        COALESCE(INDUK.nm_kab_kota, S.nm_kab_kota) AS nm_kab_kota,
        COALESCE(INDUK.nm_prov, S.nm_prov) AS nm_prov
    FROM SatkerBase AS S
    LEFT JOIN SatkerMergeRule AS M
           ON S.kd_satker LIKE M.prefix + N'%'
          AND S.kd_satker <> M.induk_kd_satker
    LEFT JOIN SatkerBase AS INDUK
           ON INDUK.kd_satker = M.induk_kd_satker
          AND M.prefix IS NOT NULL
    WHERE NOT EXISTS (
          SELECT 1 FROM SatkerExclude AS E
          WHERE E.kd_satker = S.kd_satker
      )
),
KdBrgKendaraan AS (
    SELECT kd_brg, is_sedan_only FROM (VALUES
        (N'3020101001', 1),  -- Sedan
        (N'3020101002', 0),  -- Jeep
        (N'3020101003', 0),  -- Station Wagon
        (N'3020101004', 1),  -- Sedan Listrik
        (N'3020101005', 0),  -- SUV Listrik
        (N'3020101006', 0),  -- MPV Listrik
        (N'3020101009', 0),  -- Mini Bus - Listrik (Penumpang 14 Orang Kebawah)
        (N'3020102003', 0),  -- Mini Bus (Penumpang 14 Orang Kebawah)
        (N'3020102007', 0),  -- Mini Bus - Listrik (Penumpang 14 Orang Kebawah)
        (N'3020105133', 0)   -- Mobil Listrik
    ) AS X(kd_brg, is_sedan_only)
),
KendaraanRaw AS (
    SELECT
        A.kd_satker,
        A.kd_status,
        ISNULL(K.kd_kondisi, '9') AS kd_kondisi,
        ISNULL(K.ur_kondisi, '-') AS ur_kondisi,
        COUNT(*) AS jml
    FROM DJKN.SIMAN2_M_ASET AS A
    INNER JOIN SatkerAktif AS S
            ON S.kd_satker = A.kd_satker
    INNER JOIN KdBrgKendaraan AS KB
            ON KB.kd_brg = A.kd_brg
    LEFT JOIN DJKN.SIMAN2_R_KONDISI AS K
           ON K.kd_kondisi = A.kd_kondisi
    WHERE A.status_bmn_yn = N'Y'
      AND (
            A.kd_status = '01'
         OR (A.kd_status = '02' AND KB.is_sedan_only = 0)
          )
    GROUP BY
        A.kd_satker,
        A.kd_status,
        ISNULL(K.kd_kondisi, '9'),
        ISNULL(K.ur_kondisi, '-')
),
KendaraanStats AS (
    SELECT
        KR.kd_satker,
        SUM(CASE WHEN KR.kd_status = '01' THEN KR.jml ELSE 0 END) AS Jumlah_KDJ,
        STRING_AGG(
            CASE WHEN KR.kd_status = '01'
                 THEN KR.ur_kondisi + N' (' + CAST(KR.jml AS nvarchar(10)) + N')'
            END,
            N', '
        ) WITHIN GROUP (ORDER BY KR.kd_status, KR.kd_kondisi) AS Kondisi_KDJ,
        SUM(CASE WHEN KR.kd_status = '02' THEN KR.jml ELSE 0 END) AS Jumlah_KDO,
        STRING_AGG(
            CASE WHEN KR.kd_status = '02'
                 THEN KR.ur_kondisi + N' (' + CAST(KR.jml AS nvarchar(10)) + N')'
            END,
            N', '
        ) WITHIN GROUP (ORDER BY KR.kd_status, KR.kd_kondisi) AS Kondisi_KDO
    FROM KendaraanRaw AS KR
    GROUP BY KR.kd_satker
)
SELECT
    LEFT(S.kd_satker, 5)          AS Kode_UE1,
    S.kd_satker                   AS Kode_Satker,
    CASE
        WHEN SUBSTRING(S.kd_satker, 16, 3) = N'000' THEN N'INDUK SATKER'
        ELSE N'ANAK SATKER'
    END                            AS Jenis_Satker,
    S.ur_satker                   AS Nama_Satker,
    S.alamat_satker               AS Alamat_Satker,
    S.nm_kel                      AS KelurahanDesa_Satker,
    S.nm_kec                      AS Kecamatan_Satker,
    S.nm_kab_kota                 AS KabKota_Satker,
    S.nm_prov                     AS Provinsi_Satker,
    CAST(NULL AS varchar(200))    AS Status_Gedung_Kantor,
    CAST(NULL AS varchar(900))    AS Foto,
    ISNULL(KS.Jumlah_KDJ, 0)      AS Jumlah_KDJ,
    KS.Kondisi_KDJ                AS Kondisi_KDJ,
    ISNULL(KS.Jumlah_KDO, 0)      AS Jumlah_KDO,
    KS.Kondisi_KDO                AS Kondisi_KDO
INTO #raw_satker
FROM SatkerAktif AS S
LEFT JOIN KendaraanStats AS KS
       ON KS.kd_satker = S.kd_satker;

-- ---------------------------------------------------------------------------------------------------
-- KONVERSI ke kolom DIGITALISASI_SATKER (padanan digitalisasi.convertValue dan fixCoordinates)
-- ---------------------------------------------------------------------------------------------------
-- Tipe kolom mentah (system_type_id): kolom FLOAT/REAL (62, 59) dibaca sebagai bilangan, bukan sebagai teks.
DECLARE @jenis TABLE (kolom SYSNAME NOT NULL PRIMARY KEY, tipe INT NOT NULL);
INSERT @jenis (kolom, tipe) SELECT c.name, c.system_type_id FROM tempdb.sys.columns AS c WHERE c.object_id = OBJECT_ID(N'tempdb..#raw_satker');
DECLARE @tp_11 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Jumlah_KDJ'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_13 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Jumlah_KDO'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @salah NVARCHAR(MAX) = (SELECT STRING_AGG(kolom, N', ') FROM @jenis WHERE kolom IN (N'Kode_UE1', N'Kode_Satker', N'Jenis_Satker', N'Nama_Satker', N'Alamat_Satker', N'KelurahanDesa_Satker', N'Kecamatan_Satker', N'KabKota_Satker', N'Provinsi_Satker', N'Status_Gedung_Kantor', N'Foto', N'Kondisi_KDJ', N'Kondisi_KDO') AND tipe NOT IN (35, 48, 52, 56, 99, 104, 106, 108, 127, 167, 175, 231, 239));
IF @salah IS NOT NULL
BEGIN
    SET @salah = N'DIGITALISASI_SATKER: kolom teks dari SLDK bertipe FLOAT, tanggal, uang, atau tipe lain yang bentuk tekstualnya berbeda antara aplikasi dan SQL Server (' + @salah + N'). Ubah ke teks pada query sumber. TIDAK ADA yang diubah.';
    THROW 50050, @salah, 1;
END;

INSERT INTO DIGITALISASI_STG.DIGITALISASI_SATKER
    ([Kode_UE1], [Kode_Satker], [Jenis_Satker], [Nama_Satker], [Alamat_Satker], [KelurahanDesa_Satker], [Kecamatan_Satker], [KabKota_Satker], [Provinsi_Satker], [Status_Gedung_Kantor], [Foto], [Jumlah_KDJ], [Kondisi_KDJ], [Jumlah_KDO], [Kondisi_KDO], [id_sinkron])
SELECT
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_UE1]), 10), -- Kode_UE1
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_Satker]), 40), -- Kode_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Jenis_Satker]), 20), -- Jenis_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Nama_Satker]), 500), -- Nama_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Alamat_Satker]), 0), -- Alamat_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[KelurahanDesa_Satker]), 200), -- KelurahanDesa_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kecamatan_Satker]), 200), -- Kecamatan_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[KabKota_Satker]), 200), -- KabKota_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Provinsi_Satker]), 200), -- Provinsi_Satker
    COALESCE(dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Status_Gedung_Kantor]), 200), (SELECT TOP (1) o.[Status_Gedung_Kantor] FROM dbo.DIGITALISASI_SATKER AS o WHERE o.[Kode_Satker] = dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_Satker]), 40) ORDER BY o.id DESC)), -- Status_Gedung_Kantor
    COALESCE(dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Foto]), 900), (SELECT TOP (1) o.[Foto] FROM dbo.DIGITALISASI_SATKER AS o WHERE o.[Kode_Satker] = dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_Satker]), 40) ORDER BY o.id DESC)), -- Foto
    dbo.dm_bilangan(CASE WHEN @tp_11 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Jumlah_KDJ] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Jumlah_KDJ], 3) END), -- Jumlah_KDJ
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kondisi_KDJ]), 0), -- Kondisi_KDJ
    dbo.dm_bilangan(CASE WHEN @tp_13 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Jumlah_KDO] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Jumlah_KDO], 3) END), -- Jumlah_KDO
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kondisi_KDO]), 0), -- Kondisi_KDO
    @run -- id_sinkron
FROM #raw_satker AS r
;

DROP TABLE IF EXISTS #raw_satker;
END;
GO

-- DIGITALISASI_TANAH: Aset tanah (kd_brg 2%) KL 015 beserta status hukum, jumlah bangunan di atasnya, dan koordinat
CREATE OR ALTER PROCEDURE dbo.datamart_dg_muat_tanah @run BIGINT
AS
BEGIN
    SET NOCOUNT ON;

-- ---------------------------------------------------------------------------------------------------
-- QUERY ASLI (queries/tanah.sql). Satu-satunya tambahan: INTO #raw_tanah pada SELECT akhir.
-- ---------------------------------------------------------------------------------------------------
-- Tanah (kd_brg 2%). Query asli dari pengelola data.
-- Baris bertanda [PASTI] ditambahkan agar koordinat ikut tersinkron untuk peta; selebihnya tidak diubah.
SET NOCOUNT ON;

-- 1) Hitung TGL_TARIK terbaru SEKALI SAJA, bukan sebagai correlated subquery
DECLARE @MaxTglTarik int =
    (SELECT MAX(TGL_TARIK) FROM DJKN.SIMAN2_R_STATUS_HUKUM_NEW);

-- 2) Materialisasi mapping status hukum (tabel kecil, cepat)
SELECT
    SH.kd_status_hukum,
    STRING_AGG(
        CONVERT(nvarchar(max), SH.ur_status_hukum)
        + N' [' + CONVERT(nvarchar(max), SH.label) + N']',
        N' | '
    ) WITHIN GROUP (ORDER BY SH.id_status_hukum) AS ur_status_hukum
INTO #StatusHukumMap
FROM DJKN.SIMAN2_R_STATUS_HUKUM_NEW AS SH
WHERE SH.TGL_TARIK = @MaxTglTarik
GROUP BY SH.kd_status_hukum;

CREATE UNIQUE CLUSTERED INDEX IX_temp_SH ON #StatusHukumMap (kd_status_hukum);

-- 3) Materialisasi AsetTarget SEKALI SAJA — ini kunci utamanya
SELECT
    A.id_aset,
    A.kode_register,
    A.kd_satker,
    S.ur_satker,
    A.kd_brg,
    A.ur_sskel,
    A.no_aset,
    ISNULL(A.alamat, S.alamat_satker) AS alamat,
    A.kd_rtrw,
    ISNULL(A.ur_kel, S.nm_kel) AS ur_kel,
    ISNULL(A.ur_kec, S.nm_kec) AS ur_kec,
    ISNULL(A.ur_kab, S.nm_kab_kota) AS ur_kab,
    ISNULL(A.ur_prov, S.nm_prov) AS ur_prov,
    A.luas,
    A.luas_tnhb,
    SH.ur_status_hukum,
    K.ur_kondisi,
    A.rph_aset,
    A.gps_latitude,    -- [PASTI] koordinat untuk peta
    A.gps_longitude    -- [PASTI] koordinat untuk peta
INTO #AsetTarget
FROM DJKN.SIMAN2_M_ASET AS A
INNER JOIN DJKN.SIMAN2_R_SATKER AS S
        ON S.kd_satker = A.kd_satker
LEFT JOIN #StatusHukumMap AS SH
       ON SH.kd_status_hukum = A.kd_status_hukum
LEFT JOIN DJKN.SIMAN2_R_KONDISI AS K
       ON K.kd_kondisi = A.kd_kondisi
WHERE A.status_bmn_yn = N'Y'
  AND A.kd_brg LIKE N'2%'
  AND A.kd_satker LIKE N'015%'
  AND S.status_uakpb = N'AKTIF';

-- Ini index buatan sendiri di tempdb — TIDAK butuh izin apa pun di tabel asli
CREATE UNIQUE CLUSTERED INDEX IX_temp_Aset ON #AsetTarget (id_aset);

-- 4) FotoRanked — sekarang join ke tabel kecil (#AsetTarget), bukan ke keseluruhan AsetTarget yang di-inline ulang
SELECT
    P.id_aset,
    COALESCE(
        NULLIF(P.folder, N''),
        NULLIF(P.filename, N''),
        NULLIF(P.nm_photo, N'')
    ) AS folder,
    ROW_NUMBER() OVER (
        PARTITION BY P.id_aset
        ORDER BY P.photo_utama_yn DESC, P.tanggal DESC, P.id_photo DESC
    ) AS rn
INTO #FotoRanked
FROM DJKN.SIMAN2_M_ASET_PHOTO AS P
INNER JOIN #AsetTarget AS T
        ON T.id_aset = P.id_aset
WHERE P.status_data = 1;

CREATE CLUSTERED INDEX IX_temp_Foto ON #FotoRanked (id_aset, rn);

-- 5) BangunanAgg
SELECT
    TB.id_aset_tanah,
    COUNT(*) AS jumlah
INTO #BangunanAgg
FROM DJKN.SIMAN2_M_ASET_TANAH_BANGUNAN AS TB
INNER JOIN #AsetTarget AS T
        ON T.id_aset = TB.id_aset_tanah
INNER JOIN DJKN.SIMAN2_M_ASET AS B
        ON B.id_aset = TB.id_aset_bangunan
       AND B.status_bmn_yn = N'Y'
GROUP BY TB.id_aset_tanah;

CREATE UNIQUE CLUSTERED INDEX IX_temp_Bgn ON #BangunanAgg (id_aset_tanah);

SELECT
    T.id_aset                         AS id_aset_tanah,
    T.kode_register                   AS kode_register_tanah,
    LEFT(T.kd_satker, 5)              AS Kode_UE1,
    T.kd_satker                       AS Kode_Satker,
    T.ur_satker                       AS Nama_Satker,
    T.kd_brg                          AS Kode_tanah,
    T.ur_sskel                        AS Uraian_tanah,
    T.no_aset                         AS No_aset,
    T.alamat                          AS Alamat_tanah,
    T.kd_rtrw                         AS RTRW_tanah,
    T.ur_kel                          AS KelurahanDesa_tanah,
    T.ur_kec                          AS Kecamatan_tanah,
    T.ur_kab                          AS KabKota_tanah,
    T.ur_prov                         AS Provinsi_tanah,
    T.luas                            AS Luas_Tanah,
    T.luas_tnhb                       AS Luas_Bangunan,
    ISNULL(BG.jumlah, 0)              AS Jumlah_Bangunan,
    T.ur_status_hukum                 AS Status_hukum,
    F.folder                          AS Foto,
    T.ur_kondisi                      AS Kondisi_Tanah,
    T.rph_aset                        AS Nilai_Tanah,
    T.gps_latitude                    AS GPS_Latitude,    -- [PASTI] koordinat untuk peta
    T.gps_longitude                   AS GPS_Longitude    -- [PASTI] koordinat untuk peta
INTO #raw_tanah
FROM #AsetTarget AS T
LEFT JOIN #FotoRanked AS F
       ON F.id_aset = T.id_aset
      AND F.rn = 1
LEFT JOIN #BangunanAgg AS BG
       ON BG.id_aset_tanah = T.id_aset;

DROP TABLE #StatusHukumMap, #AsetTarget, #FotoRanked, #BangunanAgg;

-- ---------------------------------------------------------------------------------------------------
-- KONVERSI ke kolom DIGITALISASI_TANAH (padanan digitalisasi.convertValue dan fixCoordinates)
-- ---------------------------------------------------------------------------------------------------
-- Tipe kolom mentah (system_type_id): kolom FLOAT/REAL (62, 59) dibaca sebagai bilangan, bukan sebagai teks.
DECLARE @jenis TABLE (kolom SYSNAME NOT NULL PRIMARY KEY, tipe INT NOT NULL);
INSERT @jenis (kolom, tipe) SELECT c.name, c.system_type_id FROM tempdb.sys.columns AS c WHERE c.object_id = OBJECT_ID(N'tempdb..#raw_tanah');
DECLARE @tp_0 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'id_aset_tanah'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_14 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Luas_Tanah'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_15 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Luas_Bangunan'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_16 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Jumlah_Bangunan'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_20 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Nilai_Tanah'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_21 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'GPS_Latitude'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_22 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'GPS_Longitude'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @salah NVARCHAR(MAX) = (SELECT STRING_AGG(kolom, N', ') FROM @jenis WHERE kolom IN (N'kode_register_tanah', N'Kode_UE1', N'Kode_Satker', N'Nama_Satker', N'Kode_tanah', N'Uraian_tanah', N'No_aset', N'Alamat_tanah', N'RTRW_tanah', N'KelurahanDesa_tanah', N'Kecamatan_tanah', N'KabKota_tanah', N'Provinsi_tanah', N'Status_hukum', N'Foto', N'Kondisi_Tanah') AND tipe NOT IN (35, 48, 52, 56, 99, 104, 106, 108, 127, 167, 175, 231, 239));
IF @salah IS NOT NULL
BEGIN
    SET @salah = N'DIGITALISASI_TANAH: kolom teks dari SLDK bertipe FLOAT, tanggal, uang, atau tipe lain yang bentuk tekstualnya berbeda antara aplikasi dan SQL Server (' + @salah + N'). Ubah ke teks pada query sumber. TIDAK ADA yang diubah.';
    THROW 50050, @salah, 1;
END;

INSERT INTO DIGITALISASI_STG.DIGITALISASI_TANAH
    ([id_aset_tanah], [kode_register_tanah], [Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_tanah], [Uraian_tanah], [No_aset], [Alamat_tanah], [RTRW_tanah], [KelurahanDesa_tanah], [Kecamatan_tanah], [KabKota_tanah], [Provinsi_tanah], [Luas_Tanah], [Luas_Bangunan], [Jumlah_Bangunan], [Status_hukum], [Foto], [Kondisi_Tanah], [Nilai_Tanah], [Latitude], [Longitude], [id_sinkron])
SELECT
    dbo.dm_bilangan(CASE WHEN @tp_0 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[id_aset_tanah] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[id_aset_tanah], 3) END), -- id_aset_tanah
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[kode_register_tanah]), 60), -- kode_register_tanah
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_UE1]), 10), -- Kode_UE1
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_Satker]), 40), -- Kode_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Nama_Satker]), 500), -- Nama_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_tanah]), 30), -- Kode_tanah
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Uraian_tanah]), 500), -- Uraian_tanah
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[No_aset]), 30), -- No_aset
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Alamat_tanah]), 0), -- Alamat_tanah
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[RTRW_tanah]), 50), -- RTRW_tanah
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[KelurahanDesa_tanah]), 200), -- KelurahanDesa_tanah
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kecamatan_tanah]), 200), -- Kecamatan_tanah
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[KabKota_tanah]), 200), -- KabKota_tanah
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Provinsi_tanah]), 200), -- Provinsi_tanah
    dbo.dm_desimal(CASE WHEN @tp_14 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Luas_Tanah] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Luas_Tanah], 3) END, 4), -- Luas_Tanah
    dbo.dm_desimal(CASE WHEN @tp_15 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Luas_Bangunan] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Luas_Bangunan], 3) END, 4), -- Luas_Bangunan
    dbo.dm_bilangan(CASE WHEN @tp_16 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Jumlah_Bangunan] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Jumlah_Bangunan], 3) END), -- Jumlah_Bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Status_hukum]), 0), -- Status_hukum
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Foto]), 0), -- Foto
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kondisi_Tanah]), 100), -- Kondisi_Tanah
    dbo.dm_desimal(CASE WHEN @tp_20 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Nilai_Tanah] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Nilai_Tanah], 3) END, 2), -- Nilai_Tanah
    CASE WHEN k.lat IS NULL OR k.lng IS NULL OR (k.lat = 0 AND k.lng = 0) THEN NULL ELSE CAST(k.lat AS DECIMAL(10, 7)) END, -- Latitude
    CASE WHEN k.lat IS NULL OR k.lng IS NULL OR (k.lat = 0 AND k.lng = 0) THEN NULL ELSE CAST(k.lng AS DECIMAL(10, 7)) END, -- Longitude
    @run -- id_sinkron
FROM #raw_tanah AS r
CROSS APPLY (SELECT lat = dbo.dm_koordinat(CASE WHEN @tp_21 = 1 THEN NULL ELSE CONVERT(NVARCHAR(4000), r.[GPS_Latitude], 3) END, CASE WHEN @tp_21 = 1 THEN CAST(r.[GPS_Latitude] AS FLOAT) END, 90), lng = dbo.dm_koordinat(CASE WHEN @tp_22 = 1 THEN NULL ELSE CONVERT(NVARCHAR(4000), r.[GPS_Longitude], 3) END, CASE WHEN @tp_22 = 1 THEN CAST(r.[GPS_Longitude] AS FLOAT) END, 180)) AS k
;

DROP TABLE IF EXISTS #raw_tanah;
END;
GO

-- DIGITALISASI_GEDUNG_KANTOR_UTAMA: Gedung kantor utama (kd_brg 4010101001) KL 015 beserta tanah di bawahnya, status asuransi, dan koordinat
CREATE OR ALTER PROCEDURE dbo.datamart_dg_muat_gedung_kantor_utama @run BIGINT
AS
BEGIN
    SET NOCOUNT ON;

-- ---------------------------------------------------------------------------------------------------
-- QUERY ASLI (queries/gedung_kantor_utama.sql). Satu-satunya tambahan: INTO #raw_gedung_kantor_utama pada SELECT akhir.
-- ---------------------------------------------------------------------------------------------------
-- Gedung kantor utama (kd_brg 4010101001). Query asli dari pengelola data.
-- Baris bertanda [PASTI] ditambahkan agar koordinat ikut tersinkron untuk peta; selebihnya tidak diubah.
SELECT
    LEFT(A.kd_satker, 5)                AS Kode_UE1,
    A.kd_satker                         AS Kode_Satker,
    S.ur_satker                         AS Nama_Satker,
    A.kode_register                     AS Kode_Register_bangunan,
    A.kd_brg                            AS Kode_bangunan,
    A.ur_sskel                          AS Uraian_bangunan,
    A.no_aset                           AS NUP_bangunan,
    ISNULL(A.alamat,  S.alamat_satker)  AS Alamat_bangunan,
    A.kd_rtrw                           AS RTRW_bangunan,
    ISNULL(A.ur_kel,  S.nm_kel)         AS KelurahanDesa_bangunan,
    ISNULL(A.ur_kec,  S.nm_kec)         AS Kecamatan_bangunan,
    ISNULL(A.ur_kab,  S.nm_kab_kota)    AS KabKota_bangunan,
    ISNULL(A.ur_prov, S.nm_prov)        AS Provinsi_bangunan,
    A.luas                              AS Luas_Bangunan,
    F.folder                            AS Foto_bangunan,
    K.ur_kondisi                        AS Kondisi_Bangunan,
    A.rph_aset                          AS Nilai_Bangunan,
    AR.is_asuransi                      AS Status_Asuransi,
    T.kd_brg                            AS Kode_tanah,
    T.ur_sskel                          AS Uraian_tanah,
    T.no_aset                           AS No_aset_tanah,
    A.gps_latitude                      AS GPS_Latitude,    -- [PASTI] koordinat untuk peta
    A.gps_longitude                     AS GPS_Longitude    -- [PASTI] koordinat untuk peta
INTO #raw_gedung_kantor_utama
FROM DJKN.SIMAN2_M_ASET AS A
INNER JOIN DJKN.SIMAN2_R_SATKER AS S
        ON S.kd_satker = A.kd_satker
       AND S.status_uakpb = N'AKTIF'
LEFT JOIN DJKN.SIMAN2_R_KONDISI AS K
       ON K.kd_kondisi = A.kd_kondisi
OUTER APPLY (
    SELECT TOP (1)
        COALESCE(NULLIF(P.folder, N''), NULLIF(P.filename, N''), NULLIF(P.nm_photo, N'')) AS folder
    FROM DJKN.SIMAN2_M_ASET_PHOTO AS P
    WHERE P.id_aset = A.id_aset
      AND P.status_data = 1
    ORDER BY P.photo_utama_yn DESC, P.tanggal DESC, P.id_photo DESC
) AS F
OUTER APPLY (
    SELECT TOP (1) G.is_asuransi
    FROM DJKN.SIMAN2_PENGELOLAAN AS G
    WHERE G.kode_register = A.kode_register
    ORDER BY G.tgl_sk DESC
) AS AR
OUTER APPLY (
    SELECT TOP (1) AT.kd_brg, AT.ur_sskel, AT.no_aset
    FROM DJKN.SIMAN2_M_ASET_TANAH_BANGUNAN AS TB
    INNER JOIN DJKN.SIMAN2_M_ASET AS AT
            ON AT.id_aset = TB.id_aset_tanah
    WHERE TB.id_aset_bangunan = A.id_aset
      AND TB.id_aset_tanah IS NOT NULL
      AND TB.status_data = 1
      AND AT.status_bmn_yn = N'Y'
      AND AT.kd_brg LIKE N'2%'
    ORDER BY TB.id_aset_tanah
) AS T
WHERE A.kd_brg = N'4010101001'
  AND A.status_bmn_yn = N'Y'
  AND A.kd_satker LIKE N'015%'
  AND A.kode_register IS NOT NULL;   -- hapus baris ini jika ada bangunan tanpa kode_register

-- ---------------------------------------------------------------------------------------------------
-- KONVERSI ke kolom DIGITALISASI_GEDUNG_KANTOR_UTAMA (padanan digitalisasi.convertValue dan fixCoordinates)
-- ---------------------------------------------------------------------------------------------------
-- Tipe kolom mentah (system_type_id): kolom FLOAT/REAL (62, 59) dibaca sebagai bilangan, bukan sebagai teks.
DECLARE @jenis TABLE (kolom SYSNAME NOT NULL PRIMARY KEY, tipe INT NOT NULL);
INSERT @jenis (kolom, tipe) SELECT c.name, c.system_type_id FROM tempdb.sys.columns AS c WHERE c.object_id = OBJECT_ID(N'tempdb..#raw_gedung_kantor_utama');
DECLARE @tp_13 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Luas_Bangunan'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_16 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Nilai_Bangunan'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_21 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'GPS_Latitude'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_22 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'GPS_Longitude'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @salah NVARCHAR(MAX) = (SELECT STRING_AGG(kolom, N', ') FROM @jenis WHERE kolom IN (N'Kode_UE1', N'Kode_Satker', N'Nama_Satker', N'Kode_Register_bangunan', N'Kode_bangunan', N'Uraian_bangunan', N'NUP_bangunan', N'Alamat_bangunan', N'RTRW_bangunan', N'KelurahanDesa_bangunan', N'Kecamatan_bangunan', N'KabKota_bangunan', N'Provinsi_bangunan', N'Foto_bangunan', N'Kondisi_Bangunan', N'Status_Asuransi', N'Kode_tanah', N'Uraian_tanah', N'No_aset_tanah') AND tipe NOT IN (35, 48, 52, 56, 99, 104, 106, 108, 127, 167, 175, 231, 239));
IF @salah IS NOT NULL
BEGIN
    SET @salah = N'DIGITALISASI_GEDUNG_KANTOR_UTAMA: kolom teks dari SLDK bertipe FLOAT, tanggal, uang, atau tipe lain yang bentuk tekstualnya berbeda antara aplikasi dan SQL Server (' + @salah + N'). Ubah ke teks pada query sumber. TIDAK ADA yang diubah.';
    THROW 50050, @salah, 1;
END;

INSERT INTO DIGITALISASI_STG.DIGITALISASI_GEDUNG_KANTOR_UTAMA
    ([Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_Register_bangunan], [Kode_bangunan], [Uraian_bangunan], [NUP_bangunan], [Alamat_bangunan], [RTRW_bangunan], [KelurahanDesa_bangunan], [Kecamatan_bangunan], [KabKota_bangunan], [Provinsi_bangunan], [Luas_Bangunan], [Foto_bangunan], [Kondisi_Bangunan], [Nilai_Bangunan], [Status_Asuransi], [Kode_tanah], [Uraian_tanah], [No_aset_tanah], [Latitude], [Longitude], [id_sinkron])
SELECT
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_UE1]), 10), -- Kode_UE1
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_Satker]), 40), -- Kode_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Nama_Satker]), 500), -- Nama_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_Register_bangunan]), 60), -- Kode_Register_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_bangunan]), 30), -- Kode_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Uraian_bangunan]), 500), -- Uraian_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[NUP_bangunan]), 30), -- NUP_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Alamat_bangunan]), 0), -- Alamat_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[RTRW_bangunan]), 50), -- RTRW_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[KelurahanDesa_bangunan]), 200), -- KelurahanDesa_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kecamatan_bangunan]), 200), -- Kecamatan_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[KabKota_bangunan]), 200), -- KabKota_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Provinsi_bangunan]), 200), -- Provinsi_bangunan
    dbo.dm_desimal(CASE WHEN @tp_13 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Luas_Bangunan] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Luas_Bangunan], 3) END, 4), -- Luas_Bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Foto_bangunan]), 0), -- Foto_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kondisi_Bangunan]), 100), -- Kondisi_Bangunan
    dbo.dm_desimal(CASE WHEN @tp_16 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Nilai_Bangunan] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Nilai_Bangunan], 3) END, 2), -- Nilai_Bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Status_Asuransi]), 20), -- Status_Asuransi
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_tanah]), 30), -- Kode_tanah
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Uraian_tanah]), 500), -- Uraian_tanah
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[No_aset_tanah]), 30), -- No_aset_tanah
    CASE WHEN k.lat IS NULL OR k.lng IS NULL OR (k.lat = 0 AND k.lng = 0) THEN NULL ELSE CAST(k.lat AS DECIMAL(10, 7)) END, -- Latitude
    CASE WHEN k.lat IS NULL OR k.lng IS NULL OR (k.lat = 0 AND k.lng = 0) THEN NULL ELSE CAST(k.lng AS DECIMAL(10, 7)) END, -- Longitude
    @run -- id_sinkron
FROM #raw_gedung_kantor_utama AS r
CROSS APPLY (SELECT lat = dbo.dm_koordinat(CASE WHEN @tp_21 = 1 THEN NULL ELSE CONVERT(NVARCHAR(4000), r.[GPS_Latitude], 3) END, CASE WHEN @tp_21 = 1 THEN CAST(r.[GPS_Latitude] AS FLOAT) END, 90), lng = dbo.dm_koordinat(CASE WHEN @tp_22 = 1 THEN NULL ELSE CONVERT(NVARCHAR(4000), r.[GPS_Longitude], 3) END, CASE WHEN @tp_22 = 1 THEN CAST(r.[GPS_Longitude] AS FLOAT) END, 180)) AS k
;

DROP TABLE IF EXISTS #raw_gedung_kantor_utama;
END;
GO

-- DIGITALISASI_GEDUNG_LAINNYA: Gedung dan bangunan lain (kd_brg 40101%, selain gedung kantor utama) KL 015 beserta fungsi, status hukum, dan asuransi
CREATE OR ALTER PROCEDURE dbo.datamart_dg_muat_gedung_lainnya @run BIGINT
AS
BEGIN
    SET NOCOUNT ON;

-- ---------------------------------------------------------------------------------------------------
-- QUERY ASLI (queries/gedung_lainnya.sql). Satu-satunya tambahan: INTO #raw_gedung_lainnya pada SELECT akhir.
-- ---------------------------------------------------------------------------------------------------
-- Gedung dan bangunan lainnya (kd_brg 40101%, selain 4010101001 = gedung kantor). Query asli dari pengelola data.
-- Baris bertanda [PASTI] ditambahkan agar koordinat ikut tersinkron untuk peta; selebihnya tidak diubah.
SELECT
    LEFT(A.kd_satker, 5)                    AS Kode_UE1,
    A.kd_satker                             AS Kode_Satker,
    S.ur_satker                             AS Nama_Satker,
    A.kode_register                         AS Kode_Register_bangunan,
    A.kd_brg                                AS Kode_bangunan,
    A.ur_sskel                              AS Uraian_bangunan,
    A.no_aset                               AS NUP_bangunan,
    ISNULL(A.alamat,  S.alamat_satker)      AS Alamat_bangunan,
    A.kd_rtrw                               AS RTRW_bangunan,
    ISNULL(A.ur_kel,  S.nm_kel)             AS KelurahanDesa_bangunan,
    ISNULL(A.ur_kec,  S.nm_kec)             AS Kecamatan_bangunan,
    ISNULL(A.ur_kab,  S.nm_kab_kota)        AS KabKota_bangunan,
    ISNULL(A.ur_prov, S.nm_prov)            AS Provinsi_bangunan,
    A.luas                                  AS Luas_Bangunan,
    F.folder                                AS Foto_bangunan,
    A.merk                                  AS Fungsi,
    SH.ur_status_hukum                      AS Status_Hukum,
    A.nm_unit_pengguna                      AS Nama_Pengguna,
    K.ur_kondisi                            AS Kondisi_Bangunan,
    A.rph_aset                              AS Nilai_Bangunan,
    AR.is_asuransi                          AS Status_Asuransi,
    A.gps_latitude                          AS GPS_Latitude,    -- [PASTI] koordinat untuk peta
    A.gps_longitude                         AS GPS_Longitude    -- [PASTI] koordinat untuk peta
INTO #raw_gedung_lainnya
FROM DJKN.SIMAN2_M_ASET AS A
INNER JOIN DJKN.SIMAN2_R_SATKER AS S
        ON S.kd_satker = A.kd_satker
       AND S.kd_satker LIKE N'015%'
       AND S.status_uakpb = N'AKTIF'
LEFT JOIN DJKN.SIMAN2_R_KONDISI AS K
       ON K.kd_kondisi = A.kd_kondisi
LEFT JOIN (
        SELECT
            SH.kd_status_hukum,
            STRING_AGG(
                CONVERT(nvarchar(max), SH.ur_status_hukum)
                + N' [' + CONVERT(nvarchar(max), SH.label) + N']', N' | '
            ) WITHIN GROUP (ORDER BY SH.id_status_hukum) AS ur_status_hukum
        FROM DJKN.SIMAN2_R_STATUS_HUKUM_NEW AS SH
        WHERE SH.TGL_TARIK = (SELECT MAX(TGL_TARIK) FROM DJKN.SIMAN2_R_STATUS_HUKUM_NEW)
        GROUP BY SH.kd_status_hukum
     ) AS SH
       ON SH.kd_status_hukum = A.kd_status_hukum
LEFT JOIN (
        SELECT id_aset, folder
        FROM (
            SELECT
                P.id_aset,
                COALESCE(NULLIF(P.folder, N''), NULLIF(P.filename, N''), NULLIF(P.nm_photo, N'')) AS folder,
                ROW_NUMBER() OVER (
                    PARTITION BY P.id_aset
                    ORDER BY P.photo_utama_yn DESC, P.tanggal DESC, P.id_photo DESC
                ) AS rn
            FROM DJKN.SIMAN2_M_ASET_PHOTO AS P
            WHERE P.status_data = 1
        ) X
        WHERE rn = 1
     ) AS F
       ON F.id_aset = A.id_aset
LEFT JOIN (
        SELECT kode_register, is_asuransi
        FROM (
            SELECT
                P.kode_register,
                P.is_asuransi,
                ROW_NUMBER() OVER (PARTITION BY P.kode_register ORDER BY P.tgl_sk DESC) AS rn
            FROM DJKN.SIMAN2_PENGELOLAAN AS P
            WHERE P.kode_register IS NOT NULL
        ) X
        WHERE rn = 1
     ) AS AR
       ON AR.kode_register = A.kode_register
WHERE A.status_bmn_yn = N'Y'
  AND A.kd_brg LIKE N'40101%'
  AND A.kd_brg <> N'4010101001'
  AND A.kd_satker LIKE N'015%'
OPTION (MAXDOP 4);

-- ---------------------------------------------------------------------------------------------------
-- KONVERSI ke kolom DIGITALISASI_GEDUNG_LAINNYA (padanan digitalisasi.convertValue dan fixCoordinates)
-- ---------------------------------------------------------------------------------------------------
-- Tipe kolom mentah (system_type_id): kolom FLOAT/REAL (62, 59) dibaca sebagai bilangan, bukan sebagai teks.
DECLARE @jenis TABLE (kolom SYSNAME NOT NULL PRIMARY KEY, tipe INT NOT NULL);
INSERT @jenis (kolom, tipe) SELECT c.name, c.system_type_id FROM tempdb.sys.columns AS c WHERE c.object_id = OBJECT_ID(N'tempdb..#raw_gedung_lainnya');
DECLARE @tp_13 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Luas_Bangunan'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_19 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Nilai_Bangunan'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_21 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'GPS_Latitude'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_22 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'GPS_Longitude'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @salah NVARCHAR(MAX) = (SELECT STRING_AGG(kolom, N', ') FROM @jenis WHERE kolom IN (N'Kode_UE1', N'Kode_Satker', N'Nama_Satker', N'Kode_Register_bangunan', N'Kode_bangunan', N'Uraian_bangunan', N'NUP_bangunan', N'Alamat_bangunan', N'RTRW_bangunan', N'KelurahanDesa_bangunan', N'Kecamatan_bangunan', N'KabKota_bangunan', N'Provinsi_bangunan', N'Foto_bangunan', N'Fungsi', N'Status_Hukum', N'Nama_Pengguna', N'Kondisi_Bangunan', N'Status_Asuransi') AND tipe NOT IN (35, 48, 52, 56, 99, 104, 106, 108, 127, 167, 175, 231, 239));
IF @salah IS NOT NULL
BEGIN
    SET @salah = N'DIGITALISASI_GEDUNG_LAINNYA: kolom teks dari SLDK bertipe FLOAT, tanggal, uang, atau tipe lain yang bentuk tekstualnya berbeda antara aplikasi dan SQL Server (' + @salah + N'). Ubah ke teks pada query sumber. TIDAK ADA yang diubah.';
    THROW 50050, @salah, 1;
END;

INSERT INTO DIGITALISASI_STG.DIGITALISASI_GEDUNG_LAINNYA
    ([Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_Register_bangunan], [Kode_bangunan], [Uraian_bangunan], [NUP_bangunan], [Alamat_bangunan], [RTRW_bangunan], [KelurahanDesa_bangunan], [Kecamatan_bangunan], [KabKota_bangunan], [Provinsi_bangunan], [Luas_Bangunan], [Foto_bangunan], [Fungsi], [Status_Hukum], [Nama_Pengguna], [Kondisi_Bangunan], [Nilai_Bangunan], [Status_Asuransi], [Latitude], [Longitude], [id_sinkron])
SELECT
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_UE1]), 10), -- Kode_UE1
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_Satker]), 40), -- Kode_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Nama_Satker]), 500), -- Nama_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_Register_bangunan]), 60), -- Kode_Register_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_bangunan]), 30), -- Kode_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Uraian_bangunan]), 500), -- Uraian_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[NUP_bangunan]), 30), -- NUP_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Alamat_bangunan]), 0), -- Alamat_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[RTRW_bangunan]), 50), -- RTRW_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[KelurahanDesa_bangunan]), 200), -- KelurahanDesa_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kecamatan_bangunan]), 200), -- Kecamatan_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[KabKota_bangunan]), 200), -- KabKota_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Provinsi_bangunan]), 200), -- Provinsi_bangunan
    dbo.dm_desimal(CASE WHEN @tp_13 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Luas_Bangunan] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Luas_Bangunan], 3) END, 4), -- Luas_Bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Foto_bangunan]), 0), -- Foto_bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Fungsi]), 500), -- Fungsi
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Status_Hukum]), 0), -- Status_Hukum
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Nama_Pengguna]), 500), -- Nama_Pengguna
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kondisi_Bangunan]), 100), -- Kondisi_Bangunan
    dbo.dm_desimal(CASE WHEN @tp_19 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Nilai_Bangunan] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Nilai_Bangunan], 3) END, 2), -- Nilai_Bangunan
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Status_Asuransi]), 20), -- Status_Asuransi
    CASE WHEN k.lat IS NULL OR k.lng IS NULL OR (k.lat = 0 AND k.lng = 0) THEN NULL ELSE CAST(k.lat AS DECIMAL(10, 7)) END, -- Latitude
    CASE WHEN k.lat IS NULL OR k.lng IS NULL OR (k.lat = 0 AND k.lng = 0) THEN NULL ELSE CAST(k.lng AS DECIMAL(10, 7)) END, -- Longitude
    @run -- id_sinkron
FROM #raw_gedung_lainnya AS r
CROSS APPLY (SELECT lat = dbo.dm_koordinat(CASE WHEN @tp_21 = 1 THEN NULL ELSE CONVERT(NVARCHAR(4000), r.[GPS_Latitude], 3) END, CASE WHEN @tp_21 = 1 THEN CAST(r.[GPS_Latitude] AS FLOAT) END, 90), lng = dbo.dm_koordinat(CASE WHEN @tp_22 = 1 THEN NULL ELSE CONVERT(NVARCHAR(4000), r.[GPS_Longitude], 3) END, CASE WHEN @tp_22 = 1 THEN CAST(r.[GPS_Longitude] AS FLOAT) END, 180)) AS k
;

DROP TABLE IF EXISTS #raw_gedung_lainnya;
END;
GO

-- DIGITALISASI_RUSUNARA: Rumah susun negara (kd_brg 4010208%) KL 015 beserta jumlah kamar tidur per tipe (E, D, C)
CREATE OR ALTER PROCEDURE dbo.datamart_dg_muat_rusunara @run BIGINT
AS
BEGIN
    SET NOCOUNT ON;

-- ---------------------------------------------------------------------------------------------------
-- QUERY ASLI (queries/rusunara.sql). Satu-satunya tambahan: INTO #raw_rusunara pada SELECT akhir.
-- ---------------------------------------------------------------------------------------------------
-- Rusunara (kd_brg 4010208%). Query asli dari pengelola data.
-- Baris bertanda [PASTI] ditambahkan agar koordinat ikut tersinkron untuk peta; selebihnya tidak diubah.
WITH SatkerScope AS (
    SELECT S.kd_satker, S.ur_satker, S.alamat_satker, S.nm_kel, S.nm_kec, S.nm_kab_kota, S.nm_prov
    FROM DJKN.SIMAN2_R_SATKER AS S
    WHERE S.kd_satker LIKE N'015%'
      AND S.status_uakpb = N'AKTIF'
),
AsetFiltered AS (
    SELECT
        A.id_aset, A.kd_satker, A.kd_brg, A.ur_sskel, A.no_aset, A.alamat, A.kd_rtrw,
        A.ur_kel, A.ur_kec, A.ur_kab, A.ur_prov, A.luas, A.kd_kondisi, A.rph_aset,
        A.gps_latitude, A.gps_longitude,   -- [PASTI] koordinat untuk peta
        A.TGL_TARIK,
        MAX(A.TGL_TARIK) OVER () AS max_tgl_tarik
    FROM DJKN.SIMAN2_M_ASET AS A
    WHERE A.status_bmn_yn = N'Y'
      AND A.kd_brg LIKE N'4010208%'
      AND A.kd_satker LIKE N'015%'
),
AsetTarget AS (
    SELECT
        F.id_aset, F.kd_satker, S.ur_satker,
        S.alamat_satker, S.nm_kel, S.nm_kec, S.nm_kab_kota, S.nm_prov,
        F.kd_brg, F.ur_sskel, F.no_aset, F.alamat, F.kd_rtrw,
        F.ur_kel, F.ur_kec, F.ur_kab, F.ur_prov,
        F.luas, F.kd_kondisi, F.rph_aset,
        F.gps_latitude, F.gps_longitude    -- [PASTI] koordinat untuk peta
    FROM AsetFiltered AS F
    INNER JOIN SatkerScope AS S ON S.kd_satker = F.kd_satker
    WHERE F.TGL_TARIK = F.max_tgl_tarik
),
InventarisRanked AS (
    SELECT
           I.id_aset, I.alamat, I.kd_rtrw, I.ur_kel, I.ur_kec,
           I.ur_kab, I.ur_prov, I.luas_bdg,
           ROW_NUMBER() OVER (PARTITION BY I.id_aset ORDER BY I.TGL_TARIK DESC, I.updated_at DESC, I.id_inv_bangunan DESC) AS rn
    FROM DJKN.SIMAN2_T_INV_BANGUNAN AS I
    INNER JOIN AsetTarget AS A ON A.id_aset = I.id_aset
),
FotoRanked AS (
    SELECT
        P.id_aset,
        COALESCE(NULLIF(LTRIM(RTRIM(P.folder)), N''), NULLIF(LTRIM(RTRIM(P.filename)), N''), NULLIF(LTRIM(RTRIM(P.nm_photo)), N'')) AS foto,
        ROW_NUMBER() OVER (
            PARTITION BY P.id_aset
            ORDER BY CASE WHEN P.photo_utama_yn = N'Y' THEN 0 ELSE 1 END, P.TGL_TARIK DESC, P.tanggal DESC, P.id_photo DESC
        ) AS rn
    FROM DJKN.SIMAN2_M_ASET_PHOTO AS P
    INNER JOIN AsetTarget AS A ON A.id_aset = P.id_aset
    WHERE P.status_data = 1
),
RuangAgg AS (
    SELECT
        R.KODE_SATKER, R.KODE_BARANG, R.NUP,
        SUM(CASE WHEN UPPER(LTRIM(RTRIM(R.tipe_ruang))) = N'RUANG TIDUR'
                      AND UPPER(LTRIM(RTRIM(R.kode_ruang))) LIKE N'E%' THEN 1 ELSE 0 END) AS kamar_tipe_e,
        SUM(CASE WHEN UPPER(LTRIM(RTRIM(R.tipe_ruang))) = N'RUANG TIDUR'
                      AND UPPER(LTRIM(RTRIM(R.kode_ruang))) LIKE N'D%' THEN 1 ELSE 0 END) AS kamar_tipe_d,
        SUM(CASE WHEN UPPER(LTRIM(RTRIM(R.tipe_ruang))) = N'RUANG TIDUR'
                      AND UPPER(LTRIM(RTRIM(R.kode_ruang))) LIKE N'C%' THEN 1 ELSE 0 END) AS kamar_tipe_c
    FROM DJKN.VW_SIMAN_RUANG_GEDUNG_BANGUNAN_015 AS R
    INNER JOIN AsetTarget AS A
            ON A.kd_satker = R.KODE_SATKER
           AND A.kd_brg = R.KODE_BARANG
           AND A.no_aset = R.NUP
    GROUP BY R.KODE_SATKER, R.KODE_BARANG, R.NUP
)
SELECT
    LEFT(A.kd_satker, 5) AS Kode_UE1,
    A.kd_satker AS Kode_Satker,
    A.ur_satker AS Nama_Satker,
    A.kd_brg AS Kode_Rusun,
    A.ur_sskel AS Uraian_Rusun,
    A.no_aset AS NUP_Rusun,
    COALESCE(NULLIF(LTRIM(RTRIM(I.alamat)), N''), NULLIF(LTRIM(RTRIM(A.alamat)), N''), A.alamat_satker) AS Alamat_Rusun,
    COALESCE(NULLIF(LTRIM(RTRIM(I.kd_rtrw)), N''), NULLIF(LTRIM(RTRIM(A.kd_rtrw)), N'')) AS RTRW_Rusun,
    COALESCE(NULLIF(LTRIM(RTRIM(I.ur_kel)), N''), NULLIF(LTRIM(RTRIM(A.ur_kel)), N''), A.nm_kel) AS KelurahanDesa_Rusun,
    COALESCE(NULLIF(LTRIM(RTRIM(I.ur_kec)), N''), NULLIF(LTRIM(RTRIM(A.ur_kec)), N''), A.nm_kec) AS Kecamatan_Rusun,
    COALESCE(NULLIF(LTRIM(RTRIM(I.ur_kab)), N''), NULLIF(LTRIM(RTRIM(A.ur_kab)), N''), A.nm_kab_kota) AS KabKota_Rusun,
    COALESCE(NULLIF(LTRIM(RTRIM(I.ur_prov)), N''), NULLIF(LTRIM(RTRIM(A.ur_prov)), N''), A.nm_prov) AS Provinsi_Rusun,
    COALESCE(NULLIF(I.luas_bdg, 0), NULLIF(A.luas, 0), A.luas) AS Luas_Rusun,
    F.foto AS Foto_Rusun,
    K.ur_kondisi AS Kondisi_Rusun,
    A.rph_aset AS Nilai_Rusun,
    COALESCE(R.kamar_tipe_e, 0) AS Kamar_tipe_E,
    COALESCE(R.kamar_tipe_d, 0) AS Kamar_tipe_D,
    COALESCE(R.kamar_tipe_c, 0) AS Kamar_tipe_C,
    A.gps_latitude AS GPS_Latitude,        -- [PASTI] koordinat untuk peta
    A.gps_longitude AS GPS_Longitude       -- [PASTI] koordinat untuk peta
INTO #raw_rusunara
FROM AsetTarget AS A
LEFT JOIN InventarisRanked AS I ON I.id_aset = A.id_aset AND I.rn = 1
LEFT JOIN FotoRanked AS F ON F.id_aset = A.id_aset AND F.rn = 1
LEFT JOIN RuangAgg AS R
       ON R.KODE_SATKER = A.kd_satker
      AND R.KODE_BARANG = A.kd_brg
      AND R.NUP = A.no_aset
LEFT JOIN DJKN.SIMAN2_R_KONDISI AS K ON K.kd_kondisi = A.kd_kondisi
ORDER BY A.kd_satker, A.no_aset;

-- ---------------------------------------------------------------------------------------------------
-- KONVERSI ke kolom DIGITALISASI_RUSUNARA (padanan digitalisasi.convertValue dan fixCoordinates)
-- ---------------------------------------------------------------------------------------------------
-- Tipe kolom mentah (system_type_id): kolom FLOAT/REAL (62, 59) dibaca sebagai bilangan, bukan sebagai teks.
DECLARE @jenis TABLE (kolom SYSNAME NOT NULL PRIMARY KEY, tipe INT NOT NULL);
INSERT @jenis (kolom, tipe) SELECT c.name, c.system_type_id FROM tempdb.sys.columns AS c WHERE c.object_id = OBJECT_ID(N'tempdb..#raw_rusunara');
DECLARE @tp_12 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Luas_Rusun'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_15 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Nilai_Rusun'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_16 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Kamar_tipe_E'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_17 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Kamar_tipe_D'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_18 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Kamar_tipe_C'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_19 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'GPS_Latitude'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_20 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'GPS_Longitude'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @salah NVARCHAR(MAX) = (SELECT STRING_AGG(kolom, N', ') FROM @jenis WHERE kolom IN (N'Kode_UE1', N'Kode_Satker', N'Nama_Satker', N'Kode_Rusun', N'Uraian_Rusun', N'NUP_Rusun', N'Alamat_Rusun', N'RTRW_Rusun', N'KelurahanDesa_Rusun', N'Kecamatan_Rusun', N'KabKota_Rusun', N'Provinsi_Rusun', N'Foto_Rusun', N'Kondisi_Rusun') AND tipe NOT IN (35, 48, 52, 56, 99, 104, 106, 108, 127, 167, 175, 231, 239));
IF @salah IS NOT NULL
BEGIN
    SET @salah = N'DIGITALISASI_RUSUNARA: kolom teks dari SLDK bertipe FLOAT, tanggal, uang, atau tipe lain yang bentuk tekstualnya berbeda antara aplikasi dan SQL Server (' + @salah + N'). Ubah ke teks pada query sumber. TIDAK ADA yang diubah.';
    THROW 50050, @salah, 1;
END;

INSERT INTO DIGITALISASI_STG.DIGITALISASI_RUSUNARA
    ([Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_Rusun], [Uraian_Rusun], [NUP_Rusun], [Alamat_Rusun], [RTRW_Rusun], [KelurahanDesa_Rusun], [Kecamatan_Rusun], [KabKota_Rusun], [Provinsi_Rusun], [Luas_Rusun], [Foto_Rusun], [Kondisi_Rusun], [Nilai_Rusun], [Kamar_tipe_E], [Kamar_tipe_D], [Kamar_tipe_C], [Latitude], [Longitude], [id_sinkron])
SELECT
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_UE1]), 10), -- Kode_UE1
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_Satker]), 40), -- Kode_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Nama_Satker]), 500), -- Nama_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_Rusun]), 30), -- Kode_Rusun
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Uraian_Rusun]), 500), -- Uraian_Rusun
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[NUP_Rusun]), 30), -- NUP_Rusun
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Alamat_Rusun]), 0), -- Alamat_Rusun
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[RTRW_Rusun]), 50), -- RTRW_Rusun
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[KelurahanDesa_Rusun]), 200), -- KelurahanDesa_Rusun
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kecamatan_Rusun]), 200), -- Kecamatan_Rusun
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[KabKota_Rusun]), 200), -- KabKota_Rusun
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Provinsi_Rusun]), 200), -- Provinsi_Rusun
    dbo.dm_desimal(CASE WHEN @tp_12 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Luas_Rusun] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Luas_Rusun], 3) END, 4), -- Luas_Rusun
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Foto_Rusun]), 0), -- Foto_Rusun
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kondisi_Rusun]), 100), -- Kondisi_Rusun
    dbo.dm_desimal(CASE WHEN @tp_15 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Nilai_Rusun] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Nilai_Rusun], 3) END, 2), -- Nilai_Rusun
    dbo.dm_bilangan(CASE WHEN @tp_16 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Kamar_tipe_E] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Kamar_tipe_E], 3) END), -- Kamar_tipe_E
    dbo.dm_bilangan(CASE WHEN @tp_17 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Kamar_tipe_D] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Kamar_tipe_D], 3) END), -- Kamar_tipe_D
    dbo.dm_bilangan(CASE WHEN @tp_18 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Kamar_tipe_C] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Kamar_tipe_C], 3) END), -- Kamar_tipe_C
    CASE WHEN k.lat IS NULL OR k.lng IS NULL OR (k.lat = 0 AND k.lng = 0) THEN NULL ELSE CAST(k.lat AS DECIMAL(10, 7)) END, -- Latitude
    CASE WHEN k.lat IS NULL OR k.lng IS NULL OR (k.lat = 0 AND k.lng = 0) THEN NULL ELSE CAST(k.lng AS DECIMAL(10, 7)) END, -- Longitude
    @run -- id_sinkron
FROM #raw_rusunara AS r
CROSS APPLY (SELECT lat = dbo.dm_koordinat(CASE WHEN @tp_19 = 1 THEN NULL ELSE CONVERT(NVARCHAR(4000), r.[GPS_Latitude], 3) END, CASE WHEN @tp_19 = 1 THEN CAST(r.[GPS_Latitude] AS FLOAT) END, 90), lng = dbo.dm_koordinat(CASE WHEN @tp_20 = 1 THEN NULL ELSE CONVERT(NVARCHAR(4000), r.[GPS_Longitude], 3) END, CASE WHEN @tp_20 = 1 THEN CAST(r.[GPS_Longitude] AS FLOAT) END, 180)) AS k
ORDER BY r.[Kode_Satker], r.[NUP_Rusun]
;

DROP TABLE IF EXISTS #raw_rusunara;
END;
GO

-- DIGITALISASI_RUMAH_NEGARA: Rumah negara (kd_brg 4010201%, 4010202%, 4010209%; tanpa mess) KL 015 beserta status penghuni dan status hukum
CREATE OR ALTER PROCEDURE dbo.datamart_dg_muat_rumah_negara @run BIGINT
AS
BEGIN
    SET NOCOUNT ON;

-- ---------------------------------------------------------------------------------------------------
-- QUERY ASLI (queries/rumah_negara.sql). Satu-satunya tambahan: INTO #raw_rumah_negara pada SELECT akhir.
-- ---------------------------------------------------------------------------------------------------
-- Rumah Negara (kd_brg 4010201%, 4010202%, 4010209%; mess 4010202016 dikecualikan). Query asli dari pengelola data.
-- Baris bertanda [PASTI] ditambahkan agar koordinat ikut tersinkron untuk peta; selebihnya tidak diubah.
WITH StatusHukumMap AS (
    SELECT
        SH.kd_status_hukum,
        STRING_AGG(
            CONVERT(nvarchar(max), SH.ur_status_hukum)
            + N' [' + CONVERT(nvarchar(max), SH.label) + N']',
            N' | '
        ) WITHIN GROUP (ORDER BY SH.id_status_hukum) AS ur_status_hukum
    FROM DJKN.SIMAN2_R_STATUS_HUKUM_NEW AS SH
    WHERE SH.TGL_TARIK = (SELECT MAX(TGL_TARIK) FROM DJKN.SIMAN2_R_STATUS_HUKUM_NEW)
    GROUP BY SH.kd_status_hukum
),
FotoRanked AS (
    SELECT
        P.id_aset,
        COALESCE(NULLIF(P.folder, N''), NULLIF(P.filename, N''), NULLIF(P.nm_photo, N'')) AS folder,
        ROW_NUMBER() OVER (
            PARTITION BY P.id_aset
            ORDER BY P.photo_utama_yn DESC, P.tanggal DESC, P.id_photo DESC
        ) AS rn
    FROM DJKN.SIMAN2_M_ASET_PHOTO AS P
    WHERE P.status_data = 1
),
PemakaiRanked AS (
    SELECT
        P.id_aset, P.jns_pemakai, P.nm_pmk,
        ROW_NUMBER() OVER (
            PARTITION BY P.id_aset
            ORDER BY P.tgl_mulai DESC, P.id_aset_pemakai DESC
        ) AS rn
    FROM DJKN.SIMAN2_M_ASET_PEMAKAI AS P
)
SELECT
    LEFT(A.kd_satker, 5)                    AS Kode_UE1,
    A.kd_satker                             AS Kode_Satker,
    S.ur_satker                             AS Nama_Satker,
    A.kode_register                         AS Kode_Register_RN,
    A.kd_brg                                AS Kode_RN,
    A.ur_sskel                              AS Uraian_RN,
    A.no_aset                               AS NUP_RN,
    ISNULL(A.alamat,  S.alamat_satker)      AS Alamat_RN,
    A.kd_rtrw                               AS RTRW_RN,
    ISNULL(A.ur_kel,  S.nm_kel)             AS KelurahanDesa_RN,
    ISNULL(A.ur_kec,  S.nm_kec)             AS Kecamatan_RN,
    ISNULL(A.ur_kab,  S.nm_kab_kota)        AS KabKota_RN,
    ISNULL(A.ur_prov, S.nm_prov)            AS Provinsi_RN,
    A.luas                                  AS Luas_RN,
    F.folder                                AS Foto_RN,
    P.jns_pemakai                           AS Status_Penghuni,
    P.nm_pmk                                AS Nama_Penghuni,
    K.ur_kondisi                            AS Kondisi_RN,
    SH.ur_status_hukum                      AS Status_Hukum,
    A.gps_latitude                          AS GPS_Latitude,    -- [PASTI] koordinat untuk peta
    A.gps_longitude                         AS GPS_Longitude    -- [PASTI] koordinat untuk peta
INTO #raw_rumah_negara
FROM DJKN.SIMAN2_M_ASET AS A
INNER JOIN DJKN.SIMAN2_R_SATKER AS S
        ON S.kd_satker = A.kd_satker
       AND S.status_uakpb = N'AKTIF'
LEFT JOIN DJKN.SIMAN2_R_KONDISI AS K
       ON K.kd_kondisi = A.kd_kondisi
LEFT JOIN StatusHukumMap AS SH
       ON SH.kd_status_hukum = A.kd_status_hukum
LEFT JOIN FotoRanked AS F
       ON F.id_aset = A.id_aset AND F.rn = 1
LEFT JOIN PemakaiRanked AS P
       ON P.id_aset = A.id_aset AND P.rn = 1
WHERE A.status_bmn_yn = N'Y'
  AND A.kd_satker LIKE N'015%'          -- filter di M_ASET langsung, bukan hanya di satker
  AND (
       A.kd_brg LIKE N'4010201%'
    OR A.kd_brg LIKE N'4010202%'
    OR A.kd_brg LIKE N'4010209%'
  )
  AND A.kd_brg <> N'4010202016';

-- ---------------------------------------------------------------------------------------------------
-- KONVERSI ke kolom DIGITALISASI_RUMAH_NEGARA (padanan digitalisasi.convertValue dan fixCoordinates)
-- ---------------------------------------------------------------------------------------------------
-- Tipe kolom mentah (system_type_id): kolom FLOAT/REAL (62, 59) dibaca sebagai bilangan, bukan sebagai teks.
DECLARE @jenis TABLE (kolom SYSNAME NOT NULL PRIMARY KEY, tipe INT NOT NULL);
INSERT @jenis (kolom, tipe) SELECT c.name, c.system_type_id FROM tempdb.sys.columns AS c WHERE c.object_id = OBJECT_ID(N'tempdb..#raw_rumah_negara');
DECLARE @tp_13 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Luas_RN'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_19 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'GPS_Latitude'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_20 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'GPS_Longitude'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @salah NVARCHAR(MAX) = (SELECT STRING_AGG(kolom, N', ') FROM @jenis WHERE kolom IN (N'Kode_UE1', N'Kode_Satker', N'Nama_Satker', N'Kode_Register_RN', N'Kode_RN', N'Uraian_RN', N'NUP_RN', N'Alamat_RN', N'RTRW_RN', N'KelurahanDesa_RN', N'Kecamatan_RN', N'KabKota_RN', N'Provinsi_RN', N'Foto_RN', N'Status_Penghuni', N'Nama_Penghuni', N'Kondisi_RN', N'Status_Hukum') AND tipe NOT IN (35, 48, 52, 56, 99, 104, 106, 108, 127, 167, 175, 231, 239));
IF @salah IS NOT NULL
BEGIN
    SET @salah = N'DIGITALISASI_RUMAH_NEGARA: kolom teks dari SLDK bertipe FLOAT, tanggal, uang, atau tipe lain yang bentuk tekstualnya berbeda antara aplikasi dan SQL Server (' + @salah + N'). Ubah ke teks pada query sumber. TIDAK ADA yang diubah.';
    THROW 50050, @salah, 1;
END;

INSERT INTO DIGITALISASI_STG.DIGITALISASI_RUMAH_NEGARA
    ([Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_Register_RN], [Kode_RN], [Uraian_RN], [NUP_RN], [Alamat_RN], [RTRW_RN], [KelurahanDesa_RN], [Kecamatan_RN], [KabKota_RN], [Provinsi_RN], [Luas_RN], [Foto_RN], [Status_Penghuni], [Nama_Penghuni], [Kondisi_RN], [Status_Hukum], [Latitude], [Longitude], [id_sinkron])
SELECT
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_UE1]), 10), -- Kode_UE1
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_Satker]), 40), -- Kode_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Nama_Satker]), 500), -- Nama_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_Register_RN]), 60), -- Kode_Register_RN
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_RN]), 30), -- Kode_RN
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Uraian_RN]), 500), -- Uraian_RN
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[NUP_RN]), 30), -- NUP_RN
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Alamat_RN]), 0), -- Alamat_RN
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[RTRW_RN]), 50), -- RTRW_RN
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[KelurahanDesa_RN]), 200), -- KelurahanDesa_RN
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kecamatan_RN]), 200), -- Kecamatan_RN
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[KabKota_RN]), 200), -- KabKota_RN
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Provinsi_RN]), 200), -- Provinsi_RN
    dbo.dm_desimal(CASE WHEN @tp_13 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Luas_RN] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Luas_RN], 3) END, 4), -- Luas_RN
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Foto_RN]), 0), -- Foto_RN
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Status_Penghuni]), 100), -- Status_Penghuni
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Nama_Penghuni]), 500), -- Nama_Penghuni
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kondisi_RN]), 100), -- Kondisi_RN
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Status_Hukum]), 0), -- Status_Hukum
    CASE WHEN k.lat IS NULL OR k.lng IS NULL OR (k.lat = 0 AND k.lng = 0) THEN NULL ELSE CAST(k.lat AS DECIMAL(10, 7)) END, -- Latitude
    CASE WHEN k.lat IS NULL OR k.lng IS NULL OR (k.lat = 0 AND k.lng = 0) THEN NULL ELSE CAST(k.lng AS DECIMAL(10, 7)) END, -- Longitude
    @run -- id_sinkron
FROM #raw_rumah_negara AS r
CROSS APPLY (SELECT lat = dbo.dm_koordinat(CASE WHEN @tp_19 = 1 THEN NULL ELSE CONVERT(NVARCHAR(4000), r.[GPS_Latitude], 3) END, CASE WHEN @tp_19 = 1 THEN CAST(r.[GPS_Latitude] AS FLOAT) END, 90), lng = dbo.dm_koordinat(CASE WHEN @tp_20 = 1 THEN NULL ELSE CONVERT(NVARCHAR(4000), r.[GPS_Longitude], 3) END, CASE WHEN @tp_20 = 1 THEN CAST(r.[GPS_Longitude] AS FLOAT) END, 180)) AS k
;

DROP TABLE IF EXISTS #raw_rumah_negara;
END;
GO

-- DIGITALISASI_MESS_RUMAH_NEGARA: Mess rumah negara (kd_brg 4010202016) KL 015 beserta status hukum dan jumlah kamar tidur tipe E dan D
CREATE OR ALTER PROCEDURE dbo.datamart_dg_muat_mess_rumah_negara @run BIGINT
AS
BEGIN
    SET NOCOUNT ON;

-- ---------------------------------------------------------------------------------------------------
-- QUERY ASLI (queries/mess_rumah_negara.sql). Satu-satunya tambahan: INTO #raw_mess_rumah_negara pada SELECT akhir.
-- ---------------------------------------------------------------------------------------------------
-- Mess Rumah Negara (kd_brg 4010202016). Query asli dari pengelola data.
-- Baris bertanda [PASTI] ditambahkan agar koordinat ikut tersinkron untuk peta; selebihnya tidak diubah.
SET NOCOUNT ON;

/* 1) Satu kali baca M_ASET (semua snapshot untuk kd_brg ini) */
DROP TABLE IF EXISTS #AsetAll, #AsetTarget;

SELECT A.id_aset, A.kd_satker, A.kd_brg, A.ur_sskel, A.no_aset, A.alamat, A.kd_rtrw,
       A.ur_kel, A.ur_kec, A.ur_kab, A.ur_prov, A.luas, A.kd_kondisi, A.rph_aset,
       A.kd_status_hukum, A.tgl_tarik,
       A.gps_latitude, A.gps_longitude   -- [PASTI] koordinat untuk peta
INTO #AsetAll
FROM DJKN.SIMAN2_M_ASET AS A
WHERE A.kd_brg = N'4010202016'
  AND A.status_bmn_yn = N'Y'
  AND A.kd_jns_bmn = 3
  AND A.kd_satker LIKE N'015%'
OPTION (RECOMPILE);

/* 2) Ambil snapshot terbaru + satker aktif, lalu index */
SELECT A.id_aset, A.kd_satker, S.ur_satker,
       S.alamat_satker, S.nm_kel, S.nm_kec, S.nm_kab_kota, S.nm_prov,
       A.kd_brg, A.ur_sskel, A.no_aset, A.alamat, A.kd_rtrw,
       A.ur_kel, A.ur_kec, A.ur_kab, A.ur_prov,
       A.luas, A.kd_kondisi, A.rph_aset, A.kd_status_hukum,
       A.gps_latitude, A.gps_longitude   -- [PASTI] koordinat untuk peta
INTO #AsetTarget
FROM #AsetAll AS A
INNER JOIN DJKN.SIMAN2_R_SATKER AS S
        ON S.kd_satker = A.kd_satker
       AND S.status_uakpb = N'AKTIF'
WHERE A.tgl_tarik = (SELECT MAX(tgl_tarik) FROM #AsetAll);

CREATE CLUSTERED INDEX CX_AT ON #AsetTarget (kd_satker, kd_brg, no_aset);
CREATE INDEX IX_AT_id ON #AsetTarget (id_aset);

/* 3) Query utama */
WITH InventarisRanked AS (
    SELECT I.id_aset, I.alamat, I.kd_rtrw, I.ur_kel, I.ur_kec, I.ur_kab, I.ur_prov, I.luas_bdg,
           ROW_NUMBER() OVER (PARTITION BY I.id_aset
                              ORDER BY I.TGL_TARIK DESC, I.updated_at DESC, I.id_inv_bangunan DESC) AS rn
    FROM DJKN.SIMAN2_T_INV_BANGUNAN AS I
    WHERE I.id_aset IN (SELECT id_aset FROM #AsetTarget)
), FotoRanked AS (
    SELECT P.id_aset,
           COALESCE(NULLIF(LTRIM(RTRIM(P.folder)), N''), NULLIF(LTRIM(RTRIM(P.filename)), N''),
                    NULLIF(LTRIM(RTRIM(P.nm_photo)), N'')) AS foto,
           ROW_NUMBER() OVER (PARTITION BY P.id_aset
                              ORDER BY CASE WHEN P.photo_utama_yn = N'Y' THEN 0 ELSE 1 END,
                                       P.TGL_TARIK DESC, P.tanggal DESC, P.id_photo DESC) AS rn
    FROM DJKN.SIMAN2_M_ASET_PHOTO AS P
    WHERE P.status_data = 1
      AND P.id_aset IN (SELECT id_aset FROM #AsetTarget)
), StatusHukumMap AS (
    SELECT LTRIM(RTRIM(R.kd_status_hukum)) AS kd_status_hukum,
           STRING_AGG(CONVERT(nvarchar(max), R.ur_status_hukum)
                      + CASE WHEN NULLIF(LTRIM(RTRIM(R.label)), N'') IS NOT NULL
                             THEN N' [' + R.label + N']' ELSE N'' END, N' | ')
                      WITHIN GROUP (ORDER BY R.id_status_hukum) AS ur_status_hukum
    FROM DJKN.SIMAN2_R_STATUS_HUKUM_NEW AS R
    WHERE R.TGL_TARIK = (SELECT MAX(TGL_TARIK) FROM DJKN.SIMAN2_R_STATUS_HUKUM_NEW)
    GROUP BY LTRIM(RTRIM(R.kd_status_hukum))
), HukumRanked AS (
    SELECT H.id_aset, NULLIF(LTRIM(RTRIM(H.kd_status_hukum)), N'') AS kd_status_hukum,
           ROW_NUMBER() OVER (PARTITION BY H.id_aset
                              ORDER BY CASE WHEN H.terakhir_yn = N'Y' THEN 0 ELSE 1 END,
                                       H.TGL_TARIK DESC, H.updated_at DESC, H.id_aset_hukum DESC) AS rn
    FROM DJKN.SIMAN2_M_ASET_HUKUM AS H
    WHERE H.status_data = 1
      AND H.id_aset IN (SELECT id_aset FROM #AsetTarget)
), RuangAgg AS (
    SELECT R.KODE_SATKER, R.KODE_BARANG, R.NUP,
           SUM(CASE WHEN UPPER(LTRIM(RTRIM(R.tipe_ruang))) = N'RUANG TIDUR'
                     AND UPPER(LTRIM(RTRIM(R.kode_ruang))) LIKE N'E%' THEN 1 ELSE 0 END) AS kamar_tipe_e,
           SUM(CASE WHEN UPPER(LTRIM(RTRIM(R.tipe_ruang))) = N'RUANG TIDUR'
                     AND UPPER(LTRIM(RTRIM(R.kode_ruang))) LIKE N'D%' THEN 1 ELSE 0 END) AS kamar_tipe_d
    FROM DJKN.VW_SIMAN_RUANG_GEDUNG_BANGUNAN_015 AS R
    INNER JOIN #AsetTarget AS A
            ON A.kd_satker = R.KODE_SATKER AND A.kd_brg = R.KODE_BARANG AND A.no_aset = R.NUP
    GROUP BY R.KODE_SATKER, R.KODE_BARANG, R.NUP
)
SELECT
    LEFT(A.kd_satker, 5) AS Kode_UE1,
    A.kd_satker          AS Kode_Satker,
    A.ur_satker          AS Nama_Satker,
    A.kd_brg             AS Kode_mess,
    A.ur_sskel           AS Uraian_mess,
    A.no_aset            AS NUP_mess,
    COALESCE(NULLIF(LTRIM(RTRIM(I.alamat)), N''), NULLIF(LTRIM(RTRIM(A.alamat)), N''), A.alamat_satker) AS Alamat_mess,
    COALESCE(NULLIF(LTRIM(RTRIM(I.kd_rtrw)), N''), NULLIF(LTRIM(RTRIM(A.kd_rtrw)), N'')) AS RTRW_mess,
    COALESCE(NULLIF(LTRIM(RTRIM(I.ur_kel)), N''), NULLIF(LTRIM(RTRIM(A.ur_kel)), N''), A.nm_kel) AS KelurahanDesa_mess,
    COALESCE(NULLIF(LTRIM(RTRIM(I.ur_kec)), N''), NULLIF(LTRIM(RTRIM(A.ur_kec)), N''), A.nm_kec) AS Kecamatan_mess,
    COALESCE(NULLIF(LTRIM(RTRIM(I.ur_kab)), N''), NULLIF(LTRIM(RTRIM(A.ur_kab)), N''), A.nm_kab_kota) AS KabKota_mess,
    COALESCE(NULLIF(LTRIM(RTRIM(I.ur_prov)), N''), NULLIF(LTRIM(RTRIM(A.ur_prov)), N''), A.nm_prov) AS Provinsi_mess,
    COALESCE(NULLIF(I.luas_bdg, 0), NULLIF(A.luas, 0), A.luas) AS Luas_mess,
    F.foto               AS Foto_mess,
    K.ur_kondisi         AS Kondisi_mess,
    A.rph_aset           AS Nilai_mess,
    SH.ur_status_hukum   AS Status_Hukum,
    COALESCE(R.kamar_tipe_e, 0) AS Kamar_tipe_E,
    COALESCE(R.kamar_tipe_d, 0) AS Kamar_tipe_D,
    A.gps_latitude       AS GPS_Latitude,    -- [PASTI] koordinat untuk peta
    A.gps_longitude      AS GPS_Longitude    -- [PASTI] koordinat untuk peta
INTO #raw_mess_rumah_negara
FROM #AsetTarget AS A
LEFT JOIN InventarisRanked AS I  ON I.id_aset = A.id_aset AND I.rn = 1
LEFT JOIN FotoRanked       AS F  ON F.id_aset = A.id_aset AND F.rn = 1
LEFT JOIN HukumRanked      AS H  ON H.id_aset = A.id_aset AND H.rn = 1
LEFT JOIN StatusHukumMap   AS SH ON SH.kd_status_hukum = COALESCE(H.kd_status_hukum, NULLIF(LTRIM(RTRIM(A.kd_status_hukum)), N''))
LEFT JOIN RuangAgg         AS R  ON R.KODE_SATKER = A.kd_satker AND R.KODE_BARANG = A.kd_brg AND R.NUP = A.no_aset
LEFT JOIN DJKN.SIMAN2_R_KONDISI AS K ON K.kd_kondisi = A.kd_kondisi
ORDER BY A.kd_satker, A.kd_brg, A.no_aset
OPTION (RECOMPILE);

-- ---------------------------------------------------------------------------------------------------
-- KONVERSI ke kolom DIGITALISASI_MESS_RUMAH_NEGARA (padanan digitalisasi.convertValue dan fixCoordinates)
-- ---------------------------------------------------------------------------------------------------
-- Tipe kolom mentah (system_type_id): kolom FLOAT/REAL (62, 59) dibaca sebagai bilangan, bukan sebagai teks.
DECLARE @jenis TABLE (kolom SYSNAME NOT NULL PRIMARY KEY, tipe INT NOT NULL);
INSERT @jenis (kolom, tipe) SELECT c.name, c.system_type_id FROM tempdb.sys.columns AS c WHERE c.object_id = OBJECT_ID(N'tempdb..#raw_mess_rumah_negara');
DECLARE @tp_12 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Luas_mess'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_15 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Nilai_mess'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_17 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Kamar_tipe_E'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_18 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'Kamar_tipe_D'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_19 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'GPS_Latitude'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @tp_20 TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = N'GPS_Longitude'), 0) IN (62, 59) THEN 1 ELSE 0 END,
        @salah NVARCHAR(MAX) = (SELECT STRING_AGG(kolom, N', ') FROM @jenis WHERE kolom IN (N'Kode_UE1', N'Kode_Satker', N'Nama_Satker', N'Kode_mess', N'Uraian_mess', N'NUP_mess', N'Alamat_mess', N'RTRW_mess', N'KelurahanDesa_mess', N'Kecamatan_mess', N'KabKota_mess', N'Provinsi_mess', N'Foto_mess', N'Kondisi_mess', N'Status_Hukum') AND tipe NOT IN (35, 48, 52, 56, 99, 104, 106, 108, 127, 167, 175, 231, 239));
IF @salah IS NOT NULL
BEGIN
    SET @salah = N'DIGITALISASI_MESS_RUMAH_NEGARA: kolom teks dari SLDK bertipe FLOAT, tanggal, uang, atau tipe lain yang bentuk tekstualnya berbeda antara aplikasi dan SQL Server (' + @salah + N'). Ubah ke teks pada query sumber. TIDAK ADA yang diubah.';
    THROW 50050, @salah, 1;
END;

INSERT INTO DIGITALISASI_STG.DIGITALISASI_MESS_RUMAH_NEGARA
    ([Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_mess], [Uraian_mess], [NUP_mess], [Alamat_mess], [RTRW_mess], [KelurahanDesa_mess], [Kecamatan_mess], [KabKota_mess], [Provinsi_mess], [Luas_mess], [Foto_mess], [Kondisi_mess], [Nilai_mess], [Status_Hukum], [Kamar_tipe_E], [Kamar_tipe_D], [Latitude], [Longitude], [id_sinkron])
SELECT
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_UE1]), 10), -- Kode_UE1
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_Satker]), 40), -- Kode_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Nama_Satker]), 500), -- Nama_Satker
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kode_mess]), 30), -- Kode_mess
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Uraian_mess]), 500), -- Uraian_mess
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[NUP_mess]), 30), -- NUP_mess
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Alamat_mess]), 0), -- Alamat_mess
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[RTRW_mess]), 50), -- RTRW_mess
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[KelurahanDesa_mess]), 200), -- KelurahanDesa_mess
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kecamatan_mess]), 200), -- Kecamatan_mess
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[KabKota_mess]), 200), -- KabKota_mess
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Provinsi_mess]), 200), -- Provinsi_mess
    dbo.dm_desimal(CASE WHEN @tp_12 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Luas_mess] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Luas_mess], 3) END, 4), -- Luas_mess
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Foto_mess]), 0), -- Foto_mess
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Kondisi_mess]), 100), -- Kondisi_mess
    dbo.dm_desimal(CASE WHEN @tp_15 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Nilai_mess] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Nilai_mess], 3) END, 2), -- Nilai_mess
    dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Status_Hukum]), 0), -- Status_Hukum
    dbo.dm_bilangan(CASE WHEN @tp_17 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Kamar_tipe_E] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Kamar_tipe_E], 3) END), -- Kamar_tipe_E
    dbo.dm_bilangan(CASE WHEN @tp_18 = 1 THEN dbo.dm_float_ke_teks(CAST(r.[Kamar_tipe_D] AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), r.[Kamar_tipe_D], 3) END), -- Kamar_tipe_D
    CASE WHEN k.lat IS NULL OR k.lng IS NULL OR (k.lat = 0 AND k.lng = 0) THEN NULL ELSE CAST(k.lat AS DECIMAL(10, 7)) END, -- Latitude
    CASE WHEN k.lat IS NULL OR k.lng IS NULL OR (k.lat = 0 AND k.lng = 0) THEN NULL ELSE CAST(k.lng AS DECIMAL(10, 7)) END, -- Longitude
    @run -- id_sinkron
FROM #raw_mess_rumah_negara AS r
CROSS APPLY (SELECT lat = dbo.dm_koordinat(CASE WHEN @tp_19 = 1 THEN NULL ELSE CONVERT(NVARCHAR(4000), r.[GPS_Latitude], 3) END, CASE WHEN @tp_19 = 1 THEN CAST(r.[GPS_Latitude] AS FLOAT) END, 90), lng = dbo.dm_koordinat(CASE WHEN @tp_20 = 1 THEN NULL ELSE CONVERT(NVARCHAR(4000), r.[GPS_Longitude], 3) END, CASE WHEN @tp_20 = 1 THEN CAST(r.[GPS_Longitude] AS FLOAT) END, 180)) AS k
ORDER BY r.[Kode_Satker], r.[Kode_mess], r.[NUP_mess]
;

DROP TABLE IF EXISTS #raw_mess_rumah_negara;
END;
GO

-- ============================================================================================================================================
-- PROSEDUR INDUK: dbo.datamart_digitalisasi_segarkan
-- ============================================================================================================================================
CREATE OR ALTER PROCEDURE dbo.datamart_digitalisasi_segarkan
AS
BEGIN
    SET NOCOUNT ON;
    SET XACT_ABORT ON;

    DECLARE @kunci INT, @run BIGINT, @sql NVARCHAR(MAX), @pesan NVARCHAR(2000), @total BIGINT, @sumber NVARCHAR(400);

    IF OBJECT_ID(N'dbo.digitalisasi_sync_log', N'U') IS NOT NULL
        THROW 50030, N'Database ini tampaknya database aplikasi PASTI (ada tabel digitalisasi_sync_log); datamart harus database terpisah. TIDAK ADA yang diubah.', 1;

    -- Satu proses pada satu waktu (kunci aplikasi bertipe sesi; lepas sendiri bila sambungan putus).
    EXEC @kunci = sp_getapplock @Resource = N'datamart_digitalisasi', @LockMode = N'Exclusive', @LockOwner = N'Session', @LockTimeout = 0;
    IF @kunci < 0
        THROW 50040, N'Proses datamart digitalisasi lain sedang berjalan; proses ini dihentikan.', 1;

    BEGIN TRY
        -- Proses sebelumnya yang masih berstatus berjalan padahal kunci kita dapat = terhenti di tengah jalan.
        UPDATE dbo.datamart_digitalisasi_run SET status = N'gagal', selesai = SYSUTCDATETIME(), pesan = N'Terhenti sebelum selesai (proses berhenti atau sambungan putus).' WHERE status = N'berjalan';

        -- Sisa staging dari proses yang gagal.
        SET @sql = N'';
        SELECT @sql += N'DROP TABLE DIGITALISASI_STG.' + QUOTENAME(name) + N';' FROM sys.tables WHERE schema_id = SCHEMA_ID(N'DIGITALISASI_STG');
        EXEC (@sql);

        SELECT @sumber = LEFT(base_object_name, 400) FROM sys.synonyms WHERE schema_id = SCHEMA_ID(N'DJKN') AND name = N'SIMAN2_M_ASET';
        INSERT dbo.datamart_digitalisasi_run (status, sumber) VALUES (N'berjalan', ISNULL(@sumber, N'langsung: DJKN.SIMAN2_* di database ini'));
        SET @run = SCOPE_IDENTITY();

        -- Tabel staging (struktur sama dengan tabel hidup).
        CREATE TABLE DIGITALISASI_STG.DIGITALISASI_SATKER (
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

        CREATE TABLE DIGITALISASI_STG.DIGITALISASI_TANAH (
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

        CREATE TABLE DIGITALISASI_STG.DIGITALISASI_GEDUNG_KANTOR_UTAMA (
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

        CREATE TABLE DIGITALISASI_STG.DIGITALISASI_GEDUNG_LAINNYA (
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

        CREATE TABLE DIGITALISASI_STG.DIGITALISASI_RUSUNARA (
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

        CREATE TABLE DIGITALISASI_STG.DIGITALISASI_RUMAH_NEGARA (
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
            Nama_Penghuni NVARCHAR(500) NULL,  -- data pribadi (nama penghuni): batasi akses ke datamart
            Kondisi_RN NVARCHAR(100) NULL,
            Status_Hukum NVARCHAR(MAX) NULL,
            Latitude DECIMAL(10, 7) NULL,
            Longitude DECIMAL(10, 7) NULL,
            id_sinkron BIGINT NULL,
            synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
        );

        CREATE TABLE DIGITALISASI_STG.DIGITALISASI_MESS_RUMAH_NEGARA (
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

        -- Muat ketujuh dataset ke staging. Galat dari prosedur muat (kolom atau tabel SLDK hilang, akses ditolak, SLDK tidak terjangkau) tercatat dan menghentikan proses.
        EXEC dbo.datamart_dg_muat_satker @run = @run;
        EXEC dbo.datamart_dg_muat_tanah @run = @run;
        EXEC dbo.datamart_dg_muat_gedung_kantor_utama @run = @run;
        EXEC dbo.datamart_dg_muat_gedung_lainnya @run = @run;
        EXEC dbo.datamart_dg_muat_rusunara @run = @run;
        EXEC dbo.datamart_dg_muat_rumah_negara @run = @run;
        EXEC dbo.datamart_dg_muat_mess_rumah_negara @run = @run;

        -- Pengaman: hasil kosong hampir pasti kegagalan di hulu (akses, filter), bukan kenyataan bahwa semua aset hilang. Datamart yang lama tidak diganti.
        IF NOT EXISTS (SELECT 1 FROM DIGITALISASI_STG.DIGITALISASI_SATKER) AND EXISTS (SELECT 1 FROM dbo.DIGITALISASI_SATKER)
            THROW 50041, N'DIGITALISASI_SATKER: SLDK mengembalikan 0 baris sedangkan datamart sudah berisi data; datamart lama TIDAK diganti. Periksa akses ke SLDK dan filter query.', 1;
        IF NOT EXISTS (SELECT 1 FROM DIGITALISASI_STG.DIGITALISASI_TANAH) AND EXISTS (SELECT 1 FROM dbo.DIGITALISASI_TANAH)
            THROW 50041, N'DIGITALISASI_TANAH: SLDK mengembalikan 0 baris sedangkan datamart sudah berisi data; datamart lama TIDAK diganti. Periksa akses ke SLDK dan filter query.', 1;
        IF NOT EXISTS (SELECT 1 FROM DIGITALISASI_STG.DIGITALISASI_GEDUNG_KANTOR_UTAMA) AND EXISTS (SELECT 1 FROM dbo.DIGITALISASI_GEDUNG_KANTOR_UTAMA)
            THROW 50041, N'DIGITALISASI_GEDUNG_KANTOR_UTAMA: SLDK mengembalikan 0 baris sedangkan datamart sudah berisi data; datamart lama TIDAK diganti. Periksa akses ke SLDK dan filter query.', 1;
        IF NOT EXISTS (SELECT 1 FROM DIGITALISASI_STG.DIGITALISASI_GEDUNG_LAINNYA) AND EXISTS (SELECT 1 FROM dbo.DIGITALISASI_GEDUNG_LAINNYA)
            THROW 50041, N'DIGITALISASI_GEDUNG_LAINNYA: SLDK mengembalikan 0 baris sedangkan datamart sudah berisi data; datamart lama TIDAK diganti. Periksa akses ke SLDK dan filter query.', 1;
        IF NOT EXISTS (SELECT 1 FROM DIGITALISASI_STG.DIGITALISASI_RUSUNARA) AND EXISTS (SELECT 1 FROM dbo.DIGITALISASI_RUSUNARA)
            THROW 50041, N'DIGITALISASI_RUSUNARA: SLDK mengembalikan 0 baris sedangkan datamart sudah berisi data; datamart lama TIDAK diganti. Periksa akses ke SLDK dan filter query.', 1;
        IF NOT EXISTS (SELECT 1 FROM DIGITALISASI_STG.DIGITALISASI_RUMAH_NEGARA) AND EXISTS (SELECT 1 FROM dbo.DIGITALISASI_RUMAH_NEGARA)
            THROW 50041, N'DIGITALISASI_RUMAH_NEGARA: SLDK mengembalikan 0 baris sedangkan datamart sudah berisi data; datamart lama TIDAK diganti. Periksa akses ke SLDK dan filter query.', 1;
        IF NOT EXISTS (SELECT 1 FROM DIGITALISASI_STG.DIGITALISASI_MESS_RUMAH_NEGARA) AND EXISTS (SELECT 1 FROM dbo.DIGITALISASI_MESS_RUMAH_NEGARA)
            THROW 50041, N'DIGITALISASI_MESS_RUMAH_NEGARA: SLDK mengembalikan 0 baris sedangkan datamart sudah berisi data; datamart lama TIDAK diganti. Periksa akses ke SLDK dan filter query.', 1;

        -- Indeks pada staging (sama dengan tabel hidup).
        CREATE INDEX idx_digitalisasi_satker_kode_ue1 ON DIGITALISASI_STG.DIGITALISASI_SATKER(Kode_UE1);
        CREATE INDEX idx_digitalisasi_satker_kode_satker ON DIGITALISASI_STG.DIGITALISASI_SATKER(Kode_Satker);
        CREATE INDEX idx_digitalisasi_satker_provinsi_satker ON DIGITALISASI_STG.DIGITALISASI_SATKER(Provinsi_Satker);
        CREATE INDEX idx_digitalisasi_tanah_kode_ue1 ON DIGITALISASI_STG.DIGITALISASI_TANAH(Kode_UE1);
        CREATE INDEX idx_digitalisasi_tanah_kode_satker ON DIGITALISASI_STG.DIGITALISASI_TANAH(Kode_Satker);
        CREATE INDEX idx_digitalisasi_tanah_provinsi_tanah ON DIGITALISASI_STG.DIGITALISASI_TANAH(Provinsi_tanah);
        CREATE INDEX idx_digitalisasi_gedung_kantor_utama_kode_ue1 ON DIGITALISASI_STG.DIGITALISASI_GEDUNG_KANTOR_UTAMA(Kode_UE1);
        CREATE INDEX idx_digitalisasi_gedung_kantor_utama_kode_satker ON DIGITALISASI_STG.DIGITALISASI_GEDUNG_KANTOR_UTAMA(Kode_Satker);
        CREATE INDEX idx_digitalisasi_gedung_kantor_utama_provinsi_bangunan ON DIGITALISASI_STG.DIGITALISASI_GEDUNG_KANTOR_UTAMA(Provinsi_bangunan);
        CREATE INDEX idx_digitalisasi_gedung_lainnya_kode_ue1 ON DIGITALISASI_STG.DIGITALISASI_GEDUNG_LAINNYA(Kode_UE1);
        CREATE INDEX idx_digitalisasi_gedung_lainnya_kode_satker ON DIGITALISASI_STG.DIGITALISASI_GEDUNG_LAINNYA(Kode_Satker);
        CREATE INDEX idx_digitalisasi_gedung_lainnya_provinsi_bangunan ON DIGITALISASI_STG.DIGITALISASI_GEDUNG_LAINNYA(Provinsi_bangunan);
        CREATE INDEX idx_digitalisasi_rusunara_kode_ue1 ON DIGITALISASI_STG.DIGITALISASI_RUSUNARA(Kode_UE1);
        CREATE INDEX idx_digitalisasi_rusunara_kode_satker ON DIGITALISASI_STG.DIGITALISASI_RUSUNARA(Kode_Satker);
        CREATE INDEX idx_digitalisasi_rusunara_provinsi_rusun ON DIGITALISASI_STG.DIGITALISASI_RUSUNARA(Provinsi_Rusun);
        CREATE INDEX idx_digitalisasi_rumah_negara_kode_ue1 ON DIGITALISASI_STG.DIGITALISASI_RUMAH_NEGARA(Kode_UE1);
        CREATE INDEX idx_digitalisasi_rumah_negara_kode_satker ON DIGITALISASI_STG.DIGITALISASI_RUMAH_NEGARA(Kode_Satker);
        CREATE INDEX idx_digitalisasi_rumah_negara_provinsi_rn ON DIGITALISASI_STG.DIGITALISASI_RUMAH_NEGARA(Provinsi_RN);
        CREATE INDEX idx_digitalisasi_mess_rumah_negara_kode_ue1 ON DIGITALISASI_STG.DIGITALISASI_MESS_RUMAH_NEGARA(Kode_UE1);
        CREATE INDEX idx_digitalisasi_mess_rumah_negara_kode_satker ON DIGITALISASI_STG.DIGITALISASI_MESS_RUMAH_NEGARA(Kode_Satker);
        CREATE INDEX idx_digitalisasi_mess_rumah_negara_provinsi_mess ON DIGITALISASI_STG.DIGITALISASI_MESS_RUMAH_NEGARA(Provinsi_mess);

        -- Ganti isi tabel hidup dalam SATU transaksi. Hak akses, indeks, dan definisi tabel hidup tidak berubah.
        BEGIN TRANSACTION;
        TRUNCATE TABLE dbo.DIGITALISASI_SATKER;
        INSERT INTO dbo.DIGITALISASI_SATKER ([Kode_UE1], [Kode_Satker], [Jenis_Satker], [Nama_Satker], [Alamat_Satker], [KelurahanDesa_Satker], [Kecamatan_Satker], [KabKota_Satker], [Provinsi_Satker], [Status_Gedung_Kantor], [Foto], [Jumlah_KDJ], [Kondisi_KDJ], [Jumlah_KDO], [Kondisi_KDO], [id_sinkron], [synced_at])
        SELECT [Kode_UE1], [Kode_Satker], [Jenis_Satker], [Nama_Satker], [Alamat_Satker], [KelurahanDesa_Satker], [Kecamatan_Satker], [KabKota_Satker], [Provinsi_Satker], [Status_Gedung_Kantor], [Foto], [Jumlah_KDJ], [Kondisi_KDJ], [Jumlah_KDO], [Kondisi_KDO], [id_sinkron], [synced_at] FROM DIGITALISASI_STG.DIGITALISASI_SATKER ORDER BY id;
        TRUNCATE TABLE dbo.DIGITALISASI_TANAH;
        INSERT INTO dbo.DIGITALISASI_TANAH ([id_aset_tanah], [kode_register_tanah], [Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_tanah], [Uraian_tanah], [No_aset], [Alamat_tanah], [RTRW_tanah], [KelurahanDesa_tanah], [Kecamatan_tanah], [KabKota_tanah], [Provinsi_tanah], [Luas_Tanah], [Luas_Bangunan], [Jumlah_Bangunan], [Status_hukum], [Foto], [Kondisi_Tanah], [Nilai_Tanah], [Latitude], [Longitude], [id_sinkron], [synced_at])
        SELECT [id_aset_tanah], [kode_register_tanah], [Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_tanah], [Uraian_tanah], [No_aset], [Alamat_tanah], [RTRW_tanah], [KelurahanDesa_tanah], [Kecamatan_tanah], [KabKota_tanah], [Provinsi_tanah], [Luas_Tanah], [Luas_Bangunan], [Jumlah_Bangunan], [Status_hukum], [Foto], [Kondisi_Tanah], [Nilai_Tanah], [Latitude], [Longitude], [id_sinkron], [synced_at] FROM DIGITALISASI_STG.DIGITALISASI_TANAH ORDER BY id;
        TRUNCATE TABLE dbo.DIGITALISASI_GEDUNG_KANTOR_UTAMA;
        INSERT INTO dbo.DIGITALISASI_GEDUNG_KANTOR_UTAMA ([Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_Register_bangunan], [Kode_bangunan], [Uraian_bangunan], [NUP_bangunan], [Alamat_bangunan], [RTRW_bangunan], [KelurahanDesa_bangunan], [Kecamatan_bangunan], [KabKota_bangunan], [Provinsi_bangunan], [Luas_Bangunan], [Foto_bangunan], [Kondisi_Bangunan], [Nilai_Bangunan], [Status_Asuransi], [Kode_tanah], [Uraian_tanah], [No_aset_tanah], [Latitude], [Longitude], [id_sinkron], [synced_at])
        SELECT [Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_Register_bangunan], [Kode_bangunan], [Uraian_bangunan], [NUP_bangunan], [Alamat_bangunan], [RTRW_bangunan], [KelurahanDesa_bangunan], [Kecamatan_bangunan], [KabKota_bangunan], [Provinsi_bangunan], [Luas_Bangunan], [Foto_bangunan], [Kondisi_Bangunan], [Nilai_Bangunan], [Status_Asuransi], [Kode_tanah], [Uraian_tanah], [No_aset_tanah], [Latitude], [Longitude], [id_sinkron], [synced_at] FROM DIGITALISASI_STG.DIGITALISASI_GEDUNG_KANTOR_UTAMA ORDER BY id;
        TRUNCATE TABLE dbo.DIGITALISASI_GEDUNG_LAINNYA;
        INSERT INTO dbo.DIGITALISASI_GEDUNG_LAINNYA ([Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_Register_bangunan], [Kode_bangunan], [Uraian_bangunan], [NUP_bangunan], [Alamat_bangunan], [RTRW_bangunan], [KelurahanDesa_bangunan], [Kecamatan_bangunan], [KabKota_bangunan], [Provinsi_bangunan], [Luas_Bangunan], [Foto_bangunan], [Fungsi], [Status_Hukum], [Nama_Pengguna], [Kondisi_Bangunan], [Nilai_Bangunan], [Status_Asuransi], [Latitude], [Longitude], [id_sinkron], [synced_at])
        SELECT [Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_Register_bangunan], [Kode_bangunan], [Uraian_bangunan], [NUP_bangunan], [Alamat_bangunan], [RTRW_bangunan], [KelurahanDesa_bangunan], [Kecamatan_bangunan], [KabKota_bangunan], [Provinsi_bangunan], [Luas_Bangunan], [Foto_bangunan], [Fungsi], [Status_Hukum], [Nama_Pengguna], [Kondisi_Bangunan], [Nilai_Bangunan], [Status_Asuransi], [Latitude], [Longitude], [id_sinkron], [synced_at] FROM DIGITALISASI_STG.DIGITALISASI_GEDUNG_LAINNYA ORDER BY id;
        TRUNCATE TABLE dbo.DIGITALISASI_RUSUNARA;
        INSERT INTO dbo.DIGITALISASI_RUSUNARA ([Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_Rusun], [Uraian_Rusun], [NUP_Rusun], [Alamat_Rusun], [RTRW_Rusun], [KelurahanDesa_Rusun], [Kecamatan_Rusun], [KabKota_Rusun], [Provinsi_Rusun], [Luas_Rusun], [Foto_Rusun], [Kondisi_Rusun], [Nilai_Rusun], [Kamar_tipe_E], [Kamar_tipe_D], [Kamar_tipe_C], [Latitude], [Longitude], [id_sinkron], [synced_at])
        SELECT [Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_Rusun], [Uraian_Rusun], [NUP_Rusun], [Alamat_Rusun], [RTRW_Rusun], [KelurahanDesa_Rusun], [Kecamatan_Rusun], [KabKota_Rusun], [Provinsi_Rusun], [Luas_Rusun], [Foto_Rusun], [Kondisi_Rusun], [Nilai_Rusun], [Kamar_tipe_E], [Kamar_tipe_D], [Kamar_tipe_C], [Latitude], [Longitude], [id_sinkron], [synced_at] FROM DIGITALISASI_STG.DIGITALISASI_RUSUNARA ORDER BY id;
        TRUNCATE TABLE dbo.DIGITALISASI_RUMAH_NEGARA;
        INSERT INTO dbo.DIGITALISASI_RUMAH_NEGARA ([Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_Register_RN], [Kode_RN], [Uraian_RN], [NUP_RN], [Alamat_RN], [RTRW_RN], [KelurahanDesa_RN], [Kecamatan_RN], [KabKota_RN], [Provinsi_RN], [Luas_RN], [Foto_RN], [Status_Penghuni], [Nama_Penghuni], [Kondisi_RN], [Status_Hukum], [Latitude], [Longitude], [id_sinkron], [synced_at])
        SELECT [Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_Register_RN], [Kode_RN], [Uraian_RN], [NUP_RN], [Alamat_RN], [RTRW_RN], [KelurahanDesa_RN], [Kecamatan_RN], [KabKota_RN], [Provinsi_RN], [Luas_RN], [Foto_RN], [Status_Penghuni], [Nama_Penghuni], [Kondisi_RN], [Status_Hukum], [Latitude], [Longitude], [id_sinkron], [synced_at] FROM DIGITALISASI_STG.DIGITALISASI_RUMAH_NEGARA ORDER BY id;
        TRUNCATE TABLE dbo.DIGITALISASI_MESS_RUMAH_NEGARA;
        INSERT INTO dbo.DIGITALISASI_MESS_RUMAH_NEGARA ([Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_mess], [Uraian_mess], [NUP_mess], [Alamat_mess], [RTRW_mess], [KelurahanDesa_mess], [Kecamatan_mess], [KabKota_mess], [Provinsi_mess], [Luas_mess], [Foto_mess], [Kondisi_mess], [Nilai_mess], [Status_Hukum], [Kamar_tipe_E], [Kamar_tipe_D], [Latitude], [Longitude], [id_sinkron], [synced_at])
        SELECT [Kode_UE1], [Kode_Satker], [Nama_Satker], [Kode_mess], [Uraian_mess], [NUP_mess], [Alamat_mess], [RTRW_mess], [KelurahanDesa_mess], [Kecamatan_mess], [KabKota_mess], [Provinsi_mess], [Luas_mess], [Foto_mess], [Kondisi_mess], [Nilai_mess], [Status_Hukum], [Kamar_tipe_E], [Kamar_tipe_D], [Latitude], [Longitude], [id_sinkron], [synced_at] FROM DIGITALISASI_STG.DIGITALISASI_MESS_RUMAH_NEGARA ORDER BY id;
        COMMIT TRANSACTION;

        -- Jejak jumlah baris.
        INSERT dbo.datamart_digitalisasi_tabel (run_id, tabel, baris, koordinat) SELECT @run, N'DIGITALISASI_SATKER', COUNT_BIG(*), NULL FROM dbo.DIGITALISASI_SATKER;
        INSERT dbo.datamart_digitalisasi_tabel (run_id, tabel, baris, koordinat) SELECT @run, N'DIGITALISASI_TANAH', COUNT_BIG(*), SUM(CASE WHEN Latitude IS NOT NULL THEN 1 ELSE 0 END) FROM dbo.DIGITALISASI_TANAH;
        INSERT dbo.datamart_digitalisasi_tabel (run_id, tabel, baris, koordinat) SELECT @run, N'DIGITALISASI_GEDUNG_KANTOR_UTAMA', COUNT_BIG(*), SUM(CASE WHEN Latitude IS NOT NULL THEN 1 ELSE 0 END) FROM dbo.DIGITALISASI_GEDUNG_KANTOR_UTAMA;
        INSERT dbo.datamart_digitalisasi_tabel (run_id, tabel, baris, koordinat) SELECT @run, N'DIGITALISASI_GEDUNG_LAINNYA', COUNT_BIG(*), SUM(CASE WHEN Latitude IS NOT NULL THEN 1 ELSE 0 END) FROM dbo.DIGITALISASI_GEDUNG_LAINNYA;
        INSERT dbo.datamart_digitalisasi_tabel (run_id, tabel, baris, koordinat) SELECT @run, N'DIGITALISASI_RUSUNARA', COUNT_BIG(*), SUM(CASE WHEN Latitude IS NOT NULL THEN 1 ELSE 0 END) FROM dbo.DIGITALISASI_RUSUNARA;
        INSERT dbo.datamart_digitalisasi_tabel (run_id, tabel, baris, koordinat) SELECT @run, N'DIGITALISASI_RUMAH_NEGARA', COUNT_BIG(*), SUM(CASE WHEN Latitude IS NOT NULL THEN 1 ELSE 0 END) FROM dbo.DIGITALISASI_RUMAH_NEGARA;
        INSERT dbo.datamart_digitalisasi_tabel (run_id, tabel, baris, koordinat) SELECT @run, N'DIGITALISASI_MESS_RUMAH_NEGARA', COUNT_BIG(*), SUM(CASE WHEN Latitude IS NOT NULL THEN 1 ELSE 0 END) FROM dbo.DIGITALISASI_MESS_RUMAH_NEGARA;

        SELECT @total = SUM(baris) FROM dbo.datamart_digitalisasi_tabel WHERE run_id = @run;
        UPDATE dbo.datamart_digitalisasi_run SET status = N'sukses', selesai = SYSUTCDATETIME(),
               pesan = CONVERT(NVARCHAR(20), @total) + N' baris di ketujuh tabel DIGITALISASI_*; rincian di dbo.datamart_digitalisasi_tabel.'
        WHERE id = @run;

        SET @sql = N'';
        SELECT @sql += N'DROP TABLE DIGITALISASI_STG.' + QUOTENAME(name) + N';' FROM sys.tables WHERE schema_id = SCHEMA_ID(N'DIGITALISASI_STG');
        EXEC (@sql);

        EXEC sp_releaseapplock @Resource = N'datamart_digitalisasi', @LockOwner = N'Session';
        PRINT N'Datamart Digitalisasi Aset selesai. Proses #' + CONVERT(NVARCHAR(20), @run) + N'; rincian: SELECT * FROM dbo.datamart_digitalisasi_tabel WHERE run_id = ' + CONVERT(NVARCHAR(20), @run) + N'.';
    END TRY
    BEGIN CATCH
        IF XACT_STATE() <> 0 ROLLBACK TRANSACTION;

        SET @pesan = LEFT(ISNULL(ERROR_MESSAGE(), N'Galat tidak diketahui') + N' (' + ISNULL(ERROR_PROCEDURE(), N'-') + N', baris ' + CONVERT(NVARCHAR(10), ERROR_LINE()) + N'). Sumber SLDK: ' + ISNULL(@sumber, N'-') + N'.', 2000);
        -- Prosedur muat yang tidak ada = pembuatannya gagal lebih dulu (kolom atau tabel sumber yang tidak ada tertangkap saat CREATE PROCEDURE, bukan saat dijalankan).
        IF ERROR_NUMBER() = 2812
            SET @pesan = LEFT(@pesan + N' Prosedur muat tidak berhasil dibuat: lihat galat SEBELUMNYA pada keluaran script (biasanya kolom atau tabel sumber SLDK yang tidak ada atau berubah nama).', 2000);
        IF @run IS NOT NULL
            UPDATE dbo.datamart_digitalisasi_run SET status = N'gagal', selesai = SYSUTCDATETIME(), pesan = @pesan WHERE id = @run;

        -- Buang staging yang setengah jadi; tabel hidup (datamart yang berlaku) tidak disentuh.
        BEGIN TRY
            SET @sql = N'';
            SELECT @sql += N'DROP TABLE DIGITALISASI_STG.' + QUOTENAME(name) + N';' FROM sys.tables WHERE schema_id = SCHEMA_ID(N'DIGITALISASI_STG');
            EXEC (@sql);
        END TRY
        BEGIN CATCH
        END CATCH;
        EXEC sp_releaseapplock @Resource = N'datamart_digitalisasi', @LockOwner = N'Session';

        THROW;
    END CATCH;
END;
GO

-- Jalankan sekarang (pemasangan pertama membuat tabel; pemasangan ulang menyegarkannya).
EXEC dbo.datamart_digitalisasi_segarkan;
GO

-- Pembersihan: bila @Bersihkan = 1 dan penyegaran terakhir BERHASIL, buang fungsi, prosedur, dan sinonim sementara. Yang tersisa hanya tabel (tujuh DIGITALISASI_* dan dua tabel jejak).
-- Bila penyegaran gagal, semuanya dipertahankan supaya penyebabnya dapat diperiksa.
IF OBJECT_ID(N'tempdb..#opsi') IS NOT NULL
   AND EXISTS (SELECT 1 FROM #opsi WHERE bersihkan = 1)
   AND (SELECT TOP (1) status FROM dbo.datamart_digitalisasi_run ORDER BY id DESC) = N'sukses'
BEGIN
    DECLARE @sql NVARCHAR(MAX) = N'';
    DROP PROCEDURE IF EXISTS dbo.datamart_digitalisasi_segarkan;
    DROP PROCEDURE IF EXISTS dbo.datamart_dg_muat_satker;
    DROP PROCEDURE IF EXISTS dbo.datamart_dg_muat_tanah;
    DROP PROCEDURE IF EXISTS dbo.datamart_dg_muat_gedung_kantor_utama;
    DROP PROCEDURE IF EXISTS dbo.datamart_dg_muat_gedung_lainnya;
    DROP PROCEDURE IF EXISTS dbo.datamart_dg_muat_rusunara;
    DROP PROCEDURE IF EXISTS dbo.datamart_dg_muat_rumah_negara;
    DROP PROCEDURE IF EXISTS dbo.datamart_dg_muat_mess_rumah_negara;
    DROP FUNCTION IF EXISTS dbo.dm_pangkas;
    DROP FUNCTION IF EXISTS dbo.dm_teks;
    DROP FUNCTION IF EXISTS dbo.dm_polos_lambat;
    DROP FUNCTION IF EXISTS dbo.dm_pangkas_pendek;
    DROP FUNCTION IF EXISTS dbo.dm_polos;
    DROP FUNCTION IF EXISTS dbo.dm_angka;
    DROP FUNCTION IF EXISTS dbo.dm_bilangan;
    DROP FUNCTION IF EXISTS dbo.dm_desimal;
    DROP FUNCTION IF EXISTS dbo.dm_float_ke_teks;
    DROP FUNCTION IF EXISTS dbo.dm_koordinat;

    -- sinonim DJKN.* buatan script (tabel sungguhan di DJKN tidak disentuh), lalu skema yang kosong
    SELECT @sql += N'DROP SYNONYM DJKN.' + QUOTENAME(name) + N';' FROM sys.synonyms WHERE schema_id = SCHEMA_ID(N'DJKN');
    EXEC (@sql);
    IF SCHEMA_ID(N'DJKN') IS NOT NULL AND NOT EXISTS (SELECT 1 FROM sys.objects WHERE schema_id = SCHEMA_ID(N'DJKN')) DROP SCHEMA DJKN;
    IF SCHEMA_ID(N'DIGITALISASI_STG') IS NOT NULL AND NOT EXISTS (SELECT 1 FROM sys.objects WHERE schema_id = SCHEMA_ID(N'DIGITALISASI_STG')) DROP SCHEMA DIGITALISASI_STG;
    PRINT N'Fungsi, prosedur, dan sinonim sementara dibuang; yang tersisa hanya tabel. Penyegaran berikutnya: jalankan ulang script ini.';
END;
DROP TABLE IF EXISTS #opsi;
GO

SET NOEXEC OFF;
GO
