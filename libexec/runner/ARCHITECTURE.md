# run-parallel.sh — Architecture

Internal design documentation for contributors. For usage, see [README.md](README.md).

## Execution modes

The script has two top-level execution paths:

| Mode       | Condition                | Scheduling                 | Display                |
| ---------- | ------------------------ | -------------------------- | ---------------------- |
| Slot-based | Default (`50%` of CPUs)  | N concurrent slots, queued | Bar chart + swim lanes |
| Slot-based | `-j N`, `-j P%`          | N concurrent slots, queued | Bar chart + swim lanes |
| Unlimited  | `-j unlimited` or `-j 0` | All jobs start at once     | Bar chart only         |

The `-j` flag accepts: an integer, a percentage (`1%`–`200%`, default `50%`), or `unlimited`. Percentages are resolved to slot counts at startup via `_cpu_count()`. `100%` always equals the exact CPU count; other percentages round to the nearest core with a minimum of 1 slot.

Each mode has an interactive (tty) and non-interactive (piped/CI) sub-path:

```
max_jobs > 0?
├── yes (slot-based — default)
│   ├── tty?  → interactive loop with swim lanes (⚙ gear symbols)
│   └── no    → poll loop, print static bars at end
└── no (unlimited — via -j unlimited)
    ├── tty?  → interactive loop with bar chart
    └── no    → wait all, print static bars
```

## Data structures

The script uses parallel arrays indexed by job position (the order arguments were passed). These arrays maintain the original display order regardless of scheduling.

### Core arrays (all modes)

| Array        | Index | Contains                                |
| ------------ | ----- | --------------------------------------- |
| `all_labels` | pos   | Display label for each entry            |
| `all_cmds`   | pos   | Full command string (empty for skipped) |
| `all_skip`   | pos   | 1 if skipped, 0 if runnable             |
| `pids`       | —     | Append-only list of launched PIDs       |

### Slot-based arrays

| Array            | Index | Contains                                          |
| ---------------- | ----- | ------------------------------------------------- |
| `_run_queue`     | queue | Job indices sorted by priority                    |
| `_label_ci`      | pos   | Color index (stable, based on alphabetical order) |
| `_predicted_dur` | pos   | Predicted duration in deciseconds                 |
| `_error_last`    | pos   | 1 if job failed in most recent run                |
| `_slot_pids`     | slot  | Current PID in each slot (0 = empty)              |
| `_slot_labels`   | slot  | Current label in each slot                        |
| `_slot_ci`       | slot  | Current color index in each slot                  |
| `_slot_job`      | slot  | Current job index in each slot                    |
| `_job_started`   | pos   | 1 if job has been launched                        |
| `_job_start_ds`  | pos   | Decisecond timestamp when job started             |

### Interactive display arrays

| Array        | Index | Contains                        |
| ------------ | ----- | ------------------------------- |
| `done_flags` | pos   | Non-empty when job has a result |
| `done_durs`  | pos   | Final duration in deciseconds   |
| `done_rcs`   | pos   | Final return code               |

## Timing

All internal timing uses **deciseconds** (0.1s precision). This avoids floating-point arithmetic in bash while providing adequate resolution for the 10 FPS display.

```bash
_now_ds() { perl -MTime::HiRes=time -e 'printf "%d\n", time*10'; }
```

Why perl: bash's `$SECONDS` only has 1-second resolution. `date +%N` is not available on macOS. `perl -MTime::HiRes` is available everywhere perl is installed and gives microsecond precision, which we truncate to deciseconds.

Formatting back to human-readable:

```bash
_fmt_ds() { echo "$((${1} / 10)).$((${1} % 10))s"; }
```

## Swim lane recording

The swim lane chart is backed by a flat array indexed as a 2D grid:

```
_lane_ci[slot * _max_ds + decisecond] = color_index
```

- `_max_ds` is 6000 (10 minutes at decisecond resolution)
- `slot` ranges from 0 to `max_jobs - 1`
- Each cell stores the color index of the job occupying that slot at that decisecond, or is unset (empty)

Recording happens once per frame in the render loop:

```bash
# Fill in any deciseconds since last recording
for (( _rd=_last_rec_ds+1; _rd<=_cur_ds && _rd<_max_ds; _rd++ )); do
    for (( _s=0; _s<max_jobs; _s++ )); do
        if [ "${_slot_ci[$_s]}" -ge 0 ]; then
            _lane_ci[$((_s * _max_ds + _rd))]=${_slot_ci[$_s]}
        fi
    done
done
```

The forward-fill loop ensures no gaps between frames (the display runs at 10 FPS but jobs can start/stop between frames).

## Render loop

The interactive display runs at approximately 10 FPS (`sleep 0.1` per frame). Each frame:

1. **Check finished slots** — poll each slot PID with `kill -0`
2. **Launch new jobs** — fill empty slots from the queue (fast-slot logic for slot 0)
3. **Record timeline** — update `_lane_ci` for elapsed deciseconds
4. **Check results** — read `.result` files for newly completed jobs
5. **Update terminal width** — re-read `tput cols` every 10 frames
6. **Compute display parameters** — mode-specific offset/scale calculations
7. **Render bar chart** — move cursor up, redraw all rows with `\033[K` line clears
8. **Render swim lanes** — call `_render_lanes` (slot-based mode only)
9. **Check completion** — break if `done_count == run_n`

The cursor is hidden (`\033[?25l`) during rendering and restored (`\033[?25h`) on exit.

## Display parameter computation

Each visualization mode computes different parameters that control how time maps to columns:

### full mode

```bash
_total_ds = current_elapsed_ds + 1   # rescales every frame
column = time_ds * max_bar / _total_ds
```

The entire run always fits. Resolution improves as the run progresses.

### scroll mode

```bash
_eff_tick_ds = _tick_ds              # fixed at 2 (0.2s per column)
_display_offset_ds = max(0, (cur_ds / tick - max_bar + 1) * tick)
column = (time_ds - _display_offset_ds) / _eff_tick_ds
```

Fixed scale. The window shifts right when the current time would exceed `max_bar` columns.

### zoom mode

```bash
_eff_tick_ds = _tick_ds              # starts at 2
while (cur_ds / _eff_tick_ds >= max_bar):
    _eff_tick_ds *= 2                # double scale when overflow
column = time_ds / _eff_tick_ds
```

Fixed origin (offset always 0). Scale doubles as needed. The time axis always starts at 0 but resolution halves each time.

## History system

### File format

`.logs/<name>/history.log` stores one line per run:

```
go=12:0 rust=34:0 python=28:1
```

Each pair is `label=duration_ds:return_code`. Space-separated, all jobs on one line.

### Duration prediction

On startup, the runner reads the last 5 lines and extracts durations per label. It computes the **median** (middle element of sorted list) as the predicted duration. This is more robust than mean — a single slow run doesn't skew predictions.

### Error tracking

Only the **most recent** history line is checked for errors. Any job with a non-zero return code gets `_error_last[i]=1`, which the queue sort uses to promote it.

### Retention

After each run, the history file is pruned to the last 50 lines:

```bash
tail -50 history.log > history.log.tmp && mv history.log.tmp history.log
```

## Locking

Concurrent runs of the same command are prevented with a directory-based lock:

```bash
lockdir="$logdir/.lock"
mkdir "$lockdir"  # atomic on all filesystems
```

### Stale lock recovery

If the lock directory exists but is older than 2 minutes (checked via `find -mmin +2`), it's assumed stale and removed. This handles cases where the script was killed without cleanup.

### Cleanup

The lock is released in the `EXIT` trap:

```bash
cleanup() {
    # ... kill children, remove .result files ...
    rmdir "$lockdir" 2>/dev/null
}
trap cleanup EXIT
```

The trap also kills all child processes and removes temporary `.result` files. A separate `INT` trap ensures `exit 130` propagates correctly on Ctrl-C.

## Color palette

The palette has 20 distinct 256-color values chosen for visual separation:

```bash
_palette=(196 46 33 208 129 214 39 161 82 166 63 203 48 99 215 35 197 118 57 172)
```

Color assignment is **stable**: each job gets a color based on its position in the alphabetically-sorted queue (`_label_ci`), not its slot. This means the same job always gets the same color across frames and across runs, making it easy to track visually.

## Pre-built block string

To avoid repeated string concatenation in the hot render path, a 512-character block string is pre-built at startup:

```bash
blocks=""
for (( _i=0; _i<512; _i++ )); do blocks+="█"; done
```

Bars are rendered by slicing this string: `${blocks:0:$len}`.
