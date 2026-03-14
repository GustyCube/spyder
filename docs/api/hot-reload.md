# Hot Reload

SPYDER supports changing many configuration fields **without restarting the process**. Changes take effect immediately and are propagated to running components via registered change listeners.

## Tier classification

Configuration fields are divided into three tiers based on how safely they can be changed at runtime.

### Tier 1 — Safe, immediate

These fields are applied instantly with no side-effects. Workers pick up the new values on their next iteration.

| Field                          | Description                                          |
|--------------------------------|------------------------------------------------------|
| `crawling.concurrency`         | Target number of worker goroutines                   |
| `crawling.rate_per_host`       | Per-host request rate (requests/sec)                 |
| `crawling.rate_burst`          | Per-host token bucket burst size                     |
| `crawling.max_domains`         | Discovery cap (0 = unlimited)                        |
| `crawling.continuous`          | Whether crawling continues after the cap is reached  |
| `crawling.http_timeout`        | HTTP request timeout for workers                     |
| `crawling.tls_timeout`         | TLS handshake timeout                                |
| `crawling.body_max_bytes`      | Maximum response body size to read                   |
| `batch.max_edges`              | Edges per output batch                               |
| `batch.flush_interval`         | How often to flush a partial batch                   |
| `batch.node_threshold_ratio`   | Node flush threshold relative to max_edges           |
| `exclude_tlds`                 | List of TLDs to skip entirely                        |
| `ua`                           | User-agent string sent in HTTP requests              |
| `api.rate_limit`               | API global rate limit (req/sec)                      |
| `api.rate_burst`               | API rate burst size                                  |
| `logging.level`                | Log verbosity (`debug`, `info`, `warn`, `error`)     |

### Tier 2 — Applied with warnings

These fields can be changed at runtime, but doing so mid-run may cause observable side-effects. A `warnings` array is returned alongside the updated config.

| Field              | Warning                                                                       |
|--------------------|-------------------------------------------------------------------------------|
| `output.format`    | Mixed-format output if changed while a batch is in flight                     |
| `output.ingest`    | Partial batches may be delivered to the old endpoint                          |
| `output.spool_dir` | Existing spooled files remain in the old directory and will not be retried    |

### Tier 3 — Requires restart

These fields require re-establishing connections or re-initializing subsystems. Attempting to patch them returns HTTP 409 with an error listing which fields require a restart.

| Field                      | Reason                                         |
|----------------------------|------------------------------------------------|
| `redis.addr`               | Reconnects the task queue                      |
| `redis.queue_addr`         | Reconnects the distributed queue               |
| `mongodb.uri`              | Reconnects the database store                  |
| `mongodb.database`         | Changes the target database                    |
| `mtls.cert`                | Rebuilds the mTLS transport                    |
| `mtls.key`                 | Rebuilds the mTLS transport                    |
| `mtls.ca`                  | Rebuilds the mTLS transport                    |
| `telemetry.metrics_addr`   | Re-binds the metrics HTTP server               |
| `dashboard.history_size`   | Requires hub reinitialization                  |

## Triggering a reload via SIGHUP

Send `SIGHUP` to the running probe to reload configuration from disk:

```bash
# Find the PID
pgrep -f spyder

# Send the signal
kill -HUP <pid>
```

SIGHUP causes SPYDER to:
1. Re-read the config file it was started with.
2. Compute a diff against the current in-memory configuration.
3. Apply all Tier 1 and Tier 2 changes (with warnings logged).
4. Log an error and skip any Tier 3 changes detected in the diff.

The process does **not** exit on SIGHUP. If the config file is invalid (parse error, validation failure), the reload is aborted and the existing config remains active.

## Triggering a reload via API

Use `PATCH /api/v1/config` with only the fields you want to change. Omitted fields are left unchanged.

### Example: raise log level to debug

```bash
curl -s -X PATCH http://localhost:9090/api/v1/config \
  -H "Authorization: Bearer $WRITE_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "logging": {"level": "debug"}
  }' | jq .
```

### Example: scale concurrency and tighten rate limits

```bash
curl -s -X PATCH http://localhost:9090/api/v1/config \
  -H "Authorization: Bearer $WRITE_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "crawling": {
      "concurrency": 64,
      "rate_per_host": 0.5,
      "rate_burst": 2
    }
  }' | jq .
```

### Example: change output ingest endpoint (Tier 2)

```bash
curl -s -X PATCH http://localhost:9090/api/v1/config \
  -H "Authorization: Bearer $WRITE_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "output": {"ingest": "http://new-ingest.example.com/batch"}
  }' | jq .
```

Successful response includes a `warnings` array:

```json
{
  "config": { "...": "..." },
  "warnings": [
    "changing ingest endpoint mid-run may cause partial batch delivery to old endpoint"
  ]
}
```

### Example: attempt a Tier 3 change (rejected)

```bash
curl -s -X PATCH http://localhost:9090/api/v1/config \
  -H "Authorization: Bearer $WRITE_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "redis": {"addr": "redis-new:6379"}
  }' | jq .
```

Response — HTTP 409:

```json
{
  "code": "APPLY_ERROR",
  "error": "fields require restart: redis.addr"
}
```

## Warnings explained

| Warning message                                                                      | What it means                                                      |
|--------------------------------------------------------------------------------------|--------------------------------------------------------------------|
| `changing output format mid-run may cause mixed format output`                       | Batches already in the accumulator will flush in the old format    |
| `changing ingest endpoint mid-run may cause partial batch delivery to old endpoint`  | The current in-flight HTTP POST may complete to the old URL        |
| `changing spool directory mid-run leaves existing spooled files in old directory`    | Old spool files are not moved and won't be retried from the new dir|

In all Tier 2 cases, the new value **is** applied. The warning is informational — no data is lost, but there may be a brief inconsistency window.
