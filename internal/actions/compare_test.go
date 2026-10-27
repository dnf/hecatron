package actions

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The declarative definitions must plan what the shell hooks they replace
// run. Each case is a project; the shell hook runs in it with every tool it
// calls replaced by a stub that does nothing, and its "+ command" lines,
// skips and errors are compared with the plan's.

var stubbed = []string{"go", "golangci-lint", "cargo", "uv", "uvx", "mix", "mvn", "gradle", "sbt", "java"}

var allActions = []string{"test", "lint", "build", "clean"}

type project struct {
	name      string
	lang      string
	fixture   string            // tests/hooks/fixtures/<lang>/<fixture>, else files
	files     map[string]string // path → content; "+x" content makes it executable
	component string            // the language's directory in the project
}

func projects() []project {
	cases := []project{
		{name: "go module", lang: "go", fixture: "module"},
		{name: "go module above", lang: "go", fixture: "module-above"},
		{name: "rust cargo", lang: "rust", fixture: "cargo"},
		{name: "rust crates below", lang: "rust", fixture: "crates-below"},
		{name: "python uv", lang: "python", fixture: "uv"},
		{name: "elixir mix", lang: "elixir", fixture: "mix"},
		{name: "java maven", lang: "java", fixture: "maven"},
		{name: "java gradle", lang: "java", fixture: "gradle"},
		{name: "java gradle above", lang: "java", fixture: "gradle-above"},
		{name: "python scripts only", lang: "python", files: map[string]string{"add.py": "", "test_add.py": ""}},
		{name: "elixir locked, deps not fetched, credo", lang: "elixir", files: map[string]string{
			"mix.exs": "defp deps, do: [{:credo, \"~> 1.7\"}]", "mix.lock": ""}},
		{name: "elixir nothing to lint", lang: "elixir", files: map[string]string{"mix.exs": "", "mix.lock": "", "deps/x": ""}},
		{name: "maven wrapper", lang: "java", files: map[string]string{"pom.xml": "", "mvnw": "+x"}},
		{name: "gradle wrapper above", lang: "java", component: "lib/java", files: map[string]string{
			"settings.gradle": "", "gradlew": "+x", "lib/java/build.gradle": ""}},
		{name: "sbt", lang: "java", files: map[string]string{"build.sbt": ""}},
	}
	for _, lang := range []string{"go", "rust", "elixir", "java"} {
		cases = append(cases, project{name: lang + " nothing", lang: lang, files: map[string]string{"README": ""}})
	}
	return cases
}

func TestPlansMatchShellHooks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the shell hooks need bash")
	}
	home, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	stubs := t.TempDir()
	for _, tool := range stubbed {
		if err := os.WriteFile(filepath.Join(stubs, tool), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range projects() {
		def, err := Load(filepath.Join(home, "languages"), p.lang)
		if err != nil {
			t.Fatal(err)
		}
		for _, action := range allActions {
			t.Run(p.name+"/"+action, func(t *testing.T) {
				root, dir := p.create(t, home)
				shell := shellLines(t, home, stubs, p.lang, action, root, dir)
				ctx := Context{Action: action, Dir: dir, Root: root}
				ops, err := Plan(def, ctx)
				if err != nil {
					t.Fatal(err)
				}
				planned := comparable(Lines(def, ctx, ops))
				if strings.Join(planned, "\n") != strings.Join(shell, "\n") {
					t.Errorf("plan:\n  %s\nshell hook:\n  %s", strings.Join(planned, "\n  "), strings.Join(shell, "\n  "))
				}
			})
		}
	}
}

// create makes the project in a new directory; root and the component's
// directory, with symlinks resolved, as the shell hooks see them.
func (p project) create(t *testing.T, home string) (root, dir string) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	component := p.component
	if p.fixture != "" {
		fixture := filepath.Join(home, "tests", "hooks", "fixtures", p.lang, p.fixture)
		if implPath, err := os.ReadFile(filepath.Join(fixture, "impl-path")); err == nil {
			copyTree(t, fixture, root)
			component = strings.ReplaceAll(strings.TrimSpace(string(implPath)), "{lang}", p.lang)
		} else {
			component = p.lang
			copyTree(t, fixture, filepath.Join(root, component))
		}
	} else {
		// files are relative to the project when the case names its
		// component, else to the component, the language's directory.
		base := root
		if component == "" {
			component = p.lang
			base = filepath.Join(root, component)
		}
		for path, content := range p.files {
			full := filepath.Join(base, path)
			mode := os.FileMode(0o644)
			if content == "+x" {
				mode, content = 0o755, "#!/bin/sh\n"
			}
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(content), mode); err != nil {
				t.Fatal(err)
			}
		}
	}
	dir = filepath.Join(root, component)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, dir
}

func copyTree(t *testing.T, from, to string) {
	if err := os.MkdirAll(to, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cp", "-R", from+"/.", to+"/").CombinedOutput(); err != nil {
		t.Fatalf("cp: %v: %s", err, out)
	}
}

func shellLines(t *testing.T, home, stubs, lang, action, root, dir string) []string {
	cmd := exec.Command("bash", filepath.Join(home, "languages", lang, "hooks", action+".sh"))
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+stubs+":"+os.Getenv("PATH"), "HECATRON_LANG="+lang, "HECATRON_ROOT="+root, "HECATRON_HOME=")
	out, _ := cmd.CombinedOutput()
	return comparable(strings.Split(strings.TrimRight(string(out), "\n"), "\n"))
}

// comparable keeps the lines both sides print: commands, skips, errors and
// notes, without the shell hooks' advice on overriding them. Removing files
// is a builtin in the plan and rm and find in the hooks, so both become
// "remove".
func comparable(lines []string) []string {
	var kept []string
	for _, line := range lines {
		switch {
		case line == "", strings.HasPrefix(line, "Write .hecatron/"), strings.HasPrefix(line, "error: ") && strings.Contains(line, " failed in "):
		case strings.HasPrefix(line, "+ rm -rf "), strings.HasPrefix(line, "remove "):
			kept = append(kept, "remove")
		default:
			kept = append(kept, line)
		}
	}
	return kept
}
