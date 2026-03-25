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
