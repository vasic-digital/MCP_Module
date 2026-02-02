// Package client provides MCP client implementations for communicating
// with MCP servers over stdio and HTTP/SSE transports.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"digital.vasic.mcp/pkg/protocol"
)

// TransportType defines the transport mechanism for MCP communication.
type TransportType string

const (
	// TransportStdio uses stdin/stdout for communication.
	TransportStdio TransportType = "stdio"
	// TransportHTTP uses HTTP with SSE for communication.
	TransportHTTP TransportType = "http"
)

// Config holds client configuration.
type Config struct {
	// Transport specifies the transport type (stdio or http).
	Transport TransportType

	// ServerCommand is the command to start the MCP server (for stdio).
	ServerCommand string

	// ServerArgs are arguments for the server command (for stdio).
	ServerArgs []string

	// ServerEnv are environment variables for the server process (for stdio).
	ServerEnv map[string]string

	// ServerURL is the base URL for the MCP server (for http).
	ServerURL string

	// Timeout is the request timeout duration.
	Timeout time.Duration

	// ClientName identifies this client during initialization.
	ClientName string

	// ClientVersion is the version of this client.
	ClientVersion string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Transport:     TransportStdio,
		Timeout:       30 * time.Second,
		ClientName:    "mcp-client",
		ClientVersion: "1.0.0",
	}
}

// Client defines the interface for an MCP client.
type Client interface {
	// Initialize performs the MCP initialization handshake.
	Initialize(ctx context.Context) (*protocol.InitializeResult, error)

	// ListTools returns the tools available on the MCP server.
	ListTools(ctx context.Context) ([]protocol.Tool, error)

	// CallTool invokes a tool on the MCP server.
	CallTool(
		ctx context.Context,
		name string,
		args map[string]interface{},
	) (*protocol.ToolResult, error)

	// ListResources returns the resources available on the MCP server.
	ListResources(ctx context.Context) ([]protocol.Resource, error)

	// ReadResource reads a resource from the MCP server.
	ReadResource(
		ctx context.Context,
		uri string,
	) (*protocol.ResourceContent, error)

	// ListPrompts returns the prompts available on the MCP server.
	ListPrompts(ctx context.Context) ([]protocol.Prompt, error)

	// GetPrompt retrieves a prompt with the given arguments.
	GetPrompt(
		ctx context.Context,
		name string,
		args map[string]string,
	) ([]protocol.PromptMessage, error)

	// Close shuts down the client and cleans up resources.
	Close() error
}

// listToolsResult is the response structure for tools/list.
type listToolsResult struct {
	Tools []protocol.Tool `json:"tools"`
}

// listResourcesResult is the response structure for resources/list.
type listResourcesResult struct {
	Resources []protocol.Resource `json:"resources"`
}

// readResourceResult is the response structure for resources/read.
type readResourceResult struct {
	Contents []protocol.ResourceContent `json:"contents"`
}

// listPromptsResult is the response structure for prompts/list.
type listPromptsResult struct {
	Prompts []protocol.Prompt `json:"prompts"`
}

// getPromptResult is the response structure for prompts/get.
type getPromptResult struct {
	Messages []protocol.PromptMessage `json:"messages"`
}

// callToolResult is the response structure for tools/call.
type callToolResult struct {
	Content []protocol.ContentBlock `json:"content"`
	IsError bool                    `json:"isError,omitempty"`
}

// parseResult is a helper to unmarshal a response result into the target.
func parseResult(resp *protocol.Response, target interface{}) error {
	if resp.IsError() {
		return resp.Error
	}
	if resp.Result == nil {
		return fmt.Errorf("response has no result")
	}
	return json.Unmarshal(resp.Result, target)
}
