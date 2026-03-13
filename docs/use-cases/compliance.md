# Compliance Scanning

SPYDER can audit external infrastructure for compliance-relevant properties: certificate validity, TLS configuration, third-party dependencies, and geographic distribution. By mapping the full graph of DNS, certificate, and HTTP relationships, it produces the data needed for certificate lifecycle management, vendor risk assessments, and regulatory compliance reporting.

## Certificate Expiration Monitoring

### Detecting Expiring Certificates

SPYDER captures `not_before` and `not_after` timestamps on every TLS certificate it encounters. Use this to identify certificates approaching expiration:

```bash
./bin/spyder -domains=production-domains.txt -concurrency=256 \
  2>/dev/null > cert-audit.json

# Find certificates expiring within 30 days
jq -r --arg cutoff "$(date -u -v+30d +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '+30 days' +%Y-%m-%dT%H:%M:%SZ)" \
  '.nodes_cert[] | select(.not_after < $cutoff) | [.subject_cn, .not_after, .issuer_cn] | @tsv' \
  cert-audit.json | sort -k2
```

### Certificate Inventory Report

Generate a full certificate inventory with expiration status:

```bash
jq -r '.nodes_cert[] | [.subject_cn, .issuer_cn, .not_before, .not_after, .spki_sha256] | @tsv' \
  cert-audit.json | \
  awk -F'\t' 'BEGIN {print "Subject\tIssuer\tIssued\tExpires\tSPKI"} {print}' | \
  column -t -s$'\t'
```

### Map Certificates to Domains

To know which services are affected by an expiring certificate, join certificate nodes with `USES_CERT` edges:

```bash
# For each certificate, list the domains using it
jq -r '
  (.nodes_cert | map({(.spki_sha256): {cn: .subject_cn, expires: .not_after}}) | add) as $certs |
  [.edges[] | select(.type=="USES_CERT") | {domain: .source, spki: .target}] |
  group_by(.spki) |
  map({
    certificate: .[0].spki,
    subject: ($certs[.[0].spki].cn // "unknown"),
    expires: ($certs[.[0].spki].expires // "unknown"),
    domains: [.[].domain]
  })
' cert-audit.json
```

### Automated Expiration Alerts

```bash
#!/bin/bash
# cert-expiry-check.sh - Run weekly via cron
WARN_DAYS=30
CRIT_DAYS=7
OUTPUT=$(mktemp)

./bin/spyder -domains=/etc/spyder/monitored-domains.txt \
  -concurrency=128 \
  -probe=cert-monitor \
  -run="cert-check-$(date +%Y%m%d)" \
  2>/dev/null > "$OUTPUT"

# Critical: expiring within 7 days
CRIT=$(jq -r --arg d "$(date -u -v+${CRIT_DAYS}d +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "+${CRIT_DAYS} days" +%Y-%m-%dT%H:%M:%SZ)" \
  '[.nodes_cert[] | select(.not_after < $d)] | length' "$OUTPUT")

# Warning: expiring within 30 days
WARN=$(jq -r --arg d "$(date -u -v+${WARN_DAYS}d +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "+${WARN_DAYS} days" +%Y-%m-%dT%H:%M:%SZ)" \
  '[.nodes_cert[] | select(.not_after < $d)] | length' "$OUTPUT")

if [ "$CRIT" -gt 0 ]; then
  echo "CRITICAL: ${CRIT} certificates expire within ${CRIT_DAYS} days"
  jq -r --arg d "$(date -u -v+${CRIT_DAYS}d +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "+${CRIT_DAYS} days" +%Y-%m-%dT%H:%M:%SZ)" \
    '.nodes_cert[] | select(.not_after < $d) | "  \(.subject_cn) expires \(.not_after)"' "$OUTPUT"
fi

if [ "$WARN" -gt 0 ]; then
  echo "WARNING: ${WARN} certificates expire within ${WARN_DAYS} days"
fi

rm -f "$OUTPUT"
```

## TLS Configuration Auditing

### Certificate Authority Audit

Verify that certificates are issued by approved CAs:

```bash
# List all certificate authorities in use
jq -r '.nodes_cert[].issuer_cn' cert-audit.json | sort | uniq -c | sort -rn

# Check for unapproved CAs
APPROVED_CAS="DigiCert|Let's Encrypt|Sectigo|GlobalSign"
jq -r '.nodes_cert[] | select(.issuer_cn | test("'"$APPROVED_CAS"'") | not) |
  "UNAPPROVED CA: \(.issuer_cn) for \(.subject_cn)"' cert-audit.json
```

### Wildcard Certificate Audit

Wildcard certificates expand the attack surface. Track their usage:

```bash
# Find all wildcard certificates
jq -r '.nodes_cert[] | select(.subject_cn | startswith("*")) |
  [.subject_cn, .issuer_cn, .not_after] | @tsv' cert-audit.json

# Count domains served by each wildcard
jq -r '
  [.nodes_cert[] | select(.subject_cn | startswith("*")) | .spki_sha256] as $wildcards |
  .edges[] | select(.type=="USES_CERT" and (.target | IN($wildcards[]))) |
  [.source, .target] | @tsv
' cert-audit.json | awk -F'\t' '{count[$2]++; domains[$2] = domains[$2] " " $1} END {for (c in count) print count[c] "\t" c "\t" domains[c]}' | sort -rn
```

### Self-Signed Certificate Detection

Self-signed certificates in production are a compliance finding in most frameworks:

```bash
jq -r '.nodes_cert[] | select(.subject_cn == .issuer_cn) |
  "SELF-SIGNED: \(.subject_cn) (expires \(.not_after))"' cert-audit.json
```

### Certificate Validity Period Audit

Some compliance frameworks require maximum certificate lifetimes (e.g., 398 days for public trust):

```bash
jq -r '.nodes_cert[] |
  ((.not_after | fromdateiso8601) - (.not_before | fromdateiso8601)) / 86400 as $days |
  select($days > 398) |
  "LONG-LIVED: \(.subject_cn) valid for \($days | floor) days (\(.not_before) to \(.not_after))"' \
  cert-audit.json
```

## Third-Party Dependency Tracking

### External Link Analysis

`LINKS_TO` edges reveal every external domain that your web properties load resources from. This is critical for vendor risk management and data processing assessments.

```bash
# List all third-party domains loaded by your sites
jq -r '.edges[] | select(.type=="LINKS_TO") | .target' cert-audit.json | \
  sort -u > third-party-domains.txt

# Count third-party dependencies per domain
jq -r '.edges[] | select(.type=="LINKS_TO") | [.source, .target] | @tsv' \
  cert-audit.json | \
  awk -F'\t' '{count[$1]++} END {for (d in count) print count[d] "\t" d}' | sort -rn
```

### Categorize Third-Party Dependencies

```bash
jq -r '.edges[] | select(.type=="LINKS_TO") | .target' cert-audit.json | \
  sort -u | \
  awk '{
    if ($0 ~ /google-analytics|gtag|googletagmanager/) cat="Analytics"
    else if ($0 ~ /facebook|twitter|linkedin|instagram/) cat="Social Media"
    else if ($0 ~ /stripe|paypal|braintree/) cat="Payment"
    else if ($0 ~ /cdn|cloudflare|cloudfront|akamai|fastly/) cat="CDN"
    else if ($0 ~ /sentry|bugsnag|datadog|newrelic/) cat="Monitoring"
    else if ($0 ~ /fonts\.google|typekit/) cat="Fonts"
    else cat="Other"
    print cat "\t" $0
  }' | sort -k1
```

### CDN and CNAME Delegation Tracking

CNAME records show where you have delegated control of your subdomains to third parties:

```bash
# All CNAME delegations to external providers
jq -r '.edges[] | select(.type=="ALIAS_OF") | [.source, .target] | @tsv' \
  cert-audit.json | \
  awk -F'\t' '{
    # Check if target is a different apex domain
    split($1, a, "."); split($2, b, ".")
    src_apex = a[length(a)-1] "." a[length(a)]
    tgt_apex = b[length(b)-1] "." b[length(b)]
    if (src_apex != tgt_apex) print "EXTERNAL DELEGATION: " $1 " -> " $2
  }'
```

### DNS Provider Dependency Report

```bash
# Generate a DNS provider dependency matrix
jq -r '.edges[] | select(.type=="USES_NS") | [.source, .target] | @tsv' \
  cert-audit.json | \
  awk -F'\t' '{
    split($2, ns, ".")
    provider = ns[length(ns)-1] "." ns[length(ns)]
    deps[provider]++
    domains[provider] = domains[provider] " " $1
  } END {
    for (p in deps) print deps[p] "\t" p
  }' | sort -rn
```

## Geographic Infrastructure Distribution Analysis

### IP Geolocation Mapping

SPYDER captures all IP addresses your domains resolve to. Combine this with geolocation data to map where your infrastructure is physically located:

```bash
# Extract all unique IPs
jq -r '.nodes_ip[].ip' cert-audit.json | sort -u > all-ips.txt

# Use an IP geolocation service to map locations
# Example with a hypothetical geo lookup
while read ip; do
  country=$(curl -s "https://ipinfo.io/${ip}/country" 2>/dev/null)
  echo "${ip}\t${country}"
done < all-ips.txt > ip-geo.tsv

# Summarize by country
awk -F'\t' '{print $2}' ip-geo.tsv | sort | uniq -c | sort -rn
```

### Data Residency Compliance

For GDPR or data sovereignty requirements, verify that infrastructure serving EU users is located in approved regions:

```bash
# Cross-reference domains with their IP locations
jq -r '.edges[] | select(.type=="RESOLVES_TO") | [.source, .target] | @tsv' \
  cert-audit.json | \
  while IFS=$'\t' read -r domain ip; do
    country=$(grep "^${ip}" ip-geo.tsv | awk -F'\t' '{print $2}')
    echo "${domain}\t${ip}\t${country}"
  done | grep -v -E '\t(DE|FR|IE|NL|FI|SE)\s*$'
  # Flag domains NOT in approved EU countries
```

## Regulatory Compliance Considerations

### GDPR Data Flow Mapping

SPYDER's `LINKS_TO` edges reveal data flows to third parties. Under GDPR, each of these represents a potential data processor relationship:

```bash
# Extract all third-party data flows for GDPR Article 30 records
jq -r '.edges[] | select(.type=="LINKS_TO") | {
  source_domain: .source,
  third_party: .target,
  observed_at: .observed_at,
  data_flow_type: "HTTP resource loading"
}' cert-audit.json > data-flow-inventory.json
```

### SOC 2 Infrastructure Controls

SOC 2 Type II audits require evidence of infrastructure monitoring. SPYDER scans provide:

- **CC6.1 (Logical access)**: Certificate inventory and authority tracking
- **CC7.1 (System monitoring)**: Regular infrastructure scans with change detection
- **CC7.2 (Anomaly detection)**: Infrastructure changes between scans
- **CC8.1 (Change management)**: DNS and certificate change tracking

```bash
# Generate SOC 2 evidence: infrastructure baseline
DATE=$(date +%Y%m%d)
./bin/spyder -domains=/etc/spyder/production-domains.txt \
  -concurrency=256 \
  -probe=compliance-scanner \
  -run="soc2-baseline-${DATE}" \
  2>/dev/null > "/var/lib/spyder/compliance/baseline-${DATE}.json"

# Generate change report against previous baseline
PREV=$(ls -t /var/lib/spyder/compliance/baseline-*.json | sed -n '2p')
if [ -n "$PREV" ]; then
  echo "=== Infrastructure Changes Since Last Baseline ==="

  echo "--- New Domains ---"
  diff <(jq -r '.nodes_domain[].host' "$PREV" | sort -u) \
       <(jq -r '.nodes_domain[].host' "/var/lib/spyder/compliance/baseline-${DATE}.json" | sort -u) | grep "^>"

  echo "--- New IPs ---"
  diff <(jq -r '.nodes_ip[].ip' "$PREV" | sort -u) \
       <(jq -r '.nodes_ip[].ip' "/var/lib/spyder/compliance/baseline-${DATE}.json" | sort -u) | grep "^>"

  echo "--- Certificate Changes ---"
  diff <(jq -r '.nodes_cert[].spki_sha256' "$PREV" | sort -u) \
       <(jq -r '.nodes_cert[].spki_sha256' "/var/lib/spyder/compliance/baseline-${DATE}.json" | sort -u)
fi
```

### PCI DSS Certificate Requirements

PCI DSS requires strong cryptographic controls. Audit certificate key sizes and algorithms:

```bash
# Flag certificates with concerning properties
jq -r '.nodes_cert[] | select(
  (.not_after | fromdateiso8601) - (.not_before | fromdateiso8601) > (398 * 86400)
) | "LONG-LIVED CERT: \(.subject_cn) (\(.not_before) to \(.not_after))"' cert-audit.json

# Verify no self-signed certificates in production
SELF_SIGNED=$(jq '[.nodes_cert[] | select(.subject_cn == .issuer_cn)] | length' cert-audit.json)
if [ "$SELF_SIGNED" -gt 0 ]; then
  echo "PCI FINDING: ${SELF_SIGNED} self-signed certificate(s) detected"
fi
```

## Scheduled Scanning with systemd Timers

### Service Unit

Create a systemd service for the compliance scan:

```ini
# /etc/systemd/system/spyder-compliance.service
[Unit]
Description=SPYDER Compliance Scanner
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
User=spyder
Group=spyder
WorkingDirectory=/opt/spyder

ExecStart=/opt/spyder/bin/spyder \
  -domains=/etc/spyder/compliance-domains.txt \
  -concurrency=256 \
  -probe=compliance-scanner \
  -run=compliance-%i \
  -ingest=https://compliance-ingest.internal/v1/batch \
  -exclude_tlds=gov,mil,int

# Resource limits
MemoryMax=2G
CPUQuota=50%

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=spyder-compliance

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/var/lib/spyder /opt/spyder/spool
PrivateTmp=true
```

### Timer Unit

Schedule the scan to run weekly:

```ini
# /etc/systemd/system/spyder-compliance.timer
[Unit]
Description=Weekly SPYDER Compliance Scan

[Timer]
OnCalendar=Mon *-*-* 02:00:00
Persistent=true
RandomizedDelaySec=1800

[Install]
WantedBy=timers.target
```

### Enable and Monitor

```bash
# Enable the timer
sudo systemctl daemon-reload
sudo systemctl enable --now spyder-compliance.timer

# Check timer status
systemctl list-timers spyder-compliance.timer

# View scan logs
journalctl -u spyder-compliance.service --since "1 week ago"

# Check last run result
systemctl status spyder-compliance.service
```

### Daily Certificate Check Timer

For more frequent certificate monitoring:

```ini
# /etc/systemd/system/spyder-cert-check.timer
[Unit]
Description=Daily SPYDER Certificate Expiration Check

[Timer]
OnCalendar=*-*-* 06:00:00
Persistent=true

[Install]
WantedBy=timers.target
```

```ini
# /etc/systemd/system/spyder-cert-check.service
[Unit]
Description=SPYDER Certificate Expiration Check
After=network-online.target

[Service]
Type=oneshot
User=spyder
ExecStart=/opt/spyder/scripts/cert-expiry-check.sh
StandardOutput=journal
StandardError=journal
SyslogIdentifier=spyder-cert-check
```

## Compliance Reporting

### Generate a Compliance Summary

```bash
./bin/spyder -domains=production-domains.txt -concurrency=256 \
  2>/dev/null > compliance-scan.json

jq '{
  scan_date: now | todate,
  total_domains: [.nodes_domain[].host] | unique | length,
  total_ips: [.nodes_ip[].ip] | unique | length,
  total_certificates: [.nodes_cert[].spki_sha256] | unique | length,
  certificate_authorities: [.nodes_cert[].issuer_cn] | unique,
  third_party_dependencies: [.edges[] | select(.type=="LINKS_TO") | .target] | unique | length,
  dns_providers: [.edges[] | select(.type=="USES_NS") | .target] | unique | length,
  external_cname_delegations: [.edges[] | select(.type=="ALIAS_OF") | .target] | unique | length,
  wildcard_certificates: [.nodes_cert[] | select(.subject_cn | startswith("*"))] | length,
  self_signed_certificates: [.nodes_cert[] | select(.subject_cn == .issuer_cn)] | length
}' compliance-scan.json
```

This summary provides a snapshot suitable for inclusion in compliance reports, audit evidence packages, and management dashboards.
