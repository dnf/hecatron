package advisor

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dnf/hecatron/internal/project"
)

var update = flag.Bool("update", false, "rewrite the golden files")

var now = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func repoHome(t *testing.T) project.Home {
	t.Helper()
	home, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return project.NewHome(home)
}

func requireTools(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fixtures use git and POSIX paths")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
}

func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// repo writes files, commits them, then writes the untracked ones.
func repo(t *testing.T, files, untracked map[string]string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, files)
	git(t, root, "init", "-q")
	git(t, root, "add", "-A")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-qm", "x", "--allow-empty")
	write(t, root, untracked)
	return root
}

// fixtureCase is one case of testdata/rules.json: files, or {"files",
// "untracked"}; "$dir" is the directory the rule is evaluated in.
type fixtureCase struct {
	files, untracked map[string]string
	dir              string
}

func parseCase(t *testing.T, raw json.RawMessage) fixtureCase {
	t.Helper()
	var structured struct {
		Files     map[string]string `json:"files"`
		Untracked map[string]string `json:"untracked"`
	}
	c := fixtureCase{}
	if json.Unmarshal(raw, &structured) == nil && structured.Files != nil {
		c.files, c.untracked = structured.Files, structured.Untracked
	} else if err := json.Unmarshal(raw, &c.files); err != nil {
		t.Fatal(err)
	}
	c.dir = c.files["$dir"]
	delete(c.files, "$dir")
	return c
}

// allRules is every data rule of every definition, by id.
func allRules(t *testing.T) (*Library, map[string]*Rule) {
	t.Helper()
	p := &project.Project{Root: t.TempDir(), Home: repoHome(t)}
	p.Dir = filepath.Join(p.Root, ".hecatron")
	defs, err := p.Definitions()
	if err != nil {
		t.Fatal(err)
	}
	lib, err := Load(p, defs.Names)
	if err != nil {
		t.Fatal(err)
	}
	rules := map[string]*Rule{}
	collect := func(set *Set) {
		if set == nil {
			return
		}
		for i := range set.Rules {
			rules[set.name+"/"+set.Rules[i].ID] = &set.Rules[i]
		}
	}
	for _, set := range lib.Languages {
		collect(set)
	}
	for _, set := range lib.Shared {
		collect(set)
	}
	collect(lib.Repo)
	return lib, rules
}

// TestDataRules: every rule in the definitions' advice has a fixture that
// triggers it and one that doesn't, in testdata/rules.json.
func TestDataRules(t *testing.T) {
	requireTools(t)
	_, rules := allRules(t)
	data, err := os.ReadFile("testdata/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw, "$comment")
	fixtures := map[string]map[string]json.RawMessage{}
	for id, cases := range raw {
		var parsed map[string]json.RawMessage
		if err := json.Unmarshal(cases, &parsed); err != nil {
			t.Fatal(id, err)
		}
		fixtures[id] = parsed
	}
	for id := range rules {
		cases := fixtures[id]
		hasTrigger, hasPass := false, false
		for name := range cases {
			hasTrigger = hasTrigger || strings.HasPrefix(name, "trigger")
			hasPass = hasPass || strings.HasPrefix(name, "pass")
		}
		if !hasTrigger || !hasPass {
			t.Errorf("%s: testdata/rules.json needs a trigger and a pass fixture", id)
		}
	}
	for id, cases := range fixtures {
		rule, ok := rules[id]
		if !ok {
			t.Errorf("testdata/rules.json: no rule %s", id)
			continue
		}
		for name, raw := range cases {
			t.Run(id+"/"+name, func(t *testing.T) {
				c := parseCase(t, raw)
				root := repo(t, c.files, c.untracked)
				f := newFiles(root, gitTracked(root))
				dir := filepath.Join(root, filepath.FromSlash(c.dir))
				if rule.Scope == "root" {
					dir = root
				}
				fires := f.eval(rule.When, dir) && !f.eval(rule.Expect, dir)
				if want := strings.HasPrefix(name, "trigger"); fires != want {
					t.Errorf("fires %v, want %v", fires, want)
				}
			})
		}
	}
}

// TestLibrary: the definitions' advice loads, and its links are https.
func TestLibrary(t *testing.T) {
	lib, rules := allRules(t)
	if len(rules) < 25 {
		t.Errorf("only %d data rules", len(rules))
	}
	ids := lib.IDs()
	doc, err := os.ReadFile("../../docs/guide/advisor.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if !strings.Contains(string(doc), "`"+id+"`") {
			t.Errorf("docs/guide/advisor.md doesn't list %s", id)
		}
	}
	for _, want := range []string{"go/sum-committed", "node/toolchain-pinned", "python/toolchain-end-of-life", "rust/toolchain-behind", "node/lock-matches-manifest", "ci/configured", "audit/failed"} {
		if !knownID(ids, want) {
			t.Errorf("no rule %s in %v", want, ids)
		}
	}
}

func TestLoadRejects(t *testing.T) {
	cases := map[string]string{
		"unknown key":    `{"rules": [], "bogus": 1}`,
		"bad level":      `{"rules": [{"id": "x", "category": "tooling", "level": "must", "expect": {"file": "x"}, "message": "m", "why": "w", "link": "https://x"}]}`,
		"bad category":   `{"rules": [{"id": "x", "category": "style", "level": "suggestion", "expect": {"file": "x"}, "message": "m", "why": "w", "link": "https://x"}]}`,
		"no link":        `{"rules": [{"id": "x", "category": "tooling", "level": "suggestion", "expect": {"file": "x"}, "message": "m", "why": "w", "link": "http://x"}]}`,
		"two tests":      `{"rules": [{"id": "x", "category": "tooling", "level": "suggestion", "expect": {"file": "x", "missing": "y"}, "message": "m", "why": "w", "link": "https://x"}]}`,
		"bad regex":      `{"rules": [{"id": "x", "category": "tooling", "level": "suggestion", "expect": {"contains": ["x", "("]}, "message": "m", "why": "w", "link": "https://x"}]}`,
		"unknown set":    `{"include": ["nope"]}`,
		"glob above":     `{"rules": [{"id": "x", "category": "tooling", "level": "suggestion", "expect": {"file": "../x"}, "message": "m", "why": "w", "link": "https://x"}]}`,
		"no expect":      `{"rules": [{"id": "x", "category": "tooling", "level": "suggestion", "message": "m", "why": "w", "link": "https://x"}]}`,
		"duplicate rule": `{"rules": [{"id": "x", "category": "tooling", "level": "suggestion", "expect": {"file": "x"}, "message": "m", "why": "w", "link": "https://x"}, {"id": "x", "category": "tooling", "level": "suggestion", "expect": {"file": "x"}, "message": "m", "why": "w", "link": "https://x"}]}`,
	}
	for name, advice := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, map[string]string{".hecatron/languages/go/advice.json": advice})
			p := &project.Project{Root: root, Dir: filepath.Join(root, ".hecatron"), Home: repoHome(t)}
			if _, err := Load(p, []string{"go"}); err == nil {
				t.Error("loaded")
			}
		})
	}
}

// project is a fixture repository read as the CLI reads it.
func loadProject(t *testing.T, root string) *project.Project {
	t.Helper()
	p, err := project.Load(root, repoHome(t))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// goldenRepo exercises the rules written in Go: an end-of-life Go and
// Python, a Rust far behind, a stale npm lock, a missing pin with its fix,
// a language CI doesn't run, an audit that failed and an acceptance past
// its date, and an ignored rule.
var goldenRepo = map[string]string{
	".hecatron/config.json": `{"schema": 1, "languages": ["go", "python", "rust", "typescript"],
  "audit": {"ignore": [{"id": "GHSA-1", "reason": "not reachable", "until": "2026-01-01"}, {"id": "GHSA-2", "reason": "later", "until": "2027-01-01"}]},
  "advisor": {"ignore": [{"id": "rust/edition", "reason": "set by the workspace generator"}]}}`,
	".gitignore":                   ".logs/\n",
	".github/workflows/ci.yml":     "jobs:\n  t:\n    steps:\n      - uses: actions/checkout@v4\n      - uses: someone/setup@main\n      - run: npm install\n      - run: go test ./...\n      - run: cargo test\n",
	"go/go.mod":                    "module x\n\ngo 1.20\n\ntoolchain go1.20.1\n\nrequire example.com/y v1.0.0\n",
	"go/go.sum":                    "",
	"python/setup.py":              "from setuptools import setup\nsetup()\n",
	"python/.python-version":       "3.8\n",
	"python/ruff.toml":             "",
	"rust/Cargo.toml":              "[package]\nname = \"x\"\n",
	"rust/src/main.rs":             "fn main() {}\n",
	"rust/Cargo.lock":              "",
	"rust/rust-toolchain.toml":     "[toolchain]\nchannel = \"1.80.0\"\n",
	"typescript/package.json":      `{"devDependencies": {"typescript": "^5.9.0", "eslint": "^9"}}`,
	"typescript/package-lock.json": `{"lockfileVersion": 3, "packages": {"": {"devDependencies": {"typescript": "^5.8.0", "eslint": "^9"}}}}`,
	"typescript/.prettierrc":       "{}",
	"typescript/tsconfig.json":     `{"compilerOptions": {"strict": true}}`,
	".logs/audit/go.log":           "go: audit failed: .\n",
}

func TestGolden(t *testing.T) {
	requireTools(t)
	t.Setenv("HECATRON_AS_OF", "off")
	t.Setenv("HECATRON_BUILTIN_LANGUAGES", "")
	root := repo(t, goldenRepo, nil)
	when := now.Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(root, ".logs/audit/go.log"), when, when); err != nil {
		t.Fatal(err)
	}
	report, err := Advise(loadProject(t, root), Options{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"agent", "human", "json"} {
		var out bytes.Buffer
		if err := Write(&out, report, format); err != nil {
			t.Fatal(err)
		}
		checkGolden(t, "golden."+format, strings.ReplaceAll(out.String(), root, "/repo"))
	}
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	golden := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("%s:\n got:\n%s\nwant:\n%s", golden, got, want)
	}
}

func ids(findings []Finding) []string {
	var result []string
	for _, f := range findings {
		result = append(result, f.ID+"@"+f.Dir)
	}
	sort.Strings(result)
	return result
}

func has(list []string, item string) bool {
	for _, entry := range list {
		if entry == item {
			return true
		}
	}
	return false
}

// TestGoRules: each rule written in Go, triggered and not.
func TestGoRules(t *testing.T) {
	requireTools(t)
	t.Setenv("HECATRON_AS_OF", "off")
	t.Setenv("HECATRON_BUILTIN_LANGUAGES", "")
	config := func(languages string) string {
		return `{"schema": 1, "languages": [` + languages + `]}`
	}
	workflow := map[string]string{".github/workflows/ci.yml": "steps:\n  - run: go test ./...\n"}
	with := func(base map[string]string, extra map[string]string) map[string]string {
		files := map[string]string{}
		for k, v := range base {
			files[k] = v
		}
		for k, v := range extra {
			files[k] = v
		}
		return files
	}
	goRepo := with(workflow, map[string]string{".hecatron/config.json": config(`"go"`), "go/go.mod": "module x\n"})
	pyRepo := with(workflow, map[string]string{".hecatron/config.json": config(`"python"`), "python/pyproject.toml": "[tool.ruff]\n", ".github/workflows/py.yml": "- run: pytest\n"})
	cases := []struct {
		name  string
		files map[string]string
		want  string // id@dir that must be found
		fires bool
	}{
		{"end of life", with(goRepo, map[string]string{"go/go.mod": "module x\n\ntoolchain go1.25.0\n"}), "go/toolchain-end-of-life@go", true},
		{"supported", with(goRepo, map[string]string{"go/go.mod": "module x\n\ntoolchain go1.26.0\n"}), "go/toolchain-end-of-life@go", false},
		{"default never judged", goRepo, "go/toolchain-end-of-life@go", false},
		{"python past five years", with(pyRepo, map[string]string{"python/.python-version": "3.9\n"}), "python/toolchain-end-of-life@python", true},
		{"python within", with(pyRepo, map[string]string{"python/.python-version": "3.12\n"}), "python/toolchain-end-of-life@python", false},
		{"pin missing", pyRepo, "python/toolchain-pinned@python", true},
		{"pinned", with(pyRepo, map[string]string{"python/.python-version": "3.12\n"}), "python/toolchain-pinned@python", false},
		{"pinned above", with(pyRepo, map[string]string{".python-version": "3.12\n"}), "python/toolchain-pinned@python", false},
		{"ci covers", goRepo, "go/ci-covers@.", false},
		{"ci misses", with(goRepo, map[string]string{".github/workflows/ci.yml": "steps:\n  - run: make\n"}), "go/ci-covers@.", true},
		{"ci through hct", with(goRepo, map[string]string{".github/workflows/ci.yml": "steps:\n  - run: hct test go python\n"}), "go/ci-covers@.", false},
		{"ci through hct, others", with(goRepo, map[string]string{".github/workflows/ci.yml": "steps:\n  - run: hct test python\n"}), "go/ci-covers@.", true},
		{"ci reusable workflow", with(goRepo, map[string]string{".github/workflows/ci.yml": "jobs:\n  t:\n    uses: org/shared/.github/workflows/test.yml@v1\n"}), "go/ci-covers@.", false},
		{"no ci", map[string]string{".hecatron/config.json": config(`"go"`), "go/go.mod": "module x\n"}, "ci/configured@.", true},
		{"ci", goRepo, "ci/configured@.", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := repo(t, c.files, nil)
			report, err := Advise(loadProject(t, root), Options{Now: now})
			if err != nil {
				t.Fatal(err)
			}
			if got := ids(report.Findings); has(got, c.want) != c.fires {
				t.Errorf("%s fires %v, want %v: %v", c.want, !c.fires, c.fires, got)
			}
		})
	}
}

func rel(version, date string, lts bool) project.Release {
	return project.Release{Version: version, Date: date, LTS: lts}
}

func TestEndOfLifePolicies(t *testing.T) {
	releases := []project.Release{
		rel("18.0.0", "2022-04-19", false), rel("20.0.0", "2023-04-18", false), rel("20.9.0", "2023-10-24", true), rel("21.0.0", "2023-10-17", false),
		rel("22.0.0", "2024-04-24", false), rel("22.11.0", "2024-10-29", true), rel("23.0.0", "2024-10-16", false),
	}
	node := &EndOfLife{LineParts: 1, Months: 8, LTSMonths: 37}
	cases := []struct {
		version string
		when    string
		past    bool
	}{
		{"20.10.0", "2026-04-01", false}, // LTS: 37 months from April 2023
		{"20", "2026-06-01", true},
		{"21.1.0", "2024-05-01", false}, // not LTS: eight months
		{"21.1.0", "2024-07-01", true},
		{"19.0.0", "2030-01-01", false}, // not in the table: not judged
	}
	for _, c := range cases {
		when, _ := time.Parse("2006-01-02", c.when)
		if past, _ := endOfLife(node, releases, c.version, when); past != c.past {
			t.Errorf("%s on %s: past %v", c.version, c.when, past)
		}
	}
	golang := &EndOfLife{LineParts: 2, Lines: 2}
	goReleases := []project.Release{rel("1.21.0", "2023-08-08", false), rel("1.22.0", "2024-02-06", false), rel("1.23.0", "2024-08-13", false)}
	if past, newest := endOfLife(golang, goReleases, "1.21.13", now); !past || newest != "1.23" {
		t.Errorf("go 1.21: %v %s", past, newest)
	}
	if past, _ := endOfLife(golang, goReleases, "1.22", now); past {
		t.Error("go 1.22 judged past")
	}
	if behind, newest := linesBehind(goReleases, "1.21.3", 2); behind != 2 || newest != "1.23" {
		t.Errorf("behind %d %s", behind, newest)
	}
}

func TestNpmLockStale(t *testing.T) {
	manifest := `{"dependencies": {"a": "^1"}, "devDependencies": {"b": "^2"}}`
	cases := []struct {
		lock, member string
		stale, ok    bool
	}{
		{`{"lockfileVersion": 3, "packages": {"": {"dependencies": {"a": "^1"}, "devDependencies": {"b": "^2"}}}}`, "", false, true},
		{`{"lockfileVersion": 3, "packages": {"": {"dependencies": {"a": "^1"}}}}`, "", true, true},
		{`{"lockfileVersion": 2, "packages": {"pkg/x": {"dependencies": {"a": "^1"}, "devDependencies": {"b": "^2"}}}}`, "pkg/x", false, true},
		{`{"lockfileVersion": 3, "packages": {"": {}}}`, "elsewhere", false, false},
		{`{"lockfileVersion": 1, "dependencies": {}}`, "", false, false},
		{`not json`, "", false, false},
	}
	for i, c := range cases {
		if stale, ok := npmLockStale([]byte(manifest), []byte(c.lock), c.member); stale != c.stale || ok != c.ok {
			t.Errorf("%d: stale %v ok %v", i, stale, ok)
		}
	}
}

// TestFix: --fix creates the pin files at the version in use, lists them,
// and never overwrites a file.
func TestFix(t *testing.T) {
	requireTools(t)
	t.Setenv("HECATRON_AS_OF", "off")
	t.Setenv("HECATRON_BUILTIN_LANGUAGES", "")
	root := repo(t, map[string]string{
		".hecatron/config.json":    `{"schema": 1, "languages": ["csharp", "python", "rust", "typescript"]}`,
		"python/pyproject.toml":    "[project]\nname = \"x\"\n",
		"typescript/package.json":  "{}",
		"rust/Cargo.toml":          "[package]\nname = \"x\"\nedition = \"2024\"\n",
		"rust/src/main.rs":         "fn main() {}\n",
		"dotnet/App.csproj":        "<Project/>",
		".github/workflows/ci.yml": "steps:\n  - run: dotnet test\n",
	}, nil)
	p := loadProject(t, root)
	report, err := Advise(p, Options{Now: now, Fix: true})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(report.Fixed)
	want := []string{"dotnet/global.json", "python/.python-version", "rust/rust-toolchain.toml", "typescript/.nvmrc"}
	if strings.Join(report.Fixed, " ") != strings.Join(want, " ") {
		t.Fatalf("fixed %v, want %v", report.Fixed, want)
	}
	defaults := map[string]string{}
	for _, lang := range []string{"csharp", "python", "rust", "typescript"} {
		v, err := p.DefaultVersion(lang)
		if err != nil {
			t.Fatal(err)
		}
		defaults[lang] = strings.TrimPrefix(v, "=")
	}
	contents := map[string]string{
		"python/.python-version":   line(defaults["python"], 2) + "\n",
		"typescript/.nvmrc":        line(defaults["typescript"], 1) + "\n",
		"rust/rust-toolchain.toml": "[toolchain]\nchannel = \"" + defaults["rust"] + "\"\n",
		"dotnet/global.json":       "{\n  \"sdk\": {\n    \"version\": \"" + defaults["csharp"] + "\",\n    \"rollForward\": \"latestFeature\"\n  }\n}\n",
	}
	for file, content := range contents {
		got, err := os.ReadFile(filepath.Join(root, file))
		if err != nil || string(got) != content {
			t.Errorf("%s: %q, want %q (%v)", file, got, content, err)
		}
	}
	// Run again: everything is pinned now, so nothing to fix, and a pin
	// file another rule wanted isn't overwritten.
	again, err := Advise(loadProject(t, root), Options{Now: now, Fix: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Fixed) != 0 {
		t.Errorf("fixed again: %v", again.Fixed)
	}
	for _, f := range again.Findings {
		if strings.HasSuffix(f.ID, "/"+idPinned) {
			t.Errorf("still unpinned: %+v", f)
		}
	}
	// A pin file that exists (and isn't read as a pin) is left alone.
	fixes, err := applyFixes(root, []Finding{{ID: "x", Fix: &Fix{Create: "python/.python-version", Content: "9.9\n"}}})
	if err != nil || len(fixes) != 0 {
		t.Errorf("overwrote: %v %v", fixes, err)
	}
}

// TestIgnore: an ignore entry silences a rule by id, by name for every
// set, or in one directory; hct check's problems for bad entries.
func TestIgnore(t *testing.T) {
	f := Finding{ID: "python/toolchain-pinned", Dir: "python"}
	cases := []struct {
		list ignoreList
		hit  bool
	}{
		{ignoreList{{"python/toolchain-pinned", "", "r"}}, true},
		{ignoreList{{"toolchain-pinned", "", "r"}}, true},
		{ignoreList{{"toolchain-pinned", "python", "r"}}, true},
		{ignoreList{{"toolchain-pinned", "other", "r"}}, false},
		{ignoreList{{"node/toolchain-pinned", "", "r"}}, false},
		{ignoreList{{"pinned", "", "r"}}, false},
	}
	for i, c := range cases {
		if _, hit := c.list.match(f); hit != c.hit {
			t.Errorf("%d: hit %v", i, hit)
		}
	}
	requireTools(t)
	root := repo(t, map[string]string{".hecatron/config.json": `{"schema": 1, "languages": ["go"], "advisor": {"ignore": [
	  {"id": "go/sum-committed", "reason": "vendored"},
	  {"id": "toolchain-pinned", "reason": "r", "path": "go"},
	  {"id": "go/nonsense", "reason": "r"},
	  {"reason": "no id"},
	  {"id": "ci/configured", "reason": " "},
	  {"id": "audit/failed", "reason": "r", "until": "2027-01-01"}
	]}}`}, nil)
	problems, err := ConfigProblems(loadProject(t, root))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"advisor.ignore go/nonsense: no such rule (hct advisor --output json lists the ids it reports)",
		`advisor.ignore[3] needs an "id"`,
		`advisor.ignore ci/configured needs a "reason"`,
		`advisor.ignore audit/failed: unknown key "until" (known: id, reason, path)`,
	}
	if strings.Join(problems, "\n") != strings.Join(want, "\n") {
		t.Errorf("problems:\n%s\nwant:\n%s", strings.Join(problems, "\n"), strings.Join(want, "\n"))
	}
	for config, want := range map[string]string{
		`{"schema": 1, "advisor": []}`:              `"advisor" must be an object`,
		`{"schema": 1, "advisor": {"ignore": {}}}`:  "advisor.ignore must be a list",
		`{"schema": 1, "advisor": {"silence": []}}`: `advisor: unknown key "silence" (known: ignore)`,
	} {
		root := repo(t, map[string]string{".hecatron/config.json": config}, nil)
		problems, err := ConfigProblems(loadProject(t, root))
		if err != nil || len(problems) != 1 || problems[0] != want {
			t.Errorf("%s: %v %v", config, problems, err)
		}
	}
	root = repo(t, map[string]string{".hecatron/config.json": `{"schema": 1}`}, nil)
	if problems, err := ConfigProblems(loadProject(t, root)); err != nil || len(problems) != 0 {
		t.Errorf("no advisor key: %v %v", problems, err)
	}
}
