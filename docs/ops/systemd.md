# systemd Service Configuration

This guide covers running SPYDER as a managed systemd service on Linux, with security hardening, journal logging, auto-restart, and timer units for scheduled scans.

## Service File Creation

### Basic Service Unit

Create the service file at `/etc/systemd/system/spyder.service`:

```bash
sudo tee /etc/systemd/system/spyder.service > /dev/null << 'EOF'
[Unit]
Description=SPYDER Probe - Internet Infrastructure Mapping
Documentation=https://github.com/gustycube/spyder
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=spyder
Group=spyder
WorkingDirectory=/opt/spyder

# Environment
EnvironmentFile=/opt/spyder/config/spyder.env

# Main process
ExecStart=/opt/spyder/bin/spyder \
  -config=/opt/spyder/config/config.yaml \
  -probe=prod-%H \
  -run=run-%i

# Graceful shutdown
KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=60

# Restart policy
Restart=on-failure
RestartSec=10

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=spyder

[Install]
WantedBy=multi-user.target
EOF
```

The `%H` specifier expands to the hostname, giving each machine a unique probe ID automatically.

### Create the System User

```bash
# Create a dedicated user with no login shell
sudo useradd -r -s /usr/sbin/nologin -d /opt/spyder -m spyder

# Create required directories
sudo mkdir -p /opt/spyder/{bin,config,spool,logs}
sudo chown -R spyder:spyder /opt/spyder
```

### Install the Binary

```bash
# From a release binary
sudo cp spyder /opt/spyder/bin/spyder
sudo chmod 755 /opt/spyder/bin/spyder
sudo chown spyder:spyder /opt/spyder/bin/spyder

# Or build from source
go build -o /tmp/spyder ./cmd/spyder
sudo mv /tmp/spyder /opt/spyder/bin/spyder
sudo chown spyder:spyder /opt/spyder/bin/spyder
```

## Environment File Setup

The environment file holds configuration that varies between deployments. Keep it separate from the service file so you can update settings without modifying the unit.

### Create the Environment File

```bash
sudo tee /opt/spyder/config/spyder.env > /dev/null << 'EOF'
# Redis configuration
REDIS_ADDR=127.0.0.1:6379

# Distributed mode (uncomment for multi-instance deployments)
# REDIS_QUEUE_ADDR=10.0.1.5:6379
# REDIS_QUEUE_KEY=spyder:queue

# Go runtime tuning
GOMAXPROCS=4
GOGC=100

# OpenTelemetry (optional)
# OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
# OTEL_EXPORTER_OTLP_INSECURE=true

# Logging
LOG_LEVEL=info
EOF

sudo chown spyder:spyder /opt/spyder/config/spyder.env
sudo chmod 600 /opt/spyder/config/spyder.env
```

### Create the Config File

```bash
sudo tee /opt/spyder/config/config.yaml > /dev/null << 'EOF'
domains: /opt/spyder/config/domains.txt
concurrency: 256
metrics_addr: ":9090"
batch_max_edges: 10000
batch_flush_sec: 2
spool_dir: /opt/spyder/spool
ua: "SPYDERProbe/1.0 (+https://yourcompany.com/security)"
exclude_tlds:
  - gov
  - mil
  - int
EOF

sudo chown spyder:spyder /opt/spyder/config/config.yaml
sudo chmod 644 /opt/spyder/config/config.yaml
```

### Create the Domains File

```bash
sudo tee /opt/spyder/config/domains.txt > /dev/null << 'EOF'
# Seed domains
google.com
amazon.com
microsoft.com
cloudflare.com
fastly.com
akamai.com
EOF

sudo chown spyder:spyder /opt/spyder/config/domains.txt
```

## Security Hardening

systemd provides extensive sandboxing options to limit what the SPYDER process can do. Since SPYDER only needs network access and write access to its spool directory, the attack surface can be reduced significantly.

### Hardened Service File

```bash
sudo tee /etc/systemd/system/spyder.service > /dev/null << 'EOF'
[Unit]
Description=SPYDER Probe - Internet Infrastructure Mapping
Documentation=https://github.com/gustycube/spyder
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=spyder
Group=spyder
WorkingDirectory=/opt/spyder
EnvironmentFile=/opt/spyder/config/spyder.env

ExecStart=/opt/spyder/bin/spyder \
  -config=/opt/spyder/config/config.yaml \
  -probe=prod-%H

# ------- Graceful Shutdown -------
KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=60

# ------- Restart Policy -------
Restart=on-failure
RestartSec=10
RestartPreventExitStatus=0

# ------- Resource Limits -------
LimitNOFILE=65536
LimitNPROC=8192
MemoryMax=8G
CPUQuota=400%

# ------- Logging -------
StandardOutput=journal
StandardError=journal
SyslogIdentifier=spyder

# ------- Filesystem Hardening -------
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/spyder/spool /opt/spyder/logs
ReadOnlyPaths=/opt/spyder/config /opt/spyder/bin

# ------- Kernel Hardening -------
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectKernelLogs=true
ProtectControlGroups=true
ProtectClock=true
ProtectHostname=true

# ------- Network -------
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
IPAddressDeny=any
IPAddressAllow=0.0.0.0/0 ::/0

# ------- System Call Filtering -------
SystemCallArchitectures=native
SystemCallFilter=@system-service
SystemCallFilter=~@mount @reboot @swap @clock @debug @module @raw-io

# ------- Misc Hardening -------
PrivateDevices=true
ProtectProc=invisible
ProcSubset=pid
RestrictRealtime=true
RestrictSUIDSGID=true
RemoveIPC=true
LockPersonality=true
MemoryDenyWriteExecute=true
UMask=0077

[Install]
WantedBy=multi-user.target
EOF
```

### What Each Directive Does

**Filesystem protection:**

| Directive | Effect |
|---|---|
| `NoNewPrivileges=true` | Prevents the process and its children from gaining new privileges via `setuid`, `setgid`, or filesystem capabilities. |
| `PrivateTmp=true` | Gives the service its own private `/tmp` and `/var/tmp`. |
| `ProtectSystem=strict` | Mounts the entire filesystem read-only except paths listed in `ReadWritePaths`. |
| `ProtectHome=true` | Makes `/home`, `/root`, and `/run/user` inaccessible. |
| `ReadWritePaths=` | Whitelists specific directories for writing. SPYDER only needs `spool/` and `logs/`. |

**Kernel protection:**

| Directive | Effect |
|---|---|
| `ProtectKernelTunables=true` | Makes `/proc/sys`, `/sys`, and similar kernel tunables read-only. |
| `ProtectKernelModules=true` | Prevents loading or unloading kernel modules. |
| `ProtectKernelLogs=true` | Denies access to the kernel log ring buffer. |
| `ProtectControlGroups=true` | Makes the cgroup filesystem read-only. |

**System call filtering:**

| Directive | Effect |
|---|---|
| `SystemCallArchitectures=native` | Only allows native system calls (prevents 32-bit compat exploits on 64-bit). |
| `SystemCallFilter=@system-service` | Allows the set of system calls typically needed by system services. |
| `SystemCallFilter=~@mount ...` | Denies mount, reboot, swap, and other dangerous system call groups. |

### Verify Security Settings

Use `systemd-analyze security` to audit the service:

```bash
sudo systemd-analyze security spyder.service
```

This outputs a score from 0.0 (fully exposed) to 10.0 (fully locked down). With the hardened configuration above, the score should be above 7.0.

```bash
# Example output
  NAME                           DESCRIPTION                              EXPOSURE
✓ NoNewPrivileges=               Service process cannot gain new privi... OK
✓ PrivateTmp=                    Service has no access to other softwa... OK
✓ ProtectSystem=                 Service has limited write access to t... OK
...
→ Overall exposure level for spyder.service: 2.1 SAFE
```

## Journal Logging Integration

SPYDER logs structured JSON to stderr, which systemd captures into the journal when `StandardError=journal` is set.

### Viewing Logs

```bash
# Follow logs in real time
sudo journalctl -u spyder -f

# Show logs from the last hour
sudo journalctl -u spyder --since "1 hour ago"

# Show logs since last boot
sudo journalctl -u spyder -b

# Show only errors
sudo journalctl -u spyder -p err

# Output as JSON for parsing
sudo journalctl -u spyder -o json --since today

# Show last 100 lines
sudo journalctl -u spyder -n 100 --no-pager
```

### Filtering Structured Logs

Since SPYDER uses structured (JSON) logging via zap, you can pipe journal output through `jq` for analysis:

```bash
# Extract error messages
sudo journalctl -u spyder -o cat | jq -r 'select(.level == "error") | "\(.ts) \(.msg) \(.err)"'

# Track batch emissions
sudo journalctl -u spyder -o cat | jq -r 'select(.msg == "batch emitted") | "\(.ts) edges=\(.edges)"'

# Monitor Redis connectivity
sudo journalctl -u spyder -o cat | jq -r 'select(.msg | contains("redis"))'

# Count errors by message
sudo journalctl -u spyder -o cat --since today | jq -r 'select(.level == "error") | .msg' | sort | uniq -c | sort -rn
```

### Journal Storage Configuration

For long-running deployments, configure journal retention to prevent disk exhaustion:

```bash
sudo tee /etc/systemd/journald.conf.d/spyder.conf > /dev/null << 'EOF'
[Journal]
# Retain up to 2GB of journal data
SystemMaxUse=2G
SystemMaxFileSize=100M
MaxRetentionSec=30day
Compress=yes
EOF

sudo systemctl restart systemd-journald
```

### Forwarding to External Log Systems

Forward SPYDER logs to a syslog server or log aggregator:

```bash
# /etc/systemd/journald.conf.d/forward.conf
[Journal]
ForwardToSyslog=yes
```

Or use `journalctl` as a log shipper:

```bash
# Stream to a file for Filebeat/Fluentd pickup
sudo journalctl -u spyder -f -o json > /var/log/spyder/spyder.json &
```

## Start/Stop/Restart Procedures

### Initial Setup

```bash
# Reload systemd after creating or modifying the service file
sudo systemctl daemon-reload

# Enable the service to start on boot
sudo systemctl enable spyder

# Start the service
sudo systemctl start spyder
```

### Service Control

```bash
# Check status
sudo systemctl status spyder

# Stop the service (sends SIGTERM, waits TimeoutStopSec)
sudo systemctl stop spyder

# Restart the service
sudo systemctl restart spyder

# Reload environment file without full restart
# (Note: SPYDER does not support SIGHUP reload; this restarts the process)
sudo systemctl restart spyder

# Check if the service is running
sudo systemctl is-active spyder

# Check if the service is enabled on boot
sudo systemctl is-enabled spyder
```

### Pre-Flight Checks

Before starting the service, verify the configuration:

```bash
# Test config file syntax
/opt/spyder/bin/spyder -config=/opt/spyder/config/config.yaml -version

# Verify file permissions
ls -la /opt/spyder/config/
ls -la /opt/spyder/bin/spyder

# Check Redis connectivity (if configured)
source /opt/spyder/config/spyder.env
redis-cli -h ${REDIS_ADDR%:*} -p ${REDIS_ADDR#*:} PING

# Dry run: start manually as the spyder user to check for errors
sudo -u spyder /opt/spyder/bin/spyder \
  -config=/opt/spyder/config/config.yaml \
  -probe=test \
  -version
```

### Checking for Problems After Start

```bash
# Verify the process is running
sudo systemctl status spyder

# Check for startup errors
sudo journalctl -u spyder --since "5 minutes ago" --no-pager

# Verify metrics endpoint is responding
curl -s http://127.0.0.1:9090/live | jq .

# Verify health
curl -s http://127.0.0.1:9090/health | jq .

# Check spool directory for failed batches
ls -la /opt/spyder/spool/
```

## Auto-Restart Configuration

The service file above uses `Restart=on-failure` to automatically restart SPYDER if it exits with a non-zero status. This section explains the available restart strategies.

### Restart Directives

| Directive | Value | Effect |
|---|---|---|
| `Restart=` | `on-failure` | Restart only on non-zero exit, signal, or timeout. Does not restart on clean exit (`exit 0`). |
| `RestartSec=` | `10` | Wait 10 seconds before restarting. Prevents rapid restart loops. |
| `RestartPreventExitStatus=` | `0` | Do not restart on these exit codes. |

Other useful `Restart=` values:

| Value | When It Restarts |
|---|---|
| `no` | Never (default). |
| `always` | On any exit, including clean shutdown. Use for probes that should always be running. |
| `on-failure` | Only on failure (non-zero exit, signal, or timeout). Recommended for most deployments. |
| `on-abnormal` | On signal or timeout, but not on non-zero exit. |

### Rate Limiting Restarts

systemd limits restarts by default to prevent infinite loops. Tune these for SPYDER:

```ini
[Service]
# Allow up to 5 restarts within 300 seconds
StartLimitIntervalSec=300
StartLimitBurst=5

# After hitting the limit, wait 60 seconds before trying again
RestartSec=10
```

If SPYDER hits the burst limit, the service enters a `failed` state. To recover:

```bash
# Reset the failure counter
sudo systemctl reset-failed spyder

# Start again
sudo systemctl start spyder
```

### Watchdog Integration

For additional reliability, enable the systemd watchdog. This requires the application to send periodic keepalive signals. While SPYDER does not currently implement `sd_notify` watchdog pings, you can use a wrapper script:

```bash
sudo tee /opt/spyder/bin/spyder-watchdog.sh > /dev/null << 'SCRIPT'
#!/bin/bash
# Wrapper that pings the watchdog by checking the metrics endpoint

/opt/spyder/bin/spyder "$@" &
SPYDER_PID=$!

while kill -0 "$SPYDER_PID" 2>/dev/null; do
    if curl -sf http://127.0.0.1:9090/live > /dev/null 2>&1; then
        systemd-notify WATCHDOG=1
    fi
    sleep 15
done

wait "$SPYDER_PID"
SCRIPT
sudo chmod +x /opt/spyder/bin/spyder-watchdog.sh
```

Then update the service file:

```ini
[Service]
Type=notify
NotifyAccess=all
WatchdogSec=60
ExecStart=/opt/spyder/bin/spyder-watchdog.sh \
  -config=/opt/spyder/config/config.yaml \
  -probe=prod-%H
```

## Timer Units for Scheduled Scans

Instead of running SPYDER continuously, you can use systemd timers to execute scans on a schedule. This is useful for periodic reconnaissance or compliance scans.

### Create a Oneshot Service

First, change the service to a oneshot type that runs a single scan and exits:

```bash
sudo tee /etc/systemd/system/spyder-scan.service > /dev/null << 'EOF'
[Unit]
Description=SPYDER Probe - Scheduled Scan
Documentation=https://github.com/gustycube/spyder
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
User=spyder
Group=spyder
WorkingDirectory=/opt/spyder
EnvironmentFile=/opt/spyder/config/spyder.env

ExecStart=/opt/spyder/bin/spyder \
  -config=/opt/spyder/config/config.yaml \
  -probe=scheduled-%H \
  -run=scan-%i

# Security hardening (same as long-running service)
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/spyder/spool /opt/spyder/logs
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
PrivateDevices=true
SystemCallArchitectures=native
LimitNOFILE=65536

# Timeout for the entire scan (4 hours)
TimeoutStartSec=14400

StandardOutput=journal
StandardError=journal
SyslogIdentifier=spyder-scan
EOF
```

### Create a Timer Unit

```bash
sudo tee /etc/systemd/system/spyder-scan.timer > /dev/null << 'EOF'
[Unit]
Description=SPYDER Scheduled Scan Timer
Documentation=https://github.com/gustycube/spyder

[Timer]
# Run daily at 2:00 AM UTC
OnCalendar=*-*-* 02:00:00
# Randomize start time within a 30-minute window to avoid thundering herd
RandomizedDelaySec=1800
# If a run was missed (e.g., machine was off), run it on next boot
Persistent=true
# Do not start immediately on enable; wait for the schedule
Unit=spyder-scan.service

[Install]
WantedBy=timers.target
EOF
```

### Enable the Timer

```bash
sudo systemctl daemon-reload
sudo systemctl enable spyder-scan.timer
sudo systemctl start spyder-scan.timer
```

### Timer Examples

**Weekly scan (Sunday at midnight):**

```ini
[Timer]
OnCalendar=Sun *-*-* 00:00:00
Persistent=true
```

**Every 6 hours:**

```ini
[Timer]
OnCalendar=*-*-* 00/6:00:00
Persistent=true
```

**First Monday of each month:**

```ini
[Timer]
OnCalendar=Mon *-*-1..7 03:00:00
Persistent=true
```

### Managing Timers

```bash
# List all timers and their next fire time
sudo systemctl list-timers --all | grep spyder

# Check timer status
sudo systemctl status spyder-scan.timer

# Trigger a scan immediately (outside the schedule)
sudo systemctl start spyder-scan.service

# View logs from the last scheduled scan
sudo journalctl -u spyder-scan --since "yesterday"

# Disable the timer
sudo systemctl stop spyder-scan.timer
sudo systemctl disable spyder-scan.timer
```

## Multiple Instances

Run multiple SPYDER instances on the same machine using systemd template units. This is useful for running probes with different configurations (e.g., different domain lists or concurrency settings).

### Template Unit

Create a template service file with `@` in the name:

```bash
sudo tee /etc/systemd/system/spyder@.service > /dev/null << 'EOF'
[Unit]
Description=SPYDER Probe - Instance %i
Documentation=https://github.com/gustycube/spyder
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=spyder
Group=spyder
WorkingDirectory=/opt/spyder
EnvironmentFile=/opt/spyder/config/spyder.env
EnvironmentFile=/opt/spyder/config/spyder-%i.env

ExecStart=/opt/spyder/bin/spyder \
  -config=/opt/spyder/config/config-%i.yaml \
  -probe=%H-%i

KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=60
Restart=on-failure
RestartSec=10

StandardOutput=journal
StandardError=journal
SyslogIdentifier=spyder-%i

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/spyder/spool /opt/spyder/logs
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
```

### Instance Configuration

Create per-instance config and env files:

```bash
# Instance "alpha"
sudo tee /opt/spyder/config/spyder-alpha.env > /dev/null << 'EOF'
REDIS_QUEUE_KEY=spyder:queue:alpha
EOF

sudo tee /opt/spyder/config/config-alpha.yaml > /dev/null << 'EOF'
domains: /opt/spyder/config/domains-alpha.txt
concurrency: 256
metrics_addr: ":9090"
batch_max_edges: 10000
batch_flush_sec: 2
spool_dir: /opt/spyder/spool
EOF

# Instance "beta"
sudo tee /opt/spyder/config/spyder-beta.env > /dev/null << 'EOF'
REDIS_QUEUE_KEY=spyder:queue:beta
EOF

sudo tee /opt/spyder/config/config-beta.yaml > /dev/null << 'EOF'
domains: /opt/spyder/config/domains-beta.txt
concurrency: 128
metrics_addr: ":9091"
batch_max_edges: 5000
batch_flush_sec: 3
spool_dir: /opt/spyder/spool
EOF
```

### Managing Template Instances

```bash
sudo systemctl daemon-reload

# Start individual instances
sudo systemctl start spyder@alpha
sudo systemctl start spyder@beta

# Enable on boot
sudo systemctl enable spyder@alpha
sudo systemctl enable spyder@beta

# Check status
sudo systemctl status spyder@alpha
sudo systemctl status spyder@beta

# View logs for a specific instance
sudo journalctl -u spyder@alpha -f

# Stop all instances
sudo systemctl stop 'spyder@*'
```
