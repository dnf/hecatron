package runner

import (
	"regexp"
	"strings"
	"testing"
)

// A command the shell couldn't find comes first in the excerpt, before what
// the language's own patterns match: it's why the rest failed.
func TestMissingCommandLeadsTheExcerpt(t *testing.T) {
	log := strings.Join([]string{
		"=== lint . (go) ===",
		"+ golangci-lint run ./...",
		"/h/languages/_hooks.sh: line 44: golangci-lint: command not found",
		"+ gofmt -l ./a.go",
		"error: go: not formatted (hecatron format go formats them):",
		"  a.go",
	}, "\n")
	language := &Patterns{List: []Pattern{{Regex: regexp.MustCompile(`^error: (?P<message>.+)$`)}}}
	for name, patterns := range map[string]*Patterns{"the language's patterns": language, "no patterns": nil} {
		excerpt := Extract([]byte(log), patterns)
		if len(excerpt.Lines) < 2 || !strings.Contains(excerpt.Lines[0].Text, "golangci-lint: command not found") {
			t.Errorf("%s: %+v", name, excerpt.Lines)
		}
	}
	if got := Extract([]byte("=== test . (go) ===\nsh: 1: protoc: not found\n"), nil); len(got.Lines) != 1 {
		t.Errorf("sh's form: %+v", got.Lines)
	}
}
