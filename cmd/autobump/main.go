package main

import (
	"os"

	log "github.com/sirupsen/logrus"
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
				log.Fatalf("Failed to read config: %v", err)
				os.Exit(1)
			}

			iterateProjects(globalConfig)
			return nil
		},
	}
)

// program entrypoint
func main() {
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "configs/autobump.yaml", "config file path")
	rootCmd.Execute()
}
