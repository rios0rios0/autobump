package entities_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

// installGitLabCIJobTokenCheck installs check as the GitLab CI job token check
// for the rest of the test, and exports a placeholder CI_JOB_TOKEN.
func installGitLabCIJobTokenCheck(t *testing.T, check func() error) {
	t.Helper()
	t.Setenv("CI_JOB_TOKEN", "fixture-ci-job-token-placeholder")
	entities.SetGitLabCIJobTokenCheck(check)
	t.Cleanup(func() { entities.SetGitLabCIJobTokenCheck(nil) })
}

// TestFinalizeGlobalConfigGitLabCIJobToken is deliberately not parallel: it sets
// CI_JOB_TOKEN with t.Setenv and installs the package-level job token check,
// which a parallel test could observe.
//
//nolint:paralleltest // uses t.Setenv and a package-level check, which must not run under t.Parallel
func TestFinalizeGlobalConfigGitLabCIJobToken(t *testing.T) {
	t.Run("should keep CI_JOB_TOKEN away from the GitLab API when the check refuses it", func(t *testing.T) {
		// given: the job runs on another GitLab server than gitlab.com
		installGitLabCIJobTokenCheck(t, func() error { return errors.New("the CI job runs on gitlab.example.com") })
		config := &entities.GlobalConfig{}

		// when
		entities.FinalizeGlobalConfig(config)

		// then
		assert.Equal(t, "fixture-ci-job-token-placeholder", config.GitLabCIJobToken,
			"git operations still send the token to the remote's own host")
		assert.Empty(t, config.GitLabAPIJobToken())
	})

	t.Run("should call the GitLab API with CI_JOB_TOKEN when the check allows it", func(t *testing.T) {
		// given
		installGitLabCIJobTokenCheck(t, func() error { return nil })
		config := &entities.GlobalConfig{}

		// when
		entities.FinalizeGlobalConfig(config)

		// then
		assert.Equal(t, "fixture-ci-job-token-placeholder", config.GitLabAPIJobToken())
	})

	t.Run("should keep CI_JOB_TOKEN away from the GitLab API when no check is installed", func(t *testing.T) {
		// given
		installGitLabCIJobTokenCheck(t, nil)
		config := &entities.GlobalConfig{}

		// when
		entities.FinalizeGlobalConfig(config)

		// then
		assert.Empty(t, config.GitLabAPIJobToken())
	})
}
