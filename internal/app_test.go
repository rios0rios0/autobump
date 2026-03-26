//go:build unit

package internal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"

	"github.com/rios0rios0/autobump/internal"
	"github.com/rios0rios0/autobump/internal/domain/entities"
)

func TestNewAppInternal(t *testing.T) {
	t.Parallel()

	t.Run("should create AppInternal when controllers are provided", func(t *testing.T) {
		// given
		controllers := &[]entities.Controller{}

		// when
		app := internal.NewAppInternal(controllers)

		// then
		require.NotNil(t, app)
		assert.NotNil(t, app.GetControllers())
	})
}

func TestGetControllers(t *testing.T) {
	t.Parallel()

	t.Run("should return the controllers passed during construction", func(t *testing.T) {
		// given
		controllers := &[]entities.Controller{}
		app := internal.NewAppInternal(controllers)

		// when
		result := app.GetControllers()

		// then
		assert.Empty(t, result)
	})
}

func TestRegisterProviders(t *testing.T) {
	t.Parallel()

	t.Run("should register all providers without error", func(t *testing.T) {
		// given
		container := dig.New()

		// when
		err := internal.RegisterProviders(container)

		// then
		require.NoError(t, err)
	})

	t.Run("should allow invoking AppInternal after registration", func(t *testing.T) {
		// given
		container := dig.New()
		require.NoError(t, internal.RegisterProviders(container))

		// when
		var app *internal.AppInternal
		err := container.Invoke(func(a *internal.AppInternal) {
			app = a
		})

		// then
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotEmpty(t, app.GetControllers())
	})
}
