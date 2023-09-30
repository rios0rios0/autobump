# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

When a new release is proposed:

1. Create a new branch `bump/x.x.x` (this isn't a long-lived branch!!!);
2. The Unreleased section on `CHANGELOG.md` gets a version number and date;
3. Open a Pull Request with the bump version changes targeting the `main` branch;
4. When the Pull Request is merged, a new git tag must be created using [GitHub environment](https://github.com/rios0rios0/autobump/tags).

Releases to productive environments should run from a tagged version.
Exceptions are acceptable depending on the circumstances (critical bug fixes that can be cherry-picked, etc.).

## [Unreleased]

### Added

- added the `.editorconfig` file to handle the files formatting
- added the `CHANGELOG.md` file to make the releases clearer

### Changed

- changed the `Makefile` to use build when the installation command is called
- changed the configuration file finding and reading to accept from the repository default configuration

### Removed

## [2.1.0] - 2023-09-06

### Added

- append next version to bump branch name

### Changed

- fixed changelog patch number calculations

## [2.0.0] - 2023-09-05

### Added

- added the feature to avoid empty bumps
- added support for C# and JavaScript/TypeScript

## [1.5.1] - 2023-08-09

### Changed

- fixed the errors in incrementing the version numbers

## [1.5.0] - 2023-08-04

### Added

- fixed CHANGELOG.md key section orders; added support for per-file version patterns

## [1.4.1] - 2023-08-03

### Changed

- fixed an error the wrong version file path is checked

## [1.4.0] - 2023-08-03

### Added

- added DCO sign-off to commit messages

## [1.3.0] - 2023-08-03

### Added

- added support for reading GitLab token from file

## [1.2.1] - 2023-08-03

### Changed

- assume empty GPG key password when interactive terminal unavailable

## [1.2.0] - 2023-08-02

### Added

- added support for project access token; refactored code

## [1.1.1] - 2023-08-02

### Changed

- fixed the issue of not using CI_JOB_TOKEN while pushing over HTTPS

## [1.1.0] - 2023-07-31

### Added

- added the feature to load version files and patterns from config

## [1.0.3] - 2023-06-30

### Changed

- fixed GitLab CI job token authentication

## [1.0.2] - 2023-06-30

### Changed

- fixed config arg for batch subcommand

## [1.0.1] - 2023-06-30

### Changed

- fixed config finder conflicting with arg parser, corrected pipeline commands

## [1.0.0] - 2023-06-30

### Added

added CI token reading, auto cloning, and a release pipeline for use in the scheduled pipeline
added language detection and batch processing mode
added config validation
added the feature to allow GPG signing
added Java support, fixed package version update
added files from previous repo; added license, README, etc.
completed the create GitLab MR feature
completed the Git commit&push functions
added PR templates

### Changed

updated the MR title to comply with naming standards
fix(gitlab): fixed the issue of it getting the incorrect GitLab project ID
chore(doc): added docs in the config file and README
fix(ssh): skip signing if signing format is SSH
formatted code with `gofumpt`
updated example config to match new parser
fixed newline creation in CHANGELOG
using cobra to parse args; split files
fixed Makefile
updated gitignore from GitHub reference file, added Nvim Session.vim ignore

### Removed

removed unnecessary GitLab username and email definition
removed git commit author overwrite
