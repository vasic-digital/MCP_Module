// Package adapter provides base adapter types for MCP server adapters,
// including stdio, Docker, and HTTP-based adapters.
package adapter

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"digital.vasic.mcp/pkg/config"
)

// State represents the lifecycle state of an adapter.
type State string

const (
	StateIdle     State = "idle"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
	StateError    State = "error"
)

// BaseAdapter provides common functionality for all adapters.
type BaseAdapter struct {
	AdapterName string
	ServerCfg   config.ServerConfig
	state       State
	mu          sync.RWMutex
}

// Name returns the adapter name.
func (b *BaseAdapter) Name() string {
	return b.AdapterName
}

// Config returns the adapter configuration as a map.
func (b *BaseAdapter) Config() map[string]interface{} {
	return map[string]interface{}{
		"name":      b.AdapterName,
		"command":   b.ServerCfg.Command,
		"args":      b.ServerCfg.Args,
		"transport": string(b.ServerCfg.Transport),
		"enabled":   b.ServerCfg.Enabled,
	}
}

// State returns the current adapter state.
func (b *BaseAdapter) State() State {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// SetState sets the adapter state.
func (b *BaseAdapter) SetState(s State) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = s
}

// StdioAdapter manages a stdio-based MCP server process.
type StdioAdapter struct {
	BaseAdapter
	cmd       *exec.Cmd
	cmdMu     sync.Mutex
}

// NewStdioAdapter creates a new stdio adapter.
func NewStdioAdapter(name string, cfg config.ServerConfig) *StdioAdapter {
	cfg.Transport = config.TransportStdio
	return &StdioAdapter{
		BaseAdapter: BaseAdapter{
			AdapterName: name,
			ServerCfg:   cfg,
			state:       StateIdle,
		},
	}
}

// Start starts the stdio server process.
func (a *StdioAdapter) Start(ctx context.Context) error {
	a.SetState(StateStarting)

	args := make([]string, len(a.ServerCfg.Args))
	copy(args, a.ServerCfg.Args)

	a.cmdMu.Lock()
	a.cmd = exec.CommandContext(ctx, a.ServerCfg.Command, args...)
	a.cmd.Env = os.Environ()
	for k, v := range a.ServerCfg.Env {
		a.cmd.Env = append(a.cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	if a.ServerCfg.WorkingDir != "" {
		a.cmd.Dir = a.ServerCfg.WorkingDir
	}

	if err := a.cmd.Start(); err != nil {
		a.cmdMu.Unlock()
		a.SetState(StateError)
		return fmt.Errorf("failed to start process: %w", err)
	}
	a.cmdMu.Unlock()

	a.SetState(StateRunning)
	return nil
}

// Stop stops the stdio server process.
func (a *StdioAdapter) Stop(_ context.Context) error {
	a.SetState(StateStopping)

	a.cmdMu.Lock()
	defer a.cmdMu.Unlock()

	if a.cmd != nil && a.cmd.Process != nil {
		if err := a.cmd.Process.Kill(); err != nil {
			a.SetState(StateError)
			return fmt.Errorf("failed to kill process: %w", err)
		}
		_ = a.cmd.Wait()
	}

	a.SetState(StateStopped)
	return nil
}

// HealthCheck checks if the process is running.
func (a *StdioAdapter) HealthCheck(_ context.Context) error {
	a.cmdMu.Lock()
	defer a.cmdMu.Unlock()

	if a.cmd == nil || a.cmd.Process == nil {
		return fmt.Errorf("process not started")
	}

	// On Unix, sending signal 0 checks if the process exists
	if err := a.cmd.Process.Signal(os.Signal(nil)); err != nil {
		return fmt.Errorf("process not running: %w", err)
	}

	return nil
}

// DockerAdapter manages a Docker-based MCP server container.
type DockerAdapter struct {
	BaseAdapter
	Container   config.ContainerConfig
	containerID string
}

// NewDockerAdapter creates a new Docker adapter.
func NewDockerAdapter(
	name string,
	serverCfg config.ServerConfig,
	containerCfg config.ContainerConfig,
) *DockerAdapter {
	return &DockerAdapter{
		BaseAdapter: BaseAdapter{
			AdapterName: name,
			ServerCfg:   serverCfg,
			state:       StateIdle,
		},
		Container: containerCfg,
	}
}

// Start starts the Docker container.
func (a *DockerAdapter) Start(ctx context.Context) error {
	a.SetState(StateStarting)

	args := []string{"run", "-d", "--name", a.AdapterName}

	if a.Container.Port > 0 && a.Container.HostPort > 0 {
		args = append(args, "-p",
			fmt.Sprintf("%d:%d", a.Container.HostPort, a.Container.Port),
		)
	}

	for k, v := range a.Container.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	for _, vol := range a.Container.Volumes {
		args = append(args, "-v", vol)
	}

	if a.Container.Network != "" {
		args = append(args, "--network", a.Container.Network)
	}

	if a.Container.RestartPolicy != "" {
		args = append(args, "--restart", a.Container.RestartPolicy)
	}

	args = append(args, a.Container.ImageRef())
	args = append(args, a.Container.Command...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.Output()
	if err != nil {
		a.SetState(StateError)
		return fmt.Errorf("failed to start container: %w", err)
	}

	a.containerID = string(output)
	a.SetState(StateRunning)
	return nil
}

// Stop stops the Docker container.
func (a *DockerAdapter) Stop(ctx context.Context) error {
	a.SetState(StateStopping)

	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", a.AdapterName)
	if err := cmd.Run(); err != nil {
		a.SetState(StateError)
		return fmt.Errorf("failed to stop container: %w", err)
	}

	a.SetState(StateStopped)
	return nil
}

// HealthCheck checks if the container is healthy.
func (a *DockerAdapter) HealthCheck(ctx context.Context) error {
	if a.Container.HealthCheck != "" && a.Container.HostPort > 0 {
		url := fmt.Sprintf(
			"http://localhost:%d%s",
			a.Container.HostPort, a.Container.HealthCheck,
		)
		client := &http.Client{Timeout: 5 * time.Second}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("failed to create health check request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}
		_ = resp.Body.Close()

		if resp.StatusCode >= 400 {
			return fmt.Errorf(
				"health check returned status %d", resp.StatusCode,
			)
		}
		return nil
	}

	// Fallback: check if container is running via docker inspect
	cmd := exec.CommandContext(
		ctx, "docker", "inspect", "-f", "{{.State.Running}}", a.AdapterName,
	)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("container not found: %w", err)
	}
	if string(output) != "true\n" {
		return fmt.Errorf("container not running")
	}
	return nil
}

// HTTPAdapter manages an HTTP-based MCP server.
type HTTPAdapter struct {
	BaseAdapter
	httpClient *http.Client
}

// NewHTTPAdapter creates a new HTTP adapter.
func NewHTTPAdapter(name string, cfg config.ServerConfig) *HTTPAdapter {
	cfg.Transport = config.TransportHTTP
	return &HTTPAdapter{
		BaseAdapter: BaseAdapter{
			AdapterName: name,
			ServerCfg:   cfg,
			state:       StateIdle,
		},
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Start marks the HTTP adapter as running (the server is external).
func (a *HTTPAdapter) Start(_ context.Context) error {
	a.SetState(StateRunning)
	return nil
}

// Stop marks the HTTP adapter as stopped.
func (a *HTTPAdapter) Stop(_ context.Context) error {
	a.SetState(StateStopped)
	return nil
}

// HealthCheck performs an HTTP health check.
func (a *HTTPAdapter) HealthCheck(ctx context.Context) error {
	if a.ServerCfg.URL == "" {
		return fmt.Errorf("no URL configured")
	}

	url := a.ServerCfg.URL + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	return nil
}
