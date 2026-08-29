package main

import (
	"os"

	"github.com/rios0rios0/cliforge/pkg/selfupdate"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/rios0rios0/autobump/internal"
	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/infrastructure/controllers"
	gitInfra "github.com/rios0rios0/gitforge/pkg/git/infrastructure"
)

// version is set at build time via ldflags.
//

var version = "dev"

// runUpdateCheck performs the update check unless the build is a local dev build
// or the invoked subcommand is one that must not trigger it (self-update, version).
func runUpdateCheck(command *cobra.Command) {
	if commands.AutobumpVersion == "dev" {
		return
	}
	switch command.Name() {
	case "self-update", "version":
		return
	}
	selfupdate.NewCommand("rios0rios0", "autobump", "autobump", commands.AutobumpVersion).CheckForUpdates()
}

func buildRootCommand(rootController *controllers.RootController) *cobra.Command {
	//nolint:exhaustruct // Minimal Command initialization with required fields only
	cmd := &cobra.Command{
		Use:   "autobump [path]",
		Short: "AutoBump is a tool that automatically updates CHANGELOG.md",
		Long: `AutoBump automates the release process: reads CHANGELOG.md, calculates
the next semantic version, updates version files, commits, pushes, and creates PRs.

Supports GitHub, GitLab, and Azure DevOps as Git hosting providers.

Usage modes:
  autobump .                 Bump version in the current directory
  autobump /path             Bump version in a specific directory
  autobump run               Batch mode using a config file (cronjob)`,
		Args: cobra.MaximumNArgs(1),
		PersistentPreRun: func(command *cobra.Command, _ []string) {
			runUpdateCheck(command)
		},
		RunE: func(command *cobra.Command, args []string) error {
			if len(args) == 0 {
				return command.Help()
			}
			rootController.Execute(command, args)
			return nil
		},
	}

	// Global persistent flags
	cmd.PersistentFlags().StringP("config", "c", "", "Path to config file (default: auto-detect)")
	cmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	cmd.PersistentFlags().Bool(
		"skip-cleanup", false,
		"Keep bump branches from previous runs instead of deleting them and closing their PRs",
	)

	// The single-repository flags belong to the root command itself now that there is no
	// `local` subcommand to carry a second copy of them. AddFlags also sets Args, so the
	// positional path is bounded in one place rather than two.
	rootController.AddFlags(cmd)

	return cmd
}

func addSubcommands(
	rootCmd *cobra.Command,
	appContext *internal.AppInternal,
	rootController *controllers.RootController,
) {
	// Find the RunController from registered controllers for deprecation aliases
	var runController *controllers.RunController
	for _, ctrl := range appContext.GetControllers() {
		if rc, ok := ctrl.(*controllers.RunController); ok {
			runController = rc
			break
		}
	}

	for _, controller := range appContext.GetControllers() {
		bind := controller.GetBind()
		ctrl := controller // capture for closure
		//nolint:exhaustruct // Minimal Command initialization with required fields only
		subCmd := &cobra.Command{
			Use:   bind.Use,
			Short: bind.Short,
			Long:  bind.Long,
			Run: func(command *cobra.Command, arguments []string) {
				ctrl.Execute(command, arguments)
			},
		}

		// Add controller-specific flags
		if rc, ok := ctrl.(*controllers.RunController); ok {
			rc.AddFlags(subCmd)
		}
		if sc, ok := ctrl.(*controllers.SelfUpdateController); ok {
			sc.AddFlags(subCmd)
		}
		if vc, ok := ctrl.(*controllers.VersionController); ok {
			vc.AddFlags(subCmd)
		}

		rootCmd.AddCommand(subCmd)
	}

	// Hidden deprecation aliases for backward compatibility
	//nolint:exhaustruct // Minimal Command initialization with required fields only
	batchCmd := &cobra.Command{
		Use:    "batch",
		Short:  "Deprecated: use 'run' instead",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			logger.Warn("'batch' is deprecated, use 'run' instead")
			runController.Execute(cmd, args)
		},
	}
	//nolint:exhaustruct // Minimal Command initialization with required fields only
	discoverCmd := &cobra.Command{
		Use:    "discover",
		Short:  "Deprecated: use 'run' instead",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			logger.Warn("'discover' is deprecated, use 'run' instead")
			runController.Execute(cmd, args)
		},
	}
	// `local` is gone, but leaving nothing in its place is worse than a deprecation
	// notice: the bare word would fall through to the root command's positional argument
	// and be treated as a path, so `autobump local` would report that ./local does not
	// exist rather than that the command was removed.
	//nolint:exhaustruct // Minimal Command initialization with required fields only
	localCmd := &cobra.Command{
		Use:    "local",
		Short:  "Deprecated: use 'autobump [path]' instead",
		Hidden: true,
		Args:   cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			logger.Warn("'local' was removed in 3.0.0, use 'autobump [path]' instead")
			rootController.Execute(cmd, args)
		},
	}
	localCmd.Flags().StringP("language", "l", "", "project language")

	rootCmd.AddCommand(batchCmd, discoverCmd, localCmd)
}

func main() {
	//nolint:exhaustruct // Minimal TextFormatter initialization with required fields only
	logger.SetFormatter(&logger.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})
	if os.Getenv("DEBUG") == "true" {
		logger.SetLevel(logger.DebugLevel)
	}

	// Bridge the build-time version to the domain package
	commands.AutobumpVersion = version //nolint:reassign // build-time version must be bridged to domain

	// Initialize the provider registry via DIG and create GitOperations with it
	providerRegistry := injectProviderRegistry()
	gitOps := gitInfra.NewGitOperations(providerRegistry)
	commands.SetGitOperations(gitOps)
	commands.SetProviderRegistry(providerRegistry)

	// Inject the root controller and create root command
	rootController := injectRootController()
	rootCmd := buildRootCommand(rootController)

	// Add all subcommands (including deprecation aliases)
	appContext := injectAppContext()
	addSubcommands(rootCmd, appContext, rootController)

	if err := rootCmd.Execute(); err != nil {
		logger.Errorf("Uncaught error: %v", err)
		os.Exit(1)
	}
}
