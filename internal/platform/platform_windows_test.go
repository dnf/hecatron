package platform

import (
	"os"
	"path/filepath"
	"testing"
)

// Git for Windows' bash is found on PATH or beside git.exe; System32's
// bash.exe (WSL's launcher) never is.
func TestBash(t *testing.T) {
	write := func(path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	root := t.TempDir()
	t.Setenv("SystemRoot", filepath.Join(root, "Windows"))
	system32 := filepath.Join(root, "Windows", "System32")
	write(filepath.Join(system32, "bash.exe"))
	if found, ok := Bash(system32); ok {
		t.Errorf("WSL's launcher taken for bash: %s", found)
	}

	git := filepath.Join(root, "Git")
	write(filepath.Join(git, "cmd", "git.exe"))
	write(filepath.Join(git, "bin", "bash.exe"))
	if found, ok := Bash(JoinPath(system32, filepath.Join(git, "cmd"))); !ok || found != filepath.Join(git, "bin", "bash.exe") {
		t.Errorf("bash beside git: %q %v", found, ok)
	}
	if found, ok := Bash(filepath.Join(git, "bin")); !ok || found != filepath.Join(git, "bin", "bash.exe") {
		t.Errorf("bash on PATH: %q %v", found, ok)
	}
}
