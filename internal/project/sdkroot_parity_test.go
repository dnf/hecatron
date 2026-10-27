package project

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A definition's "sdkrootFromXcrun" adds SDKROOT on macOS, from a stub
// xcrun, as project.py does: only on macOS, only when SDKROOT isn't set,
// and not when xcrun fails. HECATRON_DUMP_HOST_OS (testdata/dump.py) and
// hostOS take macOS's path on any platform.
func TestSdkrootFromXcrunMatchesProjectPy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("compares with project.py, which the tests run on macOS and Linux")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	home := repoHome(t)
	stubs := t.TempDir()
	for name, script := range map[string]string{
		"ok/xcrun":   "#!/bin/sh\necho /stub/MacOSX26.5.sdk\n",
		"fail/xcrun": "#!/bin/sh\necho 'xcrun: error' >&2\nexit 1\n",
	} {
		path := filepath.Join(stubs, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name, hostOS, xcrun, sdkroot string
		added                        bool
	}{
		{"macOS", "darwin", "ok", "", true},
		{"macOS with SDKROOT set", "darwin", "ok", "/mine", false},
		{"macOS with a failing xcrun", "darwin", "fail", "", false},
		{"Linux", "linux", "ok", "", false},
	}
	files := map[string]string{
		".hecatron/config.json": `{"schema": 1, "languages": ["ad", "plain"]}`,
		"ad/hello.adb":          "",
		"plain/x":               "",
	}
	builtin := map[string]string{
		"ad/config.json":    `{"sdkrootFromXcrun": true, "path": ["~/.local/bin"]}`,
		"plain/config.json": `{"path": ["~/.local/bin"]}`,
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("HECATRON_AS_OF", "off")
			t.Setenv("HECATRON_DUMP_VERSIONS", "1")
			t.Setenv("HECATRON_DUMP_HOST_OS", c.hostOS)
			t.Setenv("PATH", filepath.Join(stubs, c.xcrun)+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("SDKROOT", c.sdkroot)
			if c.sdkroot == "" {
				os.Unsetenv("SDKROOT")
			}
			savedOS, savedSDK := hostOS, xcrunSDK
			hostOS, xcrunSDK = c.hostOS, new(xcrunCache)
			defer func() { hostOS, xcrunSDK = savedOS, savedSDK }()

			root := parityCase{files: files}.create(t)
			languages := filepath.Join(t.TempDir(), "languages")
			writeFiles(t, languages, builtin)
			langs := []string{"ad", "plain"}
			want := pythonDump(t, home, languages, root, langs)
			got := goDump(t, home, languages, root, langs)
			compareDump(t, "", want, got)

			for _, lang := range langs {
				env := dumpedEnv(t, got, lang)
				wanted := c.added && lang == "ad"
				if has := strings.Contains(env, "export SDKROOT=/stub/MacOSX26.5.sdk\n"); has != wanted {
					t.Errorf("%s's env: %q (SDKROOT expected: %v)", lang, env, wanted)
				}
				if !wanted && strings.Contains(env, "SDKROOT") {
					t.Errorf("%s's env sets SDKROOT: %q", lang, env)
				}
			}
		})
	}
}

// dumpedEnv is the env query's output for a language's only component in a
// dump.
func dumpedEnv(t *testing.T, dump any, lang string) string {
	t.Helper()
	encoded, err := json.Marshal(dump)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Lang map[string]struct {
			Version map[string]struct {
				Env string `json:"env"`
			} `json:"version"`
		} `json:"lang"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded.Lang[lang].Version["."].Env
}
