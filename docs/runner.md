# The parallel runner

`hct test`, `lint`, `build`, `clean`, `format`, `deps`, `audit` and
`outdated` with no language named run every language at once. The runner
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

## With the Go binary

The Go core (`docs/design.md` §19), built from a checkout, has the same
scheduling and history, and adds output for whoever reads it:
`--output human|agent|json|ci` (or `HECATRON_OUTPUT`). Agent output is a
summary line and only the failures, each with its log path and the error
lines that matter, under a third of the size of the human summary; CI
output adds GitHub annotations at each error's file and line. Its live
display has no swim lanes yet.

[libexec/runner/README.md](../libexec/runner/README.md) documents the bash
runner's internals: its flags, history format and display code.
