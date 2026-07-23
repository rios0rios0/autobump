//go:build unit

package commands_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
)

func TestFilterStaleBumpBranches(t *testing.T) {
	t.Parallel()

	t.Run("should select every branch carrying the bump prefix", func(t *testing.T) {
		t.Parallel()

		// given
		branches := []string{"main", "chore/bump-1.0.0", "chore/bump-1.1.0", "feat/something"}

		// when
		stale := commands.FilterStaleBumpBranches(branches, entities.DefaultBumpBranchPrefix, "main")

		// then
		assert.Equal(t, []string{"chore/bump-1.0.0", "chore/bump-1.1.0"}, stale)
	})

	t.Run("should ignore branches that do not carry the bump prefix", func(t *testing.T) {
		t.Parallel()

		// given
		branches := []string{"main", "develop", "feat/login", "fix/chore/bump-1.0.0"}

		// when
		stale := commands.FilterStaleBumpBranches(branches, entities.DefaultBumpBranchPrefix, "main")

		// then
		assert.Empty(t, stale)
	})

	t.Run("should never select the default branch", func(t *testing.T) {
		t.Parallel()

		// given a default branch that would otherwise match the prefix
		branches := []string{"chore/bump-main", "chore/bump-1.0.0"}

		// when
		stale := commands.FilterStaleBumpBranches(
			branches, entities.DefaultBumpBranchPrefix, "chore/bump-main",
		)

		// then
		assert.Equal(t, []string{"chore/bump-1.0.0"}, stale)
	})

	t.Run("should select the branch matching the version about to be recreated", func(t *testing.T) {
		t.Parallel()

		// given a leftover branch for the very version the bumper is about to create
		branches := []string{"main", "chore/bump-2.33.2"}

		// when
		stale := commands.FilterStaleBumpBranches(branches, entities.DefaultBumpBranchPrefix, "main")

		// then the branch is still removed, because it is recreated straight afterwards
		assert.Equal(t, []string{"chore/bump-2.33.2"}, stale)
	})

	t.Run("should sort the result so the cleanup order is deterministic", func(t *testing.T) {
		t.Parallel()

		// given
		branches := []string{"chore/bump-2.0.0", "chore/bump-1.0.0", "chore/bump-1.5.0"}

		// when
		stale := commands.FilterStaleBumpBranches(branches, entities.DefaultBumpBranchPrefix, "main")

		// then
		assert.Equal(t, []string{"chore/bump-1.0.0", "chore/bump-1.5.0", "chore/bump-2.0.0"}, stale)
	})

	t.Run("should honour a custom bump prefix", func(t *testing.T) {
		t.Parallel()

		// given
		branches := []string{"main", "chore/bump-1.0.0", "release/prepare-9.9.9"}

		// when
		stale := commands.FilterStaleBumpBranches(branches, "release/prepare-", "main")

		// then
		assert.Equal(t, []string{"release/prepare-9.9.9"}, stale)
	})

	t.Run("should return an empty result when there are no branches", func(t *testing.T) {
		t.Parallel()

		// given
		var branches []string

		// when
		stale := commands.FilterStaleBumpBranches(branches, entities.DefaultBumpBranchPrefix, "main")

		// then
		assert.Empty(t, stale)
	})
}
