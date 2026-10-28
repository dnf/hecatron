# Contributing to hecatron

Issues and pull requests are welcome.

## Checks

Before sending a change, run:

```bash
hct self test    # ShellCheck with zero warnings, then the unit tests
```

CI runs the same checks on Ubuntu and macOS, and the unit tests again under
bash 3.2, the version macOS ships.

### What CI runs

A pull request runs only the CI jobs its changes can affect: the first job,
`changes`, lists the paths it changes against its base, and
[`libexec/maintenance/ci-jobs.sh`](libexec/maintenance/ci-jobs.sh) maps
them to jobs (documentation alone runs just the unit tests, a Go change the
Go jobs, an installer change the installer tests; a workflow change or a
path it doesn't know runs everything). Its tests,
[`tests/test_ci_jobs.py`](tests/test_ci_jobs.py), show the mapping. A push
to `main` runs every job.

The last job, `ci-ok`, always runs: it fails if any job failed or was
cancelled and passes when the rest succeeded or were skipped. It's the one
CI check to require (with the Changelog workflow's `fragment`), so a pull
request can merge itself once they pass:

```bash
gh pr merge --auto --squash
```

A new job goes in `ci-ok`'s `needs`, gets an `if:` on the `changes` job's
output, and gets its paths in `ci-jobs.sh` and its tests.

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

### The Go core

Hecatron is tested with itself: `.hecatron/config.json` describes it (Go
at the root, the Python suite as a test task, shellcheck through `hct lint`'s
shell check), so `hct test` and `hct lint` run its checks as they would a
user's. They go through the Go binary, the way users get it, once it's
built from your checkout:

```bash
hct self build    # needs Go 1.24 or later; rerun after changing Go code
hct test          # the Go tests and the Python suite, in parallel
hct lint          # golangci-lint, the format check, shellcheck
```

`bin/hecatron` runs a binary from `hct self build` on any branch (a release's
binary only at its own commit). Until you rebuild, it keeps running the one
you built, so rebuild after pulling or changing Go code; `HECATRON_NO_GO=1`
keeps to the scripts. The Python suite always tests the scripts
(`hct self test` and the test task set `HECATRON_NO_GO=1`); the binary has
its own tests, which compare it with the scripts.

To run a binary by hand from elsewhere:

```bash
go build -o hct ./cmd/hecatron
cd ~/your-project && ~/hecatron/hct list
```

It finds its checkout from its own location (or `HECATRON_HOME`), runs the
commands ported so far itself and hands every other one to the existing
scripts. Its checks, which CI runs too:

```bash
gofmt -l cmd internal && go vet ./... && go test ./...
for os in linux darwin windows; do for arch in amd64 arm64; do
    GOOS=${os} GOARCH=${arch} go build -o /dev/null ./cmd/hecatron; done; done
```

The tests compare the Go project model with `libexec/lib/project.py`, and
the binary's output with `bin/hecatron`'s, byte for byte, on fixture
projects; those comparisons need bash and Python and are skipped on Windows.

The [coverage workflow](.github/workflows/coverage.yml) runs the install test
for every language on every platform each week and writes
[docs/coverage.md](docs/coverage.md).

## Releasing

Each change adds its changelog entry as its own file,
`changelog.d/<branch>.md`, not by editing `CHANGELOG.md`, so parallel pull
requests don't conflict ([changelog.d/README.md](changelog.d/README.md)). A
change users can't see adds none and is labelled `no changelog`. To release, from an up-to-date `main`:

```bash
libexec/maintenance/release.sh 0.4.0 --dry-run   # the checks, the changes and the notes
libexec/maintenance/release.sh 0.4.0
```

It checks the tree and that there are changes to release, and runs `hct self test`, then bumps
`VERSION`, moves the fragments into a dated changelog section and updates the README's status line
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

