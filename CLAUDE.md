# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Does

AutoBump is a Go CLI tool that automates the release process: it reads `CHANGELOG.md`, calculates the next semantic version, updates the language-specific version files configured in `configs/autobump.yaml` (TypeScript `package.json`; Java `build.gradle`, `lib/build.gradle`, `pom.xml`, `src/main/resources/application.yaml`; Python `{project_name}/__init__.py`; C# `*/*.csproj`, `*/*.vdproj`; Helm `Chart.yaml`; Dart/Flutter `pubspec.yaml`; Go relies on git tags but Swagger-documented APIs get the `@version` annotation in `main.go`/`cmd/main.go`/`cmd/*/main.go` and the generated `docs.go`/`swagger.json`/`swagger.yaml` bumped when present; Terraform carries no version file and relies on git tags), commits, pushes, and creates PRs/MRs on GitHub, GitLab, or Azure DevOps. The list above mirrors the default config — treat `configs/autobump.yaml` as the source of truth. Version files are content-filtered: a globbed file only counts when one of its regex patterns matches its content.

Two version files need more than plain substitution, and those exceptions live in `versionFileHooks` (`internal/domain/commands/version_file_hooks.go`), a table keyed by base name rather than branches inside the rewrite: `pom.xml` masks its `<parent>` block so the first `<version>` match is the project's own, and `pubspec.yaml` routes the value through `langforge`'s `dart.BumpBuildNumber` so a Flutter build number (`1.2.3+7` → `1.3.0+8`) survives the release — Google Play and App Store Connect reject an upload whose build number did not increase. A third exception is a map entry, not an edit to `applyVersionPatterns`.

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

Unit tests carry **no build tag**. `//go:build unit` only hid them from plain `go test ./...`
and from IDE runs while buying nothing, because an untagged file compiles under a `-tags`
build too — the shared pipeline's phase 1 (`-tags test,unit`) still runs every one of them.
Phase 2 (`-tags integration`) selects only packages where the tag makes a test file *appear*,
so untagged tests are not run twice. Reserve `//go:build integration` for tests that need
real infrastructure; this repository has none today.

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

- **`gitforge`** (`github.com/rios0rios0/gitforge`) — Git provider adapters, changelog processing, PR creation
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

Configuration is **layered**: built-in defaults (`configs/autobump.yaml`, embedded via
`go:embed` in `configs/embed.go`) -> published defaults (the same file fetched from `main`,
best effort) -> the operator's file (`-c`, else `~/` or `~/.config/`) -> the repository's
own `.autobump.yaml`. Each layer overrides only the keys its document declares.
`internal/domain/entities/config_layers.go` is the engine; `configLoader` in
`internal/infrastructure/controllers/config_helpers.go` assembles the first three, and
`loadProjectConfigOverrides` (`service.go`) applies the fourth after the clone.

The mechanism is `yaml.v3` decoding into a *non-zero* struct: only the keys the document
carries are assigned, so absent-versus-`false` needs no pointer fields. Maps are the
exception — each value decodes into a fresh zero element — so `LanguagesConfig` is blanked
before the decode and re-merged with `MergeLanguagesConfig` afterwards, which is what keeps
the per-path version-file rules in force *across* layers rather than within one. Apply is
atomic (decode into a scratch copy, commit on success), and a comments-only document
returns `io.EOF`, which is a layer with nothing to say rather than a broken one.

Only the operator's layer is `ScopeOperator`; the other three decode through
`RestrictedConfig`, which has **no field** for `providers`, `projects`, any credential, or
`bump_branch_prefix`. That is the whole enforcement — not a check that has to run at the
right moment, but a struct with nowhere for those keys to land. `operatorOnlyKeys` exists
only to say out loud that they were ignored. Strictness is per layer: the embedded defaults
and the operator's file are strict (a typo is worth reporting), the fetched and project
layers are not (`main` may be newer than the binary, and a project file is partial by
design).

`bump_branch_prefix` is operator-only *and* validated by `ValidateBumpBranchPrefix`, because
it is the argument to a destructive operation: `cleanupStaleBumpBranches` deletes every
remote branch starting with it and closes each attached PR. It must name a branch git would
accept, must contain a `/`, and must not *stop* at one — `chore/` would match
`chore/autoupdate-*`, so a bare namespace makes one tool delete another's branches. An
operator's typo is as capable of that as a hostile repository, which is why the check runs
over their own file too. `filterStaleBumpBranches` carries the second lock: it never deletes
a branch equal to the prefix, or the default branch.

Token values support inline strings, `${ENV_VAR}` expansion, and file paths, resolved once
by `FinalizeGlobalConfig` over the *folded* configuration — running it per layer would read
a token path off disk even when the next layer replaced it. SSH push auth is configured via
`ssh_key_path`, `ssh_key_passphrase` and `ssh_auth_sock`; common agent sockets are
auto-detected when not set.

Versioning modes: `semver` (default), `fork-dot`, `fork-dash`. Fork modes increment only the trailing fork digit (e.g. `3.3.0.16` → `3.3.0.17`) and skip language-specific version-file rewrites. See `internal/domain/commands/fork_version.go`.

A repository's own `.autobump.yaml` may set `refresh`, `changelog_path`, `versioning`,
`detect_chlog`, `cleanup_stale_branches`, `exclude_forks`, `exclude_archived` and
`languages`. `ApplyProjectLayer` folds it onto the configuration *and* onto the project
entry, and the entry pass reads what *that document* declared rather than the folded
result — reading the folded value would let the operator's own global default overwrite the
`projects[]` entry they wrote beside it, which is the opposite of the precedence everything
else follows. The repository's file now wins over that entry, where before it only filled in
what the entry left empty. `resolveChangelogPath` gained the global fallback to match, so
`GlobalConfig.ChangelogPath` — decoded and never read before layering — finally means
something.

The refresh (`refresh`, top-level or per-language, opt-in) regenerates what the version
files derive. AutoBump rewrites version files with regexes and runs no package manager, so a
lockfile keeps the pre-bump resolution descriptor and the first `yarn install --immutable`
in CI rejects the release with YN0028. `internal/domain/commands/refresh_recipes.go` holds
the recipes and `refresh_commands.go` runs them, handing what they wrote to
`addFilesToWorktree` for staging.

**AutoBump owns the argv.** That is the load-bearing change: `refresh_commands` used to read
a command out of configuration, which made it an executable run with the release
credentials, which meant a repository's own file could never be trusted with it — and that
one exception is what stopped the project layer from simply overriding things like every
other layer. With the recipes as compile-time constants there is nothing left to sanitize,
which is why `SanitizeUntrustedLanguages` is gone.

`refreshRecipes` maps a language to a detector; only `typescript`/`javascript`/`node` have
one, because a lockfile only goes stale when the rewrite changes a string it keys on, and
outside a JS workspace it never does. `nodeMarkers` is ordered on purpose: a repository
migrating between package managers carries two lockfiles, and `packageManager` is the only
signal saying which is current rather than left over. Yarn Classic is detected and skipped —
it has no install mode that resolves without also linking and running install scripts.
Every recipe is resolution-only, and the extra flags are load-bearing: npm runs the root
`prepare`/`postinstall` under `--package-lock-only` since npm 7, and pnpm defaults
`frozen-lockfile` to true in CI, which is exactly where `run` mode lives. A missing binary
aborts that repository's release rather than skipping, because skipping opens exactly the
pull request the feature prevents and looks identical to having nothing to refresh. Fork
versioning skips it along with the version-file rewrite it already skips.

chlog support: projects using [chlog](https://github.com/luizjhonata/chlog) keep pending changes as YAML fragments in `.changes/unreleased/` and therefore have a permanently empty `[Unreleased]` section. `internal/domain/commands/chlog.go` detects the layout (a `.chlog.yaml` or the fragment directory), parses the fragments, and renders them as Keep a Changelog `### <Section>` entries; chlog is a command-only module, so its format is re-implemented rather than imported. The splice happens in `readChangelogLines` (`service.go`), the single boundary every changelog read goes through — so the emptiness check, SemVer calculation, and fork mode all keep operating on plain Keep a Changelog lines. Fragment entries are merged with anything already in `[Unreleased]`, and AutoBump's own SemVer rules decide the bump rather than chlog's `auto` mapping. `consumeChlogFragments` deletes the consumed fragments after the rewrite and `stageChlogConsumption` stages the removals. It also writes a `.gitkeep` into the fragment directory via `KeepChlogUnreleasedDirectory` and stages that: Git tracks files rather than directories, so the commit that removes the last fragment would otherwise remove `.changes/unreleased/` too — and with it the layout `DetectChlog` recognises, leaving the next run to read the permanently empty `[Unreleased]` section as "nothing to release". The placeholder is returned for staging even when it already exists, because one left by an aborted earlier run is still untracked. Detection is opt-out via `entities.ChlogEnabled` (`detect_chlog`). A pending `.changes/v*.md` left by `chlog batch` aborts the run with `ErrChlogPendingVersionFiles`. `ChlogFragment.Breaking` carries what `chlog new --breaking` writes: chlog stores the flag only to force a major bump when *it* picks the version and never renders it, but AutoBump reads the bump off the rendered lines, so `renderChlogBody` has to turn the flag into the marker the counter recognises — via `entities.NormalizeBreakingChangeMarker` rather than a prepend, because a writer who passes `--breaking` usually opens the body with "BREAKING CHANGE:" as well and prepending would announce it twice.

Changelog rules: `entities.NormalizeUnreleasedSection` (`internal/domain/entities/changelog_normalize.go`) is where heading repair, breaking-marker canonicalisation, deduplication, verb-based reclassification and ordering live. It runs at the end of `readChangelogLines` — the same single boundary the fragments are spliced at — which is what extends the rules to the two paths that never had them: chlog fragments (each written alone, so nobody ever sees the pending set side by side) and fork versioning (`rewriteUnreleasedAsForkRelease` copies the section without consulting the SemVer pipeline the rules used to live inside). It is idempotent, because the changelog is read several times per run, and it only ever touches `[Unreleased]`.

Three details are load-bearing. The rules operate on *entries* — a bullet plus its continuation lines — not on lines: `DeduplicateEntries` and `ReclassifyEntriesByVerb` are still gitforge's, called with the bullets alone and the results paired back by order (unambiguous, since bullets are unique inside a section once exact repeats are gone) and one entry at a time respectively. gitforge itself still reads one entry per line, so `ProcessChangelog` folds each entry onto a single line before handing it over and unfolds afterwards — without that, a continuation line opening with "removed" is counted as a change, compared for duplication, and moved to `### Removed`, orphaned from the bullet it explains. And heading repair is not cosmetic: the release renderer buckets by the exact string `### Added`, so `#### added` silently drops every entry underneath it.

Stale branch cleanup: before creating the bump branch, `cleanupStaleBumpBranches` (`internal/domain/commands/cleanup.go`) deletes every remote branch matching the bump prefix and closes the PR/MR attached to each (Azure DevOps abandons). It runs only when a bump is needed, so a PR is never closed without a replacement. It is opt-out — `entities.CleanupEnabled` treats an unset `cleanup_stale_branches` as enabled, and the persistent `--skip-cleanup` flag (applied by `applySkipCleanupFlag` in `controllers/config_helpers.go`) overrides the config per run. `entities.ResolveBumpBranchPrefix` (`bump_branch_prefix`, default `chore/bump-`) drives both branch creation and cleanup so the two cannot diverge. Cleanup is best-effort: every failure is logged and skipped rather than aborting the release.

<!-- chlog:start -->
## Changelog (chlog) — MANDATORY

If the repository you are working in uses chlog (a `.chlog.yaml` or `.chlog.yml`
config file, or a `.changes/` directory, exists at the project root), the
following is binding and ALWAYS applies: whenever you make ANY change, you MUST
create a changelog fragment as part of the same change — automatically, without
being asked, before committing.

- Do NOT edit CHANGELOG.md directly; it is generated from fragments.
- Create the fragment with:
  `chlog new --kind <Kind> --body "<imperative description>"`
- Valid kinds: Added, Changed, Deprecated, Removed, Fixed, Security
- Choose the kind that best matches the change (e.g., new feature → Added,
  bug fix → Fixed, behavior change → Changed, removal → Removed, security fix → Security).
- If the change is backward-INCOMPATIBLE with the public API (a breaking
  change), you MUST add the `--breaking` flag:
  `chlog new --kind <Kind> --breaking --body "<description>"`.
  This is the ONLY thing that triggers a major version bump — the kind alone
  never does (per SemVer, major = incompatible change). When unsure whether a
  change breaks compatibility, ask the user instead of guessing.
- Fragments are YAML files in `.changes/unreleased/`; stage them with your commit.
- `chlog check` fails the build when a fragment is missing — never skip it.
<!-- chlog:end -->
