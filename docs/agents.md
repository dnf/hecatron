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
| Which languages, where | `hct list` | `hct list --all`: every supported language |
| Is each toolchain working | `hct doctor [lang...]` | Read-only; see Installing |
| Validate config and generated files | `hct check` | |
| A language's environment in a shell | `eval "$(hct env LANG)"` | The PATH and variables its commands run with |

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
| `lint.format` | `false` (or `{"lang": false}`) turns off lint's format check |
| `audit.ignore` | Accepted vulnerabilities: `{"id", "reason", "until"}` each |
| `logs` | Log directory |

Change languages with `hct add LANG` / `hct remove LANG`, not by hand.

## Overriding how a language is built or tested

A script at `.hecatron/languages/<lang>/<action>.sh` (`test`, `lint`,
`build`, `clean`, `format`, `deps`, `audit`, `outdated`) replaces hecatron's
default for that language. It runs once per component, in the component's
directory, with `HECATRON_LANG`, `HECATRON_COMPONENT` and `HECATRON_ROOT`
set; exit non-zero to fail.

## Toolchain versions

Each language uses the version the project pins in its ecosystem's own file
(`.python-version`, `go.mod`'s `toolchain`, `rust-toolchain.toml`, `.nvmrc`,
`.java-version`, `.tool-versions`, `mise.toml`...), else `version` in
`.hecatron/languages/<lang>/config.json`, else hecatron's default, held back
to what the project says it supports, and to the releases of its last
commit's day when that is over a year old (`HECATRON_AS_OF=off` turns this
off). `hct doctor` says which version each language resolved to and why.
