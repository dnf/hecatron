//go:build unix

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// adminRights with a stub sudo on PATH that records how it was called and
// needs a password (-n fails) or doesn't.
func TestAdminRights(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root needs no sudo")
	}
	for _, c := range []struct {
		name, noPassword, want string
	}{
		{"a password, off a terminal: nothing asked", "exit 1", "-n true\n"},
		{"no password needed (CI): no prompt", "exit 0", "-n true\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			calls := filepath.Join(dir, "calls")
			writeFiles(t, dir, map[string]string{"sudo": "#!/bin/sh\necho \"$*\" >> '" + calls + "'\n[ \"$1\" = -n ] && { " + c.noPassword + "; }\nexit 0\n"})
			if err := os.Chmod(filepath.Join(dir, "sudo"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+":/usr/bin:/bin")
			stop, ok := adminRights([]string{"lua"})
			time.Sleep(50 * time.Millisecond)
			stop()
			if got := readText(t, calls); !ok || got != c.want {
				t.Errorf("ok %v, sudo called with %q", ok, got)
			}
		})
	}
	t.Run("nothing needs root: sudo isn't run", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("PATH", dir)
		if stop, ok := adminRights(nil); !ok {
			t.Error("not ok")
		} else {
			stop()
		}
	})
}

func TestSentenceList(t *testing.T) {
	for want, items := range map[string][]string{"": nil, "a": {"a"}, "a and b": {"a", "b"}, "a, b and c": {"a", "b", "c"}} {
		if got := sentenceList(items); got != want {
			t.Errorf("%v: %q", items, got)
		}
	}
}
