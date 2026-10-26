# run-parallel.sh

A bash script that runs jobs (hecatron commands, or just recipes when run standalone) in parallel with live progress display, history-based scheduling, and interactive swim lane visualization.

## Overview

`run-parallel.sh` takes a list of commands, runs them concurrently, and renders a live-updating bar chart as they complete. In slot-based mode (`-j N`), it adds swim lane timelines, error-first priority, and duration prediction from run history.

**Design goals:**
- Zero dependencies beyond bash 4+ and perl (for sub-second timing)
- Single-file, self-contained — no external libraries or frameworks
- Works in both interactive terminals and CI/piped environments
- Handles "not installed" entries gracefully (the `skip:` protocol)

## Quick start

```bash
# Run 3 jobs with default concurrency (nproc/2 slots)
./.hecatron/tools/runner/run-parallel.sh test "test go" "test rust" "test python"

# Limit to 2 concurrent slots
./.hecatron/tools/runner/run-parallel.sh -j 2 test "test go" "test rust" "test python"

# All jobs at once (no swimlanes)
./.hecatron/tools/runner/run-parallel.sh -j unlimited test "test go" "test rust" "test python"

# Mark a language as not installed
./.hecatron/tools/runner/run-parallel.sh test "test go" "skip:haskell" "test rust"
```

## Command-line reference

```
Usage: run-parallel.sh [flags] <name> "cmd args" ...
```

### Arguments

| Argument         | Description                                                     |
| ---------------- | --------------------------------------------------------------- |
| `<name>`         | Log directory name — output goes to `.logs/<name>/`             |
| `"cmd args" ...` | Commands passed to the job command (`HECATRON_JOB_COMMAND`, default `just`). `"label::cmd args"` names a job; otherwise its last word is the label, which also names its log file. |
| `"skip:<label>"` | Shown as "not installed" in output position, no job launched.   |

### Flags

| Flag       | Description                                                                    |
| ---------- | ------------------------------------------------------------------------------ |
| `-j VALUE` | Set concurrency: integer, percentage (default `50%`), `pcores`, or `unlimited` |
| `-n NICE`  | Run jobs under `sudo nice -n NICE` (e.g. `-n -20` for highest CPU priority)    |
| `-s MODE`  | Visualization mode: `scroll` (default), `full`, or `zoom`                      |
| `-h`       | Show help and exit                                                             |
| `--help`   | Show help and exit                                                             |

## Scheduling

By default the runner uses slot-based scheduling with 50% of available CPUs. Use `-j unlimited` for the legacy behavior of launching all jobs at once. Percentages from `1%` to `200%` are supported (e.g. `-j 100%` = all CPUs, `-j 150%` = 1.5x CPUs). Use `-j pcores` for performance-core-aware scheduling (P-cores minus 1), ideal for benchmarks on hybrid CPU architectures (Apple Silicon, Intel P+E cores, ARM big.LITTLE).

### Queue sorting

Jobs are sorted by a composite key: **errors first, then predicted duration descending**. This means:

1. Jobs that failed in the most recent run are scheduled first
2. Among equally-errored jobs, longer jobs start first (to minimize wall time)

### Fast slot

Slot 0 is the **fast slot**. Instead of picking the longest unstarted job (like slots 1..N), it picks the **shortest** available job. This ensures quick jobs finish and report results early, giving fast feedback while long jobs occupy the other slots.

The fast slot still prioritizes errors: if any errored job is available, it picks the shortest errored job.

### History-based prediction

Duration predictions come from `.logs/<name>/history.log`. The runner takes the **median** of the last 5 recorded durations for each job. Jobs with no history get a predicted duration of 0 (treated as shortest).

### Error tracking

The runner reads the most recent line of `history.log` to find jobs that exited with non-zero return codes. These jobs get priority in the next run via the error-first sort.

## Visualization modes

All three modes display a bar chart with one row per job. In slot-based mode, a swim lane timeline is appended below the bar chart.

### scroll (default)

Fixed time scale (0.2s per column). The visible window scrolls right as time advances past the terminal width. Bars that scrolled off-screen are no longer visible.

```
  ✓ go         1.2s ████
  ⠙ rust       3.4s ██████████████████
  ⠙ python     2.8s ██████████████
                    0    1    2    3    4
  go        ████
  rust      ██████████████████████████████
  python    ███████████████████████
```

### full

The entire run fits in the terminal width. The time scale rescales every frame so the rightmost bar always reaches the edge. Early in the run the scale is coarse; it refines as more time passes.

### zoom

Fixed origin (left edge is always t=0). The time scale starts at 0.2s/column and **doubles** whenever the chart would overflow the terminal width. Provides a complete view but with decreasing resolution over long runs.

## Interactive display

When the output is a terminal (tty), the runner renders a live-updating display at 10 FPS:

### Bar chart

Each job gets one row showing:
- **Status icon**: `⠋` spinning (running), `✓` pass (green), `✗` fail (red), `◌` queued, `-` skipped
- **Color block**: 2-char swatch in the job's assigned color (slot-based mode only)
- **Label**: job name, left-aligned
- **Duration**: elapsed or final time in `X.Ys` format
- **Bar**: proportional bar showing start/end position on the time axis

### Swim lanes (slot-based only)

Below the bar chart, a horizontal timeline shows which job occupied each slot over time:
- Each slot is one row with a slot icon aligned with the bar chart status icons
- The slot icon reflects the nice priority: 🔥 (negative nice / high priority), 🧊 (positive nice / low priority), ⚙ (default)
- Active slots show a bright yellow icon, idle slots show a grey icon
- Colored blocks show which job was running in that slot at each point
- A time axis with second labels appears above the lanes

### Non-interactive output

When piped or in CI, the runner waits for all jobs to complete, then prints a static bar chart with proportional bars scaled to the longest job.

## Log format

### Directory structure

```
.logs/<name>/
  <label>.log        Captured stdout+stderr for each job
  <label>.result     Internal: "duration_ds return_code"
  history.log        Duration history for prediction
  .lock/             Directory-based lock (prevents concurrent runs)
```

### history.log format

One line per run, space-separated `label=duration_ds:return_code` pairs:

```
go=12:0 rust=34:0 python=28:0
go=11:0 rust=35:1 python=27:0
```

- `duration_ds` is in deciseconds (1 = 0.1s)
- `return_code` is the exit code of the job
- Last 50 runs are retained (older entries are pruned)

### .result files

Temporary files written by each job subprocess. Format: `duration_ds return_code`. Cleaned up on exit.

## Integration

### Calling from a justfile

The runner is designed to be called from justfile recipes. The typical pattern:

```just
test *lang:
    #!/usr/bin/env bash
    # ... parse -j/-s flags and language args ...
    if [ -z "$_langs" ]; then
        # Parallel: build args array, call runner
        .hecatron/tools/runner/run-parallel.sh ${_jflag:+-j "$_jflag"} test "${args[@]}"
    else
        # Single language: run directly with tee
        (cd src/$_langs && run_tests) 2>&1 | tee ".logs/test/$name.log"
    fi
```

### The skip: protocol

Prefix an argument with `skip:` to mark it as not installed:

```bash
args+=("skip:haskell")  # instead of "test haskell"
```

The runner displays skipped entries in their original position as dimmed "not installed" lines. They don't count toward pass/fail totals.

### Label extraction

The display label is the **last word** of each command argument:
- `"test go"` → label: `go`
- `"build typescript"` → label: `typescript`
- `"skip:haskell"` → label: `haskell`

### Flag forwarding

The justfile recipes parse `-j`, `-n`, and `-s` from their own arguments and forward them:

```bash
.hecatron/tools/runner/run-parallel.sh ${_jflag:+-j "$_jflag"} ${_nflag:+-n "$_nflag"} ${_sflag:+-s "$_sflag"} test "${args[@]}"
```

## Requirements

| Requirement        | Purpose                                  | Required |
| ------------------ | ---------------------------------------- | -------- |
| bash 4+            | Associative-style arrays, `${!arr[@]}`   | Yes      |
| perl (Time::HiRes) | Sub-second timing (decisecond precision) | Yes      |
| tput               | Terminal width detection, cursor control | No       |

`tput` is optional — without it, the terminal width defaults to 80 columns and the interactive display still works (cursor movement uses ANSI escapes directly).
