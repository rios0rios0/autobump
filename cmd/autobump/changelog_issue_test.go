package main

import (
	"fmt"
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
	fmt.Println("=== PROCESSED CHANGELOG ===")
	fmt.Println(newChangelogString)
	fmt.Println("=== END CHANGELOG ===")

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
	fmt.Println("=== PROCESSED CHANGELOG WITH MALFORMED HEADERS ===")
	fmt.Println(newChangelogString)
	fmt.Println("=== END CHANGELOG ===")

	// Check if Fixed section is preserved even with malformed headers
	assert.Contains(t, newChangelogString, "### Fixed")
	assert.Contains(t, newChangelogString, "fixed a null pointer dereference")
	assert.Contains(t, newChangelogString, "fixed SAST tool warnings")
}