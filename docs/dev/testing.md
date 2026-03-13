# Testing Guide

This guide covers running, writing, and maintaining tests for the SPYDER codebase. SPYDER uses Go's built-in testing framework with no external test dependencies.

## Running Tests

### Full Test Suite

Run every test in the project with a single command:

```bash
go test ./...
```

For verbose output showing individual test names and results:

```bash
go test -v ./...
```

The Makefile provides a shortcut that also generates a coverage profile:

```bash
make test
```

This runs `go test ./... -coverprofile=coverage.txt` under the hood.

### Running Tests for a Specific Package

Target a single package when you are working on a particular subsystem:

```bash
# Test only the DNS resolver
go test -v ./internal/dns

# Test only the circuit breaker
go test -v ./internal/circuitbreaker

# Test only config loading and validation
go test -v ./internal/config
```

### Running a Single Test

Use the `-run` flag with a regex that matches the test function name:

```bash
# Run only the YAML config loading test
go test -v -run TestLoadFromFile_YAML ./internal/config

# Run all circuit breaker state transition tests
go test -v -run TestCircuitBreaker ./internal/circuitbreaker

# Run only the concurrent dedup test
go test -v -run TestMemory_Concurrent ./internal/dedup
```

## Test Coverage

### Generating a Coverage Profile

```bash
go test -coverprofile=coverage.txt ./...
```

### Viewing Coverage in the Terminal

```bash
go tool cover -func=coverage.txt
```

This prints per-function coverage percentages, for example:

```
github.com/gustycube/spyder/internal/config/config.go:52:   SetDefaults     100.0%
github.com/gustycube/spyder/internal/config/config.go:92:   Validate        100.0%
github.com/gustycube/spyder/internal/dedup/memory.go:15:    Seen            100.0%
```

### Viewing Coverage in a Browser

Generate an HTML report and open it:

```bash
go tool cover -html=coverage.txt -o coverage.html
open coverage.html   # macOS
xdg-open coverage.html  # Linux
```

The HTML report highlights covered lines in green and uncovered lines in red, making it easy to find gaps.

### Per-Package Coverage

Check coverage for a single package during development:

```bash
go test -cover ./internal/rate
# ok  github.com/gustycube/spyder/internal/rate  0.015s  coverage: 87.5% of statements
```

## Race Detection

Go's race detector finds data races at runtime. SPYDER uses goroutines extensively (worker pools, concurrent dedup, rate limiters), so race detection is critical.

### Running Tests with Race Detection

```bash
go test -race ./...
```

This instruments the binary with ThreadSanitizer. Tests run slower (typically 2-10x) but will catch concurrent access bugs that only manifest under specific timing conditions.

### Running Race Detection on Specific Packages

Packages with concurrent code that should always be tested with `-race`:

```bash
go test -race ./internal/dedup    # concurrent map access via sync.Map
go test -race ./internal/rate     # concurrent per-host limiter access
go test -race ./internal/circuitbreaker  # state transitions under load
```

### Building a Race-Instrumented Binary

For manual testing against a live environment:

```bash
go build -race -o bin/spyder-debug ./cmd/spyder
./bin/spyder-debug -domains=configs/domains.txt -concurrency=64
```

Any detected race will print a diagnostic to stderr and crash the program with a non-zero exit code.

## Existing Test Packages

The following packages have test coverage. Use them as examples when writing new tests.

### `internal/circuitbreaker`

Tests the three-state circuit breaker (Closed, Open, Half-Open) and the per-host breaker wrapper:

```bash
go test -v ./internal/circuitbreaker
```

Key test cases:
- `TestCircuitBreaker_ClosedState` -- successful requests keep circuit closed
- `TestCircuitBreaker_OpensOnFailures` -- exceeding failure ratio opens the circuit
- `TestCircuitBreaker_HalfOpenState` -- timeout transitions to half-open, successes close it
- `TestCircuitBreaker_HalfOpenFailure` -- failure in half-open reopens the circuit
- `TestHostBreaker` -- independent breakers per host, stats, reset
- `TestExecuteWithRetry` -- retry logic with exponential backoff
- `TestExecuteWithRetry_CircuitOpen` -- retries abort when circuit is open

### `internal/config`

Tests YAML/JSON loading, default values, validation, flag merging, and environment variable loading:

```bash
go test -v ./internal/config
```

Key test cases:
- `TestLoadFromFile_YAML` -- loads and parses a YAML config file
- `TestLoadFromFile_JSON` -- loads and parses a JSON config file
- `TestSetDefaults` -- verifies all default values (concurrency=256, batch_max_edges=10000, etc.)
- `TestValidate` -- table-driven validation with valid and invalid configs
- `TestMergeWithFlags` -- CLI flags override file config, unset flags preserve originals
- `TestLoadFromEnv` -- reads `REDIS_ADDR`, `REDIS_QUEUE_ADDR`, `REDIS_QUEUE_KEY` from environment

### `internal/dedup`

Tests the in-memory deduplication implementation:

```bash
go test -v ./internal/dedup
```

Key test cases:
- `TestMemory_Seen` -- first call returns false, second returns true
- `TestMemory_Concurrent` -- 100 goroutines racing on the same key; exactly one sees it as new
- `BenchmarkMemory_Seen` -- benchmarks for unique keys and repeated keys

### `internal/dns`

Tests DNS resolution against live DNS servers:

```bash
go test -v ./internal/dns
```

Key test cases:
- `TestResolveAll` -- resolves google.com; checks for IPs, NS records, no trailing dots
- `TestResolveAll_InvalidDomain` -- non-existent domain returns empty results without panic
- `TestResolveAll_ContextCancellation` -- cancelled context returns empty results gracefully
- `BenchmarkResolveAll` -- benchmark for DNS resolution latency

> **Note:** DNS tests make live network calls. They may be flaky in environments without DNS access (some CI containers, air-gapped networks). Consider using `-short` to skip them if needed.

### `internal/rate`

Tests the per-host token bucket rate limiter:

```bash
go test -v ./internal/rate
```

Key test cases:
- `TestPerHost_Allow` -- burst allowance, rate limiting after exhaustion, independent host limits
- `TestPerHost_Wait` -- blocking wait respects rate interval
- `TestPerHost_Concurrent` -- 20 goroutines contend on same host; rate limiting applies
- `TestPerHost_MultipleHosts` -- each host gets its own burst allowance
- `BenchmarkPerHost_Allow` -- single-host and multi-host benchmark

### `internal/robots`

Tests robots.txt caching and TLD exclusion:

```bash
go test -v ./internal/robots
```

Key test cases:
- `TestCache_Get` -- fetches from httptest server, verifies caching returns same instance
- `TestCache_Get_404` -- 404 response returns empty (allow-all) robots data
- `TestShouldSkipByTLD` -- table-driven test for TLD exclusion (gov, mil, int)

## Integration Testing with Redis

Several packages support Redis backends (dedup, queue). Integration tests for these require a running Redis instance.

### Setting Up Redis for Tests

```bash
# Start Redis locally
redis-server &

# Or with Docker
docker run -d --name spyder-redis -p 6379:6379 redis:7-alpine
```

### Running Integration Tests

Set the `REDIS_ADDR` environment variable to enable Redis-backed tests:

```bash
REDIS_ADDR=127.0.0.1:6379 go test -v ./internal/dedup
REDIS_ADDR=127.0.0.1:6379 go test -v ./internal/queue
```

Without `REDIS_ADDR`, the Redis dedup and queue tests are skipped, and only the in-memory implementations are tested.

### Full Integration Test Run

```bash
# Start Redis, run all tests, stop Redis
docker run -d --name spyder-test-redis -p 6379:6379 redis:7-alpine
REDIS_ADDR=127.0.0.1:6379 go test -race -v ./...
docker rm -f spyder-test-redis
```

## Benchmarks

Several packages include benchmarks for performance-sensitive code:

```bash
# Run all benchmarks
go test -bench=. ./...

# Run benchmarks for a specific package
go test -bench=. ./internal/dedup
go test -bench=. ./internal/rate
go test -bench=. ./internal/dns

# Run benchmarks with memory allocation stats
go test -bench=. -benchmem ./internal/dedup

# Run a specific benchmark
go test -bench=BenchmarkMemory_Seen ./internal/dedup
```

Example output:

```
BenchmarkMemory_Seen/UniqueKeys-8    5000000    234 ns/op    48 B/op    1 allocs/op
BenchmarkMemory_Seen/SameKey-8       20000000   62.3 ns/op   0 B/op     0 allocs/op
```

## Writing New Tests

### File Naming

Test files live alongside the code they test and use the `_test.go` suffix:

```
internal/
  extract/
    extract.go
    extract_test.go       # tests for extract.go
  httpclient/
    httpclient.go
    httpclient_test.go    # tests for httpclient.go
```

### Test Function Naming

Follow Go conventions. Test functions start with `Test`, benchmarks with `Benchmark`:

```go
func TestParseLinks(t *testing.T) { ... }
func TestParseLinks_EmptyBody(t *testing.T) { ... }
func BenchmarkParseLinks(b *testing.B) { ... }
```

### Table-Driven Tests

SPYDER uses table-driven tests extensively. Follow this pattern for validation and transformation logic:

```go
func TestShouldSkipByTLD(t *testing.T) {
    excluded := []string{"gov", "mil", "int"}

    tests := []struct {
        host     string
        expected bool
    }{
        {"example.gov", true},
        {"subdomain.example.gov", true},
        {"example.com", false},
        {"gov.example.com", false},
    }

    for _, tt := range tests {
        result := ShouldSkipByTLD(tt.host, excluded)
        if result != tt.expected {
            t.Errorf("ShouldSkipByTLD(%s) = %v, want %v",
                tt.host, result, tt.expected)
        }
    }
}
```

For subtests with names (useful for identifying failures):

```go
func TestValidate(t *testing.T) {
    tests := []struct {
        name    string
        cfg     Config
        wantErr bool
    }{
        {
            name:    "valid config",
            cfg:     Config{Domains: "d.txt", Concurrency: 256, BatchMaxEdges: 10000, BatchFlushSec: 2},
            wantErr: false,
        },
        {
            name:    "missing domains",
            cfg:     Config{Concurrency: 256, BatchMaxEdges: 10000, BatchFlushSec: 2},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.cfg.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Using `httptest` for HTTP Tests

When testing components that make HTTP calls, use `net/http/httptest` to avoid live network dependencies:

```go
func TestCache_Get(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/robots.txt" {
            w.WriteHeader(http.StatusOK)
            w.Write([]byte("User-agent: *\nDisallow: /private/\n"))
        } else {
            w.WriteHeader(http.StatusNotFound)
        }
    }))
    defer server.Close()

    client := &http.Client{Timeout: 2 * time.Second}
    cache := NewCache(client, "TestBot/1.0")
    ctx := context.Background()

    rd, err := cache.Get(ctx, server.URL[7:])
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if rd == nil {
        t.Fatal("expected robots data, got nil")
    }
}
```

### Using `t.TempDir()` for File Tests

For tests that need temporary files (config loading, spool writing):

```go
func TestLoadFromFile_YAML(t *testing.T) {
    yamlContent := `
probe: test-probe
domains: domains.txt
concurrency: 512
`
    tmpDir := t.TempDir()
    configFile := filepath.Join(tmpDir, "config.yaml")
    if err := os.WriteFile(configFile, []byte(yamlContent), 0644); err != nil {
        t.Fatal(err)
    }

    cfg, err := LoadFromFile(configFile)
    if err != nil {
        t.Fatalf("failed to load config: %v", err)
    }
    // assertions...
}
```

### Testing Concurrent Code

Use `sync.WaitGroup` and verify that concurrent access is safe:

```go
func TestMemory_Concurrent(t *testing.T) {
    d := NewMemory()
    var wg sync.WaitGroup
    firstSeen := 0
    var mu sync.Mutex

    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            if !d.Seen("concurrent-key") {
                mu.Lock()
                firstSeen++
                mu.Unlock()
            }
        }()
    }

    wg.Wait()

    if firstSeen != 1 {
        t.Errorf("expected exactly 1 first occurrence, got %d", firstSeen)
    }
}
```

Always run concurrent tests with `-race` to catch data races that might not cause test failures on their own.

## Linting

### Running the Linter

```bash
make lint
```

This runs `golangci-lint run`, which checks for:

- **govet** -- suspicious constructs (e.g., printf format mismatches)
- **staticcheck** -- advanced static analysis
- **errcheck** -- unchecked error returns
- **gosimple** -- code simplifications
- **ineffassign** -- ineffectual variable assignments

### Installing golangci-lint

```bash
# macOS
brew install golangci-lint

# Linux
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Verify
golangci-lint version
```

## CI Pipeline

The GitHub Actions CI pipeline (`.github/workflows/ci.yml`) runs on every push and pull request:

```yaml
name: ci
on:
  push:
  pull_request:
jobs:
  build-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23.x'
      - name: Build
        run: go build -v ./cmd/spyder
      - name: Test
        run: go test ./... -v
```

The CI pipeline:

1. Checks out the repository
2. Sets up Go 1.23.x (matching `go.mod`)
3. Builds the `spyder` binary to verify compilation
4. Runs the full test suite with verbose output

Tests must pass before a pull request can be merged. If a test fails in CI, check the Actions tab on GitHub for the full log output.

## Quick Reference

| Task | Command |
|------|---------|
| Run all tests | `go test ./...` |
| Run all tests (verbose) | `go test -v ./...` |
| Run with coverage | `make test` |
| Run with race detection | `go test -race ./...` |
| Run single package | `go test -v ./internal/dns` |
| Run single test | `go test -v -run TestResolveAll ./internal/dns` |
| Run benchmarks | `go test -bench=. ./...` |
| Coverage HTML report | `go tool cover -html=coverage.txt -o coverage.html` |
| Lint | `make lint` |
| Integration tests | `REDIS_ADDR=127.0.0.1:6379 go test -v ./...` |
