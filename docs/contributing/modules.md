# Modules

What each part of the repository does. The [architecture](architecture.md)
shows how they fit; [command flow](command-flow.md) follows one command
through them.

## Entry points

| File | What it does |
| --- | --- |
| `bin/hecatron`, `bin/hct` (a link) | Finds `HECATRON_HOME` (its own real location, following Homebrew's links), puts hecatron's bin directory (`~/.local/share/hecatron/bin`) first on PATH, and runs the Go binary `bin/.hecatron-go` when it belongs to this commit (`bin/.hecatron-go.commit` is `HEAD`, or `dev` after `hct self build`) unless `HECATRON_NO_GO=1`. Otherwise it prints `help` and `--version` itself, checks for bash 3.2 and Python 3.9, finds the project root (`project.py root`) and execs the command's script |
| `install.sh` | Installs hecatron (POSIX sh, safe to source): what it needs (git, Python 3, perl), a clone at the latest release tag in `~/.hecatron`, the release's binary checked against `SHA256SUMS`, and `~/.hecatron/env` sourced from the shell startup files |

## The Go core: `cmd/hecatron`

`package main`, one file per command or concern.

| File | What it does |
| --- | --- |
| `main.go` | `main` finds the home and puts hecatron's bin directory on PATH; `dispatch` runs a native-only command, or finds the project (in Go, else through `project.py`; then the pin warning) and runs the command's handler in `nativeCommands`, which may decline (`handled=false`), else delegates it |
| `home.go` | `findHome`: `HECATRON_HOME`, else the checkout or install the binary sits in |
| `delegate.go` | `delegated`: the script each command falls back to, run with the same arguments and environment (`exec` where the platform allows: `exec_unix.go`, `exec_other.go`); the Python 3.9 check before delegating |
| `help.go` | `hct help`, `help agents` (prints `docs/agents.md`), `self help`, and each run command's help |
| `run.go`, `hooks.go` | The run commands: argument parsing as `run.sh` reads it, named targets one at a time, every language and task through `internal/runner`, `--changed`, time limits; `languageExec` runs a language's hook for each component as `each-component.sh` does |
| `reportfiles.go`, `reports.go` | A run's CI reports (`HECATRON_REPORT_DIR`, `--output junit|sarif`); the hidden `_audit`, `_outdated` and `_gotest` commands the hooks call in place of `audit.py`, `outdated.py` |
| `doctor.go` | `doctor`'s driver: the checks, in parallel through the runner; `--fix` runs installers one at a time |
| `install.go`, `installdata.go`, `sudo.go` | `install` and `uninstall`: each language's script, or the install engine for a language whose `install.json` the binary can carry out; the hidden `_install`; asking for root once (`sudo -v`, refreshed) |
| `manage.go`, `init.go`, `list.go`, `banner.go`, `env.go`, `version.go` | The project commands `manage.py` runs (`add`, `remove`, `enable`, `disable`, `check`, `dockerfile`, `workflows`, `init`), `list`, `banner`, `env`, `version`, writing exactly what the scripts write |
| `status.go`, `sync.go`, `advisor.go`, `licenses.go`, `sbom.go`, `secrets.go`, `skill.go` | Native-only commands |
| `execute.go` | `exec`: a command run with a component's language environment (`project.LanguageEnv`), replacing the binary's process where the platform allows |
| `selfupdate.go`, `selfuninstall.go` | `self update` (and the hidden `_update-check`, `_write-env`) and `self uninstall` |
| `pin.go` | The warning when the running hecatron isn't the project's pinned one (once per command tree: `HECATRON_PIN_WARNED`) |
| `plan.go` | `hct plan <lang> <action> [dir]`: what a declarative definition's default hook would run (prototype, #83) |

## Go packages: `internal/`

| Package | What it does |
| --- | --- |
| `project` | The read-only project model, a port of `project.py` and `versions.py` that must agree with them: `FindRoot`, config and `local.json`, components and `implPath`, definitions and file resolution (`definitions.go`), detection (`detect.go`, `yields.go`, `detected.go` and its cache), toolchain versions (`resolve.go`, `versions.go`, `pinfiles.go`, `environment.go`), each component's environment, the pin (`pin.go`), config validation (`check.go`, `licenses.go`, `ci.go`), and writing files as Python writes them (`save.go`, `json.go`, `pyexport.go`, `pytext.go`, `paths.go`). It declines (`IsUnsupported`) where Python's reading of a value would differ |
| `runner` | Runs jobs in parallel with `run-parallel.sh`'s scheduling and `history.log`: concurrency (`-j`), the live display (`live.go`), human output off a terminal (`human.go`), the agent, JSON and CI formats (`render.go`), error excerpts from a definition's `errorPatterns` (`excerpt.go`), time limits (`limit.go`), stopping a job's processes (`process_*.go`) |
| `changed` | `--changed` (which languages, components and tasks a diff touches; `select.go`) and `hct sync`'s reading of git (`sync.go`) |
| `generate` | Generated files: the Dockerfile, each language's workflow, the GitLab pipeline and caching (`ci.go`), Dependabot and Renovate (`updates.go`), banners; byte for byte what the scripts write |
| `install` | The install engine: `install.json`'s schema and conditions, the plan, applying it (`apply.go`), release downloads (`download.go`), apt sources (`aptsource.go`), receipts and uninstall; `system.go` is the machine, replaced in tests. See [install engine](install-engine.md) |
| `actions` | Plans a language's default hooks from the rules in its `config.json` or a Go family (`jvm.go`); used by `hct plan` while definitions move to data |
| `deps` | Reads the audit and outdated reports of every supported tool, as `audit.py` and `outdated.py` do, with Python's JSON rules (`pyjson.go`) |
| `reports` | JUnit XML (merging the tools' own test reports: `native.go`), SARIF 2.1.0, GitLab Code Quality and the GitHub job summary |
| `licenses`, `spdx` | `hct licenses`: OSV-Scanner's and the lock files' licenses, the project's own packages (`own.go`), lock files outside the components (`unread.go`), SPDX expressions and their copyleft classes (`copyleft.go`, `copyleft.txt`), the project's policy |
| `sbom` | `hct sbom`: CycloneDX 1.6 and SPDX 2.3 JSON from `licenses.GatherAll`, purls |
| `secrets` | `hct audit`'s secret scan: runs gitleaks with `--redact` and a generated config (what git ignores, `audit.secrets.ignore`), decoding only each finding's rule, file, line, commit and fingerprint |
| `advisor` | `hct advisor`: rules from `advice.json` and `languages/_advice.json`, conditions, the rules written in Go (toolchain support, lock freshness, CI coverage, audit acceptances) |
| `status` | `hct status` |
| `selfupdate` | Finding releases, verifying and swapping in a new one, writing `env` as install.sh does, uninstalling, the cached update check and its notice |

## The scripts: `libexec/`

| Path | What it does |
| --- | --- |
| `lib/project.py` | The project model, from Python (`import project`) or shell (`python3 project.py <query>`: `root`, `shell-env`, `resolve`, `impl-dirs`, `version`, `versions`, `env`, `missing-versions`, `version-notes`, `tasks`, `task`). Detection and its cache in `.git/hecatron/` |
| `lib/versions.py` | Hecatron's version syntax and the pin-file formats |
| `lib/jvm.py` | The JDKs a JVM build needs besides its own: toolchains and a ceiling |
| `lib/helpers.sh` | Sourced by the shell commands: evaluates `project.py shell-env` (the `HECATRON_*` variables, the language lists `_ADDED_LANGS`, `_TEST_LANGS`..., lookups such as `_name`, `_check`, `_dep_available`) |
| `lib/each-component.sh` | Runs one hook for every component of a language (or those named), from each component's directory with its environment; lint's format check after |
| `lib/audit.py`, `lib/outdated.py` | Read a tool's audit or outdated report, apply accepted risks and thresholds, print findings |
| `commands/run.sh` | The run commands: named targets one at a time through `each-component.sh`; none, a job per language and task through `run-parallel.sh` (each job is `bin/hecatron <action> <target>`) |
| `commands/manage.py` | `init`, `install`, `uninstall`, `list`, `add`, `remove`, `enable`, `disable`, `dockerfile`, `workflows`, `check`, `banner` (argparse) |
| `commands/doctor.sh`, `version.sh` | `doctor` and `version` |
| `commands/self-test.sh`, `self-build.sh` | `self test` (ShellCheck, then the unit tests) and `self build` |
| `runner/run-parallel.sh` | The bash runner: slots, scheduling from `history.log`, the live display with swim lanes. Its own docs: [README](../../libexec/runner/README.md), [ARCHITECTURE](../../libexec/runner/ARCHITECTURE.md) |
| `codegen/` | `dockerfile.py`, `banner.py`; `languages_doc.py` writes [languages.md](../guide/languages.md) from the definitions; `skill.py` keeps the Agent Skill in `plugins/hecatron/` in step with `docs/agents.md` |
| `metadata/set.sh`, `onboarding/get-started.sh`, `bootstrap/` | `metadata`, `get-started`, `bootstrap` |
| `lint/shellcheck.sh` | ShellCheck on every tracked shell script (`hct lint shell`) |
| `maintenance/` | `release.sh` ([releasing](releasing.md)), `release_dates.py` (each language's `releases.json`), `ci-jobs.sh` (which CI jobs a change runs), `check_links.py` (external links in the docs, weekly), `prune-worktrees.sh` (removes the worktrees whose work is in main; `--dry-run` lists them) |

## Language definitions: `languages/`

| Path | What it does |
| --- | --- |
| `<lang>/config.json` | The definition's data: `dependency`, detection, versions (`version`, `track`, `pinFiles`, `supportFiles`...), `path` and `env`, `cache`, `errorPatterns`, and the descriptions [languages.md](../guide/languages.md) is built from. A project's `config.json` is merged on top |
| `<lang>/install.json` | The install as data ([install engine](install-engine.md)) |
| `<lang>/install.sh`, `uninstall.sh`, `doctor.sh` | The install as a script ([installer scripts](installer-scripts.md)) |
| `<lang>/installed.sh`, `releases.json` | The versions installed on this machine; the release dates resolution reads |
| `<lang>/dockerfile` | The Dockerfile snippet |
| `<lang>/advice.json` | `hct advisor`'s rules |
| `<lang>/hooks/*.sh` | The default hooks: `test`, `lint`, `build`, `clean`, `deps`, `format`, `audit`, `outdated`, and where the language has a tool for them, `docs` and `bench` |
| `_common.sh` | Installer helpers: `$OS`, `$SUDO`, `apt_install`, `brew_install`, `impl_dirs`, `each_impl`, `impl_deps`, `install_from_data`, `problem`... |
| `_hooks.sh` | Default-hook helpers: finding the build file below or above, the package manager, `*_fetch` for dependencies |
| `_deps.sh`, `_format.sh`, `_audit.sh`, `_outdated.sh`, `_docs.sh`, `_bench.sh` | Helpers for those hooks; `_audit.sh` and `_outdated.sh` hand reports to `audit.py`/`outdated.py`, or the binary's `_audit`/`_outdated` when `HECATRON_BINARY` is set |
| `_advice.json` | Advisor rule sets several languages share |

## Elsewhere

| Path | What it does |
| --- | --- |
| `plugins/hecatron/` | The Agent Skill and Claude Code plugin (`hct skill install`) |
| `examples/sayable/` | Sayable's hooks, workflow templates and config at the split (reference only) |
| `tests/` | The tests: [testing](testing.md) |
| `.hecatron/config.json` | Hecatron's own project: Go at the root, the Python suite as a test task |
