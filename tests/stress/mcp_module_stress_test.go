package stress

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"digital.vasic.mcp/pkg/adapter"
	"digital.vasic.mcp/pkg/config"
	"digital.vasic.mcp/pkg/protocol"
	"digital.vasic.mcp/pkg/registry"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcurrentRegistryAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	reg := registry.New()
	const numAdapters = 50

	for i := 0; i < numAdapters; i++ {
		name := fmt.Sprintf("adapter-%d", i)
		a := adapter.NewHTTPAdapter(name, config.ServerConfig{
			Name:      name,
			Transport: config.TransportHTTP,
			URL:       fmt.Sprintf("http://localhost:%d", 9100+i),
			Enabled:   true,
		})
		require.NoError(t, reg.Register(a))
	}

	var wg sync.WaitGroup
	const goroutines = 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("adapter-%d", id%numAdapters)
			a, found := reg.Get(name)
			assert.True(t, found)
			assert.NotNil(t, a)
			_ = reg.List()
			_ = reg.Count()
		}(i)
	}

	wg.Wait()
	assert.Equal(t, numAdapters, reg.Count())
}

func TestConcurrentProtocolMarshaling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	var wg sync.WaitGroup
	const goroutines = 80

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			req, err := protocol.NewRequest(id, "tools/call", map[string]interface{}{
				"name": fmt.Sprintf("tool-%d", id),
				"arguments": map[string]interface{}{
					"param": fmt.Sprintf("value-%d", id),
				},
			})
			assert.NoError(t, err)

			data, err := json.Marshal(req)
			assert.NoError(t, err)
			assert.NotEmpty(t, data)

			var decoded protocol.Request
			assert.NoError(t, json.Unmarshal(data, &decoded))
			assert.Equal(t, "tools/call", decoded.Method)
		}(i)
	}

	wg.Wait()
}

func TestConcurrentAdapterStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	const goroutines = 60
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("lifecycle-%d", id)
			a := adapter.NewHTTPAdapter(name, config.ServerConfig{
				Name:      name,
				Transport: config.TransportHTTP,
				URL:       fmt.Sprintf("http://localhost:%d", 10000+id),
				Enabled:   true,
			})

			ctx := context.Background()
			assert.NoError(t, a.Start(ctx))
			assert.Equal(t, adapter.StateRunning, a.State())
			assert.NoError(t, a.Stop(ctx))
			assert.Equal(t, adapter.StateStopped, a.State())
		}(i)
	}

	wg.Wait()
}

func TestConcurrentRegistryRegisterUnregister(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	reg := registry.New()
	var wg sync.WaitGroup
	const goroutines = 80

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("ephemeral-%d", id)
			a := adapter.NewHTTPAdapter(name, config.ServerConfig{
				Name:      name,
				Transport: config.TransportHTTP,
				URL:       "http://localhost:9999",
				Enabled:   true,
			})
			err := reg.Register(a)
			if err == nil {
				_ = reg.Unregister(name)
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentResponseCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	var wg sync.WaitGroup
	const goroutines = 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			resp, err := protocol.NewResponse(id, map[string]interface{}{
				"tools": []protocol.Tool{
					{Name: fmt.Sprintf("tool-%d", id)},
				},
			})
			assert.NoError(t, err)
			assert.False(t, resp.IsError())

			errResp := protocol.NewErrorResponse(
				id, protocol.CodeInternalError,
				fmt.Sprintf("error-%d", id), nil,
			)
			assert.True(t, errResp.IsError())
		}(i)
	}

	wg.Wait()
}

func TestConcurrentHealthCheckAll(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	reg := registry.New()
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("health-%d", i)
		a := adapter.NewHTTPAdapter(name, config.ServerConfig{
			Name:      name,
			Transport: config.TransportHTTP,
			URL:       "", // will fail health check
			Enabled:   true,
		})
		require.NoError(t, reg.Register(a))
	}

	var wg sync.WaitGroup
	const goroutines = 50

	ctx := context.Background()

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results := reg.HealthCheckAll(ctx)
			// All should have errors due to empty URLs
			for _, err := range results {
				assert.Error(t, err)
			}
		}()
	}

	wg.Wait()
}
