package entities

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/rios0rios0/autobump/internal/support"
)

// envVarPattern matches ${VAR_NAME} placeholders in token strings.
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)}`)

// GlobalConfig represents the top-level configuration.
type GlobalConfig struct {
	Providers              []ProviderConfig          `yaml:"providers"`
	Projects               []ProjectConfig           `yaml:"projects"`
	LanguagesConfig        map[string]LanguageConfig `yaml:"languages"`
	GpgKeyPath             string                    `yaml:"gpg_key_path"`
	GitLabAccessToken      string                    `yaml:"gitlab_access_token"`
	AzureDevOpsAccessToken string                    `yaml:"azure_devops_access_token"`
	GitHubAccessToken      string                    `yaml:"github_access_token"`
	GitLabCIJobToken       string                    `yaml:"gitlab_ci_job_token"`
}

// ProviderConfig describes a single Git hosting provider for auto-discovery.
type ProviderConfig struct {
	Type          string   `yaml:"type"`          // "github", "gitlab", "azuredevops"
	Token         string   `yaml:"token"`         // inline, ${ENV_VAR}, or file path
	Organizations []string `yaml:"organizations"` // org names or URLs to scan
}

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

	handleTokenFile("GitLab", &globalConfig.GitLabAccessToken)
	handleTokenFile("Azure DevOps", &globalConfig.AzureDevOpsAccessToken)
	handleTokenFile("GitHub", &globalConfig.GitHubAccessToken)

	// Resolve provider tokens (env vars and file paths)
	for i := range globalConfig.Providers {
		globalConfig.Providers[i].Token = resolveToken(globalConfig.Providers[i].Token)
	}

	globalConfig.GitLabCIJobToken = os.Getenv("CI_JOB_TOKEN")

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
	return support.DownloadFile(configPath)
}

// handleTokenFile reads the token from a file if it exists and replaces the token string.
func handleTokenFile(name string, token *string) {
	if *token != "" {
		if _, err := os.Stat(*token); !os.IsNotExist(err) {
			log.Infof("Reading %s access token from file %s", name, *token)
			var fileToken []byte
			fileToken, err = os.ReadFile(*token)
			if err != nil {
				log.Errorf("failed to read %s access token: %v", name, err)
			} else {
				*token = strings.TrimSpace(string(fileToken))
			}
		}
	}
}

// resolveToken expands ${ENV_VAR} references in the token string and,
// if the result is a path to an existing file, reads the token from it.
func resolveToken(raw string) string {
	if raw == "" {
		return raw
	}

	// Expand ${ENV_VAR} references
	resolved := envVarPattern.ReplaceAllStringFunc(raw, func(match string) string {
		varName := envVarPattern.FindStringSubmatch(match)[1]
		if val := os.Getenv(varName); val != "" {
			return val
		}
		log.Warnf("Environment variable %q is not set", varName)
		return ""
	})

	// If the resolved value is a path to an existing file, read the token from it
	if _, err := os.Stat(resolved); err == nil {
		data, readErr := os.ReadFile(resolved)
		if readErr != nil {
			log.Warnf("Failed to read token file %q: %v", resolved, readErr)
			return resolved
		}
		log.Infof("Read token from file %q", resolved)
		return strings.TrimSpace(string(data))
	}

	return resolved
}

// ValidateProviders validates provider configuration entries.
func ValidateProviders(providers []ProviderConfig) error {
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
			log.Error(
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

// FindConfigOnMissing finds the config file if not manually set.
func FindConfigOnMissing(configPath string) string {
	if configPath == "" {
		log.Info("No config file specified, searching for default locations")

		var err error
		configPath, err = FindConfig()
		if err != nil {
			log.Warn(
				"Config file not found in default locations, " +
					"using the repository configuration as the last resort",
			)
			configPath = DefaultConfigURL
		}

		log.Infof("Using config file: \"%v\"", configPath)
		return configPath
	}
	return configPath
}

// FindConfig finds the config file in a list of default locations using globbing.
func FindConfig() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	// list of directories to search, in descending order of priority
	locations := []string{
		".",
		".config",
		"configs",
		homeDir,
		filepath.Join(homeDir, ".config"),
	}

	// all possible config file names
	patterns := []string{
		".autobump.yaml",
		".autobump.yml",
		"autobump.yaml",
		"autobump.yml",
	}

	for _, location := range locations {
		for _, pattern := range patterns {
			configPath := filepath.Join(location, pattern)
			_, err = os.Stat(configPath)
			if err == nil {
				return configPath, nil
			}
			// if the error is not a "file not found" error, log it
			if !os.IsNotExist(err) {
				log.Warnf("Failed to check '%s' for config file: %v", configPath, err)
			}
		}
	}

	return "", ErrConfigFileNotFoundError
}
