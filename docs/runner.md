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
