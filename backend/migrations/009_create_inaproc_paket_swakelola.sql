IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_paket_swakelola')
BEGIN
    CREATE TABLE inaproc_paket_swakelola (
        row_key NVARCHAR(255) PRIMARY KEY,
        kd_klpd NVARCHAR(20) NULL,
        kd_satker NVARCHAR(50) NULL,
        kd_rup NVARCHAR(50) NULL,
        nama_klpd NVARCHAR(255) NULL,
        nama_satker NVARCHAR(255) NULL,
        nama_paket NVARCHAR(500) NULL,
        tahun_anggaran NVARCHAR(10) NULL,
        status NVARCHAR(100) NULL,
        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_ips_kd_klpd_tahun ON inaproc_paket_swakelola(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_ips_kd_satker ON inaproc_paket_swakelola(kd_satker);
    CREATE INDEX idx_ips_status ON inaproc_paket_swakelola(status);
END;