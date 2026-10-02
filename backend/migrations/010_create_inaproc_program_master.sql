IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_program_master')
BEGIN
    CREATE TABLE inaproc_program_master (
        row_key NVARCHAR(255) PRIMARY KEY,
        is_deleted BIT NULL,
        jenis_klpd NVARCHAR(100) NULL,
        kd_klpd NVARCHAR(20) NULL,
        kd_program NVARCHAR(50) NULL,
        kd_program_lokal NVARCHAR(50) NULL,
        kd_program_str NVARCHAR(50) NULL,
        kd_satker NVARCHAR(50) NULL,
        nama_klpd NVARCHAR(255) NULL,
        nama_program NVARCHAR(500) NULL,
        pagu_program BIGINT NULL,
        tahun_anggaran NVARCHAR(10) NULL,
        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_ipm_kd_klpd_tahun ON inaproc_program_master(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_ipm_kd_satker ON inaproc_program_master(kd_satker);
END;