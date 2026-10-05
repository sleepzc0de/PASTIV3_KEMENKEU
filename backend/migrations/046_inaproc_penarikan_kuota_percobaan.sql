-- Penarikan Inaproc yang stabil: kebijakan percobaan ulang (3 kali sehari, lalu istirahat 8 jam) dan hitungan kuota permintaan.

-- Isian tugas sebagai JSON, supaya tugas yang gagal bisa dicoba ulang persis sama (termasuk penarikan manual per kode).
IF COL_LENGTH('inaproc_penarikan', 'permintaan') IS NULL
BEGIN
    ALTER TABLE inaproc_penarikan ADD permintaan NVARCHAR(1000) NULL;
END;

-- Pencarian tugas yang gagal dan belum pulih (penjadwal tiap beberapa menit) membaca baris terbaru menurut waktu dibuat.
IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'idx_inaproc_penarikan_dibuat' AND object_id = OBJECT_ID('inaproc_penarikan'))
BEGIN
    CREATE INDEX idx_inaproc_penarikan_dibuat ON inaproc_penarikan(dibuat, status) INCLUDE (dataset, parameter);
END;

-- Jumlah permintaan ke Inaproc per menit (menit = detik unix / 60). Inaproc membatasi 1.000 permintaan per 60 detik dan 5.000 per jam;
-- hitungan disimpan supaya kuota yang sudah terpakai tidak "terlupa" saat server dimulai ulang atau dua salinan backend hidup bersamaan.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'inaproc_kuota_menit')
BEGIN
    CREATE TABLE inaproc_kuota_menit (
        menit BIGINT NOT NULL PRIMARY KEY,
        jumlah INT NOT NULL DEFAULT 0,
        diubah DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );
END;
