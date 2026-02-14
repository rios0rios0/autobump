package internal

import (
	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/internal/infrastructure/controllers"
	"github.com/rios0rios0/autobump/internal/infrastructure/repositories"
	"go.uber.org/dig"
)

// RegisterProviders registers all internal providers with the DIG container.
func RegisterProviders(container *dig.Container) error {
	if err := repositories.RegisterProviders(container); err != nil {
		return err
	}
	if err := entities.RegisterProviders(container); err != nil {
		return err
	}
	if err := commands.RegisterProviders(container); err != nil {
		return err
	}
	if err := controllers.RegisterProviders(container); err != nil {
		return err
	}

	// Register the main app internal
	if err := container.Provide(NewAppInternal); err != nil {
		return err
	}

	return nil
}
