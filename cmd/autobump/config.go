package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v3"
)

type GlobalConfig struct {
	ProjectsConfig    []ProjectsConfig `yaml:"projects"`
	GitLabAccessToken string           `yaml:"gitlab_access_token"`
	GpgKeyPath        string           `yaml:"gpg_key_path"`
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
		missingKeys = append(missingKeys, "gitlab_access_token")
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

		if pc.Language == "" {
			missingKeys = append(missingKeys, fmt.Sprintf("projects[%d].language", i))
		}
	}

	if len(missingKeys) > 0 {
		return errors.New("missing keys: " + strings.Join(missingKeys, ", "))
	}

	return nil
}
