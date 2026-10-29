package reports

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dnf/hecatron/internal/advisor"
	"github.com/dnf/hecatron/internal/licenses"
	"github.com/dnf/hecatron/internal/runner"
)

// A GitHub Actions job summary (#101): when GITHUB_STEP_SUMMARY names a
// file, each command appends Markdown to it, shown on the run's page.

// maxRows caps a findings table; the rest are counted.
const maxRows = 100

// AppendSummary appends Markdown to the job summary file; nothing when
// path is "". Failing to write it never fails a run: the error is for a
// warning.
func AppendSummary(path, markdown string) error {
	if path == "" || markdown == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(f, markdown); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// RunSummary is a run command's summary: its line, a table of languages
// and components with their status and time, then each failure with its
// error lines, log and the command that reruns them (agent output's).
func RunSummary(r *runner.Report, suites []Suite) string {
	var b strings.Builder
	var agent strings.Builder
	runner.WriteAgent(&agent, r)
	lines := strings.Split(strings.TrimRight(agent.String(), "\n"), "\n")
	heading := 0
	if r.Changed != nil {
		heading = 1
	}
	fmt.Fprintf(&b, "### %s\n\n", cell(lines[min(heading, len(lines)-1)]))
	if r.Changed != nil {
		fmt.Fprintf(&b, "%s\n\n", cell(r.Changed.Line))
	}
	if len(suites) > 0 {
		b.WriteString("| Language | Component | Status | Time |\n| --- | --- | --- | --- |\n")
		for _, s := range suites {
			name, component := s.Lang, s.Component
			if name == "" {
				name = s.Name
			}
			status := string(s.Status)
			if s.Failed {
				status = "**" + status + "**"
			}
			duration := ""
			if s.Duration > 0 {
				duration = runner.FormatDuration(s.Duration)
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", cell(name), cell(component), status, duration)
		}
		b.WriteString("\n")
	}
	for _, result := range r.Results {
		if !result.Unsuccessful() {
			continue
		}
		where := ""
		var dirs []string
		for _, c := range result.FailedComponents() {
			dirs = append(dirs, relDir(r.Root, c.Dir))
		}
		if len(dirs) > 0 {
			where = " (" + strings.Join(dirs, ", ") + ")"
		}
		state := fmt.Sprintf("FAIL %s%s: exit %d", result.Label, where, result.ExitCode)
		if result.Status == runner.TimedOut {
			state = fmt.Sprintf("TIMEOUT %s%s: stopped at its %s limit", result.Label, where, runner.FormatLimit(result.Limit.Duration))
		}
		fmt.Fprintf(&b, "#### %s, %s\n\n", cell(state), runner.FormatDuration(result.Duration))
		fmt.Fprintf(&b, "Log: %s\n\n", code(relPath(r.Root, result.Log)))
		var text []string
		for _, line := range result.Excerpt.Lines {
			text = append(text, line.Text)
		}
		if len(text) > 0 {
			b.WriteString(fence(strings.Join(text, "\n")))
		}
	}
	if rerun := r.Rerun(); rerun != "" {
		fmt.Fprintf(&b, "Rerun: %s\n\n", code(rerun))
	}
	return b.String()
}

// AuditSummary is a findings table for hct audit: the most severe first,
// accepted risks marked.
func AuditSummary(findings []AuditFinding) string {
	var b strings.Builder
	kept, accepted := 0, 0
	for _, f := range findings {
		if f.Accepted {
			accepted++
		} else {
			kept++
		}
	}
	fmt.Fprintf(&b, "#### Audit findings: %d", kept)
	if accepted > 0 {
		fmt.Fprintf(&b, ", %d accepted", accepted)
	}
	b.WriteString("\n\n")
	if len(findings) == 0 {
		return b.String()
	}
	b.WriteString("| Severity | Advisory | Package | Version | Where | Title |\n| --- | --- | --- | --- | --- | --- |\n")
	for i, f := range sortedFindings(findings) {
		if i == maxRows {
			fmt.Fprintf(&b, "\n%d more: hct audit lists them all.\n", len(findings)-maxRows)
			break
		}
		severity := f.Severity
		if severity == "" {
			severity = "unrated"
		}
		if f.Accepted {
			severity += " (accepted until " + f.Until + ")"
		}
		advisory := cell(f.ID)
		if f.URL != "" {
			advisory = "[" + advisory + "](" + f.URL + ")"
		}
		where := f.Lang + " " + f.Component
		if f.File != "" {
			where = f.File
		}
		if f.IsSecret() {
			where = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", cell(severity), advisory, cell(f.Package), cell(f.Version), cell(where), cell(f.Title))
	}
	b.WriteString("\n")
	return b.String()
}

// LicensesSummary is hct licenses' summary line, its policy's violations
// and the licenses rolled up.
func LicensesSummary(r *licenses.Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### %s\n\n", cell(r.Summary()))
	if r.Policy != nil && len(r.Policy.Violations) > 0 {
		b.WriteString("| Violation | Version | License | Why | Where |\n| --- | --- | --- | --- | --- |\n")
		for i, v := range r.Policy.Violations {
			if i == maxRows {
				fmt.Fprintf(&b, "\n%d more: hct licenses lists them all.\n", len(r.Policy.Violations)-maxRows)
				break
			}
			license := v.License
			if license == "" {
				license = "unknown"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s %s |\n", cell(v.Package), cell(v.Version), cell(license), cell(v.Reason), cell(v.Language), cell(v.Component))
		}
		b.WriteString("\n")
	}
	if len(r.Licenses) > 0 {
		b.WriteString("| License | Packages | Languages |\n| --- | --- | --- |\n")
		for i, l := range r.Licenses {
			if i == maxRows {
				fmt.Fprintf(&b, "\n%d more: hct licenses lists them all.\n", len(r.Licenses)-maxRows)
				break
			}
			fmt.Fprintf(&b, "| %s | %d | %s |\n", cell(l.License), l.Packages, cell(strings.Join(l.Languages, ", ")))
		}
		b.WriteString("\n")
	}
	for _, n := range r.NotCovered {
		where := n.Language
		if n.Component != "" {
			where += " (" + n.Component + ")"
		}
		fmt.Fprintf(&b, "- Not covered: %s: %s\n", cell(where), cell(n.Reason))
	}
	if len(r.NotCovered) > 0 {
		b.WriteString("\n")
	}
	return b.String()
}

// AdvisorSummary is hct advisor's findings as a table.
func AdvisorSummary(r *advisor.Report) string {
	var b strings.Builder
	recommended, suggestions := r.Count(advisor.Recommended), r.Count(advisor.Suggestion)
	fmt.Fprintf(&b, "### hct advisor: %d recommended, %d suggestions\n\n", recommended, suggestions)
	if len(r.Findings) == 0 {
		return b.String()
	}
	b.WriteString("| Level | Rule | Where | Advice |\n| --- | --- | --- | --- |\n")
	for i, f := range r.Findings {
		if i == maxRows {
			fmt.Fprintf(&b, "\n%d more: hct advisor lists them all.\n", len(r.Findings)-maxRows)
			break
		}
		where := f.Dir + "/"
		if f.Dir == "." {
			where = "the repository"
		}
		if len(f.Languages) > 0 {
			where += " (" + strings.Join(f.Languages, ", ") + ")"
		}
		advice := cell(f.Message)
		if f.Link != "" {
			advice += " ([why](" + f.Link + "))"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", cell(f.Level), cell(f.ID), cell(where), advice)
	}
	b.WriteString("\n")
	return b.String()
}

// cell is text for a Markdown table cell or heading: on one line, its
// pipes and HTML escaped.
func cell(s string) string {
	s = strings.NewReplacer("\r", " ", "\n", " ", "|", `\|`, "<", "&lt;", ">", "&gt;").Replace(s)
	return strings.TrimSpace(s)
}

// code is inline code, its delimiter longer than any backtick run in it.
func code(s string) string {
	ticks := strings.Repeat("`", longestRun(s, '`')+1)
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		s = " " + s + " "
	}
	return ticks + s + ticks
}

// fence is a fenced code block that its text can't close.
func fence(s string) string {
	ticks := strings.Repeat("`", max(3, longestRun(s, '`')+1))
	return ticks + "text\n" + s + "\n" + ticks + "\n\n"
}

func longestRun(s string, c byte) int {
	longest, run := 0, 0
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	return longest
}

// relPath is a path relative to the root when inside it.
func relPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && root != "" && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return path
}

// relDir is a component's directory as agent output shows it.
func relDir(root, dir string) string {
	rel := relPath(root, dir)
	if rel == "." {
		return "./"
	}
	return rel + "/"
}
