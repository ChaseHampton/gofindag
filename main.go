package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ChaseHampton/gofindag/internal/client"
	"github.com/ChaseHampton/gofindag/internal/config"
	"github.com/ChaseHampton/gofindag/internal/db"
	"github.com/ChaseHampton/gofindag/internal/processor"
	"github.com/ChaseHampton/gofindag/internal/search"
)

func main() {
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	db, err := db.NewDb(dbcfg)
	if err != nil {
		fmt.Printf("failed to connect to database: %v", err)
		return
	}
	searchPro := processor.NewProcessor(defaultClient, cfg.ProcessorConfig)
	err = searchPro.ProcessSearch(ctx, p, func(page *processor.SearchPage) error {
		fmt.Printf("=== Page %d ===\n", page.PageNumber)
		fmt.Printf("URL: %s\n", page.URL)
		fmt.Printf("Duration: %s\n", page.Duration)
		fmt.Printf("Total Records: %d\n", page.SearchResponse.Total)
		fmt.Printf("Records on page: %d\n", len(page.SearchResponse.Records))

		err = db.InsertMemorials(ctx, page.SearchResponse)
		if err != nil {
			fmt.Printf("failed to insert memorials: %v", err)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Failed: %s", fmt.Errorf("failed to process search: %v", err))
	}

	fmt.Println("Search completed successfully.")
}
