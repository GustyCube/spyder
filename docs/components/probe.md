# Probe Engine Component

The Probe Engine is the core orchestrator of SPYDER, responsible for coordinating domain discovery across multiple concurrent workers while enforcing policies and managing resources.

## Architecture

```go
type Probe struct {
    ua       string               // User-Agent for HTTP requests
    probeID  string               // Unique probe identifier
    runID    string               // Current run identifier
    excluded []string             // Excluded TLD list
    dedup    dedup.Interface      // Deduplication backend
    out      chan<- emit.Batch    // Output channel for batches
    hc       *http.Client         // HTTP client instance
    rob      *robots.Cache        // Robots.txt cache
    ratelim  *rate.PerHost        // Per-host rate limiter
    log      *zap.SugaredLogger   // Structured logger
}
```

## Core Functions

### Worker Pool Management

**Concurrent Processing:**
```go
func (p *Probe) Run(ctx context.Context, tasks <-chan string, workers int) {
    done := make(chan struct{})
    for i := 0; i < workers; i++ {
        go func() {
            for host := range tasks {
                p.CrawlOne(ctx, host)  // Process single domain
                metrics.TasksTotal.WithLabelValues("ok").Inc()
            }
            done <- struct{}{}
        }()
    }
    // Wait for all workers to complete
    for i := 0; i < workers; i++ { <-done }
}
```

**Key Features:**
- Configurable worker count (default: 256)
- Graceful worker shutdown via context cancellation
- Automatic task distribution across workers
- Worker completion synchronization

### Domain Processing Pipeline

**Single Domain Processing:**
```go
func (p *Probe) CrawlOne(ctx context.Context, host string) {
    // OpenTelemetry tracing
    tr := otel.Tracer("spyder/probe")
    ctx, span := tr.Start(ctx, "CrawlOne")
    defer span.End()
    
    now := time.Now().UTC()
    var nodesD []emit.NodeDomain
    var nodesIP []emit.NodeIP
    var nodesC []emit.NodeCert
    var edges []emit.Edge
    
    // 1. DNS Resolution
    ips, ns, cname, mx, _ := dns.ResolveAll(ctx, host)
    
    // 2. Policy Enforcement
    if robots.ShouldSkipByTLD(host, p.excluded) {
        p.flush(nodesD, nodesIP, nodesC, edges)
        return
    }
    
    // 3. Robots.txt Check
    rd, _ := p.rob.Get(ctx, host)
    if !robots.Allowed(rd, p.ua, "/") {
        metrics.RobotsBlocks.Inc()
        p.flush(nodesD, nodesIP, nodesC, edges)
        return
    }
    
    // 4. Rate Limiting
    p.ratelim.Wait(host)
    
    // 5. HTTP Content Fetching
    // 6. TLS Certificate Analysis
    // 7. Data Aggregation and Output
}
```

## Policy Enforcement

### TLD Exclusion

**Implementation:**
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

**Default Exclusions:**
- `gov` - Government domains
- `mil` - Military domains
- `int` - International organization domains

**Custom Exclusions:**
```bash
# Add educational domains
-exclude_tlds=gov,mil,int,edu

# No exclusions
-exclude_tlds=""
```

### Robots.txt Compliance

**Respect Robots.txt:**
```go
rd, _ := p.rob.Get(ctx, host)
if !robots.Allowed(rd, p.ua, "/") {
    metrics.RobotsBlocks.Inc()
    // Skip HTTP crawling, continue with DNS only
    return
}
```

**Features:**
- LRU cache with 24-hour TTL
- HTTPS-first fallback to HTTP
- User-Agent specific rule matching
- Graceful handling of missing robots.txt

## Resource Management

### Rate Limiting

**Per-host Token Bucket:**
```go
type PerHost struct {
    mu        sync.Mutex
    m         map[string]*rate.Limiter
    perSecond float64
    burst     int
}

func (p *PerHost) Wait(host string) {
    p.mu.Lock()
    lim, ok := p.m[host]
    if !ok {
        lim = rate.NewLimiter(rate.Limit(p.perSecond), p.burst)
        p.m[host] = lim
    }
    p.mu.Unlock()
    _ = lim.WaitN(nil, 1)
}
```

**Configuration:**
- Default: 1.0 requests/second per host
- Burst: 1 request
- Independent limits per hostname
- Automatic limiter creation

### HTTP Client Configuration

**Optimized HTTP Transport:**
```go
func Default() *http.Client {
    tr := &http.Transport{
        TLSClientConfig:       &tls.Config{InsecureSkipVerify: false},
        DisableCompression:    false,
        MaxIdleConns:          1024,
        MaxConnsPerHost:       128,
        MaxIdleConnsPerHost:   64,
        ResponseHeaderTimeout: 10 * time.Second,
        IdleConnTimeout:       30 * time.Second,
        ExpectContinueTimeout: 1 * time.Second,
    }
    return &http.Client{
        Transport: tr,
        Timeout:   15 * time.Second,
    }
}
```

## Data Collection

### DNS Data Processing

**Node Creation:**
```go
// Create domain node
ap := extract.Apex(host)
nodesD = append(nodesD, emit.NodeDomain{
    Host: host, 
    Apex: ap, 
    FirstSeen: now, 
    LastSeen: now,
})

// Create IP nodes and edges
for _, ip := range ips {
    if !p.dedup.Seen("nodeip|"+ip) {
        nodesIP = append(nodesIP, emit.NodeIP{
            IP: ip, 
            FirstSeen: now, 
            LastSeen: now,
        })
    }
    k := "edge|"+host+"|RESOLVES_TO|"+ip
    if !p.dedup.Seen(k) {
        edges = append(edges, emit.Edge{
            Type: "RESOLVES_TO", 
            Source: host, 
            Target: ip, 
            ObservedAt: now, 
            ProbeID: p.probeID, 
            RunID: p.runID,
        })
        metrics.EdgesTotal.WithLabelValues("RESOLVES_TO").Inc()
    }
}
```

### HTTP Content Processing

**Safe HTTP Fetching:**
```go
root := &url.URL{Scheme: "https", Host: host, Path: "/"}
req, _ := http.NewRequestWithContext(ctx, "GET", root.String(), nil)
req.Header.Set("User-Agent", p.ua)
resp, err := p.hc.Do(req)

if err == nil {
    ct := strings.ToLower(resp.Header.Get("Content-Type"))
    if strings.Contains(ct, "text/html") && 
       resp.StatusCode >= 200 && resp.StatusCode < 300 {
        // Limit response size to 512KB
        body := io.LimitReader(resp.Body, 512*1024)
        links, _ := extract.ParseLinks(root, body)
        outs := extract.ExternalDomains(host, links)
        
        // Create LINKS_TO edges
        for _, h := range outs {
            // ... edge creation
        }
    }
    io.Copy(io.Discard, resp.Body)
    resp.Body.Close()
}
```

### TLS Certificate Processing

**Certificate Analysis:**
```go
if cert, err := tlsinfo.FetchCert(host); err == nil && cert != nil {
    if !p.dedup.Seen("cert|"+cert.SPKI) {
        nodesC = append(nodesC, *cert)
    }
    k := "edge|"+host+"|USES_CERT|"+cert.SPKI
    if !p.dedup.Seen(k) {
        edges = append(edges, emit.Edge{
            Type: "USES_CERT", 
            Source: host, 
            Target: cert.SPKI, 
            ObservedAt: now, 
            ProbeID: p.probeID, 
            RunID: p.runID,
        })
        metrics.EdgesTotal.WithLabelValues("USES_CERT").Inc()
    }
}
```

## Deduplication Integration

### Memory Backend (Default)

**In-Process Deduplication:**
```go
type Memory struct {
    mu   sync.RWMutex
    seen map[string]struct{}
}

func (m *Memory) Seen(key string) bool {
    m.mu.RLock()
    _, exists := m.seen[key]
    m.mu.RUnlock()
    
    if !exists {
        m.mu.Lock()
        m.seen[key] = struct{}{}
        m.mu.Unlock()
        return false
    }
    return true
}
```

### Redis Backend (Distributed)

**Cross-Probe Deduplication:**
```go
func (r *Redis) Seen(key string) bool {
    pipe := r.client.Pipeline()
    existsCmd := pipe.Exists(context.Background(), key)
    setCmd := pipe.Set(context.Background(), key, "1", r.ttl)
    _, err := pipe.Exec(context.Background())
    
    return err == nil && existsCmd.Val() > 0
}
```

## Performance Characteristics

### Throughput Metrics

**Typical Performance:**
- Single worker: 2-5 domains/second
- 256 workers: 500-1000 domains/second
- Memory usage: ~1MB per 1000 workers
- Network: ~100KB/s per worker

**Bottlenecks:**
- DNS resolution latency
- HTTP response times
- robots.txt cache misses
- Rate limiting delays

### Optimization Strategies

**High-Throughput Configuration:**
```bash
./bin/spyder \
  -domains=large-list.txt \
  -concurrency=512 \
  -batch_max_edges=25000 \
  -batch_flush_sec=1
```

**Memory-Constrained Environment:**
```bash
./bin/spyder \
  -domains=domains.txt \
  -concurrency=64 \
  -batch_max_edges=5000 \
  -batch_flush_sec=5
```

## Error Handling

### Graceful Degradation

**Partial Failure Handling:**
- DNS resolution failures: Continue with other record types
- HTTP failures: Skip content analysis, continue with TLS
- TLS failures: Skip certificate analysis
- Individual worker failures: Don't affect other workers

**Error Recovery:**
```go
func (p *Probe) CrawlOne(ctx context.Context, host string) {
    defer func() {
        if r := recover(); r != nil {
            p.log.Error("worker panic", "host", host, "error", r)
            metrics.TasksTotal.WithLabelValues("error").Inc()
        }
    }()
    
    // ... processing logic with error handling
}
```

### Context Handling

**Graceful Shutdown:**
```go
func (p *Probe) Run(ctx context.Context, tasks <-chan string, workers int) {
    // Workers automatically stop when context is cancelled
    // or when tasks channel is closed
    
    for i := 0; i < workers; i++ {
        go func() {
            for {
                select {
                case host, ok := <-tasks:
                    if !ok { return } // Channel closed
                    p.CrawlOne(ctx, host)
                case <-ctx.Done(): // Context cancelled
                    return
                }
            }
        }()
    }
}
```

## Monitoring and Observability

### Metrics Integration

**Prometheus Metrics:**
```go
var (
    TasksTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "spyder_tasks_total"}, 
        []string{"status"},
    )
    EdgesTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "spyder_edges_total"}, 
        []string{"type"},
    )
    RobotsBlocks = prometheus.NewCounter(
        prometheus.CounterOpts{Name: "spyder_robots_blocked_total"},
    )
)
```

### Distributed Tracing

**OpenTelemetry Integration:**
```go
func (p *Probe) CrawlOne(ctx context.Context, host string) {
    tr := otel.Tracer("spyder/probe")
    ctx, span := tr.Start(ctx, "CrawlOne")
    defer span.End()
    
    span.SetAttributes(
        attribute.String("host", host),
        attribute.String("probe_id", p.probeID),
    )
    
    // ... processing with span context propagation
}
```

The Probe Engine's design ensures efficient, respectful, and reliable domain discovery while providing the flexibility needed for various deployment scenarios.