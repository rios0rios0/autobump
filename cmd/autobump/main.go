package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	configPath string

	rootCmd = &cobra.Command{
		Use:   "autobump",
		Short: "AutoBump is a tool that automatically updates CHANGELOG.md",
		RunE: func(cmd *cobra.Command, args []string) error {
			globalConfig, err := readConfig(configPath)
			if err != nil {
				return err
			}

			if err := iterateProjects(globalConfig); err != nil {
				return err
			}

			return nil
		},
	}
)

// program entrypoint
func main() {
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "configs/autobump.yaml", "config file path")
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
