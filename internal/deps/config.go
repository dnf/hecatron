package deps

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"
)

// ProjectConfig is the project's .hecatron/config.json as audit.py's
// _project_config reads it: from the nearest directory at or above start
// holding one, decoded but not validated; nil when there is none or it
// isn't JSON (outside a project there's nothing to apply).
func ProjectConfig(start string) any {
	dir, err := filepath.Abs(start)
	if err != nil {
		return nil
	}
	for {
		path := filepath.Join(dir, ".hecatron", "config.json")
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return LoadJSONFile(path)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}

// LoadJSONFile is a JSON file as Python's json.load reads it (strict UTF-8),
// nil when it can't be read.
func LoadJSONFile(path string) any {
	data, err := os.ReadFile(path)
	if err != nil || !utf8.Valid(data) {
		return nil
	}
	value, err := jsonLoads(string(data))
	if err != nil {
		return nil
	}
	return value
}

// ReleaseLineParts is a built-in definition's "releaseLine" (0 for none),
// as audit.py's _release_line_parts reads it.
func ReleaseLineParts(home, lang string) int {
	config, ok := LoadJSONFile(filepath.Join(home, "languages", lang, "config.json")).(*Object)
	if !ok {
		return 0
	}
	if n, ok := config.Values["releaseLine"].(Number); ok && !n.Float {
		parts := 0
		for _, c := range n.Text {
			if c == '-' {
				return 0
			}
			parts = parts*10 + int(c-'0')
		}
		return parts
	}
	return 0
}

// ReportFailure is what the scripts print when a tool's report can't be
// read: why, and the tool's output (its first 4000 characters).
func ReportFailure(stderr io.Writer, err error, lang, format, text string) {
	fmt.Fprintf(stderr, "error: %s: couldn't read the %s report (%v); its output was:\n", lang, format, err)
	if runes := []rune(text); len(runes) > 4000 {
		text = string(runes[:4000])
	}
	fmt.Fprintln(stderr, text)
}

// RunAuditReport is audit.py's `report` command once the report is read:
// the findings on stdout and the exit status, or the failure on stderr.
func RunAuditReport(stdout, stderr io.Writer, format, text, lang, component string, config any, failOn, today string) int {
	var out bytes.Buffer
	status, err := AuditReport(&out, format, text, lang, component, config, failOn, today)
	if err != nil {
		// What was printed before the error comes after it, as Python's
		// buffered stdout does when it isn't a terminal.
		ReportFailure(stderr, err, lang, format, text)
		stdout.Write(out.Bytes())
		return 1
	}
	stdout.Write(out.Bytes())
	return status
}

// RunOutdatedReport is outdated.py's `report` command once the report is
// read.
func RunOutdatedReport(stdout, stderr io.Writer, format, text, lang, component, direct, jsonDir string, fail bool) int {
	var out bytes.Buffer
	status, err := OutdatedReport(&out, format, text, lang, component, direct, jsonDir, fail)
	if err != nil {
		ReportFailure(stderr, err, lang, format, text)
		return 1
	}
	stdout.Write(out.Bytes())
	return status
}
