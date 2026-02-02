// Package server provides MCP server implementations that handle
// JSON-RPC 2.0 requests over stdio and HTTP/SSE transports.
package server

import (
	"context"
	"encoding/json"
	"fmt"

	"digital.vasic.mcp/pkg/protocol"
)

// ToolHandler is a function that handles a tool call.
type ToolHandler func(
	ctx context.Context,
	args map[string]interface{},
) (*protocol.ToolResult, error)

// ResourceHandler is a function that handles a resource read.
type ResourceHandler func(
	ctx context.Context,
	uri string,
) (*protocol.ResourceContent, error)

// PromptHandler is a function that handles a prompt get request.
type PromptHandler func(
	ctx context.Context,
	args map[string]string,
) ([]protocol.PromptMessage, error)

// Server defines the interface for an MCP server.
type Server interface {
	// RegisterTool registers a tool with a handler.
	RegisterTool(tool protocol.Tool, handler ToolHandler)

	// RegisterResource registers a resource with a handler.
	RegisterResource(resource protocol.Resource, handler ResourceHandler)

	// RegisterPrompt registers a prompt with a handler.
	RegisterPrompt(prompt protocol.Prompt, handler PromptHandler)

	// Serve starts the server and blocks until the context is cancelled.
	Serve(ctx context.Context) error

	// ServerInfo returns server identification.
	ServerInfo() protocol.ServerInfo

	// Capabilities returns the server capabilities.
	Capabilities() protocol.ServerCapabilities
}

// toolEntry pairs a tool definition with its handler.
type toolEntry struct {
	tool    protocol.Tool
	handler ToolHandler
}

// resourceEntry pairs a resource definition with its handler.
type resourceEntry struct {
	resource protocol.Resource
	handler  ResourceHandler
}

// promptEntry pairs a prompt definition with its handler.
type promptEntry struct {
	prompt  protocol.Prompt
	handler PromptHandler
}

// handleRequest dispatches a JSON-RPC request to the appropriate handler.
func handleRequest(
	ctx context.Context,
	req *protocol.Request,
	tools map[string]*toolEntry,
	resources map[string]*resourceEntry,
	prompts map[string]*promptEntry,
	info protocol.ServerInfo,
) *protocol.Response {
	switch req.Method {
	case "initialize":
		return handleInitialize(req, tools, resources, prompts, info)
	case "tools/list":
		return handleListTools(req, tools)
	case "tools/call":
		return handleCallTool(ctx, req, tools)
	case "resources/list":
		return handleListResources(req, resources)
	case "resources/read":
		return handleReadResource(ctx, req, resources)
	case "prompts/list":
		return handleListPrompts(req, prompts)
	case "prompts/get":
		return handleGetPrompt(ctx, req, prompts)
	case "notifications/initialized":
		// Notification — no response needed
		return nil
	default:
		return protocol.NewErrorResponse(
			req.ID,
			protocol.CodeMethodNotFound,
			fmt.Sprintf("method not found: %s", req.Method),
			nil,
		)
	}
}

func handleInitialize(
	req *protocol.Request,
	tools map[string]*toolEntry,
	resources map[string]*resourceEntry,
	prompts map[string]*promptEntry,
	info protocol.ServerInfo,
) *protocol.Response {
	caps := protocol.ServerCapabilities{}
	if len(tools) > 0 {
		caps.Tools = &protocol.ToolsCapability{}
	}
	if len(resources) > 0 {
		caps.Resources = &protocol.ResourcesCapability{}
	}
	if len(prompts) > 0 {
		caps.Prompts = &protocol.PromptsCapability{}
	}

	result := protocol.InitializeResult{
		ProtocolVersion: protocol.MCPProtocolVersion,
		Capabilities:    caps,
		ServerInfo:      info,
	}

	resp, err := protocol.NewResponse(req.ID, result)
	if err != nil {
		return protocol.NewErrorResponse(
			req.ID, protocol.CodeInternalError, err.Error(), nil,
		)
	}
	return resp
}

func handleListTools(
	req *protocol.Request,
	tools map[string]*toolEntry,
) *protocol.Response {
	toolList := make([]protocol.Tool, 0, len(tools))
	for _, entry := range tools {
		toolList = append(toolList, entry.tool)
	}

	resp, err := protocol.NewResponse(req.ID, map[string]interface{}{
		"tools": toolList,
	})
	if err != nil {
		return protocol.NewErrorResponse(
			req.ID, protocol.CodeInternalError, err.Error(), nil,
		)
	}
	return resp
}

func handleCallTool(
	ctx context.Context,
	req *protocol.Request,
	tools map[string]*toolEntry,
) *protocol.Response {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return protocol.NewErrorResponse(
			req.ID, protocol.CodeInvalidParams,
			"invalid parameters", err.Error(),
		)
	}

	entry, ok := tools[params.Name]
	if !ok {
		return protocol.NewErrorResponse(
			req.ID, protocol.CodeInvalidParams,
			fmt.Sprintf("unknown tool: %s", params.Name), nil,
		)
	}

	result, err := entry.handler(ctx, params.Arguments)
	if err != nil {
		return protocol.NewErrorResponse(
			req.ID, protocol.CodeInternalError, err.Error(), nil,
		)
	}

	resp, marshalErr := protocol.NewResponse(req.ID, result)
	if marshalErr != nil {
		return protocol.NewErrorResponse(
			req.ID, protocol.CodeInternalError,
			marshalErr.Error(), nil,
		)
	}
	return resp
}

func handleListResources(
	req *protocol.Request,
	resources map[string]*resourceEntry,
) *protocol.Response {
	resourceList := make([]protocol.Resource, 0, len(resources))
	for _, entry := range resources {
		resourceList = append(resourceList, entry.resource)
	}

	resp, err := protocol.NewResponse(req.ID, map[string]interface{}{
		"resources": resourceList,
	})
	if err != nil {
		return protocol.NewErrorResponse(
			req.ID, protocol.CodeInternalError, err.Error(), nil,
		)
	}
	return resp
}

func handleReadResource(
	ctx context.Context,
	req *protocol.Request,
	resources map[string]*resourceEntry,
) *protocol.Response {
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return protocol.NewErrorResponse(
			req.ID, protocol.CodeInvalidParams,
			"invalid parameters", err.Error(),
		)
	}

	entry, ok := resources[params.URI]
	if !ok {
		return protocol.NewErrorResponse(
			req.ID, protocol.CodeInvalidParams,
			fmt.Sprintf("unknown resource: %s", params.URI), nil,
		)
	}

	content, err := entry.handler(ctx, params.URI)
	if err != nil {
		return protocol.NewErrorResponse(
			req.ID, protocol.CodeInternalError, err.Error(), nil,
		)
	}

	resp, marshalErr := protocol.NewResponse(req.ID, map[string]interface{}{
		"contents": []protocol.ResourceContent{*content},
	})
	if marshalErr != nil {
		return protocol.NewErrorResponse(
			req.ID, protocol.CodeInternalError,
			marshalErr.Error(), nil,
		)
	}
	return resp
}

func handleListPrompts(
	req *protocol.Request,
	prompts map[string]*promptEntry,
) *protocol.Response {
	promptList := make([]protocol.Prompt, 0, len(prompts))
	for _, entry := range prompts {
		promptList = append(promptList, entry.prompt)
	}

	resp, err := protocol.NewResponse(req.ID, map[string]interface{}{
		"prompts": promptList,
	})
	if err != nil {
		return protocol.NewErrorResponse(
			req.ID, protocol.CodeInternalError, err.Error(), nil,
		)
	}
	return resp
}

func handleGetPrompt(
	ctx context.Context,
	req *protocol.Request,
	prompts map[string]*promptEntry,
) *protocol.Response {
	var params struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return protocol.NewErrorResponse(
			req.ID, protocol.CodeInvalidParams,
			"invalid parameters", err.Error(),
		)
	}

	entry, ok := prompts[params.Name]
	if !ok {
		return protocol.NewErrorResponse(
			req.ID, protocol.CodeInvalidParams,
			fmt.Sprintf("unknown prompt: %s", params.Name), nil,
		)
	}

	messages, err := entry.handler(ctx, params.Arguments)
	if err != nil {
		return protocol.NewErrorResponse(
			req.ID, protocol.CodeInternalError, err.Error(), nil,
		)
	}

	resp, marshalErr := protocol.NewResponse(req.ID, map[string]interface{}{
		"messages": messages,
	})
	if marshalErr != nil {
		return protocol.NewErrorResponse(
			req.ID, protocol.CodeInternalError,
			marshalErr.Error(), nil,
		)
	}
	return resp
}
