package controllers

import (
	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/spf13/cobra"
)

type VersionController struct {
	command commands.Version
}

func NewVersionController(command commands.Version) *VersionController {
	return &VersionController{command: command}
}

func (it *VersionController) GetBind() entities.ControllerBind {
	return entities.ControllerBind{
		Use:   "version",
		Short: "Show autobump version",
		Long:  "Display the version information for autobump.",
	}
}

func (it *VersionController) Execute(_ *cobra.Command, _ []string) {
	it.command.Execute()
}
