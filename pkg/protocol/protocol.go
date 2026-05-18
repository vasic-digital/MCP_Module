// Package protocol provides MCP (Model Context Protocol) types and
// JSON-RPC 2.0 message marshaling/unmarshaling.
package protocol

import (
	"context"
	"encoding/json"
	"fmt"

	"digital.vasic.mcp/pkg/i18n"
)

// translator is the package-level message-resolution seam (CONST-046,
// round-122). Defaults to NoopTranslator so production behaviour is
// unchanged for any caller that has not wired a project-side
// translator; consumers swap it via SetTranslator at startup.
//
// Per CONST-051(B) the seam is project-not-aware: this package never
// imports a HelixCode-specific catalogue; the consuming project is
// responsible for providing the real Translator implementation.
var translator i18n.Translator = i18n.NoopTranslator{}

// SetTranslator wires a project-side Translator. Callers MUST invoke it
// at startup before issuing user-facing error returns; calling it
// concurrently with request handling is not supported.
func SetTranslator(t i18n.Translator) {
	if t == nil {
		t = i18n.NoopTranslator{}
	}
	translator = t
}

// JSONRPCVersion is the JSON-RPC protocol version used by MCP.
const JSONRPCVersion = "2.0"

// MCPProtocolVersion is the current MCP protocol version.
const MCPProtocolVersion = "2024-11-05"

// Request represents a JSON-RPC 2.0 request used by MCP.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// NewRequest creates a new JSON-RPC 2.0 request.
func NewRequest(id interface{}, method string, params interface{}) (*Request, error) {
	var rawParams json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
		rawParams = data
	}
	return &Request{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  method,
		Params:  rawParams,
	}, nil
}

// NewNotification creates a JSON-RPC 2.0 notification (request without ID).
func NewNotification(method string, params interface{}) (*Request, error) {
	return NewRequest(nil, method, params)
}

// IsNotification returns true if the request has no ID (is a notification).
func (r *Request) IsNotification() bool {
	return r.ID == nil
}

// Response represents a JSON-RPC 2.0 response used by MCP.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// NewResponse creates a successful JSON-RPC 2.0 response.
func NewResponse(id interface{}, result interface{}) (*Response, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  data,
	}, nil
}

// NewErrorResponse creates an error JSON-RPC 2.0 response.
func NewErrorResponse(id interface{}, code int, message string, data interface{}) *Response {
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

// IsError returns true if the response contains an error.
func (r *Response) IsError() bool {
	return r.Error != nil
}

// RPCError represents a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error implements the error interface.
//
// The rendered wire string is resolved through the package-level
// Translator seam (CONST-046, round-122). The NoopTranslator default
// returns the msgID verbatim — operators see exactly which key failed
// to resolve, satisfying CONST-035 positive-evidence semantics. A
// consuming project may wire a richer Translator via SetTranslator
// without modifying this package (CONST-051(B) decoupling).
func (e *RPCError) Error() string {
	ctx := context.Background()
	if e.Data != nil {
		return fmt.Sprintf("%s", translator.T(ctx, "mcp_module_rpc_error_with_data", map[string]any{
			"code":    e.Code,
			"message": e.Message,
			"data":    fmt.Sprintf("%v", e.Data),
		}))
	}
	return fmt.Sprintf("%s", translator.T(ctx, "mcp_module_rpc_error_no_data", map[string]any{
		"code":    e.Code,
		"message": e.Message,
	}))
}

// Standard JSON-RPC 2.0 error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// MCP-specific error codes.
const (
	CodeServerError    = -32000
	CodeNotReady       = -32001
	CodeProcessClosed  = -32002
	CodeTimeout        = -32003
	CodeShutdown       = -32004
	CodeRequestTooLarge = -32005
)

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}

// ToolResult represents the result of calling an MCP tool.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in a tool result.
type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
}

// NewTextContent creates a text content block.
func NewTextContent(text string) ContentBlock {
	return ContentBlock{
		Type: "text",
		Text: text,
	}
}

// NewBinaryContent creates a binary content block.
func NewBinaryContent(mimeType, data string) ContentBlock {
	return ContentBlock{
		Type:     "blob",
		MimeType: mimeType,
		Data:     data,
	}
}

// Resource represents an MCP resource.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceContent holds the content of a resource.
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

// Prompt represents an MCP prompt template.
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument represents an argument for a prompt.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptMessage represents a message in a prompt response.
type PromptMessage struct {
	Role    string         `json:"role"`
	Content ContentBlock   `json:"content"`
}

// ServerCapabilities describes the capabilities of an MCP server.
type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Logging   *LoggingCapability   `json:"logging,omitempty"`
}

// ToolsCapability indicates tool support.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability indicates resource support.
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability indicates prompt support.
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// LoggingCapability indicates logging support.
type LoggingCapability struct{}

// ServerInfo describes the MCP server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientInfo describes the MCP client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeParams are the parameters for the initialize request.
type InitializeParams struct {
	ProtocolVersion string      `json:"protocolVersion"`
	Capabilities    interface{} `json:"capabilities"`
	ClientInfo      ClientInfo  `json:"clientInfo"`
}

// InitializeResult is the result of the initialize request.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// NormalizeID normalizes a JSON-RPC ID to a consistent type for map lookups.
// JSON numbers become float64 when unmarshaled, so we handle the conversion.
func NormalizeID(id interface{}) interface{} {
	switch v := id.(type) {
	case float64:
		if v == float64(int64(v)) {
			return int64(v)
		}
		return v
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		return v
	default:
		return id
	}
}
