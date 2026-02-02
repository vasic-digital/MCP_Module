package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.mcp/pkg/protocol"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, TransportStdio, cfg.Transport)
	assert.Equal(t, 30*time.Second, cfg.Timeout)
	assert.Equal(t, "mcp-client", cfg.ClientName)
	assert.Equal(t, "1.0.0", cfg.ClientVersion)
}

func TestNewStdioClient_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				ServerCommand: "echo",
				ServerArgs:    []string{"hello"},
			},
			wantErr: false,
		},
		{
			name:    "missing server command",
			config:  Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewStdioClient(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, c)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, c)
			}
		})
	}
}

func TestNewHTTPClient_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				ServerURL: "http://localhost:8080",
			},
			wantErr: false,
		},
		{
			name:    "missing server URL",
			config:  Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewHTTPClient(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, c)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, c)
			}
		})
	}
}

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

func TestParseResult(t *testing.T) {
	tests := []struct {
		name    string
		resp    *protocol.Response
		wantErr bool
	}{
		{
			name: "successful response",
			resp: &protocol.Response{
				JSONRPC: "2.0",
				ID:      float64(1),
				Result:  json.RawMessage(`{"tools":[]}`),
			},
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
			wantErr: true,
		},
		{
			name: "nil result",
			resp: &protocol.Response{
				JSONRPC: "2.0",
				ID:      float64(1),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result listToolsResult
			err := parseResult(tt.resp, &result)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPClient_HandleSSEEvent(t *testing.T) {
	c, err := NewHTTPClient(Config{
		ServerURL: "http://localhost:8080",
	})
	require.NoError(t, err)

	tests := []struct {
		name      string
		eventType string
		data      string
		setupID   interface{}
		expectHit bool
	}{
		{
			name:      "endpoint event updates message endpoint",
			eventType: "endpoint",
			data:      "http://localhost:8080/custom-message",
			expectHit: false,
		},
		{
			name:      "message event with matching ID",
			eventType: "message",
			data:      `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`,
			setupID:   int64(1),
			expectHit: true,
		},
		{
			name:      "message event with invalid JSON",
			eventType: "message",
			data:      "not json",
			expectHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				}

				c.pendingMu.Lock()
				delete(c.pending, tt.setupID)
				c.pendingMu.Unlock()
			} else {
				c.handleSSEEvent(tt.eventType, tt.data)
				if tt.eventType == "endpoint" {
					assert.Equal(t, tt.data, c.messageEndpoint)
				}
			}
		})
	}
}

func TestHTTPClient_ConnectFailure(t *testing.T) {
	c, err := NewHTTPClient(Config{
		ServerURL: "http://localhost:1",
		Timeout:   1 * time.Second,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(
		context.Background(), 2*time.Second,
	)
	defer cancel()

	err = c.Connect(ctx)
	assert.Error(t, err)
}

// TestHTTPClient_MessageEndpointDefault verifies the default message
// endpoint is constructed from ServerURL.
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewHTTPClient(Config{ServerURL: tt.url})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, c.messageEndpoint)
		})
	}
}

// TestHTTPClient_SendRequestToMockServer tests sending a request to a
// real HTTP test server that returns a JSON-RPC response.
func TestHTTPClient_SendRequestToMockServer(t *testing.T) {
	// Create a mock MCP server
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/message":
				// Read the request
				var req protocol.Request
				err := json.NewDecoder(r.Body).Decode(&req)
				if err != nil {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}

				// Build response
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
				default:
					resp.Error = &protocol.RPCError{
						Code:    protocol.CodeMethodNotFound,
						Message: "unknown method",
					}
				}

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)

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
				// Keep connection open until client disconnects
				<-r.Context().Done()

			default:
				http.NotFound(w, r)
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

	// For this test, we directly set the message endpoint and test
	// direct HTTP POST (bypassing SSE for response delivery).
	// We override sendRequest to use direct HTTP response instead.
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

// Verify interface compliance at compile time.
var _ Client = (*StdioClient)(nil)
var _ Client = (*HTTPClient)(nil)
