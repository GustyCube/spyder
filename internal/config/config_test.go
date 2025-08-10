package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFile_YAML(t *testing.T) {
	yamlContent := `
probe: test-probe
domains: domains.txt
concurrency: 512
batch_max_edges: 20000
exclude_tlds:
  - test
  - example
ingest: https://test.example.com/ingest
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

	if cfg.Probe != "test-probe" {
		t.Errorf("expected probe 'test-probe', got %s", cfg.Probe)
	}
	if cfg.Domains != "domains.txt" {
		t.Errorf("expected domains 'domains.txt', got %s", cfg.Domains)
	}
	if cfg.Concurrency != 512 {
		t.Errorf("expected concurrency 512, got %d", cfg.Concurrency)
	}
	if cfg.BatchMaxEdges != 20000 {
		t.Errorf("expected batch_max_edges 20000, got %d", cfg.BatchMaxEdges)
	}
	if len(cfg.ExcludeTLDs) != 2 || cfg.ExcludeTLDs[0] != "test" {
		t.Errorf("unexpected exclude_tlds: %v", cfg.ExcludeTLDs)
	}
	if cfg.Ingest != "https://test.example.com/ingest" {
		t.Errorf("expected ingest URL, got %s", cfg.Ingest)
	}
}

func TestLoadFromFile_JSON(t *testing.T) {
	jsonContent := `{
		"probe": "json-probe",
		"domains": "domains.json",
		"concurrency": 128,
		"batch_max_edges": 5000,
		"exclude_tlds": ["json", "test"],
		"metrics_addr": ":8080"
	}`

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(configFile, []byte(jsonContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFile(configFile)
	if err != nil {
		t.Fatalf("failed to load JSON config: %v", err)
	}

	if cfg.Probe != "json-probe" {
		t.Errorf("expected probe 'json-probe', got %s", cfg.Probe)
	}
	if cfg.Domains != "domains.json" {
		t.Errorf("expected domains 'domains.json', got %s", cfg.Domains)
	}
	if cfg.Concurrency != 128 {
		t.Errorf("expected concurrency 128, got %d", cfg.Concurrency)
	}
	if cfg.MetricsAddr != ":8080" {
		t.Errorf("expected metrics_addr ':8080', got %s", cfg.MetricsAddr)
	}
}

func TestSetDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Probe != "local-1" {
		t.Errorf("expected default probe 'local-1', got %s", cfg.Probe)
	}
	if cfg.UA != "SPYDERProbe/1.0 (+https://github.com/gustycube/spyder)" {
		t.Errorf("unexpected default UA: %s", cfg.UA)
	}
	if cfg.Concurrency != 256 {
		t.Errorf("expected default concurrency 256, got %d", cfg.Concurrency)
	}
	if cfg.BatchMaxEdges != 10000 {
		t.Errorf("expected default batch_max_edges 10000, got %d", cfg.BatchMaxEdges)
	}
	if cfg.BatchFlushSec != 2 {
		t.Errorf("expected default batch_flush_sec 2, got %d", cfg.BatchFlushSec)
	}
	if len(cfg.ExcludeTLDs) != 3 {
		t.Errorf("expected 3 default excluded TLDs, got %d", len(cfg.ExcludeTLDs))
	}
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
				Domains:       "domains.txt",
				Concurrency:   256,
				BatchMaxEdges: 10000,
				BatchFlushSec: 2,
			},
			wantErr: false,
		},
		{
			name: "missing domains",
			cfg: Config{
				Concurrency:   256,
				BatchMaxEdges: 10000,
				BatchFlushSec: 2,
			},
			wantErr: true,
		},
		{
			name: "invalid concurrency",
			cfg: Config{
				Domains:       "domains.txt",
				Concurrency:   0,
				BatchMaxEdges: 10000,
				BatchFlushSec: 2,
			},
			wantErr: true,
		},
		{
			name: "invalid batch_max_edges",
			cfg: Config{
				Domains:       "domains.txt",
				Concurrency:   256,
				BatchMaxEdges: 0,
				BatchFlushSec: 2,
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

func TestMergeWithFlags(t *testing.T) {
	cfg := &Config{
		Domains:     "original.txt",
		Probe:       "original-probe",
		Concurrency: 128,
	}

	flags := map[string]interface{}{
		"domains":     "new.txt",
		"concurrency": 512,
		"ingest":      "https://new.example.com",
	}

	cfg.MergeWithFlags(flags)

	if cfg.Domains != "new.txt" {
		t.Errorf("expected domains to be overridden to 'new.txt', got %s", cfg.Domains)
	}
	if cfg.Probe != "original-probe" {
		t.Errorf("expected probe to remain 'original-probe', got %s", cfg.Probe)
	}
	if cfg.Concurrency != 512 {
		t.Errorf("expected concurrency to be overridden to 512, got %d", cfg.Concurrency)
	}
	if cfg.Ingest != "https://new.example.com" {
		t.Errorf("expected ingest to be set, got %s", cfg.Ingest)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("REDIS_ADDR", "redis.test:6379")
	os.Setenv("REDIS_QUEUE_ADDR", "queue.test:6379")
	os.Setenv("REDIS_QUEUE_KEY", "test:queue")
	defer os.Unsetenv("REDIS_ADDR")
	defer os.Unsetenv("REDIS_QUEUE_ADDR")
	defer os.Unsetenv("REDIS_QUEUE_KEY")

	cfg := &Config{}
	cfg.LoadFromEnv()

	if cfg.RedisAddr != "redis.test:6379" {
		t.Errorf("expected RedisAddr from env, got %s", cfg.RedisAddr)
	}
	if cfg.RedisQueueAddr != "queue.test:6379" {
		t.Errorf("expected RedisQueueAddr from env, got %s", cfg.RedisQueueAddr)
	}
	if cfg.RedisQueueKey != "test:queue" {
		t.Errorf("expected RedisQueueKey from env, got %s", cfg.RedisQueueKey)
	}
}