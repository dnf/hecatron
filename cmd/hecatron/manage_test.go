package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// The project commands (init, add, remove, enable, disable, check,
// dockerfile, workflows) print and write what bin/hecatron (manage.py) does:
// each sequence of commands runs on two copies of a repository, one with
// bin/hecatron and one with the Go binary, and the output, exit status and
// every file in the two copies (but git's own) must be the same.

// commandRepo is a repository and the commands run in it, in order.
type commandRepo struct {
	files    map[string]string
	git      bool
	commands [][]string
	cwd      string // relative to the root
}

const workflowYML = "name: %s\non: push\njobs:\n  test:\n    steps:\n      - run: hct test\n"

func workflowFiles(langs ...string) map[string]string {
	files := map[string]string{}
	for _, lang := range langs {
		files[".hecatron/languages/"+lang+"/workflow.yml"] = strings.ReplaceAll(workflowYML, "%s", lang)
	}
	return files
}

func merge(maps ...map[string]string) map[string]string {
	result := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

var manageCommands = [][]string{
	{"enable"}, {"disable"}, {"disable", "go"}, {"disable", "rust", "go"}, {"enable"}, {"enable", "go"},
	{"disable", "nope"}, {"disable", "zig"}, {"enable", "--all"}, {"enable", "nope"},
	{"check"}, {"dockerfile", "--check"}, {"workflows", "--check"}, {"dockerfile"}, {"workflows"}, {"check"},
	{"add", "zig"}, {"add", "zig", "go", "rs", "zig"}, {"add", "nope"}, {"check"}, {"workflows", "--check"},
	{"remove", "zig", "zig", "python"}, {"remove", "nope"}, {"remove", "lua"}, {"dockerfile", "--check"}, {"check"},
	{"add"}, {"remove"}, {"add", "--x"}, {"enable", "go", "--all", "rust"}, {"enable", "--all", "go"},
	{"check", "x"}, {"dockerfile", "x"}, {"workflows", "--check", "--check"}, {"add", "help"}, {"check", "-h"},
	{"add", "GO"}, {"add", "--", "go"},
}

var commandRepos = map[string]commandRepo{
	"generated files": {files: merge(map[string]string{
		".hecatron/config.json": `{"schema": 1, "project": "gen", "components": ["age", "date"],
			"languages": ["go", "rust", "python", "java", "lua"],
			"generated": {"dockerfile": "docker/Dockerfile", "workflows": ".github/workflows/{lang}.yml"},
			"audit": {"failOn": "bad"}, "unknown": [1, 2.50, {"x": null}], "big": 1e3, "text": "caf\u00e9 \ud83d\ude00"}`,
		".hecatron/local.json": `{"disabled": ["python"], "other": true}`,
		"age/go/go.mod":        "module x\n", "date/go/go.mod": "module y\n",
		"age/rust/Cargo.toml": "", "age/python/pyproject.toml": "", "date/java/pom.xml": "",
		"date/java/.java-version": "17\n", "date/python/.python-version": "3.12\n",
		".hecatron/languages/python/config.json": `{"version": "3.11"}`,
		".hecatron/languages/go/workflow.yml":    "name: go\njobs:\n  t:\n    run: cd age/go && cd date/go\n",
		"justfile":                               "x:\n\ttrue\n",
	}, workflowFiles("rust", "python", "zig")), commands: manageCommands},
	"no generated files": {files: map[string]string{
		".hecatron/config.json": `{"schema": 1, "languages": ["go", "rust"], "lint": {"format": "x"}}`,
		"go/go.mod":             "",
	}, commands: manageCommands},
	"detected": {git: true, files: map[string]string{"go/go.mod": "module x\n", "rust/Cargo.toml": "", ".gitignore": "x\r\n.logs/"},
		commands: [][]string{{"disable", "go"}, {"enable"}, {"add", "python"}, {"check"}, {"disable"}}},
	"each-version": {files: merge(map[string]string{
		".hecatron/config.json": `{"schema": 1, "components": ["a", "b"], "languages": ["kotlin", "java", "python", "node"],
			"generated": {"dockerfile": "Dockerfile"}}`,
		"a/java/.java-version": "17\n", "b/python/.python-version": "3.11\n",
		".hecatron/languages/node/config.json": `{}`,
		".hecatron/languages/node/dockerfile":  "# apt: zzz aaa\n# after: python\n\nRUN echo {version}\n# each-version java\nRUN jdk {version} {exact} {major} {minor} ${X}\n# end-each-version\n\n",
	}), commands: [][]string{{"dockerfile", "--check"}, {"dockerfile"}, {"dockerfile", "--check"}, {"check"}, {"remove", "node"}, {"add", "node"}}},
	"broken snippet": {files: map[string]string{
		".hecatron/config.json":               `{"schema": 1, "languages": ["go"], "generated": {"dockerfile": "Dockerfile", "workflows": "w/{lang}.yml"}}`,
		".hecatron/languages/go/dockerfile":   "# each-version\nRUN x\n",
		".hecatron/languages/go/workflow.yml": "x\n",
	}, commands: [][]string{{"dockerfile"}, {"check"}, {"add", "rust"}, {"workflows"}}},
	"conflicts": {files: map[string]string{
		".hecatron/config.json":                  `{"schema": 1, "components": ["a", "b"], "languages": ["python", "java"]}`,
		".hecatron/languages/python/config.json": `{"version": "3.12"}`,
		".hecatron/languages/java/config.json":   `{"version": "21"}`,
		"a/python/.python-version":               "3.11\n", "b/python/.python-version": "3.12\n",
		"a/java/.java-version": "system\n", "b/java/pom.xml": "",
	}, commands: [][]string{{"check"}}},
	"coverage matrix": {files: merge(map[string]string{
		".hecatron/config.json": `{"schema": 1, "components": ["age", "date"], "languages": ["go", "rust"], "generated": {"workflows": "{lang}.yml"}}`,
		"age/go/go.mod":         "", "date/go/go.mod": "", "age/rust/Cargo.toml": "",
		".hecatron/languages/go/workflow.yml":   "matrix: [age, date]\nrun: cd ${{ matrix.component }}/go\n",
		".hecatron/languages/rust/workflow.yml": "run: cd date/rust\n",
	}), commands: [][]string{{"check"}, {"workflows"}, {"check"}}},
}

// initRepos are repositories init runs in, with these argument lists.
var initArgs = [][]string{
	{"init"}, {"init", "--no-agents"}, {"init", "--justfile", "--no-agents"},
	{"init", "--components", "a,,b", "--languages", "rust,go", "--impl-path=x/{lang}"},
	{"init", "--components="}, {"init", "--force"}, {"init", "--forc"}, {"init", "-h"}, {"init", "a", "b"},
}

var initRepos = map[string]map[string]string{
	"layout":                 {"age/impl/go/go.mod": "", "date/impl/go/go.mod": "", "age/impl/rust/Cargo.toml": ""},
	"single":                 {"go.mod": "module x\n"},
	"nothing":                {"README": ""},
	"agents":                 {"go/go.mod": "", "AGENTS.md": "# Agents\r\nbe nice", "CLAUDE.md": "rules", ".gitignore": "x"},
	"agents again":           {"go/go.mod": "", "AGENTS.md": "a\n<!-- hecatron:begin -->\nold\n<!-- hecatron:end -->\nb\n", "CLAUDE.md": "@AGENTS.md\n"},
	"agents reversed":        {"go/go.mod": "", "AGENTS.md": "<!-- hecatron:end -->é<!-- hecatron:begin -->", "CLAUDE.md": ""},
	"agents at end":          {"go/go.mod": "", "AGENTS.md": "<!-- hecatron:begin -->x<!-- hecatron:end -->", ".gitignore": ".logs/\n.hecatron/local.json\n"},
	"already set up":         {".hecatron/config.json": `{"schema": 1}`, "go/go.mod": ""},
	"unicode":                {"caf\u00e9/go.mod": "", "go/go.mod": "", "z\u00fcrich/Cargo.toml": ""},
	"claude links to agents": {"go/go.mod": "", "AGENTS.md": "# Agents\n", "CLAUDE.md": "->AGENTS.md"},
	"outside the tree":       {"lib/rb/Gemfile": "", "lib/java/pom.xml": "", "go.mod": "", "lib/go/x.go": "", "clr/x.csproj": ""},
}

func TestProjectCommandsMatchBinHecatron(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bin/hecatron needs bash and Python")
	}
	for _, tool := range []string{"bash", "python3", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("no " + tool)
		}
	}
	home := repoHome(t)
	env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=", "HECATRON_NO_GO=1", "HECATRON_AS_OF=off", "COLUMNS=80"}
	compared := 0
	run := func(t *testing.T, repo commandRepo, name string) {
		// Both named alike: init names the project after its directory.
		roots := [2]string{repoFixture(t, repo.files), repoFixture(t, repo.files)}
		for _, root := range roots {
			if repo.git {
				gitInit(t, root)
			}
		}
		for _, args := range repo.commands {
			var results [2]result
			for i, entry := range [][]string{{"bash", filepath.Join(home, "bin", "hecatron")}, {binary}} {
				results[i] = runIn(t, filepath.Join(roots[i], repo.cwd), env, append(entry, args...)...)
				results[i].stdout = strings.ReplaceAll(results[i].stdout, roots[i], "ROOT")
				results[i].stderr = strings.ReplaceAll(results[i].stderr, roots[i], "ROOT")
			}
			if results[0] != results[1] {
				t.Errorf("%s %q:\nbin/hecatron: %+v\nGo:           %+v", name, args, results[0], results[1])
			}
			if diff := compareTrees(t, roots[0], roots[1]); diff != "" {
				t.Errorf("%s %q: the files differ: %s", name, args, diff)
			}
			compared++
		}
	}
	for name, repo := range commandRepos {
		t.Run(name, func(t *testing.T) { run(t, repo, name) })
	}
	for name, files := range initRepos {
		for _, args := range initArgs {
			t.Run(name+"/"+strings.Join(args, " "), func(t *testing.T) {
				run(t, commandRepo{files: files, git: true, commands: [][]string{args, {"init"}}}, name)
			})
		}
	}
	for _, repo := range fixtureRepos(t, home) {
		t.Run(repo.name, func(t *testing.T) {
			run(t, commandRepo{files: repo.files, git: true, commands: [][]string{{"init"}, {"check"}, {"add", "go"}, {"init", "--force", "--no-agents"}}}, repo.name)
		})
	}
	t.Run("outside git", func(t *testing.T) {
		run(t, commandRepo{files: map[string]string{"go.mod": ""}, commands: [][]string{{"init"}, {"init", "nope"}}}, "outside git")
	})
	t.Run("from a subdirectory", func(t *testing.T) {
		run(t, commandRepo{files: map[string]string{"go/go.mod": "", "sub/x": ""}, git: true, cwd: "sub",
			commands: [][]string{{"init", ".."}, {"init", "--force", "../go"}, {"init"}}}, "from a subdirectory")
	})
	t.Logf("%d invocations compared with bin/hecatron, files and all", compared)
}

// repoFixture is fixture's project in a directory named "repo".
func repoFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := filepath.Join(fixture(t, nil), "repo")
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if target, ok := strings.CutPrefix(content, "->"); ok {
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

type namedFiles struct {
	name  string
	files map[string]string
}

// fixtureRepos are the sample projects under tests/hooks/fixtures and
// tests/output/fixtures, as repositories without hecatron's config.
func fixtureRepos(t *testing.T, home string) []namedFiles {
	t.Helper()
	var repos []namedFiles
	for _, base := range []string{"tests/hooks/fixtures/*/*", "tests/output/fixtures/*"} {
		dirs, _ := filepath.Glob(filepath.Join(home, base))
		for _, dir := range dirs {
			files := map[string]string{}
			filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					if d != nil && d.IsDir() && d.Name() == ".hecatron" {
						return filepath.SkipDir
					}
					return err
				}
				data, err := os.ReadFile(path)
				if err == nil {
					rel, _ := filepath.Rel(dir, path)
					files[filepath.ToSlash(rel)] = string(data)
				}
				return nil
			})
			rel, _ := filepath.Rel(home, dir)
			repos = append(repos, namedFiles{rel, files})
		}
	}
	return repos
}

func gitInit(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// compareTrees describes how two directories' files differ, but git's own
// (.git, except hecatron's files in it), and the detection cache's key,
// which holds the index's time; "" when they don't.
func compareTrees(t *testing.T, a, b string) string {
	t.Helper()
	read := func(root string) map[string]string {
		files := map[string]string{}
		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			if d.IsDir() {
				if rel == ".git" {
					inner := filepath.Join(path, "hecatron")
					for name, content := range readTree(inner) {
						files[".git/hecatron/"+name] = content
					}
					return filepath.SkipDir
				}
				files[rel+"/"] = ""
				return nil
			}
			data, _ := os.ReadFile(path)
			files[rel] = strings.ReplaceAll(string(data), root, "ROOT")
			return nil
		})
		if cache, ok := files[".git/hecatron/detected.json"]; ok {
			if i := strings.Index(cache, `"config"`); i >= 0 {
				files[".git/hecatron/detected.json"] = cache[i:]
			}
		}
		return files
	}
	filesA, filesB := read(a), read(b)
	var names []string
	for name := range filesA {
		names = append(names, name)
	}
	for name := range filesB {
		if _, ok := filesA[name]; !ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var diff bytes.Buffer
	for _, name := range names {
		contentA, inA := filesA[name]
		contentB, inB := filesB[name]
		switch {
		case !inA:
			diff.WriteString("\n only Go wrote " + name)
		case !inB:
			diff.WriteString("\n only bin/hecatron wrote " + name)
		case contentA != contentB:
			diff.WriteString("\n " + name + ":\n--- bin/hecatron\n" + contentA + "\n--- Go\n" + contentB)
		}
	}
	return diff.String()
}

func readTree(root string) map[string]string {
	files := map[string]string{}
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		data, _ := os.ReadFile(path)
		files[rel] = string(data)
		return nil
	})
	return files
}

// init's section for AGENTS.md is manage.py's AGENTS_SECTION, word for word.
func TestAgentsSectionMatchesManagePy(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	home := repoHome(t)
	cmd := exec.Command("python3", "-c", "import sys; sys.path.insert(0, sys.argv[1]); import manage; sys.stdout.write(manage.AGENTS_SECTION)",
		filepath.Join(home, "libexec", "commands"))
	cmd.Env = append(os.Environ(), "HECATRON_HOME="+home)
	want, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if agentsSection != string(want) {
		t.Errorf("manage.py:\n%s\nGo:\n%s", want, agentsSection)
	}
}
