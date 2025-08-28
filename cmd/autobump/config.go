package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type GlobalConfig struct {
	Projects               []ProjectConfig           `yaml:"projects"`
	LanguagesConfig        map[string]LanguageConfig `yaml:"languages"`
	GpgKeyPath             string                    `yaml:"gpg_key_path"`
	GitLabAccessToken      string                    `yaml:"gitlab_access_token"`
	AzureDevOpsAccessToken string                    `yaml:"azure_devops_access_token"`
	GitHubAccessToken      string                    `yaml:"github_access_token"`
	GitLabCIJobToken       string                    `yaml:"gitlab_ci_job_token"`
}

type LanguageConfig struct {
	Extensions      []string      `yaml:"extensions"`
	SpecialPatterns []string      `yaml:"special_patterns"`
	VersionFiles    []VersionFile `yaml:"version_files"`
}

type VersionFile struct {
	Path     string   `yaml:"path"`
	Patterns []string `yaml:"patterns"`
}

type ProjectConfig struct {
	Path               string `yaml:"path"`
	Name               string `yaml:"name"`
	Language           string `yaml:"language"`
	ProjectAccessToken string `yaml:"project_access_token"`
	NewVersion         string `yaml:"new_version"`
}

const defaultConfigURL = "https://raw.githubusercontent.com/rios0rios0/autobump/" +
	"main/configs/autobump.yaml"

var (
	ErrLanguagesKeyMissingError = errors.New("missing languages key")
	ErrConfigFileNotFoundError  = errors.New("config file not found")
	ErrConfigKeyMissingError    = errors.New("config keys missing")
)

// readConfig reads the config file and returns a GlobalConfig struct
func readConfig(configPath string) (*GlobalConfig, error) {
	data, err := readData(configPath)
	if err != nil {
		return nil, err
	}

	globalConfig, err := decodeConfig(data)
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

	globalConfig.GitLabCIJobToken = os.Getenv("CI_JOB_TOKEN")

	return globalConfig, nil
}

// readData reads data from a file or a URL
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
	return downloadFile(configPath)
}

// handleTokenFile reads the token from a file if it exists and replaces the token string
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

// decodeConfig decodes the config file and returns a GlobalConfig struct
func decodeConfig(data []byte) (*GlobalConfig, error) {
	var globalConfig GlobalConfig

	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	err := decoder.Decode(&globalConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	return &globalConfig, nil
}

// validateGlobalConfig validates the global config and reports missing keys and errors
func validateGlobalConfig(globalConfig *GlobalConfig, batch bool) error {
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

// findConfigOnMissing finds the config file if not manually set
func findConfigOnMissing(configPath string) string {
	if configPath == "" {
		log.Info("No config file specified, searching for default locations")

		var err error
		configPath, err = findConfig()
		if err != nil {
			log.Warn(
				"Config file not found in default locations, " +
					"using the repository configuration as the last resort",
			)
			configPath = defaultConfigURL
		}

		log.Infof("Using config file: \"%v\"", configPath)
		return configPath
	}
	return configPath
}

// findConfig finds the config file in a list of default locations using globbing
func findConfig() (string, error) {
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
