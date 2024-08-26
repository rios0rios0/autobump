package main

import (
	"errors"
	"fmt"
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

var ErrPyprojectNotFound = errors.New("pyproject.toml not found")

func (p Python) GetProjectName() (string, error) {
	return getPyprojectName(p.ProjectConfig)
}

func getPyprojectName(projectConfig ProjectConfig) (string, error) {
	pyprojectTomlPath := filepath.Join(projectConfig.Path, "pyproject.toml")

	_, err := os.Stat(pyprojectTomlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrPyprojectNotFound
		}
		return "", fmt.Errorf("error checking pyproject.toml path: %w", err)
	}

	var pyProject PyProject
	_, err = toml.DecodeFile(pyprojectTomlPath, &pyProject)
	if err != nil {
		return "", fmt.Errorf("error decoding pyproject.toml: %w", err)
	}

	return pyProject.Project.Name, nil
}
