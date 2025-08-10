# Link Extraction Component

The link extraction component (`internal/extract`) analyzes HTML content to discover external domain relationships and build comprehensive domain maps.

## Overview

The link extraction component parses HTML documents to identify all external links, creating `LINKS_TO` edges that map inter-domain relationships across the web. It provides domain apex resolution and efficient external domain filtering.

## Core Functions

### `Apex(host string) string`

Determines the apex (root) domain from a given hostname using public suffix rules.

**Parameters:**
- `host`: The hostname to analyze (e.g., "sub.example.com")

**Returns:**
- `string`: The apex domain (e.g., "example.com")

**Implementation:**
- Uses `golang.org/x/net/publicsuffix` for accurate public suffix detection
- Handles complex TLDs like ".co.uk" and ".com.au"
- Case-insensitive processing
- Fallback to original host on parsing errors

### `ParseLinks(base *url.URL, body io.Reader) ([]string, error)`

Extracts all URLs from HTML content, resolving relative URLs to absolute forms.

**Parameters:**
- `base`: Base URL for resolving relative links
- `body`: HTML content reader (typically HTTP response body)

**Returns:**
- `[]string`: Array of absolute URLs found in the HTML
- `error`: Parsing error, if any

### `ExternalDomains(baseHost string, urls []string) []string`

Filters URLs to identify unique external domains that differ from the base domain's apex.

**Parameters:**
- `baseHost`: The base domain being analyzed
- `urls`: Array of URLs to filter

**Returns:**
- `[]string`: Unique external domain names

## HTML Element Processing

### Link Elements

#### `<a>` and `<link>` Tags
- **Attribute**: `href`
- **Purpose**: Navigation links and resource references
- **Examples**: Anchor links, stylesheets, canonical URLs

#### `<script>`, `<img>`, `<iframe>`, `<source>` Tags
- **Attribute**: `src`
- **Purpose**: Embedded resources and content
- **Examples**: JavaScript files, images, embedded frames

### URL Resolution Process

1. **Attribute Extraction**: Identifies relevant attributes (href/src)
2. **URL Parsing**: Validates and parses attribute values
3. **Scheme Resolution**: Applies base URL scheme to relative URLs
4. **Host Resolution**: Applies base URL host to relative URLs
5. **Absolute URL Creation**: Constructs complete absolute URLs

## External Domain Filtering

### Domain Classification

#### Internal Domains
- **Same Apex**: Domains sharing the same apex as the base domain
- **Subdomains**: Different subdomains of the same apex domain
- **Examples**: `sub1.example.com` and `sub2.example.com` both internal to `example.com`

#### External Domains
- **Different Apex**: Domains with different apex domains
- **Cross-Domain Links**: Links pointing to entirely different organizations
- **Examples**: Links from `example.com` to `google.com` or `github.com`

### Deduplication Strategy
- **Case-Insensitive**: Hostnames normalized to lowercase
- **Per-Analysis Deduplication**: Each HTML analysis produces unique external domains
- **Memory-Efficient**: Uses map-based seen tracking

## Edge Creation

Link extraction creates `LINKS_TO` edges:
- **Source**: Base domain (the domain containing the HTML)
- **Target**: External domain (discovered external domain)
- **Relationship**: Indicates the source domain links to the target domain

## Integration Points

### Probe Pipeline Integration
1. **Input**: HTML content from HTTP responses
2. **Processing**: Link extraction and external domain filtering
3. **Output**: External domain list for edge creation

### HTTP Client Integration
- Processes HTML responses from the HTTP client component
- Uses base URL from HTTP request for relative URL resolution
- Handles various HTML encoding formats

## Performance Considerations

### Streaming Processing
- **Token-Based Parsing**: Uses `html.NewTokenizer` for memory efficiency
- **Streaming Analysis**: Processes HTML without loading entire document
- **Early Termination**: Supports EOF detection and error handling

### Memory Management
- **No DOM Creation**: Avoids building complete DOM trees
- **Selective Processing**: Only processes relevant HTML elements
- **Efficient Deduplication**: Map-based tracking prevents duplicate processing

## Security Features

### URL Validation
- **Malformed URL Handling**: Gracefully handles invalid URLs
- **Scheme Validation**: Supports HTTP and HTTPS schemes
- **Host Validation**: Requires valid hostname components

### Input Sanitization
- **Whitespace Trimming**: Removes leading/trailing whitespace from URLs
- **Attribute Case Handling**: Case-insensitive attribute matching
- **Error Resilience**: Continues processing despite individual URL parse errors

## Common Use Cases

### Website Analysis
```go
// Parse HTML content and extract external domains
baseURL, _ := url.Parse("https://example.com/page")
links, _ := extract.ParseLinks(baseURL, htmlReader)
externals := extract.ExternalDomains("example.com", links)
```

### Content Discovery
- **CDN Detection**: Identify Content Delivery Network usage
- **Third-Party Services**: Discover integrated services and analytics
- **Partner Networks**: Map business relationship networks

### Security Analysis
- **External Dependencies**: Identify external resource dependencies
- **Potential Attack Vectors**: Map external link exposure
- **Supply Chain Analysis**: Track third-party service usage

## Supported HTML Elements

### Navigation Elements
- **`<a href="">`**: Anchor links and navigation
- **`<link href="">`**: Stylesheets, icons, canonical URLs

### Resource Elements
- **`<script src="">`**: JavaScript files and libraries
- **`<img src="">`**: Images and graphics
- **`<iframe src="">`**: Embedded content and frames
- **`<source src="">`**: Media source alternatives

## URL Resolution Examples

### Relative URL Resolution
```html
<!-- Base: https://example.com/page/index.html -->
<a href="../other.html">          <!-- → https://example.com/other.html -->
<script src="/js/app.js">         <!-- → https://example.com/js/app.js -->
<img src="image.png">             <!-- → https://example.com/page/image.png -->
```

### External Domain Detection
```html
<!-- Base domain: example.com -->
<a href="https://google.com">     <!-- External: google.com -->
<script src="//cdn.example.com">  <!-- Internal: same apex -->
<img src="https://img.other.org"> <!-- External: other.org -->
```

## Error Handling

### Parse Errors
- **Malformed HTML**: Continues processing despite HTML syntax errors
- **Invalid URLs**: Skips invalid URLs without stopping analysis
- **Encoding Issues**: Handles various character encodings gracefully

### Recovery Strategies
- **Best-Effort Processing**: Extracts as many valid links as possible
- **Graceful Degradation**: Returns partial results on errors
- **Error Logging**: Reports parsing issues for debugging

## Configuration Considerations

### Public Suffix List
- **Automatic Updates**: Uses `publicsuffix` package for current TLD rules
- **Complex TLD Support**: Handles multi-level TLDs correctly
- **International Domains**: Supports internationalized domain names

### Processing Limits
- **HTML Size**: No explicit size limits (streaming processing)
- **Link Count**: No artificial limits on links per page
- **Domain Count**: Efficient deduplication prevents memory issues

## Monitoring Metrics

Link extraction generates metrics for:
- **Links Processed**: Total number of links analyzed per page
- **External Domains Found**: Count of unique external domains per analysis
- **Parse Success Rate**: Percentage of successful HTML parsing operations
- **Processing Time**: Latency for link extraction operations