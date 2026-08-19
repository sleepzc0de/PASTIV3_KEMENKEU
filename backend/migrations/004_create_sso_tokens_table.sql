IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'sso_tokens')
BEGIN
    CREATE TABLE sso_tokens (
        id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
        user_id UNIQUEIDENTIFIER NOT NULL UNIQUE,
        access_token_enc NVARCHAR(MAX) NOT NULL,
        refresh_token_enc NVARCHAR(MAX) NULL,
        expires_at DATETIME2 NOT NULL,
        created_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        CONSTRAINT FK_sso_tokens_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
    );
END;