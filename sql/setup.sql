-- Create database
IF NOT EXISTS (SELECT * FROM sys.databases WHERE name = '$(DB_NAME)')
BEGIN
    CREATE DATABASE $(DB_NAME);
END;
GO

USE $(DB_NAME);
GO

-- Create login and user
IF NOT EXISTS (SELECT * FROM sys.server_principals WHERE name = '$(APP_USER)')
BEGIN
    CREATE LOGIN $(APP_USER) WITH PASSWORD = '$(APP_PASSWORD)';
END;
GO

IF NOT EXISTS (SELECT * FROM sys.database_principals WHERE name = '$(APP_USER)')
BEGIN
    CREATE USER $(APP_USER) FOR LOGIN $(APP_USER);
    ALTER ROLE db_owner ADD MEMBER $(APP_USER);
END;
GO

USE $(DB_NAME);
GO

CREATE TABLE Collections (
    CollectionId UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
    BatchSize INT NOT NULL,
    IsComplete BIT DEFAULT 0,
    StartedAt DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),
    CompletedAt DATETIMEOFFSET NULL,
    TotalPages INT,
    SourceUrl NVARCHAR(MAX),
    CreatedAt DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),
    UpdatedAt DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET()
);
GO

CREATE TABLE Pages (
    PageId UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
    CollectionId UNIQUEIDENTIFIER NOT NULL,
    PageNumber INT NOT NULL,
    SearchUrl NVARCHAR(MAX) NOT NULL,
    Progress NVARCHAR(MAX),
    IsComplete BIT DEFAULT 0,
    RetryCount INT DEFAULT 0,
    LastAttemptAt DATETIMEOFFSET,
    CreatedAt DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),
    UpdatedAt DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),

    CONSTRAINT FK_Pages_Collections FOREIGN KEY (CollectionId)
        REFERENCES Collections (CollectionId)
        ON DELETE CASCADE
);
GO

CREATE TABLE Memorials (
    MemorialId INT IDENTITY(1,1) PRIMARY KEY,
    Url NVARCHAR(MAX) NOT NULL,
    Json NVARCHAR(MAX), -- Storing JSON as text
    Timestamp DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET()
);
GO

CREATE INDEX IX_Memorials_Timestamp ON Memorials (Timestamp);
GO

GRANT SELECT, INSERT, UPDATE, DELETE ON dbo.Collections TO [$(APP_USER)];
GRANT SELECT, INSERT, UPDATE, DELETE ON dbo.Pages TO [$(APP_USER)];
GRANT SELECT, INSERT, UPDATE, DELETE ON dbo.Memorials TO [$(APP_USER)];
GO

CREATE TYPE dbo.MemorialTableType AS TABLE
(
    Url NVARCHAR(MAX) NOT NULL,
    Json NVARCHAR(MAX) NULL,
    Timestamp DATETIMEOFFSET NULL
);
GO


CREATE TYPE dbo.PageTableType AS TABLE
(
    CollectionId UNIQUEIDENTIFIER NOT NULL,
    PageNumber INT NOT NULL,
    SearchUrl NVARCHAR(MAX) NOT NULL,
    Progress NVARCHAR(MAX) NULL,
    IsComplete BIT NULL,
    RetryCount INT NULL,
    LastAttemptAt DATETIMEOFFSET NULL
);
GO

CREATE PROCEDURE dbo.BulkInsertMemorials
    @Memorials dbo.MemorialTableType READONLY
AS
BEGIN
    SET NOCOUNT ON;
    
    INSERT INTO dbo.Memorials (
        Url,
        Json,
        Timestamp
    )
    SELECT 
        Url,
        Json,
        ISNULL(Timestamp, SYSDATETIMEOFFSET())
    FROM @Memorials;
    
    
    SELECT @@ROWCOUNT AS RowsInserted;
END;
GO

CREATE PROCEDURE dbo.BulkInsertPages
    @Pages dbo.PageTableType READONLY
AS
BEGIN
    SET NOCOUNT ON;
    
    INSERT INTO dbo.Pages (
        CollectionId,
        PageNumber,
        SearchUrl,
        Progress,
        IsComplete,
        RetryCount,
        LastAttemptAt,
        CreatedAt,
        UpdatedAt
    )
    SELECT 
        CollectionId,
        PageNumber,
        SearchUrl,
        Progress,
        ISNULL(IsComplete, 0),
        ISNULL(RetryCount, 0),
        LastAttemptAt,
        SYSDATETIMEOFFSET(),
        SYSDATETIMEOFFSET()
    FROM @Pages;
    
   
    SELECT @@ROWCOUNT AS RowsInserted;
END;
GO


GRANT EXECUTE ON dbo.BulkInsertMemorials TO [$(APP_USER)];
GRANT EXECUTE ON dbo.BulkInsertPages TO [$(APP_USER)];