package entities

import (
	gitforgeEntities "github.com/rios0rios0/gitforge/domain/entities"
)

// ServiceType is re-exported from gitforge.
type ServiceType = gitforgeEntities.ServiceType

//nolint:gochecknoglobals // re-exported constants from gitforge
var (
	UNKNOWN     = gitforgeEntities.UNKNOWN
	GITHUB      = gitforgeEntities.GITHUB
	GITLAB      = gitforgeEntities.GITLAB
	AZUREDEVOPS = gitforgeEntities.AZUREDEVOPS
	BITBUCKET   = gitforgeEntities.BITBUCKET
	CODECOMMIT  = gitforgeEntities.CODECOMMIT
)

// LatestTag is re-exported from gitforge.
type LatestTag = gitforgeEntities.LatestTag

// BranchStatus is re-exported from gitforge.
type BranchStatus = gitforgeEntities.BranchStatus

//nolint:gochecknoglobals // re-exported constants from gitforge
var (
	BranchCreated      = gitforgeEntities.BranchCreated
	BranchExistsWithPR = gitforgeEntities.BranchExistsWithPR
	BranchExistsNoPR   = gitforgeEntities.BranchExistsNoPR
)

// Language is the interface for language-specific project operations.
type Language interface {
	GetProjectName() (string, error)
}

// Repository is re-exported from gitforge.
type Repository = gitforgeEntities.Repository

// RepositoryDiscoverer is re-exported from gitforge.
type RepositoryDiscoverer = gitforgeEntities.RepositoryDiscoverer
