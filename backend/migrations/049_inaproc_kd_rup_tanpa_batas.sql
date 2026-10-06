-- Migrasi 048 melebarkan kd_rup ke NVARCHAR(450) (batas terbesar yang masih boleh diindeks), tetapi data asli masih melampauinya: satu baris
-- pencatatan non tender memuat puluhan kode RUP dipisah ";" (lebih dari 450 karakter). Daftar kode semacam itu tidak punya batas yang wajar,
-- jadi kolomnya dijadikan NVARCHAR(MAX), dan indeks pada kolom itu dibuang karena kolom MAX tidak bisa menjadi kunci indeks.
-- Tidak ada query aplikasi yang mencari berdasarkan kd_rup pada tabel-tabel ini (daftar lokal disaring per KLPD/tahun; corong dasbor memecah
-- kolomnya dengan STRING_SPLIT), jadi indeks itu tidak terpakai.
-- Idempotent: indeks dibuang hanya bila ada, kolom diubah hanya bila belum MAX.
DECLARE @daftar TABLE (tabel SYSNAME, kolom SYSNAME, indeks SYSNAME NULL);
INSERT INTO @daftar (tabel, kolom, indeks) VALUES
    ('inaproc_non_tender_pengumuman', 'kd_rup', 'idx_intp_kd_rup'),
    ('inaproc_non_tender_selesai', 'kd_rup', 'idx_intsl_kd_rup'),
    ('inaproc_pencatatan_non_tender', 'kd_rup', 'idx_ipnt_kd_rup'),
    ('inaproc_pencatatan_swakelola', 'kd_rup', 'idx_ips_kd_rup'),
    ('inaproc_tender_pengumuman', 'kd_rup', 'idx_itp_kd_rup'),
    ('inaproc_tender_selesai', 'kd_rup', 'idx_its_kd_rup'),
    ('inaproc_pencatatan_non_tender_realisasi', 'kd_rup_paket', 'idx_ipntr_kd_rup_paket'),
    ('inaproc_tender_selesai_nilai', 'kd_rup_paket', NULL);

DECLARE @tabel SYSNAME, @kolom SYSNAME, @indeks SYSNAME, @byte INT, @sql NVARCHAR(MAX);
DECLARE daftar CURSOR LOCAL FAST_FORWARD FOR SELECT tabel, kolom, indeks FROM @daftar;
OPEN daftar;
FETCH NEXT FROM daftar INTO @tabel, @kolom, @indeks;
WHILE @@FETCH_STATUS = 0
BEGIN
    SET @byte = COL_LENGTH(@tabel, @kolom); -- NULL: tidak ada; -1: sudah MAX
    IF @byte IS NOT NULL AND @byte <> -1
    BEGIN
        IF @indeks IS NOT NULL AND EXISTS (SELECT 1 FROM sys.indexes WHERE name = @indeks AND object_id = OBJECT_ID(@tabel))
        BEGIN
            SET @sql = N'DROP INDEX ' + QUOTENAME(@indeks) + N' ON ' + QUOTENAME(@tabel);
            EXEC sp_executesql @sql;
        END;
        SET @sql = N'ALTER TABLE ' + QUOTENAME(@tabel) + N' ALTER COLUMN ' + QUOTENAME(@kolom) + N' NVARCHAR(MAX) NULL';
        EXEC sp_executesql @sql;
    END;
    FETCH NEXT FROM daftar INTO @tabel, @kolom, @indeks;
END;
CLOSE daftar;
DEALLOCATE daftar;
