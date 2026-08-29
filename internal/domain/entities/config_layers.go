package entities

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	logger "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// The four configuration layers, in the order they are applied. Each one overrides only
// the keys its document declares. These strings are the vocabulary the README, CLAUDE.md
// and every log line use, so a message an operator sees can be traced back to the
// document that produced it.
const (
	LayerBuiltInDefaults   = "built-in defaults"
	LayerPublishedDefaults = "published defaults"
	LayerOperatorConfig    = "operator configuration"
	LayerProjectConfig     = "project configuration"
)

// LayerScope selects a layer's decode target, and with it the set of keys the layer is
// able to express at all.
type LayerScope int

const (
	// ScopeOperator decodes into GlobalConfig and may set every key, including
	// credentials, providers, the project list and the bump branch prefix. Only the file
	// the operator named with -c, or the one found in their home directory, gets it.
	ScopeOperator LayerScope = iota

	// ScopeRestricted decodes into RestrictedConfig, which has no field for a credential,
	// for providers, for projects or for the branch prefix. AutoBump's own shipped
	// defaults, the copy fetched from DefaultConfigURL and the .autobump.yaml inside the
	// repository being released all use it: none of the three is the operator speaking,
	// so none of them needs to be able to name a token or aim a branch deletion.
	ScopeRestricted
)

// ConfigLayer is one configuration source, already read into memory.
//
// Reading is deliberately somebody else's job. A layer is bytes, so the whole folding
// engine is testable without a file, a home directory or a network.
type ConfigLayer struct {
	// Name is the layer's place in the chain, one of the Layer* constants above.
	Name string

	// Origin is the path or URL the bytes came from, for logs and errors. It is empty
	// for the built-in defaults, which have no location an operator could look at.
	Origin string

	Data  []byte
	Scope LayerScope

	// Strict makes a key the schema does not know an error rather than a silently
	// ignored line. True for the documents this repository owns and for the operator's
	// own file, where a typo is a mistake worth hearing about; false for anything
	// fetched, which a newer release may have widened.
	Strict bool

	// Optional downgrades a decode failure to a warning and skips the layer, so a
	// document nobody wrote -- or a remote nobody can reach -- cannot stop a run.
	Optional bool
}

// describe names a layer the way a log line should: the layer's role, and where it came
// from when that is somewhere an operator can go and look.
func (l ConfigLayer) describe() string {
	if l.Origin == "" {
		return l.Name
	}
	return fmt.Sprintf("%s (%s)", l.Name, l.Origin)
}

// ResolveGlobalConfig folds the layers in order and finalises the result.
//
// Finalisation -- reading a token out of a file, expanding "~", resolving provider
// tokens, applying the environment fallbacks -- runs once, over the finished
// configuration. It cannot live inside ApplyLayer: a token path set by one layer and
// overridden by the next would otherwise be read from disk on the way past.
func ResolveGlobalConfig(layers []ConfigLayer) (*GlobalConfig, error) {
	config := &GlobalConfig{} //nolint:exhaustruct // every field is filled by the layers

	for _, layer := range layers {
		next, err := ApplyLayer(config, layer)
		if err != nil {
			if !layer.Optional {
				return nil, err
			}
			logger.Warnf("Ignoring the %s: %v", layer.describe(), err)
			continue
		}
		config = next
	}

	FinalizeGlobalConfig(config)

	return config, nil
}

// ApplyLayer folds one layer onto config and returns the result. config is not mutated.
//
// It is exported because the layers do not all arrive at the same time: the three
// operator-facing layers resolve before any repository is touched, while a project's own
// file only exists once the repository has been cloned.
func ApplyLayer(config *GlobalConfig, layer ConfigLayer) (*GlobalConfig, error) {
	if err := auditRemovedKeys(layer); err != nil {
		return nil, err
	}

	if layer.Scope == ScopeRestricted {
		return applyRestrictedLayer(config, layer)
	}

	return applyOperatorLayer(config, layer)
}

// applyOperatorLayer decodes a layer straight onto the accumulated configuration.
//
// Decoding a document into a *non-zero* struct sets only the keys the document carries
// and leaves every other field exactly as the layers before it left it -- which is the
// layering, for free, for every scalar, pointer and slice, and is why no field has to
// become a pointer for an explicit "false" to stay distinguishable from an omission.
//
// A map is the one exception. yaml.v3 decodes each map value into a fresh zero element,
// so a language this layer names would *replace* the inherited one rather than merge
// with it. Emptying the field before the decode and merging afterwards is what keeps
// MergeLanguagesConfig's rules -- version files matched by path, extensions appended and
// de-duplicated -- in force across layers rather than only within one.
func applyOperatorLayer(config *GlobalConfig, layer ConfigLayer) (*GlobalConfig, error) {
	// A shallow copy is a real copy here: yaml.v3 replaces a slice and a map wholesale
	// rather than writing through them, so nothing the decode does can reach the
	// configuration this layer is being folded onto. That is what lets an optional layer
	// fail halfway without leaving a half-applied configuration behind.
	next := *config

	inherited := config.LanguagesConfig
	next.LanguagesConfig = nil

	decoder := yaml.NewDecoder(bytes.NewReader(layer.Data))
	decoder.KnownFields(layer.Strict)

	// A document that is nothing but comments -- which every commented example is, and
	// which the shipped defaults nearly are -- reports io.EOF rather than decoding
	// nothing. That is a layer with nothing to say, not a broken one.
	if err := decoder.Decode(&next); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to decode the %s: %w", layer.describe(), err)
	}

	next.LanguagesConfig = MergeLanguagesConfig(inherited, next.LanguagesConfig)

	return &next, nil
}

// applyRestrictedLayer decodes a layer through the narrower schema, so the keys it is not
// allowed to set have nowhere to land.
func applyRestrictedLayer(config *GlobalConfig, layer ConfigLayer) (*GlobalConfig, error) {
	restricted, err := decodeRestricted(layer)
	if err != nil {
		return nil, err
	}

	return restricted.applyTo(config), nil
}

// decodeRestricted reads a layer through RestrictedConfig and reports the operator-only
// keys it tried to set.
func decodeRestricted(layer ConfigLayer) (RestrictedConfig, error) {
	var restricted RestrictedConfig //nolint:exhaustruct // the document decides what is set

	// Never strict. The point of RestrictedConfig is that it has no field for the keys a
	// restricted layer may not set, so strict decoding would turn "AutoBump ignored your
	// providers block" into "AutoBump refused to run".
	decoder := yaml.NewDecoder(bytes.NewReader(layer.Data))
	decoder.KnownFields(false)

	if err := decoder.Decode(&restricted); err != nil && !errors.Is(err, io.EOF) {
		return restricted, fmt.Errorf("failed to decode the %s: %w", layer.describe(), err)
	}

	warnOperatorOnlyKeys(layer)

	return restricted, nil
}

// ApplyProjectLayer folds the repository's own configuration onto both the accumulated
// configuration and the project entry being released, returning the new configuration and
// leaving the one it was given untouched.
//
// The project entry needs its own pass because the resolvers consult it first. Only what
// *this document* declares is carried across: reading the folded configuration instead
// would let the operator's own global default overwrite the `projects[]` entry they wrote
// beside it, which is the opposite of the precedence everything else follows.
func ApplyProjectLayer(
	config *GlobalConfig, projectConfig *ProjectConfig, layer ConfigLayer,
) (*GlobalConfig, error) {
	if err := auditRemovedKeys(layer); err != nil {
		return nil, err
	}

	restricted, err := decodeRestricted(layer)
	if err != nil {
		return nil, err
	}

	if projectConfig != nil {
		restricted.applyToProject(projectConfig)
	}

	return restricted.applyTo(config), nil
}

// applyToProject carries what the repository declared onto the project entry, so the
// repository's own file wins over the operator's `projects[]` entry for the settings both
// can express. Language is not among them: it is detected, or given with -l, never
// configured here.
func (r RestrictedConfig) applyToProject(projectConfig *ProjectConfig) {
	if r.ChangelogPath != "" {
		projectConfig.ChangelogPath = r.ChangelogPath
	}
	if r.Versioning != "" {
		projectConfig.Versioning = r.Versioning
	}
	if r.DetectChlog != nil {
		projectConfig.DetectChlog = r.DetectChlog
	}
	if r.Refresh != nil {
		projectConfig.Refresh = r.Refresh
	}
}

// RestrictedConfig is what a configuration layer that is not the operator's own may say.
//
// It is a separate struct rather than a filter over GlobalConfig because a struct with no
// field for a key is not a check that can be got round. A `providers:` block in a
// repository's .autobump.yaml has nowhere to land, whatever a later reader of this file
// believes about when the filtering happens or whether it was called.
//
// The booleans are pointers so that "absent" and "false" stay distinguishable, which is
// what lets a layer turn an inherited default *off* rather than only confirm it.
type RestrictedConfig struct {
	LanguagesConfig      map[string]LanguageConfig `yaml:"languages"`
	Refresh              *bool                     `yaml:"refresh"`
	DetectChlog          *bool                     `yaml:"detect_chlog"`
	CleanupStaleBranches *bool                     `yaml:"cleanup_stale_branches"`
	ExcludeForks         *bool                     `yaml:"exclude_forks"`
	ExcludeArchived      *bool                     `yaml:"exclude_archived"`
	ChangelogPath        string                    `yaml:"changelog_path"`
	Versioning           string                    `yaml:"versioning"`
}

// applyTo folds the restricted layer onto config, returning a copy.
func (r RestrictedConfig) applyTo(config *GlobalConfig) *GlobalConfig {
	next := *config

	if r.ChangelogPath != "" {
		next.ChangelogPath = r.ChangelogPath
	}
	if r.Versioning != "" {
		next.Versioning = r.Versioning
	}
	if r.Refresh != nil {
		next.Refresh = r.Refresh
	}
	if r.DetectChlog != nil {
		next.DetectChlog = r.DetectChlog
	}
	if r.CleanupStaleBranches != nil {
		next.CleanupStaleBranches = r.CleanupStaleBranches
	}
	if r.ExcludeForks != nil {
		next.ExcludeForks = *r.ExcludeForks
	}
	if r.ExcludeArchived != nil {
		next.ExcludeArchived = *r.ExcludeArchived
	}

	next.LanguagesConfig = MergeLanguagesConfig(config.LanguagesConfig, r.LanguagesConfig)

	return &next
}

const (
	reasonCredential = "credentials are the operator's to configure and are never read " +
		"from a repository or from the network"
	reasonDiscovery = "which repositories to release is the operator's to configure"
	reasonDeletion  = "the branch prefix decides which branches stale-branch cleanup " +
		"deletes, so it is the operator's alone to set"
)

// operatorOnlyKeys names the top-level keys a restricted layer may write and AutoBump
// will never honour. RestrictedConfig has no field for any of them, so this table changes
// nothing about what is decoded: it exists so that a document which tried is answered out
// loud rather than having the setting silently do nothing.
//
//nolint:gochecknoglobals // read-only lookup table
var operatorOnlyKeys = map[string]string{
	"providers":                 reasonDiscovery,
	"projects":                  reasonDiscovery,
	"bump_branch_prefix":        reasonDeletion,
	"gitlab_access_token":       reasonCredential,
	"github_access_token":       reasonCredential,
	"azure_devops_access_token": reasonCredential,
	"gitlab_ci_job_token":       reasonCredential,
	"gpg_key_path":              reasonCredential,
	"gpg_key_passphrase":        reasonCredential,
	"ssh_key_path":              reasonCredential,
	"ssh_key_passphrase":        reasonCredential,
	"ssh_auth_sock":             reasonCredential,
}

// removedKeys maps a key AutoBump no longer understands to the migration it needs. It is
// consulted before a layer is decoded, so an operator meets the instruction rather than
// yaml.v3's "field not found in type".
//
//nolint:gochecknoglobals // read-only lookup table
var removedKeys = map[string]string{
	"refresh_commands": "`refresh_commands` was removed in 3.0.0. AutoBump now owns the " +
		"command and a configuration only says whether to run it: replace the block with " +
		"`refresh: true`, at the top level or under the language",
}

// ErrRemovedConfigKey is returned when a layer the operator wrote carries a key that no
// longer exists.
var ErrRemovedConfigKey = errors.New("removed configuration key")

// warnOperatorOnlyKeys reports the operator-only keys a restricted layer tried to set.
func warnOperatorOnlyKeys(layer ConfigLayer) {
	for _, key := range topLevelKeys(layer.Data) {
		if reason, rejected := operatorOnlyKeys[key]; rejected {
			logger.Warnf(
				"Ignoring %q from the %s: %s", key, layer.describe(), reason,
			)
		}
	}
}

// auditRemovedKeys answers a layer that carries a key AutoBump has dropped. A layer the
// operator wrote gets an error, because a strict decode would fail on it anyway and a
// useless message is worse than a useful one; any other layer gets a warning and keeps
// going, which is what a repository's own file has always got.
func auditRemovedKeys(layer ConfigLayer) error {
	for _, key := range removedKeysIn(layer.Data) {
		migration := removedKeys[key]
		if layer.Scope == ScopeOperator {
			return fmt.Errorf("%w in the %s: %s", ErrRemovedConfigKey, layer.describe(), migration)
		}
		logger.Warnf("Ignoring %q in the %s: %s", key, layer.describe(), migration)
	}

	return nil
}

// removedKeysIn finds every removed key in the document, at the top level and under each
// language. `refresh_commands` was only ever a language field, so a scan of the top level
// alone would miss every real occurrence.
func removedKeysIn(data []byte) []string {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		// A document that does not parse is the decoder's problem to report, with its
		// line number. There is nothing useful to say about its keys.
		return nil
	}

	found := make([]string, 0)
	seen := make(map[string]struct{})

	collect := func(node *yaml.Node) {
		for i := 0; i+1 < len(node.Content); i += mappingStride {
			key := node.Content[i].Value
			if _, removed := removedKeys[key]; !removed {
				continue
			}
			if _, already := seen[key]; already {
				continue
			}
			seen[key] = struct{}{}
			found = append(found, key)
		}
	}

	root := documentMapping(&document)
	if root == nil {
		return nil
	}
	collect(root)

	for i := 0; i+1 < len(root.Content); i += mappingStride {
		if root.Content[i].Value != "languages" {
			continue
		}
		languages := root.Content[i+1]
		if languages.Kind != yaml.MappingNode {
			continue
		}
		for j := 1; j < len(languages.Content); j += mappingStride {
			if languages.Content[j].Kind == yaml.MappingNode {
				collect(languages.Content[j])
			}
		}
	}

	return found
}

// topLevelKeys returns the document's top-level mapping keys, or nothing when the
// document is empty or is not a mapping.
func topLevelKeys(data []byte) []string {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil
	}

	root := documentMapping(&document)
	if root == nil {
		return nil
	}

	keys := make([]string, 0, len(root.Content)/mappingStride)
	for i := 0; i < len(root.Content); i += mappingStride {
		keys = append(keys, root.Content[i].Value)
	}

	return keys
}

// mappingStride is how far apart a YAML mapping's keys are in Node.Content, which
// interleaves keys and values.
const mappingStride = 2

// documentMapping unwraps a decoded document down to its root mapping, or nil when there
// is not one -- an empty file, or a document whose root is a scalar or a sequence.
func documentMapping(document *yaml.Node) *yaml.Node {
	node := document
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return nil
		}
		node = node.Content[0]
	}
	if node.Kind != yaml.MappingNode {
		return nil
	}

	return node
}

// FinalizeGlobalConfig applies the resolution steps that belong to the finished
// configuration rather than to any one layer: a project name derived from its path, a
// token read out of the file it names, "~" expanded, provider tokens resolved, and the
// environment consulted where a value is still empty.
func FinalizeGlobalConfig(config *GlobalConfig) {
	for i := range config.Projects {
		if config.Projects[i].Name == "" {
			basename := path.Base(config.Projects[i].Path)
			config.Projects[i].Name = strings.TrimSuffix(basename, ".git")
		}
	}

	handleTokenFile("GPG passphrase", &config.GpgKeyPassphrase)
	handleTokenFile("SSH key passphrase", &config.SSHKeyPassphrase)
	handleTokenFile("GitLab access token", &config.GitLabAccessToken)
	handleTokenFile("Azure DevOps access token", &config.AzureDevOpsAccessToken)
	handleTokenFile("GitHub access token", &config.GitHubAccessToken)

	expandHome(&config.SSHKeyPath)
	expandHome(&config.SSHAuthSock)

	for i := range config.Providers {
		config.Providers[i].Token = config.Providers[i].ResolveToken()
	}

	config.GitLabCIJobToken = os.Getenv("CI_JOB_TOKEN")

	if config.GpgKeyPassphrase == "" {
		config.GpgKeyPassphrase = os.Getenv("GPG_PASSPHRASE")
	}
}
