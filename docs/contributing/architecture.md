# Architecture

Hecatron is one command, `hct` (`bin/hct` links to `bin/hecatron`), with two
implementations of most of it:

- **The scripts:** bash and Python 3 (standard library only), the original
  implementation, which runs everywhere bash 3.2 and Python 3.9 do and needs
  no build. `bin/hecatron` dispatches each command to a script under
  `libexec/`.
- **The Go core:** one binary (`cmd/hecatron`, `internal/`), built for
  macOS, Linux and Windows on x86_64 and arm64 and published with each
  release. It runs the commands it has ported itself and hands every other
  one to the scripts, with the same arguments, environment and exit status.
  New features are written only in Go. Why, and how the port proceeds:
  [design record §19](design.md#19-a-go-core-and-native-windows).

Both read the same data: the project's `.hecatron/` and the language
definitions in `languages/`. Neither builds or tests anything itself: they
run each language's own tools through **hooks**, small bash scripts run once
per component.

```
             hct <command> [args]
                     |
                bin/hecatron ---------------------------+
   (HECATRON_HOME, hecatron's bin dir on PATH)          |
       |                                                | a release's binary at its
       | no binary for this commit                      | own commit, or a dev build
       v                                                v
  bash dispatch                                   bin/.hecatron-go
  (bash 3.2, Python 3.9 checked)                  (cmd/hecatron)
       |                                            |          |
       |                                       ported command  not ported, or
       |                                       (internal/...)  declined by the model
       |                                            |          |
       v                                            |          v
  libexec/commands/run.sh, manage.py,  <------------+--- delegated, same argv
  doctor.sh, version.sh, ...                        |    and environment
       |                                            |
       v                                            v
  libexec/lib/project.py  ==== parity tests ====  internal/project
  (the project model)                             (the same model in Go)
       |                                            |
       +-----------------+   +----------------------+
                         v   v
            .hecatron/config.json, local.json        the project
            .hecatron/languages/<lang>/...           (overrides first)
            $HECATRON_HOME/languages/<lang>/...      built-in definitions
                         |
                         v
          hooks: languages/<lang>/hooks/<action>.sh, run once per component
          in its directory, calling the ecosystem's tools (cargo, go, npm...)
```

## The pieces

| Piece | Where | What it does |
| --- | --- | --- |
| Entry point | `bin/hecatron`, `cmd/hecatron/main.go` | Find `HECATRON_HOME` and the project, pick an implementation, dispatch |
| Project model | `libexec/lib/project.py` (+ `versions.py`), `internal/project` | Find the project (`.hecatron/config.json`, else the git repository, detected and cached in `.git/hecatron/`); read its configuration, components, languages and definitions; resolve a language's files; resolve toolchain versions and each component's environment |
| Run commands | `libexec/commands/run.sh`, `libexec/lib/each-component.sh`, `cmd/hecatron/run.go`, `hooks.go` | Turn `hct test` and friends into jobs, one per language and project task, each running the language's hook for every component |
| Runner | `libexec/runner/run-parallel.sh`, `internal/runner` | Run jobs in parallel: scheduling from `history.log`, the live display, logs, time limits, the output formats |
| Language definitions | `languages/<lang>/` | Data (`config.json`, `install.json`, `releases.json`, `advice.json`), scripts (`install.sh`, `uninstall.sh`, `doctor.sh`, `installed.sh`), the Dockerfile snippet and the default hooks |
| Shared shell helpers | `languages/_common.sh`, `_hooks.sh`, `_deps.sh`, `_format.sh`, `_audit.sh`, `_outdated.sh`, `_docs.sh`, `_bench.sh` | Installer and hook building blocks |
| Install engine | `internal/install`, `cmd/hecatron/installdata.go` | Carry out a definition's `install.json`: plan, batch packages, guarded steps, release downloads, receipts ([install engine](install-engine.md)) |
| Generators | `libexec/codegen/`, `manage.py`, `internal/generate` | The Dockerfile, CI workflows, GitLab pipeline, Dependabot and Renovate configuration, DO NOT EDIT banners |
| Reports | `libexec/lib/audit.py`, `outdated.py`, `internal/deps`, `internal/reports`, `internal/licenses` | Read tools' audit, outdated and license reports; write JUnit, SARIF, Code Quality and job summaries |
| Native-only commands | `internal/status`, `internal/changed`, `internal/advisor`, `internal/selfupdate` | `status`, `sync` and `--changed`, `advisor`, `self update` and the update notice |

Each module in detail: [modules](modules.md). How one command travels
through them: [command flow](command-flow.md).

## Rules the architecture rests on

- **The project comes from the working directory**, never from a script's
  own location: every command finds it by walking up (`project.find_root`,
  `project.FindRoot`), like git.
- **One model, two implementations that agree.** The Go model may decline
  what it can't answer exactly as `project.py` would (a value only Python
  reads a certain way); it never answers differently. Parity tests run both
  on the same fixture projects and compare output byte for byte
  ([testing](testing.md)).
- **Language definitions are generic.** Nothing under `languages/<lang>/`
  names a particular project's files; project-specific behaviour is a
  project hook in `.hecatron/languages/<lang>/`, which replaces the
  built-in file of the same name ([configuration](../guide/configuration.md#resolving-a-languages-files)).
- **Every per-language command covers every component**, through
  `each-component.sh` or its Go port; nothing hard-codes a component's
  path.
- **Installers are idempotent and repair** what their doctor detects.
- **Data before code.** A definition says what to do in the most
  declarative form that can express it: rules in `config.json`, then Go
  families for shared build systems, then scripts
  ([design record §19](design.md#language-definitions-data-families-scripts)).
