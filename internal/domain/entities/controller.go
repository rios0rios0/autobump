package entities

import "github.com/spf13/cobra"

// Controller is the interface that all CLI controllers must implement.
type Controller interface {
	GetBind() ControllerBind
	Execute(command *cobra.Command, arguments []string)
}

// ControllerBind holds the Cobra command metadata for a controller.
type ControllerBind struct {
	Use   string
	Short string
	Long  string
}
