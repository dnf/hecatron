#!/usr/bin/env bash
# Runs jobs (hecatron or just commands) in parallel with live progress display.
# Usage: run-parallel.sh [-j N|P%|pcores|unlimited] [-n NICE] [-s full|zoom] <name> "cmd" ...
# -j VALUE limits concurrent jobs (default: 50% of CPUs).
# -n NICE runs jobs under sudo nice -n NICE (e.g. -n -20).
# -s MODE sets visualization: scroll (default), full, or zoom.
# <name> is used for the log directory: .logs/<name>/
# Each subsequent argument is passed to the job command (HECATRON_JOB_COMMAND,
# default just) as a complete command.
# The label shown in output (and naming the log file) is the part before "::"
# in "label::command", or else the command's last word.
# Entries prefixed with "skip:" are shown as "not installed" in their
# original position without launching a job.
set -uo pipefail

# --- ANSI color codes ---
_c_reset=$'\033[0m'
_c_dim=$'\033[2m'
_c_green=$'\033[32m'
_c_red=$'\033[31m'
_c_yellow=$'\033[93m'
_c_orange=$'\033[38;5;208m'
_c_gray=$'\033[90m'
_c_erase=$'\033[K'
_c_bg256=$'\033[48;5;'   # append "${color}m"
_c_fg256=$'\033[38;5;'   # append "${color}m"
_c_lane_idle=$'\033[48;5;236m'

# --- Semantic colors (built from base colors) ---
_s_pass="$_c_green"
_s_fail="$_c_red"
_s_run="$_c_yellow"
_s_disable="$_c_orange"
_s_skip="$_c_dim"
_s_queued="$_c_dim"
_s_idle="$_c_gray"
_s_scroll="$_c_dim"

max_jobs=50%
_nice_val=""
_viz_mode=scroll  # scroll (default), full, or zoom
_tick_ds=2        # base: 0.2s per column (for scroll/zoom modes)
_show_help() {
    cat <<'HELP'
Usage: run-parallel.sh [flags] <name> "cmd args" ...

Run jobs (just recipes by default) in parallel with live progress display.

Arguments:
  <name>            Log directory name — output goes to .logs/<name>/
  "cmd args" ...    Commands passed to the job command (HECATRON_JOB_COMMAND,
                    default `just`). Write "label::cmd args" to name a job;
                    otherwise its last word is the label. Prefix with
                    "skip:" to mark as not installed (e.g. "skip:haskell").

Flags:
  -j VALUE          Set concurrency: integer, percentage (default 50%),
                    'pcores' (P-cores minus 1), or 'unlimited' (all at once)
  -n NICE           Run jobs under sudo nice -n NICE (e.g. -n -20)
  -s MODE           Visualization mode: scroll (default), full, or zoom
  -h, --help        Show this help and exit

Scheduling (slot-based, the default and -j N where N > 0):
  Jobs are queued by predicted duration (longest first) with errors from
  the previous run promoted to the front. Slot 0 is the "fast slot" — it
  picks the shortest available job so quick jobs finish and report early.
  Duration predictions use the median of the last 5 runs from history.log.

Visualization modes:
  scroll    Fixed time scale, window scrolls right as time advances (default)
  full      Entire run fits in the terminal width, rescales every frame
  zoom      Fixed origin, time scale doubles when the chart would overflow

Output format:
  Interactive (tty)     Live-updating bar chart with spinner, color-coded bars,
                        and swim lane timeline (slot-based mode only)
  Non-interactive       Waits for completion, then prints a scaled bar chart

Log files:
  .logs/<name>/<label>.log       Captured stdout+stderr for each job
  .logs/<name>/<label>.result    "duration_ds return_code" (internal)
  .logs/<name>/history.log       Duration history for prediction (last 50 runs)

Requirements:
  bash 4+, perl (Time::HiRes for sub-second timing), tput (optional, for
  terminal width detection and cursor control)
HELP
    exit 0
}

while [ $# -gt 0 ]; do
    case "$1" in
        -h|--help) _show_help ;;
        -j) case "${2:-50%}" in
                unlimited|0) max_jobs=0 ;;
                pcores) max_jobs=pcores ;;
                *%)
                    _pct="${2%\%}"
                    if ! [ "$_pct" -ge 1 ] 2>/dev/null || ! [ "$_pct" -le 200 ] 2>/dev/null; then
                        echo "error: -j percentage must be 1%-200% (got '$2')" >&2; exit 1
                    fi
                    max_jobs="$2" ;;
                *[!0-9]*)    echo "error: -j accepts integer, percentage (1%-200%), 'pcores', or 'unlimited'" >&2; exit 1 ;;
                *)           max_jobs="$2" ;;
            esac; shift 2 ;;
        -n) _nice_val="${2:--20}"; shift 2 ;;
        -s) case "${2:-}" in
                full)    _viz_mode=full ;;
                scroll)  _viz_mode=scroll ;;
                zoom)    _viz_mode=zoom ;;
                *) echo "error: -s accepts full, scroll, or zoom (got '${2:-}')" >&2; exit 1 ;;
            esac; shift 2 ;;
        *) break ;;
    esac
done

if [ $# -lt 2 ]; then echo "Usage: $0 [-j N|P%|pcores|unlimited] [-n NICE] [-s full|zoom] <name> 'cmd args'..." >&2; exit 1; fi

name="$1"; shift
# Each job is "<job_cmd> <job>": hecatron sets HECATRON_JOB_COMMAND to itself;
# run standalone, the runner keeps its original behaviour of calling just.
job_cmd="${HECATRON_JOB_COMMAND:-just}"
logdir="${HECATRON_LOGS:-.logs}/$name"
mkdir -p "$logdir"
# If running under sudo, fix ownership so non-sudo runs aren't blocked
if [ -n "${SUDO_USER:-}" ]; then
    chown -R "${SUDO_USER}" "$logdir"
fi

# Prevent concurrent runs of the same command
lockdir="$logdir/.lock"
if ! mkdir "$lockdir" 2>/dev/null; then
    # Clean up stale locks older than 2 minutes
    if find "$lockdir" -maxdepth 0 -mmin +2 2>/dev/null | grep -q .; then
        rmdir "$lockdir" 2>/dev/null
    fi
    if ! mkdir "$lockdir" 2>/dev/null; then
        echo "error: ${job_cmd##*/} $name is already running (lock: $lockdir)" >&2
        exit 1
    fi
fi

# Build unified ordered list — each entry is runnable, skipped, disabled, or a separator
all_labels=()
all_cmds=()      # empty string for skipped/separator/disabled entries
all_skip=()      # 1 = skipped/separator/disabled, 0 = runnable
all_sep=()       # 1 = separator (blank line), 0 = normal entry
all_disable=()   # 1 = disabled by user, 0 = not disabled
run_n=0
skip_n=0
disable_n=0
for _arg in "$@"; do
    if [[ "$_arg" == "---" ]]; then
        all_labels+=("---")
        all_cmds+=("")
        all_skip+=(1)
        all_sep+=(1)
        all_disable+=(0)
    elif [[ "$_arg" == disable:* ]]; then
        all_labels+=("${_arg#disable:}")
        all_cmds+=("")
        all_skip+=(1)
        all_sep+=(0)
        all_disable+=(1)
        disable_n=$((disable_n + 1))
    elif [[ "$_arg" == skip:* ]]; then
        all_labels+=("${_arg#skip:}")
        all_cmds+=("")
        all_skip+=(1)
        all_sep+=(0)
        all_disable+=(0)
        skip_n=$((skip_n + 1))
    else
        # "label::command" names the job explicitly; otherwise the label is the
        # command's last word ("test go" -> go). Labels name the log files.
        if [[ "$_arg" == *::* ]]; then
            _job_label="${_arg%%::*}"
            _arg="${_arg#*::}"
        else
            _job_label="${_arg##* }"
        fi
        all_labels+=("${_job_label//\//_}")
        all_cmds+=("$_arg")
        all_skip+=(0)
        all_sep+=(0)
        all_disable+=(0)
        run_n=$((run_n + 1))
    fi
done
total=${#all_labels[@]}

# Compute label column width (min 6)
label_w=6
for i in "${!all_labels[@]}"; do
    [ "${all_sep[$i]}" -eq 1 ] && continue
    (( ${#all_labels[$i]} > label_w )) && label_w=${#all_labels[$i]}
done

# Detect terminal width via multiple methods (tput alone often returns 80)
_term_width() {
    local w=0
    if [ -t 0 ]; then
        w=$(stty size 2>/dev/null | awk '{print $2}')
        [ "${w:-0}" -gt 0 ] 2>/dev/null && echo "$w" && return
    fi
    if command -v tput &>/dev/null; then
        w=$(tput cols 2>/dev/null)
        [ "${w:-0}" -gt 0 ] 2>/dev/null && echo "$w" && return
    fi
    [ "${COLUMNS:-0}" -gt 0 ] 2>/dev/null && echo "$COLUMNS" && return
    echo 80
}
_clear_screen() { printf "\033[2J\033[H"; }
_hide_cursor() { printf "\033[?25l"; }
_show_cursor() { printf "\033[?25h"; }
_cursor_up() { printf "\033[%dA" "$1"; }
_cursor_home() { printf "\033[H"; }
# Absolute-position line feed: move to the next row, column 1.
# Immune to line-wrap drift from emoji/Unicode width mismatches.
_row=0
_seek_line() { ((_row++)); printf "\033[${_row};1H"; }

_term_height() {
    local h=0
    if [ -t 0 ]; then
        h=$(stty size 2>/dev/null | awk '{print $1}')
        [ "${h:-0}" -gt 0 ] 2>/dev/null && echo "$h" && return
    fi
    if command -v tput &>/dev/null; then
        h=$(tput lines 2>/dev/null)
        [ "${h:-0}" -gt 0 ] 2>/dev/null && echo "$h" && return
    fi
    [ "${LINES:-0}" -gt 0 ] 2>/dev/null && echo "$LINES" && return
    echo 24
}
_cpu_count() {
    local n=0
    n=$(sysctl -n hw.logicalcpu 2>/dev/null) && [ "${n:-0}" -gt 0 ] && echo "$n" && return
    n=$(nproc 2>/dev/null) && [ "${n:-0}" -gt 0 ] && echo "$n" && return
    [ -f /proc/cpuinfo ] && n=$(grep -c '^processor' /proc/cpuinfo 2>/dev/null)
    [ "${n:-0}" -gt 0 ] && echo "$n" && return
    echo 4
}
_pcpu_count() {
    local n=0
    # macOS Apple Silicon: performance core count
    n=$(sysctl -n hw.perflevel0.logicalcpu 2>/dev/null) && [ "${n:-0}" -gt 0 ] && echo "${n}" && return
    # Linux hybrid (big.LITTLE / Intel P+E): count CPUs at highest frequency tier
    if [ -d /sys/devices/system/cpu/cpu0/cpufreq ]; then
        local _maxf=0 _f _freq
        for _f in /sys/devices/system/cpu/cpu*/cpufreq/cpuinfo_max_freq; do
            [ -f "${_f}" ] && read -r _freq < "${_f}" && (( _freq > _maxf )) && _maxf=${_freq}
        done
        if [ "${_maxf}" -gt 0 ]; then
            n=0
            for _f in /sys/devices/system/cpu/cpu*/cpufreq/cpuinfo_max_freq; do
                [ -f "${_f}" ] && read -r _freq < "${_f}" && [ "${_freq}" -eq "${_maxf}" ] && n=$((n+1))
            done
            [ "${n}" -gt 0 ] && echo "${n}" && return
        fi
    fi
    # Fallback: total CPU count (no hybrid topology detected)
    _cpu_count
}

# Resolve percentage/pcores -j values to integers
if [ "${max_jobs}" = "pcores" ]; then
    max_jobs=$(( $(_pcpu_count) - 1 ))
    (( max_jobs < 1 )) && max_jobs=1
elif [[ "${max_jobs}" == *% ]]; then
    _pct="${max_jobs%\%}"
    _cpus=$(_cpu_count)
    if [ "$_pct" -eq 100 ]; then
        max_jobs=$_cpus
    else
        max_jobs=$(( (_cpus * _pct + 50) / 100 ))
    fi
    (( max_jobs < 1 )) && max_jobs=1
fi

# Set up nice command prefix
_nice_cmd=""
if [ -n "${_nice_val}" ]; then
    if sudo -n true 2>/dev/null; then
        _nice_cmd="sudo -n nice -n ${_nice_val}"
    else
        echo "warning: -n ${_nice_val} requires sudo; run 'sudo -v' first or use 'sudo just ...'" >&2
    fi
fi

# Slot icon: 🔥 negative nice (high priority), 🧊 positive nice (low priority), ⚙ default
# Emojis carry their own color — only ⚙ needs ANSI coloring (yellow active, grey idle)
# _slot_icon_extra tracks the display-width difference (emoji=2 cells, ⚙=1 cell)
_slot_icon="⚙"
_slot_icon_extra=0
_s_slot_active="${_s_run}"
_s_slot_idle="${_s_idle}"
if [ -n "${_nice_cmd}" ]; then
    if [ "${_nice_val}" -lt 0 ] 2>/dev/null; then
        _slot_icon="🔥"
        _slot_icon_extra=1
        _s_slot_active=""
        _s_slot_idle=""
    elif [ "${_nice_val}" -gt 0 ] 2>/dev/null; then
        _slot_icon="🧊"
        _slot_icon_extra=1
        _s_slot_active=""
        _s_slot_idle=""
    fi
fi

# More slots than jobs → no scheduling benefit; use unlimited mode
if [ "$max_jobs" -gt 0 ] && [ "$max_jobs" -ge "$run_n" ]; then
    max_jobs=0
fi

term_w=$(_term_width)
# Layout: "  X label _dur_str _bar" → 2+1+1 + label_w + 1 + 6 + 1 = label_w + 12
prefix_w=$((label_w + 12))
max_bar=$((term_w - prefix_w))
(( max_bar < 10 )) && max_bar=10

# If all entries are skipped, print skip lines and exit early
if [ "$run_n" -eq 0 ]; then
    for i in "${!all_labels[@]}"; do
        if [ "${all_sep[$i]}" -eq 1 ]; then echo; continue; fi
        label="${all_labels[$i]}"
        if [ "${all_disable[$i]}" -eq 1 ]; then
            if [ -t 1 ]; then
                printf "  ${_s_disable}⊘ %-${label_w}s      ⊘ disabled$_c_reset\n" "$label"
            else
                printf "  ⊘ %-${label_w}s      ⊘ disabled\n" "$label"
            fi
        else
            if [ -t 1 ]; then
                printf "  $_s_skip- %-${label_w}s    - not installed$_c_reset\n" "$label"
            else
                printf "  - %-${label_w}s    - not installed\n" "$label"
            fi
        fi
    done
    echo ""
    _summary="0/0 passed"
    [ "$skip_n" -gt 0 ] && _summary="$_summary, $skip_n not installed"
    [ "$disable_n" -gt 0 ] && _summary="$_summary, $disable_n disabled"
    echo "$_summary"
    echo "Logs: $logdir/"
    rmdir "$lockdir" 2>/dev/null
    exit 0
fi

# Pre-built block string (wide enough for any terminal)
_blocks=""
for (( _i=0; _i<512; _i++ )); do _blocks+="█"; done

_bar() {
    local len=$1
    (( len > max_bar )) && len=$max_bar
    (( len < 1 )) && len=1
    printf "%s" "${_blocks:0:$len}"
}

_now_ds() { perl -MTime::HiRes=time -e 'printf "%d\n", time*10'; }
_fmt_ds() { echo "$((${1} / 10)).$((${1} % 10))s"; }

# Print a duration-scaled time axis above bar charts.
# $1 = max duration in deciseconds, $2 = 1 if interactive (adds $_c_erase)
_print_duration_axis() {
    local _max_ds=$1 _is_tty=${2:-0}
    (( _max_ds < 1 )) && _max_ds=1
    printf "%*s" "$prefix_w" ""
    local _max_sec=$(( (_max_ds + 9) / 10 ))
    local _interval _prev_end=0 _t _tick_col _pad
    if [ "$_max_sec" -le 5 ]; then _interval=1
    elif [ "$_max_sec" -le 20 ]; then _interval=5
    elif [ "$_max_sec" -le 60 ]; then _interval=10
    else _interval=30; fi
    for (( _t=0; _t<=_max_sec; _t+=_interval )); do
        _tick_col=$(( _t * 10 * max_bar / _max_ds ))
        if [ "$_tick_col" -ge "$_prev_end" ] && [ "$((_tick_col + ${#_t}))" -le "$max_bar" ]; then
            _pad=$((_tick_col - _prev_end))
            (( _pad > 0 )) && printf "%*s" "$_pad" ""
            printf "%d" "$_t"
            _prev_end=$((_tick_col + ${#_t}))
        fi
    done
    if [ "$_is_tty" -eq 1 ]; then
        printf "$_c_erase"; _seek_line
    else
        echo
    fi
}

# Print a timeline axis for slot-based visualization.
# Reads globals: _viz_mode, _total_ds, _eff_tick_ds, _display_offset_ds, max_bar, prefix_w
_print_time_axis() {
    printf "%*s" "$prefix_w" ""
    local _prev_end=0 _t _label_interval _tick_col _pad
    if [ "$_viz_mode" = "full" ]; then
        local _total_sec=$((_total_ds / 10 + 1))
        if [ "$_total_sec" -lt 10 ]; then _label_interval=1
        elif [ "$_total_sec" -lt 60 ]; then _label_interval=5
        else _label_interval=30; fi
        for (( _t=0; _t<=_total_sec; _t+=_label_interval )); do
            _tick_col=$((_t * 10 * max_bar / _total_ds))
            if [ "$_tick_col" -ge "$_prev_end" ] && [ "$((_tick_col + ${#_t}))" -le "$max_bar" ]; then
                _pad=$((_tick_col - _prev_end))
                (( _pad > 0 )) && printf "%*s" "$_pad" ""
                printf "%d" "$_t"
                _prev_end=$((_tick_col + ${#_t}))
            fi
        done
    else
        if [ "$_eff_tick_ds" -le 2 ]; then _label_interval=1
        elif [ "$_eff_tick_ds" -le 10 ]; then _label_interval=5
        elif [ "$_eff_tick_ds" -le 50 ]; then _label_interval=30
        else _label_interval=60; fi
        local _first_sec=$(( (_display_offset_ds + 9) / 10 ))
        _first_sec=$(( (_first_sec + _label_interval - 1) / _label_interval * _label_interval ))
        local _last_ds=$((_display_offset_ds + max_bar * _eff_tick_ds))
        local _last_sec=$((_last_ds / 10))
        for (( _t=_first_sec; _t<=_last_sec; _t+=_label_interval )); do
            _tick_col=$(( (_t * 10 - _display_offset_ds) / _eff_tick_ds ))
            if [ "$_tick_col" -ge "$_prev_end" ] && [ "$((_tick_col + ${#_t}))" -le "$max_bar" ]; then
                _pad=$((_tick_col - _prev_end))
                (( _pad > 0 )) && printf "%*s" "$_pad" ""
                printf "%d" "$_t"
                _prev_end=$((_tick_col + ${#_t}))
            fi
        done
    fi
    if [ "$_prev_end" -lt "$max_bar" ]; then
        printf "%*s" "$((max_bar - _prev_end))" ""
    fi
    printf "$_c_erase"; _seek_line
}

# --- Swim lane color palette (256-color, 20 distinct) ---
_palette=(196 46 33 208 129 214 39 161 82 166 63 203 48 99 215 35 197 118 57 172)
_npal=${#_palette[@]}

# Render swim lane chart (supports full/scroll/zoom modes).
# Every line ends with $_c_erase for live-refresh compatibility.
# Expects: max_jobs, max_bar, _viz_mode, _total_ds, _eff_tick_ds,
#          _display_offset_ds, _max_ds, _lane_ci[], _slot_labels[],
#          _palette[], prefix_w, label_w, term_w
_render_lanes() {
    printf "$_c_erase"; _seek_line

    # Header rule line
    local _ri
    for (( _ri=0; _ri<term_w; _ri++ )); do printf "─"; done
    printf "$_c_erase"; _seek_line

    # Time axis
    _print_time_axis
    printf "$_c_erase"; _seek_line

    # Count unstarted jobs to decide whether idle slots show a gear
    local _queued=0
    for _qi in "${_run_queue[@]}"; do
        [ "${_job_started[$_qi]}" -eq 0 ] && _queued=$((_queued + 1))
    done

    # Lane rows (slot icon + color block aligned with bar chart labels)
    local _s _lt _ds
    for (( _s=0; _s<max_jobs; _s++ )); do
        if [ -n "${_slot_labels[$_s]}" ] && [ "${_slot_ci[$_s]}" -ge 0 ]; then
            printf "  ${_s_slot_active}${_slot_icon}${_c_reset} ${_c_bg256}%dm  $_c_reset %-${label_w}s%*s" "${_palette[${_slot_ci[$_s]}]}" "${_slot_labels[$_s]}" "$((8 - _slot_icon_extra))" ""
        elif [ "$_queued" -gt 0 ]; then
            printf "  ${_s_slot_idle}${_slot_icon}${_c_reset}%*s" "$((prefix_w - 3 - _slot_icon_extra))" ""
        else
            printf "%*s" "$prefix_w" ""
        fi
        for (( _lt=0; _lt<max_bar; _lt++ )); do
            if [ "$_viz_mode" = "full" ]; then
                _ds=$((_lt * _total_ds / max_bar))
            else
                _ds=$((_display_offset_ds + _lt * _eff_tick_ds))
            fi
            if (( _ds >= _max_ds )); then
                printf " "
            else
                local _ci="${_lane_ci[$((_s * _max_ds + _ds))]:--1}"
                if [ "$_ci" -ge 0 ]; then
                    printf "${_c_bg256}%dm $_c_reset" "${_palette[$((_ci % _npal))]}"
                else
                    printf "${_c_lane_idle} $_c_reset"
                fi
            fi
        done
        printf "$_c_erase"; _seek_line
    done
}

# Scroll state (set by _calc_viewport)
_scroll_offset=0
_scroll_active=0
_vis_count=0
_scroll_max=0

# Compute viewport dimensions. Sets _scroll_active, _vis_count, _scroll_max,
# _display_height. Reads: total, _swim_height, term_h.
_calc_viewport() {
    local _full_h
    if [ "${_swim_height:-0}" -gt 0 ]; then
        _full_h=$((2 + total + _swim_height))  # axis + blank + entries + lanes
    else
        _full_h=$((1 + total))                  # axis + entries
    fi
    if [ "$_full_h" -lt "$term_h" ]; then
        _scroll_active=0
        _vis_count=$total
        _display_height=$_full_h
    else
        _scroll_active=1
        local _fixed
        if [ "${_swim_height:-0}" -gt 0 ]; then
            _fixed=$((2 + 2 + _swim_height))    # axis+blank + 2 indicator lines + lanes
        else
            _fixed=$((1 + 2))                    # axis + 2 indicator lines
        fi
        _vis_count=$((term_h - _fixed - 1))
        (( _vis_count < 1 )) && _vis_count=1
        _scroll_max=$((total - _vis_count))
        (( _scroll_max < 0 )) && _scroll_max=0
        _display_height=$((_fixed + _vis_count))
        (( _scroll_offset > _scroll_max )) && _scroll_offset=$_scroll_max
    fi
}

_old_stty=""
_key_fd=""

# Drain all pending keystrokes and update _scroll_offset.
# Only effective when _scroll_active=1 and fd 9 is open on the terminal.
_drain_keys() {
    [ "$_scroll_active" -eq 0 ] && return
    [ -z "${_key_fd:-}" ] && return
    local _key _rest
    while IFS= read -rsn1 -t 0 _key <&9 2>/dev/null; do
        case "$_key" in
            k) (( _scroll_offset > 0 )) && (( _scroll_offset-- )) ;;
            j) (( _scroll_offset < _scroll_max )) && (( _scroll_offset++ )) ;;
            $'\033')
                IFS= read -rsn2 -t 0.05 _rest <&9 2>/dev/null || true
                case "$_rest" in
                    '[A') (( _scroll_offset > 0 )) && (( _scroll_offset-- )) ;;
                    '[B') (( _scroll_offset < _scroll_max )) && (( _scroll_offset++ )) ;;
                esac ;;
        esac
    done
}

# Open the terminal for raw key input. Prefers /dev/tty (works under just(1)
# where stdin is not a terminal), falls back to stdin when it is a tty.
_enable_raw_input() {
    # Test /dev/tty in a subshell to avoid permanently redirecting stderr
    if (exec 9</dev/tty) 2>/dev/null; then
        exec 9</dev/tty
        _key_fd=1
        _old_stty=$(stty -g </dev/tty 2>/dev/null)
        stty -icanon -echo </dev/tty 2>/dev/null
    elif [ -t 0 ]; then
        exec 9<&0
        _key_fd=1
        _old_stty=$(stty -g 2>/dev/null)
        stty -icanon -echo 2>/dev/null
    fi
}

# Set up the interactive display: spinner, result arrays, viewport, cursor.
# Usage: _init_interactive_display <swim_height>
_init_interactive_display() {
    _spin='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'
    _frame=0
    done_flags=()
    done_durs=()
    done_rcs=()
    _swim_height=$1
    term_h=$(_term_height)
    _calc_viewport
    _clear_screen
    for (( _i=0; _i<_display_height; _i++ )); do echo; done
    for i in "${!all_labels[@]}"; do
        done_flags[$i]=""
        done_durs[$i]=0
        done_rcs[$i]=0
    done
    _hide_cursor
    _enable_raw_input
}

# Re-read terminal dimensions, recalculate viewport, and redraw if size changed.
_update_terminal_size() {
    _old_dh=$_display_height
    _old_tw=$term_w
    term_w=$(_term_width)
    term_h=$(_term_height)
    max_bar=$((term_w - prefix_w))
    (( max_bar < 10 )) && max_bar=10
    _calc_viewport
    (( _scroll_offset > _scroll_max )) && _scroll_offset=$_scroll_max
    if (( _display_height != _old_dh || term_w < _old_tw )); then
        _clear_screen
        for (( _i=0; _i<_display_height; _i++ )); do echo; done
    fi
}

_print_scroll_top() {
    if [ "$_scroll_active" -eq 1 ]; then
        if [ "$_scroll_offset" -gt 0 ]; then
            printf "  ${_s_scroll}▲ %d more$_c_reset$_c_erase" "$_scroll_offset"; _seek_line
        else
            printf "$_c_erase"; _seek_line
        fi
    fi
}

_print_scroll_bottom() {
    if [ "$_scroll_active" -eq 1 ]; then
        _below=$((total - _scroll_offset - _vis_count))
        if [ "$_below" -gt 0 ]; then
            printf "  ${_s_scroll}▼ %d more$_c_reset$_c_erase" "$_below"; _seek_line
        else
            printf "$_c_erase"; _seek_line
        fi
    fi
}

# Pick the next job for a given slot. Slot 0 is the "fast slot" (picks shortest
# job, errors first); other slots pick the first unstarted job (longest first).
# Sets _found_qi (-1 if none available).
_select_next_job() {
    local _s=$1
    _found_qi=-1
    if [ "$_s" -eq 0 ]; then
        local _fallback=-1 _fqi _fq
        for (( _fqi=${#_run_queue[@]}-1; _fqi>=0; _fqi-- )); do
            _fq=${_run_queue[$_fqi]}
            [ "${_job_started[$_fq]}" -eq 1 ] && continue
            [ "$_fallback" -lt 0 ] && _fallback=$_fq
            if [ "${_error_last[$_fq]}" -eq 1 ]; then
                _found_qi=$_fq; break
            fi
        done
        [ "$_found_qi" -lt 0 ] && _found_qi=$_fallback
    else
        local _fq
        for _fq in "${_run_queue[@]}"; do
            [ "${_job_started[$_fq]}" -eq 1 ] && continue
            _found_qi=$_fq; break
        done
    fi
}

# Print final results (non-interactive). Reads result files and prints scaled bars.
_print_final_results() {
    term_w=$(_term_width)
    max_bar=$((term_w - prefix_w))
    (( max_bar < 10 )) && max_bar=10

    _max_dur_ds=1
    for i in "${!all_labels[@]}"; do
        [ "${all_skip[$i]}" -eq 1 ] && continue
        read -r _dur_ds _rc < "$logdir/${all_labels[$i]}.result"
        (( _dur_ds > _max_dur_ds )) && _max_dur_ds=$_dur_ds
    done

    _print_duration_axis "$_max_dur_ds"
    for i in "${!all_labels[@]}"; do
        label="${all_labels[$i]}"
        if [ "${all_sep[$i]}" -eq 1 ]; then
            echo; continue
        fi
        if [ "${all_disable[$i]}" -eq 1 ]; then
            printf "  ⊘ %-${label_w}s      ⊘ disabled\n" "$label"
        elif [ "${all_skip[$i]}" -eq 1 ]; then
            printf "  - %-${label_w}s      - not installed\n" "$label"
        else
            read -r _dur_ds _rc < "$logdir/$label.result"
            _dur_str=$(_fmt_ds "$_dur_ds")
            _bar_len=$(( _dur_ds * max_bar / _max_dur_ds ))
            (( _bar_len < 1 )) && _bar_len=1
            if [ "$_rc" -eq 0 ]; then
                printf "  ✓ %-${label_w}s %6s %s\n" "$label" "$_dur_str" "$(_bar "$_bar_len")"
            else
                printf "  ✗ %-${label_w}s %6s %s\n" "$label" "$_dur_str" "$(_bar "$_bar_len")"
            fi
        fi
    done
}

_cleanup() {
    if [ -n "${_old_stty:-}" ]; then
        stty "$_old_stty" </dev/tty 2>/dev/null || stty "$_old_stty" 2>/dev/null
    fi
    [ -n "${_key_fd:-}" ] && exec 9<&- 2>/dev/null
    [ -t 1 ] && _show_cursor 2>/dev/null
    # shellcheck disable=SC2046  # one PID per word
    kill $(jobs -p) 2>/dev/null
    wait 2>/dev/null
    for i in "${!all_labels[@]}"; do
        [ "${all_skip[$i]}" -eq 0 ] && rm -f "$logdir/${all_labels[$i]}.result"
    done
    rmdir "$lockdir" 2>/dev/null
    # If running under sudo, fix log ownership so non-sudo runs aren't blocked
    if [ -n "${SUDO_USER:-}" ]; then
        chown -R "${SUDO_USER}" "$logdir" 2>/dev/null
    fi
}
trap _cleanup EXIT
trap 'exit 130' INT

# ===================================================================
# Job scheduling + progress display
# ===================================================================
global_start_ds=$(_now_ds)
pids=()

if [ "$max_jobs" -gt 0 ]; then
    # --- Slot-based scheduling ---

    # Build queue of runnable job indices (alphabetical order initially)
    _run_queue=()
    for i in "${!all_labels[@]}"; do
        [ "${all_skip[$i]}" -eq 0 ] && _run_queue+=("$i")
    done

    # Color index per label: position in the queue (alphabetical, stable across sorts)
    _label_ci=()
    for _q in "${!_run_queue[@]}"; do
        _label_ci[${_run_queue[$_q]}]=$((_q % _npal))
    done

    # --- History-based duration prediction + error tracking ---
    _predicted_dur=()
    _error_last=()
    for i in "${!all_labels[@]}"; do
        _predicted_dur[$i]=0
        _error_last[$i]=0
    done
    _histfile="$logdir/history.log"
    if [ -f "$_histfile" ]; then
        _hist_lines=()
        while IFS= read -r _hl; do
            _hist_lines+=("$_hl")
        done < <(tail -5 "$_histfile")

        # Duration prediction (median of last 5 runs)
        for i in "${!all_labels[@]}"; do
            [ "${all_skip[$i]}" -eq 1 ] && continue
            label="${all_labels[$i]}"
            _durs=()
            for _hl in "${_hist_lines[@]}"; do
                for _pair in $_hl; do
                    if [[ "$_pair" == "${label}="* ]]; then
                        _val="${_pair#*=}"
                        _durs+=("${_val%%:*}")  # strip :_rc if present
                    fi
                done
            done
            if [ ${#_durs[@]} -gt 0 ]; then
                # shellcheck disable=SC2207  # bash 3.2 has no mapfile; values are integers
                IFS=$'\n' _sorted_d=($(sort -n <<<"${_durs[*]}")); unset IFS
                _predicted_dur[$i]=${_sorted_d[$(( ${#_sorted_d[@]} / 2 ))]}
            fi
        done

        # Error tracking (jobs that failed in the most recent run)
        _last_run="${_hist_lines[${#_hist_lines[@]}-1]}"
        for _pair in $_last_run; do
            _elabel="${_pair%%=*}"
            _edata="${_pair#*=}"
            _erc="${_edata#*:}"
            if [ "$_erc" != "$_edata" ] && [ "$_erc" -ne 0 ] 2>/dev/null; then
                for i in "${!all_labels[@]}"; do
                    [ "${all_labels[$i]}" = "$_elabel" ] && _error_last[$i]=1
                done
            fi
        done
    fi

    # Sort _run_queue: errors first, then by predicted duration descending
    # Composite key: error * 10000000 + predicted_dur
    for (( _i=1; _i<${#_run_queue[@]}; _i++ )); do
        _key=${_run_queue[$_i]}
        _kval=$(( ${_error_last[$_key]} * 10000000 + ${_predicted_dur[$_key]} ))
        _j=$((_i - 1))
        while [ "$_j" -ge 0 ] && [ "$(( ${_error_last[${_run_queue[$_j]}]} * 10000000 + ${_predicted_dur[${_run_queue[$_j]}]} ))" -lt "$_kval" ]; do
            _run_queue[$((_j + 1))]=${_run_queue[$_j]}
            _j=$((_j - 1))
        done
        _run_queue[$((_j + 1))]=$_key
    done

    # Slot tracking
    _slot_pids=()
    _slot_labels=()
    _slot_ci=()       # color index per slot (-1 = empty)
    _slot_job=()      # job index per slot (-1 = empty)
    for (( _s=0; _s<max_jobs; _s++ )); do
        _slot_pids[$_s]=0
        _slot_labels[$_s]=""
        _slot_ci[$_s]=-1
        _slot_job[$_s]=-1
    done

    # Job tracking
    _job_started=()
    _job_start_ds=()
    for i in "${!all_labels[@]}"; do
        _job_started[$i]=0
        _job_start_ds[$i]=0
    done

    # Timeline data: flat array [slot * _max_ds + decisecond] → color index
    _max_ds=6000  # 10 minutes at decisecond resolution
    _lane_ci=()

    # Adjust prefix width for color block in bar chart
    # "  X ██ label _dur_str _bar" → 2+1+1+2+1 + label_w + 1 + 6 + 1 = label_w + 15
    prefix_w=$((label_w + 15))
    max_bar=$((term_w - prefix_w))
    (( max_bar < 10 )) && max_bar=10

    _last_rec_ds=-1

    if [ -t 1 ]; then
        # --- Interactive slot-based loop with live swim lanes ---
        # Swim lane height: blank + header + axis + lane rows
        _init_interactive_display "$((4 + max_jobs))"

        while true; do
            # Check for finished slots
            for (( _s=0; _s<max_jobs; _s++ )); do
                if [ "${_slot_pids[$_s]}" -ne 0 ] && ! kill -0 "${_slot_pids[$_s]}" 2>/dev/null; then
                    wait "${_slot_pids[$_s]}" 2>/dev/null || true
                    _slot_pids[$_s]=0
                    _slot_labels[$_s]=""
                    _slot_ci[$_s]=-1
                    _slot_job[$_s]=-1
                fi
            done

            # Launch new jobs into empty slots
            # Slot 0 is the "fast slot": picks shortest job (errors first)
            # Other slots pick longest job (errors first)
            for (( _s=0; _s<max_jobs; _s++ )); do
                [ "${_slot_pids[$_s]}" -ne 0 ] && continue

                _select_next_job "$_s"
                [ "$_found_qi" -lt 0 ] && continue

                _qi=$_found_qi
                _job_started[$_qi]=1
                label="${all_labels[$_qi]}"
                (
                    _rc=0; _st=$(_now_ds)
                    ${_nice_cmd} ${job_cmd} ${all_cmds[$_qi]} > "$logdir/$label.log" 2>&1 || _rc=$?
                    _et=$(_now_ds)
                    echo "$((_et - _st)) $_rc" > "$logdir/$label.result"
                ) &
                _slot_pids[$_s]=$!
                _slot_labels[$_s]="$label"
                _slot_ci[$_s]=${_label_ci[$_qi]}
                _slot_job[$_s]=$_qi
                _job_start_ds[$_qi]=$(_now_ds)
                pids+=($!)
            done

            # Record timeline (decisecond resolution)
            _frame_ds=$(_now_ds)
            _cur_ds=$((_frame_ds - global_start_ds))
            if [ "$_cur_ds" -gt "$_last_rec_ds" ] && [ "$_cur_ds" -lt "$_max_ds" ]; then
                for (( _rd=_last_rec_ds+1; _rd<=_cur_ds && _rd<_max_ds; _rd++ )); do
                    for (( _s=0; _s<max_jobs; _s++ )); do
                        if [ "${_slot_ci[$_s]}" -ge 0 ]; then
                            _lane_ci[$((_s * _max_ds + _rd))]=${_slot_ci[$_s]}
                        fi
                    done
                done
                _last_rec_ds=$_cur_ds
            fi

            # Check for results
            _done_count=0
            for i in "${!all_labels[@]}"; do
                [ "${all_skip[$i]}" -eq 1 ] && continue
                label="${all_labels[$i]}"
                if [ -z "${done_flags[$i]}" ] && [ -f "$logdir/$label.result" ]; then
                    read -r done_durs[$i] done_rcs[$i] < "$logdir/$label.result"
                    done_flags[$i]=1
                fi
                [ -n "${done_flags[$i]}" ] && _done_count=$((_done_count + 1))
            done

            _drain_keys
            _update_terminal_size

            # Display parameters (per-frame, based on visualization mode)
            _display_offset_ds=0
            case "$_viz_mode" in
                full)
                    _total_ds=$((_cur_ds + 1))
                    (( _total_ds < 1 )) && _total_ds=1
                    ;;
                scroll)
                    _eff_tick_ds=$_tick_ds
                    if [ $((_cur_ds / _eff_tick_ds)) -ge "$max_bar" ]; then
                        _display_offset_ds=$(( (_cur_ds / _eff_tick_ds - max_bar + 1) * _eff_tick_ds ))
                    fi
                    ;;
                zoom)
                    _eff_tick_ds=$_tick_ds
                    while [ $((_cur_ds / _eff_tick_ds)) -ge "$max_bar" ]; do
                        _eff_tick_ds=$((_eff_tick_ds * 2))
                    done
                    ;;
            esac

            # Render: bar chart + swim lanes (full redraw)
            _row=1; _cursor_home
            _print_time_axis
            printf "$_c_erase"; _seek_line

            _print_scroll_top

            # Entry rendering (viewport-aware)
            _ve=0  # visible entries rendered
            for i in "${!all_labels[@]}"; do
                if [ "$_scroll_active" -eq 1 ]; then
                    # Skip entries before scroll offset
                    if [ "$_ve" -lt "$_scroll_offset" ]; then
                        _ve=$((_ve + 1)); continue
                    fi
                    # Stop after _vis_count visible entries
                    if [ "$((_ve - _scroll_offset))" -ge "$_vis_count" ]; then break; fi
                fi
                _ve=$((_ve + 1))

                label="${all_labels[$i]}"
                if [ "${all_sep[$i]}" -eq 1 ]; then
                    printf "$_c_erase"; _seek_line; continue
                fi
                if [ "${all_disable[$i]}" -eq 1 ]; then
                    printf "  ${_s_disable}⊘    %-${label_w}s      ⊘ disabled$_c_reset$_c_erase" "$label"; _seek_line
                    continue
                fi
                if [ "${all_skip[$i]}" -eq 1 ]; then
                    printf "  $_s_skip-    %-${label_w}s      - not installed$_c_reset$_c_erase" "$label"; _seek_line
                    continue
                fi
                _ci=${_label_ci[$i]}
                if [ -n "${done_flags[$i]}" ]; then
                    _dur_ds="${done_durs[$i]}"
                    _dur_str=$(_fmt_ds "$_dur_ds")
                    if [ "${done_rcs[$i]}" -eq 0 ]; then
                        printf "  ${_s_pass}✓$_c_reset ${_c_bg256}%dm  $_c_reset $_s_pass%-${label_w}s %6s$_c_reset " "${_palette[$_ci]}" "$label" "$_dur_str"
                    else
                        printf "  ${_s_fail}✗$_c_reset ${_c_bg256}%dm  $_c_reset $_s_fail%-${label_w}s %6s$_c_reset " "${_palette[$_ci]}" "$label" "$_dur_str"
                    fi
                    _ws_ds=$((_job_start_ds[$i] - global_start_ds))
                    _we_ds=$((_ws_ds + _dur_ds))
                    if [ "$_viz_mode" = "full" ]; then
                        _col_s=$((_ws_ds * max_bar / _total_ds))
                        _col_e=$((_we_ds * max_bar / _total_ds))
                    else
                        if [ "$_we_ds" -le "$_display_offset_ds" ]; then
                            printf "%*s" "$max_bar" ""
                            printf "$_c_erase"; _seek_line
                            continue
                        fi
                        _col_s=$(((_ws_ds - _display_offset_ds) / _eff_tick_ds))
                        _col_e=$(((_we_ds - _display_offset_ds + _eff_tick_ds - 1) / _eff_tick_ds))
                    fi
                    (( _col_s < 0 )) && _col_s=0
                    (( _col_e > max_bar )) && _col_e=$max_bar
                    (( _col_e <= _col_s )) && _col_e=$((_col_s + 1))
                    (( _col_e > max_bar )) && _col_e=$max_bar
                    (( _col_s > 0 )) && printf "%*s" "$_col_s" ""
                    _blen=$((_col_e - _col_s))
                    printf "${_c_fg256}%dm%s$_c_reset" "${_palette[$_ci]}" "${_blocks:0:$_blen}"
                    _rem=$((max_bar - _col_e))
                    (( _rem > 0 )) && printf "%*s" "$_rem" ""
                    printf "$_c_erase"; _seek_line
                elif [ "${_job_start_ds[$i]}" -gt 0 ]; then
                    # Running
                    _c="${_spin:_frame%${#_spin}:1}"
                    _elapsed_ds=$((_frame_ds - _job_start_ds[$i]))
                    _dur_str=$(_fmt_ds "$_elapsed_ds")
                    printf "  $_s_run%s$_c_reset ${_c_bg256}%dm  $_c_reset $_s_run%-${label_w}s %6s$_c_reset " "$_c" "${_palette[$_ci]}" "$label" "$_dur_str"
                    _ws_ds=$((_job_start_ds[$i] - global_start_ds))
                    if [ "$_viz_mode" = "full" ]; then
                        _col_s=$((_ws_ds * max_bar / _total_ds))
                        _col_e=$((_cur_ds * max_bar / _total_ds + 1))
                    else
                        _col_s=$(((_ws_ds - _display_offset_ds) / _eff_tick_ds))
                        _col_e=$(((_cur_ds - _display_offset_ds) / _eff_tick_ds + 1))
                    fi
                    (( _col_s < 0 )) && _col_s=0
                    (( _col_e > max_bar )) && _col_e=$max_bar
                    (( _col_e <= _col_s )) && _col_e=$((_col_s + 1))
                    (( _col_e > max_bar )) && _col_e=$max_bar
                    (( _col_s > 0 )) && printf "%*s" "$_col_s" ""
                    _blen=$((_col_e - _col_s))
                    printf "${_c_fg256}%dm%s$_c_reset" "${_palette[$_ci]}" "${_blocks:0:$_blen}"
                    _rem=$((max_bar - _col_e))
                    (( _rem > 0 )) && printf "%*s" "$_rem" ""
                    printf "$_c_erase"; _seek_line
                else
                    # Queued (not yet launched)
                    printf "  ${_s_queued}◌${_c_reset} ${_c_bg256}%dm  ${_c_reset} ${_s_queued}%-${label_w}s${_c_reset}       " "${_palette[$_ci]}" "$label"
                    printf "%*s" "$max_bar" ""
                    printf "$_c_erase"; _seek_line
                fi
            done

            _print_scroll_bottom

            _render_lanes

            [ "$_done_count" -eq "$run_n" ] && break
            _frame=$((_frame + 1))
            sleep 0.1
        done

        _show_cursor
    else
        # --- Non-interactive slot-based loop (CI / piped) ---
        while true; do
            # Check for finished slots
            for (( _s=0; _s<max_jobs; _s++ )); do
                if [ "${_slot_pids[$_s]}" -ne 0 ] && ! kill -0 "${_slot_pids[$_s]}" 2>/dev/null; then
                    wait "${_slot_pids[$_s]}" 2>/dev/null || true
                    _slot_pids[$_s]=0
                    _slot_labels[$_s]=""
                    _slot_job[$_s]=-1
                fi
            done

            # Launch new jobs into empty slots (same fast-slot logic as interactive)
            for (( _s=0; _s<max_jobs; _s++ )); do
                [ "${_slot_pids[$_s]}" -ne 0 ] && continue

                _select_next_job "$_s"
                [ "$_found_qi" -lt 0 ] && continue

                _qi=$_found_qi
                _job_started[$_qi]=1
                label="${all_labels[$_qi]}"
                (
                    _rc=0; _st=$(_now_ds)
                    ${_nice_cmd} ${job_cmd} ${all_cmds[$_qi]} > "$logdir/$label.log" 2>&1 || _rc=$?
                    _et=$(_now_ds)
                    echo "$((_et - _st)) $_rc" > "$logdir/$label.result"
                ) &
                _slot_pids[$_s]=$!
                _slot_labels[$_s]="$label"
                _slot_job[$_s]=$_qi
                pids+=($!)
            done

            _done_count=0
            for i in "${!all_labels[@]}"; do
                [ "${all_skip[$i]}" -eq 1 ] && continue
                [ -f "$logdir/${all_labels[$i]}.result" ] && _done_count=$((_done_count + 1))
            done
            [ "$_done_count" -eq "$run_n" ] && break
            sleep 0.2
        done

        _print_final_results
    fi
else
    # --- Unlimited parallelism (existing behavior) ---

    # Start jobs for runnable entries only
    for i in "${!all_labels[@]}"; do
        [ "${all_skip[$i]}" -eq 1 ] && continue
        label="${all_labels[$i]}"
        (
            _rc=0
            _st=$(_now_ds)
            ${_nice_cmd} ${job_cmd} ${all_cmds[$i]} > "$logdir/$label.log" 2>&1 || _rc=$?
            _et=$(_now_ds)
            echo "$((_et - _st)) $_rc" > "$logdir/$label.result"
        ) &
        pids+=($!)
    done

    # --- Progress display ---
    if [ -t 1 ]; then
        _init_interactive_display 0

        while true; do
            _drain_keys
            _update_terminal_size

            _row=1; _cursor_home

            _frame_ds=$(_now_ds)
            # Find max duration for scaling (completed jobs or current elapsed)
            _max_dur_ds=1
            _elapsed_ds=$((_frame_ds - global_start_ds))
            for i in "${!all_labels[@]}"; do
                [ "${all_skip[$i]}" -eq 1 ] && continue
                if [ -n "${done_flags[$i]}" ]; then
                    (( done_durs[$i] > _max_dur_ds )) && _max_dur_ds=${done_durs[$i]}
                else
                    (( _elapsed_ds > _max_dur_ds )) && _max_dur_ds=$_elapsed_ds
                fi
            done

            _print_duration_axis "$_max_dur_ds" 1

            _print_scroll_top

            _done_count=0
            # Check all results first (even non-visible ones)
            for i in "${!all_labels[@]}"; do
                [ "${all_skip[$i]}" -eq 1 ] && continue
                label="${all_labels[$i]}"
                if [ -z "${done_flags[$i]}" ] && [ -f "$logdir/$label.result" ]; then
                    read -r done_durs[$i] done_rcs[$i] < "$logdir/$label.result"
                    done_flags[$i]=1
                fi
                [ -n "${done_flags[$i]}" ] && _done_count=$((_done_count + 1))
            done

            # Entry rendering (viewport-aware)
            _ve=0
            for i in "${!all_labels[@]}"; do
                if [ "$_scroll_active" -eq 1 ]; then
                    if [ "$_ve" -lt "$_scroll_offset" ]; then
                        _ve=$((_ve + 1)); continue
                    fi
                    if [ "$((_ve - _scroll_offset))" -ge "$_vis_count" ]; then break; fi
                fi
                _ve=$((_ve + 1))

                label="${all_labels[$i]}"

                if [ "${all_sep[$i]}" -eq 1 ]; then
                    printf "$_c_erase"; _seek_line; continue
                fi
                if [ "${all_disable[$i]}" -eq 1 ]; then
                    printf "  ${_s_disable}⊘ %-${label_w}s      ⊘ disabled$_c_reset$_c_erase" "$label"; _seek_line
                    continue
                fi
                if [ "${all_skip[$i]}" -eq 1 ]; then
                    printf "  $_s_skip- %-${label_w}s      - not installed$_c_reset$_c_erase" "$label"; _seek_line
                    continue
                fi

                if [ -n "${done_flags[$i]}" ]; then
                    _dur_ds="${done_durs[$i]}"
                    _dur_str=$(_fmt_ds "$_dur_ds")
                    _bar_len=$(( _dur_ds * max_bar / _max_dur_ds ))
                    (( _bar_len < 1 )) && _bar_len=1
                    if [ "${done_rcs[$i]}" -eq 0 ]; then
                        printf "  ${_s_pass}✓ %-${label_w}s %6s %s$_c_reset$_c_erase" "$label" "$_dur_str" "$(_bar "$_bar_len")"; _seek_line
                    else
                        printf "  ${_s_fail}✗ %-${label_w}s %6s %s$_c_reset$_c_erase" "$label" "$_dur_str" "$(_bar "$_bar_len")"; _seek_line
                    fi
                else
                    _elapsed_ds=$((_frame_ds - global_start_ds))
                    _dur_str=$(_fmt_ds "$_elapsed_ds")
                    _bar_len=$(( _elapsed_ds * max_bar / _max_dur_ds ))
                    (( _bar_len < 1 )) && _bar_len=1
                    _c="${_spin:_frame%${#_spin}:1}"
                    printf "  $_s_run%s %-${label_w}s %6s %s$_c_reset$_c_erase" "$_c" "$label" "$_dur_str" "$(_bar "$_bar_len")"; _seek_line
                fi
            done

            _print_scroll_bottom

            [ "$_done_count" -eq "$run_n" ] && break
            _frame=$((_frame + 1))
            sleep 0.1
        done

        _show_cursor
    else
        # Non-interactive (CI, piped): wait then print
        for _pid in "${pids[@]}"; do wait "$_pid" || true; done
        _print_final_results
    fi
fi

# --- Summary ---
failed=0
wall_ds=0
for i in "${!all_labels[@]}"; do
    [ "${all_skip[$i]}" -eq 1 ] && continue
    label="${all_labels[$i]}"
    read -r _dur_ds _rc < "$logdir/$label.result"
    (( _dur_ds > wall_ds )) && wall_ds=$_dur_ds
    [ "$_rc" -ne 0 ] && failed=$((failed + 1))
done

passed=$(( run_n - failed ))
# When using slot-based scheduling, wall time is total elapsed (not max individual)
if [ "$max_jobs" -gt 0 ]; then
    wall_ds=$(($(_now_ds) - global_start_ds))
fi
wall_str=$(_fmt_ds "$wall_ds")
_extra=""
[ "$skip_n" -gt 0 ] && _extra="$_extra, $skip_n not installed"
[ "$disable_n" -gt 0 ] && _extra="$_extra, $disable_n disabled"
echo ""
echo "$passed/$run_n passed${_extra} ($wall_str wall time)"
echo "Logs: $logdir/"

# --- Write run history (for duration prediction) ---
_hist_line=""
for i in "${!all_labels[@]}"; do
    [ "${all_skip[$i]}" -eq 1 ] && continue
    label="${all_labels[$i]}"
    read -r _dur_ds _rc < "$logdir/$label.result"
    _hist_line="${_hist_line}${_hist_line:+ }${label}=${_dur_ds}:${_rc}"
done
echo "$_hist_line" >> "$logdir/history.log"
# Keep last 50 runs
if [ -f "$logdir/history.log" ]; then
    tail -50 "$logdir/history.log" > "$logdir/history.log.tmp" && \
        mv "$logdir/history.log.tmp" "$logdir/history.log"
fi

if [ "$failed" -gt 0 ]; then
    echo ""
    for i in "${!all_labels[@]}"; do
        [ "${all_skip[$i]}" -eq 1 ] && continue
        label="${all_labels[$i]}"
        read -r _dur_ds _rc < "$logdir/$label.result"
        if [ "$_rc" -ne 0 ]; then
            echo "=== $label ==="
            cat "$logdir/$label.log"
            echo ""
        fi
    done
    exit 1
fi
