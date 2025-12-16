package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const changelogTemplate = `# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]`

const changelogOriginal = changelogTemplate + `

### Added

- Another new feature.

## [1.0.1] - 1984-01-01

### Added

- New feature.`

const changelogExpected = changelogTemplate + `

## [1.1.0] - %s

### Added

- Another new feature.

## [1.0.1] - 1984-01-01

### Added

- New feature.`

func TestIsChangelogUnreleasedEmpty(t *testing.T) {
	t.Run("should return false when unreleased section has content", func(t *testing.T) {
		// given
		changelog := strings.Split(changelogOriginal, "\n")

		// when
		result, err := isChangelogUnreleasedEmpty(changelog)

		// then
		require.NoError(t, err, "should not return an error")
		assert.False(t, result, "unreleased section should not be empty")
	})

	t.Run("should return true when unreleased section is empty", func(t *testing.T) {
		// given
		changelog := strings.Split(changelogTemplate, "\n")

		// when
		result, err := isChangelogUnreleasedEmpty(changelog)

		// then
		require.NoError(t, err, "should not return an error for empty unreleased section")
		assert.True(t, result, "unreleased section should be empty")
	})

	t.Run("should return true when changelog only has unreleased header without content", func(t *testing.T) {
		// given
		changelogWithEmptyUnreleased := `# Changelog

## [Unreleased]

## [1.0.0] - 2024-01-01

### Added

- Initial release.`
		changelog := strings.Split(changelogWithEmptyUnreleased, "\n")

		// when
		result, err := isChangelogUnreleasedEmpty(changelog)

		// then
		require.NoError(t, err, "should not return an error")
		assert.True(t, result, "unreleased section should be empty when no items present")
	})
}

func TestFindLatestVersion(t *testing.T) {
	t.Run("should return the latest version from changelog", func(t *testing.T) {
		// given
		changelog := strings.Split(changelogOriginal, "\n")

		// when
		version, err := findLatestVersion(changelog)

		// then
		require.NoError(t, err, "should not return an error")
		expectedVersion, _ := semver.NewVersion("1.0.1")
		assert.Equal(t, expectedVersion, version, "should return the latest version")
	})

	t.Run("should return error when no version is found in changelog", func(t *testing.T) {
		// given
		changelog := strings.Split(changelogTemplate, "\n")

		// when
		_, err := findLatestVersion(changelog)

		// then
		require.ErrorIs(t, err, ErrNoVersionFoundInChangelog, "should return ErrNoVersionFoundInChangelog")
	})

	t.Run("should return the highest version when multiple versions exist", func(t *testing.T) {
		// given
		changelogWithMultipleVersions := `# Changelog

## [Unreleased]

## [2.0.0] - 2024-06-01

### Changed

- Major update.

## [1.5.0] - 2024-03-01

### Added

- Feature.

## [1.0.0] - 2024-01-01

### Added

- Initial release.`
		changelog := strings.Split(changelogWithMultipleVersions, "\n")

		// when
		version, err := findLatestVersion(changelog)

		// then
		require.NoError(t, err, "should not return an error")
		expectedVersion, _ := semver.NewVersion("2.0.0")
		assert.Equal(t, expectedVersion, version, "should return the highest version")
	})
}

func TestProcessChangelog(t *testing.T) {
	t.Run("should process changelog and bump minor version for added features", func(t *testing.T) {
		// given
		changelog := strings.Split(changelogOriginal, "\n")

		// when
		version, newChangelog, err := processChangelog(changelog)

		// then
		require.NoError(t, err, "should not return an error")
		expectedVersion, _ := semver.NewVersion("1.1.0")
		assert.Equal(t, expectedVersion, version, "should bump minor version")
		require.NotNil(t, newChangelog, "new changelog should not be nil")

		newChangelogString := strings.Join(newChangelog, "\n")
		expectedChangelogWithDate := fmt.Sprintf(changelogExpected, time.Now().Format("2006-01-02"))
		assert.Equal(t, expectedChangelogWithDate, newChangelogString, "changelog should match expected format")
	})

	t.Run("should return initial version 1.0.0 for new changelog without previous versions", func(t *testing.T) {
		// given
		changelogWithUnreleasedOnly := `# Changelog

## [Unreleased]

### Added

- Initial feature.`
		changelog := strings.Split(changelogWithUnreleasedOnly, "\n")

		// when
		version, newChangelog, err := processChangelog(changelog)

		// then
		require.NoError(t, err, "should not return an error for new changelog")
		expectedVersion, _ := semver.NewVersion("1.0.0")
		assert.Equal(t, expectedVersion, version, "should return initial version 1.0.0")
		require.NotNil(t, newChangelog, "new changelog should not be nil")
	})

	t.Run("should bump major version for breaking changes", func(t *testing.T) {
		// given
		changelogWithBreakingChange := `# Changelog

## [Unreleased]

### Changed

- **BREAKING CHANGE:** API completely redesigned.

## [1.0.0] - 2024-01-01

### Added

- Initial release.`
		changelog := strings.Split(changelogWithBreakingChange, "\n")

		// when
		version, _, err := processChangelog(changelog)

		// then
		require.NoError(t, err, "should not return an error")
		expectedVersion, _ := semver.NewVersion("2.0.0")
		assert.Equal(t, expectedVersion, version, "should bump major version for breaking change")
	})

	t.Run("should bump patch version for fixes", func(t *testing.T) {
		// given
		changelogWithFix := `# Changelog

## [Unreleased]

### Fixed

- Bug fix.

## [1.0.0] - 2024-01-01

### Added

- Initial release.`
		changelog := strings.Split(changelogWithFix, "\n")

		// when
		version, _, err := processChangelog(changelog)

		// then
		require.NoError(t, err, "should not return an error")
		expectedVersion, _ := semver.NewVersion("1.0.1")
		assert.Equal(t, expectedVersion, version, "should bump patch version for fix")
	})
}

func TestUpdateSection(t *testing.T) {
	t.Run("should update section and increment minor version for added items", func(t *testing.T) {
		// given
		unreleasedSection := []string{
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- New feature.",
		}
		nextVersion, _ := semver.NewVersion("1.0.0")

		// when
		newSection, updatedVersion, err := updateSection(unreleasedSection, *nextVersion)

		// then
		require.NoError(t, err, "should not return an error")
		require.NotNil(t, updatedVersion, "updated version should not be nil")
		expectedVersion, _ := semver.NewVersion("1.1.0")
		assert.Equal(t, expectedVersion, updatedVersion, "should increment minor version")
		assert.NotEmpty(t, newSection, "new section should not be empty")
	})

	t.Run("should return error when no changes found in unreleased section", func(t *testing.T) {
		// given
		unreleasedSection := []string{
			"## [Unreleased]",
			"",
		}
		nextVersion, _ := semver.NewVersion("1.0.0")

		// when
		_, _, err := updateSection(unreleasedSection, *nextVersion)

		// then
		require.ErrorIs(t, err, ErrNoChangesFoundInUnreleased, "should return ErrNoChangesFoundInUnreleased")
	})
}

func TestFixSectionHeadings(t *testing.T) {
	t.Run("should fix incorrectly formatted section headings", func(t *testing.T) {
		// given
		unreleasedSection := []string{
			"## [Unreleased]",
			"",
			"## Added",
			"",
			"- New feature.",
		}

		// when
		fixSectionHeadings(unreleasedSection)

		// then
		assert.Equal(t, "### Added", unreleasedSection[2], "should fix heading to use ###")
	})

	t.Run("should handle already correct section headings", func(t *testing.T) {
		// given
		unreleasedSection := []string{
			"## [Unreleased]",
			"",
			"### Added",
			"",
			"- New feature.",
		}

		// when
		fixSectionHeadings(unreleasedSection)

		// then
		assert.Equal(t, "### Added", unreleasedSection[2], "should keep correct heading unchanged")
	})
}
