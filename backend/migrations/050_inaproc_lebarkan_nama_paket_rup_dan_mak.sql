-- Dua kolom lagi yang terbukti terlalu sempit pada penarikan penuh ke Inaproc asli:
--   * nama_paket pada paket RUP penyedia/swakelola (+ terumumkan): satu paket bernama gabungan panjang ("Pakaian Dinas Pegawai/Perawat (Jawa Tengah),
--     Pakaian Kerja Satpam (Jawa Tengah), ...") melebihi NVARCHAR(500) sehingga barisnya gagal disimpan (handler lama membuang baris itu).
--     Keempat tabel yang sama jenisnya dilebarkan ke NVARCHAR(MAX) (tidak ada indeks pada kolom ini).
--   * mak pada paket e-purchasing V6: melebihi NVARCHAR(255) pada dua baris (nilainya dipotong oleh jaring pengaman lebar kolom).
-- Idempotent: hanya kolom yang belum NVARCHAR(MAX) yang diubah; kolom yang tidak ada dilewati. Data tidak berubah.
DECLARE @daftar TABLE (tabel SYSNAME, kolom SYSNAME);
INSERT INTO @daftar (tabel, kolom) VALUES
    ('inaproc_paket_penyedia', 'nama_paket'),
    ('inaproc_paket_swakelola', 'nama_paket'),
    ('inaproc_paket_swakelola_terumumkan', 'nama_paket'),
    ('inaproc_paket_penyedia_terumumkan', 'nama_paket'),
    ('inaproc_ekatalog6_paket_epurchasing', 'mak');

DECLARE @tabel SYSNAME, @kolom SYSNAME, @sql NVARCHAR(MAX);
DECLARE daftar CURSOR LOCAL FAST_FORWARD FOR SELECT tabel, kolom FROM @daftar;
OPEN daftar;
FETCH NEXT FROM daftar INTO @tabel, @kolom;
WHILE @@FETCH_STATUS = 0
BEGIN
    IF COL_LENGTH(@tabel, @kolom) IS NOT NULL AND COL_LENGTH(@tabel, @kolom) <> -1 -- NULL: tidak ada; -1: sudah MAX
    BEGIN
        SET @sql = N'ALTER TABLE ' + QUOTENAME(@tabel) + N' ALTER COLUMN ' + QUOTENAME(@kolom) + N' NVARCHAR(MAX) NULL';
        EXEC sp_executesql @sql;
    END;
    FETCH NEXT FROM daftar INTO @tabel, @kolom;
END;
CLOSE daftar;
DEALLOCATE daftar;
