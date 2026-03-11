package main

import (
	"os"

	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/rios0rios0/autobump/internal"
	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/infrastructure/controllers"
	gitInfra "github.com/rios0rios0/gitforge/pkg/git/infrastructure"
)

func buildRootCommand(localController *controllers.LocalController) *cobra.Command {
	//nolint:exhaustruct // Minimal Command initialization with required fields only
	cmd := &cobra.Command{
		Use:   "autobump [path]",
		Short: "AutoBump is a tool that automatically updates CHANGELOG.md",
		Long: `AutoBump automates the release process: reads CHANGELOG.md, calculates
the next semantic version, updates version files, commits, pushes, and creates PRs.

Supports GitHub, GitLab, and Azure DevOps as Git hosting providers.

Usage modes:
  autobump local             Bump version in the current directory
  autobump local /path       Bump version in a specific directory
  autobump run               Batch mode using a config file (cronjob)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if len(args) == 0 {
				return command.Help()
			}
			localController.Execute(command, args)
			return nil
		},
	}

	// Global persistent flags
	cmd.PersistentFlags().StringP("config", "c", "", "Path to config file (default: auto-detect)")
	cmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")

	// Root-level flags (for `autobump .` shorthand)
	cmd.Flags().StringP("language", "l", "", "project language")

	return cmd
}

func addSubcommands(rootCmd *cobra.Command, appContext *internal.AppInternal) {
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
		if lc, ok := ctrl.(*controllers.LocalController); ok {
			lc.AddFlags(subCmd)
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
	rootCmd.AddCommand(batchCmd, discoverCmd)
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

	// Initialize the provider registry via DIG and create GitOperations with it
	providerRegistry := injectProviderRegistry()
	gitOps := gitInfra.NewGitOperations(providerRegistry)
	commands.SetGitOperations(gitOps)
	commands.SetProviderRegistry(providerRegistry)

	// Inject the local controller and create root command
	localController := injectLocalController()
	rootCmd := buildRootCommand(localController)

	// Add all subcommands (including deprecation aliases)
	appContext := injectAppContext()
	addSubcommands(rootCmd, appContext)

	if err := rootCmd.Execute(); err != nil {
		logger.Errorf("Uncaught error: %v", err)
		os.Exit(1)
	}
}
