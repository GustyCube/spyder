# OpenTelemetry Tracing

SPYDER supports distributed tracing via OpenTelemetry (OTEL). Traces capture the lifecycle of each domain crawl, from DNS resolution through TLS analysis and HTTP fetching, providing visibility into per-domain processing time and failure modes.

## Configuration

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-otel_endpoint` | `""` (disabled) | OTLP HTTP endpoint in `host:port` format |
| `-otel_insecure` | `true` | Use plain HTTP instead of HTTPS for OTLP |
| `-otel_service` | `spyder-probe` | Service name reported in traces |

### Enable Tracing

```bash
# Send traces to a local Jaeger instance
./bin/spyder -domains=domains.txt \
  -otel_endpoint=localhost:4318 \
  -otel_insecure=true \
  -otel_service=spyder-probe

# Send traces to a remote collector with TLS
./bin/spyder -domains=domains.txt \
  -otel_endpoint=otel-collector.monitoring.internal:4318 \
  -otel_insecure=false \
  -otel_service=spyder-prod-us-west
```

### Disable Tracing

Tracing is disabled by default. When `-otel_endpoint` is empty (the default), no trace data is collected or exported, and there is no performance overhead.

```bash
# Tracing disabled (default)
./bin/spyder -domains=domains.txt

# Explicitly disabled
./bin/spyder -domains=domains.txt -otel_endpoint=""
```

### Configuration via YAML

```yaml
# config.yaml
otel_endpoint: "jaeger.monitoring.internal:4318"
otel_insecure: false
otel_service: "spyder-production"
```

```bash
./bin/spyder -config=config.yaml -domains=domains.txt
```

## OTLP HTTP Exporter

SPYDER uses the OTLP HTTP exporter (`otlptracehttp`) to send trace data. This exporter sends trace spans as Protocol Buffers over HTTP to port 4318 (the standard OTLP HTTP port).

### How It Works

The telemetry subsystem initializes the exporter at startup:

```go
// internal/telemetry/otel.go
func Init(ctx context.Context, endpoint, serviceName string, insecure bool) (func(context.Context) error, error) {
    if endpoint == "" {
        return func(context.Context) error { return nil }, nil
    }
    clientOpts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(endpoint)}
    if insecure {
        clientOpts = append(clientOpts, otlptracehttp.WithInsecure())
    }
    exp, err := otlptracehttp.New(ctx, clientOpts...)
    // ...
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exp, trace.WithBatchTimeout(3*time.Second)),
        trace.WithResource(res),
    )
    otel.SetTracerProvider(tp)
    return tp.Shutdown, nil
}
```

Key details:

- **Protocol**: OTLP over HTTP (not gRPC)
- **Port**: 4318 is the standard OTLP HTTP receiver port
- **Batching**: Traces are batched with a 3-second flush timeout, reducing network overhead
- **Resource attributes**: Each trace includes the `service.name` attribute set from `-otel_service`
- **Shutdown**: On graceful shutdown (SIGINT/SIGTERM), pending spans are flushed before exit

### Endpoint Format

The `-otel_endpoint` value should be `host:port` without a scheme prefix. The scheme is determined by `-otel_insecure`:

```bash
# Correct: host:port only
-otel_endpoint=jaeger:4318
-otel_endpoint=otel-collector.monitoring.svc.cluster.local:4318

# Incorrect: do not include http:// or https://
# -otel_endpoint=http://jaeger:4318    (wrong)
```

## Trace Spans

### CrawlOne Span

SPYDER creates one trace span per domain crawled. The `CrawlOne` span in the probe package wraps the entire processing pipeline for a single domain:

```go
func (p *Probe) CrawlOne(ctx context.Context, host string) {
    tr := otel.Tracer("spyder/probe")
    ctx, span := tr.Start(ctx, "CrawlOne")
    defer span.End()
    // ... DNS resolution, HTTP fetch, TLS analysis, link extraction
}
```

Each `CrawlOne` span encompasses:

1. **DNS resolution**: A, AAAA, CNAME, NS, and MX record lookups
2. **Robots.txt check**: Fetch and evaluate robots.txt policy
3. **Per-host rate limiting**: Wait for rate limiter clearance
4. **HTTP GET**: Fetch the root page (with 15-second timeout)
5. **Link extraction**: Parse HTML and extract external links
6. **TLS certificate fetch**: Retrieve and analyze the TLS certificate
7. **Deduplication checks**: Filter previously-seen nodes and edges
8. **Batch emission**: Send discovered data to the output channel

### Span Attributes

The `CrawlOne` span is created under the `spyder/probe` tracer with the operation name `CrawlOne`. The span's context propagates to all child operations, so any instrumented HTTP clients or DNS resolvers within the call tree will appear as child spans.

### What Traces Reveal

A typical `CrawlOne` trace shows:

- **Total duration**: How long the entire domain crawl took
- **DNS latency**: Time spent resolving DNS records
- **HTTP latency**: Time for the HTTP GET request (up to 15-second timeout)
- **TLS handshake time**: Duration of TLS certificate retrieval
- **Error information**: Any failures are recorded on the span

Slow domains are immediately visible as long-duration spans in Jaeger or Zipkin.

## Integration with Jaeger

### Local Jaeger Setup

Run Jaeger all-in-one for development:

```bash
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4318:4318 \
  jaegertracing/all-in-one:latest
```

Then point SPYDER at it:

```bash
./bin/spyder -domains=domains.txt \
  -otel_endpoint=localhost:4318 \
  -otel_insecure=true \
  -otel_service=spyder-dev
```

Open `http://localhost:16686` to view traces.

### Finding Slow Domains

In the Jaeger UI:

1. Select service `spyder-dev` (or your `-otel_service` name)
2. Set operation to `CrawlOne`
3. Sort by duration (descending)
4. Click on the longest spans to see where time was spent

### Production Jaeger

For production, deploy Jaeger with persistent storage:

```yaml
# jaeger-values.yaml (Helm)
collector:
  service:
    otlp:
      http:
        name: otlp-http
        port: 4318
storage:
  type: elasticsearch
  options:
    es:
      server-urls: https://elasticsearch.internal:9200
      index-prefix: jaeger
```

## Integration with Zipkin

Zipkin can receive OTLP traces through an OpenTelemetry Collector that translates the protocol:

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318

exporters:
  zipkin:
    endpoint: "http://zipkin:9411/api/v2/spans"

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [zipkin]
```

Run the collector:

```bash
docker run -d --name otel-collector \
  -p 4318:4318 \
  -v $(pwd)/otel-collector-config.yaml:/etc/otelcol/config.yaml \
  otel/opentelemetry-collector:latest
```

Then point SPYDER at the collector:

```bash
./bin/spyder -domains=domains.txt \
  -otel_endpoint=localhost:4318 \
  -otel_insecure=true
```

## Trace Sampling and Performance Impact

### Batching Behavior

SPYDER's trace provider uses `WithBatcher` with a 3-second batch timeout. This means spans are accumulated in memory and sent in batches, reducing the number of HTTP requests to the collector.

### Performance Considerations

| Scenario | Overhead | Notes |
|----------|----------|-------|
| Tracing disabled (`-otel_endpoint=""`) | None | No spans created, no memory allocated |
| Tracing enabled, low concurrency (< 64) | Negligible | Few spans per second |
| Tracing enabled, high concurrency (256+) | Low (~1-2% CPU) | Batch exporter amortizes network cost |
| Tracing enabled, very high concurrency (1024+) | Moderate | Consider sampling |

### When to Use Sampling

At very high concurrency, every domain crawl produces a span. If you are processing thousands of domains per second, the trace volume may overwhelm your collector. In this case, configure sampling at the collector level:

```yaml
# otel-collector-config.yaml
processors:
  probabilistic_sampler:
    sampling_percentage: 10  # Keep 10% of traces

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [probabilistic_sampler]
      exporters: [jaeger]
```

This keeps 10% of traces, which is sufficient for performance analysis while reducing storage and network costs.

### Head-Based vs. Tail-Based Sampling

For SPYDER workloads, tail-based sampling is more useful because it can retain traces for slow or errored domains:

```yaml
processors:
  tail_sampling:
    policies:
      - name: errors
        type: status_code
        status_code: {status_codes: [ERROR]}
      - name: slow
        type: latency
        latency: {threshold_ms: 5000}
      - name: random
        type: probabilistic
        probabilistic: {sampling_percentage: 5}
```

This keeps all error traces, all traces over 5 seconds, and a 5% random sample of the rest.

## Docker Compose Setup

### Full Observability Stack

Run SPYDER with Jaeger and Prometheus for complete observability:

```yaml
# docker-compose.yml
version: "3.8"

services:
  spyder:
    build: .
    command: >
      ./bin/spyder
        -domains=/data/domains.txt
        -concurrency=256
        -otel_endpoint=jaeger:4318
        -otel_insecure=true
        -otel_service=spyder-probe
        -metrics_addr=:9090
        -ingest=https://ingest.example.com/v1/batch
    volumes:
      - ./domains.txt:/data/domains.txt:ro
    depends_on:
      - jaeger
    networks:
      - spyder-net

  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"  # Jaeger UI
      - "4318:4318"    # OTLP HTTP
    environment:
      - COLLECTOR_OTLP_ENABLED=true
    networks:
      - spyder-net

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
    networks:
      - spyder-net

networks:
  spyder-net:
    driver: bridge
```

### Prometheus Configuration

```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: spyder
    static_configs:
      - targets: ["spyder:9090"]
    scrape_interval: 30s
```

### Run the Stack

```bash
# Start all services
docker compose up -d

# View SPYDER logs
docker compose logs -f spyder

# Open Jaeger UI
open http://localhost:16686

# Open Prometheus
open http://localhost:9091
```

### With OpenTelemetry Collector

For production-grade setups, add an OpenTelemetry Collector between SPYDER and your backends:

```yaml
# docker-compose.yml (additional service)
  otel-collector:
    image: otel/opentelemetry-collector:latest
    command: ["--config=/etc/otelcol/config.yaml"]
    volumes:
      - ./otel-collector-config.yaml:/etc/otelcol/config.yaml:ro
    ports:
      - "4318:4318"
    networks:
      - spyder-net
```

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 5s
    send_batch_size: 1024

exporters:
  otlp/jaeger:
    endpoint: jaeger:4317
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp/jaeger]
```

Update the SPYDER service to point at the collector instead of Jaeger directly:

```yaml
  spyder:
    command: >
      ./bin/spyder
        -domains=/data/domains.txt
        -otel_endpoint=otel-collector:4318
        -otel_insecure=true
```

## Troubleshooting

### Traces Not Appearing

1. **Check endpoint format**: Use `host:port` without scheme prefix
2. **Verify connectivity**: `curl -v http://jaeger:4318/v1/traces` should return a response
3. **Check SPYDER logs**: Look for `otel init failed` warnings in stderr
4. **Verify collector is running**: `docker compose logs jaeger` or `docker compose logs otel-collector`

### Missing Spans After Shutdown

SPYDER flushes pending spans on graceful shutdown (SIGINT/SIGTERM). If you kill the process with SIGKILL, buffered spans will be lost. Always use graceful shutdown:

```bash
# Correct: sends SIGTERM, allows flush
kill $(pgrep spyder)
# or
docker compose stop spyder

# Incorrect: SIGKILL loses buffered spans
kill -9 $(pgrep spyder)
```

### High Collector Load

If the OTEL collector is overwhelmed:

- Increase the batch timeout (currently 3 seconds in SPYDER)
- Add sampling at the collector level
- Scale collector horizontally
- Reduce SPYDER concurrency if trace volume is too high
