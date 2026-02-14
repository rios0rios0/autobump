package controllers

import (
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"go.uber.org/dig"
)

// RegisterProviders registers all controller providers with the DIG container.
func RegisterProviders(container *dig.Container) error {
	if err := container.Provide(NewSingleController); err != nil {
		return err
	}
	if err := container.Provide(NewBatchController); err != nil {
		return err
	}
	if err := container.Provide(NewDiscoverController); err != nil {
		return err
	}
	if err := container.Provide(NewControllers); err != nil {
		return err
	}
	return nil
}

// NewControllers aggregates all controllers into a slice for the AppInternal.
func NewControllers(
	batchController *BatchController,
	discoverController *DiscoverController,
) *[]entities.Controller {
	return &[]entities.Controller{
		batchController,
		discoverController,
	}
}
