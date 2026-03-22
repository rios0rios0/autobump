<h1 align="center">AutoBump</h1>
<p align="center">
    <a href="https://github.com/rios0rios0/autobump/releases/latest">
        <img src="https://img.shields.io/github/release/rios0rios0/autobump.svg?style=for-the-badge&logo=github" alt="Latest Release"/></a>
    <a href="https://github.com/rios0rios0/autobump/blob/main/LICENSE">
        <img src="https://img.shields.io/github/license/rios0rios0/autobump.svg?style=for-the-badge&logo=github" alt="License"/></a>
    <a href="https://github.com/rios0rios0/autobump/actions/workflows/default.yaml">
        <img src="https://img.shields.io/github/actions/workflow/status/rios0rios0/autobump/default.yaml?branch=main&style=for-the-badge&logo=github" alt="Build Status"/></a>
    <a href="https://sonarcloud.io/summary/overall?id=rios0rios0_autobump">
        <img src="https://img.shields.io/sonar/coverage/rios0rios0_autobump?server=https%3A%2F%2Fsonarcloud.io&style=for-the-badge&logo=sonarqubecloud" alt="Coverage"/></a>
    <a href="https://sonarcloud.io/summary/overall?id=rios0rios0_autobump">
        <img src="https://img.shields.io/sonar/quality_gate/rios0rios0_autobump?server=https%3A%2F%2Fsonarcloud.io&style=for-the-badge&logo=sonarqubecloud" alt="Quality Gate"/></a>
    <a href="https://www.bestpractices.dev/projects/12020">
        <img src="https://img.shields.io/cii/level/12020?style=for-the-badge&logo=opensourceinitiative" alt="OpenSSF Best Practices"/></a>
</p>

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

### Quick Install (Recommended)

Install `autobump` with a single command:

```bash
curl -fsSL https://raw.githubusercontent.com/rios0rios0/autobump/main/install.sh | sh
```

Or using wget:

```bash
wget -qO- https://raw.githubusercontent.com/rios0rios0/autobump/main/install.sh | sh
```

#### Installation Options

```bash
# Install specific version
curl -fsSL https://raw.githubusercontent.com/rios0rios0/autobump/main/install.sh | sh -s -- --version v1.0.0

# Install to custom directory
curl -fsSL https://raw.githubusercontent.com/rios0rios0/autobump/main/install.sh | sh -s -- --install-dir /usr/local/bin

# Show what would be installed without doing it
curl -fsSL https://raw.githubusercontent.com/rios0rios0/autobump/main/install.sh | sh -s -- --dry-run

# Force reinstallation
curl -fsSL https://raw.githubusercontent.com/rios0rios0/autobump/main/install.sh | sh -s -- --force
```

### Download Pre-built Binaries

Download pre-built binaries from the [releases page](https://github.com/rios0rios0/autobump/releases).

## Configuration

Create a configuration file based on the example from `configs/autobump.yaml` and put it in `~/.config/autobump.yaml`.
You will need to configure at least one access token depending on which Git platform you use:

- **GitLab**: Set `gitlab_access_token` field with your GitLab personal access token (e.g., `glpat-TOKEN`)
- **Azure DevOps**: Set `azure_devops_access_token` field with your Azure DevOps personal access token
- **GitHub**: Set `github_access_token` field with your GitHub personal access token (e.g., `ghp_TOKEN`)

You can provide the token directly in the configuration file or specify a path to a file containing the token:

```yaml
# Direct token (not recommended for security)
gitlab_access_token: "???"

# Or path to token file (recommended)
gitlab_access_token: ".secure_files/gitlab_access_token.key"
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

### Optional SSH Authentication

When pushing to SSH remotes, AutoBump auto-detects the SSH agent from `SSH_AUTH_SOCK` and common socket paths (e.g., 1Password at `~/.1password/agent.sock`). For environments without an SSH agent, you can configure an explicit SSH key:

```yaml
# Path to SSH private key (supports ~ expansion)
ssh_key_path: '~/.ssh/id_ed25519'

# Passphrase for the key (optional, can be a file path)
ssh_key_passphrase: ''

# Or point to a custom SSH agent socket (e.g., 1Password)
ssh_auth_sock: '~/.1password/agent.sock'
```

## Usage

AutoBump has two main modes: **local** (single repository) and **run** (batch engine).

### 1. Local Mode

Process a single repository. Run in the project directory or specify a path:

```bash
autobump local              # Current directory
autobump local /path/to/repo  # Specific path
autobump .                  # Shorthand for local mode
```

AutoBump will automatically detect the project language, update the version files, update the CHANGELOG.md file, and create a merge request/pull request on your Git platform (GitLab, Azure DevOps, or GitHub).

You can manually specify the project language using the `-l` or `--language` flag:

```bash
autobump local -l java
```

Available languages: `go`, `java`, `python`, `typescript`, `cs`

You can also specify a custom configuration file path:

```bash
autobump local -c /path/to/custom/config.yaml
```

### 2. Run Mode (Batch + Discover)

The `run` command processes repositories from a configuration file. It auto-detects the mode based on config content:

- If `projects` is configured, iterates the static project list
- If `providers` is configured, discovers repos via provider APIs
- If both are present, both are processed

#### Static Project List

Add a `projects` section to your configuration file:

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
    project_access_token: "???"
```

#### Provider Discovery

Add a `providers` section to your configuration file:

```yaml
providers:
  # GitHub - discovers all repos in the specified organizations
  - type: "github"
    token: "ghp_TOKEN"
    organizations:
      - "my-github-org"

  # GitLab - discovers all projects in the specified groups (including subgroups)
  - type: "gitlab"
    token: "${GITLAB_TOKEN}"  # reads from environment variable
    organizations:
      - "my-gitlab-group"

  # Azure DevOps - discovers all repos in the specified organizations
  - type: "azuredevops"
    token: "/path/to/token/file"  # reads token from file
    organizations:
      - "my-azure-org"
```

The `token` field supports three formats:

- **Inline**: `"ghp_TOKEN"` -- the token value directly
- **Environment variable**: `"${ENV_VAR}"` -- reads the token from an environment variable
- **File path**: `"/path/to/file"` -- reads the token from a file on disk

Then run:

```bash
autobump run
```

AutoBump will process all configured sources (static project list and/or provider API discovery).

## How It Works

1. **Repository Discovery** *(run mode with providers)*: Queries GitHub, GitLab, and Azure DevOps APIs to find all repositories in configured organizations
2. **Language Detection**: AutoBump automatically detects the project language by looking for specific files (e.g., `go.mod`, `package.json`, `pom.xml`)
3. **Version Detection**: Reads the current version from CHANGELOG.md
4. **Version Update**: Determines the next version based on Semantic Versioning and updates language-specific version files
5. **CHANGELOG Update**: Moves unreleased changes to the new version section with the current date, deduplicating semantically overlapping entries
6. **Git Operations**: Commits changes, creates a new branch, and pushes to remote
7. **MR/PR Creation**: Creates a merge request (GitLab), pull request (GitHub), or pull request (Azure DevOps) for review

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

See [LICENSE](LICENSE) for details.
