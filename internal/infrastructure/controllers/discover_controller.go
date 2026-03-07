package controllers

import (
	"context"
	"errors"
	"fmt"

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
func (it *DiscoverController) Execute(cmd *cobra.Command, _ []string) error {
	configPath, _ := cmd.Flags().GetString("config")

	globalConfig, err := findReadAndValidateConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if len(globalConfig.Providers) == 0 {
		return errors.New("no providers configured; add a 'providers' section to the config file")
	}

	if validateErr := entities.ValidateProviders(globalConfig.Providers); validateErr != nil {
		return fmt.Errorf("provider validation failed: %w", validateErr)
	}

	return commands.DiscoverAndProcess(
		context.Background(), globalConfig, it.providerRegistry,
	)
}

// AddFlags is a no-op for the discover controller (uses inherited flags from root).
func (it *DiscoverController) AddFlags(_ *cobra.Command) {}
