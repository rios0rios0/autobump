package controllers

import (
	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type SelfUpdateController struct {
	command commands.SelfUpdate
}

func NewSelfUpdateController(command commands.SelfUpdate) *SelfUpdateController {
	return &SelfUpdateController{command: command}
}

func (it *SelfUpdateController) GetBind() entities.ControllerBind {
	return entities.ControllerBind{
		Use:   "self-update",
		Short: "Update autobump to the latest version",
		Long:  "Download and install the latest version of autobump from GitHub releases.",
	}
}

func (it *SelfUpdateController) Execute(cmd *cobra.Command, _ []string) {
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		logger.Errorf("failed to read --dry-run flag: %v", err)
		return
	}

	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		logger.Errorf("failed to read --force flag: %v", err)
		return
	}

	err = it.command.Execute(dryRun, force)
	if err != nil {
		logger.Errorf("Self-update failed: %s", err)
		return
	}
}

// AddFlags adds self-update-specific flags to the given Cobra command.
func (it *SelfUpdateController) AddFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("dry-run", false, "Show what would be updated without performing it")
	cmd.Flags().Bool("force", false, "Skip confirmation prompts")
	cmd.Args = cobra.NoArgs
}
