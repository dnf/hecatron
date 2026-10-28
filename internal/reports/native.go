package reports

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Test cases from the tools' own reports, merged into hecatron's JUnit
// (docs/runner.md, "Reports"):
//
//   - JUnit XML a build tool leaves under the component: Maven's
//     surefire-reports and failsafe-reports, Gradle's build/test-results,
//     sbt's test-reports (TEST-*.xml), written during the component's run;
//   - what a hook writes to HECATRON_TEST_REPORTS: JUnit XML (*.xml: pytest
//     --junitxml, Jest's and Vitest's junit reporters...) and go test -json
//     events (*.json), which the Go test hook saves through `hecatron
//     _gotest`.

// Case is one test case.
type Case struct {
	Class   string
	Name    string
	File    string
	Time    float64 // seconds
	Failure *Failure
	Skipped *string // its message, when skipped
}

// Failure is why a case failed: an assertion (failure) or an error.
type Failure struct {
	Error   bool
	Message string
	Type    string
	Text    string
}

// maxFailureText caps a failure's text: a test's whole output can be large.
const maxFailureText = 64 << 10

type xmlSuite struct {
	Suites []xmlSuite `xml:"testsuite"`
	Cases  []xmlCase  `xml:"testcase"`
}

type xmlCase struct {
	Name      string       `xml:"name,attr"`
	Classname string       `xml:"classname,attr"`
	File      string       `xml:"file,attr"`
	Time      string       `xml:"time,attr"`
	Failures  []xmlProblem `xml:"failure"`
	Errors    []xmlProblem `xml:"error"`
	Skipped   *xmlProblem  `xml:"skipped"`
}

type xmlProblem struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}

// ReadJUnit reads a JUnit XML report's test cases, whether its root is
// <testsuites> or <testsuite>, suites nested or not.
func ReadJUnit(data []byte) ([]Case, error) {
	var root xmlSuite
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	var cases []Case
	var walk func(s xmlSuite)
	walk = func(s xmlSuite) {
		for _, c := range s.Cases {
			out := Case{Class: c.Classname, Name: c.Name, File: c.File, Time: parseSeconds(c.Time)}
			switch {
			case len(c.Failures) > 0:
				p := c.Failures[0]
				out.Failure = &Failure{Message: p.Message, Type: p.Type, Text: capText(strings.TrimSpace(p.Text))}
			case len(c.Errors) > 0:
				p := c.Errors[0]
				out.Failure = &Failure{Error: true, Message: p.Message, Type: p.Type, Text: capText(strings.TrimSpace(p.Text))}
			case c.Skipped != nil:
				message := c.Skipped.Message
				out.Skipped = &message
			}
			cases = append(cases, out)
		}
		for _, child := range s.Suites {
			walk(child)
		}
	}
	walk(root)
	return cases, nil
}

// parseSeconds reads a time attribute; Maven writes thousands separators
// in some locales ("1,234.5").
func parseSeconds(value string) float64 {
	seconds, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(value), ",", ""), 64)
	if err != nil || seconds < 0 {
		return 0
	}
	return seconds
}

func capText(text string) string {
	if len(text) <= maxFailureText {
		return text
	}
	return text[:maxFailureText] + "\n[cut at 64 KiB]"
}

// goEvent is one of go test -json's events (cmd/test2json).
type goEvent struct {
	Action  string
	Package string
	Test    string
	Elapsed float64
	Output  string
}

// ReadGoTest reads go test -json's events into a case per test (and
// subtest), named as go test names them, its package as the class.
// Lines that aren't events are skipped.
func ReadGoTest(r io.Reader) []Case {
	type key struct{ pkg, test string }
	var order []key
	outputs := map[key]*strings.Builder{}
	cases := map[key]*Case{}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), 16<<20)
	for scanner.Scan() {
		var e goEvent
		if json.Unmarshal(scanner.Bytes(), &e) != nil || e.Test == "" {
			continue
		}
		k := key{e.Package, e.Test}
		if _, seen := outputs[k]; !seen {
			order = append(order, k)
			outputs[k] = &strings.Builder{}
		}
		switch e.Action {
		case "output":
			if !goNoise(e.Output) {
				outputs[k].WriteString(e.Output)
			}
		case "pass", "fail", "skip":
			c := &Case{Class: e.Package, Name: e.Test, Time: e.Elapsed}
			text := capText(strings.TrimRight(outputs[k].String(), "\n"))
			switch e.Action {
			case "fail":
				c.Failure = &Failure{Message: "failed", Type: "go test", Text: text}
			case "skip":
				message := strings.TrimSpace(lastLine(text))
				c.Skipped = &message
			}
			cases[k] = c
		}
	}
	var out []Case
	for _, k := range order {
		if c, ok := cases[k]; ok {
			out = append(out, *c)
		} else {
			// Never finished: the test binary died (a panic, a timeout).
			out = append(out, Case{Class: k.pkg, Name: k.test, Failure: &Failure{Message: "did not finish", Type: "go test",
				Text: capText(strings.TrimRight(outputs[k].String(), "\n"))}})
		}
	}
	return out
}

func lastLine(text string) string {
	if i := strings.LastIndexByte(text, '\n'); i >= 0 {
		return text[i+1:]
	}
	return text
}

// goNoise is what go test -json adds to a test's output that go test
// without -v doesn't print: the run markers and the results of tests that
// passed or were skipped.
func goNoise(output string) bool {
	line := strings.TrimLeft(output, " ")
	for _, prefix := range []string{"=== RUN", "=== PAUSE", "=== CONT", "=== NAME", "--- PASS", "--- SKIP"} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// goTest is a test's output as go test -json gives it, for GoTestFilter:
// its "--- FAIL" line, what it printed, and its subtests.
type goTest struct {
	depth    int // a subtest's: its output is indented 4 spaces a level
	result   string
	output   []string
	children []*goTest
}

// write writes a failed test as go test without -v does: its result line
// first, then what it printed, then its failed subtests'.
func (t *goTest) write(out io.Writer, unfinished bool) {
	if t.result == "" && !unfinished {
		return
	}
	indent := strings.Repeat("    ", t.depth)
	for _, line := range append([]string{t.result}, t.output...) {
		if line != "" && line != "\n" {
			line = indent + line
		}
		_, _ = io.WriteString(out, line)
	}
	for _, child := range t.children {
		child.write(out, unfinished)
	}
}

// GoTestFilter copies go test -json's events to raw and prints the output
// go test without -json would have: each package's result lines, and each
// failed test's result line, output and failed subtests, a package's
// output together and packages in the order they started. Lines that
// aren't events (the go command's own messages on stderr) pass through.
func GoTestFilter(in io.Reader, out, raw io.Writer) error {
	var packages []string // started, not yet written
	buffers := map[string]*bytes.Buffer{}
	ended := map[string]bool{}
	failed := false // go test ends with FAIL when a package failed
	pkgOut := func(pkg string) io.Writer {
		if b, ok := buffers[pkg]; ok {
			return b
		}
		b := &bytes.Buffer{}
		buffers[pkg] = b
		packages = append(packages, pkg)
		return b
	}
	flushPackages := func(all bool) {
		for len(packages) > 0 && (all || ended[packages[0]]) {
			_, _ = out.Write(buffers[packages[0]].Bytes())
			delete(buffers, packages[0])
			packages = packages[1:]
		}
	}
	type key struct{ pkg, test string }
	tests := map[key]*goTest{}
	var tops []key // top-level tests not yet written, in order
	node := func(pkg, name string) *goTest {
		k := key{pkg, name}
		if t, ok := tests[k]; ok {
			return t
		}
		t := &goTest{depth: strings.Count(name, "/")}
		tests[k] = t
		if i := strings.LastIndexByte(name, '/'); i >= 0 {
			parent := tests[key{pkg, name[:i]}]
			if parent == nil {
				return t // a subtest of a test already written
			}
			parent.children = append(parent.children, t)
		} else {
			tops = append(tops, k)
		}
		return t
	}
	finish := func(k key, keep, unfinished bool) {
		for i, top := range tops {
			if top == k {
				if keep {
					tests[k].write(pkgOut(k.pkg), unfinished)
				}
				tops = append(tops[:i:i], tops[i+1:]...)
				break
			}
		}
		for other := range tests {
			if other.pkg == k.pkg && (other.test == k.test || strings.HasPrefix(other.test, k.test+"/")) {
				delete(tests, other)
			}
		}
	}
	reader := bufio.NewReader(in)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := raw.Write(line); werr != nil {
				return werr
			}
			var e goEvent
			if bytes.HasPrefix(line, []byte("{")) && json.Unmarshal(line, &e) == nil && e.Action != "" {
				top := e.Test
				if i := strings.IndexByte(top, '/'); i >= 0 {
					top = top[:i]
				}
				switch {
				case e.Action == "build-output":
					_, _ = io.WriteString(out, e.Output)
				case e.Action == "start":
					pkgOut(e.Package)
				case e.Action == "output" && e.Test == "":
					if e.Output != "PASS\n" && !strings.HasPrefix(e.Output, "testing: warning: no tests to run") {
						_, _ = io.WriteString(pkgOut(e.Package), e.Output)
					}
				case e.Action == "output":
					t := node(e.Package, e.Test)
					switch trimmed := strings.TrimLeft(e.Output, " "); {
					case strings.HasPrefix(trimmed, "--- FAIL"):
						t.result = e.Output
					case !goNoise(e.Output):
						t.output = append(t.output, e.Output)
					}
				case e.Test != "" && e.Test == top && (e.Action == "pass" || e.Action == "skip"):
					finish(key{e.Package, top}, false, false)
				case e.Test != "" && e.Test == top && e.Action == "fail":
					finish(key{e.Package, top}, true, false)
				case e.Test == "" && (e.Action == "fail" || e.Action == "pass" || e.Action == "skip"):
					// Tests left unfinished when their package ended (a
					// panic, a timeout): what they printed explains it.
					for _, k := range append([]key(nil), tops...) {
						if k.pkg == e.Package {
							finish(k, true, true)
						}
					}
					pkgOut(e.Package)
					ended[e.Package] = true
					failed = failed || e.Action == "fail"
					flushPackages(false)
				}
			} else {
				_, _ = out.Write(line)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	for _, k := range append([]key(nil), tops...) {
		finish(k, true, true)
	}
	flushPackages(true)
	if failed {
		_, _ = io.WriteString(out, "FAIL\n")
	}
	return nil
}

// buildReportDirs are where build tools write JUnit XML: Maven's surefire
// and failsafe, Gradle's test-results (one directory per test task), sbt's
// test-reports.
var buildReportDirs = []string{"surefire-reports", "failsafe-reports", "test-results", "test-reports"}

// skipDirs are never searched for build reports.
var skipDirs = map[string]bool{"node_modules": true, "vendor": true, ".git": true, ".gradle": true, ".venv": true}

// mtimeSlack allows for file systems that keep modification times to the
// second (or two, FAT).
const mtimeSlack = 2 * time.Second

// BuildReports are the JUnit XML files a build tool wrote under dir between
// from and to: TEST-*.xml in a surefire-reports, failsafe-reports,
// test-results or test-reports directory (or one below it), sorted.
func BuildReports(dir string, from, to time.Time) []string {
	var found []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != dir && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasPrefix(name, "TEST-") || !strings.HasSuffix(name, ".xml") || !underReportDir(path) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if modified := info.ModTime(); modified.Before(from.Add(-mtimeSlack)) || modified.After(to.Add(mtimeSlack)) {
			return nil
		}
		found = append(found, path)
		return nil
	})
	sort.Strings(found)
	return found
}

func underReportDir(path string) bool {
	parent := filepath.Base(filepath.Dir(path))
	grandparent := filepath.Base(filepath.Dir(filepath.Dir(path)))
	for _, name := range buildReportDirs {
		if parent == name || grandparent == name {
			return true
		}
	}
	return false
}

// HookReports are the cases in the files a hook wrote to its
// HECATRON_TEST_REPORTS directory: *.xml as JUnit, *.json as go test -json
// events, in name order. Files it can't read are skipped.
func HookReports(dir string) []Case {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var cases []Case
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		switch filepath.Ext(entry.Name()) {
		case ".xml":
			cases = append(cases, readJUnitFile(path)...)
		case ".json":
			if f, err := os.Open(path); err == nil {
				cases = append(cases, ReadGoTest(f)...)
				_ = f.Close()
			}
		}
	}
	return cases
}

func readJUnitFile(path string) []Case {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	cases, _ := ReadJUnit(data)
	return cases
}
