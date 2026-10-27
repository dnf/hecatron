package runner

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/dnf/hecatron/internal/project"
)

// Each captured sample (tests/output/<tool>/<version>/<case>.log, real
// output from a failing fixture, tests/output/capture.sh) must yield its
// error lines from its language's patterns, not the fallback, and the
// excerpt recorded beside it (<case>.excerpt). HECATRON_UPDATE_EXCERPTS=1
// rewrites the recorded excerpts.

// directTools are the samples of a compiler run without a hook, which have
// no hecatron header naming the language.
var directTools = map[string]string{"javac": "java", "kotlinc": "kotlin"}

var header = regexp.MustCompile(`(?m)^=== \S+ \S+ \((\S+)\) ===$`)

func TestPatternsMatchCapturedSamples(t *testing.T) {
	root := filepath.Join("..", "..")
	logs, err := filepath.Glob(filepath.Join(root, "tests", "output", "*", "*", "*.log"))
	if err != nil || len(logs) == 0 {
		t.Fatalf("no samples (%v)", err)
	}
	update := os.Getenv("HECATRON_UPDATE_EXCERPTS") == "1"
	for _, log := range logs {
		rel, _ := filepath.Rel(filepath.Join(root, "tests", "output"), log)
		t.Run(filepath.ToSlash(rel), func(t *testing.T) {
			data, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			tool := strings.Split(filepath.ToSlash(rel), "/")[0]
			lang := directTools[tool]
			if m := header.FindSubmatch(data); m != nil {
				lang = string(m[1])
			}
			patterns := languagePatterns(t, root, lang)
			excerpt := Extract(data, patterns)
			if excerpt.Source != "patterns" {
				t.Errorf("%s's patterns match nothing (excerpt from %q)", lang, excerpt.Source)
			}
			var got strings.Builder
			for _, line := range excerpt.Lines {
				got.WriteString(describe(line) + "\n")
			}
			golden := strings.TrimSuffix(log, ".log") + ".excerpt"
			if update {
				if err := os.WriteFile(golden, []byte(got.String()), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (HECATRON_UPDATE_EXCERPTS=1 records it)", err)
			}
			if got.String() != string(want) {
				t.Errorf("excerpt:\n%s\nrecorded:\n%s", got.String(), want)
			}
		})
	}
}

// describe is an error line as the recorded excerpts show it: the text,
// then where it points.
func describe(line ErrorLine) string {
	s := line.Text
	if line.File != "" {
		s += "\n    -> " + line.File + ":" + strconv.Itoa(line.Line)
		if line.Column > 0 {
			s += ":" + strconv.Itoa(line.Column)
		}
		if line.Message != "" {
			s += " " + line.Message
		}
	}
	return s
}

func languagePatterns(t *testing.T, root, lang string) *Patterns {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "languages", lang, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := project.DecodeJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	definition := decoded.(*project.Object)
	list, _ := definition.Get("errorPatterns")
	versions, _ := definition.Get("errorToolVersions")
	patterns, problems := ParsePatterns(list, versions)
	if len(problems) > 0 {
		t.Fatalf("%s: %v", lang, problems)
	}
	return patterns
}
