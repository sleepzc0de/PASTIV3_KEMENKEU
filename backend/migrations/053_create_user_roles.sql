-- ============================================================
-- Migration 053: Peran data pengguna (Pengguna Barang, UE1, Kanwil, Satker)
-- Aman dijalankan berulang kali (idempotent)
--
-- Seorang pengguna boleh memegang banyak peran tetapi bertindak sebagai satu peran dalam satu waktu (aktif = 1). Peran aktif menentukan cakupan
-- data yang boleh dilihat. Kode satker lengkap (mis. 015040199119091000KP) memuat semua tingkat:
--   ue1    : kode 5 digit  = karakter 1-5 kode satker      (mis. 01504)
--   kanwil : kode 9 digit  = karakter 1-9 kode satker      (UE1 + 4 digit, mis. 015040199)
--   satker : kode 6 digit  = karakter 10-15 kode satker    (mis. 119091)
-- Pengguna Barang melihat seluruh data (tanpa kode). Super Admin dan Admin berasal dari users.role, bukan dari tabel ini.
-- Pengguna tanpa baris di sini melihat semua data seperti sebelumnya, kecuali PERAN_DATA_WAJIB=true.
-- ============================================================

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'user_roles')
BEGIN
    CREATE TABLE user_roles (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        user_id UNIQUEIDENTIFIER NOT NULL,
        role NVARCHAR(20) NOT NULL,
        kode NVARCHAR(9) NOT NULL CONSTRAINT df_user_roles_kode DEFAULT N'',
        aktif BIT NOT NULL CONSTRAINT df_user_roles_aktif DEFAULT 0,
        dibuat_oleh NVARCHAR(100) NULL,
        dibuat_pada DATETIME2 NOT NULL CONSTRAINT df_user_roles_dibuat DEFAULT SYSUTCDATETIME(),
        CONSTRAINT fk_user_roles_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
        CONSTRAINT uq_user_roles UNIQUE (user_id, role, kode),
        CONSTRAINT ck_user_roles CHECK (
            (role = N'pengguna_barang' AND kode = N'')
            OR (role = N'ue1'    AND LEN(kode) = 5 AND kode NOT LIKE N'%[^0-9]%')
            OR (role = N'kanwil' AND LEN(kode) = 9 AND kode NOT LIKE N'%[^0-9]%')
            OR (role = N'satker' AND LEN(kode) = 6 AND kode NOT LIKE N'%[^0-9]%')
        )
    );
END;

GO

-- Paling banyak satu peran aktif per pengguna.
IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'ux_user_roles_aktif' AND object_id = OBJECT_ID('user_roles'))
    CREATE UNIQUE INDEX ux_user_roles_aktif ON user_roles(user_id) WHERE aktif = 1;
