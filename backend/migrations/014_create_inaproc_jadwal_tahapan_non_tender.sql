IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_jadwal_tahapan_non_tender')
BEGIN
    CREATE TABLE inaproc_jadwal_tahapan_non_tender (
        row_key NVARCHAR(255) PRIMARY KEY,
        kd_akt NVARCHAR(50) NULL,
        kd_klpd NVARCHAR(20) NULL,
        kd_lpse NVARCHAR(50) NULL,
        kd_nontender NVARCHAR(50) NULL,
        kd_satker NVARCHAR(50) NULL,
        kd_satker_str NVARCHAR(50) NULL,
        kd_tahapan NVARCHAR(50) NULL,
        nama_akt NVARCHAR(500) NULL,
        nama_tahapan NVARCHAR(500) NULL,
        tahun_anggaran NVARCHAR(10) NULL,
        tgl_akhir DATETIME2 NULL,
        tgl_awal DATETIME2 NULL,
        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_ijtnt_kd_klpd_tahun ON inaproc_jadwal_tahapan_non_tender(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_ijtnt_kd_nontender ON inaproc_jadwal_tahapan_non_tender(kd_nontender);
    CREATE INDEX idx_ijtnt_kd_satker ON inaproc_jadwal_tahapan_non_tender(kd_satker);
END;