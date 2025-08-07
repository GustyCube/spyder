# Redis Configuration Guide

SPYDER uses Redis for two main purposes: distributed deduplication and work queue management. This guide covers Redis setup, configuration, and optimization for SPYDER deployments.

## Redis Use Cases in SPYDER

### 1. Deduplication Backend

Prevents duplicate data collection across multiple probe instances.

**Features:**
- Cross-probe deduplication of nodes and edges
- TTL-based expiration (default: 24 hours)
- Memory-efficient SET operations
- Atomic check-and-set operations

### 2. Distributed Work Queue

Distributes domain processing across multiple probe instances.

**Features:**
- Atomic work item leasing with BRPOPLPUSH
- Processing queue for failure recovery
- JSON-encoded work items with metadata
- Configurable lease timeouts

## Redis Installation

### Single Instance Setup

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install redis-server

# CentOS/RHEL/Fedora
sudo dnf install redis

# macOS
brew install redis

# Start Redis
sudo systemctl enable redis-server
sudo systemctl start redis-server
```

### Docker Deployment

```bash
# Single instance
docker run -d --name redis \
  -p 6379:6379 \
  -v redis-data:/data \
  redis:7-alpine redis-server --appendonly yes

# With custom configuration
docker run -d --name redis \
  -p 6379:6379 \
  -v $(pwd)/redis.conf:/usr/local/etc/redis/redis.conf \
  -v redis-data:/data \
  redis:7-alpine redis-server /usr/local/etc/redis/redis.conf
```

### Redis Cluster (High Availability)

```bash
# Docker Compose cluster setup
version: '3.8'
services:
  redis-node-1:
    image: redis:7-alpine
    ports:
      - "7001:6379"
    command: redis-server --cluster-enabled yes --cluster-node-timeout 5000
    
  redis-node-2:
    image: redis:7-alpine
    ports:
      - "7002:6379"
    command: redis-server --cluster-enabled yes --cluster-node-timeout 5000
    
  redis-node-3:
    image: redis:7-alpine
    ports:
      - "7003:6379"
    command: redis-server --cluster-enabled yes --cluster-node-timeout 5000
```

## Configuration for SPYDER

### Basic Redis Configuration

**File: `/etc/redis/redis.conf`**

```conf
# Network
bind 127.0.0.1
port 6379
protected-mode yes

# Memory
maxmemory 2gb
maxmemory-policy allkeys-lru

# Persistence
save 900 1
save 300 10 
save 60 10000
appendonly yes
appendfsync everysec

# Performance
tcp-keepalive 300
timeout 0
tcp-backlog 511

# Security
# requirepass your-strong-password
```

### SPYDER-Optimized Configuration

```conf
# Optimized for SPYDER workloads

# Memory - adjust based on available RAM
maxmemory 4gb
maxmemory-policy allkeys-lru

# Network optimizations
tcp-keepalive 300
tcp-backlog 2048
timeout 0

# Performance tuning
lazy-expire-disabled no
hash-max-ziplist-entries 512
hash-max-ziplist-value 64
list-max-ziplist-size -2

# Persistence for reliability
save 900 1
save 300 100
save 60 10000
appendonly yes
appendfsync everysec
auto-aof-rewrite-percentage 100
auto-aof-rewrite-min-size 64mb

# Logging
loglevel notice
logfile /var/log/redis/redis-server.log
```

## SPYDER Integration

### Environment Configuration

```bash
# Basic Redis connection
export REDIS_ADDR=127.0.0.1:6379

# Distributed queue
export REDIS_QUEUE_ADDR=127.0.0.1:6379
export REDIS_QUEUE_KEY=spyder:production:queue

# Multiple Redis instances
export REDIS_ADDR=redis-dedup.local:6379
export REDIS_QUEUE_ADDR=redis-queue.local:6379
```

### Queue Operations

**Seeding the Queue:**

```bash
# Using redis-cli
cat domains.txt | while read domain; do
  redis-cli LPUSH "spyder:queue" "{\"host\":\"$domain\",\"ts\":$(date +%s),\"attempt\":0}"
done

# Using SPYDER seed tool
./bin/seed -queue_addr=redis:6379 -queue_key=spyder:queue -domains=domains.txt
```

**Queue Monitoring:**

```bash
# Check queue length
redis-cli LLEN spyder:queue

# Check processing queue
redis-cli LLEN spyder:queue:processing

# View queue contents
redis-cli LRANGE spyder:queue 0 10
```

### Deduplication Operations

**Key Patterns:**

```bash
# Node keys
redis-cli EXISTS domain|example.com
redis-cli EXISTS nodeip|192.168.1.1
redis-cli EXISTS cert|B7+tPUdz9OYB...

# Edge keys  
redis-cli EXISTS edge|example.com|RESOLVES_TO|192.168.1.1
```

**Monitoring Deduplication:**

```bash
# Count dedupe keys
redis-cli EVAL "return #redis.call('keys', 'domain|*')" 0
redis-cli EVAL "return #redis.call('keys', 'edge|*')" 0

# Check TTL on dedupe keys
redis-cli TTL domain|example.com
```

## Performance Tuning

### Memory Optimization

```conf
# Enable key expiration
lazy-expire-disabled no

# Optimize data structures
hash-max-ziplist-entries 512
list-max-ziplist-size -2
set-max-intset-entries 512
zset-max-ziplist-entries 128

# Memory efficiency
maxmemory-policy allkeys-lru
maxmemory-samples 5
```

### Network Optimization

```conf
# Connection handling
tcp-backlog 2048
maxclients 10000

# Pipeline optimization
tcp-nodelay yes
tcp-keepalive 300

# Buffer sizes
client-output-buffer-limit normal 0 0 0
client-output-buffer-limit replica 256mb 64mb 60
client-output-buffer-limit pubsub 32mb 8mb 60
```

### Persistence Optimization

**For High Write Workloads:**

```conf
# Faster persistence
save 300 10000
save 60 100000
appendonly yes
appendfsync no
auto-aof-rewrite-percentage 100
auto-aof-rewrite-min-size 128mb
```

**For Data Safety:**

```conf
# Safer persistence
save 900 1
save 300 10
save 60 10000
appendonly yes
appendfsync everysec
no-appendfsync-on-rewrite no
```

## Monitoring and Maintenance

### Key Metrics to Monitor

```bash
# Memory usage
redis-cli INFO memory

# Connection stats
redis-cli INFO clients

# Command statistics
redis-cli INFO commandstats

# Persistence status
redis-cli INFO persistence

# Replication status (if applicable)
redis-cli INFO replication
```

### Health Checks

```bash
#!/bin/bash
# redis-health-check.sh

REDIS_HOST=${REDIS_ADDR%:*}
REDIS_PORT=${REDIS_ADDR#*:}

# Basic connectivity
if ! redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" ping > /dev/null 2>&1; then
    echo "❌ Redis not responding"
    exit 1
fi

# Memory usage check
MEMORY_USED=$(redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" INFO memory | grep used_memory_human | cut -d: -f2 | tr -d '\r')
echo "✅ Redis responding, memory used: $MEMORY_USED"

# Queue length check
QUEUE_LEN=$(redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" LLEN spyder:queue)
echo "📊 Queue length: $QUEUE_LEN"

# Check for stuck processing items
PROC_LEN=$(redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" LLEN spyder:queue:processing)
if [ "$PROC_LEN" -gt 1000 ]; then
    echo "⚠️  Many items in processing queue: $PROC_LEN"
fi
```

### Maintenance Operations

```bash
# Clear deduplication cache
redis-cli EVAL "return redis.call('del', unpack(redis.call('keys', 'domain|*')))" 0
redis-cli EVAL "return redis.call('del', unpack(redis.call('keys', 'edge|*')))" 0

# Clear stale processing queue items
redis-cli DEL spyder:queue:processing

# Compact memory
redis-cli MEMORY PURGE

# Manual save
redis-cli BGSAVE
```

## Security Configuration

### Authentication

```conf
# Set password
requirepass your-strong-redis-password

# Disable dangerous commands
rename-command FLUSHDB ""
rename-command FLUSHALL ""
rename-command SHUTDOWN ""
rename-command CONFIG "CONFIG-9f2c2c2c"
```

### Network Security

```conf
# Bind to specific interfaces
bind 127.0.0.1 10.0.0.10

# Enable protected mode
protected-mode yes

# Use non-standard port
port 6380
```

### TLS Configuration (Redis 6+)

```conf
# Enable TLS
port 0
tls-port 6379
tls-cert-file /etc/ssl/redis/redis.crt
tls-key-file /etc/ssl/redis/redis.key
tls-ca-cert-file /etc/ssl/redis/ca.crt

# Client certificate requirements
tls-auth-clients yes
```

## Troubleshooting

### Common Issues

**Connection Refused:**

```bash
# Check if Redis is running
sudo systemctl status redis

# Check port binding
ss -tlnp | grep 6379

# Test connectivity
telnet redis-host 6379
```

**Memory Issues:**

```bash
# Check memory usage
redis-cli INFO memory

# Find memory-hungry keys
redis-cli --bigkeys

# Monitor memory usage over time
redis-cli --latency-history -i 1
```

**Queue Backup:**

```bash
# Check queue lengths
redis-cli LLEN spyder:queue
redis-cli LLEN spyder:queue:processing

# Clear stuck processing items
# (Only if no probes are currently running)
redis-cli DEL spyder:queue:processing
```

### Performance Issues

```bash
# Monitor slow queries
redis-cli CONFIG SET slowlog-log-slower-than 10000
redis-cli SLOWLOG GET

# Check latency
redis-cli --latency -h redis-host -p 6379

# Monitor operations per second
redis-cli INFO stats | grep instantaneous_ops_per_sec
```

### Debug Commands

```bash
# View all configuration
redis-cli CONFIG GET '*'

# Monitor all commands
redis-cli MONITOR

# Check client connections
redis-cli CLIENT LIST

# View memory usage by type
redis-cli MEMORY USAGE domain|example.com
```

This Redis configuration ensures optimal performance and reliability for SPYDER's distributed operations while maintaining data consistency across multiple probe instances.