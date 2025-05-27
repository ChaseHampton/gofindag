package duplicates

import (
	"context"
	"fmt"
	"time"

	"github.com/ChaseHampton/gofindag/internal/config"
	"github.com/ChaseHampton/gofindag/internal/db"
)

type DuplicateProcessor struct {
	cfg      *config.Config
	dbWriter *db.DbWriter
	entries  chan db.DuplicateEntry
	done     chan struct{}
}

func NewDuplicateProcessor(cfg *config.Config, dbWriter *db.DbWriter) *DuplicateProcessor {
	return &DuplicateProcessor{
		cfg:      cfg,
		dbWriter: dbWriter,
		entries:  make(chan db.DuplicateEntry, cfg.ProcessorConfig.ChannelSize),
		done:     make(chan struct{}),
	}
}

func (dp *DuplicateProcessor) Channel() chan<- db.DuplicateEntry {
	return dp.entries
}

func (dp *DuplicateProcessor) Start(ctx context.Context) {
	go dp.processDuplicates(ctx)
}

func (dp *DuplicateProcessor) Stop(ctx context.Context) error {
	close(dp.entries) // Signal the processor to stop processing
	select {
	case <-dp.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (dp *DuplicateProcessor) processDuplicates(ctx context.Context) {
	defer close(dp.done)

	batch := make([]db.DuplicateEntry, 0, dp.cfg.ProcessorConfig.BatchSize)
	flushtimer := time.NewTimer(dp.cfg.ProcessorConfig.FlushTimeout)
	batchContains := make(map[int64]bool)
	defer flushtimer.Stop()

	channelClosed := false

	for {
		var entry db.DuplicateEntry
		var entryReceived bool

		if !channelClosed {
			select {
			case e, ok := <-dp.entries:
				if !ok {
					channelClosed = true
				} else {
					entry = e
					entryReceived = true
				}
			case <-flushtimer.C:
				dp.flushAndReset(ctx, &batch, &batchContains)
				flushtimer.Reset(dp.cfg.ProcessorConfig.FlushTimeout)
				continue
			case <-ctx.Done():
				dp.flushBatch(ctx, batch)
				return
			}
		}

		if channelClosed && !entryReceived {
			select {
			case e, ok := <-dp.entries:
				if !ok {
					dp.flushBatch(ctx, batch)
					return
				}
				entry = e
				entryReceived = true
			default:
				dp.flushBatch(ctx, batch)
				return
			}
		}

		if entryReceived {
			batch = append(batch, entry)
			batchContains[entry.MemorialId] = true

			if len(batch) >= dp.cfg.ProcessorConfig.BatchSize {
				dp.flushAndReset(ctx, &batch, &batchContains)
				if !channelClosed {
					flushtimer.Reset(dp.cfg.ProcessorConfig.FlushTimeout)
				}
			}
		}
	}
}

func (dp *DuplicateProcessor) flushAndReset(ctx context.Context, batch *[]db.DuplicateEntry, batchContains *map[int64]bool) {
	if len(*batch) > 0 {
		dp.flushBatch(ctx, *batch)
		*batch = (*batch)[:0]
		*batchContains = make(map[int64]bool, dp.cfg.ProcessorConfig.BatchSize)
	}
}

func (dp *DuplicateProcessor) flushBatch(ctx context.Context, batch []db.DuplicateEntry) {
	if len(batch) == 0 {
		return
	}

	fmt.Printf("Flushing duplicate batch of size %d\n", len(batch))
	err := dp.dbWriter.BatchInsertDuplicates(ctx, batch)
	if err != nil {
		fmt.Println(fmt.Errorf("failed to insert duplicate batch: %w", err))
	}
}
