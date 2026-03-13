# Docker & Kubernetes

This guide covers building and deploying SPYDER with Docker and Kubernetes, including production-ready manifests, health checks, and scaling configuration.

## Dockerfile Overview

SPYDER uses a multi-stage Dockerfile that produces a minimal, secure container image:

```dockerfile
FROM golang:1.23 AS build
WORKDIR /app
COPY . .
RUN go mod download && CGO_ENABLED=0 go build -o /spyder ./cmd/spyder

FROM gcr.io/distroless/base-debian12
USER nonroot:nonroot
COPY --from=build /spyder /usr/local/bin/spyder

LABEL org.opencontainers.image.source=https://github.com/gustycube/spyder
LABEL org.opencontainers.image.description="SPYDER - System for Probing and Yielding DNS-based Entity Relations"
LABEL org.opencontainers.image.licenses=MIT

ENTRYPOINT ["/usr/local/bin/spyder"]
```

**Stage 1 (`build`):** Compiles the Go binary with `CGO_ENABLED=0` for a fully static binary. The `golang:1.23` base image includes all build dependencies.

**Stage 2 (runtime):** Copies only the compiled binary into Google's distroless base image. This produces a final image that:

- Contains no shell, package manager, or other OS utilities
- Runs as a non-root user (`nonroot:nonroot`)
- Is typically under 30MB in size
- Has a minimal attack surface for production use

### Building the Image

```bash
# Build with default tag
docker build -t spyder-probe:latest .

# Build with version tag
docker build -t spyder-probe:v1.0.0 .

# Build for a specific platform
docker build --platform linux/amd64 -t spyder-probe:latest .

# Multi-platform build (requires buildx)
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t ghcr.io/gustycube/spyder:latest \
  --push .
```

### Building the Seed Utility

The seed utility is not included in the main image. Build it separately if needed:

```dockerfile
# Dockerfile.seed
FROM golang:1.23 AS build
WORKDIR /app
COPY . .
RUN go mod download && CGO_ENABLED=0 go build -o /seed ./cmd/seed

FROM gcr.io/distroless/base-debian12
USER nonroot:nonroot
COPY --from=build /seed /usr/local/bin/seed
ENTRYPOINT ["/usr/local/bin/seed"]
```

```bash
docker build -f Dockerfile.seed -t spyder-seed:latest .
```

## Running with Docker

### Standalone Container

Run SPYDER with a local domains file:

```bash
docker run --rm \
  -v $(pwd)/domains.txt:/data/domains.txt:ro \
  -v $(pwd)/spool:/data/spool \
  -p 9090:9090 \
  spyder-probe:latest \
  -domains=/data/domains.txt \
  -probe=docker-1 \
  -concurrency=128 \
  -metrics_addr=:9090 \
  -spool_dir=/data/spool
```

### With Redis

```bash
# Start Redis first
docker run -d --name redis \
  -p 6379:6379 \
  redis:7-alpine \
  redis-server --appendonly yes

# Run SPYDER with Redis dedup
docker run --rm \
  --link redis:redis \
  -e REDIS_ADDR=redis:6379 \
  -v $(pwd)/domains.txt:/data/domains.txt:ro \
  -p 9090:9090 \
  spyder-probe:latest \
  -domains=/data/domains.txt \
  -probe=docker-1 \
  -concurrency=128 \
  -metrics_addr=:9090
```

### Distributed Mode with Docker

```bash
# Seed the queue
docker run --rm \
  --link redis:redis \
  -v $(pwd)/domains.txt:/data/domains.txt:ro \
  spyder-seed:latest \
  -domains=/data/domains.txt \
  -redis=redis:6379

# Run probes
docker run -d --name probe-1 \
  --link redis:redis \
  -e REDIS_ADDR=redis:6379 \
  -e REDIS_QUEUE_ADDR=redis:6379 \
  -e REDIS_QUEUE_KEY=spyder:queue \
  -p 9091:9090 \
  spyder-probe:latest \
  -domains=/dev/null \
  -probe=probe-1 \
  -run=campaign-1 \
  -continuous \
  -max_domains=100000 \
  -concurrency=256 \
  -metrics_addr=:9090

docker run -d --name probe-2 \
  --link redis:redis \
  -e REDIS_ADDR=redis:6379 \
  -e REDIS_QUEUE_ADDR=redis:6379 \
  -e REDIS_QUEUE_KEY=spyder:queue \
  -p 9092:9090 \
  spyder-probe:latest \
  -domains=/dev/null \
  -probe=probe-2 \
  -run=campaign-1 \
  -continuous \
  -max_domains=100000 \
  -concurrency=256 \
  -metrics_addr=:9090
```

## Docker Compose

### Production Stack

The production `docker-compose.yml` includes Redis, Prometheus, Grafana, and the SPYDER probe:

```yaml
version: '3.8'

services:
  # Redis for deduplication and work queue
  redis:
    image: redis:7-alpine
    container_name: spyder-redis
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    command: redis-server --appendonly yes
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5
    networks:
      - spyder-network

  # Prometheus for metrics collection
  prometheus:
    image: prom/prometheus:latest
    container_name: spyder-prometheus
    ports:
      - "9091:9090"
    volumes:
      - ./configs/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/usr/share/prometheus/console_libraries'
      - '--web.console.templates=/usr/share/prometheus/consoles'
    networks:
      - spyder-network
    depends_on:
      - spyder

  # Grafana for visualization
  grafana:
    image: grafana/grafana:latest
    container_name: spyder-grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_SECURITY_ADMIN_USER=admin
    volumes:
      - grafana-data:/var/lib/grafana
      - ./configs/grafana/dashboards:/etc/grafana/provisioning/dashboards
      - ./configs/grafana/datasources:/etc/grafana/provisioning/datasources
    networks:
      - spyder-network
    depends_on:
      - prometheus

  # SPYDER probe service
  spyder:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: spyder-probe
    environment:
      - REDIS_ADDR=redis:6379
      - REDIS_QUEUE_ADDR=redis:6379
      - REDIS_QUEUE_KEY=spyder:queue
    volumes:
      - ./configs:/app/configs
      - ./spool:/app/spool
      - ./output:/app/output
    command: >
      /usr/local/bin/spyder
      -config=/app/configs/docker.yaml
    networks:
      - spyder-network
    depends_on:
      redis:
        condition: service_healthy
    restart: unless-stopped

networks:
  spyder-network:
    driver: bridge

volumes:
  redis-data:
  prometheus-data:
  grafana-data:
```

Start the full stack:

```bash
docker compose up -d
```

Verify all services are healthy:

```bash
docker compose ps
docker compose logs spyder --tail 50
curl -s http://localhost:9090/metrics | head -5
```

### Development Stack

The development `docker-compose.dev.yml` is a stripped-down configuration for local iteration:

```yaml
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    container_name: spyder-dev-redis
    ports:
      - "6379:6379"
    command: redis-server --appendonly yes
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

  spyder:
    build:
      context: .
      dockerfile: Dockerfile
      target: builder
    container_name: spyder-dev
    environment:
      - REDIS_ADDR=redis:6379
      - LOG_LEVEL=debug
    volumes:
      - ./configs:/app/configs
      - ./spool:/app/spool
      - ./bin:/app/bin
    command: >
      /app/bin/spyder
      -domains=/app/configs/domains.txt
      -concurrency=64
      -metrics_addr=:9090
    ports:
      - "9090:9090"
    depends_on:
      redis:
        condition: service_healthy
    restart: unless-stopped
```

The development stack mounts the local `bin/` directory so you can rebuild the Go binary on your host and restart the container without a full Docker build:

```bash
# Build locally
go build -o bin/spyder ./cmd/spyder

# Start dev stack
docker compose -f docker-compose.dev.yml up -d

# Rebuild and restart after code changes
go build -o bin/spyder ./cmd/spyder
docker compose -f docker-compose.dev.yml restart spyder

# View logs
docker compose -f docker-compose.dev.yml logs -f spyder
```

### Scaling Probes with Compose

Run multiple probe instances using `docker compose up --scale`:

```bash
# Start 3 probe instances
docker compose up -d --scale spyder=3
```

When scaling this way, remove the `container_name` directive from the `spyder` service (Compose requires unique names) and use a template for the probe ID. Alternatively, define named probe services:

```yaml
services:
  probe-1:
    <<: *probe-defaults
    container_name: spyder-probe-1
    command: >
      /usr/local/bin/spyder
      -config=/app/configs/docker.yaml
      -probe=probe-1

  probe-2:
    <<: *probe-defaults
    container_name: spyder-probe-2
    command: >
      /usr/local/bin/spyder
      -config=/app/configs/docker.yaml
      -probe=probe-2
```

## Kubernetes Deployment

### Namespace and ConfigMap

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: spyder
  labels:
    app.kubernetes.io/name: spyder
---
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: spyder-config
  namespace: spyder
data:
  config.yaml: |
    domains: /dev/null
    run: campaign-2026-03
    concurrency: 256
    metrics_addr: ":9090"
    batch_max_edges: 10000
    batch_flush_sec: 2
    spool_dir: /data/spool
    ua: "SPYDERProbe/1.0 (+https://yourcompany.com/security)"
    exclude_tlds:
      - gov
      - mil
      - int
  domains.txt: |
    google.com
    amazon.com
    microsoft.com
    cloudflare.com
    fastly.com
```

### Deployment

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: spyder-probe
  namespace: spyder
  labels:
    app.kubernetes.io/name: spyder
    app.kubernetes.io/component: probe
spec:
  replicas: 3
  selector:
    matchLabels:
      app.kubernetes.io/name: spyder
      app.kubernetes.io/component: probe
  template:
    metadata:
      labels:
        app.kubernetes.io/name: spyder
        app.kubernetes.io/component: probe
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: spyder
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        runAsGroup: 65534
        fsGroup: 65534
      containers:
        - name: spyder
          image: ghcr.io/gustycube/spyder:latest
          args:
            - "-config=/etc/spyder/config.yaml"
            - "-probe=$(POD_NAME)"
            - "-continuous"
            - "-max_domains=200000"
          env:
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: REDIS_ADDR
              valueFrom:
                secretKeyRef:
                  name: spyder-redis
                  key: addr
            - name: REDIS_QUEUE_ADDR
              valueFrom:
                secretKeyRef:
                  name: spyder-redis
                  key: addr
            - name: REDIS_QUEUE_KEY
              value: "spyder:queue"
          ports:
            - name: metrics
              containerPort: 9090
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /live
              port: metrics
            initialDelaySeconds: 10
            periodSeconds: 15
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /ready
              port: metrics
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 2
          startupProbe:
            httpGet:
              path: /live
              port: metrics
            initialDelaySeconds: 5
            periodSeconds: 5
            failureThreshold: 12
          resources:
            requests:
              cpu: "2"
              memory: "4Gi"
            limits:
              cpu: "4"
              memory: "8Gi"
          volumeMounts:
            - name: config
              mountPath: /etc/spyder
              readOnly: true
            - name: spool
              mountPath: /data/spool
      volumes:
        - name: config
          configMap:
            name: spyder-config
        - name: spool
          emptyDir:
            sizeLimit: 5Gi
      terminationGracePeriodSeconds: 60
```

### Service

Expose the metrics port for Prometheus scraping:

```yaml
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: spyder-probe
  namespace: spyder
  labels:
    app.kubernetes.io/name: spyder
    app.kubernetes.io/component: probe
spec:
  selector:
    app.kubernetes.io/name: spyder
    app.kubernetes.io/component: probe
  ports:
    - name: metrics
      port: 9090
      targetPort: metrics
      protocol: TCP
  clusterIP: None  # Headless service for per-pod scraping
```

### Redis Secret

```bash
kubectl -n spyder create secret generic spyder-redis \
  --from-literal=addr=redis.spyder.svc.cluster.local:6379
```

### ServiceAccount and RBAC

```yaml
# rbac.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: spyder
  namespace: spyder
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: spyder-role
  namespace: spyder
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: spyder-rolebinding
  namespace: spyder
subjects:
  - kind: ServiceAccount
    name: spyder
    namespace: spyder
roleRef:
  kind: Role
  name: spyder-role
  apiGroup: rbac.authorization.k8s.io
```

### Redis Deployment (In-Cluster)

```yaml
# redis.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
  namespace: spyder
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: redis
  template:
    metadata:
      labels:
        app.kubernetes.io/name: redis
    spec:
      containers:
        - name: redis
          image: redis:7-alpine
          command: ["redis-server", "--appendonly", "yes", "--maxmemory", "2gb", "--maxmemory-policy", "noeviction"]
          ports:
            - containerPort: 6379
          resources:
            requests:
              cpu: "500m"
              memory: "2Gi"
            limits:
              cpu: "1"
              memory: "4Gi"
          volumeMounts:
            - name: redis-data
              mountPath: /data
          livenessProbe:
            exec:
              command: ["redis-cli", "ping"]
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            exec:
              command: ["redis-cli", "ping"]
            initialDelaySeconds: 3
            periodSeconds: 5
      volumes:
        - name: redis-data
          persistentVolumeClaim:
            claimName: redis-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: redis
  namespace: spyder
spec:
  selector:
    app.kubernetes.io/name: redis
  ports:
    - port: 6379
      targetPort: 6379
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: redis-pvc
  namespace: spyder
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 10Gi
```

### Seeding from a Job

Run the seed utility as a one-shot Kubernetes Job:

```yaml
# seed-job.yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: spyder-seed
  namespace: spyder
spec:
  backoffLimit: 3
  template:
    spec:
      restartPolicy: OnFailure
      containers:
        - name: seed
          image: ghcr.io/gustycube/spyder-seed:latest
          args:
            - "-domains=/etc/spyder/domains.txt"
            - "-redis=redis.spyder.svc.cluster.local:6379"
            - "-key=spyder:queue"
          volumeMounts:
            - name: config
              mountPath: /etc/spyder
              readOnly: true
      volumes:
        - name: config
          configMap:
            name: spyder-config
```

```bash
kubectl apply -f seed-job.yaml
kubectl -n spyder logs job/spyder-seed
```

## Health Checks

SPYDER exposes three health endpoints on the metrics port (default `:9090`):

### Liveness Probe (`/live`)

Returns `200 OK` as long as the process is running. Used by Kubernetes to detect deadlocked processes.

```bash
curl -s http://localhost:9090/live | jq .
```

```json
{
  "alive": true,
  "timestamp": "2026-03-13T14:30:00Z"
}
```

### Readiness Probe (`/ready`)

Returns `200 OK` once the probe has fully initialized (config loaded, Redis connected, workers started). Returns `503 Service Unavailable` during startup. Used by Kubernetes to control traffic routing.

```bash
curl -s http://localhost:9090/ready | jq .
```

```json
{
  "ready": true,
  "timestamp": "2026-03-13T14:30:00Z",
  "metadata": {
    "probe": "probe-east-1",
    "run": "campaign-2026-03",
    "version": "1.0.0"
  }
}
```

### Health Check (`/health`)

Returns detailed component-level health status, including Redis connectivity. Returns `503` when any component is unhealthy.

```bash
curl -s http://localhost:9090/health | jq .
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
      "last_checked": "2026-03-13T14:30:00Z",
      "duration_ms": 1
    }
  ],
  "metadata": {
    "probe": "probe-east-1",
    "run": "campaign-2026-03",
    "version": "1.0.0"
  }
}
```

### Kubernetes Probe Configuration

The Deployment manifest above includes all three probe types. Key tuning parameters:

| Parameter | Liveness | Readiness | Startup |
|---|---|---|---|
| `initialDelaySeconds` | 10 | 5 | 5 |
| `periodSeconds` | 15 | 10 | 5 |
| `timeoutSeconds` | 5 | 5 | 5 |
| `failureThreshold` | 3 | 2 | 12 |

The startup probe gives the container up to 60 seconds (12 checks x 5s) to initialize before the liveness probe takes over. This prevents premature restarts during slow Redis connections or large config loads.

## Resource Limits and Scaling

### Resource Guidelines

SPYDER is CPU-bound during DNS resolution and TLS handshakes, and memory-bound during HTML parsing and dedup tracking. Use these guidelines as a starting point:

| Concurrency | CPU Request | CPU Limit | Memory Request | Memory Limit |
|---|---|---|---|---|
| 64 | 500m | 1 | 1Gi | 2Gi |
| 128 | 1 | 2 | 2Gi | 4Gi |
| 256 | 2 | 4 | 4Gi | 8Gi |
| 512 | 4 | 8 | 8Gi | 16Gi |

### Horizontal Pod Autoscaler

Scale probe replicas based on CPU utilization:

```yaml
# hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: spyder-probe
  namespace: spyder
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: spyder-probe
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 120
      policies:
        - type: Pods
          value: 2
          periodSeconds: 60
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
        - type: Pods
          value: 1
          periodSeconds: 120
```

### Pod Disruption Budget

Ensure at least one probe is always running during node maintenance:

```yaml
# pdb.yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: spyder-probe
  namespace: spyder
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: spyder
      app.kubernetes.io/component: probe
```

### Deploying Everything

Apply all manifests in order:

```bash
kubectl apply -f namespace.yaml
kubectl apply -f rbac.yaml
kubectl apply -f redis.yaml
kubectl apply -f configmap.yaml
kubectl -n spyder create secret generic spyder-redis \
  --from-literal=addr=redis.spyder.svc.cluster.local:6379
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f hpa.yaml
kubectl apply -f pdb.yaml

# Seed the queue
kubectl apply -f seed-job.yaml

# Watch rollout
kubectl -n spyder rollout status deployment/spyder-probe

# Check pod health
kubectl -n spyder get pods -l app.kubernetes.io/name=spyder
kubectl -n spyder logs -l app.kubernetes.io/component=probe --tail=20
```
