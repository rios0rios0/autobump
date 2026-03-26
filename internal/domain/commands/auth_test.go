//go:build unit

package commands_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/internal/infrastructure/repositories"
	"github.com/rios0rios0/autobump/test/domain/entitybuilders"
	gitInfra "github.com/rios0rios0/gitforge/pkg/git/infrastructure"
)

func TestCollectSSHAuthMethodsExtended(t *testing.T) {
	t.Parallel()

	t.Run("should not panic when no SSH config is set", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()

		// when / then
		assert.NotPanics(t, func() {
			_ = commands.CollectSSHAuthMethods(globalConfig)
		})
	})

	t.Run("should not panic when SSH key path does not exist", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithSSHKeyPath("/nonexistent/key").
			BuildGlobalConfig()

		// when / then
		assert.NotPanics(t, func() {
			_ = commands.CollectSSHAuthMethods(globalConfig)
		})
	})

	t.Run("should return methods when SSH key exists", func(t *testing.T) {
		// given
		keyData := generateTestSSHKey(t)
		tmpDir := t.TempDir()
		keyPath := tmpDir + "/test_key"
		require.NoError(t, os.WriteFile(keyPath, keyData, 0o600))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithSSHKeyPath(keyPath).
			BuildGlobalConfig()

		// when
		methods := commands.CollectSSHAuthMethods(globalConfig)

		// then
		assert.NotEmpty(t, methods)
	})
}

func TestDetectSSHAgentSocketsExtended(t *testing.T) {
	t.Parallel()

	t.Run("should not panic when detecting sockets", func(t *testing.T) {
		// given / when / then
		assert.NotPanics(t, func() {
			_ = commands.DetectSSHAgentSockets()
		})
	})
}

func TestDiscoverAndProcess(t *testing.T) { //nolint:tparallel // mutates package-level globals

	t.Run("should complete without error when no providers are configured", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()
		registry := repositories.NewProviderRegistry()
		commands.SetProviderRegistry(registry)
		commands.SetGitOperations(gitInfra.NewGitOperations(registry))

		// when
		err := commands.DiscoverAndProcess(context.Background(), globalConfig, registry)

		// then
		require.NoError(t, err)
	})

	t.Run("should log error and continue when provider type is invalid", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()
		globalConfig.Providers = []entities.ProviderConfig{
			{Type: "nonexistent", Token: "tok", Organizations: []string{"org"}},
		}
		registry := repositories.NewProviderRegistry()

		// when
		err := commands.DiscoverAndProcess(context.Background(), globalConfig, registry)

		// then — it logs/errors internally and continues processing
		// DiscoverAndProcess currently always returns nil; errors are only logged/counted internally
		require.NoError(t, err)
	})
}
