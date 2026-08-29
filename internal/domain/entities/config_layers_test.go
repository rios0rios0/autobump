package entities_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/entities"
)

// operatorLayer builds the layer the operator's own configuration is applied as: the only
// one that may name a credential, and the only one decoded strictly.
func operatorLayer(content string) entities.ConfigLayer {
	//nolint:exhaustruct // Optional stays false: a file the operator named must be read
	return entities.ConfigLayer{
		Name:   entities.LayerOperatorConfig,
		Origin: "/home/operator/.autobump.yaml",
		Data:   []byte(content),
		Scope:  entities.ScopeOperator,
		Strict: true,
	}
}

// defaultsLayer builds the built-in-defaults layer.
func defaultsLayer(content string) entities.ConfigLayer {
	//nolint:exhaustruct // the built-in defaults have no origin an operator can look at
	return entities.ConfigLayer{
		Name:   entities.LayerBuiltInDefaults,
		Data:   []byte(content),
		Scope:  entities.ScopeRestricted,
		Strict: true,
	}
}

func TestApplyLayer(t *testing.T) {
	t.Parallel()

	t.Run("should leave a key the layer omits exactly as it was", func(t *testing.T) {
		t.Parallel()

		// given
		base := &entities.GlobalConfig{Versioning: "fork-dot", GitHubAccessToken: "ghp_base"}

		// when
		result, err := entities.ApplyLayer(base, operatorLayer("changelog_path: 'DOCS.md'\n"))

		// then
		require.NoError(t, err)
		assert.Equal(t, "DOCS.md", result.ChangelogPath)
		assert.Equal(t, "fork-dot", result.Versioning, "an omitted key is not a decision")
		assert.Equal(t, "ghp_base", result.GitHubAccessToken)
	})

	t.Run("should let a later layer set false over an earlier true", func(t *testing.T) {
		t.Parallel()

		// given -- this is the property that makes pointers unnecessary for scalars:
		// yaml.v3 writes only the keys the document carries, so "absent" and "false" stay
		// distinguishable without any field changing type
		enabled := true
		base := &entities.GlobalConfig{ExcludeForks: true, DetectChlog: &enabled}

		// when
		result, err := entities.ApplyLayer(
			base, operatorLayer("exclude_forks: false\ndetect_chlog: false\n"),
		)

		// then
		require.NoError(t, err)
		assert.False(t, result.ExcludeForks)
		require.NotNil(t, result.DetectChlog)
		assert.False(t, *result.DetectChlog)
	})

	t.Run("should treat a comments-only document as a layer with nothing to say", func(t *testing.T) {
		t.Parallel()

		// given -- the shipped defaults are nearly all comments, so an empty decode
		// reporting io.EOF must not read as a broken layer
		base := &entities.GlobalConfig{Versioning: "fork-dash"}

		// when
		result, err := entities.ApplyLayer(base, defaultsLayer("# nothing but a comment\n"))

		// then
		require.NoError(t, err)
		assert.Equal(t, "fork-dash", result.Versioning)
	})

	t.Run("should replace a slice wholesale", func(t *testing.T) {
		t.Parallel()

		// given
		base := &entities.GlobalConfig{
			Projects: []entities.ProjectConfig{{Path: "/old/one"}, {Path: "/old/two"}},
		}

		// when
		result, err := entities.ApplyLayer(base, operatorLayer("projects:\n  - path: '/new'\n"))

		// then
		require.NoError(t, err)
		require.Len(t, result.Projects, 1)
		assert.Equal(t, "/new", result.Projects[0].Path)
	})

	t.Run("should deep-merge languages instead of replacing them", func(t *testing.T) {
		t.Parallel()

		// given -- yaml.v3 decodes each map value into a fresh zero element, so without the
		// re-merge this layer would drop go's extensions along with its version files
		base := &entities.GlobalConfig{
			LanguagesConfig: map[string]entities.LanguageConfig{
				"go":     {Extensions: []string{"go"}, VersionFiles: []entities.VersionFile{{Path: "main.go"}}},
				"python": {Extensions: []string{"py"}},
			},
		}

		// when
		result, err := entities.ApplyLayer(base, operatorLayer(
			"languages:\n  go:\n    version_files:\n      - path: 'docs/docs.go'\n",
		))

		// then
		require.NoError(t, err)
		assert.Equal(t, []string{"go"}, result.LanguagesConfig["go"].Extensions)
		require.Len(t, result.LanguagesConfig["go"].VersionFiles, 2)
		assert.Contains(t, result.LanguagesConfig, "python", "an untouched language must survive")
	})

	t.Run("should reject an unknown key in a strict layer", func(t *testing.T) {
		t.Parallel()

		// when
		result, err := entities.ApplyLayer(
			&entities.GlobalConfig{}, operatorLayer("no_such_key: true\n"),
		)

		// then
		require.Error(t, err, "a typo in the operator's own file is theirs to hear about")
		assert.Nil(t, result)
	})

	t.Run("should leave the accumulation untouched when the layer fails", func(t *testing.T) {
		t.Parallel()

		// given
		base := &entities.GlobalConfig{Versioning: "fork-dot", ChangelogPath: "DOCS.md"}

		// when
		_, err := entities.ApplyLayer(base, operatorLayer("versioning: 'semver'\nbroken: [\n"))

		// then
		require.Error(t, err)
		assert.Equal(t, "fork-dot", base.Versioning, "apply is atomic: nothing lands on a failure")
		assert.Equal(t, "DOCS.md", base.ChangelogPath)
	})

	t.Run("should not mutate the configuration it was given", func(t *testing.T) {
		t.Parallel()

		// given
		base := &entities.GlobalConfig{
			Versioning:      "fork-dot",
			LanguagesConfig: map[string]entities.LanguageConfig{"go": {Extensions: []string{"go"}}},
		}

		// when
		result, err := entities.ApplyLayer(base, operatorLayer(
			"versioning: 'semver'\nlanguages:\n  python:\n    extensions:\n      - 'py'\n",
		))

		// then
		require.NoError(t, err)
		assert.Equal(t, "semver", result.Versioning)
		assert.Equal(t, "fork-dot", base.Versioning)
		assert.NotContains(t, base.LanguagesConfig, "python")
	})

	t.Run("should reset a pointer key a later layer sets to null", func(t *testing.T) {
		t.Parallel()

		// given -- yaml.v3 zeroes a pointer, map or slice on an explicit null, so an
		// opt-out toggle can be handed back to the default rather than only flipped
		enabled := true
		base := &entities.GlobalConfig{DetectChlog: &enabled}

		// when
		result, err := entities.ApplyLayer(base, operatorLayer("detect_chlog: null\n"))

		// then
		require.NoError(t, err)
		assert.Nil(t, result.DetectChlog, "nil is 'unset', which resolves to the default")
	})

	t.Run("should leave a scalar key a later layer sets to null", func(t *testing.T) {
		t.Parallel()

		// given -- and yaml.v3 does NOT zero a string or a bool on null, so `versioning:
		// null` inherits rather than clearing. Worth pinning because the asymmetry is
		// surprising and the empty string is what "unset" means for these fields.
		base := &entities.GlobalConfig{Versioning: "fork-dot"}

		// when
		result, err := entities.ApplyLayer(base, operatorLayer("versioning: null\n"))

		// then
		require.NoError(t, err)
		assert.Equal(t, "fork-dot", result.Versioning)
	})

	t.Run("should refuse a removed key in the operator's own file", func(t *testing.T) {
		t.Parallel()

		// given -- yaml.v3 would say only "field refresh_commands not found in type
		// LanguageConfig", which tells an operator nothing about what to write instead
		layer := operatorLayer(
			"languages:\n  typescript:\n    refresh_commands:\n      - run: ['yarn']\n",
		)

		// when
		result, err := entities.ApplyLayer(&entities.GlobalConfig{}, layer)

		// then
		require.ErrorIs(t, err, entities.ErrRemovedConfigKey)
		assert.Contains(t, err.Error(), "refresh: true")
		assert.Nil(t, result)
	})

	t.Run("should ignore a removed key in a repository's own file", func(t *testing.T) {
		t.Parallel()

		// given -- a repository's refresh commands were already dropped before this
		// release, so continuing is what that path has always done
		layer := restrictedLayer(
			"languages:\n  typescript:\n    refresh_commands:\n      - run: ['yarn']\n",
		)

		// when
		result, err := entities.ApplyLayer(&entities.GlobalConfig{}, layer)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestResolveGlobalConfig(t *testing.T) {
	t.Parallel()

	t.Run("should apply the layers in order", func(t *testing.T) {
		t.Parallel()

		// given
		layers := []entities.ConfigLayer{
			defaultsLayer("versioning: 'semver'\nchangelog_path: 'CHANGELOG.md'\n"),
			operatorLayer("versioning: 'fork-dot'\n"),
		}

		// when
		result, err := entities.ResolveGlobalConfig(layers)

		// then
		require.NoError(t, err)
		assert.Equal(t, "fork-dot", result.Versioning, "the later layer wins")
		assert.Equal(t, "CHANGELOG.md", result.ChangelogPath, "the earlier layer survives")
	})

	t.Run("should merge languages across three layers", func(t *testing.T) {
		t.Parallel()

		// given
		layers := []entities.ConfigLayer{
			defaultsLayer(
				"languages:\n  typescript:\n    extensions: ['ts']\n" +
					"    version_files:\n      - path: 'package.json'\n        patterns: ['a']\n",
			),
			operatorLayer("languages:\n  typescript:\n    extensions: ['tsx']\n"),
			restrictedLayer(
				"languages:\n  typescript:\n    version_files:\n" +
					"      - path: 'package.json'\n        patterns: ['b']\n" +
					"      - path: 'plugins/*/package.json'\n        patterns: ['c']\n",
			),
		}

		// when
		result, err := entities.ResolveGlobalConfig(layers)

		// then
		typescript := result.LanguagesConfig["typescript"]
		require.NoError(t, err)
		assert.Equal(t, []string{"ts", "tsx"}, typescript.Extensions)
		require.Len(t, typescript.VersionFiles, 2,
			"version files still match by path at the third layer, not only the second")
		assert.Equal(t, []string{"b"}, typescript.VersionFiles[0].Patterns)
		assert.Equal(t, "plugins/*/package.json", typescript.VersionFiles[1].Path)
	})

	t.Run("should skip an optional layer that cannot be decoded", func(t *testing.T) {
		t.Parallel()

		// given
		//nolint:exhaustruct // Strict stays false: `main` may be newer than this binary
		published := entities.ConfigLayer{
			Name:     entities.LayerPublishedDefaults,
			Origin:   entities.DefaultConfigURL,
			Data:     []byte("this is not: [ yaml\n"),
			Scope:    entities.ScopeRestricted,
			Optional: true,
		}
		layers := []entities.ConfigLayer{
			defaultsLayer("versioning: 'semver'\n"), published,
		}

		// when
		result, err := entities.ResolveGlobalConfig(layers)

		// then
		require.NoError(t, err, "an unreachable or broken remote must not stop a release")
		assert.Equal(t, "semver", result.Versioning)
	})

	t.Run("should fail when a required layer cannot be decoded", func(t *testing.T) {
		t.Parallel()

		// given
		layers := []entities.ConfigLayer{operatorLayer("this is not: [ yaml\n")}

		// when
		result, err := entities.ResolveGlobalConfig(layers)

		// then
		require.Error(t, err, "a file the operator named must be readable")
		assert.Nil(t, result)
	})

	t.Run("should resolve a token file over the folded configuration", func(t *testing.T) {
		t.Parallel()

		// given -- finalisation runs once, at the end. Running it per layer would read a
		// token path off disk even when the next layer replaced it.
		layers := []entities.ConfigLayer{
			defaultsLayer("# nothing\n"),
			operatorLayer("github_access_token: 'ghp_inline'\nprojects:\n  - path: '/a/repo.git'\n"),
		}

		// when
		result, err := entities.ResolveGlobalConfig(layers)

		// then
		require.NoError(t, err)
		assert.Equal(t, "ghp_inline", result.GitHubAccessToken)
		assert.Equal(t, "repo", result.Projects[0].Name)
	})
}
