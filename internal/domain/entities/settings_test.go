//go:build unit

package entities_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

func TestMergeLanguagesConfig(t *testing.T) {
	t.Parallel()

	defaults := map[string]entities.LanguageConfig{
		"typescript": {
			Extensions:      []string{"ts"},
			SpecialPatterns: []string{"package.json", "tsconfig.json", "yarn.lock"},
			VersionFiles: []entities.VersionFile{
				{Path: "package.json", Patterns: []string{`(\s*"version":\s*")\d+\.\d+\.\d+(",)`}},
			},
		},
		"golang": {
			Extensions:      []string{"go"},
			SpecialPatterns: []string{"go.mod"},
		},
		"python": {
			Extensions:      []string{"py"},
			SpecialPatterns: []string{"pyproject.toml", "setup.cfg", "setup.py"},
			VersionFiles: []entities.VersionFile{
				{Path: "{project_name}/__init__.py", Patterns: []string{`(__version__\s*=\s*")\d+\.\d+\.\d+(")`}},
			},
		},
	}

	t.Run("should keep all defaults when user config is empty map", func(t *testing.T) {
		// given
		overrides := map[string]entities.LanguageConfig{}

		// when
		result := entities.MergeLanguagesConfig(defaults, overrides)

		// then
		assert.Equal(t, defaults, result)
	})

	t.Run("should append user version files to default version files", func(t *testing.T) {
		// given
		overrides := map[string]entities.LanguageConfig{
			"typescript": {
				VersionFiles: []entities.VersionFile{
					{Path: "opensearch_dashboards.json", Patterns: []string{`(\s*"version":\s*")\d+\.\d+\.\d+(",)`}},
				},
			},
		}

		// when
		result := entities.MergeLanguagesConfig(defaults, overrides)

		// then
		ts := result["typescript"]
		assert.Equal(t, []string{"ts"}, ts.Extensions)
		assert.Equal(t, []string{"package.json", "tsconfig.json", "yarn.lock"}, ts.SpecialPatterns)
		assert.Len(t, ts.VersionFiles, 2)
		assert.Equal(t, "package.json", ts.VersionFiles[0].Path)
		assert.Equal(t, "opensearch_dashboards.json", ts.VersionFiles[1].Path)
	})

	t.Run("should override default version file when user provides same path", func(t *testing.T) {
		// given
		customPattern := `(\s*"version":\s*")\d+\.\d+\.\d+\.\d+(",)`
		overrides := map[string]entities.LanguageConfig{
			"typescript": {
				VersionFiles: []entities.VersionFile{
					{Path: "package.json", Patterns: []string{customPattern}},
				},
			},
		}

		// when
		result := entities.MergeLanguagesConfig(defaults, overrides)

		// then
		ts := result["typescript"]
		assert.Len(t, ts.VersionFiles, 1)
		assert.Equal(t, "package.json", ts.VersionFiles[0].Path)
		assert.Equal(t, []string{customPattern}, ts.VersionFiles[0].Patterns)
	})

	t.Run("should keep default extensions when user provides only version files", func(t *testing.T) {
		// given
		overrides := map[string]entities.LanguageConfig{
			"typescript": {
				VersionFiles: []entities.VersionFile{
					{Path: "manifest.json", Patterns: []string{`("version":\s*")\d+\.\d+\.\d+(")`}},
				},
			},
		}

		// when
		result := entities.MergeLanguagesConfig(defaults, overrides)

		// then
		ts := result["typescript"]
		assert.Equal(t, []string{"ts"}, ts.Extensions)
		assert.Equal(t, []string{"package.json", "tsconfig.json", "yarn.lock"}, ts.SpecialPatterns)
	})

	t.Run("should add new language not present in defaults", func(t *testing.T) {
		// given
		overrides := map[string]entities.LanguageConfig{
			"ruby": {
				Extensions:      []string{"rb"},
				SpecialPatterns: []string{"Gemfile"},
			},
		}

		// when
		result := entities.MergeLanguagesConfig(defaults, overrides)

		// then
		assert.Contains(t, result, "ruby")
		assert.Equal(t, []string{"rb"}, result["ruby"].Extensions)
		assert.Contains(t, result, "typescript")
		assert.Contains(t, result, "golang")
		assert.Contains(t, result, "python")
	})

	t.Run("should keep default language untouched when not in user config", func(t *testing.T) {
		// given
		overrides := map[string]entities.LanguageConfig{
			"typescript": {
				VersionFiles: []entities.VersionFile{
					{Path: "extra.json", Patterns: []string{`("version":\s*")\d+\.\d+\.\d+(")`}},
				},
			},
		}

		// when
		result := entities.MergeLanguagesConfig(defaults, overrides)

		// then
		assert.Equal(t, defaults["golang"], result["golang"])
		assert.Equal(t, defaults["python"], result["python"])
	})

	t.Run("should deduplicate extensions when user repeats default values", func(t *testing.T) {
		// given
		overrides := map[string]entities.LanguageConfig{
			"typescript": {
				Extensions: []string{"ts", "tsx"},
			},
		}

		// when
		result := entities.MergeLanguagesConfig(defaults, overrides)

		// then
		ts := result["typescript"]
		assert.Equal(t, []string{"ts", "tsx"}, ts.Extensions)
	})
}
