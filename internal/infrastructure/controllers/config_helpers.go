package controllers

import (
	"errors"
	"fmt"

	logger "github.com/sirupsen/logrus"

	"github.com/rios0rios0/autobump/internal/domain/entities"
	downloadHelpers "github.com/rios0rios0/gitforge/pkg/config/infrastructure/helpers"
)

// downloadDefaultConfig fetches and decodes the default autobump configuration.
func downloadDefaultConfig() (*entities.GlobalConfig, error) {
	data, err := downloadHelpers.DownloadFile(entities.DefaultConfigURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download default config: %w", err)
	}
	cfg, err := entities.DecodeConfig(data, false)
	if err != nil {
		return nil, fmt.Errorf("failed to decode default config: %w", err)
	}
	return cfg, nil
}

// findReadAndValidateConfig finds, reads and validates the config file.
func findReadAndValidateConfig(configPath string) (*entities.GlobalConfig, error) {
	configPath = entities.FindConfigOnMissing(configPath)

	globalConfig, err := entities.ReadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	defaultConfig, defaultErr := downloadDefaultConfig()
	if defaultErr != nil {
		logger.Warnf("Could not download default config: %v", defaultErr)
	}

	err = entities.ValidateGlobalConfig(globalConfig, false)
	if errors.Is(err, entities.ErrLanguagesKeyMissingError) {
		if defaultErr != nil {
			return nil, fmt.Errorf("failed to download default config: %w", defaultErr)
		}
		logger.Warn("Missing languages key, using the default configuration")
		globalConfig.LanguagesConfig = defaultConfig.LanguagesConfig
	} else if err != nil {
		return nil, fmt.Errorf("failed to validate global config: %w", err)
	} else if defaultConfig != nil {
		globalConfig.LanguagesConfig = entities.MergeLanguagesConfig(
			defaultConfig.LanguagesConfig, globalConfig.LanguagesConfig,
		)
	}

	return globalConfig, nil
}
