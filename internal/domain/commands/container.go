package commands

import (
	"go.uber.org/dig"
)

// RegisterProviders registers all command providers with the DIG container.
func RegisterProviders(container *dig.Container) error {
	if err := container.Provide(NewVersionCommand); err != nil {
		return err
	}
	if err := container.Provide(NewSelfUpdateCommand); err != nil {
		return err
	}

	// Bind interfaces to implementations
	if err := container.Provide(func(impl *VersionCommand) Version {
		return impl
	}); err != nil {
		return err
	}
	if err := container.Provide(func(impl *SelfUpdateCommand) SelfUpdate {
		return impl
	}); err != nil {
		return err
	}

	return nil
}
