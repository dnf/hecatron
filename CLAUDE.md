# CLAUDE.md

## What this repository is

Hecatron manages polyglot repositories: it installs, checks and repairs each
language's toolchain, runs a project's tests, linters, builds and cleans per
language and component (or everything in parallel), and keeps generated CI
workflows and a Dockerfile in sync.

It was split out of [dnf/sayable](https://github.com/dnf/sayable), where it
lived as `.hecatron/`. **It is not yet usable standalone.** The plan for the
installable tool (`brew install hecatron`, `hecatron init`) is
[`docs/design.md`](docs/design.md); read it before changing structure, and
keep it current when a decision changes.

## Layout (current, before the restructure in docs/design.md §5)

```
languages/<lang>/        # toolchain definitions: config.json, install.sh, uninstall.sh, doctor.sh, dockerfile
languages/_common.sh     # helpers for installers and hooks
tools/runner/            # run-parallel.sh: parallel runner with live display and history
tools/language/          # config bridge (emit-config.py, helpers.sh), add/remove/enable/disable/list,
                         # install/uninstall, doctor, per-component dispatch (each-library.sh)
tools/codegen/           # DO NOT EDIT banner (util.py), Dockerfile generator
tools/metadata/          # interactive package-metadata updater
tools/onboarding/        # get-started status and install menu
tools/bootstrap/         # macOS/Linux machine setup
tools/lint/              # shellcheck over every tracked shell script
docs/                    # design, installer reference, adding a language, build conventions
examples/sayable/        # sayable's hooks, workflow templates and config at the split (reference only)
```

The scripts still assume they live at `<project>/.hecatron/` (they compute the
project root from their own path) and that a justfile drives them. Removing
those assumptions is the implementation phase of the design.

## Conventions

- **Language definitions are generic.** Nothing under `languages/<lang>/` may
  name a particular project's files. Project-specific behaviour belongs in
  hooks (see `examples/sayable/languages/<lang>/`).
- **Installers are idempotent and repair.** An installer must be safe to rerun
  and must fix the broken states its `doctor.sh` detects.
- **Every per-language command covers every component.** Dispatch through
  `each-library.sh`; never hard-code one component's path.
- **Zero warnings.** shellcheck clean at warning level
  (`bash tools/lint/shellcheck.sh`); any `# shellcheck disable=` carries a
  comment saying why.
- **Name over comment.** Extract a named function or variable instead of a
  comment explaining what a line does; comments explain why.

## Shell conventions

- **Brace-delimit all variables**: `${var}` not `$var` (exception: `$1`, `$?`, `$@`, etc.)
- **Brace-delimit before Unicode**: `${_s_pass}✓` not `$_s_pass✓` (Bash 3.2 treats Unicode as identifier chars)
- **ANSI colors**: Base `_c_*` variables define raw escapes; semantic `_s_*` variables give meaning. Always use `_s_*` in output (except structural codes like `_c_reset`).
- **One entry per line**: Language lists, case arms, and grids use one-per-line format to prevent merge conflicts.
- **Bash 3.2 compatibility** for anything a user runs directly on macOS: no `mapfile`, no associative arrays, and expand possibly-empty arrays as `${arr[@]+"${arr[@]}"}` under `set -u`.

## Commit conventions

- Imperative mood ("Add Odin installer"), one logical change per commit.
- Squash-merge pull requests.
