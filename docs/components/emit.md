# Batch Emitter Component

The batch emitter component (`internal/emit`) provides reliable data transmission with batching, retries, and on-disk spooling for fault-tolerant data delivery.

## Overview

The batch emitter aggregates discovered nodes and edges into batches and delivers them to ingestion endpoints with exponential backoff retry logic and disk-based spooling for maximum reliability.

## Core Data Types

### Node Types

#### `NodeDomain`
Represents a domain entity:
```go
type NodeDomain struct {
    Host      string    `json:"host"`       // Full hostname
    Apex      string    `json:"apex"`       // Apex/root domain  
    FirstSeen time.Time `json:"first_seen"` // First observation
    LastSeen  time.Time `json:"last_seen"`  // Last observation
}
```

#### `NodeIP`
Represents an IP address entity:
```go
type NodeIP struct {
    IP        string    `json:"ip"`         // IP address
    FirstSeen time.Time `json:"first_seen"` // First observation
    LastSeen  time.Time `json:"last_seen"`  // Last observation
}
```

#### `NodeCert`
Represents a TLS certificate entity:
```go
type NodeCert struct {
    SPKI      string    `json:"spki_sha256"` // SHA-256 of Subject Public Key Info
    SubjectCN string    `json:"subject_cn"`  // Certificate subject common name
    IssuerCN  string    `json:"issuer_cn"`   // Certificate issuer common name
    NotBefore time.Time `json:"not_before"`  // Certificate valid from
    NotAfter  time.Time `json:"not_after"`   // Certificate valid until
}
```

### Edge Type

#### `Edge`
Represents relationships between entities:
```go
type Edge struct {
    Type       string    `json:"type"`        // Edge type (RESOLVES_TO, LINKS_TO, etc.)
    Source     string    `json:"source"`      // Source entity identifier
    Target     string    `json:"target"`      // Target entity identifier
    ObservedAt time.Time `json:"observed_at"` // Observation timestamp
    ProbeID    string    `json:"probe_id"`    // Probe instance identifier
    RunID      string    `json:"run_id"`      // Probe run identifier
}
```

#### Supported Edge Types
- **`RESOLVES_TO`**: Domain → IP address (A/AAAA records)
- **`USES_NS`**: Domain → Nameserver (NS records)
- **`ALIAS_OF`**: Domain → CNAME target
- **`USES_MX`**: Domain → Mail exchanger (MX records)
- **`LINKS_TO`**: Domain → External domains (from HTML links)
- **`USES_CERT`**: Domain → TLS certificate (SPKI hash)

### Batch Structure

#### `Batch`
Container for nodes and edges:
```go
type Batch struct {
    ProbeID string       `json:"probe_id"`     // Probe instance identifier
    RunID   string       `json:"run_id"`       // Probe run identifier
    NodesD  []NodeDomain `json:"nodes_domain"` // Domain nodes
    NodesIP []NodeIP     `json:"nodes_ip"`     // IP address nodes
    NodesC  []NodeCert   `json:"nodes_cert"`   // Certificate nodes
    Edges   []Edge       `json:"edges"`        // Relationship edges
}
```

## Emitter Architecture

### `Emitter` Structure
```go
type Emitter struct {
    ingest     string           // Ingestion endpoint URL
    client     *http.Client     // HTTP client for delivery
    spoolDir   string          // On-disk spool directory
    batchSize  int             // Maximum batch size
    flushInt   time.Duration   // Flush interval
    buffer     Batch           // Current batch buffer
    mu         sync.Mutex      // Buffer protection
    logger     *zap.Logger     // Structured logging
}
```

## Core Functions

### `NewEmitter(config EmitterConfig) *Emitter`

Creates a new batch emitter with production-ready defaults.

**Configuration:**
- **Ingestion URL**: HTTPS endpoint for batch delivery
- **mTLS Support**: Client certificate authentication
- **Spool Directory**: On-disk buffer for failed deliveries
- **Batch Size**: Maximum items per batch (default: 1000)
- **Flush Interval**: Time-based batch flushing (default: 30s)

### `AddEdge(edge Edge)`

Adds an edge to the current batch buffer.

**Features:**
- **Thread-Safe**: Mutex-protected buffer access
- **Automatic Flushing**: Triggers flush when batch size reached
- **Deduplication**: Prevents duplicate edges in same batch

### `AddNode(node interface{})`

Adds nodes (Domain, IP, or Certificate) to the current batch.

**Type Detection:**
- **NodeDomain**: Added to domain node array
- **NodeIP**: Added to IP node array  
- **NodeCert**: Added to certificate node array

## Reliability Features

### Exponential Backoff Retry
- **Initial Delay**: 1 second
- **Maximum Delay**: 5 minutes
- **Multiplier**: 2x per retry
- **Jitter**: Randomization to prevent thundering herd
- **Max Attempts**: Configurable (default: 10)

### On-Disk Spooling
- **Persistent Storage**: Failed batches written to disk
- **Automatic Recovery**: Spooled batches replayed on startup
- **Format**: JSON files with timestamp prefixes
- **Cleanup**: Successful deliveries remove spool files

### Circuit Breaker Integration
- **Failure Detection**: HTTP 5xx and connection errors
- **Automatic Recovery**: Gradual traffic restoration
- **Per-Host Isolation**: Independent failure tracking

## HTTP Delivery

### Request Format
```http
POST /v1/batch HTTP/1.1
Host: ingest.example.com
Content-Type: application/json
User-Agent: spyder-probe/1.0

{
  "probe_id": "prod-us-west-1",
  "run_id": "run-20231201-143000",
  "nodes_domain": [...],
  "nodes_ip": [...], 
  "nodes_cert": [...],
  "edges": [...]
}
```

### mTLS Authentication
```go
// Client certificate configuration
cert, err := tls.LoadX509KeyPair("client.crt", "client.key")
client := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{
            Certificates: []tls.Certificate{cert},
        },
    },
}
```

## Monitoring and Metrics

### Delivery Metrics
- **Batch Success Rate**: Percentage of successful deliveries
- **Retry Attempts**: Distribution of retry attempts per batch
- **Delivery Latency**: Time from buffer to successful delivery
- **Spool Usage**: Number and size of spooled batches

### Performance Metrics
- **Batch Size Distribution**: Actual vs configured batch sizes
- **Flush Triggers**: Time-based vs size-based flush ratio
- **Buffer Utilization**: Current buffer fill percentage
- **Throughput**: Batches and items per second

## Error Handling

### Delivery Failures
- **Temporary Failures**: HTTP 5xx, timeouts, connection errors
- **Permanent Failures**: HTTP 4xx (except 429), authentication errors
- **Retry Strategy**: Exponential backoff for temporary failures
- **Dead Letter**: Permanent failures logged and discarded

### Spool Management
- **Write Failures**: Log error and attempt in-memory retry
- **Recovery Failures**: Log error and continue with new batches
- **Disk Full**: Fallback to in-memory buffering only

## Configuration Examples

### Basic Configuration
```go
emitter := emit.NewEmitter(emit.EmitterConfig{
    IngestURL: "https://ingest.example.com/v1/batch",
    BatchSize: 1000,
    FlushInterval: 30 * time.Second,
    SpoolDir: "/var/spool/spyder",
})
```

### mTLS Configuration
```go
emitter := emit.NewEmitter(emit.EmitterConfig{
    IngestURL: "https://ingest.example.com/v1/batch",
    ClientCert: "/etc/spyder/client.crt",
    ClientKey: "/etc/spyder/client.key",
    BatchSize: 500,
    FlushInterval: 60 * time.Second,
})
```

### High-Throughput Configuration
```go
emitter := emit.NewEmitter(emit.EmitterConfig{
    IngestURL: "https://ingest.example.com/v1/batch",
    BatchSize: 5000,
    FlushInterval: 10 * time.Second,
    Concurrency: 4,
    SpoolDir: "/tmp/spyder-spool",
})
```

## Integration Patterns

### Probe Integration
```go
func (p *Probe) ProcessDomain(domain string) {
    // Discover relationships
    edges := p.discoverEdges(domain)
    nodes := p.discoverNodes(domain)
    
    // Emit discoveries
    for _, edge := range edges {
        p.emitter.AddEdge(edge)
    }
    for _, node := range nodes {
        p.emitter.AddNode(node)
    }
}
```

### Graceful Shutdown
```go
func (p *Probe) Shutdown() {
    // Flush remaining batches
    p.emitter.Flush()
    
    // Wait for in-flight deliveries
    p.emitter.Wait()
    
    // Close resources
    p.emitter.Close()
}
```

## Best Practices

### Batch Sizing
- **Memory Usage**: Larger batches use more memory
- **Network Efficiency**: Larger batches reduce HTTP overhead
- **Latency**: Smaller batches reduce data freshness delay
- **Recommended**: 500-2000 items per batch

### Error Recovery
- **Spool Monitoring**: Monitor spool directory size and age
- **Retry Limits**: Configure reasonable maximum retry attempts
- **Dead Letter Handling**: Implement alerting for permanent failures

### Performance Tuning
- **Concurrent Delivery**: Use multiple emitter instances for high throughput
- **Batch Optimization**: Tune batch size based on network and server capacity
- **Flush Frequency**: Balance latency requirements with efficiency

## Security Considerations

### Data Transmission
- **HTTPS Only**: All data transmitted over encrypted connections
- **mTLS Authentication**: Mutual TLS for secure ingestion endpoints
- **Certificate Validation**: Verify server certificates

### Data Integrity
- **JSON Validation**: Validate batch structure before transmission
- **Retry Limits**: Prevent infinite retry loops
- **Spool Security**: Secure file permissions for spooled data