package commands

import (
	"go.uber.org/dig"
)

// RegisterProviders registers all command providers with the DIG container.
func RegisterProviders(container *dig.Container) error {
	return nil // Commands are called directly by controllers using service functions
}
