# Contributing to hecatron

Issues and pull requests are welcome.

## Checks

Before sending a change, run:

```bash
hct self test    # ShellCheck with zero warnings, then the unit tests
```

CI runs the same checks on Ubuntu and macOS, and the unit tests again under
bash 3.2, the version macOS ships. The suites, the Docker tests a change to
a language should pass, the Go core's checks and what CI runs on a pull
request are in [docs/contributing/testing.md](docs/contributing/testing.md).

## Releasing

Each change adds its changelog entry as its own file,
`changelog.d/<branch>.md`, not by editing `CHANGELOG.md`
([changelog.d/README.md](changelog.d/README.md)); a change users can't see
adds none and is labelled `no changelog`. Releases are made with
`libexec/maintenance/release.sh`
([docs/contributing/releasing.md](docs/contributing/releasing.md)).

## Conventions

[CLAUDE.md](CLAUDE.md) has the conventions the code follows (shell style,
idempotent installers, generic language definitions), for people and coding
agents alike. The [contributor guide](docs/contributing/README.md) covers
the architecture, the modules and how a command flows through them; changes
to how hecatron works start in the
[design record](docs/contributing/design.md), and
[adding a language](docs/contributing/adding-a-language.md) covers a new
language.

## Repository layout

Each file and package in detail: [modules](docs/contributing/modules.md).

| Path | Contents |
| --- | --- |
| [`bin/hecatron`](bin/hecatron), [`bin/hct`](bin/hct) | The command |
| [`cmd/hecatron/`](cmd/hecatron/), [`internal/`](internal/) | The Go core: the binary's commands and its packages |
| [`languages/<lang>/`](languages/) | Language definitions: `config.json`, `install.sh`, `uninstall.sh`, `doctor.sh`, Dockerfile snippet, and default `hooks/` |
| [`languages/_common.sh`](languages/_common.sh) | Shared installer helpers |
| [`libexec/lib/`](libexec/lib/) | The project model (`project.py`), shell helpers, per-component dispatch |
| [`libexec/commands/`](libexec/commands/) | The commands (`run.sh` for test/lint/build/clean, `manage.py` for configuration, `self-test.sh`) |
| [`libexec/runner/`](libexec/runner/) | The parallel runner and its docs |
| [`libexec/codegen/`](libexec/codegen/) | DO NOT EDIT banners and the Dockerfile generator |
| [`libexec/`](libexec/)`{metadata,onboarding,bootstrap,lint}/` | Metadata updates, onboarding, machine bootstrap, shellcheck |
| [`tests/`](tests/) | Unit and command tests, the Docker install tests (`docker/`, `install/`), hook fixtures (`hooks/`) and the coverage matrix (`coverage/`) |
| [`docs/`](docs/) | The [user guide](docs/guide/README.md) and the [contributor guide](docs/contributing/README.md) |
| [`assets/logo/`](assets/logo/) | The logo, favicons and social preview, and the script that builds them |
| [`examples/sayable/`](examples/sayable/) | Sayable's project hooks, CI workflow templates and config at the time of the split |

