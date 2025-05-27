package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ChaseHampton/gofindag/internal/client"
	"github.com/ChaseHampton/gofindag/internal/config"
	"github.com/ChaseHampton/gofindag/internal/db"
	"github.com/ChaseHampton/gofindag/internal/duplicates"
	"github.com/ChaseHampton/gofindag/internal/page"
	"github.com/ChaseHampton/gofindag/internal/processor"
	"github.com/ChaseHampton/gofindag/internal/search"
)

func main() {
	gen := os.Getenv("GEN")
	starttime := time.Now()

	dbcfg := config.NewDbConfig()

	cfg := config.NewConfig()
	p := &search.SearchParams{
		Ajax:      true,
		Limit:     cfg.ProcessorConfig.BatchSize,
		Page:      1,
		DeathYear: 2025,
		Skip:      0,
	}
	defaultClient := client.NewClient(&cfg.HTTPConfig)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbw, err := db.NewDb(dbcfg, cfg)
	if err != nil {
		fmt.Printf("failed to connect to database: %v", err)
		return
	}

	duper := duplicates.NewDuplicateProcessor(cfg, dbw)
	duper.Start(ctx)
	memwriter := processor.NewMemorialWriter(dbw, cfg)
	memwriter.Start(ctx)
	pageproc := processor.NewPageProcessor(dbw, cfg)
	pageproc.Start(ctx)
	memproc := processor.NewMemorialProcessor(ctx, dbw, memwriter, cfg, duper)
	searchPro := processor.NewProcessor(defaultClient, cfg.ProcessorConfig, &cfg.HTTPConfig, cfg, memproc)
	if gen != "" {
		for c := 'A'; c <= 'Z'; c++ {
			for c2 := 'A'; c2 <= 'Z'; c2++ {
				lname := string(c) + "*"
				fname := string(c2) + "*"
				temp := *p
				temp.LName = &lname
				temp.FName = &fname
				err = searchPro.CollectionStart(ctx, dbw, &temp)

				if err != nil {
					fmt.Printf("Failed: %s", fmt.Errorf("failed to start collection: %v", err))
				}
			}
		}
	}
	pager := page.NewPager(searchPro, dbw, pageproc, cfg)
	err = pager.WorkerPool(ctx)
	if err != nil {
		fmt.Printf("Failed: %s", fmt.Errorf("failed to complete collection: %v", err))
		return
	}
	duper.Stop(ctx)
	memwriter.Stop(ctx)
	fmt.Printf("Search completed successfully after %v\n", time.Since(starttime))
}
