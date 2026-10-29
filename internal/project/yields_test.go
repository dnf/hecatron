package project

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestATsconfigBesideMostlyJavaScriptLeavesTheDirectoryJavaScripts(t *testing.T) {
	home := NewHome(repoHome(t))
	t.Setenv("HECATRON_BUILTIN_LANGUAGES", "")
	root := t.TempDir()
	write(t, root, map[string]string{
		".hecatron/config.json":        `{"schema": 1, "implPath": "{lang}", "languages": ["javascript", "typescript"]}`,
		"javascript/package.json":      "{}",
		"javascript/tsconfig.json":     "{}",
		"javascript/src/a.js":          "",
		"javascript/src/b.mjs":         "",
		"javascript/src/a.d.ts":        "",
		"javascript/src/c.ts":          "",
		"javascript/node_modules/x.ts": "",
	})
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A", "-f"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	p, err := Load(root, home)
	if err != nil {
		t.Fatal(err)
	}
	if dirs, err := p.ImplDirs("javascript"); err != nil || len(dirs) != 1 || dirs[0].Dir != filepath.Join(root, "javascript") {
		t.Errorf("javascript: %v %v", dirs, err)
	}
	if dirs, err := p.ImplDirs("typescript"); err != nil || len(dirs) != 0 {
		t.Errorf("typescript: %v %v", dirs, err)
	}
}

func TestYieldsCountsOutsideDotfilesAndExceptions(t *testing.T) {
	rule := &yieldsRule{To: "javascript", Where: []string{"*.js"}, Outnumber: []string{"*.ts"}, Except: []string{"*.d.ts"}}
	for _, c := range []struct {
		names []string
		want  bool
	}{
		{[]string{"a.js", "b.d.ts", "c.d.ts"}, true},
		{[]string{"a.js", "b.ts"}, false},
		{[]string{".eslintrc.js", "a.js", "b.js", "c.ts"}, true},
		{[]string{".eslintrc.js", ".prettierrc.js", "c.ts"}, false},
		{nil, false},
	} {
		if got, err := rule.holds(c.names, ""); err != nil || got != c.want {
			t.Errorf("%v: %v %v, want %v", c.names, got, err, c.want)
		}
	}
}

func TestYieldsLeavesOutBuildOutput(t *testing.T) {
	dir := t.TempDir()
	tsconfig := filepath.Join(dir, "tsconfig.json")
	write(t, dir, map[string]string{"tsconfig.json": `{
		// comments and trailing commas, as tsconfig.json allows
		"compilerOptions": {"outDir": "./compiled/", /* here */ "rootDir": "src",},
	}`})
	rule := &yieldsRule{To: "javascript", Where: []string{"*.js", "*.mjs"}, Outnumber: []string{"*.ts"},
		Except: []string{"*.d.ts"}, SkipDirs: []string{"dist", "lib"}, OutputDir: []string{"compilerOptions", "outDir"},
		Twins: true}
	for _, c := range []struct {
		name  string
		paths []string
		want  bool
	}{
		{"compiled output in dist/", []string{"src/a.ts", "dist/a.js", "dist/b.js", "dist/c.js"}, false},
		{"in the outDir", []string{"src/a.ts", "compiled/x.js", "compiled/y/z.js"}, false},
		{"twins beside their sources", []string{"src/a.ts", "src/a.js", "src/b.ts", "src/b.mjs", "c.js"}, false},
		{"a lib/ deeper is source", []string{"src/a.ts", "src/lib/x.js", "src/lib/y.js"}, true},
		{"mostly JavaScript", []string{"src/a.js", "src/b.js", "src/a.d.ts", "src/c.ts"}, true},
	} {
		if got, err := rule.holds(c.paths, tsconfig); err != nil || got != c.want {
			t.Errorf("%s: %v %v, want %v", c.name, got, err, c.want)
		}
	}
	if got, err := rule.holds([]string{"compiled/x.js", "compiled/y.js", "a.ts"}, filepath.Join(dir, "missing.json")); err != nil || !got {
		t.Errorf("without a build file to read, the outDir isn't known: %v %v", got, err)
	}
}

func TestLooseJSON(t *testing.T) {
	for in, want := range map[string]string{
		`{"a": 1, // x` + "\n" + `}`:      `{"a": 1 ` + "\n" + `}`,
		`{"a": [1, 2,], }`:                `{"a": [1, 2] }`,
		`{"u": "http://x/*y*/", /* z */}`: `{"u": "http://x/*y*/" }`,
		`{"q": "a\",}"}`:                  `{"q": "a\",}"}`,
		`{"open": "x`:                     `{"open": "x`,
	} {
		if got := looseJSON(in); got != want {
			t.Errorf("looseJSON(%q) = %q, want %q", in, got, want)
		}
	}
}
