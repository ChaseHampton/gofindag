package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ChaseHampton/gofindag/internal/client"
	"github.com/ChaseHampton/gofindag/internal/config"
	"github.com/ChaseHampton/gofindag/internal/db"
	"github.com/ChaseHampton/gofindag/internal/page"
	"github.com/ChaseHampton/gofindag/internal/processor"
	"github.com/ChaseHampton/gofindag/internal/search"
)

func main() {
	gen := os.Getenv("GEN")
	p := &search.SearchParams{
		Ajax:      true,
		Limit:     100,
		Page:      1,
		DeathYear: 2025,
		Skip:      0,
	}

	dbcfg := config.NewDbConfig()

	cfg := config.NewConfig()
	defaultClient := client.NewClient(&cfg.HTTPConfig)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := db.NewDb(dbcfg)
	if err != nil {
		fmt.Printf("failed to connect to database: %v", err)
		return
	}
	searchPro := processor.NewProcessor(defaultClient, cfg.ProcessorConfig, &cfg.HTTPConfig, cfg)
	if gen != "" {
		for c := 'A'; c <= 'Z'; c++ {
			lname := string(c) + "*"
			temp := *p
			temp.LName = &lname
			err = searchPro.CollectionStart(ctx, db, &temp)

			if err != nil {
				fmt.Printf("Failed: %s", fmt.Errorf("failed to start collection: %v", err))
			}
		}
	}
	pager := page.NewPager(searchPro, db)
	err = pager.ParallelPageBatch(ctx)
	if err != nil {
		fmt.Printf("Failed: %s", fmt.Errorf("failed to complete collection: %v", err))
		return
	}
	fmt.Println("Search completed successfully.")
}
