# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Does

AutoBump is a Go CLI tool that automates the release process: it reads `CHANGELOG.md`, calculates the next semantic version, updates the language-specific version files configured in `configs/autobump.yaml` (TypeScript `package.json`; Java `build.gradle`, `lib/build.gradle`, `pom.xml`, `src/main/resources/application.yaml`; Python `{project_name}/__init__.py`; C# `*/*.csproj`, `*/*.vdproj`; Helm `Chart.yaml`; Go and Terraform carry no version file and rely on git tags), commits, pushes, and creates PRs/MRs on GitHub, GitLab, or Azure DevOps. The list above mirrors the default config — treat `configs/autobump.yaml` as the source of truth.

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
go test -tags unit -run "TestDetectProjectLanguage" ./internal/domain/commands/
```

All unit test files use the `//go:build unit` build tag.

## Architecture

Clean Architecture with Hexagonal (Ports & Adapters) design using `go.uber.org/dig` for dependency injection.

### Layer Structure

- **`cmd/autobump/`** — Entry point (`main.go`) and DI wiring (`dig.go`)
- **`internal/domain/`** — Business logic and contracts (no framework dependencies)
  - `commands/` — Core use cases in `service.go` (`ProcessRepo`, `IterateProjects`, `DiscoverAndProcess`, `DetectProjectLanguage`) plus `VersionCommand` and `SelfUpdateCommand`
  - `entities/` — Domain types: `GlobalConfig`, `ProjectConfig`, `LanguageConfig`, `Controller` interface
- **`internal/infrastructure/`** — Implementations
  - `controllers/` — Cobra CLI handlers: `LocalController` (single-repo), `RunController` (batch + discover engine)
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
| `autobump local` or `autobump .` | `LocalController` | Process a single repository                                |
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
- Unit tests use `t.Parallel()` and `//go:build unit` tag
- Assertions via `testify/assert` and `testify/require`
- Test data via Builder pattern in `test/domain/entitybuilders/`
- White-box testing via `export_test.go` files that expose unexported functions

## Configuration

Config file search order: `.` → `.config/` → `configs/` → `~/` → `~/.config/`. Falls back to downloading default from GitHub. Token values support inline strings, `${ENV_VAR}` expansion, and file paths. SSH push auth is configured via `ssh_key_path`, `ssh_key_passphrase`, and `ssh_auth_sock` fields; common SSH agent sockets (1Password, standard `ssh-agent`) are auto-detected when not explicitly set.

Versioning modes: `semver` (default), `fork-dot`, `fork-dash`. Fork modes increment only the trailing fork digit (e.g. `3.3.0.16` → `3.3.0.17`) and skip language-specific version-file rewrites. See `internal/domain/commands/fork_version.go`.

Per-project `.autobump.yaml` files can override `changelog_path`, `versioning`, and `languages` fields. `loadProjectConfigOverrides` in `service.go` merges these into the resolved config.
