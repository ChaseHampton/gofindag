package processor

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ChaseHampton/gofindag/internal/client"
	"github.com/ChaseHampton/gofindag/internal/config"
	"github.com/ChaseHampton/gofindag/internal/db"
	"github.com/ChaseHampton/gofindag/internal/search"
	"github.com/jmoiron/sqlx"
)

type Processor struct {
	client         *client.Client
	MaxConcurrency int
	retryAttmpts   int
	retryDelay     time.Duration
	PageDelay      time.Duration
	maxPages       int
	baseURL        string
	httpConfig     *config.HTTPConfig
	config         *config.Config
}

type SearchPage struct {
	PageNumber     int
	URL            string
	SearchResponse search.SearchResponse
	Duration       time.Duration
}

type PageHandler func(page *SearchPage, db *db.DbWriter) error

func NewProcessor(client *client.Client, cfg config.ProcessorConfig, http *config.HTTPConfig, config *config.Config) *Processor {
	return &Processor{
		client:         client,
		MaxConcurrency: cfg.MaxConcurrency,
		retryAttmpts:   cfg.RetryAttempts,
		retryDelay:     cfg.RetryDelay,
		PageDelay:      cfg.PageDelay,
		maxPages:       cfg.MaxPages,
		baseURL:        cfg.BaseURL,
		httpConfig:     http,
		config:         config,
	}
}

func (p *Processor) CollectionStart(ctx context.Context, dbw *db.DbWriter, searchParams *search.SearchParams) error {
	params := *searchParams
	url := p.buildSearchURL(*searchParams)
	pageBatch := 100
	if url == "" {
		return fmt.Errorf("failed to build search URL")
	}
	collectParams := db.GetNewCollectionParams(searchParams.Limit, url)
	collectionId, err := dbw.StartCollection(ctx, collectParams)
	if err != nil {
		return fmt.Errorf("failed to start collection: %w", err)
	}

	response, err := p.makeRequestWithRetry(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to get search page %d: %w", 1, err)
	}

	var searchresp search.SearchResponse
	if err := json.Unmarshal(response.Body, &searchresp); err != nil {
		return fmt.Errorf("failed to unmarshal search response: %w", err)
	}

	fmt.Printf("Collection started with ID: %d\n", collectionId)
	// fmt.Printf("Got Response: %s\n", string(response.Body))

	totalRecords := searchresp.Total
	fmt.Printf("Search URL: %s\nTotal Records: %d\n", url, totalRecords)
	if totalRecords > 49000 {
		return fmt.Errorf("total records exceed maximum limit: %d", totalRecords)
	}

	batch := make([]db.PageDto, 0, pageBatch)
	inserted := 0
	for i := 0; i < totalRecords; i += searchParams.Limit {
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		default:
		}
		pageNumber := (i / searchParams.Limit) + 1
		params.Page = pageNumber
		params.Skip = i
		newUrl := p.buildSearchURL(params)
		page := &db.PageDto{
			CollectionId:  collectionId,
			PageNumber:    pageNumber,
			SearchUrl:     newUrl,
			Progress:      "pending",
			IsComplete:    false,
			RetryCount:    0,
			LastAttemptAt: sql.NullTime{Time: time.Now(), Valid: false},
		}
		batch = append(batch, *page)
		if len(batch) >= pageBatch {
			if err := dbw.InsertPage(ctx, batch); err != nil {
				return fmt.Errorf("failed to insert pages: %w", err)
			}
			inserted += len(batch)
			batch = make([]db.PageDto, 0, pageBatch)
		}
	}
	if len(batch) > 0 {
		inserted += len(batch)
		if err := dbw.InsertPage(ctx, batch); err != nil {
			return fmt.Errorf("failed to insert pages: %w", err)
		}
	}
	fmt.Printf("Inserted %d pages into the database.\n", inserted)

	return nil
}

func (p *Processor) buildSearchURL(params search.SearchParams) string {
	u, err := url.Parse(p.baseURL)
	if err != nil {
		return p.baseURL
	}
	q := u.Query()
	q.Set("ajax", "true")
	if params.DeathYear > 0 {
		q.Set("deathyear", strconv.Itoa(params.DeathYear))
	}
	q.Set("page", strconv.Itoa(params.Page))
	q.Set("limit", strconv.Itoa(params.Limit))
	q.Set("skip", strconv.Itoa(params.Skip))
	if params.LName != nil {
		q.Set("lastName", *params.LName)
	}
	if params.FName != nil {
		q.Set("firstName", *params.FName)
	}
	encoded := q.Encode()
	wilcard := strings.ReplaceAll(encoded, "%2A", "*")
	u.RawQuery = wilcard
	return u.String()
}

func (p *Processor) makeRequestWithRetry(ctx context.Context, url string) (*client.Response, error) {
	var lasterr error
	client := client.NewPageClient(p.httpConfig)
	for attempt := 0; attempt <= p.retryAttmpts; attempt++ {
		if attempt > 0 {
			fmt.Printf("Retrying request to %s (attempt %d)\n", url, attempt)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(p.retryDelay):
			}
		}
		response, err := client.Get(ctx, url)
		if err != nil {
			lasterr = err
			continue
		}

		if response.StatusCode >= 200 && response.StatusCode < 300 {
			return response, nil
		}
		lasterr = fmt.Errorf("HTTP %d: %s", response.StatusCode, string(response.Body))
	}

	return nil, fmt.Errorf("failed to get response after %d attempts: %w", p.retryAttmpts+1, lasterr)
}

func (pp *Processor) ProcessSingleSearch(ctx context.Context, page *db.Page, tvpName string, tx *sqlx.Tx) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("operation cancelled: %w", ctx.Err())
	default:
	}

	searchUrl := page.SearchUrl
	// pageStart := time.Now()

	response, err := pp.makeRequestWithRetry(ctx, searchUrl)
	if err != nil {
		fmt.Println(fmt.Errorf("failed to get search page for direct URL: %w", err))
		return err
	}

	var searchresp search.SearchResponse
	if err := json.Unmarshal(response.Body, &searchresp); err != nil {
		fmt.Println(fmt.Errorf("failed to unmarshal search response: %w\nResponse Body: %s", err, string(response.Body)))
		return err
	}

	filtered, err := pp.FilterSeenMemorials(ctx, &searchresp, tx)
	if err != nil {
		fmt.Println(fmt.Errorf("failed to filter seen memorials: %w", err))
		return err
	}
	if len(filtered.Records) > 0 {
		if err := db.InsertMemorials(ctx, searchresp, page.CollectionId, page.PageNumber, tvpName, tx); err != nil {
			fmt.Println(fmt.Errorf("failed to insert memorials: %w", err))
			return err
		}
	}
	// fmt.Printf("inserted %d memorials. Filtered %d records that were already seen", len(filtered.Records), len(searchresp.Records)-len(filtered.Records))

	var ids []int64
	for _, record := range filtered.Records {
		ids = append(ids, record.MemorialID)
	}
	err = db.RecordSeenMemorials(ctx, ids, tx, pp.config.DbConfig.MemorialIdTvpName)
	if err != nil {
		fmt.Println(fmt.Errorf("failed to record seen memorials: %w", err))
		return err
	}
	return nil
}

func (pp *Processor) FilterSeenMemorials(ctx context.Context, searchResp *search.SearchResponse, tx *sqlx.Tx) (*search.SearchResponse, error) {
	var checkIds []int64
	resp := searchResp
	for _, record := range resp.Records {
		checkIds = append(checkIds, record.MemorialID)
	}
	seen, err := db.GetSeenMemorials(ctx, checkIds, tx, pp.config.DbConfig.MemorialIdTvpName)
	if err != nil {
		return nil, fmt.Errorf("failed to get unseen memorial ids: %w", err)
	}
	var temp []search.Memorial
	seenmap := make(map[int64]bool)
	for _, id := range seen {
		seenmap[id] = true
	}
	for _, record := range resp.Records {
		if !seenmap[record.MemorialID] {
			temp = append(temp, record)
		}
	}
	resp.Records = temp
	return resp, nil
}
