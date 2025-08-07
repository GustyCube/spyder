package main

import (
	
	"github.com/gustycube/spyder-probe/internal/telemetry"
	"github.com/gustycube/spyder-probe/internal/queue"
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	

	"github.com/gustycube/spyder-probe/internal/dedup"
	"github.com/gustycube/spyder-probe/internal/emit"
	"github.com/gustycube/spyder-probe/internal/logging"
	"github.com/gustycube/spyder-probe/internal/metrics"
	"github.com/gustycube/spyder-probe/internal/probe"
)

func main() {
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

	flag.StringVar(&domainsFile, "domains", "", "path to newline-separated domains")
	flag.StringVar(&ingest, "ingest", "", "ingest endpoint (optional). If empty, prints JSON batches to stdout")
	flag.StringVar(&probeID, "probe", "local-1", "probe id")
	flag.StringVar(&runID, "run", fmt.Sprintf("run-%d", time.Now().Unix()), "run id")
	flag.IntVar(&concurrency, "concurrency", 256, "concurrent workers")
	flag.StringVar(&ua, "ua", "SPYDERProbe/1.0 (+https://github.com/gustycube/project-arachnet)", "user-agent")
	flag.StringVar(&exclude, "exclude_tlds", "gov,mil,int", "comma-separated TLDs to skip crawling")
	flag.StringVar(&metricsAddr, "metrics_addr", ":9090", "metrics listen addr (empty to disable)")
	flag.IntVar(&batchMax, "batch_max_edges", 10000, "max edges per batch before flush")
	flag.IntVar(&batchFlushSec, "batch_flush_sec", 2, "seconds timer to flush a batch")
	flag.StringVar(&spoolDir, "spool_dir", "spool", "spool dir for failed batches")
	flag.StringVar(&mtlsCert, "mtls_cert", "", "client cert (PEM) for mTLS to ingest")
	flag.StringVar(&mtlsKey, "mtls_key", "", "client key (PEM) for mTLS to ingest")
	flag.StringVar(&mtlsCA, "mtls_ca", "", "CA bundle (PEM) for mTLS to ingest")
	flag.StringVar(&otelEndpoint, "otel_endpoint", "", "OTLP HTTP endpoint (host:port)")
	flag.BoolVar(&otelInsecure, "otel_insecure", true, "OTLP insecure (no TLS)")
	flag.StringVar(&otelService, "otel_service", "spyder-probe", "OTEL service.name")
	flag.Parse()

	// Optional Redis queue
	var queueAddr string = os.Getenv("REDIS_QUEUE_ADDR")
	var queueKey string = os.Getenv("REDIS_QUEUE_KEY")
	if queueKey == "" { queueKey = "spyder:queue" }
	leaseTTL := 120 * time.Second

	log := logging.New()
	shutdown, err := telemetry.Init(ctx, otelEndpoint, otelService, otelInsecure)
	if err != nil { log.Warn("otel init failed", "err", err) } else { defer shutdown(context.Background()) }
	defer log.Sync()

	if domainsFile == "" {
		log.Fatal("missing -domains")
	}

	if metricsAddr != "" {
		go metrics.Serve(metricsAddr, log)
		log.Info("metrics server started", "addr", metricsAddr)
	}

	var excluded []string
	for _, t := range strings.Split(exclude, ",") {
		t = strings.TrimSpace(t)
		if t != "" { excluded = append(excluded, t) }
	}

	f, err := os.Open(domainsFile)
	if err != nil {
		log.Fatal("open domains", "err", err)
	}
	defer f.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var d dedup.Interface
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		rd, err := dedup.NewRedis(addr, 24*time.Hour)
		if err != nil { log.Fatal("redis init", "err", err) }
		log.Info("redis dedupe enabled", "addr", addr)
		d = rd
	} else {
		d = dedup.NewMemory()
		log.Info("memory dedupe enabled")
	}

	batches := make(chan emit.Batch, 1024)
	emitter := emit.NewEmitter(ingest, probeID, runID, batchMax, time.Duration(batchFlushSec)*time.Second, spoolDir, mtlsCert, mtlsKey, mtlsCA)
	go emitter.Run(ctx, batches, log)

	tasks := make(chan string, 8192)

	// reader or queue consumer
	if queueAddr != "" {
		log.Info("redis queue enabled", "addr", queueAddr, "key", queueKey)
		q, err := queue.NewRedis(queueAddr, queueKey, leaseTTL)
		if err != nil { log.Fatal("redis queue init", "err", err) }
		go func() {
			defer close(tasks)
			for {
				select {
				case <-ctx.Done(): return
				default:
					host, ack, err := q.Lease(ctx)
					if err != nil { continue }
					if host == "" { continue }
					tasks <- host
					// ack is called in worker after crawl; we pass via a channel? keep simple: ack immediately.
					_ = ack()
				}
			}
		}
		}()
	} else {
		go func() {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 1024), 1024*1024)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") { continue }
			line = strings.ToLower(strings.TrimSuffix(line, "."))
			tasks <- line
		}
		close(tasks)
	}
		}()

	p := probe.New(ua, probeID, runID, excluded, d, batches, log)
	p.Run(ctx, tasks, concurrency)

	// wait emitter to drain
	emitter.Drain(log)
	log.Info("shutdown complete")
}
