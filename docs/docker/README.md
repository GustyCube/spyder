# Docker Deployment Guide

## Quick Start

### Development Environment

For local development with minimal services:

```bash
# Start Redis and SPYDER
make docker-dev

# View logs
docker-compose -f docker-compose.dev.yml logs -f

# Stop services
make docker-dev-down
```

### Full Stack

For the complete monitoring stack with Prometheus and Grafana:

```bash
# Build and start all services
make docker-build
make docker-up

# Access services:
# - Grafana: http://localhost:3000 (admin/admin)
# - Prometheus: http://localhost:9091
# - SPYDER metrics: http://localhost:9090/metrics

# View logs
make docker-logs

# Stop services
make docker-down
```

## Architecture

The Docker Compose setup includes:

- **SPYDER Probe**: The main reconnaissance service
- **Redis**: For deduplication and optional work queue
- **Prometheus**: Metrics collection
- **Grafana**: Visualization dashboards
- **Mock Ingest**: Testing endpoint for batch data

## Configuration

### Environment Variables

Configure SPYDER through environment variables in `docker-compose.yml`:

```yaml
environment:
  - REDIS_ADDR=redis:6379
  - REDIS_QUEUE_ADDR=redis:6379
  - REDIS_QUEUE_KEY=spyder:queue
  - LOG_LEVEL=info
```

### Config Files

- `configs/docker.yaml`: SPYDER configuration for Docker
- `configs/prometheus.yml`: Prometheus scrape configuration
- `configs/grafana/`: Grafana datasources and dashboards

### Volumes

Persistent data is stored in Docker volumes:

- `redis-data`: Redis persistence
- `prometheus-data`: Metrics history
- `grafana-data`: Dashboard configurations
- `./spool`: Failed batch persistence
- `./output`: Output data (if configured)

## Monitoring

### Grafana Dashboard

1. Access Grafana at http://localhost:3000
2. Login with `admin/admin`
3. Navigate to Dashboards → SPYDER Probe Monitoring

The dashboard includes:
- Task processing rate
- Edge discovery rate by type
- Active workers
- HTTP request latency (P95)
- Error rate percentage

### Prometheus Queries

Access Prometheus at http://localhost:9091 for direct queries:

```promql
# Tasks per second
rate(spyder_tasks_total[5m])

# Error rate
rate(spyder_tasks_total{status="error"}[5m]) / rate(spyder_tasks_total[5m])

# Active workers
spyder_active_workers

# P95 HTTP latency
histogram_quantile(0.95, rate(spyder_http_request_duration_seconds_bucket[5m]))
```

## Scaling

### Horizontal Scaling

Run multiple SPYDER instances:

```yaml
# docker-compose.scale.yml
services:
  spyder:
    scale: 3  # Run 3 instances
    deploy:
      replicas: 3
```

```bash
docker-compose -f docker-compose.yml -f docker-compose.scale.yml up --scale spyder=3
```

### Resource Limits

Add resource constraints for production:

```yaml
services:
  spyder:
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 2G
        reservations:
          cpus: '1.0'
          memory: 512M
```

## Troubleshooting

### Common Issues

#### Redis Connection Failed

```bash
# Check Redis health
docker-compose exec redis redis-cli ping

# View Redis logs
docker-compose logs redis
```

#### High Memory Usage

```bash
# Check container resources
docker stats

# Reduce concurrency in configs/docker.yaml
concurrency: 64  # Lower value
```

#### Metrics Not Showing

```bash
# Verify SPYDER metrics endpoint
curl http://localhost:9090/metrics

# Check Prometheus targets
# http://localhost:9091/targets
```

### Debug Mode

Enable debug logging:

```yaml
environment:
  - LOG_LEVEL=debug
```

### Container Shell Access

```bash
# Access SPYDER container
docker-compose exec spyder sh

# Access Redis CLI
docker-compose exec redis redis-cli
```

## Production Considerations

### Security

1. **Change default passwords**:
   - Grafana admin password
   - Add Redis password

2. **Use TLS/mTLS**:
   ```yaml
   services:
     spyder:
       volumes:
         - ./certs:/app/certs
       environment:
         - MTLS_CERT=/app/certs/client.pem
         - MTLS_KEY=/app/certs/client.key
   ```

3. **Network isolation**:
   - Use internal networks for service communication
   - Expose only necessary ports

### Persistence

Ensure data persistence for production:

```yaml
volumes:
  redis-data:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /data/redis
```

### Logging

Configure centralized logging:

```yaml
services:
  spyder:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

### Health Checks

Add health checks for all services:

```yaml
services:
  spyder:
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:9090/metrics"]
      interval: 30s
      timeout: 10s
      retries: 3
```

## Docker Swarm Deployment

For Docker Swarm mode:

```bash
# Initialize swarm
docker swarm init

# Deploy stack
docker stack deploy -c docker-compose.yml spyder-stack

# Scale service
docker service scale spyder-stack_spyder=5

# Monitor services
docker service ls
docker service logs spyder-stack_spyder
```

## Kubernetes Migration

To migrate to Kubernetes, convert Docker Compose with Kompose:

```bash
# Install kompose
curl -L https://github.com/kubernetes/kompose/releases/download/v1.28.0/kompose-linux-amd64 -o kompose

# Convert to Kubernetes manifests
./kompose convert -f docker-compose.yml

# Deploy to Kubernetes
kubectl apply -f .
```