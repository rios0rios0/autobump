package main

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type LanguageAdapter interface {
	VersionFile(*ProjectsConfig) (string, error)
	VersionPattern() string
}

func getAdapterByName(name string) LanguageAdapter {
	switch strings.ToLower(name) {
	case "python":
		return &PythonAdapter{}
	case "java":
		return &JavaAdapter{}
	default:
		return nil
	}
}

func detectLanguage(globalConfig *GlobalConfig, cwd string) (string, error) {
	var detected string

	absPath, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}

	// Check project type by special files
	for language, config := range globalConfig.LanguagesConfig {
		for _, pattern := range config.SpecialPatterns {
			_, err := os.Stat(filepath.Join(absPath, pattern))
			if !os.IsNotExist(err) {
				return language, nil
			}
		}
	}

	// Check project type by file extensions
	err = filepath.Walk(absPath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if detected != "" {
			return filepath.SkipDir
		}

		for language, config := range globalConfig.LanguagesConfig {
			for _, ext := range config.Extensions {
				if strings.HasSuffix(info.Name(), "."+ext) {
					detected = language
					return filepath.SkipDir
				}
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return "", errors.New("project language not recognized")
}

func updateVersion(adapter LanguageAdapter, path string, config *ProjectsConfig) error {
	versionFile, err := adapter.VersionFile(config)
	if err != nil {
		return err
	}

	versionFilePath := filepath.Join(config.Path, versionFile)
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

	err = ioutil.WriteFile(versionFilePath, []byte(updatedContent), 0o644)
	if err != nil {
		return err
	}

	return nil
}

type PythonAdapter struct{}

func (p *PythonAdapter) VersionFile(config *ProjectsConfig) (string, error) {
	projectName := strings.Replace(filepath.Base(config.Path), "-", "_", -1)
	return filepath.Join(projectName, "__init__.py"), nil
}

func (p *PythonAdapter) VersionPattern() string {
	return `(__version__\s*=\s*")\d+\.\d+\.\d+(")`
}

type JavaAdapter struct{}

func (j *JavaAdapter) VersionFile(config *ProjectsConfig) (string, error) {
	locations := []string{
		filepath.Join(config.Path, "build.gradle"),
		filepath.Join(config.Path, "lib", "build.gradle"),
	}
	buildGradlePath, err := findFile(locations, "build.gradle")
	if err != nil {
		return "", err
	}
	return buildGradlePath, nil
}

func (j *JavaAdapter) VersionPattern() string {
	return `(version\s*=\s*')\d+\.\d+\.\d+(')`
}
