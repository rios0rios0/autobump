package repositories

import (
	"fmt"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

// DiscovererFactory is a constructor that creates a RepositoryDiscoverer given an auth token.
type DiscovererFactory func(token string) entities.RepositoryDiscoverer

// DiscovererRegistry manages factories for creating repository discoverers.
type DiscovererRegistry struct {
	factories map[string]DiscovererFactory
}

// NewDiscovererRegistry creates an empty discoverer registry.
func NewDiscovererRegistry() *DiscovererRegistry {
	return &DiscovererRegistry{
		factories: make(map[string]DiscovererFactory),
	}
}

// Register adds a discoverer factory under the given provider name (e.g. "github").
func (r *DiscovererRegistry) Register(name string, factory DiscovererFactory) {
	r.factories[name] = factory
}

// Get returns a configured discoverer instance for the given provider name and token.
func (r *DiscovererRegistry) Get(name, token string) (entities.RepositoryDiscoverer, error) {
	factory, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider type: %q", name)
	}
	return factory(token), nil
}
