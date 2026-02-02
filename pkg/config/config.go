// Package config provides configuration types and loading for MCP
// server definitions, including stdio and containerized servers.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TransportType specifies the MCP transport mechanism.
type TransportType string

const (
	// TransportStdio is stdin/stdout transport.
	TransportStdio TransportType = "stdio"
	// TransportHTTP is HTTP with SSE transport.
	TransportHTTP TransportType = "http"
)

// ServerConfig defines an MCP server configuration.
type ServerConfig struct {
	// Name is the unique identifier for this server.
	Name string `json:"name" yaml:"name"`

	// Command is the executable to run (for stdio transport).
	Command string `json:"command,omitempty" yaml:"command,omitempty"`

	// Args are command-line arguments for the server process.
	Args []string `json:"args,omitempty" yaml:"args,omitempty"`

	// Env are environment variables for the server process.
	Env map[string]string `json:"env,omitempty" yaml:"env,omitempty"`

	// Transport specifies the transport type.
	Transport TransportType `json:"transport" yaml:"transport"`

	// URL is the server URL (for HTTP transport).
	URL string `json:"url,omitempty" yaml:"url,omitempty"`

	// WorkingDir is the working directory for the process.
	WorkingDir string `json:"working_dir,omitempty" yaml:"working_dir,omitempty"`

	// Enabled indicates whether the server is enabled.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Description is a human-readable description.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Version is the server version.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

// Validate checks the server configuration for errors.
func (c *ServerConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("server name is required")
	}

	if c.Transport == "" {
		return fmt.Errorf("transport type is required for server %s", c.Name)
	}

	switch c.Transport {
	case TransportStdio:
		if c.Command == "" {
			return fmt.Errorf(
				"command is required for stdio server %s", c.Name,
			)
		}
	case TransportHTTP:
		if c.URL == "" {
			return fmt.Errorf(
				"URL is required for HTTP server %s", c.Name,
			)
		}
	default:
		return fmt.Errorf(
			"unknown transport type %q for server %s",
			c.Transport, c.Name,
		)
	}

	return nil
}

// ContainerConfig defines configuration for a Docker-based MCP server.
type ContainerConfig struct {
	// Image is the Docker image name.
	Image string `json:"image" yaml:"image"`

	// Tag is the Docker image tag.
	Tag string `json:"tag,omitempty" yaml:"tag,omitempty"`

	// Port is the port the container exposes.
	Port int `json:"port,omitempty" yaml:"port,omitempty"`

	// HostPort is the host port to map to.
	HostPort int `json:"host_port,omitempty" yaml:"host_port,omitempty"`

	// Env are environment variables for the container.
	Env map[string]string `json:"env,omitempty" yaml:"env,omitempty"`

	// Volumes are volume mounts (host:container).
	Volumes []string `json:"volumes,omitempty" yaml:"volumes,omitempty"`

	// Network is the Docker network to join.
	Network string `json:"network,omitempty" yaml:"network,omitempty"`

	// Command overrides the container command.
	Command []string `json:"command,omitempty" yaml:"command,omitempty"`

	// HealthCheck is the health check endpoint path.
	HealthCheck string `json:"health_check,omitempty" yaml:"health_check,omitempty"`

	// RestartPolicy is the container restart policy.
	RestartPolicy string `json:"restart_policy,omitempty" yaml:"restart_policy,omitempty"`
}

// Validate checks the container configuration for errors.
func (c *ContainerConfig) Validate() error {
	if c.Image == "" {
		return fmt.Errorf("container image is required")
	}
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("invalid container port: %d", c.Port)
	}
	if c.HostPort < 0 || c.HostPort > 65535 {
		return fmt.Errorf("invalid host port: %d", c.HostPort)
	}
	return nil
}

// ImageRef returns the full image reference (image:tag).
func (c *ContainerConfig) ImageRef() string {
	if c.Tag != "" {
		return c.Image + ":" + c.Tag
	}
	return c.Image
}

// FileConfig represents a configuration file containing multiple servers.
type FileConfig struct {
	Servers    []ServerConfig    `json:"servers" yaml:"servers"`
	Containers []ContainerConfig `json:"containers,omitempty" yaml:"containers,omitempty"`
}

// LoadFromFile loads configuration from a JSON or YAML file.
func LoadFromFile(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))

	var config FileConfig
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		// Use JSON unmarshaler for YAML-compatible JSON.
		// For full YAML support, users should add a YAML dependency.
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf(
				"failed to parse config (JSON format expected for .yaml "+
					"without yaml dependency): %w", err,
			)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format: %s", ext)
	}

	return &config, nil
}

// Validate validates all configurations in the file config.
func (f *FileConfig) Validate() error {
	for i, s := range f.Servers {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("server[%d]: %w", i, err)
		}
	}
	for i, c := range f.Containers {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("container[%d]: %w", i, err)
		}
	}
	return nil
}
