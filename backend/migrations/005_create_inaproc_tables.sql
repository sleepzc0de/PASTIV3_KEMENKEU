IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_history_kaji_ulang')
BEGIN
    CREATE TABLE inaproc_history_kaji_ulang (
        datamart_id NVARCHAR(50) PRIMARY KEY,
        tahun_anggaran NVARCHAR(10) NULL,
        kd_klpd NVARCHAR(20) NULL,
        nama_klpd NVARCHAR(255) NULL,
        jenis_klpd NVARCHAR(50) NULL,
        kd_satker NVARCHAR(20) NULL,
        kd_satker_str NVARCHAR(50) NULL,
        nama_satker NVARCHAR(255) NULL,
        kd_rup_lama NVARCHAR(50) NULL,
        kd_rup_baru NVARCHAR(50) NULL,
        jenis_paket NVARCHAR(50) NULL,
        jenis_revisi NVARCHAR(100) NULL,
        alasan_kajiulang NVARCHAR(MAX) NULL,
        tgl_kaji_ulang DATETIME2 NULL,
        event_date DATETIME2 NULL,
        inserted_date_src DATETIME2 NULL,
        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_ikju_kd_klpd_tahun ON inaproc_history_kaji_ulang(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_ikju_kd_satker ON inaproc_history_kaji_ulang(kd_satker);
END;

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_sync_log')
BEGIN
    CREATE TABLE inaproc_sync_log (
        id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
        endpoint NVARCHAR(100) NOT NULL,
        kode_klpd NVARCHAR(20) NULL,
        tahun NVARCHAR(10) NULL,
        jenis_paket NVARCHAR(50) NULL,
        total_rows_synced INT NOT NULL DEFAULT 0,
        status NVARCHAR(20) NOT NULL, -- success | failed
        error_message NVARCHAR(MAX) NULL,
        synced_by UNIQUEIDENTIFIER NULL,
        started_at DATETIME2 NOT NULL,
        finished_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );
END;