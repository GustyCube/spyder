# Project Arachnet

**SPYDER** (System for Probing and Yielding DNS-based Entity Relations) is a distributed, production-ready internet mapping probe that discovers and analyzes domain relationships while maintaining ethical crawling practices.

## Quick Start

```bash
# Build the probe
go build -o bin/spyder ./cmd/spyder

# Basic domain discovery
echo -e "example.com\ngolang.org" > domains.txt
./bin/spyder -domains=domains.txt

# Production deployment with ingestion
./bin/spyder \
  -domains=domains.txt \
  -ingest=https://your-ingest-endpoint.com/v1/batch \
  -metrics_addr=:9090 \
  -probe=datacenter-1a
```

## Documentation

- **[Architecture Overview](architecture/spyder.md)** - System design and component relationships
- **[Operations Guide](guide/ops.md)** - Deployment, monitoring, and troubleshooting

## Key Features

- **Multi-layer Relationship Discovery**: DNS records, TLS certificates, and web links
- **Ethical Crawling**: robots.txt compliance and rate limiting
- **Production Ready**: Structured logging, Prometheus metrics, OpenTelemetry tracing
- **Scalable Architecture**: Redis-backed queuing and deduplication
- **Reliable Delivery**: Batch processing with retry logic and disk spooling
- **Security**: mTLS support and distroless containers

## Data Model

SPYDER maps the internet as a graph of **nodes** (domains, IPs, certificates) connected by **edges** (relationships like DNS resolution, certificate usage, and hyperlinks). This creates a comprehensive view of internet infrastructure and interconnections suitable for security research, threat intelligence, and network analysis.