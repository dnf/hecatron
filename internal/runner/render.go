package runner

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

// Format is how a run is shown (docs/contributing/design.md §19, "Output").
type Format string

const (
	Human Format = "human" // a live display on a terminal, a summary off one
	Agent Format = "agent" // a line, then only the failures with their error lines
	JSON  Format = "json"  // one document
	CI    Format = "ci"    // human, plus GitHub annotations for the error lines
)

// SelectFormat chooses the format: the --output flag, else
// HECATRON_OUTPUT, else agent output when stdout isn't a terminal and an
// agent marker is set (CLAUDECODE=1), else CI when GITHUB_ACTIONS=true,
// else human.
func SelectFormat(flag string, getenv func(string) string, stdoutTerminal bool) (Format, error) {
	if flag != "" {
		return parseFormat(flag, "--output")
	}
	if value := getenv("HECATRON_OUTPUT"); value != "" {
		return parseFormat(value, "HECATRON_OUTPUT")
	}
	if !stdoutTerminal && getenv("CLAUDECODE") == "1" {
		return Agent, nil
	}
	if getenv("GITHUB_ACTIONS") == "true" {
		return CI, nil
	}
	return Human, nil
}

func parseFormat(value, source string) (Format, error) {
	switch f := Format(value); f {
	case Human, Agent, JSON, CI:
		return f, nil
	}
	return "", fmt.Errorf("%s must be human, agent, json or ci (got '%s')", source, value)
}

// Report is a finished run, for the renderers.
type Report struct {
	Action     string   // test, lint...
	Targets    []string // as given; none for everything
	Components []string // -c
	Results    []Result
	Wall       time.Duration
	LogDir     string
	Root       string   // the project root
	Cwd        string   // where paths are shown from
	Changed    *Changed // what --changed selected, else nil
	Audit      *Audit   // hct audit's findings, for JSON output; nil for none
}

// Audit is what hct audit found, every finding of every component, as
// its JSON output lists them (docs/guide/output.md).
type Audit struct {
	Vulnerabilities []Vulnerability
	Secrets         []Secret
}

// Vulnerability is one advisory affecting one package in one component.
type Vulnerability struct {
	Component string   `json:"component"`
	Language  string   `json:"language"`
	Package   string   `json:"package"`
	Version   string   `json:"version"` // installed; "" where the auditor doesn't say
	Fixed     []string `json:"fixed"`   // the versions that fix it; [] where the auditor doesn't say
	ID        string   `json:"id"`
	Aliases   []string `json:"aliases"`
	Severity  string   `json:"severity"` // critical, high, moderate, low or unrated
	Title     string   `json:"title"`
	URL       string   `json:"url,omitempty"`
	Tool      string   `json:"tool"`           // the auditor: npm, pnpm, yarn-classic, yarn-berry, bun, pip-audit, govulncheck, composer, bundle-audit, dotnet, osv-scanner
	File      string   `json:"file,omitempty"` // the lock file or manifest, relative to the workspace
	Line      int      `json:"line,omitempty"`
	Accepted  bool     `json:"accepted"` // an accepted risk (audit.ignore): listed, not counted
	Reason    string   `json:"reason,omitempty"`
	Until     string   `json:"until,omitempty"`
}

// Secret is one possible secret the secret scan found: never the secret.
type Secret struct {
	Rule        string `json:"rule"`
	Description string `json:"description"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
	Commit      string `json:"commit,omitempty"`
	Fingerprint string `json:"fingerprint"`
}

// Changed is what --changed selected and why.
type Changed struct {
	Line       string   `json:"summary"` // in one line
	Base       string   `json:"base,omitempty"`
	MergeBase  string   `json:"mergeBase,omitempty"`
	Files      []string `json:"files"`
	Everything bool     `json:"everything"`
	Languages  []string `json:"languages,omitempty"` // selected, when not everything
}

func (r *Report) counts() (passed, failed, timedOut, skipped int) {
	for _, result := range r.Results {
		switch result.Status {
		case Passed:
			passed++
		case Failed:
			failed++
		case TimedOut:
			timedOut++
		case StatusNotInstall, StatusDisabled, Skipped:
			skipped++
		}
	}
	return
}

// Rerun is the command that runs the failed jobs again.
func (r *Report) Rerun() string {
	var failed []string
	for _, result := range r.Results {
		if result.Unsuccessful() {
			failed = append(failed, result.Label)
		}
	}
	if len(failed) == 0 {
		return ""
	}
	words := append([]string{"hct", r.Action}, failed...)
	for _, c := range r.Components {
		words = append(words, "-c", c)
	}
	return strings.Join(words, " ")
}

// shownPath is a path relative to the working directory, when that is
// shorter.
func (r *Report) shownPath(path string) string {
	if r.Cwd != "" {
		if rel, err := filepath.Rel(r.Cwd, path); err == nil && len(rel) < len(path) {
			return rel
		}
	}
	return path
}

// componentDir is a component's directory as shown: relative to the root,
// with a trailing slash.
func (r *Report) componentDir(dir string) string {
	if rel, err := filepath.Rel(r.Root, dir); err == nil && !strings.HasPrefix(rel, "..") {
		dir = rel
	}
	if dir == "." {
		return "./"
	}
	return filepath.ToSlash(dir) + "/"
}

// FormatDuration is a duration as agent output shows it: 0.4s, 12s, 4m31s,
// 1h02m.
func FormatDuration(d time.Duration) string {
	switch {
	case d < 10*time.Second:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

// WriteAgent writes agent output: one summary line; then each failure with
// its components, exit status, time, log and error lines; then the command
// that reruns them. A run where nothing failed is one line.
func WriteAgent(w io.Writer, r *Report) {
	if r.Changed != nil {
		_, _ = fmt.Fprintln(w, r.Changed.Line)
	}
	passed, failed, timedOut, skipped := r.counts()
	summary := fmt.Sprintf("hct %s: %d passed", r.Action, passed)
	if failed > 0 {
		summary += fmt.Sprintf(", %d failed", failed)
	}
	if timedOut > 0 {
		summary += fmt.Sprintf(", %d timed out", timedOut)
	}
	if skipped > 0 {
		summary += fmt.Sprintf(", %d skipped", skipped)
	}
	_, _ = fmt.Fprintf(w, "%s (%s)\n", summary, FormatDuration(r.Wall))
	for _, result := range r.Results {
		for _, dir := range result.Docs {
			_, _ = fmt.Fprintf(w, "%s: docs: written to %s\n", result.Label, dir)
		}
	}
	writeSkips(w, r)
	if failed+timedOut == 0 {
		return
	}
	for _, result := range r.Results {
		if !result.Unsuccessful() {
			continue
		}
		where := ""
		var dirs []string
		for _, c := range result.FailedComponents() {
			dirs = append(dirs, r.componentDir(c.Dir))
		}
		if len(dirs) > 0 {
			where = " (" + strings.Join(dirs, ", ") + ")"
		}
		reason := fmt.Sprintf("exit %d", result.ExitCode)
		if result.ToolchainMissing() {
			reason = "toolchain not found"
		}
		if result.Status == TimedOut {
			_, _ = fmt.Fprintf(w, "TIMEOUT %s%s stopped at its %s limit (%s), %s, log %s\n", result.Label, where,
				FormatLimit(result.Limit.Duration), result.Limit.Describe(), FormatDuration(result.Duration), r.shownPath(result.Log))
		} else {
			_, _ = fmt.Fprintf(w, "FAIL %s%s %s, %s, log %s\n", result.Label, where, reason,
				FormatDuration(result.Duration), r.shownPath(result.Log))
		}
		for _, line := range result.Excerpt.Lines {
			_, _ = fmt.Fprintf(w, "  %s\n", line.Text)
		}
	}
	_, _ = fmt.Fprintf(w, "Rerun: %s\n", r.Rerun())
}

// writeSkips writes a line for each job that had nothing to do in some
// component, with why: not scanned isn't the same as clean.
func writeSkips(w io.Writer, r *Report) {
	for _, result := range r.Results {
		reason := result.SkipReason()
		if reason == "" || result.Unsuccessful() {
			continue
		}
		var dirs []string
		for _, c := range result.Components {
			if c.Skipped != "" {
				dirs = append(dirs, r.componentDir(c.Dir))
			}
		}
		_, _ = fmt.Fprintf(w, "SKIP %s (%s): %s\n", result.Label, strings.Join(dirs, ", "), reason)
	}
}

type jsonRun struct {
	Command  string     `json:"command"`
	Status   string     `json:"status"`
	Exit     int        `json:"exitCode"`
	Duration float64    `json:"duration"`
	Summary  jsonCounts `json:"summary"`
	Logs     string     `json:"logs"`
	Jobs     []jsonJob  `json:"jobs"`
	Rerun    string     `json:"rerun,omitempty"`
	Changed  *Changed   `json:"changed,omitempty"`

	Vulnerabilities *[]Vulnerability `json:"vulnerabilities,omitempty"` // hct audit's, all of them
	Secrets         *[]Secret        `json:"secrets,omitempty"`
}

type jsonCounts struct {
	Passed   int `json:"passed"`
	Failed   int `json:"failed"`
	TimedOut int `json:"timedOut"`
	Skipped  int `json:"skipped"`
}

type jsonLimit struct {
	Seconds     float64 `json:"seconds"`
	Source      string  `json:"source"` // --timeout, HECATRON_TIMEOUT or history
	Description string  `json:"description"`
}

type jsonJob struct {
	Label      string      `json:"label"`
	Kind       string      `json:"kind"`
	Language   string      `json:"language,omitempty"`
	Status     string      `json:"status"`
	Reason     string      `json:"reason,omitempty"` // why it, or some of its components, had nothing to do
	ExitCode   *int        `json:"exitCode,omitempty"`
	Duration   *float64    `json:"duration,omitempty"`
	Log        string      `json:"log,omitempty"`
	TimeLimit  *jsonLimit  `json:"timeLimit,omitempty"`
	Components []Component `json:"components,omitempty"`
	Errors     []ErrorLine `json:"errors,omitempty"`
	Source     string      `json:"errorsFrom,omitempty"`
	Docs       []string    `json:"docs,omitempty"` // hct docs: where they were written
}

// WriteJSON writes the run as one JSON document.
func WriteJSON(w io.Writer, r *Report) error {
	passed, failed, timedOut, skipped := r.counts()
	doc := jsonRun{Command: r.Action, Status: "passed", Exit: ExitStatus(r.Results), Duration: seconds(r.Wall),
		Summary: jsonCounts{passed, failed, timedOut, skipped}, Logs: r.LogDir, Jobs: []jsonJob{}, Rerun: r.Rerun(), Changed: r.Changed}
	if failed+timedOut > 0 {
		doc.Status = "failed"
	}
	if r.Audit != nil {
		vulnerabilities := append([]Vulnerability{}, r.Audit.Vulnerabilities...)
		secrets := append([]Secret{}, r.Audit.Secrets...)
		doc.Vulnerabilities, doc.Secrets = &vulnerabilities, &secrets
	}
	for _, result := range r.Results {
		if result.Status == StatusSeparator {
			continue
		}
		job := jsonJob{Label: result.Label, Kind: "language", Language: result.Lang, Status: string(result.Status),
			Reason: result.SkipReason()}
		if result.Task {
			job.Kind = "task"
		} else if result.Lang == "" {
			job.Kind = "tool"
		}
		if result.Ran() {
			exit, duration := result.ExitCode, seconds(result.Duration)
			job.ExitCode, job.Duration, job.Log = &exit, &duration, result.Log
			job.Components, job.Docs = result.Components, result.Docs
			if l := result.Limit; l.Duration > 0 {
				job.TimeLimit = &jsonLimit{l.Duration.Seconds(), string(l.Source), l.Describe()}
			}
		}
		if result.Unsuccessful() {
			job.Errors, job.Source = result.Excerpt.Lines, result.Excerpt.Source
		}
		doc.Jobs = append(doc.Jobs, job)
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(doc)
}

func seconds(d time.Duration) float64 { return float64(Deciseconds(d)) / 10 }

// WriteAnnotations writes a GitHub Actions error annotation for each failed
// job, and one for each of its error lines that names a file and line,
// the file relative to the workspace (GITHUB_WORKSPACE, else the root).
func WriteAnnotations(w io.Writer, r *Report, workspace string) {
	if workspace == "" {
		workspace = r.Root
	}
	for _, result := range r.Results {
		if result.Status == TimedOut {
			writeTimedOutAnnotation(w, r, result)
			continue
		}
		if result.Status != Failed {
			continue
		}
		title := fmt.Sprintf("hct %s %s", r.Action, result.Label)
		annotated := 0
		for _, line := range result.Excerpt.Lines {
			if line.File == "" || line.Line == 0 {
				continue
			}
			file := r.WorkspacePath(result, line, workspace)
			properties := "file=" + escapeProperty(file) + ",line=" + fmt.Sprint(line.Line)
			if line.Column > 0 {
				properties += ",col=" + fmt.Sprint(line.Column)
			}
			message := line.Message
			if message == "" {
				message = line.Text
			}
			_, _ = fmt.Fprintf(w, "::error %s,title=%s::%s\n", properties, escapeProperty(title), escapeData(message))
			annotated++
		}
		if annotated == 0 {
			var text []string
			for _, line := range result.Excerpt.Lines {
				text = append(text, line.Text)
			}
			message := fmt.Sprintf("%s failed (exit %d); log %s", result.Label, result.ExitCode, result.Log)
			if len(text) > 0 {
				message += "\n" + strings.Join(text, "\n")
			}
			_, _ = fmt.Fprintf(w, "::error title=%s::%s\n", escapeProperty(title), escapeData(message))
		}
	}
}

// WorkspacePath is the file an error line names, joined to its component's
// directory (else the root) when relative, and relative to the workspace
// when inside it, with forward slashes.
func (r *Report) WorkspacePath(result Result, line ErrorLine, workspace string) string {
	if workspace == "" {
		workspace = r.Root
	}
	file := line.File
	if !filepath.IsAbs(file) {
		base := r.Root
		for _, c := range result.Components {
			if c.Name == line.Component && c.Dir != "" {
				base = c.Dir
				break
			}
		}
		file = filepath.Join(base, file)
	}
	if rel, err := filepath.Rel(workspace, file); err == nil && !strings.HasPrefix(rel, "..") {
		file = rel
	}
	return filepath.ToSlash(file)
}

// writeTimedOutAnnotation is a timed-out job's one annotation: the limit,
// how it was set, the log and its last lines. Its error lines are left
// unplaced: the job didn't stop at them.
func writeTimedOutAnnotation(w io.Writer, r *Report, result Result) {
	title := fmt.Sprintf("hct %s %s", r.Action, result.Label)
	message := fmt.Sprintf("%s timed out: stopped at its %s limit (%s) after %s; log %s", result.Label,
		FormatLimit(result.Limit.Duration), result.Limit.Describe(), FormatDuration(result.Duration), result.Log)
	for _, line := range result.Excerpt.Lines {
		message += "\n" + line.Text
	}
	_, _ = fmt.Fprintf(w, "::error title=%s::%s\n", escapeProperty(title), escapeData(message))
}

func escapeData(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

func escapeProperty(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C").Replace(s)
}
