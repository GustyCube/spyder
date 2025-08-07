# SPYDER Probe Pro Documentation

**SPYDER** (System for Probing and Yielding DNS-based Entity Relations) is a distributed, production-ready probe for mapping inter-domain relationships across the internet.

## Quick Navigation

### Getting Started
- [Quick Start Guide](guide/getting-started.md) - Get up and running quickly
- [Installation Guide](guide/installation.md) - Detailed installation instructions
- [CLI Reference](config/cli.md) - Complete command-line options

### Architecture & Design
- [System Overview](architecture/overview.md) - High-level architecture
- [Data Model](architecture/data-model.md) - Nodes and edges specification
- [Discovery Pipeline](architecture/pipeline.md) - How SPYDER processes domains

### Configuration
- [Command Line Interface](config/cli.md) - All CLI flags and options
- [Environment Variables](config/environment.md) - Runtime configuration
- [Redis Configuration](config/redis.md) - Distributed operations setup
- [Security Configuration](config/security.md) - mTLS and hardening

### Operations
- [Single Node Deployment](ops/single-node.md) - Production single-server setup
- [Monitoring & Observability](observability/metrics.md) - Prometheus metrics and monitoring

### Development
- [Building from Source](dev/building.md) - Build and compilation guide

### Use Cases
- [Security Research](use-cases/security.md) - Cybersecurity applications

## Core Features

- **Distributed Architecture**: Scales from single nodes to large clusters
- **Policy Awareness**: Respects robots.txt and excludes sensitive TLDs
- **Production Ready**: Structured logging, metrics, health checks, graceful shutdown
- **Data Reliability**: Batch emitter with retries and on-disk spooling
- **Security**: mTLS support for secure ingestion
- **Observability**: Prometheus metrics, OpenTelemetry tracing, structured logs

## Data Discovery

SPYDER discovers and maps relationships between:

- **Domain Names** - Web hosts and their apex domains
- **IP Addresses** - Resolved endpoints and hosting infrastructure  
- **TLS Certificates** - Certificate metadata and SPKI fingerprints

### Edge Types

- `RESOLVES_TO` - Domain → IP address (A/AAAA records)
- `USES_NS` - Domain → Nameserver (NS records)
- `ALIAS_OF` - Domain → CNAME target
- `USES_MX` - Domain → Mail exchanger (MX records)
- `LINKS_TO` - Domain → External domains (from HTML links)
- `USES_CERT` - Domain → TLS certificate (SPKI hash)

## Quick Start

```bash
# Basic usage
echo -e "example.com\ngoogle.com" > domains.txt
./bin/spyder -domains=domains.txt

# With metrics and Redis deduplication
REDIS_ADDR=127.0.0.1:6379 ./bin/spyder \
  -domains=domains.txt \
  -metrics_addr=:9090 \
  -probe=my-probe-1
```

## Production Deployment

```bash
# Production configuration
./bin/spyder \
  -domains=/opt/spyder/domains.txt \
  -ingest=https://ingest.company.com/v1/batch \
  -probe=prod-us-west-1 \
  -concurrency=256 \
  -metrics_addr=127.0.0.1:9090 \
  -mtls_cert=/etc/spyder/client.crt \
  -mtls_key=/etc/spyder/client.key
```

## Development

```bash
# Build from source
git clone https://github.com/gustycube/spyder-probe.git
cd spyder-probe
make build

# Run development server  
make run

# Start documentation site
make docs
```

## Community & Support

- **Repository**: [github.com/gustycube/spyder-probe](https://github.com/gustycube/spyder-probe)
- **Issues**: Report bugs and feature requests on GitHub
- **Documentation**: This comprehensive documentation site
- **License**: MIT License

## Architecture Overview

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Domains   │───▶│   Workers   │───▶│   Output    │
│   (Queue)   │    │   (Pool)    │    │  (Batches)  │
└─────────────┘    └─────────────┘    └─────────────┘
                           │
                    ┌─────────────┐
                    │ Components  │
                    │             │
                    │ • DNS       │
                    │ • HTTP      │
                    │ • TLS       │
                    │ • Extract   │
                    │ • Robots    │
                    │ • Rate Lim  │
                    │ • Dedup     │
                    └─────────────┘
```

SPYDER processes domains through a pipeline that performs DNS resolution, respects robots.txt policies, fetches HTTP content, analyzes TLS certificates, and extracts external links to map inter-domain relationships.

---

**Status**: Production Ready  
**Documentation Site**: Powered by VitePress  
**Development**: `npm install && npm run docs:dev` to serve locally