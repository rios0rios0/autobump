package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
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
	GitLabCIJobToken       string
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
	var err error
	var data []byte

	// check if configPath is a URL
	uri, err := url.Parse(configPath)
	if err != nil || uri.Scheme == "" || uri.Host == "" {
		// it's not a URL, read the data from file
		data, err = os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
	} else {
		// it's a URL, so read the data from the URL
		resp, err := http.Get(configPath)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		data, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
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

	// TODO: transform this structure in a loop to avoid code duplication
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
	if globalConfig.AzureDevOpsAccessToken != "" {
		_, err = os.Stat(globalConfig.AzureDevOpsAccessToken)
		if !os.IsNotExist(err) {
			log.Infof("Reading Azure DevOps access token from file %s", globalConfig.AzureDevOpsAccessToken)
			token, err := os.ReadFile(globalConfig.AzureDevOpsAccessToken)
			if err != nil {
				return nil, err
			}
			globalConfig.AzureDevOpsAccessToken = strings.TrimSpace(string(token))
		}
	}

	globalConfig.GitLabCIJobToken = os.Getenv("CI_JOB_TOKEN")

	return &globalConfig, nil
}

// validateGlobalConfig validates the global config and reports missing keys and errors
func validateGlobalConfig(globalConfig *GlobalConfig, batch bool) error {
	var missingKeys []string

	// TODO: validate if the section languages exists and if not download and merge the default configuration

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

	// TODO: it doesn't need a for loop to find the file, just a regex matching the name
	locations := []string{
		".autobump.yaml",
		".autobump.yml",
		".config/autobump.yaml",
		".config/autobump.yml",
		"autobump.yaml",
		"autobump.yml",
		"configs/.autobump.yaml",
		"configs/.autobump.yml",
		"configs/autobump.yaml",
		"configs/autobump.yml",
		fmt.Sprintf("%s/.autobump.yaml", homeDir),
		fmt.Sprintf("%s/.autobump.yml", homeDir),
		fmt.Sprintf("%s/.config/autobump.yaml", homeDir),
		fmt.Sprintf("%s/.config/autobump.yml", homeDir),
	}

	location, err := findFile(locations, "config file")
	if err != nil {
		return "", err
	}

	return location, nil
}
