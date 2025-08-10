package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete configuration for SPYDER
type Config struct {
	// Core configuration
	Domains     string   `yaml:"domains" json:"domains"`
	Probe       string   `yaml:"probe" json:"probe"`
	Run         string   `yaml:"run" json:"run"`
	UA          string   `yaml:"ua" json:"ua"`
	ExcludeTLDs []string `yaml:"exclude_tlds" json:"exclude_tlds"`

	// Performance
	Concurrency    int `yaml:"concurrency" json:"concurrency"`
	BatchMaxEdges  int `yaml:"batch_max_edges" json:"batch_max_edges"`
	BatchFlushSec  int `yaml:"batch_flush_sec" json:"batch_flush_sec"`

	// Output
	Ingest   string `yaml:"ingest" json:"ingest"`
	SpoolDir string `yaml:"spool_dir" json:"spool_dir"`

	// mTLS
	MTLSCert string `yaml:"mtls_cert" json:"mtls_cert"`
	MTLSKey  string `yaml:"mtls_key" json:"mtls_key"`
	MTLSCA   string `yaml:"mtls_ca" json:"mtls_ca"`

	// Observability
	MetricsAddr   string `yaml:"metrics_addr" json:"metrics_addr"`
	OTELEndpoint  string `yaml:"otel_endpoint" json:"otel_endpoint"`
	OTELInsecure  bool   `yaml:"otel_insecure" json:"otel_insecure"`
	OTELService   string `yaml:"otel_service" json:"otel_service"`

	// Redis
	RedisAddr      string `yaml:"redis_addr" json:"redis_addr"`
	RedisQueueAddr string `yaml:"redis_queue_addr" json:"redis_queue_addr"`
	RedisQueueKey  string `yaml:"redis_queue_key" json:"redis_queue_key"`
}

// SetDefaults sets default values for the configuration
func (c *Config) SetDefaults() {
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
	if c.Concurrency == 0 {
		c.Concurrency = 256
	}
	if c.BatchMaxEdges == 0 {
		c.BatchMaxEdges = 10000
	}
	if c.BatchFlushSec == 0 {
		c.BatchFlushSec = 2
	}
	if c.SpoolDir == "" {
		c.SpoolDir = "spool"
	}
	if c.MetricsAddr == "" {
		c.MetricsAddr = ":9090"
	}
	if c.OTELService == "" {
		c.OTELService = "spyder-probe"
	}
	if c.RedisQueueKey == "" {
		c.RedisQueueKey = "spyder:queue"
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Domains == "" {
		return fmt.Errorf("domains file path is required")
	}
	if c.Concurrency < 1 {
		return fmt.Errorf("concurrency must be at least 1")
	}
	if c.BatchMaxEdges < 1 {
		return fmt.Errorf("batch_max_edges must be at least 1")
	}
	if c.BatchFlushSec < 1 {
		return fmt.Errorf("batch_flush_sec must be at least 1")
	}
	return nil
}

// LoadFromFile loads configuration from a YAML or JSON file
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

// MergeWithFlags merges command-line flags with file configuration
// Command-line flags take precedence over file configuration
func (c *Config) MergeWithFlags(flags map[string]interface{}) {
	if v, ok := flags["domains"].(string); ok && v != "" {
		c.Domains = v
	}
	if v, ok := flags["probe"].(string); ok && v != "" {
		c.Probe = v
	}
	if v, ok := flags["run"].(string); ok && v != "" {
		c.Run = v
	}
	if v, ok := flags["ua"].(string); ok && v != "" {
		c.UA = v
	}
	if v, ok := flags["concurrency"].(int); ok && v > 0 {
		c.Concurrency = v
	}
	if v, ok := flags["ingest"].(string); ok && v != "" {
		c.Ingest = v
	}
	if v, ok := flags["metrics_addr"].(string); ok && v != "" {
		c.MetricsAddr = v
	}
	if v, ok := flags["batch_max_edges"].(int); ok && v > 0 {
		c.BatchMaxEdges = v
	}
	if v, ok := flags["batch_flush_sec"].(int); ok && v > 0 {
		c.BatchFlushSec = v
	}
	if v, ok := flags["spool_dir"].(string); ok && v != "" {
		c.SpoolDir = v
	}
	if v, ok := flags["mtls_cert"].(string); ok && v != "" {
		c.MTLSCert = v
	}
	if v, ok := flags["mtls_key"].(string); ok && v != "" {
		c.MTLSKey = v
	}
	if v, ok := flags["mtls_ca"].(string); ok && v != "" {
		c.MTLSCA = v
	}
	if v, ok := flags["otel_endpoint"].(string); ok && v != "" {
		c.OTELEndpoint = v
	}
	if v, ok := flags["otel_insecure"].(bool); ok {
		c.OTELInsecure = v
	}
	if v, ok := flags["otel_service"].(string); ok && v != "" {
		c.OTELService = v
	}
}

// LoadFromEnv loads configuration from environment variables
func (c *Config) LoadFromEnv() {
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		c.RedisAddr = v
	}
	if v := os.Getenv("REDIS_QUEUE_ADDR"); v != "" {
		c.RedisQueueAddr = v
	}
	if v := os.Getenv("REDIS_QUEUE_KEY"); v != "" {
		c.RedisQueueKey = v
	}
}