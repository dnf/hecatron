package secrets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var fakeSecret = "AKIA" + strings.Repeat("Z3", 8)

func TestParseKeepsNothingOfTheSecret(t *testing.T) {
	report := `[{"RuleID": "aws-access-token", "Description": "AWS", "StartLine": 4, "StartColumn": 2,
		"Match": "key=` + fakeSecret + `", "Secret": "` + fakeSecret + `", "Line": "key=` + fakeSecret + `",
		"File": "a/b.py", "Commit": "abc", "Fingerprint": "abc:a/b.py:aws-access-token:4", "Entropy": 3.5}]`
	findings, err := Parse([]byte(report))
	if err != nil {
		t.Fatal(err)
	}
	want := Finding{Rule: "aws-access-token", Description: "AWS", File: "a/b.py", Line: 4, Commit: "abc", Fingerprint: "abc:a/b.py:aws-access-token:4"}
	if len(findings) != 1 || findings[0] != want {
		t.Fatalf("got %+v", findings)
	}
	data, _ := json.Marshal(findings)
	if strings.Contains(string(data), fakeSecret) {
		t.Errorf("a finding holds the secret: %s", data)
	}
	var b strings.Builder
	Write(&b, findings, true)
	if strings.Contains(b.String(), fakeSecret) || !strings.Contains(b.String(), "a/b.py:4 (commit abc)") {
		t.Errorf("Write:\n%s", b.String())
	}
	if _, err := Parse([]byte("not json")); err == nil {
		t.Error("no error for a broken report")
	}
}

func TestGlobRegexp(t *testing.T) {
	cases := []struct {
		pattern string
		match   []string
		miss    []string
	}{
		{"tests/fixtures/", []string{"tests/fixtures/a.txt", "tests/fixtures/x/y"}, []string{"tests/fixtures", "src/tests/fixtures/a"}},
		{"tests/fixtures", []string{"tests/fixtures", "tests/fixtures/a"}, []string{"tests/fixturesX"}},
		{"*.pem", []string{"key.pem"}, []string{"a/key.pem", "key.pem.bak"}},
		{"**/*.pem", []string{"key.pem", "a/b/key.pem"}, []string{"key.pemx"}},
		{"./docs/**", []string{"docs/a", "docs/a/b"}, []string{"src/docs/a"}},
		{"a?c.txt", []string{"abc.txt"}, []string{"a/c.txt"}},
		{"a+b(1).txt", []string{"a+b(1).txt"}, []string{"aab1.txt"}},
	}
	for _, c := range cases {
		re := regexp.MustCompile(GlobRegexp("", c.pattern))
		for _, path := range c.match {
			if !re.MatchString(path) {
				t.Errorf("%s doesn't match %s (%s)", c.pattern, path, re)
			}
		}
		for _, path := range c.miss {
			if re.MatchString(path) {
				t.Errorf("%s matches %s (%s)", c.pattern, path, re)
			}
		}
	}
	if re := regexp.MustCompile(GlobRegexp("sub/", "fixtures/")); !re.MatchString("sub/fixtures/a") || re.MatchString("fixtures/a") {
		t.Errorf("with a prefix: %s", re)
	}
}

func TestConfig(t *testing.T) {
	root := t.TempDir()
	if got := Config(root, nil); !strings.Contains(got, "useDefault = true") || strings.Contains(got, "allowlists") {
		t.Errorf("no project config:\n%s", got)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitleaks.toml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got := Config(root, []string{`^a\.b/`, "q\"\\\n"})
	for _, want := range []string{`path = "` + filepath.Join(root, ".gitleaks.toml") + `"`, `"^a\\.b/",`, `"q\"\\\u000A",`, "[[allowlists]]"} {
		if !strings.Contains(got, want) {
			t.Errorf("lacks %s:\n%s", want, got)
		}
	}
}

func TestRelease(t *testing.T) {
	for _, platform := range [][2]string{{"linux", "amd64"}, {"linux", "arm64"}, {"darwin", "amd64"}, {"darwin", "arm64"}} {
		r, ok := Release(platform[0], platform[1])
		if !ok || len(r.SHA256) != 64 || !strings.Contains(r.URL, "/v"+Version+"/gitleaks_"+Version+"_") {
			t.Errorf("%v: %+v", platform, r)
		}
	}
	if _, ok := Release("plan9", "386"); ok {
		t.Error("a release for plan9")
	}
}
