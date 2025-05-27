package page

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/ChaseHampton/gofindag/internal/config"
	"github.com/ChaseHampton/gofindag/internal/db"
	"github.com/ChaseHampton/gofindag/internal/processor"
)

type Pager struct {
	proc  *processor.Processor
	db    *db.DbWriter
	pproc *processor.PageProcessor
	cfg   *config.Config
}

func NewPager(proc *processor.Processor, db *db.DbWriter, pproc *processor.PageProcessor, cfg *config.Config) *Pager {
	return &Pager{
		proc:  proc,
		db:    db,
		pproc: pproc,
		cfg:   cfg,
	}
}

func (p *Pager) WorkerPool(ctx context.Context) error {
	fmt.Println("Starting worker pool for page processing...")
	pagequeue := make(chan db.Page, 1000)
	errchan := make(chan error, 1)

	var wg sync.WaitGroup
	for range p.proc.MaxConcurrency {
		wg.Add(1)
		go p.pageConsumer(ctx, pagequeue, errchan, p.pproc.Channel(), &wg)
	}

	go p.pageProducer(ctx, pagequeue, errchan)

	return p.WaitForCompletion(ctx, &wg, errchan)
}

func (p *Pager) WaitForCompletion(ctx context.Context, wg *sync.WaitGroup, errchan <-chan error) error {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(p.pproc.Channel())
		p.pproc.WaitForCompletion(ctx)
		close(done)
	}()

	select {
	case err := <-errchan:
		return err
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Pager) pageProducer(ctx context.Context, pagequeue chan<- db.Page, errchan chan<- error) {
	defer close(pagequeue)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		pagebatch, err := p.db.GetReservedPageBatch(ctx)
		if err != nil {
			select {
			case errchan <- err:
			case <-ctx.Done():
			}
			return
		}

		if len(pagebatch) == 0 {
			fmt.Println("No more pages to process, ending producer.")
			return
		}
		for _, page := range pagebatch {
			select {
			case pagequeue <- page:
			case <-ctx.Done():
				return
			}
		}
		fmt.Printf("Produced %d pages for processing\n", len(pagebatch))
	}
}

func (p *Pager) pageConsumer(ctx context.Context, pagequeue <-chan db.Page, errchan chan<- error, pageup chan<- processor.PageUpdate, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case page, ok := <-pagequeue:
			if !ok {
				return
			}
			if err := jitteredPause(ctx, p.cfg.ProcessorConfig.PageDelay, 0.3); err != nil {
				select {
				case errchan <- fmt.Errorf("jittered pause error: %w", err):
				case <-ctx.Done():
				}
				return
			}
			err := p.processPage(ctx, page)

			var updatePage processor.PageUpdate
			if err != nil {
				fmt.Printf("Error processing page %d: %v\n", page.PageNumber, err)
				updatePage = processor.GetPageUpdate(&page, 1, err)
			} else {
				// fmt.Printf("Successfully processed page %d\n", page.PageNumber)
				updatePage = processor.GetPageUpdate(&page, 0, nil)
			}

			pageup <- updatePage
		case <-ctx.Done():
			return
		}
	}
}

func (p *Pager) processPage(ctx context.Context, page db.Page) error {
	return p.proc.ProcessSingleSearch(ctx, &page)
}

func jitteredPause(ctx context.Context, baseDelay time.Duration, jitterPercent float64) error {
	jitterrange := time.Duration(float64(baseDelay) * jitterPercent)

	jitter := time.Duration(rand.Int63n(int64(2*jitterrange))) - jitterrange

	delay := baseDelay + jitter
	if delay < 0 {
		delay = baseDelay / 2
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
