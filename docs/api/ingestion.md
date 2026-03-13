# Ingestion API

The ingestion API enables SPYDER probes to deliver discovered nodes and edges to a centralized collection endpoint via HTTP POST. The emitter (`internal/emit`) handles batching, retry, mTLS authentication, and on-disk spooling for fault-tolerant delivery.

## Endpoint Configuration

The ingestion endpoint is configured via the `-ingest` CLI flag or `ingest` config field. When set, the emitter POSTs JSON batches to this URL. When left empty, batches are written to stdout as JSON.

```bash
# Send batches to a remote ingestion service
./bin/spyder -domains=domains.txt -ingest=https://ingest.example.com/v1/batch

# Print batches to stdout (default when -ingest is omitted)
./bin/spyder -domains=domains.txt
```

## Request Format

The emitter sends an HTTP POST with `Content-Type: application/json`. The body is a single `emit.Batch` object containing all nodes and edges accumulated since the last flush.

### Batch JSON Structure

```json
{
  "probe_id": "prod-us-west-1",
  "run_id": "run-1701444600",
  "nodes_domain": [
    {
      "host": "www.example.com",
      "apex": "example.com",
      "first_seen": "2024-01-15T10:30:00Z",
      "last_seen": "2024-01-15T10:30:00Z"
    }
  ],
  "nodes_ip": [
    {
      "ip": "93.184.216.34",
      "first_seen": "2024-01-15T10:30:00Z",
      "last_seen": "2024-01-15T10:30:00Z"
    }
  ],
  "nodes_cert": [
    {
      "spki_sha256": "a3b2c1d4e5f6...",
      "subject_cn": "www.example.com",
      "issuer_cn": "DigiCert SHA2 Extended Validation Server CA",
      "not_before": "2024-01-01T00:00:00Z",
      "not_after": "2025-01-01T00:00:00Z"
    }
  ],
  "edges": [
    {
      "type": "RESOLVES_TO",
      "source": "www.example.com",
      "target": "93.184.216.34",
      "observed_at": "2024-01-15T10:30:00Z",
      "probe_id": "prod-us-west-1",
      "run_id": "run-1701444600"
    },
    {
      "type": "USES_CERT",
      "source": "www.example.com",
      "target": "a3b2c1d4e5f6...",
      "observed_at": "2024-01-15T10:30:00Z",
      "probe_id": "prod-us-west-1",
      "run_id": "run-1701444600"
    }
  ]
}
```

### Edge Types

| Type | Relationship | Source | Target |
|------|-------------|--------|--------|
| `RESOLVES_TO` | DNS A/AAAA record | Domain | IP address |
| `USES_NS` | DNS NS record | Domain | Nameserver |
| `ALIAS_OF` | DNS CNAME record | Domain | CNAME target |
| `USES_MX` | DNS MX record | Domain | Mail exchanger |
| `LINKS_TO` | HTML hyperlink | Domain | External domain |
| `USES_CERT` | TLS handshake | Domain | SPKI SHA-256 hash |

## Batch Flushing

The emitter flushes batches to the ingestion endpoint based on two triggers, whichever fires first:

- **Size threshold**: When the edge count reaches `batch_max_edges` (default: 10000) or the combined node count reaches half that value.
- **Time interval**: Every `batch_flush_sec` seconds (default: 2).

```bash
# Larger batches, less frequent flushes
./bin/spyder -domains=domains.txt \
  -ingest=https://ingest.example.com/v1/batch \
  -batch_max_edges=5000 \
  -batch_flush_sec=10

# Smaller batches for lower latency
./bin/spyder -domains=domains.txt \
  -ingest=https://ingest.example.com/v1/batch \
  -batch_max_edges=500 \
  -batch_flush_sec=1
```

## mTLS Authentication

The emitter supports mutual TLS for authenticating with the ingestion endpoint. Three CLI flags control mTLS:

| Flag | Description |
|------|-------------|
| `-mtls_cert` | Path to client certificate (PEM) |
| `-mtls_key` | Path to client private key (PEM) |
| `-mtls_ca` | Path to CA bundle (PEM) for server verification |

Both `-mtls_cert` and `-mtls_key` must be provided together. The `-mtls_ca` flag is optional and adds custom root CAs for verifying the server's certificate.

```bash
./bin/spyder -domains=domains.txt \
  -ingest=https://ingest.example.com/v1/batch \
  -mtls_cert=/etc/spyder/client.crt \
  -mtls_key=/etc/spyder/client.key \
  -mtls_ca=/etc/spyder/ca-bundle.crt
```

### Certificate Setup

Generate a client certificate signed by your CA:

```bash
# Generate client key
openssl genrsa -out client.key 4096

# Generate CSR
openssl req -new -key client.key -out client.csr \
  -subj "/CN=spyder-probe-us-west-1/O=SpyderProbes"

# Sign with CA
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out client.crt -days 365

# Verify the certificate
openssl verify -CAfile ca.crt client.crt
```

### YAML Configuration

```yaml
ingest: "https://ingest.example.com/v1/batch"
mtls_cert: "/etc/spyder/client.crt"
mtls_key: "/etc/spyder/client.key"
mtls_ca: "/etc/spyder/ca-bundle.crt"
```

## Retry Behavior

When a POST to the ingestion endpoint fails, the emitter retries with exponential backoff using the `cenkalti/backoff` library.

### Retry Parameters

| Parameter | Value |
|-----------|-------|
| Initial interval | 500ms (library default) |
| Multiplier | 1.5x (library default) |
| Max elapsed time | **30 seconds** |
| Randomization | Jittered to prevent thundering herd |
| Context awareness | Retries stop immediately on context cancellation |

### Retryable Conditions

- **HTTP 5xx responses**: Server errors trigger retries.
- **Connection errors**: Network failures, DNS resolution failures, and timeouts trigger retries.
- **Request creation failures**: Permanent errors (e.g., invalid URL) are wrapped with `backoff.Permanent` and do not retry.

```go
// From internal/emit/emit.go - retry logic
bo := backoff.NewExponentialBackOff()
bo.MaxElapsedTime = 30 * time.Second
return backoff.Retry(op, backoff.WithContext(bo, ctx))
```

The HTTP client itself has a 20-second timeout per individual request attempt.

## Spool Directory

When all retries are exhausted, the batch is written to disk in the spool directory as a timestamped JSON file. This prevents data loss during extended ingestion outages.

### Spool Configuration

```bash
./bin/spyder -domains=domains.txt \
  -ingest=https://ingest.example.com/v1/batch \
  -spool_dir=/var/spool/spyder
```

The spool directory is created automatically if it does not exist (mode `0755`).

### Spool File Format

Each spooled batch is written as a single JSON file with a UTC timestamp filename:

```
/var/spool/spyder/
  20240115T103045.123456789.json
  20240115T103112.987654321.json
```

Each file contains exactly one `Batch` JSON object, identical to what would have been POSTed.

### Drain and Recovery

On shutdown, the emitter calls `Drain()` which:

1. Flushes any remaining buffered data.
2. Reads all `.json` files from the spool directory.
3. Attempts to POST each spooled batch to the ingestion endpoint.
4. Removes spool files that are delivered successfully.
5. Leaves files on disk if delivery still fails.

```go
// Drain is called automatically during graceful shutdown
emitter.Drain(log)
```

This means spooled batches are retried on the next startup or shutdown cycle, providing eventual delivery guarantees.

## Error Handling and Status Codes

### Response Handling

The emitter considers any HTTP status code outside the `2xx` range as a failure:

| Status Code | Behavior |
|-------------|----------|
| `200-299` | Success, batch accepted |
| `4xx` | Retried (within the 30s backoff window) |
| `5xx` | Retried (within the 30s backoff window) |
| Connection error | Retried (within the 30s backoff window) |
| All retries exhausted | Batch spooled to disk |

### Logging

Failed deliveries are logged at `WARN` level with the error details:

```
WARN  ingest failed, spooling  {"err": "bad status: 503"}
```

Spool file creation errors are logged at `ERROR` level.

## Example curl Commands

### Submit a Batch Manually

```bash
curl -X POST https://ingest.example.com/v1/batch \
  -H "Content-Type: application/json" \
  -d '{
    "probe_id": "manual-test",
    "run_id": "test-run-001",
    "nodes_domain": [
      {"host": "example.com", "apex": "example.com", "first_seen": "2024-01-15T10:30:00Z", "last_seen": "2024-01-15T10:30:00Z"}
    ],
    "nodes_ip": [
      {"ip": "93.184.216.34", "first_seen": "2024-01-15T10:30:00Z", "last_seen": "2024-01-15T10:30:00Z"}
    ],
    "nodes_cert": [],
    "edges": [
      {"type": "RESOLVES_TO", "source": "example.com", "target": "93.184.216.34", "observed_at": "2024-01-15T10:30:00Z", "probe_id": "manual-test", "run_id": "test-run-001"}
    ]
  }'
```

### Submit with mTLS

```bash
curl -X POST https://ingest.example.com/v1/batch \
  --cert /etc/spyder/client.crt \
  --key /etc/spyder/client.key \
  --cacert /etc/spyder/ca-bundle.crt \
  -H "Content-Type: application/json" \
  -d @batch.json
```

### Replay a Spooled Batch

```bash
# Resend a spooled batch file
curl -X POST https://ingest.example.com/v1/batch \
  -H "Content-Type: application/json" \
  -d @/var/spool/spyder/20240115T103045.123456789.json
```

### Inspect Spool Contents

```bash
# List spooled files
ls -la /var/spool/spyder/

# View a spooled batch
cat /var/spool/spyder/20240115T103045.123456789.json | jq .

# Count edges in a spooled batch
cat /var/spool/spyder/20240115T103045.123456789.json | jq '.edges | length'
```

## Stdout Mode

When no `-ingest` URL is provided, the emitter writes each flushed batch as a single JSON line to stdout. This is useful for piping into other tools or for local development:

```bash
# Pipe to jq for pretty-printing
./bin/spyder -domains=domains.txt | jq .

# Write batches to a file
./bin/spyder -domains=domains.txt > output.json

# Stream to another process
./bin/spyder -domains=domains.txt | my-ingestion-tool --stdin
```

## Configuration Reference

| Flag | Config Key | Default | Description |
|------|-----------|---------|-------------|
| `-ingest` | `ingest` | `""` (stdout) | Ingestion endpoint URL |
| `-batch_max_edges` | `batch_max_edges` | `10000` | Max edges before flush |
| `-batch_flush_sec` | `batch_flush_sec` | `2` | Seconds between time-based flushes |
| `-spool_dir` | `spool_dir` | `spool` | Directory for failed batch files |
| `-mtls_cert` | `mtls_cert` | `""` | Client certificate path (PEM) |
| `-mtls_key` | `mtls_key` | `""` | Client private key path (PEM) |
| `-mtls_ca` | `mtls_ca` | `""` | CA bundle path (PEM) |
| `-probe` | `probe` | `local-1` | Probe identifier in batch metadata |
| `-run` | `run` | `run-{timestamp}` | Run identifier in batch metadata |
