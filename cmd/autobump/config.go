package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type GlobalConfig struct {
	Projects          []ProjectConfig           `yaml:"projects"`
	LanguagesConfig   map[string]LanguageConfig `yaml:"languages"`
	GpgKeyPath        string                    `yaml:"gpg_key_path"`
	GitLabAccessToken string                    `yaml:"gitlab_access_token"`
	GitLabCIJobToken  string
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
	NewVersion         string
}

// readConfig reads the config file and returns a GlobalConfig struct
func readConfig(configPath string) (*GlobalConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var globalConfig GlobalConfig
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)

	err = decoder.Decode(&globalConfig)
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

	if globalConfig.GitLabAccessToken != "" {
		_, err = os.Stat(globalConfig.GitLabAccessToken)
		if !os.IsNotExist(err) {
			log.Infof("Reading GitLab access token from file %s", globalConfig.GitLabAccessToken)
			token, err := os.ReadFile(globalConfig.GitLabAccessToken)
			if err != nil {
				return nil, err
			}
			globalConfig.GitLabAccessToken = strings.TrimSpace(string(token))
		}
	}

	globalConfig.GitLabCIJobToken = os.Getenv("CI_JOB_TOKEN")

	return &globalConfig, nil
}

// validateGlobalConfig validates the global config and reports missing keys and errors
func validateGlobalConfig(globalConfig *GlobalConfig, batch bool) error {
	missingKeys := []string{}

	if globalConfig.GpgKeyPath == "" {
		missingKeys = append(missingKeys, "gpg_key_path")
	}

	if batch == true && len(globalConfig.Projects) == 0 {
		missingKeys = append(missingKeys, "projects")
	}

	for i, projectConfig := range globalConfig.Projects {
		if projectConfig.Path == "" {
			missingKeys = append(missingKeys, fmt.Sprintf("projects[%d].path", i))
		}
		if batch == true && globalConfig.GitLabAccessToken == "" &&
			projectConfig.ProjectAccessToken == "" {
			log.Error(
				"Project access token is required when personal access token is not set in batch mode",
			)
			missingKeys = append(missingKeys, fmt.Sprintf("projects[%d].project_access_token", i))
		}
	}

	if len(missingKeys) > 0 {
		return errors.New("missing keys: " + strings.Join(missingKeys, ", "))
	}

	return nil
}

// findConfig finds the config file in a list of default locations
func findConfig() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	locations := []string{
		"autobump.yaml",
		"autobump.yml",
		"configs/autobump.yaml",
		"configs/autobump.yml",
		fmt.Sprintf("%s/.config/autobump.yaml", homeDir),
		fmt.Sprintf("%s/.config/autobump.yml", homeDir),
	}

	location, err := findFile(locations, "config file")
	if err != nil {
		return "", err
	}

	return location, nil
}
