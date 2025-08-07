# Operations Guide

## Configuration

SPYDER can be configured via command-line flags or environment variables:

### Core Flags
- `-domains`: Path to newline-separated domain list (required)
- `-ingest`: HTTP(S) ingestion endpoint (optional - prints to stdout if empty)
- `-probe`: Probe identifier (default: "local-1")
- `-run`: Run identifier (default: auto-generated timestamp)
- `-concurrency`: Worker pool size (default: 256)

### Rate Limiting
- `-ua`: User-Agent string for HTTP requests
- `-exclude_tlds`: Comma-separated TLDs to skip (default: "gov,mil,int")

### Batch Processing
- `-batch_max_edges`: Max edges per batch before flush (default: 10000)
- `-batch_flush_sec`: Timer-based flush interval in seconds (default: 2)
- `-spool_dir`: Directory for failed batch files (default: "spool")

### Security
- `-mtls_cert`: Client certificate for mTLS authentication
- `-mtls_key`: Client private key for mTLS authentication  
- `-mtls_ca`: CA bundle for mTLS validation

### Environment Variables
- `REDIS_ADDR`: Redis server address for deduplication (optional)
- `REDIS_QUEUE_ADDR`: Redis server for distributed queue (optional)
- `REDIS_QUEUE_KEY`: Queue key name (default: "spyder:queue")

## Deployment Patterns

### Single Node
```bash
# Local development
./bin/spyder -domains=domains.txt

# With metrics and Redis dedupe
REDIS_ADDR=127.0.0.1:6379 ./bin/spyder \
  -domains=domains.txt \
  -metrics_addr=:9090
```

### Distributed Queue
```bash
# Start queue consumer
REDIS_QUEUE_ADDR=127.0.0.1:6379 ./bin/spyder \
  -metrics_addr=:9090 \
  -probe=worker-1

# Seed the queue
./bin/seed -domains=domains.txt -redis=127.0.0.1:6379
```

### Production with Ingestion
```bash
./bin/spyder \
  -domains=domains.txt \
  -ingest=https://ingest.example.com/v1/batch \
  -probe=datacenter-1a \
  -run=scan-$(date +%s) \
  -mtls_cert=/etc/ssl/client.pem \
  -mtls_key=/etc/ssl/client.key \
  -metrics_addr=:9090
```

## Monitoring

### Prometheus Metrics (`:9090/metrics`)
- `spyder_tasks_total{status}`: Task completion counters
- `spyder_edges_total{type}`: Edge discovery by relationship type
- `spyder_robots_blocks_total`: Robots.txt enforcement blocks
- `spyder_http_duration_seconds`: HTTP request latency histogram

### Structured Logging
JSON-formatted logs include:
- `level`: Log severity (info, warn, error)
- `msg`: Human-readable message
- `host`: Target domain being processed
- `probe_id`: Probe identifier
- `run_id`: Run identifier
- `err`: Error details when applicable

### Health Checks
- **Metrics endpoint**: GET `/metrics` returns 200 if healthy
- **Process signals**: Responds to SIGINT/SIGTERM for graceful shutdown
- **Spool monitoring**: Check `spool/` directory for failed batches

## Redis Queue (Distributed Scheduling)

### Queue Setup
```bash
# Enable queue consumption
export REDIS_QUEUE_ADDR=127.0.0.1:6379
export REDIS_QUEUE_KEY=spyder:queue

# Start worker
./bin/spyder -metrics_addr=:9090 -probe=worker-1
```

### Seeding Domains
```bash
# Push domains to queue
./bin/seed -domains=domains.txt -redis=127.0.0.1:6379 -key=spyder:queue
```

### Queue Management
- Items are leased for 120 seconds during processing
- Failed items return to queue automatically
- Use Redis commands to inspect queue state:
  ```bash
  redis-cli LLEN spyder:queue  # Queue length
  redis-cli LRANGE spyder:queue 0 -1  # View items
  ```

## OpenTelemetry

### Configuration
- `-otel_endpoint`: OTLP HTTP endpoint (e.g., "localhost:4318")
- `-otel_insecure`: Use insecure connection (default: true)
- `-otel_service`: Service name (default: "spyder-probe")

### Trace Context
- `CrawlOne` span: Complete domain processing pipeline
- Custom attributes: `probe.id`, `run.id`, `domain`
- Propagates context through DNS, HTTP, and TLS operations

### Integration Example
```bash
# With Jaeger
./bin/spyder \
  -domains=domains.txt \
  -otel_endpoint=localhost:14268 \
  -otel_service=spyder-prod
```

## Troubleshooting

### Common Issues

**High Memory Usage**
- Check deduplication cache size with memory backend
- Consider Redis backend for large-scale deployments
- Monitor worker pool size vs. available memory

**DNS Resolution Failures**  
- Verify network connectivity and DNS servers
- Check for rate limiting from upstream DNS providers
- Review excluded TLD list for unintended filtering

**HTTP Timeouts**
- Default 20-second timeout per HTTP request
- Robots.txt failures don't block crawling (fail-open policy)
- Rate limiting prevents overwhelming target servers

**Batch Delivery Issues**
- Check `spool/` directory for failed batches
- Verify ingestion endpoint availability and authentication
- Review mTLS certificate configuration

### Performance Tuning

**Worker Concurrency**
- Default: 256 workers
- Increase for CPU-bound workloads
- Decrease if overwhelming downstream systems

**Rate Limiting**
- Default: 1 request/second per host
- Adjust in `internal/rate/limiter.go` for different patterns
- Consider target server capacity and politeness

**Batch Sizing**
- Default: 10,000 edges or 5,000 nodes per batch
- Larger batches reduce HTTP overhead
- Smaller batches provide faster feedback

### Log Analysis

**Key Log Patterns**
```bash
# Filter by probe/run
jq '.probe_id == "worker-1" and .run_id == "scan-123"' logs.jsonl

# Error analysis
jq 'select(.level == "error")' logs.jsonl

# Performance metrics
jq 'select(.msg == "task completed") | .duration' logs.jsonl
```