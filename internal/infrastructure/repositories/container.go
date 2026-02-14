package repositories

import (
	"github.com/rios0rios0/autobump/internal/infrastructure/repositories/azuredevops"
	"github.com/rios0rios0/autobump/internal/infrastructure/repositories/github"
	"github.com/rios0rios0/autobump/internal/infrastructure/repositories/gitlab"
	"go.uber.org/dig"
)

// RegisterProviders registers all repository providers with the DIG container.
func RegisterProviders(container *dig.Container) error {
	// Register provider registry with all adapters
	if err := container.Provide(func() *GitServiceRegistry {
		return NewGitServiceRegistry(
			gitlab.NewAdapter(),
			azuredevops.NewAdapter(),
			github.NewAdapter(),
		)
	}); err != nil {
		return err
	}

	// Register discoverer registry with all provider factories
	if err := container.Provide(func() *DiscovererRegistry {
		reg := NewDiscovererRegistry()
		reg.Register("github", github.NewDiscoverer)
		reg.Register("gitlab", gitlab.NewDiscoverer)
		reg.Register("azuredevops", azuredevops.NewDiscoverer)
		return reg
	}); err != nil {
		return err
	}

	return nil
}
