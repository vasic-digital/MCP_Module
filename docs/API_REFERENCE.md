# API Reference

Complete reference for all exported types, functions, methods, and constants in the
`digital.vasic.mcp` module.

---

## Package `protocol`

**Import**: `digital.vasic.mcp/pkg/protocol`

MCP protocol types and JSON-RPC 2.0 message marshaling/unmarshaling.

### Constants

```go
const JSONRPCVersion = "2.0"
```
The JSON-RPC protocol version used by MCP.

```go
const MCPProtocolVersion = "2024-11-05"
```
The current MCP protocol version.

#### Standard JSON-RPC 2.0 Error Codes

```go
const (
    CodeParseError     = -32700
    CodeInvalidRequest = -32600
    CodeMethodNotFound = -32601
    CodeInvalidParams  = -32602
    CodeInternalError  = -32603
)
```

#### MCP-Specific Error Codes

```go
const (
    CodeServerError     = -32000
    CodeNotReady        = -32001
    CodeProcessClosed   = -32002
    CodeTimeout         = -32003
    CodeShutdown        = -32004
    CodeRequestTooLarge = -32005
)
```

### Types

#### Request

```go
type Request struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      interface{}     `json:"id,omitempty"`
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}
```

A JSON-RPC 2.0 request used by MCP.

**Methods:**

- `IsNotification() bool` -- Returns true if the request has no ID (is a notification).

#### Response

```go
type Response struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      interface{}     `json:"id,omitempty"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *RPCError       `json:"error,omitempty"`
}
```

A JSON-RPC 2.0 response used by MCP.

**Methods:**

- `IsError() bool` -- Returns true if the response contains an error.

#### RPCError

```go
type RPCError struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}
```

A JSON-RPC 2.0 error object. Implements the `error` interface.

**Methods:**

- `Error() string` -- Returns a formatted error string including code, message, and optional data.

#### Tool

```go
type Tool struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description,omitempty"`
    InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}
```

An MCP tool definition.

#### ToolResult

```go
type ToolResult struct {
    Content []ContentBlock `json:"content"`
    IsError bool           `json:"isError,omitempty"`
}
```

The result of calling an MCP tool.

#### ContentBlock

```go
type ContentBlock struct {
    Type     string `json:"type"`
    Text     string `json:"text,omitempty"`
    MimeType string `json:"mimeType,omitempty"`
    Data     string `json:"data,omitempty"`
}
```

A content block in a tool result. Type is `"text"` for text content or `"blob"`
for binary content.

#### Resource

```go
type Resource struct {
    URI         string `json:"uri"`
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    MimeType    string `json:"mimeType,omitempty"`
}
```

An MCP resource.

#### ResourceContent

```go
type ResourceContent struct {
    URI      string `json:"uri"`
    MimeType string `json:"mimeType,omitempty"`
    Text     string `json:"text,omitempty"`
    Blob     string `json:"blob,omitempty"`
}
```

The content of a resource.

#### Prompt

```go
type Prompt struct {
    Name        string           `json:"name"`
    Description string           `json:"description,omitempty"`
    Arguments   []PromptArgument `json:"arguments,omitempty"`
}
```

An MCP prompt template.

#### PromptArgument

```go
type PromptArgument struct {
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    Required    bool   `json:"required,omitempty"`
}
```

An argument for a prompt.

#### PromptMessage

```go
type PromptMessage struct {
    Role    string       `json:"role"`
    Content ContentBlock `json:"content"`
}
```

A message in a prompt response.

#### ServerCapabilities

```go
type ServerCapabilities struct {
    Tools     *ToolsCapability     `json:"tools,omitempty"`
    Resources *ResourcesCapability `json:"resources,omitempty"`
    Prompts   *PromptsCapability   `json:"prompts,omitempty"`
    Logging   *LoggingCapability   `json:"logging,omitempty"`
}
```

Describes the capabilities of an MCP server.

#### ToolsCapability

```go
type ToolsCapability struct {
    ListChanged bool `json:"listChanged,omitempty"`
}
```

Indicates tool support.

#### ResourcesCapability

```go
type ResourcesCapability struct {
    Subscribe   bool `json:"subscribe,omitempty"`
    ListChanged bool `json:"listChanged,omitempty"`
}
```

Indicates resource support.

#### PromptsCapability

```go
type PromptsCapability struct {
    ListChanged bool `json:"listChanged,omitempty"`
}
```

Indicates prompt support.

#### LoggingCapability

```go
type LoggingCapability struct{}
```

Indicates logging support.

#### ServerInfo

```go
type ServerInfo struct {
    Name    string `json:"name"`
    Version string `json:"version"`
}
```

Describes the MCP server.

#### ClientInfo

```go
type ClientInfo struct {
    Name    string `json:"name"`
    Version string `json:"version"`
}
```

Describes the MCP client.

#### InitializeParams

```go
type InitializeParams struct {
    ProtocolVersion string      `json:"protocolVersion"`
    Capabilities    interface{} `json:"capabilities"`
    ClientInfo      ClientInfo  `json:"clientInfo"`
}
```

Parameters for the `initialize` request.

#### InitializeResult

```go
type InitializeResult struct {
    ProtocolVersion string             `json:"protocolVersion"`
    Capabilities    ServerCapabilities `json:"capabilities"`
    ServerInfo      ServerInfo         `json:"serverInfo"`
}
```

Result of the `initialize` request.

### Functions

#### NewRequest

```go
func NewRequest(id interface{}, method string, params interface{}) (*Request, error)
```

Creates a new JSON-RPC 2.0 request. Returns an error if `params` cannot be marshaled
to JSON.

#### NewNotification

```go
func NewNotification(method string, params interface{}) (*Request, error)
```

Creates a JSON-RPC 2.0 notification (request without ID). Equivalent to
`NewRequest(nil, method, params)`.

#### NewResponse

```go
func NewResponse(id interface{}, result interface{}) (*Response, error)
```

Creates a successful JSON-RPC 2.0 response. Returns an error if `result` cannot be
marshaled to JSON.

#### NewErrorResponse

```go
func NewErrorResponse(id interface{}, code int, message string, data interface{}) *Response
```

Creates an error JSON-RPC 2.0 response.

#### NewTextContent

```go
func NewTextContent(text string) ContentBlock
```

Creates a text content block with `Type: "text"`.

#### NewBinaryContent

```go
func NewBinaryContent(mimeType, data string) ContentBlock
```

Creates a binary content block with `Type: "blob"`.

#### NormalizeID

```go
func NormalizeID(id interface{}) interface{}
```

Normalizes a JSON-RPC ID to a consistent type for map lookups. Converts `float64`
whole numbers to `int64`, `int` to `int64`, and passes through `int64` and `string`
unchanged.

---

## Package `client`

**Import**: `digital.vasic.mcp/pkg/client`

MCP client implementations for communicating with MCP servers over stdio and
HTTP/SSE transports.

### Constants

```go
type TransportType string

const (
    TransportStdio TransportType = "stdio"
    TransportHTTP  TransportType = "http"
)
```

### Types

#### Config

```go
type Config struct {
    Transport     TransportType
    ServerCommand string
    ServerArgs    []string
    ServerEnv     map[string]string
    ServerURL     string
    Timeout       time.Duration
    ClientName    string
    ClientVersion string
}
```

Client configuration. `ServerCommand` is required for stdio transport.
`ServerURL` is required for HTTP transport.

#### Client (interface)

```go
type Client interface {
    Initialize(ctx context.Context) (*protocol.InitializeResult, error)
    ListTools(ctx context.Context) ([]protocol.Tool, error)
    CallTool(ctx context.Context, name string, args map[string]interface{}) (*protocol.ToolResult, error)
    ListResources(ctx context.Context) ([]protocol.Resource, error)
    ReadResource(ctx context.Context, uri string) (*protocol.ResourceContent, error)
    ListPrompts(ctx context.Context) ([]protocol.Prompt, error)
    GetPrompt(ctx context.Context, name string, args map[string]string) ([]protocol.PromptMessage, error)
    Close() error
}
```

Defines the interface for an MCP client. Implemented by `StdioClient` and
`HTTPClient`.

#### StdioClient

```go
type StdioClient struct { /* unexported fields */ }
```

Communicates with an MCP server via stdin/stdout.

**Methods:**

- `Start() error` -- Starts the MCP server process and begins reading responses.
- `Initialize(ctx context.Context) (*protocol.InitializeResult, error)` -- Performs the MCP initialization handshake. Sends `initialized` notification after success.
- `ListTools(ctx context.Context) ([]protocol.Tool, error)` -- Returns the tools available on the MCP server.
- `CallTool(ctx context.Context, name string, args map[string]interface{}) (*protocol.ToolResult, error)` -- Invokes a tool on the MCP server.
- `ListResources(ctx context.Context) ([]protocol.Resource, error)` -- Returns the resources available on the MCP server.
- `ReadResource(ctx context.Context, uri string) (*protocol.ResourceContent, error)` -- Reads a resource from the MCP server.
- `ListPrompts(ctx context.Context) ([]protocol.Prompt, error)` -- Returns the prompts available on the MCP server.
- `GetPrompt(ctx context.Context, name string, args map[string]string) ([]protocol.PromptMessage, error)` -- Retrieves a prompt with the given arguments.
- `Close() error` -- Shuts down the client, kills the server process, and cleans up resources.

#### HTTPClient

```go
type HTTPClient struct { /* unexported fields */ }
```

Communicates with an MCP server over HTTP/SSE.

**Methods:**

- `Connect(ctx context.Context) error` -- Establishes the SSE connection for receiving events.
- `Initialize(ctx context.Context) (*protocol.InitializeResult, error)` -- Performs the MCP initialization handshake.
- `ListTools(ctx context.Context) ([]protocol.Tool, error)` -- Returns the tools available on the MCP server.
- `CallTool(ctx context.Context, name string, args map[string]interface{}) (*protocol.ToolResult, error)` -- Invokes a tool on the MCP server.
- `ListResources(ctx context.Context) ([]protocol.Resource, error)` -- Returns the resources available on the MCP server.
- `ReadResource(ctx context.Context, uri string) (*protocol.ResourceContent, error)` -- Reads a resource from the MCP server.
- `ListPrompts(ctx context.Context) ([]protocol.Prompt, error)` -- Returns the prompts available on the MCP server.
- `GetPrompt(ctx context.Context, name string, args map[string]string) ([]protocol.PromptMessage, error)` -- Retrieves a prompt with the given arguments.
- `Close() error` -- Shuts down the HTTP client and cancels the SSE connection.

### Functions

#### DefaultConfig

```go
func DefaultConfig() Config
```

Returns a `Config` with sensible defaults: stdio transport, 30-second timeout,
client name `"mcp-client"`, version `"1.0.0"`.

#### NewStdioClient

```go
func NewStdioClient(config Config) (*StdioClient, error)
```

Creates a new stdio-based MCP client. Returns an error if `ServerCommand` is empty.

#### NewHTTPClient

```go
func NewHTTPClient(config Config) (*HTTPClient, error)
```

Creates a new HTTP/SSE-based MCP client. Returns an error if `ServerURL` is empty.

---

## Package `server`

**Import**: `digital.vasic.mcp/pkg/server`

MCP server implementations that handle JSON-RPC 2.0 requests over stdio and
HTTP/SSE transports.

### Types

#### ToolHandler

```go
type ToolHandler func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error)
```

A function that handles a tool call.

#### ResourceHandler

```go
type ResourceHandler func(ctx context.Context, uri string) (*protocol.ResourceContent, error)
```

A function that handles a resource read.

#### PromptHandler

```go
type PromptHandler func(ctx context.Context, args map[string]string) ([]protocol.PromptMessage, error)
```

A function that handles a prompt get request.

#### Server (interface)

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

Defines the interface for an MCP server. Implemented by `StdioServer` and
`HTTPServer`.

#### StdioServer

```go
type StdioServer struct { /* unexported fields */ }
```

An MCP server that communicates via stdin/stdout.

**Methods:**

- `SetIO(stdin io.Reader, stdout io.Writer)` -- Overrides the default stdin/stdout for testing.
- `RegisterTool(tool protocol.Tool, handler ToolHandler)` -- Registers a tool with a handler.
- `RegisterResource(resource protocol.Resource, handler ResourceHandler)` -- Registers a resource with a handler.
- `RegisterPrompt(prompt protocol.Prompt, handler PromptHandler)` -- Registers a prompt with a handler.
- `Serve(ctx context.Context) error` -- Starts reading from stdin and processing requests. Blocks until the context is cancelled or EOF is reached.
- `ServerInfo() protocol.ServerInfo` -- Returns the server identification.
- `Capabilities() protocol.ServerCapabilities` -- Returns the server capabilities based on registered handlers.

#### HTTPServerConfig

```go
type HTTPServerConfig struct {
    Address           string
    ReadTimeout       time.Duration
    WriteTimeout      time.Duration
    MaxRequestSize    int64
    HeartbeatInterval time.Duration
}
```

Configuration for the HTTP MCP server.

#### HTTPServer

```go
type HTTPServer struct { /* unexported fields */ }
```

An MCP server that communicates via HTTP with SSE.

**Methods:**

- `RegisterTool(tool protocol.Tool, handler ToolHandler)` -- Registers a tool with a handler.
- `RegisterResource(resource protocol.Resource, handler ResourceHandler)` -- Registers a resource with a handler.
- `RegisterPrompt(prompt protocol.Prompt, handler PromptHandler)` -- Registers a prompt with a handler.
- `Serve(ctx context.Context) error` -- Starts the HTTP server and blocks until the context is cancelled.
- `Handler() http.Handler` -- Returns the HTTP handler for embedding in existing servers.
- `ServerInfo() protocol.ServerInfo` -- Returns the server identification.
- `Capabilities() protocol.ServerCapabilities` -- Returns the server capabilities based on registered handlers.

**HTTP Endpoints:**

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/sse` | GET | SSE connection for receiving events |
| `/message` | POST | JSON-RPC message submission |
| `/health` | GET | Health check (returns JSON with status, server info, counts) |

### Functions

#### NewStdioServer

```go
func NewStdioServer(name, version string) *StdioServer
```

Creates a new stdio-based MCP server with the given name and version.

#### DefaultHTTPServerConfig

```go
func DefaultHTTPServerConfig() HTTPServerConfig
```

Returns sensible defaults: address `":8080"`, read timeout 30s, write timeout 60s,
max request size 10MB, heartbeat interval 30s.

#### NewHTTPServer

```go
func NewHTTPServer(name, version string, config HTTPServerConfig) *HTTPServer
```

Creates a new HTTP/SSE MCP server with the given name, version, and configuration.

---

## Package `registry`

**Import**: `digital.vasic.mcp/pkg/registry`

Thread-safe adapter registry for managing MCP server adapters with lifecycle
operations.

### Types

#### Adapter (interface)

```go
type Adapter interface {
    Name() string
    Config() map[string]interface{}
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    HealthCheck(ctx context.Context) error
}
```

Defines the interface for an MCP server adapter. Implemented by `StdioAdapter`,
`DockerAdapter`, and `HTTPAdapter` in the `adapter` package.

#### Registry

```go
type Registry struct { /* unexported fields */ }
```

Manages MCP server adapters in a thread-safe map.

**Methods:**

- `Register(adapter Adapter) error` -- Adds an adapter to the registry. Returns an error if the adapter is nil, has an empty name, or a duplicate name.
- `Unregister(name string) error` -- Removes an adapter from the registry. Returns an error if the adapter is not found.
- `Get(name string) (Adapter, bool)` -- Retrieves an adapter by name.
- `List() []string` -- Returns the names of all registered adapters.
- `Count() int` -- Returns the number of registered adapters.
- `StartAll(ctx context.Context) error` -- Starts all registered adapters. Returns on first error.
- `StopAll(ctx context.Context) error` -- Stops all registered adapters. Collects all errors.
- `HealthCheckAll(ctx context.Context) map[string]error` -- Runs health checks on all adapters. Returns a map of adapter name to error (nil if healthy).

### Functions

#### New

```go
func New() *Registry
```

Creates a new empty adapter registry.

---

## Package `config`

**Import**: `digital.vasic.mcp/pkg/config`

Configuration types and file loading for MCP server definitions.

### Constants

```go
type TransportType string

const (
    TransportStdio TransportType = "stdio"
    TransportHTTP  TransportType = "http"
)
```

### Types

#### ServerConfig

```go
type ServerConfig struct {
    Name        string            `json:"name" yaml:"name"`
    Command     string            `json:"command,omitempty" yaml:"command,omitempty"`
    Args        []string          `json:"args,omitempty" yaml:"args,omitempty"`
    Env         map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
    Transport   TransportType     `json:"transport" yaml:"transport"`
    URL         string            `json:"url,omitempty" yaml:"url,omitempty"`
    WorkingDir  string            `json:"working_dir,omitempty" yaml:"working_dir,omitempty"`
    Enabled     bool              `json:"enabled" yaml:"enabled"`
    Description string            `json:"description,omitempty" yaml:"description,omitempty"`
    Version     string            `json:"version,omitempty" yaml:"version,omitempty"`
}
```

Defines an MCP server configuration.

**Methods:**

- `Validate() error` -- Checks the configuration for errors. Verifies name is present, transport type is valid, and transport-specific fields are set (command for stdio, URL for HTTP).

#### ContainerConfig

```go
type ContainerConfig struct {
    Image         string            `json:"image" yaml:"image"`
    Tag           string            `json:"tag,omitempty" yaml:"tag,omitempty"`
    Port          int               `json:"port,omitempty" yaml:"port,omitempty"`
    HostPort      int               `json:"host_port,omitempty" yaml:"host_port,omitempty"`
    Env           map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
    Volumes       []string          `json:"volumes,omitempty" yaml:"volumes,omitempty"`
    Network       string            `json:"network,omitempty" yaml:"network,omitempty"`
    Command       []string          `json:"command,omitempty" yaml:"command,omitempty"`
    HealthCheck   string            `json:"health_check,omitempty" yaml:"health_check,omitempty"`
    RestartPolicy string            `json:"restart_policy,omitempty" yaml:"restart_policy,omitempty"`
}
```

Defines configuration for a Docker-based MCP server.

**Methods:**

- `Validate() error` -- Checks the configuration for errors. Verifies image is present and ports are in valid range (0-65535).
- `ImageRef() string` -- Returns the full image reference (`image:tag`). If tag is empty, returns just the image name.

#### FileConfig

```go
type FileConfig struct {
    Servers    []ServerConfig    `json:"servers" yaml:"servers"`
    Containers []ContainerConfig `json:"containers,omitempty" yaml:"containers,omitempty"`
}
```

A configuration file containing multiple servers and containers.

**Methods:**

- `Validate() error` -- Validates all server and container configurations.

### Functions

#### LoadFromFile

```go
func LoadFromFile(path string) (*FileConfig, error)
```

Loads configuration from a JSON file. Supports `.json`, `.yaml`, and `.yml`
extensions (YAML files must be in JSON-compatible format unless a YAML dependency
is added).

---

## Package `adapter`

**Import**: `digital.vasic.mcp/pkg/adapter`

Base adapter types for MCP server adapters, including stdio, Docker, and HTTP-based
adapters.

### Constants

```go
type State string

const (
    StateIdle     State = "idle"
    StateStarting State = "starting"
    StateRunning  State = "running"
    StateStopping State = "stopping"
    StateStopped  State = "stopped"
    StateError    State = "error"
)
```

### Types

#### BaseAdapter

```go
type BaseAdapter struct {
    AdapterName string
    ServerCfg   config.ServerConfig
    // unexported: state, mu
}
```

Provides common functionality for all adapters.

**Methods:**

- `Name() string` -- Returns the adapter name.
- `Config() map[string]interface{}` -- Returns the adapter configuration as a map.
- `State() State` -- Returns the current adapter state (thread-safe).
- `SetState(s State)` -- Sets the adapter state (thread-safe).

#### StdioAdapter

```go
type StdioAdapter struct {
    BaseAdapter
    // unexported: cmd, cmdMu
}
```

Manages a stdio-based MCP server process.

**Methods:**

- `Start(ctx context.Context) error` -- Starts the stdio server process.
- `Stop(ctx context.Context) error` -- Stops the stdio server process by killing it.
- `HealthCheck(ctx context.Context) error` -- Checks if the process is running.

#### DockerAdapter

```go
type DockerAdapter struct {
    BaseAdapter
    Container config.ContainerConfig
    // unexported: containerID
}
```

Manages a Docker-based MCP server container.

**Methods:**

- `Start(ctx context.Context) error` -- Starts the Docker container using `docker run`.
- `Stop(ctx context.Context) error` -- Stops and removes the container using `docker rm -f`.
- `HealthCheck(ctx context.Context) error` -- Checks container health via HTTP endpoint or `docker inspect`.

#### HTTPAdapter

```go
type HTTPAdapter struct {
    BaseAdapter
    // unexported: httpClient
}
```

Manages an HTTP-based MCP server (external).

**Methods:**

- `Start(ctx context.Context) error` -- Marks the adapter as running (server is external).
- `Stop(ctx context.Context) error` -- Marks the adapter as stopped.
- `HealthCheck(ctx context.Context) error` -- Performs an HTTP GET to the server's `/health` endpoint.

### Functions

#### NewStdioAdapter

```go
func NewStdioAdapter(name string, cfg config.ServerConfig) *StdioAdapter
```

Creates a new stdio adapter. Sets transport to `config.TransportStdio`.

#### NewDockerAdapter

```go
func NewDockerAdapter(name string, serverCfg config.ServerConfig, containerCfg config.ContainerConfig) *DockerAdapter
```

Creates a new Docker adapter.

#### NewHTTPAdapter

```go
func NewHTTPAdapter(name string, cfg config.ServerConfig) *HTTPAdapter
```

Creates a new HTTP adapter. Sets transport to `config.TransportHTTP`.
