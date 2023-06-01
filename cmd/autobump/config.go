package main

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

type GlobalConfig struct {
	ProjectsConfig []ProjectsConfig `yaml:"projects"`
	Credentials    Credentials      `yaml:"credentials"`
	GpgConfig      GpgConfig        `yaml:"gpg"`
}

type ProjectsConfig struct {
	Path       string `yaml:"path"`
	Language   string `yaml:"language"`
	NewVersion string
}

type Credentials struct {
	GitLabAccessToken string `yaml:"gitlab_access_token"`
	Username          string `yaml:"username"`
	PrettyName        string `yaml:"pretty_name"`
	Email             string `yaml:"email"`
}

type GpgConfig struct {
	Location string `yaml:"location"`
	Password string `yaml:"password"`
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
