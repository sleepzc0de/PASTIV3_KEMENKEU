-- ============================================================
-- Migration 056: Referensi Kantor Wilayah (Kanwil)
-- Aman dijalankan berulang kali (idempotent)
--
-- Satu daftar rujukan kode Kanwil (9 digit pertama kode satker = 3 digit KL + 2 digit UE1 + 4 digit wilayah, mis. 015040199) -> uraian dan singkatan.
-- Dipakai untuk menerjemahkan kode Kanwil pada peran pengguna dan rincian data aset. Dikelola superadmin lewat Administrasi > Referensi Kanwil.
--
-- Asal isi (kolom sumber):
--   satker : diambil dari nama satker pada data Digitalisasi Aset (DIGITALISASI_SATKER) yang kode satkernya berawalan kode Kanwil itu
--   sldk   : ditarik dari SLDK (DJKN.SIMAN2_R_KORWIL: kd_wileselon -> ur_korwil), kolom status_korwil disimpan apa adanya di status_sldk
--   manual : diisi atau diubah superadmin; penarikan dari SLDK tidak pernah menimpa baris manual
-- Tidak ada data awal: isinya organisasi-spesifik dan diisi dari data satker atau SLDK.
-- ============================================================

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'ref_kanwil')
BEGIN
    CREATE TABLE ref_kanwil (
        kode NVARCHAR(9) NOT NULL PRIMARY KEY,
        nama NVARCHAR(200) NOT NULL,
        singkatan NVARCHAR(30) NULL,
        urutan INT NOT NULL DEFAULT 0,
        aktif BIT NOT NULL DEFAULT 1,
        sumber NVARCHAR(10) NOT NULL DEFAULT N'manual',
        status_sldk NVARCHAR(50) NULL,
        diubah_oleh NVARCHAR(100) NULL,
        diubah_pada DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        CONSTRAINT ck_ref_kanwil_kode CHECK (LEN(kode) = 9 AND kode NOT LIKE '%[^0-9]%'),
        CONSTRAINT ck_ref_kanwil_sumber CHECK (sumber IN (N'manual', N'satker', N'sldk'))
    );
END;
