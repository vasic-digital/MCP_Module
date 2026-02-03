# Changelog

All notable changes to the `digital.vasic.mcp` module are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2025-01-01

### Added

- **pkg/protocol**: MCP protocol types implementing JSON-RPC 2.0 specification.
  - `Request`, `Response`, `RPCError` types with full JSON marshaling support.
  - `Tool`, `ToolResult`, `ContentBlock` types for MCP tool definitions and results.
  - `Resource`, `ResourceContent` types for MCP resource access.
  - `Prompt`, `PromptArgument`, `PromptMessage` types for MCP prompt templates.
  - `ServerCapabilities`, `ServerInfo`, `ClientInfo` types for initialization handshake.
  - `InitializeParams`, `InitializeResult` types for the `initialize` method.
  - Factory functions: `NewRequest`, `NewNotification`, `NewResponse`, `NewErrorResponse`.
  - Content block helpers: `NewTextContent`, `NewBinaryContent`.
  - `NormalizeID` for consistent JSON-RPC ID handling across numeric types.
  - Standard JSON-RPC 2.0 error codes and MCP-specific error codes.
  - MCP protocol version constant: `2024-11-05`.

- **pkg/client**: MCP client implementations.
  - `Client` interface defining transport-agnostic MCP operations.
  - `StdioClient` for communicating with MCP servers via stdin/stdout.
  - `HTTPClient` for communicating with MCP servers via HTTP/SSE.
  - `Config` type with `DefaultConfig()` factory.
  - Full support for: `Initialize`, `ListTools`, `CallTool`, `ListResources`,
    `ReadResource`, `ListPrompts`, `GetPrompt`.
  - Thread-safe request/response correlation with pending response channels.
  - Atomic request ID generation.

- **pkg/server**: MCP server implementations.
  - `Server` interface for tool, resource, and prompt registration.
  - `ToolHandler`, `ResourceHandler`, `PromptHandler` function types.
  - `StdioServer` with `SetIO()` for testability.
  - `HTTPServer` with SSE support, configurable timeouts, and health endpoint.
  - `HTTPServerConfig` with `DefaultHTTPServerConfig()` factory.
  - `Handler()` method for embedding HTTPServer in existing HTTP servers.
  - Automatic capability detection based on registered handlers.
  - JSON-RPC 2.0 method routing: `initialize`, `tools/list`, `tools/call`,
    `resources/list`, `resources/read`, `prompts/list`, `prompts/get`,
    `notifications/initialized`.

- **pkg/registry**: Thread-safe adapter registry.
  - `Adapter` interface: `Name`, `Config`, `Start`, `Stop`, `HealthCheck`.
  - `Registry` with `Register`, `Unregister`, `Get`, `List`, `Count`.
  - Batch lifecycle operations: `StartAll`, `StopAll`, `HealthCheckAll`.
  - Duplicate name detection and nil adapter validation.

- **pkg/config**: Configuration management.
  - `ServerConfig` for stdio and HTTP server definitions with validation.
  - `ContainerConfig` for Docker-based servers with port/volume/network support.
  - `FileConfig` for multi-server configuration files.
  - `LoadFromFile` supporting JSON file loading.
  - `ImageRef()` for Docker image reference generation.

- **pkg/adapter**: Base adapter types.
  - `BaseAdapter` with common state machine (idle/starting/running/stopping/stopped/error).
  - `StdioAdapter` for managing local MCP server processes.
  - `DockerAdapter` for managing Docker containers with `docker run`/`docker rm`.
  - `HTTPAdapter` for referencing external HTTP MCP servers.
  - HTTP-based health checks for Docker and HTTP adapters.
  - Process signal-based health checks for stdio adapters.

- **Testing**: Comprehensive test suite.
  - Table-driven tests with `testify` across all packages.
  - Compile-time interface compliance checks.
  - JSON marshaling round-trip tests for protocol types.
  - Deterministic stdio server tests using `SetIO()`.
  - HTTP handler tests using `httptest.NewRecorder()`.

- **Documentation**: Initial documentation set.
  - `README.md` with quick start example.
  - `CLAUDE.md` with development guidance for AI agents.
  - `AGENTS.md` with multi-agent coordination guide.
  - `docs/USER_GUIDE.md` with code examples for all packages.
  - `docs/ARCHITECTURE.md` with design patterns and decisions.
  - `docs/API_REFERENCE.md` with complete exported API documentation.
  - `docs/CONTRIBUTING.md` with contribution guidelines.
  - Mermaid diagrams: architecture, sequence, and class diagrams.
