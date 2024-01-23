# AutoBump

Automatically update CHANGELOG.md according to the [Keep a Changelog (version 1.1.0)](https://keepachangelog.com/en/1.1.0/) standard and the [Semantic Versioning (version 2.0.0)](https://semver.org/spec/v2.0.0.html) standard,
commit the changes, push the commits, and create a merge request/pull request on GitLab/Azure DevOps.

## Installation

AutoBump has binary releases in the [releases section](https://github.com/rios0rios0/autobump/releases).
But if you'd like to compile it by yourself, make sure you have Go and Make installed, then use the following command to create the binary:

```bash
make build
```

Run this to install it:

```bash
make install
```

## Usages

Create a configuration file based on the example from `configs/autobump.yaml` and put it in `~/.config/autobump.yaml`.
You will need to at least update your credentials:
- GitLab token `gitlab_access_token` field;
- or Azure DevOps equivalent `azure_devops_access_token`;

There are two ways to run AutoBump: for the current project and for multiple projects.

### 1. For the Current Project

Simply run this command in the project directory. AutoBump will automatically detect the project language, update the version, the CHANGELOG.md file, and create a GitLab MR.

```bash
autobump
```

You can also overwrite the project language via the `-l`, `--language` flag:

```bash
autobump -l java
```

### 2. For Multiple Projects

Modify the configuration file and add a list of your projects into the `projects` section:

```yaml
projects:
  - path: "/home/user/repo1"
    # language can be auto-detected
  - path: "/home/user/repo2"
    # language can also be manually specified
    language: "Java"
```

Then run AutoBump in batch mode:

```bash
autobump batch
```

AutoBump will now go through each of the projects and perform the same actions as with a single project.
