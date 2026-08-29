package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInjectAppContext(t *testing.T) {
	t.Parallel()

	t.Run("should create app context without panic", func(t *testing.T) {
		t.Parallel()

		// given / when
		app := injectAppContext()

		// then
		require.NotNil(t, app)
		assert.NotNil(t, app.GetControllers())
	})
}

func TestInjectRootController(t *testing.T) {
	t.Parallel()

	t.Run("should inject the root controller without panic", func(t *testing.T) {
		t.Parallel()

		// given / when
		ctrl := injectRootController()

		// then
		require.NotNil(t, ctrl)
	})
}

func TestInjectProviderRegistry(t *testing.T) {
	t.Parallel()

	t.Run("should inject provider registry without panic", func(t *testing.T) {
		t.Parallel()

		// given / when
		registry := injectProviderRegistry()

		// then
		require.NotNil(t, registry)
	})
}

func TestBuildRootCommand(t *testing.T) {
	t.Parallel()

	t.Run("should create root command with expected metadata", func(t *testing.T) {
		t.Parallel()

		// given
		ctrl := injectRootController()

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
		t.Parallel()

		// given
		ctrl := injectRootController()
		appCtx := injectAppContext()
		rootCmd := buildRootCommand(ctrl)

		// when
		addSubcommands(rootCmd, appCtx, ctrl)

		// then
		assert.True(t, rootCmd.HasSubCommands())
	})

	t.Run("should register the run, version and self-update subcommands", func(t *testing.T) {
		t.Parallel()

		// given
		ctrl := injectRootController()
		appCtx := injectAppContext()
		rootCmd := buildRootCommand(ctrl)
		addSubcommands(rootCmd, appCtx, ctrl)

		// when / then
		for _, name := range []string{"run", "version", "self-update"} {
			subCmd, _, err := rootCmd.Find([]string{name})
			require.NoError(t, err)
			require.NotNil(t, subCmd)
			assert.Equal(t, name, subCmd.Name())
		}
	})

	t.Run("should keep local as a hidden deprecated command", func(t *testing.T) {
		t.Parallel()

		// given -- `local` was removed, but registering nothing in its place is worse than
		// a deprecation notice: the bare word would fall through to the root command's
		// positional argument and be treated as a path, so `autobump local` would report
		// that ./local does not exist rather than that the command is gone
		ctrl := injectRootController()
		appCtx := injectAppContext()
		rootCmd := buildRootCommand(ctrl)
		addSubcommands(rootCmd, appCtx, ctrl)

		// when
		localCmd, _, err := rootCmd.Find([]string{"local"})

		// then
		require.NoError(t, err)
		require.NotNil(t, localCmd)
		assert.Equal(t, "local", localCmd.Name())
		assert.True(t, localCmd.Hidden, "it must not appear in --help")
	})

	t.Run("should register batch as hidden deprecated command", func(t *testing.T) {
		t.Parallel()

		// given
		ctrl := injectRootController()
		appCtx := injectAppContext()
		rootCmd := buildRootCommand(ctrl)
		addSubcommands(rootCmd, appCtx, ctrl)

		// when
		batchCmd, _, err := rootCmd.Find([]string{"batch"})

		// then
		require.NoError(t, err)
		require.NotNil(t, batchCmd)
		assert.Equal(t, "batch", batchCmd.Name())
		assert.True(t, batchCmd.Hidden)
	})

	t.Run("should register discover as hidden deprecated command", func(t *testing.T) {
		t.Parallel()

		// given
		ctrl := injectRootController()
		appCtx := injectAppContext()
		rootCmd := buildRootCommand(ctrl)
		addSubcommands(rootCmd, appCtx, ctrl)

		// when
		discoverCmd, _, err := rootCmd.Find([]string{"discover"})

		// then
		require.NoError(t, err)
		require.NotNil(t, discoverCmd)
		assert.Equal(t, "discover", discoverCmd.Name())
		assert.True(t, discoverCmd.Hidden)
	})
}

func TestSubcommandExecution(t *testing.T) {
	t.Parallel()

	t.Run("should execute run subcommand without panic", func(t *testing.T) {
		t.Parallel()

		// given
		ctrl := injectRootController()
		appCtx := injectAppContext()
		rootCmd := buildRootCommand(ctrl)
		addSubcommands(rootCmd, appCtx, ctrl)

		runCmd, _, err := rootCmd.Find([]string{"run"})
		require.NoError(t, err)
		runCmd.Flags().Bool("verbose", false, "")
		runCmd.Flags().String("config", "/nonexistent/config.yaml", "")

		// when / then -- will fail at config loading but shouldn't panic
		assert.NotPanics(t, func() {
			if runCmd.Run != nil {
				runCmd.Run(runCmd, []string{})
			}
		})
	})

	t.Run("should execute the deprecated local subcommand without panic", func(t *testing.T) {
		t.Parallel()

		// given
		ctrl := injectRootController()
		appCtx := injectAppContext()
		rootCmd := buildRootCommand(ctrl)
		addSubcommands(rootCmd, appCtx, ctrl)

		localCmd, _, err := rootCmd.Find([]string{"local"})
		require.NoError(t, err)
		localCmd.Flags().Bool("verbose", false, "")
		localCmd.Flags().String("config", "/nonexistent/config.yaml", "")

		// when / then
		assert.NotPanics(t, func() {
			if localCmd.Run != nil {
				localCmd.Run(localCmd, []string{})
			}
		})
	})

	t.Run("should execute deprecated batch command without panic", func(t *testing.T) {
		t.Parallel()

		// given
		ctrl := injectRootController()
		appCtx := injectAppContext()
		rootCmd := buildRootCommand(ctrl)
		addSubcommands(rootCmd, appCtx, ctrl)

		batchCmd, _, err := rootCmd.Find([]string{"batch"})
		require.NoError(t, err)
		// Add required flags that batch inherits from root via persistent flags
		batchCmd.Flags().Bool("verbose", false, "")
		batchCmd.Flags().String("config", "/nonexistent/config.yaml", "")

		// when / then -- batch delegates to RunController.Execute
		assert.NotPanics(t, func() {
			if batchCmd.Run != nil {
				batchCmd.Run(batchCmd, []string{})
			}
		})
	})

	t.Run("should execute deprecated discover command without panic", func(t *testing.T) {
		t.Parallel()

		// given
		ctrl := injectRootController()
		appCtx := injectAppContext()
		rootCmd := buildRootCommand(ctrl)
		addSubcommands(rootCmd, appCtx, ctrl)

		discoverCmd, _, err := rootCmd.Find([]string{"discover"})
		require.NoError(t, err)
		discoverCmd.Flags().Bool("verbose", false, "")
		discoverCmd.Flags().String("config", "/nonexistent/config.yaml", "")

		// when / then
		assert.NotPanics(t, func() {
			if discoverCmd.Run != nil {
				discoverCmd.Run(discoverCmd, []string{})
			}
		})
	})
}

func TestBuildRootCommandRunE(t *testing.T) {
	t.Parallel()

	t.Run("should show help when no args provided", func(t *testing.T) {
		t.Parallel()

		// given
		ctrl := injectRootController()
		cmd := buildRootCommand(ctrl)

		// when
		err := cmd.RunE(cmd, []string{})

		// then -- RunE calls Help() which returns nil
		require.NoError(t, err)
	})

	t.Run("should delegate to local controller when args provided", func(t *testing.T) {
		t.Parallel()

		// given
		ctrl := injectRootController()
		cmd := buildRootCommand(ctrl)

		// when -- passes a nonexistent path, Execute will log error but RunE returns nil
		err := cmd.RunE(cmd, []string{"/nonexistent/path"})

		// then
		require.NoError(t, err)
	})
}
