# Troubleshooting Guide

This guide covers common issues with SPYDER Probe and their solutions.

## Common Issues

### 1. Installation and Setup Problems

#### Go Version Issues

**Problem**: Build fails with Go version errors
```
go: module requires Go 1.22 or later
```

**Solution**:
```bash
# Check current Go version
go version

# Update Go (Linux)
sudo rm -rf /usr/local/go
wget https://go.dev/dl/go1.22.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.linux-amd64.tar.gz

# Update PATH
export PATH=$PATH:/usr/local/go/bin
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
```

#### Permission Denied Errors

**Problem**: Binary won't execute
```
bash: ./bin/spyder: Permission denied
```

**Solution**:
```bash
chmod +x ./bin/spyder

# For systemd service
sudo chown spyder:spyder /opt/spyder/spyder  
sudo chmod +x /opt/spyder/spyder
```

### 2. Runtime Configuration Issues

#### Domain File Not Found

**Problem**: 
```
FATAL: open domains: open domains.txt: no such file or directory
```

**Solution**:
```bash
# Check file exists and is readable
ls -la domains.txt

# Verify file path is correct
./bin/spyder -domains=./configs/domains.txt

# For systemd service, use absolute paths
ExecStart=/opt/spyder/spyder -domains=/opt/spyder/config/domains.txt
```

#### Invalid Domain Format

**Problem**: Domains are being skipped or causing errors

**Solution**: Ensure domains.txt format is correct:
```bash
# Good format (one domain per line)
example.com
google.com
github.com

# Bad format (avoid these)
https://example.com     # No protocols
example.com/path        # No paths  
example.com:8080        # No ports
```

### 3. Redis Connection Problems

#### Redis Connection Refused

**Problem**:
```
FATAL: redis init: dial tcp 127.0.0.1:6379: connect: connection refused
```

**Diagnosis**:
```bash
# Check Redis status
sudo systemctl status redis
redis-cli ping

# Check Redis is listening
sudo netstat -tlnp | grep 6379
```

**Solutions**:
```bash
# Start Redis
sudo systemctl start redis
sudo systemctl enable redis

# Check Redis configuration
sudo nano /etc/redis/redis.conf
# Ensure: bind 127.0.0.1
# Ensure: port 6379

# Test connection manually
redis-cli -h 127.0.0.1 -p 6379 ping
```

#### Redis Authentication Issues

**Problem**: Redis requires authentication but none provided

**Solution**:
```bash
# Option 1: Disable Redis auth (development only)
# Comment out 'requirepass' in redis.conf

# Option 2: Use Redis URL with auth
export REDIS_ADDR=redis://username:password@localhost:6379

# Option 3: Configure Redis without auth for SPYDER
sudo nano /etc/redis/redis.conf
# Comment out: # requirepass foobared
sudo systemctl restart redis
```

### 4. Network and DNS Issues

#### DNS Resolution Failures

**Problem**: High rate of DNS resolution errors

**Diagnosis**:
```bash
# Test DNS manually
nslookup google.com
dig google.com

# Check DNS servers
cat /etc/resolv.conf

# Test with different DNS servers
dig @8.8.8.8 google.com
dig @1.1.1.1 google.com
```

**Solutions**:
```bash
# Use reliable DNS servers
sudo nano /etc/systemd/resolved.conf
# Add: DNS=8.8.8.8 1.1.1.1
sudo systemctl restart systemd-resolved

# For specific SPYDER issues, reduce concurrency
./bin/spyder -domains=domains.txt -concurrency=32
```

#### HTTP Connection Timeouts

**Problem**: Many HTTP requests timing out

**Diagnosis**:
```bash
# Test HTTP connectivity manually
curl -v -m 10 https://example.com

# Check network latency  
ping example.com
traceroute example.com
```

**Solutions**:
```bash
# Increase timeout (future feature)
# Currently: adjust concurrency to reduce load
./bin/spyder -domains=domains.txt -concurrency=64

# Check firewall rules
sudo ufw status
sudo iptables -L

# For corporate networks, check proxy settings
export HTTP_PROXY=http://proxy.company.com:8080
export HTTPS_PROXY=http://proxy.company.com:8080
```

### 5. Performance Issues

#### High Memory Usage

**Problem**: SPYDER consuming excessive memory

**Diagnosis**:
```bash
# Monitor memory usage
top -p $(pgrep spyder)
ps aux | grep spyder

# Check for memory leaks
valgrind --tool=memcheck --leak-check=full ./bin/spyder -domains=small-test.txt
```

**Solutions**:
```bash
# Reduce worker concurrency
./bin/spyder -domains=domains.txt -concurrency=128

# Enable Redis deduplication to reduce memory
REDIS_ADDR=127.0.0.1:6379 ./bin/spyder -domains=domains.txt

# Split large domain files
split -l 1000 large-domains.txt batch-
for file in batch-*; do
    ./bin/spyder -domains="$file"
done
```

#### Slow Processing Speed

**Problem**: Processing speed is much slower than expected

**Diagnosis**:
```bash
# Check metrics endpoint
curl http://localhost:9090/metrics | grep spyder_tasks_total

# Monitor system resources
htop
iostat -x 1
```

**Solutions**:
```bash
# Optimize concurrency for your system
# Start with: cores * 64
./bin/spyder -domains=domains.txt -concurrency=256

# Reduce batch flush time for faster processing
./bin/spyder -domains=domains.txt -batch_flush_sec=1

# Use SSD storage for spool directory
./bin/spyder -domains=domains.txt -spool_dir=/fast/ssd/spool
```

### 6. Data and Output Issues

#### Invalid JSON Output

**Problem**: Output is not valid JSON

**Diagnosis**:
```bash
# Test output format
./bin/spyder -domains=test.txt | jq .

# Check for mixed stdout content
./bin/spyder -domains=test.txt > output.json 2>&1
cat output.json
```

**Solutions**:
```bash
# Redirect logs to stderr only
./bin/spyder -domains=test.txt 2>/dev/null | jq .

# Use ingest endpoint instead of stdout
./bin/spyder -domains=test.txt -ingest=http://localhost:8080/v1/batch
```

#### Batch Emission Failures

**Problem**: Batches failing to emit to ingest endpoint

**Diagnosis**:
```bash
# Check ingest endpoint availability
curl -v http://your-ingest-endpoint/v1/batch

# Check spool directory for failed batches
ls -la /opt/spyder/spool/

# Check logs for emission errors
sudo journalctl -u spyder | grep -i emit
```

**Solutions**:
```bash
# Test with a simple HTTP server
python3 -m http.server 8080 &
./bin/spyder -domains=test.txt -ingest=http://localhost:8080/v1/batch

# Check mTLS configuration if using client certificates
openssl x509 -in client.pem -text -noout
openssl s_client -connect ingest-endpoint:443 -cert client.pem -key client.key
```

### 7. Monitoring and Metrics Issues

#### Metrics Endpoint Not Working

**Problem**: Cannot access metrics at :9090/metrics

**Diagnosis**:
```bash
# Check if metrics server is running
netstat -tlnp | grep 9090
curl -v http://localhost:9090/metrics
```

**Solutions**:
```bash
# Ensure metrics are enabled
./bin/spyder -domains=domains.txt -metrics_addr=:9090

# Check firewall rules
sudo ufw allow 9090/tcp

# For remote access, bind to all interfaces
./bin/spyder -domains=domains.txt -metrics_addr=0.0.0.0:9090
```

#### Prometheus Not Scraping

**Problem**: Prometheus not collecting SPYDER metrics

**Diagnosis**:
```bash
# Check Prometheus targets
curl http://prometheus:9090/api/v1/targets

# Verify Prometheus configuration
sudo nano /etc/prometheus/prometheus.yml
```

**Solutions**:
```yaml
# Fix Prometheus scrape config
scrape_configs:
  - job_name: 'spyder-probe'
    static_configs:
      - targets: ['spyder-host:9090']  # Use correct hostname
    scrape_interval: 15s
    metrics_path: '/metrics'
    scheme: 'http'
```

### 8. systemd Service Issues  

#### Service Won't Start

**Problem**: systemd service fails to start

**Diagnosis**:
```bash
# Check service status
sudo systemctl status spyder

# View detailed logs
sudo journalctl -u spyder -n 50

# Test binary manually
sudo -u spyder /opt/spyder/spyder -domains=/opt/spyder/config/domains.txt
```

**Common Solutions**:
```bash
# Fix permissions
sudo chown -R spyder:spyder /opt/spyder
sudo chmod +x /opt/spyder/spyder

# Fix service file paths
sudo nano /etc/systemd/system/spyder.service
# Use absolute paths for all files

# Reload systemd after changes
sudo systemctl daemon-reload
sudo systemctl restart spyder
```

#### Service Crashes

**Problem**: Service starts but crashes immediately

**Diagnosis**:
```bash
# Check exit status
sudo systemctl show spyder --property=ExecMainStatus

# Enable core dumps
sudo systemctl edit spyder
# Add:
# [Service]
# LimitCORE=infinity

# Analyze core dump
gdb /opt/spyder/spyder core.*
```

## Debugging Commands

### Log Analysis

```bash
# Real-time log monitoring
sudo journalctl -u spyder -f

# Search for specific errors
sudo journalctl -u spyder | grep -i "error\|fatal\|panic"

# Export logs for analysis
sudo journalctl -u spyder --since="1 hour ago" --no-pager > spyder-debug.log
```

### System Resource Monitoring

```bash
# Monitor SPYDER process
watch -n 1 'ps aux | grep spyder'

# Network connections
sudo netstat -tupln | grep spyder
sudo ss -tulpn | grep spyder

# File descriptors
sudo ls -la /proc/$(pgrep spyder)/fd/ | wc -l
```

### Redis Debugging

```bash
# Monitor Redis commands
redis-cli monitor

# Check Redis memory usage
redis-cli info memory

# List SPYDER keys in Redis
redis-cli keys "spyder*"

# Clear SPYDER data from Redis
redis-cli del $(redis-cli keys "spyder*")
```

## Performance Tuning

### Optimal Concurrency Settings

```bash
# Start with conservative settings
./bin/spyder -domains=domains.txt -concurrency=64

# Monitor system load and adjust
while true; do
    uptime
    ps aux | grep spyder | grep -v grep
    sleep 5
done

# Gradually increase until performance plateaus
for conc in 64 128 256 512; do
    echo "Testing concurrency: $conc"
    time ./bin/spyder -domains=test-domains.txt -concurrency=$conc
done
```

### System Tuning

```bash
# Increase file descriptor limits
echo 'spyder soft nofile 65536' | sudo tee -a /etc/security/limits.conf  
echo 'spyder hard nofile 65536' | sudo tee -a /etc/security/limits.conf

# Optimize network settings
echo 'net.core.rmem_max = 134217728' | sudo tee -a /etc/sysctl.conf
echo 'net.core.wmem_max = 134217728' | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

## Getting Help

### Collecting Debug Information

Create a debug information script:

```bash
#!/bin/bash
# spyder-debug.sh

echo "=== SPYDER Debug Information ==="
echo "Date: $(date)"
echo "Hostname: $(hostname)"
echo

echo "=== System Information ==="
uname -a
cat /etc/os-release
echo

echo "=== SPYDER Binary Info ==="
./bin/spyder -h 2>&1 | head -20
echo

echo "=== Process Information ==="
ps aux | grep spyder
echo

echo "=== Network Connections ==="
sudo netstat -tupln | grep spyder
echo

echo "=== Recent Logs ==="  
sudo journalctl -u spyder --since="10 minutes ago" --no-pager
echo

echo "=== Redis Status ==="
redis-cli ping 2>&1
redis-cli info server 2>&1 | head -10
echo

echo "=== System Resources ==="
free -h
df -h /opt/spyder 2>/dev/null
uptime
```

### Community Support

- **GitHub Issues**: https://github.com/gustycube/spyder-probe/issues
- **Documentation**: Check the docs directory for detailed guides
- **Metrics**: Use the metrics endpoint to diagnose performance issues

### Enterprise Support

For production deployments, consider:
- Setting up comprehensive monitoring
- Implementing proper backup procedures  
- Configuring high availability setups
- Regular performance benchmarking