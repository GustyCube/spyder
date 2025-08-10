# Queue System Component

The queue system component (`internal/queue`) provides distributed work queue functionality for coordinating domain processing across multiple SPYDER instances.

## Overview

The queue system enables distributed domain processing using Redis as a reliable work queue backend. It provides lease-based processing with acknowledgment to ensure domains are processed exactly once even in failure scenarios.

## Core Structure

### `RedisQueue`

Redis-based distributed queue implementation:

```go
type RedisQueue struct {
    cli      *redis.Client  // Redis client connection
    queueKey string        // Redis key for main queue
    procKey  string        // Redis key for processing queue
    leaseTTL time.Duration // Time-to-live for leased items
}
```

### `item`

Internal queue item representation:

```go
type item struct {
    Host    string `json:"host"`     // Domain hostname
    TS      int64  `json:"ts"`       // Timestamp when added
    Attempt int    `json:"attempt"`  // Processing attempt count
}
```

## Core Functions

### `NewRedis(addr, key string, lease time.Duration) (*RedisQueue, error)`

Creates a new Redis-based queue with lease functionality.

**Parameters:**
- `addr`: Redis server address (e.g., "127.0.0.1:6379")
- `key`: Base queue key name in Redis
- `lease`: Lease duration for processing items

**Returns:**
- `*RedisQueue`: Configured Redis queue instance
- `error`: Connection error if Redis is unreachable

**Queue Structure:**
- **Main Queue**: `{key}` - holds pending domains
- **Processing Queue**: `{key}:processing` - holds leased domains

### `Lease(ctx context.Context) (string, func() error, error)`

Atomically leases a domain for processing with acknowledgment function.

**Parameters:**
- `ctx`: Context for timeout and cancellation control

**Returns:**
- `string`: Domain hostname to process (empty if no work available)
- `func() error`: Acknowledgment function to call when processing completes
- `error`: Operation error

**Operation Details:**
- **Atomic Transfer**: Uses `BRPOPLPUSH` for atomic queue-to-processing transfer
- **Blocking Operation**: Blocks up to 5 seconds waiting for work
- **JSON Deserialization**: Parses queue item to extract hostname
- **Acknowledgment**: Returned function removes item from processing queue

### `Seed(ctx context.Context, host string) error`

Adds a new domain to the processing queue.

**Parameters:**
- `ctx`: Context for timeout and cancellation
- `host`: Domain hostname to add to queue

**Returns:**
- `error`: Redis operation error

**Item Creation:**
- **Timestamp**: Records current UTC timestamp
- **Attempt Counter**: Initializes attempt count to 0
- **JSON Serialization**: Stores as JSON for cross-language compatibility

## Queue Mechanics

### Lease-Based Processing

#### Work Distribution Flow
1. **Lease Request**: Worker calls `Lease()` to get work
2. **Atomic Transfer**: Redis moves item from main queue to processing queue
3. **Work Processing**: Worker processes the leased domain
4. **Acknowledgment**: Worker calls ACK function to remove from processing queue
5. **Failure Handling**: Unacknowledged items remain in processing queue

#### Reliability Features
- **Exactly-Once Processing**: Items processed by only one worker
- **Failure Recovery**: Failed workers leave items in processing queue
- **Timeout Handling**: Context cancellation prevents indefinite blocking
- **Graceful Degradation**: Returns empty results when no work available

### Redis Operations

#### Queue Operations
```redis
# Add domain to queue (LPUSH)
LPUSH spyder:queue '{"host":"example.com","ts":1640995200,"attempt":0}'

# Lease domain for processing (BRPOPLPUSH)
BRPOPLPUSH spyder:queue spyder:queue:processing 5

# Acknowledge completion (LREM)
LREM spyder:queue:processing 1 '{"host":"example.com","ts":1640995200,"attempt":0}'
```

## Integration Patterns

### Worker Pool Integration
```go
func (w *Worker) ProcessDomains(queue *queue.RedisQueue) {
    for {
        host, ack, err := queue.Lease(ctx)
        if err != nil {
            log.Printf("Queue lease error: %v", err)
            continue
        }
        if host == "" {
            time.Sleep(1 * time.Second) // No work available
            continue
        }
        
        // Process domain
        if err := w.ProcessDomain(host); err != nil {
            log.Printf("Processing error for %s: %v", host, err)
        }
        
        // Acknowledge completion
        if err := ack(); err != nil {
            log.Printf("ACK error for %s: %v", host, err)
        }
    }
}
```

### Multi-Instance Coordination
```go
// Instance A seeds domains
for _, domain := range domains {
    queue.Seed(ctx, domain)
}

// Instances B, C, D process domains
host, ack, err := queue.Lease(ctx)
// Each instance gets different domains automatically
```

## Error Handling

### Connection Errors
- **Initialization Failure**: Returns error if Redis unreachable
- **Operation Failure**: Propagates Redis errors to caller
- **Timeout Handling**: Respects context timeouts and cancellation

### Processing Errors
- **Empty Queue**: Returns empty string when no work available
- **JSON Errors**: Returns error if item deserialization fails
- **ACK Errors**: Acknowledgment function can return Redis errors

### Recovery Mechanisms
- **Retry Logic**: Failed items remain in processing queue for retry
- **Dead Letter Handling**: Manual cleanup of stalled processing items
- **Connection Recovery**: Redis client handles automatic reconnection

## Performance Considerations

### Throughput Characteristics
- **Blocking Operations**: `BRPOPLPUSH` blocks for up to 5 seconds
- **Network Latency**: Each operation requires Redis round-trip
- **Serialization Overhead**: JSON encoding/decoding per item
- **Atomic Guarantees**: Operations are atomic but may have higher latency

### Scalability Features
- **Horizontal Scaling**: Multiple workers can process same queue
- **Load Distribution**: Redis automatically distributes work across workers
- **Memory Efficiency**: Items stored as compressed JSON strings

## Monitoring and Observability

### Queue Metrics
- **Queue Length**: Number of pending items in main queue
- **Processing Length**: Number of items currently being processed
- **Throughput**: Items processed per second across all workers
- **Lease Duration**: Time between lease and acknowledgment

### Redis Monitoring
```redis
# Check queue lengths
LLEN spyder:queue
LLEN spyder:queue:processing

# Inspect queue contents
LRANGE spyder:queue 0 10
LRANGE spyder:queue:processing 0 10
```

## Common Use Cases

### Distributed Web Crawling
```go
// Seed initial domains
for _, domain := range initialDomains {
    queue.Seed(ctx, domain)
}

// Workers lease and process domains
for {
    host, ack, err := queue.Lease(ctx)
    if host != "" {
        crawlDomain(host)
        ack()
    }
}
```

### Batch Processing
```go
// Enqueue batch of domains
for _, domain := range batch {
    queue.Seed(ctx, domain)
}

// Process until queue is empty
for {
    host, ack, err := queue.Lease(ctx)
    if host == "" {
        break // Queue is empty
    }
    processDomain(host)
    ack()
}
```

## Configuration Recommendations

### Lease Duration
- **Short Lease**: Fast failure detection, more Redis traffic
- **Long Lease**: Slower failure detection, less Redis traffic
- **Typical Range**: 30 seconds to 5 minutes

### Queue Naming
- **Environment Prefix**: `prod:queue`, `dev:queue`
- **Service Prefix**: `spyder:domains`, `crawler:urls`
- **Version Suffix**: `queue:v1`, `queue:v2`

## Security Considerations

### Redis Security
- **Authentication**: Use Redis AUTH for production
- **Network Security**: Secure Redis network communications
- **Access Control**: Limit Redis access to queue operations only

### Data Integrity
- **Validation**: Validate domain formats before queuing
- **Sanitization**: Sanitize domains to prevent injection
- **Monitoring**: Monitor for malicious or malformed domains

## Troubleshooting

### Common Issues
- **Stalled Processing**: Items stuck in processing queue
- **Queue Overflow**: Memory issues from large queues
- **Connection Loss**: Redis connectivity problems

### Debugging Commands
```redis
# Check queue status
INFO memory
LLEN spyder:queue
LLEN spyder:queue:processing

# Clear stalled processing items (caution: may cause duplication)
DEL spyder:queue:processing

# View queue contents
LRANGE spyder:queue 0 -1
```

### Recovery Procedures
1. **Identify Stalled Items**: Check processing queue age
2. **Manual Cleanup**: Remove items stuck longer than lease TTL
3. **Re-queue Items**: Move processing items back to main queue if needed
4. **Monitor Recovery**: Watch for duplicate processing during recovery