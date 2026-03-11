package controllers

import (
	"os"
	"path/filepath"

	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
)

// LocalController handles the "local" subcommand (single repo mode).
type LocalController struct{}

// NewLocalController creates a new LocalController.
func NewLocalController() *LocalController {
	return &LocalController{}
}

// GetBind returns the Cobra command metadata.
func (it *LocalController) GetBind() entities.ControllerBind {
	return entities.ControllerBind{
		Use:   "local",
		Short: "Run AutoBump for a single local repository",
		Long: `Process a single local repository: read CHANGELOG.md, calculate the next
semantic version, update version files, commit, push, and create a PR.

If no path is specified, the current working directory is used.`,
	}
}

// Execute runs the single-repo bump process.
func (it *LocalController) Execute(cmd *cobra.Command, args []string) {
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
func (it *LocalController) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("language", "l", "", "project language")
	cmd.Args = cobra.MaximumNArgs(1)
}
