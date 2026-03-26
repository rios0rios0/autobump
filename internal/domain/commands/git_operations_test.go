//go:build unit

package commands_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/test/domain/entitybuilders"
)

// createTestRepo creates a real git repo in a temp dir with an initial commit.
func createTestRepo(t *testing.T) (string, *git.Repository) {
	t.Helper()
	tmpDir := t.TempDir()
	repo, err := git.PlainInit(tmpDir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	readmePath := filepath.Join(tmpDir, "README.md")
	require.NoError(t, os.WriteFile(readmePath, []byte("# Test\n"), 0o644))
	_, err = wt.Add("README.md")
	require.NoError(t, err)

	_, err = wt.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	return tmpDir, repo
}

func TestSetupRepo(t *testing.T) {
	t.Parallel()

	t.Run("should set worktree and head when repo is valid", func(t *testing.T) {
		// given
		repoPath, repo := createTestRepo(t)
		ctx := &commands.RepoContext{
			Repo: repo,
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath(repoPath).
				BuildProjectConfig(),
		}

		// when
		err := commands.SetupRepo(ctx)

		// then
		require.NoError(t, err)
		assert.NotNil(t, ctx.Worktree)
		assert.NotNil(t, ctx.Head)
	})

	t.Run("should open repo from path when repo is nil", func(t *testing.T) {
		// given
		repoPath, _ := createTestRepo(t)
		ctx := &commands.RepoContext{
			Repo: nil,
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath(repoPath).
				BuildProjectConfig(),
		}

		// when
		err := commands.SetupRepo(ctx)

		// then
		require.NoError(t, err)
		assert.NotNil(t, ctx.Repo)
		assert.NotNil(t, ctx.Worktree)
		assert.NotNil(t, ctx.Head)
	})

	t.Run("should return error when repo path is invalid", func(t *testing.T) {
		// given
		ctx := &commands.RepoContext{
			Repo: nil,
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath("/nonexistent/path").
				BuildProjectConfig(),
		}

		// when
		err := commands.SetupRepo(ctx)

		// then
		require.Error(t, err)
	})
}

func TestCreateBumpBranch(t *testing.T) {
	t.Parallel()

	t.Run("should create bump branch when changelog has unreleased entries", func(t *testing.T) {
		// given
		repoPath, repo := createTestRepo(t)
		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		content := "# Changelog\n\n## [Unreleased]\n\n### Added\n\n- added feature\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n"
		require.NoError(t, os.WriteFile(changelogPath, []byte(content), 0o644))

		wt, err := repo.Worktree()
		require.NoError(t, err)
		head, err := repo.Head()
		require.NoError(t, err)

		ctx := &commands.RepoContext{
			Repo:     repo,
			Worktree: wt,
			Head:     head,
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath(repoPath).
				BuildProjectConfig(),
		}

		// when
		branchName, status, err := commands.CreateBumpBranch(ctx, changelogPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, "chore/bump-1.1.0", branchName)
		assert.Equal(t, entities.BranchCreated, status)
		assert.Equal(t, "1.1.0", ctx.ProjectConfig.NewVersion)
	})

	t.Run("should return BranchExistsNoPR when branch already exists", func(t *testing.T) {
		// given
		repoPath, repo := createTestRepo(t)
		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		content := "# Changelog\n\n## [Unreleased]\n\n### Added\n\n- added feature\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n"
		require.NoError(t, os.WriteFile(changelogPath, []byte(content), 0o644))

		wt, err := repo.Worktree()
		require.NoError(t, err)
		head, err := repo.Head()
		require.NoError(t, err)

		// Create the branch first
		err = wt.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName("chore/bump-1.1.0"),
			Create: true,
		})
		require.NoError(t, err)
		// Switch back to initial branch
		err = wt.Checkout(&git.CheckoutOptions{
			Branch: head.Name(),
		})
		require.NoError(t, err)

		ctx := &commands.RepoContext{
			Repo:     repo,
			Worktree: wt,
			Head:     head,
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath(repoPath).
				BuildProjectConfig(),
		}

		// when
		branchName, status, err := commands.CreateBumpBranch(ctx, changelogPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, "chore/bump-1.1.0", branchName)
		assert.Equal(t, entities.BranchExistsNoPR, status)
	})
}

func TestCheckoutToMainBranch(t *testing.T) {
	t.Parallel()

	t.Run("should checkout to main branch when main exists", func(t *testing.T) {
		// given
		repoPath, repo := createTestRepo(t)

		// Rename branch to main
		head, err := repo.Head()
		require.NoError(t, err)
		err = repo.Storer.SetReference(plumbing.NewHashReference("refs/heads/main", head.Hash()))
		require.NoError(t, err)

		wt, err := repo.Worktree()
		require.NoError(t, err)
		err = wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName("main")})
		require.NoError(t, err)

		// Create and switch to a feature branch
		err = wt.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName("feature/test"),
			Create: true,
		})
		require.NoError(t, err)

		ctx := &commands.RepoContext{
			Worktree: wt,
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath(repoPath).
				BuildProjectConfig(),
		}

		// when
		err = commands.CheckoutToMainBranch(ctx)

		// then
		require.NoError(t, err)
	})
}

func TestResolveDefaultBranch(t *testing.T) {
	t.Parallel()

	t.Run("should return main when no remote HEAD exists", func(t *testing.T) {
		// given
		_, repo := createTestRepo(t)

		// when
		branch := commands.ResolveDefaultBranch(repo)

		// then
		assert.Equal(t, "main", branch)
	})
}

func TestAddFilesToWorktree(t *testing.T) {
	t.Parallel()

	t.Run("should add version files to worktree when they exist", func(t *testing.T) {
		// given
		repoPath, repo := createTestRepo(t)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		// Write a version file
		pomPath := filepath.Join(repoPath, "pom.xml")
		require.NoError(t, os.WriteFile(pomPath, []byte("<project><version>1.0.0</version></project>"), 0o644))

		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		require.NoError(t, os.WriteFile(changelogPath, []byte("# Changelog\n"), 0o644))

		ctx := &commands.RepoContext{
			Worktree: wt,
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().
				WithLanguagesConfig(map[string]entities.LanguageConfig{
					"java": {
						VersionFiles: []entities.VersionFile{
							{Path: "pom.xml", Patterns: []string{`(<version>)[^<]+(</version>)`}},
						},
					},
				}).BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath(repoPath).
				WithLanguage("java").
				BuildProjectConfig(),
		}

		// when
		err = commands.AddFilesToWorktree(ctx, changelogPath)

		// then
		require.NoError(t, err)
		status, statusErr := wt.Status()
		require.NoError(t, statusErr)
		assert.Contains(t, status, "pom.xml")
		assert.Contains(t, status, "CHANGELOG.md")
	})
}

func TestUpdateChangelogAndVersionFiles(t *testing.T) {
	t.Parallel()

	t.Run("should update changelog and version files when both exist", func(t *testing.T) {
		// given
		repoPath, repo := createTestRepo(t)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		changelogContent := "# Changelog\n\n## [Unreleased]\n\n### Added\n\n- added feature\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n"
		require.NoError(t, os.WriteFile(changelogPath, []byte(changelogContent), 0o644))

		pomPath := filepath.Join(repoPath, "pom.xml")
		require.NoError(t, os.WriteFile(pomPath, []byte("<project>\n    <version>1.0.0</version>\n</project>\n"), 0o644))

		ctx := &commands.RepoContext{
			Repo:     repo,
			Worktree: wt,
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().
				WithLanguagesConfig(map[string]entities.LanguageConfig{
					"java": {
						VersionFiles: []entities.VersionFile{
							{Path: "pom.xml", Patterns: []string{`(\s*<version>)[^<]+(</version>)`}},
						},
					},
				}).BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath(repoPath).
				WithLanguage("java").
				BuildProjectConfig(),
		}

		// when
		err = commands.UpdateChangelogAndVersionFiles(ctx, changelogPath)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.1.0", ctx.ProjectConfig.NewVersion)

		pomContent, readErr := os.ReadFile(pomPath)
		require.NoError(t, readErr)
		assert.Contains(t, string(pomContent), "<version>1.1.0</version>")
	})
}

func TestCommitChanges(t *testing.T) {
	t.Parallel()

	t.Run("should create a commit when changes are staged", func(t *testing.T) {
		// given
		repoPath, repo := createTestRepo(t)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		// Create and stage a file
		testFile := filepath.Join(repoPath, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0o644))
		_, err = wt.Add("test.txt")
		require.NoError(t, err)

		// Use a minimal git config without signing to avoid SSH agent issues
		globalGitConfig := &config.Config{}
		globalGitConfig.Raw = config.NewConfig().Raw
		globalGitConfig.Raw.Section("user").SetOption("name", "Test User")
		globalGitConfig.Raw.Section("user").SetOption("email", "test@test.com")

		ctx := &commands.RepoContext{
			Repo:            repo,
			Worktree:        wt,
			GlobalGitConfig: globalGitConfig,
			GlobalConfig:    entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithNewVersion("1.0.0").
				BuildProjectConfig(),
		}

		// when
		hash, err := commands.CommitChanges(ctx)

		// then
		require.NoError(t, err)
		assert.NotEqual(t, plumbing.ZeroHash, hash)

		// verify commit
		commit, commitErr := repo.CommitObject(hash)
		require.NoError(t, commitErr)
		assert.Contains(t, commit.Message, "chore(bump): bumped version to 1.0.0")
	})
}

func TestAddCurrentVersionWithTag(t *testing.T) {
	t.Parallel()

	t.Run("should skip gracefully when repo has no tags", func(t *testing.T) {
		// given
		repoPath, repo := createTestRepo(t)
		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		require.NoError(t, os.WriteFile(changelogPath, []byte("# Changelog\n\n## [Unreleased]\n"), 0o644))

		ctx := &commands.RepoContext{
			Repo: repo,
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath(repoPath).
				BuildProjectConfig(),
		}

		// when
		err := commands.AddCurrentVersion(ctx, changelogPath)

		// then
		require.NoError(t, err)
	})

	t.Run("should append version to changelog when repo has a lightweight tag", func(t *testing.T) {
		// given
		repoPath, repo := createTestRepo(t)
		head, err := repo.Head()
		require.NoError(t, err)

		// Create a lightweight tag
		err = repo.Storer.SetReference(
			plumbing.NewHashReference(plumbing.NewTagReferenceName("v1.0.0"), head.Hash()),
		)
		require.NoError(t, err)

		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		require.NoError(t, os.WriteFile(changelogPath, []byte("# Changelog\n\n## [Unreleased]\n"), 0o644))

		ctx := &commands.RepoContext{
			Repo: repo,
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath(repoPath).
				BuildProjectConfig(),
		}

		// when
		err = commands.AddCurrentVersion(ctx, changelogPath)

		// then
		require.NoError(t, err)
		content, readErr := os.ReadFile(changelogPath)
		require.NoError(t, readErr)
		assert.Contains(t, string(content), "[1.0.0]")
	})
}

func TestIterateProjects(t *testing.T) {
	t.Parallel()

	t.Run("should return error when project path does not exist and is not remote", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithProjects([]entities.ProjectConfig{
				{Path: "/nonexistent/local/path", Name: "test"},
			}).
			WithLanguagesConfig(map[string]entities.LanguageConfig{}).
			BuildGlobalConfig()

		// when
		err := commands.IterateProjects(globalConfig)

		// then
		require.Error(t, err)
	})
}

func TestHostKeyCallback(t *testing.T) {
	t.Parallel()

	t.Run("should not panic when called", func(t *testing.T) {
		// given / when / then
		assert.NotPanics(t, func() {
			_ = commands.HostKeyCallback()
		})
	})
}
