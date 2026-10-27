package runner

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dnf/hecatron/internal/project"
)

// A failed job's excerpt is the lines of its log that say what went wrong:
// those matching its language's error patterns (config.json
// "errorPatterns"), else the generic ones below, else the last lines of the
// log with colour codes, progress and download noise taken out
// (docs/design.md §19, "When a tool's output changes"). An excerpt never
// fails a run: a pattern that doesn't compile is skipped.

// ExcerptLines and TailLines size an excerpt.
const (
	ExcerptLines = 10
	TailLines    = 15
	maxLineWidth = 240
)

// ErrorLine is one line of an excerpt.
type ErrorLine struct {
	Text      string `json:"text"`
	Component string `json:"component,omitempty"`
	File      string `json:"file,omitempty"`
	Line      int    `json:"line,omitempty"`
	Column    int    `json:"column,omitempty"`
	Message   string `json:"message,omitempty"`
}

// Excerpt is what a failed job's log says went wrong.
type Excerpt struct {
	Lines  []ErrorLine
	Source string // "patterns", "default" (the generic patterns), "tail", or "" (an empty log)
}

// Pattern is one error pattern: a regular expression (RE2) whose named
// groups file, line, col and message locate the error, tried for the
// tool's versions in Versions (">=7", "<2.0", ">=1.2,<3"; any when empty).
// Next, when set, finds the error's location on one of the three lines
// that follow (rustc's "--> src/lib.rs:2:25"), which is then added to the
// line.
type Pattern struct {
	Tool     string
	Versions string
	Regex    *regexp.Regexp
	Next     *regexp.Regexp
}

// Patterns are a language's error patterns, in the order they're tried,
// and how to find each tool's version in a log ("errorToolVersions": a
// regular expression whose first group is the version).
type Patterns struct {
	List         []Pattern
	ToolVersions map[string]*regexp.Regexp
}

// ParsePatterns reads a definition's "errorPatterns" and
// "errorToolVersions". What it can't read is left out, with why.
func ParsePatterns(patterns, toolVersions any) (*Patterns, []string) {
	result := &Patterns{ToolVersions: map[string]*regexp.Regexp{}}
	var problems []string
	list, _ := patterns.([]any)
	for n, item := range list {
		object, ok := item.(*project.Object)
		if !ok {
			problems = append(problems, fmt.Sprintf("errorPatterns[%d] is not an object", n))
			continue
		}
		expression, _ := stringField(object, "regex")
		re, err := regexp.Compile(expression)
		if expression == "" || err != nil {
			problems = append(problems, fmt.Sprintf("errorPatterns[%d]: bad regex %q", n, expression))
			continue
		}
		tool, _ := stringField(object, "tool")
		versions, _ := stringField(object, "versions")
		if _, ok := parseConstraint(versions); !ok {
			problems = append(problems, fmt.Sprintf("errorPatterns[%d]: bad versions %q", n, versions))
			continue
		}
		var next *regexp.Regexp
		if expression, ok := stringField(object, "next"); ok {
			if next, err = regexp.Compile(expression); err != nil {
				problems = append(problems, fmt.Sprintf("errorPatterns[%d]: bad next %q", n, expression))
				continue
			}
		}
		result.List = append(result.List, Pattern{Tool: tool, Versions: versions, Regex: re, Next: next})
	}
	if object, ok := toolVersions.(*project.Object); ok {
		for _, tool := range object.Keys {
			expression, _ := object.Values[tool].(string)
			re, err := regexp.Compile(expression)
			if err != nil || re.NumSubexp() < 1 {
				problems = append(problems, fmt.Sprintf("errorToolVersions.%s: bad regex %q", tool, expression))
				continue
			}
			result.ToolVersions[tool] = re
		}
	}
	return result, problems
}

func stringField(object *project.Object, key string) (string, bool) {
	value, ok := object.Get(key)
	s, isString := value.(string)
	return s, ok && isString
}

// DefaultPatterns are tried for a job whose language's patterns match
// nothing, and for project tasks: a compiler's file:line[:col]: error, a
// line starting with error or fatal, and test runners' FAIL lines.
var DefaultPatterns = &Patterns{List: []Pattern{
	{Regex: regexp.MustCompile(`^\s*(?P<file>[^\s:]+):(?P<line>\d+)(?::(?P<col>\d+))?:?\s+(?:fatal )?[Ee]rror\b:?\s*(?P<message>.*)$`)},
	{Regex: regexp.MustCompile(`^\s*(?:[Ee]rror|ERROR|[Ff]atal|FATAL)(?:\[[^\]]*\])?:\s+(?P<message>.+)$`)},
	{Regex: regexp.MustCompile(`^\s*(?:FAIL|FAILED|FAILURE)\b[: ]\s*(?P<message>.+)$`)},
}}

// MissingCommandPatterns are a shell's "command not found" (bash, sh, zsh),
// reported before any language's own patterns.
var MissingCommandPatterns = &Patterns{List: []Pattern{
	{Regex: regexp.MustCompile(`^.*?(?:line \d+: )?(?P<message>[^\s:]+: (?:command )?not found)$`)},
}}

var (
	ansi            = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[@-Z\\-_]`)
	componentHeader = regexp.MustCompile(`^=== \S+ (.+) \([^()]+\) ===$`)
	componentResult = regexp.MustCompile(`^\S+: \S+ (?:failed: .*|passed for \d+ component\(s\))$|^hecatron: \S+ timed out: stopped at its `)
	// Lines that say nothing about a failure: downloads, progress, and
	// package managers' fetching and compiling dependencies.
	noise = regexp.MustCompile(`(?i)^\s*(?:` +
		`download(?:ing|ed)?\b|fetch(?:ing|ed)\b|resolv(?:ing|ed) (?:deltas|dependencies|packages)|receiving objects|` +
		`unpacking|extracting|installing\b|installed\b|collecting\b|using cached|requirement already satisfied|` +
		`compiling \S+ v\d|checking \S+ v\d|updating (?:crates\.io|index)|locking \d+ packages?|adding \S+ v\d|` +
		`progress\b|\[?[=#>\-. ]{10,}\]?\s*\d|[\d.]+\s*[kmg]i?b\s*/\s*[\d.]+\s*[kmg]i?b|\d{1,3}%(?:\s|$)|` +
		`npm (?:warn|notice)|added \d+ packages|audited \d+ packages|found \d+ vulnerabilit|run .npm fund|` +
		`\d+ packages? (?:are|is) looking for funding|warning: unused manifest key` +
		`)`)
)

// cleanLine is a log line as a person would read it: colour codes gone,
// only what a carriage return left last on the terminal.
func cleanLine(line string) string {
	line = strings.TrimRight(line, "\r")
	if i := strings.LastIndexByte(line, '\r'); i >= 0 {
		line = line[i+1:]
	}
	line = ansi.ReplaceAllString(line, "")
	for strings.Contains(line, "\b") {
		i := strings.IndexByte(line, '\b')
		if i > 0 {
			line = line[:i-1] + line[i+1:]
		} else {
			line = line[1:]
		}
	}
	line = strings.TrimRight(line, " \t")
	if !utf8.ValidString(line) {
		line = strings.ToValidUTF8(line, "?")
	}
	return line
}

func truncate(line string) string {
	if utf8.RuneCountInString(line) <= maxLineWidth {
		return line
	}
	runes := []rune(line)
	return string(runes[:maxLineWidth-1]) + "…"
}

// Extract finds what went wrong in a log: lines matching patterns (nil for
// none), else the default patterns, else the log's last lines.
func Extract(log []byte, patterns *Patterns) Excerpt {
	var lines, components []string
	component := ""
	for _, raw := range strings.Split(string(log), "\n") {
		line := cleanLine(raw)
		if m := componentHeader.FindStringSubmatch(line); m != nil {
			component = m[1]
			continue
		}
		if componentResult.MatchString(line) {
			continue
		}
		lines = append(lines, line)
		components = append(components, component)
	}
	// A command the shell couldn't find (a linter not installed) comes first
	// whatever else matched: it's why the rest failed, and what to fix.
	missing := match(lines, components, MissingCommandPatterns)
	if patterns != nil {
		if found := match(lines, components, patterns); len(found) > 0 {
			return Excerpt{Lines: limitLines(append(missing, found...)), Source: "patterns"}
		}
	}
	if len(missing) > 0 {
		found := match(lines, components, DefaultPatterns)
		return Excerpt{Lines: limitLines(append(missing, found...)), Source: "default"}
	}
	if found := match(lines, components, DefaultPatterns); len(found) > 0 {
		return Excerpt{Lines: found, Source: "default"}
	}
	return tail(lines, components)
}

func match(lines, components []string, patterns *Patterns) []ErrorLine {
	versions := map[string]string{}
	for tool, re := range patterns.ToolVersions {
		for _, line := range lines {
			if m := re.FindStringSubmatch(line); m != nil {
				versions[tool] = m[1]
				break
			}
		}
	}
	var active []Pattern
	for _, p := range patterns.List {
		if version, known := versions[p.Tool]; !known || satisfies(version, p.Versions) {
			active = append(active, p)
		}
	}
	var found []ErrorLine
	seen := map[string]bool{}
	for i, line := range lines {
		for _, p := range active {
			m := p.Regex.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			e := located(strings.TrimSpace(line), components[i], p.Regex, m)
			if p.Next != nil {
				for j := i + 1; j < len(lines) && j <= i+3; j++ {
					if n := p.Next.FindStringSubmatch(lines[j]); n != nil {
						at := located(strings.TrimSpace(lines[j]), "", p.Next, n)
						e.Text += " " + at.Text
						if e.File == "" {
							e.File, e.Line, e.Column = at.File, at.Line, at.Column
						}
						break
					}
				}
			}
			e.Text = truncate(e.Text)
			if !seen[e.Text] {
				seen[e.Text] = true
				found = append(found, e)
			}
			break
		}
		if len(found) == ExcerptLines {
			break
		}
	}
	return found
}

func located(text, component string, re *regexp.Regexp, m []string) ErrorLine {
	e := ErrorLine{Text: text, Component: component}
	for i, name := range re.SubexpNames() {
		if i >= len(m) || m[i] == "" {
			continue
		}
		switch name {
		case "file":
			e.File = m[i]
		case "line":
			e.Line, _ = strconv.Atoi(m[i])
		case "col":
			e.Column, _ = strconv.Atoi(m[i])
		case "message":
			e.Message = strings.TrimSpace(m[i])
		}
	}
	if e.Line == 0 {
		e.File = ""
	}
	return e
}

func tail(lines, components []string) Excerpt {
	var kept []ErrorLine
	for i, line := range lines {
		if strings.TrimSpace(line) == "" || noise.MatchString(line) {
			continue
		}
		text := truncate(line)
		if n := len(kept); n > 0 && kept[n-1].Text == text {
			continue
		}
		kept = append(kept, ErrorLine{Text: text, Component: components[i]})
	}
	if len(kept) > TailLines {
		kept = kept[len(kept)-TailLines:]
	}
	if len(kept) == 0 {
		return Excerpt{}
	}
	return Excerpt{Lines: kept, Source: "tail"}
}

// satisfies reports whether a version meets a constraint (">=7",
// "<2.0", ">=1.2,<3", "=1.4"); an empty constraint is any version.
func satisfies(version, constraint string) bool {
	conditions, ok := parseConstraint(constraint)
	if !ok {
		return false
	}
	have := versionNumbers(version)
	for _, c := range conditions {
		cmp := compareVersions(have, c.version)
		var holds bool
		switch c.op {
		case ">=":
			holds = cmp >= 0
		case ">":
			holds = cmp > 0
		case "<=":
			holds = cmp <= 0
		case "<":
			holds = cmp < 0
		case "=":
			holds = prefixEqual(have, c.version)
		}
		if !holds {
			return false
		}
	}
	return true
}

type condition struct {
	op      string
	version []int
}

func parseConstraint(constraint string) ([]condition, bool) {
	var conditions []condition
	for _, part := range strings.Split(constraint, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		op := "="
		for _, candidate := range []string{">=", "<=", ">", "<", "="} {
			if strings.HasPrefix(part, candidate) {
				op = candidate
				break
			}
		}
		numbers := versionNumbers(strings.TrimSpace(strings.TrimPrefix(part, op)))
		if numbers == nil {
			return nil, false
		}
		conditions = append(conditions, condition{op, numbers})
	}
	return conditions, true
}

// versionNumbers are a version's leading dot-separated numbers ("v1.22.3"
// is 1 22 3, "8.3.4rc1" 8 3 4).
func versionNumbers(version string) []int {
	var numbers []int
	for _, part := range strings.Split(strings.TrimPrefix(version, "v"), ".") {
		digits := part
		if i := strings.IndexFunc(part, func(r rune) bool { return r < '0' || r > '9' }); i >= 0 {
			digits = part[:i]
		}
		n, err := strconv.Atoi(digits)
		if err != nil {
			break
		}
		numbers = append(numbers, n)
		if len(digits) < len(part) {
			break
		}
	}
	return numbers
}

func compareVersions(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		x, y := 0, 0
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func prefixEqual(have, want []int) bool {
	if len(have) < len(want) {
		return false
	}
	for i := range want {
		if have[i] != want[i] {
			return false
		}
	}
	return true
}

// limitLines is lines without repeats, at most ExcerptLines of them.
func limitLines(lines []ErrorLine) []ErrorLine {
	var kept []ErrorLine
	seen := map[string]bool{}
	for _, line := range lines {
		if !seen[line.Text] && len(kept) < ExcerptLines {
			seen[line.Text] = true
			kept = append(kept, line)
		}
	}
	return kept
}
