package selfupdate

import (
	"os"
	"path/filepath"
)

// EnvFiles are install.sh's env (sh, bash, zsh) and env.fish for an
// installation at home, byte for byte as its _hecatron_write_env writes
// them: hecatron's bin/ and the links to the toolchains it installs
// ($XDG_DATA_HOME/hecatron/bin, else ~/.local/share/hecatron/bin) put on
// PATH once. Paths are joined as the shell joins them, not cleaned, so
// they match whatever install.sh was given. A parity test runs install.sh's
// function and compares.
func EnvFiles(home string, getenv func(string) string) (env, fish string) {
	bin := home + "/bin"
	data := getenv("XDG_DATA_HOME")
	if data == "" {
		data = getenv("HOME") + "/.local/share"
	}
	links := data + "/hecatron/bin"
	env = "# hecatron: put the command, and the toolchains it installs, on PATH\n" +
		"# (written by install.sh; sourced from your shell's startup files).\n" +
		"case \":${PATH}:\" in\n" +
		"    *\":" + links + ":\"*) ;;\n" +
		"    *) export PATH=\"" + links + ":${PATH}\" ;;\n" +
		"esac\n" +
		"case \":${PATH}:\" in\n" +
		"    *\":" + bin + ":\"*) ;;\n" +
		"    *) export PATH=\"" + bin + ":${PATH}\" ;;\n" +
		"esac\n"
	fish = "# hecatron: put the command, and the toolchains it installs, on PATH\n" +
		"# (written by install.sh).\n" +
		"if not contains \"" + links + "\" $PATH\n" +
		"    set -gx PATH \"" + links + "\" $PATH\n" +
		"end\n" +
		"if not contains \"" + bin + "\" $PATH\n" +
		"    set -gx PATH \"" + bin + "\" $PATH\n" +
		"end\n"
	return env, fish
}

// WriteEnv writes EnvFiles for an installation at home into dir (home
// itself, or the staging directory that becomes it).
func WriteEnv(dir, home string, getenv func(string) string) error {
	env, fish := EnvFiles(home, getenv)
	if err := os.WriteFile(filepath.Join(dir, "env"), []byte(env), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "env.fish"), []byte(fish), 0o644)
}
