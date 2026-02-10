package python

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/rios0rios0/autobump/config"
)

// PyProject represents the pyproject.toml file.
type PyProject struct {
	Project Project `toml:"project"`
}

// Project holds the project metadata from pyproject.toml.
type Project struct {
	Name string `toml:"name"`
}

// Python implements the Language interface for Python projects.
type Python struct {
	ProjectConfig config.ProjectConfig
}

// ErrPyprojectNotFound is returned when pyproject.toml is not found.
var ErrPyprojectNotFound = errors.New("pyproject.toml not found")

// GetProjectName returns the project name from pyproject.toml.
func (p Python) GetProjectName() (string, error) {
	return getPyprojectName(p.ProjectConfig)
}

func getPyprojectName(projectConfig config.ProjectConfig) (string, error) {
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
