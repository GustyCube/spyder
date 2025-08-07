# Installation Guide

This guide covers installing SPYDER Probe Pro in various environments, from development setups to production deployments.

## System Requirements

### Minimum Requirements
- **CPU**: 2 cores
- **Memory**: 2GB RAM
- **Storage**: 1GB free space
- **OS**: Linux, macOS, or Windows
- **Go**: Version 1.22 or later (for source builds)

### Recommended for Production
- **CPU**: 4+ cores
- **Memory**: 8GB+ RAM
- **Storage**: 10GB+ free space (for spooling and logs)
- **Network**: Stable internet connection with DNS resolution

## Installation Methods

### Method 1: From Source (Recommended)

1. **Install Go 1.22+**
   ```bash
   # Ubuntu/Debian
   sudo apt update
   sudo apt install golang-go
   
   # CentOS/RHEL/Fedora
   sudo dnf install golang
   
   # macOS (with Homebrew)
   brew install go
   ```

2. **Clone and build SPYDER**
   ```bash
   git clone https://github.com/gustycube/spyder-probe.git
   cd spyder-probe
   go mod download
   make build
   ```

3. **Verify installation**
   ```bash
   ./bin/spyder -h
   ```

### Method 2: Docker Container

1. **Build Docker image**
   ```bash
   git clone https://github.com/gustycube/spyder-probe.git
   cd spyder-probe
   make docker
   ```

2. **Run with Docker**
   ```bash
   # Create domains file
   echo -e "example.com\ngoogle.com" > domains.txt
   
   # Run SPYDER container
   docker run --rm -v $(pwd)/domains.txt:/domains.txt \
     spyder-probe:latest -domains=/domains.txt
   ```

### Method 3: Pre-built Binaries

Download pre-built binaries from GitHub releases:

```bash
# Linux amd64
wget https://github.com/gustycube/spyder-probe/releases/latest/download/spyder-linux-amd64
chmod +x spyder-linux-amd64
sudo mv spyder-linux-amd64 /usr/local/bin/spyder

# macOS amd64
wget https://github.com/gustycube/spyder-probe/releases/latest/download/spyder-darwin-amd64
chmod +x spyder-darwin-amd64
sudo mv spyder-darwin-amd64 /usr/local/bin/spyder
```

## Production Installation

### System Service Setup (systemd)

1. **Create spyder user**
   ```bash
   sudo useradd -r -s /bin/false -d /opt/spyder spyder
   sudo mkdir -p /opt/spyder
   sudo chown spyder:spyder /opt/spyder
   ```

2. **Install binary and configuration**
   ```bash
   sudo cp bin/spyder /opt/spyder/
   sudo cp configs/domains.txt /opt/spyder/
   sudo chown spyder:spyder /opt/spyder/spyder /opt/spyder/domains.txt
   sudo chmod +x /opt/spyder/spyder
   ```

3. **Install systemd service**
   ```bash
   sudo cp scripts/spyder.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable spyder
   ```

4. **Start and verify service**
   ```bash
   sudo systemctl start spyder
   sudo systemctl status spyder
   sudo journalctl -u spyder -f
   ```

### Directory Structure

Create the recommended directory structure:

```bash
sudo mkdir -p /opt/spyder/{bin,config,logs,spool}
sudo chown -R spyder:spyder /opt/spyder

# Directory layout:
# /opt/spyder/
# ├── bin/spyder          # Binary
# ├── config/
# │   ├── domains.txt     # Domains to probe
# │   └── config.yaml     # Configuration file (optional)
# ├── logs/               # Log files
# └── spool/              # Failed batch storage
```

## Redis Installation (Optional)

SPYDER can use Redis for deduplication and distributed queue operations.

### Install Redis

```bash
# Ubuntu/Debian
sudo apt install redis-server

# CentOS/RHEL/Fedora
sudo dnf install redis

# macOS
brew install redis

# Docker
docker run -d --name redis -p 6379:6379 redis:7-alpine
```

### Configure Redis for SPYDER

1. **Basic Redis configuration** (`/etc/redis/redis.conf`):
   ```conf
   # Basic settings
   bind 127.0.0.1
   port 6379
   save 900 1
   save 300 10
   save 60 10000
   
   # Memory optimization
   maxmemory 2gb
   maxmemory-policy allkeys-lru
   
   # Performance
   tcp-keepalive 300
   timeout 0
   ```

2. **Start Redis**
   ```bash
   sudo systemctl enable redis
   sudo systemctl start redis
   ```

3. **Verify Redis connection**
   ```bash
   redis-cli ping
   # Should return: PONG
   ```

## Environment Setup

### Production Environment Variables

Create `/opt/spyder/.env`:

```bash
# Redis configuration
REDIS_ADDR=127.0.0.1:6379

# Distributed queue (if using multiple probes)
REDIS_QUEUE_ADDR=127.0.0.1:6379
REDIS_QUEUE_KEY=spyder:queue

# OpenTelemetry (optional)
OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4318
OTEL_EXPORTER_OTLP_INSECURE=true
```

### Firewall Configuration

Open required ports:

```bash
# Metrics endpoint (if enabled)
sudo ufw allow 9090/tcp

# For distributed setups, allow Redis access
sudo ufw allow from 10.0.0.0/8 to any port 6379
```

## Configuration Files

### Sample domains.txt

```
# High-traffic domains
google.com
facebook.com
amazon.com
microsoft.com

# CDN providers
cloudflare.com
fastly.com
akamai.com

# Social media
twitter.com
linkedin.com
instagram.com

# E-commerce
shopify.com
stripe.com
paypal.com
```

### Sample systemd service configuration

Located at `/etc/systemd/system/spyder.service`:

```ini
[Unit]
Description=SPYDER Probe
After=network-online.target redis.service
Wants=network-online.target
Requires=redis.service

[Service]
Type=simple
User=spyder
WorkingDirectory=/opt/spyder
Environment=REDIS_ADDR=127.0.0.1:6379
ExecStart=/opt/spyder/bin/spyder \
  -domains=/opt/spyder/config/domains.txt \
  -ingest=https://ingest.example.com/v1/batch \
  -metrics_addr=:9090 \
  -probe=production-1 \
  -spool_dir=/opt/spyder/spool
  
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/spyder/spool /opt/spyder/logs

[Install]
WantedBy=multi-user.target
```

## Verification

### Test Installation

1. **Basic functionality test**
   ```bash
   echo "example.com" > test-domains.txt
   ./bin/spyder -domains=test-domains.txt
   ```

2. **Verify output format**
   The output should be valid JSON with nodes and edges:
   ```json
   {
     "probe_id": "local-1",
     "run_id": "run-1704067200",
     "nodes_domain": [...],
     "nodes_ip": [...],
     "edges": [...]
   }
   ```

3. **Test metrics endpoint** (if enabled)
   ```bash
   curl http://localhost:9090/metrics
   ```

4. **Test Redis connection** (if configured)
   ```bash
   REDIS_ADDR=127.0.0.1:6379 ./bin/spyder -domains=test-domains.txt
   ```

### Performance Testing

Test with a larger domain list:

```bash
# Create larger test file
seq 1 100 | sed 's/.*/site&.example.com/' > large-test.txt

# Run performance test
time ./bin/spyder -domains=large-test.txt -concurrency=64
```

## Troubleshooting

### Common Issues

1. **Permission denied**
   ```bash
   sudo chown spyder:spyder /opt/spyder/spyder
   sudo chmod +x /opt/spyder/spyder
   ```

2. **Cannot connect to Redis**
   ```bash
   # Check Redis status
   sudo systemctl status redis
   
   # Test Redis connectivity
   redis-cli -h 127.0.0.1 -p 6379 ping
   ```

3. **DNS resolution failures**
   ```bash
   # Test DNS resolution
   nslookup example.com
   dig example.com
   ```

4. **Memory issues**
   ```bash
   # Monitor memory usage
   top -p $(pgrep spyder)
   
   # Reduce concurrency if needed
   ./bin/spyder -domains=domains.txt -concurrency=32
   ```

### Log Analysis

```bash
# View systemd logs
sudo journalctl -u spyder -f

# Check for specific errors
sudo journalctl -u spyder | grep -i error

# View metrics
curl -s http://localhost:9090/metrics | grep spyder
```

## Next Steps

- [Configuration Reference](../config/cli.md) - Complete configuration options
- [Operations Guide](../ops/single-node.md) - Production deployment
- [Monitoring Setup](../ops/monitoring.md) - Monitoring and alerting
- [Troubleshooting](../ops/troubleshooting.md) - Common issues and solutions