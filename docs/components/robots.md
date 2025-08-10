# Robots.txt Component

The robots.txt component (`internal/robots`) provides policy-aware web crawling by respecting robots.txt directives and implementing TLD exclusion policies.

## Overview

The robots.txt component ensures SPYDER operates as a respectful web crawler by fetching, caching, and enforcing robots.txt policies. It includes TLD-based exclusion and efficient caching for high-volume operations.

## Core Structure

### `Cache`

Main robots.txt caching and retrieval system:

```go
type Cache struct {
    hc  *http.Client                                    // HTTP client for fetching robots.txt
    lru *expirable.LRU[string, *robotstxt.RobotsData] // LRU cache with 24-hour expiration
    ua  string                                         // User-Agent string for requests
}
```

## Core Functions

### `NewCache(hc *http.Client, ua string) *Cache`

Creates a new robots.txt cache with optimized settings.

**Parameters:**
- `hc`: HTTP client for robots.txt retrieval
- `ua`: User-Agent string to identify SPYDER

**Returns:**
- `*Cache`: Configured robots.txt cache

**Configuration:**
- **Cache Size**: 4,096 entries maximum
- **Expiration**: 24-hour TTL for robots.txt data
- **LRU Eviction**: Automatic removal of least recently used entries

### `Get(ctx context.Context, host string) (*robotstxt.RobotsData, error)`

Retrieves and caches robots.txt data for a host.

**Parameters:**
- `ctx`: Context for timeout and cancellation control
- `host`: Target hostname for robots.txt retrieval

**Returns:**
- `*robotstxt.RobotsData`: Parsed robots.txt data
- `error`: Retrieval error (always returns nil due to graceful fallback)

**Retrieval Process:**
1. **Cache Check**: Returns cached data if available and not expired
2. **HTTPS First**: Attempts `https://host/robots.txt`
3. **HTTP Fallback**: Falls back to `http://host/robots.txt`
4. **404 Handling**: Treats 404 as empty robots.txt (allows all)
5. **Error Fallback**: Treats errors as empty robots.txt

### `Allowed(rd *robotstxt.RobotsData, ua, path string) bool`

Checks if a path is allowed for a given User-Agent.

**Parameters:**
- `rd`: Parsed robots.txt data
- `ua`: User-Agent string to match against
- `path`: URL path to check permission for

**Returns:**
- `bool`: `true` if access is allowed, `false` if disallowed

**Permission Logic:**
1. **Specific User-Agent**: Matches exact User-Agent if found
2. **Wildcard Fallback**: Falls back to `*` (all crawlers) rules
3. **No Rules Found**: Allows access by default
4. **Rule Testing**: Uses robotstxt library for directive evaluation

### `ShouldSkipByTLD(host string, excluded []string) bool`

Checks if a host should be skipped based on TLD exclusion policy.

**Parameters:**
- `host`: Target hostname to evaluate
- `excluded`: Array of TLD suffixes to exclude

**Returns:**
- `bool`: `true` if host should be skipped, `false` otherwise

**Exclusion Logic:**
- **Suffix Matching**: Matches exact TLD suffixes (e.g., `.gov`, `.mil`)
- **Exact Matching**: Handles exact domain matches
- **Case Sensitive**: Performs case-sensitive TLD matching

## Robots.txt Protocol Implementation

### Standard Compliance
- **RFC 9309**: Implements standard robots.txt protocol
- **User-Agent Matching**: Supports specific and wildcard User-Agent directives
- **Directive Support**: Handles `Allow`, `Disallow`, `Crawl-delay`, `Sitemap`

### Parsing Features
- **Flexible Parsing**: Uses `temoto/robotstxt` library for robust parsing
- **Syntax Tolerance**: Handles malformed robots.txt gracefully
- **Comment Support**: Ignores comments and blank lines appropriately

## Caching Strategy

### LRU Cache Implementation
- **Memory Efficient**: Expirable LRU with automatic cleanup
- **Size Limits**: 4,096 hosts maximum to prevent memory exhaustion
- **Time Limits**: 24-hour expiration for robots.txt compliance

### Cache Benefits
- **Performance**: Avoids repeated robots.txt fetches for same host
- **Network Efficiency**: Reduces HTTP requests to target servers
- **Respectful Operation**: Minimizes load on target servers

## TLD Exclusion Policy

### Security-Sensitive TLDs
Common exclusions for security and compliance:
- **`.gov`**: Government domains
- **`.mil`**: Military domains
- **`.edu`**: Educational institutions (optional)
- **Country-specific**: Sensitive national TLDs

### Configuration Examples
```go
excluded := []string{"gov", "mil", "int"}
if robots.ShouldSkipByTLD("example.gov", excluded) {
    // Skip this domain
}
```

## Integration Points

### Probe Pipeline Integration
1. **Pre-Request Check**: Verify robots.txt permission before HTTP requests
2. **Path Validation**: Check specific URL paths against robots.txt rules
3. **TLD Filtering**: Apply TLD exclusion policy before processing

### HTTP Client Integration
- Uses probe's HTTP client for robots.txt retrieval
- Respects timeout and cancellation contexts
- Integrates with circuit breaker and rate limiting

## Error Handling Philosophy

### Graceful Fallback Strategy
- **Permissive by Default**: Unknown states allow access
- **No Blocking Errors**: Always returns a result, never blocks on errors
- **Conservative Interpretation**: Prefers allowing access over blocking

### Error Scenarios
- **Network Failures**: Treats as "allow all" policy
- **Invalid robots.txt**: Parses successfully with permissive defaults
- **HTTP Errors**: Non-2xx/404 responses treated as "allow all"

## Performance Considerations

### Memory Usage
- **Bounded Cache**: 4,096 entry limit prevents unbounded growth
- **Automatic Expiration**: 24-hour TTL prevents stale data accumulation
- **Efficient Storage**: Only stores parsed robots.txt data, not raw content

### Network Efficiency
- **Protocol Fallback**: HTTPS first, then HTTP for compatibility
- **Single Request**: One request per host per 24-hour period (cached)
- **Timeout Respect**: Honors context timeouts for responsiveness

## Security Features

### Privacy Protection
- **TLD Exclusion**: Prevents access to sensitive domains
- **User-Agent Honesty**: Uses identifiable User-Agent string
- **Policy Compliance**: Strictly enforces robots.txt directives

### Abuse Prevention
- **Rate Integration**: Works with rate limiting for request throttling
- **Cache Limits**: Prevents memory exhaustion attacks
- **Fallback Security**: Defaults to permissive rather than failing open

## Common Use Cases

### Standard Permission Check
```go
cache := robots.NewCache(httpClient, "spyder-bot/1.0")
robotsData, _ := cache.Get(ctx, "example.com")
allowed := robots.Allowed(robotsData, "spyder-bot/1.0", "/page.html")
```

### TLD-Based Filtering
```go
excluded := []string{"gov", "mil", "int"}
if !robots.ShouldSkipByTLD("example.com", excluded) {
    // Proceed with domain processing
}
```

### Policy-Aware Crawling
```go
// Check both TLD policy and robots.txt
if robots.ShouldSkipByTLD(host, excludedTLDs) {
    return // Skip entirely
}
robotsData, _ := cache.Get(ctx, host)
if !robots.Allowed(robotsData, userAgent, path) {
    return // Robots.txt disallows
}
// Proceed with request
```

## Configuration Recommendations

### User-Agent Selection
- **Identifiable**: Use descriptive User-Agent like `spyder-probe/1.0`
- **Contact Information**: Include contact email in User-Agent
- **Version Tracking**: Include version for robots.txt debugging

### Exclusion Policies
- **Security Domains**: Always exclude `.gov`, `.mil`
- **Educational Domains**: Consider excluding `.edu` based on use case
- **Regional Policies**: Add country-specific TLDs as needed

## Monitoring and Metrics

### Key Metrics to Track
- **Cache Hit Rate**: Percentage of robots.txt served from cache
- **Permission Allow Rate**: Percentage of paths allowed by robots.txt
- **TLD Skip Rate**: Percentage of domains skipped due to TLD policy
- **Fetch Success Rate**: Success rate of robots.txt retrieval

### Debugging Information
- **Cache Statistics**: Size, hit rate, eviction rate
- **Permission Denials**: Hosts and paths blocked by robots.txt
- **TLD Exclusions**: Domains skipped due to TLD policy
- **Fetch Failures**: Failed robots.txt retrieval attempts

## Best Practices

### Respectful Crawling
- **Honor Crawl-Delay**: Implement crawl-delay directive support
- **Respect Disallow**: Never access explicitly disallowed paths
- **User-Agent Consistency**: Use same User-Agent for robots.txt and content requests

### Operational Excellence
- **Cache Monitoring**: Monitor cache performance and hit rates
- **Policy Updates**: Regularly review and update TLD exclusion lists
- **Error Analysis**: Analyze robots.txt fetch failures for patterns

### Compliance Considerations
- **Legal Compliance**: TLD exclusions help meet legal requirements
- **Terms of Service**: Robots.txt compliance often required by ToS
- **Ethical Crawling**: Demonstrates responsible internet citizenship