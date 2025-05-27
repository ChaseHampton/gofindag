package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPConfig      HTTPConfig
	ProcessorConfig ProcessorConfig
	Tvp             TvpNames
}

type HTTPConfig struct {
	Timeout         time.Duration
	MaxIdleConns    int
	MaxConnsPerHost int
	UserAgent       string
	IdleConnTimeout time.Duration
	ProxyKey        *string
	ProxyUrl        *string
}

type ProcessorConfig struct {
	MaxConcurrency int
	RetryAttempts  int
	RetryDelay     time.Duration
	PageDelay      time.Duration
	MaxPages       int
	BaseURL        string
	BatchSize      int
	FlushTimeout   time.Duration
	ChannelSize    int
}

type TvpNames struct {
	MemorialTvpName   string
	PageTvpName       string
	MemorialIdTvpName string
}

type DbConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

func NewConfig() *Config {
	proxykey := LoadOptionalString("PROXY_KEY")
	proxyurl := LoadOptionalString("PROXY_URL")
	httptimeout := LoadDefaultInt("HTTP_TIMEOUT_SECS", 30)
	maxidleconns := LoadDefaultInt("HTTP_MAX_IDLE_CONNS", 100)
	maxconnsperhost := LoadDefaultInt("HTTP_MAX_CONNS_PER_HOST", 10)
	idleconntimeout := LoadDefaultInt("HTTP_IDLE_CONN_TIMEOUT_SECS", 30)
	useragent := LoadDefaultString("HTTP_USER_AGENT", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:138.0) Gecko/20100101 Firefox/138.0")
	maxconcurrency := LoadDefaultInt("PROCESSOR_MAX_CONCURRENCY", 8)
	retryattempts := LoadDefaultInt("PROCESSOR_RETRY_ATTEMPTS", 3)
	retrydelay := LoadDefaultInt("PROCESSOR_RETRY_DELAY_MS", 5000)
	pagedelay := LoadDefaultInt("PROCESSOR_PAGE_DELAY_MS", 1500)
	maxpages := LoadDefaultInt("PROCESSOR_MAX_PAGES", 500)
	batchsize := LoadDefaultInt("PROCESSOR_BATCH_SIZE", 20)
	flushTimeout := LoadDefaultInt("PROCESSOR_FLUSH_TIMEOUT_SECS", 5)
	channelSize := LoadDefaultInt("PROCESSOR_CHANNEL_SIZE", 1000)
	memTvpName := LoadDefaultString("MEMORIAL_TVP_NAME", "dbo.MemorialTableType")
	pageTvpName := LoadDefaultString("PAGE_TVP_NAME", "dbo.PageTableType")
	memIdTvpName := LoadDefaultString("MEMORIAL_ID_TVP_NAME", "dbo.MemorialIdList")
	return &Config{
		HTTPConfig: HTTPConfig{
			Timeout:         time.Duration(httptimeout) * time.Second,
			MaxIdleConns:    maxidleconns,
			MaxConnsPerHost: maxconnsperhost,
			UserAgent:       useragent,
			IdleConnTimeout: time.Duration(idleconntimeout) * time.Second,
			ProxyKey:        proxykey,
			ProxyUrl:        proxyurl,
		},
		ProcessorConfig: ProcessorConfig{
			MaxConcurrency: maxconcurrency,
			RetryAttempts:  retryattempts,
			RetryDelay:     time.Duration(retrydelay) * time.Millisecond,
			PageDelay:      time.Duration(pagedelay) * time.Millisecond,
			MaxPages:       maxpages,
			BaseURL:        "https://www.findagrave.com/memorial/search",
			BatchSize:      batchsize,
			FlushTimeout:   time.Duration(flushTimeout) * time.Second,
			ChannelSize:    channelSize,
		},
		Tvp: TvpNames{
			MemorialTvpName:   memTvpName,
			PageTvpName:       pageTvpName,
			MemorialIdTvpName: memIdTvpName,
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
	memid := os.Getenv("MEMORIAL_ID_TVP_NAME")
	if memid == "" {
		memid = "dbo.MemorialIdList"
	}
	return &DbConfig{
		Host:     os.Getenv("DB_HOST"),
		Port:     port,
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		DBName:   os.Getenv("DB_NAME"),
	}
}

func LoadDefaultInt(name string, defaultValue int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		return defaultValue
	}
	return value
}

func LoadDefaultString(name string, defaultValue string) string {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}
	return value
}

func LoadRequiredString(name string) string {
	value := os.Getenv(name)
	if value == "" {
		panic("Required environment variable " + name + " is not set")
	}
	return value
}

func LoadOptionalString(name string) *string {
	value := os.Getenv(name)
	if value == "" {
		return nil
	}
	return &value
}
