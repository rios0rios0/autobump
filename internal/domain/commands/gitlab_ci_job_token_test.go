package commands_test

import (
	"errors"
	"testing"

	gitforgeEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/test/domain/entitybuilders"
)

// TestResolveTokenGitLabCIJobTokenServer is deliberately not parallel: it sets
// CI_JOB_TOKEN with t.Setenv and installs the package-level job token check,
// which a parallel test could observe.
func TestResolveTokenGitLabCIJobTokenServer(t *testing.T) {
	t.Run("should return no GitLab API token when the CI job token belongs to another GitLab server",
		func(t *testing.T) {
			// given
			t.Setenv("CI_JOB_TOKEN", "fixture-ci-job-token-placeholder")
			entities.SetGitLabCIJobTokenCheck(
				func() error { return errors.New("the CI job runs on gitlab.example.com") },
			)
			t.Cleanup(func() { entities.SetGitLabCIJobTokenCheck(nil) })
			globalConfig := &entities.GlobalConfig{}
			entities.FinalizeGlobalConfig(globalConfig)
			projectConfig := entitybuilders.NewProjectConfigBuilder().BuildProjectConfig()

			// when
			token := commands.ResolveToken(gitforgeEntities.GITLAB, globalConfig, projectConfig)

			// then
			assert.Empty(t, token)
		})
}
