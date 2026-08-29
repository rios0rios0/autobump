package configs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/configs"
	"github.com/rios0rios0/autobump/internal/domain/entities"
)

// TestEmbeddedDefaults guards the one layer that cannot fail gracefully.
//
// The built-in defaults are the base of every run, are compiled into the binary, and are
// the same bytes served from `main` to every installed binary. A mistake in that file is
// not a configuration problem an operator can fix -- it ships. So the shape it must keep
// is asserted here rather than trusted.
func TestEmbeddedDefaults(t *testing.T) {
	t.Parallel()

	//nolint:exhaustruct // the built-in defaults have no origin an operator can look at
	layer := entities.ConfigLayer{
		Name:   entities.LayerBuiltInDefaults,
		Data:   configs.Default,
		Scope:  entities.ScopeOperator,
		Strict: true,
	}

	t.Run("should decode strictly", func(t *testing.T) {
		t.Parallel()

		// when -- ScopeOperator + Strict is the harshest reading of the document there is,
		// so a key this binary does not know is caught here rather than in the field
		config, err := entities.ApplyLayer(&entities.GlobalConfig{}, layer)

		// then
		require.NoError(t, err)
		require.NotNil(t, config)
	})

	t.Run("should carry no credentials", func(t *testing.T) {
		t.Parallel()

		// given
		config, err := entities.ApplyLayer(&entities.GlobalConfig{}, layer)
		require.NoError(t, err)

		// then -- a placeholder token here would become the *effective* token for every
		// operator who keeps none of their own, and would shadow CI_JOB_TOKEN besides
		assert.Empty(t, config.GitLabAccessToken)
		assert.Empty(t, config.GitHubAccessToken)
		assert.Empty(t, config.AzureDevOpsAccessToken)
		assert.Empty(t, config.GpgKeyPath)
		assert.Empty(t, config.GpgKeyPassphrase)
		assert.Empty(t, config.SSHKeyPath)
		assert.Empty(t, config.SSHKeyPassphrase)
		assert.Empty(t, config.SSHAuthSock)
	})

	t.Run("should name no repositories to release", func(t *testing.T) {
		t.Parallel()

		// given
		config, err := entities.ApplyLayer(&entities.GlobalConfig{}, layer)
		require.NoError(t, err)

		// then -- an example `projects` list here would make `autobump run` walk paths
		// that belong to the documentation, and a `providers` block would make it scan an
		// organisation nobody asked for
		assert.Empty(t, config.Projects)
		assert.Empty(t, config.Providers)
	})

	t.Run("should aim no branch deletion", func(t *testing.T) {
		t.Parallel()

		// given
		config, err := entities.ApplyLayer(&entities.GlobalConfig{}, layer)
		require.NoError(t, err)

		// then
		assert.Empty(t, config.BumpBranchPrefix,
			"the prefix decides what stale-branch cleanup deletes, so the shipped defaults "+
				"must leave it to the operator")
	})

	t.Run("should describe every language it ships", func(t *testing.T) {
		t.Parallel()

		// given
		config, err := entities.ApplyLayer(&entities.GlobalConfig{}, layer)
		require.NoError(t, err)

		// then
		require.NotEmpty(t, config.LanguagesConfig)
		for name, language := range config.LanguagesConfig {
			assert.NotEmptyf(t, language.Extensions,
				"language %q must say how to detect a project by extension", name)
		}
	})

	t.Run("should keep the refresh opt-in", func(t *testing.T) {
		t.Parallel()

		// given
		config, err := entities.ApplyLayer(&entities.GlobalConfig{}, layer)
		require.NoError(t, err)

		// then -- the refresh starts a package manager, so upgrading AutoBump must never
		// be what turns it on
		assert.Nil(t, config.Refresh)
		for name, language := range config.LanguagesConfig {
			assert.Nilf(t, language.Refresh, "language %q must not enable the refresh", name)
		}
	})
}
