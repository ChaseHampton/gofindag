package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ChaseHampton/gofindag/internal/config"
	"github.com/ChaseHampton/gofindag/internal/search"

	"github.com/jmoiron/sqlx"
	mssql "github.com/microsoft/go-mssqldb"
)

type DbWriter struct {
	db  *sqlx.DB
	cfg *config.Config
}

func NewDb(cfg *config.DbConfig, appcfg *config.Config) (*DbWriter, error) {
	connStr := fmt.Sprintf("server=%s;port=%d;database=%s;user id=%s;password=%s;encrypt=true;trustservercertificate=true",
		cfg.Host, cfg.Port, cfg.DBName, cfg.User, cfg.Password)

	db, err := sqlx.Connect("sqlserver", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return &DbWriter{db: db, cfg: appcfg}, nil
}

func (d *DbWriter) InsertMemorials(ctx context.Context, mems []search.Memorial, url string, collectionId int, pagenumber int, tvpName string) error {
	if len(mems) == 0 {
		return nil
	}

	insert_data, err := ConvertPageMemorials(mems, url, collectionId, pagenumber)
	if err != nil {
		return fmt.Errorf("failed to convert memorials: %w", err)
	}
	tvp := mssql.TVP{
		TypeName: tvpName,
		Value:    insert_data,
	}
	_, err = d.db.ExecContext(ctx, "EXEC dbo.BulkInsertMemorials @Memorials = @Memorials", sql.Named("Memorials", tvp))
	if err != nil {
		return fmt.Errorf("failed to execute bulk insert: %w", err)
	}

	return nil
}

func (d *DbWriter) InsertMemorialDtos(ctx context.Context, mems []MemorialDto) error {
	if len(mems) == 0 {
		return nil
	}
	tx, err := d.FreshTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	tvp := mssql.TVP{
		TypeName: d.cfg.Tvp.MemorialTvpName,
		Value:    mems,
	}
	_, err = tx.ExecContext(ctx, "EXEC dbo.BulkInsertMemorials @Memorials = @Memorials", sql.Named("Memorials", tvp))
	if err != nil {
		return fmt.Errorf("failed to execute bulk insert: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (d *DbWriter) InsertPage(ctx context.Context, pages []PageDto) error {
	if len(pages) == 0 {
		return nil
	}
	tx, err := d.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	tvp := mssql.TVP{
		TypeName: d.cfg.Tvp.PageTvpName,
		Value:    pages,
	}
	_, err = tx.ExecContext(ctx, "EXEC dbo.BulkInsertPages @Pages = @Pages", sql.Named("Pages", tvp))
	if err != nil {
		return fmt.Errorf("failed to execute bulk insert: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil

}

func (d *DbWriter) StartCollection(ctx context.Context, input CollectionParamsDto) (int, error) {
	var result CollectionStartDto
	query := `EXEC dbo.sp_StartNewCollection @BatchSize = @p1, @SourceUrl = @p2, @StartedAt = @p3;`

	err := d.db.Get(&result, query, input.BatchSize, input.SourceUrl, sql.NullTime{Time: time.Now(), Valid: true})
	if err != nil {
		return 0, fmt.Errorf("failed to start collection: %w", err)
	}
	return result.CollectionId, nil
}

func GetPageBatch(ctx context.Context, tx *sqlx.Tx) ([]Page, error) {
	pages := make([]Page, 100)
	query := `SELECT TOP(100) * FROM dbo.Pages WHERE CollectionId in (SELECT CollectionId FROM dbo.Collections WHERE IsComplete = 0) AND IsComplete = 0 ORDER BY PageNumber;`
	err := tx.SelectContext(ctx, &pages, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get page batch: %w", err)
	}
	return pages, nil
}

func MarkPageCollected(ctx context.Context, pageid int, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "EXEC dbo.MarkPageCollected @PageId = @PageId", sql.Named("PageId", pageid))
	if err != nil {
		return fmt.Errorf("failed to mark page collected: %w", err)
	}
	return nil
}

func (d *DbWriter) FreshTransaction(ctx context.Context) (*sqlx.Tx, error) {
	tx, err := d.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}

func (d *DbWriter) GetReservedPageBatch(ctx context.Context) ([]Page, error) {
	var pages []Page
	err := d.db.SelectContext(ctx, &pages,
		"EXEC dbo.GetAndReservePageBatch @BatchSize = @BatchSize",
		sql.Named("BatchSize", 100))
	if err != nil {
		return nil, fmt.Errorf("failed to get page batch: %w", err)
	}
	return pages, nil
}

func (d *DbWriter) SetPageFailed(ctx context.Context, pageid int) error {
	tx, err := d.FreshTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, "EXEC dbo.MarkPageFailed @PageId = @PageId", sql.Named("PageId", pageid))
	if err != nil {
		return fmt.Errorf("failed to set page failed: %w", err)
	}
	return tx.Commit()
}

func (d *DbWriter) SetPageFailedNoTx(ctx context.Context, pageid int) error {
	_, err := d.db.ExecContext(ctx, "EXEC dbo.MarkPageFailed @PageId = @PageId", sql.Named("PageId", pageid))
	if err != nil {
		return fmt.Errorf("failed to set page failed: %w", err)
	}
	return nil
}

func GetAllSeenMemorialIds(ctx context.Context, checkIds []int64, tx *sqlx.Tx, memtvpname string) ([]int64, error) {
	if len(checkIds) == 0 {
		return nil, nil
	}
	var ids []int64

	query := "SELECT MemorialId FROM SeenMemorials WHERE MemorialId IN (?)"

	query, args, err := sqlx.In(query, checkIds)
	if err != nil {
		return nil, fmt.Errorf("failed to get unseen memorial ids: %w", err)
	}
	query = tx.Rebind(query)
	err = tx.SelectContext(ctx, &ids, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get unseen memorial ids: %w", err)
	}

	return ids, err
}

func (d *DbWriter) GetAllSeenMemorials(ctx context.Context) ([]int64, error) {
	var ids []int64
	query := "SELECT MemorialId FROM SeenMemorials"
	err := d.db.SelectContext(ctx, &ids, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all seen memorial ids: %w", err)
	}
	return ids, nil
}

func RecordSeenMemorials(ctx context.Context, memorialIds []int64, tx *sqlx.Tx, tvpname string) error {
	if len(memorialIds) == 0 {
		return nil
	}
	type MemorialRow struct {
		MemorialId int64 `json:"MemorialId"`
	}

	rows := make([]MemorialRow, len(memorialIds))
	for i, id := range memorialIds {
		rows[i] = MemorialRow{MemorialId: id}
	}

	memorialIdsTVP := mssql.TVP{
		TypeName: "dbo.MemorialIdList",
		Value:    rows,
	}
	_, err := tx.ExecContext(ctx,
		"EXEC dbo.sp_RecordSeenMemorialIds @MemorialIds = @MemorialIds",
		sql.Named("MemorialIds", memorialIdsTVP),
	)

	if err != nil {
		return fmt.Errorf("failed to record seen memorials: %w", err)
	}

	return nil
}

func (d *DbWriter) InsertOrUpdateDuplicate(ctx context.Context, dupe DuplicateEntry) error {
	_, err := d.db.ExecContext(ctx,
		"EXEC dbo.InsertOrUpdateDuplicate @MemorialId = @MemorialId, @CollectionId = @CollectionId, @PageNumber = @PageNumber, @Json = @Json",
		sql.Named("MemorialId", dupe.MemorialId),
		sql.Named("CollectionId", dupe.CollectionId),
		sql.Named("PageNumber", dupe.PageNumber),
		sql.Named("Json", dupe.Json),
	)
	return err
}

func (d *DbWriter) BatchInsertDuplicates(ctx context.Context, duplicates []DuplicateEntry) error {
	if len(duplicates) == 0 {
		return nil
	}

	jdata, err := json.Marshal(duplicates)
	if err != nil {
		return err
	}

	_, err = d.db.ExecContext(ctx,
		"EXEC dbo.BatchInsertDuplicates @DuplicateData = @DuplicateData",
		sql.Named("DuplicateData", string(jdata)))
	if err != nil {
		return err
	}

	return err
}
