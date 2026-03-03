package integration

import (
	"context"
	"encoding/json"
	"testing"

	"digital.vasic.mcp/pkg/adapter"
	"digital.vasic.mcp/pkg/config"
	"digital.vasic.mcp/pkg/protocol"
	"digital.vasic.mcp/pkg/registry"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtocolRequestResponseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	req, err := protocol.NewRequest(1, "tools/list", nil)
	require.NoError(t, err)
	assert.Equal(t, protocol.JSONRPCVersion, req.JSONRPC)
	assert.Equal(t, "tools/list", req.Method)
	assert.False(t, req.IsNotification())

	tools := []protocol.Tool{
		{Name: "read_file", Description: "Reads a file"},
		{Name: "write_file", Description: "Writes a file"},
	}
	resp, err := protocol.NewResponse(1, tools)
	require.NoError(t, err)
	assert.False(t, resp.IsError())

	var decoded []protocol.Tool
	err = json.Unmarshal(resp.Result, &decoded)
	require.NoError(t, err)
	assert.Len(t, decoded, 2)
	assert.Equal(t, "read_file", decoded[0].Name)
}

func TestRegistryAdapterLifecycleIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	reg := registry.New()

	httpAdapter := adapter.NewHTTPAdapter("test-http", config.ServerConfig{
		Name:      "test-http",
		Transport: config.TransportHTTP,
		URL:       "http://localhost:9999",
		Enabled:   true,
	})

	require.NoError(t, reg.Register(httpAdapter))
	assert.Equal(t, 1, reg.Count())

	retrieved, found := reg.Get("test-http")
	assert.True(t, found)
	assert.Equal(t, "test-http", retrieved.Name())

	ctx := context.Background()
	require.NoError(t, httpAdapter.Start(ctx))
	assert.Equal(t, adapter.StateRunning, httpAdapter.State())

	require.NoError(t, httpAdapter.Stop(ctx))
	assert.Equal(t, adapter.StateStopped, httpAdapter.State())
}

func TestRegistryMultipleAdaptersIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	reg := registry.New()

	for i := 0; i < 5; i++ {
		name := "adapter-" + string(rune('a'+i))
		a := adapter.NewHTTPAdapter(name, config.ServerConfig{
			Name:      name,
			Transport: config.TransportHTTP,
			URL:       "http://localhost:9100",
			Enabled:   true,
		})
		require.NoError(t, reg.Register(a))
	}

	assert.Equal(t, 5, reg.Count())
	names := reg.List()
	assert.Len(t, names, 5)

	require.NoError(t, reg.Unregister("adapter-a"))
	assert.Equal(t, 4, reg.Count())

	_, found := reg.Get("adapter-a")
	assert.False(t, found)
}

func TestServerConfigValidationIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	stdioConfig := config.ServerConfig{
		Name:      "stdio-server",
		Transport: config.TransportStdio,
		Command:   "/usr/bin/my-mcp",
		Args:      []string{"--mode", "stdio"},
		Enabled:   true,
	}
	require.NoError(t, stdioConfig.Validate())

	httpConfig := config.ServerConfig{
		Name:      "http-server",
		Transport: config.TransportHTTP,
		URL:       "http://localhost:9101",
		Enabled:   true,
	}
	require.NoError(t, httpConfig.Validate())

	containerCfg := config.ContainerConfig{
		Image:    "mcp-server",
		Tag:      "latest",
		Port:     9101,
		HostPort: 9101,
	}
	require.NoError(t, containerCfg.Validate())
	assert.Equal(t, "mcp-server:latest", containerCfg.ImageRef())
}

func TestProtocolNotificationIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	notif, err := protocol.NewNotification("notifications/initialized", nil)
	require.NoError(t, err)
	assert.True(t, notif.IsNotification())
	assert.Equal(t, "notifications/initialized", notif.Method)
	assert.Nil(t, notif.ID)
}

func TestProtocolErrorResponseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	errResp := protocol.NewErrorResponse(
		1, protocol.CodeMethodNotFound,
		"method not found", nil,
	)
	assert.True(t, errResp.IsError())
	assert.Equal(t, protocol.CodeMethodNotFound, errResp.Error.Code)
	assert.Equal(t, "method not found", errResp.Error.Message)

	errStr := errResp.Error.Error()
	assert.Contains(t, errStr, "rpc error")
	assert.Contains(t, errStr, "-32601")
}

func TestAdapterConfigMapIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	a := adapter.NewHTTPAdapter("my-adapter", config.ServerConfig{
		Name:      "my-adapter",
		Transport: config.TransportHTTP,
		URL:       "http://localhost:9200",
		Enabled:   true,
	})

	cfgMap := a.Config()
	assert.Equal(t, "my-adapter", cfgMap["name"])
	assert.Equal(t, "http", cfgMap["transport"])
	assert.Equal(t, true, cfgMap["enabled"])
}
