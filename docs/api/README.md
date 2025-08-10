# SPYDER API Reference

## Overview

SPYDER provides both command-line and programmatic interfaces for network reconnaissance and mapping operations.

## Command Line Interface

### Basic Usage

```bash
spyder -domains=<file> [options]
```

### Required Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `-domains` | `string` | Path to newline-delimited domain list file |

### Optional Parameters

#### Performance & Concurrency

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `-concurrency` | `int` | `256` | Number of worker goroutines |
| `-batch_max_edges` | `int` | `10000` | Maximum edges per output batch |
| `-batch_flush_sec` | `int` | `2` | Batch flush interval in seconds |

#### Identification

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `-probe` | `string` | `local-1` | Unique probe identifier |
| `-run` | `string` | `run-{timestamp}` | Run identifier for batch correlation |
| `-ua` | `string` | `SPYDERProbe/1.0` | HTTP User-Agent header |

#### Filtering & Exclusions

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `-exclude_tlds` | `string` | `gov,mil,int` | Comma-separated TLD exclusions |

#### Output & Ingestion

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `-ingest` | `string` | `stdout` | HTTP(S) ingest endpoint URL or stdout |
| `-spool_dir` | `string` | `spool/` | Directory for failed batch persistence |

#### Security & mTLS

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `-mtls_cert` | `string` | `""` | Client certificate for mTLS |
| `-mtls_key` | `string` | `""` | Client private key for mTLS |
| `-mtls_ca` | `string` | `""` | CA certificate bundle for mTLS |

#### Observability

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `-metrics_addr` | `string` | `:9090` | Prometheus metrics bind address |

### Environment Variables

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `REDIS_ADDR` | `string` | `""` | Redis server for distributed deduplication |
| `REDIS_QUEUE_ADDR` | `string` | `""` | Redis server for work queue distribution |
| `REDIS_QUEUE_KEY` | `string` | `spyder:queue` | Redis key for work queue |

## Output Format

### JSON Schema

SPYDER outputs data in batches containing nodes and edges discovered during reconnaissance.

#### Batch Structure

```json
{
  "probe_id": "string",
  "run_id": "string",
  "batch_id": "string",
  "timestamp": "ISO8601",
  "nodes_domain": [...],
  "nodes_ip": [...],
  "nodes_cert": [...],
  "edges": [...]
}
```

#### Node Types

##### Domain Node

```json
{
  "host": "string",
  "apex": "string",
  "first_seen": "ISO8601",
  "last_seen": "ISO8601"
}
```

##### IP Node

```json
{
  "ip": "string",
  "first_seen": "ISO8601",
  "last_seen": "ISO8601"
}
```

##### Certificate Node

```json
{
  "sha256": "string",
  "subject": "string",
  "issuer": "string",
  "not_before": "ISO8601",
  "not_after": "ISO8601"
}
```

#### Edge Types

```json
{
  "type": "RESOLVES_TO|HAS_CERT|CERT_FOR_HOST|LINKS_TO",
  "source": "string",
  "target": "string",
  "observed_at": "ISO8601",
  "probe_id": "string",
  "run_id": "string"
}
```

### Edge Type Definitions

| Type | Source | Target | Description |
|------|--------|--------|-------------|
| `RESOLVES_TO` | Domain | IP Address | DNS A/AAAA record resolution |
| `HAS_CERT` | Domain | Certificate SHA256 | TLS certificate association |
| `CERT_FOR_HOST` | Certificate SHA256 | Domain | Certificate SAN/CN entry |
| `LINKS_TO` | Domain | Domain | HTTP link discovered |

## Programmatic Usage

### Go Package Import

```go
import (
    "github.com/gustycube/spyder-probe/internal/probe"
    "github.com/gustycube/spyder-probe/internal/dedup"
    "github.com/gustycube/spyder-probe/internal/dns"
)
```

### Basic Example

```go
package main

import (
    "context"
    "log"
    
    "github.com/gustycube/spyder-probe/internal/probe"
)

func main() {
    config := probe.Config{
        Concurrency: 256,
        ProbeID:     "my-probe",
        RunID:       "run-123",
    }
    
    p := probe.New(config)
    
    domains := []string{"example.com", "github.com"}
    
    ctx := context.Background()
    if err := p.Run(ctx, domains); err != nil {
        log.Fatal(err)
    }
}
```

## Metrics

### Prometheus Metrics Endpoint

SPYDER exposes Prometheus metrics at the configured `-metrics_addr` (default `:9090/metrics`).

### Available Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `spyder_tasks_total` | Counter | Total tasks by status (ok/error) |
| `spyder_edges_total` | Counter | Total edges discovered by type |
| `spyder_active_workers` | Gauge | Currently active worker goroutines |
| `spyder_batch_size` | Histogram | Batch sizes in edges |
| `spyder_batch_duration_seconds` | Histogram | Batch processing duration |
| `spyder_http_request_duration_seconds` | Histogram | HTTP request durations |
| `spyder_dns_lookup_duration_seconds` | Histogram | DNS lookup durations |
| `spyder_tls_handshake_duration_seconds` | Histogram | TLS handshake durations |
| `spyder_redis_operations_total` | Counter | Redis operations by type |
| `spyder_redis_operation_duration_seconds` | Histogram | Redis operation durations |

### Example Queries

```promql
# Processing rate
rate(spyder_tasks_total[5m])

# Error rate
rate(spyder_tasks_total{status="error"}[5m]) / rate(spyder_tasks_total[5m])

# P95 latency
histogram_quantile(0.95, rate(spyder_http_request_duration_seconds_bucket[5m]))

# Active workers
spyder_active_workers
```

## Error Codes

| Code | Description | Resolution |
|------|-------------|------------|
| `ERR_DOMAIN_FILE` | Cannot read domains file | Check file path and permissions |
| `ERR_REDIS_CONN` | Cannot connect to Redis | Verify Redis address and connectivity |
| `ERR_INGEST_FAIL` | Failed to send batch to ingest | Check ingest endpoint and network |
| `ERR_TLS_CONFIG` | Invalid mTLS configuration | Verify certificate files and format |

## Rate Limiting

SPYDER implements per-host rate limiting to respect target systems:

- Default: 10 requests/second per host
- Configurable burst capacity
- Automatic backoff on errors
- Exponential retry with jitter

## Security Considerations

### TLD Exclusions

By default, SPYDER excludes government and military domains:
- `.gov` - Government domains
- `.mil` - Military domains  
- `.int` - International organizations

### robots.txt Compliance

SPYDER respects robots.txt directives:
- Caches robots.txt for 24 hours
- Follows User-Agent specific rules
- Falls back to `*` rules when needed

### mTLS Support

For secure ingestion endpoints:
- Client certificate authentication
- Custom CA bundle support
- TLS 1.2+ enforcement

## Troubleshooting

### Common Issues

#### High Memory Usage

Reduce concurrency or batch size:
```bash
spyder -domains=domains.txt -concurrency=64 -batch_max_edges=5000
```

#### DNS Resolution Failures

Check system DNS configuration:
```bash
nslookup example.com
```

#### Redis Connection Issues

Verify Redis connectivity:
```bash
redis-cli -h <host> -p <port> ping
```

### Debug Logging

Enable debug logging via environment variable:
```bash
LOG_LEVEL=debug spyder -domains=domains.txt
```

## Performance Tuning

### Recommended Settings

| Workload | Concurrency | Batch Size | Flush Interval |
|----------|-------------|------------|----------------|
| Light | 64 | 5000 | 5s |
| Medium | 256 | 10000 | 2s |
| Heavy | 512 | 20000 | 1s |

### Resource Requirements

| Domains | RAM | CPU | Disk |
|---------|-----|-----|------|
| 1K | 128MB | 1 core | 100MB |
| 10K | 256MB | 2 cores | 500MB |
| 100K | 512MB | 4 cores | 2GB |
| 1M | 2GB | 8 cores | 10GB |

## Support

For issues and feature requests, visit:
https://github.com/gustycube/spyder/issues