IF EXISTS (
    SELECT * FROM sys.columns
    WHERE object_id = OBJECT_ID('inaproc_history_kaji_ulang') AND name = 'datamart_id'
)
BEGIN
    -- Drop primary key constraint dulu sebelum ALTER COLUMN
    DECLARE @pkName NVARCHAR(255);
    SELECT @pkName = name FROM sys.key_constraints
    WHERE parent_object_id = OBJECT_ID('inaproc_history_kaji_ulang') AND type = 'PK';

    IF @pkName IS NOT NULL
    BEGIN
        DECLARE @sql NVARCHAR(500) = 'ALTER TABLE inaproc_history_kaji_ulang DROP CONSTRAINT ' + QUOTENAME(@pkName);
        EXEC sp_executesql @sql;
    END;

    ALTER TABLE inaproc_history_kaji_ulang ALTER COLUMN datamart_id NVARCHAR(255) NOT NULL;

    ALTER TABLE inaproc_history_kaji_ulang ADD CONSTRAINT PK_inaproc_history_kaji_ulang PRIMARY KEY (datamart_id);
END;