# Command reference

Every command is `hct <command>` (`hct` is short for `hecatron`). `hct help`
lists them, and `hct <command> help` (or `--help`) prints a command's
options. Commands find the project by walking up from the working directory,
like git; all but `init`, `banner`, `help`, `--version`, `self` and `skill`
need one.

Language names take abbreviations (`ts`, `rs`, `py`, `c++`) and a typo
gets the nearest name. Commands marked **binary only** need hecatron's Go
binary, which [install.sh](installing-hecatron.md) downloads with a release;
run through the scripts alone (a checkout on a branch, `HECATRON_NO_GO=1`),
they say so.

**Exit status:** 0 when everything run passed, 1 when anything failed, 2
for a usage error, and, for the run commands with the binary, 3 when the
only failures were toolchains not installed ([output](output.md)). A
language whose toolchain is missing is reported as "not installed" and
doesn't fail a parallel run.

- [Run commands](#run-commands): `test`, `lint`, `build`, `clean`,
  `rebuild`, `retest`, `deps`, `format`, `audit`, `outdated`, `docs`, `bench`
- [Dependency reports](#dependency-reports): `licenses`, `sbom`
- [The project at a glance](#the-project-at-a-glance): `status`, `sync`, `advisor`
- [Setting up](#setting-up): `init`, `doctor`, `install`, `uninstall`, `get-started`, `bootstrap`
- [Languages](#languages): `list`, `add`, `remove`, `enable`, `disable`
- [Generated files](#generated-files): `dockerfile`, `workflows`, `check`, `banner`
- [The project's own scripts](#the-projects-own-scripts): `env`, `exec`, `parallel`, `version`, `metadata`
- [Hecatron itself](#hecatron-itself): `self`, `skill`, `help`, `--version`

## Run commands

```
hct test|lint|build|clean|rebuild|retest|deps|format|audit|outdated|docs|bench [flags] [target ...]
```

Each runs the language's hook for the action ([hooks](hooks.md)) once for
every component. A target is a language, a project task (config.json's
`tasks`), or, for `lint`, `shell` (ShellCheck on every tracked shell
script). With no targets, every language and task with a hook for the
action runs in parallel, with a live display on a terminal
([runner](runner.md)); named targets run one after the other, their output
streamed. Each job's output is saved to `<logs>/<action>/<target>.log`.

| Command | What it does |
| --- | --- |
| `test` | Run each language's tests |
| `lint` | Run each language's linter, then its format check ([format](format.md#the-check-in-hct-lint)); languages without a lint hook rely on warnings-as-errors builds |
| `build` | Build each language |
| `clean` | Remove build artifacts (never installed dependencies or tracked files) |
| `rebuild`, `retest` | Clean, then build or test |
| `deps` | Fetch dependencies with each ecosystem's tool, held to the lock file |
| `format` | Rewrite the code with each language's formatter ([format](format.md)) |
| `audit` | Check dependencies for known vulnerabilities, and the files for committed secrets ([audit](audit.md)) |
| `outdated` | Report direct dependencies with a newer version; passes unless a tool fails ([outdated](outdated.md)) |
| `docs` | Build API documentation with each ecosystem's tool, into the tool's own directory, and say where |
| `bench` | Run benchmarks with each ecosystem's harness; the numbers are the tools' own output |

Which languages an action covers follows from their hooks: a language with
a `build.sh` (the project's or the default) is built, one with a `lint.sh`
is linted, and so on.

`docs` and `bench` run only the tools a component already has: those in the
toolchain, or in its dependencies or configuration (`cargo doc`, Sphinx
with a `docs/conf.py`, TypeDoc as a dependency; `cargo bench`, `go test
-bench`, pytest-benchmark, JMH, BenchmarkDotNet). `hct install` installs
none. A language with no default (Go has no docs generator in its
toolchain) says "No docs tool for go (skipping)", and a component without
the tool or without benchmarks skips, saying why; neither fails. `hct docs`
ends with where each language's docs were written. Neither is part of `hct
check`, generated CI or `hct test`. Each language's defaults:
[languages](languages.md#api-docs-and-benchmarks).

Flags every run command takes:

| Flag | Meaning |
| --- | --- |
| `-j VALUE` | Jobs at once: a number, a share of the CPUs (`50%`, the default), `pcores` or `unlimited` ([runner](runner.md#how-many-jobs-run-at-once)) |
| `-s MODE` | The live display's time scale: `scroll` (default), `full` or `zoom` |
| `-c NAME` | Only this component (repeatable) |
| `--output FORMAT` | `human`, `agent`, `json`, `ci`, `junit` or `sarif`; else `HECATRON_OUTPUT`, else chosen for whoever is calling ([output](output.md)). Binary only |
| `--timeout DURATION` | Stop a job after `DURATION` (`90s`, `30m`, `1h30m`; `0`: no limit); else `HECATRON_TIMEOUT`, else 5× its usual time ([runner](runner.md#time-limits)). Binary only |
| `--changed [REF]` | Only what the files changed since `REF` touch; not for `audit`, `outdated` or `clean` ([runner](runner.md#only-what-changed)). Binary only |

Flags of one command:

| Flag | Command | Meaning |
| --- | --- | --- |
| `--check` | `format` | Change nothing; fail, listing the files the formatter would change |
| `--fail-on SEVERITY` | `audit` | Fail on a finding at or above `critical`, `high` (default), `moderate` or `low`; the project's own is `"audit": {"failOn": ...}` |
| `--json` | `outdated` | Print one JSON array of every outdated dependency (component, language, package, current, wanted, latest); the languages run one at a time |
| `--fail` | `outdated` | Fail when anything is outdated |

```bash
hct test                  # everything, in parallel
hct test go rs            # Go and Rust, every component, one after the other
hct test go -c age        # Go, the age component only
hct lint shell            # ShellCheck on every tracked shell script
hct test codegen          # a project task
hct format --check        # what the formatters would change
hct audit --fail-on critical
hct audit secrets --history   # only the secret scan, over every commit
hct outdated --json py
hct docs rs               # cargo doc --no-deps; says where (target/doc)
hct bench go              # go test -run '^$' -bench . ./...
hct test --changed        # only what this branch changed
hct test --output agent   # a summary line and only the failures
```

## Dependency reports

### licenses

```
hct licenses [lang...] [-c COMPONENT] [--output human|agent|json] [--json]
```

Every dependency's license, in every component of each language (those
named, or all the project's), from the lock files, rolled up per SPDX
license. With a `"licenses"` policy in `.hecatron/config.json` (`allow`,
`deny`, `ignore`, `includeDev`), a package the policy doesn't permit fails
the command. Needs OSV-Scanner and the network. Binary only. See
[licenses](licenses.md).

```bash
hct licenses              # every language
hct licenses go --json
```

### sbom

```
hct sbom [lang...] [-c COMPONENT] [--format cyclonedx|spdx] [-o FILE] [--no-dev]
```

A software bill of materials for every component of each language (those
named, or all the project's), from what `hct licenses` reads: each
package's purl, SPDX license where known, direct or transitive, dev
dependencies marked. CycloneDX 1.6 JSON by default, or SPDX 2.3 JSON;
`SOURCE_DATE_EPOCH` makes it reproducible. Needs OSV-Scanner and the
network. Binary only. See [sbom](sbom.md).

```bash
hct sbom -o bom.cdx.json
hct sbom --format spdx --no-dev > sbom.spdx.json
```

## The project at a glance

### status

```
hct status [--output human|agent|json]
```

Hecatron's version (and a newer release, when the last daily check found
one), the project (its config saved or only detected), and per language
its directories, the toolchain version it resolves to and why, whether that
toolchain is installed, the last run of each action, and its uncommitted
changes; then the commands worth running next. Runs one `git status` and
nothing that installs. Binary only.

### sync

```
hct sync [REF] [--no-deps] [--output human|agent|json]
```

After a pull, rebase or checkout: what changed since `REF` (default: where
`HEAD` was before) means for hecatron. Changed lock files run `hct deps` for
those components (`--no-deps` only says so); changed pin or support files
say the toolchain version now needed and whether it's installed, never
installing; changed configuration, hooks or generated files run `hct
check`. Never pulls or fetches. Exit status 1 when `hct deps` or `hct check`
fails. Binary only. See [runner](runner.md#after-a-pull-hct-sync).

### advisor

```
hct advisor [--output human|agent|json] [--fail-on recommended|suggestion] [--fix] [lang...]
```

Recommends what each language's ecosystem considers good practice: lock
files committed, toolchains pinned and supported, formatter and linter
configured, CI covering each language, and more, each with why and a link
to the ecosystem's documentation. Reads files only.

| Flag | Meaning |
| --- | --- |
| `--fix` | Make the safe, mechanical changes (a toolchain pin file at the version in use), listing each; never edits code |
| `--fail-on LEVEL` | Exit 1 if there is a finding at `LEVEL` or above |
| `--output FORMAT` | As for the run commands |

Binary only. Rules and silencing them: [advisor](advisor.md).

## Setting up

### init

```
hct init [dir] [--components a,b] [--impl-path TEMPLATE] [--languages a,b]
         [--justfile] [--force] [--no-agents]
```

Saves the project's configuration as `.hecatron/config.json` (default
directory: the current one). Without it, a git repository is still a
project, detected each time its tracked files change. Init detects the
languages and layout, adds `.hecatron/local.json` and the log directory to
`.gitignore`, and writes a section for AI coding agents into `AGENTS.md`.
It never installs anything or touches CI.

| Flag | Meaning |
| --- | --- |
| `--components a,b` | The components, instead of detecting them |
| `--impl-path TEMPLATE` | Where a component's code in a language lives (`{component}/impl/{lang}`) |
| `--languages a,b` | The languages, instead of detecting them |
| `--justfile` | Also write `.hecatron/hecatron.just`, recipes delegating to hecatron, for a project justfile to `import` |
| `--force` | Run on a project that already has a configuration |
| `--no-agents` | Don't write instructions for AI agents to `AGENTS.md` |

How detection works: [init and detection](init.md).

### doctor

```
hct doctor [--fix] [lang...]
```

Checks that each language's toolchain works: its build and lint commands
run, its version is installed, and the breakages its definition knows of
are absent. With no languages, every added, enabled one. `--fix` runs each
unhealthy language's installer (asking for root once, up front), then
checks it again. See [installing toolchains](installing-toolchains.md#doctor-and-repair).

```bash
hct doctor
hct doctor --fix go lua
```

### install, uninstall

```
hct install [--plan] [lang...]
hct uninstall lang...
```

Install (or repair) toolchains: every language the project has when none
are named, continuing past failures, then each component's dependencies
once every toolchain is in. `--plan` only prints what each would install
and the commands it would run (binary only). Uninstall removes only what
hecatron installed and no other language needs. `hct install secrets`
installs gitleaks, for [the secret scan](audit.md#secrets). See
[installing toolchains](installing-toolchains.md).

```bash
hct install
hct install --plan zig
hct install secrets
hct uninstall haskell
```

### get-started

```
hct get-started [--status | --check | --install-all | --install LANG...]
```

Interactive onboarding: the project, which languages are installed, and an
install menu. `--status` prints the status table, `--check` exits 0 when
every language is installed and 1 otherwise, `--install-all` installs every
missing one and `--install LANG...` the ones named, without asking.

### bootstrap

```
hct bootstrap
```

Sets up a fresh machine for the project: base packages, then `hct
install`.

## Languages

### list

```
hct list [languages] [--all]
```

The project's languages and their status (added, disabled); `--all` lists
every language hecatron knows and where each definition's files come from
(built in, or the project's `.hecatron/languages/<lang>/`).

### add, remove

```
hct add lang...
hct remove lang...
```

Add languages to, or remove them from, config.json's `languages`; both also
regenerate the workflows and Dockerfile the project generates.

### enable, disable

```
hct disable lang...
hct enable [lang... | --all]
```

Leave languages out of your parallel runs, for you only (`.hecatron/local.json`,
not committed), or bring them back; `--all` re-enables every one.

## Generated files

See [generated files](generated-files.md) for what each contains.

### dockerfile

```
hct dockerfile [--check]
```

Writes the Dockerfile (config.json's `generated.dockerfile`) from each
language's snippet. `--check` changes nothing and fails when it is behind.

### workflows

```
hct workflows [--check] [--ci github|gitlab]
```

Writes each language's CI workflow from its `workflow.yml` template
(`generated.workflows`), and, as configured, the GitLab pipeline and the
Dependabot and Renovate configuration. `--ci` chooses the CI system
instead of `generated.ci` (binary only). `--check` changes nothing and
fails when a file is behind.

### check

```
hct check
```

Every check hecatron owns, for CI: the Dockerfile and workflows are
current, each workflow covers every component, pin files agree with the
project's `version`, and config.json's settings are valid. With the binary,
also that the hecatron running is the one the project
[pins](installing-hecatron.md#pinning-a-projects-hecatron), and the
`advisor` and `licenses` settings.

### banner

```
hct banner --generator PATH [--source PATH] [--regenerate CMD] [--prefix P] [--suffix S]
```

Prints the DO NOT EDIT banner for a project's own code generator to embed:
the generator, its source and how to regenerate. Each line starts with
`--prefix` (default `# `) and ends with `--suffix`.

```bash
hct banner --generator codegen/locales.py --source data/locales.json --prefix '// '
```

## The project's own scripts

### env

```
hct env
hct env LANG [-c COMPONENT]
```

Without a language: shell code for a project's own recipes, `eval "$(hct
env)"`, giving the `HECATRON_*` variables, the language lists and helpers
(`_name`, `_check`, `_dep_available`, `_lang_grid`...). With one: that
language's environment for a component (its version's PATH and variables),
as shell exports.

```bash
eval "$(hct env java -c age)"
```

### exec

```
hct exec LANG [-c COMPONENT] -- CMD [ARGS...]
```

**Binary only.** Runs CMD with the language's environment for a component,
what `hct env LANG -c COMPONENT` prints, without changing your shell: handy
when components use different versions of one toolchain. Without `-c`, the
component whose directory for the language holds the working directory,
else the only component in the language. CMD runs in the working directory,
with your terminal; the exit status is CMD's (127 when it isn't found).
When the component's version isn't installed, it says so (exit 3): `hct
install LANG` installs it.

```bash
hct exec zig -c parser -- zig build
cd age/python && hct exec python -- python -m pytest -x
```

### parallel

```
hct parallel [-j VALUE] [-n NICE] [-s MODE] NAME "job" ...
```

Runs a project's own list of jobs with the runner's scheduling, live display
and logs (`<logs>/NAME/`). Each job is passed to `HECATRON_JOB_COMMAND`
(default `just`); `"label::cmd args"` names a job, else its last word is the
label. `-n NICE` runs jobs under `sudo nice -n NICE`.

```bash
hct parallel cli "build-cli go" "build-cli rust"
```

### version

```
hct version VERSION
```

Bumps the project's version by running each added language's `version.sh`
hook with it, from the root; languages without one are skipped.

### metadata

```
hct metadata
```

Updates package metadata interactively: reads the file config.json's
`metadata` names (name, description, author, license, repository) and runs
each language's `metadata.sh` hook.

## Hecatron itself

### self

```
hct self update [--check] [--version X.Y.Z]
hct self uninstall [--all] [--yes]
hct self test
hct self build
```

| Command | What it does |
| --- | --- |
| `self update` | Update an installation install.sh made to the latest release (`--version X.Y.Z`: that one); `--check` only says whether one exists (exit 0 up to date, 3 newer, 1 the check failed). Binary only |
| `self uninstall` | Remove hecatron, its lines in the startup files and its Agent Skill copies, after asking (`--yes` without a terminal); `--all` also its data and download cache. Binary only |
| `self test` | Hecatron's own checks: ShellCheck, then the unit tests |
| `self build` | Build the Go binary from a checkout, for working on hecatron |

See [installing hecatron](installing-hecatron.md#updating-and-removing).
`self-test` and `self-build`, the earlier names, still work.

### skill

```
hct skill install [--user | --project | --dir DIR]
hct skill path
```

Installs hecatron's Agent Skill, the instructions an AI coding agent loads
to use `hct`: `--user` (the default) in `~/.claude/skills/hecatron`,
`--project` in `.claude/skills/hecatron` to commit, `--dir DIR` in
`DIR/hecatron` for another agent. `hct self update` refreshes the `--user`
and `--dir` copies. `hct skill path` prints where the skill is inside
hecatron. Binary only.

### help, --version

```
hct help
hct help agents
hct --version
```

`hct help agents` prints [the reference for AI agents](../agents.md) for
the installed version.
