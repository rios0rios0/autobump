package controllers

import (
	"fmt"

	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/rios0rios0/autobump/configs"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	downloadHelpers "github.com/rios0rios0/gitforge/pkg/config/infrastructure/helpers"
)

// applySkipCleanupFlag turns off stale bump-branch cleanup when --skip-cleanup is set.
// The flag is a per-run override, so it wins over the configuration file; without it the
// configured value stands, and cleanup stays enabled when nothing is configured at all.
func applySkipCleanupFlag(cmd *cobra.Command, globalConfig *entities.GlobalConfig) {
	skipCleanup, _ := cmd.Flags().GetBool("skip-cleanup")
	if !skipCleanup {
		return
	}

	disabled := false
	globalConfig.CleanupStaleBranches = &disabled
	logger.Info("Stale bump branch cleanup is disabled for this run by --skip-cleanup")
}

// configLoader assembles the configuration layers and folds them.
//
// fetch is a field rather than a direct call so that a test can supply an offline one.
// Everything below it takes bytes, so the layering itself is testable without a network at
// all -- which the old shape was not: it downloaded the published defaults on every path.
type configLoader struct {
	fetch func(url string) ([]byte, error)
}

// newConfigLoader returns the loader the application uses.
func newConfigLoader() configLoader {
	return configLoader{fetch: downloadHelpers.DownloadFile}
}

// layers returns the three operator-facing configuration layers, in override order.
//
// The project's own `.autobump.yaml` is the fourth and is not here: it lives inside the
// repository being released, which in `run` mode has not been cloned yet.
func (l configLoader) layers(configPath string) ([]entities.ConfigLayer, error) {
	//nolint:exhaustruct // each layer sets only the fields that distinguish it
	layers := []entities.ConfigLayer{
		{
			Name:   entities.LayerBuiltInDefaults,
			Data:   configs.Default,
			Scope:  entities.ScopeRestricted,
			Strict: true,
		},
	}

	// The published defaults are the same document served from `main`, so a language fix
	// reaches an installed binary without a release. They are restricted and optional:
	// bytes fetched over the network have no business naming a token, a project or a
	// branch prefix, and an unreachable GitHub must not stop a release.
	if data, err := l.fetch(entities.DefaultConfigURL); err == nil {
		//nolint:exhaustruct // Strict stays false: `main` may be newer than this binary
		layers = append(layers, entities.ConfigLayer{
			Name:     entities.LayerPublishedDefaults,
			Origin:   entities.DefaultConfigURL,
			Data:     data,
			Scope:    entities.ScopeRestricted,
			Optional: true,
		})
	} else {
		logger.Debugf(
			"Could not fetch the published defaults (%v); running on the built-in ones", err,
		)
	}

	operatorConfigPath := entities.FindOperatorConfig(configPath)
	if operatorConfigPath == "" {
		return layers, nil
	}

	data, err := entities.ReadLayerData(operatorConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read the %s: %w", entities.LayerOperatorConfig, err)
	}

	//nolint:exhaustruct // Optional stays false: a file the operator named must be read
	layers = append(layers, entities.ConfigLayer{
		Name:   entities.LayerOperatorConfig,
		Origin: operatorConfigPath,
		Data:   data,
		Scope:  entities.ScopeOperator,
		Strict: true,
	})

	return layers, nil
}

// resolve folds the operator-facing layers and validates the result.
func (l configLoader) resolve(configPath string) (*entities.GlobalConfig, error) {
	layers, err := l.layers(configPath)
	if err != nil {
		return nil, err
	}

	globalConfig, err := entities.ResolveGlobalConfig(layers)
	if err != nil {
		return nil, err
	}

	if err = entities.ValidateGlobalConfig(globalConfig); err != nil {
		return nil, fmt.Errorf("failed to validate the configuration: %w", err)
	}

	return globalConfig, nil
}

// findReadAndValidateConfig resolves the configuration every command runs on.
func findReadAndValidateConfig(configPath string) (*entities.GlobalConfig, error) {
	return newConfigLoader().resolve(configPath)
}
