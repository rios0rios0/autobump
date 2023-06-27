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
			globalConfig, err := readConfig(configPath)
			if err != nil {
				log.Fatalf("Failed to read config: %v", err)
				os.Exit(1)
			}

			cwd, err := os.Getwd()
			if err != nil {
				log.Fatalf("Failed to get the current working directory: %v", err)
				os.Exit(1)
			}

			projectsConfig := &ProjectsConfig{
				Path:     cwd,
				Language: language,
			}

			// detect the project language if not manually set
			if projectsConfig.Language == "" {
				projectLanguage, err := detectLanguage(globalConfig, projectsConfig.Path)
				if err != nil {
					log.Fatalf("Failed to detect project language: %v", err)
					os.Exit(1)
				}
				projectsConfig.Language = projectLanguage
			}

			err = processRepo(globalConfig, projectsConfig)
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
			globalConfig, err := readConfig(configPath)
			if err != nil {
				log.Fatalf("Failed to read config: %v", err)
				os.Exit(1)
			}

			iterateProjects(globalConfig)
		},
	}
)

// program entry point
func main() {
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "", "config file path")
	rootCmd.Flags().StringVarP(&language, "language", "l", "", "project language")

	// search for config file in default locations
	if configPath == "" {
		log.Info("No config file specified, searching for default locations")

		var err error
		configPath, err = findConfig()
		if err != nil {
			log.Fatalf("Failed to locate config file: \"%v\"", err)
			os.Exit(1)
		}

		log.Infof("Using config file: \"%v\"", configPath)
	}

	rootCmd.AddCommand(batchCmd)
	rootCmd.Execute()
}
