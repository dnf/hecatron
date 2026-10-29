package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// hct audit's secret scan end to end, with a stand-in gitleaks on PATH. The
// stand-in ignores --redact and puts the secret everywhere it can: in its
// report's Secret and Match, on its stdout and stderr. None of it may reach
// any of hct's outputs.

const fakeGitleaks = `#!/usr/bin/env bash
mode="$1"; shift
while [ "$#" -gt 0 ]; do
    case "$1" in
        --config) cp "$2" "${CAPTURE}/config.toml"; shift ;;
        --report-path) report="$2"; shift ;;
    esac
    shift
done
echo "${mode}" > "${CAPTURE}/mode"
echo "leak: ${FAKE_SECRET}"
echo "leak: ${FAKE_SECRET}" >&2
[ -n "${GITLEAKS_FAIL:-}" ] && { echo "boom ${FAKE_SECRET}" >&2; exit 1; }
if [ -n "${GITLEAKS_CLEAN:-}" ]; then echo '[]' > "${report}"; exit 0; fi
cat > "${report}" <<EOF
[
 {"RuleID": "aws-access-token", "Description": "AWS credentials", "StartLine": 3, "EndLine": 3,
  "Match": "key = ${FAKE_SECRET}", "Secret": "${FAKE_SECRET}", "Line": "key = ${FAKE_SECRET}",
  "File": "src/config.py", "Commit": "", "Fingerprint": "src/config.py:aws-access-token:3"},
 {"RuleID": "aws-access-token", "Description": "AWS credentials", "StartLine": 1, "EndLine": 1,
  "Match": "${FAKE_SECRET}", "Secret": "${FAKE_SECRET}",
  "File": "tests/fixtures/key.txt", "Commit": "", "Fingerprint": "tests/fixtures/key.txt:aws-access-token:1"}
]
EOF
`

// fakeSecret is built here so this file holds no credential-shaped text.
var fakeSecret = "AKIA" + strings.Repeat("Q7", 8)

func secretsProject(t *testing.T, config string) (string, []string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in gitleaks is bash")
	}
	root := fixture(t, map[string]string{
		".hecatron/config.json":  config,
		"src/config.py":          "# config\n\nkey = " + fakeSecret + "\n",
		"tests/fixtures/key.txt": fakeSecret + "\n",
	})
	bin, capture := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "gitleaks"), []byte(fakeGitleaks), 0o755); err != nil {
		t.Fatal(err)
	}
	// hct puts hecatron's bin directory on PATH, where a real gitleaks may
	// be: an empty one.
	env := []string{"HECATRON_HOME=" + repoHome(t), "HECATRON_BUILTIN_LANGUAGES=", "HECATRON_OUTPUT=", "CLAUDECODE=", "GITHUB_ACTIONS=", "GITHUB_WORKSPACE=", "CI_PROJECT_DIR=",
		"XDG_DATA_HOME=" + t.TempDir(), "PATH=" + bin + ":" + os.Getenv("PATH"), "CAPTURE=" + capture, "FAKE_SECRET=" + fakeSecret}
	return root, env, capture
}

const secretsConfig = `{"schema": 1, "project": "s", "languages": [], "implPaths": {}%s}`

func TestAuditSecretsNeverShowsTheSecret(t *testing.T) {
	root, env, capture := secretsProject(t, strings.Replace(secretsConfig, "%s", "", 1))
	reportDir, summary := t.TempDir(), filepath.Join(t.TempDir(), "summary.md")
	env = append(env, "HECATRON_REPORT_DIR="+reportDir, "GITHUB_STEP_SUMMARY="+summary)
	outputs := map[string]string{}
	for _, format := range []string{"human", "agent", "json", "ci", "sarif", "junit"} {
		got := runIn(t, root, env, binary, "audit", "secrets", "--output", format)
		if got.status != 1 {
			t.Errorf("%s: status %d, want 1: %+v", format, got.status, got)
		}
		outputs[format] = got.stdout + got.stderr
	}
	all := runIn(t, root, env, binary, "audit", "--output", "agent")
	if all.status != 1 || !strings.Contains(all.stdout, "secrets") {
		t.Errorf("hct audit runs the secret scan: %+v", all)
	}
	outputs["audit"] = all.stdout + all.stderr
	files, _ := filepath.Glob(filepath.Join(reportDir, "*"))
	files = append(files, summary)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		outputs[filepath.Base(file)] = string(data)
	}
	logs, _ := filepath.Glob(filepath.Join(root, ".hecatron", "logs", "audit", "*"))
	for _, file := range logs {
		data, _ := os.ReadFile(file)
		outputs[file] = string(data)
	}
	for _, name := range []string{"audit-secrets.sarif", "summary.md"} {
		if _, ok := outputs[name]; !ok {
			t.Errorf("no %s among %v", name, files)
		}
	}
	for name, text := range outputs {
		if strings.Contains(text, fakeSecret) || strings.Contains(text, fakeSecret[4:]) {
			t.Errorf("%s shows the secret:\n%s", name, text)
		}
	}
	if human := outputs["human"]; !strings.Contains(human, "aws-access-token  src/config.py:3  fingerprint src/config.py:aws-access-token:3") ||
		!strings.Contains(human, "secrets: 2 possible secrets in the working tree") {
		t.Errorf("human:\n%s", human)
	}
	sarif := outputs["sarif"]
	for _, want := range []string{`"ruleId": "aws-access-token"`, `"uri": "src/config.py"`, `"startLine": 3`, `"secret"`,
		`"fingerprint": "src/config.py:aws-access-token:3"`} {
		if !strings.Contains(sarif, want) {
			t.Errorf("sarif lacks %s:\n%s", want, sarif)
		}
	}
	if !strings.Contains(outputs["summary.md"], "src/config.py:3") {
		t.Errorf("summary:\n%s", outputs["summary.md"])
	}
	if mode, _ := os.ReadFile(filepath.Join(capture, "mode")); string(mode) != "dir\n" {
		t.Errorf("mode %q, want dir", mode)
	}
	if history := runIn(t, root, env, binary, "audit", "secrets", "--history"); history.status != 1 ||
		!strings.Contains(history.stdout, "in the git history") {
		t.Errorf("--history: %+v", history)
	}
	if mode, _ := os.ReadFile(filepath.Join(capture, "mode")); string(mode) != "git\n" {
		t.Errorf("--history: mode %q, want git", mode)
	}
	failing := runIn(t, root, append(env, "GITLEAKS_FAIL=1"), binary, "audit", "secrets")
	if failing.status != 1 || !strings.Contains(failing.stdout, "gitleaks failed") {
		t.Errorf("gitleaks failing: %+v", failing)
	}
	// gitleaks' own error output is shown when it fails; that's gitleaks'
	// to redact, and the stand-in doesn't.
	if clean := runIn(t, root, append(env, "GITLEAKS_CLEAN=1"), binary, "audit", "secrets"); clean.status != 0 ||
		!strings.Contains(clean.stdout, "secrets: none found in the working tree") {
		t.Errorf("clean: %+v", clean)
	}
}

func TestAuditSecretsIgnore(t *testing.T) {
	config := strings.Replace(secretsConfig, "%s", `, "audit": {"secrets": {"ignore": ["tests/fixtures/"]}}`, 1)
	root, env, capture := secretsProject(t, config)
	if err := os.WriteFile(filepath.Join(root, ".gitleaks.toml"), []byte("[extend]\nuseDefault = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := runIn(t, root, env, binary, "audit", "secrets")
	if got.status != 1 || strings.Contains(got.stdout, "tests/fixtures") || !strings.Contains(got.stdout, "1 possible secret in") {
		t.Errorf("ignored fixtures: %+v", got)
	}
	toml, err := os.ReadFile(filepath.Join(capture, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`path = "` + filepath.Join(root, ".gitleaks.toml") + `"`, `"^tests/fixtures/"`, `"(^|/)\\.git/"`} {
		if !strings.Contains(string(toml), want) {
			t.Errorf("config lacks %s:\n%s", want, toml)
		}
	}
	if check := runIn(t, root, env, binary, "check"); strings.Contains(check.stdout, "audit.secrets") {
		t.Errorf("check of a good setting: %+v", check)
	}
	bad := strings.Replace(secretsConfig, "%s", `, "audit": {"secrets": {"ignore": "tests", "other": 1}}`, 1)
	root, env, _ = secretsProject(t, bad)
	check := runIn(t, root, env, binary, "check")
	for _, want := range []string{"audit.secrets.other isn't a setting (ignore)", "audit.secrets.ignore must be a list of paths"} {
		if check.status == 0 || !strings.Contains(check.stdout, want) {
			t.Errorf("check lacks %q: %+v", want, check)
		}
	}
}

func TestAuditSecretsNotInstalled(t *testing.T) {
	root, env, _ := secretsProject(t, strings.Replace(secretsConfig, "%s", "", 1))
	env = append(env, "PATH=/usr/bin:/bin")
	if got := runIn(t, root, env, binary, "audit", "secrets"); got.status != 1 || !strings.Contains(got.stdout, "hct install secrets installs it") {
		t.Errorf("named: %+v", got)
	}
	if got := runIn(t, root, env, binary, "audit", "--output", "agent"); got.status != 0 || strings.Contains(got.stdout, "secrets") {
		t.Errorf("with the rest, not set up: %+v", got)
	}
	configured := strings.Replace(secretsConfig, "%s", `, "audit": {"secrets": {}}`, 1)
	if err := os.WriteFile(filepath.Join(root, ".hecatron", "config.json"), []byte(configured), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := runIn(t, root, env, binary, "audit", "--output", "human"); got.status != 0 || !strings.Contains(got.stdout, "secrets") {
		t.Errorf("with the rest, audit.secrets set: %+v", got)
	}
	if plan := runIn(t, root, env, binary, "install", "secrets", "--plan"); plan.status != 0 ||
		!strings.Contains(plan.stdout, "secrets: gitleaks ") {
		t.Errorf("install --plan: %+v", plan)
	}
}
