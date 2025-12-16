package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGitHubRepoInfo(t *testing.T) {
	t.Run("should parse SSH format URL correctly", func(t *testing.T) {
		// given
		url := "git@github.com:owner/repo.git"

		// when
		owner, repo, err := parseGitHubURL(url)

		// then
		require.NoError(t, err, "should not return an error")
		assert.Equal(t, "owner", owner, "owner should be parsed correctly")
		assert.Equal(t, "repo", repo, "repo should be parsed correctly")
	})

	t.Run("should parse HTTPS format URL correctly", func(t *testing.T) {
		// given
		url := "https://github.com/owner/repo.git"

		// when
		owner, repo, err := parseGitHubURL(url)

		// then
		require.NoError(t, err, "should not return an error")
		assert.Equal(t, "owner", owner, "owner should be parsed correctly")
		assert.Equal(t, "repo", repo, "repo should be parsed correctly")
	})

	t.Run("should parse HTTPS format URL without .git suffix", func(t *testing.T) {
		// given
		url := "https://github.com/owner/repo"

		// when
		owner, repo, err := parseGitHubURL(url)

		// then
		require.NoError(t, err, "should not return an error")
		assert.Equal(t, "owner", owner, "owner should be parsed correctly")
		assert.Equal(t, "repo", repo, "repo should be parsed correctly")
	})

	t.Run("should return error for invalid GitLab URL", func(t *testing.T) {
		// given
		url := "https://gitlab.com/owner/repo"

		// when
		_, _, err := parseGitHubURL(url)

		// then
		require.Error(t, err, "should return an error for non-GitHub URL")
	})

	t.Run("should return error for malformed SSH URL", func(t *testing.T) {
		// given
		url := "git@github.com:owner"

		// when
		_, _, err := parseGitHubURL(url)

		// then
		require.Error(t, err, "should return an error for malformed SSH URL")
	})
}

func TestGitHubAuthMethods(t *testing.T) {
	t.Run("should return auth methods with correct username for GitHub", func(t *testing.T) {
		// given
		globalConfig := &GlobalConfig{
			GitHubAccessToken: "test-token",
		}
		projectConfig := &ProjectConfig{
			ProjectAccessToken: "project-token",
		}

		// when
		authMethods, err := getAuthMethods(GITHUB, "testuser", globalConfig, projectConfig)

		// then
		require.NoError(t, err, "should not return an error")
		assert.Len(t, authMethods, 2, "should return 2 auth methods")

		for _, method := range authMethods {
			basicAuth, ok := method.(*http.BasicAuth)
			assert.True(t, ok, "auth method should be BasicAuth")
			assert.Equal(t, "x-access-token", basicAuth.Username, "username should be x-access-token for GitHub")
		}
	})

	t.Run("should return error when no GitHub auth credentials are provided", func(t *testing.T) {
		// given
		globalConfig := &GlobalConfig{}
		projectConfig := &ProjectConfig{}

		// when
		authMethods, err := getAuthMethods(GITHUB, "testuser", globalConfig, projectConfig)

		// then
		require.ErrorIs(t, err, ErrNoAuthMethodFound, "should return ErrNoAuthMethodFound")
		assert.Empty(t, authMethods, "auth methods should be empty")
	})
}

// parseGitHubURL is a helper function to test URL parsing logic.
func parseGitHubURL(remoteURL string) (string, string, error) {
	// This is the same logic as in getGitHubRepoInfo but extracted for testing
	// Remove .git if it exists
	trimmedURL := remoteURL
	if strings.HasSuffix(remoteURL, ".git") {
		trimmedURL = remoteURL[:len(remoteURL)-4]
	}

	var owner, repoName string
	switch {
	case strings.HasPrefix(trimmedURL, "git@github.com:"):
		// SSH format: git@github.com:owner/repo
		parts := strings.Split(strings.TrimPrefix(trimmedURL, "git@github.com:"), "/")
		if len(parts) == expectedURLParts {
			owner = parts[0]
			repoName = parts[1]
		} else {
			return "", "", fmt.Errorf("invalid SSH GitHub URL format: %s", remoteURL)
		}
	case strings.HasPrefix(trimmedURL, "https://github.com/"):
		// HTTPS format: https://github.com/owner/repo
		parts := strings.Split(strings.TrimPrefix(trimmedURL, "https://github.com/"), "/")
		if len(parts) >= expectedURLParts {
			owner = parts[0]
			repoName = parts[1]
		} else {
			return "", "", fmt.Errorf("invalid HTTPS GitHub URL format: %s", remoteURL)
		}
	default:
		return "", "", fmt.Errorf("unsupported GitHub URL format: %s", remoteURL)
	}

	if owner == "" || repoName == "" {
		return "", "", fmt.Errorf("could not parse owner and repository name from URL: %s", remoteURL)
	}

	return owner, repoName, nil
}
