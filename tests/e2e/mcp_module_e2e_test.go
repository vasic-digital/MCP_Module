package e2e

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"digital.vasic.mcp/pkg/adapter"
	"digital.vasic.mcp/pkg/config"
	"digital.vasic.mcp/pkg/protocol"
	"digital.vasic.mcp/pkg/registry"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullMCPProtocolFlowE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	// Step 1: Create initialize request
	initParams := protocol.InitializeParams{
		ProtocolVersion: protocol.MCPProtocolVersion,
		ClientInfo: protocol.ClientInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}
	initReq, err := protocol.NewRequest(1, "initialize", initParams)
	require.NoError(t, err)
	assert.Equal(t, "initialize", initReq.Method)

	// Step 2: Create initialize response
	initResult := protocol.InitializeResult{
		ProtocolVersion: protocol.MCPProtocolVersion,
		Capabilities: protocol.ServerCapabilities{
			Tools:     &protocol.ToolsCapability{ListChanged: true},
			Resources: &protocol.ResourcesCapability{Subscribe: true},
			Prompts:   &protocol.PromptsCapability{ListChanged: true},
		},
		ServerInfo: protocol.ServerInfo{
			Name:    "test-server",
			Version: "1.0.0",
		},
	}
	initResp, err := protocol.NewResponse(1, initResult)
	require.NoError(t, err)
	assert.False(t, initResp.IsError())

	// Step 3: Send initialized notification
	notif, err := protocol.NewNotification("notifications/initialized", nil)
	require.NoError(t, err)
	assert.True(t, notif.IsNotification())

	// Step 4: List tools
	toolsReq, err := protocol.NewRequest(2, "tools/list", nil)
	require.NoError(t, err)
	assert.Equal(t, "tools/list", toolsReq.Method)

	tools := []protocol.Tool{
		{
			Name:        "read_file",
			Description: "Read a file from disk",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string"},
				},
			},
		},
	}
	toolsResp, err := protocol.NewResponse(2, map[string]interface{}{
		"tools": tools,
	})
	require.NoError(t, err)
	assert.False(t, toolsResp.IsError())
}

func TestRegistryFullLifecycleE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	reg := registry.New()

	adapters := []struct {
		name string
		url  string
	}{
		{"filesystem", "http://localhost:9101"},
		{"memory", "http://localhost:9102"},
		{"sequential-thinking", "http://localhost:9103"},
	}

	for _, a := range adapters {
		httpAdapter := adapter.NewHTTPAdapter(a.name, config.ServerConfig{
			Name:      a.name,
			Transport: config.TransportHTTP,
			URL:       a.url,
			Enabled:   true,
		})
		require.NoError(t, reg.Register(httpAdapter))
	}

	assert.Equal(t, 3, reg.Count())

	ctx := context.Background()
	require.NoError(t, reg.StartAll(ctx))

	for _, a := range adapters {
		retrieved, found := reg.Get(a.name)
		assert.True(t, found, "adapter %s should be found", a.name)
		assert.Equal(t, a.name, retrieved.Name())
	}

	require.NoError(t, reg.StopAll(ctx))
}

func TestConfigFileLoadE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "mcp-config.json")

	fileConfig := config.FileConfig{
		Servers: []config.ServerConfig{
			{
				Name:        "test-stdio",
				Command:     "/usr/bin/echo",
				Args:        []string{"hello"},
				Transport:   config.TransportStdio,
				Enabled:     true,
				Description: "Test stdio server",
			},
			{
				Name:        "test-http",
				Transport:   config.TransportHTTP,
				URL:         "http://localhost:9200",
				Enabled:     true,
				Description: "Test HTTP server",
			},
		},
		Containers: []config.ContainerConfig{
			{
				Image:       "mcp-filesystem",
				Tag:         "latest",
				Port:        9101,
				HostPort:    9101,
				HealthCheck: "/health",
			},
		},
	}

	data, err := json.MarshalIndent(fileConfig, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cfgPath, data, 0644))

	loaded, err := config.LoadFromFile(cfgPath)
	require.NoError(t, err)

	require.NoError(t, loaded.Validate())
	assert.Len(t, loaded.Servers, 2)
	assert.Len(t, loaded.Containers, 1)
	assert.Equal(t, "test-stdio", loaded.Servers[0].Name)
	assert.Equal(t, "mcp-filesystem:latest", loaded.Containers[0].ImageRef())
}

func TestToolResultContentBlocksE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	textBlock := protocol.NewTextContent("Hello, World!")
	assert.Equal(t, "text", textBlock.Type)
	assert.Equal(t, "Hello, World!", textBlock.Text)

	binaryBlock := protocol.NewBinaryContent("image/png", "base64data==")
	assert.Equal(t, "blob", binaryBlock.Type)
	assert.Equal(t, "image/png", binaryBlock.MimeType)
	assert.Equal(t, "base64data==", binaryBlock.Data)

	result := protocol.ToolResult{
		Content: []protocol.ContentBlock{textBlock, binaryBlock},
		IsError: false,
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded protocol.ToolResult
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Len(t, decoded.Content, 2)
	assert.False(t, decoded.IsError)
}

func TestNormalizeIDVariantsE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	assert.Equal(t, int64(42), protocol.NormalizeID(float64(42)))
	assert.Equal(t, int64(1), protocol.NormalizeID(int64(1)))
	assert.Equal(t, int64(7), protocol.NormalizeID(int(7)))
	assert.Equal(t, "abc", protocol.NormalizeID("abc"))
	assert.Equal(t, float64(3.14), protocol.NormalizeID(float64(3.14)))
}

func TestAdapterStateTransitionsE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	a := adapter.NewHTTPAdapter("state-test", config.ServerConfig{
		Name:      "state-test",
		Transport: config.TransportHTTP,
		URL:       "http://localhost:9999",
		Enabled:   true,
	})

	assert.Equal(t, adapter.StateIdle, a.State())

	ctx := context.Background()
	require.NoError(t, a.Start(ctx))
	assert.Equal(t, adapter.StateRunning, a.State())

	require.NoError(t, a.Stop(ctx))
	assert.Equal(t, adapter.StateStopped, a.State())
}
