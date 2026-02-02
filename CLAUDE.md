# CLAUDE.md - MCP Module

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
