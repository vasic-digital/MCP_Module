package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"digital.vasic.mcp/pkg/protocol"
)

// StdioServer is an MCP server that communicates via stdin/stdout.
type StdioServer struct {
	info      protocol.ServerInfo
	tools     map[string]*toolEntry
	resources map[string]*resourceEntry
	prompts   map[string]*promptEntry
	stdin     io.Reader
	stdout    io.Writer
}

// NewStdioServer creates a new stdio-based MCP server.
func NewStdioServer(name, version string) *StdioServer {
	return &StdioServer{
		info: protocol.ServerInfo{
			Name:    name,
			Version: version,
		},
		tools:     make(map[string]*toolEntry),
		resources: make(map[string]*resourceEntry),
		prompts:   make(map[string]*promptEntry),
		stdin:     os.Stdin,
		stdout:    os.Stdout,
	}
}

// SetIO overrides the default stdin/stdout for testing.
func (s *StdioServer) SetIO(stdin io.Reader, stdout io.Writer) {
	s.stdin = stdin
	s.stdout = stdout
}

// RegisterTool registers a tool with a handler.
func (s *StdioServer) RegisterTool(
	tool protocol.Tool,
	handler ToolHandler,
) {
	s.tools[tool.Name] = &toolEntry{tool: tool, handler: handler}
}

// RegisterResource registers a resource with a handler.
func (s *StdioServer) RegisterResource(
	resource protocol.Resource,
	handler ResourceHandler,
) {
	s.resources[resource.URI] = &resourceEntry{
		resource: resource,
		handler:  handler,
	}
}

// RegisterPrompt registers a prompt with a handler.
func (s *StdioServer) RegisterPrompt(
	prompt protocol.Prompt,
	handler PromptHandler,
) {
	s.prompts[prompt.Name] = &promptEntry{
		prompt:  prompt,
		handler: handler,
	}
}

// Serve starts reading from stdin and processing requests.
func (s *StdioServer) Serve(ctx context.Context) error {
	scanner := bufio.NewScanner(s.stdin)
	const maxTokenSize = 10 * 1024 * 1024
	scanner.Buffer(make([]byte, maxTokenSize), maxTokenSize)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("scanner error: %w", err)
			}
			return nil // EOF
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req protocol.Request
		if err := json.Unmarshal(line, &req); err != nil {
			errResp := protocol.NewErrorResponse(
				nil, protocol.CodeParseError,
				"invalid JSON", err.Error(),
			)
			s.writeResponse(errResp)
			continue
		}

		resp := handleRequest(
			ctx, &req, s.tools, s.resources, s.prompts, s.info,
		)
		if resp != nil {
			s.writeResponse(resp)
		}
	}
}

// writeResponse writes a JSON-RPC response to stdout.
func (s *StdioServer) writeResponse(resp *protocol.Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(s.stdout, "%s\n", data)
}

// ServerInfo returns the server identification.
func (s *StdioServer) ServerInfo() protocol.ServerInfo {
	return s.info
}

// Capabilities returns the server capabilities.
func (s *StdioServer) Capabilities() protocol.ServerCapabilities {
	caps := protocol.ServerCapabilities{}
	if len(s.tools) > 0 {
		caps.Tools = &protocol.ToolsCapability{}
	}
	if len(s.resources) > 0 {
		caps.Resources = &protocol.ResourcesCapability{}
	}
	if len(s.prompts) > 0 {
		caps.Prompts = &protocol.PromptsCapability{}
	}
	return caps
}
