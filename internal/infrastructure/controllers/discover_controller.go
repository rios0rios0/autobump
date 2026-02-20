package controllers

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	infraRepos "github.com/rios0rios0/autobump/internal/infrastructure/repositories"
)

// DiscoverController handles the "discover" subcommand.
type DiscoverController struct {
	providerRegistry *infraRepos.ProviderRegistry
}

// NewDiscoverController creates a new DiscoverController.
func NewDiscoverController(
	providerRegistry *infraRepos.ProviderRegistry,
) *DiscoverController {
	return &DiscoverController{
		providerRegistry: providerRegistry,
	}
}

// GetBind returns the Cobra command metadata.
func (it *DiscoverController) GetBind() entities.ControllerBind {
	return entities.ControllerBind{
		Use:   "discover",
		Short: "Discover repos from configured providers and bump them automatically",
		Long: `Discover repositories by querying Git hosting provider APIs
(GitHub, GitLab, Azure DevOps) using configured tokens and organizations,
then run the bump process on each discovered repository.

Requires a 'providers' section in the configuration file.`,
	}
}

// Execute runs the discover mode.
func (it *DiscoverController) Execute(cmd *cobra.Command, _ []string) {
	configPath, _ := cmd.Flags().GetString("config")

	globalConfig, err := findReadAndValidateConfig(configPath)
	if err != nil {
		log.Errorf("failed to read config: %v", err)
		return
	}

	if len(globalConfig.Providers) == 0 {
		log.Error("no providers configured; add a 'providers' section to the config file")
		return
	}

	if validateErr := entities.ValidateProviders(globalConfig.Providers); validateErr != nil {
		log.Errorf("provider validation failed: %v", validateErr)
		return
	}

	if discoverErr := commands.DiscoverAndProcess(
		context.Background(), globalConfig, it.providerRegistry,
	); discoverErr != nil {
		log.Errorf("discover failed: %v", discoverErr)
	}
}

// AddFlags is a no-op for the discover controller (uses inherited flags from root).
func (it *DiscoverController) AddFlags(_ *cobra.Command) {}
