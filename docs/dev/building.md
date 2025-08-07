# Building SPYDER from Source

This guide covers building SPYDER from source code, including development environment setup, build processes, and cross-compilation.

## Prerequisites

### Go Installation

**Install Go 1.22 or later:**

```bash
# Linux
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# macOS (with Homebrew)
brew install go

# Verify installation
go version
```

### Development Dependencies

```bash
# Build tools
sudo apt update
sudo apt install -y git make gcc

# Optional: Development tools
sudo apt install -y golangci-lint  # Linting
go install github.com/air-verse/air@latest  # Hot reload
```

## Repository Setup

### Clone Repository

```bash
git clone https://github.com/gustycube/spyder-probe.git
cd spyder-probe

# Verify repository structure
ls -la
```

### Dependency Management

```bash
# Download dependencies
go mod download

# Verify dependencies
go mod verify

# Update dependencies (if needed)
go get -u all
go mod tidy
```

## Build Processes

### Standard Build

**Build main binary:**

```bash
# Build for current platform
go build -o bin/spyder ./cmd/spyder

# Verify build
./bin/spyder -h
```

**Build with version information:**

```bash
# Set version variables
VERSION=$(git describe --tags --always --dirty)
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

go build -ldflags "-X main.version=$VERSION -X main.commit=$COMMIT -X main.date=$DATE" \
  -o bin/spyder ./cmd/spyder

# Check version info
./bin/spyder -version
```

### Optimized Production Build

**Release build with optimizations:**

```bash
# Production build
CGO_ENABLED=0 go build \
  -ldflags "-s -w -X main.version=$(git describe --tags --always)" \
  -o bin/spyder ./cmd/spyder

# Static binary (Linux)
CGO_ENABLED=0 GOOS=linux go build \
  -ldflags "-s -w -extldflags '-static'" \
  -a -installsuffix cgo \
  -o bin/spyder-linux ./cmd/spyder
```

**Build flags explained:**

- `-ldflags "-s -w"`: Strip debug info and symbols
- `CGO_ENABLED=0`: Disable CGO for static linking
- `-a`: Force rebuilding of packages
- `-installsuffix cgo`: Add suffix for CGO builds

### Development Build

**Debug build with race detection:**

```bash
# Debug build
go build -race -o bin/spyder-debug ./cmd/spyder

# Build with debug symbols
go build -gcflags="all=-N -l" -o bin/spyder-debug ./cmd/spyder
```

## Cross-Platform Compilation

### Supported Platforms

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o bin/spyder-linux-amd64 ./cmd/spyder

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o bin/spyder-linux-arm64 ./cmd/spyder

# macOS AMD64
GOOS=darwin GOARCH=amd64 go build -o bin/spyder-darwin-amd64 ./cmd/spyder

# macOS ARM64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o bin/spyder-darwin-arm64 ./cmd/spyder

# Windows AMD64
GOOS=windows GOARCH=amd64 go build -o bin/spyder-windows-amd64.exe ./cmd/spyder
```

### Batch Cross-Compilation

**Build script for multiple platforms:**

```bash
#!/bin/bash
# build-all.sh

set -e

VERSION=${1:-$(git describe --tags --always --dirty)}
PLATFORMS=(
    "linux/amd64"
    "linux/arm64" 
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

echo "Building SPYDER $VERSION for multiple platforms..."

for platform in "${PLATFORMS[@]}"; do
    GOOS=${platform%/*}
    GOARCH=${platform#*/}
    
    output="bin/spyder-$GOOS-$GOARCH"
    if [[ $GOOS == "windows" ]]; then
        output="$output.exe"
    fi
    
    echo "Building $output..."
    
    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags "-s -w -X main.version=$VERSION" \
        -o "$output" ./cmd/spyder
        
    if [[ $? -eq 0 ]]; then
        echo "✓ Built $output"
    else
        echo "✗ Failed to build $output"
        exit 1
    fi
done

echo "All builds completed successfully"
```

## Make Targets

### Makefile Usage

The project includes a Makefile with common build targets:

```bash
# Build binary
make build

# Run linting
make lint

# Run tests
make test

# Build Docker image
make docker

# Run SPYDER with default config
make run

# Start documentation server
make docs
```

### Custom Make Targets

**Add to Makefile:**

```makefile
# Production build
.PHONY: build-prod
build-prod:
	CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/spyder ./cmd/spyder

# Debug build
.PHONY: build-debug
build-debug:
	go build -race -gcflags="all=-N -l" -o bin/spyder-debug ./cmd/spyder

# Cross-platform builds
.PHONY: build-all
build-all:
	./scripts/build-all.sh

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf bin/
	go clean -cache
```

## Docker Build

### Multi-stage Dockerfile

The project uses a multi-stage build:

```dockerfile
FROM golang:1.22 AS build
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -o /spyder ./cmd/spyder

FROM gcr.io/distroless/base-debian12
USER nonroot:nonroot
COPY --from=build /spyder /usr/local/bin/spyder
ENTRYPOINT ["/usr/local/bin/spyder"]
```

### Docker Build Commands

```bash
# Build Docker image
docker build -t spyder-probe:latest .

# Build with version tag
VERSION=$(git describe --tags --always)
docker build -t spyder-probe:$VERSION .

# Multi-platform Docker build
docker buildx build --platform linux/amd64,linux/arm64 \
  -t spyder-probe:latest .
```

## Development Workflow

### Hot Reload Development

**Using Air for hot reload:**

```bash
# Install Air
go install github.com/air-verse/air@latest

# Create Air configuration
cat > .air.toml << EOF
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = ["-domains=configs/domains.txt"]
  bin = "./tmp/spyder"
  cmd = "go build -o ./tmp/spyder ./cmd/spyder"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html"]
  kill_delay = "0s"
  log = "build-errors.log"
  send_interrupt = false
  stop_on_root = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  time = false

[misc]
  clean_on_exit = false
EOF

# Start development server
air
```

### Code Generation

**Generate mocks (if using testify):**

```bash
go install github.com/vektra/mockery/v2@latest

# Generate mocks
mockery --all --output mocks/ --case snake
```

### Testing During Development

```bash
# Run tests with coverage
go test -v -cover ./...

# Run specific test
go test -v -run TestCrawlOne ./internal/probe

# Benchmark tests
go test -bench=. ./internal/extract

# Test with race detection
go test -race ./...
```

## Build Optimization

### Binary Size Optimization

**Minimize binary size:**

```bash
# UPX compression (install upx first)
sudo apt install upx-ucl

# Build and compress
CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/spyder ./cmd/spyder
upx --best --lzma bin/spyder

# Check size reduction
ls -lh bin/spyder*
```

### Build Performance

**Improve build speed:**

```bash
# Use build cache
export GOCACHE=$HOME/.cache/go-build

# Parallel builds
export GOMAXPROCS=8

# Module proxy for faster downloads
export GOPROXY=https://proxy.golang.org,direct
```

## Continuous Integration

### GitHub Actions Build

**.github/workflows/build.yml:**

```yaml
name: Build
on: [push, pull_request]

jobs:
  build:
    runs-on: ubuntu-latest
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v3
      with:
        go-version: '1.22'
    
    - name: Download dependencies
      run: go mod download
      
    - name: Run tests
      run: go test -v -race ./...
      
    - name: Build binary
      run: go build -v ./cmd/spyder
      
    - name: Cross-compile
      run: |
        GOOS=linux GOARCH=amd64 go build -o spyder-linux-amd64 ./cmd/spyder
        GOOS=darwin GOARCH=amd64 go build -o spyder-darwin-amd64 ./cmd/spyder
        GOOS=windows GOARCH=amd64 go build -o spyder-windows-amd64.exe ./cmd/spyder
```

## Troubleshooting Build Issues

### Common Problems

**Dependency issues:**

```bash
# Clear module cache
go clean -modcache

# Re-download dependencies
rm go.sum
go mod download
```

**Build fails with missing packages:**

```bash
# Verify Go installation
go version

# Check GOPATH and GOROOT
go env GOPATH GOROOT

# Reinstall dependencies
go mod tidy
go mod download
```

**Cross-compilation issues:**

```bash
# Install cross-compilation support
go env GOOS GOARCH

# List supported platforms
go tool dist list
```

### Debug Build Issues

```bash
# Verbose build output
go build -v ./cmd/spyder

# Show build commands
go build -x ./cmd/spyder

# Check for race conditions
go build -race ./cmd/spyder
```

This comprehensive build guide ensures successful compilation of SPYDER across all supported platforms and development environments.