package db

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/ChaseHampton/gofindag/internal/search"
)

type MemorialDto struct {
	URL       string       `db:"Url"`
	Json      string       `db:"Json"`
	Timestamp sql.NullTime `db:"Timestamp"`
}

func NewMemorialDto(url string, memorial search.Memorial) (*MemorialDto, error) {
	json, err := json.Marshal(memorial)
	if err != nil {
		return nil, err
	}
	return &MemorialDto{
		URL:       url,
		Json:      string(json),
		Timestamp: sql.NullTime{Time: time.Now(), Valid: true},
	}, nil
}

func ConvertPageMemorials(memorials []search.Memorial, url string) ([]MemorialDto, error) {
	dtos := make([]MemorialDto, 0, len(memorials))
	for _, memorial := range memorials {
		dto, err := NewMemorialDto(url, memorial)
		if err != nil {
			return nil, err
		}
		dtos = append(dtos, *dto)
	}
	return dtos, nil
}
