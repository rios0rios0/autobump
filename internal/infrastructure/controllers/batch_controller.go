package controllers

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
)

// BatchController handles the "batch" subcommand.
type BatchController struct{}

// NewBatchController creates a new BatchController.
func NewBatchController() *BatchController {
	return &BatchController{}
}

// GetBind returns the Cobra command metadata.
func (it *BatchController) GetBind() entities.ControllerBind {
	return entities.ControllerBind{
		Use:   "batch",
		Short: "Run AutoBump for all projects in the configuration",
	}
}

// Execute runs the batch mode.
func (it *BatchController) Execute(cmd *cobra.Command, _ []string) error {
	configPath, _ := cmd.Flags().GetString("config")

	globalConfig, err := findReadAndValidateConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	return commands.IterateProjects(globalConfig)
}
