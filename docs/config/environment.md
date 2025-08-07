# Environment Variables Reference

SPYDER uses environment variables for configuration that changes between deployment environments or contains sensitive information.

## Redis Configuration

### `REDIS_ADDR`

Redis server address for deduplication backend.

```bash
export REDIS_ADDR=127.0.0.1:6379
export REDIS_ADDR=redis.example.com:6379
export REDIS_ADDR=redis-cluster.local:6379
```

**Default:** Not set (uses memory deduplication)

**Usage:**
- Enables distributed deduplication across multiple probe instances
- Redis server must be accessible from all probe nodes
- Uses Redis SET operations with TTL for deduplication keys
- Default TTL: 24 hours

### `REDIS_QUEUE_ADDR`

Redis server address for distributed queue operations.

```bash
export REDIS_QUEUE_ADDR=127.0.0.1:6379
export REDIS_QUEUE_ADDR=queue-redis.example.com:6379
```

**Default:** Not set (uses file-based domain input)

**Requirements:**
- Redis server with LIST support
- Network connectivity from all probe instances
- Sufficient memory for queue storage

### `REDIS_QUEUE_KEY`

Redis key name for the work queue.

```bash
export REDIS_QUEUE_KEY=spyder:queue
export REDIS_QUEUE_KEY=production:spyder:domains
export REDIS_QUEUE_KEY=env:${ENVIRONMENT}:spyder:queue
```

**Default:** `spyder:queue`

**Key Structure:**
- Main queue: `{key}` - Contains pending work items
- Processing queue: `{key}:processing` - Contains items being processed
- Uses atomic BRPOPLPUSH operations

## OpenTelemetry Configuration

### `OTEL_EXPORTER_OTLP_ENDPOINT`

OTLP endpoint for distributed tracing export.

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4318
export OTEL_EXPORTER_OTLP_ENDPOINT=https://otel-collector.monitoring.local:4318
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```

**Protocol:** HTTP/1.1 with JSON payload
**Default Port:** 4318 (HTTP), 4317 (gRPC)

### `OTEL_EXPORTER_OTLP_INSECURE`

Enable insecure (non-TLS) OTLP connections.

```bash
export OTEL_EXPORTER_OTLP_INSECURE=true   # HTTP
export OTEL_EXPORTER_OTLP_INSECURE=false  # HTTPS
```

**Default:** `true` (via CLI flag)
**Production Recommendation:** `false`

## Deployment Environment Variables

### Container Environments

**Docker Compose Example:**
```yaml
version: '3.8'
services:
  spyder:
    image: spyder-probe:latest
    environment:
      - REDIS_ADDR=redis:6379
      - REDIS_QUEUE_ADDR=redis:6379
      - REDIS_QUEUE_KEY=spyder:production:queue
      - OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4318
      - OTEL_EXPORTER_OTLP_INSECURE=true
    volumes:
      - ./domains.txt:/domains.txt
    command: ["-domains=/domains.txt", "-ingest=https://api.example.com/ingest"]
```

**Kubernetes Deployment:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: spyder-probe
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: spyder
        image: spyder-probe:latest
        env:
        - name: REDIS_ADDR
          value: "redis.default.svc.cluster.local:6379"
        - name: REDIS_QUEUE_ADDR  
          value: "redis.default.svc.cluster.local:6379"
        - name: REDIS_QUEUE_KEY
          value: "spyder:k8s:queue"
        - name: OTEL_EXPORTER_OTLP_ENDPOINT
          value: "http://jaeger-collector.monitoring.svc.cluster.local:4318"
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        args:
        - "-probe=$(POD_NAME)"
        - "-ingest=https://ingest.production.com/v1/batch"
```

### Systemd Service Environment

**Environment file (`/opt/spyder/.env`):**
```bash
# Redis Configuration
REDIS_ADDR=127.0.0.1:6379
REDIS_QUEUE_ADDR=127.0.0.1:6379
REDIS_QUEUE_KEY=spyder:production:queue

# OpenTelemetry
OTEL_EXPORTER_OTLP_ENDPOINT=http://monitoring.local:4318
OTEL_EXPORTER_OTLP_INSECURE=false

# Custom Application Settings
SPYDER_ENVIRONMENT=production
SPYDER_REGION=us-west-1
```

**Systemd service configuration:**
```ini
[Unit]
Description=SPYDER Probe
After=redis.service

[Service]
Type=simple
User=spyder
EnvironmentFile=/opt/spyder/.env
ExecStart=/opt/spyder/bin/spyder -domains=/opt/spyder/domains.txt
```

## Security Considerations

### Sensitive Data Handling

**Avoid storing sensitive data in environment variables:**
```bash
# ❌ Don't do this
export INGEST_API_KEY=secret123
export DATABASE_PASSWORD=password123

# ✅ Use files or secret management instead
export INGEST_API_KEY_FILE=/etc/secrets/api-key
export MTLS_CERT_FILE=/etc/spyder/client.crt
```

**Secret Management Integration:**
```bash
# AWS Secrets Manager
export REDIS_ADDR=$(aws secretsmanager get-secret-value --secret-id redis-addr --query SecretString --output text)

# HashiCorp Vault
export REDIS_ADDR=$(vault kv get -field=address secret/spyder/redis)

# Kubernetes Secrets
# (Mounted as files in containers)
export REDIS_ADDR=$(cat /var/secrets/redis/address)
```

### Network Security

**Redis Security:**
```bash
# Use AUTH if Redis has authentication
export REDIS_ADDR=127.0.0.1:6379
# Redis AUTH handled via Redis configuration, not environment variables

# Use TLS for Redis connections (Redis 6+)
export REDIS_TLS=true
export REDIS_ADDR=rediss://redis.secure.com:6380
```

## Development vs Production

### Development Environment
```bash
#!/bin/bash
# dev.env
export REDIS_ADDR=localhost:6379
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_EXPORTER_OTLP_INSECURE=true
export SPYDER_ENVIRONMENT=development
```

### Staging Environment
```bash
#!/bin/bash
# staging.env
export REDIS_ADDR=redis-staging.internal:6379
export REDIS_QUEUE_KEY=spyder:staging:queue
export OTEL_EXPORTER_OTLP_ENDPOINT=https://otel-staging.internal:4318
export OTEL_EXPORTER_OTLP_INSECURE=false
export SPYDER_ENVIRONMENT=staging
```

### Production Environment
```bash
#!/bin/bash
# production.env
export REDIS_ADDR=redis-cluster-prod.internal:6379
export REDIS_QUEUE_KEY=spyder:production:queue
export OTEL_EXPORTER_OTLP_ENDPOINT=https://otel-prod.monitoring.internal:4318
export OTEL_EXPORTER_OTLP_INSECURE=false
export SPYDER_ENVIRONMENT=production
```

## Validation and Testing

### Environment Validation Script
```bash
#!/bin/bash
# validate-env.sh

echo "Validating SPYDER environment configuration..."

# Check Redis connectivity
if [[ -n "$REDIS_ADDR" ]]; then
    echo "Testing Redis connection to $REDIS_ADDR..."
    if command -v redis-cli >/dev/null 2>&1; then
        redis-cli -h "${REDIS_ADDR%:*}" -p "${REDIS_ADDR#*:}" ping
    else
        echo "redis-cli not available for testing"
    fi
fi

# Check OpenTelemetry endpoint
if [[ -n "$OTEL_EXPORTER_OTLP_ENDPOINT" ]]; then
    echo "Testing OTLP endpoint $OTEL_EXPORTER_OTLP_ENDPOINT..."
    curl -s -o /dev/null -w "%{http_code}" "$OTEL_EXPORTER_OTLP_ENDPOINT/v1/traces"
fi

# Validate required environment
required_vars=("SPYDER_ENVIRONMENT")
for var in "${required_vars[@]}"; do
    if [[ -z "${!var}" ]]; then
        echo "❌ Required environment variable $var is not set"
        exit 1
    else
        echo "✅ $var=${!var}"
    fi
done

echo "Environment validation complete"
```

### Environment-Specific Configs

**Multi-environment deployment script:**
```bash
#!/bin/bash
# deploy.sh

ENVIRONMENT=${1:-development}
ENV_FILE="environments/${ENVIRONMENT}.env"

if [[ ! -f "$ENV_FILE" ]]; then
    echo "Environment file $ENV_FILE not found"
    exit 1
fi

echo "Loading environment: $ENVIRONMENT"
source "$ENV_FILE"

# Validate critical environment variables
if [[ "$ENVIRONMENT" == "production" ]]; then
    required_vars=("REDIS_ADDR" "OTEL_EXPORTER_OTLP_ENDPOINT")
    for var in "${required_vars[@]}"; do
        if [[ -z "${!var}" ]]; then
            echo "Production deployment requires $var"
            exit 1
        fi
    done
fi

# Start SPYDER with environment-specific configuration
./bin/spyder \
    -domains="domains/${ENVIRONMENT}.txt" \
    -probe="${ENVIRONMENT}-$(hostname)" \
    -ingest="${INGEST_ENDPOINT}" \
    -metrics_addr=":9090"
```

## Troubleshooting

### Common Issues

**Redis Connection Issues:**
```bash
# Test Redis connectivity
redis-cli -h "${REDIS_ADDR%:*}" -p "${REDIS_ADDR#*:}" ping

# Check Redis logs
docker logs redis-container

# Verify network connectivity
telnet "${REDIS_ADDR%:*}" "${REDIS_ADDR#*:}"
```

**Environment Variable Not Set:**
```bash
# Check if variable is set
echo "REDIS_ADDR: ${REDIS_ADDR:-'not set'}"

# List all SPYDER-related environment variables
env | grep -E "(REDIS|OTEL|SPYDER)"

# Verify environment file is loaded
source .env && env | grep REDIS
```

**OpenTelemetry Connectivity:**
```bash
# Test OTLP endpoint
curl -v "$OTEL_EXPORTER_OTLP_ENDPOINT/v1/traces"

# Check if traces are being sent
# (look for HTTP requests in SPYDER logs)
```

### Debug Commands

```bash
# Print effective configuration
./bin/spyder -h

# Test with minimal environment
env -i REDIS_ADDR=localhost:6379 ./bin/spyder -domains=test.txt

# Validate environment loading in systemd
systemctl show spyder --property=Environment
```