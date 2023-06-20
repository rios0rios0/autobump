# AutoBump

Automatically update CHANGELOG.md according to the [Keep a Changelog (version 1.1.0)](https://keepachangelog.com/en/1.1.0/) standard and the [Semantic Versioning (version 2.0.0)](https://semver.org/spec/v2.0.0.html) standard, commit the changes, push the commits, and create a merge request on GitLab.

## Installation

At the moment, AutoBump doesn't have binary releases. You will need to compile it yourself. Make sure you have Golang and make installed, then use the following command to create the binary:

```bash
make build
```

Run this to install it:

```bash
make install
```

## Usages

Create a configuration file based on the example from `configs/autobump.yaml`, then run AutoBump:

```bash
autobump -c /path/to/config.yaml
```
