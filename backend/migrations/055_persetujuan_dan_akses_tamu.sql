-- ============================================================
-- Migration 055: Persetujuan penggunaan aplikasi, NIP akun non-SSO, dan penyederhanaan role akun
-- Aman dijalankan berulang kali (idempotent)
--
-- Aturan akses baru:
--   * Pengguna tanpa peran data adalah TAMU: tidak dapat membuka fitur apa pun, hanya membaca dan menyetujui pernyataan penggunaan aplikasi.
--   * Setiap pengguna (kecuali superadmin) harus menyetujui pernyataan sekali: persetujuan_at + persetujuan_versi (versi pernyataan yang disetujui).
--   * Akun non-SSO mengisi nama lengkap, NIP, dan email saat menyetujui; NIP-nya disimpan di users.nip (akun SSO memakai employees.nip).
--   * Hanya superadmin (ditetapkan di .env, bukan di database) yang mengakses seluruh fitur. Role akun "admin" ditiadakan: akun admin lama menjadi
--     pengguna biasa dengan peran data Pengguna Barang (melihat semua data dan mengelola pengguna kecuali superadmin).
-- ============================================================

IF COL_LENGTH('users', 'persetujuan_at') IS NULL
    ALTER TABLE users ADD persetujuan_at DATETIME2 NULL;

GO

IF COL_LENGTH('users', 'persetujuan_versi') IS NULL
    ALTER TABLE users ADD persetujuan_versi NVARCHAR(20) NULL;

GO

IF COL_LENGTH('users', 'nip') IS NULL
    ALTER TABLE users ADD nip NVARCHAR(30) NULL;

GO

-- Akun admin lama (bukan superadmin permanen) -> peran data Pengguna Barang, lalu role akunnya turun menjadi user.
-- Peran dimasukkan lebih dulu: bila gagal di tengah jalan, role admin belum berubah dan skrip aman diulang.
INSERT INTO user_roles (user_id, role, kode, dibuat_oleh)
SELECT u.id, N'pengguna_barang', N'', N'migrasi 055 (admin lama)'
FROM users u
WHERE u.role = N'admin' AND u.is_protected = 0
  AND NOT EXISTS (SELECT 1 FROM user_roles r WHERE r.user_id = u.id AND r.role = N'pengguna_barang' AND r.kode = N'');

GO

UPDATE users SET role = N'user', updated_at = SYSUTCDATETIME() WHERE role = N'admin' AND is_protected = 0;
