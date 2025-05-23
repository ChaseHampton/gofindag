package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPConfig      HTTPConfig
	ProcessorConfig ProcessorConfig
}

type HTTPConfig struct {
	Timeout         time.Duration
	MaxIdleConns    int
	MaxConnsPerHost int
	UserAgent       string
	IdleConnTimeout time.Duration
}

type ProcessorConfig struct {
	MaxConcurrency int
	RetryAttempts  int
	RetryDelay     time.Duration
	PageDelay      time.Duration
	MaxPages       int
	BaseURL        string
}

type DbConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	MemorialTvpName string
	PageTvpName     string
}

func NewConfig() *Config {
	return &Config{
		HTTPConfig: HTTPConfig{
			Timeout:         30 * time.Second,
			MaxIdleConns:    100,
			MaxConnsPerHost: 10,
			UserAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:138.0) Gecko/20100101 Firefox/138.0",
			IdleConnTimeout: 30 * time.Second,
		},
		ProcessorConfig: ProcessorConfig{
			MaxConcurrency: 5,
			RetryAttempts:  3,
			RetryDelay:     5 * time.Second,
			PageDelay:      1 * time.Second,
			MaxPages:       1000,
			BaseURL:        "https://www.findagrave.com/memorial/search",
		},
	}
}

func NewDbConfig() *DbConfig {
	port, err := strconv.Atoi(os.Getenv("DB_PORT"))
	if err != nil {
		port = 1433
	}
	mem := os.Getenv("MEMORIAL_TVP_NAME")
	if mem == "" {
		mem = "dbo.MemorialTableType"
	}
	page := os.Getenv("PAGE_TVP_NAME")
	if page == "" {
		page = "dbo.PageTableType"
	}
	return &DbConfig{
		Host:            os.Getenv("DB_HOST"),
		Port:            port,
		User:            os.Getenv("DB_USER"),
		Password:        os.Getenv("DB_PASSWORD"),
		DBName:          os.Getenv("DB_NAME"),
		MemorialTvpName: mem,
		PageTvpName:     page,
	}
}
