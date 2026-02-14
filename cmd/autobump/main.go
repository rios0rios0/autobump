package main

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/rios0rios0/autobump/internal"
	"github.com/rios0rios0/autobump/internal/infrastructure/controllers"
	infraRepos "github.com/rios0rios0/autobump/internal/infrastructure/repositories"
	gitutil "github.com/rios0rios0/autobump/internal/infrastructure/repositories/git"
)

func buildRootCommand(singleController *controllers.SingleController) *cobra.Command {
	bind := singleController.GetBind()
	//nolint:exhaustruct // Minimal Command initialization with required fields only
	cmd := &cobra.Command{
		Use:   bind.Use,
		Short: bind.Short,
		Run: func(command *cobra.Command, arguments []string) {
			singleController.Execute(command, arguments)
		},
	}

	cmd.Flags().StringP("config", "c", "", "config file path")
	cmd.Flags().StringP("language", "l", "", "project language")

	return cmd
}

func addSubcommands(rootCmd *cobra.Command, appContext *internal.AppInternal) {
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

		subCmd.Flags().StringP("config", "c", "", "config file path")
		rootCmd.AddCommand(subCmd)
	}
}

func main() {
	// Initialize the provider registry via DIG and set as default
	providerRegistry := injectProviderRegistry()
	infraRepos.SetDefaultRegistry(providerRegistry)
	gitutil.SetAdapterFinder(providerRegistry)

	// Inject the single controller and create root command
	singleController := injectSingleController()
	rootCmd := buildRootCommand(singleController)

	// Add all subcommands
	appContext := injectAppContext()
	addSubcommands(rootCmd, appContext)

	if err := rootCmd.Execute(); err != nil {
		log.Errorf("Uncaught error: %v", err)
		os.Exit(1)
	}
}
