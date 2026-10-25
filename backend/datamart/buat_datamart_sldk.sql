-- ============================================================================================================================================
-- DATAMART SLDK untuk Digitalisasi Aset PASTI
--
-- Menyalin HANYA data SLDK yang dibutuhkan ketujuh query Digitalisasi Aset (backend/digitalisasi/queries/*.sql) ke database datamart, dengan NAMA SKEMA, TABEL, dan KOLOM yang
-- sama seperti di SLDK (skema DJKN, tabel SIMAN2_*). Akibatnya query Digitalisasi Aset berjalan TANPA DIUBAH terhadap datamart: bila SLDK tidak dapat dihubungi, arahkan
-- SLDK_DB_HOST / SLDK_DB_PORT / SLDK_DB_USER / SLDK_DB_PASSWORD / SLDK_DB_NAME di backend/.env ke datamart ini. Lihat docs/datamart-sldk.md.
--
-- Script ini idempotent: dijalankan pertama kali ia MEMBUAT tabel; dijalankan lagi ia MENYEGARKAN isinya. Tabel dibangun dulu di skema DJKN_STG lalu ditukar ke DJKN dalam satu
-- transaksi singkat, sehingga pembaca datamart tidak pernah melihat tabel setengah jadi dan, bila ada yang gagal, isi datamart yang lama TIDAK berubah.
--
-- CARA MEMASANG DAN MENJALANKAN (sqlcmd, di server/database DATAMART; SUMBER = awalan nama database SLDK yang dapat dibaca dari server itu):
--
--   SLDK di server yang sama :  sqlcmd -S <server> -d <db_datamart> -b -i buat_datamart_sldk.sql -v SUMBER="[Interchange]"
--   SLDK lewat linked server :  sqlcmd -S <server> -d <db_datamart> -b -i buat_datamart_sldk.sql -v SUMBER="[LNK_SLDK].[Interchange]"
--
--   SSMS: aktifkan Query > SQLCMD Mode, hapus tanda komentar pada baris :setvar di bawah, lalu jalankan. (Jangan memakai :setvar bersama -v: :setvar menang.)
--
-- Berkas ini memasang dua prosedur di datamart (dbo.datamart_sldk_muat dan dbo.datamart_sldk_segarkan; SUMBER tertanam di dalamnya) lalu langsung menjalankan penyegaran pertama.
-- PENJADWALAN: setelah terpasang, penyegaran berikutnya cukup  EXEC dbo.datamart_sldk_segarkan;  (langkah Transact-SQL biasa pada SQL Agent, tanpa sqlcmd dan tanpa variabel).
-- Mengganti SUMBER: jalankan ulang berkas ini dengan nilai baru. SLDK berat dibaca (tabel aset sekitar 206 GB): jadwalkan di luar jam kerja.
--
-- :setvar SUMBER "[Interchange]"
--
-- Yang diperlukan: hak membaca tabel SLDK di bawah; di datamart hak CREATE TABLE/SCHEMA (db_ddladmin) dan ALTER pada skema DJKN. SQL Server 2016 atau lebih baru.
-- Berikan hak baca kepada pengguna aplikasi di tingkat SKEMA (GRANT SELECT ON SCHEMA::DJKN TO <pengguna>), bukan di tingkat tabel: izin tingkat tabel hilang saat tabel ditukar.
--
-- DATA YANG DISALIN (kolom dan baris yang tidak dipakai query tidak disalin)
--
--   Tabel                                  Baris yang disalin
--   -------------------------------------  ----------------------------------------------------------------------------------------------------------------------------------
--   SIMAN2_R_SATKER                        kd_satker berawalan 015 (semua status; query menyaring AKTIF sendiri)
--   SIMAN2_R_KONDISI                       semua
--   SIMAN2_R_STATUS_HUKUM_NEW              hanya snapshot TGL_TARIK terbaru (query hanya memakai yang terbaru)
--   SIMAN2_M_ASET                          "aset inti": status_bmn_yn = Y, kd_satker 015, dan kd_brg tanah (2%), gedung/bangunan (40101%), rumah negara (4010201%, 4010202%,
--                                          4010209%), rusunara (4010208%), atau salah satu kode kendaraan dinas; semua snapshot TGL_TARIK DIPERTAHANKAN (beberapa query
--                                          memilih sendiri snapshot terbaru). Ditambah "aset tautan": aset berstatus Y (satker/jenis apa pun) yang terhubung lewat
--                                          SIMAN2_M_ASET_TANAH_BANGUNAN dengan aset inti (tanah dari gedung kantor; bangunan dari tanah), karena query membaca sisi yang lain.
--   SIMAN2_M_ASET_TANAH_BANGUNAN           semua tautan yang salah satu sisinya aset inti
--   SIMAN2_M_ASET_PHOTO                    status_data = 1 untuk aset inti
--   SIMAN2_T_INV_BANGUNAN                  aset inti berkode 4% (gedung dan bangunan)
--   SIMAN2_PENGELOLAAN                     kode_register aset inti berkode 4% (status asuransi)
--   SIMAN2_M_ASET_HUKUM                    status_data = 1 untuk aset inti berkode 4%
--   SIMAN2_M_ASET_PEMAKAI                  HANYA aset rumah negara (4010201%, 4010202%, 4010209%, selain mess 4010202016); memuat NAMA PENGHUNI (data pribadi), jadi batasi
--                                          akses ke datamart seperti ke tabel DIGITALISASI_RUMAH_NEGARA PASTI
--   VW_SIMAN_RUANG_GEDUNG_BANGUNAN_015     baris yang cocok (satker, kode barang, NUP) dengan aset inti berkode 4%; di SLDK ini VIEW, di datamart menjadi TABEL bernama sama
--
-- Daftar kolom tiap tabel adalah kolom yang DIPAKAI query (dijaga tes backend/datamart/datamart_test.go). Bila sebuah query Digitalisasi Aset diubah dan memakai kolom atau tabel
-- baru, tes gagal sampai script ini diperbarui. Tipe dan susunan (collation) kolom mengikuti SLDK karena tabel dibuat dengan SELECT ... INTO.
--
-- JEJAK: dbo.datamart_sldk_run (satu baris per proses: mulai, selesai, status berjalan/sukses/gagal, pesan) dan dbo.datamart_sldk_tabel (jumlah baris per tabel per proses).
-- Waktu penyegaran terakhir yang berhasil: SELECT MAX(selesai) FROM dbo.datamart_sldk_run WHERE status = N'sukses'.
-- ============================================================================================================================================

:on error exit

SET NOCOUNT ON;
GO

-- Skema dan tabel jejak (dibuat sekali; aman diulang).
IF SCHEMA_ID(N'DJKN')     IS NULL EXEC (N'CREATE SCHEMA DJKN');
IF SCHEMA_ID(N'DJKN_STG') IS NULL EXEC (N'CREATE SCHEMA DJKN_STG');
IF SCHEMA_ID(N'DJKN_OLD') IS NULL EXEC (N'CREATE SCHEMA DJKN_OLD');

IF OBJECT_ID(N'dbo.datamart_sldk_run', N'U') IS NULL
    CREATE TABLE dbo.datamart_sldk_run (
        id      BIGINT        IDENTITY(1, 1) NOT NULL CONSTRAINT pk_datamart_sldk_run PRIMARY KEY,
        mulai   DATETIME2(0)  NOT NULL CONSTRAINT df_datamart_sldk_run_mulai DEFAULT SYSUTCDATETIME(),   -- UTC
        selesai DATETIME2(0)  NULL,
        status  NVARCHAR(10)  NOT NULL,                                                                   -- berjalan | sukses | gagal
        sumber  NVARCHAR(300) NULL,                                                                       -- nilai SUMBER pada proses ini
        pesan   NVARCHAR(2000) NULL
    );

IF OBJECT_ID(N'dbo.datamart_sldk_tabel', N'U') IS NULL
    CREATE TABLE dbo.datamart_sldk_tabel (
        run_id BIGINT       NOT NULL,
        tabel  NVARCHAR(128) NOT NULL,
        baris  BIGINT       NOT NULL,
        CONSTRAINT pk_datamart_sldk_tabel PRIMARY KEY (run_id, tabel),
        CONSTRAINT fk_datamart_sldk_tabel_run FOREIGN KEY (run_id) REFERENCES dbo.datamart_sldk_run (id)
    );
GO

-- Prosedur dalam: pekerjaan berat (membaca SLDK, membangun staging, menukar). Sengaja prosedur tersendiri: galat resolusi nama (kolom atau tabel SLDK hilang, SUMBER salah) pada
-- tingkat eksekusi yang sama TIDAK tertangkap TRY...CATCH, tetapi galat dari prosedur yang dipanggil tertangkap oleh pemanggilnya, sehingga kegagalan selalu tercatat.
-- SUMBER tertanam di prosedur ini saat dipasang; untuk mengganti, jalankan ulang berkas ini dengan SUMBER yang baru.
CREATE OR ALTER PROCEDURE dbo.datamart_sldk_muat @run BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    SET XACT_ABORT ON;

    DECLARE @sql NVARCHAR(MAX);

    -- 1) Tabel rujukan kecil.
    SELECT S.id_satker, S.kd_satker, S.ur_satker, S.alamat_satker, S.nm_kel, S.nm_kec, S.nm_kab_kota, S.nm_prov, S.status_uakpb, S.updated_at
    INTO DJKN_STG.SIMAN2_R_SATKER
    FROM $(SUMBER).DJKN.SIMAN2_R_SATKER AS S
    WHERE S.kd_satker LIKE N'015%';

    SELECT S.kd_kondisi, S.ur_kondisi
    INTO DJKN_STG.SIMAN2_R_KONDISI
    FROM $(SUMBER).DJKN.SIMAN2_R_KONDISI AS S;

    SELECT S.id_status_hukum, S.kd_status_hukum, S.ur_status_hukum, S.label, S.TGL_TARIK
    INTO DJKN_STG.SIMAN2_R_STATUS_HUKUM_NEW
    FROM $(SUMBER).DJKN.SIMAN2_R_STATUS_HUKUM_NEW AS S
    WHERE S.TGL_TARIK = (SELECT MAX(M.TGL_TARIK) FROM $(SUMBER).DJKN.SIMAN2_R_STATUS_HUKUM_NEW AS M);

    -- 2) Aset inti: satu-satunya pembacaan berat ke tabel aset SLDK (kriteria sama dengan filter ketujuh query). Hasilnya ditahan di tabel sementara untuk
    --    menyaring tabel-tabel anak, jadi tabel aset SLDK hanya dibaca dua kali (inti, lalu tautan).
    SELECT S.id_aset, S.kode_register, S.kd_satker, S.kd_brg, S.ur_sskel, S.no_aset, S.alamat, S.kd_rtrw, S.ur_kel, S.ur_kec, S.ur_kab, S.ur_prov,
           S.luas, S.luas_tnhb, S.kd_kondisi, S.rph_aset, S.gps_latitude, S.gps_longitude, S.status_bmn_yn, S.kd_status, S.kd_status_hukum,
           S.kd_jns_bmn, S.merk, S.nm_unit_pengguna, S.TGL_TARIK
    INTO #aset_inti
    FROM $(SUMBER).DJKN.SIMAN2_M_ASET AS S
    WHERE S.status_bmn_yn = N'Y'
      AND S.kd_satker LIKE N'015%'
      AND (   S.kd_brg LIKE N'2%'
           OR S.kd_brg LIKE N'40101%'
           OR S.kd_brg LIKE N'4010201%'
           OR S.kd_brg LIKE N'4010202%'
           OR S.kd_brg LIKE N'4010208%'
           OR S.kd_brg LIKE N'4010209%'
           OR S.kd_brg IN (N'3020101001', N'3020101002', N'3020101003', N'3020101004', N'3020101005',
                           N'3020101006', N'3020101009', N'3020102003', N'3020102007', N'3020105133'));

    CREATE CLUSTERED INDEX cx_aset_inti ON #aset_inti (id_aset);

    -- 3) Tautan tanah-bangunan yang salah satu sisinya aset inti.
    SELECT S.id_aset_tanah, S.id_aset_bangunan, S.status_data
    INTO DJKN_STG.SIMAN2_M_ASET_TANAH_BANGUNAN
    FROM $(SUMBER).DJKN.SIMAN2_M_ASET_TANAH_BANGUNAN AS S
    WHERE S.id_aset_tanah IN (SELECT I.id_aset FROM #aset_inti AS I)
       OR S.id_aset_bangunan IN (SELECT I.id_aset FROM #aset_inti AS I);

    -- 4) Aset tautan: sisi lain dari tautan itu (tanah gedung kantor; bangunan di atas tanah), bila bukan aset inti. Kriteria aset inti di CASE WHEN sama persis dengan langkah 2
    --    (CASE agar baris dengan kolom NULL tidak ikut terbuang oleh NOT NULL).
    SELECT S.id_aset, S.kode_register, S.kd_satker, S.kd_brg, S.ur_sskel, S.no_aset, S.alamat, S.kd_rtrw, S.ur_kel, S.ur_kec, S.ur_kab, S.ur_prov,
           S.luas, S.luas_tnhb, S.kd_kondisi, S.rph_aset, S.gps_latitude, S.gps_longitude, S.status_bmn_yn, S.kd_status, S.kd_status_hukum,
           S.kd_jns_bmn, S.merk, S.nm_unit_pengguna, S.TGL_TARIK
    INTO #aset_tautan
    FROM $(SUMBER).DJKN.SIMAN2_M_ASET AS S
    WHERE S.status_bmn_yn = N'Y'
      AND (   S.id_aset IN (SELECT T.id_aset_tanah    FROM DJKN_STG.SIMAN2_M_ASET_TANAH_BANGUNAN AS T)
           OR S.id_aset IN (SELECT T.id_aset_bangunan FROM DJKN_STG.SIMAN2_M_ASET_TANAH_BANGUNAN AS T))
      AND CASE WHEN (    S.status_bmn_yn = N'Y'
                     AND S.kd_satker LIKE N'015%'
                     AND (   S.kd_brg LIKE N'2%'
                          OR S.kd_brg LIKE N'40101%'
                          OR S.kd_brg LIKE N'4010201%'
                          OR S.kd_brg LIKE N'4010202%'
                          OR S.kd_brg LIKE N'4010208%'
                          OR S.kd_brg LIKE N'4010209%'
                          OR S.kd_brg IN (N'3020101001', N'3020101002', N'3020101003', N'3020101004', N'3020101005',
                                          N'3020101006', N'3020101009', N'3020102003', N'3020102007', N'3020105133'))
                    ) THEN 1 ELSE 0 END = 0;

    -- Tabel aset di datamart = aset inti + aset tautan.
    SELECT U.id_aset, U.kode_register, U.kd_satker, U.kd_brg, U.ur_sskel, U.no_aset, U.alamat, U.kd_rtrw, U.ur_kel, U.ur_kec, U.ur_kab, U.ur_prov,
           U.luas, U.luas_tnhb, U.kd_kondisi, U.rph_aset, U.gps_latitude, U.gps_longitude, U.status_bmn_yn, U.kd_status, U.kd_status_hukum,
           U.kd_jns_bmn, U.merk, U.nm_unit_pengguna, U.TGL_TARIK
    INTO DJKN_STG.SIMAN2_M_ASET
    FROM (
        SELECT * FROM #aset_inti
        UNION ALL
        SELECT * FROM #aset_tautan
    ) AS U;

    -- 5) Tabel anak, disaring dengan aset inti.
    SELECT S.id_photo, S.id_aset, S.folder, S.filename, S.nm_photo, S.photo_utama_yn, S.tanggal, S.status_data, S.TGL_TARIK
    INTO DJKN_STG.SIMAN2_M_ASET_PHOTO
    FROM $(SUMBER).DJKN.SIMAN2_M_ASET_PHOTO AS S
    WHERE S.status_data = 1
      AND S.id_aset IN (SELECT I.id_aset FROM #aset_inti AS I);

    SELECT S.id_inv_bangunan, S.id_aset, S.alamat, S.kd_rtrw, S.ur_kel, S.ur_kec, S.ur_kab, S.ur_prov, S.luas_bdg, S.TGL_TARIK, S.updated_at
    INTO DJKN_STG.SIMAN2_T_INV_BANGUNAN
    FROM $(SUMBER).DJKN.SIMAN2_T_INV_BANGUNAN AS S
    WHERE S.id_aset IN (SELECT I.id_aset FROM #aset_inti AS I WHERE I.kd_brg LIKE N'4%');

    SELECT S.kode_register, S.is_asuransi, S.tgl_sk
    INTO DJKN_STG.SIMAN2_PENGELOLAAN
    FROM $(SUMBER).DJKN.SIMAN2_PENGELOLAAN AS S
    WHERE S.kode_register IN (SELECT I.kode_register FROM #aset_inti AS I WHERE I.kd_brg LIKE N'4%' AND I.kode_register IS NOT NULL);

    SELECT S.id_aset_hukum, S.id_aset, S.kd_status_hukum, S.terakhir_yn, S.status_data, S.TGL_TARIK, S.updated_at
    INTO DJKN_STG.SIMAN2_M_ASET_HUKUM
    FROM $(SUMBER).DJKN.SIMAN2_M_ASET_HUKUM AS S
    WHERE S.status_data = 1
      AND S.id_aset IN (SELECT I.id_aset FROM #aset_inti AS I WHERE I.kd_brg LIKE N'4%');

    -- Data pribadi (nama penghuni): hanya rumah negara, persis yang dipakai query rumah_negara.sql.
    SELECT S.id_aset_pemakai, S.id_aset, S.jns_pemakai, S.nm_pmk, S.tgl_mulai
    INTO DJKN_STG.SIMAN2_M_ASET_PEMAKAI
    FROM $(SUMBER).DJKN.SIMAN2_M_ASET_PEMAKAI AS S
    WHERE S.id_aset IN (SELECT I.id_aset FROM #aset_inti AS I
                        WHERE (I.kd_brg LIKE N'4010201%' OR I.kd_brg LIKE N'4010202%' OR I.kd_brg LIKE N'4010209%')
                          AND I.kd_brg <> N'4010202016');

    SELECT S.KODE_SATKER, S.KODE_BARANG, S.NUP, S.tipe_ruang, S.kode_ruang
    INTO DJKN_STG.VW_SIMAN_RUANG_GEDUNG_BANGUNAN_015
    FROM $(SUMBER).DJKN.VW_SIMAN_RUANG_GEDUNG_BANGUNAN_015 AS S
    WHERE EXISTS (SELECT 1 FROM #aset_inti AS I
                  WHERE I.kd_satker = S.KODE_SATKER AND I.kd_brg = S.KODE_BARANG AND I.no_aset = S.NUP AND I.kd_brg LIKE N'4%');

    -- 6) Pengaman: hasil kosong hampir pasti kegagalan di hulu (akses, filter), bukan kenyataan bahwa semua aset hilang. Datamart yang lama tidak diganti.
    IF NOT EXISTS (SELECT 1 FROM DJKN_STG.SIMAN2_R_SATKER)
        THROW 50020, N'SLDK mengembalikan 0 satker; datamart lama TIDAK diganti. Periksa akses ke SLDK dan nilai SUMBER.', 1;
    IF NOT EXISTS (SELECT 1 FROM #aset_inti)
        THROW 50021, N'SLDK mengembalikan 0 aset; datamart lama TIDAK diganti. Periksa akses ke SLDK dan nilai SUMBER.', 1;

    -- 7) Indeks pada tabel staging (agar query yang berjalan di datamart cepat). Kegagalan sebuah indeks hanya diperingatkan: datamart tetap benar, hanya lebih lambat.
    CREATE TABLE #indeks (urut INT IDENTITY(1, 1) PRIMARY KEY, perintah NVARCHAR(400) NOT NULL);
    INSERT #indeks (perintah) VALUES
        (N'CREATE CLUSTERED INDEX cx_aset ON DJKN_STG.SIMAN2_M_ASET (id_aset)'),
        (N'CREATE INDEX ix_aset_brg_satker ON DJKN_STG.SIMAN2_M_ASET (kd_brg, kd_satker)'),
        (N'CREATE CLUSTERED INDEX cx_satker ON DJKN_STG.SIMAN2_R_SATKER (kd_satker)'),
        (N'CREATE CLUSTERED INDEX cx_kondisi ON DJKN_STG.SIMAN2_R_KONDISI (kd_kondisi)'),
        (N'CREATE CLUSTERED INDEX cx_status_hukum ON DJKN_STG.SIMAN2_R_STATUS_HUKUM_NEW (kd_status_hukum)'),
        (N'CREATE CLUSTERED INDEX cx_photo ON DJKN_STG.SIMAN2_M_ASET_PHOTO (id_aset)'),
        (N'CREATE CLUSTERED INDEX cx_inv_bangunan ON DJKN_STG.SIMAN2_T_INV_BANGUNAN (id_aset)'),
        (N'CREATE CLUSTERED INDEX cx_pengelolaan ON DJKN_STG.SIMAN2_PENGELOLAAN (kode_register)'),
        (N'CREATE CLUSTERED INDEX cx_hukum ON DJKN_STG.SIMAN2_M_ASET_HUKUM (id_aset)'),
        (N'CREATE CLUSTERED INDEX cx_pemakai ON DJKN_STG.SIMAN2_M_ASET_PEMAKAI (id_aset)'),
        (N'CREATE CLUSTERED INDEX cx_tanah_bangunan ON DJKN_STG.SIMAN2_M_ASET_TANAH_BANGUNAN (id_aset_tanah)'),
        (N'CREATE INDEX ix_tanah_bangunan_bgn ON DJKN_STG.SIMAN2_M_ASET_TANAH_BANGUNAN (id_aset_bangunan)'),
        (N'CREATE CLUSTERED INDEX cx_ruang ON DJKN_STG.VW_SIMAN_RUANG_GEDUNG_BANGUNAN_015 (KODE_SATKER, KODE_BARANG, NUP)');

    DECLARE @i INT = 1, @maks INT = (SELECT MAX(urut) FROM #indeks), @perintah NVARCHAR(400), @peringatan NVARCHAR(2000);
    WHILE @i <= @maks
    BEGIN
        SELECT @perintah = perintah FROM #indeks WHERE urut = @i;
        BEGIN TRY
            EXEC (@perintah);
        END TRY
        BEGIN CATCH
            SET @peringatan = LEFT(N'PERINGATAN: indeks dilewati (' + @perintah + N'): ' + ERROR_MESSAGE(), 2000);
            RAISERROR (N'%s', 0, 1, @peringatan) WITH NOWAIT;
        END CATCH;
        SET @i += 1;
    END;

    -- 8) Tukar staging ke DJKN dalam satu transaksi (tabel lama dipindah ke DJKN_OLD lalu dibuang). Hanya perubahan metadata, jadi sangat singkat.
    SET @sql = N'';
    SELECT @sql += CASE WHEN OBJECT_ID(N'DJKN.' + QUOTENAME(name), N'U') IS NOT NULL
                        THEN N'ALTER SCHEMA DJKN_OLD TRANSFER DJKN.' + QUOTENAME(name) + N';' ELSE N'' END
                 + N'ALTER SCHEMA DJKN TRANSFER DJKN_STG.' + QUOTENAME(name) + N';'
    FROM sys.tables WHERE schema_id = SCHEMA_ID(N'DJKN_STG');

    BEGIN TRANSACTION;
    EXEC (@sql);
    COMMIT TRANSACTION;

    SET @sql = N'';
    SELECT @sql += N'DROP TABLE DJKN_OLD.' + QUOTENAME(name) + N';' FROM sys.tables WHERE schema_id = SCHEMA_ID(N'DJKN_OLD');
    EXEC (@sql);

    -- 9) Jejak jumlah baris.
    SET @sql = N'';
    SELECT @sql += N'INSERT dbo.datamart_sldk_tabel (run_id, tabel, baris) SELECT @r, N''' + name + N''', COUNT_BIG(*) FROM DJKN.' + QUOTENAME(name) + N';'
    FROM sys.tables WHERE schema_id = SCHEMA_ID(N'DJKN');
    EXEC sp_executesql @sql, N'@r BIGINT', @r = @run;
END;
GO

-- Prosedur luar (titik masuk, juga untuk penjadwalan): satu proses pada satu waktu, mencatat jejak, membersihkan sisa proses gagal, memanggil prosedur dalam, dan mencatat hasilnya.
CREATE OR ALTER PROCEDURE dbo.datamart_sldk_segarkan
AS
BEGIN
    SET NOCOUNT ON;
    SET XACT_ABORT ON;

    DECLARE @kunci INT, @run BIGINT, @sql NVARCHAR(MAX), @n BIGINT, @pesan NVARCHAR(2000);

    -- Kunci aplikasi bertipe sesi: lepas sendiri bila sambungan putus.
    EXEC @kunci = sp_getapplock @Resource = N'datamart_sldk', @LockMode = N'Exclusive', @LockOwner = N'Session', @LockTimeout = 0;
    IF @kunci < 0
        THROW 50010, N'Proses datamart SLDK lain sedang berjalan; proses ini dihentikan.', 1;

    BEGIN TRY
        -- Proses sebelumnya yang masih berstatus berjalan padahal kunci kita dapat = terhenti di tengah jalan.
        UPDATE dbo.datamart_sldk_run SET status = N'gagal', selesai = SYSUTCDATETIME(), pesan = N'Terhenti sebelum selesai (proses berhenti atau sambungan putus).' WHERE status = N'berjalan';

        -- Sisa staging dari proses yang gagal. SELECT ... INTO ke tabel yang sudah ada gagal, jadi dibersihkan sebelum prosedur dalam berjalan.
        SET @sql = N'';
        SELECT @sql += N'DROP TABLE ' + QUOTENAME(SCHEMA_NAME(schema_id)) + N'.' + QUOTENAME(name) + N';'
        FROM sys.tables WHERE schema_id IN (SCHEMA_ID(N'DJKN_STG'), SCHEMA_ID(N'DJKN_OLD'));
        EXEC (@sql);

        INSERT dbo.datamart_sldk_run (status, sumber) VALUES (N'berjalan', N'$(SUMBER)');
        SET @run = SCOPE_IDENTITY();

        EXEC dbo.datamart_sldk_muat @run = @run;

        SELECT @n = baris FROM dbo.datamart_sldk_tabel WHERE run_id = @run AND tabel = N'SIMAN2_M_ASET';
        UPDATE dbo.datamart_sldk_run SET status = N'sukses', selesai = SYSUTCDATETIME(),
               pesan = N'SIMAN2_M_ASET ' + CONVERT(NVARCHAR(20), @n) + N' baris (aset inti dan tautan); rincian per tabel di dbo.datamart_sldk_tabel.'
        WHERE id = @run;

        EXEC sp_releaseapplock @Resource = N'datamart_sldk', @LockOwner = N'Session';
        PRINT N'Datamart SLDK selesai. Proses #' + CONVERT(NVARCHAR(20), @run) + N'; rincian jumlah baris: SELECT * FROM dbo.datamart_sldk_tabel WHERE run_id = ' + CONVERT(NVARCHAR(20), @run) + N'.';
    END TRY
    BEGIN CATCH
        IF XACT_STATE() <> 0 ROLLBACK TRANSACTION;

        SET @pesan = LEFT(ISNULL(ERROR_MESSAGE(), N'Galat tidak diketahui') + N' (' + ISNULL(ERROR_PROCEDURE(), N'-') + N', baris ' + CONVERT(NVARCHAR(10), ERROR_LINE()) + N')', 2000);
        IF @run IS NOT NULL
            UPDATE dbo.datamart_sldk_run SET status = N'gagal', selesai = SYSUTCDATETIME(), pesan = @pesan WHERE id = @run;

        -- Buang staging yang setengah jadi; DJKN (datamart yang berlaku) tidak disentuh.
        BEGIN TRY
            SET @sql = N'';
            SELECT @sql += N'DROP TABLE DJKN_STG.' + QUOTENAME(name) + N';' FROM sys.tables WHERE schema_id = SCHEMA_ID(N'DJKN_STG');
            EXEC (@sql);
        END TRY
        BEGIN CATCH
        END CATCH;
        EXEC sp_releaseapplock @Resource = N'datamart_sldk', @LockOwner = N'Session';

        THROW;
    END CATCH;
END;
GO

-- Jalankan sekarang (pemasangan pertama membuat tabel; pemasangan ulang menyegarkannya). Untuk penjadwalan cukup: EXEC dbo.datamart_sldk_segarkan;
EXEC dbo.datamart_sldk_segarkan;
GO
