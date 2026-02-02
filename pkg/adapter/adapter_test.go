package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.mcp/pkg/config"
	"digital.vasic.mcp/pkg/registry"
)

func TestBaseAdapter_Name(t *testing.T) {
	b := &BaseAdapter{AdapterName: "test-adapter"}
	assert.Equal(t, "test-adapter", b.Name())
}

func TestBaseAdapter_Config(t *testing.T) {
	b := &BaseAdapter{
		AdapterName: "test",
		ServerCfg: config.ServerConfig{
			Command:   "npx",
			Args:      []string{"-y", "server"},
			Transport: config.TransportStdio,
			Enabled:   true,
		},
	}

	cfg := b.Config()
	assert.Equal(t, "test", cfg["name"])
	assert.Equal(t, "npx", cfg["command"])
	assert.Equal(t, "stdio", cfg["transport"])
	assert.Equal(t, true, cfg["enabled"])
}

func TestBaseAdapter_State(t *testing.T) {
	b := &BaseAdapter{state: StateIdle}
	assert.Equal(t, StateIdle, b.State())

	b.SetState(StateRunning)
	assert.Equal(t, StateRunning, b.State())

	b.SetState(StateStopped)
	assert.Equal(t, StateStopped, b.State())
}

func TestNewStdioAdapter(t *testing.T) {
	a := NewStdioAdapter("test", config.ServerConfig{
		Command: "echo",
		Args:    []string{"hello"},
	})

	assert.Equal(t, "test", a.Name())
	assert.Equal(t, StateIdle, a.State())
	assert.Equal(t, config.TransportStdio, a.ServerCfg.Transport)
}

func TestStdioAdapter_StartStop(t *testing.T) {
	a := NewStdioAdapter("test-echo", config.ServerConfig{
		Command: "sleep",
		Args:    []string{"10"},
	})

	ctx := context.Background()

	err := a.Start(ctx)
	require.NoError(t, err)
	assert.Equal(t, StateRunning, a.State())

	err = a.Stop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, StateStopped, a.State())
}

func TestStdioAdapter_Start_InvalidCommand(t *testing.T) {
	a := NewStdioAdapter("bad", config.ServerConfig{
		Command: "/nonexistent/binary",
	})

	err := a.Start(context.Background())
	assert.Error(t, err)
	assert.Equal(t, StateError, a.State())
}

func TestStdioAdapter_Stop_NotStarted(t *testing.T) {
	a := NewStdioAdapter("test", config.ServerConfig{
		Command: "echo",
	})

	// Stopping without starting should not error
	err := a.Stop(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, StateStopped, a.State())
}

func TestNewDockerAdapter(t *testing.T) {
	a := NewDockerAdapter(
		"test-docker",
		config.ServerConfig{Name: "test"},
		config.ContainerConfig{
			Image:    "mcp-server",
			Tag:      "latest",
			Port:     8080,
			HostPort: 9090,
		},
	)

	assert.Equal(t, "test-docker", a.Name())
	assert.Equal(t, StateIdle, a.State())
	assert.Equal(t, "mcp-server:latest", a.Container.ImageRef())
}

func TestDockerAdapter_Config(t *testing.T) {
	a := NewDockerAdapter(
		"test",
		config.ServerConfig{
			Command:   "docker",
			Transport: config.TransportHTTP,
			Enabled:   true,
		},
		config.ContainerConfig{Image: "test"},
	)

	cfg := a.Config()
	assert.Equal(t, "test", cfg["name"])
}

func TestNewHTTPAdapter(t *testing.T) {
	a := NewHTTPAdapter("test-http", config.ServerConfig{
		URL: "http://localhost:8080",
	})

	assert.Equal(t, "test-http", a.Name())
	assert.Equal(t, StateIdle, a.State())
	assert.Equal(t, config.TransportHTTP, a.ServerCfg.Transport)
}

func TestHTTPAdapter_StartStop(t *testing.T) {
	a := NewHTTPAdapter("test", config.ServerConfig{
		URL: "http://localhost:8080",
	})

	err := a.Start(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, StateRunning, a.State())

	err = a.Stop(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, StateStopped, a.State())
}

func TestHTTPAdapter_HealthCheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "healthy",
			})
		},
	))
	defer server.Close()

	a := NewHTTPAdapter("test", config.ServerConfig{
		URL: server.URL,
	})

	err := a.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestHTTPAdapter_HealthCheck_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	))
	defer server.Close()

	a := NewHTTPAdapter("test", config.ServerConfig{
		URL: server.URL,
	})

	err := a.HealthCheck(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestHTTPAdapter_HealthCheck_NoURL(t *testing.T) {
	a := NewHTTPAdapter("test", config.ServerConfig{})

	err := a.HealthCheck(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no URL configured")
}

func TestHTTPAdapter_HealthCheck_ConnectionRefused(t *testing.T) {
	a := NewHTTPAdapter("test", config.ServerConfig{
		URL: "http://localhost:1",
	})

	err := a.HealthCheck(context.Background())
	assert.Error(t, err)
}

// Verify all adapters implement the registry.Adapter interface.
var _ registry.Adapter = (*StdioAdapter)(nil)
var _ registry.Adapter = (*DockerAdapter)(nil)
var _ registry.Adapter = (*HTTPAdapter)(nil)
