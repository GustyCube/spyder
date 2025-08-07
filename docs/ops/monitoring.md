# Monitoring and Alerting

This guide covers monitoring SPYDER Probe deployments using Prometheus, Grafana, and alerting systems.

## Metrics Overview

SPYDER Probe exposes Prometheus metrics at the `/metrics` endpoint (default port 9090).

### Core Metrics

#### Task Metrics
- `spyder_tasks_total{status="ok|error"}` - Counter of processed tasks
- `spyder_task_duration_seconds` - Histogram of task processing time
- `spyder_active_workers` - Gauge of currently active workers

#### Edge Discovery Metrics
- `spyder_edges_total{type="RESOLVES_TO|HAS_CERT|LINKS_TO"}` - Counter of discovered edges by type
- `spyder_nodes_discovered_total{type="domain|ip|cert"}` - Counter of discovered nodes by type

#### System Metrics
- `spyder_redis_operations_total{operation="get|set|exists"}` - Redis operation counters
- `spyder_http_requests_total{status_code}` - HTTP request counters
- `spyder_batch_emissions_total{status="success|failure"}` - Batch emission results

## Prometheus Configuration

### Basic Setup

Create `/etc/prometheus/prometheus.yml`:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "spyder_alerts.yml"

scrape_configs:
  - job_name: 'spyder-probe'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 10s
    metrics_path: '/metrics'

alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093
```

### Multi-Node Setup

For distributed deployments:

```yaml
scrape_configs:
  - job_name: 'spyder-probe-cluster'
    static_configs:
      - targets: 
        - 'probe-1.internal:9090'
        - 'probe-2.internal:9090'
        - 'probe-3.internal:9090'
    labels:
      environment: 'production'
      
  - job_name: 'spyder-redis'
    static_configs:
      - targets: ['redis.internal:6379']
```

## Grafana Dashboards

### Installation and Setup

1. **Install Grafana**
   ```bash
   sudo apt-get install -y grafana
   sudo systemctl enable grafana-server
   sudo systemctl start grafana-server
   ```

2. **Add Prometheus data source**
   - URL: `http://localhost:9090`
   - Access: Server (default)

### SPYDER Probe Dashboard

Key panels to include:

#### Processing Rate Panel
```promql
# Tasks processed per second
rate(spyder_tasks_total[5m])

# Success rate
rate(spyder_tasks_total{status="ok"}[5m]) / rate(spyder_tasks_total[5m]) * 100
```

#### Edge Discovery Panel  
```promql
# Edges discovered by type
rate(spyder_edges_total[5m])

# Top domains by edge count
topk(10, increase(spyder_edges_total[1h]))
```

#### System Health Panel
```promql
# Active workers
spyder_active_workers

# Redis hit rate
rate(spyder_redis_operations_total{operation="get"}[5m]) / 
rate(spyder_redis_operations_total[5m]) * 100

# HTTP error rate
rate(spyder_http_requests_total{status_code!~"2.."}[5m]) / 
rate(spyder_http_requests_total[5m]) * 100
```

## Alerting Rules

Create `/etc/prometheus/spyder_alerts.yml`:

```yaml
groups:
  - name: spyder-probe
    rules:
      # High error rate
      - alert: SpyderHighErrorRate
        expr: (rate(spyder_tasks_total{status="error"}[5m]) / rate(spyder_tasks_total[5m])) > 0.1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "SPYDER Probe high error rate"
          description: "Error rate is {{ $value | humanizePercentage }} for instance {{ $labels.instance }}"

      # Low processing rate
      - alert: SpyderLowProcessingRate  
        expr: rate(spyder_tasks_total[5m]) < 10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "SPYDER Probe low processing rate"
          description: "Processing only {{ $value }} tasks/sec on {{ $labels.instance }}"

      # Redis connection issues
      - alert: SpyderRedisDown
        expr: rate(spyder_redis_operations_total[5m]) == 0
        for: 1m  
        labels:
          severity: critical
        annotations:
          summary: "SPYDER Probe Redis connection lost"
          description: "No Redis operations detected on {{ $labels.instance }}"

      # Batch emission failures
      - alert: SpyderBatchEmissionFailures
        expr: rate(spyder_batch_emissions_total{status="failure"}[5m]) > 0
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "SPYDER Probe batch emission failures"
          description: "{{ $value }} batch emission failures/sec on {{ $labels.instance }}"

      # Worker pool exhaustion
      - alert: SpyderWorkerPoolExhausted
        expr: spyder_active_workers == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "SPYDER Probe worker pool exhausted"  
          description: "No active workers on {{ $labels.instance }}"
```

## Log Monitoring

### Structured Log Analysis

SPYDER uses structured logging with zap. Common log queries:

```bash
# Error analysis
sudo journalctl -u spyder | jq 'select(.level == "error")'

# Performance analysis  
sudo journalctl -u spyder | jq 'select(.msg == "batch emitted") | .duration'

# Redis connectivity issues
sudo journalctl -u spyder | jq 'select(.msg | contains("redis"))'
```

### Log Aggregation with ELK Stack

#### Filebeat configuration (`/etc/filebeat/filebeat.yml`):

```yaml
filebeat.inputs:
- type: journald
  id: spyder-logs
  include_matches:
    - "_SYSTEMD_UNIT=spyder.service"

output.elasticsearch:
  hosts: ["elasticsearch:9200"]
  
processors:
  - decode_json_fields:
      fields: ["message"]
      target: ""
```

#### Logstash filter:

```ruby
filter {
  if [fields][service] == "spyder" {
    json {
      source => "message"
    }
    
    date {
      match => [ "ts", "UNIX" ]
    }
    
    mutate {
      remove_field => [ "message" ]
    }
  }
}
```

## Performance Monitoring

### Key Performance Indicators

1. **Throughput Metrics**
   - Domains processed per second
   - Edges discovered per hour
   - Data volume processed

2. **Latency Metrics**  
   - Average task processing time
   - DNS resolution latency
   - HTTP request latency

3. **Resource Utilization**
   - CPU usage per worker
   - Memory consumption
   - Network I/O rates

### Custom Metrics

Add application-specific metrics:

```go
// Custom metrics example
var (
    domainsProcessed = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "spyder_domains_processed_total",
            Help: "Total domains processed by TLD",
        },
        []string{"tld"},
    )
    
    crawlDepth = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "spyder_crawl_depth",
            Help: "Distribution of crawl depths",
            Buckets: prometheus.LinearBuckets(1, 1, 10),
        },
        []string{"probe_id"},
    )
)
```

## Health Checks

### Kubernetes Probes

```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: spyder-probe
    image: spyder-probe:latest
    ports:
    - containerPort: 9090
    livenessProbe:
      httpGet:
        path: /metrics
        port: 9090
      initialDelaySeconds: 30
      periodSeconds: 10
    readinessProbe:
      httpGet:
        path: /metrics  
        port: 9090
      initialDelaySeconds: 5
      periodSeconds: 5
```

### External Health Monitoring

Use tools like:
- **Uptime Kuma** - Simple uptime monitoring
- **Pingdom** - External service monitoring  
- **DataDog** - Comprehensive monitoring platform

## Troubleshooting Monitoring Issues

### Common Problems

1. **Metrics not appearing**
   ```bash
   # Check metrics endpoint
   curl http://localhost:9090/metrics | grep spyder
   
   # Verify Prometheus scraping
   curl http://prometheus:9090/api/v1/targets
   ```

2. **High cardinality metrics**
   ```bash
   # Check metric cardinality
   curl -s http://localhost:9090/metrics | grep spyder | wc -l
   
   # Look for high-cardinality labels
   curl -s http://localhost:9090/metrics | grep spyder | cut -d'{' -f2 | sort | uniq -c | sort -nr
   ```

3. **Dashboard not loading**
   - Check Grafana datasource configuration
   - Verify PromQL queries in Grafana query inspector
   - Check Grafana logs: `sudo journalctl -u grafana-server`

### Performance Impact

Monitor the monitoring overhead:

```promql
# Prometheus ingestion rate
rate(prometheus_tsdb_symbol_table_size_bytes[5m])

# Grafana query performance
grafana_api_dashboard_snapshot_external_enabled
```

## Integration Examples

### Slack Alerting

Configure Alertmanager for Slack notifications:

```yaml
# alertmanager.yml
global:
  slack_api_url: 'YOUR_SLACK_WEBHOOK_URL'

route:
  group_by: ['alertname']
  receiver: 'spyder-alerts'

receivers:
  - name: 'spyder-alerts'
    slack_configs:
      - channel: '#ops-alerts'
        title: 'SPYDER Alert: {{ .GroupLabels.alertname }}'
        text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'
```

### PagerDuty Integration

```yaml
receivers:
  - name: 'spyder-critical'
    pagerduty_configs:
      - service_key: 'YOUR_PAGERDUTY_SERVICE_KEY'
        description: 'SPYDER: {{ .GroupLabels.alertname }}'
```