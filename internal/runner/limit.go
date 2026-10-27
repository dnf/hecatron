package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Time limits (#124): a hung job (a deadlock, a test waiting on input or
// the network) is stopped instead of holding the run forever. By default a
// job's limit comes from its own history: LimitMultiple times the median of
// its last passing runs, at least LimitFloor; a job with no passing run
// recorded has none. --timeout or HECATRON_TIMEOUT sets one limit for every
// job instead, and 0 turns limits off.

const (
	// LimitMultiple: one job's time on one machine (history.log is local)
	// swings with load, -j, cold caches and throttling; 2-3x is routine,
	// so 5x seldom stops a healthy job and still stops a hang in a bounded
	// time.
	LimitMultiple = 5
	// LimitFloor: short jobs swing most (a 5s job that downloads or
	// recompiles after a clean takes minutes), and ten minutes is still
	// far below an agent's session limit or a CI job's.
	LimitFloor = 10 * time.Minute
	// limitRuns is how many of the job's last passing runs count.
	limitRuns = 10
	// timedOutStatus is a timed-out job's status in history.log, as
	// timeout(1) exits.
	timedOutStatus = 124
)

// LimitSource is where a job's limit came from.
type LimitSource string

const (
	LimitNone    LimitSource = ""
	LimitFlag    LimitSource = "--timeout"
	LimitEnv     LimitSource = "HECATRON_TIMEOUT"
	LimitHistory LimitSource = "history"
)

// Limit is a job's time limit; a zero Duration is none.
type Limit struct {
	Duration time.Duration
	Source   LimitSource
	Median   time.Duration // from history: the median of its passing runs
	Runs     int           // from history: how many passing runs
}

// Describe says how the limit was set: "--timeout", "HECATRON_TIMEOUT",
// "5x its median 1m40s over 5 passing runs", "the 10m minimum (5x its
// median 12s over 5 passing runs is less)".
func (l Limit) Describe() string {
	switch l.Source {
	case LimitFlag, LimitEnv:
		return string(l.Source)
	case LimitHistory:
		derived := fmt.Sprintf("%d× its median %s over %d passing run%s", LimitMultiple, FormatLimit(l.Median), l.Runs, plural(l.Runs))
		if l.Median*LimitMultiple < LimitFloor {
			return fmt.Sprintf("the %s minimum (%s is less)", FormatLimit(LimitFloor), derived)
		}
		return derived
	}
	return "none"
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// FormatLimit is a limit as it was likely written: 10m, 1h30m, 90s → 1m30s,
// 2.5s.
func FormatLimit(d time.Duration) string {
	if d < time.Second {
		return d.String()
	}
	if d%time.Second != 0 {
		return strconv.FormatFloat(d.Seconds(), 'f', -1, 64) + "s"
	}
	var out strings.Builder
	if h := d / time.Hour; h > 0 {
		fmt.Fprintf(&out, "%dh", h)
	}
	if m := d % time.Hour / time.Minute; m > 0 {
		fmt.Fprintf(&out, "%dm", m)
	}
	if s := d % time.Minute / time.Second; s > 0 {
		fmt.Fprintf(&out, "%ds", s)
	}
	return out.String()
}

// Override is --timeout or HECATRON_TIMEOUT: one limit for every job (0:
// none) instead of each job's own.
type Override struct {
	Set      bool
	Duration time.Duration
	Source   LimitSource
}

// ParseTimeout reads a limit: a Go duration (90s, 30m, 1h30m), a number
// of seconds as timeout(1) takes it, or 0 for none.
func ParseTimeout(value string) (time.Duration, error) {
	if n, err := strconv.ParseUint(value, 10, 32); err == nil {
		return time.Duration(n) * time.Second, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("a duration such as 90s, 30m or 1h30m, or 0 for none (got '%s')", value)
	}
	return d, nil
}

// SelectTimeout is the override: the --timeout flag when given, else
// HECATRON_TIMEOUT when set, else none (each job's own limit).
func SelectTimeout(flag string, flagGiven bool, getenv func(string) string) (Override, error) {
	value, source := flag, LimitFlag
	if !flagGiven {
		value, source = getenv("HECATRON_TIMEOUT"), LimitEnv
		if value == "" {
			return Override{}, nil
		}
	}
	d, err := ParseTimeout(value)
	if err != nil {
		return Override{}, fmt.Errorf("%s must be %v", source, err)
	}
	return Override{Set: true, Duration: d, Source: source}, nil
}

// Limit is a job's limit: the override's when set, else from its last
// passing runs. Failed runs are left out because a failure can stop early
// (a compile error), and timed-out ones because they ran to the limit.
func (h History) Limit(label string, o Override) Limit {
	if o.Set {
		if o.Duration == 0 {
			return Limit{}
		}
		return Limit{Duration: o.Duration, Source: o.Source}
	}
	var durations []int64
	for i := len(h.lines) - 1; i >= 0 && len(durations) < limitRuns; i-- {
		for _, pair := range strings.Fields(h.lines[i]) {
			value, ok := strings.CutPrefix(pair, label+"=")
			if !ok {
				continue
			}
			ds, status, _ := strings.Cut(value, ":")
			n, err := strconv.ParseInt(ds, 10, 64)
			if err == nil && status == "0" {
				durations = append(durations, n)
			}
		}
	}
	if len(durations) == 0 {
		return Limit{}
	}
	sort.Slice(durations, func(a, b int) bool { return durations[a] < durations[b] })
	median := time.Duration(durations[len(durations)/2]) * 100 * time.Millisecond
	return Limit{Duration: max(median*LimitMultiple, LimitFloor), Source: LimitHistory, Median: median, Runs: len(durations)}
}

// Execute runs a job within its limit, its output in out. A job that
// reaches its limit is stopped (Supervise stops its processes) and says
// so at the end of its log.
func Execute(ctx context.Context, job Job, out io.Writer) (Outcome, Status) {
	jobCtx := ctx
	if job.Limit.Duration > 0 {
		var cancel context.CancelFunc
		jobCtx, cancel = context.WithTimeout(ctx, job.Limit.Duration)
		defer cancel()
	}
	outcome := job.Exec(jobCtx, out)
	switch {
	case ctx.Err() == nil && errors.Is(jobCtx.Err(), context.DeadlineExceeded):
		_, _ = fmt.Fprintf(out, "hecatron: %s timed out: stopped at its %s limit (%s)\n", job.Label, FormatLimit(job.Limit.Duration), job.Limit.Describe())
		return outcome, TimedOut
	case outcome.ExitCode != 0:
		return outcome, Failed
	}
	return outcome, Passed
}
