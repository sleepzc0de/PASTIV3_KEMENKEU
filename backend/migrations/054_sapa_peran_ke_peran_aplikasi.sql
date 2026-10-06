-- ============================================================
-- Migration 054: Peran SAPA dipindahkan ke peran data aplikasi (user_roles)
-- Aman dijalankan berulang kali (idempotent)
--
-- SAPA tidak lagi menetapkan peran sendiri: peran satker/kanwil/ue1 di SAPA kini diturunkan dari peran data aplikasi yang sedang aktif
-- (Satker 6 digit, Kanwil 9 digit, UE1 5 digit, Pengguna Barang, atau admin/superadmin), diberikan admin di Manajemen Pengguna.
-- Agar pengguna SAPA yang sudah ada tidak kehilangan akses, penetapan lama disalin ke user_roles:
--   satker : kode satker 18 digit di sapa_peran -> kode 6 digit (karakter ke-10 sampai ke-15)
--   ue1    : kode UE1 5 digit apa adanya
--   kanwil : sapa_peran tidak menyimpan kode kanwil, jadi diturunkan dari kode satker pegawai di data SSO (9 karakter pertama employees.kode_satker);
--            yang tidak punya kode satker SSO TIDAK disalin dan harus diberi peran Kanwil oleh admin
-- Perubahan perilaku yang perlu diketahui: Kanwil kini hanya melihat usulan di bawah Kanwil-nya (dulu semua usulan), dan peran yang disalin ikut
-- membatasi data di fitur lain (Digitalisasi Aset, Dashboard) sesuai cakupannya.
-- Tabel sapa_peran dibiarkan (tidak dihapus dan tidak dibaca lagi) sebagai cadangan.
-- ============================================================

INSERT INTO user_roles (user_id, role, kode, dibuat_oleh)
SELECT p.user_id, N'satker', SUBSTRING(p.kode_satker, 10, 6), N'migrasi dari SAPA'
FROM sapa_peran p
WHERE p.peran = N'satker'
  AND p.kode_satker IS NOT NULL AND LEN(p.kode_satker) >= 15
  AND SUBSTRING(p.kode_satker, 10, 6) NOT LIKE N'%[^0-9]%'
  AND NOT EXISTS (SELECT 1 FROM user_roles r WHERE r.user_id = p.user_id AND r.role = N'satker' AND r.kode = SUBSTRING(p.kode_satker, 10, 6));

GO

INSERT INTO user_roles (user_id, role, kode, dibuat_oleh)
SELECT p.user_id, N'ue1', p.kode_ue1, N'migrasi dari SAPA'
FROM sapa_peran p
WHERE p.peran = N'ue1'
  AND p.kode_ue1 IS NOT NULL AND LEN(p.kode_ue1) = 5 AND p.kode_ue1 NOT LIKE N'%[^0-9]%'
  AND NOT EXISTS (SELECT 1 FROM user_roles r WHERE r.user_id = p.user_id AND r.role = N'ue1' AND r.kode = p.kode_ue1);

GO

INSERT INTO user_roles (user_id, role, kode, dibuat_oleh)
SELECT p.user_id, N'kanwil', LEFT(e.kode_satker, 9), N'migrasi dari SAPA (kode dari data SSO)'
FROM sapa_peran p
JOIN users u ON u.id = p.user_id
JOIN employees e ON e.id = u.employee_id
WHERE p.peran = N'kanwil'
  AND e.kode_satker IS NOT NULL AND LEN(e.kode_satker) >= 15
  AND LEFT(e.kode_satker, 15) NOT LIKE N'%[^0-9]%'
  AND NOT EXISTS (SELECT 1 FROM user_roles r WHERE r.user_id = p.user_id AND r.role = N'kanwil' AND r.kode = LEFT(e.kode_satker, 9));
