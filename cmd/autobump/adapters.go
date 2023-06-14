package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type LanguageAdapter interface {
	VersionFile(*ProjectsConfig) string
	VersionPattern() string
}

func getAdapterByName(name string) LanguageAdapter {
	switch name {
	case "Python":
		return &PythonAdapter{}
	case "Java":
		return &JavaAdapter{}
	default:
		return nil
	}
}

func updateVersion(adapter LanguageAdapter, path string, config *ProjectsConfig) error {
	versionFilePath := filepath.Join(config.Path, adapter.VersionFile(config))
	if _, err := os.Stat(versionFilePath); os.IsNotExist(err) {
		return nil
	}

	content, err := ioutil.ReadFile(versionFilePath)
	if err != nil {
		return err
	}

	versionPattern := adapter.VersionPattern()
	re := regexp.MustCompile(versionPattern)

	updatedContent := re.ReplaceAllStringFunc(string(content), func(match string) string {
		return re.ReplaceAllString(match, "${1}"+config.NewVersion+"${2}")
	})

	err = ioutil.WriteFile(versionFilePath, []byte(updatedContent), 0644)
	if err != nil {
		return err
	}

	return nil
}

type PythonAdapter struct{}

func (p *PythonAdapter) VersionFile(config *ProjectsConfig) string {
	projectName := strings.Replace(filepath.Base(config.Path), "-", "_", -1)
	return filepath.Join(projectName, "__init__.py")
}

func (p *PythonAdapter) VersionPattern() string {
	return `(__version__\s*=\s*")\d+\.\d+\.\d+(")`
}

type JavaAdapter struct{}

func (j *JavaAdapter) VersionFile(config *ProjectsConfig) string {
	return "build.gradle"
}

func (j *JavaAdapter) VersionPattern() string {
	return `(version\s*=\s*')\d+\.\d+\.\d+(')`
}
