package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type GlobalConfig struct {
	ProjectsConfig    []ProjectsConfig          `yaml:"projects"`
	LanguagesConfig   map[string]LanguageConfig `yaml:"languages"`
	GitLabAccessToken string                    `yaml:"gitlab_access_token"`
	GpgKeyPath        string                    `yaml:"gpg_key_path"`
}

type LanguageConfig struct {
	Extensions      []string `yaml:"extensions"`
	SpecialPatterns []string `yaml:"special_patterns"`
}

type ProjectsConfig struct {
	Path       string `yaml:"path"`
	Language   string `yaml:"language"`
	NewVersion string
}

func readConfig(configPath string) (*GlobalConfig, error) {
	data, err := ioutil.ReadFile(configPath)
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

	if err = validateGlobalConfig(&globalConfig); err != nil {
		return nil, err
	}

	return &globalConfig, nil
}

func validateGlobalConfig(cfg *GlobalConfig) error {
	missingKeys := []string{}

	if cfg.GitLabAccessToken == "" {

		// try to get the token from the environment variable
		gitlab_access_token := os.Getenv("CI_JOB_TOKEN")

		if gitlab_access_token != "" {
			cfg.GitLabAccessToken = gitlab_access_token
		} else {
			missingKeys = append(missingKeys, "gitlab_access_token")
		}
	}

	if cfg.GpgKeyPath == "" {
		missingKeys = append(missingKeys, "gpg_key_path")
	}

	if len(cfg.ProjectsConfig) == 0 {
		missingKeys = append(missingKeys, "projects")
	}

	for i, pc := range cfg.ProjectsConfig {
		if pc.Path == "" {
			missingKeys = append(missingKeys, fmt.Sprintf("projects[%d].path", i))
		}
	}

	if len(missingKeys) > 0 {
		return errors.New("missing keys: " + strings.Join(missingKeys, ", "))
	}

	return nil
}

func findConfig() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	locations := []string{
		"autobump.yaml",
		"autobump.yml",
		"configs/autobump.yaml",
		fmt.Sprintf("%s/.config/autobump.yaml", homeDir),
	}

	location, err := findFile(locations, "config file")
	if err != nil {
		return "", err
	}

	return location, nil
}
