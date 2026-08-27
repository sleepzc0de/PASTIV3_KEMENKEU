IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_paket_anggaran_penyedia')
BEGIN
    CREATE TABLE inaproc_paket_anggaran_penyedia (
        row_key NVARCHAR(255) PRIMARY KEY,
        asal_dana NVARCHAR(20) NULL,
        asal_dana_klpd NVARCHAR(255) NULL,
        asal_dana_satker NVARCHAR(255) NULL,
        jenis_klpd NVARCHAR(100) NULL,
        kd_kegiatan NVARCHAR(50) NULL,
        kd_klpd NVARCHAR(20) NULL,
        kd_komponen NVARCHAR(50) NULL,
        kd_rup NVARCHAR(50) NULL,
        kd_rup_lokal NVARCHAR(50) NULL,
        kd_satker NVARCHAR(50) NULL,
        kd_satker_str NVARCHAR(50) NULL,
        kd_subkegiatan NVARCHAR(50) NULL,
        mak NVARCHAR(255) NULL,
        nama_klpd NVARCHAR(255) NULL,
        nama_satker NVARCHAR(255) NULL,
        pagu BIGINT NULL,
        status_aktif_rup BIT NULL,
        status_delete_rup BIT NULL,
        status_umumkan_rup NVARCHAR(50) NULL,
        sumber_dana NVARCHAR(255) NULL,
        tahun_anggaran NVARCHAR(10) NULL,
        tahun_anggaran_dana NVARCHAR(10) NULL,
        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_ipap_kd_klpd_tahun ON inaproc_paket_anggaran_penyedia(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_ipap_kd_satker ON inaproc_paket_anggaran_penyedia(kd_satker);
    CREATE INDEX idx_ipap_kd_rup ON inaproc_paket_anggaran_penyedia(kd_rup);
END;