package main

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type PyProject struct {
	Project Project `toml:"project"`
}

type Project struct {
	Name string `toml:"name"`
}

type Python struct {
	ProjectConfig ProjectConfig
}

func (p Python) GetProjectName() (string, error) {
	return getPyprojectName(p.ProjectConfig)
}

func getPyprojectName(projectConfig ProjectConfig) (string, error) {
	pyprojectTomlPath := filepath.Join(projectConfig.Path, "pyproject.toml")

	_, err := os.Stat(pyprojectTomlPath)
	if err != nil {
		return "", err
	}

	var pyProject PyProject
	_, err = toml.DecodeFile(pyprojectTomlPath, &pyProject)
	if err != nil {
		return "", err
	}

	return pyProject.Project.Name, nil
}
