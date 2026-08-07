-- ============================================================
-- Migration 001: Tabel Users & Refresh Tokens
-- Aman dijalankan berulang kali (idempotent)
-- ============================================================

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'users')
BEGIN
    CREATE TABLE users (
        id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
        username NVARCHAR(50) NOT NULL UNIQUE,
        email NVARCHAR(100) NOT NULL UNIQUE,
        password_hash NVARCHAR(255) NULL,
        password_salt NVARCHAR(255) NULL,
        full_name NVARCHAR(100) NOT NULL,
        role NVARCHAR(20) NOT NULL DEFAULT 'user',
        is_active BIT NOT NULL DEFAULT 1,
        failed_login_attempts INT NOT NULL DEFAULT 0,
        locked_until DATETIME2 NULL,
        last_login DATETIME2 NULL,
        created_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_users_username ON users(username);
    CREATE INDEX idx_users_email ON users(email);
END;

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'refresh_tokens')
BEGIN
    CREATE TABLE refresh_tokens (
        id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
        user_id UNIQUEIDENTIFIER NOT NULL FOREIGN KEY REFERENCES users(id) ON DELETE CASCADE,
        token_hash NVARCHAR(255) NOT NULL,
        expires_at DATETIME2 NOT NULL,
        revoked BIT NOT NULL DEFAULT 0,
        created_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );

    CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
END;

-- Jaga-jaga: kalau tabel users sudah ada dari migrasi versi lama
-- dengan password_hash/password_salt masih NOT NULL, longgarkan jadi nullable
-- (user SSO tidak punya password lokal)
IF EXISTS (
    SELECT * FROM sys.columns
    WHERE object_id = OBJECT_ID('users') AND name = 'password_hash' AND is_nullable = 0
)
BEGIN
    ALTER TABLE users ALTER COLUMN password_hash NVARCHAR(255) NULL;
END;

IF EXISTS (
    SELECT * FROM sys.columns
    WHERE object_id = OBJECT_ID('users') AND name = 'password_salt' AND is_nullable = 0
)
BEGIN
    ALTER TABLE users ALTER COLUMN password_salt NVARCHAR(255) NULL;
END;