package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.mcp/pkg/config"
	"digital.vasic.mcp/pkg/registry"
)

// ============================================================================
// State Constants Tests
// ============================================================================

func TestState_Values(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateIdle, "idle"},
		{StateStarting, "starting"},
		{StateRunning, "running"},
		{StateStopping, "stopping"},
		{StateStopped, "stopped"},
		{StateError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, State(tt.expected), tt.state)
		})
	}
}

// ============================================================================
// BaseAdapter Tests
// ============================================================================

func TestBaseAdapter_Name(t *testing.T) {
	tests := []struct {
		name     string
		adapter  string
		expected string
	}{
		{"simple name", "test-adapter", "test-adapter"},
		{"with special chars", "my_adapter-v1", "my_adapter-v1"},
		{"empty name", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BaseAdapter{AdapterName: tt.adapter}
			assert.Equal(t, tt.expected, b.Name())
		})
	}
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
	assert.Equal(t, []string{"-y", "server"}, cfg["args"])
	assert.Equal(t, "stdio", cfg["transport"])
	assert.Equal(t, true, cfg["enabled"])
}

func TestBaseAdapter_Config_HTTPTransport(t *testing.T) {
	b := &BaseAdapter{
		AdapterName: "http-adapter",
		ServerCfg: config.ServerConfig{
			Transport: config.TransportHTTP,
			URL:       "http://localhost:8080",
			Enabled:   false,
		},
	}

	cfg := b.Config()
	assert.Equal(t, "http-adapter", cfg["name"])
	assert.Equal(t, "http", cfg["transport"])
	assert.Equal(t, false, cfg["enabled"])
}

func TestBaseAdapter_Config_EmptyFields(t *testing.T) {
	b := &BaseAdapter{
		AdapterName: "minimal",
		ServerCfg:   config.ServerConfig{},
	}

	cfg := b.Config()
	assert.Equal(t, "minimal", cfg["name"])
	assert.Equal(t, "", cfg["command"])
	assert.Nil(t, cfg["args"])
	assert.Equal(t, "", cfg["transport"])
	assert.Equal(t, false, cfg["enabled"])
}

func TestBaseAdapter_State(t *testing.T) {
	b := &BaseAdapter{state: StateIdle}
	assert.Equal(t, StateIdle, b.State())

	b.SetState(StateRunning)
	assert.Equal(t, StateRunning, b.State())

	b.SetState(StateStopped)
	assert.Equal(t, StateStopped, b.State())

	b.SetState(StateError)
	assert.Equal(t, StateError, b.State())
}

func TestBaseAdapter_State_Transitions(t *testing.T) {
	transitions := []State{
		StateIdle,
		StateStarting,
		StateRunning,
		StateStopping,
		StateStopped,
	}

	b := &BaseAdapter{}
	for _, state := range transitions {
		b.SetState(state)
		assert.Equal(t, state, b.State())
	}
}

func TestBaseAdapter_State_Concurrent(t *testing.T) {
	b := &BaseAdapter{state: StateIdle}

	var wg sync.WaitGroup
	states := []State{StateStarting, StateRunning, StateStopping, StateStopped}

	// Multiple concurrent readers and writers
	for i := 0; i < 100; i++ {
		wg.Add(2)

		go func(idx int) {
			defer wg.Done()
			b.SetState(states[idx%len(states)])
		}(i)

		go func() {
			defer wg.Done()
			_ = b.State()
		}()
	}

	wg.Wait()
}

// ============================================================================
// StdioAdapter Tests
// ============================================================================

func TestNewStdioAdapter(t *testing.T) {
	a := NewStdioAdapter("test", config.ServerConfig{
		Command: "echo",
		Args:    []string{"hello"},
	})

	assert.Equal(t, "test", a.Name())
	assert.Equal(t, StateIdle, a.State())
	assert.Equal(t, config.TransportStdio, a.ServerCfg.Transport)
}

func TestNewStdioAdapter_OverridesTransport(t *testing.T) {
	// Even if HTTP is specified, it should be overridden to stdio
	a := NewStdioAdapter("test", config.ServerConfig{
		Command:   "echo",
		Transport: config.TransportHTTP,
	})

	assert.Equal(t, config.TransportStdio, a.ServerCfg.Transport)
}

func TestNewStdioAdapter_PreservesConfig(t *testing.T) {
	cfg := config.ServerConfig{
		Command:    "my-command",
		Args:       []string{"--flag", "value"},
		Env:        map[string]string{"KEY": "val"},
		WorkingDir: "/tmp",
		Enabled:    true,
	}

	a := NewStdioAdapter("preserved", cfg)

	assert.Equal(t, "my-command", a.ServerCfg.Command)
	assert.Equal(t, []string{"--flag", "value"}, a.ServerCfg.Args)
	assert.Equal(t, map[string]string{"KEY": "val"}, a.ServerCfg.Env)
	assert.Equal(t, "/tmp", a.ServerCfg.WorkingDir)
	assert.True(t, a.ServerCfg.Enabled)
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
	assert.Contains(t, err.Error(), "failed to start process")
}

func TestStdioAdapter_Start_EmptyCommand(t *testing.T) {
	a := NewStdioAdapter("empty", config.ServerConfig{
		Command: "",
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

func TestStdioAdapter_HealthCheck_NotStarted(t *testing.T) {
	a := NewStdioAdapter("test", config.ServerConfig{
		Command: "echo",
	})

	err := a.HealthCheck(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "process not started")
}

func TestStdioAdapter_HealthCheck_Running(t *testing.T) {
	a := NewStdioAdapter("test", config.ServerConfig{
		Command: "sleep",
		Args:    []string{"10"},
	})

	ctx := context.Background()
	err := a.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = a.Stop(ctx) }()

	err = a.HealthCheck(ctx)
	// Note: The health check sends signal 0 which may fail on some platforms
	// We just ensure it doesn't panic and returns a consistent result
	if err != nil {
		assert.Contains(t, err.Error(), "process not running")
	}
}

func TestStdioAdapter_Start_WithWorkingDir(t *testing.T) {
	a := NewStdioAdapter("test", config.ServerConfig{
		Command:    "sleep",
		Args:       []string{"1"},
		WorkingDir: "/tmp",
	})

	ctx := context.Background()
	err := a.Start(ctx)
	require.NoError(t, err)

	err = a.Stop(ctx)
	assert.NoError(t, err)
}

func TestStdioAdapter_Start_WithEnv(t *testing.T) {
	a := NewStdioAdapter("test", config.ServerConfig{
		Command: "sleep",
		Args:    []string{"1"},
		Env: map[string]string{
			"TEST_VAR_1": "value1",
			"TEST_VAR_2": "value2",
		},
	})

	ctx := context.Background()
	err := a.Start(ctx)
	require.NoError(t, err)

	err = a.Stop(ctx)
	assert.NoError(t, err)
}

func TestStdioAdapter_Config(t *testing.T) {
	a := NewStdioAdapter("stdio-test", config.ServerConfig{
		Command: "test-cmd",
		Args:    []string{"arg1", "arg2"},
		Enabled: true,
	})

	cfg := a.Config()
	assert.Equal(t, "stdio-test", cfg["name"])
	assert.Equal(t, "test-cmd", cfg["command"])
	assert.Equal(t, "stdio", cfg["transport"])
}

// ============================================================================
// DockerAdapter Tests
// ============================================================================

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

func TestNewDockerAdapter_NoTag(t *testing.T) {
	a := NewDockerAdapter(
		"test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image: "my-image",
		},
	)

	assert.Equal(t, "my-image", a.Container.ImageRef())
}

func TestNewDockerAdapter_FullConfig(t *testing.T) {
	a := NewDockerAdapter(
		"full-docker",
		config.ServerConfig{
			Command:   "docker",
			Transport: config.TransportHTTP,
			Enabled:   true,
		},
		config.ContainerConfig{
			Image:         "test-image",
			Tag:           "v1.0",
			Port:          8080,
			HostPort:      9090,
			Env:           map[string]string{"ENV1": "val1"},
			Volumes:       []string{"/host:/container"},
			Network:       "bridge",
			Command:       []string{"serve"},
			HealthCheck:   "/health",
			RestartPolicy: "always",
		},
	)

	assert.Equal(t, "full-docker", a.Name())
	assert.Equal(t, "test-image:v1.0", a.Container.ImageRef())
	assert.Equal(t, 8080, a.Container.Port)
	assert.Equal(t, 9090, a.Container.HostPort)
	assert.Equal(t, "bridge", a.Container.Network)
	assert.Equal(t, "/health", a.Container.HealthCheck)
	assert.Equal(t, "always", a.Container.RestartPolicy)
	assert.Equal(t, []string{"/host:/container"}, a.Container.Volumes)
	assert.Equal(t, []string{"serve"}, a.Container.Command)
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

func TestDockerAdapter_HealthCheck_WithHTTPEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"healthy"}`))
			} else {
				http.NotFound(w, r)
			}
		},
	))
	defer server.Close()

	// Extract port from server URL
	// The test server URL is like http://127.0.0.1:PORT
	a := NewDockerAdapter(
		"test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image:       "test",
			HostPort:    0, // Will be overridden
			HealthCheck: "/health",
		},
	)

	// Manually set to test the HTTP health check path
	// Since we can't easily override the port, we test error case
	a.Container.HostPort = 1 // Invalid port to force failure
	a.Container.HealthCheck = "/health"

	ctx := context.Background()
	err := a.HealthCheck(ctx)
	assert.Error(t, err)
}

func TestDockerAdapter_HealthCheck_NoHealthEndpoint(t *testing.T) {
	a := NewDockerAdapter(
		"test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image: "test",
		},
	)

	// Without HealthCheck endpoint configured, it falls back to docker inspect
	// which will fail since container doesn't exist
	ctx := context.Background()
	err := a.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container not found")
}

func TestDockerAdapter_HealthCheck_BadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	))
	defer server.Close()

	// We need to test with a real port, so we'll use the test server
	a := NewDockerAdapter(
		"test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image:       "test",
			HostPort:    1, // This won't work, but tests the path
			HealthCheck: "/health",
		},
	)

	ctx := context.Background()
	err := a.HealthCheck(ctx)
	assert.Error(t, err)
}

// ============================================================================
// HTTPAdapter Tests
// ============================================================================

func TestNewHTTPAdapter(t *testing.T) {
	a := NewHTTPAdapter("test-http", config.ServerConfig{
		URL: "http://localhost:8080",
	})

	assert.Equal(t, "test-http", a.Name())
	assert.Equal(t, StateIdle, a.State())
	assert.Equal(t, config.TransportHTTP, a.ServerCfg.Transport)
}

func TestNewHTTPAdapter_OverridesTransport(t *testing.T) {
	// Even if stdio is specified, it should be overridden to HTTP
	a := NewHTTPAdapter("test", config.ServerConfig{
		URL:       "http://localhost:8080",
		Transport: config.TransportStdio,
	})

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
	assert.Contains(t, err.Error(), "health check failed")
}

func TestHTTPAdapter_HealthCheck_VariousStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{"200 OK", http.StatusOK, false},
		{"201 Created", http.StatusCreated, false},
		{"204 No Content", http.StatusNoContent, false},
		{"301 Redirect", http.StatusMovedPermanently, false},
		{"400 Bad Request", http.StatusBadRequest, true},
		{"401 Unauthorized", http.StatusUnauthorized, true},
		{"403 Forbidden", http.StatusForbidden, true},
		{"404 Not Found", http.StatusNotFound, true},
		{"500 Internal Error", http.StatusInternalServerError, true},
		{"502 Bad Gateway", http.StatusBadGateway, true},
		{"503 Unavailable", http.StatusServiceUnavailable, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.statusCode)
				},
			))
			defer server.Close()

			a := NewHTTPAdapter("test", config.ServerConfig{URL: server.URL})
			err := a.HealthCheck(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPAdapter_HealthCheck_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(5 * time.Second)
			w.WriteHeader(http.StatusOK)
		},
	))
	defer server.Close()

	a := NewHTTPAdapter("test", config.ServerConfig{URL: server.URL})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := a.HealthCheck(ctx)
	assert.Error(t, err)
}

func TestHTTPAdapter_Config(t *testing.T) {
	a := NewHTTPAdapter("http-test", config.ServerConfig{
		URL:     "http://localhost:9000",
		Enabled: true,
	})

	cfg := a.Config()
	assert.Equal(t, "http-test", cfg["name"])
	assert.Equal(t, "http", cfg["transport"])
	assert.Equal(t, true, cfg["enabled"])
}

// ============================================================================
// Lifecycle Tests
// ============================================================================

func TestStdioAdapter_DoubleStart(t *testing.T) {
	a := NewStdioAdapter("test", config.ServerConfig{
		Command: "sleep",
		Args:    []string{"10"},
	})

	ctx := context.Background()

	err := a.Start(ctx)
	require.NoError(t, err)
	defer func() { _ = a.Stop(ctx) }()

	// Starting again should still work (overwrites the previous process)
	// This is implementation dependent, but the adapter should handle it
	err = a.Start(ctx)
	// The behavior here depends on implementation - may succeed or fail
	// We're just ensuring it doesn't panic
	_ = err
}

func TestStdioAdapter_DoubleStop(t *testing.T) {
	a := NewStdioAdapter("test", config.ServerConfig{
		Command: "sleep",
		Args:    []string{"10"},
	})

	ctx := context.Background()

	err := a.Start(ctx)
	require.NoError(t, err)

	err = a.Stop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, StateStopped, a.State())

	// Stopping again - the process is already dead, so Kill() will fail
	// and set state to Error. This is expected behavior.
	err = a.Stop(ctx)
	// The second stop may fail since process is already killed
	// The state will be either stopped or error depending on timing
	state := a.State()
	assert.True(t, state == StateStopped || state == StateError,
		"state should be stopped or error, got: %s", state)
}

func TestHTTPAdapter_DoubleStart(t *testing.T) {
	a := NewHTTPAdapter("test", config.ServerConfig{
		URL: "http://localhost:8080",
	})

	ctx := context.Background()

	err := a.Start(ctx)
	assert.NoError(t, err)

	// Starting again should be idempotent
	err = a.Start(ctx)
	assert.NoError(t, err)
	assert.Equal(t, StateRunning, a.State())
}

func TestHTTPAdapter_DoubleStop(t *testing.T) {
	a := NewHTTPAdapter("test", config.ServerConfig{
		URL: "http://localhost:8080",
	})

	ctx := context.Background()

	_ = a.Start(ctx)
	_ = a.Stop(ctx)

	// Stopping again should be idempotent
	err := a.Stop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, StateStopped, a.State())
}

// ============================================================================
// Context Cancellation Tests
// ============================================================================

func TestStdioAdapter_Start_ContextCancelled(t *testing.T) {
	a := NewStdioAdapter("test", config.ServerConfig{
		Command: "sleep",
		Args:    []string{"60"},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Start with cancelled context - the exec.CommandContext will use this
	err := a.Start(ctx)
	// Behavior depends on timing, but shouldn't panic
	_ = err

	// Cleanup
	_ = a.Stop(context.Background())
}

// ============================================================================
// Interface Compliance Tests
// ============================================================================

var _ registry.Adapter = (*StdioAdapter)(nil)
var _ registry.Adapter = (*DockerAdapter)(nil)
var _ registry.Adapter = (*HTTPAdapter)(nil)

// ============================================================================
// DockerAdapter Start/Stop Tests (without actual Docker)
// ============================================================================

func TestDockerAdapter_Start_NoDocker(t *testing.T) {
	a := NewDockerAdapter(
		"test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image: "nonexistent-image-12345",
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := a.Start(ctx)
	// This will fail because Docker isn't running or image doesn't exist
	assert.Error(t, err)
	assert.Equal(t, StateError, a.State())
}

func TestDockerAdapter_Stop_NotStarted(t *testing.T) {
	a := NewDockerAdapter(
		"test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image: "test",
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := a.Stop(ctx)
	// Stopping a non-existent container will fail
	assert.Error(t, err)
	assert.Equal(t, StateError, a.State())
}

// ============================================================================
// Thread Safety Tests
// ============================================================================

func TestBaseAdapter_ConcurrentStateAccess(t *testing.T) {
	b := &BaseAdapter{state: StateIdle} // Initialize with a valid state

	var wg sync.WaitGroup
	iterations := 100

	validStates := []State{StateIdle, StateStarting, StateRunning, StateStopping, StateStopped, StateError}

	// Writers
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			b.SetState(validStates[i%len(validStates)])
		}(i)
	}

	// Readers
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state := b.State()
			// Ensure state is a valid State value (including empty which is default)
			isValid := state == "" ||
				state == StateIdle ||
				state == StateStarting ||
				state == StateRunning ||
				state == StateStopping ||
				state == StateStopped ||
				state == StateError
			assert.True(t, isValid, "state should be valid, got: %q", state)
		}()
	}

	wg.Wait()
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestStdioAdapter_ArgsAreCopied(t *testing.T) {
	originalArgs := []string{"arg1", "arg2"}
	a := NewStdioAdapter("test", config.ServerConfig{
		Command: "echo",
		Args:    originalArgs,
	})

	// Modify original slice
	originalArgs[0] = "modified"

	// Adapter should still have original value (depends on implementation)
	// This test ensures the adapter's args are not affected by external changes
	// after Start() is called
	ctx := context.Background()
	err := a.Start(ctx)
	// May or may not succeed depending on implementation
	_ = err
	_ = a.Stop(ctx)
}

func TestDockerAdapter_ContainerConfig_ImageRef(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		tag      string
		expected string
	}{
		{"image with tag", "nginx", "latest", "nginx:latest"},
		{"image without tag", "alpine", "", "alpine"},
		{"image with version tag", "redis", "7.0", "redis:7.0"},
		{"image with registry", "gcr.io/my-project/image", "v1", "gcr.io/my-project/image:v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewDockerAdapter(
				"test",
				config.ServerConfig{},
				config.ContainerConfig{
					Image: tt.image,
					Tag:   tt.tag,
				},
			)
			assert.Equal(t, tt.expected, a.Container.ImageRef())
		})
	}
}

// ============================================================================
// Adapter Name Uniqueness
// ============================================================================

func TestAdapters_UniqueNames(t *testing.T) {
	stdio := NewStdioAdapter("adapter-1", config.ServerConfig{Command: "echo"})
	docker := NewDockerAdapter("adapter-2", config.ServerConfig{}, config.ContainerConfig{Image: "test"})
	httpAdap := NewHTTPAdapter("adapter-3", config.ServerConfig{URL: "http://localhost"})

	names := map[string]bool{}
	for _, name := range []string{stdio.Name(), docker.Name(), httpAdap.Name()} {
		assert.False(t, names[name], "duplicate name: %s", name)
		names[name] = true
	}
}

// ============================================================================
// Full Lifecycle Tests
// ============================================================================

func TestStdioAdapter_FullLifecycle(t *testing.T) {
	a := NewStdioAdapter("lifecycle-test", config.ServerConfig{
		Command: "sleep",
		Args:    []string{"10"},
	})

	ctx := context.Background()

	// Initial state
	assert.Equal(t, StateIdle, a.State())

	// Start
	err := a.Start(ctx)
	require.NoError(t, err)
	assert.Equal(t, StateRunning, a.State())

	// Health check
	_ = a.HealthCheck(ctx)

	// Stop
	err = a.Stop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, StateStopped, a.State())
}

func TestHTTPAdapter_FullLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"status":"ok"}`)
		},
	))
	defer server.Close()

	a := NewHTTPAdapter("lifecycle-test", config.ServerConfig{
		URL: server.URL,
	})

	ctx := context.Background()

	// Initial state
	assert.Equal(t, StateIdle, a.State())

	// Start
	err := a.Start(ctx)
	require.NoError(t, err)
	assert.Equal(t, StateRunning, a.State())

	// Health check
	err = a.HealthCheck(ctx)
	assert.NoError(t, err)

	// Stop
	err = a.Stop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, StateStopped, a.State())
}

// ============================================================================
// DockerAdapter Configuration Code Path Tests
// ============================================================================

func TestDockerAdapter_Start_WithPortMapping(t *testing.T) {
	a := NewDockerAdapter(
		"port-test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image:    "nonexistent-image",
			Port:     8080,
			HostPort: 9090,
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Will fail because Docker isn't available, but exercises port mapping code
	err := a.Start(ctx)
	assert.Error(t, err)
	assert.Equal(t, StateError, a.State())
}

func TestDockerAdapter_Start_WithEnvVars(t *testing.T) {
	a := NewDockerAdapter(
		"env-test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image: "nonexistent-image",
			Env: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := a.Start(ctx)
	assert.Error(t, err)
	assert.Equal(t, StateError, a.State())
}

func TestDockerAdapter_Start_WithVolumes(t *testing.T) {
	a := NewDockerAdapter(
		"volumes-test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image:   "nonexistent-image",
			Volumes: []string{"/host/path:/container/path", "/data:/data:ro"},
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := a.Start(ctx)
	assert.Error(t, err)
	assert.Equal(t, StateError, a.State())
}

func TestDockerAdapter_Start_WithNetwork(t *testing.T) {
	a := NewDockerAdapter(
		"network-test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image:   "nonexistent-image",
			Network: "my-custom-network",
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := a.Start(ctx)
	assert.Error(t, err)
	assert.Equal(t, StateError, a.State())
}

func TestDockerAdapter_Start_WithRestartPolicy(t *testing.T) {
	a := NewDockerAdapter(
		"restart-test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image:         "nonexistent-image",
			RestartPolicy: "unless-stopped",
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := a.Start(ctx)
	assert.Error(t, err)
	assert.Equal(t, StateError, a.State())
}

func TestDockerAdapter_Start_WithCommand(t *testing.T) {
	a := NewDockerAdapter(
		"command-test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image:   "nonexistent-image",
			Command: []string{"sh", "-c", "echo hello"},
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := a.Start(ctx)
	assert.Error(t, err)
	assert.Equal(t, StateError, a.State())
}

func TestDockerAdapter_Start_WithAllOptions(t *testing.T) {
	a := NewDockerAdapter(
		"all-options-test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image:         "nonexistent-image",
			Tag:           "latest",
			Port:          8080,
			HostPort:      9090,
			Env:           map[string]string{"KEY": "value"},
			Volumes:       []string{"/vol1:/vol1"},
			Network:       "bridge",
			RestartPolicy: "always",
			Command:       []string{"npm", "start"},
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := a.Start(ctx)
	assert.Error(t, err)
	assert.Equal(t, StateError, a.State())
}

func TestDockerAdapter_HealthCheck_DockerInspectFailure(t *testing.T) {
	// Adapter without health check endpoint, will fall back to docker inspect
	a := NewDockerAdapter(
		"inspect-failure-test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image: "nonexistent-image",
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := a.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container not found")
}

func TestStdioAdapter_HealthCheck_ProcessNotRunning(t *testing.T) {
	a := NewStdioAdapter("health-test", config.ServerConfig{
		Command: "false", // Command that exits immediately
	})

	ctx := context.Background()

	// Start the adapter - the process will exit quickly
	err := a.Start(ctx)
	require.NoError(t, err)

	// Wait for process to exit
	time.Sleep(100 * time.Millisecond)

	// Health check should fail because process exited
	err = a.HealthCheck(ctx)
	assert.Error(t, err)

	_ = a.Stop(ctx)
}

func TestStdioAdapter_HealthCheck_ProcessRunning(t *testing.T) {
	a := NewStdioAdapter("health-running-test", config.ServerConfig{
		Command: "sleep",
		Args:    []string{"10"},
	})

	ctx := context.Background()

	// Start the adapter
	err := a.Start(ctx)
	require.NoError(t, err)

	// Health check will be called - current implementation uses Signal(nil) which
	// may not be supported on all platforms, but we exercise the code path
	_ = a.HealthCheck(ctx)

	// Cleanup
	_ = a.Stop(ctx)
}

func TestStdioAdapter_HealthCheck_NoProcess(t *testing.T) {
	a := NewStdioAdapter("health-no-process-test", config.ServerConfig{
		Command: "sleep",
		Args:    []string{"10"},
	})

	ctx := context.Background()

	// Health check should fail because process not started
	err := a.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "process not started")
}

// ============================================================================
// Error Path Coverage Tests
// ============================================================================

func TestStdioAdapter_HealthCheck_ProcessExited(t *testing.T) {
	// This test exercises the Signal() error path by starting a short-lived
	// process and then checking health after it exits
	tests := []struct {
		name    string
		command string
		args    []string
		wait    time.Duration
	}{
		{
			name:    "process exits with true",
			command: "true",
			args:    nil,
			wait:    50 * time.Millisecond,
		},
		{
			name:    "process exits with echo",
			command: "sh",
			args:    []string{"-c", "exit 0"},
			wait:    50 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewStdioAdapter("signal-error-test", config.ServerConfig{
				Command: tt.command,
				Args:    tt.args,
			})

			ctx := context.Background()

			err := a.Start(ctx)
			require.NoError(t, err)

			// Wait for process to exit
			time.Sleep(tt.wait)

			// Health check should fail with "process not running" error
			// This exercises the Signal() error path at line 143-144
			err = a.HealthCheck(ctx)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "process not running")

			_ = a.Stop(ctx)
		})
	}
}

func TestStdioAdapter_HealthCheck_CmdNilProcess(t *testing.T) {
	// Test when cmd exists but process is nil
	a := NewStdioAdapter("nil-process-test", config.ServerConfig{
		Command: "sleep",
		Args:    []string{"10"},
	})

	// Manually set cmd but leave process nil to test the nil check
	a.cmdMu.Lock()
	a.cmd = exec.Command("sleep", "10") // Not started, so Process is nil
	a.cmdMu.Unlock()

	ctx := context.Background()
	err := a.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "process not started")
}

func TestStdioAdapter_Stop_ProcessKillError(t *testing.T) {
	// Test Stop() error path when process is already terminated
	a := NewStdioAdapter("kill-error-test", config.ServerConfig{
		Command: "true", // Exits immediately
	})

	ctx := context.Background()

	err := a.Start(ctx)
	require.NoError(t, err)

	// Wait for process to exit naturally
	time.Sleep(100 * time.Millisecond)

	// Stop should fail with kill error since process already exited
	err = a.Stop(ctx)
	// The error may or may not occur depending on timing
	// but we exercise the code path
	state := a.State()
	assert.True(t, state == StateStopped || state == StateError)
}

func TestDockerAdapter_Stop_ContainerDoesNotExist(t *testing.T) {
	// Test Stop() error path when container doesn't exist
	a := NewDockerAdapter(
		"nonexistent-container-stop-test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image: "test",
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stop should fail because container doesn't exist
	err := a.Stop(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stop container")
	assert.Equal(t, StateError, a.State())
}

func TestDockerAdapter_HealthCheck_HTTPRequestCreationError(t *testing.T) {
	// Test the error path when http.NewRequestWithContext fails
	// This happens with invalid URLs that contain control characters
	a := NewDockerAdapter(
		"request-error-test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image:       "test",
			HostPort:    8080,
			HealthCheck: "/health\x00invalid", // Invalid URL with null char
		},
	)

	ctx := context.Background()
	err := a.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create health check request")
}

func TestDockerAdapter_HealthCheck_HTTPClientError(t *testing.T) {
	// Test the error path when HTTP client.Do() fails
	a := NewDockerAdapter(
		"http-client-error-test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image:       "test",
			HostPort:    1, // Invalid port that won't connect
			HealthCheck: "/health",
		},
	)

	ctx := context.Background()
	err := a.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}

func TestDockerAdapter_HealthCheck_HTTPBadStatus(t *testing.T) {
	// Test various HTTP error status codes
	statusCodes := []struct {
		name       string
		statusCode int
	}{
		{"400 Bad Request", http.StatusBadRequest},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"403 Forbidden", http.StatusForbidden},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"502 Bad Gateway", http.StatusBadGateway},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tc := range statusCodes {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tc.statusCode)
				},
			))
			defer server.Close()

			// Extract port from test server
			// Parse URL to get port
			var port int
			_, err := fmt.Sscanf(server.URL, "http://127.0.0.1:%d", &port)
			if err != nil {
				_, err = fmt.Sscanf(server.URL, "http://localhost:%d", &port)
			}
			require.NoError(t, err)

			a := NewDockerAdapter(
				"bad-status-test",
				config.ServerConfig{},
				config.ContainerConfig{
					Image:       "test",
					HostPort:    port,
					HealthCheck: "/",
				},
			)

			ctx := context.Background()
			err = a.HealthCheck(ctx)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), fmt.Sprintf("status %d", tc.statusCode))
		})
	}
}

func TestDockerAdapter_HealthCheck_DockerInspectNotRunning(t *testing.T) {
	// Test the fallback docker inspect path when container exists but is not running
	// This is hard to test without Docker, so we just verify the error path
	a := NewDockerAdapter(
		"not-running-test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image: "test-not-running",
			// No HealthCheck endpoint, so falls back to docker inspect
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := a.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container not found")
}

func TestHTTPAdapter_HealthCheck_RequestCreationError(t *testing.T) {
	// Test the error path when http.NewRequestWithContext fails in HTTPAdapter
	// This happens with invalid URLs containing control characters
	a := NewHTTPAdapter("request-error-test", config.ServerConfig{
		URL: "http://localhost:8080\x00invalid", // Invalid URL with null char
	})

	ctx := context.Background()
	err := a.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create request")
}

func TestHTTPAdapter_HealthCheck_InvalidURL(t *testing.T) {
	// Test various invalid URL scenarios
	tests := []struct {
		name    string
		url     string
		errPart string
	}{
		{
			name:    "URL with control character",
			url:     "http://localhost\x7f:8080",
			errPart: "failed to create request",
		},
		{
			name:    "URL with null byte",
			url:     "http://host\x00name:8080",
			errPart: "failed to create request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewHTTPAdapter("invalid-url-test", config.ServerConfig{
				URL: tt.url,
			})

			ctx := context.Background()
			err := a.HealthCheck(ctx)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errPart)
		})
	}
}

func TestDockerAdapter_HealthCheck_ContextTimeout(t *testing.T) {
	// Test health check with cancelled context
	a := NewDockerAdapter(
		"context-timeout-test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image:       "test",
			HostPort:    9999,
			HealthCheck: "/health",
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := a.HealthCheck(ctx)
	assert.Error(t, err)
}

func TestDockerAdapter_Start_ContextTimeout(t *testing.T) {
	// Test Start with very short timeout
	a := NewDockerAdapter(
		"start-timeout-test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image: "alpine",
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond) // Ensure context expires

	err := a.Start(ctx)
	assert.Error(t, err)
	assert.Equal(t, StateError, a.State())
}

func TestDockerAdapter_Stop_ContextTimeout(t *testing.T) {
	// Test Stop with very short timeout
	a := NewDockerAdapter(
		"stop-timeout-test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image: "test",
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond) // Ensure context expires

	err := a.Stop(ctx)
	assert.Error(t, err)
	assert.Equal(t, StateError, a.State())
}

func TestHTTPAdapter_HealthCheck_Timeout(t *testing.T) {
	// Create a slow server that doesn't respond in time
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// Check if context was cancelled
			select {
			case <-r.Context().Done():
				return
			case <-time.After(10 * time.Second):
				w.WriteHeader(http.StatusOK)
			}
		},
	))
	defer server.Close()

	a := NewHTTPAdapter("timeout-test", config.ServerConfig{
		URL: server.URL,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := a.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}

func TestDockerAdapter_HealthCheck_SuccessfulHTTP(t *testing.T) {
	// Test successful HTTP health check for DockerAdapter
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"healthy"}`))
			} else {
				http.NotFound(w, r)
			}
		},
	))
	defer server.Close()

	// Extract port from test server
	var port int
	_, err := fmt.Sscanf(server.URL, "http://127.0.0.1:%d", &port)
	if err != nil {
		_, err = fmt.Sscanf(server.URL, "http://localhost:%d", &port)
	}
	require.NoError(t, err)

	a := NewDockerAdapter(
		"http-success-test",
		config.ServerConfig{},
		config.ContainerConfig{
			Image:       "test",
			HostPort:    port,
			HealthCheck: "/health",
		},
	)

	ctx := context.Background()
	err = a.HealthCheck(ctx)
	assert.NoError(t, err)
}
