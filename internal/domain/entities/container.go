package entities

import (
	"go.uber.org/dig"
)

// RegisterProviders is an intentional no-op kept for wiring/interface compatibility.
// Entity settings are loaded at runtime via the config file path from controllers,
// so no providers are registered with the DIG container here.
func RegisterProviders(_ *dig.Container) error {
	return nil
}
