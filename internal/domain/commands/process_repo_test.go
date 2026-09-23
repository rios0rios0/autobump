package commands_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/internal/infrastructure/repositories"
	"github.com/rios0rios0/autobump/test/domain/entitybuilders"
	gitInfra "github.com/rios0rios0/gitforge/pkg/git/infrastructure"
)

// TestProcessRepoIntegration is deliberately not parallel: it mutates package-level globals that other tests read.
func TestProcessRepoIntegration(t *testing.T) {
	// Create a minimal .gitconfig so GetGlobalGitConfig() succeeds on CI where ~/.gitconfig doesn't exist
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	require.NoError(t, os.WriteFile(
		filepath.Join(fakeHome, ".gitconfig"),
		[]byte("[user]\n\tname = Test User\n\temail = test@test.com\n"),
		0o644,
	))

	t.Run("should return error when changelog_path escapes project root", func(t *testing.T) {
		// given
		repoPath, _ := createTestRepo(t)
		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		require.NoError(t, os.WriteFile(changelogPath, []byte("# Changelog\n\n## [Unreleased]\n"), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{}).
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(repoPath).
			WithName("test-project").
			BuildProjectConfig()
		projectConfig.ChangelogPath = "../../etc/passwd"

		// when
		err := commands.ProcessRepo(globalConfig, projectConfig)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid changelog_path")
	})

	t.Run("should skip when unreleased section is empty", func(t *testing.T) {
		// given
		repoPath, _ := createTestRepo(t)
		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		content := "# Changelog\n\n## [Unreleased]\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n"
		require.NoError(t, os.WriteFile(changelogPath, []byte(content), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{}).
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(repoPath).
			WithName("test-project").
			BuildProjectConfig()

		// when
		err := commands.ProcessRepo(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
	})

	t.Run("should create bump branch and update files when unreleased entries exist", func(t *testing.T) {
		// given
		fixture := newBumpFixture(t,
			"# Changelog\n\n## [Unreleased]\n\n### Added\n\n- added new feature\n\n"+
				"## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n", nil)

		// when
		err := commands.ProcessRepo(fixture.globalConfig, fixture.projectConfig)

		// then — push will fail (no remote), but branch should be created and changelog updated
		// The error is expected because there's no remote to push to
		require.Error(t, err)

		// Verify the changelog was updated with the new version
		assert.Contains(t, fixture.readChangelog(t), "[1.1.0]")
	})

	t.Run("should release chlog fragments and stage their removal when the project uses chlog", func(t *testing.T) {
		// given — the unreleased section is empty, exactly as chlog leaves it: the pending
		// work lives in the fragments, which is what AutoBump used to miss entirely
		fixture := newBumpFixture(t,
			"# Changelog\n\n## [Unreleased]\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n",
			map[string]string{
				".changes/unreleased/100-a1b2.yaml": "kind: Added\nbody: added OAuth2 login\n",
				".changes/unreleased/200-c3d4.yaml": "kind: Fixed\nbody: fixed the retry backoff\n",
			})

		// when
		err := commands.ProcessRepo(fixture.globalConfig, fixture.projectConfig)

		// then — the push fails because the test repo has no remote, but everything
		// before it must have happened
		require.Error(t, err)

		updatedChangelog := fixture.readChangelog(t)
		assert.Contains(t, updatedChangelog, "[1.1.0]")
		assert.Contains(t, updatedChangelog, "- added OAuth2 login")
		assert.Contains(t, updatedChangelog, "- fixed the retry backoff")

		// The consumed fragments must be gone from disk and their deletion staged,
		// otherwise the next run would release the same entries again.
		assert.NoFileExists(t, filepath.Join(fixture.repoPath, ".changes", "unreleased", "100-a1b2.yaml"))
		assert.NoFileExists(t, filepath.Join(fixture.repoPath, ".changes", "unreleased", "200-c3d4.yaml"))

		status, statusErr := fixture.worktree.Status()
		require.NoError(t, statusErr)
		assert.Empty(t, status, "the commit should have captured the changelog and both fragment removals")
	})

	t.Run("should leave the repository untouched when its own .autobump.yaml requests a skip", func(t *testing.T) {
		// given -- pending entries that would otherwise be released, and the skip committed
		// beside them, the way a mirror or a fork released upstream would carry it
		changelog := "# Changelog\n\n## [Unreleased]\n\n### Added\n\n- added new feature\n\n" +
			"## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n"
		fixture := newBumpFixture(t, changelog, map[string]string{
			".autobump.yaml": "skip: true\nreason: 'released upstream'\n",
		})
		headBefore := fixture.headRef(t)
		branchesBefore := fixture.branchNames(t)

		// when
		err := commands.ProcessRepo(fixture.globalConfig, fixture.projectConfig)

		// then -- no error, because nothing was attempted: not even the push that fails for
		// every other fixture in this file, which has no remote to push to
		require.NoError(t, err)
		assert.Equal(t, changelog, fixture.readChangelog(t), "the changelog must not be released")
		assert.Equal(t, headBefore, fixture.headRef(t), "HEAD must neither move nor gain a commit")
		assert.Equal(t, branchesBefore, fixture.branchNames(t), "no bump branch may be created")

		status, statusErr := fixture.worktree.Status()
		require.NoError(t, statusErr)
		assert.True(t, status.IsClean(), "the worktree must be exactly as it was found")
	})

	t.Run("should not create a missing changelog when its own .autobump.yaml requests a skip", func(t *testing.T) {
		// given -- without the skip, a missing changelog is created, committed and pushed to
		// the default branch before anything else is decided
		fixture := newBumpFixture(t, "", map[string]string{".autobump.yaml": "skip: true\n"})
		_, err := fixture.worktree.Remove("CHANGELOG.md")
		require.NoError(t, err)
		require.NoFileExists(t, fixture.changelogPath)
		_, err = fixture.worktree.Commit("drop the changelog", &git.CommitOptions{
			Author: &object.Signature{Name: "Test", Email: "test@test.com", When: time.Now()},
		})
		require.NoError(t, err)
		headBefore := fixture.headRef(t)

		// when
		err = commands.ProcessRepo(fixture.globalConfig, fixture.projectConfig)

		// then
		require.NoError(t, err)
		assert.NoFileExists(t, fixture.changelogPath)
		assert.Equal(t, headBefore, fixture.headRef(t), "no changelog commit may be made")
	})

	t.Run("should still release when its own .autobump.yaml sets skip to false", func(t *testing.T) {
		// given
		fixture := newBumpFixture(t,
			"# Changelog\n\n## [Unreleased]\n\n### Added\n\n- added new feature\n\n"+
				"## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n",
			map[string]string{".autobump.yaml": "skip: false\n"})

		// when
		err := commands.ProcessRepo(fixture.globalConfig, fixture.projectConfig)

		// then -- the push fails because the test repo has no remote, but the release was cut
		require.Error(t, err)
		assert.Contains(t, fixture.readChangelog(t), "[1.1.0]")
	})
}

// bumpFixture is a committed repository wired up so ProcessRepo can run against it.
type bumpFixture struct {
	repoPath      string
	changelogPath string
	repo          *git.Repository
	worktree      *git.Worktree
	globalConfig  *entities.GlobalConfig
	projectConfig *entities.ProjectConfig
}

// newBumpFixture writes the changelog plus any extra files (keyed by slash-separated path
// relative to the repository root), commits them so they survive branch switches, and
// registers the provider registry ProcessRepo needs.
func newBumpFixture(t *testing.T, changelog string, extraFiles map[string]string) bumpFixture {
	t.Helper()

	repoPath, repo := createTestRepo(t)
	changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
	require.NoError(t, os.WriteFile(changelogPath, []byte(changelog), 0o600))

	for relPath, content := range extraFiles {
		fullPath := filepath.Join(repoPath, filepath.FromSlash(relPath))
		makeDir(t, filepath.Dir(fullPath))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o600))
	}

	worktree, err := repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, worktree.AddWithOptions(&git.AddOptions{All: true}))
	_, err = worktree.Commit("seed the fixture", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@test.com", When: time.Now()},
	})
	require.NoError(t, err)

	registry := repositories.NewProviderRegistry()
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	return bumpFixture{
		repoPath:      repoPath,
		changelogPath: changelogPath,
		repo:          repo,
		worktree:      worktree,
		globalConfig: entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{}).
			BuildGlobalConfig(),
		projectConfig: entitybuilders.NewProjectConfigBuilder().
			WithPath(repoPath).
			WithName("test-project").
			BuildProjectConfig(),
	}
}

// readChangelog returns the current contents of the fixture's changelog.
func (f bumpFixture) readChangelog(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(f.changelogPath)
	require.NoError(t, err)
	return string(content)
}

// headRef returns where the fixture's HEAD points: the branch it is on and its commit.
func (f bumpFixture) headRef(t *testing.T) string {
	t.Helper()
	head, err := f.repo.Head()
	require.NoError(t, err)
	return head.String()
}

// branchNames returns every local branch in the fixture's repository.
func (f bumpFixture) branchNames(t *testing.T) []string {
	t.Helper()
	branches, err := f.repo.Branches()
	require.NoError(t, err)

	names := make([]string, 0)
	require.NoError(t, branches.ForEach(func(ref *plumbing.Reference) error {
		names = append(names, ref.Name().Short())
		return nil
	}))
	return names
}

// TestProcessRepoAdditionalBranches is deliberately not parallel: it mutates package-level globals that other tests read.
func TestProcessRepoAdditionalBranches(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	require.NoError(t, os.WriteFile(
		filepath.Join(fakeHome, ".gitconfig"),
		[]byte("[user]\n\tname = Test User\n\temail = test@test.com\n"),
		0o644,
	))

	registry := repositories.NewProviderRegistry()
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	t.Run("should return error when global git config is unavailable", func(t *testing.T) {
		// given -- override HOME to a dir without .gitconfig
		emptyHome := t.TempDir()
		t.Setenv("HOME", emptyHome)

		repoPath, _ := createTestRepo(t)
		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		require.NoError(t, os.WriteFile(changelogPath, []byte("# Changelog\n\n## [Unreleased]\n"), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{}).
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(repoPath).
			WithName("test-project").
			BuildProjectConfig()

		// when
		err := commands.ProcessRepo(globalConfig, projectConfig)

		// then -- expect error due to missing global git config
		require.Error(t, err)

		// restore HOME for subsequent tests
		t.Setenv("HOME", fakeHome)
	})

	t.Run("should handle custom changelog_path correctly", func(t *testing.T) {
		// given
		repoPath, _ := createTestRepo(t)
		docsDir := filepath.Join(repoPath, "docs")
		// The docs dir needs the owner execute bit for traversal, so 0o700 is least-privilege.
		// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
		require.NoError(t, os.MkdirAll(docsDir, 0o700))
		changelogPath := filepath.Join(docsDir, "CHANGES.md")
		content := "# Changelog\n\n## [Unreleased]\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n"
		require.NoError(t, os.WriteFile(changelogPath, []byte(content), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{}).
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(repoPath).
			WithName("test-project").
			WithChangelogPath("docs/CHANGES.md").
			BuildProjectConfig()

		// when
		err := commands.ProcessRepo(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
	})

	t.Run("should handle BranchExistsNoPR status", func(t *testing.T) {
		// given
		repoPath, repo := createTestRepo(t)
		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		content := "# Changelog\n\n## [Unreleased]\n\n### Added\n\n- added new feature\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n"
		require.NoError(t, os.WriteFile(changelogPath, []byte(content), 0o644))

		// Commit the changelog and create the bump branch
		wt, err := repo.Worktree()
		require.NoError(t, err)
		_, err = wt.Add("CHANGELOG.md")
		require.NoError(t, err)
		_, err = wt.Commit("add changelog", &git.CommitOptions{
			Author: &object.Signature{Name: "Test", Email: "test@test.com", When: time.Now()},
		})
		require.NoError(t, err)

		// Pre-create the bump branch
		head, err := repo.Head()
		require.NoError(t, err)
		err = wt.Checkout(&git.CheckoutOptions{
			Branch: "refs/heads/chore/bump-1.1.0",
			Create: true,
		})
		require.NoError(t, err)
		err = wt.Checkout(&git.CheckoutOptions{Branch: head.Name()})
		require.NoError(t, err)

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{}).
			BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(repoPath).
			WithName("test-project").
			BuildProjectConfig()

		// when
		err = commands.ProcessRepo(globalConfig, projectConfig)

		// then -- branch exists, no remote, handleExistingBranchWithoutPR returns nil
		require.NoError(t, err)
	})
}

// TestSetGitOperations is deliberately not parallel: it mutates package-level globals that other tests read.
func TestSetGitOperations(t *testing.T) {
	t.Run("should not panic when setting git operations", func(t *testing.T) {
		// given
		registry := repositories.NewProviderRegistry()
		ops := gitInfra.NewGitOperations(registry)

		// when / then
		assert.NotPanics(t, func() {
			commands.SetGitOperations(ops)
		})
	})
}

func TestGetLanguageInterface(t *testing.T) {
	t.Parallel()

	t.Run("should return Python language interface when language is python", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, "pyproject.toml"),
			[]byte("[project]\nname = \"test-project\"\n"),
			0o644,
		))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithLanguagesConfig(map[string]entities.LanguageConfig{
				"python": {
					Extensions:      []string{"py"},
					SpecialPatterns: []string{"pyproject.toml"},
					VersionFiles: []entities.VersionFile{
						{
							Path:     "{project_name}/__init__.py",
							Patterns: []string{`(__version__\s*=\s*")\d+\.\d+\.\d+(")`},
						},
					},
				},
			}).BuildGlobalConfig()
		projectConfig := entitybuilders.NewProjectConfigBuilder().
			WithPath(tmpDir).
			WithLanguage("python").
			WithName("test-project").
			BuildProjectConfig()

		// when — getVersionFiles will use the language interface to get the project name
		versionFiles, err := commands.GetVersionFiles(globalConfig, projectConfig)

		// then
		require.NoError(t, err)
		assert.Empty(t, versionFiles) // file doesn't exist but no error
	})
}
