package main

import (
	"testing"

	"github.com/go-faker/faker/v4"
)

func TestHasMatchingExtension_True(t *testing.T) {
	t.Parallel()

	// Arrange
	extensions := []string{"txt", "md"}

	// Act & Assert
	if !hasMatchingExtension("test.txt", extensions) {
		t.Error("Expected to find a matching extension")
	}
}

func TestHasMatchingExtension_False(t *testing.T) {
	t.Parallel()

	// Arrange
	extensions := []string{"txt", "md"}

	// Act & Assert
	if hasMatchingExtension(faker.Word(), extensions) {
		t.Error("Expected to not find a matching extension")
	}
}
