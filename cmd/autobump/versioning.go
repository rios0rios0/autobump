package main

import (
	"errors"
	"fmt"
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
		info, err := os.Stat(versionFile.Path)
		if os.IsNotExist(err) {
			log.Warnf("Version file %s does not exist", versionFile.Path)
			continue
		}
		log.Infof("Updating version file %s", versionFile.Path)

		originalFileMode := info.Mode()
		oneVersionFileExists = true

		content, err := os.ReadFile(versionFile.Path)
		if err != nil {
			return err
		}

		updatedContent := string(content)
		for _, pattern := range versionFile.Patterns {
			re := regexp.MustCompile(pattern)
			updatedContent = re.ReplaceAllStringFunc(updatedContent, func(match string) string {
				return re.ReplaceAllString(match, "${1}"+projectConfig.NewVersion+"${2}")
			})
		}

		err = os.WriteFile(versionFile.Path, []byte(updatedContent), originalFileMode)
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
// as well as the regex pattern to find the version number in the file.
func getVersionFiles(
	globalConfig *GlobalConfig,
	projectConfig *ProjectConfig,
) ([]VersionFile, error) {
	if projectConfig.Name == "" {
		projectConfig.Name = filepath.Base(projectConfig.Path)
	}
	projectName := strings.Replace(projectConfig.Name, "-", "_", -1)
	var versionFiles []VersionFile

	languageConfig, exists := globalConfig.LanguagesConfig[projectConfig.Language]
	if !exists {
		return nil, errors.New(fmt.Sprintf("Language %s not found in config", language))
	}

	for _, versionFile := range languageConfig.VersionFiles {
		matches, err := filepath.Glob(
			filepath.Join(
				projectConfig.Path,
				strings.ReplaceAll(versionFile.Path, "{project_name}", projectName),
			),
		)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			versionFiles = append(
				versionFiles, VersionFile{
					Path:     match,
					Patterns: versionFile.Patterns,
				},
			)
		}
	}
	return versionFiles, nil
}
