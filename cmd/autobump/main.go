package main

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	configPath string
	language   string

	rootCmd = &cobra.Command{
		Use:   "autobump",
		Short: "AutoBump is a tool that automatically updates CHANGELOG.md",
		Run: func(cmd *cobra.Command, args []string) {
			// find the config file if not manually set
			configPath, err := findConfigOnMissing(configPath)
			if err != nil {
				log.Fatalf("Failed to locate config file")
				os.Exit(1)
			}

			// read the config file
			globalConfig, err := readConfig(configPath)
			if err != nil {
				log.Fatalf("Failed to read config: %v", err)
				os.Exit(1)
			}

			if err = validateGlobalConfig(globalConfig, false); err != nil {
				log.Fatalf("Config validation failed: %v", err)
				os.Exit(1)
			}

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
				os.Exit(1)
			}
		},
	}

	batchCmd = &cobra.Command{
		Use:   "batch",
		Short: "Run AutoBump for all projects in the configuration",
		Run: func(cmd *cobra.Command, args []string) {
			// find the config file if not manually set
			configPath, err := findConfigOnMissing(configPath)
			if err != nil {
				log.Fatalf("Failed to locate config file")
				os.Exit(1)
			}

			// read the config file
			globalConfig, err := readConfig(configPath)
			if err != nil {
				log.Fatalf("Failed to read config: %v", err)
				os.Exit(1)
			}

			if err = validateGlobalConfig(globalConfig, true); err != nil {
				log.Fatalf("Config validation failed: %v", err)
				os.Exit(1)
			}

			iterateProjects(globalConfig)
		},
	}
)

// findConfigOnMissing finds the config file if not manually set
func findConfigOnMissing(configPath string) (string, error) {
	if configPath == "" {
		log.Info("No config file specified, searching for default locations")

		var err error
		configPath, err = findConfig()
		if err != nil {
			return "", err
		}

		log.Infof("Using config file: \"%v\"", configPath)
		return configPath, nil
	}
	return configPath, nil
}

// program entry point
func main() {
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "", "config file path")
	rootCmd.Flags().StringVarP(&language, "language", "l", "", "project language")
	batchCmd.Flags().StringVarP(&configPath, "config", "c", "", "config file path")

	rootCmd.AddCommand(batchCmd)
	rootCmd.Execute()
}
