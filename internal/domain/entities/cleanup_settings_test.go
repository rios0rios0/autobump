//go:build unit

package entities_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/test/domain/entitybuilders"
)

func TestCleanupEnabled(t *testing.T) {
	t.Parallel()

	t.Run("should be enabled when the setting is absent", func(t *testing.T) {
		t.Parallel()

		// given cleanup is opt-out, so an untouched config leaves it on
		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()

		// when
		enabled := entities.CleanupEnabled(globalConfig)

		// then
		assert.True(t, enabled)
	})

	t.Run("should be disabled when explicitly turned off", func(t *testing.T) {
		t.Parallel()

		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithCleanupStaleBranches(false).
			BuildGlobalConfig()

		// when
		enabled := entities.CleanupEnabled(globalConfig)

		// then
		assert.False(t, enabled)
	})

	t.Run("should be enabled when explicitly turned on", func(t *testing.T) {
		t.Parallel()

		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithCleanupStaleBranches(true).
			BuildGlobalConfig()

		// when
		enabled := entities.CleanupEnabled(globalConfig)

		// then
		assert.True(t, enabled)
	})

	t.Run("should be enabled when the configuration is missing entirely", func(t *testing.T) {
		t.Parallel()

		// given
		var globalConfig *entities.GlobalConfig

		// when
		enabled := entities.CleanupEnabled(globalConfig)

		// then
		assert.True(t, enabled)
	})
}

func TestGlobalConfigBuilderCloneIsIndependent(t *testing.T) {
	t.Parallel()

	t.Run("should give the clone its own cleanup toggle", func(t *testing.T) {
		t.Parallel()

		// given
		original := entitybuilders.NewGlobalConfigBuilder().WithCleanupStaleBranches(false)
		clone, ok := original.Clone().(*entitybuilders.GlobalConfigBuilder)
		require.True(t, ok)

		// when the clone is re-pointed at a different value
		clone.WithCleanupStaleBranches(true)

		// then the original keeps its own, rather than sharing one bool
		assert.False(t, entities.CleanupEnabled(original.BuildGlobalConfig()))
		assert.True(t, entities.CleanupEnabled(clone.BuildGlobalConfig()))
	})
}

func TestResolveBumpBranchPrefix(t *testing.T) {
	t.Parallel()

	t.Run("should return the default prefix when none is configured", func(t *testing.T) {
		t.Parallel()

		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig()

		// when
		prefix := entities.ResolveBumpBranchPrefix(globalConfig)

		// then
		assert.Equal(t, "chore/bump-", prefix)
	})

	t.Run("should return the configured prefix", func(t *testing.T) {
		t.Parallel()

		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithBumpBranchPrefix("release/prepare-").
			BuildGlobalConfig()

		// when
		prefix := entities.ResolveBumpBranchPrefix(globalConfig)

		// then
		assert.Equal(t, "release/prepare-", prefix)
	})

	t.Run("should fall back to the default when the configured prefix is blank", func(t *testing.T) {
		t.Parallel()

		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithBumpBranchPrefix("   ").
			BuildGlobalConfig()

		// when
		prefix := entities.ResolveBumpBranchPrefix(globalConfig)

		// then
		assert.Equal(t, entities.DefaultBumpBranchPrefix, prefix)
	})

	t.Run("should return the default prefix when the configuration is missing", func(t *testing.T) {
		t.Parallel()

		// given
		var globalConfig *entities.GlobalConfig

		// when
		prefix := entities.ResolveBumpBranchPrefix(globalConfig)

		// then
		assert.Equal(t, entities.DefaultBumpBranchPrefix, prefix)
	})
}
