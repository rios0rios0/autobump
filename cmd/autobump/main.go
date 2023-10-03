package main

import (
	"errors"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	language   string
	configPath string

	rootCmd = &cobra.Command{
		Use:   "autobump",
		Short: "AutoBump is a tool that automatically updates CHANGELOG.md",
		Run: func(cmd *cobra.Command, args []string) {
			globalConfig := findReadAndValidateConfig(configPath)

			cwd, err := os.Getwd()
			if err != nil {
				log.Fatalf("Failed to get the current working directory: %v", err)
				os.Exit(1)
			}

			projectConfig := &ProjectConfig{
				Path:     cwd,
				Language: language,
			}

			// detect the project language if not manually set
			if projectConfig.Language == "" {
				projectLanguage, err := detectLanguage(globalConfig, projectConfig.Path)
				if err != nil {
					log.Fatalf("Failed to detect project language: %v", err)
					os.Exit(1)
				}
				projectConfig.Language = projectLanguage
			}

			err = processRepo(globalConfig, projectConfig)
			if err != nil {
				log.Fatalf("Failed to process repo: %v", err)

				// TODO: rollback the process removing the branch if exists, reverting the files and going back to main

				os.Exit(1)
			}
		},
	}

	batchCmd = &cobra.Command{
		Use:   "batch",
		Short: "Run AutoBump for all projects in the configuration",
		Run: func(cmd *cobra.Command, args []string) {
			globalConfig := findReadAndValidateConfig(configPath)
			iterateProjects(globalConfig)
		},
	}
)

// findReadAndValidateConfig finds, reads and validates the config file
func findReadAndValidateConfig(configPath string) *GlobalConfig {
	// find the config file if not manually set
	configPath = findConfigOnMissing(configPath)

	// read the config file
	globalConfig, err := readConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to read config: %v", err)
		os.Exit(1)
	}

	if err = validateGlobalConfig(globalConfig, false); err != nil {
		if errors.Is(err, missingLanguagesKeyError) {
			log.Warn("Missing languages key, using the default configuration")

			var data []byte
			data, err = downloadFile(defaultConfigUrl)
			if err != nil {
				log.Fatalf("Failed to download default config: %v", err)
				os.Exit(1)
			}

			var defaultConfig *GlobalConfig
			defaultConfig, err = decodeConfig(data)
			// TODO: this merge could be done, for each language
			globalConfig.LanguagesConfig = defaultConfig.LanguagesConfig
		} else {
			log.Fatalf("Config validation failed: %v", err)
			os.Exit(1)
		}
	}

	return globalConfig
}

func main() {
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "", "config file path")
	rootCmd.Flags().StringVarP(&language, "language", "l", "", "project language")
	batchCmd.Flags().StringVarP(&configPath, "config", "c", "", "config file path")

	rootCmd.AddCommand(batchCmd)
	rootCmd.Execute()
}
