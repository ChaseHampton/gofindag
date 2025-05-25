package db

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/ChaseHampton/gofindag/internal/search"
)

type MemorialDto struct {
	CollectionId int          `db:"CollectionId"`
	PageNumber   int          `db:"PageNumber"`
	Json         string       `db:"Json"`
	Timestamp    sql.NullTime `db:"Timestamp"`
}

type PageDto struct {
	CollectionId  int          `db:"CollectionId"`
	PageNumber    int          `db:"PageNumber"`
	SearchUrl     string       `db:"SearchUrl"`
	Progress      string       `db:"Progress"`
	IsComplete    bool         `db:"IsComplete"`
	RetryCount    int          `db:"RetryCount"`
	LastAttemptAt sql.NullTime `db:"LastAttemptAt"`
}

type CollectionParamsDto struct {
	BatchSize int          `db:"BatchSize"`
	SourceUrl string       `db:"SourceUrl"`
	StartedAt sql.NullTime `db:"StartedAt"`
}

type CollectionStartDto struct {
	CollectionId int `db:"NewRecordID"`
}

type Page struct {
	PageId        int        `db:"PageId"`
	CollectionId  int        `db:"CollectionId"`
	PageNumber    int        `db:"PageNumber"`
	SearchUrl     string     `db:"SearchUrl"`
	Progress      string     `db:"Progress"`
	IsComplete    bool       `db:"IsComplete"`
	RetryCount    int        `db:"RetryCount"`
	LastAttemptAt *time.Time `db:"LastAttemptAt"`
	CreatedAt     time.Time  `db:"CreatedAt"`
	UpdatedAt     time.Time  `db:"UpdatedAt"`
}

func NewMemorialDto(url string, memorial search.Memorial, collectionId int, pagenumber int) (*MemorialDto, error) {
	json, err := json.Marshal(memorial)
	if err != nil {
		return nil, err
	}
	return &MemorialDto{
		CollectionId: collectionId,
		PageNumber:   pagenumber,
		Json:         string(json),
		Timestamp:    sql.NullTime{Time: time.Now(), Valid: true},
	}, nil
}

func ConvertPageMemorials(memorials []search.Memorial, url string, collectionId int, pagenumber int) ([]MemorialDto, error) {
	dtos := make([]MemorialDto, 0, len(memorials))
	for _, memorial := range memorials {
		dto, err := NewMemorialDto(url, memorial, collectionId, pagenumber)
		if err != nil {
			return nil, err
		}
		dtos = append(dtos, *dto)
	}
	return dtos, nil
}

func GetNewCollectionParams(batchSize int, sourceUrl string) CollectionParamsDto {
	return CollectionParamsDto{
		BatchSize: batchSize,
		SourceUrl: sourceUrl,
		StartedAt: sql.NullTime{Time: time.Now(), Valid: true},
	}
}
