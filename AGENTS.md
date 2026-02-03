# AGENTS.md - MCP Module Multi-Agent Coordination

## Overview

This document provides guidance for AI agents (Claude Code, Copilot, Cursor, etc.)
working on the `digital.vasic.mcp` Go module. It describes the module boundaries,
coordination rules, and development workflow to ensure safe multi-agent collaboration.

## Module Identity

- **Module path**: `digital.vasic.mcp`
- **Language**: Go 1.24+
- **Dependencies**: `github.com/google/uuid`, `github.com/stretchr/testify`
- **Scope**: Generic, reusable MCP (Model Context Protocol) implementation

## Package Ownership

Each package has a well-defined responsibility. Agents should scope changes to the
relevant package and avoid cross-package side effects.

| Package | Responsibility | Key Files |
|---------|---------------|-----------|
| `pkg/protocol` | MCP protocol types, JSON-RPC 2.0 marshaling | `protocol.go` |
| `pkg/client` | Client implementations (stdio, HTTP/SSE) | `client.go`, `stdio_client.go`, `http_client.go` |
| `pkg/server` | Server implementations (stdio, HTTP/SSE) | `server.go`, `stdio_server.go`, `http_server.go` |
| `pkg/registry` | Thread-safe adapter registry with lifecycle | `registry.go` |
| `pkg/config` | Configuration types and file loading | `config.go` |
| `pkg/adapter` | Base adapter types (Stdio, Docker, HTTP) | `adapter.go` |

## Coordination Rules

### 1. Interface Stability

The following interfaces are public contracts. Changes require coordination across
all dependent packages:

- `client.Client` -- used by consumers to interact with MCP servers
- `server.Server` -- implemented by StdioServer and HTTPServer
- `server.ToolHandler`, `server.ResourceHandler`, `server.PromptHandler` -- handler function types
- `registry.Adapter` -- implemented by all adapter types in `pkg/adapter`

**Rule**: Never change an interface signature without updating all implementations
and all call sites within the module.

### 2. Protocol Package is Foundational

`pkg/protocol` is imported by every other package. Changes here affect the entire
module. Agents modifying protocol types must:

- Verify JSON serialization round-trips in `protocol_test.go`
- Check that all consumer packages (client, server, registry, adapter) still compile
- Run `go test ./... -count=1 -race` after any protocol change

### 3. Transport Symmetry

The client and server packages mirror each other across two transport types:

- **Stdio**: `StdioClient` <-> `StdioServer` (newline-delimited JSON over stdin/stdout)
- **HTTP/SSE**: `HTTPClient` <-> `HTTPServer` (HTTP POST for requests, SSE for responses)

When adding features to one transport, ensure the other transport is updated
accordingly.

### 4. Concurrency Safety

All public types use appropriate synchronization:

- `StdioClient`: `sync.Mutex` for stdin writes, `sync.RWMutex` for pending map
- `HTTPClient`: `sync.RWMutex` for pending map, `sync.Once` for close
- `HTTPServer`: `sync.RWMutex` for SSE client map, `atomic.Int64` for connection count
- `Registry`: `sync.RWMutex` for adapter map

Agents must maintain these concurrency guarantees. Run `go test ./... -race` to verify.

### 5. Error Wrapping

All errors must be wrapped with `fmt.Errorf("context: %w", err)` to preserve the
error chain. Never return bare errors from public functions.

## Development Workflow

### Before Making Changes

```bash
# Verify the module builds cleanly
go build ./...

# Run all tests with race detection
go test ./... -count=1 -race

# Run only unit tests (fast feedback)
go test ./... -short
```

### After Making Changes

```bash
# Format code
gofmt -w .

# Vet for common issues
go vet ./...

# Run full test suite
go test ./... -count=1 -race

# Check for compile-time interface compliance
go build ./...
```

### Adding a New MCP Method

1. Define any new types in `pkg/protocol/protocol.go`
2. Add the method case to `handleRequest()` in `pkg/server/server.go`
3. Implement the handler function in `pkg/server/server.go`
4. Add the method to the `client.Client` interface in `pkg/client/client.go`
5. Implement in `StdioClient` (`pkg/client/stdio_client.go`)
6. Implement in `HTTPClient` (`pkg/client/http_client.go`)
7. Add tests for both server implementations and both client implementations

### Adding a New Adapter Type

1. Create the adapter struct in `pkg/adapter/adapter.go`
2. Embed `BaseAdapter` for common state management
3. Implement the `registry.Adapter` interface: `Name()`, `Config()`, `Start()`, `Stop()`, `HealthCheck()`
4. Add a factory function `NewXxxAdapter()`
5. Add tests verifying lifecycle state transitions

## File Modification Safety

### Safe to Modify Independently

- Test files (`*_test.go`) -- can be modified without coordination
- Internal (unexported) types and functions within a single package
- Adding new exported functions that do not change existing signatures

### Requires Cross-Package Verification

- Any exported type in `pkg/protocol` (affects all packages)
- Interface definitions in `pkg/client`, `pkg/server`, `pkg/registry`
- `config.ServerConfig` or `config.ContainerConfig` fields (affects adapter package)

### Never Modify

- `go.mod` module path (`digital.vasic.mcp`)
- JSON struct tags on protocol types (breaks wire compatibility)
- JSON-RPC version constant (`JSONRPCVersion = "2.0"`)

## Testing Conventions

- Table-driven tests using `testify/assert` and `testify/require`
- Test naming: `Test<Struct>_<Method>_<Scenario>`
- Compile-time interface checks: `var _ Interface = (*Struct)(nil)`
- Use `SetIO()` on StdioServer for deterministic testing without real stdin/stdout
- Use `httptest.NewRecorder()` for HTTP server handler testing

## Agent Communication

When multiple agents work on this module concurrently:

1. **Claim scope**: State which package(s) you are modifying
2. **Check interfaces first**: Before changing a type, list all consumers
3. **Run tests last**: Always verify `go test ./... -race` passes before finalizing
4. **Document changes**: Update relevant doc comments and this file if adding packages
