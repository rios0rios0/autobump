package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGitServiceRegistry(t *testing.T) {
	t.Run("should create registry with all adapters pre-registered", func(t *testing.T) {
		// given & when
		registry := NewGitServiceRegistry()

		// then
		require.NotNil(t, registry, "registry should not be nil")
		assert.Len(t, registry.adapters, 3, "registry should have 3 adapters pre-registered")
	})

	t.Run("should include GitLab adapter in registry", func(t *testing.T) {
		// given
		registry := NewGitServiceRegistry()

		// when
		adapter := registry.GetAdapterByServiceType(GITLAB)

		// then
		require.NotNil(t, adapter, "GitLab adapter should be registered")
		assert.Equal(t, GITLAB, adapter.GetServiceType(), "adapter should be GitLab type")
	})
}

func TestGitServiceRegistry_Register(t *testing.T) {
	t.Run("should add new adapter to registry", func(t *testing.T) {
		// given
		registry := NewGitServiceRegistry()
		initialCount := len(registry.adapters)
		newAdapter := &gitLabServiceAdapter{}

		// when
		registry.Register(newAdapter)

		// then
		assert.Len(t, registry.adapters, initialCount+1, "registry should have one more adapter")
	})

	t.Run("should allow registering multiple adapters", func(t *testing.T) {
		// given
		registry := &GitServiceRegistry{}

		// when
		registry.Register(&gitLabServiceAdapter{})
		registry.Register(&gitHubServiceAdapter{})

		// then
		assert.Len(t, registry.adapters, 2, "registry should have 2 adapters")
	})
}

func TestGitServiceRegistry_GetAdapterByURL(t *testing.T) {
	t.Run("should return GitLab adapter for gitlab.com URL", func(t *testing.T) {
		// given
		registry := NewGitServiceRegistry()
		url := "https://gitlab.com/org/repo.git"

		// when
		adapter := registry.GetAdapterByURL(url)

		// then
		require.NotNil(t, adapter, "adapter should not be nil")
		assert.Equal(t, GITLAB, adapter.GetServiceType(), "should return GitLab adapter")
	})

	t.Run("should return GitHub adapter for github.com URL", func(t *testing.T) {
		// given
		registry := NewGitServiceRegistry()
		url := "https://github.com/org/repo.git"

		// when
		adapter := registry.GetAdapterByURL(url)

		// then
		require.NotNil(t, adapter, "adapter should not be nil")
		assert.Equal(t, GITHUB, adapter.GetServiceType(), "should return GitHub adapter")
	})

	t.Run("should return Azure DevOps adapter for dev.azure.com URL", func(t *testing.T) {
		// given
		registry := NewGitServiceRegistry()
		url := "https://dev.azure.com/org/project/_git/repo"

		// when
		adapter := registry.GetAdapterByURL(url)

		// then
		require.NotNil(t, adapter, "adapter should not be nil")
		assert.Equal(t, AZUREDEVOPS, adapter.GetServiceType(), "should return Azure DevOps adapter")
	})

	t.Run("should return nil for unknown URL", func(t *testing.T) {
		// given
		registry := NewGitServiceRegistry()
		url := "https://unknown.com/org/repo.git"

		// when
		adapter := registry.GetAdapterByURL(url)

		// then
		assert.Nil(t, adapter, "adapter should be nil for unknown URL")
	})
}

func TestGitServiceRegistry_GetAdapterByServiceType(t *testing.T) {
	t.Run("should return correct adapter for GITHUB service type", func(t *testing.T) {
		// given
		registry := NewGitServiceRegistry()

		// when
		adapter := registry.GetAdapterByServiceType(GITHUB)

		// then
		require.NotNil(t, adapter, "adapter should not be nil")
		assert.Equal(t, GITHUB, adapter.GetServiceType(), "should return GitHub adapter")
	})

	t.Run("should return nil for UNKNOWN service type", func(t *testing.T) {
		// given
		registry := NewGitServiceRegistry()

		// when
		adapter := registry.GetAdapterByServiceType(UNKNOWN)

		// then
		assert.Nil(t, adapter, "adapter should be nil for unknown service type")
	})
}

func TestGetAdapterByURL(t *testing.T) {
	t.Run("should return adapter using default registry for valid URL", func(t *testing.T) {
		// given
		url := "https://github.com/org/repo.git"

		// when
		adapter := GetAdapterByURL(url)

		// then
		require.NotNil(t, adapter, "adapter should not be nil")
		assert.Equal(t, GITHUB, adapter.GetServiceType(), "should return GitHub adapter")
	})

	t.Run("should return nil for unknown URL using default registry", func(t *testing.T) {
		// given
		url := "https://bitbucket.org/org/repo.git"

		// when
		adapter := GetAdapterByURL(url)

		// then
		assert.Nil(t, adapter, "adapter should be nil for unsupported URL")
	})
}

func TestGetAdapterByServiceType(t *testing.T) {
	t.Run("should return adapter using default registry for valid service type", func(t *testing.T) {
		// given
		serviceType := GITLAB

		// when
		adapter := GetAdapterByServiceType(serviceType)

		// then
		require.NotNil(t, adapter, "adapter should not be nil")
		assert.Equal(t, GITLAB, adapter.GetServiceType(), "should return GitLab adapter")
	})

	t.Run("should return nil for unsupported service type using default registry", func(t *testing.T) {
		// given
		serviceType := BITBUCKET

		// when
		adapter := GetAdapterByServiceType(serviceType)

		// then
		assert.Nil(t, adapter, "adapter should be nil for unsupported service type")
	})
}

func TestGitLabServiceAdapter(t *testing.T) {
	t.Run("should return GITLAB service type", func(t *testing.T) {
		// given
		adapter := &gitLabServiceAdapter{}

		// when
		serviceType := adapter.GetServiceType()

		// then
		assert.Equal(t, GITLAB, serviceType, "should return GITLAB service type")
	})

	t.Run("should match gitlab.com URLs", func(t *testing.T) {
		// given
		adapter := &gitLabServiceAdapter{}

		// when & then
		assert.True(t, adapter.MatchesURL("https://gitlab.com/org/repo.git"), "should match gitlab.com URL")
		assert.False(t, adapter.MatchesURL("https://github.com/org/repo.git"), "should not match github.com URL")
	})

	t.Run("should return URL unchanged from PrepareCloneURL", func(t *testing.T) {
		// given
		adapter := &gitLabServiceAdapter{}
		url := "https://gitlab.com/org/repo.git"

		// when
		result := adapter.PrepareCloneURL(url)

		// then
		assert.Equal(t, url, result, "URL should remain unchanged")
	})
}

func TestAzureDevOpsServiceAdapter(t *testing.T) {
	t.Run("should return AZUREDEVOPS service type", func(t *testing.T) {
		// given
		adapter := &azureDevOpsServiceAdapter{}

		// when
		serviceType := adapter.GetServiceType()

		// then
		assert.Equal(t, AZUREDEVOPS, serviceType, "should return AZUREDEVOPS service type")
	})

	t.Run("should match dev.azure.com URLs", func(t *testing.T) {
		// given
		adapter := &azureDevOpsServiceAdapter{}

		// when & then
		assert.True(
			t,
			adapter.MatchesURL("https://dev.azure.com/org/project/_git/repo"),
			"should match dev.azure.com URL",
		)
		assert.False(t, adapter.MatchesURL("https://github.com/org/repo.git"), "should not match github.com URL")
	})

	t.Run("should strip username from Azure DevOps URL", func(t *testing.T) {
		// given
		adapter := &azureDevOpsServiceAdapter{}
		urlWithUsername := "https://user@dev.azure.com/org/project/_git/repo"

		// when
		result := adapter.PrepareCloneURL(urlWithUsername)

		// then
		expected := "https://dev.azure.com/org/project/_git/repo"
		assert.Equal(t, expected, result, "should strip username from URL")
	})
}

func TestGitHubServiceAdapter(t *testing.T) {
	t.Run("should return GITHUB service type", func(t *testing.T) {
		// given
		adapter := &gitHubServiceAdapter{}

		// when
		serviceType := adapter.GetServiceType()

		// then
		assert.Equal(t, GITHUB, serviceType, "should return GITHUB service type")
	})

	t.Run("should match github.com URLs", func(t *testing.T) {
		// given
		adapter := &gitHubServiceAdapter{}

		// when & then
		assert.True(t, adapter.MatchesURL("https://github.com/org/repo.git"), "should match github.com URL")
		assert.False(t, adapter.MatchesURL("https://gitlab.com/org/repo.git"), "should not match gitlab.com URL")
	})

	t.Run("should return URL unchanged from PrepareCloneURL", func(t *testing.T) {
		// given
		adapter := &gitHubServiceAdapter{}
		url := "https://github.com/org/repo.git"

		// when
		result := adapter.PrepareCloneURL(url)

		// then
		assert.Equal(t, url, result, "URL should remain unchanged")
	})
}

func TestGitServiceAdapterImplementsInterface(t *testing.T) {
	t.Run("should verify gitLabServiceAdapter implements GitServiceAdapter interface", func(t *testing.T) {
		// given
		var adapter GitServiceAdapter = &gitLabServiceAdapter{}

		// when
		serviceType := adapter.GetServiceType()

		// then
		assert.Equal(t, GITLAB, serviceType, "gitLabServiceAdapter should implement GitServiceAdapter")
	})

	t.Run("should verify azureDevOpsServiceAdapter implements GitServiceAdapter interface", func(t *testing.T) {
		// given
		var adapter GitServiceAdapter = &azureDevOpsServiceAdapter{}

		// when
		serviceType := adapter.GetServiceType()

		// then
		assert.Equal(t, AZUREDEVOPS, serviceType, "azureDevOpsServiceAdapter should implement GitServiceAdapter")
	})

	t.Run("should verify gitHubServiceAdapter implements GitServiceAdapter interface", func(t *testing.T) {
		// given
		var adapter GitServiceAdapter = &gitHubServiceAdapter{}

		// when
		serviceType := adapter.GetServiceType()

		// then
		assert.Equal(t, GITHUB, serviceType, "gitHubServiceAdapter should implement GitServiceAdapter")
	})
}
