-- ============================================================
-- Migration 051: Data SSO Kemenkeu tersimpan lengkap
-- Aman dijalankan berulang kali (idempotent)
--
-- Sebelumnya hanya sebagian klaim userinfo yang punya kolom sendiri di `employees`, sisanya hanya ada di raw_claims
-- (JSON) yang ditimpa tiap login. Sekarang:
--   * employees      : klaim id_token (raw_id_token_claims), hash klaim terakhir, serta waktu login pertama/terakhir dan jumlah login.
--   * employee_claims: SETIAP klaim (userinfo dan id_token) satu baris per kunci, jadi klaim apa pun, termasuk yang baru ditambah
--                      Kemenkeu kelak, bisa dicari dengan SQL biasa tanpa menambah kolom.
--   * employee_claim_history: salinan lengkap klaim tiap kali isinya berubah (mutasi, jabatan baru, dst.), tanpa menimpa yang lama.
-- Token SSO yang disimpan terenkripsi tetap di `sso_tokens` (migrasi 004).
-- ============================================================

IF COL_LENGTH('employees', 'raw_id_token_claims') IS NULL
    ALTER TABLE employees ADD raw_id_token_claims NVARCHAR(MAX) NULL;

IF COL_LENGTH('employees', 'claims_hash') IS NULL
    ALTER TABLE employees ADD claims_hash CHAR(64) NULL;

IF COL_LENGTH('employees', 'first_login_at') IS NULL
    ALTER TABLE employees ADD first_login_at DATETIME2 NULL;

IF COL_LENGTH('employees', 'last_login_at') IS NULL
    ALTER TABLE employees ADD last_login_at DATETIME2 NULL;

IF COL_LENGTH('employees', 'login_count') IS NULL
    ALTER TABLE employees ADD login_count INT NOT NULL CONSTRAINT df_employees_login_count DEFAULT 0;

GO

-- Pegawai yang sudah ada: waktu terdaftar dan terakhir diperbarui dipakai sebagai perkiraan login pertama dan terakhir.
UPDATE employees SET first_login_at = created_at WHERE first_login_at IS NULL;
UPDATE employees SET last_login_at = updated_at WHERE last_login_at IS NULL;
UPDATE employees SET login_count = 1 WHERE login_count = 0;

GO

-- Satu baris per klaim. sumber: 'userinfo' (endpoint userinfo) atau 'id_token'. tipe: string | number | bool | json | null.
-- Klaim larik/objek disimpan sebagai teks JSON dengan tipe 'json'.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'employee_claims')
BEGIN
    CREATE TABLE employee_claims (
        employee_id UNIQUEIDENTIFIER NOT NULL,
        sumber NVARCHAR(10) NOT NULL,
        claim_key NVARCHAR(150) NOT NULL,
        claim_value NVARCHAR(MAX) NULL,
        tipe NVARCHAR(10) NOT NULL,
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        CONSTRAINT pk_employee_claims PRIMARY KEY (employee_id, sumber, claim_key),
        CONSTRAINT fk_employee_claims_employee FOREIGN KEY (employee_id) REFERENCES employees(id) ON DELETE CASCADE,
        CONSTRAINT ck_employee_claims_sumber CHECK (sumber IN ('userinfo', 'id_token'))
    );

    CREATE INDEX idx_employee_claims_key ON employee_claims(claim_key, sumber);
END;

GO

-- Riwayat klaim: baris baru hanya bila isi klaim (di luar klaim yang berubah tiap login seperti iat/exp) berbeda dari baris sebelumnya.
-- claims_hash NULL = salinan awal dari raw_claims yang sudah ada sebelum riwayat dicatat.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'employee_claim_history')
BEGIN
    CREATE TABLE employee_claim_history (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        employee_id UNIQUEIDENTIFIER NOT NULL,
        direkam_pada DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        claims_hash CHAR(64) NULL,
        userinfo_claims NVARCHAR(MAX) NULL,
        id_token_claims NVARCHAR(MAX) NULL,
        scope NVARCHAR(500) NULL,
        CONSTRAINT fk_employee_claim_history_employee FOREIGN KEY (employee_id) REFERENCES employees(id) ON DELETE CASCADE
    );

    CREATE INDEX idx_employee_claim_history_employee ON employee_claim_history(employee_id, id);
END;

GO

-- Salin klaim yang sudah tersimpan di raw_claims ke employee_claims dan ke riwayat (hanya untuk pegawai yang belum punya).
INSERT INTO employee_claims (employee_id, sumber, claim_key, claim_value, tipe)
SELECT e.id, N'userinfo', j.[key], j.value,
       CASE j.type WHEN 0 THEN N'null' WHEN 1 THEN N'string' WHEN 2 THEN N'number' WHEN 3 THEN N'bool' ELSE N'json' END
FROM employees e
CROSS APPLY OPENJSON(e.raw_claims) j
WHERE e.raw_claims IS NOT NULL AND ISJSON(e.raw_claims) = 1
  AND NOT EXISTS (SELECT 1 FROM employee_claims c WHERE c.employee_id = e.id AND c.sumber = N'userinfo');

INSERT INTO employee_claim_history (employee_id, claims_hash, userinfo_claims)
SELECT e.id, NULL, e.raw_claims
FROM employees e
WHERE e.raw_claims IS NOT NULL AND ISJSON(e.raw_claims) = 1
  AND NOT EXISTS (SELECT 1 FROM employee_claim_history h WHERE h.employee_id = e.id);
