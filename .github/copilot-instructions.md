# AutoBump

AutoBump is a Go CLI tool that automatically updates CHANGELOG.md files according to Keep a Changelog and Semantic Versioning standards, commits changes, pushes commits, and creates merge/pull requests on GitLab/Azure DevOps. It supports multiple programming languages including Go, Java, Python, TypeScript, and C#.

Always reference these instructions first and fallback to search or bash commands only when you encounter unexpected information that does not match the info here.

## Working Effectively

### Bootstrap, Build, and Test
- Install dependencies: `go mod download` -- takes <1 second (after first download)
- Build the binary: `make build` -- takes ~35 seconds first time, <1 second after. NEVER CANCEL. Set timeout to 60+ minutes.
- Run tests: `go test ./...` -- takes <1 second (cached), ~7 seconds clean. NEVER CANCEL. Set timeout to 30+ minutes.
- Format code: `go fmt ./...`
- Static analysis: `go vet ./...`
- Tidy dependencies: `go mod tidy`

### Linting and Testing with Pipeline Scripts
This project uses the [rios0rios0/pipelines](https://github.com/rios0rios0/pipelines) repository for linting and testing:

**To run tests:**
```bash
# Clone the pipelines repository if not already available
git clone https://github.com/rios0rios0/pipelines.git /tmp/pipelines

# Run tests using the pipeline script
/tmp/pipelines/global/scripts/GoLang/test/run.sh
```

**To run linting:**
```bash
# Clone the pipelines repository if not already available
git clone https://github.com/rios0rios0/pipelines.git /tmp/pipelines

# Run linting using GoLangCI-Lint script
/tmp/pipelines/global/scripts/GoLang/GoLangCI-Lint/run.sh
```

Note: The CI/CD pipeline automatically uses these scripts via the reusable workflow `rios0rios0/pipelines/.github/workflows/go-binary.yaml@main`.

### Running the Application
- ALWAYS run the bootstrapping steps first.
- Run via Makefile: `make run` 
- Run directly: `go run ./cmd/autobump`
- Run built binary: `./bin/autobump`
- Test help command: `./bin/autobump --help`
- Test batch mode help: `./bin/autobump batch --help`

### Installation
- Build first: `make build`
- Install system-wide: `make install` (copies to `/usr/local/bin/autobump`)

## Validation

### CRITICAL: Manual Validation Requirements
- ALWAYS test the built binary with `./bin/autobump --help` to ensure it works
- ALWAYS run the tool in dry-run mode to validate functionality: `./bin/autobump` (will fail at authentication, which is expected)
- ALWAYS exercise the batch command help: `./bin/autobump batch --help`
- The tool should detect Go language automatically and show version progression logging before authentication failure

### Testing Scenarios
After making changes, ALWAYS run through these validation steps:
1. `make build` - must complete successfully
2. `go test ./...` - all tests must pass
3. `./bin/autobump --help` - must show help text with available commands
4. `./bin/autobump` - should detect project language and process until authentication (expected failure)
5. `go fmt ./...` and `go vet ./...` - must pass clean

### Pre-commit Validation
- Always run `go fmt ./...` before committing or CI will fail
- Always run `go vet ./...` before committing 
- Always run `go test ./...` to ensure no regressions
- For full linting validation, use the pipeline script: `/tmp/pipelines/global/scripts/GoLang/GoLangCI-Lint/run.sh`
- CI pipeline uses the rios0rios0/pipelines repository scripts which will fail if code style or quality issues exist

## Build and Test Timing Expectations
- **Build**: ~35 seconds first time, <1 second subsequent builds. NEVER CANCEL. Set timeout to 60+ minutes.
- **Tests**: <1 second (cached), ~7 seconds clean run. NEVER CANCEL. Set timeout to 30+ minutes.
- **Go mod operations**: <1 second after first download. Set timeout to 15+ minutes.

## Common Tasks

### Repository Structure
```
/home/runner/work/autobump/autobump/
├── cmd/autobump/           # Main application code
│   ├── main.go             # CLI entry point and command setup
│   ├── config.go           # Configuration handling
│   ├── project.go          # Core project processing logic
│   ├── git.go              # Git operations
│   ├── changelog.go        # Changelog processing
│   ├── versioning.go       # Version file updates
│   ├── gitlab.go           # GitLab integration
│   ├── azuredevops.go      # Azure DevOps integration
│   └── *_test.go           # Unit tests for each component
├── configs/autobump.yaml   # Default configuration template
├── Makefile                # Build automation
├── go.mod                  # Go module definition
└── .github/workflows/      # CI/CD pipeline
```

### Key Files and Their Purpose
- `cmd/autobump/main.go` - CLI command definitions, entry point
- `cmd/autobump/config.go` - Configuration file parsing and validation
- `cmd/autobump/project.go` - Main business logic for processing repositories
- `cmd/autobump/changelog.go` - CHANGELOG.md parsing and updating
- `cmd/autobump/versioning.go` - Language-specific version file updates
- `configs/autobump.yaml` - Configuration template with language detection rules
- `Makefile` - Build targets: build, debug, build-musl, run, install

### Configuration System
- Default config location: `~/.config/autobump.yaml` 
- Fallback: `configs/autobump.yaml` in repository
- Config includes language detection rules and version file patterns
- Supports both single project and batch processing modes

### Language Support
The tool auto-detects and supports:
- **Go**: Detects via `go.mod`, updates version in `go.mod`
- **Java**: Detects via `build.gradle`, `pom.xml`, updates `build.gradle` and `application.yaml`
- **Python**: Detects via `pyproject.toml`, `setup.py`, updates `__init__.py`
- **TypeScript**: Detects via `package.json`, `tsconfig.json`, updates `package.json`
- **C#**: Detects via `*.sln`, `*.csproj`, updates project files

### Testing Infrastructure
- Comprehensive unit tests in `*_test.go` files
- Uses testify for assertions and go-faker for test data
- Tests cover configuration validation, changelog processing, git operations, and version updates
- All tests should pass reliably and quickly (<1 second cached, ~7 seconds clean)

### Development Workflow
1. Make code changes
2. Run `go fmt ./...` to format
3. Run `go vet ./...` to check for issues  
4. Run `go test ./...` to verify tests pass
5. Run `make build` to ensure clean build
6. Test binary with `./bin/autobump --help`
7. Test functional operation with `./bin/autobump` (should fail at auth step)
8. (Optional) Run full linting with pipeline script for final validation

### Common Development Commands
```bash
# Full development cycle
go mod download && make build && go test ./... && ./bin/autobump --help

# Quick test cycle
go test ./... && make build

# Format and lint (quick)
go fmt ./... && go vet ./...

# Full lint using pipeline scripts
git clone https://github.com/rios0rios0/pipelines.git /tmp/pipelines 2>/dev/null || true
/tmp/pipelines/global/scripts/GoLang/GoLangCI-Lint/run.sh

# Clean rebuild
rm -rf bin && make build
```

Always validate any changes by building and testing the actual binary functionality, not just unit tests.