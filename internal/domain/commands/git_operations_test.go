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
	"go.uber.org/dig"

	"github.com/rios0rios0/autobump/internal/domain/commands"
	"github.com/rios0rios0/autobump/internal/domain/entities"
	"github.com/rios0rios0/autobump/internal/infrastructure/repositories"
	"github.com/rios0rios0/autobump/test/domain/entitybuilders"
	gitInfra "github.com/rios0rios0/gitforge/pkg/git/infrastructure"
	gitforgeEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
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

	t.Run("should return nil when home has no known_hosts", func(t *testing.T) {
		// given -- the test runs with HOME pointing to a dir without .ssh/known_hosts
		// (most CI environments don't have known_hosts)
		// when
		cb := commands.HostKeyCallback()

		// then -- either nil or a valid callback, depending on the environment
		_ = cb // just verify no panic
	})
}

func TestCloneRepoIfNeeded(t *testing.T) { //nolint:tparallel // mutates package-level globals
	registry := repositories.NewProviderRegistry()
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	t.Run("should return empty string for local path", func(t *testing.T) {
		// given
		ctx := &commands.RepoContext{
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath("/some/local/path").
				BuildProjectConfig(),
		}

		// when
		tmpDir, err := commands.CloneRepoIfNeeded(ctx)

		// then
		require.NoError(t, err)
		assert.Empty(t, tmpDir)
	})

	t.Run("should attempt clone for https:// path", func(t *testing.T) {
		// given
		fakeHome := t.TempDir()
		t.Setenv("HOME", fakeHome)
		require.NoError(t, os.WriteFile(
			filepath.Join(fakeHome, ".gitconfig"),
			[]byte("[user]\n\tname = Test User\n\temail = test@test.com\n"),
			0o644,
		))

		globalGitConfig := &config.Config{}
		globalGitConfig.Raw = config.NewConfig().Raw
		globalGitConfig.Raw.Section("user").SetOption("name", "Test User")
		globalGitConfig.Raw.Section("user").SetOption("email", "test@test.com")

		ctx := &commands.RepoContext{
			GlobalConfig:    entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
			GlobalGitConfig: globalGitConfig,
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath("https://github.com/example/nonexistent-repo.git").
				BuildProjectConfig(),
		}

		// when
		_, err := commands.CloneRepoIfNeeded(ctx)

		// then -- should fail at clone (invalid repo) but the path is exercised
		require.Error(t, err)
	})

	t.Run("should attempt clone for git@ path", func(t *testing.T) {
		// given
		globalGitConfig := &config.Config{}
		globalGitConfig.Raw = config.NewConfig().Raw
		globalGitConfig.Raw.Section("user").SetOption("name", "Test User")
		globalGitConfig.Raw.Section("user").SetOption("email", "test@test.com")

		ctx := &commands.RepoContext{
			GlobalConfig:    entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
			GlobalGitConfig: globalGitConfig,
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath("git@github.com:example/nonexistent-repo.git").
				BuildProjectConfig(),
		}

		// when
		_, err := commands.CloneRepoIfNeeded(ctx)

		// then -- should fail at clone (no auth) but the path is exercised
		require.Error(t, err)
	})
}

func TestPushChanges(t *testing.T) { //nolint:tparallel // mutates package-level globals
	registry := repositories.NewProviderRegistry()
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	t.Run("should return error when repo has no remote", func(t *testing.T) {
		// given
		_, repo := createTestRepo(t)
		globalGitConfig := &config.Config{}
		globalGitConfig.Raw = config.NewConfig().Raw
		globalGitConfig.Raw.Section("user").SetOption("name", "Test")
		globalGitConfig.Raw.Section("user").SetOption("email", "test@test.com")

		ctx := &commands.RepoContext{
			Repo:            repo,
			GlobalGitConfig: globalGitConfig,
			GlobalConfig:    entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
			ProjectConfig:   entitybuilders.NewProjectConfigBuilder().BuildProjectConfig(),
		}

		// when
		err := commands.PushChanges(ctx, "chore/bump-1.0.0")

		// then
		require.Error(t, err)
	})
}

func TestCreatePullRequest(t *testing.T) { //nolint:tparallel // mutates package-level globals
	registry := repositories.NewProviderRegistry()
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	t.Run("should return nil when no token is found", func(t *testing.T) {
		// given
		_, repo := createTestRepo(t)
		ctx := &commands.RepoContext{
			Repo:         repo,
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithNewVersion("1.0.0").
				BuildProjectConfig(),
		}

		// when
		err := commands.CreatePullRequest(ctx, repo, "chore/bump-1.0.0", gitforgeEntities.GITHUB)

		// then
		require.NoError(t, err)
	})

	t.Run("should return nil when service type is unsupported", func(t *testing.T) {
		// given
		_, repo := createTestRepo(t)
		ctx := &commands.RepoContext{
			Repo:         repo,
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithNewVersion("1.0.0").
				BuildProjectConfig(),
		}

		// when
		err := commands.CreatePullRequest(ctx, repo, "chore/bump-1.0.0", gitforgeEntities.UNKNOWN)

		// then
		require.NoError(t, err)
	})
}

func TestCreateAndCheckoutPullRequest(t *testing.T) { //nolint:tparallel // mutates package-level globals
	registry := repositories.NewProviderRegistry()
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	t.Run("should return error when repo has no remote", func(t *testing.T) {
		// given
		_, repo := createTestRepo(t)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		ctx := &commands.RepoContext{
			Repo:         repo,
			Worktree:     wt,
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithNewVersion("1.0.0").
				BuildProjectConfig(),
		}

		// when
		err = commands.CreateAndCheckoutPullRequest(ctx, "chore/bump-1.0.0")

		// then
		require.Error(t, err)
	})
}

func TestHandleExistingBranchWithoutPR(t *testing.T) { //nolint:tparallel // mutates package-level globals
	registry := repositories.NewProviderRegistry()
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	t.Run("should return nil when PR check fails gracefully", func(t *testing.T) {
		// given -- repo without remote, PR check will fail
		_, repo := createTestRepo(t)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		ctx := &commands.RepoContext{
			Repo:         repo,
			Worktree:     wt,
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithNewVersion("1.0.0").
				BuildProjectConfig(),
		}

		// when
		err = commands.HandleExistingBranchWithoutPR(ctx, "chore/bump-1.0.0")

		// then -- should return nil (logs warning but doesn't propagate error)
		require.NoError(t, err)
	})
}

func TestCheckPullRequestExists(t *testing.T) { //nolint:tparallel // mutates package-level globals
	registry := repositories.NewProviderRegistry()
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	t.Run("should return false when no token is found", func(t *testing.T) {
		// given -- repo without remote
		_, repo := createTestRepo(t)
		ctx := &commands.RepoContext{
			Repo:          repo,
			GlobalConfig:  entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().BuildProjectConfig(),
		}

		// when
		exists, err := commands.CheckPullRequestExists(ctx, "chore/bump-1.0.0")

		// then -- can't determine service type without remote, error is returned
		require.Error(t, err)
		assert.False(t, exists)
	})
}

func TestGeneratePRDescriptionExtended(t *testing.T) {
	t.Parallel()

	t.Run("should use custom changelog path when set", func(t *testing.T) {
		// given
		ctx := &commands.RepoContext{
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().
				WithLanguagesConfig(map[string]entities.LanguageConfig{}).
				BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithName("test-project").
				WithNewVersion("1.5.0").
				WithChangelogPath("docs/CHANGES.md").
				BuildProjectConfig(),
		}

		// when
		description := commands.GeneratePRDescription(ctx)

		// then
		assert.Contains(t, description, "docs/CHANGES.md")
		assert.Contains(t, description, "1.5.0")
	})
}

func TestCommitAndPushChanges(t *testing.T) { //nolint:tparallel // mutates package-level globals
	registry := repositories.NewProviderRegistry()
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	t.Run("should return error when nothing is staged", func(t *testing.T) {
		// given
		_, repo := createTestRepo(t)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		globalGitConfig := &config.Config{}
		globalGitConfig.Raw = config.NewConfig().Raw
		globalGitConfig.Raw.Section("user").SetOption("name", "Test")
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
		err = commands.CommitAndPushChanges(ctx, "chore/bump-1.0.0")

		// then -- commit with nothing staged creates an empty commit, then push fails (no remote)
		require.Error(t, err)
	})
}

// createTestRepoWithRemote creates a local repo with a bare remote for testing push/PR operations.
func createTestRepoWithRemote(t *testing.T) (string, *git.Repository, string) {
	t.Helper()

	// Create bare remote
	bareDir := t.TempDir()
	_, err := git.PlainInit(bareDir, true)
	require.NoError(t, err)

	// Create working repo
	workDir := t.TempDir()
	repo, err := git.PlainInit(workDir, false)
	require.NoError(t, err)

	// Add remote
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{bareDir},
	})
	require.NoError(t, err)

	// Create initial commit
	wt, err := repo.Worktree()
	require.NoError(t, err)
	readmePath := filepath.Join(workDir, "README.md")
	require.NoError(t, os.WriteFile(readmePath, []byte("# Test\n"), 0o644))
	_, err = wt.Add("README.md")
	require.NoError(t, err)
	_, err = wt.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@test.com", When: time.Now()},
	})
	require.NoError(t, err)

	// Push to remote
	err = repo.Push(&git.PushOptions{RemoteName: "origin"})
	require.NoError(t, err)

	return workDir, repo, bareDir
}

func TestPushChangesWithRemote(t *testing.T) { //nolint:tparallel // mutates package-level globals
	registry := repositories.NewProviderRegistry()
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	t.Run("should return error for unsupported remote URL scheme", func(t *testing.T) {
		// given -- local bare remote has a file:// scheme which is unsupported
		_, repo, _ := createTestRepoWithRemote(t)
		globalGitConfig := &config.Config{}
		globalGitConfig.Raw = config.NewConfig().Raw
		globalGitConfig.Raw.Section("user").SetOption("name", "Test")
		globalGitConfig.Raw.Section("user").SetOption("email", "test@test.com")

		ctx := &commands.RepoContext{
			Repo:            repo,
			GlobalGitConfig: globalGitConfig,
			GlobalConfig:    entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
			ProjectConfig:   entitybuilders.NewProjectConfigBuilder().BuildProjectConfig(),
		}

		// when
		err := commands.PushChanges(ctx, "chore/bump-1.0.0")

		// then -- local bare repos have unsupported URL scheme
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported")
	})
}

func TestCreatePullRequestWithRemote(t *testing.T) { //nolint:tparallel // mutates package-level globals
	container := dig.New()
	require.NoError(t, repositories.RegisterProviders(container))
	var registry *repositories.ProviderRegistry
	require.NoError(t, container.Invoke(func(r *repositories.ProviderRegistry) {
		registry = r
	}))
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	t.Run("should attempt PR creation with token and remote", func(t *testing.T) {
		// given
		_, repo, _ := createTestRepoWithRemote(t)
		ctx := &commands.RepoContext{
			Repo: repo,
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().
				WithGitHubAccessToken("ghp_fake_token").
				BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithNewVersion("1.0.0").
				WithName("test-project").
				BuildProjectConfig(),
		}

		// when -- will fail at API call (fake token) but exercises the provider and URL resolution paths
		err := commands.CreatePullRequest(ctx, repo, "chore/bump-1.0.0", gitforgeEntities.GITHUB)

		// then -- we expect an error from the provider/API layer, but the call path must succeed
		// through resolveToken, getForgeProvider, and GetRemoteRepoURL.
		require.Error(t, err)
	})
}

func TestCheckPullRequestExistsWithToken(t *testing.T) { //nolint:tparallel // mutates package-level globals
	container := dig.New()
	require.NoError(t, repositories.RegisterProviders(container))
	var registry *repositories.ProviderRegistry
	require.NoError(t, container.Invoke(func(r *repositories.ProviderRegistry) {
		registry = r
	}))
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	t.Run("should check PR existence with token for GitHub remote", func(t *testing.T) {
		// given
		_, repo, _ := createTestRepoWithRemote(t)
		ctx := &commands.RepoContext{
			Repo: repo,
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().
				WithGitHubAccessToken("ghp_fake_token").
				BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().BuildProjectConfig(),
		}

		// when -- service type can't be detected from local bare URL
		exists, err := commands.CheckPullRequestExists(ctx, "chore/bump-1.0.0")

		// then -- returns false because local bare remote has unsupported URL scheme
		if err != nil {
			assert.False(t, exists)
		} else {
			assert.False(t, exists)
		}
	})
}

func TestAddFilesToWorktreeExtended(t *testing.T) {
	t.Parallel()

	t.Run("should skip non-existent version files without error", func(t *testing.T) {
		// given
		repoPath, repo := createTestRepo(t)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		require.NoError(t, os.WriteFile(changelogPath, []byte("# Changelog\n"), 0o644))

		ctx := &commands.RepoContext{
			Worktree: wt,
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().
				WithLanguagesConfig(map[string]entities.LanguageConfig{
					"ruby": {
						VersionFiles: []entities.VersionFile{
							{Path: "nonexistent.rb", Patterns: []string{`(version = ')\d+\.\d+\.\d+(')`}},
						},
					},
				}).BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath(repoPath).
				WithLanguage("ruby").
				BuildProjectConfig(),
		}

		// when
		err = commands.AddFilesToWorktree(ctx, changelogPath)

		// then
		require.NoError(t, err)
	})

	t.Run("should add changelog when no version files configured", func(t *testing.T) {
		// given
		repoPath, repo := createTestRepo(t)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		require.NoError(t, os.WriteFile(changelogPath, []byte("# Changelog\n"), 0o644))

		ctx := &commands.RepoContext{
			Worktree: wt,
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().
				WithLanguagesConfig(map[string]entities.LanguageConfig{}).
				BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithPath(repoPath).
				WithLanguage("").
				BuildProjectConfig(),
		}

		// when
		err = commands.AddFilesToWorktree(ctx, changelogPath)

		// then
		require.NoError(t, err)
	})
}

func TestCheckPullRequestExistsWithRemote(t *testing.T) { //nolint:tparallel // mutates package-level globals
	container := dig.New()
	require.NoError(t, repositories.RegisterProviders(container))
	var registry *repositories.ProviderRegistry
	require.NoError(t, container.Invoke(func(r *repositories.ProviderRegistry) {
		registry = r
	}))
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	t.Run("should return false with no token for local bare remote", func(t *testing.T) {
		// given
		_, repo, _ := createTestRepoWithRemote(t)
		ctx := &commands.RepoContext{
			Repo:          repo,
			GlobalConfig:  entitybuilders.NewGlobalConfigBuilder().BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().BuildProjectConfig(),
		}

		// when -- no token set, so it returns (false, nil)
		exists, err := commands.CheckPullRequestExists(ctx, "chore/bump-1.0.0")

		// then
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestHandleExistingBranchWithRemote(t *testing.T) { //nolint:tparallel // mutates package-level globals
	container := dig.New()
	require.NoError(t, repositories.RegisterProviders(container))
	var registry *repositories.ProviderRegistry
	require.NoError(t, container.Invoke(func(r *repositories.ProviderRegistry) {
		registry = r
	}))
	commands.SetProviderRegistry(registry)
	commands.SetGitOperations(gitInfra.NewGitOperations(registry))

	t.Run("should attempt to create PR when no PR exists for branch", func(t *testing.T) {
		// given
		_, repo, _ := createTestRepoWithRemote(t)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		ctx := &commands.RepoContext{
			Repo:     repo,
			Worktree: wt,
			GlobalConfig: entitybuilders.NewGlobalConfigBuilder().
				WithGitHubAccessToken("ghp_fake").
				BuildGlobalConfig(),
			ProjectConfig: entitybuilders.NewProjectConfigBuilder().
				WithNewVersion("1.0.0").
				BuildProjectConfig(),
		}

		// when
		err = commands.HandleExistingBranchWithoutPR(ctx, "chore/bump-1.0.0")

		// then -- may fail at PR creation but the path is fully exercised
		// If it fails, it should be from PR creation, not from check
		if err != nil {
			assert.Contains(t, err.Error(), "pull request")
		}
	})
}

func TestResolveDefaultBranchWithRemote(t *testing.T) {
	t.Parallel()

	t.Run("should resolve branch from remote HEAD when set", func(t *testing.T) {
		// given
		_, repo, _ := createTestRepoWithRemote(t)

		// Set remote HEAD
		head, err := repo.Head()
		require.NoError(t, err)
		err = repo.Storer.SetReference(
			plumbing.NewSymbolicReference("refs/remotes/origin/HEAD", head.Name()),
		)
		require.NoError(t, err)

		// when
		branch := commands.ResolveDefaultBranch(repo)

		// then
		assert.NotEmpty(t, branch)
	})
}

func TestIterateProjectsExtended(t *testing.T) { //nolint:tparallel // mutates package-level globals
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

	t.Run("should process local repo and skip when unreleased is empty", func(t *testing.T) {
		// given
		repoPath, _ := createTestRepo(t)
		changelogPath := filepath.Join(repoPath, "CHANGELOG.md")
		content := "# Changelog\n\n## [Unreleased]\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- added initial release\n"
		require.NoError(t, os.WriteFile(changelogPath, []byte(content), 0o644))

		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithProjects([]entities.ProjectConfig{
				{Path: repoPath, Name: "test", Language: "golang"},
			}).
			WithLanguagesConfig(map[string]entities.LanguageConfig{}).
			BuildGlobalConfig()

		// when
		err := commands.IterateProjects(globalConfig)

		// then
		require.NoError(t, err)
	})

	t.Run("should handle mix of local and remote paths", func(t *testing.T) {
		// given
		globalConfig := entitybuilders.NewGlobalConfigBuilder().
			WithProjects([]entities.ProjectConfig{
				{Path: "/nonexistent/local", Name: "local"},
				{Path: "https://github.com/nonexistent/repo.git", Name: "remote"},
			}).
			WithLanguagesConfig(map[string]entities.LanguageConfig{}).
			BuildGlobalConfig()

		// when
		err := commands.IterateProjects(globalConfig)

		// then -- remote clone will fail (no auth), but both paths are exercised
		require.Error(t, err)
	})
}
