-- Kolom yang terbukti terlalu sempit untuk data Inaproc asli (penarikan pertama; lihat catatan "nilai dipotong" di riwayat penarikan):
--   * kd_rup / kd_rup_paket: satu paket bisa memuat beberapa kode RUP dipisah ";" (mis. "66261043;67104578;67104579;..."), sedangkan kolomnya
--     NVARCHAR(50). Dilebarkan ke NVARCHAR(450), batas terbesar yang masih boleh menjadi kunci indeks (900 byte), sehingga indeks kd_rup tetap
--     dipakai. Berlaku juga untuk tabel tender yang sepadan, karena paket tender pun dapat memuat beberapa RUP.
--   * nama_paket, nama_penyedia, no_realisasi pada pencatatan non tender (+ realisasi): isinya melebihi lebar kolom pada ratusan baris.
--     Dilebarkan ke NVARCHAR(MAX) (tidak ada indeks pada kolom ini).
-- Idempotent: hanya kolom yang masih lebih sempit dari sasarannya yang diubah; kolom yang tidak ada dilewati. Data tidak berubah.
DECLARE @lebar TABLE (tabel SYSNAME, kolom SYSNAME, panjang INT); -- panjang dalam karakter; -1 = MAX
INSERT INTO @lebar (tabel, kolom, panjang) VALUES
    ('inaproc_non_tender_pengumuman', 'kd_rup', 450),
    ('inaproc_non_tender_selesai', 'kd_rup', 450),
    ('inaproc_pencatatan_non_tender', 'kd_rup', 450),
    ('inaproc_pencatatan_swakelola', 'kd_rup', 450),
    ('inaproc_tender_pengumuman', 'kd_rup', 450),
    ('inaproc_tender_selesai', 'kd_rup', 450),
    ('inaproc_pencatatan_non_tender_realisasi', 'kd_rup_paket', 450),
    ('inaproc_tender_selesai_nilai', 'kd_rup_paket', 450),
    ('inaproc_pencatatan_non_tender', 'nama_paket', -1),
    ('inaproc_pencatatan_non_tender_realisasi', 'nama_paket', -1),
    ('inaproc_pencatatan_non_tender_realisasi', 'nama_penyedia', -1),
    ('inaproc_pencatatan_non_tender_realisasi', 'no_realisasi', -1);

DECLARE @tabel SYSNAME, @kolom SYSNAME, @panjang INT, @byte INT, @sql NVARCHAR(MAX);
DECLARE daftar CURSOR LOCAL FAST_FORWARD FOR SELECT tabel, kolom, panjang FROM @lebar;
OPEN daftar;
FETCH NEXT FROM daftar INTO @tabel, @kolom, @panjang;
WHILE @@FETCH_STATUS = 0
BEGIN
    SET @byte = COL_LENGTH(@tabel, @kolom); -- NULL: tidak ada; -1: sudah MAX; selain itu: byte (2 per karakter)
    IF @byte IS NOT NULL AND @byte <> -1 AND (@panjang = -1 OR @byte < @panjang * 2)
    BEGIN
        SET @sql = N'ALTER TABLE ' + QUOTENAME(@tabel) + N' ALTER COLUMN ' + QUOTENAME(@kolom) + N' NVARCHAR(' +
            CASE WHEN @panjang = -1 THEN N'MAX' ELSE CAST(@panjang AS NVARCHAR(10)) END + N') NULL';
        EXEC sp_executesql @sql;
    END;
    FETCH NEXT FROM daftar INTO @tabel, @kolom, @panjang;
END;
CLOSE daftar;
DEALLOCATE daftar;
