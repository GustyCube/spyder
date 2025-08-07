# Security Research Use Cases

SPYDER provides valuable capabilities for cybersecurity professionals, researchers, and organizations to map internet infrastructure and identify security-relevant relationships.

## Threat Intelligence Applications

### Infrastructure Mapping

**Adversary Infrastructure Discovery:**

SPYDER can help map threat actor infrastructure by analyzing domain relationships:

```bash
# Map suspected threat actor domains
echo "suspicious-domain.com" > threat-domains.txt
./bin/spyder -domains=threat-domains.txt -concurrency=64
```

**Key Relationships to Analyze:**
- **DNS Resolution**: Shared IP addresses indicate common hosting
- **Certificate Usage**: Shared certificates suggest common operators  
- **Nameserver Patterns**: Common DNS providers may indicate relationships
- **Link Analysis**: Cross-references between malicious sites

**Example Analysis:**
```json
{
  "edges": [
    {
      "type": "RESOLVES_TO",
      "source": "malware-c2.example",
      "target": "192.168.1.100",
      "observed_at": "2024-01-01T12:00:00Z"
    },
    {
      "type": "RESOLVES_TO", 
      "source": "phishing-site.example",
      "target": "192.168.1.100",
      "observed_at": "2024-01-01T12:05:00Z"
    }
  ]
}
```

*Analysis: Both domains resolve to the same IP, suggesting common hosting or operator.*

### Certificate Transparency Analysis

**Shared Certificate Detection:**

```bash
# Analyze certificate relationships
./bin/spyder -domains=target-domains.txt -exclude_tlds=gov,mil
```

**Certificate Intelligence Use Cases:**
- **Wildcard Certificate Abuse**: Detect unauthorized use of legitimate wildcards
- **Certificate Authority Patterns**: Identify preferred CAs of threat actors
- **Certificate Timing**: Analyze certificate issuance patterns
- **Subject Alternative Names**: Discover additional domains on certificates

**Example Certificate Analysis:**
```json
{
  "nodes_cert": [
    {
      "spki_sha256": "B7+tPUdz9OYBgGpZFY9V1MyXOsL88K8AJ2m2Jv8YsPM=",
      "subject_cn": "*.suspicious-domain.com",
      "issuer_cn": "Let's Encrypt Authority X3",
      "not_before": "2024-01-01T00:00:00Z",
      "not_after": "2024-04-01T00:00:00Z"
    }
  ]
}
```

### Typosquatting Detection

**Domain Similarity Analysis:**

SPYDER can help identify typosquatting by analyzing domains that:
- Resolve to similar IP ranges
- Use similar certificate patterns
- Share nameserver infrastructure
- Cross-reference each other

**Implementation:**
```bash
# Generate typosquat candidates
python3 generate-typosquats.py target-company.com > typosquats.txt

# Analyze with SPYDER
./bin/spyder -domains=typosquats.txt -ua="SecurityResearch/1.0"
```

## Attack Surface Analysis

### External Asset Discovery

**Organization Asset Mapping:**

```bash
# Map organization's external infrastructure
echo -e "company.com\nsubsidiary.com\nacquisition.com" > org-domains.txt
./bin/spyder -domains=org-domains.txt -concurrency=128
```

**Asset Types Discovered:**
- **Subdomains**: Through certificate SAN analysis
- **CDN Usage**: Via CNAME and link analysis  
- **Third-party Services**: Through external links
- **Email Infrastructure**: Via MX record analysis
- **DNS Providers**: Through NS record analysis

### Shadow IT Detection

**Unauthorized Service Discovery:**

SPYDER can identify shadow IT by analyzing:
- Unexpected external links from corporate domains
- Certificate issuance to unauthorized subdomains
- DNS patterns indicating cloud service usage

**Analysis Queries:**
```bash
# Look for cloud service indicators
grep -E "(amazonaws|azure|googleapis)" spyder-output.json

# Find unexpected certificate authorities
jq '.nodes_cert[] | select(.issuer_cn | contains("DigiCert"))' spyder-output.json
```

## Incident Response

### Compromise Assessment

**Infrastructure Relationship Analysis:**

During incident response, SPYDER helps identify:
- **Lateral Infrastructure**: Domains sharing resources with compromised assets
- **Command & Control**: External domains linked from compromised sites
- **Data Exfiltration**: Unusual external connections

**Rapid Assessment:**
```bash
# Quick assessment of compromised domain
echo "compromised-domain.com" > incident-domains.txt
./bin/spyder -domains=incident-domains.txt -concurrency=32 -batch_flush_sec=1
```

### Attribution Analysis

**Infrastructure Correlation:**

```json
{
  "analysis": {
    "shared_ip_clusters": [
      {
        "ip": "185.220.101.x",
        "domains": ["site1.com", "site2.com", "site3.com"],
        "pattern": "bulletproof_hosting"
      }
    ],
    "certificate_patterns": [
      {
        "issuer": "Let's Encrypt",
        "timing": "batch_issued_2024_01_01",
        "domains": ["domain1.com", "domain2.com"]
      }
    ]
  }
}
```

## Vulnerability Research

### Attack Path Discovery

**Infrastructure Dependency Mapping:**

SPYDER helps identify attack paths by mapping:
- **DNS Dependencies**: Critical nameserver relationships
- **Certificate Dependencies**: Shared PKI infrastructure
- **Content Dependencies**: External resource loading

**Example Attack Path Analysis:**
```bash
# Map critical infrastructure dependencies
./bin/spyder -domains=critical-services.txt -ua="VulnResearch/1.0"

# Analyze output for single points of failure
jq '.edges[] | select(.type=="USES_NS")' output.json | \
  jq -r '.target' | sort | uniq -c | sort -nr
```

### Supply Chain Analysis

**Third-party Dependency Mapping:**

```bash
# Analyze third-party dependencies
./bin/spyder -domains=target-domains.txt | \
  jq '.edges[] | select(.type=="LINKS_TO")' | \
  jq -r '.target' | sort | uniq > third-party-deps.txt
```

**Supply Chain Risk Indicators:**
- High-risk hosting providers
- Deprecated or vulnerable services
- Concentration of dependencies
- Geographic risk factors

## Threat Hunting

### Behavioral Analysis

**Pattern-based Hunting:**

SPYDER data enables hunting for suspicious patterns:

```python
# Example: Hunt for fast-flux patterns
import json

def analyze_dns_patterns(spyder_data):
    ip_to_domains = {}
    for edge in spyder_data['edges']:
        if edge['type'] == 'RESOLVES_TO':
            ip = edge['target']
            domain = edge['source']
            if ip not in ip_to_domains:
                ip_to_domains[ip] = []
            ip_to_domains[ip].append(domain)
    
    # Look for IPs with many domains (potential fast-flux)
    suspicious_ips = {ip: domains for ip, domains in ip_to_domains.items() 
                     if len(domains) > 10}
    
    return suspicious_ips
```

### IOC Expansion

**Infrastructure Pivoting:**

```bash
# Start with known IOC and expand
echo "known-bad-domain.com" > seed-iocs.txt
./bin/spyder -domains=seed-iocs.txt

# Extract related infrastructure
jq '.edges[] | select(.source=="known-bad-domain.com")' output.json | \
  jq -r '.target' > expanded-iocs.txt
```

## Compliance and Risk Assessment

### Regulatory Compliance

**Infrastructure Auditing:**

SPYDER helps with compliance by:
- Mapping data flow paths through DNS analysis
- Identifying third-party processors via link analysis
- Documenting certificate management practices
- Tracking geographic distribution of infrastructure

**GDPR Compliance Example:**
```bash
# Map EU vs non-EU infrastructure
./bin/spyder -domains=eu-domains.txt | \
  python3 analyze-geographic-distribution.py
```

### Risk Quantification

**Infrastructure Risk Scoring:**

```python
def calculate_risk_score(spyder_data):
    risk_factors = {
        'shared_hosting': 0.3,
        'weak_certificates': 0.4, 
        'suspicious_links': 0.5,
        'geographic_risk': 0.2
    }
    
    # Analyze SPYDER data for risk factors
    # Return risk score
```

## Research and Analytics

### Internet Measurement Studies

**Large-scale Infrastructure Studies:**

```bash
# Analyze top 1M domains (requires significant resources)
./bin/spyder -domains=top1m-domains.txt \
  -concurrency=1024 \
  -batch_max_edges=50000 \
  -ingest=https://research-ingest.university.edu/v1/batch
```

**Research Applications:**
- Certificate authority ecosystem analysis
- CDN adoption and geographic distribution
- DNS resolver diversity studies
- TLS configuration trends

### Competitive Intelligence

**Market Analysis:**

SPYDER can support competitive intelligence by analyzing:
- Technology stack choices (inferred from infrastructure)
- Geographic expansion patterns
- Third-party service adoption
- Security posture indicators

## Ethical and Legal Considerations

### Responsible Use

**Best Practices:**
- Respect robots.txt directives (SPYDER does this automatically)
- Use appropriate rate limiting to avoid impact
- Include contact information in User-Agent strings
- Document legitimate research purposes

**Example Responsible Configuration:**
```bash
./bin/spyder \
  -domains=research-domains.txt \
  -ua="SecurityResearch/1.0 (+https://university.edu/security-research)" \
  -concurrency=64 \
  -exclude_tlds=gov,mil,int,edu
```

### Data Handling

**Privacy Protection:**
- Limit data collection to necessary infrastructure metadata
- Implement data retention policies
- Secure storage of collected intelligence
- Anonymize data when sharing research results

### Legal Compliance

**Jurisdiction Considerations:**
- Comply with local computer crime laws
- Respect intellectual property rights
- Consider export control regulations
- Document legitimate research purposes

## Case Studies

### Case Study 1: APT Infrastructure Mapping

**Scenario:** Cybersecurity team needs to map Advanced Persistent Threat (APT) infrastructure.

**Approach:**
1. Seed SPYDER with known APT domains from threat intelligence
2. Analyze DNS, certificate, and link relationships
3. Identify infrastructure clusters and patterns
4. Generate additional IOCs for blocking/monitoring

**Results:**
- Discovered 15 additional domains sharing infrastructure
- Identified 3 distinct IP ranges used by the threat actor
- Found common certificate issuer patterns
- Enabled proactive blocking of future campaigns

### Case Study 2: Supply Chain Risk Assessment

**Scenario:** Enterprise needs to assess third-party dependencies for critical web applications.

**Approach:**
1. Map all external dependencies using SPYDER link analysis
2. Analyze certificate and hosting patterns of dependencies
3. Assess geographic and regulatory risk factors
4. Prioritize high-risk dependencies for review

**Results:**
- Identified 200+ third-party dependencies
- Found 12 dependencies using high-risk hosting
- Discovered 3 critical dependencies with weak security posture
- Enabled targeted security reviews and supplier assessments

These security use cases demonstrate SPYDER's value in supporting various cybersecurity objectives while maintaining ethical and responsible research practices.