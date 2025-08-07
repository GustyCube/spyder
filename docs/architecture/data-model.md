# SPYDER Data Model

SPYDER creates a graph-based representation of internet infrastructure, modeling entities as nodes and relationships as edges.

## Node Types

### Domain Nodes (`NodeDomain`)

Represents DNS domains and hostnames.

```go
type NodeDomain struct {
    Host      string    `json:"host"`        // Fully qualified domain name
    Apex      string    `json:"apex"`        // Apex/root domain
    FirstSeen time.Time `json:"first_seen"`  // First discovery timestamp
    LastSeen  time.Time `json:"last_seen"`   // Last observation timestamp
}
```

**Examples:**
```json
{
  "host": "www.example.com",
  "apex": "example.com",
  "first_seen": "2024-01-01T12:00:00Z",
  "last_seen": "2024-01-01T12:00:00Z"
}
```

**Key Properties:**
- **Host**: The complete domain name (e.g., `www.example.com`, `cdn.example.org`)
- **Apex**: The registrable domain using public suffix rules (e.g., `example.com` for `www.example.com`)
- **Timestamps**: Track first discovery and last observation for temporal analysis

### IP Address Nodes (`NodeIP`)

Represents IPv4 and IPv6 addresses.

```go
type NodeIP struct {
    IP        string    `json:"ip"`          // IP address (IPv4 or IPv6)
    FirstSeen time.Time `json:"first_seen"`  // First discovery timestamp
    LastSeen  time.Time `json:"last_seen"`   // Last observation timestamp
}
```

**Examples:**
```json
{
  "ip": "93.184.216.34",
  "first_seen": "2024-01-01T12:00:00Z",
  "last_seen": "2024-01-01T12:00:00Z"
}
```

**Key Properties:**
- **IP**: String representation of IPv4 or IPv6 address
- **Format**: IPv4 in dotted decimal, IPv6 in compressed format
- **Deduplication**: Same IP from different sources creates single node

### Certificate Nodes (`NodeCert`)

Represents TLS/SSL certificates and their metadata.

```go
type NodeCert struct {
    SPKI      string    `json:"spki_sha256"` // SHA-256 of Subject Public Key Info
    SubjectCN string    `json:"subject_cn"`  // Subject Common Name
    IssuerCN  string    `json:"issuer_cn"`   // Issuer Common Name
    NotBefore time.Time `json:"not_before"`  // Certificate validity start
    NotAfter  time.Time `json:"not_after"`   // Certificate validity end
}
```

**Examples:**
```json
{
  "spki_sha256": "B7+tPUdz9OYBgGpZFY9V1MyXOsL88K8AJ2m2Jv8YsPM=",
  "subject_cn": "*.example.com",
  "issuer_cn": "Let's Encrypt Authority X3",
  "not_before": "2024-01-01T00:00:00Z",
  "not_after": "2024-04-01T00:00:00Z"
}
```

**Key Properties:**
- **SPKI**: Subject Public Key Info hash for unique identification
- **Subject CN**: Primary domain name on certificate
- **Issuer CN**: Certificate Authority name
- **Validity Period**: Certificate lifetime boundaries

## Edge Types

Edges represent relationships between nodes, capturing different types of internet infrastructure connections.

### DNS Resolution (`RESOLVES_TO`)

Links domains to their resolved IP addresses.

```json
{
  "type": "RESOLVES_TO",
  "source": "example.com",
  "target": "93.184.216.34",
  "observed_at": "2024-01-01T12:00:00Z",
  "probe_id": "us-west-1a",
  "run_id": "run-20240101"
}
```

**Source**: Domain name (NodeDomain)
**Target**: IP address (NodeIP)
**Semantics**: Domain resolves to IP via A/AAAA DNS records

### Nameserver Usage (`USES_NS`)

Links domains to their authoritative nameservers.

```json
{
  "type": "USES_NS",
  "source": "example.com",
  "target": "ns1.example.com",
  "observed_at": "2024-01-01T12:00:00Z",
  "probe_id": "us-west-1a",
  "run_id": "run-20240101"
}
```

**Source**: Domain name (NodeDomain)
**Target**: Nameserver domain (NodeDomain)
**Semantics**: Domain uses nameserver for DNS resolution

### Domain Aliases (`ALIAS_OF`)

Links domains to their CNAME targets.

```json
{
  "type": "ALIAS_OF",
  "source": "www.example.com",
  "target": "example.com",
  "observed_at": "2024-01-01T12:00:00Z",
  "probe_id": "us-west-1a",
  "run_id": "run-20240101"
}
```

**Source**: Aliased domain (NodeDomain)
**Target**: Target domain (NodeDomain)
**Semantics**: Source is a CNAME alias of target

### Mail Exchange (`USES_MX`)

Links domains to their mail servers.

```json
{
  "type": "USES_MX",
  "source": "example.com",
  "target": "mail.example.com",
  "observed_at": "2024-01-01T12:00:00Z",
  "probe_id": "us-west-1a",
  "run_id": "run-20240101"
}
```

**Source**: Domain name (NodeDomain)
**Target**: Mail server domain (NodeDomain)
**Semantics**: Domain uses mail server for email delivery

### External Links (`LINKS_TO`)

Links domains to external domains found in their content.

```json
{
  "type": "LINKS_TO",
  "source": "example.com",
  "target": "partner.com",
  "observed_at": "2024-01-01T12:00:00Z",
  "probe_id": "us-west-1a",
  "run_id": "run-20240101"
}
```

**Source**: Linking domain (NodeDomain)
**Target**: Linked domain (NodeDomain)
**Semantics**: Source domain links to target domain in HTML content

**Link Sources:**
- Hyperlinks (`<a href>`)
- Stylesheets (`<link href>`)
- Scripts (`<script src>`)
- Images (`<img src>`)
- Embedded content (`<iframe src>`)

### Certificate Usage (`USES_CERT`)

Links domains to their TLS certificates.

```json
{
  "type": "USES_CERT",
  "source": "example.com",
  "target": "B7+tPUdz9OYBgGpZFY9V1MyXOsL88K8AJ2m2Jv8YsPM=",
  "observed_at": "2024-01-01T12:00:00Z",
  "probe_id": "us-west-1a",
  "run_id": "run-20240101"
}
```

**Source**: Domain name (NodeDomain)
**Target**: Certificate SPKI hash (NodeCert)
**Semantics**: Domain uses certificate for TLS/SSL

## Batch Structure

Data is emitted in batches containing multiple nodes and edges:

```go
type Batch struct {
    ProbeID string       `json:"probe_id"`     // Probe identifier
    RunID   string       `json:"run_id"`       // Run identifier
    NodesD  []NodeDomain `json:"nodes_domain"` // Domain nodes
    NodesIP []NodeIP     `json:"nodes_ip"`     // IP nodes
    NodesC  []NodeCert   `json:"nodes_cert"`   // Certificate nodes
    Edges   []Edge       `json:"edges"`        // Relationship edges
}
```

### Example Complete Batch

```json
{
  "probe_id": "us-west-1a",
  "run_id": "run-20240101-120000",
  "nodes_domain": [
    {
      "host": "example.com",
      "apex": "example.com",
      "first_seen": "2024-01-01T12:00:00Z",
      "last_seen": "2024-01-01T12:00:00Z"
    },
    {
      "host": "www.example.com",
      "apex": "example.com",
      "first_seen": "2024-01-01T12:00:00Z",
      "last_seen": "2024-01-01T12:00:00Z"
    },
    {
      "host": "ns1.example.com",
      "apex": "example.com",
      "first_seen": "2024-01-01T12:00:00Z",
      "last_seen": "2024-01-01T12:00:00Z"
    }
  ],
  "nodes_ip": [
    {
      "ip": "93.184.216.34",
      "first_seen": "2024-01-01T12:00:00Z",
      "last_seen": "2024-01-01T12:00:00Z"
    }
  ],
  "nodes_cert": [
    {
      "spki_sha256": "B7+tPUdz9OYBgGpZFY9V1MyXOsL88K8AJ2m2Jv8YsPM=",
      "subject_cn": "*.example.com",
      "issuer_cn": "DigiCert Inc",
      "not_before": "2024-01-01T00:00:00Z",
      "not_after": "2025-01-01T00:00:00Z"
    }
  ],
  "edges": [
    {
      "type": "RESOLVES_TO",
      "source": "example.com",
      "target": "93.184.216.34",
      "observed_at": "2024-01-01T12:00:00Z",
      "probe_id": "us-west-1a",
      "run_id": "run-20240101-120000"
    },
    {
      "type": "ALIAS_OF",
      "source": "www.example.com",
      "target": "example.com",
      "observed_at": "2024-01-01T12:00:00Z",
      "probe_id": "us-west-1a",
      "run_id": "run-20240101-120000"
    },
    {
      "type": "USES_NS",
      "source": "example.com",
      "target": "ns1.example.com",
      "observed_at": "2024-01-01T12:00:00Z",
      "probe_id": "us-west-1a",
      "run_id": "run-20240101-120000"
    },
    {
      "type": "USES_CERT",
      "source": "example.com",
      "target": "B7+tPUdz9OYBgGpZFY9V1MyXOsL88K8AJ2m2Jv8YsPM=",
      "observed_at": "2024-01-01T12:00:00Z",
      "probe_id": "us-west-1a",
      "run_id": "run-20240101-120000"
    }
  ]
}
```

## Deduplication Strategy

### Node Deduplication

Nodes are deduplicated based on their primary key:

- **Domains**: By `host` field (case-insensitive)
- **IPs**: By `ip` field (normalized format)
- **Certificates**: By `spki_sha256` field

### Edge Deduplication

Edges are deduplicated using a composite key:
```
edge|{source}|{type}|{target}
```

**Examples:**
- `edge|example.com|RESOLVES_TO|93.184.216.34`
- `edge|www.example.com|ALIAS_OF|example.com`
- `edge|example.com|USES_CERT|B7+tPUdz9OYBgGp...`

### Deduplication Keys

The system generates deduplication keys for tracking:

```go
// Node keys
nodeKey := "domain|" + domain.Host
nodeKey := "nodeip|" + ip.IP
nodeKey := "cert|" + cert.SPKI

// Edge keys
edgeKey := fmt.Sprintf("edge|%s|%s|%s", edge.Source, edge.Type, edge.Target)
```

## Temporal Aspects

### Timestamps

All data includes temporal information:

- **FirstSeen**: When the entity was first observed
- **LastSeen**: When the entity was last observed
- **ObservedAt**: When a relationship was observed

### Time Series Analysis

The data model supports temporal analysis:

- Track domain ownership changes
- Monitor certificate rotation
- Analyze infrastructure evolution
- Detect DNS changes over time

### Data Freshness

Batches include metadata for tracking data age:

- **ProbeID**: Identifies the probe source
- **RunID**: Identifies the specific run
- **Timestamp**: Each observation timestamp

## Graph Properties

### Connectivity

The SPYDER graph exhibits:

- **Scale-free properties**: Few highly connected nodes
- **Small world characteristics**: Short paths between nodes
- **Clustering**: Related infrastructure tends to cluster

### Node Degree Distribution

Typical degree distributions:

- **Domains**: Power-law distribution (few domains have many connections)
- **IPs**: Heavy-tailed distribution (shared hosting creates hubs)
- **Certificates**: Concentrated distribution (wildcard certs serve multiple domains)

### Relationship Semantics

Edge types have different semantic meanings:

- **DNS edges** (`RESOLVES_TO`, `USES_NS`, `USES_MX`): Technical dependencies
- **Content edges** (`LINKS_TO`): Business/content relationships
- **Certificate edges** (`USES_CERT`): Security/trust relationships
- **Alias edges** (`ALIAS_OF`): Identity relationships

## Data Validation

### Input Validation

- Domain names validated against RFC standards
- IP addresses parsed and normalized
- Certificate data validated for completeness

### Relationship Validation

- Source and target nodes must exist
- Edge types must match valid relationships
- Timestamps must be within reasonable ranges

### Schema Enforcement

JSON schema validation ensures:
- Required fields are present
- Data types are correct
- Format constraints are met
- Relationship constraints are satisfied

This data model provides a comprehensive foundation for analyzing internet infrastructure, supporting both real-time analysis and historical trend analysis.