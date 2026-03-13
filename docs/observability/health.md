# Health Checks

SPYDER exposes health check endpoints for monitoring service status and integration with container orchestrators like Kubernetes. Health endpoints are served on the same HTTP server as Prometheus metrics, controlled by the `-metrics_addr` flag.

## Endpoint Overview

All health endpoints are registered on the metrics server (default `:9090`):

| Endpoint | Handler | Purpose |
|----------|---------|---------|
| `/live` | `LivenessHandler` | Is the process alive? |
| `/ready` | `ReadinessHandler` | Is the service ready to accept work? |
| `/health` | `HealthHandler` | Detailed health with dependency checks |
| `/metrics` | Prometheus | Prometheus metrics (see [Metrics Reference](../observability/metrics.md)) |

```bash
# Start SPYDER with health endpoints on port 9090 (default)
./bin/spyder -domains=domains.txt -metrics_addr=:9090

# Use a different port
./bin/spyder -domains=domains.txt -metrics_addr=:8081

# Disable metrics and health endpoints
./bin/spyder -domains=domains.txt -metrics_addr=""
```

## Liveness Probe (`/live`)

The liveness endpoint returns HTTP 200 as long as the SPYDER process is running. It performs no dependency checks -- if the HTTP server can respond, the process is alive.

### Request

```bash
curl http://localhost:9090/live
```

### Response

```json
{
  "alive": true,
  "timestamp": "2024-01-15T10:30:00.123456Z"
}
```

**HTTP Status:** Always `200 OK` when the process is running.

This endpoint is intentionally simple. It does not check Redis, the ingestion endpoint, or any other dependency. If the process is deadlocked or otherwise unable to serve HTTP, the liveness check will time out.

## Readiness Probe (`/ready`)

The readiness endpoint reports whether the service has completed initialization and is ready to process domains. SPYDER sets readiness to `true` after all components are initialized -- Redis connections established, emitter configured, worker pool started.

### Request

```bash
curl http://localhost:9090/ready
```

### Response (Ready)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "ready": true,
  "timestamp": "2024-01-15T10:30:00.123456Z",
  "metadata": {
    "probe": "prod-us-west-1",
    "run": "run-1705312200",
    "version": "1.0.0"
  }
}
```

### Response (Not Ready)

```http
HTTP/1.1 503 Service Unavailable
Content-Type: application/json

{
  "ready": false,
  "timestamp": "2024-01-15T10:29:55.000000Z",
  "metadata": {
    "probe": "prod-us-west-1",
    "run": "run-1705312200",
    "version": "1.0.0"
  }
}
```

**HTTP Status:**
- `200 OK` when ready
- `503 Service Unavailable` when not ready

### Readiness Lifecycle

The readiness flag transitions through startup:

1. **Service starts** -- readiness is `false`.
2. **Config loaded, Redis connected, emitter initialized** -- still `false`.
3. **`healthHandler.SetReady(true)` called** -- readiness becomes `true`.
4. **Readiness remains `true`** for the lifetime of the process.

From `cmd/spyder/main.go`:
```go
// Mark service as ready after all components are initialized
healthHandler.SetReady(true)
log.Info("service marked as ready")
```

## Health Endpoint (`/health`)

The health endpoint runs all registered dependency checks and returns a detailed status report. This is the most comprehensive health endpoint, suitable for dashboards and alerting.

### Request

```bash
curl http://localhost:9090/health
```

### Response (Healthy)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00.123456Z",
  "checks": [
    {
      "name": "redis",
      "status": "healthy",
      "message": "Redis connection OK",
      "last_checked": "2024-01-15T10:30:00.123456Z",
      "duration_ms": 1
    }
  ],
  "metadata": {
    "probe": "prod-us-west-1",
    "run": "run-1705312200",
    "version": "1.0.0"
  }
}
```

### Response (Unhealthy)

```http
HTTP/1.1 503 Service Unavailable
Content-Type: application/json

{
  "status": "unhealthy",
  "timestamp": "2024-01-15T10:30:00.123456Z",
  "checks": [
    {
      "name": "redis",
      "status": "unhealthy",
      "message": "Redis connection failed: dial tcp 127.0.0.1:6379: connect: connection refused",
      "last_checked": "2024-01-15T10:30:00.123456Z",
      "duration_ms": 5
    }
  ],
  "metadata": {
    "probe": "prod-us-west-1",
    "run": "run-1705312200",
    "version": "1.0.0"
  }
}
```

### Status Values

| Status | HTTP Code | Meaning |
|--------|-----------|---------|
| `healthy` | `200` | All dependency checks passed |
| `degraded` | `200` | Some checks report issues, but service is functional |
| `unhealthy` | `503` | One or more critical checks failed |

The overall status is the worst status across all registered checks. If any check is `unhealthy`, the overall status is `unhealthy`. If no check is `unhealthy` but one is `degraded`, the overall status is `degraded`.

### Check Timeout

All health checks run with a 5-second timeout context. If a dependency check (e.g., Redis PING) takes longer than 5 seconds, it is cancelled.

## Dependency Health Checks

### Redis Checker

When Redis is configured for deduplication (`REDIS_ADDR`), a Redis health checker is registered automatically. It calls a check function to verify Redis connectivity.

```go
// Registered in cmd/spyder/main.go when Redis dedup is enabled
healthHandler.RegisterChecker("redis", health.NewRedisChecker(cfg.RedisAddr, redisHealthCheck))
```

The checker returns:
- `healthy` with message `"Redis connection OK"` on success.
- `unhealthy` with message `"Redis connection failed: <error>"` on failure.
- `healthy` with message `"Redis not configured"` when no check function is provided.

### Worker Pool Checker

The `WorkerPoolChecker` monitors worker pool utilization:

| Condition | Status | Message |
|-----------|--------|---------|
| Normal utilization | `healthy` | "Worker pool operating normally" |
| Over 90% capacity | `degraded` | "Worker pool near capacity" |
| Zero active workers | `degraded` | "No active workers" |

### Custom Checkers

You can implement the `Checker` interface to add custom health checks:

```go
type Checker interface {
    Check(ctx context.Context) Check
}
```

The returned `Check` struct contains:

```go
type Check struct {
    Name        string        `json:"name"`
    Status      Status        `json:"status"`        // "healthy", "degraded", "unhealthy"
    Message     string        `json:"message,omitempty"`
    LastChecked time.Time     `json:"last_checked"`
    Duration    time.Duration `json:"duration_ms"`
}
```

Register custom checkers before the service starts processing:

```go
healthHandler.RegisterChecker("my-dependency", &MyChecker{})
```

## Metadata

The health handler includes metadata in `/health` and `/ready` responses. SPYDER sets the following metadata at startup:

| Key | Value | Source |
|-----|-------|--------|
| `probe` | Probe identifier | `-probe` flag / `probe` config |
| `run` | Run identifier | `-run` flag / `run` config |
| `version` | `"1.0.0"` | Hardcoded |

Add custom metadata with `SetMetadata`:

```go
healthHandler.SetMetadata("region", "us-west-2")
healthHandler.SetMetadata("cluster", "prod-01")
```

## Kubernetes Probe Configuration

### Basic Configuration

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: spyder-probe
spec:
  template:
    spec:
      containers:
        - name: spyder
          image: spyder:latest
          args:
            - "-domains=/data/domains.txt"
            - "-metrics_addr=:9090"
          ports:
            - name: metrics
              containerPort: 9090
          livenessProbe:
            httpGet:
              path: /live
              port: metrics
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 3
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /ready
              port: metrics
            initialDelaySeconds: 10
            periodSeconds: 5
            timeoutSeconds: 3
            failureThreshold: 3
          startupProbe:
            httpGet:
              path: /ready
              port: metrics
            initialDelaySeconds: 5
            periodSeconds: 5
            failureThreshold: 12
```

### Probe Behavior

- **Liveness probe**: Kubernetes restarts the container if `/live` fails 3 consecutive times (30 seconds of unresponsiveness).
- **Readiness probe**: Kubernetes removes the pod from service endpoints while `/ready` returns 503. Traffic is not routed to the pod until it reports ready.
- **Startup probe**: Allows up to 60 seconds (12 attempts x 5 seconds) for initial startup before liveness checks begin.

### Health-Based Alerting

Use the `/health` endpoint for more detailed monitoring in dashboards and alerting systems, separate from Kubernetes probes:

```yaml
# Example: Prometheus blackbox exporter config
modules:
  spyder_health:
    prober: http
    timeout: 5s
    http:
      method: GET
      valid_status_codes: [200]
      fail_if_body_matches_regexp:
        - '"status":"unhealthy"'
```

## Metrics Server Integration

Health endpoints share the same HTTP server as Prometheus metrics. The server is started in a goroutine by `metrics.ServeWithHealth()`:

```go
func ServeWithHealth(addr string, healthHandler *health.Handler, log *zap.SugaredLogger) {
    http.Handle("/metrics", promhttp.Handler())
    http.HandleFunc("/health", healthHandler.HealthHandler)
    http.HandleFunc("/ready", healthHandler.ReadinessHandler)
    http.HandleFunc("/live", healthHandler.LivenessHandler)
    http.ListenAndServe(addr, nil)
}
```

All four endpoints are served on the same address and port. There is no separate health-only server.

### Verifying Endpoints

```bash
# Check all endpoints are responding
curl -s http://localhost:9090/live | jq .
curl -s http://localhost:9090/ready | jq .
curl -s http://localhost:9090/health | jq .
curl -s http://localhost:9090/metrics | head -5
```

### Security

The metrics and health server binds to all interfaces by default (`:9090`). In production, bind to localhost and use a reverse proxy:

```bash
# Bind to localhost only
./bin/spyder -domains=domains.txt -metrics_addr=127.0.0.1:9090
```

Or restrict access at the network level with Kubernetes `NetworkPolicy`:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: spyder-metrics-access
spec:
  podSelector:
    matchLabels:
      app: spyder
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: monitoring
      ports:
        - port: 9090
          protocol: TCP
```
