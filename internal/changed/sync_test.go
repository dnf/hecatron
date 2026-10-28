package changed

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dnf/hecatron/internal/project"
)

// hct sync's view of a move: the base (the reflog's previous HEAD, before a
// rebase's steps, or a ref), and what the files changed ask for.

var syncDefinitions = map[string]string{
	"go/config.json": `{"cache": {"lockFiles": ["go.sum"]}, "pinFiles": [{"file": "go.mod", "format": "go-mod-toolchain"}]}`,
	"python/config.json": `{"cache": {"lockFiles": ["uv.lock", "requirements*.txt"]},
		"pinFiles": [{"file": ".python-version", "format": "python-version"}],
		"supportFiles": [{"file": ".github/workflows/*.yml", "format": "github-setup", "key": "python-version"}]}`,
	"java/config.json": `{"cache": {"lockFiles": ["gradle/wrapper/gradle-wrapper.properties"]}}`,
}

func syncProject(t *testing.T) *project.Project {
	t.Helper()
	setup(t)
	defs := t.TempDir()
	write(t, defs, syncDefinitions)
	t.Setenv("HECATRON_BUILTIN_LANGUAGES", defs)
	root := t.TempDir()
	write(t, root, map[string]string{
		".hecatron/config.json": `{"schema": 1, "components": ["a", "b", "c"], "languages": ["go", "python", "java"]}`,
		"a/go/main.go":          "",
		"b/go/main.go":          "",
		"a/python/x.py":         "",
		"b/python/x.py":         "",
		"c/python/x.py":         "",
		"a/java/X.java":         "",
	})
	p, err := project.Load(root, project.NewHome(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// caught is a Catchup as "deps lang:components", "toolchain ..." and
// "check file" entries.
func caught(c *Catchup) []string {
	var got []string
	for _, kind := range []struct {
		name string
		list []Touched
	}{{"deps", c.Deps}, {"toolchain", c.Toolchains}} {
		for _, t := range kind.list {
			got = append(got, kind.name+" "+t.Name+":"+strings.Join(t.Components, ","))
		}
	}
	for _, file := range c.Check {
		got = append(got, "check "+file)
	}
	return got
}

func TestCatch(t *testing.T) {
	p := syncProject(t)
	cases := []struct {
		name      string
		files     []string
		generated []string
		want      []string
	}{
		{"a component's lock file", []string{"a/python/uv.lock"}, nil, []string{"deps python:a"}},
		{"a pattern", []string{"b/python/requirements-dev.txt"}, nil, []string{"deps python:b"}},
		{"a workspace's lock file", []string{"uv.lock"}, nil, []string{"deps python:a,b,c"}},
		{"a lock file between", []string{"a/uv.lock"}, nil, []string{"deps python:a"}},
		{"a nested package's", []string{"c/python/tools/uv.lock"}, nil, []string{"deps python:c"}},
		{"another language's name", []string{"a/go/uv.lock"}, nil, nil},
		{"a path pattern", []string{"a/java/gradle/wrapper/gradle-wrapper.properties"}, nil, []string{"deps java:a"}},
		{"a pin file", []string{"a/go/go.mod"}, nil, []string{"toolchain go:a"}},
		{"a pin file below a component", []string{"a/go/tools/go.mod"}, nil, nil},
		{"a pin file above", []string{".python-version"}, nil, []string{"toolchain python:a,b,c"}},
		{"a support file", []string{".github/workflows/ci.yml"}, nil, []string{"toolchain python:a,b,c"}},
		{"both", []string{"b/go/go.sum", "b/go/go.mod"}, nil, []string{"deps go:b", "toolchain go:b"}},
		{"the config", []string{".hecatron/config.json"}, nil, []string{"check .hecatron/config.json"}},
		{"local settings", []string{".hecatron/local.json"}, nil, nil},
		{"a definition", []string{".hecatron/languages/go/config.json"}, nil,
			[]string{"toolchain go:a,b", "check .hecatron/languages/go/config.json"}},
		{"a generated file", []string{"Dockerfile"}, []string{"Dockerfile"}, []string{"check Dockerfile"}},
		{"a version, generating", []string{"a/go/go.mod"}, []string{"Dockerfile"}, []string{"toolchain go:a", "check a/go/go.mod"}},
		{"nothing relevant", []string{"README.md", "a/go/main.go"}, nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Catch(p, c.files, c.generated)
			if err != nil {
				t.Fatal(err)
			}
			if list := caught(got); !reflect.DeepEqual(list, c.want) || got.Empty() != (len(c.want) == 0) {
				t.Errorf("got %q, want %q", list, c.want)
			}
		})
	}
}

func TestSyncBase(t *testing.T) {
	setup(t)
	fresh := t.TempDir()
	run(t, fresh, "init", "-q", "-b", "main")
	if _, note, err := SyncBase(fresh, ""); err != nil || !strings.HasPrefix(note, "no commit yet") {
		t.Errorf("no commit: note %q, err %v", note, err)
	}
	commit(t, fresh, map[string]string{"x": "1"})
	if _, note, err := SyncBase(fresh, ""); err != nil || !strings.HasPrefix(note, "no previous HEAD") {
		t.Errorf("one commit: note %q, err %v", note, err)
	}
	root := repository(t)
	first := strings.TrimSpace(run(t, root, "rev-parse", "HEAD"))
	commit(t, root, map[string]string{"x": "1"})
	second := strings.TrimSpace(run(t, root, "rev-parse", "HEAD"))
	if base, note, err := SyncBase(root, ""); err != nil || note != "" || base.Ref != "HEAD@{1}" || base.Commit != first {
		t.Errorf("after a commit: %+v %q %v", base, note, err)
	}
	if files, err := Between(root, first, second); err != nil || !reflect.DeepEqual(files, []string{"x"}) {
		t.Errorf("between: %q %v", files, err)
	}
	if n, ancestor, err := Commits(root, first, second); n != 1 || !ancestor || err != nil {
		t.Errorf("commits: %d %v %v", n, ancestor, err)
	}

	// A rebase writes a reflog entry per step; the base is where HEAD was
	// before it started.
	commit(t, root, map[string]string{"y": "1"})
	before := strings.TrimSpace(run(t, root, "rev-parse", "HEAD"))
	run(t, root, "checkout", "-q", "main")
	commit(t, root, map[string]string{"z": "1"})
	run(t, root, "checkout", "-q", "work")
	run(t, root, "rebase", "-q", "main")
	base, note, err := SyncBase(root, "")
	if err != nil || note != "" || base.Commit != before || base.Ref != "HEAD@{4}" {
		t.Errorf("after a rebase: %+v %q %v\n%s", base, note, err, run(t, root, "reflog"))
	}
	if _, ancestor, _ := Commits(root, base.Commit, "HEAD"); ancestor {
		t.Error("the rebased-away tip is an ancestor")
	}

	if base, _, err := SyncBase(root, "main"); err != nil || base.Ref != "main" || base.Source != "named" {
		t.Errorf("a ref: %+v %v", base, err)
	}
	var usage *UsageError
	if _, _, err := SyncBase(root, "nope"); !errors.As(err, &usage) {
		t.Errorf("not a commit: %v", err)
	}
	if _, _, err := SyncBase(t.TempDir(), ""); !errors.As(err, &usage) {
		t.Errorf("not a repository: %v", err)
	}
}
