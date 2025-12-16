package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPullRequestProvider(t *testing.T) {
	t.Run("should return gitHubServiceAdapter for GITHUB service type", func(t *testing.T) {
		// given
		serviceType := GITHUB

		// when
		provider := NewPullRequestProvider(serviceType)

		// then
		require.NotNil(t, provider, "provider should not be nil")
		_, ok := provider.(*gitHubServiceAdapter)
		assert.True(t, ok, "provider should be of type *gitHubServiceAdapter")
	})

	t.Run("should return gitLabServiceAdapter for GITLAB service type", func(t *testing.T) {
		// given
		serviceType := GITLAB

		// when
		provider := NewPullRequestProvider(serviceType)

		// then
		require.NotNil(t, provider, "provider should not be nil")
		_, ok := provider.(*gitLabServiceAdapter)
		assert.True(t, ok, "provider should be of type *gitLabServiceAdapter")
	})

	t.Run("should return azureDevOpsServiceAdapter for AZUREDEVOPS service type", func(t *testing.T) {
		// given
		serviceType := AZUREDEVOPS

		// when
		provider := NewPullRequestProvider(serviceType)

		// then
		require.NotNil(t, provider, "provider should not be nil")
		_, ok := provider.(*azureDevOpsServiceAdapter)
		assert.True(t, ok, "provider should be of type *azureDevOpsServiceAdapter")
	})

	t.Run("should return nil for UNKNOWN service type", func(t *testing.T) {
		// given
		serviceType := UNKNOWN

		// when
		provider := NewPullRequestProvider(serviceType)

		// then
		assert.Nil(t, provider, "provider should be nil for unknown service type")
	})

	t.Run("should return nil for BITBUCKET service type", func(t *testing.T) {
		// given
		serviceType := BITBUCKET

		// when
		provider := NewPullRequestProvider(serviceType)

		// then
		assert.Nil(t, provider, "provider should be nil for unsupported service type")
	})
}

func TestPullRequestProviderImplementsInterface(t *testing.T) {
	t.Run("should verify GitHubAdapter implements PullRequestProvider interface", func(t *testing.T) {
		// given
		var provider PullRequestProvider = &GitHubAdapter{}

		// when & then
		// If compilation succeeds and provider is not nil, the interface is implemented
		require.NotNil(t, provider, "GitHubAdapter should implement PullRequestProvider")
	})

	t.Run("should verify GitLabAdapter implements PullRequestProvider interface", func(t *testing.T) {
		// given
		var provider PullRequestProvider = &GitLabAdapter{}

		// when & then
		// If compilation succeeds and provider is not nil, the interface is implemented
		require.NotNil(t, provider, "GitLabAdapter should implement PullRequestProvider")
	})

	t.Run("should verify AzureDevOpsAdapter implements PullRequestProvider interface", func(t *testing.T) {
		// given
		var provider PullRequestProvider = &AzureDevOpsAdapter{}

		// when & then
		// If compilation succeeds and provider is not nil, the interface is implemented
		require.NotNil(t, provider, "AzureDevOpsAdapter should implement PullRequestProvider")
	})
}
