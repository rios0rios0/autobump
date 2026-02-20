package repositories

import (
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	registryInfra "github.com/rios0rios0/gitforge/pkg/registry/infrastructure"
)

// ProviderRegistry wraps gitforge's ProviderRegistry and implements the
// git.AdapterFinder interface so it can be passed to gitInfra.NewGitOperations.
type ProviderRegistry struct {
	*registryInfra.ProviderRegistry
}

// NewProviderRegistry creates a new provider registry backed by gitforge.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		ProviderRegistry: registryInfra.NewProviderRegistry(),
	}
}

// GetAdapterByURL returns the adapter matching the given URL, cast to LocalGitAuthProvider.
// This satisfies the git.AdapterFinder interface.
func (r *ProviderRegistry) GetAdapterByURL(url string) globalEntities.LocalGitAuthProvider {
	adapter := r.ProviderRegistry.GetAdapterByURL(url)
	if adapter == nil {
		return nil
	}
	lgap, ok := adapter.(globalEntities.LocalGitAuthProvider)
	if !ok {
		return nil
	}
	return lgap
}

// GetAdapterByServiceType returns the adapter for the given service type.
// This delegates to gitforge's registry which already returns LocalGitAuthProvider.
func (r *ProviderRegistry) GetAdapterByServiceType(
	serviceType globalEntities.ServiceType,
) globalEntities.LocalGitAuthProvider {
	return r.ProviderRegistry.GetAdapterByServiceType(serviceType)
}
