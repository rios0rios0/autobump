//go:build unit

package commands_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/test/domain/entitybuilders"
)

func TestUpdateVersion(t *testing.T) {
	t.Parallel()

	t.Run("should update only project version in pom.xml when version has SNAPSHOT suffix", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		pomContent := `<?xml version="1.0" encoding="UTF-8"?>
<project>
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>my-app</artifactId>
    <version>0.0.1-SNAPSHOT</version>
    <dependencies>
        <dependency>
            <groupId>org.apache.httpcomponents</groupId>
            <artifactId>httpclient</artifactId>
            <version>4.5.13</version>
        </dependency>
        <dependency>
            <groupId>junit</groupId>
            <artifactId>junit</artifactId>
            <version>4.13.2</version>
        </dependency>
    </dependencies>
</project>`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "pom.xml"), []byte(pomContent), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"java": {
					VersionFiles: []entities.VersionFile{
						{Path: "pom.xml", Patterns: []string{`(\s*<version>)[^<]+(</version>)`}},
					},
				},
			}).BuildGlobalConfig()

		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(tmpDir).
			WithLanguage("java").
			WithNewVersion("0.1.0").
			BuildProjectConfig()

		// when
		err := commands.UpdateVersion(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		result, err := os.ReadFile(filepath.Join(tmpDir, "pom.xml"))
		require.NoError(t, err)
		content := string(result)
		assert.Contains(t, content, "<version>0.1.0</version>")
		assert.Contains(t, content, "<version>4.5.13</version>")
		assert.Contains(t, content, "<version>4.13.2</version>")
	})

	t.Run("should update project version and not parent version in pom.xml when parent block exists", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		pomContent := `<?xml version="1.0" encoding="UTF-8"?>
<project>
    <parent>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-starter-parent</artifactId>
        <version>3.2.0</version>
        <relativePath/>
    </parent>
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>my-app</artifactId>
    <version>0.0.1-SNAPSHOT</version>
    <dependencies>
        <dependency>
            <groupId>org.projectlombok</groupId>
            <artifactId>lombok</artifactId>
            <version>1.18.34</version>
        </dependency>
    </dependencies>
</project>`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "pom.xml"), []byte(pomContent), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"java": {
					VersionFiles: []entities.VersionFile{
						{Path: "pom.xml", Patterns: []string{`(\s*<version>)[^<]+(</version>)`}},
					},
				},
			}).BuildGlobalConfig()

		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(tmpDir).
			WithLanguage("java").
			WithNewVersion("0.1.0").
			BuildProjectConfig()

		// when
		err := commands.UpdateVersion(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		result, err := os.ReadFile(filepath.Join(tmpDir, "pom.xml"))
		require.NoError(t, err)
		content := string(result)
		assert.Contains(t, content, "<version>3.2.0</version>")
		assert.Contains(t, content, "<version>0.1.0</version>")
		assert.Contains(t, content, "<version>1.18.34</version>")
		assert.NotContains(t, content, "<version>0.0.1-SNAPSHOT</version>")
	})

	t.Run("should update only project version in pom.xml when dependencies have clean semver", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		pomContent := `<?xml version="1.0" encoding="UTF-8"?>
<project>
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>my-app</artifactId>
    <version>1.0.0</version>
    <build>
        <plugins>
            <plugin>
                <artifactId>maven-compiler-plugin</artifactId>
                <version>3.13.0</version>
            </plugin>
        </plugins>
    </build>
    <dependencies>
        <dependency>
            <groupId>org.projectlombok</groupId>
            <artifactId>lombok</artifactId>
            <version>1.18.34</version>
        </dependency>
    </dependencies>
</project>`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "pom.xml"), []byte(pomContent), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"java": {
					VersionFiles: []entities.VersionFile{
						{Path: "pom.xml", Patterns: []string{`(\s*<version>)[^<]+(</version>)`}},
					},
				},
			}).BuildGlobalConfig()

		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(tmpDir).
			WithLanguage("java").
			WithNewVersion("1.1.0").
			BuildProjectConfig()

		// when
		err := commands.UpdateVersion(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		result, err := os.ReadFile(filepath.Join(tmpDir, "pom.xml"))
		require.NoError(t, err)
		content := string(result)
		assert.Contains(t, content, "<version>1.1.0</version>")
		assert.Contains(t, content, "<version>3.13.0</version>")
		assert.Contains(t, content, "<version>1.18.34</version>")
	})

	t.Run("should update project version and not plugin version in build.gradle", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		gradleContent := `plugins {
    id 'java'
    id 'com.example.plugin' version '1.0.0'
}

group 'com.example'
version '0.0.1'

repositories {
    mavenCentral()
}
`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "build.gradle"), []byte(gradleContent), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"java": {
					VersionFiles: []entities.VersionFile{
						{Path: "build.gradle", Patterns: []string{`(?m)(^\s*version\s*[=:]?\s*["'])[^"']+(["'])`}},
					},
				},
			}).BuildGlobalConfig()

		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(tmpDir).
			WithLanguage("java").
			WithNewVersion("0.1.0").
			BuildProjectConfig()

		// when
		err := commands.UpdateVersion(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		result, err := os.ReadFile(filepath.Join(tmpDir, "build.gradle"))
		require.NoError(t, err)
		content := string(result)
		assert.Contains(t, content, "version '0.1.0'")
		assert.Contains(t, content, "id 'com.example.plugin' version '1.0.0'")
	})

	t.Run("should update version in build.gradle when using single quotes and no equals sign", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		gradleContent := `plugins {
    id 'java'
    id 'application'
}

group 'com.example'
version '0.0.1'

repositories {
    mavenCentral()
}
`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "build.gradle"), []byte(gradleContent), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"java": {
					VersionFiles: []entities.VersionFile{
						{Path: "build.gradle", Patterns: []string{`(?m)(^\s*version\s*[=:]?\s*["'])[^"']+(["'])`}},
					},
				},
			}).BuildGlobalConfig()

		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(tmpDir).
			WithLanguage("java").
			WithNewVersion("0.1.0").
			BuildProjectConfig()

		// when
		err := commands.UpdateVersion(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		result, err := os.ReadFile(filepath.Join(tmpDir, "build.gradle"))
		require.NoError(t, err)
		content := string(result)
		assert.Contains(t, content, "version '0.1.0'")
	})

	t.Run("should update version in build.gradle when using double quotes and no equals sign", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		gradleContent := `plugins {
    id 'java'
}

group "com.example"
version "0.0.1"

repositories {
    mavenCentral()
}
`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "build.gradle"), []byte(gradleContent), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"java": {
					VersionFiles: []entities.VersionFile{
						{Path: "build.gradle", Patterns: []string{`(?m)(^\s*version\s*[=:]?\s*["'])[^"']+(["'])`}},
					},
				},
			}).BuildGlobalConfig()

		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(tmpDir).
			WithLanguage("java").
			WithNewVersion("0.1.0").
			BuildProjectConfig()

		// when
		err := commands.UpdateVersion(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		result, err := os.ReadFile(filepath.Join(tmpDir, "build.gradle"))
		require.NoError(t, err)
		content := string(result)
		assert.Contains(t, content, `version "0.1.0"`)
	})

	t.Run("should update version and not appVersion in Chart.yaml", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		chartContent := `apiVersion: v2
name: my-chart
description: A Helm chart for Kubernetes
type: application
version: 1.2.3
appVersion: "1.0.0"
`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "Chart.yaml"), []byte(chartContent), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"helm": {
					VersionFiles: []entities.VersionFile{
						{Path: "Chart.yaml", Patterns: []string{`(?m)(^version:\s*['"]?)[^\s'"]+(['"]?)`}},
					},
				},
			}).BuildGlobalConfig()

		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(tmpDir).
			WithLanguage("helm").
			WithNewVersion("1.3.0").
			BuildProjectConfig()

		// when
		err := commands.UpdateVersion(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		result, err := os.ReadFile(filepath.Join(tmpDir, "Chart.yaml"))
		require.NoError(t, err)
		content := string(result)
		assert.Contains(t, content, "version: 1.3.0")
		assert.Contains(t, content, `appVersion: "1.0.0"`)
		assert.NotContains(t, content, "version: 1.2.3")
	})

	t.Run("should update quoted version in Chart.yaml", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		chartContent := `apiVersion: v2
name: my-chart
description: A Helm chart for Kubernetes
type: application
version: '1.2.3'
appVersion: '1.0.0'
`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "Chart.yaml"), []byte(chartContent), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"helm": {
					VersionFiles: []entities.VersionFile{
						{Path: "Chart.yaml", Patterns: []string{`(?m)(^version:\s*['"]?)[^\s'"]+(['"]?)`}},
					},
				},
			}).BuildGlobalConfig()

		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(tmpDir).
			WithLanguage("helm").
			WithNewVersion("1.3.0").
			BuildProjectConfig()

		// when
		err := commands.UpdateVersion(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		result, err := os.ReadFile(filepath.Join(tmpDir, "Chart.yaml"))
		require.NoError(t, err)
		content := string(result)
		assert.Contains(t, content, "version: '1.3.0'")
		assert.Contains(t, content, "appVersion: '1.0.0'")
	})

	t.Run("should update pre-release version in Chart.yaml", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		chartContent := `apiVersion: v2
name: my-chart
description: A Helm chart for Kubernetes
type: application
version: 1.2.3-rc.1+build.5
appVersion: "1.0.0-beta.2"
`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "Chart.yaml"), []byte(chartContent), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"helm": {
					VersionFiles: []entities.VersionFile{
						{Path: "Chart.yaml", Patterns: []string{`(?m)(^version:\s*['"]?)[^\s'"]+(['"]?)`}},
					},
				},
			}).BuildGlobalConfig()

		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(tmpDir).
			WithLanguage("helm").
			WithNewVersion("1.3.0").
			BuildProjectConfig()

		// when
		err := commands.UpdateVersion(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		result, err := os.ReadFile(filepath.Join(tmpDir, "Chart.yaml"))
		require.NoError(t, err)
		content := string(result)
		assert.Contains(t, content, "version: 1.3.0")
		assert.NotContains(t, content, "1.2.3-rc.1")
		assert.Contains(t, content, `appVersion: "1.0.0-beta.2"`)
	})

	t.Run("should skip version file updates when language is empty", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{}).
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithLanguage("").
			WithNewVersion("1.0.0").
			BuildProjectConfig()

		// when
		err := commands.UpdateVersion(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
	})

	t.Run("should warn and continue when language not found in config", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{}).
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithLanguage("unknown_language").
			WithNewVersion("1.0.0").
			BuildProjectConfig()

		// when
		err := commands.UpdateVersion(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
	})

	t.Run("should warn when no version files configured for language", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"ruby": {
					Extensions:   []string{"rb"},
					VersionFiles: []entities.VersionFile{},
				},
			}).BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithLanguage("ruby").
			WithNewVersion("1.0.0").
			BuildProjectConfig()

		// when
		err := commands.UpdateVersion(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
	})

	t.Run("should warn when version files do not exist", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"ruby": {
					Extensions: []string{"rb"},
					VersionFiles: []entities.VersionFile{
						{Path: "nonexistent.rb", Patterns: []string{`(version\s*=\s*')\d+\.\d+\.\d+(')`}},
					},
				},
			}).BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(tmpDir).
			WithLanguage("ruby").
			WithNewVersion("1.0.0").
			BuildProjectConfig()

		// when
		err := commands.UpdateVersion(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
	})

	t.Run("should update version in build.gradle when using equals sign and single quotes", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		gradleContent := `plugins {
    id 'java'
}

group = 'com.example'
version = '0.0.1'

repositories {
    mavenCentral()
}
`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "build.gradle"), []byte(gradleContent), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"java": {
					VersionFiles: []entities.VersionFile{
						{Path: "build.gradle", Patterns: []string{`(?m)(^\s*version\s*[=:]?\s*["'])[^"']+(["'])`}},
					},
				},
			}).BuildGlobalConfig()

		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(tmpDir).
			WithLanguage("java").
			WithNewVersion("0.1.0").
			BuildProjectConfig()

		// when
		err := commands.UpdateVersion(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		result, err := os.ReadFile(filepath.Join(tmpDir, "build.gradle"))
		require.NoError(t, err)
		content := string(result)
		assert.Contains(t, content, "version = '0.1.0'")
	})
}

// goSwaggerLanguagesConfig mirrors the default Go version_files from configs/autobump.yaml,
// where the version lives in the swaggo "@version" annotation and in the generated docs.
func goSwaggerLanguagesConfig() map[string]entities.LanguageConfig {
	annotationPatterns := []string{`(?m)(^//\s*@version\s+)\S+`}
	docsGoPatterns := []string{`(\bVersion:\s*")[^"]*(")`}
	swaggerJSONPatterns := []string{`(?s)("info":\s*\{.*?"version":\s*")[^"]*(")`}
	swaggerYAMLPatterns := []string{`(?m)(^  version:\s*['"]?)[^\s'"]+(['"]?)`}
	return map[string]entities.LanguageConfig{
		"go": {
			Extensions:      []string{"go"},
			SpecialPatterns: []string{"go.mod"},
			VersionFiles: []entities.VersionFile{
				{Path: "main.go", Patterns: annotationPatterns},
				{Path: "cmd/main.go", Patterns: annotationPatterns},
				{Path: "cmd/*/main.go", Patterns: annotationPatterns},
				{Path: "docs/docs.go", Patterns: docsGoPatterns},
				{Path: "cmd/docs/docs.go", Patterns: docsGoPatterns},
				{Path: "cmd/*/docs/docs.go", Patterns: docsGoPatterns},
				{Path: "docs/swagger.json", Patterns: swaggerJSONPatterns},
				{Path: "cmd/docs/swagger.json", Patterns: swaggerJSONPatterns},
				{Path: "cmd/*/docs/swagger.json", Patterns: swaggerJSONPatterns},
				{Path: "docs/swagger.yaml", Patterns: swaggerYAMLPatterns},
				{Path: "cmd/docs/swagger.yaml", Patterns: swaggerYAMLPatterns},
				{Path: "cmd/*/docs/swagger.yaml", Patterns: swaggerYAMLPatterns},
			},
		},
	}
}

// writeGoSwaggerProject writes a swaggo-documented Go project fixture. The entrypoint
// carrying the "@version" annotation goes to mainRelPath and the generated docs go to
// docsRelDir, so tests can exercise both the root and the cmd layouts.
func writeGoSwaggerProject(t *testing.T, projectPath, mainRelPath, docsRelDir string) {
	t.Helper()

	mainGoContent := `package main

// @title Example API
// @version 1.2.3
// @description Example service used in tests.
func main() {}
`
	docsGoContent := "// Package docs Code generated by swaggo/swag. DO NOT EDIT\n" +
		"package docs\n\n" +
		"import \"github.com/swaggo/swag\"\n\n" +
		"const docTemplate = `{\n" +
		"    \"swagger\": \"2.0\",\n" +
		"    \"info\": {\n" +
		"        \"title\": \"{{escape .Title}}\",\n" +
		"        \"version\": \"{{escape .Version}}\"\n" +
		"    }\n" +
		"}`\n\n" +
		"var SwaggerInfo = &swag.Spec{\n" +
		"\tVersion:          \"1.2.3\",\n" +
		"\tHost:             \"\",\n" +
		"\tBasePath:         \"/\",\n" +
		"\tTitle:            \"Example API\",\n" +
		"\tSwaggerTemplate:  docTemplate,\n" +
		"}\n"
	swaggerJSONContent := `{
    "swagger": "2.0",
    "info": {
        "description": "Example service used in tests.",
        "title": "Example API",
        "version": "1.2.3"
    },
    "paths": {},
    "definitions": {
        "entities.Widget": {
            "example": {
                "version": "9.9.9"
            },
            "properties": {
                "version": {
                    "type": "string"
                }
            }
        }
    }
}
`
	swaggerYAMLContent := `basePath: /
definitions:
  entities.Widget:
    properties:
      version:
        type: string
    type: object
info:
  contact: {}
  description: Example service used in tests.
  title: Example API
  version: 1.2.3
paths: {}
swagger: "2.0"
`

	docsDir := filepath.Join(projectPath, docsRelDir)
	// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(projectPath, mainRelPath)), 0o700))
	// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
	require.NoError(t, os.MkdirAll(docsDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(projectPath, mainRelPath), []byte(mainGoContent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "docs.go"), []byte(docsGoContent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "swagger.json"), []byte(swaggerJSONContent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "swagger.yaml"), []byte(swaggerYAMLContent), 0o644))
}

// assertGoSwaggerProjectBumped verifies that all Swagger version markers were rewritten
// to the new version and that the decoy occurrences were left untouched.
func assertGoSwaggerProjectBumped(t *testing.T, projectPath, mainRelPath, docsRelDir string) {
	t.Helper()

	mainGo, err := os.ReadFile(filepath.Join(projectPath, mainRelPath))
	require.NoError(t, err)
	assert.Contains(t, string(mainGo), "// @version 2.0.0")
	assert.NotContains(t, string(mainGo), "1.2.3")

	docsGo, err := os.ReadFile(filepath.Join(projectPath, docsRelDir, "docs.go"))
	require.NoError(t, err)
	assert.Contains(t, string(docsGo), `Version:          "2.0.0"`)
	assert.Contains(t, string(docsGo), "{{escape .Version}}", "the docs.go template placeholder must not be touched")
	assert.NotContains(t, string(docsGo), "1.2.3")

	swaggerJSON, err := os.ReadFile(filepath.Join(projectPath, docsRelDir, "swagger.json"))
	require.NoError(t, err)
	assert.Contains(t, string(swaggerJSON), `"version": "2.0.0"`)
	assert.Contains(t, string(swaggerJSON), `"version": "9.9.9"`,
		"the string-valued version inside definitions must not be touched")
	assert.NotContains(t, string(swaggerJSON), "1.2.3")

	swaggerYAML, err := os.ReadFile(filepath.Join(projectPath, docsRelDir, "swagger.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(swaggerYAML), "  version: 2.0.0")
	assert.Contains(t, string(swaggerYAML), "      version:", "the definition property named version must not be touched")
	assert.NotContains(t, string(swaggerYAML), "1.2.3")
}

// newGoSwaggerBumpConfigs builds the global/project config pair shared by the Go
// Swagger bump scenarios: default Go version files, language "go", bump to 2.0.0.
func newGoSwaggerBumpConfigs(projectPath string) (*entities.GlobalConfig, *entities.ProjectConfig) {
	globalConfig := entitybuilders.NewGlobalConfigBuilder().
		WithLanguagesConfig(goSwaggerLanguagesConfig()).
		BuildGlobalConfig()
	projectConfig := entitybuilders.NewProjectConfigBuilder().
		WithPath(projectPath).
		WithLanguage("go").
		WithNewVersion("2.0.0").
		BuildProjectConfig()
	return globalConfig, projectConfig
}

func TestUpdateVersionGoSwagger(t *testing.T) {
	t.Parallel()

	layouts := []struct {
		name        string
		mainRelPath string
		docsRelDir  string
	}{
		{name: "docs at the root", mainRelPath: "main.go", docsRelDir: "docs"},
		{name: "docs under cmd", mainRelPath: filepath.Join("cmd", "main.go"), docsRelDir: filepath.Join("cmd", "docs")},
	}
	for _, layout := range layouts {
		t.Run("should update Swagger annotation and generated docs when Go project keeps "+layout.name, func(t *testing.T) {
			// given
			tmpDir := t.TempDir()
			writeGoSwaggerProject(t, tmpDir, layout.mainRelPath, layout.docsRelDir)
			globalConfig, projectConfig := newGoSwaggerBumpConfigs(tmpDir)

			// when
			err := commands.UpdateVersion(globalConfig, projectConfig)

			// then
			require.NoError(t, err)
			assertGoSwaggerProjectBumped(t, tmpDir, layout.mainRelPath, layout.docsRelDir)
		})
	}

	t.Run("should update tab-separated Swagger annotation when swag fmt formatting is used", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		mainGoContent := "package main\n\n//\t@title\t\tExample API\n//\t@version\t1.2.3\nfunc main() {}\n"
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGoContent), 0o644))
		globalConfig, projectConfig := newGoSwaggerBumpConfigs(tmpDir)

		// when
		err := commands.UpdateVersion(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		result, err := os.ReadFile(filepath.Join(tmpDir, "main.go"))
		require.NoError(t, err)
		assert.Contains(t, string(result), "//\t@version\t2.0.0")
	})

	t.Run("should keep main.go untouched when Go project has no Swagger annotation", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		mainGoContent := "package main\n\nfunc main() {}\n"
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGoContent), 0o644))
		globalConfig, projectConfig := newGoSwaggerBumpConfigs(tmpDir)

		// when
		err := commands.UpdateVersion(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		result, err := os.ReadFile(filepath.Join(tmpDir, "main.go"))
		require.NoError(t, err)
		assert.Equal(t, mainGoContent, string(result))
	})
}

func TestGetVersionFiles(t *testing.T) {
	t.Parallel()

	t.Run("should return only files whose content matches a version pattern when globbing", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "docs"), 0o700))
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, "main.go"),
			[]byte("package main\n\nfunc main() {}\n"),
			0o644,
		))
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, "docs", "swagger.json"),
			[]byte(`{"info": {"title": "Example API", "version": "1.2.3"}}`),
			0o644,
		))
		globalConfig, projectConfig := newGoSwaggerBumpConfigs(tmpDir)

		// when
		versionFiles, err := commands.GetVersionFiles(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		require.Len(t, versionFiles, 1)
		assert.Equal(t, filepath.Join(tmpDir, "docs", "swagger.json"), versionFiles[0].Path)
	})

	t.Run("should return an error when a version pattern is invalid", func(t *testing.T) {
		// given
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "version.rb"), []byte("version = '1.2.3'\n"), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"ruby": {
					Extensions: []string{"rb"},
					VersionFiles: []entities.VersionFile{
						{Path: "version.rb", Patterns: []string{"("}},
					},
				},
			}).BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(tmpDir).
			WithLanguage("ruby").
			BuildProjectConfig()

		// when
		versionFiles, err := commands.GetVersionFiles(globalConfig, projectConfig)

		// then
		require.Error(t, err)
		assert.Nil(t, versionFiles)
		assert.Contains(t, err.Error(), "invalid regex pattern")
	})
}
