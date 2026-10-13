-- ============================================================
-- Migration 059: Log server (hanya galat dan peringatan)
-- Aman dijalankan berulang kali (idempotent)
--
--   server_log : satu baris = satu KELOMPOK kejadian galat (error) atau peringatan (warning) yang sama (sidik jari sama) dalam jendela 1 jam.
--                Kejadian yang berulang tidak menambah baris, hanya menambah kolom jumlah dan memperbarui waktu_akhir, sehingga badai galat tidak
--                memenuhi tabel. Catatan info/debug dan log akses per permintaan TIDAK pernah disimpan. Hanya ditulis aplikasi dan dibaca superadmin;
--                dihapus oleh pembersihan otomatis: error setelah LOG_RETENSI_ERROR_HARI (30), warning setelah LOG_RETENSI_WARN_HARI (7), dan
--                paling banyak LOG_MAKS_BARIS (50000) baris. Rahasia (kata sandi, token) disamarkan sebelum disimpan; badan permintaan tidak disimpan.
-- ============================================================

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'server_log')
BEGIN
    CREATE TABLE server_log (
        id           BIGINT IDENTITY(1,1) NOT NULL CONSTRAINT pk_server_log PRIMARY KEY,
        waktu        DATETIME2(3)  NOT NULL,   -- kejadian pertama pada baris ini (UTC)
        waktu_akhir  DATETIME2(3)  NOT NULL,   -- kejadian terakhir yang digabung ke baris ini (UTC)
        level        VARCHAR(5)    NOT NULL,   -- error atau warn
        sumber       NVARCHAR(80)  NOT NULL,   -- bagian aplikasi, mis. INAPROC SYNC, HTTP, PANIK
        pesan        NVARCHAR(2000) NOT NULL,
        detail       NVARCHAR(MAX) NULL,       -- mis. jejak tumpukan panik
        jumlah       INT           NOT NULL CONSTRAINT df_server_log_jumlah DEFAULT 1,
        sidik        CHAR(40)      NOT NULL,   -- sidik jari kejadian (sumber + rute + pesan yang dinormalkan)
        rute         NVARCHAR(200) NULL,       -- pola rute (tanpa nilai parameter), hanya untuk kejadian dari permintaan HTTP
        metode       VARCHAR(10)   NULL,
        status_http  SMALLINT      NULL,
        durasi_ms    INT           NULL,
        request_id   NVARCHAR(64)  NULL,       -- sama dengan header X-Request-ID dan kolom request_id pada audit_log
        user_id      NVARCHAR(64)  NULL
    );
END;

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'ix_server_log_akhir' AND object_id = OBJECT_ID('server_log'))
    CREATE INDEX ix_server_log_akhir ON server_log (waktu_akhir DESC, id DESC);

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'ix_server_log_sidik' AND object_id = OBJECT_ID('server_log'))
    CREATE INDEX ix_server_log_sidik ON server_log (sidik, waktu);

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'ix_server_log_level_akhir' AND object_id = OBJECT_ID('server_log'))
    CREATE INDEX ix_server_log_level_akhir ON server_log (level, waktu_akhir DESC);
