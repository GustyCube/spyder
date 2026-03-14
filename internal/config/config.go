package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration wraps time.Duration with YAML/JSON unmarshaling support.
type Duration time.Duration

// Unwrap returns the underlying time.Duration.
func (d Duration) Unwrap() time.Duration { return time.Duration(d) }

// UnmarshalYAML parses a duration from a YAML value.
// Accepts Go duration strings ("15s", "24h") or plain numbers (interpreted as seconds).
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err == nil {
		// Try parsing as a Go duration string first
		if parsed, err := time.ParseDuration(s); err == nil {
			*d = Duration(parsed)
			return nil
		}
	}
	// Fall back to interpreting as a number of seconds
	var n float64
	if err := value.Decode(&n); err == nil {
		*d = Duration(time.Duration(n * float64(time.Second)))
		return nil
	}
	return fmt.Errorf("cannot decode duration from YAML node")
}

// UnmarshalJSON parses a duration from a JSON value.
// Accepts Go duration strings ("15s", "24h") or plain numbers (interpreted as seconds).
func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		parsed, err := time.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", s, err)
		}
		*d = Duration(parsed)
		return nil
	}
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		*d = Duration(time.Duration(n * float64(time.Second)))
		return nil
	}
	return fmt.Errorf("cannot decode duration from JSON")
}

// MarshalJSON outputs the duration as a string like "15s".
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// MarshalYAML outputs the duration as a string like "15s".
func (d Duration) MarshalYAML() (interface{}, error) {
	return time.Duration(d).String(), nil
}

// CrawlingConfig holds crawling-related settings.
type CrawlingConfig struct {
	Continuous   bool    `yaml:"continuous" json:"continuous"`
	MaxDomains   int     `yaml:"max_domains" json:"max_domains"`
	Concurrency  int     `yaml:"concurrency" json:"concurrency"`
	RatePerHost  float64 `yaml:"rate_per_host" json:"rate_per_host"`
	RateBurst    int     `yaml:"rate_burst" json:"rate_burst"`
	HTTPTimeout  Duration `yaml:"http_timeout" json:"http_timeout"`
	TLSTimeout   Duration `yaml:"tls_timeout" json:"tls_timeout"`
	BodyMaxBytes int64   `yaml:"body_max_bytes" json:"body_max_bytes"`
}

// BatchConfig holds batch processing settings.
type BatchConfig struct {
	MaxEdges           int      `yaml:"max_edges" json:"max_edges"`
	FlushInterval      Duration `yaml:"flush_interval" json:"flush_interval"`
	NodeThresholdRatio float64  `yaml:"node_threshold_ratio" json:"node_threshold_ratio"`
	ChannelBuffer      int      `yaml:"channel_buffer" json:"channel_buffer"`
}

// OutputConfig holds output-related settings.
type OutputConfig struct {
	Format   string `yaml:"format" json:"format"`
	Ingest   string `yaml:"ingest" json:"ingest"`
	SpoolDir string `yaml:"spool_dir" json:"spool_dir"`
}

// CircuitBreakerConfig holds circuit breaker settings.
type CircuitBreakerConfig struct {
	MaxRequests  uint32   `yaml:"max_requests" json:"max_requests"`
	Interval     Duration `yaml:"interval" json:"interval"`
	Timeout      Duration `yaml:"timeout" json:"timeout"`
	Threshold    uint32   `yaml:"threshold" json:"threshold"`
	FailureRatio float64  `yaml:"failure_ratio" json:"failure_ratio"`
}

// HTTPClientConfig holds HTTP client settings.
type HTTPClientConfig struct {
	Timeout               Duration `yaml:"timeout" json:"timeout"`
	ResponseHeaderTimeout Duration `yaml:"response_header_timeout" json:"response_header_timeout"`
	IdleConnTimeout       Duration `yaml:"idle_conn_timeout" json:"idle_conn_timeout"`
	MaxIdleConns          int      `yaml:"max_idle_conns" json:"max_idle_conns"`
	MaxConnsPerHost       int      `yaml:"max_conns_per_host" json:"max_conns_per_host"`
}

// RobotsConfig holds robots.txt cache settings.
type RobotsConfig struct {
	CacheSize int      `yaml:"cache_size" json:"cache_size"`
	CacheTTL  Duration `yaml:"cache_ttl" json:"cache_ttl"`
}

// DedupConfig holds deduplication settings.
type DedupConfig struct {
	TTL Duration `yaml:"ttl" json:"ttl"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr         string   `yaml:"addr" json:"addr"`
	QueueAddr    string   `yaml:"queue_addr" json:"queue_addr"`
	QueueKey     string   `yaml:"queue_key" json:"queue_key"`
	QueueLeaseTTL Duration `yaml:"queue_lease_ttl" json:"queue_lease_ttl"`
}

// MongoDBConfig holds MongoDB connection settings.
type MongoDBConfig struct {
	URI      string `yaml:"uri" json:"uri"`
	Database string `yaml:"database" json:"database"`
}

// DashboardConfig holds dashboard settings.
type DashboardConfig struct {
	Enabled     bool `yaml:"enabled" json:"enabled"`
	HistorySize int  `yaml:"history_size" json:"history_size"`
}

// MTLSConfig holds mTLS settings.
type MTLSConfig struct {
	Cert string `yaml:"cert" json:"cert"`
	Key  string `yaml:"key" json:"key"`
	CA   string `yaml:"ca" json:"ca"`
}

// TelemetryConfig holds observability settings.
type TelemetryConfig struct {
	MetricsAddr  string `yaml:"metrics_addr" json:"metrics_addr"`
	OTELEndpoint string `yaml:"otel_endpoint" json:"otel_endpoint"`
	OTELInsecure bool   `yaml:"otel_insecure" json:"otel_insecure"`
	OTELService  string `yaml:"otel_service" json:"otel_service"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level    string `yaml:"level" json:"level"`
	Quiet    bool   `yaml:"quiet" json:"quiet"`
	Verbose  bool   `yaml:"verbose" json:"verbose"`
	Progress bool   `yaml:"progress" json:"progress"`
}

// APIKey represents an API key with scope and name.
type APIKey struct {
	Key   string `yaml:"key" json:"key"`
	Scope string `yaml:"scope" json:"scope"`
	Name  string `yaml:"name" json:"name"`
}

// APIConfig holds API settings.
type APIConfig struct {
	RateLimit int      `yaml:"rate_limit" json:"rate_limit"`
	RateBurst int      `yaml:"rate_burst" json:"rate_burst"`
	Keys      []APIKey `yaml:"keys" json:"keys"`
}

// RetryConfig holds retry settings.
type RetryConfig struct {
	MaxElapsedTime Duration `yaml:"max_elapsed_time" json:"max_elapsed_time"`
}

// Config represents the complete configuration for SPYDER.
type Config struct {
	// Core configuration
	Domains     string   `yaml:"domains" json:"domains"`
	Probe       string   `yaml:"probe" json:"probe"`
	Run         string   `yaml:"run" json:"run"`
	UA          string   `yaml:"ua" json:"ua"`
	ExcludeTLDs []string `yaml:"exclude_tlds" json:"exclude_tlds"`

	// Nested sub-configs
	Crawling       CrawlingConfig       `yaml:"crawling" json:"crawling"`
	Batch          BatchConfig          `yaml:"batch" json:"batch"`
	Output         OutputConfig         `yaml:"output" json:"output"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker" json:"circuit_breaker"`
	HTTPClient     HTTPClientConfig     `yaml:"http_client" json:"http_client"`
	Robots         RobotsConfig         `yaml:"robots" json:"robots"`
	Dedup          DedupConfig          `yaml:"dedup" json:"dedup"`
	Redis          RedisConfig          `yaml:"redis" json:"redis"`
	MongoDB        MongoDBConfig        `yaml:"mongodb" json:"mongodb"`
	Dashboard      DashboardConfig      `yaml:"dashboard" json:"dashboard"`
	MTLS           MTLSConfig           `yaml:"mtls" json:"mtls"`
	Telemetry      TelemetryConfig      `yaml:"telemetry" json:"telemetry"`
	Logging        LoggingConfig        `yaml:"logging" json:"logging"`
	API            APIConfig            `yaml:"api" json:"api"`
	Retry          RetryConfig          `yaml:"retry" json:"retry"`
}

// SetDefaults sets default values for the configuration.
func (c *Config) SetDefaults() {
	// Core defaults
	if c.Probe == "" {
		c.Probe = "local-1"
	}
	if c.Run == "" {
		c.Run = fmt.Sprintf("run-%d", time.Now().Unix())
	}
	if c.UA == "" {
		c.UA = "SPYDERProbe/1.0 (+https://github.com/gustycube/spyder)"
	}
	if len(c.ExcludeTLDs) == 0 {
		c.ExcludeTLDs = []string{"gov", "mil", "int"}
	}

	// Crawling defaults
	if c.Crawling.Concurrency == 0 {
		c.Crawling.Concurrency = 256
	}
	if c.Crawling.RatePerHost == 0 {
		c.Crawling.RatePerHost = 1.0
	}
	if c.Crawling.RateBurst == 0 {
		c.Crawling.RateBurst = 1
	}
	if c.Crawling.HTTPTimeout == 0 {
		c.Crawling.HTTPTimeout = Duration(15 * time.Second)
	}
	if c.Crawling.TLSTimeout == 0 {
		c.Crawling.TLSTimeout = Duration(8 * time.Second)
	}
	if c.Crawling.BodyMaxBytes == 0 {
		c.Crawling.BodyMaxBytes = 524288
	}

	// Batch defaults
	if c.Batch.MaxEdges == 0 {
		c.Batch.MaxEdges = 10000
	}
	if c.Batch.FlushInterval == 0 {
		c.Batch.FlushInterval = Duration(2 * time.Second)
	}
	if c.Batch.NodeThresholdRatio == 0 {
		c.Batch.NodeThresholdRatio = 0.5
	}
	if c.Batch.ChannelBuffer == 0 {
		c.Batch.ChannelBuffer = 1024
	}

	// Output defaults
	if c.Output.Format == "" {
		c.Output.Format = "json"
	}
	if c.Output.SpoolDir == "" {
		c.Output.SpoolDir = "spool"
	}

	// Circuit breaker defaults
	if c.CircuitBreaker.MaxRequests == 0 {
		c.CircuitBreaker.MaxRequests = 3
	}
	if c.CircuitBreaker.Interval == 0 {
		c.CircuitBreaker.Interval = Duration(60 * time.Second)
	}
	if c.CircuitBreaker.Timeout == 0 {
		c.CircuitBreaker.Timeout = Duration(30 * time.Second)
	}
	if c.CircuitBreaker.Threshold == 0 {
		c.CircuitBreaker.Threshold = 5
	}
	if c.CircuitBreaker.FailureRatio == 0 {
		c.CircuitBreaker.FailureRatio = 0.6
	}

	// HTTP client defaults
	if c.HTTPClient.Timeout == 0 {
		c.HTTPClient.Timeout = Duration(15 * time.Second)
	}
	if c.HTTPClient.ResponseHeaderTimeout == 0 {
		c.HTTPClient.ResponseHeaderTimeout = Duration(10 * time.Second)
	}
	if c.HTTPClient.IdleConnTimeout == 0 {
		c.HTTPClient.IdleConnTimeout = Duration(30 * time.Second)
	}
	if c.HTTPClient.MaxIdleConns == 0 {
		c.HTTPClient.MaxIdleConns = 1024
	}
	if c.HTTPClient.MaxConnsPerHost == 0 {
		c.HTTPClient.MaxConnsPerHost = 128
	}

	// Robots defaults
	if c.Robots.CacheSize == 0 {
		c.Robots.CacheSize = 4096
	}
	if c.Robots.CacheTTL == 0 {
		c.Robots.CacheTTL = Duration(24 * time.Hour)
	}

	// Dedup defaults
	if c.Dedup.TTL == 0 {
		c.Dedup.TTL = Duration(24 * time.Hour)
	}

	// Redis defaults
	if c.Redis.QueueKey == "" {
		c.Redis.QueueKey = "spyder:queue"
	}
	if c.Redis.QueueLeaseTTL == 0 {
		c.Redis.QueueLeaseTTL = Duration(120 * time.Second)
	}

	// MongoDB defaults
	if c.MongoDB.Database == "" {
		c.MongoDB.Database = "spyder"
	}

	// Dashboard defaults
	if !c.Dashboard.Enabled {
		c.Dashboard.Enabled = true
	}
	if c.Dashboard.HistorySize == 0 {
		c.Dashboard.HistorySize = 500
	}

	// Telemetry defaults
	if c.Telemetry.MetricsAddr == "" {
		c.Telemetry.MetricsAddr = ":9090"
	}
	if !c.Telemetry.OTELInsecure {
		c.Telemetry.OTELInsecure = true
	}
	if c.Telemetry.OTELService == "" {
		c.Telemetry.OTELService = "spyder-probe"
	}

	// Logging defaults
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if !c.Logging.Progress {
		c.Logging.Progress = true
	}

	// API defaults
	if c.API.RateLimit == 0 {
		c.API.RateLimit = 100
	}
	if c.API.RateBurst == 0 {
		c.API.RateBurst = 20
	}

	// Retry defaults
	if c.Retry.MaxElapsedTime == 0 {
		c.Retry.MaxElapsedTime = Duration(30 * time.Second)
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.Domains == "" {
		return fmt.Errorf("domains file path is required")
	}
	if c.Crawling.Concurrency < 1 {
		return fmt.Errorf("crawling.concurrency must be at least 1")
	}
	if c.Batch.MaxEdges < 1 {
		return fmt.Errorf("batch.max_edges must be at least 1")
	}
	return nil
}

// LoadFromFile loads configuration from a YAML or JSON file.
func LoadFromFile(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format: %s (use .yaml, .yml, or .json)", ext)
	}

	config.SetDefaults()

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// LoadFromEnv loads configuration from environment variables.
func (c *Config) LoadFromEnv() {
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		c.Redis.Addr = v
	}
	if v := os.Getenv("REDIS_QUEUE_ADDR"); v != "" {
		c.Redis.QueueAddr = v
	}
	if v := os.Getenv("REDIS_QUEUE_KEY"); v != "" {
		c.Redis.QueueKey = v
	}
	if v := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); v != "" {
		c.Telemetry.OTELEndpoint = v
	}
	if v := os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"); v != "" {
		c.Telemetry.OTELInsecure = strings.EqualFold(v, "true")
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		c.Logging.Level = v
	}
	if v := os.Getenv("MONGODB_URI"); v != "" {
		c.MongoDB.URI = v
	}
	if v := os.Getenv("SPYDER_API_KEYS"); v != "" {
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err == nil {
			var keys []APIKey
			if err := json.Unmarshal(decoded, &keys); err == nil {
				c.API.Keys = keys
			}
		}
	}
}
