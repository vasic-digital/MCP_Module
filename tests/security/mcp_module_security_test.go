package security

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"digital.vasic.mcp/pkg/adapter"
	"digital.vasic.mcp/pkg/config"
	"digital.vasic.mcp/pkg/protocol"
	"digital.vasic.mcp/pkg/registry"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNilAdapterRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	reg := registry.New()
	err := reg.Register(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestEmptyAdapterNameRejection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	reg := registry.New()
	a := adapter.NewHTTPAdapter("", config.ServerConfig{
		Name:      "",
		Transport: config.TransportHTTP,
		URL:       "http://localhost:9000",
	})
	err := reg.Register(a)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestDuplicateAdapterRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	reg := registry.New()
	a := adapter.NewHTTPAdapter("duplicate", config.ServerConfig{
		Name:      "duplicate",
		Transport: config.TransportHTTP,
		URL:       "http://localhost:9000",
	})
	require.NoError(t, reg.Register(a))
	err := reg.Register(a)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestUnregisterNonExistent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	reg := registry.New()
	err := reg.Unregister("ghost-adapter")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestServerConfigValidationAttacks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	tests := []struct {
		name      string
		cfg       config.ServerConfig
		expectErr string
	}{
		{
			name:      "empty name",
			cfg:       config.ServerConfig{Transport: config.TransportStdio, Command: "cmd"},
			expectErr: "name is required",
		},
		{
			name:      "empty transport",
			cfg:       config.ServerConfig{Name: "test"},
			expectErr: "transport type is required",
		},
		{
			name:      "stdio without command",
			cfg:       config.ServerConfig{Name: "test", Transport: config.TransportStdio},
			expectErr: "command is required",
		},
		{
			name:      "http without url",
			cfg:       config.ServerConfig{Name: "test", Transport: config.TransportHTTP},
			expectErr: "URL is required",
		},
		{
			name:      "unknown transport",
			cfg:       config.ServerConfig{Name: "test", Transport: "websocket"},
			expectErr: "unknown transport",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectErr)
		})
	}
}

func TestContainerConfigValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	tests := []struct {
		name      string
		cfg       config.ContainerConfig
		expectErr string
	}{
		{
			name:      "missing image",
			cfg:       config.ContainerConfig{Port: 8080},
			expectErr: "image is required",
		},
		{
			name:      "negative port",
			cfg:       config.ContainerConfig{Image: "test", Port: -1},
			expectErr: "invalid container port",
		},
		{
			name:      "port too high",
			cfg:       config.ContainerConfig{Image: "test", Port: 70000},
			expectErr: "invalid container port",
		},
		{
			name:      "negative host port",
			cfg:       config.ContainerConfig{Image: "test", HostPort: -5},
			expectErr: "invalid host port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectErr)
		})
	}
}

func TestMalformedJSONRPCHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	malformedPayloads := []string{
		`{"jsonrpc": "2.0", "id": 1}`,
		`{"jsonrpc": "1.0", "method": "test"}`,
		`{}`,
		`{"method": "` + strings.Repeat("x", 10000) + `"}`,
	}

	for i, payload := range malformedPayloads {
		var req protocol.Request
		err := json.Unmarshal([]byte(payload), &req)
		// Should not panic, regardless of outcome
		_ = err
		_ = i
	}
}

func TestRPCErrorWithData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	errResp := protocol.NewErrorResponse(
		1, protocol.CodeInternalError, "internal",
		map[string]string{"detail": "stack trace"},
	)
	assert.True(t, errResp.IsError())
	assert.Contains(t, errResp.Error.Error(), "internal")
	assert.Contains(t, errResp.Error.Error(), "data=")

	errNoData := protocol.NewErrorResponse(
		2, protocol.CodeParseError, "parse error", nil,
	)
	assert.NotContains(t, errNoData.Error.Error(), "data=")
}

func TestConfigLoadUnsupportedFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	_, err := config.LoadFromFile("/nonexistent/file.json")
	assert.Error(t, err)

	_, err = config.LoadFromFile("/dev/null.xml")
	assert.Error(t, err)
}

func TestHealthCheckNotStartedAdapter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	a := adapter.NewHTTPAdapter("not-started", config.ServerConfig{
		Name:      "not-started",
		Transport: config.TransportHTTP,
		URL:       "", // deliberately empty
		Enabled:   true,
	})

	ctx := context.Background()
	err := a.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no URL configured")
}
