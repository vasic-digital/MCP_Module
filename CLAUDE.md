# CLAUDE.md - MCP Module


## Definition of Done

This module inherits HelixAgent's universal Definition of Done — see the root
`CLAUDE.md` and `docs/development/definition-of-done.md`. In one line: **no
task is done without pasted output from a real run of the real system in the
same session as the change.** Coverage and green suites are not evidence.

### Acceptance demo for this module

```bash
# StdioServer registers a Tool and answers a real MCP tools/call over stdio
cd MCP_Module && GOMAXPROCS=2 nice -n 19 go test -count=1 -race -v ./tests/integration/...
```
Expect: PASS; exercises `server.NewStdioServer`, `RegisterTool`, `protocol.ToolResult` per `MCP_Module/README.md`. HTTPServer backend exercised with the `-tags=http` build tag.


## Overview

`digital.vasic.mcp` is a generic, reusable Go module implementing the Model Context Protocol (MCP) for AI tool integration. It provides protocol types, client/server implementations, an adapter registry, and configuration management.

**Module**: `digital.vasic.mcp` (Go 1.24+)

## Build & Test

```bash
go build ./...
go test ./... -count=1 -race
go test ./... -short              # Unit tests only
go test -bench=. ./...            # Benchmarks
```

## Code Style

- Standard Go conventions, `gofmt` formatting
- Imports grouped: stdlib, third-party, internal (blank line separated)
- Line length <= 100 chars
- Naming: `camelCase` private, `PascalCase` exported, acronyms all-caps
- Errors: always check, wrap with `fmt.Errorf("...: %w", err)`
- Tests: table-driven, `testify`, naming `Test<Struct>_<Method>_<Scenario>`

## Package Structure

| Package | Purpose |
|---------|---------|
| `pkg/protocol` | MCP protocol types, JSON-RPC 2.0 marshaling |
| `pkg/client` | MCP client (StdioClient, HTTPClient) |
| `pkg/server` | MCP server (StdioServer, HTTPServer) |
| `pkg/registry` | Thread-safe adapter registry with lifecycle |
| `pkg/config` | Server and container configuration, file loading |
| `pkg/adapter` | Base adapter types (Stdio, Docker, HTTP) |

## Key Interfaces

- `client.Client` -- MCP client operations (Initialize, ListTools, CallTool, etc.)
- `server.Server` -- MCP server with tool/resource/prompt registration
- `server.ToolHandler` -- Function handling tool calls
- `registry.Adapter` -- Adapter lifecycle (Name, Start, Stop, HealthCheck)

## Design Patterns

- **Strategy**: Transport types (stdio/HTTP) for both client and server
- **Registry**: Thread-safe adapter management with lifecycle
- **Factory**: `NewStdioClient()`, `NewHTTPServer()`, etc.
- **Interface Segregation**: Small focused interfaces (Client, Server, Adapter)

## Commit Style

Conventional Commits: `feat(protocol): add streaming support`

## Integration Seams

| Direction | Sibling modules |
|-----------|-----------------|
| Upstream (this module imports) | none |
| Downstream (these import this module) | HelixLLM |

*Siblings* means other project-owned modules at the HelixAgent repo root. The root HelixAgent app and external systems are not listed here — the list above is intentionally scoped to module-to-module seams, because drift *between* sibling modules is where the "tests pass, product broken" class of bug most often lives. See root `CLAUDE.md` for the rules that keep these seams contract-tested.
