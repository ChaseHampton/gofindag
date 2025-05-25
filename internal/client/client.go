package client

import (
	"context"
	"io"
	"net/http"
	"net/url"
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

func NewPageClient(cfg *config.HTTPConfig) *Client {
	cookiejar := http.CookieJar(nil)
	transport := &http.Transport{
		MaxIdleConns:      cfg.MaxIdleConns,
		MaxConnsPerHost:   cfg.MaxConnsPerHost,
		IdleConnTimeout:   cfg.IdleConnTimeout,
		DisableKeepAlives: true,
	}
	if *cfg.ProxyKey != "" && *cfg.ProxyUrl != "" {
		proxyURL, err := url.Parse(*cfg.ProxyUrl)
		if err == nil {
			proxyURL.User = url.User(*cfg.ProxyKey)
			transport.Proxy = http.ProxyURL(proxyURL)
		}

	}
	return &Client{
		httpClient: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: transport,
			Jar:       cookiejar,
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
