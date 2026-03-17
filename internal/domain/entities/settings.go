package entities

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	logger "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	configEntities "github.com/rios0rios0/gitforge/pkg/config/domain/entities"
	configHelpers "github.com/rios0rios0/gitforge/pkg/config/domain/helpers"
	downloadHelpers "github.com/rios0rios0/gitforge/pkg/config/infrastructure/helpers"
)

// GlobalConfig represents the top-level configuration.
type GlobalConfig struct {
	Providers              []configEntities.ProviderConfig `yaml:"providers"`
	Projects               []ProjectConfig                 `yaml:"projects"`
	LanguagesConfig        map[string]LanguageConfig       `yaml:"languages"`
	GpgKeyPath             string                          `yaml:"gpg_key_path"`
	GpgKeyPassphrase       string                          `yaml:"gpg_key_passphrase"`
	GitLabAccessToken      string                          `yaml:"gitlab_access_token"`
	AzureDevOpsAccessToken string                          `yaml:"azure_devops_access_token"`
	GitHubAccessToken      string                          `yaml:"github_access_token"`
	GitLabCIJobToken       string                          `yaml:"gitlab_ci_job_token"`
}

// ProviderConfig is re-exported from gitforge for backward compatibility.
type ProviderConfig = configEntities.ProviderConfig

// LanguageConfig holds per-language detection and versioning rules.
type LanguageConfig struct {
	Extensions      []string      `yaml:"extensions"`
	SpecialPatterns []string      `yaml:"special_patterns"`
	VersionFiles    []VersionFile `yaml:"version_files"`
}

// VersionFile describes a file that contains version information.
type VersionFile struct {
	Path     string   `yaml:"path"`
	Patterns []string `yaml:"patterns"`
}

// ProjectConfig holds per-project configuration.
type ProjectConfig struct {
	Path               string `yaml:"path"`
	Name               string `yaml:"name"`
	Language           string `yaml:"language"`
	ProjectAccessToken string `yaml:"project_access_token"`
	NewVersion         string `yaml:"new_version"`
}

// DefaultConfigURL is the URL of the default configuration file.
const DefaultConfigURL = "https://raw.githubusercontent.com/rios0rios0/autobump/" +
	"main/configs/autobump.yaml"

var (
	ErrLanguagesKeyMissingError = errors.New("missing languages key")
	ErrConfigFileNotFoundError  = errors.New("config file not found")
	ErrConfigKeyMissingError    = errors.New("config keys missing")
)

// ReadConfig reads the config file and returns a GlobalConfig struct.
func ReadConfig(configPath string) (*GlobalConfig, error) {
	data, err := readData(configPath)
	if err != nil {
		return nil, err
	}

	globalConfig, err := DecodeConfig(data, true)
	if err != nil {
		return nil, err
	}

	for i := range globalConfig.Projects {
		if globalConfig.Projects[i].Name == "" {
			basename := path.Base(globalConfig.Projects[i].Path)
			basename = strings.TrimSuffix(basename, ".git")
			globalConfig.Projects[i].Name = basename
		}
	}

	handleTokenFile("GPG passphrase", &globalConfig.GpgKeyPassphrase)
	handleTokenFile("GitLab access token", &globalConfig.GitLabAccessToken)
	handleTokenFile("Azure DevOps access token", &globalConfig.AzureDevOpsAccessToken)
	handleTokenFile("GitHub access token", &globalConfig.GitHubAccessToken)

	// Resolve provider tokens (env vars and file paths)
	for i := range globalConfig.Providers {
		globalConfig.Providers[i].Token = globalConfig.Providers[i].ResolveToken()
	}

	globalConfig.GitLabCIJobToken = os.Getenv("CI_JOB_TOKEN")

	if globalConfig.GpgKeyPassphrase == "" {
		globalConfig.GpgKeyPassphrase = os.Getenv("GPG_PASSPHRASE")
	}

	return globalConfig, nil
}

// readData reads data from a file or a URL.
func readData(configPath string) ([]byte, error) {
	uri, err := url.Parse(configPath)
	if err != nil || uri.Scheme == "" || uri.Host == "" {
		// It's not a URL, read the data from file
		var data []byte
		data, err = os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		return data, nil
	}
	// It's a URL, so read the data from the URL
	return downloadHelpers.DownloadFile(configPath)
}

// handleTokenFile reads the token from a file if it exists and replaces the token string.
func handleTokenFile(name string, token *string) {
	if *token != "" {
		if _, err := os.Stat(*token); !os.IsNotExist(err) {
			logger.Infof("Reading %s from file...", name)
			var fileToken []byte
			fileToken, err = os.ReadFile(*token)
			if err != nil {
				logger.Errorf("failed to read %s from file: %v", name, err)
			} else {
				*token = strings.TrimSpace(string(fileToken))
			}
		}
	}
}

// ValidateProviders validates provider configuration entries.
func ValidateProviders(providers []configEntities.ProviderConfig) error {
	for i, p := range providers {
		if p.Type == "" {
			return fmt.Errorf(
				"%w: providers[%d].type is required",
				ErrConfigKeyMissingError, i,
			)
		}
		if p.Token == "" {
			return fmt.Errorf(
				"%w: providers[%d].token is required (set inline, via ${ENV_VAR}, or as file path)",
				ErrConfigKeyMissingError, i,
			)
		}
		if len(p.Organizations) == 0 {
			return fmt.Errorf(
				"%w: providers[%d].organizations must have at least one entry",
				ErrConfigKeyMissingError, i,
			)
		}
	}
	return nil
}

// DecodeConfig decodes the config file and returns a GlobalConfig struct
// If strict is true, unknown fields will cause an error (for user config)
// If strict is false, unknown fields will be ignored (for default config).
func DecodeConfig(data []byte, strict bool) (*GlobalConfig, error) {
	var globalConfig GlobalConfig

	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(strict)
	err := decoder.Decode(&globalConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	return &globalConfig, nil
}

// ValidateGlobalConfig validates the global config and reports missing keys and errors.
func ValidateGlobalConfig(globalConfig *GlobalConfig, batch bool) error {
	var missingKeys []string

	if batch && len(globalConfig.Projects) == 0 {
		missingKeys = append(missingKeys, "projects")
	}

	for projectIndex, projectConfig := range globalConfig.Projects {
		if projectConfig.Path == "" {
			missingKeys = append(missingKeys, fmt.Sprintf("projects[%d].path", projectIndex))
		}
		if batch && globalConfig.GitLabAccessToken == "" &&
			globalConfig.AzureDevOpsAccessToken == "" &&
			globalConfig.GitHubAccessToken == "" &&
			projectConfig.ProjectAccessToken == "" {
			logger.Error(
				"Project access token is required when personal access token " +
					"is not set in batch mode",
			)
			missingKeys = append(
				missingKeys,
				fmt.Sprintf("projects[%d].project_access_token", projectIndex),
			)
		}
	}

	if len(missingKeys) > 0 {
		return fmt.Errorf("%w: %s", ErrConfigKeyMissingError, strings.Join(missingKeys, ", "))
	}

	if globalConfig.LanguagesConfig == nil {
		return ErrLanguagesKeyMissingError
	}

	return nil
}

// MergeLanguagesConfig deep-merges user language overrides into defaults.
// Version files with the same path are replaced; new paths are appended.
// Extensions and special patterns from defaults are preserved when the user
// provides only version files. New languages are added wholesale.
func MergeLanguagesConfig(
	defaults, overrides map[string]LanguageConfig,
) map[string]LanguageConfig {
	result := make(map[string]LanguageConfig, len(defaults))
	for k, v := range defaults {
		result[k] = v
	}

	for lang, override := range overrides {
		base, exists := result[lang]
		if !exists {
			result[lang] = override
			continue
		}

		if len(override.Extensions) > 0 {
			base.Extensions = dedup(append(base.Extensions, override.Extensions...))
		}
		if len(override.SpecialPatterns) > 0 {
			base.SpecialPatterns = dedup(append(base.SpecialPatterns, override.SpecialPatterns...))
		}
		if len(override.VersionFiles) > 0 {
			base.VersionFiles = mergeVersionFiles(base.VersionFiles, override.VersionFiles)
		}

		result[lang] = base
	}

	return result
}

// mergeVersionFiles merges override version files into base.
// Files with a matching path replace the default; others are appended.
func mergeVersionFiles(base, overrides []VersionFile) []VersionFile {
	merged := make([]VersionFile, len(base))
	copy(merged, base)

	for _, ov := range overrides {
		found := false
		for i, bv := range merged {
			if bv.Path == ov.Path {
				merged[i] = ov
				found = true
				break
			}
		}
		if !found {
			merged = append(merged, ov)
		}
	}
	return merged
}

// dedup removes duplicate strings while preserving order.
func dedup(s []string) []string {
	seen := make(map[string]struct{}, len(s))
	out := make([]string, 0, len(s))
	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

// FindConfigOnMissing finds the config file if not manually set.
func FindConfigOnMissing(configPath string) string {
	if configPath == "" {
		logger.Info("No config file specified, searching for default locations")

		var err error
		configPath, err = configHelpers.FindConfigFile("autobump")
		if err != nil {
			logger.Warn(
				"Config file not found in default locations, " +
					"using the repository configuration as the last resort",
			)
			configPath = DefaultConfigURL
		}

		logger.Infof("Using config file: \"%v\"", configPath)
		return configPath
	}
	return configPath
}
