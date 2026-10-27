# Contributing to hecatron

Issues and pull requests are welcome.

## Checks

Before sending a change, run:

```bash
hct self-test    # ShellCheck with zero warnings, then the unit tests
```

CI runs the same checks on Ubuntu and macOS, and the unit tests again under
bash 3.2, the version macOS ships.

A change to a language's installer, doctor or hooks should also pass that
language's Docker tests. They install the toolchain in a throwaway container
and never change your machine:

```bash
tests/docker/install-test.sh ubuntu:24.04 go     # install, doctor, reinstall, smoke test
tests/docker/run.sh ubuntu:24.04 bash /hecatron/tests/hooks/check.sh go
                                                  # test, lint, build and clean on fixture projects
```

Changes to hecatron's own installer, `install.sh`, have a test that edits a
user's startup files, so it too runs only in a container (CI also runs the
installer on macOS):

```bash
tests/docker/run.sh ubuntu:24.04 bash /hecatron/tests/installer/check.sh
```

The [coverage workflow](.github/workflows/coverage.yml) runs the install test
for every language on every platform each week and writes
[docs/coverage.md](docs/coverage.md).

## Releasing

Each change adds a line under `## Unreleased` in [CHANGELOG.md](CHANGELOG.md).
To release, from an up-to-date `main`:

```bash
libexec/maintenance/release.sh 0.4.0 --dry-run   # the checks, the changes and the notes
libexec/maintenance/release.sh 0.4.0
```

It checks the tree and the changelog and runs `hct self-test`, then bumps
`VERSION`, dates the changelog section and updates the README's status line
in a pull request, merges it when CI passes, tags `v0.4.0` and publishes a
GitHub release with the changelog section as its notes. The installer
installs the newest tag. Before 1.0, a release that changes behaviour bumps
the minor version.

## Conventions

[CLAUDE.md](CLAUDE.md) has the conventions the code follows (shell style,
idempotent installers, generic language definitions), for people and coding
agents alike. Changes to how hecatron works start in
[docs/design.md](docs/design.md), and
[docs/adding-a-language.md](docs/adding-a-language.md) covers a new language.

## Repository layout

| Path | Contents |
| --- | --- |
| [`bin/hecatron`](bin/hecatron), [`bin/hct`](bin/hct) | The command |
| [`languages/<lang>/`](languages/) | Language definitions: `config.json`, `install.sh`, `uninstall.sh`, `doctor.sh`, Dockerfile snippet, and default `hooks/` |
| [`languages/_common.sh`](languages/_common.sh) | Shared installer helpers |
| [`libexec/lib/`](libexec/lib/) | The project model (`project.py`), shell helpers, per-component dispatch |
| [`libexec/commands/`](libexec/commands/) | The commands (`run.sh` for test/lint/build/clean, `manage.py` for configuration, `self-test.sh`) |
| [`libexec/runner/`](libexec/runner/) | The parallel runner and its docs |
| [`libexec/codegen/`](libexec/codegen/) | DO NOT EDIT banners and the Dockerfile generator |
| [`libexec/`](libexec/)`{metadata,onboarding,bootstrap,lint}/` | Metadata updates, onboarding, machine bootstrap, shellcheck |
| [`tests/`](tests/) | Unit and command tests, the Docker install tests (`docker/`, `install/`), hook fixtures (`hooks/`) and the coverage matrix (`coverage/`) |
| [`docs/`](docs/) | Design, installers, adding a language, build conventions |
| [`assets/logo/`](assets/logo/) | The logo, favicons and social preview, and the script that builds them |
| [`examples/sayable/`](examples/sayable/) | Sayable's project hooks, CI workflow templates and config at the time of the split |

