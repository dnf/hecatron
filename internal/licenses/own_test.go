package licenses

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// One fixture (tests/licenses/fixtures/<name>) as the whole project, its
// components in the directories given, and a scanner answering for each
// directory with its capture: licenses-<name>.json for the root,
// licenses-<name>--<dir with / as ->.json for a subdirectory (capture.sh's
// capture-dirs).
func oneFixture(t *testing.T, name string, targets ...Target) (string, []Target, Scanner) {
	t.Helper()
	home := repoHome(t)
	root := t.TempDir()
	copyTree(t, filepath.Join(home, "tests", "licenses", "fixtures", name), root)
	targets = append([]Target{}, targets...)
	for i := range targets {
		targets[i].Dir = filepath.Join(root, filepath.FromSlash(targets[i].Dir))
	}
	scan := func(dir string) ([]byte, error) {
		capture := "licenses-" + name + ".json"
		if rel, _ := filepath.Rel(root, dir); rel != "." {
			capture = "licenses-" + name + "--" + strings.ReplaceAll(filepath.ToSlash(rel), "/", "-") + ".json"
		}
		data, err := os.ReadFile(filepath.Join(home, "tests", "output", "osv-scanner", "2.7.0", capture))
		if err != nil {
			return nil, err
		}
		return []byte(strings.ReplaceAll(string(data), capturedAt+"/"+name, root)), nil
	}
	return root, targets, scan
}

func keys(packages []Package) []string {
	var out []string
	for _, p := range packages {
		out = append(out, p.Key()+"@"+p.Version+" "+p.Component)
	}
	sort.Strings(out)
	return out
}

func TestTheProjectsOwnPackages(t *testing.T) {
	cases := []struct {
		fixture  string
		target   Target
		own      []string
		packages []string
	}{
		// uv.lock's editable root (a dev version) and workspace member.
		{"python-uv", Target{Lang: "python", Component: ".", Dir: "."},
			[]string{"pypi:uv-proj@0.27.0.post18.dev0+g1a2b3c4 .", "pypi:uvproj-lib@0.1.0 ."},
			[]string{"pypi:iniconfig@2.0.0 .", "pypi:six@1.16.0 ."}},
		// npm's workspaces, linked from node_modules.
		{"npm-workspaces", Target{Lang: "javascript", Component: ".", Dir: "."},
			[]string{"npm:@ws/app@0.1.0 .", "npm:@ws/util@0.2.0 ."},
			[]string{"npm:is-number@7.0.0 .", "npm:left-pad@1.3.0 ."}},
		// go.mod's replace with a directory, named by its path.
		{"go-replace", Target{Lang: "go", Component: "app", Dir: "app"},
			[]string{"go:../shared@ app"},
			[]string{"go:github.com/google/uuid@1.6.0 app"}},
		// Gemfile.lock's PATH gem: the gemspec's.
		{"ruby-gem", Target{Lang: "ruby", Component: ".", Dir: "."},
			[]string{"rubygems:mygem@0.3.0 ."},
			[]string{"rubygems:rack@2.2.8 ."}},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			root, targets, scan := oneFixture(t, c.fixture, c.target)
			r, err := Gather(root, targets, scan)
			if err != nil {
				t.Fatal(err)
			}
			if got := keys(r.Own); strings.Join(got, "|") != strings.Join(c.own, "|") {
				t.Errorf("own: %q, want %q", got, c.own)
			}
			if got := keys(r.Packages); strings.Join(got, "|") != strings.Join(c.packages, "|") {
				t.Errorf("packages: %q, want %q", got, c.packages)
			}
			if n := len(r.Unknown()); n != 0 {
				t.Errorf("%d unknown: %+v", n, r.Unknown())
			}
			var out strings.Builder
			if err := Write(&out, r, "human"); err != nil || !strings.Contains(out.String(), "The project's own packages (not counted):\n  "+strings.Fields(c.own[0])[0][:strings.LastIndex(strings.Fields(c.own[0])[0], "@")]) {
				t.Errorf("human output:\n%s", out.String())
			}
		})
	}
}

// A component's manifest names it: a sibling component's lock file that
// lists it lists the project's own package.
func TestOwnNamesFromManifests(t *testing.T) {
	root := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("py/pyproject.toml", "[project]\nversion = \"1\"\nname = \"My_Lib\"\n\n[tool.x]\nname = \"other\"\n")
	write("web/package.json", `{"name": "web", "workspaces": {"packages": ["pkgs/*"]}}`)
	write("web/pkgs/ui/package.json", `{"name": "@web/ui"}`)
	write("pn/package.json", `{"name": "pn"}`)
	write("pn/pnpm-workspace.yaml", "packages:\n  - 'libs/*'\n  - \"!libs/skip\"\n")
	write("pn/libs/core/package.json", `{"name": "@pn/core"}`)
	write("rs/Cargo.toml", "[package]\nname = \"crate-a\"\n")
	write("gomod/go.mod", "module example.com/m\n\ngo 1.22\n")
	write("php/composer.json", `{"name": "Acme/Thing"}`)
	targets := []Target{{Dir: filepath.Join(root, "py")}, {Dir: filepath.Join(root, "web")}, {Dir: filepath.Join(root, "pn")},
		{Dir: filepath.Join(root, "rs")}, {Dir: filepath.Join(root, "gomod")}, {Dir: filepath.Join(root, "php")}}
	names := ownNames(targets)
	for _, want := range []struct{ ecosystem, name string }{
		{"PyPI", "my-lib"}, {"npm", "web"}, {"npm", "@web/ui"}, {"npm", "pn"}, {"npm", "@pn/core"},
		{"crates.io", "crate-a"}, {"Go", "example.com/m"}, {"Packagist", "acme/thing"},
	} {
		if !names[want.ecosystem][want.name] {
			t.Errorf("%s %s not found in %v", want.ecosystem, want.name, names)
		}
	}
	if names["PyPI"]["other"] {
		t.Error("a name outside [project] counted")
	}
	if got := packageName("PyPI", "Zope.Interface__x"); got != "zope-interface-x" {
		t.Errorf("packageName: %q", got)
	}
}

func TestOwnFromLockFiles(t *testing.T) {
	dir := t.TempDir()
	poetry := filepath.Join(dir, "poetry.lock")
	if err := os.WriteFile(poetry, []byte(`[[package]]
name = "localpkg"
version = "0.1.0"

[package.source]
type = "directory"
url = "../localpkg"

[[package]]
name = "requests"
version = "2.31.0"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if own := lockPackagesFromDirectories(poetry); len(own) != 1 || own["localpkg"] != "0.1.0" {
		t.Errorf("poetry: %v", own)
	}
	composer := filepath.Join(dir, "composer.lock")
	if err := os.WriteFile(composer, []byte(`{"packages": [{"name": "acme/local", "dist": {"type": "path", "url": "../local"}},
		{"name": "monolog/monolog", "dist": {"type": "zip"}}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if own := composerPathPackages(composer); len(own) != 1 || !(lockInfo{own: own}).isOwn("acme/local", "1.0") {
		t.Errorf("composer: %v", own)
	}
	nuget := filepath.Join(dir, "packages.lock.json")
	if err := os.WriteFile(nuget, []byte(`{"dependencies": {"net8.0": {"Core": {"type": "Project"}, "Newtonsoft.Json": {"type": "Direct"}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if directs, own := nugetDirect(nuget); directs["Newtonsoft.Json"] != "direct" || len(own) != 1 || own["Core"] != "" {
		t.Errorf("nuget: %v %v", directs, own)
	}
}

// A monorepo whose components are subdirectories with lock files of their
// own (serverless layers): each component's packages come from its own
// lock file, none twice; a lock file outside every component is warned of.
func TestSubdirectoryLockFiles(t *testing.T) {
	layers := []Target{{Lang: "javascript", Component: "a", Dir: "layers/a/nodejs"},
		{Lang: "javascript", Component: "b", Dir: "layers/b/nodejs"}}
	root, targets, scan := oneFixture(t, "monorepo", layers...)
	r, err := Gather(root, targets, scan)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"npm:is-number@7.0.0 b", "npm:left-pad@1.3.0 a", "npm:lodash@4.17.21 b"}
	if got := keys(r.Packages); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("layers: %q", got)
	}
	r.WarnUnread(root, targets)
	if len(r.Warnings) != 1 || r.Warnings[0] != "javascript: package-lock.json is outside every javascript component's directory, "+
		"so its packages aren't listed (hct list shows the components)" {
		t.Errorf("warnings: %q", r.Warnings)
	}

	// With the root a component too, its scan finds every lock file below:
	// each still counts once, for the deepest component holding it.
	withRoot := append([]Target{{Lang: "javascript", Component: "root", Dir: "."}}, layers...)
	root, targets, scan = oneFixture(t, "monorepo", withRoot...)
	if r, err = Gather(root, targets, scan); err != nil {
		t.Fatal(err)
	}
	want = []string{"npm:ansi-styles@4.3.0 root", "npm:chalk@4.1.2 root", "npm:color-convert@2.0.1 root",
		"npm:color-name@1.1.4 root", "npm:has-flag@4.0.0 root", "npm:is-number@7.0.0 b", "npm:left-pad@1.3.0 a",
		"npm:lodash@4.17.21 b", "npm:supports-color@7.2.0 root"}
	if got := keys(r.Packages); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("root and layers: %q", got)
	}
	r.WarnUnread(root, targets)
	if len(r.Warnings) != 0 {
		t.Errorf("warnings: %q", r.Warnings)
	}
}

func TestWarnUnreadSkipsInstalledPackagesAndNamesFive(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"node_modules/x/package-lock.json", ".cache/yarn.lock", "app/package-lock.json",
		"t1/yarn.lock", "t2/yarn.lock", "t3/yarn.lock", "t4/yarn.lock", "t5/yarn.lock", "app/fixtures/yarn.lock",
		"svc/Cargo.lock"} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := &Report{}
	r.WarnUnread(root, []Target{{Lang: "typescript", Component: "web", Dir: filepath.Join(root, "web")},
		{Lang: "zig", Component: "svc", Dir: filepath.Join(root, "svc")}})
	want := "typescript: app/package-lock.json, t1/yarn.lock, t2/yarn.lock, t3/yarn.lock, t4/yarn.lock and 1 more are " +
		"outside every typescript component's directory, so their packages aren't listed (hct list shows the components)"
	if len(r.Warnings) != 1 || r.Warnings[0] != want {
		t.Errorf("got %q", r.Warnings)
	}
}
