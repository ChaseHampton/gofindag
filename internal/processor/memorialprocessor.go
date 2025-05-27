// MemorialProcessor handles the memory and database operations for memorial filtering
package processor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ChaseHampton/gofindag/internal/config"
	"github.com/ChaseHampton/gofindag/internal/db"
	"github.com/ChaseHampton/gofindag/internal/duplicates"
)

type MemorialProcessor struct {
	memorialCache *db.MemorialCache
	writer        *MemorialWriter
	cfg           *config.Config
	dp            *duplicates.DuplicateProcessor
}

func NewMemorialProcessor(ctx context.Context, dbw *db.DbWriter, writer *MemorialWriter, cfg *config.Config, dproc *duplicates.DuplicateProcessor) *MemorialProcessor {
	cache := db.NewMemorialCache()
	ids, err := dbw.GetAllSeenMemorials(ctx)
	if err != nil {
		fmt.Println("Failed to load seen memorials cache from db")
	} else {
		cache.MarkSeen(ids)
		fmt.Printf("Loaded %d seen memorials into cache\n", len(ids))
	}
	return &MemorialProcessor{
		memorialCache: cache,
		writer:        writer,
		cfg:           cfg,
		dp:            dproc,
	}
}

func (mp *MemorialProcessor) ProcessMemorials(ctx context.Context, membatch MemorialBatch) error {
	if len(membatch.Memorials) == 0 {
		return nil
	}

	new, seen := mp.memorialCache.FilterMemorials(membatch.Memorials)
	dupechan := mp.dp.Channel()
	fmt.Printf("Adding %d new memorials and removing %d seen memorials.\n", len(new), len(seen))
	for _, record := range seen {
		j, err := json.Marshal(record)
		if err != nil {
			fmt.Println(fmt.Errorf("failed to marshal memorial record: %w", err))
			continue
		}
		dupechan <- db.DuplicateEntry{
			MemorialId:   record.MemorialID,
			CollectionId: membatch.CollectionId,
			PageNumber:   membatch.Page.PageNumber,
			Json:         string(j),
		}
	}

	if len(new) == 0 {
		fmt.Println("No new memorials to process, skipping insertion.")
	}

	membatch.Memorials = new
	writerChan := mp.writer.Channel()
	writerChan <- membatch
	return nil
}

func (mp *MemorialProcessor) UpdateSeenCache(ids []int64) {
	if len(ids) > 0 {
		mp.memorialCache.MarkSeen(ids)
	}
}
