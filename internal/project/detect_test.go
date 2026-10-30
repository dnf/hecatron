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
	"languages at the root":    {"go/go.mod": "", "rust/Cargo.toml": "", "python/pyproject.toml": "", "README": ""},
	"components":               {"age/impl/go/go.mod": "", "date/impl/go/go.mod": "", "age/impl/rust/Cargo.toml": "", "date/impl/rust/src/lib.rs": "", "date/impl/rust/Cargo.toml": ""},
	"one language at the root": {"go.mod": "", "main.go": ""},
	"nothing":                  {"README.md": ""},
	"tsconfig beside mostly javascript": {"runtime/JavaScript/package.json": "{\"scripts\": {\"test\": \"jasmine\"}}",
		"runtime/JavaScript/tsconfig.json": "", "runtime/JavaScript/src/a.js": "", "runtime/JavaScript/src/b.js": "",
		"runtime/JavaScript/src/a.d.ts": "", "runtime/JavaScript/src/b.d.ts": "", "runtime/JavaScript/src/c.ts": "",
		"runtime/JavaScript/.eslintrc.js": "", "runtime/JavaScript/spec/x.js": "", "runtime/JavaScript/spec/y.js": "",
		"runtime/Go/go.mod": "", "web/tsconfig.json": "", "web/package.json": "{}", "web/src/a.ts": "", "web/webpack.config.js": ""},
	"typescript with its compiled javascript tracked": {
		"dist-pkg/package.json": "{\"scripts\": {\"test\": \"x\"}}", "dist-pkg/tsconfig.json": "{}",
		"dist-pkg/src/a.ts": "", "dist-pkg/src/b.ts": "", "dist-pkg/dist/a.js": "", "dist-pkg/dist/b.js": "",
		"dist-pkg/dist/c.js": "", "dist-pkg/dist/d.js": "",
		"lib-pkg/package.json": "{\"scripts\": {\"test\": \"x\"}}", "lib-pkg/tsconfig.json": "{\"compilerOptions\": {\"outDir\": \"./lib\"}}",
		"lib-pkg/src/index.ts": "", "lib-pkg/lib/index.js": "", "lib-pkg/lib/index.mjs": "", "lib-pkg/lib/util.js": "",
		"out-pkg/package.json":  "{\"scripts\": {\"test\": \"x\"}}",
		"out-pkg/tsconfig.json": "{\n  // compiled here\n  \"compilerOptions\": {\"outDir\": \"compiled/\",},\n}",
		"out-pkg/index.ts":      "", "out-pkg/compiled/index.js": "", "out-pkg/compiled/x.js": "",
		"twin-pkg/package.json": "{\"scripts\": {\"test\": \"x\"}}", "twin-pkg/tsconfig.json": "{",
		"twin-pkg/src/a.ts": "", "twin-pkg/src/a.js": "", "twin-pkg/src/b.tsx": "", "twin-pkg/src/b.js": "",
		"twin-pkg/src/c.js": ""},
	"only tsconfigs beside mostly javascript": {"package.json": "{\"scripts\": {\"test\": \"x\"}}", "tsconfig.json": "",
		"src/a.js": "", "src/b.mjs": "", "types/a.d.ts": ""},
	"names":                       {"lib/rb/Gemfile": "", "lib/rb/x.gemspec": "", "lib/netstd/src/x.csproj": "", "lib/go/go.mod": ""},
	"preferred parent":            {"sdks/go/go.mod": "", "learning/katas/go/go.mod": "", "sdks/java/pom.xml": "", "learning/katas/java/pom.xml": "", "learning/katas/kotlin/build.gradle.kts": ""},
	"outside the layout":          {"go/go.mod": "", "java/pom.xml": "", "javascript/package.json": "{\"scripts\": {\"test\": \"x\"}}", "javascript/tsconfig.json": "", "CMakeLists.txt": "", "src/a.cpp": ""},
	"borrowed directory":          {"go.mod": "", "lib/java/pom.xml": "", "lib/rb/Gemfile": "", "lib/go/x.go": ""},
	"dotfile sources":             {"Cargo.toml": "", "src/main.rs": "", ".nvim.lua": ""},
	"premake scripts":             {"CMakeLists.txt": "", "lib/a.c": "", "contrib/premake/premake4.lua": "", "contrib/premake/zstd.lua": ""},
	"octave package":              {"DESCRIPTION": "", "INDEX": "", "inst/x.m": ""},
	"bats suite":                  {"test/a.bats": "", "test/test_helper.bash": "", "lib/x.bash": "", "package.json": "{}"},
	"bats in a package":           {"tools/x/test/a.bats": "", "cli/tests/b.bats": "", "vendor/y/test/c.bats": "", "go.mod": ""},
	"skipped trees":               {"go/go.mod": "", "examples/lua/x.lua": "", "third_party/zlib/x.c": "", "go/testdata/x.c": "", "docs/conf.py": "", "tests/x/pyproject.toml": ""},
	"skipped by any case, hidden": {"go/go.mod": "", "Examples/java/A.java": "", ".jenkins/x.groovy": "", "cli/src/templates/pom.xml": "", "tests_data/groovy/x.groovy": ""},
	"npm scripts":                 {"js/package.json": `{"scripts": {"test": "mocha"}}`, "jsonschema/package.json": `{"scripts": {"build": "x"}}`, "broken/package.json": "{", "list/package.json": `{"scripts": ["test"]}`, "null/package.json": `{"scripts": null}`, "arr/package.json": `[]`},
	"no test script":              {"jsonschema/package.json": "{}"},
	"node apps without tests": {"api/package.json": `{"scripts": {"start": "node .", "dev": "nodemon"}}`, "api/index.js": "",
		"web/package.json": `{"type": "module", "dependencies": {"express": "^4"}}`, "web/yarn.lock": "",
		"cli/package.json": `{"bin": {"x": "cli.js"}}`, "lib/package.json": `{"exports": {".": "./a.js"}}`,
		"empty/package.json": `{"dependencies": {}, "scripts": {"start": ""}}`, "deep/package.json": `{"scripts": {"dev": {"x": 1}}}`},
	"package.json for commit hooks": {"package.json": `{"devDependencies": {"husky": "^9"}, "scripts": {"prepare": "husky"}}`,
		"package-lock.json": "{}", "Package.swift": "", "Sources/A/a.swift": ""},
	"requirements beside python": {"requirements.txt": "flask", "app.py": "", "Dockerfile": "",
		"worker/requirements.txt": "", "worker/src/tasks.py": ""},
	"requirements without python": {"go.mod": "", "main.go": "", "requirements.txt": "mkdocs", "docs/requirements.txt": "",
		"docs/conf.py": "", "site/requirements.txt": "", ".github/scripts/requirements.txt": "", ".github/scripts/x.py": ""},
	"xcode projects": {"App.xcodeproj/project.pbxproj": "", "App/AppDelegate.swift": "", "Gemfile": "", "Gemfile.lock": "",
		"Legacy/Legacy.xcodeproj/project.pbxproj": "", "Legacy/Sources/a.m": "", "Pods/Pods.xcodeproj/project.pbxproj": "",
		"Pods/Alamofire/a.swift": "", "Carthage/Checkouts/x/x.xcodeproj/project.pbxproj": "", "Carthage/Checkouts/x/a.m": ""},
	"xcode workspace alone":      {"App.xcworkspace/contents.xcworkspacedata": "", "README.md": ""},
	"xcode project in a package": {"Package.swift": "", "Sources/a.swift": "", "Example/Example.xcodeproj/project.pbxproj": "", "Example/App/a.swift": "", "ios/App.xcodeproj/project.pbxproj": "", "ios/App/AppDelegate.mm": "", "package.json": `{"main": "index.js"}`, "index.js": ""},
	"shared build files":         {"cpp/CMakeLists.txt": "", "cpp/src/a.cc": "", "cpp/src/b.cc": "", "cpp/src/utf/x.c": "", "Makefile": "", "lib/c/a.c": ""},
	"sources inside another":     {"cpp/CMakeLists.txt": "", "cpp/src/utf/x.c": "", "objectivec/a.m": "", "objectivec/b.c": "", "python/pyproject.toml": "", "python/ext/a.c": "", "upb/base/b.c": ""},
	"own directory":              {"go/go.mod": "", "java/pom.xml": "", "clr-binding/Lib.csproj": "", "a/x.csproj": "", "b/y.csproj": ""},
	"unicode names":              {"caf\u00e9/go.mod": "", "go/go.mod": "", "z\u00fcrich/Cargo.toml": ""},
	"two varying parts":          {"a/x/go/go.mod": "", "b/y/go/go.mod": ""},
	"components and another":     {"src/ad/main/java/pom.xml": "", "src/fraud/main/kotlin/build.gradle.kts": "", "src/frontend/package.json": "{}", "src/frontend/tsconfig.json": ""},
	"root module, definitions":   {"go.mod": "", "cmd/x/main.go": "", "languages/go/config.json": "", "languages/go/install.sh": "", "languages/tcl/config.json": "", "languages/tcl/hooks/run.tcl": "", "languages/rust/Cargo.toml": ""},
	"go directory module":        {"go/go.mod": "", "go/a.go": "", "rust/Cargo.toml": "", "go.work": ""},
	"root module, code in lib":   {"go.mod": "", "lib/go/thrift/a.go": "", "lib/java/pom.xml": "", "lib/rb/x.gemspec": ""},
	"deep go module":             {"CMakeLists.txt": "", "src/a.cpp": "", "runtime/Go/antlr/v4/go.mod": "", "runtime/Go/antlr/v4/a.go": "", "pyproject.toml": ""},
	"test data, vendored":        {"go.mod": "", "tests_data/basic/c/a.c": "", "src/vendored/xxhash/x.c": "", "src/a.go": ""},
	"templates":                  {"Cargo.toml": "", "crates/cli/src/templates/go.mod": ""},
	"tcl in hooks":               {"go.mod": "", "languages/tcl/hooks/run.tcl": "", ".githooks/hooks/x.tcl": ""},
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
// language's files, and hooks/ is tooling (docs/guide/init.md).
func TestDetectionPlacesARootModuleAndSkipsHooks(t *testing.T) {
	checkDetectedLayouts(t, map[string]string{
		"root module, definitions": "languages/{lang} [go rust] go=.",
		"go directory module":      "{lang} [go rust] ",
		"root module, code in lib": "lib/{lang} [go java ruby] ",
		"tcl in hooks":             ". [go] ",
	})
}

// A package.json counts only with something a package or an app has, a
// requirements.txt only beside Python code, and an Xcode project for the
// language of the sources it builds; CocoaPods' and Carthage's checkouts
// are vendored code (docs/guide/init.md).
func TestDetectionOfPackagesAppsAndTooling(t *testing.T) {
	checkDetectedLayouts(t, map[string]string{
		"node apps without tests":       ". [javascript] ",
		"package.json for commit hooks": ". [swift] ",
		"requirements beside python":    ". [python] ",
		"requirements without python":   ". [go] ",
		"xcode projects":                ". [objectivec ruby swift] ",
		"xcode workspace alone":         ". [] ",
	})
}

// checkDetectedLayouts compares each detectionRepos fixture's detected
// implPath, languages and implPaths with the one wanted.
func checkDetectedLayouts(t *testing.T, wanted map[string]string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := repoHome(t)
	t.Setenv("HECATRON_BUILTIN_LANGUAGES", "")
	for name, want := range wanted {
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
