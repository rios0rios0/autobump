//go:build integration || unit || test

package entitybuilders //nolint:revive,staticcheck // Test package naming follows established project structure

import (
	"github.com/rios0rios0/autobump/internal/domain/entities"
	testkit "github.com/rios0rios0/testkit/pkg/test"
)

// RepositoryBuilder helps create test repositories with a fluent interface.
type RepositoryBuilder struct {
	*testkit.BaseBuilder
	id            string
	name          string
	organization  string
	project       string
	defaultBranch string
	remoteURL     string
}

// NewRepositoryBuilder creates a new repository builder with sensible defaults.
func NewRepositoryBuilder() *RepositoryBuilder {
	return &RepositoryBuilder{
		BaseBuilder:   testkit.NewBaseBuilder(),
		id:            "test-repo-id",
		name:          "test-repo",
		organization:  "test-org",
		project:       "",
		defaultBranch: "main",
		remoteURL:     "https://github.com/test-org/test-repo.git",
	}
}

// WithID sets the repository ID.
func (b *RepositoryBuilder) WithID(id string) *RepositoryBuilder {
	b.id = id
	return b
}

// WithName sets the repository name.
func (b *RepositoryBuilder) WithName(name string) *RepositoryBuilder {
	b.name = name
	return b
}

// WithOrganization sets the organization.
func (b *RepositoryBuilder) WithOrganization(org string) *RepositoryBuilder {
	b.organization = org
	return b
}

// WithProject sets the project (Azure DevOps only).
func (b *RepositoryBuilder) WithProject(project string) *RepositoryBuilder {
	b.project = project
	return b
}

// WithDefaultBranch sets the default branch.
func (b *RepositoryBuilder) WithDefaultBranch(branch string) *RepositoryBuilder {
	b.defaultBranch = branch
	return b
}

// WithRemoteURL sets the remote URL.
func (b *RepositoryBuilder) WithRemoteURL(url string) *RepositoryBuilder {
	b.remoteURL = url
	return b
}

// WithCloneURL is a backward-compatible alias for WithRemoteURL.
func (b *RepositoryBuilder) WithCloneURL(url string) *RepositoryBuilder {
	return b.WithRemoteURL(url)
}

// Build creates the repository (satisfies testkit.Builder interface).
func (b *RepositoryBuilder) Build() interface{} {
	return b.BuildRepository()
}

// BuildRepository creates the repository with a concrete return type.
func (b *RepositoryBuilder) BuildRepository() entities.Repository {
	return entities.Repository{
		ID:            b.id,
		Name:          b.name,
		Organization:  b.organization,
		Project:       b.project,
		DefaultBranch: b.defaultBranch,
		RemoteURL:     b.remoteURL,
	}
}

// Reset clears the builder state.
func (b *RepositoryBuilder) Reset() testkit.Builder {
	b.BaseBuilder.Reset()
	b.id = "test-repo-id"
	b.name = "test-repo"
	b.organization = "test-org"
	b.project = ""
	b.defaultBranch = "main"
	b.remoteURL = "https://github.com/test-org/test-repo.git"
	return b
}

// Clone creates a deep copy.
func (b *RepositoryBuilder) Clone() testkit.Builder {
	return &RepositoryBuilder{
		BaseBuilder:   b.BaseBuilder.Clone().(*testkit.BaseBuilder),
		id:            b.id,
		name:          b.name,
		organization:  b.organization,
		project:       b.project,
		defaultBranch: b.defaultBranch,
		remoteURL:     b.remoteURL,
	}
}
