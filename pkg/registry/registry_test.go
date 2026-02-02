package registry

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAdapter is a test implementation of the Adapter interface.
type mockAdapter struct {
	name         string
	startErr     error
	stopErr      error
	healthErr    error
	startCalled  bool
	stopCalled   bool
	healthCalled bool
}

func (m *mockAdapter) Name() string                          { return m.name }
func (m *mockAdapter) Config() map[string]interface{}        { return nil }
func (m *mockAdapter) Start(_ context.Context) error         { m.startCalled = true; return m.startErr }
func (m *mockAdapter) Stop(_ context.Context) error          { m.stopCalled = true; return m.stopErr }
func (m *mockAdapter) HealthCheck(_ context.Context) error   { m.healthCalled = true; return m.healthErr }

func TestRegistry_Register(t *testing.T) {
	tests := []struct {
		name    string
		adapter Adapter
		wantErr bool
	}{
		{
			name:    "valid adapter",
			adapter: &mockAdapter{name: "test"},
			wantErr: false,
		},
		{
			name:    "nil adapter",
			adapter: nil,
			wantErr: true,
		},
		{
			name:    "empty name",
			adapter: &mockAdapter{name: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			err := r.Register(tt.adapter)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegistry_Register_Duplicate(t *testing.T) {
	r := New()
	err := r.Register(&mockAdapter{name: "test"})
	require.NoError(t, err)

	err = r.Register(&mockAdapter{name: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_Unregister(t *testing.T) {
	r := New()
	_ = r.Register(&mockAdapter{name: "test"})

	err := r.Unregister("test")
	assert.NoError(t, err)
	assert.Equal(t, 0, r.Count())

	err = r.Unregister("nonexistent")
	assert.Error(t, err)
}

func TestRegistry_Get(t *testing.T) {
	r := New()
	adapter := &mockAdapter{name: "test"}
	_ = r.Register(adapter)

	got, ok := r.Get("test")
	assert.True(t, ok)
	assert.Equal(t, adapter, got)

	_, ok = r.Get("nonexistent")
	assert.False(t, ok)
}

func TestRegistry_List(t *testing.T) {
	r := New()
	_ = r.Register(&mockAdapter{name: "alpha"})
	_ = r.Register(&mockAdapter{name: "beta"})
	_ = r.Register(&mockAdapter{name: "gamma"})

	names := r.List()
	assert.Len(t, names, 3)
	assert.Contains(t, names, "alpha")
	assert.Contains(t, names, "beta")
	assert.Contains(t, names, "gamma")
}

func TestRegistry_Count(t *testing.T) {
	r := New()
	assert.Equal(t, 0, r.Count())

	_ = r.Register(&mockAdapter{name: "a"})
	assert.Equal(t, 1, r.Count())

	_ = r.Register(&mockAdapter{name: "b"})
	assert.Equal(t, 2, r.Count())

	_ = r.Unregister("a")
	assert.Equal(t, 1, r.Count())
}

func TestRegistry_StartAll(t *testing.T) {
	tests := []struct {
		name     string
		adapters []*mockAdapter
		wantErr  bool
	}{
		{
			name: "all start successfully",
			adapters: []*mockAdapter{
				{name: "a"},
				{name: "b"},
			},
			wantErr: false,
		},
		{
			name: "one fails to start",
			adapters: []*mockAdapter{
				{name: "a"},
				{name: "b", startErr: fmt.Errorf("start failed")},
			},
			wantErr: true,
		},
		{
			name:     "empty registry",
			adapters: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			for _, a := range tt.adapters {
				_ = r.Register(a)
			}

			err := r.StartAll(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				for _, a := range tt.adapters {
					assert.True(t, a.startCalled)
				}
			}
		})
	}
}

func TestRegistry_StopAll(t *testing.T) {
	tests := []struct {
		name     string
		adapters []*mockAdapter
		wantErr  bool
	}{
		{
			name: "all stop successfully",
			adapters: []*mockAdapter{
				{name: "a"},
				{name: "b"},
			},
			wantErr: false,
		},
		{
			name: "one fails to stop",
			adapters: []*mockAdapter{
				{name: "a"},
				{name: "b", stopErr: fmt.Errorf("stop failed")},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			for _, a := range tt.adapters {
				_ = r.Register(a)
			}

			err := r.StopAll(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			// All adapters should have Stop called regardless
			for _, a := range tt.adapters {
				assert.True(t, a.stopCalled)
			}
		})
	}
}

func TestRegistry_HealthCheckAll(t *testing.T) {
	r := New()
	healthy := &mockAdapter{name: "healthy"}
	unhealthy := &mockAdapter{
		name:      "unhealthy",
		healthErr: fmt.Errorf("not ready"),
	}
	_ = r.Register(healthy)
	_ = r.Register(unhealthy)

	results := r.HealthCheckAll(context.Background())
	assert.Len(t, results, 2)
	assert.NoError(t, results["healthy"])
	assert.Error(t, results["unhealthy"])
	assert.True(t, healthy.healthCalled)
	assert.True(t, unhealthy.healthCalled)
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := New()
	var wg sync.WaitGroup

	// Concurrent registration
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("adapter-%d", i)
			_ = r.Register(&mockAdapter{name: name})
		}(i)
	}
	wg.Wait()
	assert.Equal(t, 100, r.Count())

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("adapter-%d", i)
			_, _ = r.Get(name)
			_ = r.List()
			_ = r.Count()
		}(i)
	}
	wg.Wait()

	// Concurrent unregister
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("adapter-%d", i)
			_ = r.Unregister(name)
		}(i)
	}
	wg.Wait()
	assert.Equal(t, 50, r.Count())
}
