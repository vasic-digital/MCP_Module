package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"digital.vasic.mcp/pkg/protocol"
)

// StdioClient communicates with an MCP server via stdin/stdout.
type StdioClient struct {
	config Config
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	scanner   *bufio.Scanner
	stdinMu   sync.Mutex
	pending   map[interface{}]chan *protocol.Response
	pendingMu sync.RWMutex
	nextID    int64
	done      chan struct{}
	closeOnce sync.Once
}

// NewStdioClient creates a new stdio-based MCP client.
func NewStdioClient(config Config) (*StdioClient, error) {
	if config.ServerCommand == "" {
		return nil, fmt.Errorf("server command is required for stdio client")
	}
	if config.Timeout == 0 {
		config.Timeout = DefaultConfig().Timeout
	}
	if config.ClientName == "" {
		config.ClientName = DefaultConfig().ClientName
	}
	if config.ClientVersion == "" {
		config.ClientVersion = DefaultConfig().ClientVersion
	}

	return &StdioClient{
		config:  config,
		pending: make(map[interface{}]chan *protocol.Response),
		done:    make(chan struct{}),
	}, nil
}

// Start starts the MCP server process and begins reading responses.
func (c *StdioClient) Start() error {
	c.cmd = exec.Command(c.config.ServerCommand, c.config.ServerArgs...)
	c.cmd.Env = os.Environ()
	for k, v := range c.config.ServerEnv {
		c.cmd.Env = append(c.cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server process: %w", err)
	}

	const maxTokenSize = 10 * 1024 * 1024
	c.scanner = bufio.NewScanner(c.stdout)
	c.scanner.Buffer(make([]byte, maxTokenSize), maxTokenSize)

	go c.readLoop()

	return nil
}

// readLoop continuously reads JSON-RPC responses from the server stdout.
func (c *StdioClient) readLoop() {
	defer func() {
		c.pendingMu.Lock()
		for id, ch := range c.pending {
			close(ch)
			delete(c.pending, id)
		}
		c.pendingMu.Unlock()
	}()

	for c.scanner.Scan() {
		line := c.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var resp protocol.Response
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}

		if resp.ID != nil {
			normalizedID := protocol.NormalizeID(resp.ID)
			c.pendingMu.RLock()
			ch, exists := c.pending[normalizedID]
			c.pendingMu.RUnlock()

			if exists {
				respCopy := resp
				select {
				case ch <- &respCopy:
				default:
				}
			}
		}
	}
}

// sendRequest sends a JSON-RPC request and waits for the response.
func (c *StdioClient) sendRequest(
	ctx context.Context,
	method string,
	params interface{},
) (*protocol.Response, error) {
	id := atomic.AddInt64(&c.nextID, 1)

	req, err := protocol.NewRequest(id, method, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	respCh := make(chan *protocol.Response, 1)
	normalizedID := protocol.NormalizeID(id)

	c.pendingMu.Lock()
	c.pending[normalizedID] = respCh
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, normalizedID)
		c.pendingMu.Unlock()
	}()

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	c.stdinMu.Lock()
	_, writeErr := c.stdin.Write(append(data, '\n'))
	c.stdinMu.Unlock()
	if writeErr != nil {
		return nil, fmt.Errorf("failed to write to server: %w", writeErr)
	}

	select {
	case resp, ok := <-respCh:
		if !ok {
			return nil, fmt.Errorf("response channel closed")
		}
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, fmt.Errorf("client closed")
	}
}

// Initialize performs the MCP initialization handshake.
func (c *StdioClient) Initialize(
	ctx context.Context,
) (*protocol.InitializeResult, error) {
	params := protocol.InitializeParams{
		ProtocolVersion: protocol.MCPProtocolVersion,
		Capabilities:    map[string]interface{}{},
		ClientInfo: protocol.ClientInfo{
			Name:    c.config.ClientName,
			Version: c.config.ClientVersion,
		},
	}

	resp, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		return nil, fmt.Errorf("initialize failed: %w", err)
	}

	var result protocol.InitializeResult
	if err := parseResult(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse initialize result: %w", err)
	}

	// Send initialized notification
	notif, _ := protocol.NewNotification("notifications/initialized", map[string]interface{}{})
	data, _ := json.Marshal(notif)
	c.stdinMu.Lock()
	_, _ = c.stdin.Write(append(data, '\n'))
	c.stdinMu.Unlock()

	return &result, nil
}

// ListTools returns the tools available on the MCP server.
func (c *StdioClient) ListTools(
	ctx context.Context,
) ([]protocol.Tool, error) {
	resp, err := c.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}
	var result listToolsResult
	if err := parseResult(resp, &result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

// CallTool invokes a tool on the MCP server.
func (c *StdioClient) CallTool(
	ctx context.Context,
	name string,
	args map[string]interface{},
) (*protocol.ToolResult, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}
	resp, err := c.sendRequest(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}
	var result callToolResult
	if err := parseResult(resp, &result); err != nil {
		return nil, err
	}
	return &protocol.ToolResult{
		Content: result.Content,
		IsError: result.IsError,
	}, nil
}

// ListResources returns the resources available on the MCP server.
func (c *StdioClient) ListResources(
	ctx context.Context,
) ([]protocol.Resource, error) {
	resp, err := c.sendRequest(ctx, "resources/list", nil)
	if err != nil {
		return nil, err
	}
	var result listResourcesResult
	if err := parseResult(resp, &result); err != nil {
		return nil, err
	}
	return result.Resources, nil
}

// ReadResource reads a resource from the MCP server.
func (c *StdioClient) ReadResource(
	ctx context.Context,
	uri string,
) (*protocol.ResourceContent, error) {
	params := map[string]interface{}{
		"uri": uri,
	}
	resp, err := c.sendRequest(ctx, "resources/read", params)
	if err != nil {
		return nil, err
	}
	var result readResourceResult
	if err := parseResult(resp, &result); err != nil {
		return nil, err
	}
	if len(result.Contents) == 0 {
		return nil, fmt.Errorf("no content returned for resource: %s", uri)
	}
	return &result.Contents[0], nil
}

// ListPrompts returns the prompts available on the MCP server.
func (c *StdioClient) ListPrompts(
	ctx context.Context,
) ([]protocol.Prompt, error) {
	resp, err := c.sendRequest(ctx, "prompts/list", nil)
	if err != nil {
		return nil, err
	}
	var result listPromptsResult
	if err := parseResult(resp, &result); err != nil {
		return nil, err
	}
	return result.Prompts, nil
}

// GetPrompt retrieves a prompt with the given arguments.
func (c *StdioClient) GetPrompt(
	ctx context.Context,
	name string,
	args map[string]string,
) ([]protocol.PromptMessage, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}
	resp, err := c.sendRequest(ctx, "prompts/get", params)
	if err != nil {
		return nil, err
	}
	var result getPromptResult
	if err := parseResult(resp, &result); err != nil {
		return nil, err
	}
	return result.Messages, nil
}

// Close shuts down the client and cleans up resources.
func (c *StdioClient) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		close(c.done)

		if c.stdin != nil {
			_ = c.stdin.Close()
		}

		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
			_ = c.cmd.Wait()
		}
	})
	return closeErr
}
