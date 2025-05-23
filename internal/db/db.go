package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ChaseHampton/gofindag/internal/config"
	"github.com/ChaseHampton/gofindag/internal/search"

	"github.com/jmoiron/sqlx"
	mssql "github.com/microsoft/go-mssqldb"
)

type DbWriter struct {
	db              *sqlx.DB
	memorialTvpName string
}

func NewDb(cfg *config.DbConfig) (*DbWriter, error) {
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
	return &DbWriter{db: db, memorialTvpName: cfg.MemorialTvpName}, nil
}

func (d *DbWriter) InsertMemorials(ctx context.Context, p search.SearchResponse) error {
	if len(p.Records) == 0 {
		return nil
	}
	tx, err := d.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	insert_data, err := ConvertPageMemorials(p.Records, p.SearchURL)
	if err != nil {
		return fmt.Errorf("failed to convert memorials: %w", err)
	}
	tvp := mssql.TVP{
		TypeName: d.memorialTvpName,
		Value:    insert_data,
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
