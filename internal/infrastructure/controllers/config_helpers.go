package controllers

import (
	"errors"
	"fmt"

	logger "github.com/sirupsen/logrus"

	"github.com/rios0rios0/autobump/internal/domain/entities"
	downloadHelpers "github.com/rios0rios0/gitforge/pkg/config/infrastructure/helpers"
)

// findReadAndValidateConfig finds, reads and validates the config file.
func findReadAndValidateConfig(configPath string) (*entities.GlobalConfig, error) {
	configPath = entities.FindConfigOnMissing(configPath)

	globalConfig, err := entities.ReadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	err = entities.ValidateGlobalConfig(globalConfig, false)
	if errors.Is(err, entities.ErrLanguagesKeyMissingError) {
		logger.Warn("Missing languages key, using the default configuration")

		var data []byte
		data, err = downloadHelpers.DownloadFile(entities.DefaultConfigURL)
		if err != nil {
			return nil, fmt.Errorf("failed to download default config: %w", err)
		}

		var defaultConfig *entities.GlobalConfig
		defaultConfig, err = entities.DecodeConfig(data, false)
		if err != nil {
			return nil, fmt.Errorf("failed to decode default config: %w", err)
		}

		globalConfig.LanguagesConfig = defaultConfig.LanguagesConfig
	} else if err != nil {
		return nil, fmt.Errorf("failed to validate global config: %w", err)
	}

	return globalConfig, nil
}
