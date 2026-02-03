# Contributing

Thank you for your interest in contributing to the `digital.vasic.mcp` module.
This guide covers the development workflow, coding standards, and submission process.

## Prerequisites

- Go 1.24 or later
- Git
- Docker (for testing Docker adapter functionality)

## Getting Started

```bash
# Clone the repository
git clone <repository-url>
cd MCP_Module

# Verify the build
go build ./...

# Run all tests
go test ./... -count=1 -race
```

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feat/my-feature
# or: fix/my-bugfix, refactor/my-cleanup, docs/my-docs, test/my-tests
```

Branch naming conventions:

| Prefix | Purpose |
|--------|---------|
| `feat/` | New feature |
| `fix/` | Bug fix |
| `refactor/` | Code restructuring without behavior change |
| `docs/` | Documentation only |
| `test/` | Test additions or fixes |
| `chore/` | Build, CI, or tooling changes |

### 2. Make Changes

Follow the coding standards described below. Keep commits focused and atomic.

### 3. Run Quality Checks

```bash
# Format code
gofmt -w .

# Vet for issues
go vet ./...

# Run tests with race detection
go test ./... -count=1 -race

# Run only unit tests (fast)
go test ./... -short

# Run benchmarks
go test -bench=. ./...
```

### 4. Commit

Use Conventional Commits format:

```
<type>(<scope>): <description>

[optional body]
```

Examples:

```
feat(protocol): add streaming content block type
fix(client): handle SSE reconnection on connection drop
test(server): add benchmark for handleRequest dispatch
docs(api): document HTTPServer.Handler() usage
refactor(adapter): extract state machine to separate type
```

Valid types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `perf`

Valid scopes: `protocol`, `client`, `server`, `registry`, `config`, `adapter`

### 5. Submit

Push your branch and open a pull request. Ensure all tests pass in CI before
requesting review.

## Coding Standards

### Go Conventions

- Follow standard Go conventions per [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` formatting (no exceptions)
- Group imports: stdlib, third-party, internal (blank-line separated)
- Line length: 100 characters or fewer for readability

### Naming

| Element | Convention | Example |
|---------|-----------|---------|
| Exported types | PascalCase | `StdioClient`, `ToolHandler` |
| Unexported types | camelCase | `toolEntry`, `sseClient` |
| Constants | PascalCase or UPPER_SNAKE | `CodeParseError`, `StateRunning` |
| Acronyms | All-caps | `HTTP`, `SSE`, `URL`, `ID`, `JSON`, `RPC` |
| Receivers | 1-2 letters | `s` for server, `c` for client, `r` for registry |

### Error Handling

- Always check errors. Never ignore them with `_` in production code.
- Wrap errors with context: `fmt.Errorf("failed to start process: %w", err)`
- Use `defer` for cleanup operations
- Return `error` as the last return value

### Concurrency

- Accept `context.Context` as the first parameter of methods that perform I/O
- Protect shared state with `sync.Mutex` or `sync.RWMutex`
- Use `sync.Once` for one-time cleanup
- Use `atomic` operations for simple counters
- Document thread-safety guarantees in doc comments

### Testing

- Table-driven tests using `testify/assert` and `testify/require`
- Test naming: `Test<Struct>_<Method>_<Scenario>`
- Include compile-time interface compliance checks:
  ```go
  var _ Server = (*StdioServer)(nil)
  ```
- Use `SetIO()` for deterministic stdio server testing
- Use `httptest.NewRecorder()` for HTTP handler testing
- Aim for comprehensive coverage of both success and error paths

### Documentation

- All exported types, functions, methods, and constants must have doc comments
- Doc comments start with the name of the element:
  ```go
  // NewStdioServer creates a new stdio-based MCP server.
  func NewStdioServer(name, version string) *StdioServer {
  ```
- Package comments use `// Package <name>` format:
  ```go
  // Package protocol provides MCP protocol types and
  // JSON-RPC 2.0 message marshaling/unmarshaling.
  package protocol
  ```

## What to Contribute

### Encouraged

- Bug fixes with regression tests
- Performance improvements with benchmarks
- New adapter types (e.g., Podman, Kubernetes)
- Additional MCP method support as the protocol evolves
- Test coverage improvements
- Documentation clarifications and examples

### Requires Discussion First

- Changes to public interfaces (`Client`, `Server`, `Adapter`)
- New package additions
- Protocol type modifications (wire format changes)
- New external dependencies

### Not Accepted

- Changes that break existing public API without a migration path
- Dependencies on application-specific code (this module must remain generic)
- Changes that reduce test coverage
- Code that does not pass `gofmt`, `go vet`, and `go test -race`

## Code Review Checklist

Reviewers will check for:

- [ ] Tests pass with `go test ./... -count=1 -race`
- [ ] Code formatted with `gofmt`
- [ ] No `go vet` warnings
- [ ] Doc comments on all exported symbols
- [ ] Error handling follows wrapping conventions
- [ ] Concurrency safety maintained
- [ ] No breaking changes to public API (or migration path provided)
- [ ] Conventional Commit message format
