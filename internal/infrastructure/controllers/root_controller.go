package controllers

import (
	"os"
	"path/filepath"

	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
)

// RootController backs the root command's positional-path form: `autobump .` and
// `autobump /path/to/repo`.
//
// It deliberately does not implement entities.Controller. That interface is what
// addSubcommands turns into a subcommand, and there is no `local` subcommand any more --
// having one would give the same behaviour two spellings, which is what it had before.
type RootController struct{}

// NewRootController creates a new RootController.
func NewRootController() *RootController {
	return &RootController{}
}

// Execute runs the single-repo bump process.
func (it *RootController) Execute(cmd *cobra.Command, args []string) {
	verbose, _ := cmd.Flags().GetBool("verbose")
	if verbose {
		logger.SetLevel(logger.DebugLevel)
	}

	configPath, _ := cmd.Flags().GetString("config")
	language, _ := cmd.Flags().GetString("language")

	globalConfig, err := findReadAndValidateConfig(configPath)
	if err != nil {
		logger.Errorf("failed to read config: %v", err)
		return
	}
	applySkipCleanupFlag(cmd, globalConfig)

	repoDir := "."
	if len(args) > 0 {
		repoDir = args[0]
	}

	repoDir, err = filepath.Abs(repoDir)
	if err != nil {
		logger.Errorf("failed to resolve path: %v", err)
		return
	}

	if _, statErr := os.Stat(repoDir); statErr != nil {
		if os.IsNotExist(statErr) {
			logger.Errorf("path does not exist: %s", repoDir)
		} else {
			logger.Errorf("failed to access path %s: %v", repoDir, statErr)
		}
		return
	}

	projectConfig := &entities.ProjectConfig{
		Path:     repoDir,
		Language: language,
	}

	if projectConfig.Language == "" {
		detectedLanguage, detectErr := commands.DetectProjectLanguage(globalConfig, repoDir)
		if detectErr != nil {
			logger.Errorf("failed to detect project language: %v", detectErr)
			return
		}
		projectConfig.Language = detectedLanguage
	}

	if processErr := commands.ProcessRepo(globalConfig, projectConfig); processErr != nil {
		logger.Errorf("failed to process repo: %v", processErr)
	}
}

// AddFlags adds local-specific flags to the given Cobra command.
func (it *RootController) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("language", "l", "", "project language")
	cmd.Args = cobra.MaximumNArgs(1)
}
