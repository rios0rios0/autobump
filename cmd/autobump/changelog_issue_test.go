package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test changelog with Fixed section to reproduce the issue.
const changelogWithFixed = `# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- added tests for various components

### Changed

- updated code to satisfy various golangci-lint linters

### Removed

- removed redundant release pipeline

### Fixed

- fixed a null pointer dereference when opening repositories
- fixed SAST tool warnings
- fixed a typo in authentication method selection

## [2.14.0] - 2024-03-01

### Added

- added the feature to automatically fix incorrect section heading levels`

func TestProcessChangelogWithFixedSection(t *testing.T) {
	t.Run("should preserve Fixed section when processing changelog", func(t *testing.T) {
		// given
		changelog := strings.Split(changelogWithFixed, "\n")

		// when
		version, newChangelog, err := processChangelog(changelog)

		// then
		require.NoError(t, err, "should not return an error")
		assert.NotNil(t, version, "version should not be nil")
		assert.NotNil(t, newChangelog, "new changelog should not be nil")

		newChangelogString := strings.Join(newChangelog, "\n")

		assert.Contains(t, newChangelogString, "### Fixed", "should contain Fixed section")
		assert.Contains(t, newChangelogString, "fixed a null pointer dereference", "should contain fix item 1")
		assert.Contains(t, newChangelogString, "fixed SAST tool warnings", "should contain fix item 2")
		assert.Contains(t, newChangelogString, "fixed a typo in authentication", "should contain fix item 3")
	})

	t.Run("should order Fixed section before Removed section", func(t *testing.T) {
		// given
		changelog := strings.Split(changelogWithFixed, "\n")

		// when
		_, newChangelog, err := processChangelog(changelog)

		// then
		require.NoError(t, err, "should not return an error")

		newChangelogString := strings.Join(newChangelog, "\n")
		fixedIndex := strings.Index(newChangelogString, "### Fixed")
		removedIndex := strings.Index(newChangelogString, "### Removed")

		assert.Less(t, fixedIndex, removedIndex, "Fixed section should come before Removed section")
	})
}

// Test with malformed headers to see if that causes the issue.
const changelogWithMalformedHeaders = `# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added

- added tests for various components

### Changed

- updated code to satisfy various golangci-lint linters

### Removed

- removed redundant release pipeline

## Fixed

- fixed a null pointer dereference when opening repositories
- fixed SAST tool warnings

## [2.14.0] - 2024-03-01

### Added

- added the feature to automatically fix incorrect section heading levels`

func TestProcessChangelogWithMalformedHeaders(t *testing.T) {
	t.Run("should fix malformed headers and preserve Fixed section", func(t *testing.T) {
		// given
		changelog := strings.Split(changelogWithMalformedHeaders, "\n")

		// when
		version, newChangelog, err := processChangelog(changelog)

		// then
		require.NoError(t, err, "should not return an error")
		assert.NotNil(t, version, "version should not be nil")
		assert.NotNil(t, newChangelog, "new changelog should not be nil")

		newChangelogString := strings.Join(newChangelog, "\n")

		assert.Contains(t, newChangelogString, "### Fixed", "should contain corrected Fixed section header")
		assert.Contains(t, newChangelogString, "fixed a null pointer dereference", "should contain fix item 1")
		assert.Contains(t, newChangelogString, "fixed SAST tool warnings", "should contain fix item 2")
	})

	t.Run("should correct ## Fixed to ### Fixed", func(t *testing.T) {
		// given
		changelog := strings.Split(changelogWithMalformedHeaders, "\n")

		// when
		_, newChangelog, err := processChangelog(changelog)

		// then
		require.NoError(t, err, "should not return an error")

		newChangelogString := strings.Join(newChangelog, "\n")

		// The malformed "## Fixed" should be corrected to "### Fixed"
		// Note: we check that the output contains the corrected version
		assert.Contains(t, newChangelogString, "### Fixed", "should contain corrected ### Fixed header")
	})
}

// Test with all sections to ensure complete ordering is correct.
const changelogWithAllSections = `# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added

- new feature A
- new feature B

### Changed

- changed feature C

### Deprecated

- deprecated feature D

### Removed

- removed feature E

### Fixed

- fixed bug F
- fixed bug G

### Security

- security fix H

## [1.0.0] - 2024-01-01

### Added

- initial release`

func TestProcessChangelogWithAllSections(t *testing.T) {
	t.Run("should preserve all changelog sections", func(t *testing.T) {
		// given
		changelog := strings.Split(changelogWithAllSections, "\n")

		// when
		version, newChangelog, err := processChangelog(changelog)

		// then
		require.NoError(t, err, "should not return an error")
		assert.NotNil(t, version, "version should not be nil")
		assert.NotNil(t, newChangelog, "new changelog should not be nil")

		newChangelogString := strings.Join(newChangelog, "\n")

		assert.Contains(t, newChangelogString, "### Added", "should contain Added section")
		assert.Contains(t, newChangelogString, "### Changed", "should contain Changed section")
		assert.Contains(t, newChangelogString, "### Deprecated", "should contain Deprecated section")
		assert.Contains(t, newChangelogString, "### Fixed", "should contain Fixed section")
		assert.Contains(t, newChangelogString, "### Removed", "should contain Removed section")
		assert.Contains(t, newChangelogString, "### Security", "should contain Security section")
	})

	t.Run("should maintain correct section ordering", func(t *testing.T) {
		// given
		changelog := strings.Split(changelogWithAllSections, "\n")

		// when
		_, newChangelog, err := processChangelog(changelog)

		// then
		require.NoError(t, err, "should not return an error")

		newChangelogString := strings.Join(newChangelog, "\n")

		addedIndex := strings.Index(newChangelogString, "### Added")
		changedIndex := strings.Index(newChangelogString, "### Changed")
		deprecatedIndex := strings.Index(newChangelogString, "### Deprecated")
		fixedIndex := strings.Index(newChangelogString, "### Fixed")
		removedIndex := strings.Index(newChangelogString, "### Removed")
		securityIndex := strings.Index(newChangelogString, "### Security")

		assert.Less(t, addedIndex, changedIndex, "Added should come before Changed")
		assert.Less(t, changedIndex, deprecatedIndex, "Changed should come before Deprecated")
		assert.Less(t, deprecatedIndex, fixedIndex, "Deprecated should come before Fixed")
		assert.Less(t, fixedIndex, removedIndex, "Fixed should come before Removed")
		assert.Less(t, removedIndex, securityIndex, "Removed should come before Security")
	})
}
