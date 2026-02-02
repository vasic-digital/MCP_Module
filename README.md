# MCP Module

Generic, reusable Go module implementing the Model Context Protocol (MCP) for AI tool integration.

## Module

```
digital.vasic.mcp
```

## Packages

- **pkg/protocol** -- MCP protocol types and JSON-RPC 2.0 marshaling
- **pkg/client** -- MCP client implementations (stdio, HTTP/SSE)
- **pkg/server** -- MCP server implementations (stdio, HTTP/SSE)
- **pkg/registry** -- Thread-safe adapter registry with lifecycle management
- **pkg/config** -- Configuration types and file loading (JSON/YAML)
- **pkg/adapter** -- Base adapter types (stdio, Docker, HTTP)

## Usage

```go
import (
    "digital.vasic.mcp/pkg/protocol"
    "digital.vasic.mcp/pkg/server"
)

// Create an MCP server
s := server.NewStdioServer("my-server", "1.0.0")

// Register a tool
s.RegisterTool(
    protocol.Tool{
        Name:        "hello",
        Description: "Say hello",
    },
    func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error) {
        return &protocol.ToolResult{
            Content: []protocol.ContentBlock{
                protocol.NewTextContent("Hello, world!"),
            },
        }, nil
    },
)

// Serve over stdio
s.Serve(context.Background())
```

## Testing

```bash
go test ./... -count=1 -race
```

## License

See LICENSE file.
