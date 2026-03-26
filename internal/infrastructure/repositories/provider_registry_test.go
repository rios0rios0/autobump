//go:build unit

package repositories_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"

	"github.com/rios0rios0/autobump/internal/infrastructure/repositories"
	gitforgeEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	"github.com/rios0rios0/gitforge/pkg/providers/infrastructure/github"
)

func TestNewProviderRegistry(t *testing.T) {
	t.Parallel()

	t.Run("should create a non-nil registry", func(t *testing.T) {
		// given / when
		reg := repositories.NewProviderRegistry()

		// then
		require.NotNil(t, reg)
	})
}

func TestGetAdapterByURL(t *testing.T) {
	t.Parallel()

	t.Run("should return nil when URL does not match any provider", func(t *testing.T) {
		// given
		reg := repositories.NewProviderRegistry()

		// when
		adapter := reg.GetAdapterByURL("https://unknown-host.example.com/repo.git")

		// then
		assert.Nil(t, adapter)
	})

	t.Run("should not panic when URL matches a known provider", func(t *testing.T) {
		// given
		reg := repositories.NewProviderRegistry()

		// when / then
		assert.NotPanics(t, func() {
			_ = reg.GetAdapterByURL("https://github.com/user/repo.git")
		})
	})
}

func TestGetAdapterByServiceType(t *testing.T) {
	t.Parallel()

	t.Run("should return nil when service type is unknown", func(t *testing.T) {
		// given
		reg := repositories.NewProviderRegistry()

		// when
		adapter := reg.GetAdapterByServiceType(gitforgeEntities.UNKNOWN)

		// then
		assert.Nil(t, adapter)
	})

	t.Run("should not panic when service type is GitHub", func(t *testing.T) {
		// given
		reg := repositories.NewProviderRegistry()

		// when / then
		assert.NotPanics(t, func() {
			_ = reg.GetAdapterByServiceType(gitforgeEntities.GITHUB)
		})
	})
}

func TestRegisterProviders(t *testing.T) {
	t.Parallel()

	t.Run("should register providers without error", func(t *testing.T) {
		// given
		container := dig.New()

		// when
		err := repositories.RegisterProviders(container)

		// then
		require.NoError(t, err)
	})

	t.Run("should resolve ProviderRegistry after registration", func(t *testing.T) {
		// given
		container := dig.New()
		require.NoError(t, repositories.RegisterProviders(container))

		// when
		var reg *repositories.ProviderRegistry
		err := container.Invoke(func(r *repositories.ProviderRegistry) {
			reg = r
		})

		// then
		require.NoError(t, err)
		require.NotNil(t, reg)
	})

	t.Run("should register all three provider adapters", func(t *testing.T) {
		// given
		container := dig.New()
		require.NoError(t, repositories.RegisterProviders(container))

		var reg *repositories.ProviderRegistry
		require.NoError(t, container.Invoke(func(r *repositories.ProviderRegistry) {
			reg = r
		}))

		// when
		ghAdapter := reg.GetAdapterByURL("https://github.com/org/repo.git")
		glAdapter := reg.GetAdapterByURL("https://gitlab.com/org/repo.git")
		adoAdapter := reg.GetAdapterByURL("https://dev.azure.com/org/project/_git/repo")

		// then
		assert.NotNil(t, ghAdapter)
		assert.NotNil(t, glAdapter)
		assert.NotNil(t, adoAdapter)
	})

	t.Run("should return nil adapter for unknown URL with full registry", func(t *testing.T) {
		// given
		container := dig.New()
		require.NoError(t, repositories.RegisterProviders(container))

		var reg *repositories.ProviderRegistry
		require.NoError(t, container.Invoke(func(r *repositories.ProviderRegistry) {
			reg = r
		}))

		// when
		adapter := reg.GetAdapterByURL("https://unknown.example.com/repo.git")

		// then
		assert.Nil(t, adapter)
	})
}

func TestNewDiscoverer(t *testing.T) {
	t.Parallel()

	t.Run("should return a factory that produces a RepositoryDiscoverer", func(t *testing.T) {
		// given
		factory := repositories.NewDiscoverer(func(token string) gitforgeEntities.ForgeProvider {
			return github.NewProvider(token)
		})

		// when
		discoverer := factory("fake-token")

		// then
		require.NotNil(t, discoverer)
	})

	t.Run("should produce discoverer with correct name", func(t *testing.T) {
		// given
		factory := repositories.NewDiscoverer(func(token string) gitforgeEntities.ForgeProvider {
			return github.NewProvider(token)
		})

		// when
		discoverer := factory("fake-token")

		// then
		assert.NotEmpty(t, discoverer.Name())
	})
}
