package entities

import "github.com/spf13/cobra"

// Controller is the interface that all CLI controllers must implement.
// Execute returns an error to match gitforge's Controller interface.
type Controller interface {
	GetBind() ControllerBind
	Execute(command *cobra.Command, arguments []string) error
}
