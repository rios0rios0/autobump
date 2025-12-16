package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"testing"

	"github.com/go-faker/faker/v4"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAuthMethods(t *testing.T) {
	t.Run(
		"should return auth methods when GitLab access token and project access token are provided",
		func(t *testing.T) {
			// given
			gitlabAccessToken := faker.Password()
			projectAccessToken := faker.Password()
			globalConfig := GlobalConfig{
				GitLabAccessToken: gitlabAccessToken,
			}
			projectConfig := ProjectConfig{
				ProjectAccessToken: projectAccessToken,
			}

			// when
			authMethods, err := getAuthMethods(GITLAB, faker.Username(), &globalConfig, &projectConfig)

			// then
			require.NoError(t, err, "should not return an error")
			assert.Len(t, authMethods, 2, "should return 2 auth methods")

			basicAuthFound := false
			for _, authMethod := range authMethods {
				if auth, ok := authMethod.(*http.BasicAuth); ok {
					assert.True(t,
						auth.Password == gitlabAccessToken || auth.Password == projectAccessToken,
						"password should be either gitlabAccessToken or projectAccessToken",
					)
					basicAuthFound = true
				}
			}
			assert.True(t, basicAuthFound, "should find BasicAuth method")
		},
	)

	t.Run("should return error when no auth method is found", func(t *testing.T) {
		// given
		globalConfig := GlobalConfig{}
		projectConfig := ProjectConfig{}

		// when
		authMethods, err := getAuthMethods(GITLAB, faker.Username(), &globalConfig, &projectConfig)

		// then
		require.ErrorIs(t, err, ErrNoAuthMethodFound, "should return ErrNoAuthMethodFound")
		assert.Empty(t, authMethods, "auth methods should be empty")
	})

	t.Run("should return error when auth is not implemented for service type", func(t *testing.T) {
		// given
		globalConfig := GlobalConfig{}
		projectConfig := ProjectConfig{}

		// when
		authMethods, err := getAuthMethods(UNKNOWN, faker.Username(), &globalConfig, &projectConfig)

		// then
		require.ErrorIs(t, err, ErrAuthNotImplemented, "should return ErrAuthNotImplemented")
		assert.Empty(t, authMethods, "auth methods should be empty")
	})
}

func TestGetRemoteServiceType(t *testing.T) {
	t.Run("should return GITLAB for gitlab.com remote URL", func(t *testing.T) {
		// given
		repo, err := git.Init(memory.NewStorage(), nil)
		require.NoError(t, err)

		_, err = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{"https://gitlab.com/user/repo.git"},
		})
		require.NoError(t, err)

		// when
		serviceType, err := getRemoteServiceType(repo)

		// then
		require.NoError(t, err, "should not return an error")
		assert.Equal(t, GITLAB, serviceType, "should return GITLAB service type")
	})

	t.Run("should return UNKNOWN for unknown remote URL", func(t *testing.T) {
		// given
		repo, err := git.Init(memory.NewStorage(), nil)
		require.NoError(t, err)

		_, err = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{faker.URL()},
		})
		require.NoError(t, err)

		// when
		serviceType, err := getRemoteServiceType(repo)

		// then
		require.NoError(t, err, "should not return an error")
		assert.Equal(t, UNKNOWN, serviceType, "should return UNKNOWN service type")
	})

	t.Run("should return GITHUB for github.com remote URL", func(t *testing.T) {
		// given
		repo, err := git.Init(memory.NewStorage(), nil)
		require.NoError(t, err)

		_, err = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{"https://github.com/user/repo.git"},
		})
		require.NoError(t, err)

		// when
		serviceType, err := getRemoteServiceType(repo)

		// then
		require.NoError(t, err, "should not return an error")
		assert.Equal(t, GITHUB, serviceType, "should return GITHUB service type")
	})

	t.Run("should return AZUREDEVOPS for dev.azure.com remote URL", func(t *testing.T) {
		// given
		repo, err := git.Init(memory.NewStorage(), nil)
		require.NoError(t, err)

		_, err = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{"https://dev.azure.com/org/project/_git/repo"},
		})
		require.NoError(t, err)

		// when
		serviceType, err := getRemoteServiceType(repo)

		// then
		require.NoError(t, err, "should not return an error")
		assert.Equal(t, AZUREDEVOPS, serviceType, "should return AZUREDEVOPS service type")
	})
}

func TestGetLatestTag(t *testing.T) {
	t.Run("should return the latest tag from repository", func(t *testing.T) {
		// given
		fs := memfs.New()
		repo, err := git.Init(memory.NewStorage(), fs)
		require.NoError(t, err)

		wt, err := repo.Worktree()
		require.NoError(t, err)

		file, err := fs.Create("example.txt")
		require.NoError(t, err)

		_, err = file.Write([]byte(faker.Sentence()))
		require.NoError(t, err)
		file.Close()

		_, err = wt.Add("example.txt")
		require.NoError(t, err)

		_, err = wt.Commit(faker.Sentence(), &git.CommitOptions{
			Author: &object.Signature{
				Name:  faker.Name(),
				Email: faker.Email(),
			},
			All: true,
		})
		require.NoError(t, err)

		head, err := repo.Head()
		require.NoError(t, err)

		randMax := big.NewInt(10)
		major, err := rand.Int(rand.Reader, randMax)
		require.NoError(t, err)
		minor, err := rand.Int(rand.Reader, randMax)
		require.NoError(t, err)
		patch, err := rand.Int(rand.Reader, randMax)
		require.NoError(t, err)

		testTag := fmt.Sprintf("%d.%d.%d", major, minor, patch)
		_, err = repo.CreateTag(testTag, head.Hash(), nil)
		require.NoError(t, err)

		// when
		tag, err := getLatestTag(repo)

		// then
		require.NoError(t, err, "should not return an error")
		assert.NotNil(t, tag, "tag should not be nil")
		assert.Equal(t, testTag, tag.Tag.String(), "should return the correct tag")
	})

	t.Run("should return error when no tags are found in repository", func(t *testing.T) {
		// given
		fs := memfs.New()
		repo, err := git.Init(memory.NewStorage(), fs)
		require.NoError(t, err)

		wt, err := repo.Worktree()
		require.NoError(t, err)

		file, err := fs.Create("example.txt")
		require.NoError(t, err)

		_, err = file.Write([]byte("hello world"))
		require.NoError(t, err)
		file.Close()

		_, err = wt.Add("example.txt")
		require.NoError(t, err)

		_, err = wt.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  faker.Name(),
				Email: faker.Email(),
			},
			All: true,
		})
		require.NoError(t, err)

		// when
		_, err = getLatestTag(repo)

		// then
		require.ErrorIs(t, err, ErrNoTagsFound, "should return ErrNoTagsFound")
	})
}

func TestGetServiceTypeByURL(t *testing.T) {
	t.Run("should return BITBUCKET for bitbucket.org URL", func(t *testing.T) {
		// given
		url := "https://bitbucket.org/owner/repo.git"

		// when
		result := getServiceTypeByURL(url)

		// then
		assert.Equal(t, BITBUCKET, result, "should return BITBUCKET service type")
	})

	t.Run("should return CODECOMMIT for git-codecommit URL", func(t *testing.T) {
		// given
		url := "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/my-repo"

		// when
		result := getServiceTypeByURL(url)

		// then
		assert.Equal(t, CODECOMMIT, result, "should return CODECOMMIT service type")
	})

	t.Run("should return UNKNOWN for unrecognized URL", func(t *testing.T) {
		// given
		url := "https://custom-git-server.example.com/repo.git"

		// when
		result := getServiceTypeByURL(url)

		// then
		assert.Equal(t, UNKNOWN, result, "should return UNKNOWN service type")
	})
}

func TestGetAmountCommits(t *testing.T) {
	t.Run("should return correct number of commits", func(t *testing.T) {
		// given
		fs := memfs.New()
		repo, err := git.Init(memory.NewStorage(), fs)
		require.NoError(t, err)

		wt, err := repo.Worktree()
		require.NoError(t, err)

		// Create first commit
		file1, err := fs.Create("file1.txt")
		require.NoError(t, err)
		_, err = file1.Write([]byte("content1"))
		require.NoError(t, err)
		file1.Close()

		_, err = wt.Add("file1.txt")
		require.NoError(t, err)

		_, err = wt.Commit("first commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  faker.Name(),
				Email: faker.Email(),
			},
		})
		require.NoError(t, err)

		// Create second commit
		file2, err := fs.Create("file2.txt")
		require.NoError(t, err)
		_, err = file2.Write([]byte("content2"))
		require.NoError(t, err)
		file2.Close()

		_, err = wt.Add("file2.txt")
		require.NoError(t, err)

		_, err = wt.Commit("second commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  faker.Name(),
				Email: faker.Email(),
			},
		})
		require.NoError(t, err)

		// when
		count, err := getAmountCommits(repo)

		// then
		require.NoError(t, err, "should not return an error")
		assert.Equal(t, 2, count, "should return 2 commits")
	})

	t.Run("should return error for empty repository with no commits", func(t *testing.T) {
		// given
		repo, err := git.Init(memory.NewStorage(), nil)
		require.NoError(t, err)

		// when
		_, err = getAmountCommits(repo)

		// then
		require.Error(t, err, "should return an error for empty repository")
		assert.Contains(t, err.Error(), "could not get commits", "error should mention commits issue")
	})
}

func TestGetRemoteRepoURL(t *testing.T) {
	t.Run("should return remote URL when origin is configured", func(t *testing.T) {
		// given
		repo, err := git.Init(memory.NewStorage(), nil)
		require.NoError(t, err)

		expectedURL := "https://github.com/owner/repo.git"
		_, err = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{expectedURL},
		})
		require.NoError(t, err)

		// when
		url, err := getRemoteRepoURL(repo)

		// then
		require.NoError(t, err, "should not return an error")
		assert.Equal(t, expectedURL, url, "should return the remote URL")
	})

	t.Run("should return error when no origin remote exists", func(t *testing.T) {
		// given
		repo, err := git.Init(memory.NewStorage(), nil)
		require.NoError(t, err)

		// when
		_, err = getRemoteRepoURL(repo)

		// then
		require.Error(t, err, "should return an error")
		assert.Contains(t, err.Error(), "could not get remote", "error should mention remote issue")
	})
}
