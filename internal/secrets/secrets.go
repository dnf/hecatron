// Package secrets is hct audit's secret scan (docs/audit.md, "Secrets"):
// gitleaks, pinned and checksummed, run on the working tree (or the git
// history) with what git ignores and the project's ignored paths left out.
// A secret never leaves gitleaks: it runs with --redact, and only a
// finding's rule, file, line, commit and fingerprint are read from its
// report.
package secrets

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dnf/hecatron/internal/install"
)

// Version is the gitleaks hecatron installs.
const Version = "8.30.1"

// releases are gitleaks' release archives' SHA-256s, from its
// gitleaks_<version>_checksums.txt.
var releases = map[string]struct{ platform, sha256 string }{
	"linux/amd64":  {"linux_x64", "551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb"},
	"linux/arm64":  {"linux_arm64", "e4a487ee7ccd7d3a7f7ec08657610aa3606637dab924210b3aee62570fb4b080"},
	"darwin/amd64": {"darwin_x64", "dfe101a4db2255fc85120ac7f3d25e4342c3c20cf749f2c20a18081af1952709"},
	"darwin/arm64": {"darwin_arm64", "b40ab0ae55c505963e365f271a8d3846efbc170aa17f2607f13df610a9aeb6a5"},
}

// Release is gitleaks' release for a platform (Go's GOOS and GOARCH);
// false when there's none.
func Release(goos, goarch string) (install.ReleaseBinary, bool) {
	r, ok := releases[goos+"/"+goarch]
	if !ok {
		return install.ReleaseBinary{}, false
	}
	return install.ReleaseBinary{Name: "gitleaks", SHA256: r.sha256, Format: "tar.gz", Member: "gitleaks",
		URL: fmt.Sprintf("https://github.com/gitleaks/gitleaks/releases/download/v%s/gitleaks_%s_%s.tar.gz", Version, Version, r.platform)}, true
}

// InstalledVersion is what `gitleaks version` says, "" when it can't run.
func InstalledVersion(program string) string {
	out, err := exec.Command(program, "version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "v")
}

// Options are one scan's.
type Options struct {
	Program string   // gitleaks
	Root    string   // the project root, scanned
	History bool     // scan the git history (gitleaks git) instead of the working tree
	Ignore  []string // audit.secrets.ignore: paths relative to Root
	Env     []string // gitleaks' and git's environment
}

// Finding is one possible secret. It holds nothing of the secret itself.
type Finding struct {
	Rule        string `json:"rule"`
	Description string `json:"description"`
	File        string `json:"file"` // relative to Root, slash-separated
	Line        int    `json:"line"`
	Commit      string `json:"commit,omitempty"`
	// Fingerprint is gitleaks': [commit:]file:rule:line, what
	// .gitleaksignore lists to accept a finding.
	Fingerprint string `json:"fingerprint"`
}

// Scan runs gitleaks and returns its findings, in its order.
func Scan(ctx context.Context, o Options) ([]Finding, error) {
	work, err := os.MkdirTemp("", "hecatron-secrets-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(work) }()
	// Paths in gitleaks' report, and what its allowlist matches, are
	// relative to the directory it scans (the root) or, for history, to the
	// repository's top.
	prefix := ""
	if o.History {
		prefix = gitOutput(o.Root, o.Env, "rev-parse", "--show-prefix")
	}
	var skip []string
	for _, pattern := range o.Ignore {
		skip = append(skip, GlobRegexp(prefix, pattern))
	}
	if !o.History {
		// The working tree's ignored files aren't committed, so they're
		// left out; the history is what was committed, ignored or not.
		skip = append(skip, `(^|/)\.git/`)
		skip = append(skip, gitIgnored(o.Root, o.Env)...)
	}
	config := filepath.Join(work, "gitleaks.toml")
	if err := os.WriteFile(config, []byte(Config(o.Root, skip)), 0o644); err != nil {
		return nil, err
	}
	report := filepath.Join(work, "report.json")
	mode := "dir"
	if o.History {
		mode = "git"
	}
	cmd := exec.CommandContext(ctx, o.Program, mode, ".", "--config", config, "--redact", "--exit-code", "0",
		"--no-banner", "--no-color", "--log-level", "error", "--gitleaks-ignore-path", o.Root,
		"--report-format", "json", "--report-path", report)
	var stderr bytes.Buffer
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = o.Root, o.Env, io.Discard, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gitleaks failed (%v): %s", err, lastLines(stderr.String(), 10))
	}
	data, err := os.ReadFile(report)
	if err != nil {
		return nil, err
	}
	findings, err := Parse(data)
	if err != nil {
		return nil, err
	}
	matchers := ignoreMatchers(o.Ignore)
	kept := findings[:0]
	for _, f := range findings {
		f.File = strings.TrimPrefix(filepath.ToSlash(f.File), prefix)
		if !ignored(matchers, f.File) {
			kept = append(kept, f)
		}
	}
	return kept, nil
}

// Parse reads gitleaks' JSON report. Only the fields Finding holds are
// decoded: Secret, Match and the rest never are.
func Parse(data []byte) ([]Finding, error) {
	var entries []struct {
		RuleID      string
		Description string
		File        string
		StartLine   int
		Commit      string
		Fingerprint string
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("couldn't read gitleaks' report: %w", err)
	}
	findings := make([]Finding, 0, len(entries))
	for _, e := range entries {
		findings = append(findings, Finding{Rule: e.RuleID, Description: e.Description, File: e.File, Line: e.StartLine,
			Commit: e.Commit, Fingerprint: e.Fingerprint})
	}
	return findings, nil
}

// Config is the gitleaks configuration a scan runs with: the project's
// .gitleaks.toml (which gitleaks would read by itself, but a --config
// replaces), else gitleaks' default rules, and an allowlist of the paths
// to skip (regular expressions).
func Config(root string, skip []string) string {
	var b strings.Builder
	b.WriteString("# hecatron's configuration for one gitleaks scan (hct audit)\n[extend]\n")
	if own := filepath.Join(root, ".gitleaks.toml"); isFile(own) {
		fmt.Fprintf(&b, "path = %s\n", tomlString(own))
	} else {
		b.WriteString("useDefault = true\n")
	}
	if len(skip) > 0 {
		b.WriteString("\n[[allowlists]]\ndescription = \"hct audit: what git ignores, and audit.secrets.ignore\"\npaths = [\n")
		for _, s := range skip {
			fmt.Fprintf(&b, "  %s,\n", tomlString(s))
		}
		b.WriteString("]\n")
	}
	return b.String()
}

// GlobRegexp is an audit.secrets.ignore pattern as a regular expression
// matching slash-separated paths below prefix: * and ? within a name, **
// across directories, and a trailing / for everything below a directory.
func GlobRegexp(prefix, pattern string) string {
	pattern = strings.TrimPrefix(strings.TrimPrefix(filepath.ToSlash(pattern), "./"), "/")
	directory := strings.HasSuffix(pattern, "/")
	pattern = strings.TrimSuffix(pattern, "/")
	var b strings.Builder
	b.WriteString("^" + regexp.QuoteMeta(prefix))
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; {
		case strings.HasPrefix(pattern[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(pattern[i:], "**"):
			b.WriteString(".*")
			i++
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	if directory {
		b.WriteString("/")
	} else {
		b.WriteString("(?:/|$)")
	}
	return b.String()
}

func ignoreMatchers(patterns []string) []*regexp.Regexp {
	var matchers []*regexp.Regexp
	for _, pattern := range patterns {
		if re, err := regexp.Compile(GlobRegexp("", pattern)); err == nil {
			matchers = append(matchers, re)
		}
	}
	return matchers
}

func ignored(matchers []*regexp.Regexp, path string) bool {
	for _, re := range matchers {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

// gitIgnored are what git ignores in the working tree (a directory once),
// as regular expressions; none outside a repository.
func gitIgnored(root string, env []string) []string {
	out := gitOutput(root, env, "ls-files", "-z", "--others", "--ignored", "--exclude-standard", "--directory")
	var skip []string
	for _, path := range strings.Split(out, "\x00") {
		switch {
		case path == "":
		case strings.HasSuffix(path, "/"):
			skip = append(skip, "^"+regexp.QuoteMeta(path))
		default:
			skip = append(skip, "^"+regexp.QuoteMeta(path)+"$")
		}
	}
	return skip
}

func gitOutput(dir string, env []string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env = dir, env
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	if len(args) > 0 && args[0] == "rev-parse" {
		return strings.TrimSpace(string(out))
	}
	return string(out)
}

// tomlString is s as a TOML basic string.
func tomlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteRune('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, "\\u%04X", r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func lastLines(text string, n int) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// Write prints the findings and a summary line; where is "the working
// tree" or "the git history".
func Write(w io.Writer, findings []Finding, history bool) {
	where := "the working tree"
	if history {
		where = "the git history"
	}
	width := 0
	for _, f := range findings {
		width = max(width, len(f.Rule))
	}
	for _, f := range findings {
		location := fmt.Sprintf("%s:%d", f.File, f.Line)
		if f.Commit != "" {
			location += " (commit " + f.Commit[:min(12, len(f.Commit))] + ")"
		}
		_, _ = fmt.Fprintf(w, "  %-*s  %s  fingerprint %s\n", width, f.Rule, location, f.Fingerprint)
	}
	switch len(findings) {
	case 0:
		_, _ = fmt.Fprintf(w, "secrets: none found in %s (gitleaks)\n", where)
	case 1:
		_, _ = fmt.Fprintf(w, "secrets: 1 possible secret in %s (gitleaks)\n", where)
	default:
		_, _ = fmt.Fprintf(w, "secrets: %d possible secrets in %s (gitleaks)\n", len(findings), where)
	}
	if len(findings) > 0 {
		_, _ = fmt.Fprintln(w, "Remove and rotate a real secret. For a false positive, add its fingerprint to .gitleaksignore,")
		_, _ = fmt.Fprintln(w, `or its path to "audit": {"secrets": {"ignore": [...]}} in .hecatron/config.json (docs/audit.md).`)
	}
}

// ErrNotInstalled is Scan's caller's when gitleaks isn't on PATH.
var ErrNotInstalled = errors.New("gitleaks isn't installed; hct install secrets installs it")
