package runner

import (
	"fmt"
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

// mullvadvpn-app's xcodebuild: a script phase's cargo isn't there, and
// make fails, among warnings and notes about every other script phase.
// The excerpt is the errors, not the log's last lines.
func TestXcodebuildErrorsLeadTheExcerpt(t *testing.T) {
	log := strings.Join([]string{
		"=== build ios (swift) ===",
		"+ xcodebuild build -project MullvadVPN.xcodeproj -scheme MullvadVPN",
		"note: Run script build phase 'Run SwiftLint' will be run during every build (in target 'MullvadVPN' from project 'MullvadVPN')",
		"warning: Run script build phase 'Disable NSLock usage' will be run during every build because it does not specify any outputs.",
		"/Users/runner/app/ios/MullvadVPN.xcodeproj: warning: tasks in 'Copy Headers' are delayed by unsandboxed script phases",
		"/bin/sh: /Users/runner/.cargo/bin/cargo: No such file or directory",
		"make: *** [/Users/runner/app/ios/build/lib/libwg-go-arm64.a] Error 2",
		"ExternalBuildToolExecution WireGuardGoBridge (in target 'WireGuardGoBridge' from project 'MullvadVPN')",
		"/Users/runner/app/ios/MullvadVPN.xcodeproj: error: Signing for \"PacketTunnel\" requires a development team.",
		"** BUILD FAILED **",
		"",
		"The following build commands failed:",
		"\tExternalBuildToolExecution WireGuardGoBridge (in target 'WireGuardGoBridge' from project 'MullvadVPN')",
		"\tBuilding project MullvadVPN with scheme MullvadVPN",
		"(2 failures)",
	}, "\n")
	excerpt := Extract([]byte(log), nil)
	var texts []string
	for _, line := range excerpt.Lines {
		texts = append(texts, line.Text)
	}
	want := []string{
		"/bin/sh: /Users/runner/.cargo/bin/cargo: No such file or directory",
		"make: *** [/Users/runner/app/ios/build/lib/libwg-go-arm64.a] Error 2",
		"/Users/runner/app/ios/MullvadVPN.xcodeproj: error: Signing for \"PacketTunnel\" requires a development team.",
		"** BUILD FAILED **",
		"ExternalBuildToolExecution WireGuardGoBridge (in target 'WireGuardGoBridge' from project 'MullvadVPN')",
	}
	if excerpt.Source != "default" || strings.Join(texts, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %s:\n%s", excerpt.Source, strings.Join(texts, "\n"))
	}
}

// NetNewsWire's xcodebuild: the error ends in a colon and the reason is
// the indented line under it.
func TestAnErrorEndingInAColonTakesTheLineUnderIt(t *testing.T) {
	log := strings.Join([]string{
		"=== build . (swift) ===",
		"xcodebuild: error: Could not resolve package dependencies:",
		"  failed downloading 'https://github.com/a/B/releases/download/2.9.5/B.zip' which is required by binary target 'B': badResponseStatusCode(504)",
		"  fatalError",
		"",
		"error: go: not formatted:",
		"",
		"  a.go",
	}, "\n")
	excerpt := Extract([]byte(log), nil)
	want := []string{
		"xcodebuild: error: Could not resolve package dependencies: failed downloading 'https://github.com/a/B/releases/download/2.9.5/B.zip' which is required by binary target 'B': badResponseStatusCode(504)",
		"error: go: not formatted:",
	}
	if len(excerpt.Lines) != 2 || excerpt.Lines[0].Text != want[0] || excerpt.Lines[1].Text != want[1] {
		t.Errorf("got %+v", excerpt.Lines)
	}
	if !strings.HasSuffix(excerpt.Lines[0].Message, "badResponseStatusCode(504)") {
		t.Errorf("message: %q", excerpt.Lines[0].Message)
	}
}

// A language's patterns that match warnings too: the errors come first,
// even after more warnings than an excerpt holds.
func TestErrorsComeBeforeWarnings(t *testing.T) {
	lines := []string{"=== build . (kotlin) ==="}
	for i := 0; i < ExcerptLines+2; i++ {
		lines = append(lines, fmt.Sprintf("w: file:///a/B.kt:%d:1 'x' is deprecated. warning", i+1))
	}
	lines = append(lines, "e: file:///a/C.kt:3:5 Unresolved reference 'y'.")
	language := &Patterns{List: []Pattern{{Regex: regexp.MustCompile(`^[we]: (?P<message>.+)$`)}}}
	excerpt := Extract([]byte(strings.Join(lines, "\n")), language)
	if len(excerpt.Lines) != ExcerptLines || !strings.Contains(excerpt.Lines[0].Text, "Unresolved reference") {
		t.Errorf("got %+v", excerpt.Lines)
	}
}
