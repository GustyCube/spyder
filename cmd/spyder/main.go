package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gustycube/spyder-probe/internal/config"
	"github.com/gustycube/spyder-probe/internal/dedup"
	"github.com/gustycube/spyder-probe/internal/emit"
	"github.com/gustycube/spyder-probe/internal/health"
	"github.com/gustycube/spyder-probe/internal/logging"
	"github.com/gustycube/spyder-probe/internal/metrics"
	"github.com/gustycube/spyder-probe/internal/probe"
	"github.com/gustycube/spyder-probe/internal/queue"
	"github.com/gustycube/spyder-probe/internal/telemetry"
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
	flag.StringVar(&outputFormat, "output_format", "", "output format (json, jsonl, csv)")
	flag.BoolVar(&quiet, "quiet", false, "suppress progress output")
	flag.BoolVar(&verbose, "verbose", false, "verbose logging")
	flag.BoolVar(&progress, "progress", true, "show progress indicators")
	flag.BoolVar(&showVersion, "version", false, "show version and exit")
	
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
	flags := make(map[string]interface{})
	if domainsFile != "" {
		flags["domains"] = domainsFile
	}
	if probeID != "" {
		flags["probe"] = probeID
	}
	if runID != "" {
		flags["run"] = runID
	}
	if ua != "" {
		flags["ua"] = ua
	}
	if concurrency > 0 {
		flags["concurrency"] = concurrency
	}
	if ingest != "" {
		flags["ingest"] = ingest
	}
	if metricsAddr != "" {
		flags["metrics_addr"] = metricsAddr
	}
	if batchMax > 0 {
		flags["batch_max_edges"] = batchMax
	}
	if batchFlushSec > 0 {
		flags["batch_flush_sec"] = batchFlushSec
	}
	if spoolDir != "" {
		flags["spool_dir"] = spoolDir
	}
	if mtlsCert != "" {
		flags["mtls_cert"] = mtlsCert
	}
	if mtlsKey != "" {
		flags["mtls_key"] = mtlsKey
	}
	if mtlsCA != "" {
		flags["mtls_ca"] = mtlsCA
	}
	if otelEndpoint != "" {
		flags["otel_endpoint"] = otelEndpoint
	}
	if otelService != "" {
		flags["otel_service"] = otelService
	}
	if outputFormat != "" {
		flags["output_format"] = outputFormat
	}
	flags["otel_insecure"] = otelInsecure

	cfg.MergeWithFlags(flags)

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

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatal("invalid configuration", "err", err)
	}

	// Initialize telemetry
	shutdown, err := telemetry.Init(ctx, cfg.OTELEndpoint, cfg.OTELService, cfg.OTELInsecure)
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

	// Start metrics and health server
	if cfg.MetricsAddr != "" {
		go metrics.ServeWithHealth(cfg.MetricsAddr, healthHandler, log)
		log.Info("metrics and health server started", "addr", cfg.MetricsAddr)
	}

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
	if cfg.RedisAddr != "" {
		rd, err := dedup.NewRedis(cfg.RedisAddr, 24*time.Hour)
		if err != nil {
			log.Fatal("redis init", "err", err)
		}
		log.Info("redis dedupe enabled", "addr", cfg.RedisAddr)
		d = rd
		
		// Register Redis health check
		redisHealthCheck = func() error {
			// Simple ping check - would need to expose from dedup.Redis
			return nil
		}
		healthHandler.RegisterChecker("redis", health.NewRedisChecker(cfg.RedisAddr, redisHealthCheck))
	} else {
		d = dedup.NewMemory()
		log.Info("memory dedupe enabled")
	}

	// Initialize emitter
	batches := make(chan emit.Batch, 1024)
	emitter := emit.NewEmitter(
		cfg.Ingest,
		cfg.Probe,
		cfg.Run,
		cfg.BatchMaxEdges,
		time.Duration(cfg.BatchFlushSec)*time.Second,
		cfg.SpoolDir,
		cfg.MTLSCert,
		cfg.MTLSKey,
		cfg.MTLSCA,
	)
	go emitter.Run(ctx, batches, log)

	// Initialize task queue
	tasks := make(chan string, 8192)

	// Use Redis queue or file reader
	if cfg.RedisQueueAddr != "" {
		log.Info("redis queue enabled", "addr", cfg.RedisQueueAddr, "key", cfg.RedisQueueKey)
		q, err := queue.NewRedis(cfg.RedisQueueAddr, cfg.RedisQueueKey, 120*time.Second)
		if err != nil {
			log.Fatal("redis queue init", "err", err)
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

	// Log configuration
	log.Info("starting spyder",
		"probe", cfg.Probe,
		"run", cfg.Run,
		"concurrency", cfg.Concurrency,
		"exclude_tlds", cfg.ExcludeTLDs,
		"config_file", configFile,
	)

	// Mark service as ready
	healthHandler.SetReady(true)
	log.Info("service marked as ready")

	// Start probe
	p := probe.New(cfg.UA, cfg.Probe, cfg.Run, cfg.ExcludeTLDs, d, batches, log)
	p.Run(ctx, tasks, cfg.Concurrency)

	// Wait for emitter to drain
	emitter.Drain(log)
	log.Info("shutdown complete")
}