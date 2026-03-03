package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"digital.vasic.mcp/pkg/adapter"
	"digital.vasic.mcp/pkg/config"
	"digital.vasic.mcp/pkg/protocol"
	"digital.vasic.mcp/pkg/registry"
)

func BenchmarkProtocolNewRequest(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	params := map[string]interface{}{
		"name":      "read_file",
		"arguments": map[string]interface{}{"path": "/tmp/test.txt"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = protocol.NewRequest(i, "tools/call", params)
	}
}

func BenchmarkProtocolNewResponse(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	result := map[string]interface{}{
		"tools": []protocol.Tool{
			{Name: "tool1", Description: "first tool"},
			{Name: "tool2", Description: "second tool"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = protocol.NewResponse(i, result)
	}
}

func BenchmarkProtocolMarshalUnmarshal(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	req, _ := protocol.NewRequest(1, "tools/call", map[string]interface{}{
		"name": "test",
		"arguments": map[string]interface{}{
			"param1": "value1",
			"param2": 42,
		},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(req)
		var decoded protocol.Request
		_ = json.Unmarshal(data, &decoded)
	}
}

func BenchmarkRegistryGet(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	reg := registry.New()
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("adapter-%d", i)
		a := adapter.NewHTTPAdapter(name, config.ServerConfig{
			Name:      name,
			Transport: config.TransportHTTP,
			URL:       "http://localhost:9000",
			Enabled:   true,
		})
		_ = reg.Register(a)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		name := fmt.Sprintf("adapter-%d", i%100)
		_, _ = reg.Get(name)
	}
}

func BenchmarkRegistryList(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	reg := registry.New()
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("adapter-%d", i)
		a := adapter.NewHTTPAdapter(name, config.ServerConfig{
			Name:      name,
			Transport: config.TransportHTTP,
			URL:       "http://localhost:9000",
			Enabled:   true,
		})
		_ = reg.Register(a)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reg.List()
	}
}

func BenchmarkAdapterStateTransition(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a := adapter.NewHTTPAdapter("bench", config.ServerConfig{
			Name:      "bench",
			Transport: config.TransportHTTP,
			URL:       "http://localhost:9000",
			Enabled:   true,
		})
		_ = a.Start(ctx)
		_ = a.Stop(ctx)
	}
}

func BenchmarkNormalizeID(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	ids := []interface{}{
		float64(42), int64(100), int(7), "string-id", float64(3.14),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = protocol.NormalizeID(ids[i%len(ids)])
	}
}

func BenchmarkServerConfigValidation(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	cfg := config.ServerConfig{
		Name:      "benchmark-server",
		Transport: config.TransportHTTP,
		URL:       "http://localhost:9000",
		Enabled:   true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cfg.Validate()
	}
}
