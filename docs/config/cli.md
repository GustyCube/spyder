# Command Line Interface Reference

This document provides comprehensive information about SPYDER's command-line flags and configuration options.

## Required Parameters

### `-domains` (Required)

Path to newline-separated file containing domains to probe.

```bash
-domains=configs/domains.txt
```

**File Format:**
```
# Comments start with #
example.com
google.com
github.com

# Blank lines are ignored
facebook.com
twitter.com
```

**Input Processing:**
- Comments (`#`) and blank lines are ignored
- Domain names are normalized (lowercase, trailing dots removed)
- Duplicates within the file are processed only once
- Maximum line length: 1MB

## Core Configuration

### `-ingest`

HTTP(S) endpoint for batch ingestion. If empty, outputs JSON to stdout.

```bash
# Send to ingest API
-ingest=https://ingest.example.com/v1/batch

# Output to stdout (default)
-ingest=""
```

**API Requirements:**
- Must accept POST requests
- Content-Type: `application/json`
- Returns 2xx status codes for success
- Should handle batch sizes up to 50MB

### `-probe`

Unique identifier for this probe instance.

```bash
-probe=us-west-1a
-probe=production-node-01
-probe=dev-local
```

**Default:** `local-1`

**Usage:**
- Used in metrics labels
- Included in all emitted data
- Should be unique across probe instances
- Helpful for debugging and monitoring

### `-run`

Identifier for the current run/session.

```bash
-run=scan-20240101
-run=daily-maintenance
```

**Default:** `run-{unix-timestamp}`

**Usage:**
- Groups related discoveries
- Useful for batch processing analysis
- Included in all emitted data

## Performance Tuning

### `-concurrency`

Number of concurrent worker goroutines.

```bash
-concurrency=512    # High throughput
-concurrency=64     # Conservative
-concurrency=1024   # Maximum throughput
```

**Default:** `256`

**Considerations:**
- Higher values increase memory usage
- Limited by system file descriptors
- Network bandwidth becomes bottleneck at high values
- Optimal value depends on target domains' response times

### `-batch_max_edges`

Maximum number of edges per batch before forced flush.

```bash
-batch_max_edges=50000   # Large batches
-batch_max_edges=1000    # Small batches
```

**Default:** `10000`

**Trade-offs:**
- Larger batches: Better throughput, higher memory usage
- Smaller batches: Lower latency, more API calls
- Consider ingest API limits

### `-batch_flush_sec`

Time interval (seconds) for batch flushing.

```bash
-batch_flush_sec=1    # Low latency
-batch_flush_sec=10   # High throughput
```

**Default:** `2`

**Behavior:**
- Forces batch emission after specified seconds
- Prevents indefinite data accumulation
- Balances latency vs. throughput

## Content Processing

### `-ua`

User-Agent string for HTTP requests.

```bash
-ua="SPYDER-Probe/2.0 (+https://example.com/about)"
-ua="Research-Bot/1.0"
```

**Default:** `SPYDERProbe/1.0 (+https://github.com/gustycube/spyder)`

**Best Practices:**
- Include contact information
- Identify purpose clearly
- Follow RFC 7231 format
- Some sites block generic user agents

### `-exclude_tlds`

Comma-separated list of top-level domains to skip crawling.

```bash
-exclude_tlds=gov,mil,int,edu
-exclude_tlds=""  # No exclusions
```

**Default:** `gov,mil,int`

**Notes:**
- DNS resolution still performed
- Only HTTP crawling is skipped
- Case-insensitive matching
- Subdomain matching (`.gov` matches `www.example.gov`)

## Reliability & Storage

### `-spool_dir`

Directory for storing failed batch files.

```bash
-spool_dir=/var/spool/spyder
-spool_dir=./failed-batches
```

**Default:** `spool`

**Behavior:**
- Created automatically if not exists
- Failed batches stored as timestamped JSON files
- Automatic retry on restart
- Files cleaned up after successful transmission

## Security & mTLS

### `-mtls_cert`

Path to client certificate file (PEM format) for mTLS authentication.

```bash
-mtls_cert=/etc/spyder/client.crt
```

**Requirements:**
- PEM-encoded X.509 certificate
- Must correspond to `-mtls_key`
- Used for ingest API authentication

### `-mtls_key`

Path to client private key file (PEM format) for mTLS.

```bash
-mtls_key=/etc/spyder/client.key
```

**Requirements:**
- PEM-encoded private key
- Must correspond to `-mtls_cert`
- Should be readable only by spyder process

### `-mtls_ca`

Path to Certificate Authority bundle (PEM format).

```bash
-mtls_ca=/etc/spyder/ca-bundle.crt
```

**Usage:**
- Validates server certificates
- Used when system CA bundle insufficient
- Multiple CA certificates supported

## Observability

### `-metrics_addr`

Listen address for Prometheus metrics endpoint.

```bash
-metrics_addr=:9090          # All interfaces
-metrics_addr=127.0.0.1:9090 # Localhost only
-metrics_addr=""             # Disable metrics
```

**Default:** `:9090`

**Endpoint:** `http://{metrics_addr}/metrics`

**Security:**
- Consider firewall rules
- May expose sensitive information
- Use localhost binding for security

### `-otel_endpoint`

OpenTelemetry OTLP HTTP endpoint for distributed tracing.

```bash
-otel_endpoint=http://jaeger:4318
-otel_endpoint=https://otel-collector.example.com:4318
```

**Default:** `""` (disabled)

**Protocol:** OTLP over HTTP
**Port:** Typically 4318 for HTTP, 4317 for gRPC

### `-otel_insecure`

Use insecure HTTP for OpenTelemetry (no TLS).

```bash
-otel_insecure=true   # HTTP
-otel_insecure=false  # HTTPS
```

**Default:** `true`

**Production Recommendation:** Use `false` with proper TLS

### `-otel_service`

Service name for OpenTelemetry traces.

```bash
-otel_service=spyder-probe-prod
-otel_service=spyder-dev
```

**Default:** `spyder-probe`

## Complete Example Configurations

### Development Setup

```bash
./bin/spyder \
  -domains=test-domains.txt \
  -concurrency=32 \
  -metrics_addr=:9090 \
  -probe=dev-local \
  -run=test-$(date +%s)
```

### Production Single Node

```bash
./bin/spyder \
  -domains=/opt/spyder/domains.txt \
  -ingest=https://ingest.production.com/v1/batch \
  -probe=prod-us-west-1 \
  -concurrency=256 \
  -metrics_addr=127.0.0.1:9090 \
  -spool_dir=/opt/spyder/spool \
  -ua="CompanySpyder/1.0 (+https://company.com/contact)" \
  -mtls_cert=/opt/spyder/certs/client.crt \
  -mtls_key=/opt/spyder/certs/client.key \
  -mtls_ca=/opt/spyder/certs/ca.crt
```

### High-Throughput Configuration

```bash
./bin/spyder \
  -domains=large-domain-list.txt \
  -ingest=https://high-throughput-ingest.com/v1/batch \
  -probe=htp-node-01 \
  -concurrency=1024 \
  -batch_max_edges=50000 \
  -batch_flush_sec=1 \
  -metrics_addr=:9090
```

### Distributed Setup with Redis

```bash
# Environment variables
export REDIS_ADDR=redis.cluster.local:6379
export REDIS_QUEUE_ADDR=redis.cluster.local:6379
export REDIS_QUEUE_KEY=spyder:production:queue

./bin/spyder \
  -ingest=https://distributed-ingest.com/v1/batch \
  -probe=dist-worker-$(hostname) \
  -concurrency=256 \
  -metrics_addr=:9090 \
  -otel_endpoint=http://jaeger.monitoring.svc.cluster.local:4318 \
  -otel_service=spyder-production
```

## Validation and Testing

### Configuration Validation

Test configuration without processing:

```bash
# Test with single domain
echo "example.com" > test.txt
./bin/spyder -domains=test.txt -concurrency=1

# Test metrics endpoint
curl http://localhost:9090/metrics

# Test mTLS configuration
openssl s_client -connect ingest.example.com:443 \
  -cert client.crt -key client.key -CAfile ca.crt
```

### Performance Testing

```bash
# Memory usage test
echo "$(seq 1 1000 | sed 's/^/test/' | sed 's/$/.example.com/')" > perf-test.txt
/usr/bin/time -v ./bin/spyder -domains=perf-test.txt -concurrency=64

# Throughput test
time ./bin/spyder -domains=large-list.txt -ingest=""  | wc -l
```

## Environment Variable Equivalents

Some flags can be set via environment variables:

```bash
# Redis configuration
export REDIS_ADDR=127.0.0.1:6379

# Queue configuration  
export REDIS_QUEUE_ADDR=127.0.0.1:6379
export REDIS_QUEUE_KEY=spyder:queue

# OpenTelemetry
export OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4318
export OTEL_EXPORTER_OTLP_INSECURE=true
```

## Common Flag Combinations

### Security Research
```bash
-exclude_tlds=gov,mil,int,edu
-ua="SecurityResearch/1.0 (+https://university.edu/security)"
-concurrency=64
```

### Infrastructure Mapping
```bash
-exclude_tlds=gov,mil
-concurrency=512
-batch_max_edges=25000
```

### Development/Testing
```bash
-concurrency=16
-metrics_addr=127.0.0.1:9090
-otel_insecure=true
```

## Troubleshooting Flags

### Debug Single Domain
```bash
echo "problem-domain.com" > debug.txt
./bin/spyder -domains=debug.txt -concurrency=1
```

### Reduced Resource Usage
```bash
./bin/spyder -domains=domains.txt \
  -concurrency=32 \
  -batch_max_edges=1000 \
  -batch_flush_sec=5
```

### Maximum Reliability
```bash
./bin/spyder -domains=domains.txt \
  -spool_dir=/persistent/storage/spool \
  -batch_flush_sec=1 \
  -mtls_cert=client.crt \
  -mtls_key=client.key
```