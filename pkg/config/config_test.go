package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ServerConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid stdio config",
			config: ServerConfig{
				Name:      "test",
				Command:   "npx",
				Transport: TransportStdio,
			},
			wantErr: false,
		},
		{
			name: "valid http config",
			config: ServerConfig{
				Name:      "test",
				URL:       "http://localhost:8080",
				Transport: TransportHTTP,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: ServerConfig{
				Command:   "npx",
				Transport: TransportStdio,
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "missing transport",
			config: ServerConfig{
				Name:    "test",
				Command: "npx",
			},
			wantErr: true,
			errMsg:  "transport type is required",
		},
		{
			name: "stdio without command",
			config: ServerConfig{
				Name:      "test",
				Transport: TransportStdio,
			},
			wantErr: true,
			errMsg:  "command is required",
		},
		{
			name: "http without URL",
			config: ServerConfig{
				Name:      "test",
				Transport: TransportHTTP,
			},
			wantErr: true,
			errMsg:  "URL is required",
		},
		{
			name: "unknown transport",
			config: ServerConfig{
				Name:      "test",
				Command:   "npx",
				Transport: "websocket",
			},
			wantErr: true,
			errMsg:  "unknown transport type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContainerConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ContainerConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: ContainerConfig{
				Image:    "mcp-server",
				Tag:      "latest",
				Port:     8080,
				HostPort: 9090,
			},
			wantErr: false,
		},
		{
			name: "missing image",
			config: ContainerConfig{
				Port: 8080,
			},
			wantErr: true,
			errMsg:  "image is required",
		},
		{
			name: "invalid port",
			config: ContainerConfig{
				Image: "mcp-server",
				Port:  -1,
			},
			wantErr: true,
			errMsg:  "invalid container port",
		},
		{
			name: "invalid host port",
			config: ContainerConfig{
				Image:    "mcp-server",
				HostPort: 70000,
			},
			wantErr: true,
			errMsg:  "invalid host port",
		},
		{
			name: "zero ports are valid",
			config: ContainerConfig{
				Image: "mcp-server",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContainerConfig_ImageRef(t *testing.T) {
	tests := []struct {
		name     string
		config   ContainerConfig
		expected string
	}{
		{
			name:     "with tag",
			config:   ContainerConfig{Image: "mcp-server", Tag: "v1.0"},
			expected: "mcp-server:v1.0",
		},
		{
			name:     "without tag",
			config:   ContainerConfig{Image: "mcp-server"},
			expected: "mcp-server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.ImageRef())
		})
	}
}

func TestLoadFromFile_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	content := `{
		"servers": [
			{
				"name": "filesystem",
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-filesystem"],
				"transport": "stdio",
				"enabled": true
			},
			{
				"name": "remote-server",
				"url": "http://localhost:9090",
				"transport": "http",
				"enabled": true
			}
		],
		"containers": [
			{
				"image": "mcp-redis",
				"tag": "latest",
				"port": 6379,
				"host_port": 16379
			}
		]
	}`

	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	config, err := LoadFromFile(configPath)
	require.NoError(t, err)

	assert.Len(t, config.Servers, 2)
	assert.Equal(t, "filesystem", config.Servers[0].Name)
	assert.Equal(t, TransportStdio, config.Servers[0].Transport)
	assert.Equal(t, "npx", config.Servers[0].Command)
	assert.True(t, config.Servers[0].Enabled)

	assert.Equal(t, "remote-server", config.Servers[1].Name)
	assert.Equal(t, TransportHTTP, config.Servers[1].Transport)

	assert.Len(t, config.Containers, 1)
	assert.Equal(t, "mcp-redis", config.Containers[0].Image)
}

func TestLoadFromFile_NotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/config.json")
	assert.Error(t, err)
}

func TestLoadFromFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad.json")
	err := os.WriteFile(configPath, []byte("not json"), 0644)
	require.NoError(t, err)

	_, err = LoadFromFile(configPath)
	assert.Error(t, err)
}

func TestLoadFromFile_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(""), 0644)
	require.NoError(t, err)

	_, err = LoadFromFile(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported config file format")
}

func TestFileConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  FileConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: FileConfig{
				Servers: []ServerConfig{
					{Name: "test", Command: "npx", Transport: TransportStdio},
				},
				Containers: []ContainerConfig{
					{Image: "test-image"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid server",
			config: FileConfig{
				Servers: []ServerConfig{
					{Name: "", Transport: TransportStdio},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid container",
			config: FileConfig{
				Containers: []ContainerConfig{
					{Image: ""},
				},
			},
			wantErr: true,
		},
		{
			name:    "empty config is valid",
			config:  FileConfig{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoadFromFile_YAML_Extension(t *testing.T) {
	// Test that .yaml extension is accepted (with JSON content)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `{"servers": [{"name": "test", "command": "echo", "transport": "stdio"}]}`
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	config, err := LoadFromFile(configPath)
	require.NoError(t, err)
	assert.Len(t, config.Servers, 1)
}

func TestLoadFromFile_YML_Extension(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	content := `{"servers": []}`
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	config, err := LoadFromFile(configPath)
	require.NoError(t, err)
	assert.Empty(t, config.Servers)
}
