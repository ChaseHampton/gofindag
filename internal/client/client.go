package client

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/ChaseHampton/gofindag/internal/config"
)

type Client struct {
	httpClient *http.Client
	userAgent  string
}

type Response struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
	Duration   time.Duration
}

func NewClient(cfg *config.HTTPConfig) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:    cfg.MaxIdleConns,
				MaxConnsPerHost: cfg.MaxConnsPerHost,
				IdleConnTimeout: cfg.IdleConnTimeout,
			},
		},
		userAgent: cfg.UserAgent,
	}
}

func (c *Client) Get(ctx context.Context, url string) (*Response, error) {
	return c.makeRequest(ctx, "GET", url, nil)
}

func (c *Client) makeRequest(ctx context.Context, method, url string, body io.Reader) (*Response, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	duration := time.Since(start)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	response := &Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       responseBody,
		Duration:   duration,
	}

	return response, nil
}
