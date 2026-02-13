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
- Run directly: `go run .`
- Run built binary: `./bin/autobump`
- Test help: `./bin/autobump --help`
- Batch mode help: `./bin/autobump batch --help`
- Discover mode help: `./bin/autobump discover --help`

### Installation

- Build first: `make build`
- Install system-wide: `make install` (copies to `/usr/local/bin/autobump`)

## Architecture

The project follows **Clean Architecture** with dependencies always pointing inward toward the domain layer.

### Repository Structure

```
autobump/
├── main.go                              # Entry point, calls cmd.Execute()
├── cmd/
│   └── root.go                          # CLI commands: root, batch, discover (Cobra)
├── domain/
│   ├── models.go                        # Entities: ServiceType, LatestTag, BranchStatus,
│   │                                    #   Repository; Interfaces: Language, RepositoryDiscoverer
│   ├── changelog.go                     # Changelog parsing, version calculation, entry
│   │                                    #   deduplication (token-based similarity)
│   ├── changelog_test.go                # BDD tests for changelog processing
│   ├── changelog_dedup_test.go          # BDD tests for deduplication logic
│   └── export_test.go                   # Exports unexported functions for white-box testing
├── application/
│   └── service.go                       # Use cases: ProcessRepo, IterateProjects,
│                                        #   DiscoverAndProcess, DetectProjectLanguage
├── config/
│   └── config.go                        # GlobalConfig, ProjectConfig, ProviderConfig,
│                                        #   LanguageConfig; config reading/validation/token resolution
├── infrastructure/
│   ├── git/
│   │   └── git.go                       # Git operations: clone, branch, commit (GPG), push
│   │                                    #   (SSH/HTTPS), tag retrieval (wraps go-git/v5)
│   ├── language/
│   │   └── python/
│   │       └── python.go                # Python Language implementation (pyproject.toml)
│   └── provider/
│       ├── interfaces.go                # GitServiceAdapter, PullRequestProvider interfaces
│       ├── registry.go                  # GitServiceRegistry (adapter lookup by URL/type)
│       ├── discoverer_registry.go       # DiscovererRegistry (factory-based discoverer creation)
│       ├── github/
│       │   ├── github.go                # GitHub adapter: auth, PR creation/existence
│       │   └── discoverer.go            # GitHub repo discovery (org + user fallback)
│       ├── gitlab/
│       │   ├── gitlab.go                # GitLab adapter: auth, MR creation/existence
│       │   └── discoverer.go            # GitLab repo discovery (group + user fallback)
│       └── azuredevops/
│           ├── azuredevops.go           # Azure DevOps adapter: auth, PR creation/existence
│           └── discoverer.go            # Azure DevOps repo discovery (projects + repos)
├── internal/
│   └── support/
│       └── utils.go                     # File I/O, HTTP downloads, GPG key handling, URL utils
├── configs/
│   └── autobump.yaml                    # Default configuration template
├── Makefile                             # Build: build, debug, build-musl, run, install
├── go.mod                               # Module: github.com/rios0rios0/autobump (Go 1.26)
└── .github/
    └── workflows/default.yaml           # CI/CD pipeline
```

### Layer Responsibilities

| Layer | Directory | Responsibility |
|---|---|---|
| **Domain** | `domain/` | Business entities, interfaces, changelog logic. No external dependencies. |
| **Application** | `application/` | Use-case orchestration (process repo, batch, discover). Depends only on domain + config. |
| **Infrastructure** | `infrastructure/` | External adapters: Git (go-git), providers (GitHub/GitLab/Azure DevOps APIs), language implementations. |
| **Config** | `config/` | Configuration structs, file reading, validation, token resolution (`${ENV_VAR}`, file path, inline). |
| **CMD** | `cmd/` | CLI entry point using Cobra. Wires dependencies and delegates to application layer. |
| **Internal** | `internal/support/` | Shared utilities: file I/O, HTTP, GPG, URL manipulation. |

### Key Design Patterns

- **Adapter pattern**: `GitServiceAdapter` interface with GitHub/GitLab/Azure DevOps implementations
- **Registry pattern**: `GitServiceRegistry` for adapter lookup, `DiscovererRegistry` for discoverer factories
- **Factory pattern**: Discoverer creation from provider config
- **Strategy pattern**: Language detection via file-pattern matching

### Key Domain Interfaces

- `Language` -- `GetProjectName() (string, error)`
- `RepositoryDiscoverer` -- `Name() string`, `DiscoverRepositories(ctx, org) ([]Repository, error)`
- `GitServiceAdapter` -- `GetServiceType()`, `MatchesURL()`, `GetAuthMethods()`, `CreatePullRequest()`, `PullRequestExists()`

### Key Domain Functions

- `ProcessChangelog(lines) (*semver.Version, []string, error)` -- processes changelog, calculates next version
- `DeduplicateEntries(entries) []string` -- removes exact duplicates and merges semantically overlapping entries using token overlap
- `UpdateSection(unreleased, version) ([]string, *semver.Version, error)` -- updates unreleased section, deduplicates, sorts, calculates version bump
- `FindLatestVersion(lines) (*semver.Version, error)` -- finds highest version in changelog

## CLI Commands

| Command | Description |
|---|---|
| `autobump` | Single project mode: detects language, bumps version, creates PR |
| `autobump batch` | Batch mode: processes all projects listed in config |
| `autobump discover` | Discovery mode: queries provider APIs to find repos, then bumps each |

### CLI Flags

- `--config/-c` -- config file path (available on all commands)
- `--language/-l` -- override detected language (root command only)

## Configuration

- Default config location: `~/.config/autobump.yaml`
- Fallback: `configs/autobump.yaml` in repository, then remote default URL
- Supports `projects` list (batch mode) and `providers` list (discover mode)
- Token resolution: inline string, `${ENV_VAR}` expansion, or file path auto-detection

### Provider Configuration (discover mode)

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

### Test Files

| File | Tests |
|---|---|
| `domain/changelog_test.go` | Changelog processing, version bumping, section parsing, formatting |
| `domain/changelog_dedup_test.go` | Entry normalization, tokenization, version extraction, overlap ratio, deduplication |
| `domain/export_test.go` | Exports unexported functions for white-box testing |

### Running Tests

```bash
make test             # Full test suite via pipeline scripts (ALWAYS use this)
go test ./domain/...  # Quick domain-only check during development (acceptable)
```

## Validation

### After Making Changes

1. `make lint` -- must report 0 issues
2. `make test` -- all tests must pass
3. `make build` -- must complete successfully
4. `./bin/autobump --help` -- must show help text with all three commands
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
go test ./domain/... && make build

# Clean rebuild
rm -rf bin && make build

# Full security + quality gate
make lint && make test && make sast
```

Always validate any changes by building and testing the actual binary functionality, not just unit tests.
