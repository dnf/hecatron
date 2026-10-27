package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The Go model must answer as project.py does. Each case is a project; a
// Python helper (testdata/dump.py) prints what project.py makes of it, and
// the Go model's answers are compared with that. Where the Go model says
// "unsupported" the comparison is skipped (the CLI hands those projects to
// project.py), but the Go model must never answer where project.py fails.

// skipped marks a value the Go model declined to compute.
type skipped struct{ reason string }

func TestModelMatchesProjectPy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("compares with project.py, which the tests run on macOS and Linux")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	home := repoHome(t)
	answered, declined := 0, 0
	for _, c := range append(parityCases(), versionCases()...) {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("HECATRON_AS_OF", "off") // a fixture's commit is today's, and the date is the case's
			for name, value := range c.env {
				t.Setenv(name, value)
			}
			if c.versions {
				t.Setenv("HECATRON_DUMP_VERSIONS", "1")
			}
			root := c.create(t)
			builtin := ""
			if c.builtin != nil {
				builtin = filepath.Join(t.TempDir(), "languages")
				writeFiles(t, builtin, c.builtin)
			}
			cwd := filepath.Join(root, c.cwd)
			langs := c.langs
			if langs == nil {
				langs = defaultLangs
			}
			want := pythonDump(t, home, builtin, cwd, langs)
			got := goDump(t, home, builtin, cwd, langs)
			n, skips := compareDump(t, "", want, got)
			answered += n
			declined += skips
		})
	}
	t.Logf("%d values compared with project.py, %d declined as unsupported", answered, declined)
}

var defaultLangs = []string{"go", "rust", "csharp", "typescript", "javascript", "python", "java", "mylang", "nope"}

func repoHome(t *testing.T) string {
	t.Helper()
	home, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return home
}

func pythonDump(t *testing.T, home, builtin, cwd string, langs []string) any {
	t.Helper()
	cmd := exec.Command("python3", append([]string{filepath.Join(home, "internal", "project", "testdata", "dump.py")}, langs...)...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "HECATRON_HOME="+home, "HECATRON_BUILTIN_LANGUAGES="+builtin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("dump.py: %v\n%s", err, stderr.String())
	}
	value, err := decodeJSON(out)
	if err != nil {
		t.Fatalf("dump.py output: %v\n%s", err, out)
	}
	return normalize(value)
}

// goDump is dump.py's output from the Go model.
func goDump(t *testing.T, home, builtin, cwd string, langs []string) any {
	t.Helper()
	t.Setenv("HECATRON_BUILTIN_LANGUAGES", builtin)
	root, err := FindRoot(cwd)
	var p *Project
	if err == nil {
		p, err = Load(root, NewHome(home))
	} else if top := gitOutput(cwd, "rev-parse", "--show-toplevel"); top != "" {
		// After dump.py, which detected the project and cached it.
		p, err = LoadDetected(top, NewHome(home))
	}
	if err != nil {
		return loadFailure(err)
	}
	out := map[string]any{
		"root":       p.Root,
		"name":       p.Name,
		"components": strs(p.Components),
		"implPath":   p.ImplPath,
		"languages":  strs(p.Languages),
		"disabled":   strs(p.Disabled),
		"logs":       p.Logs,
	}
	out["generated"] = answer(func() (any, error) {
		result := map[string]any{}
		for _, key := range []string{"dockerfile", "workflows"} {
			value, err := p.Generated(key)
			if err != nil {
				return nil, err
			}
			result[key] = value
		}
		return result, nil
	})
	out["tasks"] = answer(func() (any, error) {
		result := map[string]any{}
		for _, action := range []string{"test", "lint", "build", "clean", "audit"} {
			tasks, err := p.Tasks(action)
			if err != nil {
				return nil, err
			}
			list := []any{}
			for _, task := range tasks {
				list = append(list, []any{task.Name, task.Command})
			}
			result[action] = list
		}
		return result, nil
	})
	out["tools"] = answer(func() (any, error) {
		tools, err := p.Tools()
		return strMap(tools), err
	})
	out["definitions"] = answer(func() (any, error) {
		defs, err := p.Definitions()
		if err != nil {
			return nil, err
		}
		result := &Object{Values: map[string]any{}}
		for _, name := range defs.Names {
			result.set(name, defs.Get(name))
		}
		return normalize(result), nil
	})
	out["shell"] = answer(func() (any, error) {
		s, err := p.Shell()
		if err != nil {
			return nil, err
		}
		exports := []any{}
		for _, pair := range s.Exports {
			exports = append(exports, []any{pair[0], pair[1]})
		}
		hooks := map[string]any{}
		for _, hook := range Hooks {
			hooks[hook] = strs(s.HookLanguages(hook))
		}
		return map[string]any{"exports": exports, "known": strs(s.Known), "added": strs(s.Added),
			"hooks": hooks, "abbreviations": strMap(s.Abbreviations), "lintFormat": strs(s.LintFormat)}, nil
	})
	out["configProblems"] = answer(func() (any, error) {
		problems, err := p.ConfigProblems()
		return strs(problems), err
	})
	out["asOf"] = answer(func() (any, error) { return p.AsOf() })
	out["conflicts"] = answer(func() (any, error) {
		conflicts, err := p.VersionConflicts()
		list := []any{}
		for _, c := range conflicts {
			list = append(list, []any{c.Lang, c.Component, c.Source, c.PinSpec, c.ProjectSpec})
		}
		return list, err
	})
	languages := map[string]any{}
	for _, lang := range langs {
		language := goLanguage(p, lang)
		if os.Getenv("HECATRON_DUMP_VERSIONS") != "" {
			for key, value := range goVersions(p, lang) {
				language[key] = value
			}
		}
		languages[lang] = language
	}
	out["lang"] = languages
	return out
}

// goVersions are dump.py's version fields from the Go model.
func goVersions(p *Project, lang string) map[string]any {
	versions := map[string]any{}
	for _, component := range append(append([]string(nil), p.Components...), "") {
		key := component
		if key == "" {
			key = "(none)"
		}
		versions[key] = answer(func() (any, error) {
			resolved, err := p.ResolveVersion(lang, component)
			if err != nil {
				return nil, err
			}
			match, ok, err := p.InstalledVersion(lang, component)
			if err != nil {
				return nil, err
			}
			installed := []any{}
			if ok {
				installed = []any{match.Version, match.Home}
			}
			env, err := p.LanguageEnv(lang, component)
			if err != nil {
				return nil, err
			}
			return map[string]any{"spec": resolved.Spec, "source": resolved.Source, "exact": resolved.Exact(),
				"selects": resolved.Selects, "installed": installed, "env": env.Exports()}, nil
		})
	}
	return map[string]any{
		"default": answer(func() (any, error) { return p.DefaultVersion(lang) }),
		"installedAll": answer(func() (any, error) {
			all, err := p.InstalledVersions(lang)
			list := []any{}
			for _, entry := range all {
				list = append(list, []any{entry.Version, entry.Home})
			}
			return list, err
		}),
		"version": versions,
		"versions": answer(func() (any, error) {
			specs, err := p.Versions(lang)
			list := []any{}
			for _, s := range specs {
				list = append(list, []any{s.Spec, strs(s.Sources)})
			}
			return list, err
		}),
		"missing": answer(func() (any, error) {
			missing, err := p.MissingVersions(lang)
			list := []any{}
			for _, m := range missing {
				list = append(list, []any{m.Component, m.Spec, m.Source})
			}
			return list, err
		}),
		"notes": answer(func() (any, error) {
			notes, err := p.VersionNotes(lang)
			list := []any{}
			for _, n := range notes {
				list = append(list, []any{n.Component, n.Note})
			}
			return list, err
		}),
		"staticPath": answer(func() (any, error) {
			dirs, err := p.StaticPath(lang)
			return strs(dirs), err
		}),
	}
}

func goLanguage(p *Project, lang string) map[string]any {
	resolve := map[string]any{}
	for _, file := range append([]string{"config.json", "install.sh", "doctor.sh", "workflow.yml", "dockerfile", "installed.sh"}, Hooks...) {
		resolve[file] = p.Resolve(lang, file)
	}
	return map[string]any{
		"implTemplate": p.ImplTemplate(lang),
		"dirNames": answer(func() (any, error) {
			names, err := p.DirNames(lang)
			return strs(names), err
		}),
		"implDir": answer(func() (any, error) {
			result := map[string]any{}
			for _, component := range p.Components {
				dir, err := p.ImplDir(component, lang)
				if err != nil {
					return nil, err
				}
				result[component] = dir
			}
			return result, nil
		}),
		"implDirs": answer(func() (any, error) {
			dirs, err := p.ImplDirs(lang)
			list := []any{}
			for _, d := range dirs {
				list = append(list, []any{d.Component, d.Dir})
			}
			return list, err
		}),
		"resolve":           resolve,
		"projectDefinition": p.HasProjectDefinition(lang),
		"resolveName": answer(func() (any, error) {
			name, err := p.ResolveName(lang)
			if name == "" {
				return nil, err // Python's None
			}
			return name, err
		}),
	}
}

func loadFailure(err error) any {
	if e, ok := err.(*Error); ok {
		return map[string]any{"error": e.Message}
	}
	return skipped{err.Error()}
}

func answer(compute func() (any, error)) any {
	value, err := compute()
	if err != nil {
		if IsUnsupported(err) {
			return skipped{err.Error()}
		}
		if e, ok := err.(*Error); ok {
			return map[string]any{"error": e.Message}
		}
		return map[string]any{"goError": err.Error()}
	}
	return value
}

func strs(list []string) []any {
	result := []any{}
	for _, s := range list {
		result = append(result, s)
	}
	return result
}

func strMap(m map[string]string) map[string]any {
	result := map[string]any{}
	for k, v := range m {
		result[k] = v
	}
	return result
}

// normalize turns decoded JSON into plain Go values for comparison: objects
// become maps (keeping their key order under "\x00keys"), numbers float64.
func normalize(value any) any {
	switch v := value.(type) {
	case *Object:
		if v == nil {
			return nil
		}
		result := map[string]any{"\x00keys": strs(v.Keys)}
		for _, key := range v.Keys {
			result[key] = normalize(v.Values[key])
		}
		return result
	case []any:
		result := []any{}
		for _, item := range v {
			result = append(result, normalize(item))
		}
		return result
	case json.Number:
		f, err := strconv.ParseFloat(string(v), 64)
		if err != nil {
			return string(v)
		}
		return f
	}
	return value
}

// compareDump reports each difference between project.py's answer (want)
// and the Go model's (got), and counts the values compared and skipped.
func compareDump(t *testing.T, path string, want, got any) (compared, skips int) {
	t.Helper()
	if s, ok := got.(skipped); ok {
		if !pythonFailed(want) {
			t.Logf("%s: Go declined (%s) where project.py answered", path, s.reason)
		}
		return 0, 1
	}
	if wantMap, ok := want.(map[string]any); ok {
		if raised := wantMap["raised"]; raised != nil {
			t.Errorf("%s: project.py raised %v, but Go answered %#v", path, raised, got)
			return 1, 0
		}
		if crash := wantMap["crash"]; crash != nil {
			t.Errorf("%s: project.py crashed (%v) loading the project, but Go answered", path, crash)
			return 1, 0
		}
		gotMap, ok := got.(map[string]any)
		if !ok {
			t.Errorf("%s: project.py gave %#v, Go %#v", path, want, got)
			return 1, 0
		}
		_, wantKeyed := wantMap["\x00keys"]
		_, gotKeyed := gotMap["\x00keys"]
		if wantKeyed != gotKeyed {
			delete(wantMap, "\x00keys") // a dict dump.py built itself, against a Go map
			delete(gotMap, "\x00keys")
		}
		keys := map[string]bool{}
		for k := range wantMap {
			keys[k] = true
		}
		for k := range gotMap {
			keys[k] = true
		}
		names := make([]string, 0, len(keys))
		for k := range keys {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			c, s := compareDump(t, path+"/"+strings.TrimPrefix(k, "\x00"), wantMap[k], gotMap[k])
			compared += c
			skips += s
		}
		return compared, skips
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s:\n project.py: %#v\n Go:         %#v", path, want, got)
	}
	return 1, 0
}

// pythonFailed reports whether dump.py says project.py raised, crashed or
// refused the project.
func pythonFailed(want any) bool {
	m, ok := want.(map[string]any)
	return ok && (m["raised"] != nil || m["crash"] != nil || m["error"] != nil)
}

// A parity case: files making up a project (and, when builtin is set,
// fixture built-in definitions in place of languages/).
type parityCase struct {
	name     string
	files    map[string]string
	builtin  map[string]string
	cwd      string // relative to the root
	langs    []string
	git      bool              // a git repository: without a config, read as detected
	commit   string            // with git: commit everything, dated this (YYYY-MM-DD)
	versions bool              // compare toolchain versions and environments too
	env      map[string]string // set for both
}

func (c parityCase) create(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, root, c.files)
	if c.cwd != "" {
		if err := os.MkdirAll(filepath.Join(root, c.cwd), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if c.git {
		commands := [][]string{{"init", "-q"}, {"add", "-A"}}
		if c.commit != "" {
			commands = append(commands, []string{"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-qm", "x"})
		}
		for _, args := range commands {
			cmd := exec.Command("git", args...)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "GIT_COMMITTER_DATE="+c.commit+"T12:00:00", "GIT_AUTHOR_DATE="+c.commit+"T12:00:00")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
	}
	return root
}

// writeFiles creates each file under dir; a name ending in "/" is an empty
// directory.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func config(fields string) map[string]string {
	return map[string]string{".hecatron/config.json": fmt.Sprintf(`{"schema": 1%s}`, fields)}
}

func with(base map[string]string, more map[string]string) map[string]string {
	result := map[string]string{}
	for k, v := range base {
		result[k] = v
	}
	for k, v := range more {
		result[k] = v
	}
	return result
}

func parityCases() []parityCase {
	components := with(config(`, "project": "multi", "components": ["age", "date", "time"],
		"implPath": "{component}/impl/{lang}",
		"languages": ["typescript", "go", "csharp", "javascript", "rust"],
		"tools": {"cli": {"dependency": "cmake"}, "shell": {"dependency": "shellcheck"}, "docs": {}},
		"generated": {"dockerfile": "age/Dockerfile", "workflows": ".github/workflows/{lang}.yml"},
		"tasks": {"test": {"zeta": "echo z", "alpha": "echo a"}, "lint": {}},
		"metadata": "age/config/metadata.json", "logs": "../logs"`), map[string]string{
		"age/impl/go/go.mod":                 "",
		"age/impl/dotnet/x.csproj":           "",
		"date/impl/csharp/x.csproj":          "",
		"date/impl/javascript/tsconfig.json": "",
		"date/impl/javascript/package.json":  "",
		"time/impl/javascript/package.json":  "",
		"time/impl/rust/":                    "",
	})
	overrides := with(config(`, "languages": ["go", "rust", "mylang"]`), map[string]string{
		".hecatron/languages/go/config.json":     `{"abbreviation": "golang", "dirNames": ["golang"], "extra": [1, 2.50, {"b": null, "a": true}]}`,
		".hecatron/languages/go/test.sh":         "",
		".hecatron/languages/rust/lint.sh":       "",
		".hecatron/languages/mylang/config.json": `{"name": "Mine", "abbreviation": "my", "detect": ["*.my"]}`,
		".hecatron/languages/mylang/build.sh":    "",
		".hecatron/languages/mylang/install.sh":  "",
		"golang/main.go":                         "",
		"mylang/":                                "",
	})
	return []parityCase{
		{name: "minimal", files: config("")},
		{name: "detected", git: true, files: map[string]string{"go/go.mod": "module x\n", "go/main.go": "package main\n",
			"rust/Cargo.toml": "", "README": ""}, cwd: "go"},
		{name: "detected components", git: true, files: map[string]string{
			"age/go/go.mod": "module x\n", "date/python/pyproject.toml": "", "date/go/go.mod": "module y\n"}},
		{name: "detected, nothing found", git: true, files: map[string]string{"README": ""}},
		{name: "found from a subdirectory", files: config(`, "languages": ["go"]`), cwd: "a/b/c"},
		{name: "several components", files: components},
		{name: "several components from inside one", files: components, cwd: "date/impl"},
		{name: "project overrides and a project-only language", files: overrides},
		{name: "local.json disables", files: with(config(`, "languages": ["rust", "go", "python"]`), map[string]string{
			".hecatron/local.json": `{"disabled": ["rust", "go", "haskell"]}`})},
		{name: "local.json empty object", files: with(config(`, "languages": ["go"]`), map[string]string{
			".hecatron/local.json": `{}`})},
		{name: "a language in a nested path", files: with(config(`, "components": ["core"],
			"implPath": "lib/{component}/{lang}",
			"implPaths": {"csharp": "src/dotnet/app", "go": "tools/{component}/{lang}", "rust": ""},
			"languages": ["csharp", "go", "rust"]`), map[string]string{
			"src/dotnet/app/x.csproj":   "",
			"tools/core/go/go.mod":      "",
			"lib/core/rust/Cargo.toml":  "",
			"lib/core/python/README.md": "",
		})},
		{name: "explicit single component", files: config(`, "components": ["."], "languages": ["go"]`)},
		{name: "empty values fall back", files: config(`, "components": [], "project": "", "implPath": "", "tasks": null, "generated": {}`)},
		{name: "hecatron version met", files: config(`, "hecatron": ">= 0.1.0"`)},
		{name: "hecatron null", files: config(`, "hecatron": null`)},
		{name: "hecatron a number", files: config(`, "hecatron": 0.1`)},
		{name: "hecatron too new", files: config(`, "hecatron": ">=999.0.0"`)},
		{name: "schema true", files: map[string]string{".hecatron/config.json": `{"schema": true}`}},
		{name: "schema 1.0", files: map[string]string{".hecatron/config.json": `{"schema": 1.0}`}},
		{name: "schema 99", files: map[string]string{".hecatron/config.json": `{"schema": 99}`}},
		{name: "schema 2.5", files: map[string]string{".hecatron/config.json": `{"schema": 2.5}`}},
		{name: "schema 1e20", files: map[string]string{".hecatron/config.json": `{"schema": 1e20}`}},
		{name: "schema 1e-7", files: map[string]string{".hecatron/config.json": `{"schema": 1e-7}`}},
		{name: "schema a string", files: map[string]string{".hecatron/config.json": `{"schema": "1"}`}},
		{name: "schema a quoting string", files: map[string]string{".hecatron/config.json": `{"schema": "it's\n\u00e9\u2028"}`}},
		{name: "schema missing", files: map[string]string{".hecatron/config.json": `{"languages": []}`}},
		{name: "schema false", files: map[string]string{".hecatron/config.json": `{"schema": false}`}},
		{name: "schema a list", files: map[string]string{".hecatron/config.json": `{"schema": [1]}`}},
		{name: "config null", files: map[string]string{".hecatron/config.json": `null`}},
		{name: "config a list", files: map[string]string{".hecatron/config.json": `[]`}},
		{name: "config not JSON", files: map[string]string{".hecatron/config.json": `{"schema": 1,}`}},
		{name: "config NaN", files: map[string]string{".hecatron/config.json": `{"schema": 1, "x": NaN}`}},
		{name: "config with a BOM", files: map[string]string{".hecatron/config.json": "\ufeff{\"schema\": 1}"}},
		{name: "config with duplicate keys", files: map[string]string{".hecatron/config.json": `{"schema": 2, "languages": ["go"], "schema": 1, "languages": ["rust"]}`}},
		{name: "no project", files: map[string]string{"README": ""}},
		{name: "config.json a directory", files: map[string]string{".hecatron/config.json/": ""}},
		{name: "languages a string", files: config(`, "languages": "go"`)},
		{name: "languages null", files: config(`, "languages": null`)},
		{name: "components a string", files: config(`, "components": "abc"`)},
		{name: "local.json not JSON", files: with(config(""), map[string]string{".hecatron/local.json": `{`})},
		{name: "local.json a list", files: with(config(""), map[string]string{".hecatron/local.json": `[]`})},
		{name: "project definition not JSON", files: with(config(`, "languages": ["go"]`), map[string]string{
			".hecatron/languages/go/config.json": `{"abbreviation": }`})},
		{name: "project definition a list", files: with(config(`, "languages": ["go"]`), map[string]string{
			".hecatron/languages/go/config.json": `[]`})},
		{name: "tools not objects", files: config(`, "tools": {"x": "y"}`)},
		{name: "tasks a list", files: config(`, "tasks": ["x"]`)},
		{name: "audit problems", files: config(`, "audit": {"failOn": "severe", "ignore": [
			{"id": "GHSA-1", "reason": "fixed upstream", "until": "2027-01-31"},
			{"id": "GHSA-2", "reason": "  ", "until": "2027-1-31"},
			{"id": "GHSA-3", "reason": null, "until": "2027-01-31\n"},
			{"id": 7, "until": 20270131},
			{"reason": "no id"},
			"not an object",
			{"id": "", "reason": "empty id"}]}`)},
		{name: "lint format per language", files: with(config(`, "languages": ["go", "python", "rust"],
			"lint": {"format": {"go": false, "rust": true}}`), map[string]string{
			".hecatron/languages/python/format.sh": "", ".hecatron/languages/go/format.sh": ""})},
		{name: "lint format off", files: config(`, "languages": ["go"], "lint": {"format": false}`)},
		{name: "lint format not a bool", files: config(`, "languages": ["go"], "lint": {"format": {"go": 0}}`)},
		{name: "lint format a string", files: config(`, "lint": {"format": "yes"}`)},
		{name: "lint not an object", files: config(`, "lint": []`)},
		{name: "lint format null", files: config(`, "languages": ["go"], "lint": {"format": null}`)},
		{name: "audit not an object", files: config(`, "audit": ["x"]`)},
		{name: "audit failOn fine", files: config(`, "audit": {"failOn": "moderate", "ignore": null}`)},
		{name: "fixture definitions", builtin: map[string]string{
			"alpha/config.json":     `{"abbreviation": "al", "dependency": "alpha", "dirNames": ["a"], "detectUnless": ["*.beta"]}`,
			"alpha/hooks/test.sh":   "",
			"alpha/install.sh":      "",
			"beta/config.json":      `{"lintDependency": "betalint", "path": ["~/.beta/bin", "{home}/bin"]}`,
			"beta/hooks/lint.sh":    "",
			"gamma/config.json":     `{"abbreviation": "-g"}`,
			"notalang/README":       "",
			".hidden/config.json":   `{}`,
			"delta/config.json/":    "",
			"epsilon/config.json":   `{"path": "not a list"}`,
			"zeta/config.json":      `{"dirNames": null, "detectUnless": "x"}`,
			"eta/config.json":       `{"dependency": 5}`,
			"theta/config.json":     `{"detectUnless": ["[z-a]"]}`,
			"theta/hooks/build.sh/": "",
		}, files: with(config(`, "components": ["one", "two"], "languages": ["alpha", "beta", "gamma", "theta", "zeta"]`), map[string]string{
			"one/a/x.alpha":    "",
			"one/alpha/x.beta": "",
			"two/alpha/":       "",
			"one/theta/x":      "",
		}), langs: []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta", "AL", "Alpha", "nope"}},
	}
}
