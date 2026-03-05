//go:build integration || unit || test

package entitybuilders //nolint:revive,staticcheck // Test package naming follows established project structure

import (
	"github.com/rios0rios0/autobump/internal/domain/entities"
	testkit "github.com/rios0rios0/testkit/pkg/test"
)

// ProjectConfigBuilder helps create test ProjectConfig instances with a fluent interface.
type ProjectConfigBuilder struct {
	*testkit.BaseBuilder
	path               string
	name               string
	language           string
	projectAccessToken string
	newVersion         string
}

// NewProjectConfigBuilder creates a new ProjectConfig builder with sensible defaults.
func NewProjectConfigBuilder() *ProjectConfigBuilder {
	return &ProjectConfigBuilder{
		BaseBuilder:        testkit.NewBaseBuilder(),
		path:               "",
		name:               "",
		language:           "",
		projectAccessToken: "",
		newVersion:         "",
	}
}

// WithPath sets the project path.
func (b *ProjectConfigBuilder) WithPath(path string) *ProjectConfigBuilder {
	b.path = path
	return b
}

// WithName sets the project name.
func (b *ProjectConfigBuilder) WithName(name string) *ProjectConfigBuilder {
	b.name = name
	return b
}

// WithLanguage sets the project language.
func (b *ProjectConfigBuilder) WithLanguage(language string) *ProjectConfigBuilder {
	b.language = language
	return b
}

// WithProjectAccessToken sets the project access token.
func (b *ProjectConfigBuilder) WithProjectAccessToken(token string) *ProjectConfigBuilder {
	b.projectAccessToken = token
	return b
}

// WithNewVersion sets the new version.
func (b *ProjectConfigBuilder) WithNewVersion(version string) *ProjectConfigBuilder {
	b.newVersion = version
	return b
}

// Build creates the ProjectConfig (satisfies testkit.Builder interface).
func (b *ProjectConfigBuilder) Build() interface{} {
	return b.BuildProjectConfig()
}

// BuildProjectConfig creates the ProjectConfig with a concrete return type.
func (b *ProjectConfigBuilder) BuildProjectConfig() *entities.ProjectConfig {
	return &entities.ProjectConfig{
		Path:               b.path,
		Name:               b.name,
		Language:           b.language,
		ProjectAccessToken: b.projectAccessToken,
		NewVersion:         b.newVersion,
	}
}

// Reset clears the builder state.
func (b *ProjectConfigBuilder) Reset() testkit.Builder {
	b.BaseBuilder.Reset()
	b.path = ""
	b.name = ""
	b.language = ""
	b.projectAccessToken = ""
	b.newVersion = ""
	return b
}

// Clone creates a deep copy.
func (b *ProjectConfigBuilder) Clone() testkit.Builder {
	return &ProjectConfigBuilder{
		BaseBuilder:        b.BaseBuilder.Clone().(*testkit.BaseBuilder),
		path:               b.path,
		name:               b.name,
		language:           b.language,
		projectAccessToken: b.projectAccessToken,
		newVersion:         b.newVersion,
	}
}
