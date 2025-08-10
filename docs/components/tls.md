# TLS Analysis Component

The TLS analysis component (`internal/tlsinfo`) provides secure TLS certificate inspection and analysis for domain security mapping.

## Overview

The TLS analysis component establishes secure connections to domains and extracts certificate metadata, enabling SPYDER to map TLS infrastructure relationships and certificate usage patterns across domains.

## Core Function

### `FetchCert(host string) (*emit.NodeCert, error)`

Establishes a TLS connection and extracts certificate information from the presented certificate chain.

**Parameters:**
- `host`: The domain name to connect to for certificate inspection

**Returns:**
- `*emit.NodeCert`: Certificate node containing metadata, or `nil` if no certificate
- `error`: Connection or parsing error, if any

## TLS Connection Process

### Secure Connection Establishment
1. **TLS Dialer Configuration**: Uses `tls.Dialer` with proper ServerName indication (SNI)
2. **Connection Target**: Connects to host on port 443 (HTTPS)
3. **Timeout Control**: 8-second connection timeout for responsiveness
4. **Certificate Extraction**: Retrieves leaf certificate from peer chain

### Certificate Analysis
- **Subject Public Key Info (SPKI) Hashing**: SHA-256 hash of the certificate's public key
- **Common Name Extraction**: Subject and Issuer Common Names
- **Validity Period**: Certificate not-before and not-after timestamps
- **Base64 Encoding**: SPKI hash encoded for JSON serialization

## Certificate Node Structure

### `emit.NodeCert` Fields

```go
type NodeCert struct {
    SPKI      string    `json:"spki_sha256"`    // Base64-encoded SHA-256 of SPKI
    SubjectCN string    `json:"subject_cn"`     // Certificate subject common name
    IssuerCN  string    `json:"issuer_cn"`      // Certificate issuer common name
    NotBefore time.Time `json:"not_before"`     // Certificate valid from date
    NotAfter  time.Time `json:"not_after"`      // Certificate valid until date
}
```

### SPKI Hash Generation
- **Algorithm**: SHA-256 of `RawSubjectPublicKeyInfo`
- **Encoding**: Base64 standard encoding
- **Purpose**: Unique certificate identification and relationship mapping

## Edge Creation

TLS certificate analysis creates `USES_CERT` edges:
- **Source**: Domain name (host)
- **Target**: Certificate SPKI hash
- **Purpose**: Map which domains use which certificates

## Security Features

### Certificate Verification
- **SNI Support**: Proper Server Name Indication for virtual hosting
- **Certificate Chain**: Analyzes presented certificate chain
- **Leaf Certificate Focus**: Extracts data from the leaf (end-entity) certificate

### Connection Security
- **Standard TLS**: Uses Go's standard TLS implementation
- **Secure Defaults**: Relies on system certificate verification policies
- **Timeout Protection**: Prevents hanging on unresponsive servers

## Error Handling

### Connection Errors
- **Network Unreachable**: Host not accessible on port 443
- **TLS Handshake Failure**: Invalid certificate or TLS configuration
- **Timeout**: Connection establishment exceeds 8-second limit
- **DNS Resolution**: Host name resolution failures

### Certificate Errors
- **Missing Certificate**: Server presents no certificate (returns `nil`)
- **Empty Chain**: Peer certificates list is empty
- **Invalid Certificate**: Malformed certificate data

## Integration Points

### Probe Pipeline Integration
1. **Input**: Domain names from the processing queue
2. **Processing**: TLS connection and certificate analysis
3. **Output**: Certificate nodes and USES_CERT edges

### Deduplication Integration
- SPKI hashes serve as unique certificate identifiers
- Multiple domains using the same certificate share the same SPKI
- Efficient deduplication of certificate data

## Performance Considerations

### Connection Management
- **Individual Connections**: Each certificate fetch uses a new connection
- **Timeout Control**: 8-second limit prevents slow connections from blocking pipeline
- **Context Cancellation**: Supports graceful shutdown

### Resource Usage
- **Memory Efficient**: Extracts only necessary certificate metadata
- **No Certificate Storage**: Does not store full certificate data
- **Hash-Based Identification**: Compact certificate fingerprinting

## Common Use Cases

### Certificate Mapping
- **Shared Certificates**: Identify domains using the same TLS certificate
- **CDN Detection**: Map Content Delivery Network certificate usage
- **Certificate Authority Analysis**: Track issuer relationships

### Security Analysis
- **Certificate Validity**: Monitor certificate expiration dates
- **Subject Analysis**: Identify certificate subject patterns
- **Issuer Tracking**: Map Certificate Authority relationships

### Infrastructure Discovery
- **Load Balancer Detection**: Identify shared certificate infrastructure
- **Hosting Provider Analysis**: Map hosting service certificate patterns
- **Multi-Domain Certificates**: Discover SAN certificate usage

## Configuration Considerations

### Network Configuration
- **Firewall Rules**: Ensure outbound HTTPS (port 443) connectivity
- **Proxy Settings**: Configure for environments requiring HTTP proxies
- **DNS Resolution**: Reliable DNS resolution required for host connections

### TLS Configuration
- **Certificate Verification**: Uses system certificate store for verification
- **Protocol Versions**: Supports modern TLS protocol versions
- **Cipher Suites**: Uses Go's secure default cipher suite selection

## Monitoring Metrics

TLS analysis generates metrics for:
- **Connection Success Rate**: Percentage of successful TLS connections
- **Certificate Extraction Rate**: Rate of valid certificate retrieval
- **Connection Latency**: Time to establish TLS connection and extract certificate
- **Error Classification**: Types and frequency of TLS/certificate errors

## Security Considerations

### Certificate Validation
- **Chain Verification**: Validates certificate chain integrity
- **Expiration Checks**: Identifies expired or soon-to-expire certificates
- **Revocation Status**: Uses system-level revocation checking

### Privacy and Compliance
- **No Certificate Storage**: Only metadata extracted and stored
- **SPKI Fingerprinting**: Certificates identified by public key hash
- **No Private Key Access**: Only public certificate information accessed

## Troubleshooting

### Common Issues
- **Port 443 Blocked**: Firewall or network policy blocking HTTPS
- **SNI Issues**: Server not properly configured for virtual hosting
- **Certificate Errors**: Invalid, expired, or self-signed certificates
- **Timeout Issues**: Slow servers or network connectivity problems

### Debugging Steps
1. **Network Connectivity**: Verify host reachability on port 443
2. **Manual TLS Test**: Use `openssl s_client` to test TLS connectivity
3. **Certificate Inspection**: Examine certificate chain manually
4. **DNS Resolution**: Verify hostname resolves correctly