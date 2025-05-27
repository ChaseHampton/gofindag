package processor

import (
	"github.com/ChaseHampton/gofindag/internal/db"
	"github.com/ChaseHampton/gofindag/internal/search"
)

type MemorialBatch struct {
	Memorials    []search.Memorial
	SearchURL    string
	CollectionId int
	Page         db.Page
	ResultChan   chan<- MemorialBatchResult
}

type MemorialBatchResult struct {
	Error error
	Batch *MemorialBatch
}

type PageUpdate struct {
	PageId int
	Status PageStatus
	Error  error
}

type PageStatus int

const (
	PageCompleted PageStatus = iota
	PageFailed
)

func GetPageUpdate(page *db.Page, status PageStatus, err error) PageUpdate {
	return PageUpdate{
		PageId: page.PageId,
		Status: status,
		Error:  err,
	}
}
