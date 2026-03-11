package controllers

import (
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	infraRepos "github.com/rios0rios0/autobump/internal/infrastructure/repositories"
)

// RunController handles the "run" subcommand (batch + discover engine).
type RunController struct {
	providerRegistry *infraRepos.ProviderRegistry
}

// NewRunController creates a new RunController.
func NewRunController(
	providerRegistry *infraRepos.ProviderRegistry,
) *RunController {
	return &RunController{
		providerRegistry: providerRegistry,
	}
}

// GetBind returns the Cobra command metadata.
func (it *RunController) GetBind() entities.ControllerBind {
	return entities.ControllerBind{
		Use:   "run",
		Short: "Run the version bump engine",
		Long: `Process repositories in batch mode using a configuration file.

Reads the configuration file to find repositories either from a static
'projects' list or by discovering them via provider APIs ('providers' section).
If both sections are present, both are processed.

This is the main command intended to be used in a cronjob or CI pipeline.`,
	}
}

// Execute runs the batch and/or discover mode based on config content.
func (it *RunController) Execute(cmd *cobra.Command, _ []string) {
	verbose, _ := cmd.Flags().GetBool("verbose")
	if verbose {
		logger.SetLevel(logger.DebugLevel)
	}

	configPath, _ := cmd.Flags().GetString("config")

	globalConfig, err := findReadAndValidateConfig(configPath)
	if err != nil {
		logger.Errorf("failed to read config: %v", err)
		return
	}

	hasProviders := len(globalConfig.Providers) > 0
	hasProjects := len(globalConfig.Projects) > 0

	if !hasProviders && !hasProjects {
		logger.Error(
			"no 'providers' or 'projects' configured; " +
				"add at least one section to the config file",
		)
		return
	}

	// Run provider-based discovery if providers are configured
	if hasProviders {
		if validateErr := entities.ValidateProviders(globalConfig.Providers); validateErr != nil {
			logger.Errorf("provider validation failed: %v", validateErr)
		} else {
			logger.Info("Running provider discovery...")
			if discoverErr := commands.DiscoverAndProcess(
				cmd.Context(), globalConfig, it.providerRegistry,
			); discoverErr != nil {
				logger.Errorf("discover failed: %v", discoverErr)
			}
		}
	}

	// Run static project list if projects are configured
	if hasProjects {
		logger.Info("Processing static project list...")
		if iterateErr := commands.IterateProjects(globalConfig); iterateErr != nil {
			logger.Errorf("batch processing failed: %v", iterateErr)
		}
	}
}

// AddFlags adds run-specific flags to the given Cobra command.
func (it *RunController) AddFlags(_ *cobra.Command) {}
