// Package reports writes a run's results in the formats CI systems and
// forges read (docs/guide/ci.md; docs/guide/runner.md, "Reports"): JUnit XML,
// SARIF 2.1.0, GitLab Code Quality JSON and a GitHub Actions job summary.
// They're written with the normal output, not instead of it.
package reports

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dnf/hecatron/internal/runner"
)

// Suite is one language in one component (or a project task, or a tool),
// as a JUnit test suite.
type Suite struct {
	Name      string // lang/component, or the job's label
	Lang      string
	Component string
	Status    runner.Status
	Failed    bool   // the component (or job) failed or timed out
	Skipped   string // why the component had nothing to do (runner.Component.Skipped)
	ExitCode  int
	Duration  time.Duration
	Log       string
	Excerpt   []runner.ErrorLine
	Cases     []Case // from the tools' own reports; none when they wrote none
}

// SuiteOptions say where to find what the tools reported themselves.
type SuiteOptions struct {
	// Patterns are a language's error patterns, for each component's
	// excerpt; nil for none.
	Patterns func(lang string) *runner.Patterns
	// TestReports is the directory a component's hooks were given as
	// HECATRON_TEST_REPORTS; "" for none.
	TestReports func(lang, component string) string
	// BuildReports reads the JUnit XML build tools write under each
	// component (test runs only).
	BuildReports bool
}

// Suites are a run's results as test suites: one per language and
// component that ran (or per task or tool), and one, skipped, per language
// that didn't (not installed, disabled).
func Suites(r *runner.Report, opts SuiteOptions) []Suite {
	var suites []Suite
	for _, result := range r.Results {
		switch {
		case result.Status == runner.StatusSeparator:
		case !result.Ran():
			suites = append(suites, Suite{Name: result.Label, Lang: result.Lang, Status: result.Status})
		case len(result.Components) == 0:
			suites = append(suites, Suite{Name: result.Label, Lang: result.Lang, Status: result.Status,
				Failed: result.Unsuccessful(), ExitCode: result.ExitCode, Duration: result.Duration, Log: result.Log,
				Excerpt: result.Excerpt.Lines})
		default:
			suites = append(suites, componentSuites(result, opts)...)
		}
	}
	return suites
}

func componentSuites(result runner.Result, opts SuiteOptions) []Suite {
	var parts map[string][]byte
	if result.Unsuccessful() {
		if log, err := os.ReadFile(result.Log); err == nil {
			parts = runner.ComponentLogs(log)
		}
	}
	var patterns *runner.Patterns
	if opts.Patterns != nil {
		patterns = opts.Patterns(result.Lang)
	}
	var suites []Suite
	for _, c := range result.Components {
		s := Suite{Name: result.Label + "/" + c.Name, Lang: result.Lang, Component: c.Name, Status: result.Status,
			Failed: c.Failed, ExitCode: result.ExitCode, Duration: c.Duration, Log: result.Log}
		if s.Duration == 0 && len(result.Components) == 1 {
			s.Duration = result.Duration
		}
		if !c.Failed {
			s.Status, s.ExitCode = runner.Passed, 0
		}
		if c.Skipped != "" && !c.Failed {
			s.Status, s.Skipped = runner.Skipped, c.Skipped
		}
		if c.Failed {
			if part, ok := parts[c.Name]; ok {
				s.Excerpt = runner.Extract(part, patterns).Lines
			}
			if len(s.Excerpt) == 0 {
				for _, line := range result.Excerpt.Lines {
					if line.Component == c.Name {
						s.Excerpt = append(s.Excerpt, line)
					}
				}
			}
		}
		if opts.TestReports != nil {
			if dir := opts.TestReports(result.Lang, c.Name); dir != "" {
				s.Cases = append(s.Cases, HookReports(dir)...)
			}
		}
		if opts.BuildReports && !c.Started.IsZero() {
			for _, path := range BuildReports(c.Dir, c.Started, c.Started.Add(c.Duration)) {
				s.Cases = append(s.Cases, readJUnitFile(path)...)
			}
		}
		suites = append(suites, s)
	}
	return suites
}

// cases are a suite's test cases: the tools' own, plus one for the
// component when it failed and none of theirs did (a build error before
// any test ran); else one case for the component.
func (s Suite) cases() []Case {
	cases := append([]Case(nil), s.Cases...)
	failedCase := false
	for _, c := range cases {
		failedCase = failedCase || c.Failure != nil
	}
	if len(cases) > 0 && (!s.Failed || failedCase) {
		return cases
	}
	own := Case{Class: s.class(), Name: s.caseName(), Time: s.Duration.Seconds()}
	if len(cases) > 0 {
		own.Time = 0 // its time is the cases'
	}
	switch {
	case s.Status == runner.StatusNotInstall || s.Status == runner.StatusDisabled:
		message := string(s.Status)
		own.Skipped = &message
	case s.Status == runner.Skipped:
		message := "skipped: " + s.Skipped
		own.Skipped = &message
	case s.Failed:
		own.Failure = s.failure()
	}
	return append(cases, own)
}

func (s Suite) class() string {
	if s.Lang != "" {
		return s.Lang
	}
	return s.Name
}

func (s Suite) caseName() string {
	if s.Component != "" {
		return s.Component
	}
	return s.Name
}

// failure is a failed component's: its first error line as the message,
// the excerpt and log as the text.
func (s Suite) failure() *Failure {
	f := &Failure{Type: string(s.Status), Message: fmt.Sprintf("exit %d", s.ExitCode)}
	if s.Status == runner.TimedOut {
		f.Message = "timed out"
	}
	var text []string
	for _, line := range s.Excerpt {
		text = append(text, line.Text)
	}
	if len(text) > 0 {
		f.Message = text[0]
	}
	if s.Log != "" {
		text = append(text, "log: "+s.Log)
	}
	f.Text = strings.Join(text, "\n")
	return f
}

type counts struct{ tests, failures, errors, skipped int }

func (c *counts) add(cases []Case) {
	for _, tc := range cases {
		c.tests++
		switch {
		case tc.Failure != nil && tc.Failure.Error:
			c.errors++
		case tc.Failure != nil:
			c.failures++
		case tc.Skipped != nil:
			c.skipped++
		}
	}
}

func (c counts) attrs() string {
	return fmt.Sprintf(` tests="%d" failures="%d" errors="%d" skipped="%d"`, c.tests, c.failures, c.errors, c.skipped)
}

// WriteJUnit writes the suites as JUnit XML (the form Jenkins, GitLab,
// Azure Pipelines, Buildkite and CircleCI read): <testsuites> named after
// the command, a <testsuite> per suite, its cases with their failures.
func WriteJUnit(w io.Writer, action string, wall time.Duration, suites []Suite) error {
	var total counts
	all := make([][]Case, len(suites))
	for i, s := range suites {
		all[i] = s.cases()
		total.add(all[i])
	}
	var b strings.Builder
	b.WriteString(xml.Header)
	fmt.Fprintf(&b, "<testsuites name=%s%s time=%s>\n", attr("hct "+action), total.attrs(), seconds(wall.Seconds()))
	for i, s := range suites {
		var c counts
		c.add(all[i])
		fmt.Fprintf(&b, "  <testsuite name=%s%s time=%s>\n", attr(s.Name), c.attrs(), seconds(s.Duration.Seconds()))
		if s.Log != "" {
			fmt.Fprintf(&b, "    <properties>\n      <property name=\"log\" value=%s/>\n    </properties>\n", attr(s.Log))
		}
		for _, tc := range all[i] {
			fmt.Fprintf(&b, "    <testcase name=%s classname=%s", attr(tc.Name), attr(tc.Class))
			if tc.File != "" {
				fmt.Fprintf(&b, " file=%s", attr(tc.File))
			}
			fmt.Fprintf(&b, " time=%s", seconds(tc.Time))
			switch {
			case tc.Failure != nil:
				element := "failure"
				if tc.Failure.Error {
					element = "error"
				}
				fmt.Fprintf(&b, ">\n      <%s message=%s type=%s>", element, attr(oneLine(tc.Failure.Message)), attr(tc.Failure.Type))
				b.WriteString(text(tc.Failure.Text))
				fmt.Fprintf(&b, "</%s>\n    </testcase>\n", element)
			case tc.Skipped != nil:
				fmt.Fprintf(&b, ">\n      <skipped message=%s/>\n    </testcase>\n", attr(oneLine(*tc.Skipped)))
			default:
				b.WriteString("/>\n")
			}
		}
		b.WriteString("  </testsuite>\n")
	}
	b.WriteString("</testsuites>\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func seconds(s float64) string { return fmt.Sprintf(`"%.3f"`, s) }

func attr(value string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(value))
	return `"` + b.String() + `"`
}

// text is element text: escaped, with its line breaks and tabs kept
// readable.
func text(value string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(value))
	return strings.NewReplacer("&#xA;", "\n", "&#x9;", "\t").Replace(b.String())
}

func oneLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 500 {
		s = strings.ToValidUTF8(s[:500], "")
	}
	return s
}
