package main

import (
	"errors"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type Config struct {
	language   string
	configPath string
}

func initRootCmd(config *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "autobump",
		Short: "AutoBump is a tool that automatically updates CHANGELOG.md",
		Run: func(_ *cobra.Command, _ []string) {
			globalConfig, err := findReadAndValidateConfig(config.configPath)
			if err != nil {
				log.Fatalf("Failed to read config: %v", err)
			}

			cwd, err := os.Getwd()
			if err != nil {
				log.Fatalf("Failed to get the current working directory: %v", err)
			}

			projectConfig := &ProjectConfig{
				Path:     cwd,
				Language: config.language,
			}

			// detect the project language if not manually set
			if projectConfig.Language == "" {
				var projectLanguage string
				projectLanguage, err = detectProjectLanguage(globalConfig, projectConfig.Path)
				if err != nil {
					log.Fatalf("Failed to detect project language: %v", err)
				}
				projectConfig.Language = projectLanguage
			}

			err = processRepo(globalConfig, projectConfig)
			if err != nil {
				log.Fatalf("Failed to process repo: %v", err)
				// TODO: rollback the process removing the branch if exists,
				//       reverting the files and going back to main
			}
		},
	}
}

func initBatchCmd(config *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "batch",
		Short: "Run AutoBump for all projects in the configuration",
		Run: func(_ *cobra.Command, _ []string) {
			globalConfig, err := findReadAndValidateConfig(config.configPath)
			if err != nil {
				log.Fatalf("Failed to read config: %v", err)
			}

			err = iterateProjects(globalConfig)
			if err != nil {
				log.Fatalf("Failed to iterate projects: %v", err)
			}
		},
	}
}

// findReadAndValidateConfig finds, reads and validates the config file
func findReadAndValidateConfig(configPath string) (*GlobalConfig, error) {
	// find the config file if not manually set
	configPath = findConfigOnMissing(configPath)

	// read the config file
	globalConfig, err := readConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	err = validateGlobalConfig(globalConfig, false)
	if errors.Is(err, ErrLanguagesKeyMissingError) {
		log.Warn("Missing languages key, using the default configuration")

		var data []byte
		data, err = downloadFile(defaultConfigURL)
		if err != nil {
			return nil, fmt.Errorf("failed to download default config: %w", err)
		}

		var defaultConfig *GlobalConfig
		defaultConfig, err = decodeConfig(data, false)
		if err != nil {
			return nil, fmt.Errorf("failed to decode default config: %w", err)
		}

		// TODO: this merge could be done for each language
		globalConfig.LanguagesConfig = defaultConfig.LanguagesConfig
	} else if err != nil {
		return nil, fmt.Errorf("failed to validate global config: %w", err)
	}

	return globalConfig, nil
}

func main() {
	config := &Config{}
	rootCmd := initRootCmd(config)
	batchCmd := initBatchCmd(config)

	rootCmd.Flags().StringVarP(&config.configPath, "config", "c", "", "config file path")
	rootCmd.Flags().StringVarP(&config.language, "language", "l", "", "project language")
	batchCmd.Flags().StringVarP(&config.configPath, "config", "c", "", "config file path")

	rootCmd.AddCommand(batchCmd)
	err := rootCmd.Execute()
	if err != nil {
		log.Fatalf("Uncaught error: %v", err)
	}
}
