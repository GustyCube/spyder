# Distributed Mode Deployment

This guide covers deploying SPYDER as a multi-instance distributed system using Redis as a shared work queue. Distributed mode lets you scale horizontally across many machines, each running independent probe instances that coordinate through Redis.

## Architecture Overview

In distributed mode, SPYDER instances share work through a Redis-backed queue rather than reading from a local domains file. The architecture consists of three components:

1. **Redis work queue** -- a shared FIFO list that holds domains to crawl
2. **Seed utility** (`cmd/seed`) -- a CLI tool that pushes initial domains into the queue
3. **Probe instances** -- one or more `spyder` processes that lease domains from the queue, crawl them, and (in continuous mode) push discovered domains back

```
                    ┌──────────────┐
                    │  cmd/seed    │
                    │  (one-shot)  │
                    └──────┬───────┘
                           │ LPUSH
                           ▼
                    ┌──────────────┐
              ┌────▶│    Redis     │◀────┐
              │     │  spyder:queue│     │
              │     └──────────────┘     │
              │            │             │
         LPUSH (new       BRPopLPush    LPUSH (new
         discoveries)      │            discoveries)
              │            │             │
        ┌─────┴──┐   ┌────┴───┐   ┌────┴─────┐
        │ Probe  │   │ Probe  │   │ Probe    │
        │  ID=1  │   │  ID=2  │   │  ID=N    │
        └────────┘   └────────┘   └──────────┘
```

Each probe instance calls `BRPopLPush` to atomically pop a domain from the queue into a processing list, crawl it, then acknowledge completion by removing it from the processing list. This gives at-least-once delivery semantics -- if a probe crashes mid-crawl, the item remains in the processing list for recovery.

## Prerequisites

- Redis 6.0 or later (Redis 7 recommended)
- Two or more machines with network access to the Redis instance
- SPYDER binary or Docker image on each machine
- Shared Redis for both deduplication (`REDIS_ADDR`) and work queue (`REDIS_QUEUE_ADDR`) -- these can be the same or separate Redis instances

## Redis Queue Setup

### Install and Configure Redis

The queue Redis instance should be tuned for reliability rather than pure speed. Enable persistence so the queue survives restarts:

```bash
# /etc/redis/redis-queue.conf
bind 0.0.0.0
port 6379
protected-mode no
requirepass your-secret-password

# Persistence -- AOF gives best durability for queue data
appendonly yes
appendfsync everysec
auto-aof-rewrite-percentage 100
auto-aof-rewrite-min-size 64mb

# Memory -- queue items are small, 1GB is plenty for millions of domains
maxmemory 1gb
maxmemory-policy noeviction

# Timeout -- keep connections alive for long-polling probes
timeout 0
tcp-keepalive 300
```

Start Redis with this config:

```bash
redis-server /etc/redis/redis-queue.conf
```

### Environment Variables

SPYDER reads two environment variables for queue configuration:

| Variable | Default | Description |
|---|---|---|
| `REDIS_QUEUE_ADDR` | (none) | Redis address for the work queue (e.g., `10.0.1.5:6379`). When set, SPYDER reads from Redis instead of a local file. |
| `REDIS_QUEUE_KEY` | `spyder:queue` | The Redis list key used as the work queue. |
| `REDIS_ADDR` | (none) | Redis address for deduplication. In distributed mode this should point to the same (or a shared) Redis so all probes share dedup state. |

When `REDIS_QUEUE_ADDR` is set, SPYDER ignores the `-domains` flag and instead enters a blocking loop that leases domains from the Redis queue until the context is cancelled.

### Verify Redis Connectivity

```bash
# From each probe machine
redis-cli -h 10.0.1.5 -p 6379 PING
# Expected: PONG

# Check queue length
redis-cli -h 10.0.1.5 -p 6379 LLEN spyder:queue
# Expected: (integer) 0
```

## Seeding the Queue

The `cmd/seed` utility reads a domains file and pushes each domain into the Redis queue. Build it from source:

```bash
go build -o seed ./cmd/seed
```

Usage:

```bash
./seed \
  -domains=/opt/spyder/config/domains.txt \
  -redis=10.0.1.5:6379 \
  -key=spyder:queue
```

| Flag | Default | Description |
|---|---|---|
| `-domains` | (required) | Path to a newline-separated domains file. Lines starting with `#` and blank lines are skipped. |
| `-redis` | `127.0.0.1:6379` | Redis address. |
| `-key` | `spyder:queue` | Redis list key. |

Each domain is serialized as a JSON object with the host, timestamp, and attempt counter, then pushed via `LPUSH`:

```json
{"host":"example.com","ts":1710300000,"attempt":0}
```

You can seed the queue from any machine with Redis access. Seed before starting probes, or seed while probes are running -- probes will pick up new items immediately.

### Seeding Large Domain Lists

For large lists (millions of domains), pipe through `split` to avoid holding the entire file in memory:

```bash
split -l 100000 domains.txt /tmp/chunk_

for chunk in /tmp/chunk_*; do
  ./seed -domains="$chunk" -redis=10.0.1.5:6379
  echo "Seeded $chunk"
done
```

### Re-seeding and Idempotency

The seed utility does not deduplicate. If you seed the same domain twice, it will be crawled twice (though the probe's dedup layer will skip redundant edge emissions). To avoid duplicate work, clear the queue before re-seeding:

```bash
redis-cli -h 10.0.1.5 DEL spyder:queue
redis-cli -h 10.0.1.5 DEL spyder:queue:processing
./seed -domains=domains.txt -redis=10.0.1.5:6379
```

## Running Multiple Probe Instances

### Basic Multi-Instance Deployment

Each probe instance needs a unique `-probe` ID so edges can be traced back to the originating instance. All instances share the same `-run` ID for a given scan campaign.

**Instance 1 (probe-east-1):**

```bash
export REDIS_ADDR=10.0.1.5:6379
export REDIS_QUEUE_ADDR=10.0.1.5:6379
export REDIS_QUEUE_KEY=spyder:queue

/opt/spyder/bin/spyder \
  -domains=/dev/null \
  -probe=probe-east-1 \
  -run=campaign-2026-03 \
  -concurrency=256 \
  -metrics_addr=:9090 \
  -batch_max_edges=10000 \
  -batch_flush_sec=2 \
  -spool_dir=/opt/spyder/spool
```

**Instance 2 (probe-east-2):**

```bash
export REDIS_ADDR=10.0.1.5:6379
export REDIS_QUEUE_ADDR=10.0.1.5:6379
export REDIS_QUEUE_KEY=spyder:queue

/opt/spyder/bin/spyder \
  -domains=/dev/null \
  -probe=probe-east-2 \
  -run=campaign-2026-03 \
  -concurrency=256 \
  -metrics_addr=:9090 \
  -batch_max_edges=10000 \
  -batch_flush_sec=2 \
  -spool_dir=/opt/spyder/spool
```

::: tip
The `-domains` flag is still required by the config validator, but SPYDER will not read from it when `REDIS_QUEUE_ADDR` is set. Point it at `/dev/null` or an empty file.
:::

### Using a Config File

For consistency across instances, use a shared YAML config and override only the probe ID per instance:

```yaml
# /opt/spyder/config/distributed.yaml
domains: /dev/null
run: campaign-2026-03
concurrency: 256
metrics_addr: ":9090"
batch_max_edges: 10000
batch_flush_sec: 2
spool_dir: /opt/spyder/spool
ua: "SPYDERProbe/1.0 (+https://yourcompany.com/security)"
exclude_tlds:
  - gov
  - mil
  - int
```

Then run with:

```bash
/opt/spyder/bin/spyder \
  -config=/opt/spyder/config/distributed.yaml \
  -probe=probe-east-1
```

## Continuous Mode (Recursive Crawling)

The `-continuous` flag enables recursive domain discovery. When a probe crawls a domain and finds new domains (through DNS records, TLS certificates, or HTML links), those discoveries are fed back into the work queue for future crawling.

### How It Works

In distributed mode with `-continuous`, SPYDER uses a `RedisSink` that pushes discovered domains back into the shared Redis queue. All instances benefit from each other's discoveries:

1. Probe A crawls `example.com`, discovers `cdn.example.net` in a CNAME record
2. The `RedisSink` dedup-checks `cdn.example.net`, then `LPUSH`es it to `spyder:queue`
3. Probe B (or Probe A) leases `cdn.example.net` from the queue and crawls it
4. The process continues until the queue is empty or `-max_domains` is reached

### Running with Continuous Mode

```bash
/opt/spyder/bin/spyder \
  -config=/opt/spyder/config/distributed.yaml \
  -probe=probe-east-1 \
  -continuous \
  -max_domains=500000
```

| Flag | Default | Description |
|---|---|---|
| `-continuous` | `false` | Enable recursive crawling. Discovered domains are submitted back to the work queue. |
| `-max_domains` | `0` (unlimited) | Cap the total number of discovered domains that get re-queued. Each probe tracks its own counter independently. Set this to prevent runaway expansion. |

### Controlling Crawl Scope

Without `-max_domains`, continuous mode will keep discovering and crawling until no new domains appear. For large-scale scans, set limits to keep the crawl bounded:

```bash
# Each probe will submit at most 100,000 new discoveries
/opt/spyder/bin/spyder \
  -config=/opt/spyder/config/distributed.yaml \
  -probe=probe-east-1 \
  -continuous \
  -max_domains=100000
```

Use `-exclude_tlds` to prevent crawling into sensitive or irrelevant TLDs:

```bash
/opt/spyder/bin/spyder \
  -config=/opt/spyder/config/distributed.yaml \
  -probe=probe-east-1 \
  -continuous \
  -max_domains=100000 \
  -exclude_tlds=gov,mil,int,edu
```

### Single-Node Continuous Mode

If `REDIS_QUEUE_ADDR` is not set, `-continuous` uses an in-memory `ChannelSink` instead of `RedisSink`. Discovered domains are fed back into the probe through a Go channel. This is useful for single-machine recursive crawling:

```bash
/opt/spyder/bin/spyder \
  -domains=seeds.txt \
  -probe=local-1 \
  -continuous \
  -max_domains=50000 \
  -concurrency=128
```

In this mode, the seed domains are read from the file first, then the probe drains discovered domains from the channel until the context is cancelled or the max is reached.

## Load Balancing and Sharding Strategies

### Queue-Based Load Balancing

The Redis queue provides natural load balancing: faster probes consume more items. No explicit assignment or partitioning is needed. This works well when all probes have similar network conditions.

### Regional Sharding

For geographically distributed scans, use separate queue keys per region to minimize latency between probes and their targets:

```bash
# Seed region-specific queues
./seed -domains=domains-us.txt -redis=10.0.1.5:6379 -key=spyder:queue:us
./seed -domains=domains-eu.txt -redis=10.0.1.5:6379 -key=spyder:queue:eu
./seed -domains=domains-ap.txt -redis=10.0.1.5:6379 -key=spyder:queue:ap
```

```bash
# US probes
export REDIS_QUEUE_KEY=spyder:queue:us
/opt/spyder/bin/spyder -config=distributed.yaml -probe=probe-us-1

# EU probes
export REDIS_QUEUE_KEY=spyder:queue:eu
/opt/spyder/bin/spyder -config=distributed.yaml -probe=probe-eu-1
```

### Dedicated Redis Instances

For very large deployments (10+ probes), separate the dedup Redis from the queue Redis to avoid contention:

```bash
# Dedup Redis -- high memory, read-heavy
export REDIS_ADDR=10.0.1.10:6379

# Queue Redis -- low memory, write-heavy
export REDIS_QUEUE_ADDR=10.0.1.11:6379
```

### Scaling Concurrency

Each probe's `-concurrency` flag controls the number of goroutines performing crawls. Guidelines for tuning:

| Probe CPU Cores | Recommended Concurrency | Notes |
|---|---|---|
| 2 | 64-128 | Suitable for lightweight VMs |
| 4 | 128-256 | Good general-purpose setting |
| 8 | 256-512 | High-throughput configuration |
| 16+ | 512-1024 | Requires `LimitNOFILE=65536` in systemd |

Increase file descriptor limits on each probe machine:

```bash
# /etc/security/limits.d/spyder.conf
spyder soft nofile 65536
spyder hard nofile 65536
```

## Monitoring Distributed Deployments

### Prometheus Multi-Target Configuration

Scrape all probe instances from a single Prometheus server:

```yaml
# /etc/prometheus/prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'spyder-distributed'
    static_configs:
      - targets:
          - 'probe-east-1.internal:9090'
          - 'probe-east-2.internal:9090'
          - 'probe-west-1.internal:9090'
        labels:
          cluster: 'production'

  - job_name: 'spyder-redis'
    static_configs:
      - targets: ['redis-exporter.internal:9121']
```

### Key Distributed Metrics

Track per-instance and aggregate metrics:

```promql
# Aggregate throughput across all instances
sum(rate(spyder_tasks_total{status="ok"}[5m]))

# Per-instance throughput
rate(spyder_tasks_total{status="ok"}[5m])

# Per-instance error rate
rate(spyder_tasks_total{status="error"}[5m]) /
rate(spyder_tasks_total[5m])

# Edge discovery rate across the cluster
sum(rate(spyder_edges_total[5m]))
```

### Queue Depth Monitoring

Monitor the Redis queue to detect stalls or backlogs. Use the Redis Exporter or a simple script:

```bash
#!/bin/bash
# /opt/spyder/bin/queue-monitor.sh
REDIS_HOST=10.0.1.5

while true; do
  PENDING=$(redis-cli -h "$REDIS_HOST" LLEN spyder:queue)
  PROCESSING=$(redis-cli -h "$REDIS_HOST" LLEN spyder:queue:processing)
  echo "$(date -u +%FT%TZ) pending=$PENDING processing=$PROCESSING"
  sleep 30
done
```

Set up Prometheus alerts for queue health:

```yaml
groups:
  - name: spyder-distributed
    rules:
      - alert: SpyderQueueBacklog
        expr: redis_list_length{key="spyder:queue"} > 100000
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "SPYDER queue backlog growing"
          description: "Queue has {{ $value }} pending items"

      - alert: SpyderQueueStalled
        expr: delta(redis_list_length{key="spyder:queue"}[10m]) == 0 and redis_list_length{key="spyder:queue"} > 0
        for: 15m
        labels:
          severity: critical
        annotations:
          summary: "SPYDER queue is stalled"
          description: "Queue length unchanged for 15 minutes with {{ $value }} items remaining"

      - alert: SpyderProbeDown
        expr: up{job="spyder-distributed"} == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "SPYDER probe {{ $labels.instance }} is down"
```

### Health Check Endpoints

Each probe exposes health endpoints on its metrics port:

| Endpoint | Purpose |
|---|---|
| `GET /live` | Liveness check -- returns 200 if the process is running |
| `GET /ready` | Readiness check -- returns 200 once the probe has initialized and is consuming from the queue |
| `GET /health` | Detailed health -- returns component-level status including Redis connectivity |
| `GET /metrics` | Prometheus metrics |

```bash
# Check a specific probe
curl -s http://probe-east-1.internal:9090/health | jq .
```

```json
{
  "status": "healthy",
  "timestamp": "2026-03-13T14:30:00Z",
  "checks": [
    {
      "name": "redis",
      "status": "healthy",
      "message": "Redis connection OK",
      "last_checked": "2026-03-13T14:30:00Z"
    }
  ],
  "metadata": {
    "probe": "probe-east-1",
    "run": "campaign-2026-03",
    "version": "1.0.0"
  }
}
```

## Operational Procedures

### Starting a Distributed Scan

```bash
# 1. Verify Redis is running
redis-cli -h 10.0.1.5 PING

# 2. Clear any stale queue data
redis-cli -h 10.0.1.5 DEL spyder:queue
redis-cli -h 10.0.1.5 DEL spyder:queue:processing

# 3. Seed the queue
./seed -domains=domains.txt -redis=10.0.1.5:6379

# 4. Verify seed count
redis-cli -h 10.0.1.5 LLEN spyder:queue

# 5. Start probes (on each machine)
sudo systemctl start spyder

# 6. Monitor progress
watch -n 5 'redis-cli -h 10.0.1.5 LLEN spyder:queue'
```

### Graceful Shutdown

SPYDER handles `SIGTERM` and `SIGINT` gracefully. When a probe receives a shutdown signal:

1. The context is cancelled, stopping the queue lease loop
2. In-flight crawls complete (up to 30 seconds with systemd `TimeoutStopSec`)
3. The batch emitter drains remaining edges to the ingest endpoint or spool directory
4. Any items in the processing list that were not acknowledged will remain for recovery

```bash
# Stop a single probe
sudo systemctl stop spyder

# Stop all probes across machines (using pssh or similar)
pssh -h probe-hosts.txt 'sudo systemctl stop spyder'
```

### Recovering from Crashes

If a probe crashes, its leased items remain in the `spyder:queue:processing` list. Move them back to the main queue:

```bash
# Check for stuck items
redis-cli -h 10.0.1.5 LLEN spyder:queue:processing

# Move all processing items back to the queue
redis-cli -h 10.0.1.5 --no-auth-warning <<'EOF'
EVAL "
  local count = 0
  while redis.call('RPOPLPUSH', KEYS[1], KEYS[2]) do
    count = count + 1
  end
  return count
" 2 spyder:queue:processing spyder:queue
EOF
```
