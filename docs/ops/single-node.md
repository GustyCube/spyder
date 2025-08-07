# Single Node Deployment Guide

This guide covers deploying SPYDER on a single server for production use, including installation, configuration, and monitoring.

## Prerequisites

### System Requirements

**Minimum Production Requirements:**
- **OS**: Linux (Ubuntu 20.04+, CentOS 8+, or RHEL 8+)
- **CPU**: 4 cores
- **Memory**: 8GB RAM
- **Storage**: 50GB free space
- **Network**: Stable internet connection

**Recommended Configuration:**
- **CPU**: 8+ cores
- **Memory**: 16GB+ RAM
- **Storage**: 100GB+ SSD
- **Network**: 1Gbps+ connection

### Dependencies

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install -y curl wget unzip systemctl

# CentOS/RHEL/Fedora
sudo dnf install -y curl wget unzip systemd

# Optional: Redis for deduplication
sudo apt install redis-server  # Ubuntu
sudo dnf install redis         # CentOS/Fedora
```

## Installation

### Method 1: Binary Installation

```bash
# Create spyder user
sudo useradd -r -s /bin/false -d /opt/spyder spyder

# Create directories
sudo mkdir -p /opt/spyder/{bin,config,logs,spool}
sudo chown -R spyder:spyder /opt/spyder

# Download and install binary
wget https://github.com/gustycube/spyder-probe/releases/latest/download/spyder-linux-amd64
sudo mv spyder-linux-amd64 /opt/spyder/bin/spyder
sudo chmod +x /opt/spyder/bin/spyder
sudo chown spyder:spyder /opt/spyder/bin/spyder
```

### Method 2: Build from Source

```bash
# Install Go
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Build SPYDER
git clone https://github.com/gustycube/spyder-probe.git
cd spyder-probe
go build -o /opt/spyder/bin/spyder ./cmd/spyder
sudo chown spyder:spyder /opt/spyder/bin/spyder
```

## Configuration

### Basic Configuration

**Create domains file:**
```bash
sudo tee /opt/spyder/config/domains.txt > /dev/null << EOF
# High-value targets
google.com
amazon.com
microsoft.com
facebook.com
apple.com

# CDN providers
cloudflare.com
fastly.com
akamai.com

# Security vendors
crowdstrike.com
okta.com
duo.com
EOF

sudo chown spyder:spyder /opt/spyder/config/domains.txt
```

**Environment configuration:**
```bash
sudo tee /opt/spyder/config/.env > /dev/null << EOF
# Basic configuration
SPYDER_ENVIRONMENT=production
SPYDER_REGION=us-east-1

# Redis configuration (if using Redis)
REDIS_ADDR=127.0.0.1:6379

# OpenTelemetry (optional)
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
OTEL_EXPORTER_OTLP_INSECURE=true
EOF

sudo chown spyder:spyder /opt/spyder/config/.env
sudo chmod 600 /opt/spyder/config/.env
```

### systemd Service Configuration

**Create service file:**
```bash
sudo tee /etc/systemd/system/spyder.service > /dev/null << 'EOF'
[Unit]
Description=SPYDER Probe - Internet Infrastructure Mapping
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=spyder
Group=spyder
WorkingDirectory=/opt/spyder

# Environment
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
EnvironmentFile=/opt/spyder/config/.env

# Command
ExecStart=/opt/spyder/bin/spyder \
  -domains=/opt/spyder/config/domains.txt \
  -probe=prod-single-$(hostname) \
  -run=prod-$(date +%Y%m%d) \
  -concurrency=256 \
  -metrics_addr=127.0.0.1:9090 \
  -batch_max_edges=10000 \
  -batch_flush_sec=2 \
  -spool_dir=/opt/spyder/spool \
  -ua="CompanySpyder/1.0 (+https://company.com/security)"

# Process management
Restart=on-failure
RestartSec=10
KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=30

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=spyder

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/spyder/logs /opt/spyder/spool
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true

[Install]
WantedBy=multi-user.target
EOF
```

**Enable and start service:**
```bash
sudo systemctl daemon-reload
sudo systemctl enable spyder
sudo systemctl start spyder
```

## Production Configuration

### High-Throughput Setup

**For high-volume scanning:**
```bash
sudo tee /opt/spyder/config/production.env > /dev/null << EOF
# High-performance configuration
REDIS_ADDR=127.0.0.1:6379

# Optimized settings
GOMAXPROCS=8
GOGC=100
EOF

# Update systemd service for high throughput
sudo tee /etc/systemd/system/spyder.service > /dev/null << 'EOF'
[Unit]
Description=SPYDER Probe - High Throughput
After=network-online.target redis.service
Wants=network-online.target
Requires=redis.service

[Service]
Type=simple
User=spyder
Group=spyder
WorkingDirectory=/opt/spyder
EnvironmentFile=/opt/spyder/config/production.env

ExecStart=/opt/spyder/bin/spyder \
  -domains=/opt/spyder/config/domains.txt \
  -probe=prod-ht-$(hostname) \
  -concurrency=512 \
  -batch_max_edges=25000 \
  -batch_flush_sec=1 \
  -metrics_addr=127.0.0.1:9090 \
  -spool_dir=/opt/spyder/spool

Restart=on-failure
RestartSec=5

# Resource limits
LimitNOFILE=65536
LimitNPROC=8192

[Install]
WantedBy=multi-user.target
EOF
```

### Redis Configuration

**Install and configure Redis:**
```bash
# Install Redis
sudo apt install redis-server

# Configure Redis for SPYDER
sudo tee -a /etc/redis/redis.conf > /dev/null << EOF

# SPYDER-specific configuration
maxmemory 4gb
maxmemory-policy allkeys-lru
save 300 1000
appendonly yes
appendfsync everysec
EOF

# Start Redis
sudo systemctl enable redis-server
sudo systemctl start redis-server
```

### Log Rotation

**Configure logrotate:**
```bash
sudo tee /etc/logrotate.d/spyder > /dev/null << EOF
/var/log/spyder/*.log {
    daily
    missingok
    rotate 30
    compress
    delaycompress
    notifempty
    create 644 spyder spyder
    postrotate
        /bin/systemctl reload spyder
    endscript
}
EOF
```

## Monitoring Setup

### Prometheus Configuration

**Install Prometheus:**
```bash
# Download Prometheus
wget https://github.com/prometheus/prometheus/releases/download/v2.40.0/prometheus-2.40.0.linux-amd64.tar.gz
tar xzf prometheus-2.40.0.linux-amd64.tar.gz
sudo mv prometheus-2.40.0.linux-amd64 /opt/prometheus
sudo chown -R prometheus:prometheus /opt/prometheus

# Create configuration
sudo tee /opt/prometheus/prometheus.yml > /dev/null << EOF
global:
  scrape_interval: 30s

scrape_configs:
  - job_name: 'spyder'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 15s
    
  - job_name: 'node-exporter'
    static_configs:
      - targets: ['localhost:9100']

rule_files:
  - "spyder.rules.yml"

alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - localhost:9093
EOF

# Create systemd service for Prometheus
sudo tee /etc/systemd/system/prometheus.service > /dev/null << EOF
[Unit]
Description=Prometheus
After=network.target

[Service]
Type=simple
User=prometheus
ExecStart=/opt/prometheus/prometheus \
  --config.file=/opt/prometheus/prometheus.yml \
  --storage.tsdb.path=/opt/prometheus/data \
  --web.console.templates=/opt/prometheus/consoles \
  --web.console.libraries=/opt/prometheus/console_libraries \
  --web.listen-address=0.0.0.0:9091

Restart=always

[Install]
WantedBy=multi-user.target
EOF
```

### Alerting Rules

**Create alerting rules:**
```bash
sudo tee /opt/prometheus/spyder.rules.yml > /dev/null << EOF
groups:
- name: spyder
  rules:
  - alert: SpyderDown
    expr: up{job="spyder"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "SPYDER probe is down"

  - alert: SpyderLowThroughput
    expr: rate(spyder_tasks_total[5m]) < 50
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "SPYDER throughput is low"
      description: "Processing {{ $value }} domains/sec"

  - alert: SpyderHighErrorRate
    expr: rate(spyder_tasks_total{status="error"}[5m]) / rate(spyder_tasks_total[5m]) > 0.1
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "SPYDER error rate is high"
      description: "{{ $value | humanizePercentage }} error rate"

  - alert: SpyderHighMemory
    expr: go_memstats_heap_alloc_bytes / 1024 / 1024 > 2048
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "SPYDER memory usage high"
      description: "Using {{ $value }}MB memory"
EOF
```

### Grafana Dashboard

**Install Grafana:**
```bash
sudo apt-get install -y software-properties-common
sudo add-apt-repository "deb https://packages.grafana.com/oss/deb stable main"
wget -q -O - https://packages.grafana.com/gpg.key | sudo apt-key add -
sudo apt-get update
sudo apt-get install grafana

sudo systemctl enable grafana-server
sudo systemctl start grafana-server
```

## Security Hardening

### Firewall Configuration

```bash
# Configure UFW
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Allow SSH
sudo ufw allow ssh

# Allow metrics (localhost only)
sudo ufw allow from 127.0.0.1 to any port 9090
sudo ufw allow from 127.0.0.1 to any port 9091

# Allow Grafana (if external access needed)
sudo ufw allow 3000

sudo ufw enable
```

### File Permissions

```bash
# Secure configuration files
sudo chmod 750 /opt/spyder
sudo chmod 700 /opt/spyder/config
sudo chmod 600 /opt/spyder/config/.env
sudo chmod 644 /opt/spyder/config/domains.txt

# Secure logs and spool
sudo chmod 755 /opt/spyder/logs
sudo chmod 755 /opt/spyder/spool
```

### SELinux Configuration (if applicable)

```bash
# Allow SPYDER network access
sudo setsebool -P httpd_can_network_connect 1

# Create custom policy if needed
sudo audit2allow -a -M spyder-policy
sudo semodule -i spyder-policy.pp
```

## Operational Procedures

### Startup Procedure

```bash
# Start dependencies
sudo systemctl start redis-server

# Start SPYDER
sudo systemctl start spyder

# Verify startup
sudo systemctl status spyder
sudo journalctl -u spyder -f

# Check metrics
curl http://127.0.0.1:9090/metrics | grep spyder
```

### Shutdown Procedure

```bash
# Graceful shutdown
sudo systemctl stop spyder

# Wait for processing to complete
sleep 30

# Check for spool files
ls -la /opt/spyder/spool/

# Stop dependencies if needed
sudo systemctl stop redis-server
```

### Health Checks

**Create health check script:**
```bash
sudo tee /opt/spyder/bin/health-check.sh > /dev/null << 'EOF'
#!/bin/bash

set -e

echo "=== SPYDER Health Check ==="

# Check service status
echo "Service Status:"
systemctl is-active spyder

# Check metrics endpoint
echo "Metrics Endpoint:"
curl -s http://127.0.0.1:9090/metrics | head -5

# Check processing rate
echo "Processing Rate:"
RATE=$(curl -s http://127.0.0.1:9090/metrics | grep 'spyder_tasks_total' | head -1)
echo "$RATE"

# Check memory usage
echo "Memory Usage:"
free -h

# Check disk space
echo "Disk Usage:"
df -h /opt/spyder

# Check Redis (if configured)
if [[ -n "$REDIS_ADDR" ]]; then
    echo "Redis Status:"
    redis-cli ping
fi

echo "=== Health Check Complete ==="
EOF

sudo chmod +x /opt/spyder/bin/health-check.sh
```

### Backup Procedures

```bash
# Backup configuration
sudo tar -czf /backup/spyder-config-$(date +%Y%m%d).tar.gz \
    /opt/spyder/config/ \
    /etc/systemd/system/spyder.service

# Backup spool files
sudo tar -czf /backup/spyder-spool-$(date +%Y%m%d).tar.gz \
    /opt/spyder/spool/

# Backup Redis data (if applicable)
redis-cli SAVE
sudo cp /var/lib/redis/dump.rdb /backup/redis-$(date +%Y%m%d).rdb
```

### Maintenance Tasks

**Weekly maintenance script:**
```bash
sudo tee /opt/spyder/bin/weekly-maintenance.sh > /dev/null << 'EOF'
#!/bin/bash

echo "Starting weekly maintenance..."

# Rotate logs
sudo logrotate -f /etc/logrotate.d/spyder

# Clean old spool files
find /opt/spyder/spool -name "*.json" -mtime +7 -delete

# Update domain list (if automated)
# curl -o /tmp/new-domains.txt https://internal.company.com/domains.txt
# sudo mv /tmp/new-domains.txt /opt/spyder/config/domains.txt

# Restart SPYDER for fresh start
sudo systemctl restart spyder

# Wait for startup
sleep 30

# Run health check
/opt/spyder/bin/health-check.sh

echo "Weekly maintenance complete"
EOF

sudo chmod +x /opt/spyder/bin/weekly-maintenance.sh

# Add to crontab
echo "0 2 * * 0 /opt/spyder/bin/weekly-maintenance.sh" | sudo crontab -u root -
```

This single-node deployment provides a robust, monitored SPYDER installation suitable for production use with comprehensive operational procedures.