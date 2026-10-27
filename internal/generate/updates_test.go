package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnf/hecatron/internal/project"
)

// updatesFixture is a project with several components and languages whose
// ecosystems depend on the files in each directory: uv or pip, bun or npm,
// Maven or Gradle; bash has none, and Haskell only a Renovate manager.
func updatesFixture(t *testing.T, config string, extra map[string]string) *project.Project {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".hecatron/config.json":             config,
		"age/python/pyproject.toml":         "",
		"age/python/uv.lock":                "",
		"date/python/pyproject.toml":        "",
		"age/javascript/package.json":       "{}",
		"age/javascript/bun.lock":           "",
		"date/javascript/package.json":      "{}",
		"date/javascript/package-lock.json": "{}",
		"age/rust/Cargo.toml":               "",
		"date/rust/Cargo.toml":              "",
		"age/java/pom.xml":                  "",
		"date/java/build.gradle.kts":        "",
		"age/go/go.mod":                     "module x\n",
		"age/csharp/x.csproj":               "",
		"age/bash/test.sh":                  "",
		"age/haskell/x.cabal":               "",
	}
	for name, content := range extra {
		files[name] = content
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

const updatesConfig = `{"schema": 1, "project": "fx", "components": ["age", "date"],
	"languages": ["bash", "csharp", "go", "haskell", "java", "javascript", "python", "rust"],
	"generated": {"workflows": ".github/workflows/{lang}.yml", "dependabot": true, "renovate": "renovate.json"},
	"dependabot": {"schedule": {"interval": "daily", "time": "05:00"}, "open-pull-requests-limit": 10,
		"groups": {"minor-and-patch": {"update-types": ["minor", "patch"]}},
		"ecosystems": {"npm": {"versioning-strategy": "increase", "ignore": [{"dependency-name": "left-pad", "versions": ["2.x"]}]}}},
	"renovate": {"extends": ["config:recommended"], "schedule": ["before 6am on monday"]}}`

func TestDependabotAndRenovate(t *testing.T) {
	p := updatesFixture(t, updatesConfig, nil)
	settings, problems := p.UpdateSettings()
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	if settings.Dependabot != project.DefaultDependabotPath || settings.Renovate != "renovate.json" {
		t.Errorf("paths: %+v", settings)
	}
	dependabot, err := Dependabot(p, settings, true)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "dependabot.yml", dependabot)
	for _, want := range []string{
		"package-ecosystem: uv\n    directory: /age/python", "package-ecosystem: pip\n    directory: /date/python",
		"package-ecosystem: bun\n    directory: /age/javascript", "package-ecosystem: npm\n    directory: /date/javascript",
		"package-ecosystem: maven\n    directory: /age/java", "package-ecosystem: gradle\n    directory: /date/java",
		"package-ecosystem: github-actions\n    directory: /\n",
	} {
		if !strings.Contains(strings.ReplaceAll(dependabot, "'", ""), want) {
			t.Errorf("no %q", want)
		}
	}
	renovate, err := Renovate(p, settings, true)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "renovate.json", renovate)

	// A new component changes them, so hct check notices.
	grown := updatesFixture(t, updatesConfig, map[string]string{"time/rust/Cargo.toml": ""})
	grown.Components = append(grown.Components, "time")
	again, err := Dependabot(grown, settings, true)
	if err != nil || again == dependabot || !strings.Contains(again, "/time/rust") {
		t.Errorf("a new component didn't change dependabot.yml (%v)", err)
	}
}

func TestUpdatesAtTheRoot(t *testing.T) {
	root := updatesFixture(t, `{"schema": 1, "implPath": ".", "languages": ["rust"], "generated": {"renovate": true, "dependabot": ".github/dependabot.yml"}}`,
		map[string]string{"Cargo.toml": ""})
	settings, problems := root.UpdateSettings()
	if len(problems) > 0 || settings.Renovate != project.DefaultRenovatePath {
		t.Fatal(settings, problems)
	}
	dependabot, err := Dependabot(root, settings, false)
	if err != nil || !strings.HasSuffix(dependabot, "updates:\n  - package-ecosystem: cargo\n    directory: /\n    schedule:\n      interval: weekly\n") {
		t.Errorf("%v:\n%s", err, dependabot)
	}
	renovate, err := Renovate(root, settings, false)
	if err != nil || strings.Contains(renovate, "includePaths") || !strings.Contains(renovate, `"enabledManagers": [`+"\n    \"cargo\"\n  ]") {
		t.Errorf("a component at the root is everywhere: %v\n%s", err, renovate)
	}
}

func TestUpdateSettingsProblems(t *testing.T) {
	for config, want := range map[string]string{
		`"generated": {"dependabot": 1}`:           "generated.dependabot must be true or a path (.github/dependabot.yml)",
		`"generated": {"renovate": ""}`:            "generated.renovate must be true or a path (renovate.json)",
		`"dependabot": []`:                         `"dependabot" must be an object of settings for each "updates" entry`,
		`"dependabot": {"directory": "/"}`:         "dependabot.directory is generated",
		`"dependabot": {"ecosystems": {"npm": 1}}`: "dependabot.ecosystems.npm must be an object of settings",
		`"renovate": {"enabledManagers": []}`:      "renovate.enabledManagers is generated",
		`"renovate": "x"`:                          `"renovate" must be an object of Renovate settings`,
	} {
		p := updatesFixture(t, `{"schema": 1, "languages": [], `+config+`}`, nil)
		if _, problems := p.UpdateSettings(); len(problems) != 1 || problems[0] != want {
			t.Errorf("%s: got %q, want %q", config, problems, want)
		}
	}
}

// Every built-in definition's "dependabot" reads.
func TestBuiltinDependabotRules(t *testing.T) {
	p := updatesFixture(t, `{"schema": 1, "languages": []}`, nil)
	defs, err := p.Definitions()
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range defs.Names {
		if _, err := p.DependabotRules(lang); err != nil {
			t.Error(err)
		}
		if _, err := p.RenovateManagers(lang); err != nil {
			t.Error(err)
		}
	}
}
