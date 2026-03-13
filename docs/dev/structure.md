# Code Structure

This document describes the directory layout, package responsibilities, data flow, and key abstractions in the SPYDER codebase.

## Directory Layout

```
spyder/
├── cmd/
│   ├── spyder/          # Main probe binary
│   │   └── main.go      # CLI flags, config loading, orchestration
│   └── seed/            # Queue seeder binary
│       └── main.go      # Reads domains file, pushes to Redis queue
├── internal/            # Private packages (not importable by external code)
│   ├── circuitbreaker/  # Per-host circuit breaker
│   ├── config/          # Configuration loading and validation
│   ├── dedup/           # Deduplication (memory + Redis backends)
│   ├── discover/        # Discovery sink system for recursive crawling
│   ├── dns/             # DNS resolution (A, AAAA, NS, CNAME, MX, TXT)
│   ├── emit/            # Batch emitter with retry, spooling, and mTLS
│   ├── extract/         # HTML link extraction and apex domain calculation
│   ├── health/          # Health check HTTP handler
│   ├── httpclient/      # Resilient HTTP client with connection pooling
│   ├── logging/         # Structured logging (zap)
│   ├── metrics/         # Prometheus metrics registration and server
│   ├── output/          # Output formatting (JSON, JSONL, CSV)
│   ├── probe/           # Core probe engine: worker pool and CrawlOne logic
│   ├── queue/           # Redis-based distributed work queue
│   ├── rate/            # Per-host token bucket rate limiter
│   ├── robots/          # robots.txt fetching, caching, and TLD filtering
│   ├── telemetry/       # OpenTelemetry initialization
│   ├── tlsinfo/         # TLS certificate metadata extraction
│   └── ui/              # Terminal UI: progress indicators and log output
├── configs/             # Example configuration files and domain lists
├── docs/                # VitePress documentation site
├── scripts/             # Build and deployment scripts
├── .github/workflows/   # CI/CD pipeline definitions
├── Makefile             # Build, test, lint, docker targets
├── Dockerfile           # Multi-stage container build
├── go.mod               # Go module definition (go 1.23)
└── go.sum               # Dependency checksums
```

## Entry Points

### `cmd/spyder` -- Main Probe

The primary binary. Reads a domain list (from file or Redis queue), runs concurrent workers to probe each domain, and outputs structured JSON batches.

Startup sequence:
1. Parse CLI flags
2. Load config file (if `-config` is set)
3. Apply environment variables (`REDIS_ADDR`, `REDIS_QUEUE_ADDR`, `REDIS_QUEUE_KEY`)
4. Merge CLI flags over config (flags take precedence)
5. Validate final configuration
6. Initialize telemetry, metrics server, health handler
7. Initialize dedup backend (memory or Redis)
8. Initialize emitter (stdout or HTTP ingest endpoint)
9. Initialize task channel and discovery sink
10. Start probe worker pool
11. Drain emitter on shutdown

```bash
# Minimal usage
./bin/spyder -domains=configs/domains.txt

# Full-featured
./bin/spyder \
  -config=configs/spyder.yaml \
  -concurrency=512 \
  -ingest=https://ingest.example.com/v1/batch \
  -continuous \
  -max_domains=100000
```

### `cmd/seed` -- Queue Seeder

A utility that reads a domains file and pushes each domain into a Redis queue. Used in distributed deployments where multiple probes consume from a shared queue.

```bash
./bin/seed -domains=configs/domains.txt -redis=127.0.0.1:6379 -key=spyder:queue
```

## Package Responsibilities

### `internal/probe`

The central orchestrator. `Probe.Run()` starts N worker goroutines that read domain names from a channel and call `CrawlOne()` for each.

`CrawlOne()` performs the complete probing pipeline for a single domain:
1. DNS resolution (A/AAAA, NS, CNAME, MX)
2. Robots.txt policy check
3. TLD exclusion check (skip .gov, .mil, .int)
4. Per-host rate limiting
5. HTTP GET of the root page (with 512KB body limit)
6. HTML link extraction for external domains
7. TLS certificate metadata extraction
8. Deduplication of all nodes and edges
9. Submission of discovered domains to the discovery sink
10. Flushing the batch to the emitter channel

Dependencies: `dns`, `httpclient`, `robots`, `rate`, `extract`, `tlsinfo`, `dedup`, `discover`, `emit`, `metrics`

### `internal/dns`

Performs concurrent DNS lookups using Go's `net.DefaultResolver`. The `ResolveAll()` function resolves a domain and returns:
- IP addresses (A/AAAA combined)
- Nameserver hostnames (NS)
- CNAME target
- Mail exchanger hostnames (MX)
- TXT records

All hostnames are normalized by stripping trailing dots. Lookup failures for individual record types are silently ignored -- a domain that has A records but no MX records simply returns an empty MX slice.

### `internal/httpclient`

Provides `httpclient.Default()` which returns an `*http.Client` configured for high-throughput crawling:
- Max idle connections: 1024
- Max connections per host: 128
- Response timeout: 10 seconds
- Overall timeout: 15 seconds

`ResilientClient` wraps the base client with circuit breaker integration.

### `internal/extract`

Two key functions:
- `ParseLinks(base *url.URL, body io.Reader) ([]string, error)` -- parses HTML with `golang.org/x/net/html` tokenizer, extracts href/src attributes from `<a>`, `<link>`, `<script>`, `<img>`, and `<iframe>` tags, and resolves them against the base URL.
- `ExternalDomains(host string, links []string) []string` -- filters parsed links to only those pointing to domains outside the current host's apex domain.
- `Apex(host string) string` -- computes the apex (registrable) domain using the public suffix list.

### `internal/emit`

Defines the data model (`Batch`, `Edge`, `NodeDomain`, `NodeIP`, `NodeCert`) and the `Emitter` that handles output.

The `Emitter`:
- Accumulates batches from the probe workers
- Flushes when edge count exceeds `batch_max_edges` (default: 10,000) or a timer fires (default: 2 seconds)
- Outputs to stdout (JSON) when no ingest URL is configured
- POSTs to an HTTP endpoint with exponential backoff retry (via `cenkalti/backoff`)
- Spools failed batches to disk as timestamped JSON files
- Replays spooled files on `Drain()` (called during shutdown)
- Supports mTLS client certificates for authenticated ingest endpoints

Edge type constants: `RESOLVES_TO`, `USES_NS`, `ALIAS_OF`, `USES_MX`, `LINKS_TO`, `USES_CERT`

### `internal/dedup`

Prevents duplicate nodes and edges from being emitted.

**Interface:**

```go
type Interface interface {
    Seen(key string) bool
}
```

`Seen()` returns `true` if the key was already recorded (duplicate), `false` if it is new (first occurrence). Implementations must be safe for concurrent use.

**Backends:**
- `NewMemory()` -- in-process `sync.Map`-based dedup. Fast, zero configuration, but not shared across probe instances.
- `NewRedis(addr string, ttl time.Duration, log *zap.SugaredLogger)` -- Redis SET with TTL. Enables distributed dedup across multiple probe instances. Activated by setting `REDIS_ADDR`.

Keys follow a prefix convention: `nodeip|<ip>`, `domain|<host>`, `cert|<spki>`, `edge|<src>|<type>|<target>`.

### `internal/discover`

Manages the discovery sink system for recursive/continuous crawling. When a probe discovers a new domain (via DNS NS/CNAME/MX records or HTML links), it submits it to a `Sink` for potential future crawling.

See [Discovery Sink System](#discovery-sink-system) below for full details.

### `internal/config`

Loads and validates configuration. Supports YAML and JSON config files.

`Config` struct holds all configurable fields with `yaml` and `json` struct tags. Methods:
- `SetDefaults()` -- fills in defaults for unset fields
- `Validate()` -- checks required fields and value ranges
- `LoadFromEnv()` -- reads `REDIS_ADDR`, `REDIS_QUEUE_ADDR`, `REDIS_QUEUE_KEY` from environment
- `MergeWithFlags(map[string]interface{})` -- overlays CLI flag values onto the config
- `LoadFromFile(path string) (*Config, error)` -- loads from a YAML or JSON file, applies defaults, and validates

### `internal/queue`

Redis-based distributed work queue using Redis lists:
- `Seed(ctx, host)` -- pushes a domain onto the queue (LPUSH)
- `Lease(ctx) (host, ack, err)` -- atomically pops a domain and moves it to a processing list (BRPOPLPUSH)
- Lease timeout: 120 seconds

Used by `cmd/seed` to populate the queue and by `cmd/spyder` to consume from it.

### `internal/rate`

Per-host token bucket rate limiter using `golang.org/x/time/rate`:
- `New(rps float64, burst int) *PerHost` -- creates a limiter
- `Allow(host string) bool` -- non-blocking check
- `Wait(host string)` -- blocking wait until a token is available

Each host gets an independent limiter, created on first access. Default configuration: 1.0 request per second per host, burst of 1.

### `internal/robots`

Fetches and caches robots.txt files:
- `NewCache(client *http.Client, ua string) *Cache` -- creates a cache backed by an LRU (4096 entries, 24-hour TTL)
- `Get(ctx, host) (*robotstxt.RobotsData, error)` -- fetches robots.txt (HTTPS first, falls back to HTTP), caches the result
- `Allowed(rd *robotstxt.RobotsData, ua, path string) bool` -- checks if the path is allowed for the user-agent
- `ShouldSkipByTLD(host string, excluded []string) bool` -- returns true if the host's TLD is in the exclusion list

### `internal/circuitbreaker`

Three-state circuit breaker (Closed, Open, Half-Open):
- Configurable failure threshold, failure ratio, timeout, and interval
- `Execute(fn func() error) error` -- runs a function through the circuit breaker
- `ExecuteWithRetry(cb, fn, maxRetries, delay)` -- retries with exponential delay, aborts on open circuit

`HostBreaker` wraps per-host circuit breakers with independent state:
- `Execute(host string, fn func() error) error`
- `State(host string) State`
- `Reset(host string)`
- `Stats() map[string]State`

### `internal/tlsinfo`

Connects to a host on port 443, performs a TLS handshake, and extracts certificate metadata:
- SPKI SHA-256 fingerprint
- Subject and issuer common names
- Validity period (NotBefore, NotAfter)

Returns a `NodeCert` struct. Timeout: 8 seconds.

### `internal/metrics`

Registers Prometheus metrics and starts the metrics HTTP server:
- `TasksTotal` -- counter with `status` label (ok/error)
- `EdgesTotal` -- counter with `type` label (RESOLVES_TO, USES_NS, etc.)
- `RobotsBlocks` -- counter for robots.txt denials
- `ServeWithHealth(addr, healthHandler, log)` -- serves `/metrics` and health endpoints

### `internal/health`

HTTP health check handler with:
- Liveness endpoint (`/healthz`)
- Readiness endpoint (`/readyz`)
- Configurable health checkers (e.g., Redis connectivity)
- Metadata (probe ID, run ID, version)

### `internal/logging`

Initializes a `zap.SugaredLogger` with structured JSON output. Respects the `LOG_LEVEL` environment variable (debug, info, warn, error).

### `internal/telemetry`

Initializes the OpenTelemetry SDK:
- OTLP HTTP exporter pointing at `otel_endpoint`
- Service name: `otel_service` (default: `spyder-probe`)
- Returns a `shutdown` function for graceful cleanup

### `internal/output`

Formats batch data for output. Supports three formats:
- `json` -- pretty-printed JSON (default)
- `jsonl` / `ndjson` -- newline-delimited JSON (one batch per line)
- `csv` -- comma-separated with header row (edges only)

### `internal/ui`

Terminal UI components:
- Progress indicators showing domains processed, edges discovered, and throughput
- Logger integration for clean output alongside progress bars

## Data Flow

The following diagram shows how data moves through SPYDER during a probe run.

```
                         ┌──────────────────┐
                         │  domains.txt     │
                         │  (or Redis queue) │
                         └────────┬─────────┘
                                  │
                                  ▼
                         ┌──────────────────┐
                         │  Task Channel    │
                         │  (chan string)    │
                         └────────┬─────────┘
                                  │
                    ┌─────────────┼─────────────┐
                    ▼             ▼              ▼
              ┌──────────┐ ┌──────────┐  ┌──────────┐
              │ Worker 1 │ │ Worker 2 │  │ Worker N │
              │ CrawlOne │ │ CrawlOne │  │ CrawlOne │
              └────┬─────┘ └────┬─────┘  └────┬─────┘
                   │             │              │
          ┌────────┴──────┬──────┴────┬─────────┘
          ▼               ▼           ▼
   ┌────────────┐  ┌────────────┐  ┌────────────┐
   │ DNS Resolve│  │ HTTP GET / │  │ TLS Cert   │
   │ A/NS/MX/CN│  │ + Extract  │  │ Fetch      │
   └─────┬──────┘  └─────┬──────┘  └─────┬──────┘
         │               │               │
         └───────────┬────┴───────────────┘
                     ▼
              ┌──────────────┐
              │ Dedup Check  │──── seen? ──→ skip
              │ (Seen(key))  │
              └──────┬───────┘
                     │ new
                     ▼
              ┌──────────────┐
              │ Discovery    │──→ sink.Submit(host) ──→ back to task channel
              │ Sink         │                         (if continuous mode)
              └──────┬───────┘
                     │
                     ▼
              ┌──────────────┐
              │ Batch Channel│
              │ (chan Batch)  │
              └──────┬───────┘
                     │
                     ▼
              ┌──────────────┐
              │ Emitter      │
              │ accumulate   │
              │ + flush      │
              └──────┬───────┘
                     │
              ┌──────┴──────┐
              ▼              ▼
       ┌───────────┐  ┌───────────┐
       │  stdout   │  │  HTTP     │
       │  (JSON)   │  │  ingest   │
       └───────────┘  │  endpoint │
                      └─────┬─────┘
                            │ on failure
                            ▼
                      ┌───────────┐
                      │  Spool    │
                      │  (disk)   │
                      └───────────┘
```

### Step-by-Step

1. **Input:** Domains are read from a text file (one per line) or leased from a Redis queue. Blank lines and lines starting with `#` are skipped. Domains are lowercased and trailing dots are stripped.

2. **Task distribution:** Domains are sent to a buffered channel (capacity 8192). N worker goroutines consume from this channel concurrently (default N=256).

3. **CrawlOne:** Each worker probes a single domain through the full pipeline:
   - **DNS resolution** returns IPs, nameservers, CNAME, and MX records
   - **Policy checks** skip excluded TLDs and domains blocked by robots.txt
   - **Rate limiting** enforces per-host request spacing (1 req/sec default)
   - **HTTP fetch** GETs the root page over HTTPS (512KB body limit, 15s timeout)
   - **Link extraction** parses HTML for external domain references
   - **TLS analysis** extracts certificate metadata and SPKI fingerprint

4. **Deduplication:** Every node and edge is checked against the dedup backend before being added to the batch. Keys use a prefix convention (`domain|`, `nodeip|`, `cert|`, `edge|`) to namespace different entity types.

5. **Discovery:** Newly discovered domains (from NS, CNAME, MX, and link extraction) are submitted to the discovery sink. In continuous mode, these domains feed back into the task channel for recursive crawling.

6. **Batch emission:** Worker batches flow through a channel (capacity 1024) to the emitter, which accumulates them and flushes when the edge count reaches `batch_max_edges` or a timer fires. Output goes to stdout (JSON) or an HTTP ingest endpoint with retry.

7. **Spooling and replay:** If the ingest endpoint is unreachable, batches are spooled to disk as JSON files. On shutdown, `Drain()` replays spooled files.

## Key Interfaces

### `dedup.Interface`

```go
type Interface interface {
    Seen(key string) bool
}
```

Central abstraction for deduplication. `Seen()` atomically checks and records a key. Returns `false` on first call (new item), `true` on subsequent calls (duplicate).

Implementations:
- `dedup.NewMemory()` -- in-process `sync.Map`
- `dedup.NewRedis(addr, ttl, log)` -- Redis SET with configurable TTL

Used by: `probe.Probe` to deduplicate nodes and edges before emission.

### `discover.Sink`

```go
type Sink interface {
    Submit(ctx context.Context, host string) bool
    Discovered() int64
}
```

Receives newly discovered domains and optionally feeds them back for future crawling. `Submit()` returns `true` if the domain was new and successfully enqueued.

Implementations:
- `discover.NoopSink{}` -- discards all discoveries (non-recursive mode)
- `discover.NewChannelSink(ch, dedup, maxDomains)` -- sends to an in-memory channel
- `discover.NewRedisSink(queue, dedup, maxDomains)` -- pushes to a Redis queue

Used by: `probe.Probe` when it encounters new domains in DNS records or HTML links.

## Configuration Loading Order

Configuration values are resolved in a layered system where later sources override earlier ones:

```
1. Defaults (SetDefaults)
       ↓
2. Config file (-config=spyder.yaml)
       ↓
3. Environment variables (REDIS_ADDR, REDIS_QUEUE_ADDR, REDIS_QUEUE_KEY)
       ↓
4. CLI flags (-concurrency=512, -ingest=..., etc.)
```

### 1. Defaults

`Config.SetDefaults()` fills in values for any field that is zero-valued:

| Field | Default |
|-------|---------|
| `Probe` | `local-1` |
| `Run` | `run-<unix-timestamp>` |
| `UA` | `SPYDERProbe/1.0 (+https://github.com/gustycube/spyder)` |
| `ExcludeTLDs` | `[gov, mil, int]` |
| `Concurrency` | `256` |
| `BatchMaxEdges` | `10000` |
| `BatchFlushSec` | `2` |
| `SpoolDir` | `spool` |
| `OutputFormat` | `json` |
| `MetricsAddr` | `:9090` |
| `OTELService` | `spyder-probe` |
| `RedisQueueKey` | `spyder:queue` |

### 2. Config File

If `-config=path.yaml` is provided, the file is parsed (YAML or JSON based on extension) and its values replace the defaults. `SetDefaults()` runs after loading to fill any fields the file did not specify.

```yaml
# configs/spyder.yaml
probe: us-east-1a
domains: configs/top-1m.txt
concurrency: 512
batch_max_edges: 50000
exclude_tlds:
  - gov
  - mil
  - int
  - edu
ingest: https://ingest.example.com/v1/batch
```

### 3. Environment Variables

`Config.LoadFromEnv()` reads Redis-related environment variables. These override any values from the config file:

```bash
export REDIS_ADDR=redis.prod:6379          # dedup backend
export REDIS_QUEUE_ADDR=redis.prod:6379    # work queue
export REDIS_QUEUE_KEY=spyder:prod:queue   # queue key name
```

This layer exists so Redis addresses can be injected by container orchestrators (Docker, Kubernetes) without modifying config files.

### 4. CLI Flags

Command-line flags have the highest precedence. `Config.MergeWithFlags()` only overwrites a field if the flag was explicitly provided (non-zero value):

```bash
./bin/spyder -config=configs/spyder.yaml -concurrency=1024 -ingest=https://other.example.com
```

In this example, `concurrency` becomes 1024 and `ingest` is overridden, while all other values come from the config file.

## Discovery Sink System

The discovery sink system controls what happens when SPYDER encounters a domain it has not seen before. This is the mechanism behind recursive/continuous crawling.

### NoopSink

```go
type NoopSink struct{}
```

Discards all discoveries. Used in the default (non-recursive) mode where SPYDER only probes the domains from the input file and stops.

Selected when `-continuous` is not set.

### ChannelSink

```go
type ChannelSink struct {
    ch         chan<- string
    dedup      dedup.Interface
    maxDomains int64
    count      atomic.Int64
}
```

Feeds discovered domains into an in-memory Go channel that loops back into the task channel. The probe reads seed domains from the file first, then continuously consumes discovered domains until the context is cancelled or `maxDomains` is reached.

Selected when `-continuous` is set and no Redis queue is configured.

Flow: `probe.CrawlOne` -> `sink.Submit(host)` -> channel -> task channel -> `probe.CrawlOne`

Guards:
- **Dedup check:** `dedup.Seen("discovered|" + host)` prevents re-submitting the same domain
- **Max domains:** `maxDomains` caps total discoveries (0 = unlimited)
- **Context cancellation:** `Submit()` respects context deadline

### RedisSink

```go
type RedisSink struct {
    q          *queue.RedisQueue
    dedup      dedup.Interface
    maxDomains int64
    count      atomic.Int64
}
```

Pushes discovered domains into the Redis queue via `queue.Seed()`. Multiple probe instances can share the same queue, enabling distributed recursive crawling where any probe instance can discover domains that other instances will pick up.

Selected when `-continuous` is set and `REDIS_QUEUE_ADDR` is configured.

Flow: `probe.CrawlOne` -> `sink.Submit(host)` -> Redis LPUSH -> any probe instance leases it

### Selection Logic

The sink is selected in `cmd/spyder/main.go` based on two flags:

| `-continuous` | `REDIS_QUEUE_ADDR` | Sink |
|---|---|---|
| false | any | `NoopSink` |
| true | empty | `ChannelSink` |
| true | set | `RedisSink` |

### Example: Distributed Recursive Crawl

```bash
# Terminal 1: Seed the queue
./bin/seed -domains=configs/seeds.txt -redis=redis:6379

# Terminal 2: Probe instance A (continuous, Redis queue + dedup)
REDIS_ADDR=redis:6379 REDIS_QUEUE_ADDR=redis:6379 \
  ./bin/spyder -continuous -max_domains=500000 \
  -domains=/dev/null -ingest=https://ingest.example.com/v1/batch

# Terminal 3: Probe instance B (same configuration)
REDIS_ADDR=redis:6379 REDIS_QUEUE_ADDR=redis:6379 \
  ./bin/spyder -continuous -max_domains=500000 \
  -domains=/dev/null -ingest=https://ingest.example.com/v1/batch
```

Both probes consume from the same Redis queue. When either probe discovers a new domain, it pushes it back into the queue. The shared Redis dedup backend ensures no domain is probed twice across the cluster.
