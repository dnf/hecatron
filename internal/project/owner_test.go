package project

import (
	"os"
	"path/filepath"
	"testing"
)

// The built-in definitions' file patterns, as a workspace at the root
// claims changes with them: its sources, build, lock and toolchain files,
// not what else sits beside its packages.
func TestLanguageFiles(t *testing.T) {
	t.Setenv("HECATRON_BUILTIN_LANGUAGES", "")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".hecatron"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := `{"schema": 1, "components": ["."], "languages": ["go", "java", "javascript", "rust"], "implPath": "."}`
	if err := os.WriteFile(filepath.Join(root, ".hecatron", "config.json"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(root, NewHome(repoHome(t)))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct{ claimed, not []string }{
		"rust": {[]string{"Cargo.toml", "Cargo.lock", "build.rs", "crates/a/src/lib.rs", "rust-toolchain.toml",
			"rust-toolchain", ".cargo/config.toml", "crates/a/Cargo.toml"},
			[]string{"erl_crash.dump", "README.md", "crates/a/README.md", ".github/workflows/ci.yml", "x.go"}},
		"go": {[]string{"go.mod", "go.sum", "go.work", "cmd/x/main.go", "internal/x/testdata/in.txt"},
			[]string{"docs/guide.md", "Cargo.toml"}},
		"java": {[]string{"pom.xml", "src/main/java/A.java", "settings.gradle.kts", "gradle/wrapper/gradle-wrapper.properties",
			"src/main/resources/app.properties"}, []string{"README.md"}},
		"javascript": {[]string{"package.json", "package-lock.json", "src/index.ts", "lib/a.mjs", ".nvmrc"},
			[]string{"README.md", "Cargo.toml"}},
	}
	for lang, c := range cases {
		patterns, err := p.LanguageFiles(lang)
		if err != nil {
			t.Fatal(err)
		}
		if patterns == nil {
			t.Fatalf("%s has no sourceFiles", lang)
		}
		for _, file := range c.claimed {
			if !MatchesLanguageFile(file, patterns) {
				t.Errorf("%s doesn't claim %s", lang, file)
			}
		}
		for _, file := range c.not {
			if MatchesLanguageFile(file, patterns) {
				t.Errorf("%s claims %s", lang, file)
			}
		}
	}
}

// A file is in a nested repository when a directory holding it below the
// root has a .git directory or file; git status lists one it can read as
// its directory, with a trailing slash.
func TestOutsideNestedRepos(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"clone/.git", "a/b/worktree", "plain"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "a/b/worktree/.git"), []byte("gitdir: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := OutsideNestedRepos(root, []string{"a/b/c.rs", "a/b/worktree/", "a/b/worktree/src/x.rs", "clone/", "clone/x",
		"plain/x", "top.txt"})
	want := []string{"a/b/c.rs", "plain/x", "top.txt"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}
