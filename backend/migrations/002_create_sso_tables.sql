USE pasti_v3_db;
GO

-- ================= Tabel Employees (data lengkap pegawai dari SSO Kemenkeu) =================
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'employees')
BEGIN
    CREATE TABLE employees (
        id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
        sso_sub NVARCHAR(255) NOT NULL UNIQUE,       -- 'sub' claim, identifier unik dari SSO
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
        raw_claims NVARCHAR(MAX) NULL,               -- snapshot mentah JSON userinfo (audit trail)
        created_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_employees_nip ON employees(nip);
    CREATE INDEX idx_employees_email ON employees(email);
END
GO

-- ================= Alter tabel users: hubungkan ke employees + flag proteksi =================
IF NOT EXISTS (SELECT * FROM sys.columns WHERE object_id = OBJECT_ID('users') AND name = 'employee_id')
BEGIN
    ALTER TABLE users ADD employee_id UNIQUEIDENTIFIER NULL;
    ALTER TABLE users ADD CONSTRAINT FK_users_employee FOREIGN KEY (employee_id) REFERENCES employees(id);
END
GO

IF NOT EXISTS (SELECT * FROM sys.columns WHERE object_id = OBJECT_ID('users') AND name = 'auth_provider')
BEGIN
    ALTER TABLE users ADD auth_provider NVARCHAR(20) NOT NULL DEFAULT 'local'; -- 'local' | 'sso'
END
GO

IF NOT EXISTS (SELECT * FROM sys.columns WHERE object_id = OBJECT_ID('users') AND name = 'is_protected')
BEGIN
    ALTER TABLE users ADD is_protected BIT NOT NULL DEFAULT 0; -- superadmin permanen
END
GO

-- Password kini opsional untuk user SSO (tidak pernah login manual)
IF EXISTS (SELECT * FROM sys.columns WHERE object_id = OBJECT_ID('users') AND name = 'password_hash' AND is_nullable = 0)
BEGIN
    ALTER TABLE users ALTER COLUMN password_hash NVARCHAR(255) NULL;
    ALTER TABLE users ALTER COLUMN password_salt NVARCHAR(255) NULL;
END
GO

-- ================= Tabel sso_states (PKCE + CSRF state, sementara selama proses login) =================
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'sso_states')
BEGIN
    CREATE TABLE sso_states (
        state NVARCHAR(255) PRIMARY KEY,
        code_verifier NVARCHAR(255) NOT NULL,
        expires_at DATETIME2 NOT NULL,
        created_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );
END
GO