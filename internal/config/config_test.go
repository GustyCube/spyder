package config

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// yamlUnmarshal is a helper to wrap yaml.Unmarshal for tests.
func yamlUnmarshal(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}

func TestLoadNestedYAML(t *testing.T) {
	yamlContent := `
domains: domains.txt
probe: test-probe
run: test-run
ua: "TestBot/1.0"
exclude_tlds:
  - test
  - example

crawling:
  continuous: true
  max_domains: 500
  concurrency: 512
  rate_per_host: 2.5
  rate_burst: 3
  http_timeout: 20s
  tls_timeout: 10s
  body_max_bytes: 1048576

batch:
  max_edges: 20000
  flush_interval: 5s
  node_threshold_ratio: 0.7
  channel_buffer: 2048

output:
  format: jsonl
  ingest: https://ingest.example.com
  spool_dir: /tmp/spool

circuit_breaker:
  max_requests: 5
  interval: 120s
  timeout: 60s
  threshold: 10
  failure_ratio: 0.8

http_client:
  timeout: 30s
  response_header_timeout: 20s
  idle_conn_timeout: 60s
  max_idle_conns: 2048
  max_conns_per_host: 256

robots:
  cache_size: 8192
  cache_ttl: 48h

dedup:
  ttl: 12h

redis:
  addr: redis.example.com:6379
  queue_addr: queue.example.com:6379
  queue_key: myapp:queue
  queue_lease_ttl: 300s

mongodb:
  uri: mongodb://localhost:27017
  database: testdb

dashboard:
  enabled: true
  history_size: 1000

mtls:
  cert: /path/to/cert.pem
  key: /path/to/key.pem
  ca: /path/to/ca.pem

telemetry:
  metrics_addr: ":8080"
  otel_endpoint: https://otel.example.com
  otel_insecure: true
  otel_service: my-service

logging:
  level: debug
  quiet: true
  verbose: true
  progress: true

api:
  rate_limit: 200
  rate_burst: 50
  keys:
    - key: abc123
      scope: admin
      name: admin-key

retry:
  max_elapsed_time: 60s
`

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFile(configFile)
	if err != nil {
		t.Fatalf("failed to load YAML config: %v", err)
	}

	// Core
	assertEqual(t, "Domains", cfg.Domains, "domains.txt")
	assertEqual(t, "Probe", cfg.Probe, "test-probe")
	assertEqual(t, "Run", cfg.Run, "test-run")
	assertEqual(t, "UA", cfg.UA, "TestBot/1.0")
	if len(cfg.ExcludeTLDs) != 2 || cfg.ExcludeTLDs[0] != "test" {
		t.Errorf("unexpected ExcludeTLDs: %v", cfg.ExcludeTLDs)
	}

	// Crawling
	if !cfg.Crawling.Continuous {
		t.Error("expected Crawling.Continuous to be true")
	}
	assertEqualInt(t, "Crawling.MaxDomains", cfg.Crawling.MaxDomains, 500)
	assertEqualInt(t, "Crawling.Concurrency", cfg.Crawling.Concurrency, 512)
	assertEqualFloat(t, "Crawling.RatePerHost", cfg.Crawling.RatePerHost, 2.5)
	assertEqualInt(t, "Crawling.RateBurst", cfg.Crawling.RateBurst, 3)
	assertEqualDuration(t, "Crawling.HTTPTimeout", cfg.Crawling.HTTPTimeout, 20*time.Second)
	assertEqualDuration(t, "Crawling.TLSTimeout", cfg.Crawling.TLSTimeout, 10*time.Second)
	if cfg.Crawling.BodyMaxBytes != 1048576 {
		t.Errorf("expected Crawling.BodyMaxBytes 1048576, got %d", cfg.Crawling.BodyMaxBytes)
	}

	// Batch
	assertEqualInt(t, "Batch.MaxEdges", cfg.Batch.MaxEdges, 20000)
	assertEqualDuration(t, "Batch.FlushInterval", cfg.Batch.FlushInterval, 5*time.Second)
	assertEqualFloat(t, "Batch.NodeThresholdRatio", cfg.Batch.NodeThresholdRatio, 0.7)
	assertEqualInt(t, "Batch.ChannelBuffer", cfg.Batch.ChannelBuffer, 2048)

	// Output
	assertEqual(t, "Output.Format", cfg.Output.Format, "jsonl")
	assertEqual(t, "Output.Ingest", cfg.Output.Ingest, "https://ingest.example.com")
	assertEqual(t, "Output.SpoolDir", cfg.Output.SpoolDir, "/tmp/spool")

	// CircuitBreaker
	if cfg.CircuitBreaker.MaxRequests != 5 {
		t.Errorf("expected CircuitBreaker.MaxRequests 5, got %d", cfg.CircuitBreaker.MaxRequests)
	}
	assertEqualDuration(t, "CircuitBreaker.Interval", cfg.CircuitBreaker.Interval, 120*time.Second)
	assertEqualDuration(t, "CircuitBreaker.Timeout", cfg.CircuitBreaker.Timeout, 60*time.Second)
	if cfg.CircuitBreaker.Threshold != 10 {
		t.Errorf("expected CircuitBreaker.Threshold 10, got %d", cfg.CircuitBreaker.Threshold)
	}
	assertEqualFloat(t, "CircuitBreaker.FailureRatio", cfg.CircuitBreaker.FailureRatio, 0.8)

	// HTTPClient
	assertEqualDuration(t, "HTTPClient.Timeout", cfg.HTTPClient.Timeout, 30*time.Second)
	assertEqualDuration(t, "HTTPClient.ResponseHeaderTimeout", cfg.HTTPClient.ResponseHeaderTimeout, 20*time.Second)
	assertEqualDuration(t, "HTTPClient.IdleConnTimeout", cfg.HTTPClient.IdleConnTimeout, 60*time.Second)
	assertEqualInt(t, "HTTPClient.MaxIdleConns", cfg.HTTPClient.MaxIdleConns, 2048)
	assertEqualInt(t, "HTTPClient.MaxConnsPerHost", cfg.HTTPClient.MaxConnsPerHost, 256)

	// Robots
	assertEqualInt(t, "Robots.CacheSize", cfg.Robots.CacheSize, 8192)
	assertEqualDuration(t, "Robots.CacheTTL", cfg.Robots.CacheTTL, 48*time.Hour)

	// Dedup
	assertEqualDuration(t, "Dedup.TTL", cfg.Dedup.TTL, 12*time.Hour)

	// Redis
	assertEqual(t, "Redis.Addr", cfg.Redis.Addr, "redis.example.com:6379")
	assertEqual(t, "Redis.QueueAddr", cfg.Redis.QueueAddr, "queue.example.com:6379")
	assertEqual(t, "Redis.QueueKey", cfg.Redis.QueueKey, "myapp:queue")
	assertEqualDuration(t, "Redis.QueueLeaseTTL", cfg.Redis.QueueLeaseTTL, 300*time.Second)

	// MongoDB
	assertEqual(t, "MongoDB.URI", cfg.MongoDB.URI, "mongodb://localhost:27017")
	assertEqual(t, "MongoDB.Database", cfg.MongoDB.Database, "testdb")

	// Dashboard
	if !cfg.Dashboard.Enabled {
		t.Error("expected Dashboard.Enabled to be true")
	}
	assertEqualInt(t, "Dashboard.HistorySize", cfg.Dashboard.HistorySize, 1000)

	// MTLS
	assertEqual(t, "MTLS.Cert", cfg.MTLS.Cert, "/path/to/cert.pem")
	assertEqual(t, "MTLS.Key", cfg.MTLS.Key, "/path/to/key.pem")
	assertEqual(t, "MTLS.CA", cfg.MTLS.CA, "/path/to/ca.pem")

	// Telemetry
	assertEqual(t, "Telemetry.MetricsAddr", cfg.Telemetry.MetricsAddr, ":8080")
	assertEqual(t, "Telemetry.OTELEndpoint", cfg.Telemetry.OTELEndpoint, "https://otel.example.com")
	if !cfg.Telemetry.OTELInsecure {
		t.Error("expected Telemetry.OTELInsecure to be true")
	}
	assertEqual(t, "Telemetry.OTELService", cfg.Telemetry.OTELService, "my-service")

	// Logging
	assertEqual(t, "Logging.Level", cfg.Logging.Level, "debug")
	if !cfg.Logging.Quiet {
		t.Error("expected Logging.Quiet to be true")
	}
	if !cfg.Logging.Verbose {
		t.Error("expected Logging.Verbose to be true")
	}
	if !cfg.Logging.Progress {
		t.Error("expected Logging.Progress to be true")
	}

	// API
	assertEqualInt(t, "API.RateLimit", cfg.API.RateLimit, 200)
	assertEqualInt(t, "API.RateBurst", cfg.API.RateBurst, 50)
	if len(cfg.API.Keys) != 1 {
		t.Fatalf("expected 1 API key, got %d", len(cfg.API.Keys))
	}
	assertEqual(t, "API.Keys[0].Key", cfg.API.Keys[0].Key, "abc123")
	assertEqual(t, "API.Keys[0].Scope", cfg.API.Keys[0].Scope, "admin")
	assertEqual(t, "API.Keys[0].Name", cfg.API.Keys[0].Name, "admin-key")

	// Retry
	assertEqualDuration(t, "Retry.MaxElapsedTime", cfg.Retry.MaxElapsedTime, 60*time.Second)
}

func TestSetDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	// Core
	assertEqual(t, "Probe", cfg.Probe, "local-1")
	assertEqual(t, "UA", cfg.UA, "SPYDERProbe/1.0 (+https://github.com/gustycube/spyder)")
	if len(cfg.ExcludeTLDs) != 3 {
		t.Errorf("expected 3 default excluded TLDs, got %d", len(cfg.ExcludeTLDs))
	}
	if cfg.Run == "" {
		t.Error("expected Run to have a default value")
	}

	// Crawling
	assertEqualInt(t, "Crawling.Concurrency", cfg.Crawling.Concurrency, 256)
	assertEqualFloat(t, "Crawling.RatePerHost", cfg.Crawling.RatePerHost, 1.0)
	assertEqualInt(t, "Crawling.RateBurst", cfg.Crawling.RateBurst, 1)
	assertEqualDuration(t, "Crawling.HTTPTimeout", cfg.Crawling.HTTPTimeout, 15*time.Second)
	assertEqualDuration(t, "Crawling.TLSTimeout", cfg.Crawling.TLSTimeout, 8*time.Second)
	if cfg.Crawling.BodyMaxBytes != 524288 {
		t.Errorf("expected Crawling.BodyMaxBytes 524288, got %d", cfg.Crawling.BodyMaxBytes)
	}

	// Batch
	assertEqualInt(t, "Batch.MaxEdges", cfg.Batch.MaxEdges, 10000)
	assertEqualDuration(t, "Batch.FlushInterval", cfg.Batch.FlushInterval, 2*time.Second)
	assertEqualFloat(t, "Batch.NodeThresholdRatio", cfg.Batch.NodeThresholdRatio, 0.5)
	assertEqualInt(t, "Batch.ChannelBuffer", cfg.Batch.ChannelBuffer, 1024)

	// Output
	assertEqual(t, "Output.Format", cfg.Output.Format, "json")
	assertEqual(t, "Output.SpoolDir", cfg.Output.SpoolDir, "spool")

	// CircuitBreaker
	if cfg.CircuitBreaker.MaxRequests != 3 {
		t.Errorf("expected CircuitBreaker.MaxRequests 3, got %d", cfg.CircuitBreaker.MaxRequests)
	}
	assertEqualDuration(t, "CircuitBreaker.Interval", cfg.CircuitBreaker.Interval, 60*time.Second)
	assertEqualDuration(t, "CircuitBreaker.Timeout", cfg.CircuitBreaker.Timeout, 30*time.Second)
	if cfg.CircuitBreaker.Threshold != 5 {
		t.Errorf("expected CircuitBreaker.Threshold 5, got %d", cfg.CircuitBreaker.Threshold)
	}
	assertEqualFloat(t, "CircuitBreaker.FailureRatio", cfg.CircuitBreaker.FailureRatio, 0.6)

	// HTTPClient
	assertEqualDuration(t, "HTTPClient.Timeout", cfg.HTTPClient.Timeout, 15*time.Second)
	assertEqualDuration(t, "HTTPClient.ResponseHeaderTimeout", cfg.HTTPClient.ResponseHeaderTimeout, 10*time.Second)
	assertEqualDuration(t, "HTTPClient.IdleConnTimeout", cfg.HTTPClient.IdleConnTimeout, 30*time.Second)
	assertEqualInt(t, "HTTPClient.MaxIdleConns", cfg.HTTPClient.MaxIdleConns, 1024)
	assertEqualInt(t, "HTTPClient.MaxConnsPerHost", cfg.HTTPClient.MaxConnsPerHost, 128)

	// Robots
	assertEqualInt(t, "Robots.CacheSize", cfg.Robots.CacheSize, 4096)
	assertEqualDuration(t, "Robots.CacheTTL", cfg.Robots.CacheTTL, 24*time.Hour)

	// Dedup
	assertEqualDuration(t, "Dedup.TTL", cfg.Dedup.TTL, 24*time.Hour)

	// Redis
	assertEqual(t, "Redis.QueueKey", cfg.Redis.QueueKey, "spyder:queue")
	assertEqualDuration(t, "Redis.QueueLeaseTTL", cfg.Redis.QueueLeaseTTL, 120*time.Second)

	// MongoDB
	assertEqual(t, "MongoDB.Database", cfg.MongoDB.Database, "spyder")

	// Dashboard
	if !cfg.Dashboard.Enabled {
		t.Error("expected Dashboard.Enabled default true")
	}
	assertEqualInt(t, "Dashboard.HistorySize", cfg.Dashboard.HistorySize, 500)

	// Telemetry
	assertEqual(t, "Telemetry.MetricsAddr", cfg.Telemetry.MetricsAddr, ":9090")
	if !cfg.Telemetry.OTELInsecure {
		t.Error("expected Telemetry.OTELInsecure default true")
	}
	assertEqual(t, "Telemetry.OTELService", cfg.Telemetry.OTELService, "spyder-probe")

	// Logging
	assertEqual(t, "Logging.Level", cfg.Logging.Level, "info")
	if !cfg.Logging.Progress {
		t.Error("expected Logging.Progress default true")
	}

	// API
	assertEqualInt(t, "API.RateLimit", cfg.API.RateLimit, 100)
	assertEqualInt(t, "API.RateBurst", cfg.API.RateBurst, 20)

	// Retry
	assertEqualDuration(t, "Retry.MaxElapsedTime", cfg.Retry.MaxElapsedTime, 30*time.Second)
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("REDIS_ADDR", "redis.test:6379")
	t.Setenv("REDIS_QUEUE_ADDR", "queue.test:6379")
	t.Setenv("REDIS_QUEUE_KEY", "test:queue")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "https://otel.test:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("MONGODB_URI", "mongodb://test:27017")

	// Base64-encoded JSON array of API keys
	keys := []APIKey{{Key: "testkey", Scope: "read", Name: "test"}}
	keysJSON, _ := json.Marshal(keys)
	t.Setenv("SPYDER_API_KEYS", base64.StdEncoding.EncodeToString(keysJSON))

	cfg := &Config{}
	cfg.LoadFromEnv()

	assertEqual(t, "Redis.Addr", cfg.Redis.Addr, "redis.test:6379")
	assertEqual(t, "Redis.QueueAddr", cfg.Redis.QueueAddr, "queue.test:6379")
	assertEqual(t, "Redis.QueueKey", cfg.Redis.QueueKey, "test:queue")
	assertEqual(t, "Telemetry.OTELEndpoint", cfg.Telemetry.OTELEndpoint, "https://otel.test:4317")
	if !cfg.Telemetry.OTELInsecure {
		t.Error("expected OTELInsecure true from env")
	}
	assertEqual(t, "Logging.Level", cfg.Logging.Level, "debug")
	assertEqual(t, "MongoDB.URI", cfg.MongoDB.URI, "mongodb://test:27017")

	if len(cfg.API.Keys) != 1 {
		t.Fatalf("expected 1 API key from env, got %d", len(cfg.API.Keys))
	}
	assertEqual(t, "API.Keys[0].Key", cfg.API.Keys[0].Key, "testkey")
	assertEqual(t, "API.Keys[0].Scope", cfg.API.Keys[0].Scope, "read")
	assertEqual(t, "API.Keys[0].Name", cfg.API.Keys[0].Name, "test")
}

func TestDurationUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected time.Duration
	}{
		{"15 seconds", "d: 15s", 15 * time.Second},
		{"24 hours", "d: 24h", 24 * time.Hour},
		{"500 milliseconds", "d: 500ms", 500 * time.Millisecond},
		{"number as seconds", "d: 30", 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result struct {
				D Duration `yaml:"d"`
			}
			if err := yamlUnmarshal([]byte(tt.yaml), &result); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}
			if result.D.Unwrap() != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result.D.Unwrap())
			}
		})
	}

	// Test JSON unmarshaling
	jsonTests := []struct {
		name     string
		json     string
		expected time.Duration
	}{
		{"JSON string 15s", `{"d":"15s"}`, 15 * time.Second},
		{"JSON string 24h", `{"d":"24h"}`, 24 * time.Hour},
		{"JSON string 500ms", `{"d":"500ms"}`, 500 * time.Millisecond},
		{"JSON number as seconds", `{"d":30}`, 30 * time.Second},
	}

	for _, tt := range jsonTests {
		t.Run(tt.name, func(t *testing.T) {
			var result struct {
				D Duration `json:"d"`
			}
			if err := json.Unmarshal([]byte(tt.json), &result); err != nil {
				t.Fatalf("failed to unmarshal JSON: %v", err)
			}
			if result.D.Unwrap() != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result.D.Unwrap())
			}
		})
	}

	// Test JSON marshaling
	t.Run("JSON marshal", func(t *testing.T) {
		d := Duration(15 * time.Second)
		data, err := json.Marshal(d)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}
		if string(data) != `"15s"` {
			t.Errorf("expected \"15s\", got %s", string(data))
		}
	})
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Domains:  "domains.txt",
				Crawling: CrawlingConfig{Concurrency: 256},
				Batch:    BatchConfig{MaxEdges: 10000},
			},
			wantErr: false,
		},
		{
			name: "missing domains",
			cfg: Config{
				Crawling: CrawlingConfig{Concurrency: 256},
				Batch:    BatchConfig{MaxEdges: 10000},
			},
			wantErr: true,
		},
		{
			name: "invalid concurrency",
			cfg: Config{
				Domains:  "domains.txt",
				Crawling: CrawlingConfig{Concurrency: 0},
				Batch:    BatchConfig{MaxEdges: 10000},
			},
			wantErr: true,
		},
		{
			name: "invalid batch max_edges",
			cfg: Config{
				Domains:  "domains.txt",
				Crawling: CrawlingConfig{Concurrency: 256},
				Batch:    BatchConfig{MaxEdges: 0},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Helper functions

func assertEqual(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("expected %s %q, got %q", field, want, got)
	}
}

func assertEqualInt(t *testing.T, field string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("expected %s %d, got %d", field, want, got)
	}
}

func assertEqualFloat(t *testing.T, field string, got, want float64) {
	t.Helper()
	if got != want {
		t.Errorf("expected %s %f, got %f", field, want, got)
	}
}

func assertEqualDuration(t *testing.T, field string, got Duration, want time.Duration) {
	t.Helper()
	if got.Unwrap() != want {
		t.Errorf("expected %s %v, got %v", field, want, got.Unwrap())
	}
}
