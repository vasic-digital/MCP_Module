# User Guide

This guide covers how to use the `digital.vasic.mcp` Go module to build MCP
(Model Context Protocol) clients and servers. It includes code examples for every
major feature across all six packages.

## Installation

```bash
go get digital.vasic.mcp
```

Requires Go 1.24 or later.

## Quick Start: Building an MCP Server

The fastest way to get started is to create a stdio-based MCP server that exposes
a tool:

```go
package main

import (
    "context"
    "fmt"
    "os"

    "digital.vasic.mcp/pkg/protocol"
    "digital.vasic.mcp/pkg/server"
)

func main() {
    s := server.NewStdioServer("my-server", "1.0.0")

    s.RegisterTool(
        protocol.Tool{
            Name:        "greet",
            Description: "Greet a person by name",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "name": map[string]interface{}{
                        "type":        "string",
                        "description": "The name to greet",
                    },
                },
                "required": []string{"name"},
            },
        },
        func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error) {
            name, _ := args["name"].(string)
            return &protocol.ToolResult{
                Content: []protocol.ContentBlock{
                    protocol.NewTextContent(fmt.Sprintf("Hello, %s!", name)),
                },
            }, nil
        },
    )

    if err := s.Serve(context.Background()); err != nil {
        fmt.Fprintf(os.Stderr, "server error: %v\n", err)
        os.Exit(1)
    }
}
```

## Protocol Types

The `protocol` package defines all MCP and JSON-RPC 2.0 types used across the
module.

### Creating Requests

```go
import "digital.vasic.mcp/pkg/protocol"

// Create a JSON-RPC 2.0 request
req, err := protocol.NewRequest(1, "tools/list", nil)

// Create a notification (no ID, no response expected)
notif, err := protocol.NewNotification("notifications/initialized", map[string]interface{}{})

// Check if a request is a notification
if req.IsNotification() {
    // No response will be sent
}
```

### Creating Responses

```go
// Successful response
resp, err := protocol.NewResponse(1, map[string]interface{}{
    "tools": []protocol.Tool{},
})

// Error response
errResp := protocol.NewErrorResponse(
    1,
    protocol.CodeMethodNotFound,
    "method not found: foo/bar",
    nil,
)

// Check for error
if resp.IsError() {
    fmt.Println(resp.Error.Error())
}
```

### Content Blocks

```go
// Text content
text := protocol.NewTextContent("Hello, world!")

// Binary content (base64-encoded)
binary := protocol.NewBinaryContent("image/png", "iVBORw0KGgo...")
```

### Error Codes

Standard JSON-RPC 2.0 error codes:

| Constant | Code | Description |
|----------|------|-------------|
| `CodeParseError` | -32700 | Invalid JSON |
| `CodeInvalidRequest` | -32600 | Invalid JSON-RPC request |
| `CodeMethodNotFound` | -32601 | Method not found |
| `CodeInvalidParams` | -32602 | Invalid method parameters |
| `CodeInternalError` | -32603 | Internal server error |

MCP-specific error codes:

| Constant | Code | Description |
|----------|------|-------------|
| `CodeServerError` | -32000 | General server error |
| `CodeNotReady` | -32001 | Server not ready |
| `CodeProcessClosed` | -32002 | Server process terminated |
| `CodeTimeout` | -32003 | Request timed out |
| `CodeShutdown` | -32004 | Server is shutting down |
| `CodeRequestTooLarge` | -32005 | Request body too large |

## Server Package

### Stdio Server

Communicates over stdin/stdout using newline-delimited JSON:

```go
s := server.NewStdioServer("my-server", "1.0.0")

// Register tools, resources, and prompts
s.RegisterTool(tool, handler)
s.RegisterResource(resource, handler)
s.RegisterPrompt(prompt, handler)

// Start serving (blocks until context is cancelled or EOF)
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
err := s.Serve(ctx)
```

For testing, override stdin/stdout:

```go
var input bytes.Buffer
var output bytes.Buffer
s.SetIO(&input, &output)
```

### HTTP Server

Communicates over HTTP with Server-Sent Events (SSE):

```go
cfg := server.DefaultHTTPServerConfig()
cfg.Address = ":9090"
cfg.HeartbeatInterval = 15 * time.Second

s := server.NewHTTPServer("my-server", "1.0.0", cfg)
s.RegisterTool(tool, handler)

// Start serving (blocks until context is cancelled)
err := s.Serve(ctx)
```

The HTTP server exposes three endpoints:

- `GET /sse` -- SSE connection for receiving events
- `POST /message` -- JSON-RPC message submission
- `GET /health` -- Health check endpoint

Embed in an existing HTTP server:

```go
s := server.NewHTTPServer("my-server", "1.0.0", cfg)
s.RegisterTool(tool, handler)

// Use Handler() to embed in another mux
mainMux := http.NewServeMux()
mainMux.Handle("/mcp/", http.StripPrefix("/mcp", s.Handler()))
```

### Registering Tools

```go
s.RegisterTool(
    protocol.Tool{
        Name:        "read_file",
        Description: "Read a file from disk",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "path": map[string]interface{}{
                    "type":        "string",
                    "description": "Absolute file path",
                },
            },
            "required": []string{"path"},
        },
    },
    func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error) {
        path, _ := args["path"].(string)
        data, err := os.ReadFile(path)
        if err != nil {
            return &protocol.ToolResult{
                Content: []protocol.ContentBlock{
                    protocol.NewTextContent(fmt.Sprintf("Error: %v", err)),
                },
                IsError: true,
            }, nil
        }
        return &protocol.ToolResult{
            Content: []protocol.ContentBlock{
                protocol.NewTextContent(string(data)),
            },
        }, nil
    },
)
```

### Registering Resources

```go
s.RegisterResource(
    protocol.Resource{
        URI:         "config://app/settings",
        Name:        "Application Settings",
        Description: "Current application configuration",
        MimeType:    "application/json",
    },
    func(ctx context.Context, uri string) (*protocol.ResourceContent, error) {
        settings := loadSettings()
        data, _ := json.Marshal(settings)
        return &protocol.ResourceContent{
            URI:      uri,
            MimeType: "application/json",
            Text:     string(data),
        }, nil
    },
)
```

### Registering Prompts

```go
s.RegisterPrompt(
    protocol.Prompt{
        Name:        "code_review",
        Description: "Generate a code review prompt",
        Arguments: []protocol.PromptArgument{
            {Name: "code", Description: "The code to review", Required: true},
            {Name: "language", Description: "Programming language"},
        },
    },
    func(ctx context.Context, args map[string]string) ([]protocol.PromptMessage, error) {
        code := args["code"]
        lang := args["language"]
        if lang == "" {
            lang = "unknown"
        }
        return []protocol.PromptMessage{
            {
                Role: "user",
                Content: protocol.NewTextContent(
                    fmt.Sprintf("Review this %s code:\n\n```%s\n%s\n```",
                        lang, lang, code),
                ),
            },
        }, nil
    },
)
```

## Client Package

### Stdio Client

Communicates with a server process via stdin/stdout:

```go
cfg := client.Config{
    Transport:     client.TransportStdio,
    ServerCommand: "/usr/local/bin/my-mcp-server",
    ServerArgs:    []string{"--verbose"},
    ServerEnv:     map[string]string{"LOG_LEVEL": "debug"},
    Timeout:       30 * time.Second,
    ClientName:    "my-app",
    ClientVersion: "1.0.0",
}

c, err := client.NewStdioClient(cfg)
if err != nil {
    log.Fatal(err)
}
defer c.Close()

// Start the server process
if err := c.Start(); err != nil {
    log.Fatal(err)
}

// Perform MCP initialization handshake
result, err := c.Initialize(ctx)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Connected to %s v%s\n",
    result.ServerInfo.Name, result.ServerInfo.Version)

// List available tools
tools, err := c.ListTools(ctx)
for _, t := range tools {
    fmt.Printf("Tool: %s - %s\n", t.Name, t.Description)
}

// Call a tool
toolResult, err := c.CallTool(ctx, "greet", map[string]interface{}{
    "name": "Alice",
})
for _, content := range toolResult.Content {
    fmt.Println(content.Text)
}
```

### HTTP Client

Communicates with an HTTP/SSE MCP server:

```go
cfg := client.Config{
    Transport:     client.TransportHTTP,
    ServerURL:     "http://localhost:9090",
    Timeout:       30 * time.Second,
    ClientName:    "my-app",
    ClientVersion: "1.0.0",
}

c, err := client.NewHTTPClient(cfg)
if err != nil {
    log.Fatal(err)
}
defer c.Close()

// Establish SSE connection
if err := c.Connect(ctx); err != nil {
    log.Fatal(err)
}

// Initialize (same API as StdioClient)
result, err := c.Initialize(ctx)

// Use the same Client interface methods
tools, _ := c.ListTools(ctx)
resources, _ := c.ListResources(ctx)
prompts, _ := c.ListPrompts(ctx)
```

### Using the Client Interface

Both `StdioClient` and `HTTPClient` implement the `client.Client` interface,
enabling transport-agnostic code:

```go
func discoverTools(ctx context.Context, c client.Client) error {
    result, err := c.Initialize(ctx)
    if err != nil {
        return fmt.Errorf("initialization failed: %w", err)
    }

    tools, err := c.ListTools(ctx)
    if err != nil {
        return fmt.Errorf("listing tools failed: %w", err)
    }

    for _, tool := range tools {
        fmt.Printf("[%s] %s: %s\n",
            result.ServerInfo.Name, tool.Name, tool.Description)
    }
    return nil
}
```

## Config Package

### Server Configuration

```go
import "digital.vasic.mcp/pkg/config"

// Stdio server config
stdioCfg := config.ServerConfig{
    Name:        "filesystem-server",
    Command:     "/usr/local/bin/mcp-filesystem",
    Args:        []string{"--root", "/workspace"},
    Env:         map[string]string{"LOG_LEVEL": "info"},
    Transport:   config.TransportStdio,
    WorkingDir:  "/workspace",
    Enabled:     true,
    Description: "Filesystem access MCP server",
    Version:     "1.0.0",
}

// Validate the configuration
if err := stdioCfg.Validate(); err != nil {
    log.Fatal(err)
}

// HTTP server config
httpCfg := config.ServerConfig{
    Name:      "api-server",
    Transport: config.TransportHTTP,
    URL:       "http://localhost:9090",
    Enabled:   true,
}
```

### Container Configuration

```go
containerCfg := config.ContainerConfig{
    Image:         "ghcr.io/my-org/mcp-server",
    Tag:           "latest",
    Port:          8080,
    HostPort:      9101,
    Env:           map[string]string{"API_KEY": "${API_KEY}"},
    Volumes:       []string{"/data:/app/data:ro"},
    Network:       "mcp-network",
    HealthCheck:   "/health",
    RestartPolicy: "unless-stopped",
}

// Get the full image reference
ref := containerCfg.ImageRef() // "ghcr.io/my-org/mcp-server:latest"

// Validate
if err := containerCfg.Validate(); err != nil {
    log.Fatal(err)
}
```

### Loading Configuration from Files

```go
// Load from JSON file
cfg, err := config.LoadFromFile("mcp-servers.json")
if err != nil {
    log.Fatal(err)
}

// Validate all entries
if err := cfg.Validate(); err != nil {
    log.Fatal(err)
}

// Iterate servers
for _, srv := range cfg.Servers {
    fmt.Printf("Server: %s (%s)\n", srv.Name, srv.Transport)
}
```

Example JSON configuration file:

```json
{
    "servers": [
        {
            "name": "filesystem",
            "command": "/usr/local/bin/mcp-fs",
            "args": ["--root", "/workspace"],
            "transport": "stdio",
            "enabled": true
        },
        {
            "name": "api-tools",
            "transport": "http",
            "url": "http://localhost:9200",
            "enabled": true
        }
    ],
    "containers": [
        {
            "image": "ghcr.io/my-org/mcp-postgres",
            "tag": "1.0",
            "port": 8080,
            "host_port": 9102,
            "health_check": "/health"
        }
    ]
}
```

## Registry Package

The registry provides thread-safe lifecycle management for MCP adapters:

```go
import (
    "digital.vasic.mcp/pkg/adapter"
    "digital.vasic.mcp/pkg/config"
    "digital.vasic.mcp/pkg/registry"
)

// Create a registry
reg := registry.New()

// Create and register adapters
stdioCfg := config.ServerConfig{
    Name:    "fs-server",
    Command: "/usr/local/bin/mcp-fs",
    Enabled: true,
}
stdioAdapter := adapter.NewStdioAdapter("fs-server", stdioCfg)
if err := reg.Register(stdioAdapter); err != nil {
    log.Fatal(err)
}

httpCfg := config.ServerConfig{
    Name:    "api-server",
    URL:     "http://localhost:9200",
    Enabled: true,
}
httpAdapter := adapter.NewHTTPAdapter("api-server", httpCfg)
reg.Register(httpAdapter)

// List all registered adapters
names := reg.List()
fmt.Printf("Registered: %v (count: %d)\n", names, reg.Count())

// Start all adapters
if err := reg.StartAll(ctx); err != nil {
    log.Fatalf("Failed to start adapters: %v", err)
}

// Health check all adapters
results := reg.HealthCheckAll(ctx)
for name, err := range results {
    if err != nil {
        fmt.Printf("  %s: UNHEALTHY - %v\n", name, err)
    } else {
        fmt.Printf("  %s: HEALTHY\n", name)
    }
}

// Stop all on shutdown
if err := reg.StopAll(ctx); err != nil {
    log.Printf("Errors during shutdown: %v", err)
}
```

## Adapter Package

### Stdio Adapter

Manages a stdio-based MCP server process:

```go
cfg := config.ServerConfig{
    Name:       "my-server",
    Command:    "/usr/local/bin/mcp-server",
    Args:       []string{"--config", "/etc/mcp.json"},
    Env:        map[string]string{"DEBUG": "true"},
    WorkingDir: "/opt/mcp",
}

a := adapter.NewStdioAdapter("my-server", cfg)

// Lifecycle
err := a.Start(ctx)       // Starts the process
state := a.State()         // "running"
err = a.HealthCheck(ctx)   // Checks process is alive
err = a.Stop(ctx)          // Kills the process
```

### Docker Adapter

Manages a Docker container running an MCP server:

```go
serverCfg := config.ServerConfig{
    Name: "postgres-mcp",
}
containerCfg := config.ContainerConfig{
    Image:         "ghcr.io/my-org/mcp-postgres",
    Tag:           "latest",
    Port:          8080,
    HostPort:      9102,
    Env:           map[string]string{"DB_URL": "postgres://localhost/mydb"},
    Network:       "mcp-net",
    HealthCheck:   "/health",
    RestartPolicy: "unless-stopped",
}

a := adapter.NewDockerAdapter("postgres-mcp", serverCfg, containerCfg)

err := a.Start(ctx)       // Runs: docker run -d --name postgres-mcp ...
err = a.HealthCheck(ctx)   // HTTP GET to health check endpoint
err = a.Stop(ctx)          // Runs: docker rm -f postgres-mcp
```

### HTTP Adapter

Manages a reference to an external HTTP-based MCP server:

```go
cfg := config.ServerConfig{
    Name: "remote-server",
    URL:  "http://mcp-server.internal:8080",
}

a := adapter.NewHTTPAdapter("remote-server", cfg)

err := a.Start(ctx)       // Marks as running (server is external)
err = a.HealthCheck(ctx)   // HTTP GET to /health endpoint
err = a.Stop(ctx)          // Marks as stopped
```

### Adapter State Machine

All adapters follow this lifecycle:

```
idle -> starting -> running -> stopping -> stopped
                       |                      ^
                       +----> error -----------+
```

Access the current state with `adapter.State()`, which returns one of:

- `adapter.StateIdle`
- `adapter.StateStarting`
- `adapter.StateRunning`
- `adapter.StateStopping`
- `adapter.StateStopped`
- `adapter.StateError`

## Complete Example: Multi-Tool MCP Server

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
    "strings"

    "digital.vasic.mcp/pkg/protocol"
    "digital.vasic.mcp/pkg/server"
)

func main() {
    s := server.NewStdioServer("dev-tools", "1.0.0")

    // Tool: execute shell command
    s.RegisterTool(
        protocol.Tool{
            Name:        "run_command",
            Description: "Execute a shell command",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "command": map[string]interface{}{
                        "type": "string",
                    },
                },
                "required": []string{"command"},
            },
        },
        func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error) {
            command, _ := args["command"].(string)
            out, err := exec.CommandContext(ctx, "sh", "-c", command).CombinedOutput()
            text := string(out)
            if err != nil {
                text += "\nError: " + err.Error()
            }
            return &protocol.ToolResult{
                Content: []protocol.ContentBlock{
                    protocol.NewTextContent(text),
                },
                IsError: err != nil,
            }, nil
        },
    )

    // Resource: environment variables
    s.RegisterResource(
        protocol.Resource{
            URI:         "env://variables",
            Name:        "Environment Variables",
            Description: "Current process environment",
            MimeType:    "application/json",
        },
        func(ctx context.Context, uri string) (*protocol.ResourceContent, error) {
            env := make(map[string]string)
            for _, e := range os.Environ() {
                parts := strings.SplitN(e, "=", 2)
                if len(parts) == 2 {
                    env[parts[0]] = parts[1]
                }
            }
            data, _ := json.MarshalIndent(env, "", "  ")
            return &protocol.ResourceContent{
                URI:      uri,
                MimeType: "application/json",
                Text:     string(data),
            }, nil
        },
    )

    // Prompt: explain code
    s.RegisterPrompt(
        protocol.Prompt{
            Name:        "explain_code",
            Description: "Generate a prompt to explain code",
            Arguments: []protocol.PromptArgument{
                {Name: "code", Required: true},
                {Name: "language"},
            },
        },
        func(ctx context.Context, args map[string]string) ([]protocol.PromptMessage, error) {
            return []protocol.PromptMessage{
                {
                    Role: "user",
                    Content: protocol.NewTextContent(
                        fmt.Sprintf("Explain this %s code:\n\n```\n%s\n```",
                            args["language"], args["code"]),
                    ),
                },
            }, nil
        },
    )

    if err := s.Serve(context.Background()); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}
```
