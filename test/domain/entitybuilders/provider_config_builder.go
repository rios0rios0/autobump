//go:build integration || unit || test

package entitybuilders //nolint:revive,staticcheck // Test package naming follows established project structure

import (
	"github.com/rios0rios0/autobump/internal/domain/entities"
	testkit "github.com/rios0rios0/testkit/pkg/test"
)

// ProviderConfigBuilder helps create test ProviderConfig instances with a fluent interface.
type ProviderConfigBuilder struct {
	*testkit.BaseBuilder
	providerType  string
	token         string
	organizations []string
}

// NewProviderConfigBuilder creates a new ProviderConfig builder with sensible defaults.
func NewProviderConfigBuilder() *ProviderConfigBuilder {
	return &ProviderConfigBuilder{
		BaseBuilder:   testkit.NewBaseBuilder(),
		providerType:  "",
		token:         "",
		organizations: nil,
	}
}

// WithType sets the provider type.
func (b *ProviderConfigBuilder) WithType(providerType string) *ProviderConfigBuilder {
	b.providerType = providerType
	return b
}

// WithToken sets the provider token.
func (b *ProviderConfigBuilder) WithToken(token string) *ProviderConfigBuilder {
	b.token = token
	return b
}

// WithOrganizations sets the organizations.
func (b *ProviderConfigBuilder) WithOrganizations(organizations []string) *ProviderConfigBuilder {
	b.organizations = organizations
	return b
}

// Build creates the ProviderConfig (satisfies testkit.Builder interface).
func (b *ProviderConfigBuilder) Build() interface{} {
	return b.BuildProviderConfig()
}

// BuildProviderConfig creates the ProviderConfig with a concrete return type.
func (b *ProviderConfigBuilder) BuildProviderConfig() entities.ProviderConfig {
	return entities.ProviderConfig{
		Type:          b.providerType,
		Token:         b.token,
		Organizations: b.organizations,
	}
}

// Reset clears the builder state.
func (b *ProviderConfigBuilder) Reset() testkit.Builder {
	b.BaseBuilder.Reset()
	b.providerType = ""
	b.token = ""
	b.organizations = nil
	return b
}

// Clone creates a deep copy.
func (b *ProviderConfigBuilder) Clone() testkit.Builder {
	var orgsCopy []string
	if b.organizations != nil {
		orgsCopy = make([]string, len(b.organizations))
		copy(orgsCopy, b.organizations)
	}

	return &ProviderConfigBuilder{
		BaseBuilder:   b.BaseBuilder.Clone().(*testkit.BaseBuilder),
		providerType:  b.providerType,
		token:         b.token,
		organizations: orgsCopy,
	}
}
