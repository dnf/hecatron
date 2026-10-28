package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEnv(t *testing.T) {
	env := []string{"A=1", "PATH=/x", "=C:=C:\\", "B=2", "PATH=/y"}
	if got := Getenv(env, "PATH"); got != "/y" {
		t.Errorf("PATH: %q, want the last", got)
	}
	if _, ok := LookupEnv(env, "C"); ok {
		t.Errorf("C is unset")
	}
	set := Setenv(env, "PATH", "/z")
	if got := Getenv(set, "PATH"); got != "/z" || len(set) != 4 {
		t.Errorf("Setenv: %q", set)
	}
	// Windows' environment says Path; names compare without case there.
	got := Getenv([]string{"Path=/w"}, "PATH")
	if want := map[bool]string{true: "/w", false: ""}[runtime.GOOS == "windows"]; got != want {
		t.Errorf("Path as PATH: %q, want %q", got, want)
	}
}

func TestLookPath(t *testing.T) {
	dir := t.TempDir()
	name := "tool"
	if runtime.GOOS == "windows" {
		name = "tool.exe" // an executable by its extension
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := JoinPath(t.TempDir(), dir)
	found, ok := LookPath("tool", path)
	if !ok || found != filepath.Join(dir, name) {
		t.Errorf("tool: %q %v", found, ok)
	}
	if _, ok := LookPath("data", path); ok {
		t.Errorf("data isn't executable")
	}
	if _, ok := LookPath("missing", path); ok {
		t.Errorf("missing found")
	}
}
