# Rate Limiting Component

The rate limiting component (`internal/rate`) provides per-host rate limiting to ensure respectful probing and prevent overwhelming target servers.

## Overview

The rate limiting component implements a token bucket rate limiter with per-host isolation, automatic cleanup, and configurable burst capacity. It ensures SPYDER operates as a responsible internet citizen by respecting server capacity and preventing abuse.

## Core Structure

### `PerHost`

Main rate limiting structure that manages per-host limiters:

```go
type PerHost struct {
    mu         sync.Mutex           // Thread-safe access protection
    m          map[string]*limitEntry // Per-host limiter storage
    perSecond  float64              // Requests per second rate
    burst      int                  // Burst capacity
    maxEntries int                  // Maximum stored entries (10,000)
}
```

### `limitEntry`

Individual host rate limiting entry:

```go
type limitEntry struct {
    limiter  *rate.Limiter  // Token bucket limiter for the host
    lastUsed time.Time      // Last access time for cleanup
}
```

## Core Functions

### `New(perSecond float64, burst int) *PerHost`

Creates a new per-host rate limiter with automatic cleanup.

**Parameters:**
- `perSecond`: Maximum requests per second per host
- `burst`: Maximum burst capacity per host

**Returns:**
- `*PerHost`: Configured rate limiter instance

**Features:**
- **Automatic Cleanup**: Starts background goroutine for memory management
- **Memory Protection**: Limits maximum entries to 10,000 hosts
- **Thread Safety**: Mutex-protected concurrent access

### `Allow(host string) bool`

Checks if a request is allowed under the rate limit without blocking.

**Parameters:**
- `host`: The target hostname for rate limiting

**Returns:**
- `bool`: `true` if request is allowed, `false` if rate limited

**Behavior:**
- **Immediate Response**: Non-blocking check
- **Token Consumption**: Consumes token if available
- **Lazy Initialization**: Creates limiter entry if not exists

### `Wait(host string)`

Blocks until a request token becomes available for the host.

**Parameters:**
- `host`: The target hostname for rate limiting

**Behavior:**
- **Blocking Operation**: Waits until token is available
- **Guaranteed Execution**: Always allows request after wait
- **Context-Free**: Uses background context for waiting

## Rate Limiting Algorithm

### Token Bucket Implementation
- **Algorithm**: Uses `golang.org/x/time/rate` token bucket
- **Token Refill**: Continuous refill at specified rate
- **Burst Handling**: Allows bursts up to configured capacity
- **Precision**: Supports fractional requests per second

### Per-Host Isolation
- **Independent Limits**: Each host has its own rate limiter
- **No Cross-Contamination**: One host's rate limiting doesn't affect others
- **Dynamic Creation**: Limiters created on first access per host

## Automatic Cleanup System

### Background Cleanup Process
```go
func (p *PerHost) cleanup() {
    ticker := time.NewTicker(5 * time.Minute) // Every 5 minutes
    // Remove entries older than 1 hour when exceeding maxEntries
}
```

### Cleanup Triggers
- **Time-Based**: Runs every 5 minutes
- **Memory-Based**: Only cleans when exceeding 10,000 entries
- **Age-Based**: Removes entries unused for over 1 hour

### Memory Management
- **Prevents Memory Leaks**: Removes unused host entries
- **Production Ready**: Handles long-running operation scenarios
- **Configurable Limits**: Maximum 10,000 concurrent host entries

## Thread Safety

### Concurrent Access Protection
- **Mutex Locking**: Protects map operations with mutex
- **Read/Write Consistency**: Ensures consistent limiter state
- **Race Condition Prevention**: Safe for concurrent goroutine access

### Lock Optimization
- **Minimal Lock Duration**: Releases lock before token bucket operations
- **Per-Host Granularity**: Independent limiters reduce contention
- **Lazy Initialization**: Creates entries only when needed

## Integration Points

### Probe Pipeline Integration
1. **Pre-Request Check**: `Allow()` for immediate rate limit checking
2. **Blocking Wait**: `Wait()` for guaranteed request execution
3. **Host-Based**: Applied per target hostname

### Configuration Integration
- **Rate Configuration**: Configurable via probe settings
- **Burst Configuration**: Adjustable burst capacity per deployment
- **Cleanup Tuning**: Fixed cleanup intervals for production stability

## Performance Considerations

### Memory Usage
- **Per-Host Storage**: Memory usage scales with unique hosts
- **Automatic Cleanup**: Prevents unlimited memory growth
- **Lightweight Entries**: Minimal memory footprint per host

### CPU Usage
- **Efficient Algorithms**: Uses optimized token bucket implementation
- **Background Cleanup**: Minimal CPU overhead for maintenance
- **Lock Contention**: Minimal due to per-host isolation

## Use Cases

### Respectful Probing
```go
limiter := rate.New(1.0, 3)  // 1 req/sec, burst of 3
if limiter.Allow("example.com") {
    // Make request immediately
} else {
    // Rate limited, handle accordingly
}
```

### Guaranteed Execution
```go
limiter := rate.New(0.5, 1)  // 0.5 req/sec, burst of 1
limiter.Wait("example.com")  // Wait for token
// Request is guaranteed to be allowed
```

## Configuration Examples

### Conservative Settings
```go
rate.New(0.1, 1)  // 1 request per 10 seconds, no burst
```

### Standard Settings
```go
rate.New(1.0, 3)  // 1 request per second, burst of 3
```

### Aggressive Settings
```go
rate.New(10.0, 20)  // 10 requests per second, burst of 20
```

## Error Handling

### Graceful Degradation
- **No Error Returns**: Rate limiting always succeeds
- **Blocking Behavior**: `Wait()` blocks until success
- **Immediate Feedback**: `Allow()` provides immediate status

### Resource Management
- **Memory Limits**: Automatic cleanup prevents resource exhaustion
- **Goroutine Management**: Single cleanup goroutine per limiter instance
- **Clean Shutdown**: Cleanup goroutine terminates with limiter

## Monitoring Metrics

Rate limiting should be monitored for:
- **Rate Limit Hit Rate**: Percentage of requests that are rate limited
- **Average Wait Time**: Time spent waiting for rate limit clearance
- **Active Host Count**: Number of hosts currently being rate limited
- **Memory Usage**: Memory consumption of rate limiter storage

## Best Practices

### Rate Selection
- **Server Respect**: Choose rates that respect target server capacity
- **Network Conditions**: Consider network latency and server response times
- **Burst Sizing**: Configure burst to handle legitimate traffic spikes

### Host Management
- **Hostname Consistency**: Use consistent hostname formats for effective limiting
- **Apex vs Subdomain**: Consider whether to limit by apex domain or individual hosts
- **DNS Resolution**: Apply rate limiting after DNS resolution to actual target hosts

## Security Considerations

### DoS Prevention
- **Self-Protection**: Prevents SPYDER from overwhelming target servers
- **Reputation Protection**: Maintains good internet citizenship
- **Compliance**: Helps comply with terms of service and robots.txt

### Resource Protection
- **Memory Bounds**: Automatic cleanup prevents memory exhaustion attacks
- **CPU Bounds**: Efficient algorithms prevent CPU exhaustion
- **Goroutine Bounds**: Single cleanup goroutine prevents goroutine leaks

## Troubleshooting

### Common Issues
- **Rate Too High**: Servers returning errors or blocking requests
- **Rate Too Low**: Probe performance slower than expected
- **Memory Growth**: Cleanup not removing old entries effectively

### Debugging Steps
1. **Monitor Rate Limit Hits**: Check how often rate limits are triggered
2. **Server Response Analysis**: Monitor target server response patterns
3. **Memory Usage Tracking**: Watch rate limiter memory consumption
4. **Performance Profiling**: Analyze impact on overall probe performance

## Advanced Configuration

### Dynamic Rate Adjustment
While not built-in, rate limits can be adjusted by:
- Creating new rate limiter instances
- Implementing rate adjustment based on server response patterns
- Using different rates for different host categories

### Integration with Circuit Breakers
- Rate limiting complements circuit breaker functionality
- Provides primary request throttling
- Circuit breakers handle failure scenarios
- Together they provide comprehensive traffic control