package controllers

import (
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"go.uber.org/dig"
)

// RegisterProviders registers all controller providers with the DIG container.
func RegisterProviders(container *dig.Container) error {
	if err := container.Provide(NewRootController); err != nil {
		return err
	}
	if err := container.Provide(NewRunController); err != nil {
		return err
	}
	if err := container.Provide(NewSelfUpdateController); err != nil {
		return err
	}
	if err := container.Provide(NewVersionController); err != nil {
		return err
	}
	if err := container.Provide(NewControllers); err != nil {
		return err
	}
	return nil
}

// NewControllers aggregates the controllers that become subcommands.
//
// RootController is not among them: it backs the root command's positional-path form and
// is injected there directly, so registering it here would recreate the `local`
// subcommand this release removed.
func NewControllers(
	runController *RunController,
	selfUpdateController *SelfUpdateController,
	versionController *VersionController,
) *[]entities.Controller {
	return &[]entities.Controller{
		runController,
		selfUpdateController,
		versionController,
	}
}
