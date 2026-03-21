# AutoBump

AutoBump is a Go CLI tool that automatically updates CHANGELOG.md files according to [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Semantic Versioning](https://semver.org/spec/v2.0.0.html) standards, commits changes, pushes commits, and creates merge/pull requests on GitHub, GitLab, and Azure DevOps. It supports multiple programming languages including Go, Java, Python, TypeScript, and C#.

Always reference these instructions first and fall back to search or bash commands only when you encounter unexpected information that does not match the info here.

## Working Effectively

### Bootstrap, Build, and Test

- Install dependencies: `go mod download` -- takes <1 second (after first download)
- Build the binary: `make build` -- takes ~35 seconds first time, <1 second after. NEVER CANCEL. Set timeout to 60+ minutes.
- Run tests: `make test` -- NEVER run `go test` directly.
- Run linting: `make lint` -- NEVER run `golangci-lint` directly.
- Run security analysis: `make sast` -- NEVER run `gitleaks`, `semgrep`, `trivy`, `hadolint`, or `codeql` directly.
- Tidy dependencies: `go mod tidy`

### Linting, Testing, and SAST with Makefile

This project uses the [rios0rios0/pipelines](https://github.com/rios0rios0/pipelines) repository for shared CI/CD scripts. The `Makefile` imports these scripts via `SCRIPTS_DIR`. Always use `make` targets:

```bash
make lint    # golangci-lint via pipeline scripts
make test    # unit + integration tests via pipeline scripts
make sast    # CodeQL, Semgrep, Trivy, Hadolint, Gitleaks
```

Note: The CI/CD pipeline automatically uses these scripts via the reusable workflow `rios0rios0/pipelines/.github/workflows/go-binary.yaml@main`.

### Running the Application

- ALWAYS run the bootstrapping steps first.
- Run via Makefile: `make run`
- Run directly: `go run ./cmd/autobump`
- Run built binary: `./bin/autobump`
- Test help: `./bin/autobump --help`
- Local mode help: `./bin/autobump local --help`
- Run mode help: `./bin/autobump run --help`

### Installation

- Build first: `make build`
- Install to user bin: `make install` (copies to `~/.local/bin/autobump`)

## Architecture

The project follows **Clean Architecture** with dependencies always pointing inward toward the domain layer. Dependency injection is handled by [go.uber.org/dig](https://github.com/uber-go/dig). Provider and Git forge abstractions are sourced from the [rios0rios0/gitforge](https://github.com/rios0rios0/gitforge) library.

### Repository Structure

```
autobump/
├── cmd/
│   └── autobump/
│       ├── main.go                      # Entry point: wires DI, builds Cobra commands
│       └── dig.go                       # DIG injection helpers: injectAppContext,
│                                        #   injectLocalController, injectRunController,
│                                        #   injectProviderRegistry
├── internal/
│   ├── app.go                           # AppInternal: aggregates all controllers
│   ├── container.go                     # RegisterProviders: wires all DIG layers
│   ├── domain/
│   │   ├── commands/
│   │   │   ├── service.go               # Use cases: ProcessRepo, IterateProjects,
│   │   │   │                            #   DiscoverAndProcess, DetectProjectLanguage
│   │   │   ├── container.go             # No-op RegisterProviders (commands called directly)
│   │   │   ├── export_test.go           # Exports unexported functions for white-box testing
│   │   │   └── service_test.go          # BDD unit tests for command functions
│   │   └── entities/
│   │       ├── changelog.go             # Changelog parsing, version calculation, entry
│   │       │                            #   deduplication (token-based similarity)
│   │       ├── controller.go            # Controller interface and ControllerBind struct
│   │       ├── repository.go            # Re-exports gitforge entities: ServiceType,
│   │       │                            #   LatestTag, BranchStatus, Repository,
│   │       │                            #   RepositoryDiscoverer, Language interface
│   │       ├── settings.go              # GlobalConfig, ProjectConfig, ProviderConfig,
│   │       │                            #   LanguageConfig, VersionFile; config
│   │       │                            #   reading/validation/token resolution
│   │       ├── container.go             # No-op RegisterProviders (entities are runtime-loaded)
│   │       └── export_test.go           # Exports unexported changelog functions for testing
│   ├── infrastructure/
│   │   ├── controllers/
│   │   │   ├── local_controller.go      # "local" subcommand (single repo mode)
│   │   │   ├── run_controller.go        # "run" subcommand (batch + discover engine)
│   │   │   ├── config_helpers.go        # Shared config reading/validation helper
│   │   │   └── container.go             # RegisterProviders for all controllers via DIG
│   │   └── repositories/
│   │       ├── provider_registry.go     # ProviderRegistry wrapping gitforge's registry
│   │       ├── container.go             # Registers GitHub/GitLab/Azure DevOps adapters
│   │       │                            #   and discoverer factories with the registry
│   │       └── python/
│   │           └── python.go            # Python Language implementation (pyproject.toml)
│   └── support/
│       └── utils.go                     # File I/O, HTTP downloads, URL utils
├── test/
│   └── domain/
│       └── entitybuilders/
│           └── repository_builder.go    # Test builder for Repository entities
├── configs/
│   ├── autobump.yaml                    # Default configuration template
│   └── CHANGELOG.template.md           # Default CHANGELOG template
├── Makefile                             # Build: build, debug, build-musl, run, install
├── go.mod                               # Module: github.com/rios0rios0/autobump (Go 1.26)
└── .github/
    └── workflows/default.yaml           # CI/CD pipeline (go-binary reusable workflow)
```

### Layer Responsibilities

| Layer | Directory | Responsibility |
|---|---|---|
| **Entities** | `internal/domain/entities/` | Business entities, interfaces, changelog logic, config structs. Re-exports gitforge types. |
| **Commands** | `internal/domain/commands/` | Use-case orchestration (process repo, batch, discover, language detection). |
| **Controllers** | `internal/infrastructure/controllers/` | CLI entry points (Cobra). Wires config reading and command invocation. |
| **Repositories** | `internal/infrastructure/repositories/` | Provider registry (wraps gitforge), language implementations. |
| **Support** | `internal/support/` | Shared utilities: file I/O, HTTP downloads, URL manipulation. |
| **CMD** | `cmd/autobump/` | Binary entry point. Wires DIG containers and builds Cobra command tree. |

### Key Design Patterns

- **Dependency Injection**: `go.uber.org/dig` wires all layers; each package exposes `RegisterProviders(container)`
- **Adapter pattern**: `ForgeProvider`/`LocalGitAuthProvider` interfaces (from gitforge) with GitHub/GitLab/Azure DevOps implementations
- **Registry pattern**: `ProviderRegistry` wraps gitforge's registry for adapter and discoverer lookup
- **Factory pattern**: Discoverer and provider creation from token string via registered factories
- **Strategy pattern**: Language detection via file-pattern matching (special patterns and extensions)
- **Controller pattern**: Each CLI subcommand is a `Controller` implementing `GetBind()` and `Execute()`

### Key Domain Interfaces

- `Language` (in `internal/domain/entities/`) -- `GetProjectName() (string, error)`
- `Controller` (in `internal/domain/entities/`) -- `GetBind() ControllerBind`, `Execute(cmd, args)`
- `RepositoryDiscoverer` -- re-exported from gitforge; `Name() string`, `DiscoverRepositories(ctx, org) ([]Repository, error)`
- `ForgeProvider`/`LocalGitAuthProvider` -- gitforge interfaces for Git hosting provider adapters

### Key Domain Functions

- `ProcessChangelog(lines) (*semver.Version, []string, error)` -- processes changelog, calculates next version
- `DeduplicateEntries(entries) []string` -- removes exact duplicates and merges semantically overlapping entries using token overlap
- `UpdateSection(unreleased, version) ([]string, *semver.Version, error)` -- updates unreleased section, deduplicates, sorts, calculates version bump
- `FindLatestVersion(lines) (*semver.Version, error)` -- finds highest version in changelog

## CLI Commands

| Command | Description |
|---|---|
| `autobump` | Shows help (use `autobump .` as shorthand for local mode) |
| `autobump local` | Single project mode: detects language, bumps version, creates PR |
| `autobump run` | Engine mode: auto-detects batch (static project list) and/or discover (provider APIs) from config |
| `autobump batch` | **Deprecated**: hidden alias for `run` (shows deprecation warning) |
| `autobump discover` | **Deprecated**: hidden alias for `run` (shows deprecation warning) |

### CLI Flags

- `--config/-c` -- config file path (persistent, available on all commands)
- `--verbose/-v` -- enable verbose output (persistent, available on all commands)
- `--language/-l` -- override detected language (`local` command and root shorthand only)

## Configuration

- Default config search order: `.`, `.config/`, `configs/`, `~/`, `~/.config/` (file names: `autobump.yaml`, `autobump.yml`, `.autobump.yaml`, `.autobump.yml`)
- Final fallback: remote default URL (`configs/autobump.yaml` in this repository)
- Config structs live in `internal/domain/entities/settings.go`: `GlobalConfig`, `ProjectConfig`, `ProviderConfig`, `LanguageConfig`, `VersionFile`
- Supports `projects` list and/or `providers` list (both processed by `run` command)
- Token resolution: inline string, `${ENV_VAR}` expansion, or file path auto-detection
- SSH push auth: `ssh_key_path`, `ssh_key_passphrase`, `ssh_auth_sock` fields; auto-detects common SSH agent sockets (1Password, standard `ssh-agent`) when not explicitly set

### Provider Configuration (run mode with providers)

```yaml
providers:
  - type: "github"           # "github", "gitlab", "azuredevops"
    token: "${GITHUB_TOKEN}" # inline, ${ENV_VAR}, or file path
    organizations:
      - "my-org"
```

## Language Support

The tool auto-detects and supports:

- **Go**: Detects via `go.mod`, updates version in `go.mod`
- **Java**: Detects via `build.gradle`, `pom.xml`, updates `build.gradle` and `application.yaml`
- **Python**: Detects via `pyproject.toml`, `setup.py`, updates `__init__.py`
- **TypeScript**: Detects via `package.json`, `tsconfig.json`, updates `package.json`
- **C#**: Detects via `*.sln`, `*.csproj`, updates project files

## Testing

### Standards

- All tests follow **BDD** structure with `// given`, `// when`, `// then` comment blocks
- Test descriptions use `"should ... when ..."` format via `t.Run()` subtests
- Tests use `testify/assert` and `testify/require` for assertions
- Tests use `t.Parallel()` at both parent and subtest level
- Test files use build tags (`//go:build unit`) to separate unit from integration tests

### Test Files

| File | Tests |
|---|---|
| `internal/domain/commands/service_test.go` | Language detection, repo processing, discover/batch logic |
| `internal/domain/commands/export_test.go` | Exports unexported command functions for white-box testing |
| `internal/domain/entities/export_test.go` | Exports unexported changelog/dedup functions for white-box testing |
| `test/domain/entitybuilders/repository_builder.go` | Test builder for `Repository` entities |

### Running Tests

```bash
make test               # Full test suite via pipeline scripts (ALWAYS use this)
go test ./internal/...  # Quick internal-only check during development (acceptable)
```

## Validation

### After Making Changes

1. `make lint` -- must report 0 issues
2. `make test` -- all tests must pass
3. `make build` -- must complete successfully
4. `./bin/autobump --help` -- must show help text with `local` and `run` commands
5. `make sast` -- should report no new findings

### Pre-commit

- Always run `make lint` before committing (CI will fail otherwise)
- Always run `make test` to ensure no regressions
- Always run `make sast` to catch security issues

## Build and Test Timing Expectations

- **Build**: ~35 seconds first time, <1 second subsequent. NEVER CANCEL. Set timeout to 60+ minutes.
- **Tests**: <1 second cached, ~7 seconds clean. NEVER CANCEL. Set timeout to 30+ minutes.
- **Lint**: ~5-15 seconds. Set timeout to 60+ minutes.
- **SAST**: ~1-3 minutes. Set timeout to 60+ minutes.
- **Go mod operations**: <1 second after first download. Set timeout to 15+ minutes.

## Common Development Commands

```bash
# Full validation cycle
make lint && make test && make build && ./bin/autobump --help

# Quick test cycle during development
go test ./internal/... && make build

# Clean rebuild
rm -rf bin && make build

# Full security + quality gate
make lint && make test && make sast
```

Always validate any changes by building and testing the actual binary functionality, not just unit tests.
