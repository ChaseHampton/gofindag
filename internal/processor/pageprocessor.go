package processor

import (
	"context"
	"fmt"
	"sync"

	"github.com/ChaseHampton/gofindag/internal/config"
	"github.com/ChaseHampton/gofindag/internal/db"
)

type PageProcessor struct {
	dbWriter   *db.DbWriter
	cfg        *config.Config
	updatechan chan PageUpdate
	updatewg   sync.WaitGroup
}

func NewPageProcessor(dbWriter *db.DbWriter, cfg *config.Config) *PageProcessor {
	return &PageProcessor{
		dbWriter:   dbWriter,
		cfg:        cfg,
		updatechan: make(chan PageUpdate, cfg.ProcessorConfig.ChannelSize),
	}
}

func (pp *PageProcessor) Channel() chan<- PageUpdate {
	return pp.updatechan
}

func (pp *PageProcessor) Start(ctx context.Context) {
	go pp.processUpdates(ctx)
}

func (pp *PageProcessor) WaitForCompletion(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		pp.updatewg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (pp *PageProcessor) processUpdates(ctx context.Context) {
	for {
		select {
		case update, ok := <-pp.updatechan:
			if !ok {
				return // Channel closed, exit the loop
			}
			pp.updatewg.Add(1)
			go func(up PageUpdate) {
				defer pp.updatewg.Done()
				pp.handlePageUpdate(ctx, update)
			}(update)
		case <-ctx.Done():
			return
		}
	}
}

func (pp *PageProcessor) handlePageUpdate(ctx context.Context, update PageUpdate) {
	select {
	case <-ctx.Done():
		fmt.Println("Context done, exiting page update handler")
		return
	default:
	}
	switch update.Status {
	case PageCompleted:
		tx, err := pp.dbWriter.FreshTransaction(ctx)
		if err != nil {
			fmt.Println(fmt.Errorf("failed to start page complete update transaction in page update handle: %w", err))
			return
		}
		defer tx.Rollback()
		err = db.MarkPageCollected(ctx, update.PageId, tx)
		if err != nil {
			fmt.Println(fmt.Errorf("failed to mark page as collected: %w", err))
			return
		}
		if err := tx.Commit(); err != nil {
			fmt.Println(fmt.Errorf("failed to commit page update transaction: %w", err))
			return
		}
	case PageFailed:
		err := pp.dbWriter.SetPageFailed(ctx, update.PageId)
		if err != nil {
			fmt.Println(fmt.Errorf("failed to set page as failed: %w", err))
			return
		}
	}
}
