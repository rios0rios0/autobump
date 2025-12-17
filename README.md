# AutoBump

Automatically update CHANGELOG.md according to the [Keep a Changelog (version 1.1.0)](https://keepachangelog.com/en/1.1.0/) standard and the [Semantic Versioning (version 2.0.0)](https://semver.org/spec/v2.0.0.html) standard,
commit the changes, push the commits, and create a merge request/pull request on GitLab, Azure DevOps, or GitHub.

## Supported Languages

AutoBump supports automatic language detection and version updates for:
- **Go**: Detects via `go.mod`, updates version in `go.mod`
- **Java**: Detects via `build.gradle`, `pom.xml`, updates `build.gradle` and `application.yaml`
- **Python**: Detects via `pyproject.toml`, `setup.py`, updates `__init__.py`
- **TypeScript**: Detects via `package.json`, `tsconfig.json`, updates `package.json`
- **C#**: Detects via `*.sln`, `*.csproj`, updates project files

## Installation

### Binary Releases

AutoBump has binary releases in the [releases section](https://github.com/rios0rios0/autobump/releases).

### Build from Source

If you'd like to compile it yourself, make sure you have Go 1.25+ and Make installed, then use the following commands:

```bash
# Download dependencies
go mod download

# Build the binary
make build

# Install system-wide (optional)
make install
```

This will create the binary at `./bin/autobump` and optionally install it to `/usr/local/bin/autobump`.

## Configuration

Create a configuration file based on the example from `configs/autobump.yaml` and put it in `~/.config/autobump.yaml`.
You will need to configure at least one access token depending on which Git platform you use:
- **GitLab**: Set `gitlab_access_token` field with your GitLab personal access token (e.g., `glpat-TOKEN`)
- **Azure DevOps**: Set `azure_devops_access_token` field with your Azure DevOps personal access token
- **GitHub**: Set `github_access_token` field with your GitHub personal access token (e.g., `ghp_TOKEN`)

You can provide the token directly in the configuration file or specify a path to a file containing the token:

```yaml
# Direct token (not recommended for security)
gitlab_access_token: "glpat-TOKEN"

# Or path to token file (recommended)
gitlab_access_token: ".secure_files/gitlab_access_token.key"
```

### Environment Variable Support

AutoBump can also read access tokens from environment variables, which is particularly useful in CI/CD environments like GitHub Actions and Azure Pipelines:

- **GitHub**: Reads from `GITHUB_TOKEN` (primary) or `GH_TOKEN` (fallback)
- **Azure DevOps**: Reads from `SYSTEM_ACCESSTOKEN`
- **GitLab**: Reads from `CI_JOB_TOKEN`

**Precedence order:**
1. Config file values (highest priority)
2. Environment variables
3. For GitHub: `GITHUB_TOKEN` takes priority over `GH_TOKEN`

This means you can run AutoBump in GitHub Actions without any configuration:

```yaml
# GitHub Actions workflow example
- name: Run AutoBump
  run: autobump batch
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Or in Azure Pipelines:

```yaml
# Azure Pipelines example
- script: autobump batch
  env:
    SYSTEM_ACCESSTOKEN: $(System.AccessToken)
```

### Optional GPG Key Signing

You can optionally configure GPG key signing for commits:

```yaml
gpg_key_path: "/home/user/.gnupg/autobump.asc"
```

To export your GPG key:
```bash
gpg --export-secret-key --armor $(git config user.signingkey) > ~/.gnupg/autobump.asc
```

## Usage

There are two ways to run AutoBump: for a single project or for multiple projects in batch mode.

### 1. Single Project Mode

Simply run this command in the project directory. AutoBump will automatically detect the project language, update the version files, update the CHANGELOG.md file, and create a merge request/pull request on your Git platform (GitLab, Azure DevOps, or GitHub).

```bash
autobump
```

You can manually specify the project language using the `-l` or `--language` flag:

```bash
autobump -l java
```

Available languages: `go`, `java`, `python`, `typescript`, `cs`

You can also specify a custom configuration file path:

```bash
autobump -c /path/to/custom/config.yaml
```

### 2. Batch Mode

Modify the configuration file and add a list of your projects into the `projects` section:

```yaml
projects:
  # Local repository path with auto-detected language
  - path: "/home/user/repo1"
  
  # Local repository with manually specified language
  - path: "/home/user/repo2"
    language: "Java"
  
  # Git URL - AutoBump will clone automatically into a temporary directory
  - path: "git@github.com:example/repo3.git"
  
  # Project with specific access token (overrides global token)
  - path: "https://gitlab.com/user/repo4.git"
    project_access_token: "glpat-PROJECT-SPECIFIC-TOKEN"
```

Then run AutoBump in batch mode:

```bash
autobump batch
```

AutoBump will iterate through each project and perform the same actions as in single project mode.

## How It Works

1. **Language Detection**: AutoBump automatically detects the project language by looking for specific files (e.g., `go.mod`, `package.json`, `pom.xml`)
2. **Version Detection**: Reads the current version from CHANGELOG.md
3. **Version Update**: Determines the next version based on Semantic Versioning and updates language-specific version files
4. **CHANGELOG Update**: Moves unreleased changes to the new version section with the current date
5. **Git Operations**: Commits changes, creates a new branch, and pushes to remote
6. **MR/PR Creation**: Creates a merge request (GitLab), pull request (GitHub), or pull request (Azure DevOps) for review

## Development

### Prerequisites
- Go 1.25+
- Make

### Building
```bash
go mod download
make build
```

### Testing
```bash
go test ./...
```

### Linting
This project uses the linting configuration from the [rios0rios0/pipelines](https://github.com/rios0rios0/pipelines) repository. The CI/CD pipeline automatically runs linting via the reusable workflow.

## License

See [LICENSE](LICENSE) file for details.
