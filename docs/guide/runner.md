# The parallel runner

`hct test`, `lint`, `build`, `clean`, `format`, `deps`, `audit`,
`outdated`, `docs` and `bench` with no language named run every language at once. The runner
decides how many jobs run together and in what order, from the machine it
runs on and from how long each job took before, so a run gives its answers
as early as it can and finishes as soon as it can.

```bash
hct test             # every language, in parallel
hct test go rust     # these two, one after the other, output streamed
hct test -j 8        # at most 8 jobs at once
```

## How many jobs run at once

By default, half the machine's logical CPUs: most toolchains run their
own work in parallel (Gradle, Cargo, `go test`, pytest-xdist), so one job
per CPU would oversubscribe the machine and make every job slower.
`-j` changes it:

| `-j` | Jobs at once |
| --- | --- |
| `50%` (the default) | Half the logical CPUs |
| `1%`–`200%` | That share of the logical CPUs (`100%` = one per CPU) |
| `8` | Exactly 8 |
| `pcores` | The performance cores, less one: on Apple Silicon and on Linux with big and little cores (Intel P- and E-cores, ARM big.LITTLE), jobs stay off the slow cores, and one is left for the rest of the machine; elsewhere, the CPU count less one |
| `unlimited` | Every job at once |

The CPUs are counted where hecatron runs (macOS's `sysctl`, Linux's
`nproc`), so the same command makes good use of a laptop, a 64-core
workstation and a 4-CPU CI runner.

## Which job runs next

Each run's durations and exit statuses are kept (`<logs>/<action>/history.log`,
the last 50 runs), and the next run uses them:

1. **Failures first.** Jobs that failed last time start first, so a fix is
   confirmed, or the failure seen again, in the first moments of the run.
2. **Longest first.** Among the rest, the job expected to take longest
   starts first (expected = the median of its last 5 runs). Starting long
   jobs early keeps the run from ending with one long job running alone,
   so the whole run finishes sooner.
3. **A fast lane.** One slot does the opposite and takes the shortest job
   waiting, so quick results keep arriving while the long jobs run. It
   still takes a failed job first.

A job with no history counts as short, so a new language gets a slot early.
Two runs of the same action in one project can't overlap: the second stops
with an error naming the first, so they don't overwrite each other's logs.

## What you see

On a terminal, a live display redraws ten times a second:

- **A row per job**: a spinner while it runs, ✓ or ✗ when it ends, its time,
  and a bar placed on the run's time axis.
- **Swim lanes** under it: one row per slot, showing which job ran in it and
  when, so idle slots and the job that made the run long are easy to spot.
- **Three time scales** (`-s`): `scroll` (the default, a fixed scale that
  scrolls), `full` (the whole run always fits), `zoom` (from the start,
  zooming out as the run grows).
- With more jobs than the terminal has rows, `j`/`k` or the arrow keys
  scroll the list.

Off a terminal (CI, a pipe), it prints one summary when the run ends.

Each job's whole output is saved to `<logs>/<action>/<language>.log`
(`.logs/` by default), so the display stays readable and nothing is lost.

## Beyond languages

- **Project tasks** from `.hecatron/config.json` (`"tasks": {"test":
  {"codegen": "..."}}`) run in the same run, scheduled like languages.
- `-c COMPONENT` runs one component's jobs; `hct disable LANG` leaves a
  language out of your runs, for you only.
- `hct parallel NAME "job" ...` runs your own list of jobs with the same
  scheduling and display.

## Time limits

A hung job (a deadlock, a test waiting on input or the network) is stopped
instead of holding the run, or the agent waiting on it, forever.

- **By default, from the job's own history:** 5× the median of its last 10
  passing runs in `history.log`, at least 10 minutes; a job with no passing
  run recorded has no limit. One job's time on one machine swings with
  load, `-j`, cold caches and throttling (2–3× is routine), so 5× seldom
  stops a healthy job; short jobs swing the most (a 5-second job that
  downloads or recompiles after a clean takes minutes), hence the floor.
  Failed runs are left out (a failure can stop early) and so are timed-out
  ones (they ran to the limit). A run with `-c` records the language's
  time for those components only, which counts too.
- **`--timeout DURATION`** (`90s`, `30m`, `1h30m`, or seconds as a bare
  number), else **`HECATRON_TIMEOUT`**, sets one limit for every job instead;
  `--timeout 0` turns limits off.
- **At the limit** the job's process group gets SIGTERM, then SIGKILL five
  seconds later if anything is left, so the hook's children (mvn, a test
  JVM, `sleep`) stop with it. On Linux, processes that left the group
  (`setsid`, a daemon such as Gradle's, started by the job) are found by a
  token in their environment and stopped too; on macOS they are not. On
  Windows only the hook's own process is stopped (a job object would stop
  the rest; not done yet).
- The job is reported as **timed out**, apart from failures, with its limit
  and where the limit came from, in every output: `TIMEOUT go (go/) stopped
  at its 10m limit (the 10m minimum ...)` in agent output, `"status": "timed
  out"` and `"timeLimit"` in JSON, `✗ ... timed out` in human output, an
  annotation in CI. Its log ends with the same line; `history.log` records
  it with status 124, as `timeout(1)` exits. The run exits 1.
- Named runs (`hct test go`) take their limits from the parallel runs'
  history too. The bash runner (`run-parallel.sh`, used without the binary)
  has no time limits.

## Only what changed

`hct test --changed [REF]` (also `lint`, `build`, `format`, `deps`,
`rebuild` and `retest`) runs only the languages, components and project
tasks that the files changed since `REF` touch.

- **The base** is the merge base of `HEAD` with `REF`; with no `REF`, in CI
  (`CI` or `TF_BUILD` set), the pull or merge request's base:
  `GITHUB_BASE_REF`, `CI_MERGE_REQUEST_DIFF_BASE_SHA`,
  `BITBUCKET_PR_DESTINATION_BRANCH` or `SYSTEM_PULLREQUEST_TARGETBRANCH`
  (`origin/<branch>`, else the local branch); else the default branch
  (`origin/HEAD`, else `origin/main`, `origin/master`, `main`, `master`).
  A CI run outside a pull request, a base that isn't in a shallow clone, or
  no merge base within its history runs everything, with a one-line note;
  hecatron fetches nothing. A `REF` that names no commit is a usage error.
- **The changes** are git's diff from the base to the working tree
  (committed, staged and not; a rename as both paths) and the untracked
  files `.gitignore` doesn't ignore.
- **What a changed file selects:**

  | File | Selects | Why |
  | --- | --- | --- |
  | In a language's directory (`hct list`; the deepest, as `hct status` maps changes) | That language, that component | Its code |
  | Under `.hecatron/languages/<lang>/` | That language, every component | Its hooks or definition |
  | Anything else under `.hecatron/` (`config.json`) | Everything | The layout, languages, tasks or settings may have changed |
  | A build or version file a definition names (`detect`, `sharedBuildFiles`, `pinFiles`, `supportFiles`), above the language's directories | That language, in the components below it; every component for a path relative to the root (`.github/workflows/*.yml`) | A root `CMakeLists.txt` or `.tool-versions` changes how they build or which toolchain runs |
  | A shared build, workspace or lock file (`settings.gradle.kts`, `gradle/`, `go.work`, `Cargo.lock`, `package-lock.json`, `pnpm-workspace.yaml`, `uv.lock`, `Directory.Build.props`, `*.sln`, `.editorconfig`...) | Every language and component below it | Any of them may build with it |
  | A shell script (`*.sh`, `*.bash`) | `hct lint`'s shell check | |
  | Anything else (docs, CI files no definition reads) | Nothing | |

- **Project tasks** run when a changed file matches one of the paths
  config.json's `"taskPaths"` lists for them (`{"codegen": ["codegen/",
  "*.proto"]}`: a directory, a glob on the name, or a glob on the path
  from the root), and, with none listed, whenever any file changed: a task
  may read anything.
- `-c` narrows the selection further; naming languages with `--changed` is
  a usage error (it picks them). `audit` and `outdated` don't take it:
  their results follow advisories and releases, not the files.
- **Output:** a line saying what was compared and what runs and why comes
  first (human, CI and agent output), e.g. `Changed since origin/main (the
  default branch, merge base 1a2b3c4d5e): 3 files; running go [age]
  (age/go/x.go +1), python (.python-version: a build or version file above
  it)`; JSON output has it as `"changed"`. With nothing selected, that line
  alone, and the exit status is 0.
- The bash runner (`run.sh`, without the binary) has no `--changed`.

## After a pull: `hct sync`

`hct sync [REF]` says, in a few lines, what the working tree moving (a
pull, a rebase, a branch switch) means for hecatron, and does the safe part:

```
hct sync: since 3f2a1c9e01 (HEAD@{1}, 12 commits): 31 files changed
  deps: typescript (web/package-lock.json), python [api] (api/uv.lock): fetched
  toolchain: go (go/go.mod): needs 1.27.4, not installed; hct install go installs it (ask the user first)
  check: .hecatron/config.json changed; hct check passed
```

- **The base** is `REF`, else where `HEAD` was before it last moved: the
  reflog's `HEAD@{1}`, or, when the last moves were a rebase's steps, where
  `HEAD` was before the rebase started. With no such entry (a fresh clone),
  it says there's nothing to compare with and exits 0. The changes are the
  files that differ between the base and `HEAD` (uncommitted changes are
  yours, not the pull's). It reads git only: it never pulls, rebases or
  fetches.
- **A lock file changed** (a definition's `cache.lockFiles`: `uv.lock`,
  `package-lock.json`, `go.sum`...) in a component's directory, below it,
  or above it (a workspace's): `hct deps` runs for those languages and
  components, through the runner (its logs, history and time limits; the
  live display on a terminal). `--no-deps` says what to run instead.
- **A pin or support file changed** (`pinFiles`, `supportFiles`, looked for
  from each component's directory up to the root), or the language's
  definition under `.hecatron/languages/<lang>/` (its project `version`):
  the version each of those components now needs and whether it's
  installed (the definition's `installed.sh`). It never installs: a
  missing version suggests `hct install <lang>`, marked "ask the user
  first", as `hct status` marks it.
- **`.hecatron/` changed** (`config.json`, hooks, definitions, templates;
  not `local.json`), or a file hecatron generates (the Dockerfile,
  workflows, `.gitlab-ci.yml`, Dependabot or Renovate configuration), or a
  version when the project generates files: `hct check` runs, and its
  output is shown when it fails.
- **Nothing relevant:** one line saying so.
- `--output human|agent|json` is chosen as for `hct test`; JSON output
  holds the base, the files, each part's result and `hct deps`'s own JSON
  report. Exit status 1 when `hct deps` or `hct check` fails, 2 for a usage
  error (a `REF` that names no commit), else 0. Needs the binary.

## With the Go binary

The Go core ([design record §19](../contributing/design.md#19-a-go-core-and-native-windows)), built from a checkout, has the same
scheduling and history, and adds output for whoever reads it:
`--output human|agent|json|ci` (or `HECATRON_OUTPUT`), and reports for CI
(below). Agent output is a
summary line and only the failures, each with its log path and the error
lines that matter, under a third of the size of the human summary; CI
output adds GitHub annotations at each error's file and line. Its live
display has no swim lanes yet.

## Reports

With the binary, a run also writes what CI systems and forges read
([hecatron in CI](ci.md)), alongside its normal output, so one run feeds both
the log and the forge:

| Report | Commands | `HECATRON_REPORT_DIR` file | On stdout instead |
| --- | --- | --- | --- |
| JUnit XML | test, lint, build | `<action>.junit.xml` | `--output junit` (any run command) |
| SARIF 2.1.0 | lint, audit | `<action>.sarif` | `--output sarif` |
| GitLab Code Quality JSON | lint | `lint.codequality.json` | |
| GitHub Actions job summary | every run command, `licenses`, `advisor` | appended to `GITHUB_STEP_SUMMARY` | |

A run of named targets adds them to the name (`hct test go` writes
`test-go.junit.xml`), so the commands one CI job runs don't overwrite each
other's files. A relative `HECATRON_REPORT_DIR` is relative to where hct
runs. Writing a report never changes the exit status; a failure to write
one is a warning. The scripts without the binary write none.

**JUnit:** one `<testsuite>` per language and component (`go/age`), and
per project task, tool, and language skipped as not installed or
disabled. Where the tool reports its own tests, they are the suite's test
cases (a failed component none of whose cases failed, such as a build
error before any test ran, adds a case for the component); otherwise the
suite has one case for the component, its failure's text the component's
error lines and log. Merged today:

- **Maven (Surefire, Failsafe), Gradle and sbt:** any `TEST-*.xml` under a
  `surefire-reports`, `failsafe-reports`, `test-results` or `test-reports`
  directory in the component, written while its tests ran (by any
  language: Java, Kotlin, Scala, Groovy...).
- **Go:** asked for JUnit, the default test hook runs `go test -json`
  through `hecatron _gotest`, which saves the events and prints what `go
  test` without `-json` would (each failed test's lines, the package
  results), so the log and the error lines are unchanged.
- **pytest:** the default test hook adds `--junitxml`.
- **Any hook:** the test hooks get `HECATRON_TEST_REPORTS`, an empty
  directory, when JUnit is wanted; JUnit XML (`*.xml`: Jest's `jest-junit`,
  Vitest's `--reporter=junit`, PHPUnit's `--log-junit`, `dotnet test
  --logger junit`...) or `go test -json` events (`*.json`) a hook writes
  there are merged.

Not merged yet by the default hooks: Jest, Vitest, .NET, PHPUnit, RSpec,
Cargo and the rest (one case per component).

**SARIF and Code Quality** (lint): each error line in a failed lint job's
log that names a file and line (the language's error patterns, else the
generic ones; every one, not the excerpt's ten), at its file relative to
the workspace (`GITHUB_WORKSPACE`, `CI_PROJECT_DIR`, else the project
root), as an `error` under the rule `lint/<language>`; in Code Quality, a
`major` issue with a fingerprint. Audit's SARIF is in
[audit.md](audit.md#in-ci).

**Job summary:** when `GITHUB_STEP_SUMMARY` names a file, each run
appends Markdown: its summary line, a table of languages and components
with their status and time, then each failure with its error lines (agent
output's), its log and the command that reruns them; `hct audit` adds a
table of findings, `hct licenses` its violations and licenses, `hct
advisor` its findings.

The pipeline `hct workflows` writes for GitLab sets
`HECATRON_REPORT_DIR` and hands each job's `test-<lang>.junit.xml` and
`lint-<lang>.codequality.json` to the merge request's test and code
quality widgets (`artifacts:reports`). On GitHub, upload SARIF with
`github/codeql-action/upload-sarif` and show JUnit with a reporting action.

[libexec/runner/README.md](../../libexec/runner/README.md) documents the bash
runner's internals: its flags, history format and display code.
