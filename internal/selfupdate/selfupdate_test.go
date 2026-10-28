package selfupdate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"0.6.0", "0.5.0", true}, {"0.10.0", "0.9.9", true}, {"1.0.0", "0.99.99", true},
		{"0.5.0", "0.5.0", false}, {"0.4.9", "0.5.0", false}, {"", "0.5.0", false},
		{"0.6", "0.5.0", false}, {"0.6.0-rc1", "0.5.0", false}, {"+1.0.0", "0.5.0", false},
	} {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func TestChecksum(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	sums := []byte(strings.Repeat("0", 64) + "  hecatron-linux-arm64\n" + strings.ToUpper(sum) + " *hecatron-linux-amd64\nnot a line\n")
	if got, ok := checksum(sums, "hecatron-linux-amd64"); !ok || got != sum {
		t.Errorf("got %q %v", got, ok)
	}
	if _, ok := checksum(sums, "hecatron-darwin-arm64"); ok {
		t.Errorf("found a line that isn't there")
	}
}

func TestDue(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		checked time.Time
		want    bool
	}{
		{time.Time{}, true},
		{now.Add(-time.Hour), false},
		{now.Add(-23 * time.Hour), false},
		{now.Add(-24 * time.Hour), true},
		{now.Add(2 * time.Hour), true}, // a clock set back
	} {
		if got := (Cache{Checked: c.checked}).Due(now); got != c.want {
			t.Errorf("checked %v: due %v", c.checked, got)
		}
	}
	if got := (Cache{Latest: "0.6.0"}).Newer("0.5.0"); got != "0.6.0" {
		t.Errorf("Newer: %q", got)
	}
	if got := (Cache{Latest: "0.5.0"}).Newer("0.5.0"); got != "" {
		t.Errorf("Newer, same: %q", got)
	}
}

func TestCacheRoundTrip(t *testing.T) {
	home := t.TempDir()
	if got := ReadCache(home); !got.Checked.IsZero() {
		t.Errorf("no cache: %+v", got)
	}
	want := Cache{Checked: time.Now().UTC().Truncate(time.Second), Latest: "1.2.3"}
	if err := WriteCache(home, want); err != nil {
		t.Fatal(err)
	}
	if got := ReadCache(home); !got.Checked.Equal(want.Checked) || got.Latest != want.Latest {
		t.Errorf("got %+v", got)
	}
}

func TestNoticeAllowed(t *testing.T) {
	env := func(vars ...string) func(string) string {
		return func(name string) string {
			for _, v := range vars {
				if k, value, _ := strings.Cut(v, "="); k == name {
					return value
				}
			}
			return ""
		}
	}
	for _, c := range []struct {
		name    string
		getenv  func(string) string
		command string
		args    []string
		stdout  bool
		stderr  bool
		want    bool
	}{
		{"human on a terminal", env(), "test", nil, true, true, true},
		{"stderr not a terminal", env(), "test", nil, true, false, false},
		{"CI", env("CI=true"), "test", nil, true, true, false},
		{"turned off", env("HECATRON_NO_UPDATE_CHECK=1"), "test", nil, true, true, false},
		{"--output agent", env(), "test", []string{"--output", "agent"}, true, true, false},
		{"--output=json", env(), "test", []string{"--output=json"}, true, true, false},
		{"--output junit", env(), "test", []string{"go", "--output", "junit"}, true, true, false},
		{"--output sarif", env(), "lint", []string{"--output=sarif"}, true, true, false},
		{"--output ci", env(), "test", []string{"--output", "ci"}, true, true, false},
		{"--output human", env("HECATRON_OUTPUT=agent"), "test", []string{"--output", "human"}, true, true, true},
		{"HECATRON_OUTPUT=json", env("HECATRON_OUTPUT=json"), "test", nil, true, true, false},
		{"an agent", env("CLAUDECODE=1"), "test", nil, false, true, false},
		{"GitHub Actions", env("GITHUB_ACTIONS=true"), "test", nil, true, true, false},
		{"after --", env(), "parallel", []string{"x", "--", "--output", "json"}, true, true, true},
		{"env", env(), "env", nil, true, true, false},
		{"self update", env(), "self", []string{"update"}, true, true, false},
		{"self test", env(), "self", []string{"test"}, true, true, true},
		{"internal", env(), "_update-check", nil, true, true, false},
	} {
		if got := NoticeAllowed(c.getenv, c.command, c.args, c.stdout, c.stderr); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

func TestClient(t *testing.T) {
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/releases/latest":
			_, _ = w.Write([]byte(`{"tag_name": "v0.6.0", "assets": [{"name": "SHA256SUMS", "browser_download_url": "x"}]}`))
		case "/releases/tags/v0.1.0":
			_, _ = w.Write([]byte(`{"tag_name": "nightly"}`))
		case "/releases/tags/v0.2.0":
			w.WriteHeader(http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	getenv := func(name string) string {
		return map[string]string{"HECATRON_UPDATE_URL": server.URL + "/", "GITHUB_TOKEN": "t0ken"}[name]
	}
	client := NewClient(getenv, "0.5.0", server.Client())
	ctx := context.Background()
	release, err := client.Latest(ctx)
	if err != nil || release.Version != "0.6.0" || release.Tag != "v0.6.0" || release.Assets["SHA256SUMS"] != "x" {
		t.Fatalf("latest: %+v %v", release, err)
	}
	if auth != "Bearer t0ken" {
		t.Errorf("Authorization %q", auth)
	}
	if _, err := client.Get(ctx, "9.9.9"); !IsNoRelease(err) {
		t.Errorf("missing: %v", err)
	}
	if _, err := client.Get(ctx, "0.1.0"); err == nil {
		t.Errorf("a tag that isn't a version was taken")
	}
	if _, err := client.Get(ctx, "v0.2.0"); err == nil || !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Errorf("rate limit: %v", err)
	}
	if _, err := client.Binary(ctx, release); err == nil || !strings.Contains(err.Error(), "no binary for") {
		t.Errorf("no binary: %v", err)
	}
	if NewClient(func(string) string { return "" }, "0.5.0", nil).URL != DefaultURL {
		t.Errorf("default URL")
	}
}
