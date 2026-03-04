package repositories

import (
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	"github.com/rios0rios0/gitforge/pkg/providers/infrastructure/azuredevops"
	"github.com/rios0rios0/gitforge/pkg/providers/infrastructure/github"
	"github.com/rios0rios0/gitforge/pkg/providers/infrastructure/gitlab"
	"go.uber.org/dig"
)

// newDiscoverer wraps a ForgeProvider factory into a RepositoryDiscoverer factory.
func newDiscoverer(
	factory func(string) globalEntities.ForgeProvider,
) func(string) globalEntities.RepositoryDiscoverer {
	return func(token string) globalEntities.RepositoryDiscoverer {
		//nolint:errcheck // gitforge providers always implement RepositoryDiscoverer
		return factory(token).(globalEntities.RepositoryDiscoverer)
	}
}

// RegisterProviders registers all repository providers with the DIG container.
func RegisterProviders(container *dig.Container) error {
	return container.Provide(func() *ProviderRegistry {
		reg := NewProviderRegistry()

		// Register token-less adapters for URL matching and service-type detection
		reg.RegisterAdapter(github.NewProvider(""))
		reg.RegisterAdapter(gitlab.NewProvider(""))
		reg.RegisterAdapter(azuredevops.NewProvider(""))

		// Register factories for token-based construction
		reg.RegisterFactory("github", github.NewProvider)
		reg.RegisterFactory("gitlab", gitlab.NewProvider)
		reg.RegisterFactory("azuredevops", azuredevops.NewProvider)

		// Register discoverer factories
		reg.RegisterDiscoverer("github", newDiscoverer(github.NewProvider))
		reg.RegisterDiscoverer("gitlab", newDiscoverer(gitlab.NewProvider))
		reg.RegisterDiscoverer("azuredevops", newDiscoverer(azuredevops.NewProvider))

		return reg
	})
}
