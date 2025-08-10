# Deduplication Component

The deduplication component (`internal/dedup`) prevents redundant processing by tracking previously seen domains across probe instances and time.

## Overview

The deduplication component provides efficient domain tracking to prevent redundant work in distributed SPYDER deployments. It offers both memory-based and Redis-based implementations for different deployment scenarios.

## Core Interface

### `Interface`

Common interface for all deduplication implementations:

```go
type Interface interface {
    Seen(key string) bool  // Returns true if key was previously seen
}
```

## Implementations

### Memory-Based Deduplication

#### `Memory` Structure
```go
type Memory struct {
    m sync.Map  // Thread-safe map for concurrent access
}
```

#### `NewMemory() *Memory`

Creates a new memory-based deduplicator.

**Returns:**
- `*Memory`: Memory-based deduplicator instance

**Characteristics:**
- **Thread-Safe**: Uses `sync.Map` for concurrent access
- **Process-Local**: Deduplication scope limited to single process
- **Persistent**: Entries persist for process lifetime
- **Zero Configuration**: No external dependencies

#### `Seen(key string) bool`

Checks if a key has been seen before, marking it as seen if new.

**Parameters:**
- `key`: The string key to check (typically domain name)

**Returns:**
- `bool`: `true` if key was previously seen, `false` if new

**Behavior:**
- **Atomic Operation**: Uses `LoadOrStore` for atomic check-and-set
- **Memory Efficient**: Stores empty struct{} as values
- **Immediate Response**: No network latency or timeouts

### Redis-Based Deduplication

#### `Redis` Structure
```go
type Redis struct {
    cli        *redis.Client  // Redis client connection
    ttl        time.Duration  // Time-to-live for entries
    errorCount int           // Error counter for monitoring
}
```

#### `NewRedis(addr string, ttl time.Duration) (*Redis, error)`

Creates a new Redis-based deduplicator with TTL support.

**Parameters:**
- `addr`: Redis server address (e.g., "127.0.0.1:6379")
- `ttl`: Time-to-live for deduplication entries

**Returns:**
- `*Redis`: Redis-based deduplicator instance
- `error`: Connection error if Redis is unreachable

**Features:**
- **Connection Validation**: Pings Redis server during initialization
- **TTL Support**: Automatic expiration of old entries
- **Distributed**: Shared state across multiple probe instances

#### `Seen(key string) bool`

Checks if a key has been seen using Redis SET NX operation.

**Parameters:**
- `key`: The string key to check (prefixed with "seen:")

**Returns:**
- `bool`: `true` if key was previously seen, `false` if new

**Implementation Details:**
- **Key Prefix**: Adds "seen:" prefix to all keys
- **Atomic Operation**: Uses `SETNX` for atomic check-and-set
- **Timeout Protection**: 2-second context timeout for Redis operations
- **Error Tolerance**: Returns `false` (not seen) on Redis errors
- **Error Throttling**: Logs every 100th error to prevent spam

## Deployment Strategies

### Single-Node Deployment
```go
// Memory-based deduplication for single probe instances
deduper := dedup.NewMemory()
if !deduper.Seen("example.com") {
    // Process domain (first time seen)
}
```

### Distributed Deployment
```go
// Redis-based deduplication for distributed probes
deduper, err := dedup.NewRedis("127.0.0.1:6379", 24*time.Hour)
if err != nil {
    log.Fatal("Redis connection failed")
}
if !deduper.Seen("example.com") {
    // Process domain (first time seen across cluster)
}
```

## Key Generation Strategies

### Domain-Based Keys
```go
// Simple domain deduplication
key := "domain:" + domain
```

### Content-Based Keys
```go
// URL-specific deduplication
key := "url:" + url
```

### Time-Window Keys
```go
// Daily deduplication windows
key := "daily:" + time.Now().Format("2006-01-02") + ":" + domain
```

## Performance Characteristics

### Memory Implementation
- **Latency**: Sub-microsecond operation latency
- **Throughput**: Millions of operations per second
- **Memory**: Linear growth with unique keys
- **Concurrency**: Excellent concurrent performance

### Redis Implementation
- **Latency**: ~1-2ms operation latency (network dependent)
- **Throughput**: Thousands of operations per second
- **Memory**: Bounded by TTL and Redis memory
- **Concurrency**: Shared state across processes

## Error Handling

### Memory Implementation
- **No Errors**: Memory operations cannot fail
- **Guaranteed Consistency**: Thread-safe operations
- **No Network Dependencies**: Pure in-memory operation

### Redis Implementation
- **Graceful Degradation**: Treats Redis errors as "not seen"
- **Error Monitoring**: Counts and logs errors for observability
- **Timeout Protection**: 2-second timeout prevents hanging
- **Connection Recovery**: Automatic reconnection on network issues

## TTL and Expiration

### Redis TTL Benefits
- **Memory Management**: Automatic cleanup of old entries
- **Freshness Control**: Ensures recent data processing
- **Storage Efficiency**: Prevents unbounded Redis growth

### TTL Configuration Examples
```go
// Hourly deduplication
deduper, _ := dedup.NewRedis("redis:6379", 1*time.Hour)

// Daily deduplication
deduper, _ := dedup.NewRedis("redis:6379", 24*time.Hour)

// Weekly deduplication
deduper, _ := dedup.NewRedis("redis:6379", 7*24*time.Hour)
```

## Integration Patterns

### Probe Pipeline Integration
```go
type ProbeConfig struct {
    Deduper dedup.Interface
}

func (p *Probe) ProcessDomain(domain string) {
    if p.Deduper.Seen(domain) {
        return // Skip already processed domain
    }
    // Continue with domain processing
}
```

### Configuration-Driven Selection
```go
func NewDeduper(redisAddr string, ttl time.Duration) dedup.Interface {
    if redisAddr != "" {
        if redis, err := dedup.NewRedis(redisAddr, ttl); err == nil {
            return redis
        }
        log.Println("Redis unavailable, falling back to memory")
    }
    return dedup.NewMemory()
}
```

## Monitoring and Observability

### Memory Implementation Metrics
- **Total Keys**: Number of unique keys stored
- **Memory Usage**: Approximate memory consumption
- **Hit Rate**: Percentage of keys that were already seen

### Redis Implementation Metrics
- **Connection Status**: Redis server connectivity
- **Error Rate**: Redis operation failure rate
- **Response Latency**: Redis operation timing
- **Key Count**: Number of active keys in Redis
- **Memory Usage**: Redis server memory consumption

## Common Use Cases

### Domain Deduplication
```go
// Prevent processing same domain multiple times
if !deduper.Seen("domain:"+domain) {
    processDomain(domain)
}
```

### URL Deduplication
```go
// Prevent fetching same URL multiple times
urlKey := "url:" + url
if !deduper.Seen(urlKey) {
    fetchAndProcessURL(url)
}
```

### Batch Deduplication
```go
// Process only new domains from a batch
var newDomains []string
for _, domain := range domains {
    if !deduper.Seen("batch:"+domain) {
        newDomains = append(newDomains, domain)
    }
}
processBatch(newDomains)
```

## Best Practices

### Key Naming Conventions
- **Use Prefixes**: Namespace keys to avoid collisions
- **Consistent Format**: Use consistent key formatting
- **Descriptive Names**: Include context in key names

### Error Handling
- **Graceful Degradation**: Never block on deduplication failures
- **Monitoring**: Track error rates and patterns
- **Fallback Strategy**: Consider memory fallback for Redis failures

### Performance Optimization
- **Batch Operations**: Group multiple checks when possible
- **Key Length**: Keep keys reasonably short for memory efficiency
- **TTL Selection**: Balance freshness needs with performance

## Security Considerations

### Redis Security
- **Authentication**: Use Redis AUTH for production deployments
- **Network Security**: Secure Redis network communications
- **Access Control**: Limit Redis access to probe processes only

### Data Privacy
- **Key Content**: Be mindful of sensitive data in keys
- **TTL Compliance**: Ensure TTL meets data retention policies
- **Access Logging**: Monitor deduplication key access patterns

## Troubleshooting

### Common Issues
- **Redis Connectivity**: Network or authentication failures
- **Memory Growth**: Unbounded memory usage in Memory implementation
- **Key Collisions**: Different operations using same keys

### Debugging Steps
1. **Check Redis Connection**: Verify Redis server availability
2. **Monitor Error Logs**: Review Redis operation error messages
3. **Analyze Key Patterns**: Examine key naming and distribution
4. **Performance Analysis**: Monitor operation latency and throughput