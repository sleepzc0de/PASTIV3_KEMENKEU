-- ============================================================
-- Verifikasi Schema: Deteksi kolom yang salah tipe
-- Jalankan kapan saja untuk audit cepat, tidak mengubah apapun
-- ============================================================

PRINT '=== Cek kolom employees yang seharusnya NVARCHAR tapi bukan ===';
SELECT c.name AS column_name, t.name AS data_type
FROM sys.columns c
JOIN sys.types t ON c.user_type_id = t.user_type_id
WHERE c.object_id = OBJECT_ID('employees')
  AND t.name = 'uniqueidentifier'
  AND c.name != 'id';

PRINT '=== Cek kolom users yang seharusnya NVARCHAR tapi bukan ===';
SELECT c.name AS column_name, t.name AS data_type
FROM sys.columns c
JOIN sys.types t ON c.user_type_id = t.user_type_id
WHERE c.object_id = OBJECT_ID('users')
  AND t.name = 'uniqueidentifier'
  AND c.name NOT IN ('id', 'employee_id');

PRINT '=== Cek kolom sso_states yang seharusnya NVARCHAR tapi bukan ===';
SELECT c.name AS column_name, t.name AS data_type
FROM sys.columns c
JOIN sys.types t ON c.user_type_id = t.user_type_id
WHERE c.object_id = OBJECT_ID('sso_states')
  AND t.name = 'uniqueidentifier';

PRINT '=== Daftar semua tabel PASTI V3 ===';
SELECT name FROM sys.tables WHERE name IN ('users', 'refresh_tokens', 'employees', 'sso_states');

PRINT 'Jika ketiga query pertama TIDAK mengembalikan baris apapun, skema sudah benar.';