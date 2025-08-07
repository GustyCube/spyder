# Security Configuration

SPYDER includes multiple security features for safe operation in production environments. This guide covers mTLS configuration, secure deployment practices, and security considerations.

## mTLS Configuration

### Certificate Setup

**Generate Client Certificates:**

```bash
# Create CA private key
openssl genrsa -out ca.key 4096

# Create CA certificate
openssl req -new -x509 -key ca.key -sha256 -subj "/C=US/ST=CA/O=MyOrg/CN=MyCA" -days 3650 -out ca.crt

# Create client private key
openssl genrsa -out client.key 4096

# Create client certificate signing request
openssl req -new -key client.key -out client.csr -subj "/C=US/ST=CA/O=MyOrg/CN=spyder-client"

# Sign client certificate
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out client.crt -days 365 -sha256
```

**SPYDER mTLS Configuration:**

```bash
./bin/spyder \
  -domains=domains.txt \
  -ingest=https://secure-ingest.example.com/v1/batch \
  -mtls_cert=/etc/spyder/client.crt \
  -mtls_key=/etc/spyder/client.key \
  -mtls_ca=/etc/spyder/ca.crt
```

### Certificate Management

**File Permissions:**

```bash
# Secure certificate storage
sudo mkdir -p /etc/spyder/certs
sudo chmod 750 /etc/spyder/certs
sudo chown spyder:spyder /etc/spyder/certs

# Set certificate permissions
sudo chmod 644 /etc/spyder/certs/client.crt
sudo chmod 644 /etc/spyder/certs/ca.crt
sudo chmod 600 /etc/spyder/certs/client.key
sudo chown spyder:spyder /etc/spyder/certs/*
```

**Certificate Rotation:**

```bash
#!/bin/bash
# rotate-certs.sh

CERT_DIR="/etc/spyder/certs"
BACKUP_DIR="/etc/spyder/certs/backup"

# Backup existing certificates
mkdir -p "$BACKUP_DIR"
cp "$CERT_DIR"/{client.crt,client.key} "$BACKUP_DIR/"

# Deploy new certificates
cp /tmp/new-client.crt "$CERT_DIR/client.crt"
cp /tmp/new-client.key "$CERT_DIR/client.key"

# Set permissions
chown spyder:spyder "$CERT_DIR"/{client.crt,client.key}
chmod 644 "$CERT_DIR/client.crt"
chmod 600 "$CERT_DIR/client.key"

# Restart SPYDER
sudo systemctl restart spyder
```

## Network Security

### Firewall Configuration

**iptables Rules:**

```bash
# Allow outbound HTTPS (443)
iptables -A OUTPUT -p tcp --dport 443 -j ACCEPT

# Allow outbound DNS (53)
iptables -A OUTPUT -p udp --dport 53 -j ACCEPT
iptables -A OUTPUT -p tcp --dport 53 -j ACCEPT

# Allow metrics endpoint (local only)
iptables -A INPUT -p tcp -s 127.0.0.1 --dport 9090 -j ACCEPT
iptables -A INPUT -p tcp --dport 9090 -j REJECT

# Allow Redis access (internal network only)
iptables -A INPUT -p tcp -s 10.0.0.0/8 --dport 6379 -j ACCEPT
iptables -A INPUT -p tcp --dport 6379 -j REJECT
```

**ufw Configuration:**

```bash
# Basic rules
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Allow SSH
sudo ufw allow ssh

# Allow metrics (localhost only)
sudo ufw allow from 127.0.0.1 to any port 9090

# Allow Redis (internal network)
sudo ufw allow from 10.0.0.0/8 to any port 6379

# Enable firewall
sudo ufw enable
```

### Network Isolation

**Container Network Security:**

```yaml
# docker-compose.yml
version: '3.8'
services:
  spyder:
    image: spyder-probe:latest
    networks:
      - spyder-internal
    # No published ports for security
    
  redis:
    image: redis:7-alpine
    networks:
      - spyder-internal
    # Internal network only

networks:
  spyder-internal:
    driver: bridge
    internal: true  # No external connectivity
```

**Kubernetes Network Policies:**

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: spyder-network-policy
spec:
  podSelector:
    matchLabels:
      app: spyder-probe
  policyTypes:
  - Ingress
  - Egress
  egress:
  - to: []  # Allow all outbound (for DNS/HTTP)
    ports:
    - protocol: TCP
      port: 443
    - protocol: UDP
      port: 53
  - to:
    - podSelector:
        matchLabels:
          app: redis
    ports:
    - protocol: TCP
      port: 6379
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: monitoring
    ports:
    - protocol: TCP
      port: 9090
```

## Application Security

### Secure Defaults

**Runtime Security:**

```bash
# Run as non-root user
sudo useradd -r -s /bin/false -d /opt/spyder spyder

# Secure systemd service
[Unit]
Description=SPYDER Probe
After=network.target

[Service]
Type=simple
User=spyder
Group=spyder
WorkingDirectory=/opt/spyder

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/spyder/spool
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true

ExecStart=/opt/spyder/bin/spyder -domains=/opt/spyder/domains.txt
```

**Container Security:**

```dockerfile
FROM golang:1.22 AS build
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o /spyder ./cmd/spyder

FROM gcr.io/distroless/base-debian12
USER nonroot:nonroot
COPY --from=build /spyder /usr/local/bin/spyder

# Security labels
LABEL security.capabilities="NET_BIND_SERVICE"
LABEL security.user="nonroot"
LABEL security.no-new-privileges="true"

ENTRYPOINT ["/usr/local/bin/spyder"]
```

### Input Validation

**Domain Validation:**

```go
// Secure domain parsing
func validateDomain(domain string) error {
    // Length check
    if len(domain) > 253 {
        return errors.New("domain too long")
    }
    
    // Character validation
    for _, r := range domain {
        if !unicode.IsLetter(r) && !unicode.IsDigit(r) && 
           r != '.' && r != '-' {
            return errors.New("invalid character in domain")
        }
    }
    
    // Basic format check
    if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
        return errors.New("invalid domain format")
    }
    
    return nil
}
```

**URL Validation:**

```go
// Secure URL parsing
func validateURL(rawURL string) (*url.URL, error) {
    u, err := url.Parse(rawURL)
    if err != nil {
        return nil, err
    }
    
    // Only allow HTTP/HTTPS
    if u.Scheme != "http" && u.Scheme != "https" {
        return nil, errors.New("invalid URL scheme")
    }
    
    // Validate hostname
    if err := validateDomain(u.Hostname()); err != nil {
        return nil, err
    }
    
    return u, nil
}
```

## Data Protection

### Sensitive Data Handling

**Avoid Logging Sensitive Information:**

```go
// ❌ Don't log full URLs or sensitive data
log.Info("processing", "url", fullURL)

// ✅ Log only necessary information
log.Info("processing", "host", u.Hostname(), "scheme", u.Scheme)
```

**Configuration Security:**

```bash
# ❌ Avoid environment variables for secrets
export INGEST_API_KEY=secret123

# ✅ Use files or secret managers
export INGEST_API_KEY_FILE=/run/secrets/api_key

# ✅ Use proper file permissions
chmod 600 /etc/spyder/secrets/*
chown spyder:spyder /etc/spyder/secrets/*
```

### Data Minimization

**Limit Data Collection:**

```go
// Only collect necessary HTTP content
body := io.LimitReader(resp.Body, 512*1024)

// Limit certificate data
cert := &emit.NodeCert{
    SPKI:      spkiHash,
    SubjectCN: leaf.Subject.CommonName,
    IssuerCN:  leaf.Issuer.CommonName,
    // Don't include full certificate
}
```

**Data Retention:**

```bash
# Automatic log rotation
/var/log/spyder/*.log {
    daily
    missingok
    rotate 30
    compress
    delaycompress
    notifempty
    postrotate
        /bin/systemctl reload spyder
    endscript
}
```

## Monitoring Security

### Secure Metrics Endpoint

**Local-only Metrics:**

```bash
# Bind to localhost only
-metrics_addr=127.0.0.1:9090

# Use authentication proxy
nginx_auth_proxy:
  location /metrics {
    auth_basic "Restricted";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://127.0.0.1:9090/metrics;
  }
```

**Metrics Filtering:**

```yaml
# Prometheus scrape config with filtering
- job_name: 'spyder'
  static_configs:
    - targets: ['spyder:9090']
  metric_relabel_configs:
    # Remove potentially sensitive labels
    - source_labels: [__name__]
      regex: 'spyder_.*_hostname.*'
      action: drop
```

### Audit Logging

**Structured Audit Logs:**

```go
// Log security-relevant events
log.Info("domain_processed", 
    "domain", domain,
    "probe_id", probeID,
    "timestamp", time.Now(),
    "robots_allowed", allowed,
)

log.Warn("robots_blocked",
    "domain", domain,
    "user_agent", userAgent,
    "timestamp", time.Now(),
)
```

## Deployment Security

### Secure Deployment Checklist

**Pre-deployment:**

- [ ] Update all dependencies to latest versions
- [ ] Scan container images for vulnerabilities
- [ ] Review firewall rules
- [ ] Validate certificate configuration
- [ ] Test mTLS connectivity
- [ ] Verify secure file permissions

**Post-deployment:**

- [ ] Monitor for failed authentication attempts
- [ ] Validate metrics access controls
- [ ] Check log rotation configuration
- [ ] Verify network policy enforcement
- [ ] Test backup and recovery procedures

### Vulnerability Management

**Dependency Scanning:**

```bash
# Go vulnerability check
go list -json -m all | nancy sleuth

# Container scanning
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$HOME/.cache":/tmp/.cache \
  aquasec/trivy image spyder-probe:latest
```

**Regular Security Updates:**

```bash
#!/bin/bash
# security-update.sh

# Update Go dependencies
go get -u all
go mod tidy

# Update base container image
docker pull gcr.io/distroless/base-debian12

# Rebuild with security patches
make docker

# Run security scan
docker run --rm aquasec/trivy image spyder-probe:latest
```

## Incident Response

### Security Event Detection

**Monitor for:**

- Failed mTLS handshakes
- Unusual traffic patterns
- High error rates
- Unexpected certificate changes
- Metrics endpoint access attempts

**Alerting Rules:**

```yaml
# Prometheus alerting rules
groups:
- name: spyder-security
  rules:
  - alert: SpyderTLSErrors
    expr: increase(spyder_tls_errors_total[5m]) > 10
    labels:
      severity: warning
    annotations:
      summary: "High TLS error rate in SPYDER"
      
  - alert: SpyderRobotBlocks  
    expr: increase(spyder_robots_blocked_total[1h]) > 100
    labels:
      severity: info
    annotations:
      summary: "Many robots.txt blocks"
```

### Response Procedures

**Security Incident Response:**

1. **Isolate**: Stop affected SPYDER instances
2. **Assess**: Review logs and metrics
3. **Contain**: Block malicious sources if identified
4. **Recover**: Update configuration and restart
5. **Learn**: Update security measures based on incident

**Emergency Procedures:**

```bash
# Emergency shutdown
sudo systemctl stop spyder

# Block suspicious IPs
sudo ufw deny from 192.168.1.100

# Rotate certificates immediately
./scripts/rotate-certs.sh

# Clear Redis deduplication cache
redis-cli FLUSHDB
```

This security configuration ensures SPYDER operates safely in production environments while maintaining the flexibility needed for effective internet infrastructure mapping.