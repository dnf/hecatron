# Command flow

How a command travels from `hct` to the tools it runs, through both
implementations. The pieces are in [modules](modules.md).

## 1. The launcher: `bin/hecatron`

1. **Home.** `HECATRON_HOME` is the environment's, else `bin/hecatron`'s
   real location, one level up (it follows Homebrew's links).
2. **PATH.** Hecatron's bin directory (`~/.local/share/hecatron/bin`, or
   under `$XDG_DATA_HOME`), where installers put tools without root, goes
   first, so hooks find them in a shell that never sourced the env.
3. **The binary.** If `bin/.hecatron-go` exists, `HECATRON_NO_GO` isn't set,
   and `bin/.hecatron-go.commit` names the checkout's `HEAD` (install.sh's
   release binary) or says `dev` (`hct self build`'s), it `exec`s the binary
   with the same arguments. Else it carries on in bash.
4. **Bash dispatch.** `help` and `--version` answer at once; then bash 3.2
   and Python 3.9 are checked; `self` commands are handled; every command
   but `init` and `banner` finds the project (`project.py root`: the nearest
   `.hecatron/config.json`, else the git repository's top, detected) and
   exports `HECATRON_ROOT`; then the command's script is `exec`ed:

   | Command | Script |
   | --- | --- |
   | `test` ... `outdated`, `docs`, `bench` | `libexec/commands/run.sh <action>` |
   | `doctor`, `version` | `libexec/commands/doctor.sh`, `version.sh` |
   | `init`, `install`, `list`, `add`, `check`, `dockerfile`, `workflows`, `banner`... | `libexec/commands/manage.py <command>` |
   | `get-started`, `metadata`, `bootstrap` | `libexec/onboarding/`, `metadata/`, `bootstrap/` |
   | `env`, `parallel` | `project.py env`, or `helpers.sh`; `libexec/runner/run-parallel.sh` |
   | `status`, `sync`, `advisor`, `licenses`, `sbom`, `exec`, `skill`, `self update` | a message: these need the binary |

## 2. The Go binary: `cmd/hecatron`

1. **Home and PATH** as above (`findHome`, `linksOnPath`).
2. **Native-only commands** (`status`, `sync`, `advisor`, `licenses`,
   `sbom`, `exec`, `skill`, `self`, `plan`, and the hidden `_audit`, `_outdated`, `_install`,
   `_gotest`, `_update-check`, `_write-env`) run at once.
3. **The project.** Every other command but `init` and `banner` finds the
   project: `project.FindRoot` and `project.Load` in Go; a git repository
   without a config is detected in Go too, writing the same cache
   `project.py` writes (`.git/hecatron/detected.json`); a config the Go model
   can't read exactly as Python would falls back to `project.py root`. Then
   the pin warning, once ([pinning](../guide/installing-hecatron.md#pinning-a-projects-hecatron)).
4. **Native or delegated.** The command's handler in `nativeCommands` runs it,
   or declines (`handled=false`) before printing or writing anything: an
   argument form it doesn't read as the script does, a project value the
   model declines. A declined or unported command is delegated
   (`delegate.go`): the same script `bin/hecatron` would run, with the same
   arguments and environment, after the Python 3.9 check, replacing the
   binary's process where the platform allows.

## 3. A run command: `hct test`

The Go path (`cmd/hecatron/run.go`, `hooks.go`, `internal/runner`):

1. **Arguments** as `run.sh` reads them (`-j`, `-s`, `-c`, `--output`,
   `--timeout`, `--changed`, the action's own flags); a usage error exits 2.
2. **Targets.** Named targets resolve to languages (abbreviations, typos),
   project tasks or `shell`; an unknown or unadded one is left to `run.sh`
   for its own message. With `--changed`, `internal/changed` selects them
   from the diff.
3. **Jobs.** Without targets: one job per language with a hook for the
   action (`Shell().HookLanguages`), marked disabled (`local.json`) or not
   installed (its `dependency` isn't on PATH), then one per project task
   and, for `lint`, `shell`.
4. **Each language job** (`languageExec`) resolves the hook
   (`.hecatron/languages/<lang>/<action>.sh`, else
   `$HECATRON_HOME/languages/<lang>/hooks/<action>.sh`), then for each
   component: its directory (`implPath`, `implPaths`), its environment
   (the resolved toolchain version's `path` and `env`, then the other
   languages' static `path` entries), and runs `bash <hook> <component>`
   there, with `HECATRON_ROOT`, `HECATRON_COMPONENT`, `HECATRON_LANG`,
   `HECATRON_BINARY` and stdin closed. Lint then runs the format hook with
   `HECATRON_FORMAT_CHECK=1`, unless the lint hook checked formatting
   itself. Every component runs even after one fails.
5. **The hook** sources `languages/_hooks.sh`, finds the component's build
   file (here, below or above), picks the ecosystem's command and runs it
   (`cargo test`, `go test ./...`, the package's `npm test`...). Audit and
   outdated hooks save the tool's JSON report and hand it to
   `hecatron _audit` / `_outdated` (`internal/deps`), or `audit.py` /
   `outdated.py` without the binary.
6. **The runner** (`internal/runner`) schedules the jobs (failures first,
   longest first, a fast lane; `-j` slots), stops one at its time limit,
   writes `<logs>/<action>/<label>.log` and `history.log`, and renders the
   results as human, agent, JSON or CI output. Reports for CI are written
   beside them (`internal/reports`).
7. **Exit status:** 0, 1 for a failure, 2 for usage, 3 when only missing
   toolchains failed, 130 when interrupted.

The script path (`run.sh`) does the same in bash: named targets run one at
a time through `libexec/lib/each-component.sh`, which asks `project.py`
for the hook (`resolve`), the directories (`impl-dirs`) and each
component's environment (`env`); without targets it builds the job list
from `helpers.sh`'s language lists and hands it to
`libexec/runner/run-parallel.sh`, whose jobs are `bin/hecatron <action>
<target>`, the named-target path again.

## 4. Installing: `hct install`

1. The languages: every added one, or those named.
2. For each, the binary chooses: the project's own `install.sh` always
   wins; `HECATRON_INSTALLER=script` forces scripts; else an `install.json`
   the binary can carry out on this platform; else `install.sh`
   ([install engine](install-engine.md#which-runs)).
3. Root is asked for once, up front, if any of them may need it (`sudo.go`).
4. Data languages are planned and applied together (`internal/install`:
   one package-manager call, then guarded steps and downloads, receipts);
   script languages run their `install.sh` from the project root with the
   language's version environment.
5. With no languages named, the dependencies the installers deferred
   (`HECATRON_DEFER_DEPS`) are fetched with `hct deps` once every toolchain
   is in.

Without the binary, `manage.py install` runs each `install.sh` in turn.

## 5. Generating: `hct workflows`, `hct check`

`manage.py` (or `cmd/hecatron/manage.go` with `internal/generate`) reads
the project, renders each generated file in memory (banner, templates,
`{{version:lang}}` placeholders, time limits, caching), and writes it, or
with `--check` compares it with what's on disk. The Go port works out
everything before writing anything, so a project it declines reaches
`manage.py` untouched, and writes files byte for byte as Python does.
`hct check` runs every such comparison, plus the workflow coverage of every
component, pin-file agreement and config validation.
