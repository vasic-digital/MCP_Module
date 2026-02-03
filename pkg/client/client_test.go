package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.mcp/pkg/protocol"
)

// ============================================================================
// DefaultConfig Tests
// ============================================================================

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, TransportStdio, cfg.Transport)
	assert.Equal(t, 30*time.Second, cfg.Timeout)
	assert.Equal(t, "mcp-client", cfg.ClientName)
	assert.Equal(t, "1.0.0", cfg.ClientVersion)
}

func TestDefaultConfig_AllFieldsSet(t *testing.T) {
	cfg := DefaultConfig()
	assert.NotEmpty(t, cfg.Transport)
	assert.NotZero(t, cfg.Timeout)
	assert.NotEmpty(t, cfg.ClientName)
	assert.NotEmpty(t, cfg.ClientVersion)
}

// ============================================================================
// TransportType Tests
// ============================================================================

func TestTransportType_Values(t *testing.T) {
	assert.Equal(t, TransportType("stdio"), TransportStdio)
	assert.Equal(t, TransportType("http"), TransportHTTP)
}

// ============================================================================
// NewStdioClient Tests
// ============================================================================

func TestNewStdioClient_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with command only",
			config: Config{
				ServerCommand: "echo",
			},
			wantErr: false,
		},
		{
			name: "valid config with all fields",
			config: Config{
				ServerCommand: "echo",
				ServerArgs:    []string{"hello"},
				ServerEnv:     map[string]string{"KEY": "value"},
				Timeout:       10 * time.Second,
				ClientName:    "test-client",
				ClientVersion: "2.0.0",
			},
			wantErr: false,
		},
		{
			name:    "missing server command",
			config:  Config{},
			wantErr: true,
			errMsg:  "server command is required",
		},
		{
			name: "empty server command",
			config: Config{
				ServerCommand: "",
				ServerArgs:    []string{"arg1"},
			},
			wantErr: true,
			errMsg:  "server command is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewStdioClient(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, c)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, c)
			}
		})
	}
}

func TestNewStdioClient_DefaultsApplied(t *testing.T) {
	c, err := NewStdioClient(Config{
		ServerCommand: "echo",
	})
	require.NoError(t, err)

	assert.Equal(t, DefaultConfig().Timeout, c.config.Timeout)
	assert.Equal(t, DefaultConfig().ClientName, c.config.ClientName)
	assert.Equal(t, DefaultConfig().ClientVersion, c.config.ClientVersion)
}

func TestNewStdioClient_CustomValuesPreserved(t *testing.T) {
	cfg := Config{
		ServerCommand: "echo",
		Timeout:       5 * time.Second,
		ClientName:    "custom-client",
		ClientVersion: "3.0.0",
	}
	c, err := NewStdioClient(cfg)
	require.NoError(t, err)

	assert.Equal(t, 5*time.Second, c.config.Timeout)
	assert.Equal(t, "custom-client", c.config.ClientName)
	assert.Equal(t, "3.0.0", c.config.ClientVersion)
}

// ============================================================================
// NewHTTPClient Tests
// ============================================================================

func TestNewHTTPClient_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with URL only",
			config: Config{
				ServerURL: "http://localhost:8080",
			},
			wantErr: false,
		},
		{
			name: "valid config with all fields",
			config: Config{
				ServerURL:     "http://localhost:8080",
				Timeout:       10 * time.Second,
				ClientName:    "test-client",
				ClientVersion: "2.0.0",
			},
			wantErr: false,
		},
		{
			name:    "missing server URL",
			config:  Config{},
			wantErr: true,
			errMsg:  "server URL is required",
		},
		{
			name: "empty server URL",
			config: Config{
				ServerURL: "",
			},
			wantErr: true,
			errMsg:  "server URL is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewHTTPClient(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, c)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, c)
			}
		})
	}
}

func TestNewHTTPClient_DefaultsApplied(t *testing.T) {
	c, err := NewHTTPClient(Config{
		ServerURL: "http://localhost:8080",
	})
	require.NoError(t, err)

	assert.Equal(t, DefaultConfig().Timeout, c.config.Timeout)
	assert.Equal(t, DefaultConfig().ClientName, c.config.ClientName)
	assert.Equal(t, DefaultConfig().ClientVersion, c.config.ClientVersion)
}

func TestNewHTTPClient_CustomValuesPreserved(t *testing.T) {
	cfg := Config{
		ServerURL:     "http://localhost:9090",
		Timeout:       5 * time.Second,
		ClientName:    "custom-client",
		ClientVersion: "3.0.0",
	}
	c, err := NewHTTPClient(cfg)
	require.NoError(t, err)

	assert.Equal(t, 5*time.Second, c.config.Timeout)
	assert.Equal(t, "custom-client", c.config.ClientName)
	assert.Equal(t, "3.0.0", c.config.ClientVersion)
}

// ============================================================================
// HTTPClient MessageEndpoint Tests
// ============================================================================

func TestHTTPClient_MessageEndpointDefault(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "without trailing slash",
			url:      "http://localhost:8080",
			expected: "http://localhost:8080/message",
		},
		{
			name:     "with trailing slash",
			url:      "http://localhost:8080/",
			expected: "http://localhost:8080/message",
		},
		{
			name:     "with path",
			url:      "http://localhost:8080/api/v1/",
			expected: "http://localhost:8080/api/v1/message",
		},
		{
			name:     "https URL",
			url:      "https://secure.example.com",
			expected: "https://secure.example.com/message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewHTTPClient(Config{ServerURL: tt.url})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, c.messageEndpoint)
		})
	}
}

// ============================================================================
// HTTPClient Close Tests
// ============================================================================

func TestHTTPClient_Close(t *testing.T) {
	c, err := NewHTTPClient(Config{
		ServerURL: "http://localhost:8080",
	})
	require.NoError(t, err)

	// Close should not panic
	err = c.Close()
	assert.NoError(t, err)

	// Double close should be safe
	err = c.Close()
	assert.NoError(t, err)
}

func TestHTTPClient_Close_WithPendingChannels(t *testing.T) {
	c, err := NewHTTPClient(Config{
		ServerURL: "http://localhost:8080",
	})
	require.NoError(t, err)

	// Add some pending channels
	ch1 := make(chan *protocol.Response, 1)
	ch2 := make(chan *protocol.Response, 1)
	c.pendingMu.Lock()
	c.pending[int64(1)] = ch1
	c.pending[int64(2)] = ch2
	c.pendingMu.Unlock()

	// Close should clean up pending channels
	err = c.Close()
	assert.NoError(t, err)

	// Channels should be closed
	_, ok := <-ch1
	assert.False(t, ok)
	_, ok = <-ch2
	assert.False(t, ok)
}

// ============================================================================
// StdioClient Close Tests
// ============================================================================

func TestStdioClient_Close(t *testing.T) {
	c, err := NewStdioClient(Config{
		ServerCommand: "echo",
	})
	require.NoError(t, err)

	// Close should not panic even without Start
	err = c.Close()
	assert.NoError(t, err)

	// Double close should be safe
	err = c.Close()
	assert.NoError(t, err)
}

func TestStdioClient_Close_WithPendingChannels(t *testing.T) {
	c, err := NewStdioClient(Config{
		ServerCommand: "echo",
	})
	require.NoError(t, err)

	// Add some pending channels
	ch := make(chan *protocol.Response, 1)
	c.pendingMu.Lock()
	c.pending[int64(1)] = ch
	c.pendingMu.Unlock()

	// Close should not panic
	err = c.Close()
	assert.NoError(t, err)
}

// ============================================================================
// parseResult Tests
// ============================================================================

func TestParseResult(t *testing.T) {
	tests := []struct {
		name    string
		resp    *protocol.Response
		target  interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "successful response with tools",
			resp: &protocol.Response{
				JSONRPC: "2.0",
				ID:      float64(1),
				Result:  json.RawMessage(`{"tools":[{"name":"test"}]}`),
			},
			target:  &listToolsResult{},
			wantErr: false,
		},
		{
			name: "successful response with resources",
			resp: &protocol.Response{
				JSONRPC: "2.0",
				ID:      float64(1),
				Result:  json.RawMessage(`{"resources":[{"uri":"file:///test"}]}`),
			},
			target:  &listResourcesResult{},
			wantErr: false,
		},
		{
			name: "error response",
			resp: &protocol.Response{
				JSONRPC: "2.0",
				ID:      float64(1),
				Error: &protocol.RPCError{
					Code:    protocol.CodeMethodNotFound,
					Message: "method not found",
				},
			},
			target:  &listToolsResult{},
			wantErr: true,
		},
		{
			name: "nil result",
			resp: &protocol.Response{
				JSONRPC: "2.0",
				ID:      float64(1),
			},
			target:  &listToolsResult{},
			wantErr: true,
			errMsg:  "response has no result",
		},
		{
			name: "invalid JSON in result",
			resp: &protocol.Response{
				JSONRPC: "2.0",
				ID:      float64(1),
				Result:  json.RawMessage(`not valid json`),
			},
			target:  &listToolsResult{},
			wantErr: true,
		},
		{
			name: "error response with data",
			resp: &protocol.Response{
				JSONRPC: "2.0",
				ID:      float64(1),
				Error: &protocol.RPCError{
					Code:    protocol.CodeInternalError,
					Message: "internal error",
					Data:    "additional details",
				},
			},
			target:  &listToolsResult{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseResult(tt.resp, tt.target)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseResult_ParsesCorrectly(t *testing.T) {
	resp := &protocol.Response{
		JSONRPC: "2.0",
		ID:      float64(1),
		Result: json.RawMessage(`{
			"tools": [
				{"name": "tool1", "description": "First tool"},
				{"name": "tool2", "description": "Second tool"}
			]
		}`),
	}

	var result listToolsResult
	err := parseResult(resp, &result)
	require.NoError(t, err)
	assert.Len(t, result.Tools, 2)
	assert.Equal(t, "tool1", result.Tools[0].Name)
	assert.Equal(t, "tool2", result.Tools[1].Name)
}

// ============================================================================
// HTTPClient HandleSSEEvent Tests
// ============================================================================

func TestHTTPClient_HandleSSEEvent(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		data      string
		setupID   interface{}
		expectHit bool
		validate  func(t *testing.T, c *HTTPClient)
	}{
		{
			name:      "endpoint event updates message endpoint",
			eventType: "endpoint",
			data:      "http://localhost:8080/custom-message",
			expectHit: false,
			validate: func(t *testing.T, c *HTTPClient) {
				assert.Equal(t, "http://localhost:8080/custom-message", c.messageEndpoint)
			},
		},
		{
			name:      "message event with matching ID int64",
			eventType: "message",
			data:      `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`,
			setupID:   int64(1),
			expectHit: true,
		},
		{
			name:      "empty event type treated as message",
			eventType: "",
			data:      `{"jsonrpc":"2.0","id":2,"result":{"tools":[]}}`,
			setupID:   int64(2),
			expectHit: true,
		},
		{
			name:      "message event with invalid JSON",
			eventType: "message",
			data:      "not json",
			expectHit: false,
		},
		{
			name:      "message event with no matching ID",
			eventType: "message",
			data:      `{"jsonrpc":"2.0","id":999,"result":{}}`,
			setupID:   int64(1),
			expectHit: false,
		},
		{
			name:      "message event with nil ID in response",
			eventType: "message",
			data:      `{"jsonrpc":"2.0","result":{}}`,
			setupID:   int64(1),
			expectHit: false,
		},
		{
			name:      "unknown event type ignored",
			eventType: "custom_event",
			data:      "some data",
			expectHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewHTTPClient(Config{ServerURL: "http://localhost:8080"})
			require.NoError(t, err)

			if tt.setupID != nil {
				ch := make(chan *protocol.Response, 1)
				c.pendingMu.Lock()
				c.pending[tt.setupID] = ch
				c.pendingMu.Unlock()

				c.handleSSEEvent(tt.eventType, tt.data)

				if tt.expectHit {
					select {
					case resp := <-ch:
						assert.NotNil(t, resp)
					default:
						t.Error("expected response on channel")
					}
				} else {
					select {
					case <-ch:
						t.Error("unexpected response on channel")
					default:
						// Expected
					}
				}

				c.pendingMu.Lock()
				delete(c.pending, tt.setupID)
				c.pendingMu.Unlock()
			} else {
				c.handleSSEEvent(tt.eventType, tt.data)
				if tt.validate != nil {
					tt.validate(t, c)
				}
			}
		})
	}
}

func TestHTTPClient_HandleSSEEvent_ChannelFull(t *testing.T) {
	c, err := NewHTTPClient(Config{ServerURL: "http://localhost:8080"})
	require.NoError(t, err)

	// Create a channel that's already full
	ch := make(chan *protocol.Response, 1)
	ch <- &protocol.Response{} // Fill the channel

	c.pendingMu.Lock()
	c.pending[int64(1)] = ch
	c.pendingMu.Unlock()

	// This should not block since the channel is full
	c.handleSSEEvent("message", `{"jsonrpc":"2.0","id":1,"result":{}}`)

	// Channel should still have the original response
	resp := <-ch
	assert.NotNil(t, resp)
}

// ============================================================================
// HTTPClient Connect Tests
// ============================================================================

func TestHTTPClient_ConnectFailure(t *testing.T) {
	c, err := NewHTTPClient(Config{
		ServerURL: "http://localhost:1",
		Timeout:   1 * time.Second,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to SSE endpoint")
}

func TestHTTPClient_Connect_BadStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		},
	))
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   5 * time.Second,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SSE connection failed with status")
}

func TestHTTPClient_Connect_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/sse" {
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				flusher, ok := w.(http.Flusher)
				if !ok {
					http.Error(w, "no flusher", 500)
					return
				}
				_, _ = fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
				flusher.Flush()
				<-r.Context().Done()
			}
		},
	))
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   5 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	assert.NoError(t, err)
}

// ============================================================================
// HTTPClient ReadSSE Tests
// ============================================================================

func TestHTTPClient_ReadSSE_ParsesEvents(t *testing.T) {
	c, err := NewHTTPClient(Config{ServerURL: "http://localhost:8080"})
	require.NoError(t, err)

	// Create a pipe to simulate SSE stream
	sseData := `event: endpoint
data: http://custom/message

event: message
data: {"jsonrpc":"2.0","id":1,"result":{}}

:heartbeat

`
	body := io.NopCloser(strings.NewReader(sseData))

	// Setup a pending channel for ID 1
	ch := make(chan *protocol.Response, 1)
	c.pendingMu.Lock()
	c.pending[int64(1)] = ch
	c.pendingMu.Unlock()

	// Run readSSE in background
	done := make(chan struct{})
	go func() {
		c.readSSE(body)
		close(done)
	}()

	// Wait for readSSE to finish
	<-done

	// Verify endpoint was updated
	assert.Equal(t, "http://custom/message", c.messageEndpoint)

	// Verify message was received
	select {
	case resp := <-ch:
		assert.NotNil(t, resp)
	default:
		t.Error("expected response on channel")
	}
}

// ============================================================================
// HTTPClient SendRequest Tests
// ============================================================================

func TestHTTPClient_SendRequest_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/sse" {
				w.Header().Set("Content-Type", "text/event-stream")
				flusher, _ := w.(http.Flusher)
				_, _ = fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
				flusher.Flush()
				<-r.Context().Done()
			} else if r.URL.Path == "/message" {
				// Don't respond, let the context cancel
				time.Sleep(5 * time.Second)
			}
		},
	))
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   5 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	_ = c.Connect(ctx)

	// Cancel context immediately
	cancel()

	// sendRequest should return context error
	_, err = c.sendRequest(ctx, "test", nil)
	assert.Error(t, err)
}

func TestHTTPClient_SendRequest_BadHTTPStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/sse" {
				w.Header().Set("Content-Type", "text/event-stream")
				flusher, _ := w.(http.Flusher)
				_, _ = fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
				flusher.Flush()
				<-r.Context().Done()
			} else if r.URL.Path == "/message" {
				w.WriteHeader(http.StatusInternalServerError)
			}
		},
	))
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   5 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	_ = c.Connect(ctx)

	_, err = c.sendRequest(ctx, "test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

// ============================================================================
// Mock MCP Server for Integration Tests
// ============================================================================

func createMockMCPServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/sse":
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				flusher, ok := w.(http.Flusher)
				if !ok {
					http.Error(w, "no flusher", 500)
					return
				}
				_, _ = fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
				flusher.Flush()
				<-r.Context().Done()

			case "/message":
				var req protocol.Request
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}

				resp := protocol.Response{
					JSONRPC: "2.0",
					ID:      req.ID,
				}

				switch req.Method {
				case "initialize":
					result, _ := json.Marshal(protocol.InitializeResult{
						ProtocolVersion: protocol.MCPProtocolVersion,
						Capabilities:    protocol.ServerCapabilities{},
						ServerInfo: protocol.ServerInfo{
							Name:    "mock-server",
							Version: "1.0.0",
						},
					})
					resp.Result = result
				case "tools/list":
					result, _ := json.Marshal(listToolsResult{
						Tools: []protocol.Tool{
							{Name: "test_tool", Description: "A test tool"},
						},
					})
					resp.Result = result
				case "tools/call":
					result, _ := json.Marshal(callToolResult{
						Content: []protocol.ContentBlock{
							protocol.NewTextContent("tool result"),
						},
					})
					resp.Result = result
				case "resources/list":
					result, _ := json.Marshal(listResourcesResult{
						Resources: []protocol.Resource{
							{URI: "file:///test", Name: "test"},
						},
					})
					resp.Result = result
				case "resources/read":
					result, _ := json.Marshal(readResourceResult{
						Contents: []protocol.ResourceContent{
							{URI: "file:///test", Text: "content"},
						},
					})
					resp.Result = result
				case "prompts/list":
					result, _ := json.Marshal(listPromptsResult{
						Prompts: []protocol.Prompt{
							{Name: "test_prompt"},
						},
					})
					resp.Result = result
				case "prompts/get":
					result, _ := json.Marshal(getPromptResult{
						Messages: []protocol.PromptMessage{
							{Role: "user", Content: protocol.NewTextContent("hello")},
						},
					})
					resp.Result = result
				default:
					resp.Error = &protocol.RPCError{
						Code:    protocol.CodeMethodNotFound,
						Message: "unknown method",
					}
				}

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)

			default:
				http.NotFound(w, r)
			}
		},
	))
}

func TestHTTPClient_SendRequestToMockServer(t *testing.T) {
	server := createMockMCPServer(t)
	defer server.Close()

	ctx := context.Background()

	// Test direct HTTP POST to /message
	reqData, err := json.Marshal(protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/list",
	})
	require.NoError(t, err)

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		server.URL+"/message",
		bytes.NewReader(reqData),
	)
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer func() { _ = httpResp.Body.Close() }()

	var resp protocol.Response
	err = json.NewDecoder(httpResp.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp.IsError())

	var result listToolsResult
	err = json.Unmarshal(resp.Result, &result)
	require.NoError(t, err)
	assert.Len(t, result.Tools, 1)
	assert.Equal(t, "test_tool", result.Tools[0].Name)
}

// ============================================================================
// HTTPClient Method Tests with Mock Server
// ============================================================================

func TestHTTPClient_Initialize_DirectHTTP(t *testing.T) {
	server := createMockMCPServer(t)
	defer server.Close()

	ctx := context.Background()

	initReq := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
		Params: json.RawMessage(`{
			"protocolVersion": "2024-11-05",
			"capabilities": {},
			"clientInfo": {"name": "test", "version": "1.0.0"}
		}`),
	}
	reqData, _ := json.Marshal(initReq)

	httpReq, _ := http.NewRequestWithContext(
		ctx, http.MethodPost,
		server.URL+"/message",
		bytes.NewReader(reqData),
	)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer func() { _ = httpResp.Body.Close() }()

	var resp protocol.Response
	_ = json.NewDecoder(httpResp.Body).Decode(&resp)
	assert.False(t, resp.IsError())

	var result protocol.InitializeResult
	_ = json.Unmarshal(resp.Result, &result)
	assert.Equal(t, "mock-server", result.ServerInfo.Name)
}

func TestHTTPClient_CallTool_DirectHTTP(t *testing.T) {
	server := createMockMCPServer(t)
	defer server.Close()

	ctx := context.Background()

	callReq := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"test_tool","arguments":{}}`),
	}
	reqData, _ := json.Marshal(callReq)

	httpReq, _ := http.NewRequestWithContext(
		ctx, http.MethodPost,
		server.URL+"/message",
		bytes.NewReader(reqData),
	)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer func() { _ = httpResp.Body.Close() }()

	var resp protocol.Response
	_ = json.NewDecoder(httpResp.Body).Decode(&resp)
	assert.False(t, resp.IsError())
}

func TestHTTPClient_ListResources_DirectHTTP(t *testing.T) {
	server := createMockMCPServer(t)
	defer server.Close()

	ctx := context.Background()

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "resources/list",
	}
	reqData, _ := json.Marshal(req)

	httpReq, _ := http.NewRequestWithContext(
		ctx, http.MethodPost,
		server.URL+"/message",
		bytes.NewReader(reqData),
	)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer func() { _ = httpResp.Body.Close() }()

	var resp protocol.Response
	_ = json.NewDecoder(httpResp.Body).Decode(&resp)
	assert.False(t, resp.IsError())

	var result listResourcesResult
	_ = json.Unmarshal(resp.Result, &result)
	assert.Len(t, result.Resources, 1)
}

func TestHTTPClient_ReadResource_DirectHTTP(t *testing.T) {
	server := createMockMCPServer(t)
	defer server.Close()

	ctx := context.Background()

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"file:///test"}`),
	}
	reqData, _ := json.Marshal(req)

	httpReq, _ := http.NewRequestWithContext(
		ctx, http.MethodPost,
		server.URL+"/message",
		bytes.NewReader(reqData),
	)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer func() { _ = httpResp.Body.Close() }()

	var resp protocol.Response
	_ = json.NewDecoder(httpResp.Body).Decode(&resp)
	assert.False(t, resp.IsError())
}

func TestHTTPClient_ListPrompts_DirectHTTP(t *testing.T) {
	server := createMockMCPServer(t)
	defer server.Close()

	ctx := context.Background()

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "prompts/list",
	}
	reqData, _ := json.Marshal(req)

	httpReq, _ := http.NewRequestWithContext(
		ctx, http.MethodPost,
		server.URL+"/message",
		bytes.NewReader(reqData),
	)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer func() { _ = httpResp.Body.Close() }()

	var resp protocol.Response
	_ = json.NewDecoder(httpResp.Body).Decode(&resp)
	assert.False(t, resp.IsError())
}

func TestHTTPClient_GetPrompt_DirectHTTP(t *testing.T) {
	server := createMockMCPServer(t)
	defer server.Close()

	ctx := context.Background()

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "prompts/get",
		Params:  json.RawMessage(`{"name":"test_prompt","arguments":{}}`),
	}
	reqData, _ := json.Marshal(req)

	httpReq, _ := http.NewRequestWithContext(
		ctx, http.MethodPost,
		server.URL+"/message",
		bytes.NewReader(reqData),
	)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer func() { _ = httpResp.Body.Close() }()

	var resp protocol.Response
	_ = json.NewDecoder(httpResp.Body).Decode(&resp)
	assert.False(t, resp.IsError())
}

// ============================================================================
// StdioClient ReadResource Edge Cases
// ============================================================================

func TestStdioClient_ReadResource_EmptyContents(t *testing.T) {
	// Test that ReadResource returns error when no contents
	resp := &protocol.Response{
		JSONRPC: "2.0",
		ID:      float64(1),
		Result:  json.RawMessage(`{"contents":[]}`),
	}

	var result readResourceResult
	err := parseResult(resp, &result)
	require.NoError(t, err)
	assert.Empty(t, result.Contents)
}

// ============================================================================
// Concurrent Access Tests
// ============================================================================

func TestHTTPClient_ConcurrentPendingAccess(t *testing.T) {
	c, err := NewHTTPClient(Config{ServerURL: "http://localhost:8080"})
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			ch := make(chan *protocol.Response, 1)
			c.pendingMu.Lock()
			c.pending[id] = ch
			c.pendingMu.Unlock()

			time.Sleep(time.Millisecond)

			c.pendingMu.Lock()
			delete(c.pending, id)
			c.pendingMu.Unlock()
		}(int64(i))
	}
	wg.Wait()
}

func TestStdioClient_ConcurrentPendingAccess(t *testing.T) {
	c, err := NewStdioClient(Config{ServerCommand: "echo"})
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			ch := make(chan *protocol.Response, 1)
			c.pendingMu.Lock()
			c.pending[id] = ch
			c.pendingMu.Unlock()

			time.Sleep(time.Millisecond)

			c.pendingMu.Lock()
			delete(c.pending, id)
			c.pendingMu.Unlock()
		}(int64(i))
	}
	wg.Wait()
}

// ============================================================================
// Result Type Tests
// ============================================================================

func TestListToolsResult_Unmarshal(t *testing.T) {
	data := `{"tools":[{"name":"t1"},{"name":"t2"}]}`
	var result listToolsResult
	err := json.Unmarshal([]byte(data), &result)
	require.NoError(t, err)
	assert.Len(t, result.Tools, 2)
}

func TestListResourcesResult_Unmarshal(t *testing.T) {
	data := `{"resources":[{"uri":"file:///a","name":"a"}]}`
	var result listResourcesResult
	err := json.Unmarshal([]byte(data), &result)
	require.NoError(t, err)
	assert.Len(t, result.Resources, 1)
}

func TestReadResourceResult_Unmarshal(t *testing.T) {
	data := `{"contents":[{"uri":"file:///a","text":"hello"}]}`
	var result readResourceResult
	err := json.Unmarshal([]byte(data), &result)
	require.NoError(t, err)
	assert.Len(t, result.Contents, 1)
	assert.Equal(t, "hello", result.Contents[0].Text)
}

func TestListPromptsResult_Unmarshal(t *testing.T) {
	data := `{"prompts":[{"name":"p1"}]}`
	var result listPromptsResult
	err := json.Unmarshal([]byte(data), &result)
	require.NoError(t, err)
	assert.Len(t, result.Prompts, 1)
}

func TestGetPromptResult_Unmarshal(t *testing.T) {
	data := `{"messages":[{"role":"user","content":{"type":"text","text":"hi"}}]}`
	var result getPromptResult
	err := json.Unmarshal([]byte(data), &result)
	require.NoError(t, err)
	assert.Len(t, result.Messages, 1)
	assert.Equal(t, "user", result.Messages[0].Role)
}

func TestCallToolResult_Unmarshal(t *testing.T) {
	data := `{"content":[{"type":"text","text":"result"}],"isError":false}`
	var result callToolResult
	err := json.Unmarshal([]byte(data), &result)
	require.NoError(t, err)
	assert.Len(t, result.Content, 1)
	assert.False(t, result.IsError)
}

func TestCallToolResult_Unmarshal_Error(t *testing.T) {
	data := `{"content":[{"type":"text","text":"error message"}],"isError":true}`
	var result callToolResult
	err := json.Unmarshal([]byte(data), &result)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// ============================================================================
// StdioClient Start Tests
// ============================================================================

func TestStdioClient_Start_InvalidCommand(t *testing.T) {
	c, err := NewStdioClient(Config{
		ServerCommand: "/nonexistent/binary/that/does/not/exist",
	})
	require.NoError(t, err)

	err = c.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start server process")
}

func TestStdioClient_Start_WithEnv(t *testing.T) {
	c, err := NewStdioClient(Config{
		ServerCommand: "sleep",
		ServerArgs:    []string{"1"},
		ServerEnv:     map[string]string{"TEST_VAR": "test_value"},
	})
	require.NoError(t, err)

	err = c.Start()
	require.NoError(t, err)

	err = c.Close()
	assert.NoError(t, err)
}

// ============================================================================
// HTTPClient Full Integration Tests with SSE
// ============================================================================

// mockSSEServer creates an HTTP server that properly handles SSE and responds
// to MCP requests via SSE events.
func mockSSEServer(t *testing.T) *httptest.Server {
	clients := make(map[string]chan *protocol.Response)
	clientsMu := sync.Mutex{}

	return httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/sse":
				flusher, ok := w.(http.Flusher)
				if !ok {
					http.Error(w, "no flusher", 500)
					return
				}

				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Connection", "keep-alive")

				clientID := fmt.Sprintf("client-%d", time.Now().UnixNano())
				respCh := make(chan *protocol.Response, 100)

				clientsMu.Lock()
				clients[clientID] = respCh
				clientsMu.Unlock()

				defer func() {
					clientsMu.Lock()
					delete(clients, clientID)
					clientsMu.Unlock()
				}()

				_, _ = fmt.Fprintf(w, "event: connected\ndata: {\"clientId\":\"%s\"}\n\n", clientID)
				_, _ = fmt.Fprintf(w, "event: endpoint\ndata: http://%s/message\n\n", r.Host)
				flusher.Flush()

				for {
					select {
					case resp := <-respCh:
						data, _ := json.Marshal(resp)
						_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
						flusher.Flush()
					case <-r.Context().Done():
						return
					}
				}

			case "/message":
				var req protocol.Request
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}

				resp := protocol.Response{
					JSONRPC: "2.0",
					ID:      req.ID,
				}

				switch req.Method {
				case "initialize":
					result, _ := json.Marshal(protocol.InitializeResult{
						ProtocolVersion: protocol.MCPProtocolVersion,
						Capabilities:    protocol.ServerCapabilities{},
						ServerInfo:      protocol.ServerInfo{Name: "mock", Version: "1.0"},
					})
					resp.Result = result
				case "tools/list":
					result, _ := json.Marshal(listToolsResult{
						Tools: []protocol.Tool{{Name: "test_tool"}},
					})
					resp.Result = result
				case "tools/call":
					result, _ := json.Marshal(callToolResult{
						Content: []protocol.ContentBlock{protocol.NewTextContent("result")},
					})
					resp.Result = result
				case "resources/list":
					result, _ := json.Marshal(listResourcesResult{
						Resources: []protocol.Resource{{URI: "file:///test", Name: "test"}},
					})
					resp.Result = result
				case "resources/read":
					result, _ := json.Marshal(readResourceResult{
						Contents: []protocol.ResourceContent{{URI: "file:///test", Text: "content"}},
					})
					resp.Result = result
				case "prompts/list":
					result, _ := json.Marshal(listPromptsResult{
						Prompts: []protocol.Prompt{{Name: "test_prompt"}},
					})
					resp.Result = result
				case "prompts/get":
					result, _ := json.Marshal(getPromptResult{
						Messages: []protocol.PromptMessage{{Role: "user", Content: protocol.NewTextContent("hi")}},
					})
					resp.Result = result
				default:
					resp.Error = &protocol.RPCError{
						Code:    protocol.CodeMethodNotFound,
						Message: "unknown",
					}
				}

				// Broadcast to SSE clients
				clientsMu.Lock()
				for _, ch := range clients {
					select {
					case ch <- &resp:
					default:
					}
				}
				clientsMu.Unlock()

				w.WriteHeader(http.StatusAccepted)
			}
		},
	))
}

func TestHTTPClient_Initialize_Full(t *testing.T) {
	server := mockSSEServer(t)
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   10 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	require.NoError(t, err)

	// Give time for SSE connection to stabilize
	time.Sleep(100 * time.Millisecond)

	result, err := c.Initialize(ctx)
	require.NoError(t, err)
	assert.Equal(t, "mock", result.ServerInfo.Name)
}

func TestHTTPClient_ListTools_Full(t *testing.T) {
	server := mockSSEServer(t)
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   10 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	tools, err := c.ListTools(ctx)
	require.NoError(t, err)
	assert.Len(t, tools, 1)
	assert.Equal(t, "test_tool", tools[0].Name)
}

func TestHTTPClient_CallTool_Full(t *testing.T) {
	server := mockSSEServer(t)
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   10 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	result, err := c.CallTool(ctx, "test_tool", map[string]interface{}{"key": "value"})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Content, 1)
}

func TestHTTPClient_ListResources_Full(t *testing.T) {
	server := mockSSEServer(t)
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   10 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	resources, err := c.ListResources(ctx)
	require.NoError(t, err)
	assert.Len(t, resources, 1)
}

func TestHTTPClient_ReadResource_Full(t *testing.T) {
	server := mockSSEServer(t)
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   10 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	content, err := c.ReadResource(ctx, "file:///test")
	require.NoError(t, err)
	assert.Equal(t, "content", content.Text)
}

func TestHTTPClient_ReadResource_EmptyContents(t *testing.T) {
	// Create a server that returns empty contents
	respChan := make(chan *protocol.Response, 10)

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/sse":
				flusher, ok := w.(http.Flusher)
				if !ok {
					http.Error(w, "no flusher", 500)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				_, _ = fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
				_, _ = fmt.Fprintf(w, "event: endpoint\ndata: http://%s/message\n\n", r.Host)
				flusher.Flush()

				// Wait for responses and send them via SSE
				for {
					select {
					case resp := <-respChan:
						data, _ := json.Marshal(resp)
						_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
						flusher.Flush()
					case <-r.Context().Done():
						return
					}
				}
			case "/message":
				var req protocol.Request
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}

				// Return empty contents for resources/read
				resp := &protocol.Response{
					JSONRPC: "2.0",
					ID:      req.ID,
				}
				emptyResult, _ := json.Marshal(readResourceResult{Contents: []protocol.ResourceContent{}})
				resp.Result = emptyResult

				// Send response via SSE channel
				respChan <- resp
				w.WriteHeader(http.StatusAccepted)
			}
		},
	))
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   5 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	_, err = c.ReadResource(ctx, "file:///test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no content returned")
}

func TestHTTPClient_ListPrompts_Full(t *testing.T) {
	server := mockSSEServer(t)
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   10 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	prompts, err := c.ListPrompts(ctx)
	require.NoError(t, err)
	assert.Len(t, prompts, 1)
}

func TestHTTPClient_GetPrompt_Full(t *testing.T) {
	server := mockSSEServer(t)
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   10 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	messages, err := c.GetPrompt(ctx, "test_prompt", map[string]string{"key": "value"})
	require.NoError(t, err)
	assert.Len(t, messages, 1)
}

// ============================================================================
// Interface Compliance Tests
// ============================================================================

var _ Client = (*StdioClient)(nil)
var _ Client = (*HTTPClient)(nil)

// ============================================================================
// Error Wrapping Tests
// ============================================================================

func TestParseResult_ErrorFromResponse(t *testing.T) {
	rpcErr := &protocol.RPCError{
		Code:    protocol.CodeInternalError,
		Message: "something went wrong",
		Data:    "details",
	}
	resp := &protocol.Response{
		JSONRPC: "2.0",
		ID:      float64(1),
		Error:   rpcErr,
	}

	var result listToolsResult
	err := parseResult(resp, &result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, rpcErr))
}

// ============================================================================
// StdioClient Method Tests using Mock Pipes
// ============================================================================

// mockStdioPipes creates a pair of connected pipes for testing StdioClient
type mockStdioPipes struct {
	clientWriter *io.PipeWriter
	clientReader *io.PipeReader
	serverWriter *io.PipeWriter
	serverReader *io.PipeReader
}

func newMockStdioPipes() *mockStdioPipes {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	return &mockStdioPipes{
		clientWriter: clientWriter,
		clientReader: clientReader,
		serverWriter: serverWriter,
		serverReader: serverReader,
	}
}

func (m *mockStdioPipes) close() {
	_ = m.clientWriter.Close()
	_ = m.clientReader.Close()
	_ = m.serverWriter.Close()
	_ = m.serverReader.Close()
}

// mockStdioServer simulates a MCP server that responds to requests
func mockStdioServer(t *testing.T, serverReader *io.PipeReader, serverWriter *io.PipeWriter, handler func(*protocol.Request) *protocol.Response) {
	scanner := bufio.NewScanner(serverReader)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req protocol.Request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}

		resp := handler(&req)
		if resp != nil {
			data, _ := json.Marshal(resp)
			_, _ = serverWriter.Write(append(data, '\n'))
		}
	}
}

// createStdioClientWithPipes creates a StdioClient connected to mock pipes
func createStdioClientWithPipes(pipes *mockStdioPipes) *StdioClient {
	client := &StdioClient{
		config: Config{
			ServerCommand: "test",
			ClientName:    "test-client",
			ClientVersion: "1.0.0",
		},
		pending: make(map[interface{}]chan *protocol.Response),
		done:    make(chan struct{}),
		stdin:   pipes.clientWriter,
		stdout:  pipes.clientReader,
	}
	const maxTokenSize = 10 * 1024 * 1024
	client.scanner = bufio.NewScanner(client.stdout)
	client.scanner.Buffer(make([]byte, maxTokenSize), maxTokenSize)
	go client.readLoop()
	return client
}

func TestStdioClient_Initialize_WithMockServer(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	// Start mock server
	go mockStdioServer(t, pipes.serverReader, pipes.serverWriter, func(req *protocol.Request) *protocol.Response {
		if req.Method == "initialize" {
			result, _ := json.Marshal(protocol.InitializeResult{
				ProtocolVersion: protocol.MCPProtocolVersion,
				Capabilities:    protocol.ServerCapabilities{},
				ServerInfo:      protocol.ServerInfo{Name: "mock", Version: "1.0"},
			})
			return &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  result,
			}
		}
		return nil
	})

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := client.Initialize(ctx)
	require.NoError(t, err)
	assert.Equal(t, protocol.MCPProtocolVersion, result.ProtocolVersion)
	assert.Equal(t, "mock", result.ServerInfo.Name)
}

func TestStdioClient_Initialize_Error(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	// Start mock server that returns an error
	go mockStdioServer(t, pipes.serverReader, pipes.serverWriter, func(req *protocol.Request) *protocol.Response {
		if req.Method == "initialize" {
			return &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &protocol.RPCError{
					Code:    protocol.CodeInternalError,
					Message: "initialization failed",
				},
			}
		}
		return nil
	})

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.Initialize(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "initialization failed")
}

func TestStdioClient_ListTools_WithMockServer(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	go mockStdioServer(t, pipes.serverReader, pipes.serverWriter, func(req *protocol.Request) *protocol.Response {
		if req.Method == "tools/list" {
			result, _ := json.Marshal(listToolsResult{
				Tools: []protocol.Tool{
					{Name: "tool1", Description: "First tool"},
					{Name: "tool2", Description: "Second tool"},
				},
			})
			return &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  result,
			}
		}
		return nil
	})

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tools, err := client.ListTools(ctx)
	require.NoError(t, err)
	assert.Len(t, tools, 2)
	assert.Equal(t, "tool1", tools[0].Name)
}

func TestStdioClient_ListTools_Error(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	go mockStdioServer(t, pipes.serverReader, pipes.serverWriter, func(req *protocol.Request) *protocol.Response {
		if req.Method == "tools/list" {
			return &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &protocol.RPCError{
					Code:    protocol.CodeInternalError,
					Message: "tools unavailable",
				},
			}
		}
		return nil
	})

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.ListTools(ctx)
	assert.Error(t, err)
}

func TestStdioClient_CallTool_WithMockServer(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	go mockStdioServer(t, pipes.serverReader, pipes.serverWriter, func(req *protocol.Request) *protocol.Response {
		if req.Method == "tools/call" {
			result, _ := json.Marshal(callToolResult{
				Content: []protocol.ContentBlock{protocol.NewTextContent("result data")},
				IsError: false,
			})
			return &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  result,
			}
		}
		return nil
	})

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := client.CallTool(ctx, "test_tool", map[string]interface{}{"arg": "value"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
}

func TestStdioClient_CallTool_Error(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	go mockStdioServer(t, pipes.serverReader, pipes.serverWriter, func(req *protocol.Request) *protocol.Response {
		if req.Method == "tools/call" {
			return &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &protocol.RPCError{
					Code:    protocol.CodeInternalError,
					Message: "tool execution failed",
				},
			}
		}
		return nil
	})

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.CallTool(ctx, "test_tool", nil)
	assert.Error(t, err)
}

func TestStdioClient_ListResources_WithMockServer(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	go mockStdioServer(t, pipes.serverReader, pipes.serverWriter, func(req *protocol.Request) *protocol.Response {
		if req.Method == "resources/list" {
			result, _ := json.Marshal(listResourcesResult{
				Resources: []protocol.Resource{
					{URI: "file:///doc1.txt", Name: "Document 1"},
					{URI: "file:///doc2.txt", Name: "Document 2"},
				},
			})
			return &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  result,
			}
		}
		return nil
	})

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resources, err := client.ListResources(ctx)
	require.NoError(t, err)
	assert.Len(t, resources, 2)
	assert.Equal(t, "file:///doc1.txt", resources[0].URI)
}

func TestStdioClient_ListResources_Error(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	go mockStdioServer(t, pipes.serverReader, pipes.serverWriter, func(req *protocol.Request) *protocol.Response {
		if req.Method == "resources/list" {
			return &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &protocol.RPCError{
					Code:    protocol.CodeInternalError,
					Message: "resources unavailable",
				},
			}
		}
		return nil
	})

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.ListResources(ctx)
	assert.Error(t, err)
}

func TestStdioClient_ReadResource_WithMockServer(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	go mockStdioServer(t, pipes.serverReader, pipes.serverWriter, func(req *protocol.Request) *protocol.Response {
		if req.Method == "resources/read" {
			result, _ := json.Marshal(readResourceResult{
				Contents: []protocol.ResourceContent{
					{URI: "file:///test.txt", Text: "Hello, World!"},
				},
			})
			return &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  result,
			}
		}
		return nil
	})

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	content, err := client.ReadResource(ctx, "file:///test.txt")
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", content.Text)
}

func TestStdioClient_ReadResource_NoContents(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	go mockStdioServer(t, pipes.serverReader, pipes.serverWriter, func(req *protocol.Request) *protocol.Response {
		if req.Method == "resources/read" {
			result, _ := json.Marshal(readResourceResult{
				Contents: []protocol.ResourceContent{},
			})
			return &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  result,
			}
		}
		return nil
	})

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.ReadResource(ctx, "file:///test.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no content returned")
}

func TestStdioClient_ReadResource_Error(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	go mockStdioServer(t, pipes.serverReader, pipes.serverWriter, func(req *protocol.Request) *protocol.Response {
		if req.Method == "resources/read" {
			return &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &protocol.RPCError{
					Code:    protocol.CodeInternalError,
					Message: "resource not found",
				},
			}
		}
		return nil
	})

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.ReadResource(ctx, "file:///test.txt")
	assert.Error(t, err)
}

func TestStdioClient_ListPrompts_WithMockServer(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	go mockStdioServer(t, pipes.serverReader, pipes.serverWriter, func(req *protocol.Request) *protocol.Response {
		if req.Method == "prompts/list" {
			result, _ := json.Marshal(listPromptsResult{
				Prompts: []protocol.Prompt{
					{Name: "summarize", Description: "Summarize text"},
					{Name: "translate", Description: "Translate text"},
				},
			})
			return &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  result,
			}
		}
		return nil
	})

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	prompts, err := client.ListPrompts(ctx)
	require.NoError(t, err)
	assert.Len(t, prompts, 2)
	assert.Equal(t, "summarize", prompts[0].Name)
}

func TestStdioClient_ListPrompts_Error(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	go mockStdioServer(t, pipes.serverReader, pipes.serverWriter, func(req *protocol.Request) *protocol.Response {
		if req.Method == "prompts/list" {
			return &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &protocol.RPCError{
					Code:    protocol.CodeInternalError,
					Message: "prompts unavailable",
				},
			}
		}
		return nil
	})

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.ListPrompts(ctx)
	assert.Error(t, err)
}

func TestStdioClient_GetPrompt_WithMockServer(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	go mockStdioServer(t, pipes.serverReader, pipes.serverWriter, func(req *protocol.Request) *protocol.Response {
		if req.Method == "prompts/get" {
			result, _ := json.Marshal(getPromptResult{
				Messages: []protocol.PromptMessage{
					{Role: "user", Content: protocol.NewTextContent("Summarize this")},
					{Role: "assistant", Content: protocol.NewTextContent("Here is the summary...")},
				},
			})
			return &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  result,
			}
		}
		return nil
	})

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	messages, err := client.GetPrompt(ctx, "summarize", map[string]string{"text": "input"})
	require.NoError(t, err)
	assert.Len(t, messages, 2)
	assert.Equal(t, "user", messages[0].Role)
}

func TestStdioClient_GetPrompt_Error(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	go mockStdioServer(t, pipes.serverReader, pipes.serverWriter, func(req *protocol.Request) *protocol.Response {
		if req.Method == "prompts/get" {
			return &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &protocol.RPCError{
					Code:    protocol.CodeInternalError,
					Message: "prompt not found",
				},
			}
		}
		return nil
	})

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.GetPrompt(ctx, "unknown", nil)
	assert.Error(t, err)
}

func TestStdioClient_SendRequest_ContextCancelled(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	// Start a server that reads but never responds
	go func() {
		scanner := bufio.NewScanner(pipes.serverReader)
		for scanner.Scan() {
			// Read requests but don't respond
		}
	}()

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.ListTools(ctx)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
}

func TestStdioClient_SendRequest_ClientClosed(t *testing.T) {
	pipes := newMockStdioPipes()

	// Start a server that reads
	go func() {
		scanner := bufio.NewScanner(pipes.serverReader)
		for scanner.Scan() {
			// Read requests
		}
	}()

	client := createStdioClientWithPipes(pipes)

	// Close client immediately
	_ = client.Close()
	pipes.close()

	ctx := context.Background()
	_, err := client.ListTools(ctx)
	assert.Error(t, err)
}

func TestStdioClient_ReadLoop_InvalidJSON(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	// Start server that reads requests and sends responses
	go func() {
		scanner := bufio.NewScanner(pipes.serverReader)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var req protocol.Request
			if json.Unmarshal(line, &req) != nil {
				continue
			}
			// Send invalid JSON first (client should ignore)
			_, _ = pipes.serverWriter.Write([]byte("not valid json\n"))
			// Then send valid response
			resp := &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"tools":[]}`),
			}
			data, _ := json.Marshal(resp)
			_, _ = pipes.serverWriter.Write(append(data, '\n'))
		}
	}()

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tools, err := client.ListTools(ctx)
	require.NoError(t, err)
	assert.Empty(t, tools)
}

func TestStdioClient_ReadLoop_EmptyLines(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	// Start server that reads requests and sends responses with empty lines
	go func() {
		scanner := bufio.NewScanner(pipes.serverReader)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var req protocol.Request
			if json.Unmarshal(line, &req) != nil {
				continue
			}
			// Send empty lines first
			_, _ = pipes.serverWriter.Write([]byte("\n\n"))
			// Then send valid response
			resp := &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"tools":[{"name":"test"}]}`),
			}
			data, _ := json.Marshal(resp)
			_, _ = pipes.serverWriter.Write(append(data, '\n'))
		}
	}()

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tools, err := client.ListTools(ctx)
	require.NoError(t, err)
	assert.Len(t, tools, 1)
}

func TestStdioClient_ReadLoop_ResponseWithNoMatchingID(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	// Start server that reads requests and sends mismatched response first
	go func() {
		scanner := bufio.NewScanner(pipes.serverReader)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var req protocol.Request
			if json.Unmarshal(line, &req) != nil {
				continue
			}
			// Send response with wrong ID first
			resp := &protocol.Response{
				JSONRPC: "2.0",
				ID:      float64(999),
				Result:  json.RawMessage(`{"tools":[]}`),
			}
			data, _ := json.Marshal(resp)
			_, _ = pipes.serverWriter.Write(append(data, '\n'))
			// Then send correct response
			resp2 := &protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"tools":[{"name":"correct"}]}`),
			}
			data2, _ := json.Marshal(resp2)
			_, _ = pipes.serverWriter.Write(append(data2, '\n'))
		}
	}()

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tools, err := client.ListTools(ctx)
	require.NoError(t, err)
	assert.Len(t, tools, 1)
	assert.Equal(t, "correct", tools[0].Name)
}

func TestStdioClient_ConcurrentRequests(t *testing.T) {
	pipes := newMockStdioPipes()
	defer pipes.close()

	// Start mock server that handles concurrent requests
	go mockStdioServer(t, pipes.serverReader, pipes.serverWriter, func(req *protocol.Request) *protocol.Response {
		result, _ := json.Marshal(listToolsResult{
			Tools: []protocol.Tool{{Name: fmt.Sprintf("tool-for-%v", req.ID)}},
		})
		return &protocol.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}
	})

	client := createStdioClientWithPipes(pipes)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	results := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.ListTools(ctx)
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	for err := range results {
		assert.NoError(t, err)
	}
}

// ============================================================================
// HTTPClient Error Path Tests
// ============================================================================

func TestHTTPClient_Initialize_Error(t *testing.T) {
	server := mockSSEServer(t)
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   10 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Initialize should succeed
	result, err := c.Initialize(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestHTTPClient_ListTools_Error(t *testing.T) {
	// Create a server that returns an error for tools/list
	respChan := make(chan *protocol.Response, 10)

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/sse":
				flusher, ok := w.(http.Flusher)
				if !ok {
					http.Error(w, "no flusher", 500)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
				_, _ = fmt.Fprintf(w, "event: endpoint\ndata: http://%s/message\n\n", r.Host)
				flusher.Flush()

				for {
					select {
					case resp := <-respChan:
						data, _ := json.Marshal(resp)
						_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
						flusher.Flush()
					case <-r.Context().Done():
						return
					}
				}
			case "/message":
				var req protocol.Request
				_ = json.NewDecoder(r.Body).Decode(&req)

				resp := &protocol.Response{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &protocol.RPCError{
						Code:    protocol.CodeInternalError,
						Message: "tools unavailable",
					},
				}
				respChan <- resp
				w.WriteHeader(http.StatusAccepted)
			}
		},
	))
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   5 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	_, err = c.ListTools(ctx)
	assert.Error(t, err)
}

func TestHTTPClient_CallTool_Error(t *testing.T) {
	respChan := make(chan *protocol.Response, 10)

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/sse":
				flusher, ok := w.(http.Flusher)
				if !ok {
					http.Error(w, "no flusher", 500)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
				_, _ = fmt.Fprintf(w, "event: endpoint\ndata: http://%s/message\n\n", r.Host)
				flusher.Flush()

				for {
					select {
					case resp := <-respChan:
						data, _ := json.Marshal(resp)
						_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
						flusher.Flush()
					case <-r.Context().Done():
						return
					}
				}
			case "/message":
				var req protocol.Request
				_ = json.NewDecoder(r.Body).Decode(&req)

				resp := &protocol.Response{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &protocol.RPCError{
						Code:    protocol.CodeInternalError,
						Message: "tool execution failed",
					},
				}
				respChan <- resp
				w.WriteHeader(http.StatusAccepted)
			}
		},
	))
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   5 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	_, err = c.CallTool(ctx, "test_tool", nil)
	assert.Error(t, err)
}

func TestHTTPClient_ListResources_Error(t *testing.T) {
	respChan := make(chan *protocol.Response, 10)

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/sse":
				flusher, ok := w.(http.Flusher)
				if !ok {
					http.Error(w, "no flusher", 500)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
				_, _ = fmt.Fprintf(w, "event: endpoint\ndata: http://%s/message\n\n", r.Host)
				flusher.Flush()

				for {
					select {
					case resp := <-respChan:
						data, _ := json.Marshal(resp)
						_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
						flusher.Flush()
					case <-r.Context().Done():
						return
					}
				}
			case "/message":
				var req protocol.Request
				_ = json.NewDecoder(r.Body).Decode(&req)

				resp := &protocol.Response{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &protocol.RPCError{
						Code:    protocol.CodeInternalError,
						Message: "resources unavailable",
					},
				}
				respChan <- resp
				w.WriteHeader(http.StatusAccepted)
			}
		},
	))
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   5 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	_, err = c.ListResources(ctx)
	assert.Error(t, err)
}

func TestHTTPClient_ListPrompts_Error(t *testing.T) {
	respChan := make(chan *protocol.Response, 10)

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/sse":
				flusher, ok := w.(http.Flusher)
				if !ok {
					http.Error(w, "no flusher", 500)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
				_, _ = fmt.Fprintf(w, "event: endpoint\ndata: http://%s/message\n\n", r.Host)
				flusher.Flush()

				for {
					select {
					case resp := <-respChan:
						data, _ := json.Marshal(resp)
						_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
						flusher.Flush()
					case <-r.Context().Done():
						return
					}
				}
			case "/message":
				var req protocol.Request
				_ = json.NewDecoder(r.Body).Decode(&req)

				resp := &protocol.Response{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &protocol.RPCError{
						Code:    protocol.CodeInternalError,
						Message: "prompts unavailable",
					},
				}
				respChan <- resp
				w.WriteHeader(http.StatusAccepted)
			}
		},
	))
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   5 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	_, err = c.ListPrompts(ctx)
	assert.Error(t, err)
}

func TestHTTPClient_GetPrompt_Error(t *testing.T) {
	respChan := make(chan *protocol.Response, 10)

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/sse":
				flusher, ok := w.(http.Flusher)
				if !ok {
					http.Error(w, "no flusher", 500)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
				_, _ = fmt.Fprintf(w, "event: endpoint\ndata: http://%s/message\n\n", r.Host)
				flusher.Flush()

				for {
					select {
					case resp := <-respChan:
						data, _ := json.Marshal(resp)
						_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
						flusher.Flush()
					case <-r.Context().Done():
						return
					}
				}
			case "/message":
				var req protocol.Request
				_ = json.NewDecoder(r.Body).Decode(&req)

				resp := &protocol.Response{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &protocol.RPCError{
						Code:    protocol.CodeInternalError,
						Message: "prompt not found",
					},
				}
				respChan <- resp
				w.WriteHeader(http.StatusAccepted)
			}
		},
	))
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   5 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	_, err = c.GetPrompt(ctx, "unknown", nil)
	assert.Error(t, err)
}

func TestHTTPClient_SSEConnectionClosed(t *testing.T) {
	// Create a server that closes SSE connection immediately after endpoint event
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/sse":
				flusher, ok := w.(http.Flusher)
				if !ok {
					http.Error(w, "no flusher", 500)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
				_, _ = fmt.Fprintf(w, "event: endpoint\ndata: http://%s/message\n\n", r.Host)
				flusher.Flush()
				// Close connection immediately
				return
			case "/message":
				w.WriteHeader(http.StatusAccepted)
			}
		},
	))
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   5 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond) // Wait for SSE to close

	// Request should fail because SSE is closed
	_, err = c.ListTools(ctx)
	assert.Error(t, err)
}

func TestHTTPClient_ReadResource_Error(t *testing.T) {
	respChan := make(chan *protocol.Response, 10)

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/sse":
				flusher, ok := w.(http.Flusher)
				if !ok {
					http.Error(w, "no flusher", 500)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
				_, _ = fmt.Fprintf(w, "event: endpoint\ndata: http://%s/message\n\n", r.Host)
				flusher.Flush()

				for {
					select {
					case resp := <-respChan:
						data, _ := json.Marshal(resp)
						_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
						flusher.Flush()
					case <-r.Context().Done():
						return
					}
				}
			case "/message":
				var req protocol.Request
				_ = json.NewDecoder(r.Body).Decode(&req)

				resp := &protocol.Response{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &protocol.RPCError{
						Code:    protocol.CodeInternalError,
						Message: "resource not found",
					},
				}
				respChan <- resp
				w.WriteHeader(http.StatusAccepted)
			}
		},
	))
	defer server.Close()

	c, err := NewHTTPClient(Config{
		ServerURL: server.URL,
		Timeout:   5 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = c.Connect(ctx)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	_, err = c.ReadResource(ctx, "file:///unknown")
	assert.Error(t, err)
}
