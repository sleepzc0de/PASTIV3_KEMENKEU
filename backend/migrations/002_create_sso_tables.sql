-- ============================================================
-- Migration 002: Tabel Employees, SSO States, & Alter Users
-- Aman dijalankan berulang kali (idempotent)
-- ============================================================

-- ================= Tabel Employees =================
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'employees')
BEGIN
    CREATE TABLE employees (
        id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
        sso_sub NVARCHAR(255) NOT NULL UNIQUE,
        nip NVARCHAR(30) NULL,
        nip9 NVARCHAR(20) NULL,
        nik NVARCHAR(30) NULL,
        name NVARCHAR(150) NULL,
        email NVARCHAR(150) NULL,
        preferred_username NVARCHAR(150) NULL,
        jabatan NVARCHAR(255) NULL,
        jenis_jabatan NVARCHAR(100) NULL,
        satker NVARCHAR(255) NULL,
        kode_satker NVARCHAR(50) NULL,
        organisasi NVARCHAR(255) NULL,
        kode_organisasi NVARCHAR(50) NULL,
        kode_kl NVARCHAR(50) NULL,
        nama_kl NVARCHAR(255) NULL,
        phone_number NVARCHAR(30) NULL,
        picture NVARCHAR(500) NULL,
        raw_claims NVARCHAR(MAX) NULL,
        created_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_employees_nip ON employees(nip);
    CREATE INDEX idx_employees_email ON employees(email);
END;

-- ================= PERBAIKAN: pastikan sso_sub bertipe NVARCHAR =================
-- Ini mengatasi kasus di mana tabel employees sudah pernah dibuat dengan
-- tipe kolom sso_sub yang salah (misalnya UNIQUEIDENTIFIER dari migrasi lama),
-- yang menyebabkan error: "Conversion failed when converting from a
-- character string to uniqueidentifier."
IF EXISTS (
    SELECT * FROM sys.columns c
    JOIN sys.types t ON c.user_type_id = t.user_type_id
    WHERE c.object_id = OBJECT_ID('employees')
      AND c.name = 'sso_sub'
      AND t.name != 'nvarchar'
)
BEGIN
    -- Drop unique constraint/index dulu kalau ada, supaya ALTER COLUMN tidak gagal
    DECLARE @constraintName NVARCHAR(255);
    SELECT @constraintName = i.name
    FROM sys.indexes i
    JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
    JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
    WHERE i.object_id = OBJECT_ID('employees') AND c.name = 'sso_sub' AND i.is_unique = 1;

    IF @constraintName IS NOT NULL
    BEGIN
        DECLARE @sql NVARCHAR(500) = 'DROP INDEX ' + @constraintName + ' ON employees';
        EXEC sp_executesql @sql;
    END;

    ALTER TABLE employees ALTER COLUMN sso_sub NVARCHAR(255) NOT NULL;

    IF NOT EXISTS (
        SELECT * FROM sys.indexes
        WHERE object_id = OBJECT_ID('employees') AND name = 'UQ_employees_sso_sub'
    )
    BEGIN
        CREATE UNIQUE INDEX UQ_employees_sso_sub ON employees(sso_sub);
    END;
END;

-- ================= Alter tabel users: employee_id =================
IF NOT EXISTS (
    SELECT * FROM sys.columns
    WHERE object_id = OBJECT_ID('users') AND name = 'employee_id'
)
BEGIN
    ALTER TABLE users ADD employee_id UNIQUEIDENTIFIER NULL;
END;

IF NOT EXISTS (SELECT * FROM sys.foreign_keys WHERE name = 'FK_users_employee')
BEGIN
    ALTER TABLE users ADD CONSTRAINT FK_users_employee FOREIGN KEY (employee_id) REFERENCES employees(id);
END;

-- ================= Alter tabel users: auth_provider =================
IF NOT EXISTS (
    SELECT * FROM sys.columns
    WHERE object_id = OBJECT_ID('users') AND name = 'auth_provider'
)
BEGIN
    ALTER TABLE users ADD auth_provider NVARCHAR(20) NOT NULL DEFAULT 'local';
END;

-- ================= Alter tabel users: is_protected =================
IF NOT EXISTS (
    SELECT * FROM sys.columns
    WHERE object_id = OBJECT_ID('users') AND name = 'is_protected'
)
BEGIN
    ALTER TABLE users ADD is_protected BIT NOT NULL DEFAULT 0;
END;

-- ================= Tabel sso_states =================
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'sso_states')
BEGIN
    CREATE TABLE sso_states (
        state NVARCHAR(255) PRIMARY KEY,
        code_verifier NVARCHAR(255) NOT NULL,
        expires_at DATETIME2 NOT NULL,
        created_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );
END;

-- ================= PERBAIKAN: pastikan state di sso_states bertipe NVARCHAR =================
IF EXISTS (
    SELECT * FROM sys.columns c
    JOIN sys.types t ON c.user_type_id = t.user_type_id
    WHERE c.object_id = OBJECT_ID('sso_states')
      AND c.name = 'state'
      AND t.name != 'nvarchar'
)
BEGIN
    ALTER TABLE sso_states ALTER COLUMN state NVARCHAR(255) NOT NULL;
END;

IF EXISTS (
    SELECT * FROM sys.columns c
    JOIN sys.types t ON c.user_type_id = t.user_type_id
    WHERE c.object_id = OBJECT_ID('sso_states')
      AND c.name = 'code_verifier'
      AND t.name != 'nvarchar'
)
BEGIN
    ALTER TABLE sso_states ALTER COLUMN code_verifier NVARCHAR(255) NOT NULL;
END;