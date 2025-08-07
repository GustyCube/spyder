# Getting Started with SPYDER Probe Pro

SPYDER (System for Probing and Yielding DNS-based Entity Relations) is a distributed, policy-aware probe for mapping inter-domain relationships including DNS records, TLS certificate metadata, and external links from root pages.

## Quick Start

### Prerequisites

- Go 1.22 or later
- (Optional) Redis for distributed operation and deduplication
- (Optional) Docker for containerized deployment

### Basic Setup

1. **Download and build SPYDER:**
   ```bash
   git clone https://github.com/gustycube/spyder-probe
   cd spyder-probe
   go mod download
   go build -o bin/spyder ./cmd/spyder
   ```

2. **Create a domains list:**
   ```bash
   echo -e "example.com\ngoogle.com\ngithub.com" > configs/domains.txt
   ```

3. **Run basic probe:**
   ```bash
   ./bin/spyder -domains=configs/domains.txt
   ```

This will probe the specified domains and output JSON batches to stdout containing discovered relationships.

## What SPYDER Discovers

SPYDER creates a graph of relationships between:

- **Domains**: Web hosts and their apex domains
- **IP Addresses**: Resolved IP addresses for domains
- **TLS Certificates**: Certificate metadata and SPKI fingerprints

### Edge Types

- `RESOLVES_TO`: Domain → IP address (A/AAAA records)
- `USES_NS`: Domain → Nameserver (NS records)
- `ALIAS_OF`: Domain → CNAME target
- `USES_MX`: Domain → Mail exchanger (MX records)
- `LINKS_TO`: Domain → External domains (from HTML links)
- `USES_CERT`: Domain → TLS certificate (SPKI hash)

## Configuration Options

### Essential Flags

```bash
# Required: domains to probe
-domains=configs/domains.txt

# Optional: send results to ingest API
-ingest=https://your-api.example.com/v1/batch

# Optional: enable metrics
-metrics_addr=:9090
```

### Advanced Options

```bash
# Probe identification
-probe=us-west-1a           # Probe identifier
-run=run-20240101           # Run identifier

# Performance tuning
-concurrency=256            # Concurrent workers
-batch_max_edges=10000      # Max edges per batch
-batch_flush_sec=2          # Batch flush interval

# Content fetching
-ua="SPYDER/1.0"           # User-Agent string
-exclude_tlds=gov,mil,int   # Skip sensitive TLDs

# Reliability
-spool_dir=spool           # Failed batch storage
```

## Environment Variables

```bash
# Redis for deduplication
export REDIS_ADDR=127.0.0.1:6379

# Redis queue for distributed operation
export REDIS_QUEUE_ADDR=127.0.0.1:6379
export REDIS_QUEUE_KEY=spyder:queue

# OpenTelemetry tracing
export OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4318
```

## Sample Output

SPYDER outputs structured JSON batches:

```json
{
  "probe_id": "local-1",
  "run_id": "run-1704067200",
  "nodes_domain": [
    {
      "host": "example.com",
      "apex": "example.com",
      "first_seen": "2024-01-01T00:00:00Z",
      "last_seen": "2024-01-01T00:00:00Z"
    }
  ],
  "nodes_ip": [
    {
      "ip": "93.184.216.34",
      "first_seen": "2024-01-01T00:00:00Z",
      "last_seen": "2024-01-01T00:00:00Z"
    }
  ],
  "edges": [
    {
      "type": "RESOLVES_TO",
      "source": "example.com",
      "target": "93.184.216.34",
      "observed_at": "2024-01-01T00:00:00Z",
      "probe_id": "local-1",
      "run_id": "run-1704067200"
    }
  ]
}
```

## Next Steps

- [Installation Guide](installation.md) - Detailed installation and deployment
- [Architecture Overview](../architecture/overview.md) - System architecture
- [Configuration Reference](../config/cli.md) - Complete configuration options
- [Operations Guide](../ops/single-node.md) - Production deployment