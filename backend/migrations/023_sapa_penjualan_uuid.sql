-- ============================================================
-- Migration 023: SAPA - UUID untuk usulan penjualan
-- Aman dijalankan berulang kali (idempotent)
--
-- Alamat halaman dan API usulan memakai UUID (acak, tidak berurutan) sebagai pengganti nomor id berurutan, supaya
-- alamat usulan lain tidak bisa ditebak dengan menambah/mengurangi angka. id BIGINT tetap menjadi kunci internal
-- (relasi tahap dan dokumen); hak akses tetap diperiksa di aplikasi untuk setiap permintaan.
-- ============================================================

-- NEWID() dievaluasi per baris, jadi baris yang sudah ada mendapat UUID masing-masing. Bila ternyata ada yang kembar,
-- pembuatan indeks unik di bawah gagal dan seluruh migrasi dibatalkan.
IF NOT EXISTS (SELECT * FROM sys.columns WHERE object_id = OBJECT_ID('sapa_penjualan') AND name = 'uuid')
BEGIN
    ALTER TABLE sapa_penjualan ADD uuid UNIQUEIDENTIFIER NOT NULL CONSTRAINT df_sapa_penjualan_uuid DEFAULT NEWID();
END;
GO

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'ux_sapa_penjualan_uuid' AND object_id = OBJECT_ID('sapa_penjualan'))
BEGIN
    CREATE UNIQUE INDEX ux_sapa_penjualan_uuid ON sapa_penjualan(uuid);
END;
GO
