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

func TestGetAuthMethods_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	gitlabAccessToken := faker.Password()
	projectAccessToken := faker.Password()
	globalConfig := GlobalConfig{
		GitLabAccessToken: gitlabAccessToken,
	}
	projectConfig := ProjectConfig{
		ProjectAccessToken: projectAccessToken,
	}

	// Act
	authMethods, err := getAuthMethods(GITLAB, faker.Username(), &globalConfig, &projectConfig)

	// Assert
	require.NoError(t, err)
	assert.Len(t, authMethods, 2)

	basicAuthFound := false
	for _, authMethod := range authMethods {
		if auth, ok := authMethod.(*http.BasicAuth); ok {
			if auth.Password != gitlabAccessToken && auth.Password != projectAccessToken {
				t.Errorf("expected password to be either gitlabAccessToken or projectAccessToken, got %v",
					auth.Password)
			}
			basicAuthFound = true
		}
	}

	assert.True(t, basicAuthFound)
}

func TestGetAuthMethods_NoAuthMethodFound(t *testing.T) {
	t.Parallel()

	// Arrange
	globalConfig := GlobalConfig{}
	projectConfig := ProjectConfig{}

	// Act
	authMethods, err := getAuthMethods(GITLAB, faker.Username(), &globalConfig, &projectConfig)

	// Assert
	require.ErrorIs(t, err, ErrNoAuthMethodFound)
	assert.Empty(t, authMethods)
}

func TestGetAuthMethods_AuthNotImplemented(t *testing.T) {
	t.Parallel()

	// Arrange
	globalConfig := GlobalConfig{}
	projectConfig := ProjectConfig{}

	// Act
	authMethods, err := getAuthMethods(UNKNOWN, faker.Username(), &globalConfig, &projectConfig)

	// Assert
	require.ErrorIs(t, err, ErrAuthNotImplemented)
	assert.Empty(t, authMethods)
}

func TestGetRemoteServiceType_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, err := git.Init(memory.NewStorage(), nil)
	require.NoError(t, err)

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"https://gitlab.com/user/repo.git"},
	})
	require.NoError(t, err)

	// Act
	serviceType, err := getRemoteServiceType(repo)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, GITLAB, serviceType)
}

func TestGetRemoteServiceType_UnknownService(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, err := git.Init(memory.NewStorage(), nil)
	require.NoError(t, err)

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{faker.URL()},
	})
	require.NoError(t, err)

	// Act
	serviceType, err := getRemoteServiceType(repo)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, UNKNOWN, serviceType)
}

func TestGetLatestTag_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	fs := memfs.New()
	repo, err := git.Init(memory.NewStorage(), fs)
	require.NoError(t, err)

	// Add a file to the repository
	wt, err := repo.Worktree()
	require.NoError(t, err)

	// Write a new file to the in-memory filesystem
	file, err := fs.Create("example.txt")
	require.NoError(t, err)

	_, err = file.Write([]byte(faker.Sentence()))
	require.NoError(t, err)
	file.Close()

	// Add the new file to the staging area
	_, err = wt.Add("example.txt")
	require.NoError(t, err)

	// Commit the changes
	_, err = wt.Commit(faker.Sentence(), &git.CommitOptions{
		Author: &object.Signature{
			Name:  faker.Name(),
			Email: faker.Email(),
		},
		All: true,
	})
	require.NoError(t, err)

	// Get the HEAD reference
	head, err := repo.Head()
	require.NoError(t, err)

	// Create a random tag on the commit
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

	// Act
	tag, err := getLatestTag(repo)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, tag)
	assert.Equal(t, testTag, tag.Tag.String())
}

func TestGetLatestTag_NoTagsFound(t *testing.T) {
	t.Parallel()

	// Arrange
	fs := memfs.New()
	repo, err := git.Init(memory.NewStorage(), fs)
	require.NoError(t, err)

	// Add a file to the repository
	wt, err := repo.Worktree()
	require.NoError(t, err)

	// Write a new file to the in-memory filesystem
	file, err := fs.Create("example.txt")
	require.NoError(t, err)

	_, err = file.Write([]byte("hello world"))
	require.NoError(t, err)
	file.Close()

	// Add the new file to the staging area
	_, err = wt.Add("example.txt")
	require.NoError(t, err)

	// Commit the changes
	_, err = wt.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  faker.Name(),
			Email: faker.Email(),
		},
		All: true,
	})
	require.NoError(t, err)

	// Act
	_, err = getLatestTag(repo)
	// Assert

	require.ErrorIs(t, err, ErrNoTagsFound)
}
