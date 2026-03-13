# Queue Protocol

The queue protocol (`internal/queue`) provides distributed work coordination for SPYDER probes using Redis as a reliable message broker. It implements a lease/acknowledge pattern that guarantees each domain is processed by exactly one worker, even across multiple probe instances.

## Architecture

SPYDER uses two Redis lists to manage work distribution:

```
                  LPUSH (seed)
                      |
                      v
            +-------------------+
            |   spyder:queue    |  <-- Pending work items
            +-------------------+
                      |
               BRPopLPush (lease)
                      |
                      v
            +-------------------+
            | spyder:queue:     |  <-- Items being processed
            |   processing      |
            +-------------------+
                      |
                LRem (ack)
                      |
                      v
                  [removed]
```

- **Main queue** (`spyder:queue`): Holds domains waiting to be processed. New work enters here.
- **Processing queue** (`spyder:queue:processing`): Holds domains that have been leased to a worker. Items stay here until acknowledged.

## Queue Item Format

Each queue item is a JSON object stored as a Redis string value:

```json
{"host": "example.com", "ts": 1705312200, "attempt": 0}
```

| Field | Type | Description |
|-------|------|-------------|
| `host` | `string` | Domain hostname to process (lowercased, trailing dot stripped) |
| `ts` | `int64` | Unix timestamp (UTC) when the item was enqueued |
| `attempt` | `int` | Processing attempt counter (starts at 0) |

## Queue Key Structure

The queue uses a configurable base key (default: `spyder:queue`) with a fixed suffix for the processing list:

| Key | Default Value | Purpose |
|-----|---------------|---------|
| Base key | `spyder:queue` | Main pending queue |
| Processing key | `spyder:queue:processing` | In-flight items |

Configure the base key with the `-redis_queue_key` flag or `redis_queue_key` config field. The processing key is always `{base_key}:processing`.

```bash
# Custom queue key
./bin/spyder -domains=domains.txt \
  -redis_queue_addr=127.0.0.1:6379 \
  -redis_queue_key=prod:spyder:queue
```

This creates keys `prod:spyder:queue` and `prod:spyder:queue:processing`.

## Lease/Ack Pattern

### Lease Operation

A worker calls `Lease(ctx)` to atomically pop an item from the main queue and push it onto the processing queue. This uses Redis `BRPopLPush`, which blocks for up to 5 seconds waiting for work.

```go
host, ack, err := q.Lease(ctx)
if err != nil {
    log.Error("queue lease failed", "err", err)
    continue
}
if host == "" {
    // No work available, BRPopLPush timed out after 5s
    continue
}
```

**Redis command executed:**
```redis
BRPOPLPUSH spyder:queue spyder:queue:processing 5
```

The `BRPopLPush` operation is atomic -- the item is removed from the main queue and added to the processing queue in a single Redis transaction. No other worker can lease the same item.

### Ack Operation

After processing completes, the worker calls the returned `ack()` function to remove the item from the processing queue:

```go
// Process the domain
err = processDomain(host)

// Acknowledge completion
if err := ack(); err != nil {
    log.Error("ack failed", "host", host, "err", err)
}
```

**Redis command executed:**
```redis
LREM spyder:queue:processing 1 '{"host":"example.com","ts":1705312200,"attempt":0}'
```

The `LREM` removes exactly one occurrence of the raw JSON string from the processing list. The ack function captures the original JSON bytes from the lease, so the removal matches exactly.

### Complete Worker Loop

This is how SPYDER's main loop leases and processes domains from the queue (from `cmd/spyder/main.go`):

```go
go func() {
    defer close(tasks)
    for {
        select {
        case <-ctx.Done():
            return
        default:
            host, ack, err := q.Lease(ctx)
            if err != nil {
                continue
            }
            if host == "" {
                continue
            }
            tasks <- host
            _ = ack()
        }
    }
}()
```

## Seed Utility

The `cmd/seed` tool populates the queue from a file of domains. Each line in the file becomes a queue item with the current UTC timestamp and attempt counter of 0.

### Usage

```bash
go run ./cmd/seed \
  -domains=domains.txt \
  -redis=127.0.0.1:6379 \
  -key=spyder:queue
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-domains` | (required) | Path to newline-separated domains file |
| `-redis` | `127.0.0.1:6379` | Redis server address |
| `-key` | `spyder:queue` | Redis queue key name |

### Input File Format

```text
# Comments are skipped
example.com
www.example.org
subdomain.example.net.
```

Lines are trimmed, lowercased, and trailing dots are stripped. Empty lines and lines starting with `#` are ignored.

### Seeding from the Command Line

You can also seed the queue directly with `redis-cli`:

```bash
# Seed a single domain
redis-cli LPUSH spyder:queue '{"host":"example.com","ts":1705312200,"attempt":0}'

# Seed multiple domains from a file
while IFS= read -r domain; do
  domain=$(echo "$domain" | tr '[:upper:]' '[:lower:]' | sed 's/\.$//')
  [ -z "$domain" ] && continue
  [[ "$domain" == \#* ]] && continue
  ts=$(date +%s)
  redis-cli LPUSH spyder:queue "{\"host\":\"$domain\",\"ts\":$ts,\"attempt\":0}"
done < domains.txt
```

## Failure Recovery

### Unacknowledged Items

If a worker crashes or is terminated before calling `ack()`, the item remains in the processing queue. These items are not automatically retried -- they require manual intervention or an external recovery process.

### Inspecting Stalled Items

```bash
# Check how many items are stuck in processing
redis-cli LLEN spyder:queue:processing

# View items currently being processed
redis-cli LRANGE spyder:queue:processing 0 -1

# View the raw JSON of stuck items
redis-cli LRANGE spyder:queue:processing 0 10
```

### Manual Recovery

Move stalled items back to the main queue for reprocessing:

```bash
# Move all processing items back to the main queue
while true; do
  item=$(redis-cli RPOPLPUSH spyder:queue:processing spyder:queue)
  [ -z "$item" ] && break
done

# Nuclear option: delete the entire processing queue (items are lost)
redis-cli DEL spyder:queue:processing
```

### Dead Letter Handling

SPYDER does not have a built-in dead letter queue. For production deployments, consider implementing an external process that periodically checks the processing queue for items older than the lease TTL and either re-queues them or moves them to a dead letter key:

```bash
# Example: find items older than 5 minutes in processing queue
# and move them back to the main queue
redis-cli LRANGE spyder:queue:processing 0 -1 | while read -r item; do
  ts=$(echo "$item" | jq -r '.ts')
  now=$(date +%s)
  age=$((now - ts))
  if [ "$age" -gt 300 ]; then
    redis-cli LREM spyder:queue:processing 1 "$item"
    redis-cli LPUSH spyder:queue "$item"
  fi
done
```

## Monitoring Queue Depth

### Key Metrics

```bash
# Pending items (waiting to be leased)
redis-cli LLEN spyder:queue

# In-flight items (leased, not yet acknowledged)
redis-cli LLEN spyder:queue:processing

# Total items across both queues
echo $(( $(redis-cli LLEN spyder:queue) + $(redis-cli LLEN spyder:queue:processing) ))
```

### Continuous Monitoring

```bash
# Watch queue depth every 2 seconds
watch -n 2 'echo "pending: $(redis-cli LLEN spyder:queue)  processing: $(redis-cli LLEN spyder:queue:processing)"'
```

### Prometheus Integration

SPYDER does not directly export queue depth as a Prometheus metric, but you can use a Redis exporter to monitor list lengths:

```yaml
# prometheus.yml - using redis_exporter
scrape_configs:
  - job_name: 'redis'
    static_configs:
      - targets: ['redis-exporter:9121']
```

Then query:
```promql
redis_list_length{key="spyder:queue"}
redis_list_length{key="spyder:queue:processing"}
```

## Enabling the Redis Queue

The Redis queue is activated by setting the `REDIS_QUEUE_ADDR` environment variable or the `redis_queue_addr` config field. When not set, SPYDER reads domains from the file specified by `-domains` and processes them in a single pass.

```bash
# Enable Redis queue mode
export REDIS_QUEUE_ADDR=127.0.0.1:6379
./bin/spyder -domains=domains.txt

# Or via config file
cat > config.yaml <<EOF
domains: domains.txt
redis_queue_addr: "127.0.0.1:6379"
redis_queue_key: "spyder:queue"
EOF
./bin/spyder -config=config.yaml
```

### Continuous Mode with Redis Queue

When both `-continuous` and a Redis queue are configured, newly discovered domains are fed back into the queue for recursive crawling:

```bash
export REDIS_QUEUE_ADDR=127.0.0.1:6379
./bin/spyder -domains=seed.txt -continuous -max_domains=10000
```

This seeds the queue with domains from `seed.txt`, and as the probe discovers new domains (via DNS records and HTML links), they are pushed back into the Redis queue for processing by any available worker.

## Configuration Reference

| Setting | Default | Description |
|---------|---------|-------------|
| `REDIS_QUEUE_ADDR` env | `""` (disabled) | Redis server address for queue |
| `redis_queue_key` config / CLI | `spyder:queue` | Base key name in Redis |
| Lease timeout | 5 seconds | `BRPopLPush` blocking duration |
| Lease TTL | 120 seconds | Passed to `NewRedis` in main |

## Redis Connection

The queue connects to Redis using the `go-redis/v9` client with default settings. The connection is validated on startup with a `PING` command -- if Redis is unreachable, SPYDER exits with a fatal error.

```go
cli := redis.NewClient(&redis.Options{Addr: addr})
if err := cli.Ping(context.Background()).Err(); err != nil {
    return nil, err  // Fatal in main.go
}
```

No authentication, TLS, or connection pooling options are exposed through the queue interface. For secured Redis deployments, consider using a local stunnel or Redis proxy.
