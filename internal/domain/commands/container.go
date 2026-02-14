package commands

import (
	"go.uber.org/dig"
)

// RegisterProviders is intentionally a no-op: commands are invoked directly by controllers,
// so command providers are not registered with the DIG container.
func RegisterProviders(_ *dig.Container) error {
	return nil
}
