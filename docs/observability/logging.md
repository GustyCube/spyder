# Logging

SPYDER uses [Uber Zap](https://github.com/uber-go/zap) for structured logging. All log output is JSON-formatted and written to stderr, making it straightforward to parse with `jq`, ship to log aggregation systems, or integrate with systemd journal.

## Logger Initialization

SPYDER initializes a production Zap logger at startup:

```go
// internal/logging/logging.go
package logging

import "go.uber.org/zap"

type Logger = zap.SugaredLogger

func New() *Logger {
    l, _ := zap.NewProduction()
    return l.Sugar()
}
```

The `SugaredLogger` provides a key-value style API used throughout the codebase:

```go
log.Info("starting spyder", "probe", cfg.Probe, "run", cfg.Run, "concurrency", cfg.Concurrency)
log.Warnw("ingest failed, spooling", "err", err)
log.Debugw("robots.txt fetch", "host", host, "err", err)
```

## Log Levels

SPYDER uses four log levels:

| Level | Usage | Examples |
|-------|-------|---------|
| `debug` | Detailed operational information for troubleshooting | robots.txt fetch results, link parsing errors, per-host details |
| `info` | Normal operational events | startup configuration, mode selection, shutdown |
| `warn` | Recoverable problems that do not stop processing | failed ingest calls (data spooled), OTEL init failures, Redis errors |
| `error` | Serious failures requiring attention | spool file creation failures, unrecoverable errors |

### Setting the Log Level

Use the `LOG_LEVEL` environment variable to control log verbosity:

```bash
# Show all logs including debug
LOG_LEVEL=debug ./bin/spyder -domains=domains.txt

# Default production level (info and above)
LOG_LEVEL=info ./bin/spyder -domains=domains.txt

# Warnings and errors only
LOG_LEVEL=warn ./bin/spyder -domains=domains.txt

# Errors only
LOG_LEVEL=error ./bin/spyder -domains=domains.txt
```

### Verbose Mode

The `-verbose` flag provides an alternative way to enable debug-level logging:

```bash
./bin/spyder -domains=domains.txt -verbose
```

This is equivalent to `LOG_LEVEL=debug` and is useful for quick debugging sessions without modifying environment variables.

## JSON Log Format

All log lines are JSON objects written to stderr. The Zap production encoder produces output in this format:

```json
{"level":"info","ts":1704067200.123,"caller":"spyder/main.go:362","msg":"starting spyder","probe":"local-1","run":"run-1704067200","concurrency":256,"continuous":false,"exclude_tlds":["gov","mil","int"],"config_file":""}
```

### Standard Fields

Every log line contains these fields:

| Field | Type | Description |
|-------|------|-------------|
| `level` | string | Log level (`debug`, `info`, `warn`, `error`) |
| `ts` | float | Unix timestamp with fractional seconds |
| `caller` | string | Source file and line number |
| `msg` | string | Human-readable log message |

### Context Fields

Additional fields depend on the log message:

**Startup logs:**
```json
{"level":"info","ts":1704067200.1,"msg":"starting spyder","probe":"prod-us-west","run":"scan-20240101","concurrency":512,"continuous":true,"exclude_tlds":["gov","mil","int"]}
{"level":"info","ts":1704067200.2,"msg":"redis dedupe enabled","addr":"redis.internal:6379"}
{"level":"info","ts":1704067200.3,"msg":"continuous mode enabled (in-memory)","max_domains":5000}
{"level":"info","ts":1704067200.4,"msg":"metrics and health server started","addr":":9090"}
```

**Operational logs:**
```json
{"level":"debug","ts":1704067205.5,"msg":"robots.txt fetch","host":"example.com","err":"context deadline exceeded"}
{"level":"warn","ts":1704067210.8,"msg":"ingest failed, spooling","err":"Post \"https://ingest.internal/v1/batch\": dial tcp: connection refused"}
{"level":"warn","ts":1704067215.2,"msg":"redis dedup error","count":3,"err":"read tcp: i/o timeout"}
```

**Shutdown logs:**
```json
{"level":"info","ts":1704067300.0,"msg":"service marked as ready"}
{"level":"info","ts":1704068400.0,"msg":"shutdown complete"}
```

## Log Analysis with jq

Since all logs are JSON, `jq` is a natural tool for analysis.

### Filter by Level

```bash
# Show only warnings and errors
./bin/spyder -domains=domains.txt 2>&1 >/dev/null | \
  jq -r 'select(.level == "warn" or .level == "error") | "\(.ts | todate) [\(.level)] \(.msg)"'
```

### Extract Error Summaries

```bash
# Count errors by message
./bin/spyder -domains=domains.txt 2>&1 >/dev/null | \
  jq -r 'select(.level == "error" or .level == "warn") | .msg' | \
  sort | uniq -c | sort -rn
```

### Filter by Component

```bash
# Show only probe-related logs (by caller path)
./bin/spyder -domains=domains.txt 2>&1 >/dev/null | \
  jq -r 'select(.caller | contains("probe/")) | "\(.ts | todate) \(.msg) \(.host // "")"'
```

### Monitor Ingest Failures

```bash
# Watch for ingest failures in real time
./bin/spyder -domains=domains.txt 2>&1 >/dev/null | \
  jq -r 'select(.msg | contains("ingest failed")) | "\(.ts | todate) \(.err)"'
```

### Track Redis Errors

```bash
# Watch Redis dedup errors and their frequency
./bin/spyder -domains=domains.txt 2>&1 >/dev/null | \
  jq -r 'select(.msg | contains("redis")) | "\(.ts | todate) \(.msg) count=\(.count // "n/a") err=\(.err // "n/a")"'
```

### Convert Timestamps

Zap uses Unix epoch timestamps. Convert them to human-readable format:

```bash
./bin/spyder -domains=domains.txt 2>&1 >/dev/null | \
  jq -r '"\(.ts | todate) [\(.level | ascii_upcase)] \(.msg)"'
```

## Separating Logs from Output

SPYDER writes JSON data output to stdout and logs to stderr. This separation is important for pipeline usage:

```bash
# Capture data output to file, view logs on terminal
./bin/spyder -domains=domains.txt > output.json

# Capture logs to file, view data on terminal
./bin/spyder -domains=domains.txt 2> spyder.log

# Capture both separately
./bin/spyder -domains=domains.txt > output.json 2> spyder.log

# Pipe data output while monitoring logs
./bin/spyder -domains=domains.txt 2>/dev/null | jq '.edges | length'
```

When using the `-ingest` flag, stdout is not used for data output (data goes to the ingest endpoint), so logs on stderr are the primary operational output.

## Journal Integration (systemd)

When running SPYDER as a systemd service, logs go directly to the journal:

### systemd Service Configuration

```ini
# /etc/systemd/system/spyder.service
[Service]
ExecStart=/opt/spyder/bin/spyder -domains=/etc/spyder/domains.txt -concurrency=256
StandardOutput=journal
StandardError=journal
SyslogIdentifier=spyder
```

### Querying Journal Logs

```bash
# View all SPYDER logs
journalctl -u spyder.service

# Follow logs in real time
journalctl -u spyder.service -f

# Show logs since last boot
journalctl -u spyder.service -b

# Show logs from the last hour
journalctl -u spyder.service --since "1 hour ago"

# Show only warnings and errors
journalctl -u spyder.service -p warning

# Export logs as JSON for jq processing
journalctl -u spyder.service -o json | \
  jq -r '.MESSAGE' | \
  jq -r 'select(.level == "warn") | "\(.ts | todate) \(.msg)"'
```

### Journal Storage Configuration

For long-term log retention, configure journald:

```ini
# /etc/systemd/journald.conf
[Journal]
Storage=persistent
SystemMaxUse=2G
MaxRetentionSec=90day
Compress=yes
```

## Log Shipping

### Forward to Elasticsearch/OpenSearch

Use `journalbeat` or `filebeat` to ship SPYDER logs to a search backend:

```yaml
# filebeat.yml
filebeat.inputs:
- type: journald
  id: spyder-logs
  include_matches:
    - _SYSTEMD_UNIT=spyder.service

output.elasticsearch:
  hosts: ["https://elasticsearch.internal:9200"]
  index: "spyder-logs-%{+yyyy.MM.dd}"

processors:
- decode_json_fields:
    fields: ["message"]
    target: "spyder"
    overwrite_keys: true
```

### Forward to Loki

For Grafana Loki integration, use `promtail`:

```yaml
# promtail.yml
scrape_configs:
- job_name: spyder
  journal:
    labels:
      job: spyder
    matches: _SYSTEMD_UNIT=spyder.service
  relabel_configs:
  - source_labels: ['__journal__systemd_unit']
    target_label: unit
  pipeline_stages:
  - json:
      expressions:
        level: level
        msg: msg
  - labels:
      level:
```

## Troubleshooting with Logs

### Debug a Specific Domain

Run SPYDER in verbose mode with a single domain to see every step:

```bash
echo "problem-domain.com" > debug.txt
./bin/spyder -domains=debug.txt -concurrency=1 -verbose 2>&1 >/dev/null | jq .
```

This shows debug-level logs for DNS resolution, robots.txt checking, HTTP fetching, TLS analysis, and link extraction for that single domain.

### Identify Common Failure Patterns

```bash
# Top error messages from a scan
./bin/spyder -domains=domains.txt 2>scan.log >/dev/null
jq -r 'select(.level == "warn" or .level == "error") | .msg' scan.log | \
  sort | uniq -c | sort -rn | head -10
```

Common patterns:

| Message | Meaning | Action |
|---------|---------|--------|
| `robots.txt fetch` (debug) | Could not retrieve robots.txt | Usually benign; site may not have one |
| `create request` (warn) | Invalid URL construction | Check domain format in input file |
| `ingest failed, spooling` (warn) | Ingest endpoint unreachable | Check network connectivity to ingest API |
| `redis dedup error` (warn) | Redis connection issue | Check Redis availability and network |
| `parse links` (debug) | HTML parsing failure | Usually benign; non-standard HTML |
| `otel init failed` (warn) | OTEL collector unreachable | Check `-otel_endpoint` configuration |

### Monitor Log Volume

High log volume can indicate problems (e.g., a Redis outage generating repeated warnings):

```bash
# Count log lines per level over a time window
journalctl -u spyder.service --since "1 hour ago" -o json | \
  jq -r '.MESSAGE' | jq -r '.level' 2>/dev/null | \
  sort | uniq -c | sort -rn
```

Expected distribution for a healthy scan: mostly `info` at startup/shutdown, very few `warn`, and zero `error`. A flood of `warn` lines typically indicates an infrastructure issue (Redis down, ingest endpoint unreachable, or DNS resolver problems).
