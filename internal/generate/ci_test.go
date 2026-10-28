package generate

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnf/hecatron/internal/install"
	"github.com/dnf/hecatron/internal/project"
)

// The generated CI is checked against golden files in testdata/ci (go test
// ./internal/generate -update rewrites them); tests/test_generated_ci.py
// checks that they parse and that the GitLab pipeline matches GitLab's
// schema.

var update = flag.Bool("update", false, "rewrite the golden files")

// testVersion is the hecatron version the golden files are generated with.
const testVersion = "0.4.0"

const ciTemplate = `name: Rust
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - run: hct test rust
  lint:
    runs-on: ubuntu-latest
    steps:
      - name: Check out
        uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - run: hct lint rust
`

// ciFixture is a project with two components and languages with and
// without caches: bash (none), go (dependencies only), python and rust
// (toolchain and dependencies) and elixir (a directory in each component).
func ciFixture(t *testing.T, config string) *project.Project {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".hecatron/config.json":                 config,
		".hecatron/languages/rust/workflow.yml": ciTemplate,
		"age/rust/Cargo.toml":                   "",
		"date/rust/Cargo.toml":                  "",
		"age/python/pyproject.toml":             "",
		"age/go/go.mod":                         "module x\n",
		"age/bash/test.sh":                      "",
		"age/elixir/mix.exs":                    "",
		"date/elixir/mix.exs":                   "",
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HECATRON_BUILTIN_LANGUAGES", "")
	home, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(root, project.NewHome(home))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

const ciConfig = `{"schema": 1, "project": "fx", "components": ["age", "date"],
	"languages": ["bash", "elixir", "go", "python", "rust"],
	"generated": {"workflows": ".github/workflows/{lang}.yml", "ci": ["github", "gitlab"]}%s}`

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "ci", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (go test ./internal/generate -update writes it)", err)
	}
	if got != string(want) {
		t.Errorf("%s differs from what's generated (go test ./internal/generate -update rewrites it):\n%s", path, got)
	}
}

func TestGitLabCI(t *testing.T) {
	for _, tc := range []struct{ name, extra string }{
		{"gitlab-ci.yml", ""},
		{"gitlab-ci-cache.yml", `, "ci": {"cache": true, "image": "debian:13"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := ciFixture(t, strings.Replace(ciConfig, "%s", tc.extra, 1))
			settings, problems := p.CISettings()
			if len(problems) > 0 {
				t.Fatal(problems)
			}
			got, err := GitLabCI(p, settings, testVersion)
			if err != nil {
				t.Fatal(err)
			}
			golden(t, tc.name, got)
		})
	}
}

func TestGitHubWorkflowCache(t *testing.T) {
	p := ciFixture(t, strings.Replace(ciConfig, "%s", `, "ci": {"cache": true}`, 1))
	plain, ok, err := Workflow(p, "rust")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if !strings.HasSuffix(plain, WithTimeouts(ciTemplate, project.DefaultTimeoutMinutes)) {
		t.Error("a workflow without cache isn't its template, with time limits, under the banner")
	}
	cached, _, err := CachedWorkflow(p, "rust", testVersion)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "github-rust-cache.yml", cached)
	if got := strings.Count(cached, "uses: actions/cache@v4"); got != 4 {
		t.Errorf("want two cache steps in each of the two jobs, got %d", got)
	}
}

func TestWithGitHubCacheLeavesOtherWorkflows(t *testing.T) {
	c := languageCache{lang: "x", caches: []cacheEntry{{dependencyCache, []string{"~/.x"}, []string{"**/x.lock"}, ""}}}
	for _, body := range []string{
		"jobs:\n  t:\n    steps:\n      - run: true\n",            // no checkout
		"jobs:\n  t:\n    uses: actions/checkout@v4\n",            // not a step
		"jobs:\n  t:\n    steps:\n      - uses: actions/checkout", // no version
	} {
		if got := withGitHubCache(body, c, "v1"); got != body {
			t.Errorf("changed:\n%s", got)
		}
	}
	if got := withGitHubCache("jobs:\n  t:\n    steps:\n    - uses: actions/checkout@v4\n    - run: true\n", c, "v1"); !strings.Contains(got,
		"    - uses: actions/checkout@v4\n    - name: Cache x dependencies\n      uses: actions/cache@v4\n") ||
		!strings.HasSuffix(got, "    - run: true\n") {
		t.Errorf("cache step not after a checkout at the steps' own indent:\n%s", got)
	}
}

func TestCISettingsProblems(t *testing.T) {
	for config, want := range map[string]string{
		`"generated": {"ci": "jenkins"}`:            "generated.ci must be github or gitlab, or a list of them",
		`"generated": {"ci": ["gitlab", "gitlab"]}`: "generated.ci must be github or gitlab, or a list of them",
		`"ci": []`:                                              `"ci" must be an object`,
		`"ci": {"cache": "yes"}`:                                "ci.cache must be true or false",
		`"ci": {"image": ""}`:                                   "ci.image must be an image name (ubuntu:24.04)",
		`"ci": {"timeout": 5}`:                                  "ci.timeout is not a setting (cache, image, timeoutMinutes, languages)",
		`"ci": {"timeoutMinutes": 0}`:                           "ci.timeoutMinutes must be a whole number of minutes, 1 or more",
		`"ci": {"timeoutMinutes": 1.5}`:                         "ci.timeoutMinutes must be a whole number of minutes, 1 or more",
		`"ci": {"languages": [1]}`:                              `ci.languages must be an object of languages to minutes ({"pascal": 120})`,
		`"ci": {"languages": {"go": "long"}}`:                   "ci.languages.go must be a whole number of minutes, 1 or more",
		`"ci": {"timeoutMinutes": 30, "languages": {"go": 90}}`: "",
	} {
		p := ciFixture(t, `{"schema": 1, "languages": [], `+config+`}`)
		_, problems := p.CISettings()
		if want == "" && len(problems) > 0 || want != "" && (len(problems) != 1 || problems[0] != want) {
			t.Errorf("%s: got %q, want %q", config, problems, want)
		}
	}
	p := ciFixture(t, `{"schema": 1, "languages": []}`)
	if settings, problems := p.CISettings(); len(problems) > 0 || len(settings.Systems) != 1 ||
		settings.Systems[0] != project.CIGitHub || settings.Cache || settings.Image != project.DefaultCIImage {
		t.Errorf("defaults: %+v %q", settings, problems)
	}
}

// Every built-in definition's cache is usable: home directories under ~/
// or ones relative to a component's directory, none inside another (a
// GitLab job links each), and keys GitLab takes.
func TestBuiltinCaches(t *testing.T) {
	languages, err := filepath.Glob("../../languages/*/config.json")
	if err != nil || len(languages) == 0 {
		t.Fatal(err)
	}
	var config strings.Builder
	config.WriteString(`{"schema": 1, "languages": [`)
	for i, path := range languages {
		if i > 0 {
			config.WriteString(", ")
		}
		config.WriteString(`"` + filepath.Base(filepath.Dir(path)) + `"`)
	}
	config.WriteString("]}")
	p := ciFixture(t, config.String())
	for _, lang := range p.Languages {
		spec, err := p.Cache(lang)
		if err != nil {
			t.Fatal(err)
		}
		dirs := append(append([]string(nil), spec.Toolchain...), spec.Dependencies...)
		for i, dir := range dirs {
			if strings.Contains(dir, "..") || strings.HasPrefix(dir, "/") || strings.HasSuffix(dir, "/") || dir == "~/" {
				t.Errorf("%s: cache directory %q must be under ~/ or a component's directory", lang, dir)
			}
			for j, other := range dirs {
				if i != j && (dir == other || strings.HasPrefix(dir, other+"/")) {
					t.Errorf("%s: cache directory %q is inside %q", lang, dir, other)
				}
			}
		}
		if len(spec.LockFiles) > 0 && len(spec.Dependencies) == 0 {
			t.Errorf("%s: lockFiles without dependencies to cache", lang)
		}
		c, err := projectCache(p, lang)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range c.caches {
			if len(entry.keyFiles) > gitlabKeyFiles {
				t.Errorf("%s: %s cache is keyed on %d files; GitLab takes %d", lang, entry.kind, len(entry.keyFiles), gitlabKeyFiles)
			}
		}
	}
}

func TestYAMLScalar(t *testing.T) {
	for in, want := range map[string]string{
		"hct test rust":  "hct test rust",
		"ubuntu:24.04":   "'ubuntu:24.04'",
		"**/Cargo.lock":  "'**/Cargo.lock'",
		"true":           "'true'",
		"1.0":            "'1.0'",
		".5":             "'.5'",
		".tool-versions": ".tool-versions",
		"it's":           "'it''s'",
		"":               "''",
	} {
		if got := yamlScalar(in); got != want {
			t.Errorf("yamlScalar(%q) = %s, want %s", in, got, want)
		}
	}
}

// A toolchain hecatron's executor installs into its own directory is
// cached there, keyed on what it would install: the definition and the
// default version (a literal hash), the project's install.json and pins,
// the platform.
func TestEngineToolchainCache(t *testing.T) {
	p := ciFixture(t, strings.Replace(ciConfig, "%s", `, "ci": {"cache": true}`, 1))
	if dir, _, err := engineToolchain(p, "bash"); err != nil || dir != "" {
		t.Errorf("bash comes from apt, which isn't cached: %q %v", dir, err)
	}
	defer func(saved func(*install.Definition) bool) { engineInstalls = saved }(engineInstalls)
	engineInstalls = func(*install.Definition) bool { return true }
	dir := filepath.Join(p.Root, ".hecatron", "languages", "rust")
	if err := os.WriteFile(filepath.Join(dir, "install.json"), []byte(`{"schema": 1, "toolchain": {"versions": "managed",
		"archive": {"url": "https://example.com/rust-{version}.tar.gz"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := projectCache(p, "rust")
	if err != nil {
		t.Fatal(err)
	}
	toolchain := c.caches[0]
	if toolchain.kind != toolchainCache || len(toolchain.dirs) != 1 || toolchain.dirs[0] != "~/.local/share/hecatron/toolchains/rust" ||
		len(toolchain.data) != 16 || !contains(toolchain.keyFiles, ".hecatron/languages/rust/install.json") {
		t.Errorf("rust's toolchain cache: %+v", toolchain)
	}
	key, restore := githubCacheKey("rust", toolchain, "v1")
	if !strings.HasPrefix(key, "hecatron-rust-toolchain-${{ runner.os }}-v1-${{ runner.arch }}-"+toolchain.data+"-${{ hashFiles(") ||
		!strings.HasPrefix(key, restore) {
		t.Errorf("key %q, restore-keys %q", key, restore)
	}
	yaml, links := gitlabCache(c, "v1")
	if !strings.Contains(strings.Join(yaml, "\n"), "prefix: rust-toolchain-v1-"+toolchain.data) || len(links) == 0 {
		t.Errorf("GitLab: %q %q", yaml, links)
	}
}
