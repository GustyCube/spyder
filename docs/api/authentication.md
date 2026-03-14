# API Authentication

All SPYDER control API endpoints require a valid API key passed as a Bearer token in the `Authorization` header.

```
Authorization: Bearer spyder_<64-hex-chars>
```

## Scopes

SPYDER uses a hierarchical three-scope model. Higher scopes include all permissions of lower scopes.

| Scope   | Includes  | Description                                                          |
|---------|-----------|----------------------------------------------------------------------|
| `read`  | —         | Read-only access to stats, config, and query endpoints               |
| `write` | `read`    | Read access plus the ability to modify runtime state and config      |
| `admin` | `write`   | Full access including key management and destructive operations      |

### Scope mapping by endpoint class

| Endpoint group           | Required scope |
|--------------------------|---------------|
| Submit domains           | `write`       |
| Pause / Resume           | `write`       |
| **Stop** probe           | `admin`       |
| Get config / stats       | `read`        |
| Patch config             | `write`       |
| Scale workers            | `write`       |
| Reset breakers           | `write`       |
| Set rate limits          | `write`       |
| Clear dedup (all keys)   | `admin`       |
| Delete a single dedup key| `write`       |
| Flush emitter            | `write`       |
| Manage excludes          | `write`       |
| DNS / TLS diagnostics    | `read`        |
| List / Create / Delete keys | `admin`    |
| Query domains/edges/IPs  | `read`        |

## Configuring keys in `config.yaml`

Keys are listed under the `api.keys` section:

```yaml
api:
  rate_limit: 100   # requests/sec (global, all callers combined)
  rate_burst: 20
  keys:
    - key: spyder_0000000000000000000000000000000000000000000000000000000000000001
      name: ops-team
      scope: admin
    - key: spyder_0000000000000000000000000000000000000000000000000000000000000002
      name: dashboard-ro
      scope: read
    - key: spyder_0000000000000000000000000000000000000000000000000000000000000003
      name: ci-pipeline
      scope: write
```

Key values must be at least 8 characters. In production, generate them with:

```bash
python3 -c "import secrets; print('spyder_' + secrets.token_hex(32))"
```

## Configuring keys via environment variable

To avoid storing keys in the config file, export `SPYDER_API_KEYS` as a base64-encoded JSON array:

```bash
# Build the JSON array
KEYS_JSON='[
  {"key":"spyder_abc123","name":"ops-team","scope":"admin"},
  {"key":"spyder_def456","name":"ci-pipeline","scope":"write"}
]'

# Base64-encode it (no line breaks)
export SPYDER_API_KEYS=$(echo -n "$KEYS_JSON" | base64)
```

SPYDER decodes and merges these keys with any keys already present in the config file at startup.

### Kubernetes secret example

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: spyder-api-keys
type: Opaque
stringData:
  SPYDER_API_KEYS: |
    W3sia2V5Ijoic3B5ZGVyX2FiYzEyMyIsIm5hbWUiOiJvcHMtdGVhbSIsInNjb3BlIjoiYWRtaW4ifV0=
```

Then mount it as an environment variable in your Pod spec.

## Creating keys at runtime

Keys can be created without restarting via the API (requires an existing `admin` key):

```bash
curl -s -X POST http://localhost:9090/api/v1/keys \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "new-service", "scope": "write"}' | jq .
```

Response:

```json
{
  "key": "spyder_a1b2c3d4e5f6...",
  "name": "new-service",
  "scope": "write"
}
```

> **The key value is only shown once.** Store it immediately.

## Revoking keys

Delete a key by its token value (requires `admin`):

```bash
curl -s -X DELETE http://localhost:9090/api/v1/keys/spyder_a1b2c3d4e5f6... \
  -H "Authorization: Bearer $ADMIN_KEY" | jq .
```

## curl examples by scope

### Read-only: get current stats

```bash
curl -s http://localhost:9090/api/v1/stats \
  -H "Authorization: Bearer $READ_KEY" | jq .
```

### Write: submit a domain

```bash
curl -s -X POST http://localhost:9090/api/v1/domains \
  -H "Authorization: Bearer $WRITE_KEY" \
  -H "Content-Type: application/json" \
  -d '{"host": "example.com"}' | jq .
```

### Write: bulk submit from a file

```bash
curl -s -X POST http://localhost:9090/api/v1/domains/bulk \
  -H "Authorization: Bearer $WRITE_KEY" \
  -H "Content-Type: text/plain" \
  --data-binary @domains.txt | jq .
```

### Admin: stop the probe

```bash
curl -s -X POST http://localhost:9090/api/v1/control/stop \
  -H "Authorization: Bearer $ADMIN_KEY" | jq .
```

## Error responses

Authentication failures return standard error objects:

| Status | Code            | Cause                                          |
|--------|-----------------|------------------------------------------------|
| 401    | `UNAUTHORIZED`  | Missing or malformed `Authorization` header, or unknown key |
| 403    | `FORBIDDEN`     | Key exists but its scope is insufficient       |
| 429    | `RATE_LIMITED`  | Global API rate limit exceeded                 |

```json
{
  "code": "UNAUTHORIZED",
  "error": "missing or invalid authorization header"
}
```
