// Package runner runs a command's jobs (a language's hook for its
// components, a project task) in parallel, as libexec/runner/run-parallel.sh
// does, and describes each finished job as a Result for the renderers
// (docs/design.md §19, "Output").
//
// Scheduling is run-parallel.sh's: with fewer slots than jobs, the queue is
// ordered by predicted duration (the median of the last five runs in
// history.log), longest first, with the jobs that failed last time ahead of
// the rest; slot 0 is the fast slot, taking the shortest job left (one that
// failed last time first), so quick jobs report early. With as many slots
// as jobs, or -j unlimited, every job starts at once.
package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Kind is what an entry in the job list is.
type Kind int

const (
	Runnable     Kind = iota
	NotInstalled      // its toolchain isn't on PATH: shown, not run
	Disabled          // the user disabled the language: shown, not run
	Separator         // a blank line between languages and project tasks
)

// Job is one entry in the job list.
type Job struct {
	Label string // names the log file and the row
	Kind  Kind
	Lang  string // the language, "" for a project task or tool
	Task  bool   // a project task (config.json "tasks")
	Limit Limit  // its time limit (limit.go)
	Exec  func(ctx context.Context, out io.Writer) Outcome
}

// Outcome is what a job's Exec reports.
type Outcome struct {
	ExitCode   int
	Components []Component
}

// Component is one component a language job ran for.
type Component struct {
	Name             string `json:"name"`
	Dir              string `json:"dir"`
	Failed           bool   `json:"failed"`
	ToolchainMissing bool   `json:"toolchainMissing,omitempty"`
}

// Status is how a job ended, or why it didn't run.
type Status string

const (
	Passed            Status = "passed"
	Failed            Status = "failed"
	TimedOut          Status = "timed out" // stopped at its time limit
	StatusNotInstall  Status = "not installed"
	StatusDisabled    Status = "disabled"
	StatusSeparator   Status = ""
	statusNotFinished Status = "running"
)

// Result is a finished job.
type Result struct {
	Label      string
	Lang       string
	Task       bool
	Status     Status
	ExitCode   int
	Duration   time.Duration
	Limit      Limit
	Log        string
	Components []Component
	Excerpt    Excerpt
	Started    time.Duration // after the run started
}

// ToolchainMissing reports whether the job failed in components whose
// language command wasn't on PATH, and only in those.
func (r *Result) ToolchainMissing() bool {
	if r.Status != Failed {
		return false
	}
	missing := false
	for _, c := range r.Components {
		if c.Failed && !c.ToolchainMissing {
			return false
		}
		missing = missing || c.ToolchainMissing
	}
	return missing
}

// Ran reports whether the job ran: it passed, failed or timed out.
func (r *Result) Ran() bool {
	return r.Status == Passed || r.Status == Failed || r.Status == TimedOut
}

// Unsuccessful reports whether the job failed or timed out.
func (r *Result) Unsuccessful() bool { return r.Status == Failed || r.Status == TimedOut }

// FailedComponents are the components a failed job failed in.
func (r *Result) FailedComponents() []Component {
	var failed []Component
	for _, c := range r.Components {
		if c.Failed {
			failed = append(failed, c)
		}
	}
	return failed
}

// Observer follows a run, for the live display. Calls are serialised.
type Observer interface {
	Started(index int, at time.Duration)
	Finished(index int, result Result)
}

// Options control a run.
type Options struct {
	Slots    int    // concurrent jobs; 0 is every job at once
	LogDir   string // <logs>/<action>
	Observer Observer
	Now      func() time.Time // for tests
}

// SlotMode reports whether a run of jobs with slots schedules them
// (run-parallel.sh: more slots than jobs is the same as unlimited).
func SlotMode(slots int, jobs []Job) bool {
	return slots > 0 && slots < Runnables(jobs)
}

// Runnables counts the jobs that run.
func Runnables(jobs []Job) int {
	n := 0
	for _, job := range jobs {
		if job.Kind == Runnable {
			n++
		}
	}
	return n
}

// Run runs the jobs, writing each one's output to <LogDir>/<label>.log, and
// returns a result for every entry (skipped ones included), in job order,
// and the wall time.
func Run(ctx context.Context, jobs []Job, opts Options) ([]Result, time.Duration) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	results := make([]Result, len(jobs))
	var queue []int
	for i, job := range jobs {
		results[i] = Result{Label: job.Label, Lang: job.Lang, Task: job.Task, Limit: job.Limit, Status: statusNotFinished}
		switch job.Kind {
		case NotInstalled:
			results[i].Status = StatusNotInstall
		case Disabled:
			results[i].Status = StatusDisabled
		case Separator:
			results[i].Status = StatusSeparator
		default:
			results[i].Log = filepath.Join(opts.LogDir, job.Label+".log")
			queue = append(queue, i)
		}
	}
	slots := opts.Slots
	if !SlotMode(slots, jobs) {
		slots = len(queue)
	}
	history := ReadHistory(filepath.Join(opts.LogDir, "history.log"))
	queue, failedLast := history.Order(jobs, queue)

	start := now()
	type done struct{ slot, index int }
	finished := make(chan done)
	var mu sync.Mutex // serialises the observer
	launch := func(slot, i int) {
		at := now().Sub(start)
		mu.Lock()
		results[i].Started = at
		if opts.Observer != nil {
			opts.Observer.Started(i, at)
		}
		mu.Unlock()
		go func() {
			began := now()
			outcome, status := runJob(ctx, jobs[i], results[i].Log)
			r := &results[i]
			r.Duration = now().Sub(began)
			r.ExitCode = outcome.ExitCode
			r.Components = outcome.Components
			r.Status = status
			finished <- done{slot, i}
		}()
	}
	started := map[int]bool{}
	next := func(slot int) int {
		if slot == 0 && slots < len(queue) {
			fallback := -1
			for q := len(queue) - 1; q >= 0; q-- {
				i := queue[q]
				if started[i] {
					continue
				}
				if fallback < 0 {
					fallback = i
				}
				if failedLast[i] {
					return i
				}
			}
			return fallback
		}
		for _, i := range queue {
			if !started[i] {
				return i
			}
		}
		return -1
	}
	fill := func(slot int) {
		if i := next(slot); i >= 0 {
			started[i] = true
			launch(slot, i)
		}
	}
	for slot := 0; slot < slots; slot++ {
		fill(slot)
	}
	for remaining := len(queue); remaining > 0; remaining-- {
		d := <-finished
		mu.Lock()
		if opts.Observer != nil {
			opts.Observer.Finished(d.index, results[d.index])
		}
		mu.Unlock()
		fill(d.slot)
	}
	wall := now().Sub(start)
	if !SlotMode(opts.Slots, jobs) {
		// run-parallel.sh's wall time without slots: the longest job.
		wall = 0
		for _, i := range queue {
			if results[i].Duration > wall {
				wall = results[i].Duration
			}
		}
	}
	history.Append(filepath.Join(opts.LogDir, "history.log"), results)
	return results, wall
}

// runJob runs a job with its output in its log file.
func runJob(ctx context.Context, job Job, logPath string) (Outcome, Status) {
	log, err := os.Create(logPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hecatron:", err)
		return Outcome{ExitCode: 1}, Failed
	}
	defer log.Close()
	return Execute(ctx, job, log)
}

// Deciseconds is a duration as run-parallel.sh records it.
func Deciseconds(d time.Duration) int64 { return int64(d / (100 * time.Millisecond)) }

// History is history.log: a line per run, "label=deciseconds:status ...",
// the last 50 runs, shared with run-parallel.sh.
type History struct{ lines []string }

// ReadHistory reads a history file; a missing one is empty.
func ReadHistory(path string) History {
	data, err := os.ReadFile(path)
	if err != nil {
		return History{}
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		lines = append(lines, line)
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return History{lines}
}

// Order is the queue in scheduling order: jobs that failed in the last run
// first, then by predicted duration, longest first (stable), and which
// jobs failed last time.
func (h History) Order(jobs []Job, queue []int) ([]int, map[int]bool) {
	recent := h.lines
	if len(recent) > 5 {
		recent = recent[len(recent)-5:]
	}
	predicted := map[int]int64{}
	failedLast := map[int]bool{}
	for _, i := range queue {
		var durations []int64
		for _, line := range recent {
			for _, pair := range strings.Fields(line) {
				if value, ok := strings.CutPrefix(pair, jobs[i].Label+"="); ok {
					ds, _, _ := strings.Cut(value, ":")
					n, err := strconv.ParseInt(ds, 10, 64)
					if err == nil {
						durations = append(durations, n)
					}
				}
			}
		}
		if len(durations) > 0 {
			sort.Slice(durations, func(a, b int) bool { return durations[a] < durations[b] })
			predicted[i] = durations[len(durations)/2]
		}
	}
	if len(recent) > 0 {
		for _, pair := range strings.Fields(recent[len(recent)-1]) {
			label, data, _ := strings.Cut(pair, "=")
			_, status, hasStatus := strings.Cut(data, ":")
			if n, err := strconv.Atoi(status); hasStatus && err == nil && n != 0 {
				for _, i := range queue {
					if jobs[i].Label == label {
						failedLast[i] = true
					}
				}
			}
		}
	}
	key := func(i int) int64 {
		k := predicted[i]
		if failedLast[i] {
			k += 10000000
		}
		return k
	}
	ordered := append([]int(nil), queue...)
	sort.SliceStable(ordered, func(a, b int) bool { return key(ordered[a]) > key(ordered[b]) })
	return ordered, failedLast
}

// Last is a job's entry in the last run history.log records: its duration
// and exit status (0 passed), and false when that run didn't include it.
func (h History) Last(label string) (time.Duration, int, bool) {
	if len(h.lines) == 0 {
		return 0, 0, false
	}
	for _, pair := range strings.Fields(h.lines[len(h.lines)-1]) {
		value, ok := strings.CutPrefix(pair, label+"=")
		if !ok {
			continue
		}
		ds, status, _ := strings.Cut(value, ":")
		n, err := strconv.ParseInt(ds, 10, 64)
		code, statusErr := strconv.Atoi(status)
		if err != nil || statusErr != nil {
			return 0, 0, false
		}
		return time.Duration(n) * 100 * time.Millisecond, code, true
	}
	return 0, 0, false
}

// Append records a run and keeps the last 50.
func (h History) Append(path string, results []Result) {
	var pairs []string
	for _, r := range results {
		if r.Ran() {
			// A status as run-parallel.sh records it, a hecatron command's
			// (timeout(1)'s for a time limit).
			status := 0
			switch r.Status {
			case Failed:
				status = 1
			case TimedOut:
				status = timedOutStatus
			}
			pairs = append(pairs, fmt.Sprintf("%s=%d:%d", r.Label, Deciseconds(r.Duration), status))
		}
	}
	lines := append(append([]string(nil), h.lines...), strings.Join(pairs, " "))
	if len(lines) > 50 {
		lines = lines[len(lines)-50:]
	}
	_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// Lock keeps two runs of a command from sharing a log directory, as
// run-parallel.sh's .lock directory does (it honours that one too): a lock
// older than two minutes is stale.
func Lock(logDir string) (unlock func(), err error) {
	lock := filepath.Join(logDir, ".lock")
	if os.Mkdir(lock, 0o755) != nil {
		if info, statErr := os.Stat(lock); statErr == nil && time.Since(info.ModTime()) > 2*time.Minute {
			os.Remove(lock)
		}
		if err := os.Mkdir(lock, 0o755); err != nil {
			return nil, fmt.Errorf("%s", lock)
		}
	}
	return func() { os.Remove(lock) }, nil
}

// ExitStatus is a run's exit status: 0 when nothing failed, 3 when every
// failure is a component whose toolchain command wasn't on PATH, else 1 (a
// timed-out job included).
func ExitStatus(results []Result) int {
	failed, missing := 0, 0
	for i := range results {
		if results[i].Status == TimedOut {
			failed++
		}
		if results[i].Status == Failed {
			failed++
			if results[i].ToolchainMissing() {
				missing++
			}
		}
	}
	switch {
	case failed == 0:
		return 0
	case missing == failed:
		return 3
	}
	return 1
}
