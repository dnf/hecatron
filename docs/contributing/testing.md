# Testing

## Checks

Before sending a change, run:

```bash
hct self test    # ShellCheck with zero warnings, then the unit tests
```

CI runs the same checks on Ubuntu and macOS, and the unit tests again under
bash 3.2, the version macOS ships.

| Suite | Where | Runs |
| --- | --- | --- |
| ShellCheck | `libexec/lint/shellcheck.sh` | Every tracked shell script, zero warnings |
| Python unit and command tests | `tests/test_*.py` | `HECATRON_NO_GO=1 python3 -m unittest discover -s tests`: the scripts, on projects built in temporary directories; also the checks that read the docs (the language count, [languages.md](../guide/languages.md), the skill reference, [coverage.md](../guide/coverage.md)) |
| Go tests | `cmd/hecatron/*_test.go`, `internal/*/` (fixtures in `testdata/`) | `go test ./...`, including the parity tests below |
| Hook fixtures | `tests/hooks/fixtures/<lang>/<fixture>/`, `tests/hooks/check.sh` | A small library per build system with a real test, run through test, lint, build and clean (in Docker) |
| Dependency fixtures | `tests/audit/`, `tests/outdated/`, `tests/deps/`, `tests/format/`, `tests/licenses/` | Audit, outdated, deps, format and licenses on fixture projects, and captured reports |
| Recorded tool output | `tests/output/<tool>/<version>/` | Logs and reports captured from real tool versions (`tests/output/capture.sh`, `capture-deps.sh`), each failing log with the excerpt it must give |
| Install tests | `tests/install/check.sh`, `tests/docker/install-test.sh`, `tests/docker/smoke/<lang>.sh` | A toolchain installed in a fresh container, checked from a login shell, reinstalled, and a smoke test run |
| Install parity | `tests/docker/parity.sh`, `tests/docker/parity/<lang>/` | A language installed from its scripts and from its data must end up the same ([install engine](install-engine.md#parity)) |
| hecatron's installer | `tests/installer/check.sh` | `install.sh` in a container (it edits startup files) |
| Coverage | `tests/coverage/`, [`.github/workflows/coverage.yml`](../../.github/workflows/coverage.yml) | Every language's install test on every platform, weekly, written to [coverage.md](../guide/coverage.md) |
| Schemas | `tests/schemas/` | The JSON schemas generated files are checked against (GitLab CI, Dependabot, SARIF) |

A downstream job that checks out sayable and runs `hct check` and `hct test
go python` against it, so a change that breaks the project hecatron came
from fails CI, is planned but not in CI yet.

Real open-source projects (`tests/real-projects/`): a backlog of about
150 public repositories (`projects.json`: polyglot projects, a well-known
project or two per language, and monorepos), each cloned in a fresh
container and run as a newcomer would, `hct init`, `install`, `doctor`
and `test <lang>` with no configuration (`run.sh`). A monthly workflow
(`.github/workflows/real-projects.yml`) runs one job per project on
GitHub's runners and opens a pull request with the results:
`results.json` (each run: commit, detection, toolchain versions, per-step
exit codes and times, test counts, failure category and excerpt) and a
flat `results-by-language.json`, both described by `schema.json`, and
`docs/real-projects.md` rendered from them (`report.py`). It measures what
detection and the default hooks get right on code hecatron has never
seen. `report.py` also writes a shields.io endpoint file per tested
project (`badges/OWNER_REPO.json`: passing, N/M languages or failing), so
a project can show a badge that follows its latest result without running
anything itself; the page lists the Markdown for each passing project.

### What CI runs

A pull request runs only the CI jobs its changes can affect: the first job,
`changes`, lists the paths it changes against its base, and
[`libexec/maintenance/ci-jobs.sh`](../../libexec/maintenance/ci-jobs.sh) maps
them to jobs (documentation alone runs just the unit tests, a Go change the
Go jobs, an installer change the installer tests; a workflow change or a
path it doesn't know runs everything). Its tests,
[`tests/test_ci_jobs.py`](../../tests/test_ci_jobs.py), show the mapping. A push
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

### Docker tests

A change to a language's installer, doctor or hooks should also pass that
language's Docker tests. They install the toolchain in a throwaway container
and never change your machine:

```bash
tests/docker/install-test.sh ubuntu:24.04 go     # install, doctor, reinstall, smoke test
tests/docker/run.sh ubuntu:24.04 bash /hecatron/tests/hooks/check.sh go
                                                  # test, lint, build and clean on fixture projects
```

A language with an `install.json` should also install the same both ways.
`tests/docker/parity.sh ubuntu:24.04 raku racket` installs each language in
two fresh containers, once with its scripts and once from its data, and
fails unless both pass `tests/install/check.sh` and end with the same
packages, commands and doctor report.

Changes to hecatron's own installer, `install.sh`, have a test that edits a
user's startup files, so it too runs only in a container (CI also runs the
installer on macOS):

```bash
tests/docker/run.sh ubuntu:24.04 bash /hecatron/tests/installer/check.sh
```

## The Go core

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

### Parity tests

The tests compare the Go project model with `libexec/lib/project.py`, and
the binary's output with `bin/hecatron`'s, byte for byte, on fixture
projects; those comparisons need bash and Python and are skipped on Windows.
The report readers (`internal/deps`) are compared with `audit.py` and
`outdated.py` on every captured report and thousands of variants of them;
the generators and the project commands on fixture repositories, every
file written included; detection on clones of real projects. Where the Go
model declines a project, the test checks that the command was delegated.

The [coverage workflow](../../.github/workflows/coverage.yml) runs the install test
for every language on every platform each week and writes
[coverage.md](../guide/coverage.md).
