package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPullRequestProvider(t *testing.T) {
	tests := []struct {
		name        string
		serviceType ServiceType
		expectType  string
		expectNil   bool
	}{
		{
			name:        "GitHub provider",
			serviceType: GITHUB,
			expectType:  "*main.GitHubAdapter",
			expectNil:   false,
		},
		{
			name:        "GitLab provider",
			serviceType: GITLAB,
			expectType:  "*main.GitLabAdapter",
			expectNil:   false,
		},
		{
			name:        "Azure DevOps provider",
			serviceType: AZUREDEVOPS,
			expectType:  "*main.AzureDevOpsAdapter",
			expectNil:   false,
		},
		{
			name:        "Unknown provider",
			serviceType: UNKNOWN,
			expectType:  "",
			expectNil:   true,
		},
		{
			name:        "Unsupported provider",
			serviceType: BITBUCKET,
			expectType:  "",
			expectNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewPullRequestProvider(tt.serviceType)

			if tt.expectNil {
				assert.Nil(t, provider)
			} else {
				require.NotNil(t, provider)
				assert.Equal(t, tt.expectType, fmt.Sprintf("%T", provider))
			}
		})
	}
}

func TestPullRequestProviderImplementsInterface(t *testing.T) {
	// Test that all adapters implement the PullRequestProvider interface
	var _ PullRequestProvider = &GitHubAdapter{}
	var _ PullRequestProvider = &GitLabAdapter{}
	var _ PullRequestProvider = &AzureDevOpsAdapter{}
}
