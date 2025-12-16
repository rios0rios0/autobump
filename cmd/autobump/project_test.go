package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasMatchingExtension(t *testing.T) {
	t.Run("should return true when file has matching extension", func(t *testing.T) {
		// given
		extensions := []string{"txt", "md"}
		filename := "test.txt"

		// when
		result := hasMatchingExtension(filename, extensions)

		// then
		assert.True(t, result, "should find a matching extension")
	})

	t.Run("should return false when file has no matching extension", func(t *testing.T) {
		// given
		extensions := []string{"txt", "md"}
		filename := "test.go"

		// when
		result := hasMatchingExtension(filename, extensions)

		// then
		assert.False(t, result, "should not find a matching extension")
	})

	t.Run("should return false when filename has no extension", func(t *testing.T) {
		// given
		extensions := []string{"txt", "md"}
		filename := "Makefile"

		// when
		result := hasMatchingExtension(filename, extensions)

		// then
		assert.False(t, result, "should not find a matching extension for file without extension")
	})

	t.Run("should return true for md extension", func(t *testing.T) {
		// given
		extensions := []string{"txt", "md"}
		filename := "README.md"

		// when
		result := hasMatchingExtension(filename, extensions)

		// then
		assert.True(t, result, "should find md extension")
	})
}

func TestStripUsernameFromURL(t *testing.T) {
	t.Run("should strip username from Azure DevOps HTTPS URL", func(t *testing.T) {
		// given
		url := "https://user@dev.azure.com/org/project/_git/repo"

		// when
		result := stripUsernameFromURL(url)

		// then
		expected := "https://dev.azure.com/org/project/_git/repo"
		assert.Equal(t, expected, result, "should strip username from URL")
	})

	t.Run("should return URL unchanged when no username present", func(t *testing.T) {
		// given
		url := "https://dev.azure.com/org/project/_git/repo"

		// when
		result := stripUsernameFromURL(url)

		// then
		assert.Equal(t, url, result, "should return URL unchanged")
	})

	t.Run("should return SSH URL unchanged", func(t *testing.T) {
		// given
		url := "git@github.com:owner/repo.git"

		// when
		result := stripUsernameFromURL(url)

		// then
		assert.Equal(t, url, result, "should return SSH URL unchanged")
	})

	t.Run("should handle HTTP URL with username", func(t *testing.T) {
		// given
		url := "http://user@example.com/repo.git"

		// when
		result := stripUsernameFromURL(url)

		// then
		expected := "http://example.com/repo.git"
		assert.Equal(t, expected, result, "should strip username from HTTP URL")
	})

	t.Run("should return empty string unchanged", func(t *testing.T) {
		// given
		url := ""

		// when
		result := stripUsernameFromURL(url)

		// then
		assert.Empty(t, result, "should return empty string unchanged")
	})
}
