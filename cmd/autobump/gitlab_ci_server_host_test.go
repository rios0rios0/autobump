package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

// TestInstallGitLabCIServerHostReader is deliberately not parallel: it sets the
// GitLab CI variables with t.Setenv and installs the package-level reader.
func TestInstallGitLabCIServerHostReader(t *testing.T) {
	t.Run("should record the server a self-managed CI job runs on from its own CI variables", func(t *testing.T) {
		// given
		for _, name := range []string{"CI_SERVER_URL", "CI_SERVER_FQDN", "CI_API_V4_URL"} {
			t.Setenv(name, "")
		}
		t.Setenv("CI_SERVER_HOST", "gitlab.example.com")
		t.Setenv("CI_JOB_TOKEN", "fixture-ci-job-token-placeholder")
		t.Cleanup(func() { entities.SetGitLabCIServerHostReader(nil) })
		installGitLabCIServerHostReader()
		config := &entities.GlobalConfig{}

		// when
		entities.FinalizeGlobalConfig(config)

		// then
		assert.Equal(t, "gitlab.example.com", config.GitLabCIServerHost)
	})
}
