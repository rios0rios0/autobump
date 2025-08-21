package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test changelog with Fixed section to reproduce the issue
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
	// Arrange
	changelog := strings.Split(changelogWithFixed, "\n")

	// Act
	version, newChangelog, err := processChangelog(changelog)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, version)
	assert.NotNil(t, newChangelog)

	newChangelogString := strings.Join(newChangelog, "\n")
	t.Log("=== PROCESSED CHANGELOG ===")
	t.Log(newChangelogString)
	t.Log("=== END CHANGELOG ===")

	// Check if Fixed section is preserved
	assert.Contains(t, newChangelogString, "### Fixed")
	assert.Contains(t, newChangelogString, "fixed a null pointer dereference")
	assert.Contains(t, newChangelogString, "fixed SAST tool warnings")
	assert.Contains(t, newChangelogString, "fixed a typo in authentication")

	// Check the order: Fixed should come before Removed according to user requirements
	fixedIndex := strings.Index(newChangelogString, "### Fixed")
	removedIndex := strings.Index(newChangelogString, "### Removed")
	assert.True(t, fixedIndex < removedIndex, "Fixed section should come before Removed section")
}

// Test with malformed headers to see if that causes the issue
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
	// Arrange
	changelog := strings.Split(changelogWithMalformedHeaders, "\n")

	// Act
	version, newChangelog, err := processChangelog(changelog)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, version)
	assert.NotNil(t, newChangelog)

	newChangelogString := strings.Join(newChangelog, "\n")
	t.Log("=== PROCESSED CHANGELOG WITH MALFORMED HEADERS ===")
	t.Log(newChangelogString)
	t.Log("=== END CHANGELOG ===")

	// Check if Fixed section is preserved even with malformed headers
	assert.Contains(t, newChangelogString, "### Fixed")
	assert.Contains(t, newChangelogString, "fixed a null pointer dereference")
	assert.Contains(t, newChangelogString, "fixed SAST tool warnings")
}

// Test with all sections to ensure complete ordering is correct
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
	// Arrange
	changelog := strings.Split(changelogWithAllSections, "\n")

	// Act
	version, newChangelog, err := processChangelog(changelog)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, version)
	assert.NotNil(t, newChangelog)

	newChangelogString := strings.Join(newChangelog, "\n")
	t.Log("=== PROCESSED CHANGELOG WITH ALL SECTIONS ===")
	t.Log(newChangelogString)
	t.Log("=== END CHANGELOG ===")

	// Check that all sections are preserved
	assert.Contains(t, newChangelogString, "### Added")
	assert.Contains(t, newChangelogString, "### Changed")
	assert.Contains(t, newChangelogString, "### Deprecated")
	assert.Contains(t, newChangelogString, "### Fixed")
	assert.Contains(t, newChangelogString, "### Removed")
	assert.Contains(t, newChangelogString, "### Security")

	// Verify the correct order: Added, Changed, Deprecated, Fixed, Removed, Security
	addedIndex := strings.Index(newChangelogString, "### Added")
	changedIndex := strings.Index(newChangelogString, "### Changed")
	deprecatedIndex := strings.Index(newChangelogString, "### Deprecated")
	fixedIndex := strings.Index(newChangelogString, "### Fixed")
	removedIndex := strings.Index(newChangelogString, "### Removed")
	securityIndex := strings.Index(newChangelogString, "### Security")

	assert.True(t, addedIndex < changedIndex, "Added should come before Changed")
	assert.True(t, changedIndex < deprecatedIndex, "Changed should come before Deprecated")
	assert.True(t, deprecatedIndex < fixedIndex, "Deprecated should come before Fixed")
	assert.True(t, fixedIndex < removedIndex, "Fixed should come before Removed")
	assert.True(t, removedIndex < securityIndex, "Removed should come before Security")
}
