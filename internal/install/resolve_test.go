package install

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var languages = []string{"go", "rust", "python", "java", "elixir"}

func load(t *testing.T, lang string) (*Spec, string) {
	t.Helper()
	spec, version, err := Load(filepath.Join("..", "..", "languages"), lang)
	if err != nil {
		t.Fatal(err)
	}
	return spec, version
}

func TestResolvesOnEveryPlatform(t *testing.T) {
	for _, lang := range languages {
		spec, version := load(t, lang)
		for _, platform := range Platforms {
			if _, err := Resolve(spec, version, platform, false); err != nil {
				t.Errorf("%s on %s %s: %v", lang, platform.OS, platform.Arch, err)
			}
		}
	}
}

func TestResolvedSteps(t *testing.T) {
	cases := []struct {
		lang     string
		platform Platform
		want     string
	}{
		{"go", Platform{"windows", "aarch64"}, "https://dl.google.com/go/go1.27.3.windows-arm64.zip"},
		{"go", Platform{"macos", "x86_64"}, "https://dl.google.com/go/go1.27.3.darwin-amd64.tar.gz"},
		{"rust", Platform{"linux", "aarch64"}, "https://sh.rustup.rs"},
		{"rust", Platform{"windows", "x86_64"}, "https://static.rust-lang.org/rustup/dist/x86_64-pc-windows-msvc/rustup-init.exe"},
		{"python", Platform{"windows", "x86_64"}, "https://github.com/astral-sh/uv/releases/download/0.13.6/uv-x86_64-pc-windows-msvc.zip"},
	}
	for _, c := range cases {
		spec, version := load(t, c.lang)
		steps, err := Resolve(spec, version, c.platform, false)
		if err != nil {
			t.Fatal(err)
		}
		if steps[0].Download != c.want {
			t.Errorf("%s on %v: downloads %s, want %s", c.lang, c.platform, steps[0].Download, c.want)
		}
	}
	spec, version := load(t, "python")
	steps, _ := Resolve(spec, version, Platform{"linux", "x86_64"}, false)
	if last := strings.Join(steps[len(steps)-1].Run, " "); last != "uv python install 3.15.1" {
		t.Errorf("python installs with %q", last)
	}
	spec, version = load(t, "elixir")
	steps, _ = Resolve(spec, version, Platform{"windows", "x86_64"}, false)
	if steps[0].Requires[0] != "erlang" || steps[1].Manager != "scoop" {
		t.Errorf("elixir on Windows: %+v", steps)
	}
}

// Every URL an install downloads exists, on every platform. Needs the
// network: HECATRON_NETWORK_TESTS=1.
func TestDownloadsExist(t *testing.T) {
	if os.Getenv("HECATRON_NETWORK_TESTS") == "" {
		t.Skip("set HECATRON_NETWORK_TESTS=1 to check the download URLs")
	}
	for _, lang := range languages {
		spec, version := load(t, lang)
		for _, platform := range Platforms {
			steps, err := Resolve(spec, version, platform, true)
			if err != nil {
				t.Errorf("%s on %s %s: %v", lang, platform.OS, platform.Arch, err)
				continue
			}
			for _, step := range steps {
				for _, url := range []string{step.Download, step.Checksum} {
					if !strings.HasPrefix(url, "https://") {
						continue
					}
					if status := head(url); status != http.StatusOK {
						t.Errorf("%s on %s %s: %s answers %d", lang, platform.OS, platform.Arch, url, status)
					}
				}
			}
		}
	}
}

func head(url string) int {
	response, err := client.Head(url)
	if err != nil {
		return 0
	}
	_ = response.Body.Close() // unread: only the status matters
	return response.StatusCode
}
