package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// _xcode plan prints internal/xcode's plan, as lines for the hooks or as
// JSON.
func TestXcodePlan(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src", "App.xcodeproj"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := runIn(t, dir, nil, binary, "_xcode", "plan")
	if want := "unsure\tno Xcode project or workspace here\nnested\tsrc/App.xcodeproj\n"; r.status != 0 || r.stdout != want {
		t.Errorf("got %+v, want %q", r, want)
	}
	r = runIn(t, t.TempDir(), nil, binary, "_xcode", "plan", "--json", dir)
	var plan struct {
		Nested []string
		Unsure string
	}
	if err := json.Unmarshal([]byte(r.stdout), &plan); err != nil || len(plan.Nested) != 1 || plan.Unsure == "" {
		t.Errorf("got %+v (%v)", r, err)
	}
	if r := runIn(t, dir, nil, binary, "_xcode"); r.status != 2 {
		t.Errorf("without plan: %+v", r)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "App.xcodeproj", "project.pbxproj"), []byte("{ broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := runIn(t, dir, nil, binary, "_xcode", "plan", "src"); r.status != 1 || r.stderr == "" {
		t.Errorf("a broken project: %+v", r)
	}
}
