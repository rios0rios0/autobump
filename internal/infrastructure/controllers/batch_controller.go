package controllers

import (
	logger "github.com/sirupsen/logrus"
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
func (it *BatchController) Execute(cmd *cobra.Command, _ []string) {
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

	if iterateErr := commands.IterateProjects(globalConfig); iterateErr != nil {
		logger.Errorf("batch processing failed: %v", iterateErr)
	}
}
