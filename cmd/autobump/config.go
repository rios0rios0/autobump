package main

import (
	"gopkg.in/yaml.v3"
	"io/ioutil"
)

type GlobalConfig struct {
	ProjectsConfig    []ProjectsConfig `yaml:"projects"`
	GitLabAccessToken string           `yaml:"gitlab_access_token"`
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
	err = yaml.Unmarshal(data, &globalConfig)
	if err != nil {
		return nil, err
	}

	return &globalConfig, nil
}
