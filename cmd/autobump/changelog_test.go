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

func TestIsChangelogUnreleasedEmpty_False(t *testing.T) {
	t.Parallel()

	// Arrange
	changelog := strings.Split(changelogOriginal, "\n")

	// Act
	result, err := isChangelogUnreleasedEmpty(changelog)

	// Assert
	require.NoError(t, err)
	assert.False(t, result)
}

func TestIsChangelogUnreleasedEmpty_True(t *testing.T) {
	t.Parallel()

	// Arrange
	changelog := strings.Split(changelogTemplate, "\n")

	// Act
	result, err := isChangelogUnreleasedEmpty(changelog)

	// Assert
	require.ErrorIs(t, err, ErrNoVersionFoundInChangelog)
	assert.True(t, result)
}

func TestFindLatestVersion_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	changelog := strings.Split(changelogOriginal, "\n")

	// Act
	version, err := findLatestVersion(changelog)

	// Assert
	require.NoError(t, err)

	expectedVersion, err := semver.NewVersion("1.0.1")
	require.NoError(t, err)

	assert.Equal(t, expectedVersion, version)
}

func TestFindLatestVersion_NoPreviousVersions(t *testing.T) {
	t.Parallel()

	// Arrange
	changelog := strings.Split(changelogTemplate, "\n")

	// Act
	_, err := findLatestVersion(changelog)

	// Assert
	require.ErrorIs(t, err, ErrNoVersionFoundInChangelog)
}

func TestProcessChangelog_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	changelog := strings.Split(changelogOriginal, "\n")

	// Act
	version, newChangelog, err := processChangelog(changelog)

	// Assert
	require.NoError(t, err)

	expectedVersion, err := semver.NewVersion("1.1.0")
	require.NoError(t, err)

	assert.Equal(t, expectedVersion, version)
	assert.NotNil(t, newChangelog)

	newChangelogString := strings.Join(newChangelog, "\n")
	expectedChangelogWithDate := fmt.Sprintf(changelogExpected, time.Now().Format("2006-01-02"))

	assert.Equal(t, expectedChangelogWithDate, newChangelogString)
}

func TestProcessChangelog_NoPreviousVersions(t *testing.T) {
	t.Parallel()

	// Arrange
	changelog := strings.Split(changelogTemplate, "\n")

	// Act
	_, _, err := processChangelog(changelog)

	// Assert
	require.ErrorIs(t, err, ErrNoVersionFoundInChangelog)
}
