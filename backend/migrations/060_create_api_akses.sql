-- ============================================================
-- Migration 060: API Akses Data PASTI (pendaftaran aplikasi klien, kunci API, batas laju/kuota, dan pencatatan pemakaian)
-- Aman dijalankan berulang kali (idempotent)
--
--   api_akses_pengaturan    : bawaan global batas laju dan kuota serta pengaturan lain, dapat diubah superadmin dari aplikasi tanpa deploy ulang
--   api_akses_klien         : aplikasi klien yang mendaftar (menunggu | disetujui | ditolak | dicabut) beserta override batas per klien, allowlist IP,
--                             dan pembatasan UE1. Klien tidak pernah dihapus (jejak audit); yang tidak lagi dipakai berstatus dicabut.
--   api_akses_klien_dataset : dataset yang BOLEH diakses klien (scope, least privilege). Yang diminta saat mendaftar ada di api_akses_klien.dataset_diminta.
--   api_akses_kunci         : kunci API. Hanya hash (HMAC-SHA256) yang disimpan; kunci aslinya ditampilkan satu kali saat dibuat dan tidak dapat dilihat lagi.
--   api_akses_kuota         : salinan penghitung kuota jendela berjalan (ditulis berkala), supaya kuota tidak kembali penuh saat aplikasi dimulai ulang.
--   api_akses_log           : satu baris per permintaan klien (riwayat permintaan terakhir). Tanpa kunci, tanpa isi respons. Dihapus setelah log_retensi_hari.
--   api_akses_agregat       : hitungan per jam per klien per endpoint per status (untuk grafik/ringkasan jangka panjang). Dihapus setelah agregat_retensi_hari.
-- ============================================================

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'api_akses_pengaturan')
BEGIN
    CREATE TABLE api_akses_pengaturan (
        kunci       NVARCHAR(60)  NOT NULL CONSTRAINT pk_api_akses_pengaturan PRIMARY KEY,
        nilai       NVARCHAR(200) NOT NULL,
        diubah_at   DATETIME2(0)  NOT NULL CONSTRAINT df_api_akses_pengaturan_at DEFAULT SYSUTCDATETIME(),
        diubah_oleh NVARCHAR(100) NULL
    );
END;

-- Nilai bawaan (hanya diisi bila belum ada, jadi perubahan superadmin tidak pernah ditimpa migrasi ini).
IF NOT EXISTS (SELECT 1 FROM api_akses_pengaturan WHERE kunci = N'limit_permintaan')       INSERT INTO api_akses_pengaturan (kunci, nilai) VALUES (N'limit_permintaan', N'1000');
IF NOT EXISTS (SELECT 1 FROM api_akses_pengaturan WHERE kunci = N'limit_jendela_detik')    INSERT INTO api_akses_pengaturan (kunci, nilai) VALUES (N'limit_jendela_detik', N'60');
IF NOT EXISTS (SELECT 1 FROM api_akses_pengaturan WHERE kunci = N'kuota_permintaan')       INSERT INTO api_akses_pengaturan (kunci, nilai) VALUES (N'kuota_permintaan', N'5000');
IF NOT EXISTS (SELECT 1 FROM api_akses_pengaturan WHERE kunci = N'kuota_periode_detik')    INSERT INTO api_akses_pengaturan (kunci, nilai) VALUES (N'kuota_periode_detik', N'3600');
IF NOT EXISTS (SELECT 1 FROM api_akses_pengaturan WHERE kunci = N'pendaftaran_dibuka')     INSERT INTO api_akses_pengaturan (kunci, nilai) VALUES (N'pendaftaran_dibuka', N'1');
IF NOT EXISTS (SELECT 1 FROM api_akses_pengaturan WHERE kunci = N'kunci_masa_berlaku_hari') INSERT INTO api_akses_pengaturan (kunci, nilai) VALUES (N'kunci_masa_berlaku_hari', N'0');
IF NOT EXISTS (SELECT 1 FROM api_akses_pengaturan WHERE kunci = N'rotasi_tenggang_jam')     INSERT INTO api_akses_pengaturan (kunci, nilai) VALUES (N'rotasi_tenggang_jam', N'24');
IF NOT EXISTS (SELECT 1 FROM api_akses_pengaturan WHERE kunci = N'halaman_maks')           INSERT INTO api_akses_pengaturan (kunci, nilai) VALUES (N'halaman_maks', N'500');
IF NOT EXISTS (SELECT 1 FROM api_akses_pengaturan WHERE kunci = N'log_retensi_hari')       INSERT INTO api_akses_pengaturan (kunci, nilai) VALUES (N'log_retensi_hari', N'14');
IF NOT EXISTS (SELECT 1 FROM api_akses_pengaturan WHERE kunci = N'agregat_retensi_hari')   INSERT INTO api_akses_pengaturan (kunci, nilai) VALUES (N'agregat_retensi_hari', N'400');

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'api_akses_klien')
BEGIN
    CREATE TABLE api_akses_klien (
        id                  BIGINT IDENTITY(1,1) NOT NULL CONSTRAINT pk_api_akses_klien PRIMARY KEY,
        kode                NVARCHAR(16)   NOT NULL,                 -- ID klien yang tampil (KL-XXXXXXXX); bukan rahasia
        nama_aplikasi       NVARCHAR(150)  NOT NULL,
        instansi            NVARCHAR(200)  NOT NULL,
        unit                NVARCHAR(200)  NULL,
        pic_nama            NVARCHAR(150)  NOT NULL,
        pic_jabatan         NVARCHAR(150)  NULL,
        pic_email           NVARCHAR(200)  NOT NULL,
        pic_telepon         NVARCHAR(40)   NULL,
        tujuan              NVARCHAR(2000) NOT NULL,
        dataset_diminta     NVARCHAR(1000) NOT NULL,                 -- kode dataset dipisah koma
        status              NVARCHAR(12)   NOT NULL CONSTRAINT df_api_akses_klien_status DEFAULT N'menunggu',
        catatan             NVARCHAR(1000) NULL,                     -- alasan ditolak/dicabut atau catatan persetujuan (dari superadmin)
        ip_diizinkan        NVARCHAR(1000) NULL,                     -- allowlist IP/CIDR dipisah koma; NULL = semua alamat
        ue1_diizinkan       NVARCHAR(500)  NULL,                     -- batasi baris ke kode UE1 (5 digit) dipisah koma; NULL = semua
        limit_permintaan    INT NULL,                                -- override; NULL = ikut bawaan global
        limit_jendela_detik INT NULL,
        kuota_permintaan    INT NULL,
        kuota_periode_detik INT NULL,
        didaftarkan_at      DATETIME2(0)   NOT NULL CONSTRAINT df_api_akses_klien_daftar DEFAULT SYSUTCDATETIME(),
        didaftarkan_ip      NVARCHAR(64)   NULL,
        diputuskan_at       DATETIME2(0)   NULL,
        diputuskan_oleh     NVARCHAR(100)  NULL,
        diubah_at           DATETIME2(0)   NULL,
        CONSTRAINT ck_api_akses_klien_status CHECK (status IN (N'menunggu', N'disetujui', N'ditolak', N'dicabut'))
    );
END;

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'ux_api_akses_klien_kode' AND object_id = OBJECT_ID('api_akses_klien'))
    CREATE UNIQUE INDEX ux_api_akses_klien_kode ON api_akses_klien (kode);

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'ix_api_akses_klien_status' AND object_id = OBJECT_ID('api_akses_klien'))
    CREATE INDEX ix_api_akses_klien_status ON api_akses_klien (status, didaftarkan_at DESC);

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'api_akses_klien_dataset')
BEGIN
    CREATE TABLE api_akses_klien_dataset (
        klien_id BIGINT       NOT NULL,
        dataset  NVARCHAR(60) NOT NULL,
        CONSTRAINT pk_api_akses_klien_dataset PRIMARY KEY (klien_id, dataset),
        CONSTRAINT fk_api_akses_klien_dataset_klien FOREIGN KEY (klien_id) REFERENCES api_akses_klien (id) ON DELETE CASCADE
    );
END;

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'api_akses_kunci')
BEGIN
    CREATE TABLE api_akses_kunci (
        id                  BIGINT IDENTITY(1,1) NOT NULL CONSTRAINT pk_api_akses_kunci PRIMARY KEY,
        klien_id            BIGINT        NOT NULL,
        prefix              NVARCHAR(16)  NOT NULL,                  -- bagian kunci yang bukan rahasia (pengenal), dipakai mencari barisnya
        hash                CHAR(64)      NOT NULL,                  -- HMAC-SHA256 (hex) dari seluruh kunci; kunci aslinya tidak disimpan
        label               NVARCHAR(100) NULL,
        dibuat_at           DATETIME2(0)  NOT NULL CONSTRAINT df_api_akses_kunci_dibuat DEFAULT SYSUTCDATETIME(),
        dibuat_oleh         NVARCHAR(100) NULL,
        kedaluwarsa_at      DATETIME2(0)  NULL,                      -- NULL = tidak kedaluwarsa
        dicabut_at          DATETIME2(0)  NULL,
        dicabut_oleh        NVARCHAR(100) NULL,
        alasan_cabut        NVARCHAR(200) NULL,
        terakhir_dipakai_at DATETIME2(0)  NULL,
        terakhir_ip         NVARCHAR(64)  NULL,
        CONSTRAINT fk_api_akses_kunci_klien FOREIGN KEY (klien_id) REFERENCES api_akses_klien (id)
    );
END;

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'ux_api_akses_kunci_prefix' AND object_id = OBJECT_ID('api_akses_kunci'))
    CREATE UNIQUE INDEX ux_api_akses_kunci_prefix ON api_akses_kunci (prefix);

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'ix_api_akses_kunci_klien' AND object_id = OBJECT_ID('api_akses_kunci'))
    CREATE INDEX ix_api_akses_kunci_klien ON api_akses_kunci (klien_id);

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'api_akses_kuota')
BEGIN
    CREATE TABLE api_akses_kuota (
        klien_id      BIGINT       NOT NULL CONSTRAINT pk_api_akses_kuota PRIMARY KEY,
        periode_mulai DATETIME2(0) NOT NULL,                         -- awal jendela kuota (UTC, kelipatan periode)
        jumlah        INT          NOT NULL,
        diubah_at     DATETIME2(0) NOT NULL CONSTRAINT df_api_akses_kuota_at DEFAULT SYSUTCDATETIME(),
        CONSTRAINT fk_api_akses_kuota_klien FOREIGN KEY (klien_id) REFERENCES api_akses_klien (id) ON DELETE CASCADE
    );
END;

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'api_akses_log')
BEGIN
    CREATE TABLE api_akses_log (
        id           BIGINT IDENTITY(1,1) NOT NULL CONSTRAINT pk_api_akses_log PRIMARY KEY,
        waktu        DATETIME2(3)  NOT NULL,
        klien_id     BIGINT        NULL,                             -- NULL: kunci tidak dikenal/tidak sah
        kunci_prefix NVARCHAR(16)  NULL,
        request_id   NVARCHAR(40)  NULL,                             -- sama dengan header X-Request-ID dan entri Log Audit
        metode       NVARCHAR(8)   NOT NULL,
        endpoint     NVARCHAR(100) NOT NULL,                         -- pola rute (tanpa nilai parameter), mis. /aset/:dataset
        dataset      NVARCHAR(60)  NULL,
        status_http  SMALLINT      NOT NULL,
        hasil        NVARCHAR(20)  NOT NULL,                         -- ok | galat_klien | galat_server | batas_laju | kuota_habis | ditolak_ip | tidak_sah | ditolak_scope
        durasi_ms    INT           NOT NULL CONSTRAINT df_api_akses_log_durasi DEFAULT 0,
        baris        INT           NULL,                             -- jumlah baris data yang dikembalikan
        ip           NVARCHAR(64)  NULL,
        user_agent   NVARCHAR(200) NULL
    );
END;

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'ix_api_akses_log_waktu' AND object_id = OBJECT_ID('api_akses_log'))
    CREATE INDEX ix_api_akses_log_waktu ON api_akses_log (waktu DESC, id DESC);

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'ix_api_akses_log_klien_waktu' AND object_id = OBJECT_ID('api_akses_log'))
    CREATE INDEX ix_api_akses_log_klien_waktu ON api_akses_log (klien_id, waktu DESC);

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'api_akses_agregat')
BEGIN
    CREATE TABLE api_akses_agregat (
        jam          DATETIME2(0)  NOT NULL,                         -- awal jam (UTC)
        klien_id     BIGINT        NOT NULL,                         -- 0: kunci tidak dikenal/tidak sah
        endpoint     NVARCHAR(100) NOT NULL,
        status_http  SMALLINT      NOT NULL,
        hasil        NVARCHAR(20)  NOT NULL,                         -- sama dengan api_akses_log.hasil
        jumlah       BIGINT        NOT NULL,
        baris        BIGINT        NOT NULL CONSTRAINT df_api_akses_agregat_baris DEFAULT 0,
        lat_total_ms BIGINT        NOT NULL,
        lat_maks_ms  INT           NOT NULL,
        b50          INT           NOT NULL CONSTRAINT df_api_akses_agregat_b50 DEFAULT 0,     -- histogram latensi: jumlah permintaan <=50 ms
        b100         INT           NOT NULL CONSTRAINT df_api_akses_agregat_b100 DEFAULT 0,    -- <=100 ms
        b250         INT           NOT NULL CONSTRAINT df_api_akses_agregat_b250 DEFAULT 0,
        b500         INT           NOT NULL CONSTRAINT df_api_akses_agregat_b500 DEFAULT 0,
        b1000        INT           NOT NULL CONSTRAINT df_api_akses_agregat_b1000 DEFAULT 0,
        b2500        INT           NOT NULL CONSTRAINT df_api_akses_agregat_b2500 DEFAULT 0,
        blebih       INT           NOT NULL CONSTRAINT df_api_akses_agregat_blebih DEFAULT 0,  -- > 2500 ms
        CONSTRAINT pk_api_akses_agregat PRIMARY KEY (jam, klien_id, endpoint, status_http, hasil)
    );
END;

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'ix_api_akses_agregat_klien' AND object_id = OBJECT_ID('api_akses_agregat'))
    CREATE INDEX ix_api_akses_agregat_klien ON api_akses_agregat (klien_id, jam);
