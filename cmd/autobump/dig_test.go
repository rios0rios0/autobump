//go:build unit

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInjectAppContext(t *testing.T) {
	t.Parallel()

	t.Run("should create app context without panic", func(t *testing.T) {
		// given / when
		app := injectAppContext()

		// then
		require.NotNil(t, app)
		assert.NotNil(t, app.GetControllers())
	})
}

func TestInjectLocalController(t *testing.T) {
	t.Parallel()

	t.Run("should inject local controller without panic", func(t *testing.T) {
		// given / when
		ctrl := injectLocalController()

		// then
		require.NotNil(t, ctrl)
	})
}

func TestInjectProviderRegistry(t *testing.T) {
	t.Parallel()

	t.Run("should inject provider registry without panic", func(t *testing.T) {
		// given / when
		registry := injectProviderRegistry()

		// then
		require.NotNil(t, registry)
	})
}

func TestBuildRootCommand(t *testing.T) {
	t.Parallel()

	t.Run("should create root command with expected metadata", func(t *testing.T) {
		// given
		ctrl := injectLocalController()

		// when
		cmd := buildRootCommand(ctrl)

		// then
		require.NotNil(t, cmd)
		assert.Equal(t, "autobump [path]", cmd.Use)
		assert.NotNil(t, cmd.PersistentFlags().Lookup("config"))
		assert.NotNil(t, cmd.PersistentFlags().Lookup("verbose"))
		assert.NotNil(t, cmd.Flags().Lookup("language"))
	})
}

func TestAddSubcommands(t *testing.T) {
	t.Parallel()

	t.Run("should add subcommands to root command", func(t *testing.T) {
		// given
		ctrl := injectLocalController()
		appCtx := injectAppContext()
		rootCmd := buildRootCommand(ctrl)

		// when
		addSubcommands(rootCmd, appCtx)

		// then
		assert.True(t, rootCmd.HasSubCommands())
	})
}
