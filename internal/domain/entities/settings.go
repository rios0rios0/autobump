package entities

import (
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	logger "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

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
	Extensions      []string         `yaml:"extensions"`
	SpecialPatterns []string         `yaml:"special_patterns"`
	VersionFiles    []VersionFile    `yaml:"version_files"`
	RefreshCommands []RefreshCommand `yaml:"refresh_commands"`
}

// VersionFile describes a file that contains version information.
type VersionFile struct {
	Path     string   `yaml:"path"`
	Patterns []string `yaml:"patterns"`
}

// RefreshCommand regenerates the files that derive from a version file AutoBump has
// just rewritten, so they travel in the bump commit instead of drifting until a
// pipeline rejects the release.
//
// A lockfile is the motivating case: bumping the range one workspace package declares
// on its sibling invalidates the resolution descriptor recorded in `yarn.lock`, and a
// CI job running `yarn install --immutable` then refuses the install the bump PR was
// opened to validate. AutoBump cannot know that relationship — only the package
// manager does — so it runs the command that does and stages what the command wrote.
type RefreshCommand struct {
	// Run is the command and its arguments, executed directly rather than through a
	// shell so that quoting and interpolation cannot change what runs.
	Run []string `yaml:"run"`

	// Files are glob patterns, relative to the project root, naming what the command
	// regenerates. Only these are staged: a refresh must not sweep unrelated work
	// into the release commit, which is a real risk in `local` mode where the
	// operator's own uncommitted changes sit in the same worktree.
	Files []string `yaml:"files"`
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
	ErrLanguagesKeyMissingError = errors.New("missing languages key")
	ErrConfigFileNotFoundError  = errors.New("config file not found")
	ErrConfigKeyMissingError    = errors.New("config keys missing")
)

// ReadConfig reads the config file and returns a GlobalConfig struct.
func ReadConfig(configPath string) (*GlobalConfig, error) {
	data, err := readData(configPath)
	if err != nil {
		return nil, err
	}

	globalConfig, err := DecodeConfig(data, true)
	if err != nil {
		return nil, err
	}

	for i := range globalConfig.Projects {
		if globalConfig.Projects[i].Name == "" {
			basename := path.Base(globalConfig.Projects[i].Path)
			basename = strings.TrimSuffix(basename, ".git")
			globalConfig.Projects[i].Name = basename
		}
	}

	handleTokenFile("GPG passphrase", &globalConfig.GpgKeyPassphrase)
	handleTokenFile("SSH key passphrase", &globalConfig.SSHKeyPassphrase)
	handleTokenFile("GitLab access token", &globalConfig.GitLabAccessToken)
	handleTokenFile("Azure DevOps access token", &globalConfig.AzureDevOpsAccessToken)
	handleTokenFile("GitHub access token", &globalConfig.GitHubAccessToken)

	expandHome(&globalConfig.SSHKeyPath)
	expandHome(&globalConfig.SSHAuthSock)

	// Resolve provider tokens (env vars and file paths)
	for i := range globalConfig.Providers {
		globalConfig.Providers[i].Token = globalConfig.Providers[i].ResolveToken()
	}

	globalConfig.GitLabCIJobToken = os.Getenv("CI_JOB_TOKEN")

	if globalConfig.GpgKeyPassphrase == "" {
		globalConfig.GpgKeyPassphrase = os.Getenv("GPG_PASSPHRASE")
	}

	return globalConfig, nil
}

// readData reads data from a file or a URL.
func readData(configPath string) ([]byte, error) {
	uri, err := url.Parse(configPath)
	if err != nil || uri.Scheme == "" || uri.Host == "" {
		// It's not a URL, read the data from file
		var data []byte
		data, err = os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		return data, nil
	}
	// It's a URL, so read the data from the URL
	return downloadHelpers.DownloadFile(configPath)
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

// DecodeConfig decodes the config file and returns a GlobalConfig struct
// If strict is true, unknown fields will cause an error (for user config)
// If strict is false, unknown fields will be ignored (for default config).
func DecodeConfig(data []byte, strict bool) (*GlobalConfig, error) {
	var globalConfig GlobalConfig

	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(strict)
	err := decoder.Decode(&globalConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	return &globalConfig, nil
}

// ValidateGlobalConfig validates the global config and reports missing keys and errors.
func ValidateGlobalConfig(globalConfig *GlobalConfig, batch bool) error {
	var missingKeys []string

	if batch && len(globalConfig.Projects) == 0 {
		missingKeys = append(missingKeys, "projects")
	}

	for projectIndex, projectConfig := range globalConfig.Projects {
		if projectConfig.Path == "" {
			missingKeys = append(missingKeys, fmt.Sprintf("projects[%d].path", projectIndex))
		}
		if batch && globalConfig.GitLabAccessToken == "" &&
			globalConfig.AzureDevOpsAccessToken == "" &&
			globalConfig.GitHubAccessToken == "" &&
			projectConfig.ProjectAccessToken == "" {
			logger.Error(
				"Project access token is required when personal access token " +
					"is not set in batch mode",
			)
			missingKeys = append(
				missingKeys,
				fmt.Sprintf("projects[%d].project_access_token", projectIndex),
			)
		}
	}

	if len(missingKeys) > 0 {
		return fmt.Errorf("%w: %s", ErrConfigKeyMissingError, strings.Join(missingKeys, ", "))
	}

	if globalConfig.LanguagesConfig == nil {
		return ErrLanguagesKeyMissingError
	}

	return nil
}

// MergeLanguagesConfig deep-merges user language overrides into defaults.
// Version files with the same path are replaced; new paths are appended.
// Extensions and special patterns from defaults are preserved when the user
// provides only version files. New languages are added wholesale.
//
// Refresh commands are the one field that replaces rather than merges. They name a
// package manager, and appending one to another would run both: an `npm` default
// left in place under a `yarn` override would write a `package-lock.json` into a
// repository that has no business carrying one.
//
// Replacement is keyed on the field being *present*, not on it being non-empty, so
// an explicit `refresh_commands: []` clears a globally configured command instead of
// reading as an omission. Without that distinction a project could never opt out of
// a refresh its language configures for everyone.
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
		if override.RefreshCommands != nil {
			base.RefreshCommands = slices.Clone(override.RefreshCommands)
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

// ReadProjectConfig reads a per-project config file and returns a GlobalConfig.
// Only the LanguagesConfig field is meaningful; other fields are ignored by the caller.
// The file is decoded in non-strict mode to allow partial config files gracefully.
func ReadProjectConfig(configPath string) (*GlobalConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read project config file %s: %w", configPath, err)
	}
	return DecodeConfig(data, false)
}

// CopyGlobalConfigWithLanguageOverrides creates a shallow copy of the given GlobalConfig
// and replaces its LanguagesConfig with the result of merging the original languages
// with the provided overrides. The original config is not mutated.
//
// The overrides here come from a `.autobump.yaml` inside the repository being released,
// which in `run` mode is a repository AutoBump discovered rather than one the operator
// wrote. They are therefore untrusted, and passed through SanitizeUntrustedLanguages
// before the merge.
func CopyGlobalConfigWithLanguageOverrides(
	original *GlobalConfig,
	languageOverrides map[string]LanguageConfig,
) *GlobalConfig {
	copied := *original
	copied.LanguagesConfig = MergeLanguagesConfig(
		original.LanguagesConfig, SanitizeUntrustedLanguages(languageOverrides),
	)
	return &copied
}

// SanitizeUntrustedLanguages strips what a repository-owned config file is not allowed
// to say. It returns a copy; the input is not mutated.
//
// Only refresh commands are stripped, and only when they would *introduce* one. Every
// other language field describes how to find and rewrite a version string, which is
// bounded by what a regular expression can do to a file AutoBump was already going to
// rewrite. A refresh command is different in kind: it is an executable, run with the
// runner's environment and provider credentials, before the pull request is opened.
// Honouring one from a discovered repository would let anything in a scanned
// organisation execute code on the machine doing the release.
//
// Clearing is still allowed, because an empty list only ever removes execution: a
// project that cannot use its language's configured refresh must be able to say so.
// That is why the check is on the contents rather than on presence.
func SanitizeUntrustedLanguages(overrides map[string]LanguageConfig) map[string]LanguageConfig {
	if len(overrides) == 0 {
		return overrides
	}

	sanitized := make(map[string]LanguageConfig, len(overrides))
	for lang, override := range overrides {
		if len(override.RefreshCommands) > 0 {
			logger.Warnf(
				"Ignoring %d refresh command(s) declared for language %q by the project's own "+
					"config: refresh commands are executables and are only honoured from the "+
					"global configuration",
				len(override.RefreshCommands), lang,
			)
			override.RefreshCommands = nil
		}
		sanitized[lang] = override
	}

	return sanitized
}

// FindConfigOnMissing finds the config file if not manually set.
func FindConfigOnMissing(configPath string) string {
	if configPath == "" {
		logger.Info("No config file specified, searching for default locations")

		var err error
		configPath, err = configHelpers.FindConfigFile("autobump")
		if err != nil {
			logger.Warn(
				"Config file not found in default locations, " +
					"using the repository configuration as the last resort",
			)
			configPath = DefaultConfigURL
		}

		logger.Infof("Using config file: \"%v\"", configPath)
		return configPath
	}
	return configPath
}
