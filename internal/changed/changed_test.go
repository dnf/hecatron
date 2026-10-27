package changed

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dnf/hecatron/internal/project"
)

// Fixture repositories: a main branch, then a branch with one commit (and
// sometimes uncommitted changes); what --changed selects on the branch.

var definitions = map[string]string{
	"cpp/config.json":    `{"sharedBuildFiles": ["CMakeLists.txt"]}`,
	"go/config.json":     `{"detect": ["go.mod"]}`,
	"kotlin/config.json": `{}`,
	"python/config.json": `{"pinFiles": [{"file": ".python-version", "format": "python-version"}],
		"supportFiles": [{"file": ".github/workflows/*.yml", "format": "github-setup", "key": "python-version"}]}`,
}

var mainFiles = map[string]string{
	".hecatron/config.json": `{"schema": 1, "components": ["a", "b"], "languages": ["cpp", "go", "kotlin", "python"],
		"tasks": {"test": {"codegen": "true", "always": "true"}}, "taskPaths": {"codegen": ["codegen/", "*.proto"]}}`,
	".hecatron/languages/go/test.sh": "go test ./...\n",
	".gitignore":                     "*.log\n",
	"a/cpp/x.cpp":                    "",
	"b/cpp/x.cpp":                    "",
	"a/go/main.go":                   "",
	"b/go/main.go":                   "",
	"a/kotlin/X.kt":                  "",
	"b/kotlin/X.kt":                  "",
	"a/python/x.py":                  "",
	"codegen/gen.py":                 "",
	"README.md":                      "",
	"CMakeLists.txt":                 "",
	"settings.gradle.kts":            "",
	".github/workflows/ci.yml":       "",
	"docs/guide.md":                  "",
}

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// setup isolates git from the machine's configuration and points the model
// at the fixture definitions.
func setup(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	defs := t.TempDir()
	write(t, defs, definitions)
	for name, value := range map[string]string{
		"HECATRON_BUILTIN_LANGUAGES": defs, "HECATRON_AS_OF": "off", "GIT_CONFIG_NOSYSTEM": "1",
		"GIT_CONFIG_GLOBAL": filepath.Join(defs, "no-gitconfig"), "GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@example.com",
		"GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@example.com",
	} {
		t.Setenv(name, value)
	}
}

// repository is a fixture repository on a branch off main.
func repository(t *testing.T) string {
	t.Helper()
	root, _ := filepath.EvalSymlinks(t.TempDir())
	root = filepath.Join(root, "repo")
	write(t, root, mainFiles)
	run(t, root, "init", "-q", "-b", "main")
	run(t, root, "add", "-A")
	run(t, root, "commit", "-qm", "main")
	run(t, root, "checkout", "-qb", "work")
	return root
}

func commit(t *testing.T, root string, files map[string]string) {
	t.Helper()
	write(t, root, files)
	run(t, root, "add", "-A")
	run(t, root, "commit", "-qm", "change")
}

func noEnv(string) string { return "" }

func selection(t *testing.T, root, ref string, getenv func(string) string) *Selection {
	t.Helper()
	base, note, err := FindBase(root, ref, getenv)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	if note == "" {
		if files, err = Files(root, base.Commit); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root, project.NewHome(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	s, err := Select(p, base, files, []string{"codegen", "always"})
	if err != nil {
		t.Fatal(err)
	}
	s.Note = note
	return s
}

// selected is a selection as "lang:components" and "task:name" entries.
func selected(s *Selection) []string {
	if s.All() {
		return []string{"everything"}
	}
	var got []string
	for _, l := range s.Languages {
		got = append(got, l.Name+":"+strings.Join(l.Components, ","))
	}
	for _, task := range []string{"codegen", "always"} {
		if _, ok := s.tasks[task]; ok {
			got = append(got, "task:"+task)
		}
	}
	if s.shell != "" {
		got = append(got, "shell")
	}
	return got
}

func TestSelect(t *testing.T) {
	setup(t)
	cases := []struct {
		name   string
		commit map[string]string
		edit   map[string]string // uncommitted
		want   []string
		line   string // a part of the summary line
	}{
		{"one language, one component", map[string]string{"a/go/main.go": "package main\n"}, nil,
			[]string{"go:a", "task:always"}, "running go [a] (a/go/main.go), task always (no taskPaths: any change)"},
		{"a shared build file", map[string]string{"CMakeLists.txt": "project(x)\n"}, nil,
			[]string{"cpp:a,b", "task:always"}, "running cpp (CMakeLists.txt: a build or version file above it), task always"},
		{"gradle settings, above everything", map[string]string{"settings.gradle.kts": "x\n"}, nil,
			[]string{"cpp:a,b", "go:a,b", "kotlin:a,b", "python:a", "task:always"}, "running cpp, go, kotlin, python (settings.gradle.kts: a build file above it), task always"},
		{"a version source with a path", map[string]string{".github/workflows/ci.yml": "x\n"}, nil,
			[]string{"python:a", "task:always"}, "running python (.github/workflows/ci.yml: where its version is read from)"},
		{"the config", map[string]string{".hecatron/config.json": strings.Replace(mainFiles[".hecatron/config.json"], `"a", "b"`, `"b", "a"`, 1)}, nil,
			[]string{"everything"}, "everything runs: .hecatron/config.json, the project's configuration"},
		{"a hook", map[string]string{".hecatron/languages/go/test.sh": "go test -race ./...\n"}, nil,
			[]string{"go:a,b", "task:always"}, "running go (.hecatron/languages/go/test.sh: its hooks or definition)"},
		{"a task's paths", map[string]string{"codegen/gen.py": "x\n", "a/x.proto": ""}, nil,
			[]string{"task:codegen", "task:always"}, "task codegen (a/x.proto)"},
		{"nothing relevant to languages", map[string]string{"README.md": "x\n", "docs/guide.md": "x\n"}, nil,
			[]string{"task:always"}, ": 2 files;"},
		{"uncommitted and untracked, not ignored", nil, map[string]string{"b/kotlin/X.kt": "x\n", "a/python/new.py": "", "a/cpp/build.log": "", "tools/run.sh": ""},
			[]string{"kotlin:b", "python:a", "task:always", "shell"}, "3 files; running kotlin [b] (b/kotlin/X.kt), python (a/python/new.py), task always (no taskPaths: any change), shell (tools/run.sh)"},
		{"no change", nil, nil, nil, ": 0 files; nothing to run"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := repository(t)
			if c.commit != nil {
				commit(t, root, c.commit)
			}
			write(t, root, c.edit)
			s := selection(t, root, "", noEnv)
			t.Log(s.Line())
			if got := selected(s); !reflect.DeepEqual(got, c.want) {
				t.Errorf("selected %v, want %v (files %v)", got, c.want, s.Files)
			}
			if line := s.Line(); !strings.Contains(line, c.line) || !strings.HasPrefix(line, "Changed since main (the default branch, merge base ") {
				t.Errorf("line %q, want it to hold %q", line, c.line)
			}
			if s.Empty() != (len(c.want) == 0) {
				t.Errorf("Empty() = %v", s.Empty())
			}
		})
	}
}

// With nothing but a task without taskPaths left out, a change to
// nothing relevant selects nothing.
func TestNothingSelected(t *testing.T) {
	setup(t)
	root := repository(t)
	commit(t, root, map[string]string{"README.md": "x\n"})
	base, _, _ := FindBase(root, "", noEnv)
	files, _ := Files(root, base.Commit)
	p, err := project.Load(root, project.NewHome(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	s, _ := Select(p, base, files, []string{"codegen"})
	if !s.Empty() || !strings.HasSuffix(s.Line(), "; nothing to run: none is in a language's directory or a build file (README.md)") {
		t.Errorf("%v: %s", selected(s), s.Line())
	}
}

func TestBase(t *testing.T) {
	setup(t)
	root := repository(t)
	commit(t, root, map[string]string{"a/go/main.go": "x\n"})
	commit(t, root, map[string]string{"b/go/main.go": "x\n"})
	if s := selection(t, root, "HEAD~1", noEnv); !reflect.DeepEqual(selected(s), []string{"go:b", "task:always"}) ||
		!strings.HasPrefix(s.Line(), "Changed since HEAD~1 (--changed, ") {
		t.Errorf("--changed HEAD~1: %v, %s", selected(s), s.Line())
	}
	if _, _, err := FindBase(root, "no-such-branch", noEnv); err == nil || err.Error() != "--changed: 'no-such-branch' is not a commit" {
		t.Errorf("an unknown ref: %v", err)
	}
	if _, _, err := FindBase(t.TempDir(), "", noEnv); err == nil || err.Error() != "--changed needs a git repository" {
		t.Errorf("outside a repository: %v", err)
	}
	ci := func(vars map[string]string) func(string) string {
		return func(name string) string { return vars[name] }
	}
	// GitHub's base is a branch, normally origin/<base>; here, the local one.
	if s := selection(t, root, "", ci(map[string]string{"CI": "true", "GITHUB_BASE_REF": "main"})); s.Base.Source != "GITHUB_BASE_REF" || len(s.Languages) != 1 {
		t.Errorf("GITHUB_BASE_REF: %+v, %s", s.Base, s.Line())
	}
	sha := strings.TrimSpace(run(t, root, "rev-parse", "main"))
	if s := selection(t, root, "", ci(map[string]string{"CI": "true", "CI_MERGE_REQUEST_DIFF_BASE_SHA": sha})); s.Base.Commit != sha {
		t.Errorf("GitLab's base: %+v", s.Base)
	}
	if s := selection(t, root, "", ci(map[string]string{"TF_BUILD": "True", "SYSTEM_PULLREQUEST_TARGETBRANCH": "refs/heads/main"})); s.Base.Ref != "main" || s.Note != "" {
		t.Errorf("Azure's base: %+v %s", s.Base, s.Note)
	}
	if s := selection(t, root, "", ci(map[string]string{"CI": "true"})); !s.All() || s.Line() != "--changed: a CI run outside a pull request; running everything" {
		t.Errorf("a push build: %s", s.Line())
	}
	if s := selection(t, root, "", ci(map[string]string{"CI": "true", "GITHUB_BASE_REF": "gone"})); !s.All() ||
		s.Line() != "--changed: GITHUB_BASE_REF's gone isn't in this clone (shallow? fetch it); running everything" {
		t.Errorf("a base not fetched: %s", s.Line())
	}
	if s := selection(t, root, "", ci(map[string]string{"CI": "false"})); s.Base.Source != "the default branch" {
		t.Errorf("CI=false: %+v", s.Base)
	}
}

// A shallow clone has the default branch but not the history down to the
// merge base: everything runs, with a note.
func TestShallowClone(t *testing.T) {
	setup(t)
	root := repository(t)
	commit(t, root, map[string]string{"a/go/main.go": "x\n"})
	run(t, root, "checkout", "-q", "main")
	commit(t, root, map[string]string{"b/go/main.go": "x\n"})
	clone := filepath.Join(filepath.Dir(root), "clone")
	run(t, filepath.Dir(root), "clone", "-q", "--depth", "1", "--no-single-branch", "--branch", "work", "file://"+root, clone)
	s := selection(t, clone, "", noEnv)
	if !s.All() || s.Line() != "--changed: no merge base with origin/main in this clone (shallow? fetch more history); running everything" {
		t.Errorf("%v: %s", selected(s), s.Line())
	}
}

func TestMatches(t *testing.T) {
	yes := [][2]string{{"CMakeLists.txt", "a/CMakeLists.txt"}, {"*.sln", "x.sln"}, {"codegen/", "codegen/a/b.py"},
		{".github/workflows/*.yml", ".github/workflows/ci.yml"}, {"gradle/", "gradle/libs.versions.toml"}, {"codegen", "codegen"}}
	no := [][2]string{{"CMakeLists.txt", "a/CMakeLists.txt.bak"}, {"codegen/", "codegenx/a"}, {".github/workflows/*.yml", "ci.yml"}}
	for _, c := range yes {
		if !isBuildFile(c[1], []string{c[0]}) {
			t.Errorf("%s doesn't match %s", c[1], c[0])
		}
	}
	for _, c := range no {
		if isBuildFile(c[1], []string{c[0]}) {
			t.Errorf("%s matches %s", c[1], c[0])
		}
	}
	for file, want := range map[string]string{"gradle/wrapper/x.jar": ".", "sub/gradle/libs.versions.toml": "sub", "a/b/Cargo.lock": "a/b", "go.work": "."} {
		if got := buildFileDir(file); got != want {
			t.Errorf("buildFileDir(%s) = %s, want %s", file, got, want)
		}
	}
}
