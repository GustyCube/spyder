package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gustycube/spyder/internal/api"
	"github.com/gustycube/spyder/internal/circuitbreaker"
	"github.com/gustycube/spyder/internal/config"
	"github.com/gustycube/spyder/internal/dashboard"
	"github.com/gustycube/spyder/internal/dedup"
	"github.com/gustycube/spyder/internal/discover"
	"github.com/gustycube/spyder/internal/emit"
	"github.com/gustycube/spyder/internal/health"
	"github.com/gustycube/spyder/internal/httpclient"
	"github.com/gustycube/spyder/internal/hub"
	"github.com/gustycube/spyder/internal/logging"
	"github.com/gustycube/spyder/internal/metrics"
	"github.com/gustycube/spyder/internal/output"
	"github.com/gustycube/spyder/internal/pool"
	"github.com/gustycube/spyder/internal/probe"
	"github.com/gustycube/spyder/internal/queue"
	"github.com/gustycube/spyder/internal/rate"
	"github.com/gustycube/spyder/internal/robots"
	"github.com/gustycube/spyder/internal/store"
	"github.com/gustycube/spyder/internal/telemetry"
)

func main() {
	var configFile string
	var domainsFile string
	var ingest string
	var probeID string
	var runID string
	var concurrency int
	var ua string
	var exclude string
	var metricsAddr string
	var batchMax int
	var batchFlushSec int
	var spoolDir string
	var otelEndpoint string
	var otelInsecure bool
	var otelService string
	var mtlsCert, mtlsKey, mtlsCA string
	var outputFormat string
	var quiet bool
	var verbose bool
	var progress bool
	var showVersion bool
	var continuous bool
	var maxDomains int64

	// Add config file flag
	flag.StringVar(&configFile, "config", "", "path to config file (YAML or JSON)")
	flag.StringVar(&domainsFile, "domains", "", "path to newline-separated domains")
	flag.StringVar(&ingest, "ingest", "", "ingest endpoint (optional). If empty, prints JSON batches to stdout")
	flag.StringVar(&probeID, "probe", "", "probe id")
	flag.StringVar(&runID, "run", "", "run id")
	flag.IntVar(&concurrency, "concurrency", 0, "concurrent workers")
	flag.StringVar(&ua, "ua", "", "user-agent")
	flag.StringVar(&exclude, "exclude_tlds", "", "comma-separated TLDs to skip crawling")
	flag.StringVar(&metricsAddr, "metrics_addr", "", "metrics listen addr (empty to disable)")
	flag.IntVar(&batchMax, "batch_max_edges", 0, "max edges per batch before flush")
	flag.IntVar(&batchFlushSec, "batch_flush_sec", 0, "seconds timer to flush a batch")
	flag.StringVar(&spoolDir, "spool_dir", "", "spool dir for failed batches")
	flag.StringVar(&mtlsCert, "mtls_cert", "", "client cert (PEM) for mTLS to ingest")
	flag.StringVar(&mtlsKey, "mtls_key", "", "client key (PEM) for mTLS to ingest")
	flag.StringVar(&mtlsCA, "mtls_ca", "", "CA bundle (PEM) for mTLS to ingest")
	flag.StringVar(&otelEndpoint, "otel_endpoint", "", "OTLP HTTP endpoint (host:port)")
	flag.BoolVar(&otelInsecure, "otel_insecure", true, "OTLP insecure (no TLS)")
	flag.StringVar(&otelService, "otel_service", "", "OTEL service.name")
	flag.StringVar(&outputFormat, "output_format", "", "output format (json, jsonl, csv, pretty)")
	flag.BoolVar(&quiet, "quiet", false, "suppress progress output")
	flag.BoolVar(&verbose, "verbose", false, "verbose logging")
	flag.BoolVar(&progress, "progress", true, "show progress indicators")
	flag.BoolVar(&showVersion, "version", false, "show version and exit")
	var enableDashboard bool
	var mongoURI string
	var mongoDB string
	flag.BoolVar(&continuous, "continuous", false, "enable recursive crawling of discovered domains")
	flag.Int64Var(&maxDomains, "max_domains", 0, "max discovered domains to crawl in continuous mode (0 = unlimited)")
	flag.BoolVar(&enableDashboard, "dashboard", true, "enable live web dashboard on metrics port")
	flag.StringVar(&mongoURI, "mongodb", "", "MongoDB connection URI (e.g. mongodb://localhost:27017)")
	flag.StringVar(&mongoDB, "mongodb_db", "spyder", "MongoDB database name")

	// Custom usage function
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "SPYDER (System for Probing and Yielding DNS-based Entity Relations)\n")
		fmt.Fprintf(os.Stderr, "A high-performance network reconnaissance tool for mapping inter-domain relationships\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -domains=domains.txt -concurrency=128\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -config=config.yaml -progress=false\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -domains=domains.txt -output_format=csv > output.csv\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nEnvironment Variables:\n")
		fmt.Fprintf(os.Stderr, "  REDIS_ADDR       Redis server for deduplication\n")
		fmt.Fprintf(os.Stderr, "  REDIS_QUEUE_ADDR Redis server for work queue\n")
		fmt.Fprintf(os.Stderr, "  LOG_LEVEL        Log level (debug, info, warn, error)\n")
		fmt.Fprintf(os.Stderr, "\nFor more information: https://github.com/gustycube/spyder\n")
	}

	flag.Parse()

	// Handle version flag
	if showVersion {
		fmt.Println("SPYDER Probe v1.0.0")
		fmt.Println("Built with Go", strings.TrimPrefix(runtime.Version(), "go"))
		fmt.Println("https://github.com/gustycube/spyder")
		os.Exit(0)
	}

	// Show help if no domains file specified and no config file
	if domainsFile == "" && configFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	ctx := context.Background()
	log := logging.New()
	defer log.Sync()

	// Load configuration
	var cfg *config.Config
	var err error

	if configFile != "" {
		// Load from config file
		cfg, err = config.LoadFromFile(configFile)
		if err != nil {
			log.Fatal("failed to load config file", "file", configFile, "err", err)
		}
		log.Info("loaded config from file", "file", configFile)
	} else {
		// Create default config
		cfg = &config.Config{}
		cfg.SetDefaults()
	}

	// Load environment variables
	cfg.LoadFromEnv()

	// Override with command-line flags (if provided)
	if domainsFile != "" {
		cfg.Domains = domainsFile
	}
	if probeID != "" {
		cfg.Probe = probeID
	}
	if runID != "" {
		cfg.Run = runID
	}
	if ua != "" {
		cfg.UA = ua
	}
	if concurrency > 0 {
		cfg.Crawling.Concurrency = concurrency
	}
	if ingest != "" {
		cfg.Output.Ingest = ingest
	}
	if metricsAddr != "" {
		cfg.Telemetry.MetricsAddr = metricsAddr
	}
	if batchMax > 0 {
		cfg.Batch.MaxEdges = batchMax
	}
	if batchFlushSec > 0 {
		cfg.Batch.FlushInterval = config.Duration(time.Duration(batchFlushSec) * time.Second)
	}
	if spoolDir != "" {
		cfg.Output.SpoolDir = spoolDir
	}
	if mtlsCert != "" {
		cfg.MTLS.Cert = mtlsCert
	}
	if mtlsKey != "" {
		cfg.MTLS.Key = mtlsKey
	}
	if mtlsCA != "" {
		cfg.MTLS.CA = mtlsCA
	}
	if otelEndpoint != "" {
		cfg.Telemetry.OTELEndpoint = otelEndpoint
	}
	if otelService != "" {
		cfg.Telemetry.OTELService = otelService
	}
	if outputFormat != "" {
		cfg.Output.Format = outputFormat
	}
	cfg.Telemetry.OTELInsecure = otelInsecure

	// Handle exclude_tlds flag specially
	if exclude != "" {
		cfg.ExcludeTLDs = []string{}
		for _, t := range strings.Split(exclude, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				cfg.ExcludeTLDs = append(cfg.ExcludeTLDs, t)
			}
		}
	}

	// Map CLI-only flags to config
	cfg.Crawling.Continuous = continuous
	if maxDomains > 0 {
		cfg.Crawling.MaxDomains = int(maxDomains)
	}
	cfg.Dashboard.Enabled = enableDashboard
	if mongoURI != "" {
		cfg.MongoDB.URI = mongoURI
	}
	if mongoDB != "spyder" {
		cfg.MongoDB.Database = mongoDB
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatal("invalid configuration", "err", err)
	}

	// Initialize telemetry
	shutdown, err := telemetry.Init(ctx, cfg.Telemetry.OTELEndpoint, cfg.Telemetry.OTELService, cfg.Telemetry.OTELInsecure)
	if err != nil {
		log.Warn("otel init failed", "err", err)
	} else {
		defer shutdown(context.Background())
	}

	// Initialize health handler
	healthHandler := health.NewHandler(log)
	healthHandler.SetMetadata("probe", cfg.Probe)
	healthHandler.SetMetadata("run", cfg.Run)
	healthHandler.SetMetadata("version", "1.0.0")

	// Create explicit ServeMux
	mux := http.NewServeMux()

	// Register health routes (unauthenticated)
	healthHandler.RegisterRoutes(mux)

	// Open domains file
	f, err := os.Open(cfg.Domains)
	if err != nil {
		log.Fatal("open domains", "err", err)
	}
	defer f.Close()

	// Setup signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Initialize deduplication
	var d dedup.Interface
	var redisHealthCheck func() error
	if cfg.Redis.Addr != "" {
		rd, err := dedup.NewRedis(cfg.Redis.Addr, 24*time.Hour, log)
		if err != nil {
			log.Fatal("redis init", "err", err)
		}
		log.Info("redis dedupe enabled", "addr", cfg.Redis.Addr)
		d = rd

		// Register Redis health check
		redisHealthCheck = func() error {
			// Simple ping check - would need to expose from dedup.Redis
			return nil
		}
		healthHandler.RegisterChecker("redis", health.NewRedisChecker(cfg.Redis.Addr, redisHealthCheck))
	} else {
		d = dedup.NewMemory()
		log.Info("memory dedupe enabled")
	}

	// Initialize output writer for stdout mode
	var stdoutWriter emit.StdoutWriter
	if cfg.Output.Ingest == "" {
		ow, err := output.NewStdoutWriter(cfg.Output.Format)
		if err != nil {
			log.Fatal("output writer init", "err", err)
		}
		stdoutWriter = ow
	}

	// Initialize database store (optional)
	var batchStore emit.BatchStore
	if cfg.MongoDB.URI != "" {
		ms, err := store.NewMongo(ctx, cfg.MongoDB.URI, cfg.MongoDB.Database, log)
		if err != nil {
			log.Fatal("mongodb init", "err", err)
		}
		defer ms.Close(context.Background())
		batchStore = ms
		log.Info("mongodb storage enabled", "uri", cfg.MongoDB.URI, "db", cfg.MongoDB.Database)
	}

	// Initialize emitter
	emitterBatches := make(chan emit.Batch, cfg.Batch.ChannelBuffer)
	emitter, err := emit.NewEmitter(emit.EmitterConfig{
		Ingest:             cfg.Output.Ingest,
		ProbeID:            cfg.Probe,
		RunID:              cfg.Run,
		BatchMaxEdges:      cfg.Batch.MaxEdges,
		FlushEvery:         cfg.Batch.FlushInterval.Unwrap(),
		NodeThresholdRatio: cfg.Batch.NodeThresholdRatio,
		SpoolDir:           cfg.Output.SpoolDir,
		MTLSCert:           cfg.MTLS.Cert,
		MTLSKey:            cfg.MTLS.Key,
		MTLSCA:             cfg.MTLS.CA,
		RetryMaxElapsed:    cfg.Retry.MaxElapsedTime.Unwrap(),
		StdoutWriter:       stdoutWriter,
		Store:              batchStore,
	})
	if err != nil {
		log.Fatal("emitter init", "err", err)
	}
	go emitter.Run(ctx, emitterBatches, log)

	// Initialize event hub (fans out batches to emitter + dashboard SSE)
	batches := make(chan emit.Batch, cfg.Batch.ChannelBuffer)
	eventHub := hub.New(batches, emitterBatches, cfg.Dashboard.HistorySize)
	go eventHub.Run(ctx, log)

	// Register dashboard routes and start metrics server
	if cfg.Dashboard.Enabled {
		dashHandler := dashboard.NewHandler(eventHub, log)
		dashHandler.RegisterRoutes(mux)
		log.Info("dashboard enabled", "url", "http://localhost"+cfg.Telemetry.MetricsAddr)
	}
	if cfg.Telemetry.MetricsAddr != "" {
		go metrics.ServeWithHealth(cfg.Telemetry.MetricsAddr, mux, log)
		log.Info("metrics and health server started", "addr", cfg.Telemetry.MetricsAddr)
	}

	// Initialize rate limiter
	ratelim := rate.New(cfg.Crawling.RatePerHost, cfg.Crawling.RateBurst, 10000, 5*time.Minute, 1*time.Hour)

	// Initialize HTTP client with circuit breaker
	hc := httpclient.NewResilientClient(
		httpclient.ClientConfig{
			Timeout:               cfg.HTTPClient.Timeout.Unwrap(),
			ResponseHeaderTimeout: cfg.HTTPClient.ResponseHeaderTimeout.Unwrap(),
			IdleConnTimeout:       cfg.HTTPClient.IdleConnTimeout.Unwrap(),
			MaxIdleConns:          cfg.HTTPClient.MaxIdleConns,
			MaxConnsPerHost:       cfg.HTTPClient.MaxConnsPerHost,
		},
		circuitbreaker.Config{
			MaxRequests:  cfg.CircuitBreaker.MaxRequests,
			Interval:     cfg.CircuitBreaker.Interval.Unwrap(),
			Timeout:      cfg.CircuitBreaker.Timeout.Unwrap(),
			Threshold:    cfg.CircuitBreaker.Threshold,
			FailureRatio: cfg.CircuitBreaker.FailureRatio,
		},
	)

	// Initialize robots cache with a plain HTTP client
	robotsClient := &http.Client{Timeout: cfg.HTTPClient.Timeout.Unwrap()}
	rob := robots.NewCache(robotsClient, cfg.UA, cfg.Robots.CacheSize, cfg.Robots.CacheTTL.Unwrap())

	// Initialize task queue and discovery sink
	tasks := make(chan string, 8192)
	var sink discover.Sink

	if cfg.Redis.QueueAddr != "" {
		log.Info("redis queue enabled", "addr", cfg.Redis.QueueAddr, "key", cfg.Redis.QueueKey)
		q, err := queue.NewRedis(cfg.Redis.QueueAddr, cfg.Redis.QueueKey, cfg.Redis.QueueLeaseTTL.Unwrap())
		if err != nil {
			log.Fatal("redis queue init", "err", err)
		}
		if cfg.Crawling.Continuous {
			sink = discover.NewRedisSink(q, d, int64(cfg.Crawling.MaxDomains))
			log.Info("continuous mode enabled (redis)", "max_domains", cfg.Crawling.MaxDomains)
		} else {
			sink = discover.NoopSink{}
		}
		go func() {
			defer close(tasks)
			for {
				select {
				case <-ctx.Done():
					return
				default:
					host, ack, err := q.Lease(ctx)
					if err != nil {
						continue
					}
					if host == "" {
						continue
					}
					tasks <- host
					_ = ack()
				}
			}
		}()
	} else {
		if cfg.Crawling.Continuous {
			// In continuous mode, discovered domains feed back into tasks
			discoveryCh := make(chan string, 8192)
			sink = discover.NewChannelSink(discoveryCh, d, int64(cfg.Crawling.MaxDomains))
			log.Info("continuous mode enabled (in-memory)", "max_domains", cfg.Crawling.MaxDomains)

			go func() {
				defer close(tasks)
				// First, read all seed domains from file
				sc := bufio.NewScanner(f)
				sc.Buffer(make([]byte, 0, 1024), 1024*1024)
				for sc.Scan() {
					line := strings.TrimSpace(sc.Text())
					if line == "" || strings.HasPrefix(line, "#") {
						continue
					}
					line = strings.ToLower(strings.TrimSuffix(line, "."))
					tasks <- line
				}
				// Then drain discovered domains until context is cancelled
				for {
					select {
					case h := <-discoveryCh:
						tasks <- h
					case <-ctx.Done():
						return
					}
				}
			}()
		} else {
			sink = discover.NoopSink{}
			go func() {
				defer close(tasks)
				sc := bufio.NewScanner(f)
				sc.Buffer(make([]byte, 0, 1024), 1024*1024)
				for sc.Scan() {
					line := strings.TrimSpace(sc.Text())
					if line == "" || strings.HasPrefix(line, "#") {
						continue
					}
					line = strings.ToLower(strings.TrimSuffix(line, "."))
					tasks <- line
				}
			}()
		}
	}

	// Log configuration
	log.Info("starting spyder",
		"probe", cfg.Probe,
		"run", cfg.Run,
		"concurrency", cfg.Crawling.Concurrency,
		"continuous", cfg.Crawling.Continuous,
		"exclude_tlds", cfg.ExcludeTLDs,
		"config_file", configFile,
	)

	// Mark service as ready
	healthHandler.SetReady(true)
	log.Info("service marked as ready")

	// Build probe
	probeCfg := probe.ProbeConfig{
		UA:           cfg.UA,
		ProbeID:      cfg.Probe,
		RunID:        cfg.Run,
		ExcludeTLDs:  cfg.ExcludeTLDs,
		HTTPTimeout:  cfg.Crawling.HTTPTimeout.Unwrap(),
		TLSTimeout:   cfg.Crawling.TLSTimeout.Unwrap(),
		BodyMaxBytes: cfg.Crawling.BodyMaxBytes,
	}
	p := probe.New(probeCfg, d, sink, batches, ratelim, hc, rob, log)

	// Create dynamic worker pool
	wp := pool.New(ctx, tasks, func(ctx context.Context, host string) {
		p.CrawlOne(ctx, host)
		metrics.TasksTotal.WithLabelValues("ok").Inc()
	})
	wp.Scale(cfg.Crawling.Concurrency)

	// Create RuntimeConfig for hot reload
	runtimeCfg := config.NewRuntimeConfig(*cfg)

	// Create API key store
	keyStore := api.NewKeyStore(cfg.API.Keys)

	// Create query store
	queryStore := api.NewMemoryQueryStore(eventHub)

	// Create API server
	apiServer := api.NewServer(
		runtimeCfg, wp, eventHub, emitter, d, sink, ratelim, hc, rob,
		keyStore, queryStore, healthHandler, log, cancel, tasks,
	)
	apiServer.RegisterRoutes(mux)

	// Register OnChange callbacks for runtime config updates
	runtimeCfg.OnChange(func(old, new_ config.Config) {
		if old.UA != new_.UA {
			p.SetUA(new_.UA)
			rob.SetUA(new_.UA)
			log.Info("updated user-agent", "ua", new_.UA)
		}
		if len(old.ExcludeTLDs) != len(new_.ExcludeTLDs) || fmt.Sprintf("%v", old.ExcludeTLDs) != fmt.Sprintf("%v", new_.ExcludeTLDs) {
			p.SetExcludeTLDs(new_.ExcludeTLDs)
			log.Info("updated exclude TLDs", "tlds", new_.ExcludeTLDs)
		}
		if old.Crawling.RatePerHost != new_.Crawling.RatePerHost || old.Crawling.RateBurst != new_.Crawling.RateBurst {
			ratelim.SetRate(new_.Crawling.RatePerHost, new_.Crawling.RateBurst)
			log.Info("updated rate limits", "rate", new_.Crawling.RatePerHost, "burst", new_.Crawling.RateBurst)
		}
		if old.API.RateLimit != new_.API.RateLimit || old.API.RateBurst != new_.API.RateBurst {
			apiServer.SetAPIRate(new_.API.RateLimit, new_.API.RateBurst)
			log.Info("updated API rate limits", "rate", new_.API.RateLimit, "burst", new_.API.RateBurst)
		}
	})

	// SIGHUP handler for config file reload
	if configFile != "" {
		sighup := make(chan os.Signal, 1)
		signal.Notify(sighup, syscall.SIGHUP)
		go func() {
			for range sighup {
				log.Info("received SIGHUP, reloading config", "file", configFile)
				if err := runtimeCfg.ReloadFromFile(configFile); err != nil {
					log.Error("config reload failed", "err", err)
				} else {
					// Reload API keys
					newCfg := runtimeCfg.Get()
					keyStore.Reload(newCfg.API.Keys)
					log.Info("config reloaded successfully")
				}
			}
		}()
	}

	// Wait for workers to finish
	wp.Wait()

	// Wait for emitter to drain
	emitter.Drain(log)
	log.Info("shutdown complete")
}
