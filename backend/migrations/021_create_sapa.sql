-- ============================================================
-- Migration 021: SAPA (Sistem Administrasi Pengelolaan Aset) - modul Penjualan
-- Aman dijalankan berulang kali (idempotent)
--
-- Peran SAPA (satker / kanwil / ue1) terpisah dari peran aplikasi (user / admin / superadmin) dan ditetapkan
-- admin per pengguna. Satu usulan penjualan (sapa_penjualan) punya satu catatan per tahap (sapa_penjualan_tahap)
-- dan dokumen Word hasil pengisian template (sapa_dokumen). Template Word diunggah admin (sapa_template, berversi).
-- ============================================================

-- Peran SAPA seorang pengguna beserta cakupannya (kode satker 18 digit untuk satker, kode UE1 5 digit untuk ue1).
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'sapa_peran')
BEGIN
    CREATE TABLE sapa_peran (
        user_id UNIQUEIDENTIFIER NOT NULL PRIMARY KEY,
        peran NVARCHAR(20) NOT NULL,
        kode_satker NVARCHAR(18) NULL,
        kode_ue1 NVARCHAR(5) NULL,
        diubah_oleh NVARCHAR(100) NULL,
        diubah_pada DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        CONSTRAINT fk_sapa_peran_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
        CONSTRAINT ck_sapa_peran CHECK (peran IN ('satker', 'kanwil', 'ue1'))
    );
END;

-- Referensi Unit Eselon I: kode 5 digit -> nama dan sebutan Sekretaris (tujuan Nota Dinas usulan Satker).
-- Diisi admin; tidak ada data awal supaya tidak ada nama yang salah.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'sapa_ref_ue1')
BEGIN
    CREATE TABLE sapa_ref_ue1 (
        kode NVARCHAR(5) NOT NULL PRIMARY KEY,
        nama NVARCHAR(200) NOT NULL,
        sebutan_sekretaris NVARCHAR(300) NOT NULL,
        diubah_oleh NVARCHAR(100) NULL,
        diubah_pada DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );
END;

-- Template Word per jenis dokumen (sk_tim, ba_penelitian, nd_satker, nd_ue1). Tiap unggahan menjadi versi baru;
-- paling banyak satu versi aktif per jenis. Bila belum ada unggahan, aplikasi memakai template bawaannya (bila ada).
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'sapa_template')
BEGIN
    CREATE TABLE sapa_template (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        kunci NVARCHAR(30) NOT NULL,
        versi INT NOT NULL,
        nama_file NVARCHAR(255) NOT NULL,
        ukuran INT NOT NULL,
        konten VARBINARY(MAX) NOT NULL,
        aktif BIT NOT NULL DEFAULT 1,
        catatan NVARCHAR(500) NULL,
        diunggah_oleh NVARCHAR(100) NULL,
        diunggah_pada DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        CONSTRAINT uq_sapa_template_versi UNIQUE (kunci, versi)
    );

    CREATE UNIQUE INDEX ux_sapa_template_aktif ON sapa_template(kunci) WHERE aktif = 1;
END;

-- Penomoran Noreg usulan (PJ-<tahun>-<urutan>): satu baris per kunci dan tahun.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'sapa_urutan')
BEGIN
    CREATE TABLE sapa_urutan (
        kunci NVARCHAR(20) NOT NULL,
        tahun INT NOT NULL,
        nilai INT NOT NULL,
        CONSTRAINT pk_sapa_urutan PRIMARY KEY (kunci, tahun)
    );
END;

-- Usulan penjualan. noreg = "nomor tiket"/"Noreg aplikasi" pada dokumen.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'sapa_penjualan')
BEGIN
    CREATE TABLE sapa_penjualan (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        noreg NVARCHAR(30) NOT NULL,
        kode_satker NVARCHAR(18) NOT NULL,
        nama_satker NVARCHAR(300) NOT NULL,
        kode_ue1 NVARCHAR(5) NOT NULL,
        dibuat_oleh_id UNIQUEIDENTIFIER NULL,
        dibuat_oleh NVARCHAR(100) NULL,
        dibuat_pada DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        diperbarui_pada DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        CONSTRAINT uq_sapa_penjualan_noreg UNIQUE (noreg),
        CONSTRAINT fk_sapa_penjualan_user FOREIGN KEY (dibuat_oleh_id) REFERENCES users(id) ON DELETE SET NULL
    );

    CREATE INDEX idx_sapa_penjualan_kode_satker ON sapa_penjualan(kode_satker);
    CREATE INDEX idx_sapa_penjualan_kode_ue1 ON sapa_penjualan(kode_ue1);
END;

-- Catatan satu tahap pada satu usulan. Tahap yang belum disentuh tidak punya baris (dianggap "belum").
-- data = isian formulir (JSON); nomor/tanggal = nomor dan tanggal dokumen dari aplikasi lain (Nadine).
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'sapa_penjualan_tahap')
BEGIN
    CREATE TABLE sapa_penjualan_tahap (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        penjualan_id BIGINT NOT NULL,
        kunci NVARCHAR(30) NOT NULL,
        status NVARCHAR(20) NOT NULL,        -- draft | selesai | dilewati
        data NVARCHAR(MAX) NULL,
        nomor NVARCHAR(150) NULL,
        tanggal DATE NULL,
        catatan NVARCHAR(1000) NULL,
        diperbarui_oleh NVARCHAR(100) NULL,
        diperbarui_pada DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        CONSTRAINT uq_sapa_penjualan_tahap UNIQUE (penjualan_id, kunci),
        CONSTRAINT fk_sapa_penjualan_tahap FOREIGN KEY (penjualan_id) REFERENCES sapa_penjualan(id) ON DELETE CASCADE,
        CONSTRAINT ck_sapa_penjualan_tahap_status CHECK (status IN ('draft', 'selesai', 'dilewati'))
    );
END;

-- Dokumen Word hasil pengisian template. Dokumen lama tetap disimpan ketika tahap dibuat ulang (riwayat).
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'sapa_dokumen')
BEGIN
    CREATE TABLE sapa_dokumen (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        penjualan_id BIGINT NOT NULL,
        tahap NVARCHAR(30) NOT NULL,
        jenis NVARCHAR(30) NOT NULL,
        nama_file NVARCHAR(255) NOT NULL,
        ukuran INT NOT NULL,
        konten VARBINARY(MAX) NOT NULL,
        peringatan NVARCHAR(MAX) NULL,       -- JSON: daftar peringatan saat mengisi template
        dibuat_oleh NVARCHAR(100) NULL,
        dibuat_pada DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        CONSTRAINT fk_sapa_dokumen FOREIGN KEY (penjualan_id) REFERENCES sapa_penjualan(id) ON DELETE CASCADE
    );

    CREATE INDEX idx_sapa_dokumen_penjualan ON sapa_dokumen(penjualan_id, id);
END;
