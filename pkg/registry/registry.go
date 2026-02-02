// Package registry provides a thread-safe adapter registry for managing
// MCP server adapters with lifecycle operations.
package registry

import (
	"context"
	"fmt"
	"sync"
)

// Adapter defines the interface for an MCP server adapter.
type Adapter interface {
	// Name returns the unique name of the adapter.
	Name() string

	// Config returns the adapter configuration.
	Config() map[string]interface{}

	// Start starts the adapter.
	Start(ctx context.Context) error

	// Stop stops the adapter.
	Stop(ctx context.Context) error

	// HealthCheck checks if the adapter is healthy.
	HealthCheck(ctx context.Context) error
}

// Registry manages MCP server adapters.
type Registry struct {
	adapters map[string]Adapter
	mu       sync.RWMutex
}

// New creates a new adapter registry.
func New() *Registry {
	return &Registry{
		adapters: make(map[string]Adapter),
	}
}

// Register adds an adapter to the registry.
func (r *Registry) Register(adapter Adapter) error {
	if adapter == nil {
		return fmt.Errorf("adapter cannot be nil")
	}

	name := adapter.Name()
	if name == "" {
		return fmt.Errorf("adapter name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.adapters[name]; exists {
		return fmt.Errorf("adapter already registered: %s", name)
	}

	r.adapters[name] = adapter
	return nil
}

// Unregister removes an adapter from the registry.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.adapters[name]; !exists {
		return fmt.Errorf("adapter not found: %s", name)
	}

	delete(r.adapters, name)
	return nil
}

// Get retrieves an adapter by name.
func (r *Registry) Get(name string) (Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[name]
	return adapter, ok
}

// List returns the names of all registered adapters.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	return names
}

// Count returns the number of registered adapters.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.adapters)
}

// StartAll starts all registered adapters. Returns on first error.
func (r *Registry) StartAll(ctx context.Context) error {
	r.mu.RLock()
	adapters := make([]Adapter, 0, len(r.adapters))
	for _, a := range r.adapters {
		adapters = append(adapters, a)
	}
	r.mu.RUnlock()

	for _, a := range adapters {
		if err := a.Start(ctx); err != nil {
			return fmt.Errorf("failed to start adapter %s: %w", a.Name(), err)
		}
	}
	return nil
}

// StopAll stops all registered adapters. Collects all errors.
func (r *Registry) StopAll(ctx context.Context) error {
	r.mu.RLock()
	adapters := make([]Adapter, 0, len(r.adapters))
	for _, a := range r.adapters {
		adapters = append(adapters, a)
	}
	r.mu.RUnlock()

	var errs []error
	for _, a := range adapters {
		if err := a.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf(
				"failed to stop adapter %s: %w", a.Name(), err,
			))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors stopping adapters: %v", errs)
	}
	return nil
}

// HealthCheckAll runs health checks on all adapters.
// Returns a map of adapter name to error (nil if healthy).
func (r *Registry) HealthCheckAll(
	ctx context.Context,
) map[string]error {
	r.mu.RLock()
	adapters := make([]Adapter, 0, len(r.adapters))
	for _, a := range r.adapters {
		adapters = append(adapters, a)
	}
	r.mu.RUnlock()

	results := make(map[string]error)
	for _, a := range adapters {
		results[a.Name()] = a.HealthCheck(ctx)
	}
	return results
}
