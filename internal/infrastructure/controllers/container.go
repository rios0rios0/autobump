package controllers

import (
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"go.uber.org/dig"
)

// RegisterProviders registers all controller providers with the DIG container.
func RegisterProviders(container *dig.Container) error {
	if err := container.Provide(NewLocalController); err != nil {
		return err
	}
	if err := container.Provide(NewRunController); err != nil {
		return err
	}
	if err := container.Provide(NewControllers); err != nil {
		return err
	}
	return nil
}

// NewControllers aggregates all controllers into a slice for the AppInternal.
func NewControllers(
	runController *RunController,
	localController *LocalController,
) *[]entities.Controller {
	return &[]entities.Controller{
		runController,
		localController,
	}
}
