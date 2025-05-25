IF NOT EXISTS (SELECT * FROM sys.databases WHERE name = '$(DB_NAME)')
BEGIN
    CREATE DATABASE $(DB_NAME);
END;
GO

USE $(DB_NAME);
GO

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
    CollectionId int IDENTITY(1,1) PRIMARY KEY,
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
    PageId INT IDENTITY(1,1) PRIMARY KEY,
    CollectionId int NOT NULL,
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
    CollectionId INT NOT NULL,
    PageNumber INT NOT NULL,
    Json NVARCHAR(MAX), -- Storing JSON as text
    Timestamp DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET()

    CONSTRAINT FK_Memorials_Collections FOREIGN KEY (CollectionId)
        REFERENCES Collections (CollectionId)
        ON DELETE CASCADE
);
GO

CREATE TABLE SeenMemorials (
    MemorialId BIGINT PRIMARY KEY,
    FirstSeen DATETIME2 NOT NULL DEFAULT GETDATE()
);
GO

CREATE INDEX IX_SeenMemorials_FirstSeen ON SeenMemorials (FirstSeen);
GO

CREATE INDEX IX_Memorials_Timestamp ON Memorials (Timestamp);
GO

CREATE INDEX IX_Pages_Reservation 
ON Pages (IsComplete, Progress, LastAttemptAt) 
INCLUDE (PageId, CollectionId, PageNumber, SearchUrl);
GO

GRANT SELECT, INSERT, UPDATE, DELETE ON dbo.Collections TO [$(APP_USER)];
GRANT SELECT, INSERT, UPDATE, DELETE ON dbo.Pages TO [$(APP_USER)];
GRANT SELECT, INSERT, UPDATE, DELETE ON dbo.Memorials TO [$(APP_USER)];
GO

CREATE TYPE dbo.MemorialIdList AS TABLE (
    MemorialId BIGINT NOT NULL PRIMARY KEY
);
GO

CREATE TYPE dbo.MemorialTableType AS TABLE
(
    CollectionId INT NOT NULL,
    PageNumber INT NOT NULL,
    Json NVARCHAR(MAX) NULL,
    Timestamp DATETIMEOFFSET NULL
);
GO


CREATE TYPE dbo.PageTableType AS TABLE
(
    CollectionId int NOT NULL,
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
        CollectionId,
        PageNumber,
        Json,
        Timestamp
    )
    SELECT 
        CollectionId,
        PageNumber,
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

CREATE PROCEDURE sp_StartNewCollection
@BatchSize int = 100,
@SourceUrl nvarchar(500),
@StartedAt datetimeoffset = null
AS
BEGIN
    SET NOCOUNT ON;

    if @StartedAt is null
        SET @StartedAt = SYSDATETIMEOFFSET()

    DECLARE @NewID int;

    -- Insert the new record (CollectionId will be auto-generated)
    INSERT INTO Collections (
        BatchSize,
        IsComplete,
        StartedAt,
        CompletedAt,
        TotalPages,
        SourceUrl,
        CreatedAt,
        UpdatedAt
    )
    VALUES (
        @BatchSize,
        0,
        @StartedAt,
        null,
        0,
        @SourceUrl,
        SYSDATETIMEOFFSET(),
        SYSDATETIMEOFFSET()
    );

    -- Get the identity value of the newly inserted record
    SET @NewID = SCOPE_IDENTITY();

    -- Return the ID of the newly inserted record
    SELECT @NewID AS NewRecordID;

END;
GO

CREATE PROCEDURE dbo.MarkPageCollected
    @PageID INT
AS
BEGIN
    SET NOCOUNT ON;
    
    BEGIN TRY
        UPDATE Pages 
        SET 
            IsComplete = 1,
            Progress = 'completed',
            UpdatedAt = SYSDATETIMEOFFSET(),
            LastAttemptAt = SYSDATETIMEOFFSET()
        WHERE PageId = @PageID;
        
        -- Check if the record was actually updated
        IF @@ROWCOUNT = 0
        BEGIN
            RAISERROR('Page with ID %d not found', 16, 1, @PageID);
            RETURN;
        END
        
    END TRY
    BEGIN CATCH
        -- Re-raise the error
        THROW;
    END CATCH
END
GO

CREATE PROCEDURE dbo.GetAndReservePageBatch
    @BatchSize INT = 100
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE TOP(@BatchSize) Pages
    SET 
        Progress = N'processing',
        UpdatedAt = SYSDATETIMEOFFSET(),
        LastAttemptAt = SYSDATETIMEOFFSET(),
        RetryCount = ISNULL(RetryCount, 0) + 1
    OUTPUT 
        INSERTED.PageId,
        INSERTED.CollectionId,
        INSERTED.PageNumber,
        INSERTED.SearchUrl,
        INSERTED.Progress,
        INSERTED.IsComplete,
        INSERTED.RetryCount,
        INSERTED.LastAttemptAt,
        INSERTED.CreatedAt,
        INSERTED.UpdatedAt
    WHERE 
        IsComplete = 0 
        AND (Progress IS NULL OR Progress = 'pending' OR Progress = 'failed')
END
GO

CREATE PROCEDURE dbo.MarkPageFailed
    @PageID INT
AS
BEGIN
    SET NOCOUNT ON;

    UPDATE Pages WITH (ROWLOCK) 
    SET  Progress      = N'failed',
         UpdatedAt     = SYSDATETIMEOFFSET(),
         LastAttemptAt = SYSDATETIMEOFFSET()
    WHERE PageId    = @PageID
      AND IsComplete = 0;

    IF @@ROWCOUNT = 0
        RAISERROR (N'Page %d not found or already complete', 16, 1, @PageID);
END
GO

CREATE PROCEDURE sp_GetUnseenMemorialIds
    @MemorialIds dbo.MemorialIdList READONLY
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT m.MemorialId
    FROM @MemorialIds m
    LEFT JOIN SeenMemorials s ON m.MemorialId = s.MemorialId
    WHERE s.MemorialId IS NULL;
END
GO

CREATE PROCEDURE sp_RecordSeenMemorialIds
    @MemorialIds dbo.MemorialIdList READONLY
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Only insert new records, ignore duplicates
    MERGE seen_memorials AS target
    USING @MemorialIds AS source
    ON target.memorial_id = source.memorial_id
    WHEN NOT MATCHED THEN
        INSERT (memorial_id) VALUES (source.memorial_id);
    
    SELECT @@ROWCOUNT as NewRecordsInserted;
END
GO

GRANT EXECUTE ON dbo.MarkPageFailed TO [$(APP_USER)];
GRANT EXECUTE ON dbo.MarkPageCollected TO [$(APP_USER)];
GRANT EXECUTE ON dbo.BulkInsertMemorials TO [$(APP_USER)];
GRANT EXECUTE ON dbo.BulkInsertPages TO [$(APP_USER)];
GRANT EXECUTE ON sp_StartNewCollection TO [$(APP_USER)];
GRANT EXECUTE ON sp_GetUnseenMemorialIds TO [$(APP_USER)];
GRANT EXECUTE ON sp_RecordSeenMemorialIds TO [$(APP_USER)];
GRANT EXECUTE ON dbo.GetAndReservePageBatch TO [$(APP_USER)];