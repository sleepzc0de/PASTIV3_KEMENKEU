-- Nama kolom sama persis dengan nama field di respons API Inaproc
-- (/tender/pencatatan-swakelola). Daftar yang sama dipakai handler Go untuk
-- membangun INSERT, jadi jangan menambah/mengubah kolom di sini tanpa menyesuaikan
-- daftar field di backend/handlers/inaproc_tender_swakelola.go (ada tes yang menjaganya).
-- "pct" pada kd_swakelola_pct / status_swakelola_pct / nilai_pdn_pct / nilai_umk_pct berarti
-- pencatatan (bukan persen); nilai_pdn_pct dan nilai_umk_pct adalah nilai rupiah.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_pencatatan_swakelola')
BEGIN
    CREATE TABLE inaproc_pencatatan_swakelola (
        row_key NVARCHAR(255) PRIMARY KEY,

        -- Satker & KLPD
        kd_klpd NVARCHAR(20) NULL,
        jenis_klpd NVARCHAR(100) NULL,
        nama_klpd NVARCHAR(255) NULL,
        kd_satker NVARCHAR(50) NULL,
        kd_satker_str NVARCHAR(50) NULL,
        nama_satker NVARCHAR(500) NULL,

        -- Paket (kode dibaca sebagai teks: bentuknya angka, tetapi bukan untuk dihitung)
        kd_lpse NVARCHAR(50) NULL,
        kd_swakelola_pct NVARCHAR(50) NULL,
        kd_pkt_dce NVARCHAR(50) NULL,
        kd_rup NVARCHAR(50) NULL,
        tahun_anggaran NVARCHAR(10) NULL,
        nama_paket NVARCHAR(1000) NULL,
        sumber_dana NVARCHAR(100) NULL,
        uraian_pekerjaan NVARCHAR(MAX) NULL,
        tipe_swakelola NVARCHAR(20) NULL,
        tipe_swakelola_nama NVARCHAR(255) NULL,

        -- PPK
        nama_ppk NVARCHAR(500) NULL,
        nip_ppk NVARCHAR(50) NULL,

        -- Status
        status_swakelola_pct NVARCHAR(100) NULL,
        status_swakelola_pct_ket NVARCHAR(255) NULL,
        alasan_pembatalan NVARCHAR(MAX) NULL,
        informasi_lainnya NVARCHAR(MAX) NULL,

        -- Nilai (DECIMAL, bukan BIGINT, supaya pecahan rupiah tidak terpotong)
        pagu DECIMAL(24, 2) NULL,
        nilai_pdn_pct DECIMAL(24, 2) NULL,
        nilai_umk_pct DECIMAL(24, 2) NULL,
        total_realisasi DECIMAL(24, 2) NULL,

        -- Tanggal
        tgl_buat_paket DATETIME2 NULL,
        tgl_mulai_paket DATETIME2 NULL,
        tgl_selesai_paket DATETIME2 NULL,

        -- Field lain dari API yang belum dipetakan ke kolom di atas (objek JSON), supaya tidak ada data yang hilang.
        extra_json NVARCHAR(MAX) NULL,

        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_ips_kd_klpd_tahun ON inaproc_pencatatan_swakelola(kd_klpd, tahun_anggaran);
    CREATE INDEX idx_ips_kd_swakelola_pct ON inaproc_pencatatan_swakelola(kd_swakelola_pct);
    CREATE INDEX idx_ips_kd_rup ON inaproc_pencatatan_swakelola(kd_rup);
END;
