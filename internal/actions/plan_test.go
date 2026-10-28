package actions

import (
	"path/filepath"
	"strings"
	"testing"
)

// planned is the test action's plan for Python in a project of these files,
// as lines.
func planned(t *testing.T, files map[string]string) []string {
	t.Helper()
	home, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	def, err := Load(filepath.Join(home, "languages"), "python")
	if err != nil {
		t.Fatal(err)
	}
	root, dir := project{lang: "python", files: files}.create(t, home)
	ctx := Context{Action: "test", Dir: dir, Root: root}
	ops, err := Plan(def, ctx)
	if err != nil {
		t.Fatal(err)
	}
	return Lines(def, ctx, ops)
}

func TestPytestCollectsTestThingFiles(t *testing.T) {
	lines := planned(t, map[string]string{"pyproject.toml": "", "tests/TestFileStream.py": ""})
	want := "+ uv run --all-extras --all-groups --with pytest pytest -o python_files=test_*.py *_test.py Test*.py"
	if strings.Join(lines, "\n") != want {
		t.Errorf("plan: %q, want %q", lines, want)
	}
}

func TestFindNameSkipsHiddenAndVendoredDirectories(t *testing.T) {
	_, dir := project{lang: "python", files: map[string]string{
		".venv/lib/TestA.py": "", "node_modules/x/TestB.py": "", "src/pkg/TestC.py": ""}}.create(t, "")
	for glob, want := range map[string]bool{"TestA.py": false, "TestB.py": false, "TestC.py": true, "Test*.py": true} {
		if got := FindName(dir, glob); got != want {
			t.Errorf("FindName(%q) = %v, want %v", glob, got, want)
		}
	}
}
