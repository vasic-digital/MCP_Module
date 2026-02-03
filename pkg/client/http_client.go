package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"digital.vasic.mcp/pkg/protocol"
)

// HTTPClient communicates with an MCP server over HTTP/SSE.
type HTTPClient struct {
	config     Config
	httpClient *http.Client
	nextID     int64
	sseCancel  context.CancelFunc
	sseDone    chan struct{}
	closeOnce  sync.Once

	// SSE event handling
	pending   map[interface{}]chan *protocol.Response
	pendingMu sync.RWMutex

	// messageEndpoint is discovered from the SSE connection.
	messageEndpoint   string
	messageEndpointMu sync.RWMutex
}

// NewHTTPClient creates a new HTTP/SSE-based MCP client.
func NewHTTPClient(config Config) (*HTTPClient, error) {
	if config.ServerURL == "" {
		return nil, fmt.Errorf("server URL is required for HTTP client")
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

	return &HTTPClient{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		pending:         make(map[interface{}]chan *protocol.Response),
		sseDone:         make(chan struct{}),
		messageEndpoint: strings.TrimRight(config.ServerURL, "/") + "/message",
	}, nil
}

// Connect establishes the SSE connection for receiving events.
func (c *HTTPClient) Connect(ctx context.Context) error {
	sseURL := strings.TrimRight(c.config.ServerURL, "/") + "/sse"

	sseCtx, cancel := context.WithCancel(ctx)
	c.sseCancel = cancel

	req, err := http.NewRequestWithContext(sseCtx, http.MethodGet, sseURL, nil)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to connect to SSE endpoint: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		cancel()
		return fmt.Errorf("SSE connection failed with status: %d", resp.StatusCode)
	}

	go c.readSSE(resp.Body)

	return nil
}

// readSSE reads SSE events from the server.
func (c *HTTPClient) readSSE(body io.ReadCloser) {
	defer func() {
		_ = body.Close()
		close(c.sseDone)
	}()

	scanner := bufio.NewScanner(body)
	var eventType string
	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Empty line means end of event
			if len(dataLines) > 0 {
				data := strings.Join(dataLines, "\n")
				c.handleSSEEvent(eventType, data)
				eventType = ""
				dataLines = nil
			}
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		} else if strings.HasPrefix(line, ":") {
			// Comment/heartbeat, ignore
			continue
		}
	}
}

// handleSSEEvent processes an SSE event.
func (c *HTTPClient) handleSSEEvent(eventType, data string) {
	switch eventType {
	case "endpoint":
		c.messageEndpointMu.Lock()
		c.messageEndpoint = data
		c.messageEndpointMu.Unlock()
	case "message", "":
		var resp protocol.Response
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			return
		}
		if resp.ID != nil {
			normalizedID := protocol.NormalizeID(resp.ID)
			c.pendingMu.RLock()
			ch, exists := c.pending[normalizedID]
			c.pendingMu.RUnlock()
			if exists {
				select {
				case ch <- &resp:
				default:
				}
			}
		}
	}
}

// sendRequest sends a JSON-RPC request via HTTP POST and waits for
// the response via SSE.
func (c *HTTPClient) sendRequest(
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

	c.messageEndpointMu.RLock()
	endpoint := c.messageEndpoint
	c.messageEndpointMu.RUnlock()

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, endpoint, bytes.NewReader(data),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	_ = httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK &&
		httpResp.StatusCode != http.StatusAccepted &&
		httpResp.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf(
			"server returned status %d", httpResp.StatusCode,
		)
	}

	select {
	case resp, ok := <-respCh:
		if !ok {
			return nil, fmt.Errorf("response channel closed")
		}
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.sseDone:
		return nil, fmt.Errorf("SSE connection closed")
	}
}

// Initialize performs the MCP initialization handshake.
func (c *HTTPClient) Initialize(
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

	return &result, nil
}

// ListTools returns the tools available on the MCP server.
func (c *HTTPClient) ListTools(
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
func (c *HTTPClient) CallTool(
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
func (c *HTTPClient) ListResources(
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
func (c *HTTPClient) ReadResource(
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
func (c *HTTPClient) ListPrompts(
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
func (c *HTTPClient) GetPrompt(
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

// Close shuts down the HTTP client.
func (c *HTTPClient) Close() error {
	c.closeOnce.Do(func() {
		if c.sseCancel != nil {
			c.sseCancel()
		}
		c.pendingMu.Lock()
		for id, ch := range c.pending {
			close(ch)
			delete(c.pending, id)
		}
		c.pendingMu.Unlock()
	})
	return nil
}
