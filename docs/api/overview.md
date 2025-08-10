# API Reference Overview

SPYDER provides comprehensive APIs for data ingestion, queue management, and system integration in distributed internet mapping deployments.

## API Categories

### Data Ingestion API
The primary interface for receiving processed domain relationship data from SPYDER probes.

- **Batch Ingestion**: Efficient bulk data submission
- **Real-time Processing**: Low-latency data pipeline integration
- **Reliability**: Built-in retry logic and error handling
- **Authentication**: mTLS and API key support

### Queue Management API  
Redis-based distributed work queue for coordinating probe instances.

- **Work Distribution**: Automatic load balancing across probes
- **Lease Management**: Reliable work item processing
- **Failure Recovery**: Automatic retry and dead letter handling
- **Monitoring**: Queue depth and processing metrics

### Health Check API
System health and readiness endpoints for operational monitoring.

- **Liveness Probes**: Service availability checks
- **Readiness Probes**: Service capability verification  
- **Dependency Checks**: External service connectivity status
- **Metrics Exposure**: Prometheus-compatible metrics

## Data Model Overview

### Entity Types

#### Nodes
- **Domain Nodes**: Hostnames and apex domains
- **IP Nodes**: IPv4 and IPv6 addresses  
- **Certificate Nodes**: TLS certificate metadata

#### Edges
Directed relationships between entities:
- `RESOLVES_TO`: Domain → IP (DNS A/AAAA)
- `USES_NS`: Domain → Nameserver (DNS NS) 
- `ALIAS_OF`: Domain → CNAME target
- `USES_MX`: Domain → Mail server (DNS MX)
- `LINKS_TO`: Domain → External domain (HTML links)
- `USES_CERT`: Domain → Certificate (TLS handshake)

### Batch Structure
```json
{
  "probe_id": "prod-us-west-1",
  "run_id": "run-20231201-143000", 
  "nodes_domain": [...],
  "nodes_ip": [...],
  "nodes_cert": [...],
  "edges": [...]
}
```

## Authentication Methods

### Mutual TLS (mTLS)
Recommended for production deployments:
- **Client Certificates**: X.509 client certificate authentication
- **Certificate Validation**: Server validates client certificate chain
- **Encryption**: All data encrypted in transit
- **Non-Repudiation**: Strong identity verification

### API Key Authentication
Alternative for development and testing:
- **Header-Based**: `Authorization: Bearer <api-key>`
- **Query Parameter**: `?api_key=<api-key>` (not recommended)
- **Rate Limiting**: Per-key request throttling
- **Revocation**: Immediate key deactivation capability

## Base URLs

### Production Environment
```
https://api.spyder.example.com/v1/
```

### Development Environment  
```
https://api-dev.spyder.example.com/v1/
```

### Local Development
```
http://localhost:8080/v1/
```

## Common Headers

### Required Headers
```http
Content-Type: application/json
User-Agent: spyder-probe/1.0
```

### Authentication Headers
```http
# mTLS (automatic with client certificates)
# OR API Key
Authorization: Bearer your-api-key-here
```

### Optional Headers
```http
X-Request-ID: unique-request-identifier
X-Probe-Version: 1.2.3
X-Run-ID: run-20231201-143000
```

## Response Formats

### Success Response
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "status": "success",
  "message": "Batch processed successfully",
  "batch_id": "batch-20231201-143001-001",
  "items_processed": 1234,
  "processing_time_ms": 450
}
```

### Error Response
```http
HTTP/1.1 400 Bad Request  
Content-Type: application/json

{
  "status": "error",
  "error_code": "INVALID_BATCH_FORMAT",
  "message": "Invalid JSON in request body",
  "details": {
    "line": 42,
    "column": 15,
    "expected": "string"
  },
  "request_id": "req-20231201-143001-abc"
}
```

## Rate Limits

### Default Limits
- **Batch API**: 100 requests/minute per client
- **Queue API**: 1000 requests/minute per client  
- **Health API**: No limits (monitoring use)

### Rate Limit Headers
```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 87
X-RateLimit-Reset: 1640995260
```

### Rate Limit Exceeded
```http
HTTP/1.1 429 Too Many Requests
Retry-After: 60

{
  "status": "error", 
  "error_code": "RATE_LIMIT_EXCEEDED",
  "message": "Rate limit exceeded, retry after 60 seconds"
}
```

## Error Codes

### Client Errors (4xx)
- **`INVALID_REQUEST`**: Malformed request structure
- **`AUTHENTICATION_REQUIRED`**: Missing or invalid authentication  
- **`AUTHORIZATION_DENIED`**: Insufficient permissions
- **`INVALID_BATCH_FORMAT`**: JSON parsing errors
- **`VALIDATION_FAILED`**: Data validation errors
- **`RATE_LIMIT_EXCEEDED`**: Too many requests

### Server Errors (5xx)  
- **`INTERNAL_ERROR`**: Unexpected server error
- **`DATABASE_UNAVAILABLE`**: Backend storage issues
- **`PROCESSING_TIMEOUT`**: Batch processing timeout
- **`SERVICE_UNAVAILABLE`**: Temporary service disruption

## Pagination

APIs returning large result sets support cursor-based pagination:

### Request Parameters
```http
GET /v1/batches?cursor=eyJpZCI6MTIzNH0&limit=100
```

### Response Format
```json
{
  "data": [...],
  "pagination": {
    "next_cursor": "eyJpZCI6MTMzOH0",
    "has_more": true,
    "total_count": 5420
  }
}
```

## Versioning

### API Versioning Strategy
- **URL Versioning**: `/v1/`, `/v2/` in path
- **Backward Compatibility**: v1 supported for 12 months after v2 release
- **Version Headers**: Optional `Accept: application/vnd.spyder.v1+json`

### Deprecation Process
1. **Announcement**: 6 months advance notice
2. **Warning Headers**: `Warning: 299 - "Deprecated API version"`
3. **Migration Period**: 12 months parallel support
4. **Sunset**: Complete removal of deprecated version

## SDKs and Libraries

### Official SDKs
- **Go**: `go get github.com/gustycube/spyder-go-sdk`
- **Python**: `pip install spyder-client`
- **JavaScript**: `npm install @spyder/client`

### Community Libraries
- **Java**: Third-party Maven packages available
- **Ruby**: Community-maintained gem
- **PHP**: Composer packages

## Support and Resources

### Documentation
- **API Reference**: Detailed endpoint documentation
- **Data Model**: Complete schema definitions  
- **Integration Guide**: Step-by-step implementation
- **Examples**: Code samples in multiple languages

### Community Resources
- **GitHub Issues**: Bug reports and feature requests
- **Discussion Forums**: Implementation questions and best practices
- **Stack Overflow**: Tagged questions with `spyder-api`

### Enterprise Support
- **Technical Support**: Dedicated support engineers
- **SLA Guarantees**: Response time commitments
- **Custom Integrations**: Professional services available
- **Training Programs**: API integration workshops