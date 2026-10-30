package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// hct sbom end to end, on hct licenses' fixture project with its stand-in
// osv-scanner. XDG_DATA_HOME is a temporary directory, so hecatron's bin
// directory, which hct puts first on PATH, holds no real osv-scanner.
func TestSBOMCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in osv-scanner is bash")
	}
	root, env := licensesProject(t, strings.Replace(licensesConfig, "%s", "", 1))
	env = append(env, "XDG_DATA_HOME="+t.TempDir(), "SOURCE_DATE_EPOCH=1700000000")
	got := runIn(t, root, env, binary, "sbom")
	var bom struct {
		BOMFormat, SpecVersion, SerialNumber string
		Metadata                             struct {
			Timestamp string
			Component struct{ Name string }
			Tools     struct {
				Components []struct{ Name, Version string }
			}
		}
		Components []struct {
			Name, Purl, Scope string
		}
	}
	if got.status != 0 || json.Unmarshal([]byte(got.stdout), &bom) != nil || bom.BOMFormat != "CycloneDX" ||
		bom.SpecVersion != "1.6" || bom.Metadata.Timestamp != "2023-11-14T22:13:20Z" || bom.Metadata.Component.Name != "lic" ||
		len(bom.Metadata.Tools.Components) != 1 || bom.Metadata.Tools.Components[0].Name != "hecatron" {
		t.Fatalf("sbom: %+v", got)
	}
	purls := map[string]string{}
	for _, c := range bom.Components {
		purls[c.Purl] = c.Scope
	}
	if purls["pkg:npm/lodash@4.17.21"] != "required" || purls["pkg:npm/is-number@7.0.0"] != "optional" ||
		purls["pkg:pub/lints@3.0.0"] != "optional" || purls["pkg:pypi/requests@2.31.0"] != "required" {
		t.Errorf("packages: %v", purls)
	}
	if !strings.Contains(got.stderr, "hecatron: sbom: dart: listed without licenses: ") ||
		!strings.Contains(got.stderr, "hecatron: sbom: zig: not listed: ") {
		t.Errorf("notes: %q", got.stderr)
	}
	if again := runIn(t, root, env, binary, "sbom"); again.stdout != got.stdout {
		t.Error("SOURCE_DATE_EPOCH doesn't make the SBOM reproducible")
	}

	out := filepath.Join(t.TempDir(), "bom.spdx.json")
	spdx := runIn(t, root, env, binary, "sbom", "--format", "spdx", "-o", out, "--no-dev", "ts")
	data, _ := os.ReadFile(out)
	text := string(data)
	if spdx.status != 0 || spdx.stdout != "" || !strings.Contains(text, `"spdxVersion": "SPDX-2.3"`) ||
		!strings.Contains(text, "pkg:npm/lodash@4.17.21") || strings.Contains(text, "pkg:npm/is-number@") ||
		strings.Contains(text, "pkg:pypi/") {
		t.Errorf("--format spdx -o: %+v\n%s", spdx, text)
	}

	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	git("init", "-q")
	git("commit", "-q", "--allow-empty", "-m", "x")
	head, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if inGit := runIn(t, root, env, binary, "sbom", "--format=spdx", "python"); !strings.Contains(inGit.stdout,
		`"sourceInfo": "git commit `+strings.TrimSpace(string(head))+`"`) {
		t.Errorf("the commit: %+v", inGit)
	}

	for _, args := range [][]string{{"sbom", "nope"}, {"sbom", "--bogus"}, {"sbom", "-c", "zz"}, {"sbom", "--format", "xml"},
		{"sbom", "-o"}} {
		if got := runIn(t, root, env, append([]string{binary}, args...)...); got.status != 2 || got.stderr == "" {
			t.Errorf("%v: %+v", args, got)
		}
	}
	if got := runIn(t, root, append(env, "SOURCE_DATE_EPOCH=yesterday"), binary, "sbom"); got.status != 2 ||
		!strings.Contains(got.stderr, "SOURCE_DATE_EPOCH") {
		t.Errorf("a bad SOURCE_DATE_EPOCH: %+v", got)
	}
	if help := runIn(t, root, env, binary, "sbom", "help"); help.status != 0 || !strings.HasPrefix(help.stdout, "Usage: hecatron sbom") {
		t.Errorf("help: %+v", help)
	}
	noScanner := append(append([]string{}, env...), "PATH=/usr/bin:/bin")
	if got := runIn(t, root, noScanner, binary, "sbom", "python"); got.status != 3 ||
		!strings.Contains(got.stderr, "hecatron: sbom: OSV-Scanner isn't installed; hecatron install python installs it") {
		t.Errorf("without osv-scanner: %+v", got)
	}
}
