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

// updateVersion updates the version in the version files.
// This function fails fast upon the first error.
func updateVersion(path string, globalConfig *GlobalConfig, projectConfig *ProjectConfig) error {
	versionFiles, err := getVersionFiles(globalConfig, projectConfig)
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

		versionPattern, err := getVersionPattern(globalConfig, projectConfig)
		if err != nil {
			return err
		}

		re := regexp.MustCompile(versionPattern)
		updatedContent := re.ReplaceAllStringFunc(string(content), func(match string) string {
			return re.ReplaceAllString(match, "${1}"+projectConfig.NewVersion+"${2}")
		})

		err = ioutil.WriteFile(versionFile, []byte(updatedContent), originalFileMode)
		if err != nil {
			return err
		}
	}

	if !oneVersionFileExists {
		return errors.New(fmt.Sprintf("No version file found for %s", projectConfig.Language))
	}

	return nil
}

// getVersionFiles returns the files in a project that contains the software's version number
func getVersionFiles(globalConfig *GlobalConfig, projectConfig *ProjectConfig) ([]string, error) {
	if projectConfig.Name == "" {
		projectConfig.Name = filepath.Base(projectConfig.Path)
	}
	projectName := strings.Replace(projectConfig.Name, "-", "_", -1)
	var versionFiles []string

	languageConfig, exists := globalConfig.LanguagesConfig[projectConfig.Language]
	if !exists {
		return nil, errors.New(fmt.Sprintf("Language %s not found in config", language))
	}

	for _, versionFile := range languageConfig.VersionFiles {
		versionFiles = append(
			versionFiles, filepath.Join(
				projectConfig.Path, strings.ReplaceAll(versionFile, "{project_name}", projectName),
			),
		)
	}
	return versionFiles, nil
}

// getVersionPattern returns the pattern that matches the version number in the version files
func getVersionPattern(globalConfig *GlobalConfig, projectConfig *ProjectConfig) (string, error) {
	versionPattern, exists := globalConfig.LanguagesConfig[projectConfig.Language]
	if !exists {
		return "", errors.New(fmt.Sprintf("Language %s not found in config", language))
	}
	return versionPattern.VersionPattern, nil
}
