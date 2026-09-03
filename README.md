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

- **Dart/Flutter**: Detects via `pubspec.yaml`, updates its `version` field. A Flutter build number is carried across the bump and incremented (`1.2.3+7` → `1.3.0+8`), preserving zero padding, because stores reject an upload whose build number did not increase
- **Go**: Detects via `go.mod`; versions managed through git tags; APIs documented with Swagger (`swaggo`) also get the `@version` annotation in the entrypoint (`main.go`, `cmd/main.go`, or `cmd/*/main.go`) and the generated `docs.go`/`swagger.json`/`swagger.yaml` (under `docs/`, `cmd/docs/`, or `cmd/*/docs/`) updated
- **Helm**: Detects via `Chart.yaml`, updates the `version` field in `Chart.yaml`
- **Java**: Detects via `build.gradle`, `pom.xml`, updates `build.gradle` and `application.yaml`
- **Python**: Detects via `pyproject.toml`, `setup.py`, updates `__init__.py`
- **Terraform**: Detects via `*.tf`, `versions.tf`; versions managed through git tags (no version file)
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

### Configuration Layers

AutoBump folds four configuration sources, each overriding only the keys it declares:

| # | Layer | Where it comes from | May set |
|---|-------|---------------------|---------|
| 1 | **built-in defaults** | `configs/autobump.yaml`, compiled into the binary | languages and behaviour |
| 2 | **published defaults** | the same file fetched from `main`, best effort | languages and behaviour |
| 3 | **operator configuration** | `-c <path\|URL>`, else `~/.autobump.yaml` or `~/.config/autobump.yaml` | **everything** |
| 4 | **project configuration** | `.autobump.yaml` in the repository being released | languages and behaviour |

Layer 1 always exists, so AutoBump knows every language it supports with no configuration
and no network. Layer 2 lets a language fix reach an installed binary without a release;
when it cannot be fetched, the run says so and carries on. Layer 3 is the only one that
may name a credential, a `providers`/`projects` list, or the bump branch prefix -- the
other three decode through a schema that has no field for them, so a repository cannot
hand AutoBump credentials or aim its branch deletion.

Set the same key at several levels and the last one wins:

```yaml
# configs/autobump.yaml (built in)      -> versioning defaults to 'semver'
# ~/.autobump.yaml                      -> versioning: 'fork-dash'
# the repository's .autobump.yaml       -> versioning: 'fork-dot'
# effective                             -> fork-dot
```

The working directory is never searched for the operator's configuration. AutoBump
normally runs with the repository it is releasing as the working directory, and that
repository may carry its own `.autobump.yaml`; reading it as the operator's configuration
would substitute a project's overrides for the operator's rather than layering them.

### Getting started

Copy [`configs/autobump.example.yaml`](configs/autobump.example.yaml) to
`~/.config/autobump.yaml` and fill in what you need.
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

### Stale Branch Cleanup

Before creating the branch for a release, AutoBump deletes every remote branch carrying the bump prefix (`chore/bump-*`) and closes the pull request attached to each one — on Azure DevOps the pull request is abandoned. Without this, running AutoBump repeatedly on a repository nobody reviews leaves a trail of abandoned release branches and open pull requests.

Merged and unmerged branches are treated alike: a bump branch is disposable, because the branch needed for the current release is recreated immediately afterwards. Cleanup only runs once a bump is actually needed, so a pull request is never closed without a replacement being opened for it.

Cleanup is **enabled by default**. Turn it off permanently in the config:

```yaml
# Keep the bump branches from previous runs
cleanup_stale_branches: false

# Customise the branch prefix used for both creation and cleanup
bump_branch_prefix: 'chore/bump-'
```

Or disable it for a single run with `--skip-cleanup`:

```bash
autobump run --skip-cleanup
```

> **Note:** closing pull requests needs a token for the platform. Without one, AutoBump still deletes the branches and logs a warning that the pull requests were left open.

## Usage

AutoBump has two modes: a **single repository** (`autobump .`) and the **run** engine.

These flags work with every mode:

| Flag                | Description                                                                              |
|---------------------|------------------------------------------------------------------------------------------|
| `-c`, `--config`    | Path to the config file (auto-detected when omitted)                                     |
| `-v`, `--verbose`   | Enable verbose output                                                                    |
| `--skip-cleanup`    | Keep the bump branches from earlier runs instead of deleting them and closing their PRs  |

### 1. Single Repository

Process a single repository. Run in the project directory or name a path:

```bash
autobump .                    # Current directory
autobump /path/to/repo        # Specific path
```

> The `local` subcommand was removed in 3.0.0. `autobump local` still works, hidden and
> deprecated, so that the word is not silently read as a path -- but it warns and will go.

AutoBump will automatically detect the project language, update the version files, update the CHANGELOG.md file, and create a merge request/pull request on your Git platform (GitLab, Azure DevOps, or GitHub).

You can manually specify the project language using the `-l` or `--language` flag:

```bash
autobump -l java .
```

Available languages: `cs`, `dart`, `go`, `helm`, `java`, `python`, `terraform`, `typescript`

You can also specify a custom configuration file path:

```bash
autobump -c /path/to/custom/config.yaml .
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

## Per-Project Configuration (`.autobump.yaml`)

Drop a `.autobump.yaml` (or `.autobump.yml`, `autobump.yaml`, `autobump.yml`) at the root
of any repository to override settings for that project only. It is the last
[configuration layer](#configuration-layers): AutoBump finds it in every mode, so your own
configuration does not need to know anything about it, and what it sets wins over what you
configured for that project.

The keys a per-project file may set:

| Key | Purpose |
|-----|---------|
| `refresh` | `true` or `false` — see [Refresh](#refresh). A repository may turn its own refresh on: it is the party that knows whether a version bump leaves its lockfile stale |
| `changelog_path` | Changelog filename relative to the project root (e.g. `CHANGELOG_PROPRIETARY.md`) |
| `versioning` | `semver` (default), `fork-dot`, or `fork-dash` |
| `detect_chlog` | `false` to ignore [chlog](#fragment-based-changelogs-chlog) fragments (on by default) |
| `cleanup_stale_branches` | Only `false`, to keep this project's bump branches. An enable here would arrive after `--skip-cleanup` had been applied and would override it |
| `exclude_forks`, `exclude_archived` | Accepted, but they filter what *discovery selects*; by the time this file is read the repository has already been selected and cloned, so they do nothing here |
| `languages` | Per-language `extensions`, `special_patterns`, `version_files`, and `refresh` under the same rule |

And the keys it may **not**, which are reported and ignored when a repository sets one:

| Key | Why it is yours alone |
|-----|------------------------|
| `gitlab_access_token`, `github_access_token`, `azure_devops_access_token` | A repository handing AutoBump a credential — or a *path* to read one from — is a credential it chose |
| `gpg_key_path`, `gpg_key_passphrase`, `ssh_key_path`, `ssh_key_passphrase`, `ssh_auth_sock` | Same |
| `providers`, `projects` | Which repositories get released, and which servers are talked to, is not a repository's to decide |
| `bump_branch_prefix` | The prefix decides which branches stale-branch cleanup **deletes**, and whose pull requests it closes |

This is not a trust check that runs at the right moment — the three non-operator layers
decode through a struct that has no field for those keys, so there is nowhere for them to
land.

```yaml
# .autobump.yaml at the root of a fork repository
changelog_path: 'CHANGELOG_PROPRIETARY.md'
versioning: 'fork-dot'
```

## Refresh

AutoBump rewrites version files with regular expressions. It never runs a package manager,
so anything *derived* from a version file is left behind — and a lockfile is derived from
one. Bump the range a workspace package declares on its sibling and `yarn.lock` still
records the old resolution descriptor, at which point the first CI job running
`yarn install --immutable` rejects the very pull request the bump opened:

```
YN0028: The lockfile would have been modified by this install, which is explicitly forbidden.
```

`refresh` closes that gap. Turn it on and AutoBump regenerates the lockfile after the
version files are rewritten and before anything is committed, staging only the lockfile:

```yaml
refresh: true              # every language

languages:
  typescript:
    refresh: true          # or just this one
```

It can also be set on a `projects[]` entry, or in the repository's own `.autobump.yaml`.
The project wins over the language, which wins over the top level.

**AutoBump owns the command.** A configuration says *whether* to refresh, never *what to
run*: the recipes below are compile-time constants of the program. That is deliberate — an
argv read from configuration was an executable run with the release credentials, which
meant a repository's own file could never be trusted with it, which meant the project layer
could never simply override things the way every other layer does.

| Package manager | Detected by | Command | Stages |
|-----------------|-------------|---------|--------|
| Yarn Berry (2+) | `packageManager`, `.yarnrc.yml`, or a `yarn.lock` with `__metadata:` | `yarn install --mode=update-lockfile` (with `YARN_IGNORE_PATH=1`, `YARN_ENABLE_SCRIPTS=0`) | `yarn.lock` |
| npm | `packageManager` or `package-lock.json` | `npm install --package-lock-only --ignore-scripts` | `package-lock.json` |
| pnpm | `packageManager` or `pnpm-lock.yaml` | `pnpm install --lockfile-only --no-frozen-lockfile --ignore-scripts --ignore-pnpmfile` | `pnpm-lock.yaml` |

All three resolve without linking, so **no package lifecycle script runs** — and the flags
that say so are not decoration. npm has run the root package's `prepare` and `postinstall`
under `--package-lock-only` since npm 7. pnpm turns `frozen-lockfile` on by default in CI,
which is exactly where `run` mode lives, and a frozen install aborts on the out-of-date
lockfile the refresh exists to repair. Yarn needs no `--no-immutable`: `update-lockfile`
disables immutable installs itself from 3.2, and passing `--immutable` alongside it is an
error.

**Lifecycle scripts are not the only way a repository can supply code**, which is why two of
the recipes carry more than `--ignore-scripts`. pnpm loads `.pnpmfile.cjs` from the project
root and calls its hooks *during resolution* — `--ignore-pnpmfile` is what suppresses that.
Yarn's launcher honours `yarnPath` in the project's own `.yarnrc.yml` and execs the file it
names, which is checked-in JavaScript running with AutoBump's environment; `YARN_IGNORE_PATH`
is what stops it. Neither is a lifecycle script, so no `--ignore-scripts` reaches them.

Notes worth knowing before you turn it on:

- **Opt-in, and off by default.** The refresh starts a package manager, so upgrading
  AutoBump must never be what begins running programs on your machine. Nothing runs until
  some layer says `refresh: true`.
- **A repository may turn its own refresh on.** `refresh: true` in the project's own
  `.autobump.yaml` is honoured, because a lockfile that goes stale on a version bump is a
  fact about *that* repository's build, and the repository is the only party that reliably
  knows it. AutoBump still owns the argv — a configuration says *whether* to refresh, never
  *what to run*.
- **The fetched defaults may not.** `refresh: true` in the copy downloaded from
  `DefaultConfigURL` is warned about and ignored: it is a document that arrives over the
  network rather than one a repository committed, and that is a remote party choosing to
  start a process. Turning the refresh **off** is still honoured from any layer, since off
  can only ever remove an action.
- **You keep the veto, but you have to write it.** `refresh: false` in your own
  configuration is not merely a default a later layer replaces — it is recorded as an
  explicit refusal, and a repository's `refresh: true` cannot overturn it, at the top level
  or per language. The distinction is between saying no and saying nothing: omit the key
  and a repository may enable its own refresh, which is the point of the feature.
- **Read this before you add a `providers` block.** With `projects:` you name each
  repository, so the file that turns the refresh on was committed by people you chose to
  release. Discovery is not that: you name an org or group and AutoBump releases whatever
  the API returns, so *anyone who can land a four-line config file in any repository the
  provider can see* can start a package manager on your release host, with your
  environment. The recipes suppress `.pnpmfile.cjs`, `yarnPath` and lifecycle scripts, but
  a repository's own `.npmrc`/`.yarnrc.yml` is still read during resolution and can expand
  `${VAR}` out of that environment. If that is not a trust boundary you want, set
  `refresh: false` in your own configuration — the veto above — and turn it on per
  repository through `projects[]` instead.
- **Detection order matters.** A repository migrating between package managers carries two
  lockfiles, so `packageManager` is consulted first: it says which one is *current* rather
  than which one is *left over*.
- **Yarn Classic (1.x) is skipped, with a warning.** It has no install mode that resolves
  the lockfile without also linking and running install scripts, so there is no safe recipe
  for it. A `yarn.lock` alone does not mean Berry — the header is read, not the file name.
- **A missing package manager aborts that repository's release.** Skipping would open
  exactly the pull request the refresh prevents, and would look identical to a release that
  had nothing to refresh. In `run` mode the blast radius is one repository.
- **Only the lockfile is staged**, so a refresh cannot sweep unrelated work into the
  release commit — which matters when you release from your own worktree.
- **JavaScript is the only ecosystem with a recipe.** A lockfile only goes stale when the
  rewrite changes a string the lockfile keys on: `go.mod`/`go.sum` never record the
  module's own version, Python's version file is `{project_name}/__init__.py` while poetry
  and pdm exclude the root project, and `pom.xml`/`build.gradle`/`Chart.yaml`/`*.csproj`
  have no lockfile carrying it. `refresh: true` elsewhere warns and does nothing.
- **Fork versioning skips it**, because it rewrites no version file in the first place.
- Each command is given 10 minutes before it is killed, so a resolution hanging on an
  unreachable registry cannot stall every repository queued behind it. It runs in its own
  process group and the whole group is killed, so a shell's children go with it; on top of
  that, AutoBump stops reading output 10 seconds after the command itself exits, which
  bounds the call even when something it spawned still holds the pipe open.

### Migrating from 2.x

`refresh_commands` was removed. Replace the block with the flag:

```yaml
# before
languages:
  typescript:
    refresh_commands:
      - run: ['yarn', 'install', '--mode=update-lockfile']
        files: ['yarn.lock']

# after
languages:
  typescript:
    refresh: true
```

AutoBump says so by name if it finds the old key rather than reporting an unknown field.
Two other changes are worth knowing about: the `local` subcommand is gone (use
`autobump .`), and a repository's own `.autobump.yaml` now overrides the `projects[]` entry
you wrote for it rather than only filling in what that entry left empty.

## Fragment-Based Changelogs (`chlog`)

[chlog](https://github.com/luizjhonata/chlog) removes changelog merge conflicts by giving
every change its own YAML file under `.changes/unreleased/` instead of having developers
edit a shared `CHANGELOG.md`. That leaves the `[Unreleased]` section permanently empty,
which would otherwise make AutoBump conclude there is nothing to release.

AutoBump detects the layout and handles it automatically — no configuration, and no
`chlog` binary required on the runner. A repository counts as a chlog user when it has a
`.chlog.yaml` (or `.chlog.yml`) **or** a `.changes/unreleased/` directory.

When fragments are pending, AutoBump:

- reads every fragment and turns it into ordinary `### <Kind>` entries, so chlog's six
  default kinds (`Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, `Security`) land in
  the matching Keep a Changelog sections. A custom kind is filed under `### Changed`
  rather than being dropped
- honours `chlog new --breaking` by writing the entry as `**BREAKING CHANGE:** <body>`,
  which is what makes the release a major one. chlog stores the flag but never renders it,
  and AutoBump reads the bump off the rendered lines. A body that already opens with a
  breaking-change marker — in any spelling, including a doubled one — is **not** given a
  second: the marker is announced exactly once
- applies the [changelog rules](#changelog-rules) to the pending fragments as a set, which
  is where they matter most: every fragment is written alone, in its own file, so nothing
  else ever sees them side by side
- **merges** those entries with anything already written by hand in `[Unreleased]`, so
  nothing is lost while a repository is migrating to chlog
- computes the next version with its own SemVer rules, **not** chlog's `auto` mapping. An
  `Added` entry means a minor bump, a `**BREAKING CHANGE:**` entry means a major bump, and
  anything else means a patch — the same rules every other repository gets. A release may
  therefore differ from what `chlog batch auto` would have chosen, which caps major bumps
  below `1.0.0` and treats `Changed`/`Removed` as breaking
- honours the `changelogPath` declared in `.chlog.yaml`, unless `changelog_path` overrides it
- deletes the consumed fragments and stages the removals in the same commit that publishes
  their content, so the next run does not release them twice
- leaves a `.gitkeep` behind in `.changes/unreleased/` and stages it, because Git tracks
  files rather than directories: without it, the commit that removes the last fragment also
  removes the directory, and the next run clones a repository that no longer looks like a
  chlog user

If `chlog batch` has already produced a `.changes/v<version>.md` that was never merged,
AutoBump stops with an error instead of releasing on top of it: that file carries a version
chlog already decided. Run `chlog merge` (or delete the file) and try again.

Set `detect_chlog: false` — globally or per project — to switch the behaviour off.

## Changelog Rules

Every release normalises the `[Unreleased]` section before publishing it, whether its
entries were written by hand or compiled from [chlog](#fragment-based-changelogs-chlog)
fragments, and in every versioning mode:

| Rule                     | What it does                                                                                                                       |
|--------------------------|------------------------------------------------------------------------------------------------------------------------------------|
| **Heading repair**       | `#### added` and `### ADDED` become `### Added`, and a section opened twice is merged into one                                      |
| **Breaking-change form** | `BREAKING CHANGE:`, `**BREAKING CHANGE**:`, `BREAKING-CHANGE:` and a doubled marker all become one `**BREAKING CHANGE:**`           |
| **Deduplication**        | Identical entries collapse; entries whose significant words overlap almost entirely collapse into the fuller or higher-versioned one |
| **Reclassification**     | An entry is filed under the section its leading verb names, so `- removed the old flag` under `### Changed` moves to `### Removed`  |
| **Ordering**             | Sections come out in a fixed order and entries are sorted alphabetically inside each one                                            |

A multi-line entry moves as a unit: its continuation lines travel with the bullet they
explain rather than being judged, sorted, or reclassified on their own.

The rules only ever touch `[Unreleased]`. Released sections are history and are left
exactly as they are.

## Versioning Modes

AutoBump supports three versioning strategies, selectable via the `versioning`
key. All three honor the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
format; only the way the next version is computed differs.

| Mode        | Pattern   | Example transition       | When to use                                      |
|-------------|-----------|--------------------------|--------------------------------------------------|
| `semver`    | `X.Y.Z`   | `1.0.0` → `1.1.0`        | Default. Standard Semantic Versioning            |
| `fork-dot`  | `X.Y.Z.N` | `3.3.0.16` → `3.3.0.17`  | Forks following the [Forking Technique](https://github.com/rios0rios0/guide/wiki/Forking-Technique) with a 4th increment digit |
| `fork-dash` | `X.Y.Z-N` | `1.21.0-9` → `1.21.0-10` | Forks where CI/CD does not accept four-segment versions |

In fork modes, AutoBump:

- Reads the current version from the **last non-`Unreleased` header** in the changelog
- Increments **only the trailing fork digit** (`N`); the upstream `X.Y.Z` is preserved
- **Skips** language-specific version-file rewrites (forks typically maintain those manually or via separate pipelines)
- Still moves `[Unreleased]` content into a freshly dated `## [<next>] - YYYY-MM-DD` section

Example fork repository:

```yaml
# ~/.config/autobump.yaml
projects:
  - path: '~/Development/dev.azure.com/MyOrg/forks/opensearch-dashboards'
    changelog_path: 'CHANGELOG_PROPRIETARY.md'
    versioning: 'fork-dot'

  - path: '~/Development/dev.azure.com/MyOrg/forks/oui'
    changelog_path: 'CHANGELOG_PROPRIETARY.md'
    versioning: 'fork-dash'
```

Or, equivalently, place a `.autobump.yaml` in each fork's root:

```yaml
# .autobump.yaml inside the opensearch-dashboards fork
changelog_path: 'CHANGELOG_PROPRIETARY.md'
versioning: 'fork-dot'
```

## How It Works

1. **Repository Discovery** *(run mode with providers)*: Queries GitHub, GitLab, and Azure DevOps APIs to find all repositories in configured organizations
2. **Language Detection**: AutoBump automatically detects the project language by looking for specific files (e.g., `go.mod`, `package.json`, `pom.xml`)
3. **Version Detection**: Reads the current version from CHANGELOG.md
4. **Version Update**: Determines the next version based on Semantic Versioning and updates language-specific version files
5. **Refresh**: Regenerates the lockfile when [`refresh`](#refresh) is on, so it travels in the same commit
6. **CHANGELOG Update**: Folds in any pending [chlog](#fragment-based-changelogs-chlog) fragments, applies the [changelog rules](#changelog-rules), and moves the result to the new version section with the current date
7. **Git Operations**: Commits changes, creates a new branch, and pushes to remote
8. **MR/PR Creation**: Creates a merge request (GitLab), pull request (GitHub), or pull request (Azure DevOps) for review

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

See [LICENSE](LICENSE) for details.
