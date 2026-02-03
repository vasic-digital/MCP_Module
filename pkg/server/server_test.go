package server

import (
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
// StdioServer Creation Tests
// ============================================================================

func TestNewStdioServer(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")
	assert.Equal(t, "test-server", s.info.Name)
	assert.Equal(t, "1.0.0", s.info.Version)
	assert.NotNil(t, s.tools)
	assert.NotNil(t, s.resources)
	assert.NotNil(t, s.prompts)
}

func TestNewStdioServer_EmptyName(t *testing.T) {
	s := NewStdioServer("", "1.0.0")
	assert.Equal(t, "", s.info.Name)
	assert.Equal(t, "1.0.0", s.info.Version)
}

func TestNewStdioServer_EmptyVersion(t *testing.T) {
	s := NewStdioServer("server", "")
	assert.Equal(t, "server", s.info.Name)
	assert.Equal(t, "", s.info.Version)
}

func TestStdioServer_SetIO(t *testing.T) {
	s := NewStdioServer("test", "1.0.0")

	stdin := strings.NewReader("")
	var stdout bytes.Buffer

	s.SetIO(stdin, &stdout)

	assert.Equal(t, stdin, s.stdin)
	assert.Equal(t, &stdout, s.stdout)
}

// ============================================================================
// StdioServer Registration Tests
// ============================================================================

func TestStdioServer_RegisterTool(t *testing.T) {
	s := NewStdioServer("test", "1.0.0")

	tool := protocol.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{"type": "string"},
			},
		},
	}
	handler := func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error) {
		return &protocol.ToolResult{
			Content: []protocol.ContentBlock{protocol.NewTextContent("result")},
		}, nil
	}

	s.RegisterTool(tool, handler)

	assert.Contains(t, s.tools, "test_tool")
	assert.Equal(t, tool.Name, s.tools["test_tool"].tool.Name)
}

func TestStdioServer_RegisterMultipleTools(t *testing.T) {
	s := NewStdioServer("test", "1.0.0")

	tools := []string{"tool1", "tool2", "tool3"}
	for _, name := range tools {
		s.RegisterTool(protocol.Tool{Name: name}, nil)
	}

	assert.Len(t, s.tools, 3)
	for _, name := range tools {
		assert.Contains(t, s.tools, name)
	}
}

func TestStdioServer_RegisterResource(t *testing.T) {
	s := NewStdioServer("test", "1.0.0")

	resource := protocol.Resource{
		URI:         "file:///test.txt",
		Name:        "test.txt",
		Description: "A test file",
		MimeType:    "text/plain",
	}
	handler := func(ctx context.Context, uri string) (*protocol.ResourceContent, error) {
		return &protocol.ResourceContent{URI: uri, Text: "content"}, nil
	}

	s.RegisterResource(resource, handler)

	assert.Contains(t, s.resources, "file:///test.txt")
}

func TestStdioServer_RegisterPrompt(t *testing.T) {
	s := NewStdioServer("test", "1.0.0")

	prompt := protocol.Prompt{
		Name:        "test_prompt",
		Description: "A test prompt",
		Arguments: []protocol.PromptArgument{
			{Name: "input", Required: true},
		},
	}
	handler := func(ctx context.Context, args map[string]string) ([]protocol.PromptMessage, error) {
		return []protocol.PromptMessage{
			{Role: "user", Content: protocol.NewTextContent("message")},
		}, nil
	}

	s.RegisterPrompt(prompt, handler)

	assert.Contains(t, s.prompts, "test_prompt")
}

// ============================================================================
// StdioServer Initialize Tests
// ============================================================================

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

func TestStdioServer_Initialize_WithCapabilities(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")
	s.RegisterTool(protocol.Tool{Name: "tool1"}, nil)
	s.RegisterResource(protocol.Resource{URI: "file:///test"}, nil)
	s.RegisterPrompt(protocol.Prompt{Name: "prompt1"}, nil)

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
	stdin := bytes.NewReader(append(reqData, '\n'))
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	_ = json.Unmarshal(stdout.Bytes(), &resp)

	var result protocol.InitializeResult
	_ = json.Unmarshal(resp.Result, &result)

	assert.NotNil(t, result.Capabilities.Tools)
	assert.NotNil(t, result.Capabilities.Resources)
	assert.NotNil(t, result.Capabilities.Prompts)
}

// ============================================================================
// StdioServer ListTools Tests
// ============================================================================

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

func TestStdioServer_ListTools_Empty(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")

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
	_ = json.Unmarshal(stdout.Bytes(), &resp)
	assert.False(t, resp.IsError())

	var result struct {
		Tools []protocol.Tool `json:"tools"`
	}
	_ = json.Unmarshal(resp.Result, &result)
	assert.Empty(t, result.Tools)
}

// ============================================================================
// StdioServer CallTool Tests
// ============================================================================

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

func TestStdioServer_CallTool_HandlerError(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")
	s.RegisterTool(
		protocol.Tool{Name: "failing"},
		func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error) {
			return nil, errors.New("tool execution failed")
		},
	)

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"failing","arguments":{}}`),
	}
	reqData, _ := json.Marshal(req)
	stdin := bytes.NewReader(append(reqData, '\n'))
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	_ = json.Unmarshal(stdout.Bytes(), &resp)
	assert.True(t, resp.IsError())
	assert.Equal(t, protocol.CodeInternalError, resp.Error.Code)
}

func TestStdioServer_CallTool_InvalidParams(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")
	s.RegisterTool(protocol.Tool{Name: "test"}, func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error) {
		return &protocol.ToolResult{}, nil
	})

	// Send raw JSON directly to avoid marshaling issues with invalid params
	rawRequest := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":"invalid_params_not_object"}` + "\n"
	stdin := strings.NewReader(rawRequest)
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	err := json.Unmarshal(stdout.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.IsError())
	assert.Equal(t, protocol.CodeInvalidParams, resp.Error.Code)
}

// ============================================================================
// StdioServer ListResources Tests
// ============================================================================

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

	var result struct {
		Resources []protocol.Resource `json:"resources"`
	}
	_ = json.Unmarshal(resp.Result, &result)
	assert.Len(t, result.Resources, 1)
}

func TestStdioServer_ListResources_Empty(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")

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
	_ = json.Unmarshal(stdout.Bytes(), &resp)
	assert.False(t, resp.IsError())
}

// ============================================================================
// StdioServer ReadResource Tests
// ============================================================================

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

	var result struct {
		Contents []protocol.ResourceContent `json:"contents"`
	}
	_ = json.Unmarshal(resp.Result, &result)
	assert.Len(t, result.Contents, 1)
	assert.Equal(t, "hello from file", result.Contents[0].Text)
}

func TestStdioServer_ReadResource_Unknown(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"file:///nonexistent"}`),
	}
	reqData, _ := json.Marshal(req)
	stdin := bytes.NewReader(append(reqData, '\n'))
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	_ = json.Unmarshal(stdout.Bytes(), &resp)
	assert.True(t, resp.IsError())
	assert.Contains(t, resp.Error.Message, "unknown resource")
}

func TestStdioServer_ReadResource_HandlerError(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")
	s.RegisterResource(
		protocol.Resource{URI: "file:///failing"},
		func(ctx context.Context, uri string) (*protocol.ResourceContent, error) {
			return nil, errors.New("read failed")
		},
	)

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"file:///failing"}`),
	}
	reqData, _ := json.Marshal(req)
	stdin := bytes.NewReader(append(reqData, '\n'))
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	_ = json.Unmarshal(stdout.Bytes(), &resp)
	assert.True(t, resp.IsError())
}

func TestStdioServer_ReadResource_InvalidParams(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")

	// Send raw JSON directly to avoid marshaling issues with invalid params
	rawRequest := `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":"invalid_params_not_object"}` + "\n"
	stdin := strings.NewReader(rawRequest)
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	err := json.Unmarshal(stdout.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.IsError())
	assert.Equal(t, protocol.CodeInvalidParams, resp.Error.Code)
}

// ============================================================================
// StdioServer ListPrompts Tests
// ============================================================================

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

	var result struct {
		Prompts []protocol.Prompt `json:"prompts"`
	}
	_ = json.Unmarshal(resp.Result, &result)
	assert.Len(t, result.Prompts, 1)
}

// ============================================================================
// StdioServer GetPrompt Tests
// ============================================================================

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

	var result struct {
		Messages []protocol.PromptMessage `json:"messages"`
	}
	_ = json.Unmarshal(resp.Result, &result)
	assert.Len(t, result.Messages, 1)
	assert.Contains(t, result.Messages[0].Content.Text, "Alice")
}

func TestStdioServer_GetPrompt_Unknown(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "prompts/get",
		Params:  json.RawMessage(`{"name":"unknown","arguments":{}}`),
	}
	reqData, _ := json.Marshal(req)
	stdin := bytes.NewReader(append(reqData, '\n'))
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	_ = json.Unmarshal(stdout.Bytes(), &resp)
	assert.True(t, resp.IsError())
	assert.Contains(t, resp.Error.Message, "unknown prompt")
}

func TestStdioServer_GetPrompt_HandlerError(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")
	s.RegisterPrompt(
		protocol.Prompt{Name: "failing"},
		func(ctx context.Context, args map[string]string) ([]protocol.PromptMessage, error) {
			return nil, errors.New("prompt error")
		},
	)

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "prompts/get",
		Params:  json.RawMessage(`{"name":"failing","arguments":{}}`),
	}
	reqData, _ := json.Marshal(req)
	stdin := bytes.NewReader(append(reqData, '\n'))
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	_ = json.Unmarshal(stdout.Bytes(), &resp)
	assert.True(t, resp.IsError())
}

func TestStdioServer_GetPrompt_InvalidParams(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")

	// Send raw JSON directly to avoid marshaling issues with invalid params
	rawRequest := `{"jsonrpc":"2.0","id":1,"method":"prompts/get","params":"invalid_params_not_object"}` + "\n"
	stdin := strings.NewReader(rawRequest)
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	var resp protocol.Response
	err := json.Unmarshal(stdout.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.IsError())
	assert.Equal(t, protocol.CodeInvalidParams, resp.Error.Code)
}

// ============================================================================
// StdioServer Error Handling Tests
// ============================================================================

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

func TestStdioServer_EmptyLine(t *testing.T) {
	s := NewStdioServer("test-server", "1.0.0")

	// Empty lines should be ignored
	stdin := strings.NewReader("\n\n\n")
	var stdout bytes.Buffer
	s.SetIO(stdin, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	// No output expected
	assert.Empty(t, stdout.String())
}

// ============================================================================
// StdioServer ServerInfo and Capabilities Tests
// ============================================================================

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
			name: "with tools only",
			setup: func(s *StdioServer) {
				s.RegisterTool(protocol.Tool{Name: "t"}, nil)
			},
			expectTools: true,
		},
		{
			name: "with resources only",
			setup: func(s *StdioServer) {
				s.RegisterResource(
					protocol.Resource{URI: "file:///x"},
					nil,
				)
			},
			expectRes: true,
		},
		{
			name: "with prompts only",
			setup: func(s *StdioServer) {
				s.RegisterPrompt(protocol.Prompt{Name: "p"}, nil)
			},
			expectPrompt: true,
		},
		{
			name: "with all capabilities",
			setup: func(s *StdioServer) {
				s.RegisterTool(protocol.Tool{Name: "t"}, nil)
				s.RegisterResource(protocol.Resource{URI: "file:///x"}, nil)
				s.RegisterPrompt(protocol.Prompt{Name: "p"}, nil)
			},
			expectTools:  true,
			expectRes:    true,
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

// ============================================================================
// HTTPServer Tests
// ============================================================================

func TestDefaultHTTPServerConfig(t *testing.T) {
	cfg := DefaultHTTPServerConfig()
	assert.Equal(t, ":8080", cfg.Address)
	assert.Equal(t, 30*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 60*time.Second, cfg.WriteTimeout)
	assert.Equal(t, int64(10*1024*1024), cfg.MaxRequestSize)
	assert.Equal(t, 30*time.Second, cfg.HeartbeatInterval)
}

func TestNewHTTPServer(t *testing.T) {
	s := NewHTTPServer("test-server", "1.0.0", DefaultHTTPServerConfig())
	assert.Equal(t, "test-server", s.info.Name)
	assert.Equal(t, "1.0.0", s.info.Version)
	assert.NotNil(t, s.tools)
	assert.NotNil(t, s.resources)
	assert.NotNil(t, s.prompts)
	assert.NotNil(t, s.clients)
	assert.NotNil(t, s.mux)
	assert.NotNil(t, s.server)
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
	assert.Equal(t, float64(1), health["tools"])
}

func TestHTTPServer_Health_WithResources(t *testing.T) {
	s := NewHTTPServer("test-server", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterResource(protocol.Resource{URI: "file:///a"}, nil)
	s.RegisterResource(protocol.Resource{URI: "file:///b"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var health map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &health)
	assert.Equal(t, float64(2), health["resources"])
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

func TestHTTPServer_Capabilities_All(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterTool(protocol.Tool{Name: "t"}, nil)
	s.RegisterResource(protocol.Resource{URI: "file:///x"}, nil)
	s.RegisterPrompt(protocol.Prompt{Name: "p"}, nil)

	caps := s.Capabilities()
	assert.NotNil(t, caps.Tools)
	assert.NotNil(t, caps.Resources)
	assert.NotNil(t, caps.Prompts)
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

// ============================================================================
// HTTPServer Registration Tests
// ============================================================================

func TestHTTPServer_RegisterTool(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterTool(
		protocol.Tool{Name: "mytool", Description: "My tool"},
		func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error) {
			return &protocol.ToolResult{}, nil
		},
	)

	assert.Len(t, s.tools, 1)
	assert.Contains(t, s.tools, "mytool")
}

func TestHTTPServer_RegisterResource(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterResource(
		protocol.Resource{URI: "file:///test", Name: "test"},
		func(ctx context.Context, uri string) (*protocol.ResourceContent, error) {
			return &protocol.ResourceContent{URI: uri}, nil
		},
	)

	assert.Len(t, s.resources, 1)
	assert.Contains(t, s.resources, "file:///test")
}

func TestHTTPServer_RegisterPrompt(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterPrompt(
		protocol.Prompt{Name: "myprompt"},
		func(ctx context.Context, args map[string]string) ([]protocol.PromptMessage, error) {
			return nil, nil
		},
	)

	assert.Len(t, s.prompts, 1)
	assert.Contains(t, s.prompts, "myprompt")
}

// ============================================================================
// HTTPServer Serve Tests
// ============================================================================

func TestHTTPServer_Serve_ContextCancel(t *testing.T) {
	cfg := DefaultHTTPServerConfig()
	cfg.Address = ":0" // Use random port
	s := NewHTTPServer("test", "1.0.0", cfg)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- s.Serve(ctx)
	}()

	// Cancel after brief delay
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		// Should complete without error or with context error
		if err != nil {
			assert.Contains(t, err.Error(), "context")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not stop after context cancel")
	}
}

// ============================================================================
// HTTPServer ListTools/Resources/Prompts Tests
// ============================================================================

func TestHTTPServer_ListTools(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterTool(protocol.Tool{Name: "tool1"}, nil)
	s.RegisterTool(protocol.Tool{Name: "tool2"}, nil)

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/list",
	}
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/message", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, httpReq)

	var resp protocol.Response
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.IsError())

	var result struct {
		Tools []protocol.Tool `json:"tools"`
	}
	_ = json.Unmarshal(resp.Result, &result)
	assert.Len(t, result.Tools, 2)
}

func TestHTTPServer_ListResources(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterResource(protocol.Resource{URI: "file:///a"}, nil)

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "resources/list",
	}
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/message", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, httpReq)

	var resp protocol.Response
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.IsError())
}

func TestHTTPServer_ListPrompts(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterPrompt(protocol.Prompt{Name: "p1"}, nil)

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "prompts/list",
	}
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/message", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, httpReq)

	var resp protocol.Response
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.IsError())
}

// ============================================================================
// HTTPServer ReadResource/GetPrompt Tests
// ============================================================================

func TestHTTPServer_ReadResource(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterResource(
		protocol.Resource{URI: "file:///test"},
		func(ctx context.Context, uri string) (*protocol.ResourceContent, error) {
			return &protocol.ResourceContent{URI: uri, Text: "content"}, nil
		},
	)

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"file:///test"}`),
	}
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/message", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, httpReq)

	var resp protocol.Response
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.IsError())
}

func TestHTTPServer_GetPrompt(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterPrompt(
		protocol.Prompt{Name: "test"},
		func(ctx context.Context, args map[string]string) ([]protocol.PromptMessage, error) {
			return []protocol.PromptMessage{
				{Role: "user", Content: protocol.NewTextContent("hi")},
			}, nil
		},
	)

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "prompts/get",
		Params:  json.RawMessage(`{"name":"test","arguments":{}}`),
	}
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/message", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, httpReq)

	var resp protocol.Response
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.IsError())
}

// ============================================================================
// HTTPServer MethodNotFound Test
// ============================================================================

func TestHTTPServer_MethodNotFound(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())

	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "unknown/method",
	}
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/message", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, httpReq)

	var resp protocol.Response
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.IsError())
	assert.Equal(t, protocol.CodeMethodNotFound, resp.Error.Code)
}

// ============================================================================
// Concurrent Tests
// ============================================================================

func TestHTTPServer_ConcurrentRequests(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterTool(
		protocol.Tool{Name: "echo"},
		func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error) {
			return &protocol.ToolResult{
				Content: []protocol.ContentBlock{protocol.NewTextContent("ok")},
			}, nil
		},
	)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req := protocol.Request{
				JSONRPC: "2.0",
				ID:      float64(id),
				Method:  "tools/call",
				Params:  json.RawMessage(`{"name":"echo","arguments":{}}`),
			}
			body, _ := json.Marshal(req)

			resp, err := http.Post(
				ts.URL+"/message",
				"application/json",
				bytes.NewReader(body),
			)
			if err != nil {
				t.Errorf("request failed: %v", err)
				return
			}
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
		}(i)
	}
	wg.Wait()
}

// ============================================================================
// Interface Compliance Tests
// ============================================================================

var _ Server = (*StdioServer)(nil)
var _ Server = (*HTTPServer)(nil)

// ============================================================================
// handleRequest Tests (via integration)
// ============================================================================

func TestHandleRequest_AllMethods(t *testing.T) {
	methods := []string{
		"initialize",
		"tools/list",
		"tools/call",
		"resources/list",
		"resources/read",
		"prompts/list",
		"prompts/get",
		"notifications/initialized",
		"unknown/method",
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
			s.RegisterTool(protocol.Tool{Name: "test"}, func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error) {
				return &protocol.ToolResult{}, nil
			})
			s.RegisterResource(protocol.Resource{URI: "file:///test"}, func(ctx context.Context, uri string) (*protocol.ResourceContent, error) {
				return &protocol.ResourceContent{URI: uri}, nil
			})
			s.RegisterPrompt(protocol.Prompt{Name: "test"}, func(ctx context.Context, args map[string]string) ([]protocol.PromptMessage, error) {
				return []protocol.PromptMessage{}, nil
			})

			var params json.RawMessage
			switch method {
			case "tools/call":
				params = json.RawMessage(`{"name":"test","arguments":{}}`)
			case "resources/read":
				params = json.RawMessage(`{"uri":"file:///test"}`)
			case "prompts/get":
				params = json.RawMessage(`{"name":"test","arguments":{}}`)
			case "initialize":
				params = json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"1"}}`)
			default:
				params = json.RawMessage(`{}`)
			}

			req := protocol.Request{
				JSONRPC: "2.0",
				ID:      float64(1),
				Method:  method,
				Params:  params,
			}
			body, _ := json.Marshal(req)

			httpReq := httptest.NewRequest(http.MethodPost, "/message", bytes.NewReader(body))
			httpReq.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			s.mux.ServeHTTP(w, httpReq)

			// Notification should return 204 No Content
			if method == "notifications/initialized" {
				assert.Equal(t, http.StatusNoContent, w.Code)
			} else {
				assert.Equal(t, http.StatusOK, w.Code)
			}
		})
	}
}

// ============================================================================
// StdioServer Context Cancellation Tests
// ============================================================================

func TestStdioServer_Serve_ContextCancel(t *testing.T) {
	s := NewStdioServer("test", "1.0.0")

	// Create a pipe that won't produce data
	pr, pw := io.Pipe()
	var stdout bytes.Buffer
	s.SetIO(pr, &stdout)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- s.Serve(ctx)
	}()

	// Cancel context
	cancel()
	_ = pw.Close()

	select {
	case err := <-done:
		assert.Equal(t, context.Canceled, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not stop after context cancel")
	}
}

// ============================================================================
// Multiple Requests Tests
// ============================================================================

func TestStdioServer_MultipleRequests(t *testing.T) {
	s := NewStdioServer("test", "1.0.0")
	s.RegisterTool(protocol.Tool{Name: "add"}, func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error) {
		a, _ := args["a"].(float64)
		b, _ := args["b"].(float64)
		return &protocol.ToolResult{
			Content: []protocol.ContentBlock{
				protocol.NewTextContent(fmt.Sprintf("%.0f", a+b)),
			},
		}, nil
	})

	// Build multiple requests
	var input bytes.Buffer
	for i := 1; i <= 3; i++ {
		req := protocol.Request{
			JSONRPC: "2.0",
			ID:      float64(i),
			Method:  "tools/call",
			Params:  json.RawMessage(fmt.Sprintf(`{"name":"add","arguments":{"a":%d,"b":%d}}`, i, i)),
		}
		data, _ := json.Marshal(req)
		input.Write(data)
		input.WriteByte('\n')
	}

	var stdout bytes.Buffer
	s.SetIO(&input, &stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.Serve(ctx)

	// Should have 3 responses
	responses := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	assert.Len(t, responses, 3)
}

// ============================================================================
// HTTPServer SSE Handler Tests
// ============================================================================

func TestHTTPServer_HandleSSE_Success(t *testing.T) {
	cfg := DefaultHTTPServerConfig()
	cfg.HeartbeatInterval = 50 * time.Millisecond
	s := NewHTTPServer("test", "1.0.0", cfg)

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	rec := httptest.NewRecorder()

	// Start the SSE handler in a goroutine and cancel after a short time
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	s.handleSSE(rec, req)

	// Check response headers
	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))

	// Check that connected event was sent
	body := rec.Body.String()
	assert.Contains(t, body, "event: connected")
	assert.Contains(t, body, "event: endpoint")
}

func TestHTTPServer_HandleSSE_WrongMethod(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())

	req := httptest.NewRequest(http.MethodPost, "/sse", nil)
	rec := httptest.NewRecorder()

	s.handleSSE(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestHTTPServer_HandleSSE_NoFlusher(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	// Create a custom ResponseWriter that doesn't support Flusher
	rec := &nonFlushableResponseWriter{ResponseWriter: httptest.NewRecorder()}

	s.handleSSE(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// nonFlushableResponseWriter is a ResponseWriter that doesn't implement Flusher
type nonFlushableResponseWriter struct {
	http.ResponseWriter
	Code int
}

func (w *nonFlushableResponseWriter) WriteHeader(code int) {
	w.Code = code
	w.ResponseWriter.WriteHeader(code)
}

func TestHTTPServer_HandleSSE_Heartbeat(t *testing.T) {
	cfg := DefaultHTTPServerConfig()
	cfg.HeartbeatInterval = 50 * time.Millisecond
	s := NewHTTPServer("test", "1.0.0", cfg)

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	rec := httptest.NewRecorder()

	// Run for enough time to get a heartbeat
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	s.handleSSE(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, ":heartbeat")
}

func TestHTTPServer_HandleMessage_WrongMethod(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/message", nil)
	rec := httptest.NewRecorder()

	s.handleMessage(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestHTTPServer_HandleMessage_EmptyBody(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())

	req := httptest.NewRequest(http.MethodPost, "/message", nil)
	rec := httptest.NewRecorder()

	s.handleMessage(rec, req)

	// Should return parse error
	var resp protocol.Response
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, protocol.CodeParseError, resp.Error.Code)
}

func TestHTTPServer_HandleMessage_InvalidJSON(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())

	req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader("not json"))
	rec := httptest.NewRecorder()

	s.handleMessage(rec, req)

	var resp protocol.Response
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, protocol.CodeParseError, resp.Error.Code)
}

func TestHTTPServer_HandleMessage_ValidRequest(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterTool(protocol.Tool{Name: "echo"}, func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error) {
		return &protocol.ToolResult{
			Content: []protocol.ContentBlock{protocol.NewTextContent("echo")},
		}, nil
	})

	reqBody := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{}}}`
	req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()

	s.handleMessage(rec, req)

	var resp protocol.Response
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Nil(t, resp.Error)
}

func TestHTTPServer_HandleHealth(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	s.handleHealth(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var health map[string]interface{}
	err := json.NewDecoder(rec.Body).Decode(&health)
	require.NoError(t, err)
	assert.Equal(t, "healthy", health["status"])
	assert.Equal(t, "test", health["server"])
	assert.Equal(t, "1.0.0", health["version"])
}

// threadSafeRecorder wraps httptest.ResponseRecorder with thread-safe access.
type threadSafeRecorder struct {
	*httptest.ResponseRecorder
	mu sync.Mutex
}

func (r *threadSafeRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ResponseRecorder.Write(p)
}

func (r *threadSafeRecorder) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	// httptest.ResponseRecorder.Flush() just sets Flushed = true
	r.ResponseRecorder.Flush()
}

func (r *threadSafeRecorder) GetBody() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ResponseRecorder.Body.String()
}

func TestHTTPServer_BroadcastToSSE(t *testing.T) {
	cfg := DefaultHTTPServerConfig()
	cfg.HeartbeatInterval = 5 * time.Second // Long interval to avoid interference
	s := NewHTTPServer("test", "1.0.0", cfg)

	// Set up an SSE client with thread-safe recorder
	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	rec := &threadSafeRecorder{ResponseRecorder: httptest.NewRecorder()}

	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)

	// Start SSE handler
	done := make(chan struct{})
	go func() {
		s.handleSSE(rec, req)
		close(done)
	}()

	// Give it time to set up
	time.Sleep(50 * time.Millisecond)

	// Broadcast a message
	resp := &protocol.Response{
		JSONRPC: "2.0",
		ID:      float64(1),
		Result:  json.RawMessage(`"test"`),
	}
	s.broadcastToSSE(resp)

	// Give it time to send
	time.Sleep(50 * time.Millisecond)

	// Cancel and wait for handler to finish
	cancel()
	<-done

	body := rec.GetBody()
	assert.Contains(t, body, "event: message")
}

func TestHTTPServer_WriteError(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())

	rec := httptest.NewRecorder()

	s.writeError(rec, float64(1), protocol.CodeInternalError, "test error", map[string]string{"detail": "info"})

	var resp protocol.Response
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, protocol.CodeInternalError, resp.Error.Code)
	assert.Equal(t, "test error", resp.Error.Message)
}

func TestHTTPServer_ServerInfo_Extended(t *testing.T) {
	s := NewHTTPServer("my-server", "2.0.0", DefaultHTTPServerConfig())

	info := s.ServerInfo()
	assert.Equal(t, "my-server", info.Name)
	assert.Equal(t, "2.0.0", info.Version)
}

func TestHTTPServer_Capabilities_Extended(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())

	// Empty server has nil capabilities
	caps := s.Capabilities()
	assert.Nil(t, caps.Tools)
	assert.Nil(t, caps.Resources)
	assert.Nil(t, caps.Prompts)

	// Add items and check capabilities become non-nil
	s.RegisterTool(protocol.Tool{Name: "tool"}, nil)
	s.RegisterResource(protocol.Resource{URI: "file:///test", Name: "test"}, func(ctx context.Context, uri string) (*protocol.ResourceContent, error) {
		return &protocol.ResourceContent{Text: "content"}, nil
	})
	s.RegisterPrompt(protocol.Prompt{Name: "prompt"}, func(ctx context.Context, args map[string]string) ([]protocol.PromptMessage, error) {
		return nil, nil
	})

	caps = s.Capabilities()
	assert.NotNil(t, caps.Tools)
	assert.NotNil(t, caps.Resources)
	assert.NotNil(t, caps.Prompts)
}

func TestHTTPServer_Serve_InvalidAddress(t *testing.T) {
	cfg := DefaultHTTPServerConfig()
	cfg.Address = "invalid:address:too:many:colons"
	s := NewHTTPServer("test", "1.0.0", cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := s.Serve(ctx)
	assert.Error(t, err)
}

func TestHTTPServer_Handler_Extended(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterTool(protocol.Tool{Name: "tool1"}, nil)

	handler := s.Handler()
	assert.NotNil(t, handler)

	// Test that handler works for health endpoint
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// ============================================================================
// handleListTools / handleListResources / handleListPrompts Cursor Tests
// ============================================================================

func TestHTTPServer_ListTools_WithCursor(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterTool(protocol.Tool{Name: "tool1"}, nil)

	// Request with cursor parameter
	reqBody := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"cursor":"next-page"}}`
	req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()

	s.handleMessage(rec, req)

	var resp protocol.Response
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Nil(t, resp.Error)
}

func TestHTTPServer_ListResources_WithCursor(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterResource(protocol.Resource{URI: "file:///test", Name: "test"}, func(ctx context.Context, uri string) (*protocol.ResourceContent, error) {
		return &protocol.ResourceContent{Text: "content"}, nil
	})

	reqBody := `{"jsonrpc":"2.0","id":1,"method":"resources/list","params":{"cursor":"page2"}}`
	req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()

	s.handleMessage(rec, req)

	var resp protocol.Response
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Nil(t, resp.Error)
}

func TestHTTPServer_ListPrompts_WithCursor(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterPrompt(protocol.Prompt{Name: "prompt1"}, func(ctx context.Context, args map[string]string) ([]protocol.PromptMessage, error) {
		return []protocol.PromptMessage{{Role: "user", Content: protocol.NewTextContent("msg")}}, nil
	})

	reqBody := `{"jsonrpc":"2.0","id":1,"method":"prompts/list","params":{"cursor":"page3"}}`
	req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()

	s.handleMessage(rec, req)

	var resp protocol.Response
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Nil(t, resp.Error)
}

// ============================================================================
// WriteResponse Error Path
// ============================================================================

func TestStdioServer_WriteResponse_EncoderError(t *testing.T) {
	s := NewStdioServer("test", "1.0.0")

	// Use a writer that will fail
	failWriter := &failingWriter{}
	s.SetIO(strings.NewReader(""), failWriter)

	// This won't directly test writeResponse failure, but exercises the path
	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
	}
	data, _ := json.Marshal(req)
	s.SetIO(bytes.NewReader(append(data, '\n')), failWriter)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = s.Serve(ctx)
}

type failingWriter struct{}

func (w *failingWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("write failed")
}

// ============================================================================
// Additional Coverage Tests
// ============================================================================

func TestHTTPServer_Initialize_Success(t *testing.T) {
	s := NewHTTPServer("test", "1.0.0", DefaultHTTPServerConfig())
	s.RegisterTool(protocol.Tool{Name: "tool1"}, nil)

	reqBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}`
	req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()

	s.handleMessage(rec, req)

	var resp protocol.Response
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}
