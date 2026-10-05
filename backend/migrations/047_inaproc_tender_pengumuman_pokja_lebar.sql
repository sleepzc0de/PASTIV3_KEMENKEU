-- Pokja sebuah tender terdiri dari beberapa anggota, dan API Inaproc mengirim semua NIP/nama mereka dalam satu kolom
-- (nip_pokja, nama_pokja). Migrasi 029 membuatnya NVARCHAR(50)/NVARCHAR(255) sehingga penarikan pengumuman menggugurkan baris yang
-- anggotanya banyak ("String or binary data would be truncated ... column 'nip_pokja'"). Tabel pengumuman non tender (migrasi 018)
-- sudah memakai NVARCHAR(MAX) untuk hal yang sama; di sini disamakan. Idempotent: COL_LENGTH bernilai -1 untuk kolom MAX.
IF COL_LENGTH('inaproc_tender_pengumuman', 'nip_pokja') IS NOT NULL AND COL_LENGTH('inaproc_tender_pengumuman', 'nip_pokja') <> -1
BEGIN
    ALTER TABLE inaproc_tender_pengumuman ALTER COLUMN nip_pokja NVARCHAR(MAX) NULL;
END;

IF COL_LENGTH('inaproc_tender_pengumuman', 'nama_pokja') IS NOT NULL AND COL_LENGTH('inaproc_tender_pengumuman', 'nama_pokja') <> -1
BEGIN
    ALTER TABLE inaproc_tender_pengumuman ALTER COLUMN nama_pokja NVARCHAR(MAX) NULL;
END;
