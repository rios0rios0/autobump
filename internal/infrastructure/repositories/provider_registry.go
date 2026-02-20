package repositories

import (
	"github.com/rios0rios0/gitforge/domain/entities"
	domainRepos "github.com/rios0rios0/gitforge/domain/repositories"
	"github.com/rios0rios0/gitforge/infrastructure/registry"
)

// ProviderRegistry wraps gitforge's ProviderRegistry and implements the
// git.AdapterFinder interface so it can be passed to git.SetAdapterFinder.
type ProviderRegistry struct {
	*registry.ProviderRegistry
}

// NewProviderRegistry creates a new provider registry backed by gitforge.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		ProviderRegistry: registry.NewProviderRegistry(),
	}
}

// GetAdapterByURL returns the adapter matching the given URL, cast to LocalGitAuthProvider.
// This satisfies the git.AdapterFinder interface.
func (r *ProviderRegistry) GetAdapterByURL(url string) domainRepos.LocalGitAuthProvider {
	adapter := r.ProviderRegistry.GetAdapterByURL(url)
	if adapter == nil {
		return nil
	}
	lgap, ok := adapter.(domainRepos.LocalGitAuthProvider)
	if !ok {
		return nil
	}
	return lgap
}

// GetAdapterByServiceType returns the adapter for the given service type.
// This delegates to gitforge's registry which already returns LocalGitAuthProvider.
func (r *ProviderRegistry) GetAdapterByServiceType(
	serviceType entities.ServiceType,
) domainRepos.LocalGitAuthProvider {
	return r.ProviderRegistry.GetAdapterByServiceType(serviceType)
}
