# HTTP Client Component

The HTTP client component (`internal/httpclient`) provides resilient HTTP connectivity with circuit breaker protection for robust web content fetching.

## Overview

The HTTP client component delivers production-ready HTTP functionality with automatic failure detection, circuit breaker protection, and optimized connection management for high-volume domain probing.

## Core Components

### Standard HTTP Client

#### `Default() *http.Client`

Creates a production-optimized HTTP client with carefully tuned settings:

**Transport Configuration:**
- **TLS Verification**: Enabled (`InsecureSkipVerify: false`)
- **Compression**: Enabled for bandwidth efficiency
- **Connection Pooling**: Up to 1024 idle connections total
- **Per-Host Limits**: 128 connections, 64 idle per host
- **Timeouts**: 
  - Response headers: 10 seconds
  - Idle connection: 30 seconds
  - Expect-Continue: 1 second
  - Overall request: 15 seconds

### Resilient Client

#### `ResilientClient`

Wraps the standard HTTP client with circuit breaker functionality for handling unreliable hosts:

```go
type ResilientClient struct {
    client      *http.Client
    hostBreaker *circuitbreaker.HostBreaker
}
```

#### `NewResilientClient(client *http.Client) *ResilientClient`

Creates a resilient client with circuit breaker protection:

**Circuit Breaker Configuration:**
- **Max Requests**: 3 requests allowed in half-open state
- **Interval**: 60 seconds monitoring window
- **Timeout**: 30 seconds before attempting recovery
- **Threshold**: 5 failures required to trip circuit
- **Failure Ratio**: 0.6 (60%) failure rate threshold

## Core Methods

### `Do(req *http.Request) (*http.Response, error)`

Executes HTTP requests with circuit breaker protection:

**Features:**
- Host-based circuit breaking
- Automatic failure detection (5xx responses and connection errors)
- Per-host state management
- Error classification and handling

### `Get(url string) (*http.Response, error)`

Convenience method for GET requests with circuit breaker protection.

### `GetWithContext(ctx context.Context, url string) (*http.Response, error)`

Context-aware GET requests supporting timeout and cancellation.

## Circuit Breaker States

### Closed State (Normal Operation)
- Requests pass through normally
- Failure counting active
- Transitions to Open on threshold breach

### Open State (Protecting Backend)
- All requests fail immediately
- No actual HTTP requests made
- Automatic transition to Half-Open after timeout

### Half-Open State (Testing Recovery)
- Limited requests allowed through
- Success resets to Closed state
- Failure returns to Open state

## Error Handling

### HTTP Error Types

#### `HTTPError`
Custom error type for HTTP response errors:

```go
type HTTPError struct {
    StatusCode int
    Status     string
}
```

### Error Classification

**Circuit Breaker Failures:**
- Connection timeouts
- Connection refused
- DNS resolution failures
- 5xx HTTP status codes (server errors)

**Non-Breaking Errors:**
- 4xx HTTP status codes (client errors)
- Redirect responses (3xx)
- Successful responses (2xx)

## Monitoring and Diagnostics

### `Stats() map[string]struct{...}`

Returns circuit breaker statistics for all hosts:

```go
{
    State    string  // "closed", "open", "half-open"
    Requests uint32  // Total requests attempted
    Failures uint32  // Total failures recorded
}
```

### `ResetBreaker(host string)`

Manually resets circuit breaker for a specific host.

## Integration Points

### Probe Pipeline Integration
1. **Input**: HTTP requests from content fetching operations
2. **Processing**: Circuit breaker evaluation and HTTP execution
3. **Output**: HTTP responses or failure notifications

### Configuration Integration
- Inherits timeout settings from probe configuration
- Respects context cancellation from pipeline
- Integrates with rate limiting controls

## Performance Considerations

### Connection Management
- Persistent connections reduce handshake overhead
- Connection pooling optimizes resource usage
- Per-host connection limits prevent resource exhaustion

### Circuit Breaker Benefits
- Prevents cascade failures to unhealthy hosts
- Reduces wasted resources on failing endpoints
- Automatic recovery without manual intervention

## Security Features

### TLS Configuration
- Certificate verification enabled by default
- Modern TLS protocol support
- Secure cipher suite selection

### Request Isolation
- Per-host circuit breaker state
- No cross-host contamination
- Independent failure tracking

## Common Use Cases

### Content Fetching
```go
client := httpclient.NewResilientClient(nil)
resp, err := client.GetWithContext(ctx, "https://example.com")
```

### Link Discovery
- Fetch HTML content for link extraction
- Handle unreliable or overloaded servers
- Maintain high throughput with failure resilience

### TLS Certificate Analysis
- Establish TLS connections for certificate inspection
- Handle certificate errors gracefully
- Support both HTTP and HTTPS endpoints

## Configuration Tuning

### Connection Pool Sizing
- **MaxIdleConns**: Total idle connections (default: 1024)
- **MaxConnsPerHost**: Per-host connection limit (default: 128)
- **MaxIdleConnsPerHost**: Per-host idle connections (default: 64)

### Timeout Tuning
- **ResponseHeaderTimeout**: Header read timeout (default: 10s)
- **Overall Timeout**: Complete request timeout (default: 15s)
- **IdleConnTimeout**: Idle connection cleanup (default: 30s)

### Circuit Breaker Tuning
- **Threshold**: Failures required to open circuit (default: 5)
- **FailureRatio**: Failure percentage threshold (default: 60%)
- **Interval**: Monitoring window duration (default: 60s)
- **Timeout**: Recovery attempt delay (default: 30s)

## Error Recovery Strategies

### Automatic Recovery
- Circuit breakers automatically attempt recovery
- Gradual request flow restoration
- Self-healing behavior without intervention

### Manual Recovery
- Reset specific host circuit breakers
- Force recovery for critical endpoints
- Diagnostic information for troubleshooting

## Best Practices

### Client Lifecycle
- Reuse client instances for connection pooling benefits
- Configure timeouts appropriate for use case
- Monitor circuit breaker statistics for health assessment

### Error Handling
- Check for `HTTPError` types for HTTP-specific handling
- Distinguish between temporary and permanent failures
- Implement appropriate retry strategies at higher levels