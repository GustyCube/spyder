# SPYDER Discovery Pipeline

The SPYDER discovery pipeline processes domains through multiple stages, collecting various types of infrastructure data while respecting policies and rate limits.

## Pipeline Overview

```
┌─────────────┐    ┌──────────────┐    ┌──────────────┐    ┌─────────────┐
│   Input     │───▶│  Validation  │───▶│   Policy     │───▶│   Worker    │
│  Domains    │    │  & Parsing   │    │   Checks     │    │    Pool     │
└─────────────┘    └──────────────┘    └──────────────┘    └─────────────┘
                                                                   │
┌─────────────┐    ┌──────────────┐    ┌──────────────┐           ▼
│   Output    │◀───│ Deduplication│◀───│   Data       │    ┌─────────────┐
│  Batches    │    │  & Merging   │    │ Collection   │◀───│ Concurrent  │
└─────────────┘    └──────────────┘    └──────────────┘    │ Processing  │
                                                            └─────────────┘
```

## Stage 1: Input Processing

### Domain Input Sources

**File-based Input (Default)**
```go
// Read domains from file
f, err := os.Open(domainsFile)
scanner := bufio.NewScanner(f)
for scanner.Scan() {
    line := strings.TrimSpace(scanner.Text())
    if line == "" || strings.HasPrefix(line, "#") { 
        continue // Skip empty lines and comments
    }
    line = strings.ToLower(strings.TrimSuffix(line, "."))
    tasks <- line
}
```

**Redis Queue Input (Distributed)**
```go
// Lease tasks from Redis queue
for {
    host, ack, err := q.Lease(ctx)
    if err != nil { continue }
    if host == "" { continue }
    tasks <- host
    _ = ack() // Acknowledge immediately after leasing
}
```

### Domain Validation

Input domains undergo validation:

1. **Syntax Validation**: Check valid hostname format
2. **Normalization**: Convert to lowercase, remove trailing dots
3. **Filtering**: Remove empty lines and comments
4. **Deduplication**: Skip already queued domains

## Stage 2: Policy Enforcement

### TLD Exclusion Check

```go
func ShouldSkipByTLD(host string, excluded []string) bool {
    for _, t := range excluded {
        if strings.HasSuffix(host, "."+t) || host == t { 
            return true 
        }
    }
    return false
}
```

**Default Excluded TLDs:**
- `gov` - Government domains
- `mil` - Military domains  
- `int` - International organization domains

### Robots.txt Compliance

```go
// Check robots.txt permissions
rd, _ := p.rob.Get(ctx, host)
if !robots.Allowed(rd, p.ua, "/") {
    metrics.RobotsBlocks.Inc()
    // Skip HTTP crawling but continue DNS collection
    return
}
```

**Robots.txt Logic:**
1. Try HTTPS first: `https://example.com/robots.txt`
2. Fallback to HTTP: `http://example.com/robots.txt`
3. Cache results with 24-hour TTL
4. Default to allow if robots.txt unavailable

## Stage 3: Worker Pool Processing

### Concurrent Processing Architecture

```go
func (p *Probe) Run(ctx context.Context, tasks <-chan string, workers int) {
    done := make(chan struct{})
    for i := 0; i < workers; i++ {
        go func() {
            for host := range tasks {
                p.CrawlOne(ctx, host) // Process single domain
                metrics.TasksTotal.WithLabelValues("ok").Inc()
            }
            done <- struct{}{}
        }()
    }
    // Wait for all workers to complete
    for i := 0; i < workers; i++ { <-done }
}
```

### Per-Host Rate Limiting

```go
// Apply per-host rate limiting before HTTP requests
p.ratelim.Wait(host)

// Token bucket implementation
type PerHost struct {
    mu sync.Mutex
    m  map[string]*rate.Limiter
    perSecond float64
    burst int
}
```

**Rate Limiting Configuration:**
- Default: 1.0 requests/second per host
- Burst: 1 request
- Individual limiters per hostname
- Automatic limiter creation

## Stage 4: Data Collection

### DNS Resolution Phase

```go
// Comprehensive DNS lookup
ips, ns, cname, mx, _ := dns.ResolveAll(ctx, host)

// Create nodes and edges for each record type
for _, ip := range ips {
    if !p.dedup.Seen("nodeip|"+ip) { 
        nodesIP = append(nodesIP, emit.NodeIP{...})
    }
    edgeKey := "edge|"+host+"|RESOLVES_TO|"+ip
    if !p.dedup.Seen(edgeKey) { 
        edges = append(edges, emit.Edge{...})
    }
}
```

**DNS Record Types Collected:**
- **A/AAAA Records**: IPv4/IPv6 addresses
- **NS Records**: Authoritative nameservers
- **CNAME Records**: Canonical name aliases
- **MX Records**: Mail exchange servers
- **TXT Records**: Text records (collected but not currently processed)

### HTTP Content Analysis Phase

```go
// Fetch root page content
root := &url.URL{Scheme: "https", Host: host, Path: "/"}
req, _ := http.NewRequestWithContext(ctx, "GET", root.String(), nil)
req.Header.Set("User-Agent", p.ua)
resp, err := p.hc.Do(req)

if err == nil && isHTMLContent(resp) {
    body := io.LimitReader(resp.Body, 512*1024) // Limit to 512KB
    links, _ := extract.ParseLinks(root, body)
    externalDomains := extract.ExternalDomains(host, links)
    
    // Create LINKS_TO edges for external domains
    for _, targetHost := range externalDomains {
        edges = append(edges, emit.Edge{
            Type: "LINKS_TO", 
            Source: host, 
            Target: targetHost,
            ...
        })
    }
}
```

**HTTP Processing Steps:**
1. **Request Construction**: HTTPS-first with custom User-Agent
2. **Content-Type Validation**: Only process `text/html` responses
3. **Size Limiting**: Restrict response body to 512KB
4. **Link Extraction**: Parse HTML for external references
5. **Domain Filtering**: Extract only external domains (different apex)

### TLS Certificate Analysis Phase

```go
// Fetch TLS certificate information
if cert, err := tlsinfo.FetchCert(host); err == nil && cert != nil {
    if !p.dedup.Seen("cert|"+cert.SPKI) { 
        nodesC = append(nodesC, *cert)
    }
    edgeKey := "edge|"+host+"|USES_CERT|"+cert.SPKI
    if !p.dedup.Seen(edgeKey) { 
        edges = append(edges, emit.Edge{...})
    }
}
```

**Certificate Data Extracted:**
- **SPKI Hash**: SHA-256 of Subject Public Key Info
- **Subject CN**: Certificate subject common name
- **Issuer CN**: Certificate authority name
- **Validity Period**: Not-before and not-after timestamps

## Stage 5: Deduplication

### Deduplication Strategy

**Memory-based Deduplication (Single Node)**
```go
type Memory struct {
    mu   sync.RWMutex
    seen map[string]struct{}
}

func (m *Memory) Seen(key string) bool {
    m.mu.RLock()
    defer m.mu.RUnlock()
    _, exists := m.seen[key]
    if !exists {
        m.mu.RUnlock()
        m.mu.Lock()
        m.seen[key] = struct{}{}
        m.mu.Unlock()
        m.mu.RLock()
        return false
    }
    return true
}
```

**Redis-based Deduplication (Distributed)**
```go
func (r *Redis) Seen(key string) bool {
    pipe := r.client.Pipeline()
    existsCmd := pipe.Exists(context.Background(), key)
    setCmd := pipe.Set(context.Background(), key, "1", r.ttl)
    _, err := pipe.Exec(context.Background())
    
    return err == nil && existsCmd.Val() > 0
}
```

### Deduplication Keys

**Node Deduplication Keys:**
- Domains: `domain|example.com`
- IPs: `nodeip|192.168.1.1`
- Certificates: `cert|B7+tPUdz9OYBgGp...`

**Edge Deduplication Keys:**
- Format: `edge|{source}|{type}|{target}`
- Example: `edge|example.com|RESOLVES_TO|192.168.1.1`

## Stage 6: Batch Formation

### Batch Accumulation

```go
type Emitter struct {
    mu        sync.Mutex
    acc       Batch          // Accumulator for current batch
    batchMax  int           // Maximum edges per batch
    flushEvery time.Duration // Time-based flush interval
}

func (e *Emitter) append(b Batch) {
    e.mu.Lock()
    defer e.mu.Unlock()
    e.acc.NodesD = append(e.acc.NodesD, b.NodesD...)
    e.acc.NodesIP = append(e.acc.NodesIP, b.NodesIP...)
    e.acc.NodesC = append(e.acc.NodesC, b.NodesC...)
    e.acc.Edges = append(e.acc.Edges, b.Edges...)
}
```

### Batch Flush Triggers

**Size-based Flushing:**
- Max edges per batch: 10,000 (configurable)
- Max nodes per batch: 5,000 (configurable)

**Time-based Flushing:**
- Default interval: 2 seconds (configurable)
- Ensures regular data emission

**Context-based Flushing:**
- Graceful shutdown flush
- Process termination flush

## Stage 7: Output Processing

### Output Destinations

**Standard Output (Default)**
```go
if e.ingest == "" {
    _ = json.NewEncoder(os.Stdout).Encode(e.acc)
}
```

**HTTP Ingest API**
```go
func (e *Emitter) post(b Batch) error {
    buf := &bytes.Buffer{}
    _ = json.NewEncoder(buf).Encode(b)
    
    op := func() error {
        req, _ := http.NewRequest("POST", e.ingest, bytes.NewReader(buf.Bytes()))
        req.Header.Set("Content-Type", "application/json")
        resp, err := e.client.Do(req)
        // ... error handling and response processing
    }
    
    // Retry with exponential backoff
    bo := backoff.NewExponentialBackOff()
    bo.MaxElapsedTime = 30 * time.Second
    return backoff.Retry(op, bo)
}
```

### Reliability Mechanisms

**Retry Logic:**
- Exponential backoff for failed HTTP requests
- Maximum retry time: 30 seconds
- Automatic retry on transient failures

**Spooling for Failed Batches:**
```go
func (e *Emitter) spool(b Batch, log *zap.SugaredLogger) {
    name := time.Now().UTC().Format("20060102T150405.000000000") + ".json"
    path := filepath.Join(e.spoolDir, name)
    f, err := os.Create(path)
    // ... write batch to disk
}
```

**Spool Recovery:**
- Automatic replay of spooled batches on restart
- Failed batch cleanup after successful transmission
- Persistent storage for reliability

## Performance Characteristics

### Throughput Optimization

**Concurrent Processing:**
- Default: 256 concurrent workers
- Configurable based on system resources
- Worker pool pattern for efficiency

**Memory Management:**
- Streaming JSON processing
- Limited HTTP response sizes
- Bounded channel buffers

**Network Optimization:**
- HTTP connection pooling
- Keep-alive connections
- Connection reuse across requests

### Latency Considerations

**DNS Resolution:**
- Concurrent lookups for multiple record types
- Context-aware timeout handling
- Default Go resolver with system configuration

**HTTP Requests:**
- 15-second total timeout
- 10-second response header timeout
- Connection timeout protection

**TLS Handshakes:**
- 8-second TLS connection timeout
- Certificate chain validation
- Secure cipher suite selection

## Error Handling

### Graceful Degradation

**Partial Failure Handling:**
- Continue processing on individual domain failures
- Collect available data even if some operations fail
- Log failures without stopping pipeline

**Resource Protection:**
- Memory limits for HTTP responses
- Connection timeout enforcement
- Graceful handling of network errors

### Error Recovery

**Transient Error Handling:**
- Retry logic for temporary failures
- Exponential backoff for rate limiting
- Circuit breaker patterns for failing services

**Persistent Error Handling:**
- Skip permanently failing domains
- Log errors for later analysis
- Maintain processing progress despite failures

This pipeline design ensures reliable, efficient, and respectful data collection while maintaining high throughput and strong error resilience.