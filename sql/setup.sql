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
    MemorialId BIGINT PRIMARY KEY,
    CollectionId INT NOT NULL,
    PageNumber INT NOT NULL,
    Json NVARCHAR(MAX), -- Storing JSON as text
    Timestamp DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET()

    CONSTRAINT FK_Memorials_Collections FOREIGN KEY (CollectionId)
        REFERENCES Collections (CollectionId)
        ON DELETE CASCADE
);
GO

ALTER TABLE dbo.Memorials 
ADD CONSTRAINT PK_Memorials PRIMARY KEY CLUSTERED (MemorialId);
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
    MemorialId BIGINT NOT NULL PRIMARY KEY,
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

WITH OrderedSource AS (
    SELECT
        MemorialId,
        CollectionId,
        PageNumber,
        Json,
        ISNULL(Timestamp, SYSDATETIMEOFFSET()) AS Timestamp,
        ROW_NUMBER() OVER (ORDER BY MemorialId) as rn
    FROM @Memorials
)
MERGE dbo.Memorials AS target
USING OrderedSource AS source ON target.MemorialId = source.MemorialId
WHEN NOT MATCHED THEN
    INSERT (MemorialId, CollectionId, PageNumber, Json, Timestamp)
    VALUES (source.MemorialId, source.CollectionId, source.PageNumber,
            source.Json, source.Timestamp)
WHEN MATCHED THEN
    UPDATE SET
        CollectionId = source.CollectionId,
        PageNumber = source.PageNumber,
        Json = source.Json,
        Timestamp = source.Timestamp;

SELECT @@ROWCOUNT AS RowsAffected;
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
    LEFT JOIN Memorials s ON m.MemorialId = s.MemorialId
    WHERE s.MemorialId IS NULL;
END
GO

CREATE PROCEDURE sp_RecordSeenMemorialIds
    @MemorialIds dbo.MemorialIdList READONLY
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Only insert new records, ignore duplicates
    MERGE SeenMemorials AS target
    USING @MemorialIds AS source
    ON target.MemorialId = source.MemorialId
    WHEN NOT MATCHED THEN
        INSERT (MemorialId) VALUES (source.MemorialId);
    
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

--- ==========================================
--- Dupe Tracking in separate script
--- ==========================================

-- Duplicate tracking table for debugging
CREATE TABLE MemorialDuplicates (
    DupeId BIGINT IDENTITY(1,1) PRIMARY KEY,
    MemorialId BIGINT NOT NULL,
    CollectionId INT NOT NULL,
    PageNumber INT NOT NULL,
    Json NVARCHAR(MAX),
    FirstSeenAt DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),
    LastSeenAt DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),
    OccurrenceCount INT DEFAULT 1,
    -- Optional: store a hash of the JSON for faster duplicate detection
    JsonHash AS CAST(HASHBYTES('SHA2_256', Json) AS VARBINARY(32)) PERSISTED,
    
    -- Index for efficient duplicate detection
    INDEX IX_MemorialDuplicates_Hash (JsonHash),
    INDEX IX_MemorialDuplicates_Memorial (MemorialId),
    INDEX IX_MemorialDuplicates_Collection (CollectionId),
    INDEX IX_MemorialDuplicates_FirstSeen (FirstSeenAt)
);
GO

-- Stored procedure for inserting/updating duplicates efficiently
CREATE OR ALTER PROCEDURE InsertOrUpdateDuplicate
    @MemorialId BIGINT,
    @CollectionId INT,
    @PageNumber INT,
    @Json NVARCHAR(MAX)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @JsonHashValue VARBINARY(32) = HASHBYTES('SHA2_256', @Json);
    
    -- Try to update existing duplicate entry
    UPDATE MemorialDuplicates 
    SET 
        LastSeenAt = SYSDATETIMEOFFSET(),
        OccurrenceCount = OccurrenceCount + 1
    WHERE JsonHash = @JsonHashValue 
      AND MemorialId = @MemorialId
      AND CollectionId = @CollectionId;
    
    -- If no existing entry found, insert new one
    IF @@ROWCOUNT = 0
    BEGIN
        INSERT INTO MemorialDuplicates (MemorialId, CollectionId, PageNumber, Json)
        VALUES (@MemorialId, @CollectionId, @PageNumber, @Json);
    END
END;
GO

-- Batch insert procedure for high-volume scenarios
CREATE OR ALTER PROCEDURE BatchInsertDuplicates
    @DuplicateData NVARCHAR(MAX) -- JSON array of duplicate entries
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Create temp table for batch processing
    CREATE TABLE #TempDuplicates (
        MemorialId BIGINT,
        CollectionId INT,
        PageNumber INT,
        Json NVARCHAR(MAX)
    );
    
    -- Parse JSON array into temp table
    INSERT INTO #TempDuplicates (MemorialId, CollectionId, PageNumber, Json)
    SELECT 
        MemorialId,
        CollectionId,
        PageNumber,
        Json
    FROM OPENJSON(@DuplicateData)
    WITH (
        MemorialId BIGINT,
        CollectionId INT,  
        PageNumber INT,
        Json NVARCHAR(MAX)
    );
    
    -- Merge into main duplicate table
    MERGE MemorialDuplicates AS target
    USING (
        SELECT 
            MemorialId,
            CollectionId,
            PageNumber,
            Json,
            HASHBYTES('SHA2_256', Json) AS JsonHash
        FROM #TempDuplicates
    ) AS source ON target.JsonHash = source.JsonHash
                 AND target.MemorialId = source.MemorialId
                 AND target.CollectionId = source.CollectionId
    WHEN MATCHED THEN
        UPDATE SET 
            LastSeenAt = SYSDATETIMEOFFSET(),
            OccurrenceCount = OccurrenceCount + 1
    WHEN NOT MATCHED THEN
        INSERT (MemorialId, CollectionId, PageNumber, Json)
        VALUES (source.MemorialId, source.CollectionId, source.PageNumber, source.Json);
    
    DROP TABLE #TempDuplicates;
END;
GO

-- Query to analyze duplicate patterns
CREATE OR ALTER VIEW DuplicateAnalysis AS
SELECT 
    CollectionId,
    COUNT(*) as UniqueMemorials,
    SUM(OccurrenceCount) as TotalDuplicateInstances,
    AVG(CAST(OccurrenceCount AS FLOAT)) as AvgDuplicatesPerMemorial,
    MAX(OccurrenceCount) as MaxDuplicateCount,
    MIN(FirstSeenAt) as EarliestDuplicate,
    MAX(LastSeenAt) as LatestDuplicate
FROM MemorialDuplicates
GROUP BY CollectionId;
GO