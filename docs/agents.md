# hecatron for AI agents

A compact reference for coding agents working in a repository that uses
hecatron. `hct help agents` prints this file for the installed version.
People: see the [README](../README.md).

hecatron runs every language's toolchain through the same commands. It calls
each language's own tools (cargo, go, gradle, uv, npm...); it has no build
system of its own. `hct` and `hecatron` are the same command.

## Commands

| Goal | Command | Notes |
| --- | --- | --- |
| Run tests | `hct test [lang...]` | No language: all, in parallel |
| Lint | `hct lint [lang...]` | Includes each language's format check |
| Fix formatting | `hct format [lang...]` | `--check` changes nothing and lists unformatted files |
| Build / clean | `hct build`, `hct clean` | `rebuild`, `retest`: clean first |
| Fetch dependencies | `hct deps [lang...]` | Held to the lock file; a stale lock fails |
| Known vulnerabilities | `hct audit [lang...]` | `--fail-on critical\|high\|moderate\|low` (default high) |
| Newer dependency versions | `hct outdated [lang...]` | Report only; `--json`, `--fail` |
| API docs | `hct docs [lang...]` | The ecosystem's tool (`cargo doc`, Sphinx, Javadoc...) into its own directory; ends with `<lang>: docs: written to <dir>` (JSON: `"docs"`). Only tools the project already has; none: skipped |
| Benchmarks | `hct bench [lang...]` | `cargo bench`, `go test -bench`, pytest-benchmark, JMH, BenchmarkDotNet...; none: skipped. Not part of `hct test` or CI |
| Dependency licenses | `hct licenses [lang...]` | Per license (expression and identifier) and package, each with its copyleft class (`none`, `weak`, `strong`, `network`, `maximal`, `unclassified`); unknowns and the project's own packages apart; exit 1 when the config's `licenses` policy refuses one; `--timeout DURATION`; needs OSV-Scanner and the binary |
| Software bill of materials | `hct sbom [lang...] [-o FILE]` | CycloneDX 1.6 JSON (`--format spdx`: SPDX 2.3); `--no-dev`; `SOURCE_DATE_EPOCH` for reproducible output; needs OSV-Scanner and the binary |
| Where to start | `HECATRON_OUTPUT=agent hct status` | Each language's directories, toolchain version and whether it's installed, last runs, uncommitted changes, and what to run next; one `git status`, installs nothing |
| After a pull, rebase or checkout | `hct sync [ref]` | Since where HEAD was (the reflog) or `ref`: runs `hct deps` where lock files changed (`--no-deps`: says so), says the version each changed pin now needs and whether it's installed (installs nothing), runs `hct check` when the config, hooks or generated files changed; never pulls or fetches; exit 1 when deps or check fails. Needs the binary |
| What the ecosystems recommend | `hct advisor [lang...]` | Lock files, toolchain pins and end of life, linter config, CI coverage; each with why and a link. Reads files only; `--fix` adds pin files; `--fail-on recommended` |
| Which languages, where | `hct list` | `hct list --all`: every supported language |
| Is each toolchain working | `hct doctor [lang...]` | Read-only; see Installing |
| Validate config and generated files | `hct check` | |
| A language's environment in a shell | `eval "$(hct env LANG)"` | The PATH and variables its commands run with |
| Run one command with a component's toolchain | `hct exec LANG [-c COMPONENT] -- CMD...` | That environment, in the working directory; exit status is CMD's; needs the binary |
| A newer hecatron | `hct self update --check` | Exit 3: a newer release exists (`hct status` says so from its daily check). `hct self update` installs it: ask the user first, as a new version can change generated files |

- `--changed [REF]` (`test`, `lint`, `build`, `format`, `deps`): only the
  languages and components whose files changed since the merge base with
  `REF` (default: the default branch; in CI, the pull request's base),
  uncommitted and untracked files included. Its first line says what it
  compared, what runs and why; nothing changed that hecatron runs: that
  line, exit 0. Needs the binary.
- Targets: language names or abbreviations (`go`, `rs`, `ts`), or a
  project task named in config.json `tasks`.
- `-c COMPONENT` (repeatable): one component only. `-j N`: concurrency.
- Run one language at a time while iterating (`hct test go`): its output
  streams, and the run is shorter.
- `hct <command> help` gives a command's flags.

## Results

- Exit status 0: passed. 1: something failed. 2: a usage error.
- Each run saves a log per target: `<logs>/<action>/<target>.log`, where
  `<logs>` is config.json `logs` (default `.logs`), or `.git/hecatron/logs`
  in a repository without `.hecatron/config.json`.
- A failed language's log holds the tool's own output; read its end first.
- With the Go binary (`go build -o hct ./cmd/hecatron` in a hecatron
  checkout; `bin/hecatron` doesn't do this yet), the run commands also
  print output for agents: `HECATRON_OUTPUT=agent` (or `--output agent`,
  and by default when `CLAUDECODE=1` is set and stdout isn't a terminal)
  gives one summary line, then each failure with its log and its error
  lines, then a `Rerun:` command; a passing run is one line.
  `--output json` gives one JSON document (`--output junit`, and `sarif`
  for lint and audit, give those formats). Its exit status 3 means a
  failure whose language command (`go`, `cargo`...) isn't on PATH:
  install the toolchain (ask first, below).
- A job that runs far longer than it usually does is stopped and reported
  as timed out (`TIMEOUT` in agent output), with its limit: by default 5×
  its usual time, at least 10 minutes. `--timeout 30m` (or
  `HECATRON_TIMEOUT`) sets one for every job; `--timeout 0` none.
- "no <action> command for <dir>": the default hook found no build file it
  knows there; write a hook (below).

## Installing toolchains: ask first

`hct install [lang...]` and `hct doctor --fix` install or repair toolchains
on this machine (system packages, files in the home directory). Ask the
user before running them. `hct doctor` without `--fix` only checks, and is
safe.

## Project configuration

`.hecatron/config.json`, committed. Without it, hecatron detects the
languages and layout as `hct init` would and writes nothing; `hct init`
saves that.

| Key | Meaning |
| --- | --- |
| `languages` | Languages the project has |
| `components` | Parts of the project, each implemented per language (default one, `.`) |
| `implPath` | Where a component's code in a language is: `{component}`, `{lang}` placeholders |
| `implPaths` | One language's own path (`{"csharp": "dotnet"}`) |
| `tasks` | Extra jobs per action, run from the root: `{"test": {"codegen": "make gen-test"}}` |
| `taskPaths` | Paths a task depends on, for `--changed`: `{"codegen": ["codegen/", "*.proto"]}` |
| `lint.format` | `false` (or `{"lang": false}`) turns off lint's format check |
| `audit.ignore` | Accepted vulnerabilities: `{"id", "reason", "until"}` each |
| `advisor.ignore` | Silenced advisor rules: `{"id", "reason"}` each, optional `"path"` |
| `licenses` | License policy for `hct licenses`: `allow`, `deny` (SPDX IDs), `ignore` (`{"package": "npm:name", "reason", "until"}`), `includeDev` |
| `logs` | Log directory |

Change languages with `hct add LANG` / `hct remove LANG`, not by hand.

## Overriding how a language is built or tested

A script at `.hecatron/languages/<lang>/<action>.sh` (`test`, `lint`,
`build`, `clean`, `format`, `deps`, `audit`, `outdated`, `docs`,
`bench`) replaces hecatron's default for that language. It runs once per
component, in the component's directory, with `HECATRON_LANG`,
`HECATRON_COMPONENT` and `HECATRON_ROOT` set; exit non-zero to fail.

## Toolchain versions

Each language uses the version the project pins in its ecosystem's own file
(`.python-version`, `go.mod`'s `toolchain`, `rust-toolchain.toml`, `.nvmrc`,
`.java-version`, `.tool-versions`, `mise.toml`...), else `version` in
`.hecatron/languages/<lang>/config.json`, else hecatron's default, held back
to what the project says it supports, and to the releases of its last
commit's day when that is over a year old (`HECATRON_AS_OF=off` turns this
off). `hct doctor` says which version each language resolved to and why.
