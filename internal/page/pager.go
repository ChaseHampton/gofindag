package page

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ChaseHampton/gofindag/internal/db"
	"github.com/ChaseHampton/gofindag/internal/processor"
)

type Pager struct {
	proc *processor.Processor
	db   *db.DbWriter
}

func NewPager(proc *processor.Processor, db *db.DbWriter) *Pager {
	return &Pager{
		proc: proc,
		db:   db,
	}
}

func (p *Pager) ParallelPageBatch(ctx context.Context) error {
	i := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if i > 2 {
			break
		}
		log.Printf("Processing page index: %d", i)
		i++
		pageBatch, err := p.db.GetReservedPageBatch(ctx)
		if err != nil {
			return err
		}
		if len(pageBatch) == 0 {
			log.Println("No more pages to process.")
			break
		}
		err = p.processPageBatch(ctx, pageBatch)
		if err != nil {
			log.Printf("Error processing page batch: %v", err)
			return err
		}
		log.Printf("Processed %d pages", len(pageBatch))
		time.Sleep(15 * time.Second)
	}
	return nil
}

func (p *Pager) processPageBatch(ctx context.Context, pageBatch []db.Page) error {
	maxworkers := p.proc.MaxConcurrency
	semaphore := make(chan struct{}, maxworkers)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var pageerrs []error

	for _, page := range pageBatch {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		wg.Add(1)
		semaphore <- struct{}{}

		go func(page db.Page) {
			defer wg.Done()
			defer func() { <-semaphore }()

			committed := false
			tx, err := p.db.FreshTransaction(ctx)
			if err != nil {
				log.Println("Error starting transaction:", err)

				p.db.SetPageFailedNoTx(ctx, page.PageId)
				mu.Lock()
				pageerrs = append(pageerrs, fmt.Errorf("failed to start transaction for page %d: %w", page.PageNumber, err))
				mu.Unlock()
				return
			}
			defer func() {
				if !committed {
					tx.Rollback()
				}
			}()

			err = p.proc.ProcessSingleSearch(ctx, &page, p.db.MemorialTvpName, tx)
			if err != nil {
				log.Printf("Error processing page %d: %v", page.PageNumber, err)
				_ = db.SetPageFailed(ctx, page.PageId, tx)
				if cErr := tx.Commit(); cErr != nil {
					mu.Lock()
					pageerrs = append(pageerrs, fmt.Errorf("failed to commit failure transaction for page %d: %w", page.PageNumber, cErr))
					mu.Unlock()
					return
				}
				committed = true
				mu.Lock()
				pageerrs = append(pageerrs, fmt.Errorf("failed to process page %d: %w", page.PageNumber, err))
				mu.Unlock()
				return
			}

			err = db.MarkPageCollected(ctx, page.PageId, tx)
			if err != nil {
				log.Printf("Error marking page %d as collected: %v", page.PageNumber, err)
				_ = db.SetPageFailed(ctx, page.PageId, tx)
				if cErr := tx.Commit(); cErr != nil {
					mu.Lock()
					pageerrs = append(pageerrs, fmt.Errorf("failed to commit failure transaction for page %d: %w", page.PageNumber, cErr))
					mu.Unlock()
					return
				}
				committed = true
				mu.Lock()
				pageerrs = append(pageerrs, fmt.Errorf("failed to mark page %d as collected: %w", page.PageNumber, err))
				mu.Unlock()
				return
			}

			err = tx.Commit()
			if err != nil {
				log.Printf("Error committing transaction for page %d: %v", page.PageNumber, err)
				_ = db.SetPageFailed(ctx, page.PageId, tx)
				if cErr := tx.Commit(); cErr != nil {
					mu.Lock()
					pageerrs = append(pageerrs, fmt.Errorf("failed to commit failure transaction for page %d: %w", page.PageNumber, cErr))
					mu.Unlock()
					return
				}
				committed = true
				mu.Lock()
				pageerrs = append(pageerrs, fmt.Errorf("failed to commit transaction for page %d: %w", page.PageNumber, err))
				mu.Unlock()
				return
			}
			committed = true
		}(page)
	}
	wg.Wait()
	if len(pageerrs) > 0 {
		return errors.Join(pageerrs...)
	}
	return nil
}
