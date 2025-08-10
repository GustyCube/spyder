# DNS Resolution Component

The DNS resolution component (`internal/dns`) provides comprehensive DNS record lookups for domain discovery and relationship mapping.

## Overview

The DNS resolver performs multiple parallel queries to gather all DNS records associated with a domain, enabling SPYDER to build comprehensive maps of domain relationships and infrastructure.

## Core Function

### `ResolveAll(ctx context.Context, host string)`

Performs comprehensive DNS resolution for a given hostname, returning all discoverable DNS records:

**Parameters:**
- `ctx`: Context for timeout and cancellation control
- `host`: The domain name to resolve

**Returns:**
- `ips []string`: A/AAAA records (IPv4 and IPv6 addresses)
- `nsHosts []string`: NS records (nameserver hosts)
- `cname string`: CNAME record (canonical name target)
- `mxHosts []string`: MX records (mail exchanger hosts)
- `txts []string`: TXT records (text records)

## DNS Record Types

### A/AAAA Records (IP Resolution)
- Resolves both IPv4 (A) and IPv6 (AAAA) addresses
- Creates `RESOLVES_TO` edges between domains and IP addresses
- Used for infrastructure mapping and hosting analysis

### NS Records (Nameservers)
- Identifies authoritative nameservers for the domain
- Creates `USES_NS` edges between domains and nameserver hosts
- Critical for DNS infrastructure mapping

### CNAME Records (Canonical Names)
- Discovers canonical name targets for aliases
- Creates `ALIAS_OF` edges between domains and their targets
- Important for CDN and hosting service detection

### MX Records (Mail Exchangers)
- Identifies mail server hosts for the domain
- Creates `USES_MX` edges between domains and mail servers
- Used for email infrastructure analysis

### TXT Records (Text Records)
- Retrieves TXT records for policy and verification analysis
- Currently collected but not used for edge creation
- Useful for SPF, DKIM, and other policy analysis

## Implementation Details

### Error Handling
- All DNS queries are performed with error tolerance
- Failed lookups return empty results without stopping the process
- Uses Go's standard `net` package resolver

### Host Normalization
- Automatically strips trailing dots from DNS responses
- Ensures consistent hostname formatting across the system

### Context Support
- Respects context timeouts and cancellation
- Allows for graceful shutdown during long-running operations

## Integration

The DNS component integrates with the probe pipeline:

1. **Input**: Domain names from the processing queue
2. **Processing**: Parallel DNS record resolution
3. **Output**: Structured DNS data for edge creation

## Performance Considerations

- Uses Go's default resolver with built-in caching
- Performs parallel queries for different record types
- Context-aware for timeout management
- No rate limiting at this level (handled by higher-level components)

## Configuration

DNS resolution behavior is controlled by:
- System DNS configuration (`/etc/resolv.conf`)
- Context timeouts from the probe configuration
- Network interface settings

## Error Scenarios

Common error scenarios handled gracefully:
- NXDOMAIN responses (non-existent domains)
- Timeout conditions
- DNS server unavailability
- Network connectivity issues
- Malformed DNS responses

## Monitoring

DNS resolution metrics are tracked at the probe level:
- Resolution success/failure rates
- Query latency metrics
- Record type distribution
- Error categorization

## Security Considerations

- Uses system DNS resolver (respects local DNS policy)
- No custom DNS server configuration
- Subject to DNS cache poisoning protections of the underlying system
- TXT record collection for security policy analysis (SPF, DMARC)