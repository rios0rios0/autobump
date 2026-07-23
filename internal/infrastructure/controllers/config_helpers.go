package controllers

import (
	"errors"
	"fmt"

	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/rios0rios0/autobump/internal/domain/entities"
	downloadHelpers "github.com/rios0rios0/gitforge/pkg/config/infrastructure/helpers"
)

// applySkipCleanupFlag turns off stale bump-branch cleanup when --skip-cleanup is set.
// The flag is a per-run override, so it wins over the configuration file; without it the
// configured value stands, and cleanup stays enabled when nothing is configured at all.
func applySkipCleanupFlag(cmd *cobra.Command, globalConfig *entities.GlobalConfig) {
	skipCleanup, _ := cmd.Flags().GetBool("skip-cleanup")
	if !skipCleanup {
		return
	}

	disabled := false
	globalConfig.CleanupStaleBranches = &disabled
	logger.Info("Stale bump branch cleanup is disabled for this run by --skip-cleanup")
}

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

	var defaultConfig *entities.GlobalConfig
	var defaultErr error
	if configPath != entities.DefaultConfigURL {
		defaultConfig, defaultErr = downloadDefaultConfig()
		if defaultErr != nil {
			logger.Warnf("Could not download default config: %v", defaultErr)
		}
	}

	err = entities.ValidateGlobalConfig(globalConfig, false)
	switch {
	case errors.Is(err, entities.ErrLanguagesKeyMissingError):
		if defaultErr != nil {
			return nil, defaultErr
		}
		logger.Warn("Missing languages key, using the default configuration")
		globalConfig.LanguagesConfig = defaultConfig.LanguagesConfig
	case err != nil:
		return nil, fmt.Errorf("failed to validate global config: %w", err)
	case defaultConfig != nil:
		globalConfig.LanguagesConfig = entities.MergeLanguagesConfig(
			defaultConfig.LanguagesConfig, globalConfig.LanguagesConfig,
		)
	}

	return globalConfig, nil
}
