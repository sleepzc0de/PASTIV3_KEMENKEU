IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_paket_swakelola_terumumkan')
BEGIN
    CREATE TABLE inaproc_paket_swakelola_terumumkan (
        row_key NVARCHAR(255) PRIMARY KEY,
        jenis_klpd NVARCHAR(100) NULL,
        kd_klpd NVARCHAR(20) NULL,
        kd_klpd_penyelenggara NVARCHAR(20) NULL,
        kd_rup NVARCHAR(50) NULL,
        kd_rup_lokal NVARCHAR(50) NULL,
        kd_satker NVARCHAR(50) NULL,
        kd_satker_str NVARCHAR(50) NULL,
        nama_klpd NVARCHAR(255) NULL,
        nama_klpd_penyelenggara NVARCHAR(255) NULL,
        nama_paket NVARCHAR(500) NULL,
        nama_ppk NVARCHAR(255) NULL,
        nama_satker NVARCHAR(255) NULL,
        nama_satker_penyelenggara NVARCHAR(255) NULL,
        nip_ppk NVARCHAR(50) NULL,
        pagu BIGINT NULL,
        status_aktif_rup BIT NULL,
        status_delete_rup BIT NULL,
        status_umumkan_rup NVARCHAR(100) NULL,
        tahun_anggaran NVARCHAR(10) NULL,
        tgl_akhir_pelaksanaan_kontrak DATETIME2 NULL,
        tgl_awal_pelaksanaan_kontrak DATETIME2 NULL,
        tgl_buat_paket DATETIME2 NULL,
        tgl_pengumuman_paket DATETIME2 NULL,
        tipe_swakelola NVARCHAR(20) NULL,
        uraian_pekerjaan NVARCHAR(MAX) NULL,
        username_ppk NVARCHAR(100) NULL,
        volume_pekerjaan NVARCHAR(255) NULL,
        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_ipst_kd_klpd_tahun ON inaproc_paket_swakelola_terumumkan(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_ipst_kd_satker ON inaproc_paket_swakelola_terumumkan(kd_satker);
    CREATE INDEX idx_ipst_status_umumkan ON inaproc_paket_swakelola_terumumkan(status_umumkan_rup);
END;