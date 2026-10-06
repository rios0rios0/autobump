package commands_test

import (
	"go/format"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/test/domain/entitybuilders"
)

const (
	libraryModulePath = "github.com/acme/lib"

	// driftChangelog last released 4.2.9 and holds one fix, so the next release is 4.2.10.
	// Its history quotes the old path, which a release must never rewrite.
	driftChangelog = "# Changelog\n\n## [Unreleased]\n\n### Fixed\n\n- fixed a bug\n\n" +
		"## [4.2.9] - 2026-10-01\n\n### Fixed\n\n- fixed `go get github.com/acme/lib` in the README\n"

	// breakingChangelog last released 4.2.9 and holds a breaking change, so the next
	// release is 5.0.0.
	breakingChangelog = "# Changelog\n\n## [Unreleased]\n\n### Changed\n\n" +
		"- **BREAKING CHANGE:** removed `Client.Close`\n\n" +
		"## [4.2.9] - 2026-10-01\n\n### Fixed\n\n- fixed a bug\n"
)

// createGoLibraryRepo lays out a Go module in a real git repository and commits it: a
// go.mod, a package importing another of the module's packages, a README, a vendored
// dependency, a fixture, a nested module and the changelog, plus any extra files. The
// tags are created on that commit. A file nobody committed is written last.
func createGoLibraryRepo(
	t *testing.T,
	modulePath, changelog string,
	extra map[string]string,
	tags ...string,
) (string, *git.Repository) {
	t.Helper()

	repoPath := t.TempDir()
	repo, err := git.PlainInit(repoPath, false)
	require.NoError(t, err)
	wt, err := repo.Worktree()
	require.NoError(t, err)

	files := map[string]string{
		"go.mod": "module " + modulePath + "\n\ngo 1.27\n",
		"pkg/a/a.go": "package a\n\nimport (\n\t\"fmt\"\n\n\t\"" + modulePath + "/pkg/b\"\n)\n\n" +
			"// A prints B.\nfunc A() string { return fmt.Sprint(b.B) }\n",
		"pkg/b/b.go": "package b\n\n// B is a constant.\nconst B = 1\n",
		"README.md": "# lib\n\n[Releases](https://github.com/acme/lib/releases)\n\n" +
			"```bash\ngo get " + modulePath + "\n```\n",
		"CHANGELOG.md":                       changelog,
		"vendor/github.com/other/dep/dep.go": "package dep\n\nimport _ \"" + modulePath + "/pkg/b\"\n",
		"testdata/fixture.go":                "package fixture\n\nimport _ \"" + modulePath + "/pkg/b\"\n",
		"tools/go.mod": "module " + modulePath + "/tools\n\ngo 1.27\n\nrequire " +
			modulePath + " v0.0.0\n",
		"tools/main.go": "package main\n\nimport _ \"" + modulePath + "/pkg/b\"\n\nfunc main() {}\n",
	}
	maps.Copy(files, extra)

	for name, content := range files {
		fullPath := filepath.Join(repoPath, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
		_, err = wt.Add(name)
		require.NoError(t, err)
	}

	head, err := wt.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@test.com", When: time.Now()},
	})
	require.NoError(t, err)
	for _, tag := range tags {
		_, err = repo.CreateTag(tag, head, nil)
		require.NoError(t, err)
	}

	// An untracked file is not the module's, whatever it says.
	require.NoError(t, os.WriteFile(
		filepath.Join(repoPath, "notes.md"), []byte("go get "+modulePath+"\n"), 0o644,
	))

	return repoPath, repo
}

// goLibraryCtx builds the context a release of the repository runs with.
func goLibraryCtx(t *testing.T, repoPath string, repo *git.Repository) *commands.RepoContext {
	t.Helper()

	wt, err := repo.Worktree()
	require.NoError(t, err)

	return &commands.RepoContext{
		Repo:     repo,
		Worktree: wt,
		GlobalConfig: entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{"go": {}}).
			BuildGlobalConfig(),
		ProjectConfig: entitybuilders.NewProjectConfigBuilder().
			WithPath(repoPath).
			WithLanguage("go").
			BuildProjectConfig(),
	}
}

func readRepoFile(t *testing.T, repoPath, name string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(repoPath, filepath.FromSlash(name)))
	require.NoError(t, err)
	return string(content)
}

func repoFiles(repoPath string, names ...string) []string {
	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, filepath.Join(repoPath, filepath.FromSlash(name)))
	}
	return paths
}

// releaseSection returns the body of the changelog section for version.
func releaseSection(t *testing.T, changelog, version string) string {
	t.Helper()

	_, section, found := strings.Cut(changelog, "## ["+version+"]")
	require.True(t, found, "the changelog has no section for %s", version)
	section, _, _ = strings.Cut(section, "\n## [")
	return section
}

func TestGoModulePathForMajor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		from       string
		major      uint64
		wantPath   string
		wantNeeded bool
	}{
		{
			name: "should add the suffix when an unsuffixed path is released at v2",
			from: "github.com/acme/lib", major: 2, wantPath: "github.com/acme/lib/v2", wantNeeded: true,
		},
		{
			name: "should add the suffix when an unsuffixed path fell behind v4",
			from: "github.com/acme/lib", major: 4, wantPath: "github.com/acme/lib/v4", wantNeeded: true,
		},
		{
			name: "should move the suffix to the next major",
			from: "github.com/acme/lib/v4", major: 5, wantPath: "github.com/acme/lib/v5", wantNeeded: true,
		},
		{name: "should keep a path that already names the major", from: "github.com/acme/lib/v4", major: 4},
		{name: "should never move a path backwards", from: "github.com/acme/lib/v5", major: 4},
		{name: "should keep v0 and v1 unsuffixed", from: "github.com/acme/lib", major: 1},
		{name: "should leave a gopkg.in path to its redirector", from: "gopkg.in/acme/lib.v3", major: 4},
		{name: "should leave a path Go rejects alone", from: "github.com/acme/lib/v1", major: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// given / when
			path, needed := commands.GoModulePathForMajor(test.from, test.major)

			// then
			assert.Equal(t, test.wantNeeded, needed)
			assert.Equal(t, test.wantPath, path)
		})
	}
}

func TestRewriteModuleDirective(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "should keep the comments and directives around the module path",
			input: "// Deprecated: use another\nmodule github.com/acme/lib // the library\n\ngo 1.27\n",
			want:  "// Deprecated: use another\nmodule github.com/acme/lib/v4 // the library\n\ngo 1.27\n",
		},
		{
			name:  "should replace a quoted module path, quotes included",
			input: "module \"github.com/acme/lib\"\n\ngo 1.27\n",
			want:  "module github.com/acme/lib/v4\n\ngo 1.27\n",
		},
		{
			name:  "should replace a module path written as a block",
			input: "module (\n\tgithub.com/acme/lib\n)\n",
			want:  "module (\n\tgithub.com/acme/lib/v4\n)\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// given / when
			rewritten, err := commands.RewriteModuleDirective([]byte(test.input), "github.com/acme/lib/v4")

			// then
			require.NoError(t, err)
			assert.Equal(t, test.want, string(rewritten))
		})
	}

	t.Run("should fail when go.mod declares no module", func(t *testing.T) {
		t.Parallel()

		// given / when
		_, err := commands.RewriteModuleDirective([]byte("go 1.27\n"), "github.com/acme/lib/v4")

		// then
		require.ErrorIs(t, err, commands.ErrGoModulePathNotFound)
	})
}

func TestRewriteGoImports(t *testing.T) {
	t.Parallel()

	const to = "github.com/acme/lib/v4"

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "should move the module's own imports and leave every other module alone",
			input: "package a\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/acme/lib\"\n\tb2 \"github.com/acme/lib/pkg/b\"\n" +
				"\t\"github.com/acme/lib-extra/pkg/c\"\n\t\"github.com/acme/lib/v5/pkg/d\"\n)\n",
			want: "package a\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/acme/lib/v4\"\n\tb2 \"github.com/acme/lib/v4/pkg/b\"\n" +
				"\t\"github.com/acme/lib-extra/pkg/c\"\n\t\"github.com/acme/lib/v5/pkg/d\"\n)\n",
		},
		{
			name:  "should keep a raw string import raw",
			input: "package a\n\nimport _ `github.com/acme/lib/pkg/b`\n",
			want:  "package a\n\nimport _ `github.com/acme/lib/v4/pkg/b`\n",
		},
		{
			name:  "should change nothing but the path in a file that was not gofmt-clean",
			input: "package a\n\nimport    b \"github.com/acme/lib/pkg/b\"\n\nvar   X = b.B\n",
			want:  "package a\n\nimport    b \"github.com/acme/lib/v4/pkg/b\"\n\nvar   X = b.B\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// given / when
			rewritten, err := commands.RewriteGoImports([]byte(test.input), "github.com/acme/lib", to)

			// then
			require.NoError(t, err)
			assert.Equal(t, test.want, string(rewritten))
		})
	}

	t.Run("should keep a gofmt-clean file gofmt-clean when a longer path moves a trailing comment", func(t *testing.T) {
		t.Parallel()

		// given
		input := []byte("package a\n\nimport (\n\t\"github.com/acme/lib/pkg/b\" // the b package\n" +
			"\t\"strings\"                   // the strings package\n)\n\nvar _ = b.B + strings.Count(\"\", \"\")\n")
		formatted, err := format.Source(input)
		require.NoError(t, err)
		require.Equal(t, string(input), string(formatted), "the fixture must start gofmt-clean")

		// when
		rewritten, err := commands.RewriteGoImports(input, "github.com/acme/lib", to)

		// then
		require.NoError(t, err)
		assert.Contains(t, string(rewritten), "\"github.com/acme/lib/v4/pkg/b\" // the b package")
		reformatted, err := format.Source(rewritten)
		require.NoError(t, err)
		assert.Equal(t, string(reformatted), string(rewritten))
	})

	t.Run("should return the file untouched when it imports nothing of the module", func(t *testing.T) {
		t.Parallel()

		// given
		input := []byte("package a\n\nimport \"fmt\"\n\nvar _ = fmt.Sprint\n")

		// when
		rewritten, err := commands.RewriteGoImports(input, "github.com/acme/lib", to)

		// then
		require.NoError(t, err)
		assert.Equal(t, string(input), string(rewritten))
	})

	t.Run("should fail when the imports do not parse", func(t *testing.T) {
		t.Parallel()

		// given / when
		_, err := commands.RewriteGoImports([]byte("package {{ .Name }}\n"), "github.com/acme/lib", to)

		// then
		require.Error(t, err)
	})
}

func TestRewriteModuleReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "should move an install command",
			input: "go get github.com/acme/lib\n",
			want:  "go get github.com/acme/lib/v4\n",
		},
		{
			name:  "should move a command with flags, a package and a version query",
			input: "go install -v github.com/acme/lib/cmd/tool@latest\n",
			want:  "go install -v github.com/acme/lib/v4/cmd/tool@latest\n",
		},
		{
			name:  "should move a quoted import path in a code sample",
			input: "import \"github.com/acme/lib/pkg/b\"\n",
			want:  "import \"github.com/acme/lib/v4/pkg/b\"\n",
		},
		{
			name:  "should move a package named in inline code",
			input: "Import `github.com/acme/lib/pkg/b` to start.\n",
			want:  "Import `github.com/acme/lib/v4/pkg/b` to start.\n",
		},
		{
			name:  "should move a documentation link",
			input: "[docs](https://pkg.go.dev/github.com/acme/lib)\n",
			want:  "[docs](https://pkg.go.dev/github.com/acme/lib/v4)\n",
		},
		{
			name:  "should move a linker flag",
			input: "LDFLAGS := -X github.com/acme/lib/internal/version.Version=$(VERSION)\n",
			want:  "LDFLAGS := -X github.com/acme/lib/v4/internal/version.Version=$(VERSION)\n",
		},
		{
			name:  "should move a quoted module directive",
			input: "├── go.mod  # Module: github.com/acme/lib (Go 1.27)\r\n",
			want:  "├── go.mod  # Module: github.com/acme/lib/v4 (Go 1.27)\r\n",
		},
		{
			name: "should leave repository links, other modules and plain prose alone",
			input: "[Releases](https://github.com/acme/lib/releases)\ngo get github.com/acme/library\n" +
				"go get github.com/acme/lib/v5\nthe github.com/acme/lib library\n",
			want: "[Releases](https://github.com/acme/lib/releases)\ngo get github.com/acme/library\n" +
				"go get github.com/acme/lib/v5\nthe github.com/acme/lib library\n",
		},
		{
			name:  "should move every reference on a line",
			input: "go get github.com/acme/lib && go install github.com/acme/lib/cmd/tool@latest\n",
			want:  "go get github.com/acme/lib/v4 && go install github.com/acme/lib/v4/cmd/tool@latest\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// given / when
			rewritten := commands.RewriteModuleReferences(
				[]byte(test.input), "github.com/acme/lib", "github.com/acme/lib/v4",
			)

			// then
			assert.Equal(t, test.want, string(rewritten))
		})
	}
}

func TestPrepareGoModulePathChange(t *testing.T) {
	t.Parallel()

	t.Run("should repair a path that fell behind the major version the library releases", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath, repo := createGoLibraryRepo(t, libraryModulePath, driftChangelog, nil, "4.2.9", "v4.2.9")
		ctx := goLibraryCtx(t, repoPath, repo)

		// when
		change, err := commands.PrepareGoModulePathChange(ctx, filepath.Join(repoPath, "CHANGELOG.md"))

		// then
		require.NoError(t, err)
		require.NotNil(t, change)
		assert.Equal(t, libraryModulePath, change.From)
		assert.Equal(t, libraryModulePath+"/v4", change.To)
		assert.Equal(t, uint64(4), change.Major)
		assert.False(t, change.NewMajor)
		assert.ElementsMatch(t, repoFiles(repoPath, "go.mod", "pkg/a/a.go", "README.md"), change.Files)
		assert.Equal(t, "module github.com/acme/lib\n\ngo 1.27\n", readRepoFile(t, repoPath, "go.mod"),
			"nothing is written until the change is applied")
	})

	t.Run("should rewrite only the module's own tracked files when applied", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath, repo := createGoLibraryRepo(t, libraryModulePath, driftChangelog, nil, "v4.2.9")
		ctx := goLibraryCtx(t, repoPath, repo)
		untouched := map[string]string{}
		for _, name := range []string{
			"CHANGELOG.md", "vendor/github.com/other/dep/dep.go", "testdata/fixture.go",
			"tools/go.mod", "tools/main.go", "notes.md", "pkg/b/b.go",
		} {
			untouched[name] = readRepoFile(t, repoPath, name)
		}
		change, err := commands.PrepareGoModulePathChange(ctx, filepath.Join(repoPath, "CHANGELOG.md"))
		require.NoError(t, err)

		// when
		written, err := commands.ApplyGoModulePathChange(change)

		// then
		require.NoError(t, err)
		assert.ElementsMatch(t, change.Files, written)
		assert.Equal(t, "module github.com/acme/lib/v4\n\ngo 1.27\n", readRepoFile(t, repoPath, "go.mod"))
		assert.Contains(t, readRepoFile(t, repoPath, "pkg/a/a.go"), "\t\"github.com/acme/lib/v4/pkg/b\"\n")
		readme := readRepoFile(t, repoPath, "README.md")
		assert.Contains(t, readme, "go get github.com/acme/lib/v4\n")
		assert.Contains(t, readme, "(https://github.com/acme/lib/releases)")
		for name, content := range untouched {
			assert.Equal(t, content, readRepoFile(t, repoPath, name), "%s must be left as it was", name)
		}
	})

	t.Run("should move the path to the next major when the release breaks compatibility", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath, repo := createGoLibraryRepo(t, libraryModulePath+"/v4", breakingChangelog, nil, "v4.2.9")
		ctx := goLibraryCtx(t, repoPath, repo)

		// when
		change, err := commands.PrepareGoModulePathChange(ctx, filepath.Join(repoPath, "CHANGELOG.md"))

		// then
		require.NoError(t, err)
		require.NotNil(t, change)
		assert.Equal(t, libraryModulePath+"/v4", change.From)
		assert.Equal(t, libraryModulePath+"/v5", change.To)
		assert.Equal(t, uint64(5), change.Major)
		assert.True(t, change.NewMajor)
	})

	t.Run("should leave the history in chlog fragments alone", func(t *testing.T) {
		t.Parallel()

		// given
		fragment := "kind: 'Fixed'\nbody: 'fixed `go get github.com/acme/lib`'\ntime: '2026-10-02T10:00:00Z'\n"
		repoPath, repo := createGoLibraryRepo(t, libraryModulePath, driftChangelog,
			map[string]string{".changes/unreleased/1790000000000000000-abcd.yaml": fragment}, "v4.2.9")
		ctx := goLibraryCtx(t, repoPath, repo)

		// when
		change, err := commands.PrepareGoModulePathChange(ctx, filepath.Join(repoPath, "CHANGELOG.md"))

		// then
		require.NoError(t, err)
		require.NotNil(t, change)
		assert.ElementsMatch(t, repoFiles(repoPath, "go.mod", "pkg/a/a.go", "README.md"), change.Files)
	})

	t.Run("should leave the path alone when it already names the released major", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath, repo := createGoLibraryRepo(t, libraryModulePath+"/v4", driftChangelog, nil, "v4.2.9")
		ctx := goLibraryCtx(t, repoPath, repo)

		// when
		change, err := commands.PrepareGoModulePathChange(ctx, filepath.Join(repoPath, "CHANGELOG.md"))

		// then
		require.NoError(t, err)
		assert.Nil(t, change)
	})

	t.Run("should leave an application alone when no tag is one Go resolves", func(t *testing.T) {
		t.Parallel()

		// given -- binaries are released under plain "4.2.9" tags, which Go never reads
		repoPath, repo := createGoLibraryRepo(t, libraryModulePath, driftChangelog, nil, "4.2.9")
		ctx := goLibraryCtx(t, repoPath, repo)

		// when
		change, err := commands.PrepareGoModulePathChange(ctx, filepath.Join(repoPath, "CHANGELOG.md"))

		// then
		require.NoError(t, err)
		assert.Nil(t, change)
	})

	t.Run("should leave the path alone in a fork versioning mode", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath, repo := createGoLibraryRepo(t, libraryModulePath, driftChangelog, nil, "v4.2.9")
		ctx := goLibraryCtx(t, repoPath, repo)
		ctx.ProjectConfig.Versioning = entities.VersioningForkDot

		// when
		change, err := commands.PrepareGoModulePathChange(ctx, filepath.Join(repoPath, "CHANGELOG.md"))

		// then
		require.NoError(t, err)
		assert.Nil(t, change)
	})

	t.Run("should do nothing for a project without a go.mod", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath, repo := createTestRepo(t)
		head, err := repo.Head()
		require.NoError(t, err)
		_, err = repo.CreateTag("v4.2.9", head.Hash(), nil)
		require.NoError(t, err)
		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		require.NoError(t, os.WriteFile(changelogPath, []byte(driftChangelog), 0o644))
		ctx := goLibraryCtx(t, repoPath, repo)

		// when
		change, err := commands.PrepareGoModulePathChange(ctx, changelogPath)

		// then
		require.NoError(t, err)
		assert.Nil(t, change)
	})
}

func TestUpdateChangelogAndVersionFilesGoModulePath(t *testing.T) {
	t.Parallel()

	t.Run("should release a repaired path as a fix without raising the version", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath, repo := createGoLibraryRepo(t, libraryModulePath, driftChangelog, nil, "v4.2.9")
		ctx := goLibraryCtx(t, repoPath, repo)

		// when
		err := commands.UpdateChangelogAndVersionFiles(ctx, filepath.Join(repoPath, "CHANGELOG.md"))

		// then
		require.NoError(t, err)
		assert.Equal(t, "4.2.10", ctx.ProjectConfig.NewVersion)
		require.NotNil(t, ctx.GoModulePathChange)

		release := releaseSection(t, readRepoFile(t, repoPath, "CHANGELOG.md"), "4.2.10")
		assert.Contains(t, release, "### Fixed")
		assert.Contains(t, release, "- fixed a bug\n")
		assert.Contains(t, release, "- fixed the Go module path to `github.com/acme/lib/v4`, which Go requires of "+
			"a v4 release and without which `go get` could not resolve it: import its packages from "+
			"`github.com/acme/lib/v4/...` instead of `github.com/acme/lib/...`\n")
		assert.Contains(t, readRepoFile(t, repoPath, "go.mod"), "module github.com/acme/lib/v4\n")

		status, err := ctx.Worktree.Status()
		require.NoError(t, err)
		for _, name := range []string{"go.mod", "pkg/a/a.go", "README.md", "CHANGELOG.md"} {
			assert.Equal(t, git.Modified, status.File(name).Staging, "%s must be staged", name)
		}
		assert.Equal(t, git.Untracked, status.File("notes.md").Staging)
	})

	t.Run("should announce the new path as the breaking change a new major is", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath, repo := createGoLibraryRepo(t, libraryModulePath+"/v4", breakingChangelog, nil, "v4.2.9")
		ctx := goLibraryCtx(t, repoPath, repo)

		// when
		err := commands.UpdateChangelogAndVersionFiles(ctx, filepath.Join(repoPath, "CHANGELOG.md"))

		// then
		require.NoError(t, err)
		assert.Equal(t, "5.0.0", ctx.ProjectConfig.NewVersion)

		release := releaseSection(t, readRepoFile(t, repoPath, "CHANGELOG.md"), "5.0.0")
		assert.Contains(t, release, "### Changed")
		assert.Contains(t, release, "- **BREAKING CHANGE:** changed the Go module path to `github.com/acme/lib/v5`, "+
			"which Go requires of a v5 release: import its packages from `github.com/acme/lib/v5/...` "+
			"instead of `github.com/acme/lib/v4/...`\n")
		assert.Contains(t, readRepoFile(t, repoPath, "go.mod"), "module github.com/acme/lib/v5\n")
		assert.Contains(t, readRepoFile(t, repoPath, "pkg/a/a.go"), "\"github.com/acme/lib/v5/pkg/b\"")
	})

	t.Run("should release an application without touching its module path", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath, repo := createGoLibraryRepo(t, libraryModulePath, driftChangelog, nil, "4.2.9")
		ctx := goLibraryCtx(t, repoPath, repo)

		// when
		err := commands.UpdateChangelogAndVersionFiles(ctx, filepath.Join(repoPath, "CHANGELOG.md"))

		// then
		require.NoError(t, err)
		assert.Equal(t, "4.2.10", ctx.ProjectConfig.NewVersion)
		assert.Nil(t, ctx.GoModulePathChange)
		assert.Equal(t, "module github.com/acme/lib\n\ngo 1.27\n", readRepoFile(t, repoPath, "go.mod"))
		assert.NotContains(t, readRepoFile(t, repoPath, "CHANGELOG.md"), "Go module path")
	})
}

func TestGeneratePRDescriptionGoModulePath(t *testing.T) {
	t.Parallel()

	t.Run("should describe the module path change when the release made one", func(t *testing.T) {
		t.Parallel()

		// given
		ctx := &commands.RepoContext{
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().
				WithLanguagesConfig(map[string]entities.LanguageConfig{}).
				BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithName("lib").
				WithNewVersion("5.0.0").
				BuildProjectConfig(),
			GoModulePathChange: &commands.GoModulePathChange{
				From:  "github.com/acme/lib/v4",
				To:    "github.com/acme/lib/v5",
				Major: 5,
				Files: []string{"go.mod", "pkg/a/a.go", "README.md"},
			},
		}

		// when
		description := commands.GeneratePRDescription(ctx)

		// then
		assert.Contains(t, description,
			"- Changed the Go module path from `github.com/acme/lib/v4` to `github.com/acme/lib/v5` in 3 file(s)\n")
		assert.Contains(t, description,
			"- [ ] Plan the consumers' move to `github.com/acme/lib/v5`: their imports change with the module path\n")
	})
}
