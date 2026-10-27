package runner

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Live is the display on a terminal while jobs run: a row per job with its
// state (queued, running, passed, failed, skipped), its time, and a bar
// from when it started to when it ended on the run's time line. It redraws
// ten times a second in place.
type Live struct {
	out     io.Writer
	size    func() (columns, rows int)
	color   bool
	results []Result
	started []time.Duration // -1 until started
	ended   []bool

	mu      sync.Mutex
	begin   time.Time
	frame   int
	drawn   int
	stop    chan struct{}
	stopped chan struct{}
}

// NewLive is a live display of jobs on a terminal.
func NewLive(out *os.File, jobs []Job) *Live {
	l := &Live{out: out, color: os.Getenv("NO_COLOR") == "", stop: make(chan struct{}), stopped: make(chan struct{})}
	l.size = func() (int, int) {
		columns, rows, ok := TerminalSize(out)
		if !ok || columns <= 0 || rows <= 0 {
			return 80, 24
		}
		return columns, rows
	}
	for _, job := range jobs {
		r := Result{Label: job.Label}
		switch job.Kind {
		case NotInstalled:
			r.Status = StatusNotInstall
		case Disabled:
			r.Status = StatusDisabled
		case Separator:
			r.Status = StatusSeparator
		default:
			r.Status = statusNotFinished
		}
		l.results = append(l.results, r)
		l.started = append(l.started, -1)
		l.ended = append(l.ended, false)
	}
	return l
}

// Started marks a job started.
func (l *Live) Started(index int, at time.Duration) {
	l.mu.Lock()
	l.started[index] = at
	l.mu.Unlock()
}

// Finished marks a job finished.
func (l *Live) Finished(index int, result Result) {
	l.mu.Lock()
	l.results[index] = result
	l.ended[index] = true
	l.mu.Unlock()
}

// Start begins redrawing.
func (l *Live) Start() {
	l.begin = time.Now()
	_, _ = fmt.Fprint(l.out, "\x1b[?25l")
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			l.draw()
			select {
			case <-l.stop:
				l.draw()
				_, _ = fmt.Fprint(l.out, "\x1b[?25h")
				close(l.stopped)
				return
			case <-ticker.C:
			}
		}
	}()
}

// Stop draws the last frame and restores the cursor.
func (l *Live) Stop() {
	close(l.stop)
	<-l.stopped
}

const spinner = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"

func (l *Live) colorize(code, text string) string {
	if !l.color || code == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (l *Live) draw() {
	l.mu.Lock()
	defer l.mu.Unlock()
	columns, rows := l.size()
	elapsed := time.Since(l.begin)
	labelW := labelWidth(l.results)
	prefixW := labelW + 12
	maxBar := max(columns-prefixW-1, 10)
	total := max(Deciseconds(elapsed), 1)

	var lines []string
	axis := &strings.Builder{}
	writeAxis(axis, total, prefixW, maxBar)
	lines = append(lines, l.colorize("2", strings.TrimRight(axis.String(), "\n")))
	running, done, failed, queued := 0, 0, 0, 0
	spin := []rune(spinner)
	for i, r := range l.results {
		var text, code string
		switch {
		case r.Status == StatusSeparator:
			lines = append(lines, "")
			continue
		case r.Status == StatusDisabled:
			text, code = fmt.Sprintf("  ⊘ %-*s      ⊘ disabled", labelW, r.Label), "38;5;208"
		case r.Status == StatusNotInstall:
			text, code = fmt.Sprintf("  - %-*s      - not installed", labelW, r.Label), "2"
		case l.ended[i]:
			done++
			mark, c, note := "✓", "32", ""
			if r.Unsuccessful() {
				mark, c = "✗", "31"
				failed++
			}
			if r.Status == TimedOut {
				note = " timed out"
			}
			start := Deciseconds(l.started[i])
			text, code = fmt.Sprintf("  %s %-*s %6s %s%s", mark, labelW, r.Label, formatDeciseconds(Deciseconds(r.Duration)),
				bar(start, start+Deciseconds(r.Duration), total, maxBar), note), c
		case l.started[i] >= 0:
			running++
			start := Deciseconds(l.started[i])
			text, code = fmt.Sprintf("  %c %-*s %6s %s", spin[l.frame%len(spin)], labelW, r.Label,
				formatDeciseconds(Deciseconds(elapsed-l.started[i])), bar(start, total, total, maxBar)), "93"
		default:
			queued++
			text, code = fmt.Sprintf("  ◌ %-*s", labelW, r.Label), "2"
		}
		lines = append(lines, l.colorize(code, clip(text, columns-1)))
	}
	status := fmt.Sprintf("  %d done, %d running, %d queued", done, running, queued)
	if failed > 0 {
		status += fmt.Sprintf(", %d failed", failed)
	}
	status += fmt.Sprintf("  (%s)", formatDeciseconds(Deciseconds(elapsed)))
	if limit := rows - 2; len(lines) > limit && limit > 1 {
		hidden := len(lines) - (limit - 1)
		lines = append(lines[:limit-1], l.colorize("2", fmt.Sprintf("  ▼ %d more", hidden)))
	}
	lines = append(lines, l.colorize("2", clip(status, columns-1)))

	var out strings.Builder
	if l.drawn > 0 {
		fmt.Fprintf(&out, "\x1b[%dA\r", l.drawn)
	}
	for _, line := range lines {
		out.WriteString(line + "\x1b[K\n")
	}
	for i := len(lines); i < l.drawn; i++ {
		out.WriteString("\x1b[K\n")
	}
	if extra := l.drawn - len(lines); extra > 0 {
		fmt.Fprintf(&out, "\x1b[%dA", extra)
	}
	_, _ = io.WriteString(l.out, out.String())
	l.drawn = len(lines)
	l.frame++
}

// bar is a job's span on the time line: blanks to its start, blocks to its
// end, at least one block.
func bar(start, end, total int64, width int) string {
	from := int(start * int64(width) / total)
	to := int(end * int64(width) / total)
	from = min(from, width-1)
	to = min(max(to, from+1), width)
	return strings.Repeat(" ", from) + strings.Repeat("█", to-from)
}

func clip(text string, width int) string {
	if width < 1 || utf8.RuneCountInString(text) <= width {
		return text
	}
	return string([]rune(text)[:width])
}
