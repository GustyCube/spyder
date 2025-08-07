# SPYDER Architecture Overview

SPYDER (System for Probing and Yielding DNS-based Entity Relations) is designed as a scalable, distributed system for mapping inter-domain relationships across the internet.

## High-Level Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Domain List   │    │   Redis Queue   │    │   Ingest API    │
│   (File/Queue)  │───▶│   (Optional)    │───▶│   (HTTP/HTTPS)  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                        │                        │
         ▼                        ▼                        ▼
┌─────────────────────────────────────────────────────────────────┐
│                    SPYDER Probe Engine                         │
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │   Worker    │  │   Worker    │  │   Worker    │   ... (N)   │
│  │   Pool      │  │   Pool      │  │   Pool      │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
│           │               │               │                     │
│           ▼               ▼               ▼                     │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │ DNS Resolver│  │ HTTP Client │  │ TLS Analyzer│             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
└─────────────────────────────────────────────────────────────────┘
         │                        │                        │
         ▼                        ▼                        ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Prometheus    │    │   Redis Dedup   │    │   Batch Emitter │
│   Metrics       │    │   (Optional)    │    │   + Spooling    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Core Components

### 1. Probe Engine (`internal/probe`)

The central orchestrator that coordinates domain probing:

- **Worker Pool Management**: Manages concurrent workers for parallel processing
- **Task Distribution**: Distributes domain probing tasks across workers
- **Policy Enforcement**: Applies robots.txt and TLD exclusion policies
- **Rate Limiting**: Enforces per-host rate limits to be respectful

**Key Features:**
- Configurable concurrency (default: 256 workers)
- Graceful shutdown with proper resource cleanup
- OpenTelemetry tracing integration
- Prometheus metrics collection

### 2. DNS Resolution (`internal/dns`)

Performs comprehensive DNS lookups:

- **A/AAAA Records**: IPv4 and IPv6 address resolution
- **NS Records**: Nameserver discovery
- **CNAME Records**: Canonical name mapping
- **MX Records**: Mail exchanger identification
- **TXT Records**: Text record collection (future use)

**Implementation:**
- Uses Go's `net.DefaultResolver`
- Context-aware with timeout handling
- Concurrent resolution for multiple record types

### 3. HTTP Client (`internal/httpclient`)

Optimized HTTP client for web content fetching:

- **Connection Pooling**: Reuses connections for efficiency
- **Timeout Management**: Configurable response timeouts
- **TLS Configuration**: Secure HTTPS connections
- **Content Limiting**: Restricts response body size (512KB max)

**Configuration:**
- Max idle connections: 1024
- Max connections per host: 128
- Response timeout: 10 seconds
- Overall timeout: 15 seconds

### 4. Link Extraction (`internal/extract`)

Parses HTML content to discover external relationships:

- **HTML Parsing**: Uses `golang.org/x/net/html` tokenizer
- **Link Discovery**: Extracts `href` and `src` attributes
- **External Filtering**: Identifies cross-domain relationships
- **Apex Domain Calculation**: Uses public suffix list

**Extraction Sources:**
- `<a href="...">` - Hyperlinks
- `<link href="...">` - Stylesheets and resources
- `<script src="...">` - JavaScript resources
- `<img src="...">` - Images
- `<iframe src="...">` - Embedded content

### 5. TLS Analysis (`internal/tlsinfo`)

Extracts TLS certificate metadata:

- **Certificate Chain**: Analyzes server certificates
- **SPKI Fingerprinting**: SHA-256 hash of Subject Public Key Info
- **Validity Periods**: Not-before and not-after timestamps
- **Subject/Issuer**: Common name extraction

**Security:**
- Proper certificate chain validation
- Timeout protection (8 seconds)
- Secure TLS configuration

### 6. Rate Limiting (`internal/rate`)

Per-host token bucket rate limiting:

- **Token Bucket Algorithm**: Using `golang.org/x/time/rate`
- **Per-Host Limiting**: Independent limits for each domain
- **Configurable Rates**: Adjustable requests per second
- **Burst Support**: Allows burst traffic within limits

**Default Configuration:**
- 1.0 requests per second per host
- Burst size: 1 request

### 7. Robots.txt Handling (`internal/robots`)

Respectful crawling with robots.txt compliance:

- **LRU Cache**: 4096 entries with 24-hour TTL
- **Fallback Logic**: HTTPS first, then HTTP
- **User-Agent Matching**: Respects specific and wildcard rules
- **Default Allow**: Assumes allowed if robots.txt unavailable

### 8. Deduplication (`internal/dedup`)

Prevents duplicate data collection:

- **Memory Backend**: In-process hash set (default)
- **Redis Backend**: Distributed deduplication across probes
- **Key Generation**: Consistent hashing for nodes and edges
- **TTL Support**: Automatic expiration for Redis backend

### 9. Batch Emitter (`internal/emit`)

Efficient data output with reliability:

- **Batch Aggregation**: Combines multiple discoveries
- **Configurable Limits**: Max edges per batch (10,000)
- **Timed Flushing**: Regular batch emission (2 seconds)
- **Retry Logic**: Exponential backoff for failed requests
- **Spooling**: On-disk storage for failed batches
- **mTLS Support**: Client certificate authentication

### 10. Queue System (`internal/queue`)

Distributed task distribution:

- **Redis-based**: Uses Redis lists for work distribution
- **Atomic Operations**: BRPOPLPUSH for atomic task leasing
- **TTL Management**: Lease timeout handling
- **Processing Tracking**: Separate processing queue

## Data Flow

### Single-Node Operation

1. **Input**: Read domains from file
2. **Distribution**: Send domains to worker pool
3. **Processing**: Each worker performs:
   - DNS resolution
   - Robots.txt check
   - Rate limiting
   - HTTP content fetch
   - TLS certificate analysis
   - Link extraction
4. **Deduplication**: Check for previously seen data
5. **Batch Formation**: Aggregate results into batches
6. **Output**: Emit to stdout or HTTP endpoint

### Distributed Operation

1. **Queue Population**: Seed Redis queue with domains
2. **Lease Management**: Workers lease tasks from queue
3. **Processing**: Same as single-node
4. **Distributed Dedup**: Use Redis for cross-probe deduplication
5. **Result Aggregation**: All probes send to common ingest API

## Scalability Considerations

### Vertical Scaling

- **Worker Concurrency**: Increase `-concurrency` parameter
- **Memory**: More workers require more memory
- **CPU**: Processing is CPU-bound for parsing operations
- **Network**: Higher concurrency increases network usage

### Horizontal Scaling

- **Multiple Probes**: Run multiple SPYDER instances
- **Redis Queue**: Shared work distribution
- **Redis Dedup**: Prevent duplicate work across probes
- **Load Balancing**: Distribute probe workload

### Performance Tuning

```bash
# High-throughput configuration
./bin/spyder \
  -domains=large-list.txt \
  -concurrency=512 \
  -batch_max_edges=50000 \
  -batch_flush_sec=1 \
  -ingest=https://fast-ingest.example.com/v1/batch
```

## Security Architecture

### Input Validation
- Domain name sanitization
- URL parsing validation
- Content-Type verification

### Network Security
- mTLS for ingest API communication
- Secure TLS for all HTTPS requests
- DNS over HTTPS support (configurable)

### Resource Protection
- Memory limits for HTTP responses
- Connection timeouts
- Rate limiting to prevent abuse

### Data Privacy
- No sensitive content storage
- Configurable TLD exclusions
- Robots.txt compliance

## Monitoring Integration

### Metrics Collection
- Prometheus metrics endpoint
- Counter and gauge metrics
- Custom labels for filtering

### Distributed Tracing
- OpenTelemetry integration
- Span creation for major operations
- Context propagation

### Structured Logging
- JSON-formatted logs
- Configurable log levels
- Error context preservation

## Error Handling

### Graceful Degradation
- Continue processing on individual failures
- Skip unreachable hosts
- Partial result collection

### Retry Logic
- Exponential backoff for transient failures
- Configurable retry attempts
- Circuit breaker patterns

### Recovery Mechanisms
- Batch spooling for failed ingestion
- Automatic spool replay on restart
- Graceful shutdown with data preservation

## Configuration Management

### Environment Variables
- Redis connection strings
- Feature toggles
- External service endpoints

### Command-Line Flags
- Runtime parameters
- Performance tuning
- Output configuration

### Policy Configuration
- Excluded TLD lists
- User-Agent customization
- Rate limiting parameters

This architecture enables SPYDER to scale from single-machine development environments to large-scale distributed deployments while maintaining reliability, security, and observability.