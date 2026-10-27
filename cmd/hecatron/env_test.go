package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// `hecatron env LANG [-c COMPONENT]` prints what bin/hecatron prints (from
// project.py's env query), byte for byte: with the real definitions and the
// toolchains installed on this machine, and with fixture definitions whose
// installed.sh lists toolchains in the project.

var envDefinitions = map[string]string{
	"py/config.json": `{"abbreviation": "py", "dirNames": ["python"], "version": "=3.15.1",
		"pinFiles": [{"file": ".python-version", "format": "python-version"}],
		"supportFiles": [{"file": "pyproject.toml", "format": "python-classifiers"}],
		"oldestInstallable": "3.8", "asOfPrecision": "line",
		"path": ["{home}/bin", "~/.local/bin"], "env": {"UV_PYTHON": "{home}/bin/python3", "Q": "it's {version}"}}`,
	"py/releases.json": `{"releases": [["3.8.10", "2021-05-03"], ["3.9.5", "2021-05-03"], ["3.12.4", "2024-06-06"], ["3.15.1", "2026-09-01"]]}`,
	"py/installed.sh":  `printf '3.12.4\t%s/tc/py312\n3.15.0\t%s/tc/py 315\n3.9.5\t%s/tc/py39\n' "$PWD" "$PWD" "$PWD"`,
	"net/config.json": `{"version": "=10.0.403", "releaseLine": 3, "pinFiles": [{"file": "global.json", "format": "global-json"}],
		"supportFiles": [{"file": ".github/workflows/*.yml", "format": "github-setup", "key": "dotnet-version", "selects": false}],
		"path": ["{home}"], "env": {"DOTNET_ROOT": "{home}"}}`,
	"net/installed.sh": `printf '8.0.100\t%s/tc/net\n10.0.403\t%s/tc/net\n' "$PWD" "$PWD"`,
	"bad/config.json":  `{"version": "=x"}`,
	"odd/config.json":  `{"version": "=1.0", "path": ["{home}/{nope}"]}`,
	"odd/installed.sh": `printf '1.0\t/opt\n'`,
}

var envProjects = map[string]map[string]string{
	"real definitions": {
		".hecatron/config.json": `{"schema": 1, "components": ["a", "b"], "languages": ["python", "java", "go", "rust",
			"csharp", "fsharp", "javascript", "typescript", "ruby", "php", "kotlin", "swift"]}`,
		"a/python/.python-version":       "3.12\n",
		"b/python/pyproject.toml":        "classifiers = ['Programming Language :: Python :: 3.11']\n",
		"a/java/.java-version":           "21\n",
		"b/java/pom.xml":                 "<source>1.7</source>",
		"a/go/go.mod":                    "module x\ntoolchain go1.22.3\n",
		"a/rust/rust-toolchain.toml":     "[toolchain]\nchannel = \"stable\"\n",
		"a/csharp/global.json":           `{"sdk": {"version": "8.0.100", "rollForward": "latestFeature"}}`,
		"b/csharp/x.csproj":              "",
		"a/javascript/.nvmrc":            "20\n",
		"a/typescript/package.json":      "{}",
		"b/ruby/.ruby-version":           "3.3.0\n",
		"a/php/composer.json":            `{"config": {"platform": {"php": "8.2.0"}}}`,
		".github/workflows/ci.yml":       "dotnet-version: 9.0.x\nnode-version: 22\n",
		"b/javascript/package.json":      "{}",
		".hecatron/languages/go/test.sh": "",
	},
	"fixture definitions": {
		".hecatron/config.json":     `{"schema": 1, "components": ["a", "b", "c"], "languages": ["py", "net", "bad", "odd"]}`,
		"a/python/.python-version":  "3.12\n",
		"b/py/pyproject.toml":       "classifiers = ['Programming Language :: Python :: 3.9']\n",
		"c/py/.python-version":      "3.13\n",
		"a/net/x.csproj":            "",
		"b/net/global.json":         `{"sdk": {"version": "8.0.100", "rollForward": "disable"}}`,
		".github/workflows/ci.yml":  "dotnet-version: 9.0.x\n",
		"tc/py312/bin/x":            "",
		"tc/py 315/bin/x":           "",
		"tc/py39/bin/x":             "",
		"tc/net/x":                  "",
		".hecatron/languages/bad/x": "",
	},
}

var envArgs = [][]string{
	{"env", "python"}, {"env", "python", "-c", "a"}, {"env", "python", "-c", "b"}, {"env", "python", "-c", "nope"},
	{"env", "python", "-c"}, {"env", "python", "-c", ""}, {"env", "python", "a"}, {"env", "py"}, {"env", "py", "-c", "a"},
	{"env", "py", "-c", "b"}, {"env", "py", "-c", "c"}, {"env", "net"}, {"env", "net", "-c", "b"},
	{"env", "java"}, {"env", "java", "-c", "b"}, {"env", "go", "-c", "a"}, {"env", "rust"}, {"env", "csharp", "-c", "b"},
	{"env", "fsharp"}, {"env", "javascript", "-c", "a"}, {"env", "typescript"}, {"env", "ruby", "-c", "b"},
	{"env", "php"}, {"env", "kotlin"}, {"env", "swift"}, {"env", "bad"}, {"env", "odd"}, {"env", "nope"}, {"env", ""},
}

func TestLanguageEnvMatchesBinHecatron(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bin/hecatron needs bash and Python")
	}
	for _, tool := range []string{"bash", "python3"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("no " + tool)
		}
	}
	home := repoHome(t)
	definitions := filepath.Join(t.TempDir(), "languages")
	for name, content := range envDefinitions {
		path := filepath.Join(definitions, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	compared := 0
	for name, files := range envProjects {
		root := fixture(t, files)
		builtin := ""
		if name == "fixture definitions" {
			builtin = definitions
		}
		for _, asOf := range []string{"off", "2021-06-01", "2019-01-01", "2021-13-01", "20210601"} {
			env := []string{"HECATRON_HOME=" + home, "HECATRON_BUILTIN_LANGUAGES=" + builtin, "HECATRON_AS_OF=" + asOf, "HECATRON_NO_GO=1"}
			for _, args := range envArgs {
				t.Run(name+"/"+asOf+"/"+strings.Join(args, " "), func(t *testing.T) {
					dir := filepath.Join(root, "a")
					want := runIn(t, dir, env, append([]string{"bash", filepath.Join(home, "bin", "hecatron")}, args...)...)
					got := runIn(t, dir, env, append([]string{binary}, args...)...)
					if got != want {
						t.Errorf("bin/hecatron %q:\n status %d, stdout:\n%s\n stderr:\n%s\nGo: status %d, stdout:\n%s\n stderr:\n%s",
							args, want.status, want.stdout, want.stderr, got.status, got.stdout, got.stderr)
					}
					compared++
				})
			}
		}
	}
	t.Logf("%d env invocations compared with bin/hecatron", compared)
}
