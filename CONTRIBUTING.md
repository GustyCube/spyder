# Contributing to SPYDER

Thank you for your interest in contributing to SPYDER (System for Probing and Yielding DNS-based Entity Relations)! This document provides guidelines and information for contributors.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Contributing Guidelines](#contributing-guidelines)
- [Pull Request Process](#pull-request-process)
- [Testing](#testing)
- [Security](#security)

## Code of Conduct

By participating in this project, you agree to abide by our Code of Conduct:

- Be respectful and inclusive
- Focus on constructive feedback
- Help maintain a welcoming environment for all contributors
- Report any unacceptable behavior to the project maintainers

## Getting Started

### Prerequisites

- Go 1.22 or higher
- Docker (for containerized development/testing)
- Git
- Basic understanding of networking, DNS, and web crawling concepts

### Development Setup

1. **Fork and Clone**
   ```bash
   git clone https://github.com/yourusername/spyder.git
   cd spyder
   ```

2. **Install Dependencies**
   ```bash
   go mod download
   go mod tidy
   ```

3. **Build the Project**
   ```bash
   go build -o bin/spyder ./cmd/spyder
   go build -o bin/seed ./cmd/seed
   ```

4. **Run Basic Tests**
   ```bash
   go test ./...
   ```

## Project Structure

```
spyder/
├── cmd/
│   ├── spyder/          # Main probe application
│   └── seed/            # Redis queue seeding utility
├── internal/
│   ├── dedup/           # Deduplication (memory/Redis)
│   ├── dns/             # DNS resolution
│   ├── emit/            # Batch emission and delivery
│   ├── extract/         # Link extraction and parsing
│   ├── httpclient/      # HTTP client configuration
│   ├── logging/         # Structured logging (zap)
│   ├── metrics/         # Prometheus metrics
│   ├── probe/           # Core crawling logic
│   ├── queue/           # Redis queue management
│   ├── rate/            # Rate limiting
│   ├── robots/          # robots.txt handling
│   ├── telemetry/       # OpenTelemetry integration
│   └── tlsinfo/         # TLS certificate analysis
├── configs/             # Configuration examples
├── docs/                # Documentation
└── scripts/             # Deployment scripts
```

## Contributing Guidelines

### Types of Contributions

We welcome the following types of contributions:

- **Bug Reports**: Help us identify and fix issues
- **Feature Requests**: Suggest new functionality
- **Code Contributions**: Bug fixes, features, optimizations
- **Documentation**: Improve docs, examples, and guides
- **Testing**: Add tests, improve test coverage
- **Performance**: Optimizations and benchmarking

### Before Contributing

1. **Search Existing Issues**: Check if your bug/feature is already reported
2. **Create an Issue**: Discuss significant changes before implementing
3. **Security Issues**: Report security vulnerabilities privately

### Code Style

- Follow standard Go conventions (`go fmt`, `go vet`)
- Use meaningful variable and function names
- Add comments for complex logic
- Keep functions focused and reasonably sized
- Follow the project's existing patterns

### Commit Guidelines

- Use clear, descriptive commit messages
- Follow conventional commits format when possible:
  ```
  type(scope): description
  
  Examples:
  feat(dns): add IPv6 resolution support
  fix(robots): handle malformed robots.txt files
  docs(readme): update installation instructions
  ```

- Keep commits focused on a single change
- Reference issue numbers in commit messages

## Pull Request Process

### Before Submitting

1. **Update Your Fork**
   ```bash
   git checkout main
   git pull upstream main
   git push origin main
   ```

2. **Create a Feature Branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

3. **Make Your Changes**
   - Write clean, tested code
   - Update documentation if needed
   - Add/update tests for new functionality

4. **Test Your Changes**
   ```bash
   go test ./...
   go build ./cmd/spyder
   go build ./cmd/seed
   ```

5. **Lint Your Code**
   ```bash
   golangci-lint run  # if available
   go vet ./...
   go fmt ./...
   ```

### Submitting the PR

1. **Push Your Branch**
   ```bash
   git push origin feature/your-feature-name
   ```

2. **Create Pull Request**
   - Use a descriptive title
   - Fill out the PR template
   - Link to related issues
   - Describe your changes and testing approach

3. **PR Requirements**
   - All CI checks must pass
   - Code review approval required
   - No merge conflicts
   - Documentation updated if needed

### PR Review Process

- Maintainers will review within 48-72 hours
- Address feedback promptly
- Keep discussions focused and constructive
- Be patient during the review process

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests for specific package
go test ./internal/probe/
```

### Writing Tests

- Add tests for new functionality
- Include edge cases and error conditions
- Use table-driven tests for multiple scenarios
- Mock external dependencies when appropriate

### Integration Testing

- Test with real DNS queries (use public domains)
- Respect rate limits during testing
- Clean up test artifacts
- Use environment variables for test configuration

## Security

### Security Considerations

SPYDER is a network security tool that interacts with external systems. Please consider:

- **Rate Limiting**: Ensure respectful crawling practices
- **robots.txt Compliance**: Maintain ethical crawling behavior  
- **Data Privacy**: Handle collected data responsibly
- **Resource Usage**: Prevent DoS conditions
- **Input Validation**: Sanitize user inputs and external data

### Reporting Security Issues

**Do not report security vulnerabilities through public GitHub issues.**

Instead, please report them privately to:
- Email the maintainers directly
- Use GitHub's private vulnerability reporting feature

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact assessment
- Suggested fixes (if any)

## Development Tips

### Local Testing

```bash
# Test with a small domain list
echo -e "example.com\nhttpbin.org" > test-domains.txt
./bin/spyder -domains=test-domains.txt -concurrency=2

# Test with Redis (requires Redis server)
REDIS_ADDR=localhost:6379 ./bin/spyder -domains=test-domains.txt

# Test Docker build
docker build -t spyder-test .
```

### Debugging

- Use structured logging extensively
- Add debug flags for development
- Test with different concurrency levels
- Monitor resource usage during development

### Performance

- Profile with `go tool pprof` for bottlenecks
- Test with larger domain lists
- Monitor memory usage and leaks
- Benchmark critical code paths

## Questions and Support

- **General Questions**: Open a GitHub Discussion
- **Bug Reports**: Create a GitHub Issue
- **Feature Requests**: Create a GitHub Issue with the enhancement label
- **Security Issues**: Report privately as described above

## Recognition

Contributors will be recognized in:
- GitHub contributors list
- Release notes for significant contributions
- Special mention for major features or fixes

Thank you for contributing to SPYDER! Your efforts help make internet mapping more accessible and secure.