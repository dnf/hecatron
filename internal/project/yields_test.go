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
		if got, err := rule.holds(c.names); err != nil || got != c.want {
			t.Errorf("%v: %v %v, want %v", c.names, got, err, c.want)
		}
	}
}
