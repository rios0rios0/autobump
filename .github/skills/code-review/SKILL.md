---
name: code-review
description: "Review pull requests and diffs in autobump — the Go CLI that releases Keep a Changelog entries and opens bump PRs across GitHub, GitLab and Azure DevOps — against the rios0rios0/guide standards: Clean Architecture with Uber Dig, the gitforge/langforge/cliforge boundaries, untagged unit tests, and chlog fragments. Use when reviewing a PR, a branch, or staged changes here."
---

# Code review — `autobump`

`autobump` moves `[Unreleased]` changelog entries into a versioned section following Keep a Changelog and SemVer, updates the language's version file, commits, pushes, and opens the merge request — on GitHub, GitLab, or Azure DevOps. Because it rewrites other repositories' changelogs and creates their releases, a defect here propagates everywhere it runs.

## When to use this skill

Use it whenever you are asked to review a pull request, a diff, a branch, or staged changes
in this repository — and before opening a pull request of your own, as a self-check. It is a
**review** skill: it produces findings, not commits.

## Source of truth

The canonical engineering standards live in the
**[rios0rios0/guide wiki](https://github.com/rios0rios0/guide/wiki)**. This file is a
repo-tailored index into that guide plus the rules that only apply here. Precedence, highest
first:

1. This repository's `.github/copilot-instructions.md`, `CLAUDE.md`, and `CONTRIBUTING.md` —
   they describe *this* codebase and its load-bearing invariants.
2. The **rios0rios0/guide** wiki — the shared standard.
3. General language idiom.

When the guide and a general convention disagree, the guide wins. When this file and the
guide disagree, the guide wins and this file should be corrected in the same pull request.

### Guide pages that apply here

| Topic | Page |
|-------|------|
| Go — overview and proverbs | [GoLang](https://github.com/rios0rios0/guide/wiki/GoLang) |
| Go Conventions — file, command, repository and mapper naming | [GoLang-Conventions](https://github.com/rios0rios0/guide/wiki/GoLang-Conventions) |
| Go Formatting and Linting — gofmt, goimports, golangci-lint | [GoLang-Formatting-and-Linting](https://github.com/rios0rios0/guide/wiki/GoLang-Formatting-and-Linting) |
| Go Type System — interface design and prohibited patterns | [GoLang-Type-System](https://github.com/rios0rios0/guide/wiki/GoLang-Type-System) |
| Go Logging — Logrus, structured fields | [GoLang-Logging](https://github.com/rios0rios0/guide/wiki/GoLang-Logging) |
| Go Testing — BDD, parallelism, build tags | [GoLang-Testing](https://github.com/rios0rios0/guide/wiki/GoLang-Testing) |
| Go Project Structure | [GoLang-Project-Structure](https://github.com/rios0rios0/guide/wiki/GoLang-Project-Structure) |
| YAML Conventions — `.yaml`, single quotes, unquoted scalars | [YAML](https://github.com/rios0rios0/guide/wiki/YAML) |
| Architecture — Clean Architecture and SOLID | [Architecture](https://github.com/rios0rios0/guide/wiki/Architecture) |
| Backend Design — layers, actors, dependency direction | [Backend-Design](https://github.com/rios0rios0/guide/wiki/Backend-Design) |
| Tests — BDD structure, description patterns, test doubles | [Tests](https://github.com/rios0rios0/guide/wiki/Tests) |
| Mapper Design Pattern — replacing `switch`/`case` | [Mapper-Design-Pattern](https://github.com/rios0rios0/guide/wiki/Mapper-Design-Pattern) |
| Git Flow — branches, commits, SemVer, breaking changes | [Git-Flow](https://github.com/rios0rios0/guide/wiki/Git-Flow) |
| Documentation & Change Control — changelog and docs discipline | [Documentation-&-Change-Control](https://github.com/rios0rios0/guide/wiki/Documentation-&-Change-Control) |
| CHANGELOG Formatting — capitalisation and backticks | [CHANGELOG-Formatting](https://github.com/rios0rios0/guide/wiki/CHANGELOG-Formatting) |
| Security — OWASP checklist, secret hygiene, SAST | [Security](https://github.com/rios0rios0/guide/wiki/Security) |
| CI & CD — pipeline stages and the local quality gates | [CI-&-CD](https://github.com/rios0rios0/guide/wiki/CI-&-CD) |
| Code Style — baseline naming and the operations vocabulary | [Code-Style](https://github.com/rios0rios0/guide/wiki/Code-Style) |

## How to run the review

1. **Establish the range.** Resolve the default branch with
   `git symbolic-ref refs/remotes/origin/HEAD` (strip `refs/remotes/origin/`; fall back to `main`),
   then read the diff with `git diff <default>...HEAD` and the file list with
   `git diff <default>...HEAD --name-only`.
2. **Read whole files, not just hunks.** A hunk cannot show a layering violation, a missing
   test, or a duplicated helper. Open every changed file in full, plus the files it imports
   from the layer below.
3. **Check the change set as a unit** — not only the code. A change that alters behaviour,
   configuration, or architecture is incomplete without its changelog entry and its
   documentation update, and that omission is a finding in its own right.
4. **Map every finding to a rule.** Each finding must name the rule it breaks and link the
   guide page (or the repository file) that states it. A comment that cannot be traced to a
   rule is a suggestion, not a defect — label it as such.
5. **Report, do not rewrite.** Produce the review in the output format below. Only edit files
   when the request explicitly asks for fixes.

## What matters most in `autobump`

These are the checks that catch real defects in this repository. Work through
them before the generic ones.

```
cmd/autobump/          main.go (Cobra wiring) + dig.go (DI injection helpers)
internal/
  app.go               AppInternal — aggregates controllers
  container.go         RegisterProviders — wires every Dig layer
  domain/
    commands/          ProcessRepo, IterateProjects, DiscoverAndProcess, fork_version.go,
                       chlog.go, cleanup.go, self_update*.go, version.go
    entities/          config and repository models
  infrastructure/      provider adapters and controllers
test/domain/entitybuilders/   testkit-based builders
```

- **Provider, language, and CLI abstractions are not re-implemented here.** Git hosting comes from [`gitforge`](https://github.com/rios0rios0/gitforge), language detection from [`langforge`](https://github.com/rios0rios0/langforge), self-update and version commands from [`cliforge`](https://github.com/rios0rios0/cliforge). A pull request that copies provider or ecosystem logic into `autobump` instead of extending the shared library is a Critical finding — the duplicate will drift.
- **Changelog manipulation is the product.** Any change to `chlog.go`, the fragment parser, or section insertion must keep both layouts working: the classic `## [Unreleased]` sections and the `.changes/unreleased/` fragment layout. Ask for a test covering the layout that was *not* the reason for the change.
- **Fork versioning has its own rules.** `fork_version.go` implements fork-dot and fork-dash modes (`1.0.0.1` / `1.0.0-1`) from the guide's [Forking Technique](https://github.com/rios0rios0/guide/wiki/Forking-Technique). The fourth digit resets to `0` on every rebase onto a new upstream version; a change that breaks that reset is a Critical finding.
- **SemVer decisions must stay honest.** Only a breaking change raises MAJOR. A change that lets a kind alone (for example `Removed`) drive a major bump contradicts both SemVer and the chlog contract.
- **Dig registration is bottom-up** in `internal/container.go` — repositories, then entities, then commands, then controllers. A provider registered out of order, or a concrete type resolved outside the container, is a finding.
- **Unit test files carry no build tag here.** That is deliberate: plain `go test ./...` must run them. Reserve `//go:build integration` for tests that need real infrastructure. A new `//go:build unit` line is a finding.
- Deprecated commands (`batch`, `discover`) stay as hidden aliases that warn; silently removing one is a breaking change and needs the three-place breaking-change flag.

### Commands a reviewer should be able to quote

```bash
make lint && make test && make build && ./bin/autobump --help
go test ./internal/...          # quick inner-loop check
make sast                       # full security gate before pushing
```

### Local quality gates

`make lint`, `make test`, and the SAST targets (`make sast`, or the per-tool targets such as
`make semgrep` / `make gitleaks` where this repository's Makefile defines those instead) are
the gate. They import the shared targets from
[rios0rios0/pipelines](https://github.com/rios0rios0/pipelines), which load the correct
configuration before invoking each tool — so **never invoke the tool binaries directly**
(`golangci-lint`, `pytest`, `eslint`, `semgrep`, `trivy`, `hadolint`, `gitleaks`). A change
that adds a direct binary invocation to a script or workflow, or that edits a tool
configuration to make a finding disappear, is itself a finding.

## Architecture and layering

See [Architecture](https://github.com/rios0rios0/guide/wiki/Architecture) and [Backend Design](https://github.com/rios0rios0/guide/wiki/Backend-Design).

Dependencies point **inward**, always. Infrastructure depends on the domain; the domain
never imports infrastructure, a framework, a driver, or an SDK. The layer boundary is the
first thing to check on any new file:

- An import of a concrete client, ORM, HTTP library, or cloud SDK from inside the domain
  layer is a **Critical** finding, no matter how convenient.
- Entities hold business rules and stay framework-agnostic — no serialisation tags, no ORM
  annotations, no transport concerns. Tags belong on DTOs in the infrastructure layer.
- New external dependencies enter through a port (an interface owned by the consumer) and an
  adapter that implements it. Code that reaches past the port to the concrete type breaks
  the pattern even when it compiles.
- Mappers isolate the layers: a repository mapper converts between models and entities, a
  controller mapper between requests/responses and entities. Leaking a transport DTO into
  the domain is a finding.
- Operation names come from the standard vocabulary: `List`, `Get`, `Insert`, `Update`,
  `Delete`, `BatchInsert`, `BatchUpdate`, `BatchDelete`, `DeleteAll`.

## Go conventions

See [Go Conventions](https://github.com/rios0rios0/guide/wiki/GoLang-Conventions), [Go Type System](https://github.com/rios0rios0/guide/wiki/GoLang-Type-System),
[Go Logging](https://github.com/rios0rios0/guide/wiki/GoLang-Logging), and
[Formatting and Linting](https://github.com/rios0rios0/guide/wiki/GoLang-Formatting-and-Linting).

- File names are `snake_case` — `list_users_command.go`, never `listUsersCommand.go`.
- Receivers are a one- or two-letter abbreviation of the type (`c` for `Command`, `r` for
  `Repository`, `m` for `Mapper`), **consistent across every method of that type**. Never
  `self`, `this`, or `me`.
- Naming patterns: `<Operation><Entity>Command` in `<operation>_<entity>_command.go` with a
  single `Execute`; `<Entity>Repository` in `<entity>_repository.go` for the port;
  `<Library><Entity>Repository` in `<library>_<entity>_repository.go` for the adapter.
  Repository methods read `FindByX`, `FindAllByX`, `HasX`, `Save`, `SaveAll`, `DeleteByX`.
- **Accept interfaces, return structs**, and keep interfaces small — a consumer that needs
  one method should not depend on a ten-method interface.
- Logging uses Logrus imported as `logger`, with structured fields
  (`logger.WithFields(...)`/`WithError(...)`). `fmt.Println`, `println`, and the standard
  `log` package are not application logging.
- Errors are wrapped with context: `fmt.Errorf("…: %w", err)`. Never discard an error with
  `_` without a comment explaining why it cannot matter.
- No `any` in exported signatures — use a constraint or a real interface. Every `//nolint`
  directive carries a justification comment.
- Entities carry no struct tags.

### Dispatch tables over `switch`

See [Mapper Design Pattern](https://github.com/rios0rios0/guide/wiki/Mapper-Design-Pattern). Two or three stable cases may stay a
`switch`. Four or more, or a set that grows with features, becomes a map from key to handler
so that adding a case is a new entry rather than an edit to the dispatcher. Flag new
`switch`/`if-else` chains that dispatch on a string or enum key.

### YAML

See [YAML Conventions](https://github.com/rios0rios0/guide/wiki/YAML). The extension is `.yaml`, never `.yml`. String values are
single-quoted; double quotes appear only where interpolation or an escape needs them;
booleans and numbers are never quoted. This applies to workflows, compose files, manifests,
and YAML blocks inside Markdown.

## Tests

See [Tests](https://github.com/rios0rios0/guide/wiki/Tests).

- Tests are co-located with the source as `_test.go`, with `export_test.go` exposing unexported functions for white-box coverage where needed.
- `t.Parallel()` at both parent and subtest level, except where the test mutates package globals, swaps `os.Stdout`, or calls `t.Setenv` — those carry a comment explaining why they are serial.
- Test data comes from the testkit builders in `test/domain/entitybuilders/` (`GlobalConfig`, `ProjectConfig`, `ProviderConfig`, `Repository`), not from repeated literals.
- `testify/assert` and `testify/require` for assertions — never bare `t.Error`.

### Rules that hold for every test in this repository

- **BDD blocks are mandatory.** Every test body carries `// given` / `// when` / `// then`
  (`# given` / `# when` / `# then` in Python) delimiting preconditions, the action, and the
  assertions. A test without them is a finding.
- **Descriptions follow the layer.** Commands: `"should call <listener> when …"`.
  Controllers: `"should respond <HTTP_STATUS> when …"`. Services and repositories:
  `"should … when …"`, with at least one success and one failure case per public method.
- **Mock libraries are banned** — the category, not just the names: `testify/mock`,
  `golang/mock`, `mockery`, `go-sqlmock`, `httpmock`, `gock`, Mockito, EasyMock, PowerMock,
  `unittest.mock`, `pytest-mock`, `responses`, `requests-mock`, `jest.mock()` module mocking,
  `sinon`, `nock`, `testdouble`. Doubles are hand-rolled: stubs return canned answers,
  in-memory implementations hold state, and a spy that records calls is used only when
  nothing observable exists. Reaching for a mock library almost always means a port is
  missing — extract the interface the consumer needs instead.
- **Driver-level and transport-level mocks are the worst case.** SQL and HTTP behaviour is
  proven against the real thing: a real database with the real migrations for stores, a real
  local test server for outbound clients. Those are not mocks; they are real implementations
  under test control.
- **Builders construct test data.** Complex entities and DTOs come from builders, not from
  long literal structs repeated across files.
- A test deleted, skipped, or weakened to make a change pass is a Critical finding.

## Documentation and change control

See [Documentation & Change Control](https://github.com/rios0rios0/guide/wiki/Documentation-&-Change-Control) and
[CHANGELOG Formatting](https://github.com/rios0rios0/guide/wiki/CHANGELOG-Formatting).

This repository uses **chlog fragments**. `CHANGELOG.md` is generated and is never edited by
hand.

- Every change ships a fragment created with `chlog new --kind <Kind> --body "…"`, staged in
  the **same commit** as the code. Kinds: `Added`, `Changed`, `Deprecated`, `Removed`,
  `Fixed`, `Security`.
- A backward-incompatible change to the public interface additionally carries `--breaking`.
  The kind alone never triggers a major bump.
- A hand-edited `CHANGELOG.md`, or a code change with no fragment under
  `.changes/unreleased/`, is a **Critical** finding — `chlog check` fails the build for it.
- Fragment bodies start with a lowercase verb in simple past tense, capitalise proper nouns
  (GitHub, Go, Docker), and wrap code identifiers and versions in backticks.
- `README.md` is updated whenever usage, setup, configuration, or architecture changes;
  `.github/copilot-instructions.md` and `CLAUDE.md` whenever the workflow, commands, or
  structure changes. Documentation and code ship in one commit.

## Git Flow and pull-request hygiene

See [Git Flow](https://github.com/rios0rios0/guide/wiki/Git-Flow) and [Merge Guide](https://github.com/rios0rios0/guide/wiki/Merge-Guide).

- Branch names are `feat/`, `fix/`, `refactor/`, `chore/`, `test/`, or `docs/` followed by a
  ticket ID or a short slug — `feat/TICKET-000`, `fix/input-mask`.
- Commit subjects are `type(SCOPE): message`: simple past tense (`added`, `fixed`, `changed`,
  `removed`), lowercase first word, no trailing period, code identifiers in backticks.
- Branches are synchronised with `git rebase`, never `git merge`. A merge commit from the
  default branch inside a feature branch is a finding.
- Breaking changes are flagged in **three** places: the commit footer
  (`**BREAKING CHANGE:** …`), the changelog, and the pull-request description. One or two of
  the three is not enough.
- Versions follow [SemVer](https://semver.org/): MAJOR for incompatible changes, MINOR for
  features, PATCH for fixes.

## Security

See [Security](https://github.com/rios0rios0/guide/wiki/Security).

- **No hard-coded secrets.** API keys, tokens, passwords, and private keys belong in
  environment variables or a secret manager — never in source, tests, fixtures, or the
  changelog. A secret that reaches a commit must be rotated, not merely deleted.
- **Never write a PEM header sentinel or a realistic key shape into a fixture** (a GitHub,
  OpenAI, AWS, or Slack token prefix, JWT-shaped strings, or the dashed `BEGIN …` banners).
  Gitleaks matches the shape, not the value, so a placeholder that merely *looks* like a
  credential fails the pipeline — spelling those prefixes out here would trip it too. Use
  inert placeholders such as `fixture-token-placeholder`.
- **Suppressions must be justified.** Entries in `.gitleaksignore`, `.trivyignore`,
  `.semgrepignore`, or `.codeql-false-positives` need a fingerprint, a dated comment, and a
  reason. A suppression added to silence a real finding is a Critical.
- Validate and sanitise every external input; use parameterised queries; apply least
  privilege; keep secrets out of logs.
- Dependency manifest changes are reviewed for new transitive vulnerabilities. When a fix
  exists, bump the version rather than suppressing the finding.

## What not to flag

A review that raises noise gets ignored. Do not report these:

- The `//nolint` directives that already carry a justification comment.
- Generated coverage and report artefacts in the working tree.
- Anything the guide does not require and this file does not list, unless it is a genuine correctness or security defect — say so plainly and label it a Suggestion.

## Review output format

```
## Code review: <branch or PR>

### Critical (must fix before merge)
- `path/to/file.ext:LINE` — <what is wrong> — violates <rule> (<guide page or repo file>)

### Warning (should fix)
- `path/to/file.ext:LINE` — <what is wrong> — violates <rule>

### Suggestion (optional)
- `path/to/file.ext:LINE` — <improvement>

### Change-control checklist
- [ ] Changelog entry present for every behavioural change
- [ ] `README.md` updated if usage, setup, or architecture changed
- [ ] `.github/copilot-instructions.md` and `CLAUDE.md` updated if the workflow, commands, or structure changed
- [ ] Commit messages follow `type(SCOPE): message` in simple past tense
- [ ] Breaking changes flagged in the commit footer, the changelog, and the PR description

### Verdict: APPROVE / REQUEST CHANGES
<one paragraph: the blocking findings, or why the change is ready>
```

## Severity

| Severity       | Use for                                                                                                                            |
|----------------|------------------------------------------------------------------------------------------------------------------------------------|
| **Critical**   | Broken dependency direction, a leaked secret, an injection or authentication flaw, a missing changelog entry, a banned mock library, a load-bearing invariant broken, a test deleted rather than fixed. |
| **Warning**    | Naming that departs from the guide, a missing test for a new branch of logic, an unexplained magic value, a stale README or instructions file, a `switch` that should be a map. |
| **Suggestion** | Readability, consistency with neighbouring modules, and performance ideas that no rule mandates.                                     |

Rank findings most severe first, and state plainly when nothing blocks the merge — an empty
Critical section is a valid, useful review.
