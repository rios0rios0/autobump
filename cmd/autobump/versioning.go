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

var (
	ErrNoVersionFileFound       = errors.New("no version file found")
	ErrLanguageNotFoundInConfig = errors.New("language not found in config")
)

// updateVersion updates the version in the version files.
// This function fails fast upon the first error.
// If the language is empty (unknown), it skips version file updates and only updates the changelog.
// If no version files are found for a detected language, it logs a warning and continues.
func updateVersion(globalConfig *GlobalConfig, projectConfig *ProjectConfig) error {
	// If language is empty/unknown, skip version file updates
	if projectConfig.Language == "" {
		log.Info("Language is unknown, skipping version file updates (only changelog will be updated)")
		return nil
	}

	versionFiles, err := getVersionFiles(globalConfig, projectConfig)
	if err != nil {
		// If language config not found, just warn and continue with changelog only
		if errors.Is(err, ErrLanguageNotFoundInConfig) {
			log.Warnf("Language '%s' not found in config, skipping version file updates", projectConfig.Language)
			return nil
		}
		return err
	}

	// If no version files configured for this language, just continue
	if len(versionFiles) == 0 {
		log.Warnf("No version files configured for language '%s', only changelog will be updated", projectConfig.Language)
		return nil
	}

	oneVersionFileExists := false
	for _, versionFile := range versionFiles {
		// check if the file exists
		var info os.FileInfo
		info, err = os.Stat(versionFile.Path)
		if os.IsNotExist(err) {
			log.Warnf("Version file %s does not exist", versionFile.Path)
			continue
		}
		log.Infof("Updating version file %s", versionFile.Path)

		originalFileMode := info.Mode()
		oneVersionFileExists = true

		var content []byte
		content, err = os.ReadFile(versionFile.Path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", versionFile.Path, err)
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
			return fmt.Errorf("failed to write to file %s: %w", versionFile.Path, err)
		}
	}

	// If no version files exist, just warn and continue (don't fail)
	if !oneVersionFileExists {
		log.Warnf("No version files found for language '%s', only changelog will be updated", projectConfig.Language)
	}

	return nil
}

// getVersionFiles returns the files in a project that contains the software's version number
// as well as the regex pattern to find the version number in the file.
func getVersionFiles(
	globalConfig *GlobalConfig,
	projectConfig *ProjectConfig,
) ([]VersionFile, error) {
	// If language is empty/unknown, return empty list
	if projectConfig.Language == "" {
		return []VersionFile{}, nil
	}

	if projectConfig.Name == "" {
		projectConfig.Name = filepath.Base(projectConfig.Path)
	}
	projectName := strings.ReplaceAll(projectConfig.Name, "-", "_")
	var versionFiles []VersionFile

	// try to get the project name from the language interface
	var languageInterface Language
	getLanguageInterface(*projectConfig, &languageInterface)

	if languageInterface != nil {
		languageProjectName, err := languageInterface.GetProjectName()
		if err == nil && languageProjectName != "" {
			log.Infof("Using project name '%s' from language interface", languageProjectName)
			projectName = strings.ReplaceAll(languageProjectName, "-", "_")
		}
	} else {
		log.Infof("Language '%s' does not have a language interface", projectConfig.Language)
	}

	languageConfig, exists := globalConfig.LanguagesConfig[projectConfig.Language]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrLanguageNotFoundInConfig, projectConfig.Language)
	}

	for _, versionFile := range languageConfig.VersionFiles {
		matches, err := filepath.Glob(
			filepath.Join(
				projectConfig.Path,
				strings.ReplaceAll(versionFile.Path, "{project_name}", projectName),
			),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get version files: %w", err)
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
