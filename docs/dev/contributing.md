# Contributing to SPYDER

Thank you for your interest in contributing to SPYDER. This guide covers the workflow, standards, and expectations for all contributions.

## Getting Started

### Fork and Clone

1. **Fork** the repository on GitHub: [github.com/gustycube/spyder-probe](https://github.com/gustycube/spyder-probe)

2. **Clone** your fork locally:

   ```bash
   git clone https://github.com/<your-username>/spyder-probe.git
   cd spyder-probe
   ```

3. **Add the upstream remote** to stay in sync:

   ```bash
   git remote add upstream https://github.com/gustycube/spyder-probe.git
   git fetch upstream
   ```

4. **Install dependencies:**

   ```bash
   go mod download
   go mod verify
   ```

5. **Verify the build:**

   ```bash
   go build -o bin/spyder ./cmd/spyder
   go test ./...
   ```

### Prerequisites

- **Go 1.23 or later** (check with `go version`)
- **golangci-lint** for linting (see [installation](#installing-golangci-lint))
- **Redis** (optional, only needed for integration tests)
- **Git** with a configured user name and email

## Branch Naming

Create a branch from `main` with a descriptive prefix:

| Prefix | Purpose | Example |
|--------|---------|---------|
| `feature/` | New functionality | `feature/dns-over-https` |
| `fix/` | Bug fixes | `fix/robots-cache-ttl` |
| `docs/` | Documentation changes | `docs/add-helm-chart-guide` |
| `refactor/` | Code restructuring without behavior change | `refactor/emit-batch-pool` |
| `test/` | Adding or improving tests | `test/probe-integration` |

```bash
# Sync with upstream before branching
git fetch upstream
git checkout main
git merge upstream/main

# Create your branch
git checkout -b feature/dns-over-https
```

## Code Style

### Formatting

All Go code must be formatted with `gofmt`. This is non-negotiable -- unformatted code will fail CI.

```bash
# Format all files
gofmt -w .

# Check formatting without modifying files
gofmt -l .
```

Most editors (VS Code with gopls, GoLand, Vim with vim-go) run `gofmt` on save automatically.

### Linting

SPYDER uses `golangci-lint` with the following linters enabled:

- **govet** -- reports suspicious constructs like printf calls with mismatched format strings
- **staticcheck** -- advanced static analysis covering correctness, performance, and style
- **errcheck** -- ensures error return values are not silently discarded
- **gosimple** -- suggests simpler code constructions
- **ineffassign** -- detects assignments to variables that are never read afterward

Run the linter before committing:

```bash
make lint
```

Or directly:

```bash
golangci-lint run
```

### Installing golangci-lint

```bash
# macOS
brew install golangci-lint

# Linux / CI
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Verify installation
golangci-lint version
```

### Code Conventions

Follow standard Go conventions:

- **Exported names** use PascalCase: `ResolveAll`, `NewEmitter`, `Batch`
- **Unexported names** use camelCase: `flush`, `append`, `spool`
- **Interfaces** use `-er` or descriptive names: `dedup.Interface`, `discover.Sink`
- **Error variables** use `Err` prefix: `ErrOpenState`
- **Constants** use PascalCase: `EdgeResolvesTo`, `FormatJSON`
- **Package names** are short, lowercase, single-word: `dns`, `emit`, `rate`, `probe`

Keep functions focused. If a function exceeds ~50 lines, consider whether it can be decomposed.

Handle errors explicitly. Never discard errors from operations that can fail meaningfully:

```go
// Bad: silently discarding an error
data, _ := json.Marshal(batch)

// Good: handle or propagate
data, err := json.Marshal(batch)
if err != nil {
    return fmt.Errorf("encoding batch: %w", err)
}
```

Use `fmt.Errorf` with `%w` to wrap errors for context while preserving the error chain.

## Commit Conventions

SPYDER uses [Conventional Commits](https://www.conventionalcommits.org/). Every commit message must follow this format:

```
<type>(<scope>): <description>
```

### Commit Types

| Type | When to use | Example |
|------|-------------|---------|
| `feat` | New feature or capability | `feat(dns): add DOH support` |
| `fix` | Bug fix | `fix(robots): handle 403 responses` |
| `docs` | Documentation only | `docs(README): update install instructions` |
| `chore` | Maintenance, dependencies | `chore: upgrade go-redis to v9.8` |
| `refactor` | Code change that neither fixes a bug nor adds a feature | `refactor(emit): extract batch accumulator` |
| `test` | Adding or correcting tests | `test(dedup): add Redis integration test` |
| `perf` | Performance improvement | `perf(extract): reduce allocations in link parser` |

### Scope

The scope is optional but encouraged. Use the package name or component:

```
feat(probe): add per-domain timeout configuration
fix(circuitbreaker): reset failure count on half-open success
docs(config): document YAML config file format
refactor(emit): simplify spool replay logic
```

### Description

- Use imperative mood: "add", "fix", "remove" (not "added", "fixes", "removed")
- Do not capitalize the first letter
- Do not end with a period
- Keep under 72 characters

### Multi-line Commits

For complex changes, add a body after a blank line:

```
feat(discover): add RedisSink for distributed domain discovery

Implements a Sink that pushes discovered domains into a Redis queue
instead of an in-memory channel. This allows multiple probe instances
to share discovered domains across a cluster.

Closes #42
```

## Pull Request Process

### Before Submitting

1. **Sync with upstream:**

   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run the full test suite:**

   ```bash
   go test -race ./...
   ```

3. **Run the linter:**

   ```bash
   make lint
   ```

4. **Verify the build:**

   ```bash
   go build -o bin/spyder ./cmd/spyder
   go build -o bin/seed ./cmd/seed
   ```

5. **Check for unformatted code:**

   ```bash
   gofmt -l .
   ```

   This should produce no output. If it does, run `gofmt -w .` and commit the changes.

### Creating the Pull Request

1. Push your branch to your fork:

   ```bash
   git push origin feature/dns-over-https
   ```

2. Open a pull request against `main` on the upstream repository.

3. Fill in the PR template:
   - **Title:** Short, descriptive summary (under 70 characters)
   - **Summary:** What changed and why
   - **Testing:** How you tested the changes
   - **Breaking changes:** Note any backward-incompatible changes

### PR Title Examples

```
feat(dns): add DNS-over-HTTPS resolver option
fix(emit): prevent panic on nil batch flush
docs: add Kubernetes deployment guide
refactor(probe): extract CrawlOne into smaller functions
test(rate): add benchmark for multi-host limiter
```

### What to Expect During Review

- A maintainer will review your PR, typically within a few days
- You may receive requests for changes -- this is normal and constructive
- CI must pass (build + tests) before merging
- Approval from at least one maintainer is required

## Code Review Expectations

### For Authors

- **Keep PRs focused.** One logical change per PR. If you find yourself writing "also" in the PR description, consider splitting it.
- **Write tests for new code.** New functions and packages should have corresponding test coverage.
- **Update documentation** if your change affects user-facing behavior, configuration options, or APIs.
- **Respond to feedback promptly.** If you disagree with a suggestion, explain your reasoning rather than ignoring it.

### For Reviewers

Reviews should check for:

- **Correctness:** Does the code do what it claims? Are edge cases handled?
- **Concurrency safety:** Does new code that runs in goroutines use proper synchronization? Test with `-race`.
- **Error handling:** Are errors propagated or logged, not silently discarded?
- **Performance:** Does the change introduce unnecessary allocations, unbounded growth, or blocking calls in hot paths?
- **Consistency:** Does the code follow existing patterns in the codebase?
- **Tests:** Are there tests? Do they cover the interesting cases (not just the happy path)?

## Adding a New Internal Package

If your contribution requires a new package under `internal/`:

1. Create the package directory:

   ```bash
   mkdir internal/newpkg
   ```

2. Follow the existing package structure:

   ```
   internal/newpkg/
     newpkg.go          # implementation
     newpkg_test.go     # tests
   ```

3. Keep the package name short and descriptive (one word if possible).

4. Define interfaces for components that may have multiple implementations (see `dedup.Interface`, `discover.Sink` as examples).

5. Add the package to the [Code Structure](structure.md) documentation.

## Development Workflow Summary

```bash
# 1. Fork and clone
git clone https://github.com/<you>/spyder-probe.git
cd spyder-probe
git remote add upstream https://github.com/gustycube/spyder-probe.git

# 2. Create a branch
git fetch upstream && git checkout -b feature/my-change upstream/main

# 3. Make changes, write tests
# ... edit files ...

# 4. Verify
gofmt -w .
make lint
go test -race ./...

# 5. Commit
git add -A
git commit -m "feat(pkg): short description of change"

# 6. Push and open PR
git push origin feature/my-change
# Open PR on GitHub
```

## License

SPYDER is licensed under the [MIT License](https://opensource.org/licenses/MIT). By submitting a contribution, you agree that your work will be licensed under the same terms.

All source files should not include license headers -- the repository-level `LICENSE` file applies to all contents.
