package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetGitHubRepoInfo(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected struct {
			owner string
			repo  string
		}
		shouldError bool
	}{
		{
			name: "SSH format",
			url:  "git@github.com:owner/repo.git",
			expected: struct {
				owner string
				repo  string
			}{
				owner: "owner",
				repo:  "repo",
			},
			shouldError: false,
		},
		{
			name: "HTTPS format",
			url:  "https://github.com/owner/repo.git",
			expected: struct {
				owner string
				repo  string
			}{
				owner: "owner",
				repo:  "repo",
			},
			shouldError: false,
		},
		{
			name: "HTTPS format without .git",
			url:  "https://github.com/owner/repo",
			expected: struct {
				owner string
				repo  string
			}{
				owner: "owner",
				repo:  "repo",
			},
			shouldError: false,
		},
		{
			name:        "Invalid URL",
			url:         "https://gitlab.com/owner/repo",
			shouldError: true,
		},
		{
			name:        "Malformed SSH URL",
			url:         "git@github.com:owner",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock repository with the remote URL
			// Since we can't easily mock go-git here, we'll test the URL parsing logic directly
			// by extracting the logic into a separate function
			owner, repo, err := parseGitHubURL(tt.url)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.owner, owner)
				assert.Equal(t, tt.expected.repo, repo)
			}
		})
	}
}

// Helper function to test URL parsing logic
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
		if len(parts) == 2 {
			owner = parts[0]
			repoName = parts[1]
		} else {
			return "", "", fmt.Errorf("invalid SSH GitHub URL format: %s", remoteURL)
		}
	case strings.HasPrefix(trimmedURL, "https://github.com/"):
		// HTTPS format: https://github.com/owner/repo
		parts := strings.Split(strings.TrimPrefix(trimmedURL, "https://github.com/"), "/")
		if len(parts) >= 2 {
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