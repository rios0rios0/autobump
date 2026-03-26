//go:build unit

package repositories_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/infrastructure/repositories"
	gitforgeEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
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
