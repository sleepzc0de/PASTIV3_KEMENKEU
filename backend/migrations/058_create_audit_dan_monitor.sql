-- ============================================================
-- Migration 058: Log audit aktivitas pengguna dan riwayat pemantauan resource
-- Aman dijalankan berulang kali (idempotent)
--
--   audit_log        : satu baris per aktivitas pengguna yang dicatat middleware audit (perubahan data, unduhan/ekspor, pencarian data pegawai,
--                      login berhasil/gagal, akses ditolak). Hanya ditulis aplikasi dan dibaca superadmin; tidak ada rute ubah/hapus, baris hanya
--                      dihapus oleh pembersihan retensi (AUDIT_RETENSI_HARI, bawaan 365 hari). Isi permintaan (badan) TIDAK pernah disimpan.
--   monitor_snapshot : ringkasan pemakaian resource server, aplikasi, dan database tiap 5 menit (rata-rata dan puncak pada jendela itu), dipakai grafik
--                      riwayat dan rekomendasi "resource mana yang perlu ditambah". Dihapus setelah MONITOR_RETENSI_HARI (bawaan 30 hari).
-- ============================================================

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'audit_log')
BEGIN
    CREATE TABLE audit_log (
        id          BIGINT IDENTITY(1,1) NOT NULL CONSTRAINT pk_audit_log PRIMARY KEY,
        waktu       DATETIME2(3) NOT NULL CONSTRAINT df_audit_log_waktu DEFAULT SYSUTCDATETIME(),
        request_id  NVARCHAR(40)  NULL,
        user_id     UNIQUEIDENTIFIER NULL,
        username    NVARCHAR(100) NULL,
        peran       NVARCHAR(30)  NULL,       -- peran yang berlaku saat itu (superadmin, pengguna_barang, ue1, kanwil, satker)
        kode_peran  NVARCHAR(20)  NULL,
        kategori    NVARCHAR(30)  NOT NULL,   -- auth, pengguna, peran, sapa, digitalisasi, pengadaan, referensi, ekspor, hris2, audit, lainnya
        aksi        NVARCHAR(100) NOT NULL,   -- kode aksi, mis. auth.login.gagal, pengguna.hapus
        label       NVARCHAR(300) NOT NULL,   -- uraian yang dibaca manusia
        metode      NVARCHAR(8)   NOT NULL,
        rute        NVARCHAR(250) NOT NULL,   -- pola rute (tanpa nilai parameter), mis. /api/v1/users/:id
        objek_tipe  NVARCHAR(40)  NULL,
        objek_id    NVARCHAR(120) NULL,
        status_http INT           NOT NULL,
        sukses      BIT           NOT NULL,
        durasi_ms   INT           NOT NULL CONSTRAINT df_audit_log_durasi DEFAULT 0,
        ip          NVARCHAR(64)  NULL,
        user_agent  NVARCHAR(300) NULL,
        detail      NVARCHAR(MAX) NULL        -- JSON kecil: parameter rute, alasan gagal, kata kunci pencarian; tanpa kata sandi/token/badan permintaan
    );
END;

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'ix_audit_log_waktu' AND object_id = OBJECT_ID('audit_log'))
    CREATE INDEX ix_audit_log_waktu ON audit_log (waktu DESC, id DESC);

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'ix_audit_log_user_waktu' AND object_id = OBJECT_ID('audit_log'))
    CREATE INDEX ix_audit_log_user_waktu ON audit_log (user_id, waktu DESC);

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'ix_audit_log_kategori_waktu' AND object_id = OBJECT_ID('audit_log'))
    CREATE INDEX ix_audit_log_kategori_waktu ON audit_log (kategori, waktu DESC);

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'ix_audit_log_aksi_waktu' AND object_id = OBJECT_ID('audit_log'))
    CREATE INDEX ix_audit_log_aksi_waktu ON audit_log (aksi, waktu DESC);

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'monitor_snapshot')
BEGIN
    CREATE TABLE monitor_snapshot (
        id                 BIGINT IDENTITY(1,1) NOT NULL CONSTRAINT pk_monitor_snapshot PRIMARY KEY,
        waktu              DATETIME2(0) NOT NULL,   -- akhir jendela pengukuran (UTC)
        jendela_detik      INT          NOT NULL,
        cpu_rata           FLOAT NULL,              -- % pemakaian CPU server (0-100)
        cpu_maks           FLOAT NULL,
        load1_maks         FLOAT NULL,              -- load average 1 menit (Linux)
        mem_total          BIGINT NULL,             -- byte
        mem_terpakai_rata  BIGINT NULL,
        mem_terpakai_maks  BIGINT NULL,
        swap_terpakai_maks BIGINT NULL,
        disk_total         BIGINT NULL,
        disk_terpakai      BIGINT NULL,
        proses_rss_maks    BIGINT NULL,             -- memori proses aplikasi
        heap_maks          BIGINT NULL,
        goroutine_maks     INT NULL,
        db_buka_maks       INT NULL,                -- koneksi database yang terbuka / sedang dipakai / batas
        db_dipakai_maks    INT NULL,
        db_batas_koneksi   INT NULL,
        db_tunggu_delta    BIGINT NULL,             -- permintaan koneksi yang harus menunggu pada jendela ini
        db_ukuran_mb       FLOAT NULL,              -- ukuran file database
        db_terpakai_mb     FLOAT NULL,
        req_total          INT NULL,
        req_4xx            INT NULL,
        req_5xx            INT NULL,
        lat_rata_ms        FLOAT NULL,
        lat_p95_ms         FLOAT NULL,
        req_berjalan_maks  INT NULL
    );
END;

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'ix_monitor_snapshot_waktu' AND object_id = OBJECT_ID('monitor_snapshot'))
    CREATE INDEX ix_monitor_snapshot_waktu ON monitor_snapshot (waktu);
