# Architecture

This document describes the design decisions, architectural patterns, and internal
structure of the `digital.vasic.mcp` module.

## Design Goals

1. **Generic and reusable** -- no dependency on HelixAgent or any specific application
2. **Transport-agnostic** -- support stdio and HTTP/SSE with a unified interface
3. **Minimal dependencies** -- only `google/uuid` and `stretchr/testify` (test-only)
4. **Concurrency-safe** -- all shared state protected by appropriate synchronization
5. **Composable** -- packages can be used independently or together

## Package Dependency Graph

```
pkg/protocol  (foundation -- no internal deps)
    ^
    |
    +-- pkg/config    (uses protocol.TransportType concept, no direct import)
    |       ^
    |       |
    +-- pkg/client    (imports protocol)
    |
    +-- pkg/server    (imports protocol, uuid)
    |
    +-- pkg/adapter   (imports config)
    |       ^
    |       |
    +-- pkg/registry  (standalone, defines Adapter interface)
```

The dependency flow is strictly one-directional. `pkg/protocol` is the foundation
layer imported by `pkg/client` and `pkg/server`. The `pkg/config` package is
imported by `pkg/adapter`. The `pkg/registry` package defines its own `Adapter`
interface and has no internal dependencies, allowing adapter types from `pkg/adapter`
to satisfy it without a circular import.

## Design Patterns

### 1. Adapter Pattern

The `pkg/adapter` package implements the Adapter pattern, providing a unified
lifecycle interface for three fundamentally different MCP server deployment types:

- **StdioAdapter** -- manages a local subprocess communicating via stdin/stdout
- **DockerAdapter** -- manages a Docker container with port mapping and health checks
- **HTTPAdapter** -- wraps an external HTTP-based MCP server with health monitoring

All three embed `BaseAdapter`, which provides common state management (the state
machine) and configuration access. Each adapter implements `Start()`, `Stop()`, and
`HealthCheck()` according to its deployment model.

```go
type BaseAdapter struct {
    AdapterName string
    ServerCfg   config.ServerConfig
    state       State
    mu          sync.RWMutex
}
```

### 2. Factory Pattern

Each concrete type has a dedicated factory function that validates inputs and
returns a properly initialized instance:

| Factory | Returns | Purpose |
|---------|---------|---------|
| `server.NewStdioServer(name, version)` | `*StdioServer` | Stdio MCP server |
| `server.NewHTTPServer(name, version, cfg)` | `*HTTPServer` | HTTP/SSE MCP server |
| `client.NewStdioClient(cfg)` | `*StdioClient` | Stdio MCP client |
| `client.NewHTTPClient(cfg)` | `*HTTPClient` | HTTP/SSE MCP client |
| `adapter.NewStdioAdapter(name, cfg)` | `*StdioAdapter` | Stdio adapter |
| `adapter.NewDockerAdapter(name, srv, ctr)` | `*DockerAdapter` | Docker adapter |
| `adapter.NewHTTPAdapter(name, cfg)` | `*HTTPAdapter` | HTTP adapter |
| `registry.New()` | `*Registry` | Adapter registry |
| `config.LoadFromFile(path)` | `*FileConfig` | Config from file |

Factory functions return errors when required parameters are missing (e.g.,
`NewStdioClient` requires `ServerCommand`; `NewHTTPClient` requires `ServerURL`).

### 3. Registry Pattern

The `pkg/registry` package provides a thread-safe, named collection of adapters
with batch lifecycle operations:

```
Register(adapter) -> StartAll(ctx) -> HealthCheckAll(ctx) -> StopAll(ctx)
                     ^                                         |
                     |           Unregister(name)              |
                     +-----------------------------------------+
```

Key properties:

- **Thread-safe**: All operations use `sync.RWMutex`
- **Name-unique**: Duplicate registration returns an error
- **Batch operations**: `StartAll`, `StopAll`, `HealthCheckAll` operate on snapshots
  of the adapter list to avoid holding the lock during I/O
- **Error collection**: `StopAll` collects all errors rather than failing fast,
  ensuring all adapters get a chance to shut down

### 4. Facade Pattern

The `server.Server` interface acts as a facade, hiding the complexity of JSON-RPC
message handling, request routing, and transport management behind three simple
registration methods:

```go
type Server interface {
    RegisterTool(tool protocol.Tool, handler ToolHandler)
    RegisterResource(resource protocol.Resource, handler ResourceHandler)
    RegisterPrompt(prompt protocol.Prompt, handler PromptHandler)
    Serve(ctx context.Context) error
    ServerInfo() protocol.ServerInfo
    Capabilities() protocol.ServerCapabilities
}
```

Consumers register handlers and call `Serve()`. The server handles:

- JSON-RPC 2.0 request parsing and validation
- Method routing to the correct handler
- Response serialization
- Transport-specific details (stdio scanning, HTTP routing, SSE broadcasting)

### 5. Strategy Pattern (Transport)

Both client and server use the Strategy pattern for transport selection:

**Client side**: The `client.Client` interface defines transport-agnostic operations.
`StdioClient` and `HTTPClient` implement it using different communication strategies.
Consumers program against `client.Client` and select the transport at construction
time.

**Server side**: `StdioServer` and `HTTPServer` both implement `server.Server`.
The shared `handleRequest()` function contains all business logic (method dispatch),
while each server type provides only transport-specific I/O.

## Request Processing Flow

### Stdio Transport

```
Client                          Server
  |                               |
  |-- JSON line over stdin ------>|
  |                               |-- parse JSON-RPC request
  |                               |-- route to handler
  |                               |-- execute handler
  |                               |-- serialize response
  |<-- JSON line over stdout -----|
```

### HTTP/SSE Transport

```
Client                          Server
  |                               |
  |-- GET /sse ------------------>|-- establish SSE stream
  |<-- event: endpoint -----------|
  |                               |
  |-- POST /message (JSON-RPC) -->|-- parse request
  |<-- HTTP 200 (JSON-RPC) ------|-- route + handle
  |<-- SSE event: message --------|-- broadcast to SSE clients
```

## Concurrency Model

### StdioClient

- **stdin writes**: Protected by `sync.Mutex` (`stdinMu`), ensuring atomic writes
- **pending responses**: Protected by `sync.RWMutex` (`pendingMu`), mapping request
  IDs to response channels
- **request IDs**: Generated with `atomic.AddInt64` for lock-free ID generation
- **read loop**: Single goroutine reads stdout, dispatches to pending channels
- **close**: `sync.Once` ensures cleanup runs exactly once

### HTTPClient

- **pending responses**: Protected by `sync.RWMutex`, same pattern as StdioClient
- **SSE connection**: Managed with a cancellable context (`sseCancel`)
- **close**: `sync.Once` cancels SSE context and closes all pending channels

### HTTPServer

- **SSE clients**: Protected by `sync.RWMutex` (`clientsMu`), mapping client IDs
  to writer references
- **active connections**: Tracked with `atomic.Int64` for the health endpoint
- **broadcast**: Takes a read-lock snapshot of clients, then writes without holding
  the lock (tolerating individual write failures)

### Registry

- **adapter map**: Protected by `sync.RWMutex`
- **batch operations**: Copy the adapter list under a read lock, then iterate
  without holding the lock. This prevents deadlocks when adapter operations
  take a long time.

## JSON-RPC 2.0 Compliance

The protocol package implements the JSON-RPC 2.0 specification:

- **Requests**: `{"jsonrpc":"2.0","id":1,"method":"...","params":{...}}`
- **Notifications**: Same as requests but without `id` (no response expected)
- **Responses**: `{"jsonrpc":"2.0","id":1,"result":{...}}` or
  `{"jsonrpc":"2.0","id":1,"error":{"code":...,"message":"..."}}`
- **ID normalization**: JSON numbers are `float64` when unmarshaled; `NormalizeID()`
  converts whole-number floats to `int64` for consistent map lookups

## MCP Protocol Version

The module implements MCP protocol version `2024-11-05`. This version is exchanged
during the `initialize` handshake and defines the available methods:

| Method | Direction | Purpose |
|--------|-----------|---------|
| `initialize` | Client -> Server | Protocol handshake |
| `notifications/initialized` | Client -> Server | Post-handshake notification |
| `tools/list` | Client -> Server | List available tools |
| `tools/call` | Client -> Server | Invoke a tool |
| `resources/list` | Client -> Server | List available resources |
| `resources/read` | Client -> Server | Read a resource |
| `prompts/list` | Client -> Server | List available prompts |
| `prompts/get` | Client -> Server | Get a prompt with arguments |

## Configuration Design

The `config` package separates two concerns:

1. **ServerConfig** -- describes how to reach or launch an MCP server (command,
   args, URL, transport type). Validated with `Validate()`.
2. **ContainerConfig** -- describes how to run an MCP server in a Docker container
   (image, ports, volumes, health check). Validated independently.

`FileConfig` combines both into a loadable configuration file (JSON format).
YAML support requires adding a YAML dependency; the current implementation accepts
JSON-formatted files with `.yaml`/`.yml` extensions.

## Error Handling Strategy

1. **Protocol errors**: Returned as `*RPCError` implementing the `error` interface,
   with standard error codes from JSON-RPC 2.0 and MCP extensions.
2. **Transport errors**: Wrapped with `fmt.Errorf("context: %w", err)` to preserve
   the error chain for `errors.Is()` and `errors.As()` callers.
3. **Handler errors**: Tool/resource/prompt handler errors are converted to
   `CodeInternalError` responses automatically by the server.
4. **Lifecycle errors**: `StopAll()` collects all errors rather than short-circuiting,
   ensuring best-effort cleanup.

## Testing Strategy

- **Protocol**: JSON marshaling round-trip tests for all types
- **Server**: Deterministic tests using `SetIO()` with `bytes.Buffer` for stdio;
  `httptest.NewRecorder()` for HTTP handlers
- **Client**: Integration-style tests with mock servers
- **Registry**: Unit tests for CRUD and lifecycle operations
- **Config**: Validation tests for all configuration variants
- **Adapter**: Lifecycle state transition tests
- **Interface compliance**: Compile-time checks with `var _ Interface = (*Struct)(nil)`
