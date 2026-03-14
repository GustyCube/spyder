# API Configuration Reference

The `api:` section of `config.yaml` controls the built-in REST control API. The API server runs on the same address as the Prometheus metrics endpoint (configured under `telemetry.metrics_addr`, default `:9090`).

## Full `api:` section

```yaml
api:
  # Global rate limit applied across all callers combined.
  # Requests per second allowed before returning HTTP 429.
  # Default: 100
  rate_limit: 100

  # Token bucket burst size. Allows short bursts above rate_limit.
  # Default: 20
  rate_burst: 20

  # API keys. Each key has a name (for logging), a scope, and the key token.
  # Scopes are hierarchical: admin > write > read.
  # Keys can also be supplied via the SPYDER_API_KEYS environment variable
  # (base64-encoded JSON array) to avoid storing secrets in the config file.
  keys:
    - key: spyder_0000000000000000000000000000000000000000000000000000000000000001
      name: ops-team
      scope: admin
    - key: spyder_0000000000000000000000000000000000000000000000000000000000000002
      name: dashboard
      scope: read
```

## Field reference

### `api.rate_limit`

| Property | Value    |
|----------|----------|
| Type     | integer  |
| Default  | `100`    |
| Tier     | 1 (hot-reloadable) |

Global requests-per-second limit for the entire API, applied as a single token bucket shared across all callers and all endpoints. When the limit is exceeded the server returns `HTTP 429 Too Many Requests` with a `Retry-After: 1` header.

This setting can be changed at runtime without restart:

```bash
curl -s -X PATCH http://localhost:9090/api/v1/config \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"api": {"rate_limit": 50}}'
```

### `api.rate_burst`

| Property | Value    |
|----------|----------|
| Type     | integer  |
| Default  | `20`     |
| Tier     | 1 (hot-reloadable) |

Burst size for the API token bucket. Allows up to `rate_burst` requests to be served instantly even if the sustained rate would otherwise be exceeded. Set to `1` to enforce strict per-second limits with no bursting.

### `api.keys`

| Property | Value    |
|----------|----------|
| Type     | list of APIKey objects |
| Default  | `[]` (no keys; all requests rejected) |

Each key object has three fields:

| Field   | Required | Description                                     |
|---------|----------|-------------------------------------------------|
| `key`   | yes      | The token sent in `Authorization: Bearer <key>` |
| `name`  | yes      | Human-readable label for audit logs             |
| `scope` | yes      | One of `read`, `write`, or `admin`              |

Keys can also be managed at runtime via the API (requires an existing `admin` key):

```bash
# Create a new key
curl -s -X POST http://localhost:9090/api/v1/keys \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "automation", "scope": "write"}'

# List all keys
curl -s http://localhost:9090/api/v1/keys \
  -H "Authorization: Bearer $ADMIN_KEY"

# Delete a key
curl -s -X DELETE http://localhost:9090/api/v1/keys/spyder_abc123... \
  -H "Authorization: Bearer $ADMIN_KEY"
```

Keys created via the API are stored in memory. They survive hot-reloads but **are lost on restart** unless you add them to the config file or `SPYDER_API_KEYS`.

## Environment variable: `SPYDER_API_KEYS`

To avoid putting secrets in config files, export `SPYDER_API_KEYS` as a base64-encoded JSON array of key objects:

```bash
KEYS_JSON='[{"key":"spyder_abc","name":"ops","scope":"admin"}]'
export SPYDER_API_KEYS=$(echo -n "$KEYS_JSON" | base64)
```

This is merged with any keys in `config.yaml` at startup.

## Changing the listen address

The API shares a port with the Prometheus metrics endpoint. Change it under `telemetry.metrics_addr` (requires restart — Tier 3):

```yaml
telemetry:
  metrics_addr: ":8080"   # API and /metrics both served here
```

## Adjusting rate limits at runtime

`rate_limit` and `rate_burst` are Tier 1 (hot-reloadable). They can be patched without restart, taking effect on the next incoming request:

```bash
# Drop to 10 req/sec with no burst during maintenance
curl -s -X PATCH http://localhost:9090/api/v1/config \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"api": {"rate_limit": 10, "rate_burst": 1}}'

# Restore defaults
curl -s -X PATCH http://localhost:9090/api/v1/config \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"api": {"rate_limit": 100, "rate_burst": 20}}'
```

## Running without authentication (development only)

If `api.keys` is empty and `SPYDER_API_KEYS` is unset, **every request is rejected with HTTP 401**. There is no unauthenticated mode. For local development, add a key directly in the config:

```yaml
api:
  keys:
    - key: spyder_dev00000000000000000000000000000000000000000000000000000000000
      name: local-dev
      scope: admin
```

Do not use this key in production.
