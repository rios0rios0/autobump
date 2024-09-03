package main

import (
	"testing"

	"github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/require"
)

func TestValidateGlobalConfig_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	globalConfig := GlobalConfig{
		Projects: []ProjectConfig{
			{Path: "/home/user/test", ProjectAccessToken: faker.Password()},
		},
		LanguagesConfig:   map[string]LanguageConfig{"Go": {}},
		GpgKeyPath:        "/home/user/.gnupg/autobump.asc",
		GitLabAccessToken: faker.Password(),
	}

	// Act
	err := validateGlobalConfig(&globalConfig, false)

	// Assert
	require.NoError(t, err)
}

func TestValidateGlobalConfig_MissingProjectsInBatchMode(t *testing.T) {
	t.Parallel()

	// Arrange
	globalConfig := GlobalConfig{
		LanguagesConfig: map[string]LanguageConfig{"Go": {}},
	}

	// Act
	err := validateGlobalConfig(&globalConfig, true)

	// Assert
	require.ErrorIs(t, err, ErrConfigKeyMissingError)
}

func TestValidateGlobalConfig_MissingProjectPath(t *testing.T) {
	t.Parallel()

	// Arrange
	globalConfig := GlobalConfig{
		Projects: []ProjectConfig{
			{Path: "", ProjectAccessToken: faker.Password()},
		},
		LanguagesConfig: map[string]LanguageConfig{"Go": {}},
	}

	// Act
	err := validateGlobalConfig(&globalConfig, false)

	// Assert
	require.ErrorIs(t, err, ErrConfigKeyMissingError)
}

func TestValidateGlobalConfig_MissingProjectAccessTokenInBatchMode(t *testing.T) {
	t.Parallel()

	// Arrange
	globalConfig := GlobalConfig{
		Projects: []ProjectConfig{
			{Path: faker.Word(), ProjectAccessToken: ""},
		},
		LanguagesConfig:   map[string]LanguageConfig{"Go": {}},
		GitLabAccessToken: "",
	}

	// Act
	err := validateGlobalConfig(&globalConfig, true)

	// Assert
	require.ErrorIs(t, err, ErrConfigKeyMissingError)
}

func TestValidateGlobalConfig_MissingLanguagesConfig(t *testing.T) {
	t.Parallel()

	// Arrange
	globalConfig := GlobalConfig{
		Projects: []ProjectConfig{
			{Path: faker.Word(), ProjectAccessToken: faker.Password()},
		},
		LanguagesConfig: nil,
	}

	// Act
	err := validateGlobalConfig(&globalConfig, false)

	// Assert
	require.ErrorIs(t, err, ErrLanguagesKeyMissingError)
}
