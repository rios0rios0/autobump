//go:build unit

package commands_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/internal/infrastructure/repositories"
	"github.com/rios0rios0/autobump/test/domain/entitybuilders"
	gitInfra "github.com/rios0rios0/gitforge/pkg/git/infrastructure"
	gitforgeEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
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

func TestCollectAuthMethods(t *testing.T) { //nolint:tparallel // mutates package-level globals

	t.Run("should return nil when service type is unknown", func(t *testing.T) {
		// given
		registry := repositories.NewProviderRegistry()
		commands.SetProviderRegistry(registry)
		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

		// when
		methods := commands.CollectAuthMethods(gitforgeEntities.UNKNOWN, "user", globalConfig, projectConfig)

		// then
		assert.Nil(t, methods)
	})

	t.Run("should return nil when providerRegistry is nil", func(t *testing.T) {
		// given
		commands.SetProviderRegistry(nil)
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithGitHubAccessToken("token").
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

		// when
		methods := commands.CollectAuthMethods(gitforgeEntities.GITHUB, "user", globalConfig, projectConfig)

		// then
		assert.Nil(t, methods)
	})

	t.Run("should return auth methods for valid GitHub config", func(t *testing.T) {
		// given
		container := dig.New()
		require.NoError(t, repositories.RegisterProviders(container))
		var registry *repositories.ProviderRegistry
		require.NoError(t, container.Invoke(func(r *repositories.ProviderRegistry) {
			registry = r
		}))
		commands.SetProviderRegistry(registry)

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithGitHubAccessToken("ghp_test_token").
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

		// when
		methods := commands.CollectAuthMethods(gitforgeEntities.GITHUB, "user", globalConfig, projectConfig)

		// then
		assert.NotEmpty(t, methods)
	})
}

func TestSSHAgentAuthFromSocket(t *testing.T) {
	t.Parallel()

	t.Run("should return nil when socket does not exist", func(t *testing.T) {
		// given
		socketPath := filepath.Join(t.TempDir(), "nonexistent.sock")

		// when
		method := commands.SSHAgentAuthFromSocket(socketPath)

		// then
		assert.Nil(t, method)
	})

	t.Run("should return PublicKeysCallback when socket is valid", func(t *testing.T) {
		// given -- create a real Unix socket listener
		socketPath := filepath.Join(t.TempDir(), "test-agent.sock")
		listener, err := net.Listen("unix", socketPath)
		require.NoError(t, err)
		defer listener.Close()

		// when
		method := commands.SSHAgentAuthFromSocket(socketPath)

		// then
		assert.NotNil(t, method)
	})
}

func TestDetectSSHAgentSocketsWithEnv(t *testing.T) { //nolint:tparallel // uses t.Setenv

	t.Run("should detect SSH_AUTH_SOCK when it points to a valid socket", func(t *testing.T) {
		// given
		socketPath := filepath.Join(t.TempDir(), "test.sock")
		listener, err := net.Listen("unix", socketPath)
		require.NoError(t, err)
		defer listener.Close()
		t.Setenv("SSH_AUTH_SOCK", socketPath)

		// when
		sockets := commands.DetectSSHAgentSockets()

		// then
		assert.Contains(t, sockets, socketPath)
	})

	t.Run("should not include SSH_AUTH_SOCK when it points to nonexistent path", func(t *testing.T) {
		// given
		t.Setenv("SSH_AUTH_SOCK", "/nonexistent/sock")

		// when
		sockets := commands.DetectSSHAgentSockets()

		// then
		assert.NotContains(t, sockets, "/nonexistent/sock")
	})
}

func TestCollectSSHAuthMethodsWithSocket(t *testing.T) { //nolint:tparallel // uses t.Setenv

	t.Run("should return methods from explicit SSH auth sock", func(t *testing.T) {
		// given
		socketPath := filepath.Join(t.TempDir(), "test-agent.sock")
		listener, err := net.Listen("unix", socketPath)
		require.NoError(t, err)
		defer listener.Close()

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithSSHAuthSock(socketPath).
			BuildGlobalConfig()

		// when
		methods := commands.CollectSSHAuthMethods(globalConfig)

		// then
		assert.NotEmpty(t, methods)
	})

	t.Run("should auto-detect SSH_AUTH_SOCK when no explicit config", func(t *testing.T) {
		// given
		socketPath := filepath.Join(t.TempDir(), "test.sock")
		listener, err := net.Listen("unix", socketPath)
		require.NoError(t, err)
		defer listener.Close()
		t.Setenv("SSH_AUTH_SOCK", socketPath)

		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()

		// when
		methods := commands.CollectSSHAuthMethods(globalConfig)

		// then
		assert.NotEmpty(t, methods)
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

		// then
		require.NoError(t, err)
	})

	t.Run("should handle multiple providers with mixed valid and invalid types", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()
		globalConfig.Providers = []entities.ProviderConfig{
			{Type: "nonexistent", Token: "tok1", Organizations: []string{"org1"}},
			{Type: "also-invalid", Token: "tok2", Organizations: []string{"org2"}},
		}
		registry := repositories.NewProviderRegistry()
		commands.SetProviderRegistry(registry)
		commands.SetGitOperations(gitInfra.NewGitOperations(registry))

		// when
		err := commands.DiscoverAndProcess(context.Background(), globalConfig, registry)

		// then -- errors are logged internally, function always returns nil
		require.NoError(t, err)
	})
}
