package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/ChaseHampton/gofindag/internal/client"
	"github.com/ChaseHampton/gofindag/internal/config"
	"github.com/ChaseHampton/gofindag/internal/search"
)

type Processor struct {
	client         *client.Client
	maxConcurrency int
	retryAttmpts   int
	retryDelay     time.Duration
	pageDelay      time.Duration
	maxPages       int
	baseURL        string
}

type SearchPage struct {
	PageNumber     int
	URL            string
	SearchResponse search.SearchResponse
	Duration       time.Duration
}

type PageHandler func(page *SearchPage) error

func NewProcessor(client *client.Client, cfg config.ProcessorConfig) *Processor {
	return &Processor{
		client:         client,
		maxConcurrency: cfg.MaxConcurrency,
		retryAttmpts:   cfg.RetryAttempts,
		retryDelay:     cfg.RetryDelay,
		pageDelay:      cfg.PageDelay,
		maxPages:       cfg.MaxPages,
		baseURL:        cfg.BaseURL,
	}
}

func (p *Processor) ProcessSearch(ctx context.Context, searchParams *search.SearchParams, handler PageHandler) error {
	currentParams := *searchParams
	pageNumber := max(currentParams.Page, 1)
	if currentParams.Limit < 20 {
		currentParams.Limit = 20
	}
	for pageNumber <= p.maxPages {
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		default:
		}
		searchUrl := p.buildSearchURL(currentParams)

		pageStart := time.Now()
		response, err := p.makeRequestWithRetry(ctx, searchUrl)
		if err != nil {
			return fmt.Errorf("failed to get search page %d: %w", pageNumber, err)
		}

		var searchresp search.SearchResponse
		if err := json.Unmarshal(response.Body, &searchresp); err != nil {
			return fmt.Errorf("failed to unmarshal search response: %w", err)
		}

		page := &SearchPage{
			PageNumber:     pageNumber,
			URL:            searchUrl,
			SearchResponse: searchresp,
			Duration:       time.Since(pageStart),
		}

		if err := handler(page); err != nil {
			return fmt.Errorf("failed to handle search page %d: %w", pageNumber, err)
		}

		recordCount := len(searchresp.Records)

		if recordCount == 0 || recordCount < currentParams.Limit {
			fmt.Printf("No more records found on page %d\n", pageNumber)
			break
		}
		if !searchresp.NextURL {
			fmt.Printf("Reached the end of search results at page %d\n", pageNumber)
			break
		}

		currentParams.Page++
		currentParams.Skip += currentParams.Limit
		pageNumber++

		if pageNumber <= p.maxPages {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(p.pageDelay):
			}
		}
	}
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

	u.RawQuery = q.Encode()
	return u.String()
}

func (p *Processor) makeRequestWithRetry(ctx context.Context, url string) (*client.Response, error) {
	var lasterr error
	for attempt := 0; attempt <= p.retryAttmpts; attempt++ {
		if attempt > 0 {
			fmt.Printf("Retrying request to %s (attempt %d)\n", url, attempt)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(p.retryDelay):
			}
		}
		response, err := p.client.Get(ctx, url)
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
