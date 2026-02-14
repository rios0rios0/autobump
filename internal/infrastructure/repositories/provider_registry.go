package repositories

import (
	"github.com/rios0rios0/autobump/internal/domain/entities"
	domainRepos "github.com/rios0rios0/autobump/internal/domain/repositories"
)

// GitServiceRegistry manages all registered Git service adapters.
type GitServiceRegistry struct {
	adapters []domainRepos.GitServiceAdapter
}

// NewGitServiceRegistry creates a new registry with the given adapters.
func NewGitServiceRegistry(adapters ...domainRepos.GitServiceAdapter) *GitServiceRegistry {
	return &GitServiceRegistry{
		adapters: adapters,
	}
}

// Register adds a new Git service adapter to the registry.
func (r *GitServiceRegistry) Register(adapter domainRepos.GitServiceAdapter) {
	r.adapters = append(r.adapters, adapter)
}

// GetAdapterByURL returns the appropriate adapter for the given URL.
func (r *GitServiceRegistry) GetAdapterByURL(url string) domainRepos.GitServiceAdapter {
	for _, adapter := range r.adapters {
		if adapter.MatchesURL(url) {
			return adapter
		}
	}
	return nil
}

// GetAdapterByServiceType returns the adapter for the given service type.
func (r *GitServiceRegistry) GetAdapterByServiceType(
	serviceType entities.ServiceType,
) domainRepos.GitServiceAdapter {
	for _, adapter := range r.adapters {
		if adapter.GetServiceType() == serviceType {
			return adapter
		}
	}
	return nil
}

// defaultRegistry is the default registry instance used by global functions.
var defaultRegistry *GitServiceRegistry //nolint:gochecknoglobals // required for backward compatibility

// SetDefaultRegistry sets the default registry for global functions.
func SetDefaultRegistry(reg *GitServiceRegistry) {
	defaultRegistry = reg
}

// getDefaultRegistry returns the default registry, lazily initializing an empty one if needed.
func getDefaultRegistry() *GitServiceRegistry {
	if defaultRegistry == nil {
		defaultRegistry = NewGitServiceRegistry()
	}
	return defaultRegistry
}

// GetAdapterByURL returns the appropriate adapter for the given URL using the default registry.
func GetAdapterByURL(url string) domainRepos.GitServiceAdapter {
	return getDefaultRegistry().GetAdapterByURL(url)
}

// GetAdapterByServiceType returns the adapter for the given service type using the default registry.
func GetAdapterByServiceType(serviceType entities.ServiceType) domainRepos.GitServiceAdapter {
	return getDefaultRegistry().GetAdapterByServiceType(serviceType)
}

// NewPullRequestProvider creates the appropriate provider based on the service type.
func NewPullRequestProvider(serviceType entities.ServiceType) domainRepos.PullRequestProvider {
	adapter := GetAdapterByServiceType(serviceType)
	if adapter != nil {
		return adapter
	}
	return nil
}
