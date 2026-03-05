//go:build integration || unit || test

package entitybuilders //nolint:revive,staticcheck // Test package naming follows established project structure

import (
	"github.com/rios0rios0/autobump/internal/domain/entities"
	testkit "github.com/rios0rios0/testkit/pkg/test"
)

// GlobalConfigBuilder helps create test GlobalConfig instances with a fluent interface.
type GlobalConfigBuilder struct {
	*testkit.BaseBuilder
	providers              []entities.ProviderConfig
	projects               []entities.ProjectConfig
	languagesConfig        map[string]entities.LanguageConfig
	gpgKeyPath             string
	gitLabAccessToken      string
	azureDevOpsAccessToken string
	gitHubAccessToken      string
	gitLabCIJobToken       string
}

// NewGlobalConfigBuilder creates a new GlobalConfig builder with sensible defaults.
func NewGlobalConfigBuilder() *GlobalConfigBuilder {
	return &GlobalConfigBuilder{
		BaseBuilder:            testkit.NewBaseBuilder(),
		providers:              nil,
		projects:               nil,
		languagesConfig:        make(map[string]entities.LanguageConfig),
		gpgKeyPath:             "",
		gitLabAccessToken:      "",
		azureDevOpsAccessToken: "",
		gitHubAccessToken:      "",
		gitLabCIJobToken:       "",
	}
}

// WithProviders sets the providers.
func (b *GlobalConfigBuilder) WithProviders(providers []entities.ProviderConfig) *GlobalConfigBuilder {
	b.providers = providers
	return b
}

// WithProjects sets the projects.
func (b *GlobalConfigBuilder) WithProjects(projects []entities.ProjectConfig) *GlobalConfigBuilder {
	b.projects = projects
	return b
}

// WithLanguagesConfig sets the languages configuration.
func (b *GlobalConfigBuilder) WithLanguagesConfig(
	languagesConfig map[string]entities.LanguageConfig,
) *GlobalConfigBuilder {
	b.languagesConfig = languagesConfig
	return b
}

// WithGpgKeyPath sets the GPG key path.
func (b *GlobalConfigBuilder) WithGpgKeyPath(gpgKeyPath string) *GlobalConfigBuilder {
	b.gpgKeyPath = gpgKeyPath
	return b
}

// WithGitLabAccessToken sets the GitLab access token.
func (b *GlobalConfigBuilder) WithGitLabAccessToken(token string) *GlobalConfigBuilder {
	b.gitLabAccessToken = token
	return b
}

// WithAzureDevOpsAccessToken sets the Azure DevOps access token.
func (b *GlobalConfigBuilder) WithAzureDevOpsAccessToken(token string) *GlobalConfigBuilder {
	b.azureDevOpsAccessToken = token
	return b
}

// WithGitHubAccessToken sets the GitHub access token.
func (b *GlobalConfigBuilder) WithGitHubAccessToken(token string) *GlobalConfigBuilder {
	b.gitHubAccessToken = token
	return b
}

// WithGitLabCIJobToken sets the GitLab CI job token.
func (b *GlobalConfigBuilder) WithGitLabCIJobToken(token string) *GlobalConfigBuilder {
	b.gitLabCIJobToken = token
	return b
}

// Build creates the GlobalConfig (satisfies testkit.Builder interface).
func (b *GlobalConfigBuilder) Build() interface{} {
	return b.BuildGlobalConfig()
}

// BuildGlobalConfig creates the GlobalConfig with a concrete return type.
func (b *GlobalConfigBuilder) BuildGlobalConfig() *entities.GlobalConfig {
	return &entities.GlobalConfig{
		Providers:              b.providers,
		Projects:               b.projects,
		LanguagesConfig:        b.languagesConfig,
		GpgKeyPath:             b.gpgKeyPath,
		GitLabAccessToken:      b.gitLabAccessToken,
		AzureDevOpsAccessToken: b.azureDevOpsAccessToken,
		GitHubAccessToken:      b.gitHubAccessToken,
		GitLabCIJobToken:       b.gitLabCIJobToken,
	}
}

// Reset clears the builder state.
func (b *GlobalConfigBuilder) Reset() testkit.Builder {
	b.BaseBuilder.Reset()
	b.providers = nil
	b.projects = nil
	b.languagesConfig = make(map[string]entities.LanguageConfig)
	b.gpgKeyPath = ""
	b.gitLabAccessToken = ""
	b.azureDevOpsAccessToken = ""
	b.gitHubAccessToken = ""
	b.gitLabCIJobToken = ""
	return b
}

// Clone creates a deep copy.
func (b *GlobalConfigBuilder) Clone() testkit.Builder {
	var providersCopy []entities.ProviderConfig
	if b.providers != nil {
		providersCopy = make([]entities.ProviderConfig, len(b.providers))
		copy(providersCopy, b.providers)
	}

	var projectsCopy []entities.ProjectConfig
	if b.projects != nil {
		projectsCopy = make([]entities.ProjectConfig, len(b.projects))
		copy(projectsCopy, b.projects)
	}

	languagesConfigCopy := make(map[string]entities.LanguageConfig, len(b.languagesConfig))
	for k, v := range b.languagesConfig {
		languagesConfigCopy[k] = v
	}

	return &GlobalConfigBuilder{
		BaseBuilder:            b.BaseBuilder.Clone().(*testkit.BaseBuilder),
		providers:              providersCopy,
		projects:               projectsCopy,
		languagesConfig:        languagesConfigCopy,
		gpgKeyPath:             b.gpgKeyPath,
		gitLabAccessToken:      b.gitLabAccessToken,
		azureDevOpsAccessToken: b.azureDevOpsAccessToken,
		gitHubAccessToken:      b.gitHubAccessToken,
		gitLabCIJobToken:       b.gitLabCIJobToken,
	}
}
