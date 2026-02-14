package entities

import (
	"go.uber.org/dig"
)

// RegisterProviders registers all entity providers with the DIG container.
func RegisterProviders(container *dig.Container) error {
	return nil // Settings loaded at runtime via config file path from controllers
}
