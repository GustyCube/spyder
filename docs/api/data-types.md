# API Data Types Reference

Complete reference for all data structures used in SPYDER's APIs, including JSON schemas, validation rules, and examples.

## Core Entity Types

### Domain Node (`NodeDomain`)

Represents a domain name entity in the internet infrastructure graph.

#### JSON Schema
```json
{
  "type": "object",
  "required": ["host", "apex", "first_seen", "last_seen"],
  "properties": {
    "host": {
      "type": "string",
      "pattern": "^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\\.([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?))*$",
      "maxLength": 253,
      "description": "Fully qualified domain name"
    },
    "apex": {
      "type": "string", 
      "pattern": "^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\\.([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?))*$",
      "description": "Root/apex domain (e.g., 'example.com' for 'sub.example.com')"
    },
    "first_seen": {
      "type": "string",
      "format": "date-time",
      "description": "ISO 8601 timestamp of first observation"
    },
    "last_seen": {
      "type": "string", 
      "format": "date-time",
      "description": "ISO 8601 timestamp of last observation"
    }
  }
}
```

#### Example
```json
{
  "host": "mail.example.com",
  "apex": "example.com", 
  "first_seen": "2023-12-01T14:30:00Z",
  "last_seen": "2023-12-01T14:30:00Z"
}
```

#### Validation Rules
- **Host Format**: Valid DNS hostname (RFC 1123)
- **Apex Derivation**: Must be valid public suffix + 1 label
- **Timestamp Order**: `first_seen` ≤ `last_seen`
- **Length Limits**: Host ≤ 253 characters, labels ≤ 63 characters

### IP Address Node (`NodeIP`)

Represents an IPv4 or IPv6 address entity.

#### JSON Schema
```json
{
  "type": "object",
  "required": ["ip", "first_seen", "last_seen"],
  "properties": {
    "ip": {
      "type": "string",
      "oneOf": [
        {"format": "ipv4"},
        {"format": "ipv6"}
      ],
      "description": "IPv4 or IPv6 address"
    },
    "first_seen": {
      "type": "string",
      "format": "date-time"
    },
    "last_seen": {
      "type": "string",
      "format": "date-time"
    }
  }
}
```

#### Examples
```json
// IPv4 Example
{
  "ip": "192.0.2.1",
  "first_seen": "2023-12-01T14:30:00Z",
  "last_seen": "2023-12-01T14:30:00Z"
}

// IPv6 Example
{
  "ip": "2001:db8::1",
  "first_seen": "2023-12-01T14:30:00Z", 
  "last_seen": "2023-12-01T14:30:00Z"
}
```

#### Validation Rules
- **IP Format**: Valid IPv4 (RFC 791) or IPv6 (RFC 4291) address
- **Normalization**: IPv6 addresses should be canonical form
- **Private Ranges**: Private/reserved ranges may be filtered
- **Timestamp Order**: `first_seen` ≤ `last_seen`

### Certificate Node (`NodeCert`)

Represents a TLS certificate entity identified by its SPKI hash.

#### JSON Schema
```json
{
  "type": "object",
  "required": ["spki_sha256", "subject_cn", "issuer_cn", "not_before", "not_after"],
  "properties": {
    "spki_sha256": {
      "type": "string",
      "pattern": "^[A-Za-z0-9+/]+=*$",
      "description": "Base64-encoded SHA-256 of Subject Public Key Info"
    },
    "subject_cn": {
      "type": "string",
      "maxLength": 256,
      "description": "Certificate subject common name"
    },
    "issuer_cn": {
      "type": "string", 
      "maxLength": 256,
      "description": "Certificate issuer common name"
    },
    "not_before": {
      "type": "string",
      "format": "date-time",
      "description": "Certificate validity start time"
    },
    "not_after": {
      "type": "string",
      "format": "date-time", 
      "description": "Certificate validity end time"
    }
  }
}
```

#### Example
```json
{
  "spki_sha256": "YLh1dUR9y6Kja30RrAn7JKnbQG/uEtLMkBgFF2Fuihg=",
  "subject_cn": "*.example.com",
  "issuer_cn": "Let's Encrypt Authority X3",
  "not_before": "2023-11-01T00:00:00Z",
  "not_after": "2024-01-29T23:59:59Z"
}
```

#### Validation Rules
- **SPKI Format**: Valid Base64 encoding, 44 characters (32 bytes)
- **CN Length**: Subject/Issuer CN ≤ 256 characters
- **Validity Period**: `not_before` < `not_after`
- **Time Range**: Certificate times within reasonable bounds

## Relationship Edge Type (`Edge`)

Represents directed relationships between entities in the internet infrastructure graph.

### JSON Schema
```json
{
  "type": "object", 
  "required": ["type", "source", "target", "observed_at", "probe_id", "run_id"],
  "properties": {
    "type": {
      "type": "string",
      "enum": ["RESOLVES_TO", "USES_NS", "ALIAS_OF", "USES_MX", "LINKS_TO", "USES_CERT"],
      "description": "Relationship type"
    },
    "source": {
      "type": "string",
      "maxLength": 512,
      "description": "Source entity identifier"
    },
    "target": {
      "type": "string", 
      "maxLength": 512,
      "description": "Target entity identifier"
    },
    "observed_at": {
      "type": "string",
      "format": "date-time",
      "description": "When relationship was observed"
    },
    "probe_id": {
      "type": "string",
      "pattern": "^[a-zA-Z0-9-_]+$",
      "maxLength": 64,
      "description": "Probe instance identifier"
    },
    "run_id": {
      "type": "string",
      "pattern": "^[a-zA-Z0-9-_]+$", 
      "maxLength": 64,
      "description": "Probe run identifier"
    }
  }
}
```

### Edge Type Definitions

#### `RESOLVES_TO` - DNS A/AAAA Records
- **Source**: Domain name (e.g., `example.com`)
- **Target**: IP address (e.g., `192.0.2.1`)
- **Description**: Domain resolves to IP address via A or AAAA record

```json
{
  "type": "RESOLVES_TO",
  "source": "example.com",
  "target": "192.0.2.1", 
  "observed_at": "2023-12-01T14:30:00Z",
  "probe_id": "probe-1",
  "run_id": "run-20231201-143000"
}
```

#### `USES_NS` - DNS NS Records
- **Source**: Domain name (e.g., `example.com`)
- **Target**: Nameserver hostname (e.g., `ns1.example.com`)
- **Description**: Domain uses nameserver for DNS resolution

```json
{
  "type": "USES_NS",
  "source": "example.com", 
  "target": "ns1.example.com",
  "observed_at": "2023-12-01T14:30:00Z",
  "probe_id": "probe-1",
  "run_id": "run-20231201-143000"
}
```

#### `ALIAS_OF` - DNS CNAME Records
- **Source**: Domain name (e.g., `www.example.com`)
- **Target**: Canonical domain name (e.g., `example.com`)
- **Description**: Domain is an alias (CNAME) of target domain

```json
{
  "type": "ALIAS_OF",
  "source": "www.example.com",
  "target": "example.com",
  "observed_at": "2023-12-01T14:30:00Z", 
  "probe_id": "probe-1",
  "run_id": "run-20231201-143000"
}
```

#### `USES_MX` - DNS MX Records  
- **Source**: Domain name (e.g., `example.com`)
- **Target**: Mail server hostname (e.g., `mail.example.com`)
- **Description**: Domain uses mail server for email delivery

```json
{
  "type": "USES_MX",
  "source": "example.com",
  "target": "mail.example.com",
  "observed_at": "2023-12-01T14:30:00Z",
  "probe_id": "probe-1", 
  "run_id": "run-20231201-143000"
}
```

#### `LINKS_TO` - HTML Link Relationships
- **Source**: Domain name (e.g., `example.com`)
- **Target**: External domain name (e.g., `partner.com`)
- **Description**: Source domain contains HTML links to target domain

```json
{
  "type": "LINKS_TO",
  "source": "example.com",
  "target": "partner.com",
  "observed_at": "2023-12-01T14:30:00Z",
  "probe_id": "probe-1",
  "run_id": "run-20231201-143000"
}
```

#### `USES_CERT` - TLS Certificate Usage
- **Source**: Domain name (e.g., `example.com`)
- **Target**: Certificate SPKI hash (e.g., `YLh1dUR9y6Kja30RrAn7JKnbQG/uEtLMkBgFF2Fuihg=`)
- **Description**: Domain presents TLS certificate during handshake

```json
{
  "type": "USES_CERT", 
  "source": "example.com",
  "target": "YLh1dUR9y6Kja30RrAn7JKnbQG/uEtLMkBgFF2Fuihg=",
  "observed_at": "2023-12-01T14:30:00Z",
  "probe_id": "probe-1",
  "run_id": "run-20231201-143000"
}
```

## Batch Container Type (`Batch`)

Container for submitting collections of nodes and edges in a single API call.

### JSON Schema
```json
{
  "type": "object",
  "required": ["probe_id", "run_id", "nodes_domain", "nodes_ip", "nodes_cert", "edges"],
  "properties": {
    "probe_id": {
      "type": "string",
      "pattern": "^[a-zA-Z0-9-_]+$",
      "maxLength": 64,
      "description": "Probe instance identifier"
    },
    "run_id": {
      "type": "string", 
      "pattern": "^[a-zA-Z0-9-_]+$",
      "maxLength": 64,
      "description": "Probe run identifier"  
    },
    "nodes_domain": {
      "type": "array",
      "items": {"$ref": "#/definitions/NodeDomain"},
      "maxItems": 10000,
      "description": "Array of domain nodes"
    },
    "nodes_ip": {
      "type": "array",
      "items": {"$ref": "#/definitions/NodeIP"},
      "maxItems": 10000,
      "description": "Array of IP address nodes"
    },
    "nodes_cert": {
      "type": "array",
      "items": {"$ref": "#/definitions/NodeCert"}, 
      "maxItems": 10000,
      "description": "Array of certificate nodes"
    },
    "edges": {
      "type": "array", 
      "items": {"$ref": "#/definitions/Edge"},
      "maxItems": 50000,
      "description": "Array of relationship edges"
    }
  }
}
```

### Example Complete Batch
```json
{
  "probe_id": "prod-us-west-1",
  "run_id": "run-20231201-143000",
  "nodes_domain": [
    {
      "host": "example.com",
      "apex": "example.com",
      "first_seen": "2023-12-01T14:30:00Z",
      "last_seen": "2023-12-01T14:30:00Z"
    },
    {
      "host": "mail.example.com", 
      "apex": "example.com",
      "first_seen": "2023-12-01T14:30:00Z",
      "last_seen": "2023-12-01T14:30:00Z"
    }
  ],
  "nodes_ip": [
    {
      "ip": "192.0.2.1",
      "first_seen": "2023-12-01T14:30:00Z",
      "last_seen": "2023-12-01T14:30:00Z"
    }
  ],
  "nodes_cert": [
    {
      "spki_sha256": "YLh1dUR9y6Kja30RrAn7JKnbQG/uEtLMkBgFF2Fuihg=",
      "subject_cn": "*.example.com",
      "issuer_cn": "Let's Encrypt Authority X3", 
      "not_before": "2023-11-01T00:00:00Z",
      "not_after": "2024-01-29T23:59:59Z"
    }
  ],
  "edges": [
    {
      "type": "RESOLVES_TO",
      "source": "example.com",
      "target": "192.0.2.1",
      "observed_at": "2023-12-01T14:30:00Z",
      "probe_id": "prod-us-west-1",
      "run_id": "run-20231201-143000"
    },
    {
      "type": "USES_CERT",
      "source": "example.com",
      "target": "YLh1dUR9y6Kja30RrAn7JKnbQG/uEtLMkBgFF2Fuihg=",
      "observed_at": "2023-12-01T14:30:00Z", 
      "probe_id": "prod-us-west-1",
      "run_id": "run-20231201-143000"
    }
  ]
}
```

### Validation Rules
- **Batch Size**: Maximum 50,000 edges and 10,000 nodes per type
- **Consistency**: All edges must reference valid probe_id and run_id
- **Entity References**: Edge source/target must correspond to included or existing entities
- **Timestamp Consistency**: Batch timestamps should be within reasonable range

## Common Patterns

### Timestamp Format
All timestamps use ISO 8601 format with UTC timezone:
```
YYYY-MM-DDTHH:MM:SSZ
```

### Identifier Patterns
- **Probe ID**: `^[a-zA-Z0-9-_]+$` (alphanumeric, hyphen, underscore)
- **Run ID**: `^[a-zA-Z0-9-_]+$` (alphanumeric, hyphen, underscore)  
- **Domain Names**: RFC 1123 compliant hostnames
- **IP Addresses**: RFC 791 (IPv4) or RFC 4291 (IPv6) compliant

### Size Limits
- **Domain Names**: 253 characters maximum
- **IP Addresses**: Standard IPv4/IPv6 format limits
- **Certificate CN**: 256 characters maximum
- **SPKI Hash**: 44 characters (Base64 of 32 bytes)
- **Batch Items**: 50,000 edges, 10,000 nodes per type