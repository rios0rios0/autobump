package main

import (
	"testing"

	"github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/require"
)

func TestValidateGlobalConfig(t *testing.T) {
	t.Run("should validate successfully when all required fields are present", func(t *testing.T) {
		// given
		globalConfig := GlobalConfig{
			Projects: []ProjectConfig{
				{Path: "/home/user/test", ProjectAccessToken: faker.Password()},
			},
			LanguagesConfig:   map[string]LanguageConfig{"Go": {}},
			GpgKeyPath:        "/home/user/.gnupg/autobump.asc",
			GitLabAccessToken: faker.Password(),
		}

		// when
		err := validateGlobalConfig(&globalConfig, false)

		// then
		require.NoError(t, err, "should not return an error for valid config")
	})

	t.Run("should return error when projects are missing in batch mode", func(t *testing.T) {
		// given
		globalConfig := GlobalConfig{
			LanguagesConfig: map[string]LanguageConfig{"Go": {}},
		}

		// when
		err := validateGlobalConfig(&globalConfig, true)

		// then
		require.ErrorIs(t, err, ErrConfigKeyMissingError, "should return ErrConfigKeyMissingError")
	})

	t.Run("should return error when project path is missing", func(t *testing.T) {
		// given
		globalConfig := GlobalConfig{
			Projects: []ProjectConfig{
				{Path: "", ProjectAccessToken: faker.Password()},
			},
			LanguagesConfig: map[string]LanguageConfig{"Go": {}},
		}

		// when
		err := validateGlobalConfig(&globalConfig, false)

		// then
		require.ErrorIs(t, err, ErrConfigKeyMissingError, "should return ErrConfigKeyMissingError for missing path")
	})

	t.Run(
		"should return error when project access token is missing in batch mode without global token",
		func(t *testing.T) {
			// given
			globalConfig := GlobalConfig{
				Projects: []ProjectConfig{
					{Path: faker.Word(), ProjectAccessToken: ""},
				},
				LanguagesConfig:   map[string]LanguageConfig{"Go": {}},
				GitLabAccessToken: "",
			}

			// when
			err := validateGlobalConfig(&globalConfig, true)

			// then
			require.ErrorIs(
				t,
				err,
				ErrConfigKeyMissingError,
				"should return ErrConfigKeyMissingError for missing access token",
			)
		},
	)

	t.Run("should return error when languages config is missing", func(t *testing.T) {
		// given
		globalConfig := GlobalConfig{
			Projects: []ProjectConfig{
				{Path: faker.Word(), ProjectAccessToken: faker.Password()},
			},
			LanguagesConfig: nil,
		}

		// when
		err := validateGlobalConfig(&globalConfig, false)

		// then
		require.ErrorIs(t, err, ErrLanguagesKeyMissingError, "should return ErrLanguagesKeyMissingError")
	})
}
