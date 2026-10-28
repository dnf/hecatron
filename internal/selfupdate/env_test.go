//go:build unix

package selfupdate

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The installation directory as install.sh chooses it, and its env
// writer, taken from install.sh itself.
const installTarget = `${HECATRON_INSTALL_DIR:-${HOME}/.hecatron}`

var writeEnvFunction = regexp.MustCompile(`(?ms)^_hecatron_write_env\(\) \{\n.*?^\}\n`)

// EnvFiles writes what install.sh's _hecatron_write_env writes, byte for
// byte, for the directory install.sh installs into (HECATRON_INSTALL_DIR
// or ~/.hecatron), with and without XDG_DATA_HOME.
func TestEnvFilesMatchInstallSh(t *testing.T) {
	script, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), `_hecatron_target="`+installTarget+`"`) {
		t.Fatalf("install.sh no longer chooses its directory as %s; update this test", installTarget)
	}
	function := writeEnvFunction.Find(script)
	if function == nil {
		t.Fatal("no _hecatron_write_env in install.sh")
	}
	for _, c := range []struct {
		name       string
		installDir bool
		xdg        bool
	}{
		{"default directory", false, false},
		{"HECATRON_INSTALL_DIR", true, false},
		{"XDG_DATA_HOME", false, true},
		{"both", true, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			base := t.TempDir()
			vars := map[string]string{"HOME": filepath.Join(base, "home dir"), "HECATRON_INSTALL_DIR": "", "XDG_DATA_HOME": ""}
			if c.installDir {
				vars["HECATRON_INSTALL_DIR"] = filepath.Join(base, "opt", "hecatron $x")
			}
			if c.xdg {
				vars["XDG_DATA_HOME"] = filepath.Join(base, "data")
			}
			home := vars["HECATRON_INSTALL_DIR"]
			if home == "" {
				home = vars["HOME"] + "/.hecatron"
			}
			goDir := filepath.Join(base, "go")
			for _, dir := range []string{home, goDir} {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			// install.sh's function, into the directory it would choose;
			// Go's into another, for that same installation.
			cmd := exec.Command("sh", "-c", string(function)+`
target="`+installTarget+`"
[ "${target}" = "$1" ] || { echo "target ${target}, want $1" >&2; exit 1; }
_hecatron_write_env "${target}"`, "sh", home)
			cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
			for name, value := range vars {
				if value != "" {
					cmd.Env = append(cmd.Env, name+"="+value)
				}
			}
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("install.sh's writer: %v\n%s", err, out)
			}
			if err := WriteEnv(goDir, home, func(name string) string { return vars[name] }); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"env", "env.fish"} {
				want, _ := os.ReadFile(filepath.Join(home, name))
				got, _ := os.ReadFile(filepath.Join(goDir, name))
				if string(got) != string(want) || len(want) == 0 {
					t.Errorf("%s differs:\ninstall.sh:\n%s\nGo:\n%s", name, want, got)
				}
			}
		})
	}
}
