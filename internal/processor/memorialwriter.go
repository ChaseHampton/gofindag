package processor

import (
	"context"
	"fmt"
	"time"

	"github.com/ChaseHampton/gofindag/internal/config"
	"github.com/ChaseHampton/gofindag/internal/db"
)

type MemorialWriter struct {
	dbWriter  *db.DbWriter
	cfg       *config.Config
	batchChan chan MemorialBatch
	done      chan struct{}
}

func NewMemorialWriter(dbWriter *db.DbWriter, cfg *config.Config) *MemorialWriter {
	return &MemorialWriter{
		dbWriter:  dbWriter,
		cfg:       cfg,
		batchChan: make(chan MemorialBatch, cfg.ProcessorConfig.ChannelSize),
		done:      make(chan struct{}),
	}
}

func (mw *MemorialWriter) Channel() chan<- MemorialBatch {
	return mw.batchChan
}

func (mw *MemorialWriter) Start(ctx context.Context) {
	go mw.processBatches(ctx)
}

func (mw *MemorialWriter) Stop(ctx context.Context) error {
	close(mw.batchChan) // Signal the processor to stop processing
	select {
	case <-mw.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (mw *MemorialWriter) processBatches(ctx context.Context) {
	defer close(mw.done)

	bbatch := make([]db.MemorialDto, 0, mw.cfg.ProcessorConfig.BatchSize)
	flushtimer := time.NewTimer(mw.cfg.ProcessorConfig.FlushTimeout)
	defer flushtimer.Stop()

	channelClosed := false

	for {
		var batch MemorialBatch
		var batchReceived bool

		if !channelClosed {
			select {
			case b, ok := <-mw.batchChan:
				if !ok {
					channelClosed = true
					// Continue processing to drain any remaining batches
				} else {
					batch = b
					batchReceived = true
				}
			case <-flushtimer.C:
				mw.flushBatch(ctx, &bbatch)
				flushtimer.Reset(mw.cfg.ProcessorConfig.FlushTimeout)
				continue
			case <-ctx.Done():
				mw.flushBatch(ctx, &bbatch)
				return
			}
		}

		if channelClosed && !batchReceived {
			select {
			case b, ok := <-mw.batchChan:
				if !ok {
					mw.flushBatch(ctx, &bbatch)
					return
				}
				batch = b
				batchReceived = true
			default:
				mw.flushBatch(ctx, &bbatch)
				return
			}
		}

		if batchReceived {
			result := mw.processBatch(ctx, batch)
			batch.ResultChan <- result

			dtos, err := mw.GetDtos(ctx, batch)
			if err != nil {
				fmt.Println(fmt.Errorf("failed to convert memorials to DTOs: %w", err))
				continue
			}

			bbatch = append(bbatch, dtos...)

			if len(bbatch) >= mw.cfg.ProcessorConfig.BatchSize {
				err := mw.dbWriter.InsertMemorialDtos(ctx, bbatch)
				if err != nil {
					fmt.Println(fmt.Errorf("failed to insert memorial DTOs: %w", err))
				}
				bbatch = bbatch[:0]
				if !channelClosed {
					flushtimer.Reset(mw.cfg.ProcessorConfig.FlushTimeout)
				}
			}
		}
	}
}

func (mw *MemorialWriter) processBatch(ctx context.Context, batch MemorialBatch) MemorialBatchResult {
	if len(batch.Memorials) == 0 {
		return MemorialBatchResult{
			Error: nil,
			Batch: &batch,
		}
	}

	seen := make([]int64, 0, len(batch.Memorials))
	for _, record := range batch.Memorials {
		seen = append(seen, record.MemorialID)
	}
	err := mw.UpdateSeenRecords(ctx, seen)
	return MemorialBatchResult{
		Error: err,
		Batch: &batch,
	}
}

func (mw *MemorialWriter) UpdateSeenRecords(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := mw.dbWriter.FreshTransaction(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	err = db.RecordSeenMemorials(ctx, ids, tx, mw.cfg.Tvp.MemorialIdTvpName)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (mw *MemorialWriter) GetDtos(ctx context.Context, batch MemorialBatch) ([]db.MemorialDto, error) {
	dtos, err := db.ConvertPageMemorials(batch.Memorials, batch.SearchURL, batch.CollectionId, batch.Page.PageNumber)
	if err != nil {
		return nil, err
	}
	return dtos, nil
}

func (mw *MemorialWriter) flushBatch(ctx context.Context, batch *[]db.MemorialDto) {
	if len(*batch) > 0 {
		fmt.Printf("Flushing memorial batch of size %d\n", len(*batch))
		err := mw.dbWriter.InsertMemorialDtos(ctx, *batch)
		if err != nil {
			fmt.Println(fmt.Errorf("failed to flush memorial DTOs: %w", err))
		}
		*batch = (*batch)[:0]
	}
}
