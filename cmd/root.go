package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/rios0rios0/autobump/application"
	"github.com/rios0rios0/autobump/config"
	"github.com/rios0rios0/autobump/infrastructure/provider"
	"github.com/rios0rios0/autobump/infrastructure/provider/azuredevops"
	"github.com/rios0rios0/autobump/infrastructure/provider/github"
	"github.com/rios0rios0/autobump/infrastructure/provider/gitlab"
	"github.com/rios0rios0/autobump/internal/support"
)

// cliConfig holds the CLI-level configuration flags.
type cliConfig struct {
	language   string
	configPath string
}

func initRootCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "autobump",
		Short: "AutoBump is a tool that automatically updates CHANGELOG.md",
		RunE: func(_ *cobra.Command, _ []string) error {
			globalConfig, err := findReadAndValidateConfig(cfg.configPath)
			if err != nil {
				return fmt.Errorf("failed to read config: %w", err)
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get the current working directory: %w", err)
			}

			projectConfig := &config.ProjectConfig{
				Path:     cwd,
				Language: cfg.language,
			}

			// In single-repo mode, detect the language eagerly and fail if unrecognized.
			if projectConfig.Language == "" {
				detectedLanguage, detectErr := application.DetectProjectLanguage(globalConfig, cwd)
				if detectErr != nil {
					return fmt.Errorf("failed to detect project language: %w", detectErr)
				}
				projectConfig.Language = detectedLanguage
			}

			// TODO: rollback the process removing the branch if exists,
			//       reverting the files and going back to main
			return application.ProcessRepo(globalConfig, projectConfig)
		},
	}
}

func initBatchCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "batch",
		Short: "Run AutoBump for all projects in the configuration",
		RunE: func(_ *cobra.Command, _ []string) error {
			globalConfig, err := findReadAndValidateConfig(cfg.configPath)
			if err != nil {
				return fmt.Errorf("failed to read config: %w", err)
			}

			return application.IterateProjects(globalConfig)
		},
	}
}

// findReadAndValidateConfig finds, reads and validates the config file.
func findReadAndValidateConfig(configPath string) (*config.GlobalConfig, error) {
	// find the config file if not manually set
	configPath = config.FindConfigOnMissing(configPath)

	// read the config file
	globalConfig, err := config.ReadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	err = config.ValidateGlobalConfig(globalConfig, false)
	if errors.Is(err, config.ErrLanguagesKeyMissingError) {
		log.Warn("Missing languages key, using the default configuration")

		var data []byte
		data, err = support.DownloadFile(config.DefaultConfigURL)
		if err != nil {
			return nil, fmt.Errorf("failed to download default config: %w", err)
		}

		var defaultConfig *config.GlobalConfig
		defaultConfig, err = config.DecodeConfig(data, false)
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

func initDiscoverCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "discover",
		Short: "Discover repos from configured providers and bump them automatically",
		Long: `Discover repositories by querying Git hosting provider APIs
(GitHub, GitLab, Azure DevOps) using configured tokens and organizations,
then run the bump process on each discovered repository.

Requires a 'providers' section in the configuration file.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			globalConfig, err := findReadAndValidateConfig(cfg.configPath)
			if err != nil {
				return fmt.Errorf("failed to read config: %w", err)
			}

			if len(globalConfig.Providers) == 0 {
				return errors.New("no providers configured; add a 'providers' section to the config file")
			}

			if validateErr := config.ValidateProviders(globalConfig.Providers); validateErr != nil {
				return validateErr
			}

			discovererRegistry := buildDiscovererRegistry()
			return application.DiscoverAndProcess(
				context.Background(), globalConfig, discovererRegistry,
			)
		},
	}
}

// buildDiscovererRegistry creates and returns the discoverer registry with all provider factories.
func buildDiscovererRegistry() *provider.DiscovererRegistry {
	reg := provider.NewDiscovererRegistry()
	reg.Register("github", github.NewDiscoverer)
	reg.Register("gitlab", gitlab.NewDiscoverer)
	reg.Register("azuredevops", azuredevops.NewDiscoverer)
	return reg
}

// buildProviderRegistry creates and returns the provider registry with all adapters.
func buildProviderRegistry() *provider.GitServiceRegistry {
	return provider.NewGitServiceRegistry(
		gitlab.NewAdapter(),
		azuredevops.NewAdapter(),
		github.NewAdapter(),
	)
}

// Execute sets up the CLI and runs the root command.
func Execute() error {
	// Initialize the provider registry
	provider.SetDefaultRegistry(buildProviderRegistry())

	cfg := &cliConfig{}
	rootCmd := initRootCmd(cfg)
	batchCmd := initBatchCmd(cfg)
	discoverCmd := initDiscoverCmd(cfg)

	rootCmd.Flags().StringVarP(&cfg.configPath, "config", "c", "", "config file path")
	rootCmd.Flags().StringVarP(&cfg.language, "language", "l", "", "project language")
	batchCmd.Flags().StringVarP(&cfg.configPath, "config", "c", "", "config file path")
	discoverCmd.Flags().StringVarP(&cfg.configPath, "config", "c", "", "config file path")

	rootCmd.AddCommand(batchCmd, discoverCmd)
	return rootCmd.Execute()
}
