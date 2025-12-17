package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildPullRequestPayload(t *testing.T) {
	t.Run("should build payload with refs/heads prefix for target branch", func(t *testing.T) {
		// given
		sourceBranch := "chore/bump"
		targetBranch := "main"
		newVersion := "1.2.0"

		// when
		payload := buildPullRequestPayload(sourceBranch, targetBranch, newVersion)

		// then
		assert.Equal(t, "refs/heads/chore/bump", payload["sourceRefName"], "source ref should have refs/heads prefix")
		assert.Equal(t, "refs/heads/main", payload["targetRefName"], "target ref should have refs/heads prefix")
		assert.Equal(t, "chore(bump): bumped version to 1.2.0", payload["title"], "title should contain version")
		assert.Contains(t, payload["description"], "1.2.0", "description should contain version")
		assert.Contains(t, payload["description"], "AutoBump", "description should mention AutoBump")
	})

	t.Run("should not double prefix refs/heads on target branch", func(t *testing.T) {
		// given
		sourceBranch := "feature/test"
		targetBranch := "refs/heads/main"
		newVersion := "2.0.0"

		// when
		payload := buildPullRequestPayload(sourceBranch, targetBranch, newVersion)

		// then
		assert.Equal(t, "refs/heads/main", payload["targetRefName"], "should not double prefix refs/heads")
	})

	t.Run("should handle different version formats", func(t *testing.T) {
		// given
		sourceBranch := "release/v1"
		targetBranch := "develop"
		newVersion := "1.0.0-beta.1"

		// when
		payload := buildPullRequestPayload(sourceBranch, targetBranch, newVersion)

		// then
		assert.Equal(
			t,
			"chore(bump): bumped version to 1.0.0-beta.1",
			payload["title"],
			"should handle pre-release version",
		)
	})
}

func TestAzureDevOpsServiceAdapter_GetAuthMethods(t *testing.T) {
	t.Run("should return auth methods when Azure DevOps access token is provided", func(t *testing.T) {
		// given
		adapter := &azureDevOpsServiceAdapter{}
		globalConfig := &GlobalConfig{
			AzureDevOpsAccessToken: "test-pat-token",
		}
		projectConfig := &ProjectConfig{}

		// when
		authMethods := adapter.GetAuthMethods("", globalConfig, projectConfig)

		// then
		assert.Len(t, authMethods, 1, "should return 1 auth method")
	})

	t.Run("should return auth methods when project access token is provided", func(t *testing.T) {
		// given
		adapter := &azureDevOpsServiceAdapter{}
		globalConfig := &GlobalConfig{}
		projectConfig := &ProjectConfig{
			ProjectAccessToken: "project-token",
		}

		// when
		authMethods := adapter.GetAuthMethods("", globalConfig, projectConfig)

		// then
		assert.Len(t, authMethods, 1, "should return 1 auth method for project token")
	})

	t.Run("should return both auth methods when both tokens are provided", func(t *testing.T) {
		// given
		adapter := &azureDevOpsServiceAdapter{}
		globalConfig := &GlobalConfig{
			AzureDevOpsAccessToken: "global-token",
		}
		projectConfig := &ProjectConfig{
			ProjectAccessToken: "project-token",
		}

		// when
		authMethods := adapter.GetAuthMethods("", globalConfig, projectConfig)

		// then
		assert.Len(t, authMethods, 2, "should return 2 auth methods")
	})

	t.Run("should return empty when no tokens are provided", func(t *testing.T) {
		// given
		adapter := &azureDevOpsServiceAdapter{}
		globalConfig := &GlobalConfig{}
		projectConfig := &ProjectConfig{}

		// when
		authMethods := adapter.GetAuthMethods("", globalConfig, projectConfig)

		// then
		assert.Empty(t, authMethods, "should return empty auth methods")
	})
}
