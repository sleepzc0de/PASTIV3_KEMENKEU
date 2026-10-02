IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_non_tender_ekontrak')
BEGIN
    CREATE TABLE inaproc_non_tender_ekontrak (
        row_key NVARCHAR(255) PRIMARY KEY,
        kd_klpd NVARCHAR(20) NULL,
        kd_tender NVARCHAR(50) NULL,
        tahun_anggaran NVARCHAR(10) NULL,
        nama_paket NVARCHAR(1000) NULL,
        alamat_satker NVARCHAR(1000) NULL,
        -- Tiga field berikut selalu berupa array JSON ("[]" kalau kosong), sesuai kontrak API.
        bapbast_history_json NVARCHAR(MAX) NULL,
        spmkspp_history_json NVARCHAR(MAX) NULL,
        penilaian_kinerja_penyedia NVARCHAR(MAX) NULL,
        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,
        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_inte_kd_klpd_tahun ON inaproc_non_tender_ekontrak(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_inte_kd_tender ON inaproc_non_tender_ekontrak(kd_tender);
END;
