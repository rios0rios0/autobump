package domain

import (
	"context"
	"time"

	"github.com/Masterminds/semver/v3"
)

// ServiceType represents the type of Git hosting service.
type ServiceType int

const (
	UNKNOWN ServiceType = iota
	GITHUB
	GITLAB
	AZUREDEVOPS
	BITBUCKET
	CODECOMMIT
)

// LatestTag holds information about the latest Git tag.
type LatestTag struct {
	Tag  *semver.Version
	Date time.Time
}

// BranchStatus represents the status of the bump branch.
type BranchStatus int

const (
	BranchCreated      BranchStatus = iota // Branch was newly created
	BranchExistsWithPR                     // Branch exists and PR exists - skip entirely
	BranchExistsNoPR                       // Branch exists but no PR - need to create PR
)

// Language is the interface for language-specific project operations.
type Language interface {
	GetProjectName() (string, error)
}

// Repository represents a Git repository discovered from a hosting provider.
type Repository struct {
	ID            string
	Name          string
	Organization  string
	Project       string // Azure DevOps only; empty for GitHub/GitLab
	DefaultBranch string
	CloneURL      string
}

// RepositoryDiscoverer can list repositories from a Git hosting provider.
type RepositoryDiscoverer interface {
	// Name returns the provider identifier (e.g. "github").
	Name() string
	// DiscoverRepositories lists all repositories in an organization or group.
	DiscoverRepositories(ctx context.Context, org string) ([]Repository, error)
}
