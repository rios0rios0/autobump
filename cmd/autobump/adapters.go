package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

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

// updateVersion updates the version in the version files.
// This function fails fast upon the first error.
func updateVersion(path string, globalConfig *GlobalConfig, projectsConfig *ProjectsConfig) error {
	versionFiles, err := getVersionFiles(globalConfig, projectsConfig)
	if err != nil {
		return err
	}

	oneVersionFileExists := false
	for _, versionFile := range versionFiles {
		// check if the file exists
		info, err := os.Stat(versionFile)
		if os.IsNotExist(err) {
			log.Warnf("Version file %s does not exist", versionFile)
			continue
		}

		originalFileMode := info.Mode()
		oneVersionFileExists = true

		content, err := ioutil.ReadFile(versionFile)
		if err != nil {
			return err
		}

		versionPattern, err := getVersionPattern(globalConfig, projectsConfig)
		if err != nil {
			return err
		}

		re := regexp.MustCompile(versionPattern)
		updatedContent := re.ReplaceAllStringFunc(string(content), func(match string) string {
			return re.ReplaceAllString(match, "${1}"+projectsConfig.NewVersion+"${2}")
		})

		err = ioutil.WriteFile(versionFile, []byte(updatedContent), originalFileMode)
		if err != nil {
			return err
		}
	}

	if !oneVersionFileExists {
		return errors.New(fmt.Sprintf("No version file found for %s", projectsConfig.Language))
	}

	return nil
}

func getVersionFiles(globalConfig *GlobalConfig, projectsConfig *ProjectsConfig) ([]string, error) {
	projectName := strings.Replace(filepath.Base(projectsConfig.Path), "-", "_", -1)
	var versionFiles []string

	languageConfig, exists := globalConfig.LanguagesConfig[projectsConfig.Language]
	if !exists {
		return nil, errors.New(fmt.Sprintf("Language %s not found in config", language))
	}

	for _, versionFile := range languageConfig.VersionFiles {
		versionFiles = append(
			versionFiles, filepath.Join(
				projectsConfig.Path, strings.ReplaceAll(versionFile, "{project_name}", projectName),
			),
		)
	}
	return versionFiles, nil
}

func getVersionPattern(globalConfig *GlobalConfig, projectsConfig *ProjectsConfig) (string, error) {
	versionPattern, exists := globalConfig.LanguagesConfig[projectsConfig.Language]
	if !exists {
		return "", errors.New(fmt.Sprintf("Language %s not found in config", language))
	}
	return versionPattern.VersionPattern, nil
}
