package controllers

import (
	"errors"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/internal/support"
)

// SingleController handles the root "autobump" command (single repo mode).
type SingleController struct{}

// NewSingleController creates a new SingleController.
func NewSingleController() *SingleController {
	return &SingleController{}
}

// GetBind returns the Cobra command metadata.
func (it *SingleController) GetBind() entities.ControllerBind {
	return entities.ControllerBind{
		Use:   "autobump",
		Short: "AutoBump is a tool that automatically updates CHANGELOG.md",
	}
}

// Execute runs the single-repo bump process.
func (it *SingleController) Execute(cmd *cobra.Command, _ []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	language, _ := cmd.Flags().GetString("language")

	globalConfig, err := findReadAndValidateConfig(configPath)
	if err != nil {
		log.Errorf("failed to read config: %v", err)
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Errorf("failed to get the current working directory: %v", err)
		return err
	}

	projectConfig := &entities.ProjectConfig{
		Path:     cwd,
		Language: language,
	}

	if projectConfig.Language == "" {
		detectedLanguage, detectErr := commands.DetectProjectLanguage(globalConfig, cwd)
		if detectErr != nil {
			log.Errorf("failed to detect project language: %v", detectErr)
			return detectErr
		}
		projectConfig.Language = detectedLanguage
	}

	if processErr := commands.ProcessRepo(globalConfig, projectConfig); processErr != nil {
		log.Errorf("failed to process repo: %v", processErr)
		return processErr
	}

	return nil
}

// findReadAndValidateConfig finds, reads and validates the config file.
func findReadAndValidateConfig(configPath string) (*entities.GlobalConfig, error) {
	configPath = entities.FindConfigOnMissing(configPath)

	globalConfig, err := entities.ReadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	err = entities.ValidateGlobalConfig(globalConfig, false)
	if errors.Is(err, entities.ErrLanguagesKeyMissingError) {
		log.Warn("Missing languages key, using the default configuration")

		var data []byte
		data, err = support.DownloadFile(entities.DefaultConfigURL)
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
