package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests of the model on its own, which run everywhere (Windows included);
// parity_test.go compares it with project.py.

func TestPyFloatRepr(t *testing.T) {
	for text, want := range map[string]string{
		"1.0": "1.0", "2.5": "2.5", "1e2": "100.0", "1e15": "1000000000000000.0", "1e16": "1e+16",
		"1.5e16": "1.5e+16", "1e-4": "0.0001", "1e-5": "1e-05", "-0.0": "-0.0", "0.1": "0.1",
		"123456789.125": "123456789.125", "1e400": "inf", "-1e400": "-inf", "1e-7": "1e-07",
	} {
		got, err := pyNumber(json.Number(text))
		if err != nil || got != want {
			t.Errorf("%s: %q (%v), want %q", text, got, err, want)
		}
	}
}

func TestPyStringRepr(t *testing.T) {
	for s, want := range map[string]string{
		"1":          "'1'",
		"it's":       `"it's"`,
		`it's "x"`:   `'it\'s "x"'`,
		"a\nb\tc\\":  `'a\nb\tc\\'`,
		"\x00\x7f":   `'\x00\x7f'`,
		"é ":         `'é\xa0'`,
		"\u2028":     `'\u2028'`,
		"\U0001F600": "'\U0001F600'",
	} {
		if got := pyStringRepr(s); got != want {
			t.Errorf("%q: %s, want %s", s, got, want)
		}
	}
}

func TestFnmatch(t *testing.T) {
	for _, c := range []struct {
		name, pattern string
		want          bool
	}{
		{"tsconfig.json", "tsconfig.json", true},
		{"x.csproj", "*.csproj", true},
		{".hidden.csproj", "*.csproj", true},
		{"a/b", "a*", true},
		{"abc", "a?c", true},
		{"abc", "a[!b]c", false},
		{"adc", "a[!b]c", true},
		{"a^c", "a[^]c", true},
		{"a[c", "a[c", true},
		{"a.c", "a.c", true},
		{"abc", "a.c", false},
	} {
		got, err := fnmatch(c.name, c.pattern)
		if err != nil || got != c.want {
			t.Errorf("fnmatch(%q, %q) = %v (%v), want %v", c.name, c.pattern, got, err, c.want)
		}
	}
	if _, err := fnmatch("x", "[z-a]"); !IsUnsupported(err) {
		t.Errorf("a reversed range: %v, want unsupported", err)
	}
}

func TestDecodeKeepsKeyOrderAndLastValue(t *testing.T) {
	value, err := decodeJSON([]byte(`{"b": 1, "a": {"y": 1, "x": 2}, "b": 3}`))
	if err != nil {
		t.Fatal(err)
	}
	object := value.(*Object)
	if strings.Join(object.Keys, ",") != "b,a" || string(object.Values["b"].(json.Number)) != "3" {
		t.Errorf("keys %v, b = %v", object.Keys, object.Values["b"])
	}
	for _, bad := range []string{`{"a": NaN}`, `{} {}`, "\xff", `{`} {
		if _, err := decodeJSON([]byte(bad)); !IsUnsupported(err) {
			t.Errorf("%q: %v, want unsupported", bad, err)
		}
	}
}

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func testHome(t *testing.T) Home {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, map[string]string{
		"VERSION":                          "0.3.0\n",
		"languages/go/config.json":         `{"abbreviation": "golang", "dirNames": ["golang"]}`,
		"languages/go/hooks/test.sh":       "",
		"languages/csharp/config.json":     `{"abbreviation": "cs", "dirNames": ["dotnet"]}`,
		"languages/javascript/config.json": `{"detectUnless": ["tsconfig.json"]}`,
	})
	t.Setenv("HECATRON_BUILTIN_LANGUAGES", "")
	return NewHome(dir)
}

func TestFindRootWalksUp(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{".hecatron/config.json": `{"schema": 1}`, "a/b/.keep": ""})
	got, err := FindRoot(filepath.Join(root, "a", "b"))
	if err != nil || got != root {
		t.Errorf("FindRoot: %q, %v", got, err)
	}
	if _, err := FindRoot(t.TempDir()); err == nil || !strings.HasPrefix(err.Error(), "not a hecatron project") {
		t.Errorf("no project: %v", err)
	}
}

func TestLoadRefuses(t *testing.T) {
	home := testHome(t)
	for config, want := range map[string]string{
		`{"schema": 99}`:                      "schema 99 is not supported (this hecatron reads schema 1)",
		`{"schema": "1"}`:                     "schema '1' is not supported",
		`{}`:                                  "schema None is not supported",
		`null`:                                "config.json not found",
		`{"schema": 1, "hecatron": ">=0.10"}`: "this project needs hecatron 0.10 or newer (installed: 0.3.0)",
	} {
		root := t.TempDir()
		write(t, root, map[string]string{".hecatron/config.json": config})
		_, err := Load(root, home)
		if e, ok := err.(*Error); !ok || !strings.Contains(e.Message, want) {
			t.Errorf("%s: %v, want an error saying %q", config, err, want)
		}
	}
	for _, config := range []string{`[]`, `{"schema": 1, "languages": "go"}`, `{"schema": 1, "components": [1]}`} {
		root := t.TempDir()
		write(t, root, map[string]string{".hecatron/config.json": config})
		if _, err := Load(root, home); !IsUnsupported(err) {
			t.Errorf("%s: %v, want unsupported", config, err)
		}
	}
}

func TestModel(t *testing.T) {
	home := testHome(t)
	root := t.TempDir()
	write(t, root, map[string]string{
		".hecatron/config.json": `{"schema": 1, "components": ["age", "date"], "implPath": "{component}/impl/{lang}",
			"languages": ["javascript", "go", "csharp"], "tasks": {"test": {"z": "echo z", "a": "echo a"}}}`,
		".hecatron/local.json":               `{"disabled": ["go"]}`,
		".hecatron/languages/go/config.json": `{"abbreviation": "gl"}`,
		".hecatron/languages/go/lint.sh":     "",
		"age/impl/golang/go.mod":             "",
		"date/impl/dotnet/x.csproj":          "",
		"date/impl/javascript/tsconfig.json": "",
	})
	p, err := Load(root, home)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(p.Languages, ",") != "csharp,go,javascript" || strings.Join(p.Disabled, ",") != "go" {
		t.Errorf("languages %v, disabled %v", p.Languages, p.Disabled)
	}
	dirs, err := p.ImplDirs("go")
	if err != nil || len(dirs) != 1 || dirs[0].Dir != filepath.Join(root, "age", "impl", "golang") {
		t.Errorf("go: %v %v", dirs, err)
	}
	if dirs, _ := p.ImplDirs("javascript"); len(dirs) != 0 {
		t.Errorf("javascript/ holding a tsconfig.json is TypeScript's: %v", dirs)
	}
	if dir, _ := p.ImplDir("date", "csharp"); dir != filepath.Join(root, "date", "impl", "dotnet") {
		t.Errorf("csharp in date: %s", dir)
	}
	if got := p.Resolve("go", "lint.sh"); got != filepath.Join(root, ".hecatron", "languages", "go", "lint.sh") {
		t.Errorf("project hook: %s", got)
	}
	if got := p.Resolve("go", "test.sh"); got != filepath.Join(home.Languages, "go", "hooks", "test.sh") {
		t.Errorf("built-in hook: %s", got)
	}
	if name, _ := p.ResolveName("GL"); name != "go" {
		t.Errorf("the project's abbreviation overrides the built-in one: %q", name)
	}
	tasks, _ := p.Tasks("test")
	if len(tasks) != 2 || tasks[0].Name != "z" {
		t.Errorf("tasks keep their order: %v", tasks)
	}
}

func TestConfigProblems(t *testing.T) {
	home := testHome(t)
	root := t.TempDir()
	write(t, root, map[string]string{".hecatron/config.json": `{"schema": 1, "audit": {"failOn": "x",
		"ignore": [{"id": "A", "reason": "r", "until": "2027-01-01"}, {"id": "B"}, 3]}}`})
	p, err := Load(root, home)
	if err != nil {
		t.Fatal(err)
	}
	problems, err := p.ConfigProblems()
	want := []string{`audit.ignore B needs a "reason"`, `audit.ignore B needs an "until" date (YYYY-MM-DD)`,
		`audit.ignore[2] needs an "id"`, "audit.failOn must be one of critical, high, moderate, low"}
	if err != nil || strings.Join(problems, "\n") != strings.Join(want, "\n") {
		t.Errorf("problems:\n%s\n(%v)", strings.Join(problems, "\n"), err)
	}
}

func TestLanguageEnvAddsOtherLanguagesStaticPaths(t *testing.T) {
	home := testHome(t)
	root := t.TempDir()
	tools := t.TempDir()
	for _, dir := range []string{"own", "shared", "cargo", "off"} {
		if err := os.MkdirAll(filepath.Join(tools, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	path := func(dirs ...string) string {
		quoted := make([]string, len(dirs))
		for i, dir := range dirs {
			quoted[i] = `"` + filepath.Join(tools, dir) + `"`
		}
		return `{"path": [` + strings.Join(quoted, ", ") + `]}`
	}
	write(t, root, map[string]string{
		".hecatron/config.json":               `{"schema": 1, "languages": ["rb", "rs", "zz", "off"]}`,
		".hecatron/local.json":                `{"disabled": ["off"]}`,
		".hecatron/languages/rb/config.json":  path("own", "shared"),
		".hecatron/languages/rs/config.json":  path("cargo", "shared"),
		".hecatron/languages/zz/config.json":  `{"path": ["{home}/bin"]}`,
		".hecatron/languages/off/config.json": path("off"),
	})
	p, err := Load(root, home)
	if err != nil {
		t.Fatal(err)
	}
	env, err := p.LanguageEnv("rb", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(tools, "own"), filepath.Join(tools, "shared"), filepath.Join(tools, "cargo")}
	if strings.Join(env.Path, ":") != strings.Join(want, ":") {
		t.Errorf("rb's PATH entries: %v, want its own, then rs's not already there (not the disabled off's)", env.Path)
	}
}

// An install.sh may need root where its definition's "installNeedsRoot"
// says, and anywhere when the definition doesn't say or the script is the
// project's own.
func TestScriptNeedsRoot(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		".hecatron/config.json":                `{"schema": 1, "languages": ["lua", "perl", "go", "mine"]}`,
		".hecatron/languages/go/install.sh":    "",
		".hecatron/languages/mine/config.json": `{"abbreviation": "mi"}`,
		".hecatron/languages/mine/install.sh":  "",
	})
	home := testHome(t)
	write(t, filepath.Dir(home.Languages), map[string]string{
		"languages/lua/config.json":  `{"installNeedsRoot": ["linux"]}`,
		"languages/perl/config.json": `{"installNeedsRoot": ["linux", "macos"]}`,
		"languages/go/config.json":   `{"installNeedsRoot": ["linux"]}`,
	})
	p, err := Load(root, home)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		lang, os string
		want     bool
	}{
		{"lua", "linux", true}, {"lua", "macos", false}, {"perl", "macos", true},
		{"go", "macos", true}, {"mine", "macos", true},
	} {
		if got, err := p.ScriptNeedsRoot(c.lang, c.os); got != c.want || err != nil {
			t.Errorf("%s on %s: %v %v", c.lang, c.os, got, err)
		}
	}
}
