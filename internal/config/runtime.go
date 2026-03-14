package config

import (
	"fmt"
	"strings"
	"sync"
)

// RuntimeConfig wraps Config with a mutex, change listeners, and hot reload support.
type RuntimeConfig struct {
	mu        sync.RWMutex
	cfg       Config
	listeners []func(old, new_ Config)
}

// NewRuntimeConfig creates a new RuntimeConfig from the given Config.
func NewRuntimeConfig(cfg Config) *RuntimeConfig {
	return &RuntimeConfig{cfg: cfg}
}

// Get returns a snapshot of the current configuration.
func (rc *RuntimeConfig) Get() Config {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.cfg
}

// OnChange registers a listener that is called after a successful Apply.
func (rc *RuntimeConfig) OnChange(fn func(old, new_ Config)) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.listeners = append(rc.listeners, fn)
}

// Apply applies a ConfigPatch to the current configuration.
// Tier 1 fields are applied immediately.
// Tier 2 fields are applied with warnings.
// Tier 3 fields are rejected with an error (require restart).
func (rc *RuntimeConfig) Apply(patch ConfigPatch) ([]string, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Check for Tier 3 fields (require restart)
	var restartFields []string
	if patch.Redis != nil {
		if patch.Redis.Addr != nil {
			restartFields = append(restartFields, "redis.addr")
		}
		if patch.Redis.QueueAddr != nil {
			restartFields = append(restartFields, "redis.queue_addr")
		}
	}
	if patch.MTLS != nil {
		if patch.MTLS.Cert != nil {
			restartFields = append(restartFields, "mtls.cert")
		}
		if patch.MTLS.Key != nil {
			restartFields = append(restartFields, "mtls.key")
		}
		if patch.MTLS.CA != nil {
			restartFields = append(restartFields, "mtls.ca")
		}
	}
	if patch.MongoDB != nil {
		if patch.MongoDB.URI != nil {
			restartFields = append(restartFields, "mongodb.uri")
		}
		if patch.MongoDB.Database != nil {
			restartFields = append(restartFields, "mongodb.database")
		}
	}
	if patch.Telemetry != nil && patch.Telemetry.MetricsAddr != nil {
		restartFields = append(restartFields, "telemetry.metrics_addr")
	}
	if patch.Dashboard != nil && patch.Dashboard.HistorySize != nil {
		restartFields = append(restartFields, "dashboard.history_size")
	}
	if len(restartFields) > 0 {
		return nil, fmt.Errorf("fields require restart: %s", strings.Join(restartFields, ", "))
	}

	old := rc.cfg
	var warnings []string

	// Apply Tier 1 fields (safe, immediate)
	if patch.Crawling != nil {
		if patch.Crawling.Concurrency != nil {
			rc.cfg.Crawling.Concurrency = *patch.Crawling.Concurrency
		}
		if patch.Crawling.RatePerHost != nil {
			rc.cfg.Crawling.RatePerHost = *patch.Crawling.RatePerHost
		}
		if patch.Crawling.RateBurst != nil {
			rc.cfg.Crawling.RateBurst = *patch.Crawling.RateBurst
		}
		if patch.Crawling.MaxDomains != nil {
			rc.cfg.Crawling.MaxDomains = *patch.Crawling.MaxDomains
		}
		if patch.Crawling.Continuous != nil {
			rc.cfg.Crawling.Continuous = *patch.Crawling.Continuous
		}
		if patch.Crawling.HTTPTimeout != nil {
			rc.cfg.Crawling.HTTPTimeout = *patch.Crawling.HTTPTimeout
		}
		if patch.Crawling.TLSTimeout != nil {
			rc.cfg.Crawling.TLSTimeout = *patch.Crawling.TLSTimeout
		}
		if patch.Crawling.BodyMaxBytes != nil {
			rc.cfg.Crawling.BodyMaxBytes = *patch.Crawling.BodyMaxBytes
		}
	}
	if patch.Batch != nil {
		if patch.Batch.MaxEdges != nil {
			rc.cfg.Batch.MaxEdges = *patch.Batch.MaxEdges
		}
		if patch.Batch.FlushInterval != nil {
			rc.cfg.Batch.FlushInterval = *patch.Batch.FlushInterval
		}
		if patch.Batch.NodeThresholdRatio != nil {
			rc.cfg.Batch.NodeThresholdRatio = *patch.Batch.NodeThresholdRatio
		}
	}
	if patch.ExcludeTLDs != nil {
		rc.cfg.ExcludeTLDs = *patch.ExcludeTLDs
	}
	if patch.UA != nil {
		rc.cfg.UA = *patch.UA
	}
	if patch.API != nil {
		if patch.API.RateLimit != nil {
			rc.cfg.API.RateLimit = *patch.API.RateLimit
		}
		if patch.API.RateBurst != nil {
			rc.cfg.API.RateBurst = *patch.API.RateBurst
		}
	}
	if patch.Logging != nil {
		if patch.Logging.Level != nil {
			rc.cfg.Logging.Level = *patch.Logging.Level
		}
	}

	// Apply Tier 2 fields (with warnings)
	if patch.Output != nil {
		if patch.Output.Format != nil {
			rc.cfg.Output.Format = *patch.Output.Format
			warnings = append(warnings, "changing output format mid-run may cause mixed format output")
		}
		if patch.Output.Ingest != nil {
			rc.cfg.Output.Ingest = *patch.Output.Ingest
			warnings = append(warnings, "changing ingest endpoint mid-run may cause partial batch delivery to old endpoint")
		}
		if patch.Output.SpoolDir != nil {
			rc.cfg.Output.SpoolDir = *patch.Output.SpoolDir
			warnings = append(warnings, "changing spool directory mid-run leaves existing spooled files in old directory")
		}
	}

	// Notify listeners
	new_ := rc.cfg
	for _, fn := range rc.listeners {
		fn(old, new_)
	}

	return warnings, nil
}

// ReloadFromFile reloads configuration from a file and applies the diff.
func (rc *RuntimeConfig) ReloadFromFile(path string) error {
	newCfg, err := LoadFromFile(path)
	if err != nil {
		return fmt.Errorf("reload: %w", err)
	}

	// Build a patch from the diff
	patch := diffConfigs(rc.cfg, *newCfg)
	warnings, err := rc.Apply(patch)
	if err != nil {
		return err
	}
	_ = warnings // logged by caller
	return nil
}

// diffConfigs compares two configs and returns a patch with the differences.
// Only includes Tier 1 and Tier 2 fields (Tier 3 are handled by Apply's rejection logic).
func diffConfigs(old, new_ Config) ConfigPatch {
	var p ConfigPatch

	if old.UA != new_.UA {
		p.UA = &new_.UA
	}
	if !slicesEqual(old.ExcludeTLDs, new_.ExcludeTLDs) {
		p.ExcludeTLDs = &new_.ExcludeTLDs
	}

	// Crawling
	cp := &CrawlingPatch{}
	changed := false
	if old.Crawling.Concurrency != new_.Crawling.Concurrency {
		cp.Concurrency = &new_.Crawling.Concurrency
		changed = true
	}
	if old.Crawling.RatePerHost != new_.Crawling.RatePerHost {
		cp.RatePerHost = &new_.Crawling.RatePerHost
		changed = true
	}
	if old.Crawling.RateBurst != new_.Crawling.RateBurst {
		cp.RateBurst = &new_.Crawling.RateBurst
		changed = true
	}
	if old.Crawling.MaxDomains != new_.Crawling.MaxDomains {
		cp.MaxDomains = &new_.Crawling.MaxDomains
		changed = true
	}
	if old.Crawling.Continuous != new_.Crawling.Continuous {
		cp.Continuous = &new_.Crawling.Continuous
		changed = true
	}
	if changed {
		p.Crawling = cp
	}

	// Batch
	bp := &BatchPatch{}
	changed = false
	if old.Batch.MaxEdges != new_.Batch.MaxEdges {
		bp.MaxEdges = &new_.Batch.MaxEdges
		changed = true
	}
	if old.Batch.FlushInterval != new_.Batch.FlushInterval {
		bp.FlushInterval = &new_.Batch.FlushInterval
		changed = true
	}
	if changed {
		p.Batch = bp
	}

	// Output (Tier 2)
	op := &OutputPatch{}
	changed = false
	if old.Output.Format != new_.Output.Format {
		op.Format = &new_.Output.Format
		changed = true
	}
	if old.Output.Ingest != new_.Output.Ingest {
		op.Ingest = &new_.Output.Ingest
		changed = true
	}
	if old.Output.SpoolDir != new_.Output.SpoolDir {
		op.SpoolDir = &new_.Output.SpoolDir
		changed = true
	}
	if changed {
		p.Output = op
	}

	// API
	ap := &APIPatch{}
	changed = false
	if old.API.RateLimit != new_.API.RateLimit {
		ap.RateLimit = &new_.API.RateLimit
		changed = true
	}
	if old.API.RateBurst != new_.API.RateBurst {
		ap.RateBurst = &new_.API.RateBurst
		changed = true
	}
	if changed {
		p.API = ap
	}

	// Logging
	lp := &LoggingPatch{}
	if old.Logging.Level != new_.Logging.Level {
		lp.Level = &new_.Logging.Level
		p.Logging = lp
	}

	// Tier 3 fields - include them so Apply() can reject them properly
	if old.Redis.Addr != new_.Redis.Addr || old.Redis.QueueAddr != new_.Redis.QueueAddr {
		rp := &RedisPatch{}
		if old.Redis.Addr != new_.Redis.Addr {
			rp.Addr = &new_.Redis.Addr
		}
		if old.Redis.QueueAddr != new_.Redis.QueueAddr {
			rp.QueueAddr = &new_.Redis.QueueAddr
		}
		p.Redis = rp
	}
	if old.MongoDB.URI != new_.MongoDB.URI {
		mp := &MongoDBPatch{}
		mp.URI = &new_.MongoDB.URI
		p.MongoDB = mp
	}

	return p
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ConfigPatch represents a partial config update. Nil pointer = don't change.
type ConfigPatch struct {
	UA          *string         `json:"ua,omitempty"`
	ExcludeTLDs *[]string       `json:"exclude_tlds,omitempty"`
	Crawling    *CrawlingPatch  `json:"crawling,omitempty"`
	Batch       *BatchPatch     `json:"batch,omitempty"`
	Output      *OutputPatch    `json:"output,omitempty"`
	API         *APIPatch       `json:"api,omitempty"`
	Logging     *LoggingPatch   `json:"logging,omitempty"`
	Redis       *RedisPatch     `json:"redis,omitempty"`
	MTLS        *MTLSPatch      `json:"mtls,omitempty"`
	MongoDB     *MongoDBPatch   `json:"mongodb,omitempty"`
	Telemetry   *TelemetryPatch `json:"telemetry,omitempty"`
	Dashboard   *DashboardPatch `json:"dashboard,omitempty"`
}

// CrawlingPatch holds partial updates for crawling configuration.
type CrawlingPatch struct {
	Concurrency  *int      `json:"concurrency,omitempty"`
	RatePerHost  *float64  `json:"rate_per_host,omitempty"`
	RateBurst    *int      `json:"rate_burst,omitempty"`
	MaxDomains   *int      `json:"max_domains,omitempty"`
	Continuous   *bool     `json:"continuous,omitempty"`
	HTTPTimeout  *Duration `json:"http_timeout,omitempty"`
	TLSTimeout   *Duration `json:"tls_timeout,omitempty"`
	BodyMaxBytes *int64    `json:"body_max_bytes,omitempty"`
}

// BatchPatch holds partial updates for batch configuration.
type BatchPatch struct {
	MaxEdges           *int      `json:"max_edges,omitempty"`
	FlushInterval      *Duration `json:"flush_interval,omitempty"`
	NodeThresholdRatio *float64  `json:"node_threshold_ratio,omitempty"`
}

// OutputPatch holds partial updates for output configuration.
type OutputPatch struct {
	Format   *string `json:"format,omitempty"`
	Ingest   *string `json:"ingest,omitempty"`
	SpoolDir *string `json:"spool_dir,omitempty"`
}

// APIPatch holds partial updates for API configuration.
type APIPatch struct {
	RateLimit *int `json:"rate_limit,omitempty"`
	RateBurst *int `json:"rate_burst,omitempty"`
}

// LoggingPatch holds partial updates for logging configuration.
type LoggingPatch struct {
	Level *string `json:"level,omitempty"`
}

// RedisPatch holds partial updates for Redis configuration (Tier 3 - rejected at runtime).
type RedisPatch struct {
	Addr      *string `json:"addr,omitempty"`
	QueueAddr *string `json:"queue_addr,omitempty"`
}

// MTLSPatch holds partial updates for mTLS configuration (Tier 3 - rejected at runtime).
type MTLSPatch struct {
	Cert *string `json:"cert,omitempty"`
	Key  *string `json:"key,omitempty"`
	CA   *string `json:"ca,omitempty"`
}

// MongoDBPatch holds partial updates for MongoDB configuration (Tier 3 - rejected at runtime).
type MongoDBPatch struct {
	URI      *string `json:"uri,omitempty"`
	Database *string `json:"database,omitempty"`
}

// TelemetryPatch holds partial updates for telemetry configuration (Tier 3 - rejected at runtime).
type TelemetryPatch struct {
	MetricsAddr *string `json:"metrics_addr,omitempty"`
}

// DashboardPatch holds partial updates for dashboard configuration (Tier 3 - rejected at runtime).
type DashboardPatch struct {
	HistorySize *int `json:"history_size,omitempty"`
}
