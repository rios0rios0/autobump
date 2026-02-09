# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

When a new release is proposed:

1. Create a new branch `bump/x.x.x` (this isn't a long-lived branch!!!);
2. The Unreleased section on `CHANGELOG.md` gets a version number and date;
3. Open a Pull Request with the bump version changes targeting the `main` branch;
4. When the Pull Request is merged, a new Git tag must be created using [GitHub environment](https://github.com/rios0rios0/autobump/tags).

Releases to productive environments should run from a tagged version.
Exceptions are acceptable depending on the circumstances (critical bug fixes that can be cherry-picked, etc.).

## [Unreleased]

### Changed

- upgraded Go version to `1.25.7` and all dependencies

## [2.17.1] - 2026-02-07

### Fixed

- fixed GitLeaks false positives on the documentation files

## [2.17.0] - 2026-02-07

### Added

- added testing for Azure DevOps package and functions

### Changed

- upgraded Go version to `1.25.6` and all dependencies

### Removed

- removed the file version YAML configuration from GoLang since it doesn't have it

## [2.16.0] - 2025-12-16

### Added

- added `PullRequestExists` method to Azure DevOps, GitLab, and GitHub adapters
- added a feature to check if PR exists when the branch already exists and create PR if missing

### Changed

- changed branch existence check to include remote branches (not just local)
- changed changelog processing to bump directly to `1.0.0` when no previous version exists (instead of calculating)
- changed version file handling to warn instead of error when no version files are found for detected language
- corrected lint issues within the repository

### Fixed

- fixed Azure DevOps URL parsing to correctly extract repository name from `/_git/{repo}` path
- fixed GPG key file handling that caused the "file already closed" error by reading the entire file into memory
- fixed context timeout in Azure DevOps API calls (was using nanoseconds instead of seconds)
- fixed shallow clone issue by removing `Depth: 1` option that caused "object not found" push errors

## [2.15.1] - 2025-12-16

### Changed

- corrected Azure DevOps cloning strategy to avoid errors while using tokens

## [2.15.0] - 2025-12-16

### Added

- added a feature to enable pull request creation using personal access tokens with GitHub
- added unit and integration tests for core components

### Changed

- updated code to satisfy `golangci-lint` linters and improve code style
- changed the configuration merging method to correctly merge nested fields and avoid unintended overwrites

### Fixed

- fixed to ensure changelog section ordering: `Fixed` now appears before `Removed`
- fixed to prevent nil pointer dereference when opening repositories
- fixed typo in authentication method selection

### Removed

- remove redundant release pipeline

## [2.14.0] - 2024-03-01

### Added

- added the feature to automatically fix incorrect section heading levels

## [2.13.0] - 2024-02-13

### Added

- added the workflow to have a complete pipeline with security and quality checks

### Changed

- changed to increment the version number only by one since it is the most common practice and eliminates discontinuity in the version numbers

### Fixed

- fixed the error where `-` characters are not replaced with `_` in the project names
- fixed the issue where some sections are left out in the `CHANGELOG.md` file

## [2.12.0] - 2024-01-25

### Added

- added the feature to read project names from the language's configuration file

### Changed

- refactored the project to remove warnings from golangci-lint

## [2.11.0] - 2024-01-23

### Added

- automatically try all authentication methods for Git cloning and pushing
- exit with an error code when batch processing fails

### Fixed

- fixed incorrect logic in the project processing phase that causes the program to quit early

## [2.9.1] - 2024-01-18

### Fixed

- downgraded `go-git` to v5.9.0 so the GitHub Actions pipeline can compile the program

## [2.9.0] - 2024-01-15

### Added

- created the feature to add the new version when the `CHANGELOG.md` doesn't have a previous version

### Changed

- changed the main method to create the `CHANGELOG.md` file if it doesn't exist
- corrected the breaking change prefix (it hasn't been detected before)
- made changelog processing adhere to keep a changelog version 1.1.0 standard
- upgraded all libraries to the latest version avoiding security issues

## [2.8.3] - 2023-10-03

### Changed

- corrected the issue with the regex for C# projects in the `.vdproj` files (final fix)

## [2.8.2] - 2023-10-03

### Changed

- corrected the issue with the regex for C# projects in the `.vdproj` files

## [2.8.1] - 2023-10-01

### Changed

- corrected the configuration merging without the `reflect` library

## [2.8.0] - 2023-10-01

### Added

- added Go support with a nonexistent version file (because Go doesn't have a version file)
- added support to Azure DevOps and support to have Azure DevOps token in a file
- added the `.editorconfig` file to handle the file formatting
- added the `CHANGELOG.md` file to clarify the releases
- added the feature to download the default configuration when the language detection is not present
- added the feature to read the GPG keys from the default keyring
- added the feature to read the configuration from the default repository URL

### Changed

- changed the `Makefile` to use build when the installation command is called
- changed the configuration file finding and reading to accept from the repository default configuration

### Removed

- removed the `Makefile` unnecessary `install` command

## [2.1.0] - 2023-09-06

### Added

- added feature to append the next version to bump branch name

### Changed

- fixed `CHANGELOG.md` patch number calculations

## [2.0.0] - 2023-09-05

### Added

- added the feature to avoid empty bumps

### Changed

- **BREAKING CHANGE**: changed to add support for C# and JavaScript/TypeScript

## [1.5.1] - 2023-08-09

### Changed

- fixed the errors in incrementing the version numbers

## [1.5.0] - 2023-08-04

### Added

- added support for per-file version patterns

### Changed

- fixed `CHANGELOG.md` key section orders

## [1.4.1] - 2023-08-03

### Changed

- fixed the error when a wrong version file path is given

## [1.4.0] - 2023-08-03

### Added

- added support for `DCO sign-off` to commit messages

## [1.3.0] - 2023-08-03

### Added

- added support for reading GitLab tokens from a file

## [1.2.1] - 2023-08-03

### Changed

- changed to assume an empty GPG key password when the interactive terminal is unavailable

## [1.2.0] - 2023-08-02

### Added

- added support for the project access token

## [1.1.1] - 2023-08-02

### Changed

- fixed the issue of not using `CI_JOB_TOKEN` while pushing over HTTPS

## [1.1.0] - 2023-07-31

### Added

- added the feature to load version files and patterns from the configuration

## [1.0.3] - 2023-06-30

### Changed

- fixed GitLab CI job token authentication

## [1.0.2] - 2023-06-30

### Changed

- fixed configuration argument for batch subcommand

## [1.0.1] - 2023-06-30

### Changed

- fixed configuration finder conflicting with the argument parser and corrected pipeline commands

## [1.0.0] - 2023-06-30

### Added

- added CI token reading, auto cloning, and a release pipeline for use in the scheduled pipeline
- added Java support and fixed package version update
- added configuration validation
- added documentation in the configuration file and `README.md`
- added language detection and batch processing mode
- added the Git commit and push functions
- added the feature to allow GPG signing
- added the feature to create GitLab Merge Request

### Changed

- changed to use `Cobra` to parse arguments
- fixed newline creation in `CHANGELOG.md`
- fixed the issue of getting the incorrect GitLab project ID
- fixed the skipping of signing if the signing format is SSH
- updated `.gitignore` from GitHub reference file
- updated configuration example to match the new parser
- updated the MR title to comply with naming standards

### Removed

- removed Git commit author overwrite
- removed unnecessary GitLab username and email definition
