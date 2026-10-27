package runner

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

var updateGolden = flag.Bool("update", false, "rewrite the renderers' golden files")

func TestParseSlots(t *testing.T) {
	cpus := runtime.NumCPU()
	cases := map[string]int{"": max((cpus*50+50)/100, 1), "50%": max((cpus*50+50)/100, 1), "100%": cpus,
		"200%": cpus * 2, "1%": max((cpus+50)/100, 1), "3": 3, "0": 0, "unlimited": 0}
	for value, want := range cases {
		got, err := ParseSlots(value)
		if err != nil || got != want {
			t.Errorf("ParseSlots(%q) = %d, %v; want %d", value, got, err, want)
		}
	}
	if n, err := ParseSlots("pcores"); err != nil || n < 1 {
		t.Errorf("pcores: %d, %v", n, err)
	}
	for _, bad := range []string{"0%", "201%", "x%", "-1", "two", "1.5"} {
		if _, err := ParseSlots(bad); err == nil {
			t.Errorf("ParseSlots(%q) accepted", bad)
		}
	}
}

func labelled(labels ...string) []Job {
	var jobs []Job
	for _, label := range labels {
		jobs = append(jobs, Job{Label: label})
	}
	return jobs
}

// The queue is longest predicted first (the median of the last five runs),
// jobs that failed in the last run ahead of all others.
func TestHistoryOrdersTheQueue(t *testing.T) {
	jobs := labelled("a", "b", "c", "d", "e")
	h := History{lines: []string{
		"a=999:0 b=999:0", // older than the last five
		"a=10:0 b=50:0 c=5:0",
		"a=12:0 b=40:0 c=5:0",
		"a=11:0 b=60:0 c=5:0",
		"a=13:0 b=55:0 c=5:0",
		"a=10:0 b=50:0 c=5:1 d=1:1",
	}}
	order, failed := h.Order(jobs, []int{0, 1, 2, 3, 4})
	if want := []int{2, 3, 1, 0, 4}; !reflect.DeepEqual(order, want) {
		t.Errorf("order %v, want %v", order, want)
	}
	if !failed[2] || !failed[3] || failed[0] {
		t.Errorf("failed last time: %v", failed)
	}
	order, _ = History{}.Order(jobs, []int{0, 1, 2, 3, 4})
	if want := []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(order, want) {
		t.Errorf("no history: %v, want job order", order)
	}
}

// A run with fewer slots than jobs never runs more at once, starts the
// longest job and (in the fast slot) the shortest first, and records the
// run in history.log.
func TestSlotsAndTheFastSlot(t *testing.T) {
	dir := t.TempDir()
	labels := []string{"long", "mid", "short", "tiny"}
	history := "long=90:0 mid=50:0 short=20:0 tiny=10:0\n"
	if err := os.WriteFile(filepath.Join(dir, "history.log"), []byte(history), 0o644); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var started []string
	running, most := 0, 0
	release := make(chan struct{})
	var jobs []Job
	for _, label := range labels {
		label := label
		jobs = append(jobs, Job{Label: label, Exec: func(ctx context.Context, out io.Writer) Outcome {
			mu.Lock()
			started = append(started, label)
			running++
			most = max(most, running)
			mu.Unlock()
			_, _ = fmt.Fprintln(out, "ran", label)
			<-release
			mu.Lock()
			running--
			mu.Unlock()
			if label == "mid" {
				return Outcome{ExitCode: 3}
			}
			return Outcome{}
		}})
	}
	go func() {
		for range labels {
			time.Sleep(20 * time.Millisecond)
			release <- struct{}{}
		}
	}()
	results, _ := Run(context.Background(), jobs, Options{Slots: 2, LogDir: dir})
	if most > 2 {
		t.Errorf("%d jobs at once with 2 slots", most)
	}
	first := append([]string(nil), started[:2]...)
	sort.Strings(first)
	if got := first; !reflect.DeepEqual(got, []string{"long", "tiny"}) {
		t.Errorf("first started %v, want the fast slot's tiny and slot 1's long", got)
	}
	for i, r := range results {
		log, _ := os.ReadFile(r.Log)
		if string(log) != "ran "+labels[i]+"\n" {
			t.Errorf("%s's log: %q", r.Label, log)
		}
	}
	if results[1].Status != Failed || results[1].ExitCode != 3 || results[0].Status != Passed {
		t.Errorf("statuses: %+v", results)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "history.log"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[1], "mid=") || !strings.HasSuffix(strings.Fields(lines[1])[1], ":1") {
		t.Errorf("history.log:\n%s", data)
	}
	if ExitStatus(results) != 1 {
		t.Errorf("exit status %d", ExitStatus(results))
	}
}

func TestHistoryKeepsFiftyRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.log")
	for i := 0; i < 60; i++ {
		ReadHistory(path).Append(path, []Result{{Label: "a", Status: Passed, Duration: time.Duration(i) * 100 * time.Millisecond}})
	}
	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 50 || lines[0] != "a=10:0" || lines[49] != "a=59:0" {
		t.Errorf("%d lines, %q ... %q", len(lines), lines[0], lines[len(lines)-1])
	}
}

func TestLock(t *testing.T) {
	dir := t.TempDir()
	unlock, err := Lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Lock(dir); err == nil {
		t.Error("locked twice")
	}
	unlock()
	if _, err := Lock(dir); err != nil {
		t.Errorf("after unlocking: %v", err)
	}
	old := time.Now().Add(-3 * time.Minute)
	if err := os.Chtimes(filepath.Join(dir, ".lock"), old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := Lock(dir); err != nil {
		t.Errorf("a stale lock: %v", err)
	}
}

func TestExitStatus(t *testing.T) {
	missing := Result{Status: Failed, Components: []Component{{Failed: true, ToolchainMissing: true}}}
	broken := Result{Status: Failed, Components: []Component{{Failed: true}}}
	cases := []struct {
		results []Result
		want    int
	}{
		{[]Result{{Status: Passed}, {Status: StatusNotInstall}}, 0},
		{[]Result{{Status: Passed}, missing}, 3},
		{[]Result{missing, broken}, 1},
		{[]Result{{Status: Failed}}, 1},
	}
	for _, c := range cases {
		if got := ExitStatus(c.results); got != c.want {
			t.Errorf("%+v: %d, want %d", c.results, got, c.want)
		}
	}
}

func TestSelectFormat(t *testing.T) {
	cases := []struct {
		flag     string
		env      map[string]string
		terminal bool
		want     Format
		err      bool
	}{
		{"", nil, true, Human, false},
		{"", nil, false, Human, false},
		{"", map[string]string{"CLAUDECODE": "1"}, false, Agent, false},
		{"", map[string]string{"CLAUDECODE": "1"}, true, Human, false},
		{"", map[string]string{"GITHUB_ACTIONS": "true"}, false, CI, false},
		{"", map[string]string{"CLAUDECODE": "1", "GITHUB_ACTIONS": "true"}, false, Agent, false},
		{"", map[string]string{"HECATRON_OUTPUT": "json", "CLAUDECODE": "1"}, false, JSON, false},
		{"human", map[string]string{"HECATRON_OUTPUT": "json", "CLAUDECODE": "1"}, false, Human, false},
		{"agent", nil, true, Agent, false},
		{"", map[string]string{"HECATRON_OUTPUT": "xml"}, true, "", true},
		{"yaml", nil, true, "", true},
	}
	for _, c := range cases {
		got, err := SelectFormat(c.flag, func(name string) string { return c.env[name] }, c.terminal)
		if got != c.want || (err != nil) != c.err {
			t.Errorf("flag %q env %v terminal %v: %q, %v; want %q", c.flag, c.env, c.terminal, got, err, c.want)
		}
	}
}

func TestFallbackExcerpt(t *testing.T) {
	var log strings.Builder
	log.WriteString("=== test a (zig) ===\n")
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&log, "step %d\n", i)
	}
	log.WriteString("Downloading foo 1.2MiB\n  45% [=========>          ] 3.4 MB/7.6 MB\r 100% [====================] 7.6 MB/7.6 MB\n")
	log.WriteString("\x1b[31mbroken thing\x1b[0m\nrepeated\nrepeated\nrepeated\n\nzig: test failed: a\n")
	excerpt := Extract([]byte(log.String()), nil)
	var got []string
	for _, line := range excerpt.Lines {
		got = append(got, line.Text)
		if line.Component != "a" {
			t.Errorf("%q from component %q", line.Text, line.Component)
		}
	}
	want := []string{"step 8", "step 9", "step 10", "step 11", "step 12", "step 13", "step 14", "step 15",
		"step 16", "step 17", "step 18", "step 19", "step 20", "broken thing", "repeated"}
	if excerpt.Source != "tail" || !reflect.DeepEqual(got, want) {
		t.Errorf("%s:\n%q\nwant\n%q", excerpt.Source, got, want)
	}
	if e := Extract(nil, nil); e.Source != "" || len(e.Lines) != 0 {
		t.Errorf("an empty log: %+v", e)
	}
}

func TestDefaultPatternsAndVersions(t *testing.T) {
	log := "building\nsrc/x.zig:3:5: error: expected ';'\nnote: here\n"
	e := Extract([]byte(log), nil)
	if e.Source != "default" || len(e.Lines) != 1 || e.Lines[0].File != "src/x.zig" || e.Lines[0].Line != 3 ||
		e.Lines[0].Column != 5 || e.Lines[0].Message != "expected ';'" {
		t.Errorf("%+v", e)
	}
	list, _ := ParsePatterns(nil, nil)
	if len(list.List) != 0 {
		t.Error("patterns from nothing")
	}
	old := &Patterns{
		List: []Pattern{
			{Tool: "tool", Versions: "<2", Regex: DefaultPatterns.List[2].Regex},
			{Tool: "tool", Versions: ">=2", Regex: DefaultPatterns.List[1].Regex},
		},
		ToolVersions: map[string]*regexp.Regexp{"tool": regexp.MustCompile(`^tool v(\S+)$`)},
	}
	if e := Extract([]byte("tool v2.1.0\nFAIL: x\nerror: y\n"), old); len(e.Lines) != 1 || e.Lines[0].Text != "error: y" {
		t.Errorf("tool 2.1 used %+v", e.Lines)
	}
	if e := Extract([]byte("tool v1.9\nFAIL: x\nerror: y\n"), old); len(e.Lines) != 1 || e.Lines[0].Text != "FAIL: x" {
		t.Errorf("tool 1.9 used %+v", e.Lines)
	}
	for version, constraint := range map[string]string{"1.2.3": ">=1.2,<1.3", "7": "7", "7.1": "=7", "v10.0": ">9.9"} {
		if !satisfies(version, constraint) {
			t.Errorf("%s doesn't satisfy %s", version, constraint)
		}
	}
	for version, constraint := range map[string]string{"1.3": ">=1.2,<1.3", "8": "7", "6.9": ">=7", "1": "nonsense"} {
		if satisfies(version, constraint) {
			t.Errorf("%s satisfies %s", version, constraint)
		}
	}
}

// The renderers' output for a fixed run, against golden files
// (testdata/render.*; go test -update rewrites them).
func TestRenderers(t *testing.T) {
	report := &Report{
		Action: "test", Root: "/repo", Cwd: "/repo", LogDir: "/repo/.logs/test", Wall: 271*time.Second + 300*time.Millisecond,
		Results: []Result{
			{Label: "go", Lang: "go", Status: Failed, ExitCode: 1, Duration: 12 * time.Second, Log: "/repo/.logs/test/go.log",
				Components: []Component{{Name: ".", Dir: "/repo/go", Failed: true}},
				Excerpt: Excerpt{Source: "patterns", Lines: []ErrorLine{
					{Text: "--- FAIL: TestAdd (0.00s)", Component: "."},
					{Text: "calc_test.go:7: Add(2, 3) = -1, want 5", Component: ".", File: "calc_test.go", Line: 7, Message: "Add(2, 3) = -1, want 5"},
				}}},
			{Label: "python", Lang: "python", Status: Passed, Duration: 2500 * time.Millisecond, Log: "/repo/.logs/test/python.log",
				Components: []Component{{Name: ".", Dir: "/repo/python"}}},
			{Label: "rust", Lang: "rust", Status: StatusNotInstall},
			{Label: "zig", Lang: "zig", Status: StatusDisabled},
			{Label: "php", Lang: "php", Status: Failed, ExitCode: 1, Duration: 61 * time.Second, Log: "/repo/.logs/test/php.log",
				Components: []Component{{Name: ".", Dir: "/repo/php", Failed: true, ToolchainMissing: true}},
				Excerpt:    Excerpt{Source: "tail", Lines: []ErrorLine{{Text: "bash: line 1: php: command not found", Component: "."}}}},
			{Label: "kotlin", Lang: "kotlin", Status: TimedOut, ExitCode: 1, Duration: 600 * time.Second, Log: "/repo/.logs/test/kotlin.log",
				Limit:      Limit{Duration: 10 * time.Minute, Source: LimitHistory, Median: 70 * time.Second, Runs: 5},
				Components: []Component{{Name: ".", Dir: "/repo/kotlin", Failed: true}},
				Excerpt:    Excerpt{Source: "tail", Lines: []ErrorLine{{Text: "> Task :test", Component: "."}}}},
			{Label: "python2", Lang: "python", Status: Passed, Duration: 2 * time.Second, Log: "/repo/.logs/test/python2.log",
				Limit: Limit{Duration: 90 * time.Second, Source: LimitFlag}},
			{Label: "---", Status: StatusSeparator},
			{Label: "docs", Task: true, Status: Passed, Duration: 300 * time.Millisecond, Log: "/repo/.logs/test/docs.log"},
		},
	}
	var agent, jsonOut, chart, annotations bytes.Buffer
	WriteAgent(&agent, report)
	if err := WriteJSON(&jsonOut, report); err != nil {
		t.Fatal(err)
	}
	WriteChart(&chart, report, true, 60)
	WriteAnnotations(&annotations, report, "")
	golden(t, "render.agent", agent.String())
	golden(t, "render.json", jsonOut.String())
	golden(t, "render.chart", chart.String())
	golden(t, "render.annotations", annotations.String())

	passing := &Report{Action: "lint", Wall: 3 * time.Second, Results: []Result{{Label: "go", Status: Passed}, {Label: "c", Status: StatusNotInstall}}}
	agent.Reset()
	WriteAgent(&agent, passing)
	if agent.String() != "hct lint: 1 passed, 1 skipped (3.0s)\n" {
		t.Errorf("a passing run: %q", agent.String())
	}
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("%s:\n%s\nwant:\n%s", name, got, want)
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{400 * time.Millisecond: "0.4s", 12 * time.Second: "12s",
		271 * time.Second: "4m31s", 3725 * time.Second: "1h02m"}
	for d, want := range cases {
		if got := FormatDuration(d); got != want {
			t.Errorf("%v: %q, want %q", d, got, want)
		}
	}
}
