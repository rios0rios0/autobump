package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// LanguageAdapter is the interface for language-specific adapters
type LanguageAdapter interface {
	UpdateVersion(path string, config *ProjectsConfig) (string, error)
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

func (p *PythonAdapter) UpdateVersion(path string, config *ProjectsConfig) (string, error) {
	projectName := strings.Replace(filepath.Base(config.Path), "-", "_", -1)
	versionFilePath := filepath.Join(path, projectName, p.VersionFile())
	log.Debugf("Checking file %s", versionFilePath)
	if _, err := os.Stat(versionFilePath); os.IsNotExist(err) {
		log.Warnf("File %s does not exists", versionFilePath)
		return "", nil
	}

	content, err := ioutil.ReadFile(versionFilePath)
	if err != nil {
		log.Warnf("Failed to read file %s", versionFilePath)
		return "", err
	}

	versionIdentifier := p.VersionIdentifier()
	versionPattern := fmt.Sprintf(`%s"(\d+\.\d+\.\d+)"`, regexp.QuoteMeta(versionIdentifier))
	re := regexp.MustCompile(versionPattern)

	updatedContent := re.ReplaceAllString(string(content), versionIdentifier+"\""+config.NewVersion+"\"")
	err = ioutil.WriteFile(versionFilePath, []byte(updatedContent), 0644)
	if err != nil {
		log.Warnf("Failed to write changes to file %s", versionFilePath)
		return "", err
	}
	log.Debugf("Successfully updated file %s", versionFilePath)

	return versionFilePath, nil
}

func (p *PythonAdapter) VersionFile() string {
	return "__init__.py"
}

func (p *PythonAdapter) VersionIdentifier() string {
	return "__version__ = "
}
