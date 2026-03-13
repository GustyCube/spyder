# Threat Intelligence

SPYDER supports threat intelligence workflows by mapping the infrastructure behind malicious domains. Starting from known indicators of compromise (IOCs), it expands the graph through DNS resolution, TLS certificate analysis, and HTTP link extraction to uncover related infrastructure that shares hosting, certificates, or content relationships with known threats.

## IOC Expansion from Known Malicious Domains

### Seed and Expand

The core threat intel workflow with SPYDER starts with a known malicious domain and expands outward by analyzing its infrastructure relationships:

```bash
# Start with known IOCs
cat <<EOF > iocs.txt
malware-delivery.example
phishing-login.example
c2-callback.example
EOF

./bin/spyder -domains=iocs.txt -concurrency=64 -exclude_tlds=gov,mil,int,edu \
  2>/dev/null > ioc-expansion.json
```

### Pivot on Shared IP Addresses

Domains sharing an IP with a known malicious domain are candidates for related infrastructure:

```bash
# Extract IPs from known-bad domains
jq -r '.edges[] | select(.type=="RESOLVES_TO") | [.source, .target] | @tsv' \
  ioc-expansion.json > ip-map.tsv

# Find all domains on those IPs (requires a second scan)
awk -F'\t' '{print $2}' ip-map.tsv | sort -u > suspect-ips.txt

# Cross-reference against a broader scan
./bin/spyder -domains=broad-domains.txt 2>/dev/null | \
  jq -r '.edges[] | select(.type=="RESOLVES_TO") | [.source, .target] | @tsv' | \
  grep -F -f suspect-ips.txt
```

### Pivot on Shared Certificates

Certificates bind multiple domains together. A certificate used by a known-bad domain that also covers other domains via Subject Alternative Names reveals related infrastructure:

```bash
# Extract certificate fingerprints from IOC scan
jq -r '.edges[] | select(.type=="USES_CERT") | .target' \
  ioc-expansion.json | sort -u > bad-certs.txt

# Find other domains using the same certificates
./bin/spyder -domains=broad-domains.txt 2>/dev/null | \
  jq -r '.edges[] | select(.type=="USES_CERT") | [.source, .target] | @tsv' | \
  grep -F -f bad-certs.txt
```

### Pivot on Nameserver Infrastructure

Threat actors often register multiple domains through the same registrar and delegate them to the same nameservers:

```bash
# Extract nameservers from IOC domains
jq -r '.edges[] | select(.type=="USES_NS") | .target' \
  ioc-expansion.json | sort -u

# Find other domains using the same nameservers
jq -r '.edges[] | select(.type=="USES_NS") | [.source, .target] | @tsv' \
  ioc-expansion.json | sort -k2
```

If `malware-delivery.example` and `phishing-login.example` both use `ns1.shadydns.example` and `ns2.shadydns.example`, that nameserver provider is a strong pivot point for discovering additional threat actor domains.

## Fast-Flux Detection Through DNS Pattern Analysis

Fast-flux networks rapidly rotate the IP addresses behind a domain, making takedowns difficult. SPYDER can detect this pattern by running repeated scans and comparing DNS resolution results.

### Detecting IP Rotation

```bash
# Scan the same domains at two different times
./bin/spyder -domains=suspect-domains.txt -run=scan-t1 2>/dev/null > scan-t1.json
sleep 300
./bin/spyder -domains=suspect-domains.txt -run=scan-t2 2>/dev/null > scan-t2.json

# Compare IP resolutions
diff <(jq -r '.edges[] | select(.type=="RESOLVES_TO") | [.source, .target] | @tsv' scan-t1.json | sort) \
     <(jq -r '.edges[] | select(.type=="RESOLVES_TO") | [.source, .target] | @tsv' scan-t2.json | sort)
```

Domains with significantly different resolution sets between scans are candidates for fast-flux.

### Scoring IP Diversity

```bash
# Count unique IPs per domain
jq -r '.edges[] | select(.type=="RESOLVES_TO") | [.source, .target] | @tsv' \
  scan-output.json | \
  awk -F'\t' '{ips[$1]++} END {for (d in ips) if (ips[d] > 5) print ips[d] "\t" d}' | \
  sort -rn
```

A domain resolving to many different IPs (especially in different subnets) is a strong fast-flux indicator. Legitimate CDNs also show high IP diversity, so combine this with AS number analysis to distinguish CDNs from botnets.

### Automated Fast-Flux Analysis

```python
import json
from collections import defaultdict
from ipaddress import ip_address, ip_network

def detect_fast_flux(scan_files):
    """Analyze multiple scan results for fast-flux indicators."""
    domain_ips = defaultdict(set)

    for scan_file in scan_files:
        with open(scan_file) as f:
            for line in f:
                data = json.loads(line)
                for edge in data.get("edges", []):
                    if edge["type"] == "RESOLVES_TO":
                        domain_ips[edge["source"]].add(edge["target"])

    suspects = {}
    for domain, ips in domain_ips.items():
        if len(ips) < 3:
            continue

        # Count unique /24 subnets
        subnets = set()
        for ip_str in ips:
            try:
                addr = ip_address(ip_str)
                if addr.version == 4:
                    subnets.add(ip_network(f"{ip_str}/24", strict=False))
            except ValueError:
                continue

        if len(subnets) >= 3:
            suspects[domain] = {
                "ip_count": len(ips),
                "subnet_count": len(subnets),
                "ips": list(ips)
            }

    return suspects
```

## C2 Infrastructure Mapping

### Mapping Command and Control Networks

C2 servers often share infrastructure patterns. SPYDER can map these by starting from a known C2 domain and expanding through DNS and certificate relationships:

```bash
# Seed with known C2 domain
echo "known-c2.example" > c2-seed.txt

# Run recursive crawl to discover related infrastructure
./bin/spyder -domains=c2-seed.txt \
  -continuous \
  -max_domains=500 \
  -concurrency=64 \
  -exclude_tlds=gov,mil,int,edu \
  -ua="ThreatResearch/1.0" \
  2>/dev/null > c2-map.json
```

### Identifying C2 Indicators

```bash
# Domains discovered through links from C2 (potential staging/exfil)
jq -r '.edges[] | select(.type=="LINKS_TO") | [.source, .target] | @tsv' \
  c2-map.json | sort -k1

# Infrastructure sharing (domains on same IP as C2)
C2_IPS=$(jq -r '.edges[] | select(.type=="RESOLVES_TO" and .source=="known-c2.example") | .target' c2-map.json)
for ip in $C2_IPS; do
  echo "=== Domains on $ip ==="
  jq -r --arg ip "$ip" '.edges[] | select(.type=="RESOLVES_TO" and .target==$ip) | .source' c2-map.json
done
```

### Building a C2 Infrastructure Profile

```bash
# Generate a C2 profile from SPYDER output
jq '{
  c2_domain: "known-c2.example",
  resolved_ips: [.edges[] | select(.type=="RESOLVES_TO" and .source=="known-c2.example") | .target] | unique,
  nameservers: [.edges[] | select(.type=="USES_NS" and .source=="known-c2.example") | .target] | unique,
  certificates: [.edges[] | select(.type=="USES_CERT" and .source=="known-c2.example") | .target] | unique,
  linked_domains: [.edges[] | select(.type=="LINKS_TO" and .source=="known-c2.example") | .target] | unique,
  co_hosted_domains: [.edges[] | select(.type=="RESOLVES_TO") | .source] | unique | length
}' c2-map.json
```

## Certificate-Based Threat Actor Tracking

### Certificate Fingerprint Tracking

TLS certificates provide durable tracking identifiers. Even when a threat actor changes IPs and domains, they may reuse the same certificate or certificate authority patterns.

```bash
# Extract certificate details for threat actor domains
jq '[.nodes_cert[] | {
  spki: .spki_sha256,
  subject: .subject_cn,
  issuer: .issuer_cn,
  valid_from: .not_before,
  valid_until: .not_after
}]' threat-scan.json
```

### Certificate Authority Pattern Analysis

Threat actors tend to use consistent certificate providers. Track which CAs issue certificates to suspicious domains:

```bash
# CA distribution across suspect domains
jq -r '.nodes_cert[].issuer_cn' threat-scan.json | \
  sort | uniq -c | sort -rn
```

A cluster of domains all using certificates from the same free CA, issued within a narrow time window, is a strong indicator of coordinated infrastructure setup.

### Certificate Timeline Analysis

```bash
# Find certificates issued in a tight time window (batch registration)
jq -r '.nodes_cert[] | [.not_before, .subject_cn, .issuer_cn] | @tsv' \
  threat-scan.json | sort -k1
```

If many certificates were issued on the same day for different domains, the domains were likely registered together as part of a campaign.

### Wildcard Certificate Abuse Detection

```bash
# Find wildcard certificates and their associated domains
jq -r '.nodes_cert[] | select(.subject_cn | startswith("*")) | .subject_cn' \
  threat-scan.json | sort -u

# Find which domains use these wildcards
WILDCARDS=$(jq -r '.nodes_cert[] | select(.subject_cn | startswith("*")) | .spki_sha256' threat-scan.json)
for spki in $WILDCARDS; do
  echo "=== Certificate: $spki ==="
  jq -r --arg s "$spki" '.edges[] | select(.type=="USES_CERT" and .target==$s) | .source' threat-scan.json
done
```

## Integration with Threat Intelligence Platforms

### STIX/TAXII Export

Convert SPYDER output to STIX indicators for ingestion into threat intelligence platforms:

```python
import json
import uuid
from datetime import datetime

def spyder_to_stix(spyder_file):
    """Convert SPYDER edges to STIX 2.1 indicators."""
    indicators = []

    with open(spyder_file) as f:
        for line in f:
            data = json.loads(line)
            for edge in data.get("edges", []):
                if edge["type"] == "RESOLVES_TO":
                    indicators.append({
                        "type": "indicator",
                        "spec_version": "2.1",
                        "id": f"indicator--{uuid.uuid4()}",
                        "created": datetime.utcnow().isoformat() + "Z",
                        "name": f"{edge['source']} resolves to {edge['target']}",
                        "pattern": f"[domain-name:value = '{edge['source']}']",
                        "pattern_type": "stix",
                        "valid_from": edge["observed_at"],
                        "labels": ["malicious-activity"]
                    })

    return {"type": "bundle", "id": f"bundle--{uuid.uuid4()}", "objects": indicators}
```

### MISP Event Creation

```bash
# Extract IOCs for MISP import
jq -r '.edges[] | select(.type=="RESOLVES_TO") |
  "domain-ip|" + .source + "|" + .target' threat-scan.json > misp-import.csv

jq -r '.nodes_cert[] |
  "x509-fingerprint-sha256|" + .spki_sha256' threat-scan.json >> misp-import.csv
```

### Blocklist Generation

```bash
# Generate domain blocklist from expanded IOCs
jq -r '.nodes_domain[].host' threat-scan.json | sort -u > blocklist-domains.txt

# Generate IP blocklist
jq -r '.nodes_ip[].ip' threat-scan.json | sort -u > blocklist-ips.txt

# Generate in common firewall formats
awk '{print "deny ip any host " $1}' blocklist-ips.txt > acl-rules.txt
```

## Continuous Monitoring with Recursive Mode

### Persistent Threat Monitoring

Use `-continuous` mode to continuously expand the threat actor's known infrastructure as new domains are discovered:

```bash
./bin/spyder -domains=threat-seeds.txt \
  -continuous \
  -max_domains=10000 \
  -concurrency=128 \
  -exclude_tlds=gov,mil,int,edu \
  -ingest=https://threat-ingest.internal/v1/batch \
  -probe=threat-monitor-1 \
  -ua="ThreatIntel/1.0"
```

In this mode, when SPYDER discovers that `malware-c2.example` links to `exfil-staging.example`, the second domain is automatically added to the crawl queue. Its DNS, certificates, and links are then analyzed, potentially discovering `payload-host.example`, and so on.

### Scheduled Threat Sweeps

Run periodic scans against known threat actor infrastructure to detect changes:

```bash
#!/bin/bash
# threat-sweep.sh - Run daily against known threat IOCs
DATE=$(date +%Y%m%d-%H%M)
SEEDS="/etc/spyder/threat-iocs.txt"
OUTPUT="/var/lib/spyder/threat-sweeps/sweep-${DATE}.json"

./bin/spyder \
  -domains="${SEEDS}" \
  -continuous \
  -max_domains=5000 \
  -concurrency=256 \
  -probe=threat-sweep \
  -run="sweep-${DATE}" \
  -exclude_tlds=gov,mil,int,edu \
  2>/dev/null > "${OUTPUT}"

# Alert on new infrastructure
PREV=$(ls -t /var/lib/spyder/threat-sweeps/sweep-*.json | sed -n '2p')
if [ -n "$PREV" ]; then
  NEW_DOMAINS=$(diff <(jq -r '.nodes_domain[].host' "$PREV" | sort -u) \
                     <(jq -r '.nodes_domain[].host' "$OUTPUT" | sort -u) | grep "^>" | wc -l)
  NEW_IPS=$(diff <(jq -r '.nodes_ip[].ip' "$PREV" | sort -u) \
                 <(jq -r '.nodes_ip[].ip' "$OUTPUT" | sort -u) | grep "^>" | wc -l)

  if [ "$NEW_DOMAINS" -gt 0 ] || [ "$NEW_IPS" -gt 0 ]; then
    echo "ALERT: ${NEW_DOMAINS} new domains, ${NEW_IPS} new IPs detected in threat sweep" | \
      mail -s "SPYDER Threat Sweep Alert" soc@company.com
  fi
fi
```

### Redis-Backed Continuous Monitoring

For ongoing threat monitoring across a team of analysts, use Redis-backed queuing so that any analyst can add new seed domains and the crawl continues:

```bash
export REDIS_ADDR=redis.soc.internal:6379
export REDIS_QUEUE_ADDR=redis.soc.internal:6379
export REDIS_QUEUE_KEY=spyder:threat-intel

./bin/spyder \
  -continuous \
  -max_domains=50000 \
  -concurrency=256 \
  -ingest=https://threat-ingest.internal/v1/batch \
  -probe=threat-worker-$(hostname) \
  -domains=/etc/spyder/threat-seeds.txt
```

New IOCs can be pushed to the Redis queue independently, and the running probe instances will pick them up and begin expanding the infrastructure graph.

## Operational Considerations

### Rate Limiting and Ethics

When scanning threat infrastructure, use responsible rate limiting and identification:

```bash
./bin/spyder -domains=iocs.txt \
  -concurrency=64 \
  -ua="ThreatResearch/1.0 (+https://your-org.com/security-research)" \
  -exclude_tlds=gov,mil,int,edu
```

SPYDER respects `robots.txt` by default. For threat intel use cases where you need to scan domains that block crawlers, be aware that robots.txt compliance may limit visibility into certain domains.

### Data Handling

Threat intelligence data from SPYDER scans should be handled with appropriate controls:

- Store scan results in access-controlled storage
- Limit retention to the period required by your threat intel program
- Anonymize or redact data when sharing outside your organization
- Tag data with classification labels appropriate to your environment
- Consider legal jurisdiction when scanning infrastructure in other countries
