package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.mcp/pkg/protocol"
)

func TestStdioServer_Initialize(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")

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

	reqData, err := json.Marshal(initReq)
	require.NoError(t, err)

	stdin := bytes.NewReader(append(reqData, '\n'))
	var stdout bytes.Buffer

	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = s.Serve(ctx)
	assert.NoError(t, err)

	var resp protocol.Response
	err = json.Unmarshal(stdout.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.IsError())

	var result protocol.InitializeResult
	err = json.Unmarshal(resp.Result, &result)
	require.NoError(t, err)
	assert.Equal(t, "test-server", result.ServerInfo.Name)
	assert.Equal(t, protocol.MCPProtocolVersion, result.ProtocolVersion)
}

func TestStdioServer_ListTools(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")
	s.RegisterTool(
		protocol.Tool{
			Name:        "echo",
			Description: "Echo input",
		},
		func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error) {
			return &protocol.ToolResult{
				Content: []protocol.ContentBlock{
					protocol.NewTextContent("echoed"),
				},
			}, nil
		},
	)

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/list",
	}
	reqData, _ := json.Marshal(req)
	stdin := bytes.NewReader(append(reqData, '\n'))
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	err := json.Unmarshal(stdout.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.IsError())

	var result struct {
		Tools []protocol.Tool `json:"tools"`
	}
	err = json.Unmarshal(resp.Result, &result)
	require.NoError(t, err)
	assert.Len(t, result.Tools, 1)
	assert.Equal(t, "echo", result.Tools[0].Name)
}

func TestStdioServer_CallTool(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")
	s.RegisterTool(
		protocol.Tool{Name: "greet", Description: "Greet someone"},
		func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error) {
			name, _ := args["name"].(string)
			return &protocol.ToolResult{
				Content: []protocol.ContentBlock{
					protocol.NewTextContent(fmt.Sprintf("Hello, %s!", name)),
				},
			}, nil
		},
	)

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"greet","arguments":{"name":"World"}}`),
	}
	reqData, _ := json.Marshal(req)
	stdin := bytes.NewReader(append(reqData, '\n'))
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	err := json.Unmarshal(stdout.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.IsError())

	var result protocol.ToolResult
	err = json.Unmarshal(resp.Result, &result)
	require.NoError(t, err)
	assert.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "Hello, World!")
}

func TestStdioServer_CallTool_Unknown(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"nonexistent","arguments":{}}`),
	}
	reqData, _ := json.Marshal(req)
	stdin := bytes.NewReader(append(reqData, '\n'))
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	err := json.Unmarshal(stdout.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.IsError())
	assert.Contains(t, resp.Error.Message, "unknown tool")
}

func TestStdioServer_ListResources(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")
	s.RegisterResource(
		protocol.Resource{
			URI:      "file:///tmp/test.txt",
			Name:     "test.txt",
			MimeType: "text/plain",
		},
		func(ctx context.Context, uri string) (*protocol.ResourceContent, error) {
			return &protocol.ResourceContent{
				URI:  uri,
				Text: "file contents",
			}, nil
		},
	)

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "resources/list",
	}
	reqData, _ := json.Marshal(req)
	stdin := bytes.NewReader(append(reqData, '\n'))
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	err := json.Unmarshal(stdout.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.IsError())
}

func TestStdioServer_ReadResource(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")
	s.RegisterResource(
		protocol.Resource{
			URI:  "file:///tmp/test.txt",
			Name: "test.txt",
		},
		func(ctx context.Context, uri string) (*protocol.ResourceContent, error) {
			return &protocol.ResourceContent{
				URI:  uri,
				Text: "hello from file",
			}, nil
		},
	)

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"file:///tmp/test.txt"}`),
	}
	reqData, _ := json.Marshal(req)
	stdin := bytes.NewReader(append(reqData, '\n'))
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	err := json.Unmarshal(stdout.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.IsError())
}

func TestStdioServer_ListPrompts(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")
	s.RegisterPrompt(
		protocol.Prompt{
			Name:        "review",
			Description: "Review code",
			Arguments: []protocol.PromptArgument{
				{Name: "code", Required: true},
			},
		},
		func(ctx context.Context, args map[string]string) ([]protocol.PromptMessage, error) {
			return []protocol.PromptMessage{
				{
					Role:    "user",
					Content: protocol.NewTextContent("Review: " + args["code"]),
				},
			}, nil
		},
	)

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "prompts/list",
	}
	reqData, _ := json.Marshal(req)
	stdin := bytes.NewReader(append(reqData, '\n'))
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	err := json.Unmarshal(stdout.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.IsError())
}

func TestStdioServer_GetPrompt(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")
	s.RegisterPrompt(
		protocol.Prompt{Name: "greet"},
		func(ctx context.Context, args map[string]string) ([]protocol.PromptMessage, error) {
			return []protocol.PromptMessage{
				{
					Role:    "assistant",
					Content: protocol.NewTextContent("Hello " + args["name"]),
				},
			}, nil
		},
	)

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "prompts/get",
		Params:  json.RawMessage(`{"name":"greet","arguments":{"name":"Alice"}}`),
	}
	reqData, _ := json.Marshal(req)
	stdin := bytes.NewReader(append(reqData, '\n'))
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	err := json.Unmarshal(stdout.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.IsError())
}

func TestStdioServer_MethodNotFound(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "unknown/method",
	}
	reqData, _ := json.Marshal(req)
	stdin := bytes.NewReader(append(reqData, '\n'))
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	err := json.Unmarshal(stdout.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.IsError())
	assert.Equal(t, protocol.CodeMethodNotFound, resp.Error.Code)
}

func TestStdioServer_InvalidJSON(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")
	stdin := strings.NewReader("not valid json\n")
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	err := json.Unmarshal(stdout.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.IsError())
	assert.Equal(t, protocol.CodeParseError, resp.Error.Code)
}

func TestStdioServer_Notification(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")

	// Notification has no ID, so no response should be written
	req := protocol.Request{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  json.RawMessage(`{}`),
	}
	reqData, _ := json.Marshal(req)
	stdin := bytes.NewReader(append(reqData, '\n'))
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	assert.Empty(t, stdout.String())
}

func TestStdioServer_ServerInfo(t *testing.T) {
	s := NewStdioServer("my-server", "2.0.0")
	info := s.ServerInfo()
	assert.Equal(t, "my-server", info.Name)
	assert.Equal(t, "2.0.0", info.Version)
}

func TestStdioServer_Capabilities(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(s *StdioServer)
		expectTools  bool
		expectRes    bool
		expectPrompt bool
	}{
		{
			name:  "empty server",
			setup: func(s *StdioServer) {},
		},
		{
			name: "with tools",
			setup: func(s *StdioServer) {
				s.RegisterTool(protocol.Tool{Name: "t"}, nil)
			},
			expectTools: true,
		},
		{
			name: "with resources",
			setup: func(s *StdioServer) {
				s.RegisterResource(
					protocol.Resource{URI: "file:///x"},
					nil,
				)
			},
			expectRes: true,
		},
		{
			name: "with prompts",
			setup: func(s *StdioServer) {
				s.RegisterPrompt(protocol.Prompt{Name: "p"}, nil)
			},
			expectPrompt: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStdioServer("test", "1.0.0")
			tt.setup(s)
			caps := s.Capabilities()
			if tt.expectTools {
				assert.NotNil(t, caps.Tools)
			} else {
				assert.Nil(t, caps.Tools)
			}
			if tt.expectRes {
				assert.NotNil(t, caps.Resources)
			} else {
				assert.Nil(t, caps.Resources)
			}
			if tt.expectPrompt {
				assert.NotNil(t, caps.Prompts)
			} else {
				assert.Nil(t, caps.Prompts)
			}
		})
	}
}

func TestHTTPServer_Health(t *testing.T) {
	s := NewHTTPServer("test-server", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterTool(
		protocol.Tool{Name: "test"},
		func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error) {
			return nil, nil
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var health map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &health)
	require.NoError(t, err)
	assert.Equal(t, "healthy", health["status"])
	assert.Equal(t, "test-server", health["server"])
}

func TestHTTPServer_Message_Initialize(t *testing.T) {
	s := NewHTTPServer("test-server", "1.0.0", DefaultHTTPServerConfig())

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
	body, _ := json.Marshal(initReq)

	req := httptest.NewRequest(
		http.MethodPost, "/message", bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp protocol.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.IsError())
}

func TestHTTPServer_Message_ToolCall(t *testing.T) {
	s := NewHTTPServer("test-server", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterTool(
		protocol.Tool{Name: "echo"},
		func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error) {
			msg, _ := args["message"].(string)
			return &protocol.ToolResult{
				Content: []protocol.ContentBlock{
					protocol.NewTextContent(msg),
				},
			}, nil
		},
	)

	callReq := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"echo","arguments":{"message":"hi"}}`),
	}
	body, _ := json.Marshal(callReq)

	req := httptest.NewRequest(
		http.MethodPost, "/message", bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp protocol.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.IsError())
}

func TestHTTPServer_Message_InvalidJSON(t *testing.T) {
	s := NewHTTPServer("test-server", "1.0.0", DefaultHTTPServerConfig())

	req := httptest.NewRequest(
		http.MethodPost, "/message",
		strings.NewReader("not json"),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var resp protocol.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.IsError())
	assert.Equal(t, protocol.CodeParseError, resp.Error.Code)
}

func TestHTTPServer_Message_WrongMethod(t *testing.T) {
	s := NewHTTPServer("test-server", "1.0.0", DefaultHTTPServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/message", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHTTPServer_Message_Notification(t *testing.T) {
	s := NewHTTPServer("test-server", "1.0.0", DefaultHTTPServerConfig())

	notifReq := protocol.Request{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  json.RawMessage(`{}`),
	}
	body, _ := json.Marshal(notifReq)

	req := httptest.NewRequest(
		http.MethodPost, "/message", bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestHTTPServer_Message_InvalidVersion(t *testing.T) {
	s := NewHTTPServer("test-server", "1.0.0", DefaultHTTPServerConfig())

	badReq := protocol.Request{
		JSONRPC: "1.0",
		ID:      float64(1),
		Method:  "test",
	}
	body, _ := json.Marshal(badReq)

	req := httptest.NewRequest(
		http.MethodPost, "/message", bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var resp protocol.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.IsError())
	assert.Equal(t, protocol.CodeInvalidRequest, resp.Error.Code)
}

func TestHTTPServer_SSE_WrongMethod(t *testing.T) {
	s := NewHTTPServer("test-server", "1.0.0", DefaultHTTPServerConfig())

	req := httptest.NewRequest(http.MethodPost, "/sse", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHTTPServer_ServerInfo(t *testing.T) {
	s := NewHTTPServer("my-server", "3.0.0", DefaultHTTPServerConfig())
	info := s.ServerInfo()
	assert.Equal(t, "my-server", info.Name)
	assert.Equal(t, "3.0.0", info.Version)
}

func TestHTTPServer_Capabilities(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterTool(protocol.Tool{Name: "t"}, nil)

	caps := s.Capabilities()
	assert.NotNil(t, caps.Tools)
	assert.Nil(t, caps.Resources)
	assert.Nil(t, caps.Prompts)
}

func TestHTTPServer_Handler(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	handler := s.Handler()
	assert.NotNil(t, handler)

	// Use handler directly
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var health map[string]interface{}
	err = json.Unmarshal(body, &health)
	require.NoError(t, err)
	assert.Equal(t, "healthy", health["status"])
}

// Verify interface compliance at compile time.
var _ Server = (*StdioServer)(nil)
var _ Server = (*HTTPServer)(nil)
