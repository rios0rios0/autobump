package entities_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

// installGitLabCIServerHostReader installs read as the reader of the CI job's
// GitLab server for the rest of the test, and exports a placeholder CI_JOB_TOKEN.
func installGitLabCIServerHostReader(t *testing.T, read func() (string, error)) {
	t.Helper()
	t.Setenv("CI_JOB_TOKEN", "fixture-ci-job-token-placeholder")
	entities.SetGitLabCIServerHostReader(read)
	t.Cleanup(func() { entities.SetGitLabCIServerHostReader(nil) })
}

// TestFinalizeGlobalConfigGitLabCIServerHost is deliberately not parallel: it sets
// CI_JOB_TOKEN with t.Setenv and installs the package-level reader, which a
// parallel test could observe.
//
//nolint:paralleltest // uses t.Setenv and a package-level reader, which must not run under t.Parallel
func TestFinalizeGlobalConfigGitLabCIServerHost(t *testing.T) {
	t.Run("should record the GitLab server the CI job runs on when the reader names one", func(t *testing.T) {
		// given
		installGitLabCIServerHostReader(t, func() (string, error) { return "gitlab.example.com", nil })
		config := &entities.GlobalConfig{}

		// when
		entities.FinalizeGlobalConfig(config)

		// then
		assert.Equal(t, "fixture-ci-job-token-placeholder", config.GitLabCIJobToken)
		assert.Equal(t, "gitlab.example.com", config.GitLabCIServerHost)
	})

	t.Run("should record no server when the reader cannot tell which one the job runs on", func(t *testing.T) {
		// given
		installGitLabCIServerHostReader(t, func() (string, error) { return "", errors.New("no CI_SERVER_* variable") })
		config := &entities.GlobalConfig{}

		// when
		entities.FinalizeGlobalConfig(config)

		// then
		assert.Empty(t, config.GitLabCIServerHost)
	})

	t.Run("should record no server when no reader is installed", func(t *testing.T) {
		// given
		installGitLabCIServerHostReader(t, nil)
		config := &entities.GlobalConfig{}

		// when
		entities.FinalizeGlobalConfig(config)

		// then
		assert.Empty(t, config.GitLabCIServerHost)
	})
}
