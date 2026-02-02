package protocol

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequest(t *testing.T) {
	tests := []struct {
		name       string
		id         interface{}
		method     string
		params     interface{}
		wantErr    bool
		wantNotif  bool
	}{
		{
			name:      "basic request with int ID",
			id:        1,
			method:    "tools/list",
			params:    nil,
			wantNotif: false,
		},
		{
			name:      "request with string ID",
			id:        "req-1",
			method:    "tools/call",
			params:    map[string]string{"name": "test"},
			wantNotif: false,
		},
		{
			name:      "notification without ID",
			id:        nil,
			method:    "notifications/initialized",
			params:    map[string]interface{}{},
			wantNotif: true,
		},
		{
			name:    "request with unmarshalable params",
			id:      1,
			method:  "test",
			params:  make(chan int),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewRequest(tt.id, tt.method, tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, JSONRPCVersion, req.JSONRPC)
			assert.Equal(t, tt.id, req.ID)
			assert.Equal(t, tt.method, req.Method)
			assert.Equal(t, tt.wantNotif, req.IsNotification())
		})
	}
}

func TestNewNotification(t *testing.T) {
	notif, err := NewNotification("notifications/initialized", nil)
	require.NoError(t, err)
	assert.Nil(t, notif.ID)
	assert.True(t, notif.IsNotification())
	assert.Equal(t, "notifications/initialized", notif.Method)
}

func TestNewResponse(t *testing.T) {
	tests := []struct {
		name    string
		id      interface{}
		result  interface{}
		wantErr bool
	}{
		{
			name:   "simple result",
			id:     1,
			result: map[string]string{"status": "ok"},
		},
		{
			name:   "nil result",
			id:     1,
			result: nil,
		},
		{
			name:    "unmarshalable result",
			id:      1,
			result:  make(chan int),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := NewResponse(tt.id, tt.result)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, JSONRPCVersion, resp.JSONRPC)
			assert.Equal(t, tt.id, resp.ID)
			assert.False(t, resp.IsError())
		})
	}
}

func TestNewErrorResponse(t *testing.T) {
	tests := []struct {
		name    string
		id      interface{}
		code    int
		message string
		data    interface{}
	}{
		{
			name:    "parse error",
			id:      nil,
			code:    CodeParseError,
			message: "Parse error",
			data:    nil,
		},
		{
			name:    "method not found with data",
			id:      1,
			code:    CodeMethodNotFound,
			message: "Method not found",
			data:    "unknown_method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewErrorResponse(tt.id, tt.code, tt.message, tt.data)
			assert.True(t, resp.IsError())
			assert.Equal(t, tt.code, resp.Error.Code)
			assert.Equal(t, tt.message, resp.Error.Message)
			assert.Equal(t, tt.data, resp.Error.Data)
		})
	}
}

func TestRPCError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *RPCError
		contains string
	}{
		{
			name:     "error without data",
			err:      &RPCError{Code: CodeInternalError, Message: "internal"},
			contains: "code=-32603",
		},
		{
			name:     "error with data",
			err:      &RPCError{Code: CodeParseError, Message: "bad json", Data: "details"},
			contains: "data=details",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, tt.err.Error(), tt.contains)
		})
	}
}

func TestRequest_JSONMarshalRoundtrip(t *testing.T) {
	tests := []struct {
		name   string
		req    *Request
	}{
		{
			name: "request with params",
			req: &Request{
				JSONRPC: JSONRPCVersion,
				ID:      float64(1),
				Method:  "tools/call",
				Params:  json.RawMessage(`{"name":"test"}`),
			},
		},
		{
			name: "notification",
			req: &Request{
				JSONRPC: JSONRPCVersion,
				Method:  "notifications/initialized",
				Params:  json.RawMessage(`{}`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.req)
			require.NoError(t, err)

			var decoded Request
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.req.JSONRPC, decoded.JSONRPC)
			assert.Equal(t, tt.req.Method, decoded.Method)
		})
	}
}

func TestResponse_JSONMarshalRoundtrip(t *testing.T) {
	tests := []struct {
		name string
		resp *Response
	}{
		{
			name: "success response",
			resp: &Response{
				JSONRPC: JSONRPCVersion,
				ID:      float64(1),
				Result:  json.RawMessage(`{"tools":[]}`),
			},
		},
		{
			name: "error response",
			resp: &Response{
				JSONRPC: JSONRPCVersion,
				ID:      float64(1),
				Error: &RPCError{
					Code:    CodeMethodNotFound,
					Message: "not found",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.resp)
			require.NoError(t, err)

			var decoded Response
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.resp.JSONRPC, decoded.JSONRPC)
			if tt.resp.Error != nil {
				assert.Equal(t, tt.resp.Error.Code, decoded.Error.Code)
			}
		})
	}
}

func TestTool_JSON(t *testing.T) {
	tool := Tool{
		Name:        "read_file",
		Description: "Read a file from disk",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path",
				},
			},
			"required": []string{"path"},
		},
	}

	data, err := json.Marshal(tool)
	require.NoError(t, err)

	var decoded Tool
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "read_file", decoded.Name)
	assert.Equal(t, "Read a file from disk", decoded.Description)
	assert.NotNil(t, decoded.InputSchema)
}

func TestResource_JSON(t *testing.T) {
	resource := Resource{
		URI:         "file:///tmp/test.txt",
		Name:        "test.txt",
		Description: "A test file",
		MimeType:    "text/plain",
	}

	data, err := json.Marshal(resource)
	require.NoError(t, err)

	var decoded Resource
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "file:///tmp/test.txt", decoded.URI)
	assert.Equal(t, "text/plain", decoded.MimeType)
}

func TestPrompt_JSON(t *testing.T) {
	prompt := Prompt{
		Name:        "code_review",
		Description: "Review code changes",
		Arguments: []PromptArgument{
			{Name: "code", Description: "The code to review", Required: true},
			{Name: "language", Description: "Programming language"},
		},
	}

	data, err := json.Marshal(prompt)
	require.NoError(t, err)

	var decoded Prompt
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "code_review", decoded.Name)
	assert.Len(t, decoded.Arguments, 2)
	assert.True(t, decoded.Arguments[0].Required)
}

func TestServerCapabilities_JSON(t *testing.T) {
	caps := ServerCapabilities{
		Tools:     &ToolsCapability{ListChanged: true},
		Resources: &ResourcesCapability{Subscribe: true, ListChanged: true},
		Prompts:   &PromptsCapability{ListChanged: false},
		Logging:   &LoggingCapability{},
	}

	data, err := json.Marshal(caps)
	require.NoError(t, err)

	var decoded ServerCapabilities
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.NotNil(t, decoded.Tools)
	assert.True(t, decoded.Tools.ListChanged)
	assert.True(t, decoded.Resources.Subscribe)
}

func TestNormalizeID(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "float64 whole number",
			input:    float64(42),
			expected: int64(42),
		},
		{
			name:     "float64 fractional",
			input:    float64(42.5),
			expected: float64(42.5),
		},
		{
			name:     "int64",
			input:    int64(42),
			expected: int64(42),
		},
		{
			name:     "int",
			input:    int(42),
			expected: int64(42),
		},
		{
			name:     "string",
			input:    "req-1",
			expected: "req-1",
		},
		{
			name:     "nil",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeID(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewTextContent(t *testing.T) {
	c := NewTextContent("hello world")
	assert.Equal(t, "text", c.Type)
	assert.Equal(t, "hello world", c.Text)
}

func TestNewBinaryContent(t *testing.T) {
	c := NewBinaryContent("image/png", "base64data")
	assert.Equal(t, "blob", c.Type)
	assert.Equal(t, "image/png", c.MimeType)
	assert.Equal(t, "base64data", c.Data)
}

func TestToolResult_JSON(t *testing.T) {
	result := ToolResult{
		Content: []ContentBlock{
			NewTextContent("file contents here"),
		},
		IsError: false,
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded ToolResult
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Content, 1)
	assert.Equal(t, "text", decoded.Content[0].Type)
	assert.False(t, decoded.IsError)
}

func TestInitializeParams_JSON(t *testing.T) {
	params := InitializeParams{
		ProtocolVersion: MCPProtocolVersion,
		Capabilities:    map[string]interface{}{},
		ClientInfo: ClientInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}

	data, err := json.Marshal(params)
	require.NoError(t, err)

	var decoded InitializeParams
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, MCPProtocolVersion, decoded.ProtocolVersion)
	assert.Equal(t, "test-client", decoded.ClientInfo.Name)
}

func TestInitializeResult_JSON(t *testing.T) {
	result := InitializeResult{
		ProtocolVersion: MCPProtocolVersion,
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{ListChanged: true},
		},
		ServerInfo: ServerInfo{
			Name:    "test-server",
			Version: "1.0.0",
		},
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded InitializeResult
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "test-server", decoded.ServerInfo.Name)
	assert.NotNil(t, decoded.Capabilities.Tools)
}
