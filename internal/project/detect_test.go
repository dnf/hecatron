package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

// Detection must give the config manage.py's detected_config gives, and
// the same languages left out: on fixture repositories, and on the
// repositories under $HECATRON_DETECT_REPOS (clones of real projects; only
// read).

const detectedConfigPy = `
import json, os, sys
sys.path.insert(0, os.path.join(os.environ["HECATRON_HOME"], "libexec", "commands"))
import manage
config, not_added = manage.detected_config(sys.argv[1], *[a or None for a in sys.argv[2:]])
print(json.dumps({"config": config, "notAdded": dict(sorted(not_added.items()))}, indent=2))
`

func pythonDetected(t *testing.T, home, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("python3", append([]string{"-c", detectedConfigPy, root}, args...)...)
	cmd.Env = append(os.Environ(), "HECATRON_HOME="+home)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("detected_config: %v", err)
	}
	return string(out)
}

func goDetected(t *testing.T, home, root string, args ...string) string {
	t.Helper()
	for len(args) < 3 {
		args = append(args, "")
	}
	config, notAdded, err := DetectedConfig(root, NewHome(home), args[0], args[1], args[2])
	if err != nil {
		t.Fatalf("DetectedConfig: %v", err)
	}
	reasons := &Object{Values: map[string]any{}}
	for _, lang := range sortedKeys(notAdded) {
		reasons.set(lang, notAdded[lang])
	}
	result := &Object{Values: map[string]any{}}
	result.set("config", config)
	result.set("notAdded", reasons)
	text, err := PyJSON(result)
	if err != nil {
		t.Fatal(err)
	}
	return text + "\n"
}

func sortedKeys(m map[string]string) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// detectionRepos are fixture repositories shaped like the layouts
// detection has to tell apart (tests/test_hecatron.py's InitTest).
var detectionRepos = map[string]map[string]string{
	"languages at the root":       {"go/go.mod": "", "rust/Cargo.toml": "", "python/pyproject.toml": "", "README": ""},
	"components":                  {"age/impl/go/go.mod": "", "date/impl/go/go.mod": "", "age/impl/rust/Cargo.toml": "", "date/impl/rust/src/lib.rs": "", "date/impl/rust/Cargo.toml": ""},
	"one language at the root":    {"go.mod": "", "main.go": ""},
	"nothing":                     {"README.md": ""},
	"names":                       {"lib/rb/Gemfile": "", "lib/rb/x.gemspec": "", "lib/netstd/src/x.csproj": "", "lib/go/go.mod": ""},
	"preferred parent":            {"sdks/go/go.mod": "", "learning/katas/go/go.mod": "", "sdks/java/pom.xml": "", "learning/katas/java/pom.xml": "", "learning/katas/kotlin/build.gradle.kts": ""},
	"outside the layout":          {"go/go.mod": "", "java/pom.xml": "", "javascript/package.json": "{\"scripts\": {\"test\": \"x\"}}", "javascript/tsconfig.json": "", "CMakeLists.txt": "", "src/a.cpp": ""},
	"borrowed directory":          {"go.mod": "", "lib/java/pom.xml": "", "lib/rb/Gemfile": "", "lib/go/x.go": ""},
	"dotfile sources":             {"Cargo.toml": "", "src/main.rs": "", ".nvim.lua": ""},
	"premake scripts":             {"CMakeLists.txt": "", "lib/a.c": "", "contrib/premake/premake4.lua": "", "contrib/premake/zstd.lua": ""},
	"octave package":              {"DESCRIPTION": "", "INDEX": "", "inst/x.m": ""},
	"skipped trees":               {"go/go.mod": "", "examples/lua/x.lua": "", "third_party/zlib/x.c": "", "go/testdata/x.c": "", "docs/conf.py": "", "tests/x/pyproject.toml": ""},
	"skipped by any case, hidden": {"go/go.mod": "", "Examples/java/A.java": "", ".jenkins/x.groovy": "", "cli/src/templates/pom.xml": "", "tests_data/groovy/x.groovy": ""},
	"npm scripts":                 {"js/package.json": `{"scripts": {"test": "mocha"}}`, "jsonschema/package.json": `{"scripts": {"build": "x"}}`, "broken/package.json": "{", "list/package.json": `{"scripts": ["test"]}`, "null/package.json": `{"scripts": null}`, "arr/package.json": `[]`},
	"no test script":              {"jsonschema/package.json": "{}"},
	"shared build files":          {"cpp/CMakeLists.txt": "", "cpp/src/a.cc": "", "cpp/src/b.cc": "", "cpp/src/utf/x.c": "", "Makefile": "", "lib/c/a.c": ""},
	"sources inside another":      {"cpp/CMakeLists.txt": "", "cpp/src/utf/x.c": "", "objectivec/a.m": "", "objectivec/b.c": "", "python/pyproject.toml": "", "python/ext/a.c": "", "upb/base/b.c": ""},
	"own directory":               {"go/go.mod": "", "java/pom.xml": "", "clr-binding/Lib.csproj": "", "a/x.csproj": "", "b/y.csproj": ""},
	"unicode names":               {"caf\u00e9/go.mod": "", "go/go.mod": "", "z\u00fcrich/Cargo.toml": ""},
	"two varying parts":           {"a/x/go/go.mod": "", "b/y/go/go.mod": ""},
	"components and another":      {"src/ad/main/java/pom.xml": "", "src/fraud/main/kotlin/build.gradle.kts": "", "src/frontend/package.json": "{}", "src/frontend/tsconfig.json": ""},
	"root module, definitions":    {"go.mod": "", "cmd/x/main.go": "", "languages/go/config.json": "", "languages/go/install.sh": "", "languages/tcl/config.json": "", "languages/tcl/hooks/run.tcl": "", "languages/rust/Cargo.toml": ""},
	"go directory module":         {"go/go.mod": "", "go/a.go": "", "rust/Cargo.toml": "", "go.work": ""},
	"root module, code in lib":    {"go.mod": "", "lib/go/thrift/a.go": "", "lib/java/pom.xml": "", "lib/rb/x.gemspec": ""},
	"tcl in hooks":                {"go.mod": "", "languages/tcl/hooks/run.tcl": "", ".githooks/hooks/x.tcl": ""},
}

func TestDetectionMatchesManagePy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("compares with manage.py")
	}
	for _, tool := range []string{"python3", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("no " + tool)
		}
	}
	home := repoHome(t)
	t.Setenv("HECATRON_BUILTIN_LANGUAGES", "")
	options := [][]string{nil, {"a,,b", "", ""}, {"", "src/{lang}", ""}, {"", "", "rust,go,,go"}, {".", "", ""}}
	for name, files := range detectionRepos {
		root := parityCase{files: files, git: true}.create(t)
		for _, args := range options {
			if want, got := pythonDetected(t, home, root, args...), goDetected(t, home, root, args...); got != want {
				t.Errorf("%s %q:\nmanage.py:\n%s\nGo:\n%s", name, args, want, got)
			}
		}
	}
	repos := os.Getenv("HECATRON_DETECT_REPOS")
	if repos == "" {
		return
	}
	entries, err := os.ReadDir(repos)
	if err != nil {
		t.Fatal(err)
	}
	compared := 0
	for _, entry := range entries {
		root := filepath.Join(repos, entry.Name())
		if !isDir(filepath.Join(root, ".git")) {
			continue
		}
		want, got := pythonDetected(t, home, root), goDetected(t, home, root)
		if got != want {
			t.Errorf("%s:\nmanage.py:\n%s\nGo:\n%s", entry.Name(), want, got)
		}
		compared++
	}
	t.Logf("%d repositories compared", compared)
}

// A directory named for a language counts only when it holds the
// language's files, and hooks/ is tooling (docs/design.md §10).
func TestDetectionPlacesARootModuleAndSkipsHooks(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := repoHome(t)
	t.Setenv("HECATRON_BUILTIN_LANGUAGES", "")
	for name, want := range map[string]string{
		"root module, definitions": "languages/{lang} [go rust] go=.",
		"go directory module":      "{lang} [go rust] ",
		"root module, code in lib": "lib/{lang} [go java ruby] ",
		"tcl in hooks":             ". [go] ",
	} {
		root := parityCase{files: detectionRepos[name], git: true}.create(t)
		config, _, err := DetectedConfig(root, NewHome(home), "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		got := fmt.Sprint(config.Values["implPath"], " ", config.Values["languages"], " ")
		if implPaths, ok := config.Values["implPaths"].(*Object); ok {
			for _, lang := range implPaths.Keys {
				got += fmt.Sprint(lang, "=", implPaths.Values[lang])
			}
		}
		if got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
}
