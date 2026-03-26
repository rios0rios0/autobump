//go:build unit

package controllers_test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/internal/infrastructure/controllers"
	"github.com/rios0rios0/autobump/internal/infrastructure/repositories"
)

func TestNewLocalController(t *testing.T) {
	t.Parallel()

	t.Run("should create a non-nil controller", func(t *testing.T) {
		// given / when
		ctrl := controllers.NewLocalController()

		// then
		require.NotNil(t, ctrl)
	})
}

func TestLocalControllerGetBind(t *testing.T) {
	t.Parallel()

	t.Run("should return bind with local command metadata", func(t *testing.T) {
		// given
		ctrl := controllers.NewLocalController()

		// when
		bind := ctrl.GetBind()

		// then
		assert.Equal(t, "local", bind.Use)
		assert.NotEmpty(t, bind.Short)
		assert.NotEmpty(t, bind.Long)
	})
}

func TestLocalControllerAddFlags(t *testing.T) {
	t.Parallel()

	t.Run("should add language flag to command", func(t *testing.T) {
		// given
		ctrl := controllers.NewLocalController()
		cmd := &cobra.Command{}

		// when
		ctrl.AddFlags(cmd)

		// then
		flag := cmd.Flags().Lookup("language")
		require.NotNil(t, flag)
		assert.Equal(t, "l", flag.Shorthand)
	})
}

func TestNewRunController(t *testing.T) {
	t.Parallel()

	t.Run("should create a non-nil controller", func(t *testing.T) {
		// given
		registry := repositories.NewProviderRegistry()

		// when
		ctrl := controllers.NewRunController(registry)

		// then
		require.NotNil(t, ctrl)
	})
}

func TestRunControllerGetBind(t *testing.T) {
	t.Parallel()

	t.Run("should return bind with run command metadata", func(t *testing.T) {
		// given
		registry := repositories.NewProviderRegistry()
		ctrl := controllers.NewRunController(registry)

		// when
		bind := ctrl.GetBind()

		// then
		assert.Equal(t, "run", bind.Use)
		assert.NotEmpty(t, bind.Short)
		assert.NotEmpty(t, bind.Long)
	})
}

func TestRunControllerAddFlags(t *testing.T) {
	t.Parallel()

	t.Run("should not panic when adding flags", func(t *testing.T) {
		// given
		registry := repositories.NewProviderRegistry()
		ctrl := controllers.NewRunController(registry)
		cmd := &cobra.Command{}

		// when / then
		assert.NotPanics(t, func() {
			ctrl.AddFlags(cmd)
		})
	})
}

func TestNewControllers(t *testing.T) {
	t.Parallel()

	t.Run("should aggregate controllers into a slice", func(t *testing.T) {
		// given
		local := controllers.NewLocalController()
		run := controllers.NewRunController(repositories.NewProviderRegistry())

		// when
		result := controllers.NewControllers(run, local)

		// then
		require.NotNil(t, result)
		assert.Len(t, *result, 2)
		assert.IsType(t, (*[]entities.Controller)(nil), result)
	})
}
