# Prometheus Metrics Reference

SPYDER exposes comprehensive Prometheus metrics for monitoring performance, health, and operational status.

## Metrics Endpoint

**Default Configuration:**
```bash
./bin/spyder -metrics_addr=:9090
```

**Access Metrics:**
```bash
curl http://localhost:9090/metrics
```

**Disable Metrics:**
```bash
./bin/spyder -metrics_addr=""
```

## Core Metrics

### Task Processing Metrics

#### `spyder_tasks_total`

Counter tracking total tasks processed by status.

**Type:** Counter  
**Labels:** `status`

```prometheus
# HELP spyder_tasks_total tasks processed
# TYPE spyder_tasks_total counter
spyder_tasks_total{status="ok"} 1234
spyder_tasks_total{status="error"} 5
```

**Label Values:**
- `ok`: Successfully processed domains
- `error`: Failed domain processing (panics, critical errors)

**Query Examples:**
```promql
# Processing rate (domains/sec)
rate(spyder_tasks_total[5m])

# Error rate
rate(spyder_tasks_total{status="error"}[5m]) / rate(spyder_tasks_total[5m])

# Total domains processed
sum(spyder_tasks_total)
```

### Edge Discovery Metrics

#### `spyder_edges_total`

Counter tracking discovered edges by type.

**Type:** Counter  
**Labels:** `type`

```prometheus
# HELP spyder_edges_total edges emitted
# TYPE spyder_edges_total counter
spyder_edges_total{type="RESOLVES_TO"} 2468
spyder_edges_total{type="LINKS_TO"} 1357
spyder_edges_total{type="USES_CERT"} 891
spyder_edges_total{type="USES_NS"} 567
spyder_edges_total{type="USES_MX"} 234
spyder_edges_total{type="ALIAS_OF"} 123
```

**Edge Types:**
- `RESOLVES_TO`: DNS A/AAAA records
- `LINKS_TO`: External HTTP links
- `USES_CERT`: TLS certificates
- `USES_NS`: Nameserver records
- `USES_MX`: Mail exchange records
- `ALIAS_OF`: CNAME records

**Query Examples:**
```promql
# Edge discovery rate by type
rate(spyder_edges_total[5m])

# Most common edge types
topk(5, sum by (type) (spyder_edges_total))

# DNS vs HTTP edge ratio
sum(spyder_edges_total{type=~"RESOLVES_TO|USES_NS|USES_MX|ALIAS_OF"}) /
sum(spyder_edges_total{type="LINKS_TO"})
```

### Policy Enforcement Metrics

#### `spyder_robots_blocked_total`

Counter tracking domains blocked by robots.txt.

**Type:** Counter  
**No Labels**

```prometheus
# HELP spyder_robots_blocked_total robots.txt blocks
# TYPE spyder_robots_blocked_total counter
spyder_robots_blocked_total 45
```

**Query Examples:**
```promql
# Robots.txt block rate
rate(spyder_robots_blocked_total[5m])

# Percentage of domains blocked
rate(spyder_robots_blocked_total[5m]) / rate(spyder_tasks_total[5m]) * 100
```

## Derived Metrics

### Performance Indicators

**Throughput Metrics:**
```promql
# Domains processed per second
rate(spyder_tasks_total[5m])

# Edges discovered per second
rate(spyder_edges_total[5m])

# Average edges per domain
rate(spyder_edges_total[5m]) / rate(spyder_tasks_total[5m])
```

**Efficiency Metrics:**
```promql
# Success rate
rate(spyder_tasks_total{status="ok"}[5m]) / rate(spyder_tasks_total[5m])

# Discovery efficiency (edges per successful domain)
rate(spyder_edges_total[5m]) / rate(spyder_tasks_total{status="ok"}[5m])
```

### Health Indicators

**Error Monitoring:**
```promql
# Error rate threshold alert
rate(spyder_tasks_total{status="error"}[5m]) > 0.1

# High robots.txt blocking (potential configuration issue)
rate(spyder_robots_blocked_total[5m]) / rate(spyder_tasks_total[5m]) > 0.5
```

## Go Runtime Metrics

SPYDER automatically exposes Go runtime metrics:

### Memory Metrics

```prometheus
# Memory usage
go_memstats_alloc_bytes
go_memstats_heap_alloc_bytes
go_memstats_heap_inuse_bytes
go_memstats_heap_sys_bytes

# Garbage collection
go_memstats_gc_sys_bytes
go_gc_duration_seconds
```

### Goroutine Metrics

```prometheus
# Active goroutines
go_goroutines

# Thread count
go_threads
```

**Query Examples:**
```promql
# Memory growth rate
rate(go_memstats_alloc_bytes[5m])

# GC frequency
rate(go_gc_duration_seconds_count[5m])

# Goroutine count per worker (approximately)
go_goroutines / 256  # assuming 256 workers
```

## HTTP Metrics (from Prometheus client)

### Request Metrics

```prometheus
# HTTP requests to metrics endpoint
promhttp_metric_handler_requests_total
promhttp_metric_handler_requests_in_flight_gauge

# Response times
promhttp_metric_handler_request_duration_seconds
```

## Custom Dashboards

### Grafana Dashboard Configuration

**Main Dashboard Panels:**

```json
{
  "dashboard": {
    "title": "SPYDER Probe Monitoring",
    "panels": [
      {
        "title": "Processing Rate",
        "type": "stat",
        "targets": [
          {
            "expr": "rate(spyder_tasks_total[5m])",
            "legendFormat": "domains/sec"
          }
        ]
      },
      {
        "title": "Edge Discovery by Type",
        "type": "piechart", 
        "targets": [
          {
            "expr": "sum by (type) (spyder_edges_total)",
            "legendFormat": "{{type}}"
          }
        ]
      },
      {
        "title": "Success Rate",
        "type": "stat",
        "targets": [
          {
            "expr": "rate(spyder_tasks_total{status=\"ok\"}[5m]) / rate(spyder_tasks_total[5m]) * 100",
            "legendFormat": "success %"
          }
        ]
      },
      {
        "title": "Memory Usage",
        "type": "graph",
        "targets": [
          {
            "expr": "go_memstats_heap_alloc_bytes / 1024 / 1024",
            "legendFormat": "Heap MB"
          }
        ]
      }
    ]
  }
}
```

### Alerting Rules

**Prometheus Alerting Rules:**

```yaml
groups:
- name: spyder.rules
  rules:
  - alert: SpyderHighErrorRate
    expr: rate(spyder_tasks_total{status="error"}[5m]) / rate(spyder_tasks_total[5m]) > 0.05
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "SPYDER error rate is high"
      description: "Error rate is {{ $value | humanizePercentage }} over the last 5 minutes"

  - alert: SpyderLowThroughput
    expr: rate(spyder_tasks_total[5m]) < 10
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "SPYDER throughput is low"
      description: "Processing only {{ $value }} domains per second"

  - alert: SpyderHighMemoryUsage
    expr: go_memstats_heap_alloc_bytes / 1024 / 1024 > 1024
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "SPYDER memory usage is high"
      description: "Using {{ $value }}MB of heap memory"

  - alert: SpyderManyRobotsBlocks
    expr: rate(spyder_robots_blocked_total[5m]) / rate(spyder_tasks_total[5m]) > 0.8
    for: 10m
    labels:
      severity: info
    annotations:
      summary: "Many domains blocked by robots.txt"
      description: "{{ $value | humanizePercentage }} of domains blocked"

  - alert: SpyderDown
    expr: up{job="spyder"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "SPYDER probe is down"
      description: "SPYDER probe has been down for more than 1 minute"
```

## Monitoring Best Practices

### Scrape Configuration

**Prometheus Configuration:**

```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
- job_name: 'spyder'
  static_configs:
  - targets: ['spyder:9090']
  scrape_interval: 30s
  metrics_path: /metrics
  
  # Optional: Basic authentication
  basic_auth:
    username: monitoring
    password_file: /etc/prometheus/spyder.password
    
  # Optional: TLS configuration
  tls_config:
    insecure_skip_verify: false
    ca_file: /etc/ssl/certs/ca.pem
```

### Security Considerations

**Metrics Endpoint Security:**

```bash
# Bind to localhost only
-metrics_addr=127.0.0.1:9090

# Use reverse proxy with authentication
nginx_config:
  location /metrics {
    auth_basic "Prometheus";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://127.0.0.1:9090/metrics;
  }
```

### Performance Impact

**Metrics Collection Overhead:**

- CPU: < 1% additional overhead
- Memory: ~1MB for metric storage
- Network: ~1KB/scrape (depends on activity)

**High-Frequency Scraping:**

```yaml
# For high-resolution monitoring
scrape_configs:
- job_name: 'spyder-detailed'
  static_configs:
  - targets: ['spyder:9090']
  scrape_interval: 5s  # High frequency
  metrics_path: /metrics
```

### Troubleshooting

**Common Issues:**

```bash
# Check if metrics endpoint is responding
curl -v http://localhost:9090/metrics

# Verify metric format
curl -s http://localhost:9090/metrics | grep spyder

# Check for parsing errors in Prometheus
curl -s http://prometheus:9090/api/v1/targets
```

**Debug Queries:**

```promql
# Check for counter resets (restarts)
resets(spyder_tasks_total[1h])

# Verify metric freshness
time() - timestamp(spyder_tasks_total)

# Check for missing metrics
absent(spyder_tasks_total)
```

This comprehensive metrics setup provides full visibility into SPYDER's performance, health, and operational characteristics for effective monitoring and troubleshooting.