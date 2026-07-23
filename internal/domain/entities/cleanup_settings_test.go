//go:build unit

package entities_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
