# SPYDER Architecture

## Overview

**SPYDER** (System for Probing and Yielding DNS-based Entity Relations) is a distributed, production-ready probe designed to map inter-domain relationships across the internet while respecting robots.txt policies and implementing comprehensive rate limiting.

## Core Components

### 1. Probe Engine (`internal/probe/probe.go`)
The central crawler that orchestrates domain discovery and relationship mapping:

- **DNS Resolution**: Discovers A records, NS records, CNAME chains, and MX records
- **HTTP Crawling**: Fetches root pages to extract external domain links
- **TLS Certificate Analysis**: Captures certificate metadata and SPKI fingerprints
- **Policy Enforcement**: Respects robots.txt and excludes sensitive TLDs (gov, mil, int)

### 2. Data Models (`internal/emit/emit.go`)

#### Node Types
- **NodeDomain**: Domain entities with apex domain mapping
- **NodeIP**: IP address entities with first/last seen timestamps
- **NodeCert**: TLS certificate entities with SPKI SHA256 fingerprints

#### Relationship Types
- `RESOLVES_TO`: Domain → IP address mappings
- `USES_NS`: Domain → nameserver relationships
- `ALIAS_OF`: CNAME chain mappings
- `USES_MX`: Mail exchange relationships
- `LINKS_TO`: External domain links found in HTML
- `USES_CERT`: Domain → certificate associations

### 3. Deduplication System (`internal/dedup/`)
Prevents duplicate data collection across probe runs:

- **Memory Backend**: Fast LRU cache for single-process deployments
- **Redis Backend**: Distributed deduplication for multi-node deployments
- **Key Format**: `{type}|{identifier}` for nodes, `edge|{source}|{relation}|{target}` for relationships

### 4. Batch Emitter (`internal/emit/emit.go`)
Handles reliable data delivery with resilience features:

- **Batching**: Accumulates up to 10,000 edges or 5,000 nodes before flushing
- **Timer-based Flushing**: Flushes batches every 2 seconds by default
- **Retry Logic**: Exponential backoff with 30-second max elapsed time
- **Spool Directory**: On-disk backup for failed HTTP deliveries
- **mTLS Support**: Client certificate authentication for secure ingestion

### 5. Rate Limiting (`internal/rate/limiter.go`)
Multi-layer traffic control:

- **Per-Host Token Bucket**: 1 request/second per domain by default
- **Global Concurrency**: 256 concurrent workers by default
- **Respectful Crawling**: Prevents overwhelming target servers

### 6. Policy Engine (`internal/robots/cache.go`)
Implements robots.txt compliance:

- **LRU Cache**: Efficient robots.txt caching with configurable TTL
- **User-Agent Matching**: Respects specific bot directives
- **Default Allow**: Fails open when robots.txt is unavailable

## Architecture Patterns

### Worker Pool
- Fixed-size worker pool processes domains concurrently
- Each worker handles complete domain analysis pipeline
- Graceful shutdown with context cancellation

### Publisher-Subscriber
- Probe workers publish batches to emission channel
- Single emitter goroutine consumes and delivers batches
- Decouples discovery from data delivery

### Circuit Breaker Pattern
- Failed HTTP deliveries trigger spool-to-disk
- Automatic retry of spooled data on next startup
- Prevents data loss during downstream outages

## Observability

### Structured Logging (Zap)
- JSON-formatted logs for machine processing
- Configurable log levels and output destinations
- Request tracing with correlation IDs

### Prometheus Metrics (`internal/metrics/metrics.go`)
- Task completion counters by status
- Edge type distribution counters
- Robots.txt block counters
- HTTP response time histograms

### OpenTelemetry Tracing (`internal/telemetry/otel.go`)
- Distributed tracing support via OTLP
- Span creation for major operations
- Custom attributes for probe and run identification

## Data Flow

1. **Input**: Domain list from file or Redis queue
2. **DNS Resolution**: Parallel A/AAAA, NS, CNAME, MX lookups
3. **Policy Check**: robots.txt validation and TLD exclusion
4. **HTTP Fetch**: Root page retrieval with User-Agent headers
5. **Link Extraction**: HTML parsing for external domain discovery
6. **TLS Analysis**: Certificate fingerprint collection
7. **Deduplication**: Key-based duplicate detection
8. **Batching**: Accumulation until size or time threshold
9. **Delivery**: HTTP POST to ingestion endpoint with retry
10. **Monitoring**: Metrics emission and structured logging

## Production Features

- **Graceful Shutdown**: SIGINT/SIGTERM handling with resource cleanup
- **Configuration**: Flag-based and environment variable configuration
- **Docker Support**: Distroless container images for security
- **systemd Integration**: Service unit files for Linux deployments
- **CI/CD**: Automated linting, testing, and building