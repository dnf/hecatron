# CLAUDE.md

## What this repository is

Hecatron manages polyglot repositories: it installs, checks and repairs each
language's toolchain, runs a project's tests, linters, builds and cleans per
language and component (or everything in parallel), and keeps generated CI
workflows and a Dockerfile in sync.

It was split out of [dnf/sayable](https://github.com/dnf/sayable), where it
lived as `.hecatron/`. The design is [`docs/design.md`](docs/design.md); read
it before changing structure, and keep it current when a decision changes.

## Layout

```
bin/hecatron, bin/hct    # entry point: finds HECATRON_HOME, the project root, dispatches
libexec/lib/project.py   # the project model: config, components, language definitions, file resolution
libexec/lib/helpers.sh   # shell helpers built from project.py's shell-env
libexec/lib/each-component.sh  # run one hook for every component
libexec/commands/        # run.sh (test/lint/build/clean/rebuild/retest), manage.py (init, add, list,
                         # install, dockerfile, workflows, check, banner...), doctor.sh, version.sh
libexec/runner/          # run-parallel.sh and its docs
libexec/codegen/         # banner.py, dockerfile.py
libexec/{metadata,onboarding,bootstrap,lint}/
languages/<lang>/        # built-in definitions: config.json, install.sh, uninstall.sh, doctor.sh, dockerfile,
                         # advice.json (hct advisor's rules; languages/_advice.json: shared sets)
tests/                   # python3 -m unittest discover -s tests
docs/                    # design.md (keep current), installers.md, adding-a-language.md, build-conventions.md
examples/sayable/        # sayable's hooks, workflow templates and config at the split (reference only)
cmd/hecatron/            # the Go core's entry point (docs/design.md §19): ports commands, delegates the rest
internal/                # Go packages: project (the model and version resolution, a port of project.py and
                         # versions.py, and detection), generate (Dockerfile, workflows), runner (jobs, output),
                         # status (hct status), deps (audit and outdated reports), licenses and spdx
                         # (hct licenses), advisor (hct advisor), reports (JUnit, SARIF, job summary),
                         # actions, install, selfupdate (hct self update and the update notice)
```

Every command finds the project by walking up from the working directory
(`project.find_root`) and reads it through `project.py`; nothing computes a
project path from its own location.

## Checks

```bash
bin/hecatron self test    # shellcheck (zero warnings), then the unit tests
```

## Conventions

- **Language definitions are generic.** Nothing under `languages/<lang>/` may
  name a particular project's files. Project-specific behaviour belongs in
  hooks (see `examples/sayable/languages/<lang>/`).
- **Installers are idempotent and repair.** An installer must be safe to rerun
  and must fix the broken states its `doctor.sh` detects.
- **Every per-language command covers every component.** Dispatch through
  `each-component.sh`; never hard-code a component's path.
- **Zero warnings.** shellcheck clean at warning level
  (`bash tools/lint/shellcheck.sh`); any `# shellcheck disable=` carries a
  comment saying why.
- **Language count.** Any line in a `.md` file or `llms.txt` that states how
  many languages hecatron supports ends with `<!-- language-count -->`
  (inside the cell, in a table row), so `grep -rn language-count` finds
  every place to update. `tests/test_language_count.py` fails on a marked
  line with another count and on an unmarked line stating it. CHANGELOG.md
  is history and is exempt.
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
