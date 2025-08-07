# SPYDER Probe (Project Arachnet) — Production Build

**SPYDER** (System for Probing and Yielding DNS-based Entity Relations) is a distributed, policy-aware probe for mapping inter-domain relationships: DNS, TLS cert metadata, and *external* links (root page only), while respecting `robots.txt` and default-skipping sensitive TLDs.

## Production Features
- Structured logging via **zap**
- Metrics endpoint (**Prometheus**) at `:9090/metrics` (configurable)
- **robots.txt** caching (LRU) and enforcement
- Per-host token-bucket rate limiting + global concurrency
- Retries with exponential backoff
- Optional **Redis** deduper for cross-process de-duplication
- Batch emitter with **ingest retries** and **on-disk spool** fallback
- mTLS support for ingest (client cert/key + CA)
- Graceful shutdown
- CI: **golangci-lint**, build, test
- Dockerfile (distroless) + example **systemd** unit
- VitePress docs site

## Build & Run
```bash
go mod download
go build -o bin/spyder ./cmd/spyder

# minimal run
echo -e "example.com\ngolang.org" > configs/domains.txt
./bin/spyder -domains=configs/domains.txt

# with ingest + metrics + redis dedupe
REDIS_ADDR=127.0.0.1:6379 ./bin/spyder -domains=configs/domains.txt   -ingest=https://ingest.example.com/v1/batch   -metrics_addr=:9090   -probe=us-west-1a
```

## Config (flags or env)
- `-domains` (required): path to newline-separated hosts
- `-ingest`: HTTP(S) endpoint; if empty, outputs JSON batches to stdout
- `-probe`: probe ID
- `-run`: run ID
- `-concurrency`: worker count (default 256)
- `-ua`: User-Agent string
- `-exclude_tlds`: comma list (`gov,mil,int`)
- `-metrics_addr`: e.g. `:9090` or empty to disable
- `-batch_max_edges`: default 10000
- `-batch_flush_sec`: default 2
- `-spool_dir`: default `spool/` (for failed batch files)
- `-mtls_cert`, `-mtls_key`, `-mtls_ca`: optional mTLS for ingest

Env:
- `REDIS_ADDR` for Redis-backed dedupe (optional)

## Docs
```bash
cd docs && npm i && npm run docs:dev
```

See `docs/` for architecture and ops guides.
