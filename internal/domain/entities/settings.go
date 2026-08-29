package entities

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	logger "github.com/sirupsen/logrus"

	configEntities "github.com/rios0rios0/gitforge/pkg/config/domain/entities"
	configHelpers "github.com/rios0rios0/gitforge/pkg/config/domain/helpers"
	downloadHelpers "github.com/rios0rios0/gitforge/pkg/config/infrastructure/helpers"
)

// Versioning modes supported by the bumper.
//
// VersioningSemver follows Semantic Versioning (default).
// VersioningForkDot increments the trailing fork digit in X.Y.Z.N versions.
// VersioningForkDash increments the trailing fork digit in X.Y.Z-N versions.
// See the "Forking Technique" guide for the rationale behind the fork modes.
const (
	VersioningSemver   = "semver"
	VersioningForkDot  = "fork-dot"
	VersioningForkDash = "fork-dash"
)

// DefaultBumpBranchPrefix is the prefix of the branches AutoBump creates for a release.
// The next version is appended to it (e.g. "chore/bump-1.2.3"), and stale-branch cleanup
// matches the same prefix.
const DefaultBumpBranchPrefix = "chore/bump-"

// GlobalConfig represents the top-level configuration.
type GlobalConfig struct {
	Providers              []configEntities.ProviderConfig `yaml:"providers"`
	Projects               []ProjectConfig                 `yaml:"projects"`
	LanguagesConfig        map[string]LanguageConfig       `yaml:"languages"`
	ExcludeForks           bool                            `yaml:"exclude_forks"`
	ExcludeArchived        bool                            `yaml:"exclude_archived"`
	CleanupStaleBranches   *bool                           `yaml:"cleanup_stale_branches"`
	DetectChlog            *bool                           `yaml:"detect_chlog"`
	Refresh                *bool                           `yaml:"refresh"`
	BumpBranchPrefix       string                          `yaml:"bump_branch_prefix"`
	ChangelogPath          string                          `yaml:"changelog_path"`
	Versioning             string                          `yaml:"versioning"`
	GpgKeyPath             string                          `yaml:"gpg_key_path"`
	GpgKeyPassphrase       string                          `yaml:"gpg_key_passphrase"`
	SSHKeyPath             string                          `yaml:"ssh_key_path"`
	SSHKeyPassphrase       string                          `yaml:"ssh_key_passphrase"`
	SSHAuthSock            string                          `yaml:"ssh_auth_sock"`
	GitLabAccessToken      string                          `yaml:"gitlab_access_token"`
	AzureDevOpsAccessToken string                          `yaml:"azure_devops_access_token"`
	GitHubAccessToken      string                          `yaml:"github_access_token"`
	GitLabCIJobToken       string                          `yaml:"gitlab_ci_job_token"`
}

// ProviderConfig is re-exported from gitforge for backward compatibility.
type ProviderConfig = configEntities.ProviderConfig

// LanguageConfig holds per-language detection and versioning rules.
type LanguageConfig struct {
	Extensions      []string      `yaml:"extensions"`
	SpecialPatterns []string      `yaml:"special_patterns"`
	VersionFiles    []VersionFile `yaml:"version_files"`

	// Refresh turns the post-bump refresh on for this language. Nil inherits the
	// top-level setting, which in turn defaults to off.
	Refresh *bool `yaml:"refresh"`
}

// VersionFile describes a file that contains version information.
type VersionFile struct {
	Path     string   `yaml:"path"`
	Patterns []string `yaml:"patterns"`
}

// ProjectConfig holds per-project configuration.
type ProjectConfig struct {
	Path               string `yaml:"path"`
	Name               string `yaml:"name"`
	Language           string `yaml:"language"`
	ProjectAccessToken string `yaml:"project_access_token"`
	NewVersion         string `yaml:"new_version"`
	ChangelogPath      string `yaml:"changelog_path"`
	Versioning         string `yaml:"versioning"`
	DetectChlog        *bool  `yaml:"detect_chlog"`
	Refresh            *bool  `yaml:"refresh"`
}

// ResolveVersioning returns the effective versioning mode for a project.
// Project-level setting wins over global, which wins over the semver default.
// Unknown modes are normalized to the semver default to keep the bumper safe.
func ResolveVersioning(globalConfig *GlobalConfig, projectConfig *ProjectConfig) string {
	mode := ""
	if projectConfig != nil {
		mode = strings.TrimSpace(projectConfig.Versioning)
	}
	if mode == "" && globalConfig != nil {
		mode = strings.TrimSpace(globalConfig.Versioning)
	}
	switch mode {
	case VersioningForkDot, VersioningForkDash:
		return mode
	case "", VersioningSemver:
		return VersioningSemver
	default:
		logger.Warnf("Unknown versioning mode %q, falling back to %q", mode, VersioningSemver)
		return VersioningSemver
	}
}

// ChlogEnabled reports whether a project should be inspected for a chlog
// (https://github.com/luizjhonata/chlog) fragment layout. Detection is opt-out, so an
// absent setting means enabled; only an explicit "detect_chlog: false" turns it off.
// Project-level setting wins over global, matching ResolveVersioning.
//
// Leaving it on costs nothing for a repository that does not use chlog: without the
// fragment directory, detection is a single stat call and changes no behaviour.
func ChlogEnabled(globalConfig *GlobalConfig, projectConfig *ProjectConfig) bool {
	if projectConfig != nil && projectConfig.DetectChlog != nil {
		return *projectConfig.DetectChlog
	}
	if globalConfig != nil && globalConfig.DetectChlog != nil {
		return *globalConfig.DetectChlog
	}
	return true
}

// CleanupEnabled reports whether stale bump-branch cleanup should run.
// Cleanup is opt-out, so an absent setting means enabled; only an explicit
// "cleanup_stale_branches: false" (or the --skip-cleanup flag) turns it off.
func CleanupEnabled(globalConfig *GlobalConfig) bool {
	if globalConfig == nil || globalConfig.CleanupStaleBranches == nil {
		return true
	}
	return *globalConfig.CleanupStaleBranches
}

// ResolveBumpBranchPrefix returns the configured bump-branch prefix, falling back to
// DefaultBumpBranchPrefix. The same prefix drives both branch creation and cleanup, so
// a custom prefix can never leave cleanup matching branches the bumper no longer makes.
func ResolveBumpBranchPrefix(globalConfig *GlobalConfig) string {
	if globalConfig != nil {
		if prefix := strings.TrimSpace(globalConfig.BumpBranchPrefix); prefix != "" {
			return prefix
		}
	}
	return DefaultBumpBranchPrefix
}

// DefaultConfigURL is the URL of the default configuration file.
const DefaultConfigURL = "https://raw.githubusercontent.com/rios0rios0/autobump/" +
	"main/configs/autobump.yaml"

var (
	ErrConfigKeyMissingError   = errors.New("config keys missing")
	ErrBumpBranchPrefixInvalid = errors.New("invalid bump branch prefix")
)

// ReadLayerData reads a configuration layer from a file path or an HTTP(S) URL.
func ReadLayerData(configPath string) ([]byte, error) {
	data, err := downloadHelpers.ReadData(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", configPath, err)
	}
	return data, nil
}

// expandHome replaces a leading "~/" with the user's home directory.
func expandHome(value *string) {
	if strings.HasPrefix(*value, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			*value = filepath.Join(home, (*value)[2:])
		}
	}
}

// handleTokenFile reads the token from a file if it exists and replaces the token string.
func handleTokenFile(name string, token *string) {
	if *token != "" {
		if _, err := os.Stat(*token); !os.IsNotExist(err) {
			logger.Infof("Reading %s from file...", name)
			var fileToken []byte
			fileToken, err = os.ReadFile(*token)
			if err != nil {
				logger.Errorf("failed to read %s from file: %v", name, err)
			} else {
				*token = strings.TrimSpace(string(fileToken))
			}
		}
	}
}

// ValidateProviders validates provider configuration entries.
func ValidateProviders(providers []configEntities.ProviderConfig) error {
	for i, p := range providers {
		if p.Type == "" {
			return fmt.Errorf(
				"%w: providers[%d].type is required",
				ErrConfigKeyMissingError, i,
			)
		}
		if p.Token == "" {
			return fmt.Errorf(
				"%w: providers[%d].token is required (set inline, via ${ENV_VAR}, or as file path)",
				ErrConfigKeyMissingError, i,
			)
		}
		if len(p.Organizations) == 0 {
			return fmt.Errorf(
				"%w: providers[%d].organizations must have at least one entry",
				ErrConfigKeyMissingError, i,
			)
		}
	}
	return nil
}

// ValidateGlobalConfig checks the finished configuration -- the result of folding every
// layer -- and reports what is missing or unusable.
//
// The `batch` parameter it used to take was always false from both call sites, so the
// branches guarded by it were unreachable; the checks that survive are the ones that
// applied either way. Languages are no longer among them: the built-in defaults are the
// base of every run, so LanguagesConfig cannot be empty by the time this is reached.
func ValidateGlobalConfig(globalConfig *GlobalConfig) error {
	var missingKeys []string

	for projectIndex, projectConfig := range globalConfig.Projects {
		if projectConfig.Path == "" {
			missingKeys = append(missingKeys, fmt.Sprintf("projects[%d].path", projectIndex))
		}
	}

	if len(missingKeys) > 0 {
		return fmt.Errorf("%w: %s", ErrConfigKeyMissingError, strings.Join(missingKeys, ", "))
	}

	return ValidateBumpBranchPrefix(globalConfig.BumpBranchPrefix)
}

// protectedBranchNames are the branches a bump prefix must not be able to reach. A prefix
// matches by string prefix, so "main" would match "main" itself and every branch under a
// "main..." name with it.
//
//nolint:gochecknoglobals // read-only lookup table
var protectedBranchNames = map[string]struct{}{
	"main": {}, "master": {}, "develop": {}, "head": {}, "trunk": {},
}

// invalidPrefixRunes are the characters git refuses in a ref name. A prefix that cannot
// name a branch can only misbehave: it will never match one AutoBump created, and the
// operator will believe cleanup is running when it is matching nothing.
const invalidPrefixRunes = " \t~^:?*[\\"

// ValidateBumpBranchPrefix checks a configured bump-branch prefix before anything uses it.
//
// The prefix is not only what new branches are named after -- it is the argument to a
// destructive operation. cleanupStaleBumpBranches deletes every remote branch that starts
// with it and closes the pull request attached to each, so a prefix that is wider than the
// operator meant does not produce a confusing branch name, it deletes other people's work.
// An operator's typo is as capable of that as a hostile repository would be, which is why
// this runs over the operator's own file too.
//
// A failure here is an error rather than a fallback to the default. Quietly substituting
// the default would mean cleanup ran against branches the operator never named.
func ValidateBumpBranchPrefix(prefix string) error {
	if prefix == "" {
		// Unset is not invalid -- it means DefaultBumpBranchPrefix, which is valid by
		// construction and covered by this function's tests.
		return nil
	}

	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		return fmt.Errorf(
			"%w: an empty prefix matches every branch in the repository", ErrBumpBranchPrefixInvalid,
		)
	}
	if trimmed != prefix {
		return fmt.Errorf(
			"%w %q: leading or trailing whitespace cannot be part of a branch name",
			ErrBumpBranchPrefixInvalid, prefix,
		)
	}

	if err := validatePrefixShape(prefix); err != nil {
		return err
	}

	if _, protected := protectedBranchNames[strings.ToLower(prefix)]; protected {
		return fmt.Errorf(
			"%w %q: that is a protected branch name, and cleanup deletes what the prefix matches",
			ErrBumpBranchPrefixInvalid, prefix,
		)
	}

	return nil
}

// validatePrefixShape enforces what the prefix has to look like: a name git accepts, and
// a namespace it cannot escape from.
func validatePrefixShape(prefix string) error {
	if strings.ContainsAny(prefix, invalidPrefixRunes) ||
		strings.Contains(prefix, "..") || strings.Contains(prefix, "//") ||
		strings.HasPrefix(prefix, "-") || strings.HasPrefix(prefix, "/") ||
		strings.HasSuffix(prefix, ".lock") || prefix == "@" {
		return fmt.Errorf(
			"%w %q: it is not a name git will accept for a branch",
			ErrBumpBranchPrefixInvalid, prefix,
		)
	}

	for _, r := range prefix {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf(
				"%w: control characters cannot be part of a branch name", ErrBumpBranchPrefixInvalid,
			)
		}
	}

	if strings.HasPrefix(prefix, "refs/") {
		return fmt.Errorf(
			"%w %q: cleanup matches short branch names, so a refs/ prefix silently matches "+
				"nothing and leaves cleanup looking enabled while it does nothing; use %q",
			ErrBumpBranchPrefixInvalid, prefix, DefaultBumpBranchPrefix,
		)
	}

	slash := strings.LastIndex(prefix, "/")
	if slash < 0 {
		return fmt.Errorf(
			"%w %q: it must contain a %q so the match cannot escape into the repository's "+
				"ordinary branch names; the default is %q",
			ErrBumpBranchPrefixInvalid, prefix, "/", DefaultBumpBranchPrefix,
		)
	}
	if slash == len(prefix)-1 {
		return fmt.Errorf(
			"%w %q: a bare namespace matches everything under it -- %q would match every "+
				"other tool's branches in the same namespace, and cleanup deletes what it "+
				"matches. Name the branches too, as %q does",
			ErrBumpBranchPrefixInvalid, prefix, prefix, DefaultBumpBranchPrefix,
		)
	}

	return nil
}

// MergeLanguagesConfig deep-merges one layer's language overrides onto the accumulated
// ones. Version files with the same path are replaced and new paths are appended;
// extensions and special patterns are appended and de-duplicated, so a layer that names
// only version files keeps the ones it inherited. New languages are added wholesale.
//
// `refresh` is a pointer rather than a bool for the same reason every other opt-out in
// this file is: a layer has to be able to turn an inherited refresh *off*, and a plain
// false is indistinguishable from a key nobody wrote.
func MergeLanguagesConfig(
	defaults, overrides map[string]LanguageConfig,
) map[string]LanguageConfig {
	result := make(map[string]LanguageConfig, len(defaults))
	maps.Copy(result, defaults)

	for lang, override := range overrides {
		base, exists := result[lang]
		if !exists {
			result[lang] = override
			continue
		}

		if len(override.Extensions) > 0 {
			base.Extensions = dedup(append(slices.Clone(base.Extensions), override.Extensions...))
		}
		if len(override.SpecialPatterns) > 0 {
			base.SpecialPatterns = dedup(append(slices.Clone(base.SpecialPatterns), override.SpecialPatterns...))
		}
		if len(override.VersionFiles) > 0 {
			base.VersionFiles = mergeVersionFiles(base.VersionFiles, override.VersionFiles)
		}
		if override.Refresh != nil {
			base.Refresh = override.Refresh
		}

		result[lang] = base
	}

	return result
}

// mergeVersionFiles merges override version files into base.
// Files with a matching path replace the default; others are appended.
func mergeVersionFiles(base, overrides []VersionFile) []VersionFile {
	merged := slices.Clone(base)

	for _, ov := range overrides {
		found := false
		for i, bv := range merged {
			if bv.Path == ov.Path {
				merged[i] = ov
				found = true
				break
			}
		}
		if !found {
			merged = append(merged, ov)
		}
	}
	return merged
}

// dedup removes duplicate strings while preserving order.
func dedup(s []string) []string {
	seen := make(map[string]struct{}, len(s))
	out := make([]string, 0, len(s))
	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

// FindProjectConfigFile searches for a per-project autobump config file in the given directory.
// It checks for .autobump.yaml, .autobump.yml, autobump.yaml, autobump.yml in priority order.
// Returns the path to the first file found, or empty string if none is found.
func FindProjectConfigFile(projectDir string) string {
	patterns := []string{".autobump.yaml", ".autobump.yml", "autobump.yaml", "autobump.yml"}
	for _, pat := range patterns {
		p := filepath.Join(projectDir, pat)
		fi, err := os.Stat(p)
		if err == nil && fi.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

// CopyGlobalConfigWithLanguageOverrides creates a shallow copy of the given GlobalConfig
// and replaces its LanguagesConfig with the result of merging the original languages
// with the provided overrides. The original config is not mutated.
func CopyGlobalConfigWithLanguageOverrides(
	original *GlobalConfig,
	languageOverrides map[string]LanguageConfig,
) *GlobalConfig {
	copied := *original
	copied.LanguagesConfig = MergeLanguagesConfig(original.LanguagesConfig, languageOverrides)
	return &copied
}

// RefreshEnabled reports whether the files derived from a version file should be
// regenerated for this project -- lockfiles above all.
//
// Unlike chlog detection and stale-branch cleanup, the refresh is opt-**in**. It starts a
// package manager, and a tool whose job is rewriting text files should not begin
// executing programs because somebody upgraded it.
//
// The project entry wins over the language, which wins over the top-level default.
// Layering has already folded a repository's own `refresh:` into the top level and its
// `languages.<lang>.refresh` into the language, so those three are the whole ladder.
func RefreshEnabled(
	globalConfig *GlobalConfig, projectConfig *ProjectConfig, language string,
) bool {
	if projectConfig != nil && projectConfig.Refresh != nil {
		return *projectConfig.Refresh
	}

	if globalConfig != nil && language != "" {
		if languageConfig, exists := globalConfig.LanguagesConfig[language]; exists &&
			languageConfig.Refresh != nil {
			return *languageConfig.Refresh
		}
	}

	if globalConfig != nil && globalConfig.Refresh != nil {
		return *globalConfig.Refresh
	}

	return false
}

// FindOperatorConfig locates the operator's own configuration file: the one named with
// -c, or the one in their home directory.
//
// It returns "" when there is none, and that is not an error. AutoBump's built-in
// defaults are the base of every run, so an operator who keeps no configuration is not
// missing anything the tool needs to work -- only the credentials and the project list,
// which the modes that need them validate for themselves.
//
// The working directory is deliberately not searched. AutoBump normally runs with the
// repository it is releasing as the working directory, and that repository may carry its
// own `.autobump.yaml`. Answering the operator-configuration question with that file does
// not reorder a preference, it substitutes a project's overrides for the operator's: the
// project's settings stop being overrides and replace the layers beneath them, and the
// file is decoded strictly although a project file is allowed to be partial. The
// project's file is still read -- as the last layer, through the schema that is scoped
// for it.
func FindOperatorConfig(configPath string) string {
	if configPath != "" {
		return configPath
	}

	logger.Debug("No config file specified, searching the operator's home directory")

	configPath, err := configHelpers.FindGlobalConfigFile("autobump")
	if err != nil {
		logger.Infof(
			"No configuration found in the home directory (%v); running on AutoBump's "+
				"built-in defaults. Name your own with -c if you keep it elsewhere",
			err,
		)
		return ""
	}

	logger.Infof("Using config file: %q", configPath)

	return configPath
}
