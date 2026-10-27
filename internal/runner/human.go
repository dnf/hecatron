package runner

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// Human output off a terminal is run-parallel.sh's: a time axis, a bar per
// job scaled to the longest, the tally and the log directory, then the log
// of each failed job.
//
// Output here and in render.go goes to the terminal: a write that fails
// there has nowhere to be reported, so its error is dropped.

func labelWidth(results []Result) int {
	width := 6
	for _, r := range results {
		if r.Status != StatusSeparator {
			width = max(width, utf8.RuneCountInString(r.Label))
		}
	}
	return width
}

// formatDeciseconds is run-parallel.sh's duration: 12.3s.
func formatDeciseconds(ds int64) string { return fmt.Sprintf("%d.%ds", ds/10, ds%10) }

// WriteNothingRan is the output when every job was skipped.
func WriteNothingRan(w io.Writer, r *Report) {
	width := labelWidth(r.Results)
	notInstalled, disabled := 0, 0
	for _, result := range r.Results {
		switch result.Status {
		case StatusSeparator:
			_, _ = fmt.Fprintln(w)
		case StatusDisabled:
			disabled++
			_, _ = fmt.Fprintf(w, "  ⊘ %-*s      ⊘ disabled\n", width, result.Label)
		default:
			notInstalled++
			_, _ = fmt.Fprintf(w, "  - %-*s    - not installed\n", width, result.Label)
		}
	}
	summary := "0/0 passed"
	if notInstalled > 0 {
		summary += fmt.Sprintf(", %d not installed", notInstalled)
	}
	if disabled > 0 {
		summary += fmt.Sprintf(", %d disabled", disabled)
	}
	_, _ = fmt.Fprintf(w, "\n%s\nLogs: %s/\n", summary, r.LogDir)
}

// WriteChart writes the bar chart of a finished run, width columns wide.
func WriteChart(w io.Writer, r *Report, slotMode bool, width int) {
	labelW := labelWidth(r.Results)
	prefixW := labelW + 12
	if slotMode {
		prefixW = labelW + 15
	}
	maxBar := max(width-prefixW, 10)
	longest := int64(1)
	for _, result := range r.Results {
		if result.Ran() {
			longest = max(longest, Deciseconds(result.Duration))
		}
	}
	writeAxis(w, longest, prefixW, maxBar)
	for _, result := range r.Results {
		switch result.Status {
		case StatusSeparator:
			_, _ = fmt.Fprintln(w)
		case StatusDisabled:
			_, _ = fmt.Fprintf(w, "  ⊘ %-*s      ⊘ disabled\n", labelW, result.Label)
		case StatusNotInstall:
			_, _ = fmt.Fprintf(w, "  - %-*s      - not installed\n", labelW, result.Label)
		default:
			ds := Deciseconds(result.Duration)
			bar := min(max(int(ds*int64(maxBar)/longest), 1), maxBar)
			mark, note := "✓", ""
			switch result.Status {
			case Failed:
				mark = "✗"
			case TimedOut:
				mark, note = "✗", " timed out ("+FormatLimit(result.Limit.Duration)+" limit: "+result.Limit.Describe()+")"
			}
			_, _ = fmt.Fprintf(w, "  %s %-*s %6s %s%s\n", mark, labelW, result.Label, formatDeciseconds(ds), strings.Repeat("█", bar), note)
		}
	}
}

func writeAxis(w io.Writer, longest int64, prefixW, maxBar int) {
	var line strings.Builder
	line.WriteString(strings.Repeat(" ", prefixW))
	maxSec := (longest + 9) / 10
	interval := int64(30)
	switch {
	case maxSec <= 5:
		interval = 1
	case maxSec <= 20:
		interval = 5
	case maxSec <= 60:
		interval = 10
	}
	end := 0
	for t := int64(0); t <= maxSec; t += interval {
		col := int(t * 10 * int64(maxBar) / longest)
		label := fmt.Sprint(t)
		if col >= end && col+len(label) <= maxBar {
			line.WriteString(strings.Repeat(" ", col-end) + label)
			end = col + len(label)
		}
	}
	_, _ = fmt.Fprintln(w, line.String())
}

// WriteTally writes the passed count, the log directory and each failed
// job's log, as run-parallel.sh ends.
func WriteTally(w io.Writer, r *Report) {
	passed, failed, timedOut, _ := r.counts()
	notInstalled, disabled := 0, 0
	for _, result := range r.Results {
		switch result.Status {
		case StatusNotInstall:
			notInstalled++
		case StatusDisabled:
			disabled++
		}
	}
	extra := ""
	if timedOut > 0 {
		extra += fmt.Sprintf(", %d timed out", timedOut)
	}
	if notInstalled > 0 {
		extra += fmt.Sprintf(", %d not installed", notInstalled)
	}
	if disabled > 0 {
		extra += fmt.Sprintf(", %d disabled", disabled)
	}
	_, _ = fmt.Fprintf(w, "\n%d/%d passed%s (%s wall time)\nLogs: %s/\n", passed, passed+failed+timedOut, extra,
		formatDeciseconds(Deciseconds(r.Wall)), r.LogDir)
	if failed+timedOut == 0 {
		return
	}
	_, _ = fmt.Fprintln(w)
	for _, result := range r.Results {
		if !result.Unsuccessful() {
			continue
		}
		_, _ = fmt.Fprintf(w, "=== %s ===\n", result.Label)
		if data, err := os.ReadFile(result.Log); err == nil {
			_, _ = w.Write(data)
		}
		_, _ = fmt.Fprintln(w)
	}
}
