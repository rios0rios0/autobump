package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
)

// LanguageAdapter is the interface for language-specific adapters
type LanguageAdapter interface {
	UpdateVersion(path string, config *ProjectsConfig) error
	VersionFile() string
	VersionIdentifier() string
}

func getAdapterByName(name string) LanguageAdapter {
	switch name {
	case "Python":
		return &PythonAdapter{}
	default:
		return nil
	}
}

// PythonAdapter is the adapter for Python projects
type PythonAdapter struct{}

func (p *PythonAdapter) UpdateVersion(path string, config *ProjectsConfig) error {
	projectName := filepath.Base(config.Path)
	versionFilePath := filepath.Join(path, projectName, p.VersionFile())
	if _, err := os.Stat(versionFilePath); os.IsNotExist(err) {
		return nil
	}

	content, err := ioutil.ReadFile(versionFilePath)
	if err != nil {
		return err
	}

	versionIdentifier := p.VersionIdentifier()
	versionPattern := fmt.Sprintf(`%s(\d+\.\d+\.\d+)`, regexp.QuoteMeta(versionIdentifier))
	re := regexp.MustCompile(versionPattern)

	updatedContent := re.ReplaceAllString(string(content), versionIdentifier+config.NewVersion)
	err = ioutil.WriteFile(versionFilePath, []byte(updatedContent), 0644)
	if err != nil {
		return err
	}

	return nil
}

func (p *PythonAdapter) VersionFile() string {
	return "__init__.py"
}

func (p *PythonAdapter) VersionIdentifier() string {
	return "__version__ = "
}
