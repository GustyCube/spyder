# Infrastructure Mapping

SPYDER maps internet infrastructure by resolving DNS records, analyzing TLS certificates, and extracting HTTP links across domains. This produces a graph of relationships that reveals network topology, hosting providers, CDN usage, and shared infrastructure patterns.

## Network Topology Discovery

### DNS-Based Topology Mapping

SPYDER resolves A, AAAA, CNAME, NS, and MX records for every domain it processes. The resulting `RESOLVES_TO`, `ALIAS_OF`, `USES_NS`, and `USES_MX` edges form a DNS dependency graph that mirrors the real topology of your infrastructure.

**Map a set of organizational domains:**

```bash
cat <<EOF > org-domains.txt
company.com
api.company.com
docs.company.com
staging.company.com
blog.company.com
EOF

./bin/spyder -domains=org-domains.txt -concurrency=64
```

**Extract the IP-to-domain mapping:**

```bash
./bin/spyder -domains=org-domains.txt 2>/dev/null | \
  jq -r '.edges[] | select(.type=="RESOLVES_TO") | [.source, .target] | @tsv' | \
  sort -k2
```

Example output:

```
api.company.com     104.21.32.15
docs.company.com    104.21.32.15
company.com         104.21.32.16
blog.company.com    185.199.108.153
staging.company.com 34.102.136.180
```

This immediately shows that `api.company.com` and `docs.company.com` share an IP (likely behind the same load balancer or CDN edge), while `blog.company.com` sits on GitHub Pages infrastructure and `staging.company.com` runs on Google Cloud.

### CNAME Chain Analysis

CNAME records (exposed as `ALIAS_OF` edges) reveal CDN and service provider delegations:

```bash
./bin/spyder -domains=org-domains.txt 2>/dev/null | \
  jq -r '.edges[] | select(.type=="ALIAS_OF") | "\(.source) -> \(.target)"'
```

```
www.company.com -> company.com.cdn.cloudflare.net
docs.company.com -> hosting.gitbook.io
status.company.com -> statuspage.betteruptime.com
```

Each CNAME target reveals which third-party service hosts that subdomain. This is invaluable for understanding external dependencies without access to internal DNS zone files.

## CDN and Hosting Provider Identification

### Detecting CDN Usage

CDN providers appear as CNAME targets and as shared IP ranges. Extract CDN indicators from SPYDER output:

```bash
# Find all CNAME targets that suggest CDN usage
./bin/spyder -domains=domains.txt 2>/dev/null | \
  jq -r '.edges[] | select(.type=="ALIAS_OF") | .target' | \
  grep -iE '(cloudflare|cloudfront|akamai|fastly|cdn|edgecast|azureedge)' | \
  sort -u
```

**Identify which domains use which CDN:**

```bash
./bin/spyder -domains=domains.txt 2>/dev/null | \
  jq -r '.edges[] | select(.type=="ALIAS_OF") | "\(.source)\t\(.target)"' | \
  awk -F'\t' '{
    cdn="unknown"
    if ($2 ~ /cloudflare/) cdn="Cloudflare"
    else if ($2 ~ /cloudfront/) cdn="CloudFront"
    else if ($2 ~ /akamai/) cdn="Akamai"
    else if ($2 ~ /fastly/) cdn="Fastly"
    else if ($2 ~ /azureedge/) cdn="Azure CDN"
    print $1 "\t" cdn
  }' | sort -k2
```

### Hosting Provider Clustering

Group domains by their resolved IP addresses to find shared hosting:

```bash
./bin/spyder -domains=domains.txt 2>/dev/null | \
  jq -r '.edges[] | select(.type=="RESOLVES_TO") | [.target, .source] | @tsv' | \
  sort | \
  awk -F'\t' '{
    if ($1 == prev_ip) {
      domains = domains ", " $2
    } else {
      if (NR > 1 && count > 1) print prev_ip "\t(" count " domains)\t" domains
      prev_ip = $1; domains = $2; count = 0
    }
    count++
  } END { if (count > 1) print prev_ip "\t(" count " domains)\t" domains }'
```

This surfaces IPs that host multiple domains, a strong indicator of shared hosting, CDN edge nodes, or consolidated infrastructure.

## DNS Dependency Mapping

### Nameserver Analysis

NS records reveal which DNS providers each domain depends on. A single DNS provider outage can take down every domain delegated to it.

**Extract nameserver relationships:**

```bash
./bin/spyder -domains=domains.txt 2>/dev/null | \
  jq -r '.edges[] | select(.type=="USES_NS") | [.source, .target] | @tsv' | \
  sort -k2
```

**Find the most common nameserver providers:**

```bash
./bin/spyder -domains=domains.txt 2>/dev/null | \
  jq -r '.edges[] | select(.type=="USES_NS") | .target' | \
  sed 's/^[^.]*\.//' | \
  sort | uniq -c | sort -rn | head -20
```

Example output:

```
   142 cloudflare.com
    87 awsdns-34.org
    87 awsdns-34.net
    87 awsdns-34.com
    34 domaincontrol.com
    21 googledomains.com
    12 ns.cloudflare.com
```

This shows that 142 of the scanned domains depend on Cloudflare DNS, while 87 use AWS Route 53.

### DNS Provider Concentration Risk

Calculate the concentration of DNS providers to assess single-point-of-failure risk:

```bash
./bin/spyder -domains=domains.txt 2>/dev/null | \
  jq -r '.edges[] | select(.type=="USES_NS") | .target' | \
  sed 's/^[^.]*\.//' | sort -u | wc -l
```

If a small number of providers serve a large percentage of your domains, you have a concentration risk.

### Mail Server Mapping

MX records show email infrastructure dependencies:

```bash
./bin/spyder -domains=domains.txt 2>/dev/null | \
  jq -r '.edges[] | select(.type=="USES_MX") | [.source, .target] | @tsv' | \
  sort -k2
```

**Identify email providers:**

```bash
./bin/spyder -domains=domains.txt 2>/dev/null | \
  jq -r '.edges[] | select(.type=="USES_MX") | .target' | \
  sed 's/^[0-9]*\.//' | \
  awk '{
    if ($0 ~ /google/) print "Google Workspace"
    else if ($0 ~ /outlook|microsoft/) print "Microsoft 365"
    else if ($0 ~ /mimecast/) print "Mimecast"
    else if ($0 ~ /protonmail/) print "ProtonMail"
    else print $0
  }' | sort | uniq -c | sort -rn
```

## Shared Infrastructure Detection

### Common IPs Across Domains

Domains that resolve to the same IP address share underlying infrastructure. This can indicate shared hosting, CDN edge proximity, or organizational relationships.

```bash
# Find IPs shared by 3 or more domains
./bin/spyder -domains=domains.txt 2>/dev/null | \
  jq -r '.edges[] | select(.type=="RESOLVES_TO") | [.target, .source] | @tsv' | \
  sort -k1 | \
  awk -F'\t' '
    {a[$1] = a[$1] ? a[$1] "," $2 : $2; c[$1]++}
    END {for (ip in c) if (c[ip] >= 3) print ip "\t" c[ip] "\t" a[ip]}
  ' | sort -t$'\t' -k2 -rn
```

### Shared Certificate Detection

Domains sharing a TLS certificate are typically operated by the same entity or served from the same infrastructure:

```bash
# Group domains by certificate SPKI hash
./bin/spyder -domains=domains.txt 2>/dev/null | \
  jq -r '.edges[] | select(.type=="USES_CERT") | [.target, .source] | @tsv' | \
  sort -k1 | \
  awk -F'\t' '
    {a[$1] = a[$1] ? a[$1] ", " $2 : $2; c[$1]++}
    END {for (spki in c) if (c[spki] >= 2) print c[spki] " domains share cert " substr(spki,1,16) "...: " a[spki]}
  ' | sort -rn
```

### Cross-Reference Link Analysis

`LINKS_TO` edges show content-level relationships between domains:

```bash
# Find domains that link to each other (bidirectional links)
./bin/spyder -domains=domains.txt 2>/dev/null | \
  jq -r '.edges[] | select(.type=="LINKS_TO") | [.source, .target] | sort | @tsv' | \
  sort | uniq -c | sort -rn | head -20
```

## Recursive Crawling for Deep Infrastructure Discovery

### Using Continuous Mode

The `-continuous` flag enables recursive crawling. When SPYDER discovers a new domain through DNS, TLS, or HTTP analysis, it adds that domain to the crawl queue. This expands the graph outward from your seed domains.

**Basic recursive crawl:**

```bash
./bin/spyder -domains=seed-domains.txt -continuous -concurrency=128
```

**Bounded recursive crawl (recommended for infrastructure mapping):**

```bash
./bin/spyder -domains=seed-domains.txt \
  -continuous \
  -max_domains=5000 \
  -concurrency=256 \
  -batch_flush_sec=2
```

The `-max_domains` flag limits how many discovered domains SPYDER will recursively crawl. Without it, a recursive crawl starting from a well-connected domain can expand to hundreds of thousands of domains.

### How Discovery Feeds Back

Each crawled domain can discover new domains through several channels:

1. **NS records**: Nameserver hostnames are discovered and queued
2. **CNAME records**: Alias targets are discovered and queued
3. **MX records**: Mail server hostnames are discovered and queued
4. **HTTP links**: External domains found in page content are discovered and queued
5. **TLS SAN entries**: Subject Alternative Name domains on certificates are observed

In continuous mode, every newly discovered domain from these sources is fed back into the crawl queue (after deduplication). This produces a breadth-first expansion of the infrastructure graph.

### Distributed Recursive Crawling

For large-scale infrastructure mapping, use Redis-backed queuing so multiple probe instances share the discovery queue:

```bash
export REDIS_ADDR=redis.internal:6379
export REDIS_QUEUE_ADDR=redis.internal:6379
export REDIS_QUEUE_KEY=spyder:infra-map

# Start on multiple nodes
./bin/spyder \
  -domains=seed-domains.txt \
  -continuous \
  -max_domains=50000 \
  -concurrency=512 \
  -ingest=https://ingest.internal/v1/batch \
  -probe=worker-$(hostname)
```

All workers share the same Redis queue, so discovered domains are distributed across workers automatically.

## Practical Examples

### Map a Company's Full External Footprint

```bash
# Start with known domains
cat <<EOF > company-seeds.txt
company.com
company.io
company-api.com
company-cdn.com
EOF

# Run bounded recursive crawl
./bin/spyder -domains=company-seeds.txt \
  -continuous \
  -max_domains=2000 \
  -concurrency=128 \
  -exclude_tlds=gov,mil,int \
  -ua="InfraAudit/1.0 (+https://company.com/security)" \
  2>/dev/null > infra-output.json

# Summarize findings
echo "=== Domains discovered ==="
jq -r '.nodes_domain[].host' infra-output.json | sort -u | wc -l

echo "=== Unique IPs ==="
jq -r '.nodes_ip[].ip' infra-output.json | sort -u | wc -l

echo "=== Edge type distribution ==="
jq -r '.edges[].type' infra-output.json | sort | uniq -c | sort -rn

echo "=== Certificate issuers ==="
jq -r '.nodes_cert[].issuer_cn' infra-output.json | sort | uniq -c | sort -rn
```

### Generate a Network Dependency Report

```bash
./bin/spyder -domains=domains.txt 2>/dev/null | \
  jq '{
    dns_providers: [.edges[] | select(.type=="USES_NS") | .target] | unique | length,
    unique_ips: [.nodes_ip[].ip] | unique | length,
    cert_authorities: [.nodes_cert[].issuer_cn] | unique,
    mx_providers: [.edges[] | select(.type=="USES_MX") | .target] | unique | length,
    external_links: [.edges[] | select(.type=="LINKS_TO") | .target] | unique | length,
    cname_delegations: [.edges[] | select(.type=="ALIAS_OF") | .target] | unique
  }'
```

### Continuous Monitoring with Scheduled Scans

For ongoing infrastructure visibility, schedule SPYDER scans and compare results over time:

```bash
#!/bin/bash
# infrastructure-scan.sh
DATE=$(date +%Y%m%d)
OUTPUT="/var/lib/spyder/scans/infra-${DATE}.json"

./bin/spyder \
  -domains=/etc/spyder/infrastructure-domains.txt \
  -concurrency=256 \
  -probe=infra-monitor \
  -run="infra-${DATE}" \
  -ingest=https://ingest.internal/v1/batch \
  2>/dev/null > "${OUTPUT}"

# Compare with previous scan
PREV=$(ls -t /var/lib/spyder/scans/infra-*.json | sed -n '2p')
if [ -n "$PREV" ]; then
  echo "New IPs:"
  diff <(jq -r '.nodes_ip[].ip' "$PREV" | sort) \
       <(jq -r '.nodes_ip[].ip' "$OUTPUT" | sort) | grep "^>"

  echo "New domains:"
  diff <(jq -r '.nodes_domain[].host' "$PREV" | sort) \
       <(jq -r '.nodes_domain[].host' "$OUTPUT" | sort) | grep "^>"
fi
```

This approach surfaces infrastructure changes (new IPs, new domains, changed DNS providers) between scans, giving you continuous visibility into your external footprint.
