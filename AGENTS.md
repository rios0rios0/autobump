# AGENTS.md

This file provides guidance to AI coding agents (Claude Code, Codex, Cursor, GitHub Copilot) when working with code in this repository.

## What This Project Does

AutoBump is a Go CLI tool that automates the release process: it reads `CHANGELOG.md`, calculates the next semantic version, updates the language's version files, commits, pushes, and creates PRs/MRs on GitHub, GitLab, or Azure DevOps. `configs/autobump.yaml` is the source of truth for which files each language bumps. Go and Terraform release through git tags; a Go API documented with Swagger/OpenAPI also gets its `@version` annotation and generated `docs.go`/`swagger.*`/`openapi.*` bumped when present. Version files are content-filtered: a globbed file only counts when one of its regex patterns matches its content.

Version files that need more than regex substitution are entries in `versionFileHooks` (`internal/domain/commands/version_file_hooks.go`), keyed by base name: `pom.xml` masks its `<parent>` block so the first `<version>` match is the project's own, and `pubspec.yaml` goes through `langforge`'s `dart.BumpBuildNumber` so the Flutter build number increases (`1.2.3+7` → `1.3.0+8`). Add the next exception as a map entry, not as a branch in `applyVersionPatterns`.

## Build & Development Commands

```bash
make build          # Compile to bin/autobump (stripped)
make debug          # Compile with debug symbols
make run            # go run ./cmd/autobump
make install        # Build and copy to ~/.local/bin/autobump
make lint           # golangci-lint (always use this, never call golangci-lint directly)
make test           # Full test suite via pipeline scripts
make sast           # Security analysis (CodeQL, Semgrep, Trivy, Hadolint, Gitleaks)
```

Run a single test during development:
```bash
go test -run "TestDetectProjectLanguage" ./internal/domain/commands/
```

`lint`, `test` and `sast` come from the shared [rios0rios0/pipelines](https://github.com/rios0rios0/pipelines)
makefiles, which `Makefile` includes from `$(SCRIPTS_DIR)` (default
`~/Development/github.com/rios0rios0/pipelines`); without that checkout those targets do not exist.

Unit tests carry **no build tag**, so plain `go test ./...` runs them; the shared pipeline's
phase 1 (`-tags test,unit`) still runs every one, and phase 2 (`-tags integration`) selects
only packages where the tag makes a test file appear. Do not add `//go:build unit`. Reserve
`//go:build integration` for tests that need real infrastructure; this repository has none today.

A test that cannot run in parallel says so in a comment above it — "is deliberately not
parallel: it mutates package-level globals", or `t.Setenv`, which the runtime forbids in a
parallel test. Do not add `t.Parallel()` to one of those without reading why it is absent.

## Architecture

Clean Architecture with Hexagonal (Ports & Adapters) design using `go.uber.org/dig` for dependency injection.

### Layer Structure

- **`cmd/autobump/`** — Entry point (`main.go`) and DI wiring (`dig.go`)
- **`internal/domain/`** — Business logic and contracts (no framework dependencies)
  - `commands/` — Core use cases in `service.go` (`ProcessRepo`, `IterateProjects`, `DiscoverAndProcess`, `DetectProjectLanguage`) plus `VersionCommand` and `SelfUpdateCommand`
  - `entities/` — Domain types: `GlobalConfig`, `ProjectConfig`, `LanguageConfig`, `Controller` interface, plus `config_layers.go` (the layering engine)
- **`internal/infrastructure/`** — Implementations
  - `controllers/` — Cobra CLI handlers: `RootController` (single-repo, backs `autobump .`), `RunController` (batch + discover engine)
  - `repositories/` — Git provider adapters wrapping `gitforge`'s `ProviderRegistry`
- **`internal/support/`** — Shared utilities (file I/O, HTTP, URL helpers)
- **`test/domain/entitybuilders/`** — testkit-based Builder pattern for test data

### Dependency Flow

```
main.go → dig.go → container.go → registers all providers into dig container
                                    ├── repositories (gitforge adapters: GitHub, GitLab, AzureDevOps)
                                    ├── controllers (Cobra commands)
                                    └── AppInternal (aggregates controllers)
```

Dependencies always point inward: infrastructure → domain, never the reverse.

### Key External Libraries

- **`gitforge`** (`github.com/rios0rios0/gitforge/v4`) — Git provider adapters, changelog processing, PR creation
- **`langforge`** (`github.com/rios0rios0/langforge`) — Language detection via marker files
- **`cliforge`** (`github.com/rios0rios0/cliforge`) — CLI framework: self-update, version commands, startup update checks
- **`testkit`** (`github.com/rios0rios0/testkit`) — Test builder base classes

### CLI Modes

| Command                          | Controller        | Use Case                                                    |
|----------------------------------|-------------------|-------------------------------------------------------------|
| `autobump .` or `autobump <path>` | `RootController`  | Process a single repository (the `local` subcommand is a hidden deprecated alias) |
| `autobump run`                   | `RunController`        | Process repos from config (auto-detects batch vs discover) |
| `autobump version`               | `VersionController`    | Print the build-time version                               |
| `autobump self-update`           | `SelfUpdateController` | Download and install the latest release from GitHub         |

The `run` command auto-detects the mode: if the config has a `providers` section, it discovers repos via APIs; if it has a `projects` section, it iterates the static list. Both can run together.

### Language Detection Strategy

Three-stage fallback in `DetectProjectLanguage`:
1. **langforge registry** — Marker files (go.mod, package.json, etc.)
2. **Config special patterns** — Custom glob patterns from `autobump.yaml`
3. **File extensions** — langforge classifier-based fallback

## Testing Conventions

- BDD structure with `// given`, `// when`, `// then` comment blocks
- Test names: `t.Run("should ... when ...", ...)` format
- Unit tests use `t.Parallel()` at both the parent and subtest level, and carry no build tag. The exceptions are documented in a comment on the test function
- Assertions via `testify/assert` and `testify/require`
- Test data via Builder pattern in `test/domain/entitybuilders/`
- White-box testing via `export_test.go` files that expose unexported functions

## Configuration

Configuration is **layered**; each layer overrides only the keys its document declares:
built-in defaults (`configs/autobump.yaml`, embedded via `go:embed` in `configs/embed.go`) ->
published defaults (the same file fetched from `main`, best effort) -> the operator's file
(`-c`, else `~/` or `~/.config/`) -> the repository's own `.autobump.yaml`.
`internal/domain/entities/config_layers.go` is the engine; `configLoader` in
`internal/infrastructure/controllers/config_helpers.go` assembles the first three, and
`loadProjectConfigOverrides` (`service.go`) applies the fourth after the clone. `yaml.v3`
decodes into a non-zero struct, so absent-versus-`false` needs no pointer fields; maps decode
into fresh elements, so `LanguagesConfig` is blanked before the decode and re-merged with
`MergeLanguagesConfig`. Apply is atomic, and a comments-only document (`io.EOF`) is a layer
with nothing to say. The embedded defaults and the operator's file decode strictly; the
fetched and project layers do not.

The trust rules below are security boundaries. The reasoning lives in the doc comments of the
functions named; read it before changing one.

- Only the operator's layer is `ScopeOperator`. The others decode through `RestrictedConfig`,
  which has **no field** for `providers`, `projects`, any credential, or `bump_branch_prefix`.
  The schema is the enforcement; never add one. `operatorOnlyKeys` only reports what was ignored.
- `cleanup_stale_branches` may be switched **off** by a restricted layer, never on
  (`acceptSwitchOff`): an enable would arrive after `applySkipCleanupFlag` and override
  `--skip-cleanup`.
- `refresh` is judged by `acceptRefresh`: off is honoured from any layer; on only from the
  repository's own file (`LayerProjectConfig`), never from `LayerPublishedDefaults`.
- The operator's veto is **not** the layer order: the project layer folds after the
  operator's, and `RefreshEnabled` reads `projectConfig.Refresh` first. `resolveRefreshVeto`
  records an explicit operator `refresh: false` (from the document's keys, not the decoded
  pointer) in the unexported `GlobalConfig.refreshVetoed`, and both `acceptRefresh` and
  `applyToProject` refuse a project enable against it. An omitted key is not a veto
  (documented in the README for discovery mode).
- `bump_branch_prefix` is operator-only *and* validated by `ValidateBumpBranchPrefix` (a name
  git accepts, containing a `/`, not ending at one), because cleanup deletes what it matches.
  `filterStaleBumpBranches` selects only the prefix followed by a version `createBumpBranch`
  produces (`bumpBranchVersionRegex`: `X.Y.Z`, `X.Y.Z.N` or `X.Y.Z-N`), never the default
  branch; a prefix match alone is not ownership.

Token values support inline strings, `${ENV_VAR}` expansion, and file paths, resolved once by
`FinalizeGlobalConfig` over the *folded* configuration, never per layer. SSH push auth uses
`ssh_key_path`, `ssh_key_passphrase` and `ssh_auth_sock`; common agent sockets are
auto-detected when not set.

Pushes run on go-git's **pure-Go** SSH client: `collectSSHAuthMethods` can only use a key file
or a Unix socket it can dial, and `core.sshCommand` or an `ssh` wrapper on `PATH` is ignored
(commit signing does shell out to `gpg.ssh.program`). `ssh_diagnostics.go` explains the
failure (`explainSSHPushFailure` → `ErrNoSSHCredential`) only for an SSH remote with no
collected method. Everything that reaches its renderers has already failed to dial, so a
detected socket there is a dead one (`UnusableAgentSockets`); `TestDetectSSHEnvironment` covers
that seam with a real socket. Known-hosts discovery (`ssh_known_hosts.go`: `SSH_KNOWN_HOSTS`,
`~/.ssh`, `/etc/ssh`, and Windows profiles under WSL) is memoized per process, and `main.go`
hands the same callback to gitforge's `SetDefaultSSHHostKeyCallback`.

Versioning modes: `semver` (default), `fork-dot`, `fork-dash`. Fork modes increment only the trailing fork digit (e.g. `3.3.0.16` → `3.3.0.17`) and skip language-specific version-file rewrites and the refresh. See `internal/domain/commands/fork_version.go`.

A repository's own `.autobump.yaml` may set `refresh`, `changelog_path`, `versioning`,
`detect_chlog`, `cleanup_stale_branches`, `exclude_forks`, `exclude_archived`, `languages`,
`skip` and `reason`. `ApplyProjectLayer` folds it onto the configuration *and* onto the
project entry, where it wins over the operator's `projects[]` entry; the entry pass reads what
*that document* declared, not the folded result, so a global default never overwrites the
entry written beside it. `resolveChangelogPath` falls back to `GlobalConfig.ChangelogPath`.

`skip` (with an optional `reason`) is the one key only the project layer may set:
`applyToProject` writes it to the unexported `ProjectConfig.skip`/`skipReason` (read via
`IsSkipped`/`SkipReason`), and `applyTo` warns when the built-in or published defaults carry
it. `ProcessRepo` calls `honourSkipRequest` right after `loadProjectConfigOverrides`, before
`resolveChangelogPath`, `setupChangelog`, stale-branch cleanup, branch, commit, push or PR, so
a skipped repository is left untouched. Every mode goes through `ProcessRepo`.

## Release Pipeline

- **Refresh** (`refresh`, opt-in, top-level or per language) regenerates lockfiles after the
  regex version rewrite: recipes in `refresh_recipes.go`, run by `refresh_commands.go`, output
  staged via `addFilesToWorktree`. **AutoBump owns the argv**: recipes are compile-time
  constants, and configuration only says *whether* to run. Only `typescript`/`javascript`/`node`
  have recipes; `nodeMarkers` is ordered (`packageManager` before lockfiles); Yarn Classic is
  skipped. Keep every resolution-only flag, including `--ignore-pnpmfile` and
  `YARN_IGNORE_PATH=1`, which block repository-supplied JavaScript that `--ignore-scripts` does
  not reach. A missing package manager aborts that repository's release
  (`ErrRefreshManagerMissing`) instead of skipping.
- **Changelog reads** all go through `readChangelogLines` (`service.go`), which splices chlog
  fragments and then runs `entities.NormalizeUnreleasedSection` (`changelog_normalize.go`:
  heading repair, breaking-marker canonicalisation, deduplication, verb reclassification,
  ordering; `[Unreleased]` only, idempotent). Put new changelog rules there so chlog fragments
  and fork mode get them. Rules move *entries* (a bullet plus its continuation lines), not
  lines. Heading repair matters: the renderer buckets by the exact string `### Added`.
- **chlog** (`chlog.go`, opt-out via `entities.ChlogEnabled` / `detect_chlog`): fragments in
  `.changes/unreleased/` render as Keep a Changelog entries merged with `[Unreleased]`, and
  AutoBump's own SemVer rules pick the bump. `consumeChlogFragments` deletes consumed
  fragments and `stageChlogConsumption` stages the removals plus the `.gitkeep` from
  `KeepChlogUnreleasedDirectory`, so the directory (and chlog detection) survives.
  `ChlogFragment.Breaking` goes through `entities.NormalizeBreakingChangeMarker`, never a
  prepend. A pending `.changes/v*.md` aborts with `ErrChlogPendingVersionFiles`. chlog's
  format is re-implemented, not imported (command-only module).
- **Entries are single lines.** `entities.UnwrapChangelogEntries` joins wrapped entries over
  the *whole document*, at the `readChangelogLines` boundary and again right before
  `SortChangelogEntries` in `ProcessChangelog`/`ProcessNewChangelog`, because a continuation
  line breaks the sort's contiguity. Only wrapped prose joins: a nested list item is its own
  entry, a fenced code block is verbatim, and `#` closes an entry only at column 0.
  **Never hand-wrap a `CHANGELOG.md` entry or a chlog fragment body**; write each as one line.
- **Go module major versions** (`go_module_path.go`): for repositories with a `vX.Y.Z` tag
  (`publishesGoModuleVersions`), `prepareGoModulePathChange` runs first in
  `updateChangelogAndVersionFiles` and moves the module path, imports and references to the
  released major, preparing every edit in memory before anything is written. It touches
  tracked files only (`trackedModuleFiles`; symlinks and submodules skipped), rewrites
  references outside Go source only after a `moduleReferenceContexts` anchor
  (`followedByMajorSuffix` prevents `/v4/v5`), and excludes the changelog and chlog directory.
  A path that fell behind is repaired under `Fixed`; that note is merged in
  `updateChangelogFile`, not at `readChangelogLines`, so an AutoBump-written entry never
  triggers a release. Imports are rewritten at their literal's offsets and gofmt-clean files
  re-formatted; files whose imports do not parse are left alone.
- **Workspace** (`workspace.go`): `cloneRepo` clones into `autobump-<digits>` under
  `os.TempDir()` (`repo/`, plus `tooling/` for refresh caches). `ProcessRepo` defers
  `removeWorkspace` *before* checking the clone's error, so every outcome removes it.
  `recordRemoteDefaultBranch` writes `refs/remotes/origin/HEAD` after the clone because
  `resolveDefaultBranch` reads only that reference. `refreshCacheVariables` moves caches only
  (never `YARN_CACHE_FOLDER` or `npm_config_store_dir`); a new recipe adds its cache variables
  there. `CleanupStaleWorkspaces` deletes only names `isWorkspaceName` accepts; never widen it
  to a glob.
- **Missing changelog**: in AutoBump's own clone (`RepoContext.Cloned`),
  `proposeInitialChangelog` creates it on `initialChangelogBranch` (`chore/add-changelog`,
  outside the bump prefix), pushes, opens a PR, and returns to the starting branch; it never
  commits to the starting branch. `autobump .` commits it in place
  (`commitInitialChangelogInPlace`). Only creating the file fails the release; proposing it is
  best-effort.
- **Stale branch cleanup** (`cleanup.go`): `cleanupStaleBumpBranches` runs before
  `createBumpBranch`, only when a bump is needed, and is best-effort and opt-out
  (`entities.CleanupEnabled`, `--skip-cleanup` via `applySkipCleanupFlag`).
  `entities.ResolveBumpBranchPrefix` (default `chore/bump-`) drives both creation and cleanup.
  `removeStaleBumpBranch` deletes the branch *before* closing its PR/MR (Azure DevOps
  abandons), and closes nothing when the deletion is refused; side effects come in through
  `staleBranchActions` so the order is testable without a forge.

<!-- chlog:start -->
## Changelog (chlog) — MANDATORY

If the repository you are working in uses chlog (a `.chlog.yaml` or `.chlog.yml`
config file, or a `.changes/` directory, exists at the project root), the
following is binding and ALWAYS applies: whenever you make ANY change, you MUST
create a changelog fragment as part of the same change — automatically, without
being asked, before committing.

- Do NOT edit CHANGELOG.md directly; it is generated from fragments.
- Create the fragment with:
  `chlog new --kind <Kind> --body '<past-tense description>'`
- Write an apostrophe inside the single-quoted body as `'\''`.
- Valid kinds: Added, Changed, Deprecated, Removed, Fixed, Security
- Choose the kind that best matches the change (e.g., new feature → Added,
  bug fix → Fixed, behavior change → Changed, removal → Removed, security fix → Security).
- If the change is backward-INCOMPATIBLE with the public API (a breaking
  change), you MUST add the `--breaking` flag:
  `chlog new --kind <Kind> --breaking --body '<past-tense description>'`.
  This is the ONLY thing that triggers a major version bump — the kind alone
  never does (per SemVer, major = incompatible change). When unsure whether a
  change breaks compatibility, ask the user instead of guessing.
- Fragments are YAML files in `.changes/unreleased/`; stage them with your commit.
- `chlog check` fails the build when a fragment is missing — never skip it.
<!-- chlog:end -->
