//go:build windows

package platform

import (
	"os"
	"path/filepath"
	"strings"
)

// executableNamed is path, or path with an extension from PATHEXT, when
// that is a file: Windows marks executables by their extension, not a
// mode bit.
func executableNamed(path string) (string, bool) {
	extensions := pathExt()
	if ext := strings.ToLower(filepath.Ext(path)); ext != "" {
		for _, e := range extensions {
			if ext == e && isFile(path) {
				return path, true
			}
		}
	}
	for _, ext := range extensions {
		if isFile(path + ext) {
			return path + ext, true
		}
	}
	return "", false
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// pathExt is PATHEXT's extensions, lowercase, or cmd.exe's default.
func pathExt() []string {
	value := os.Getenv("PATHEXT")
	if value == "" {
		value = ".COM;.EXE;.BAT;.CMD"
	}
	var extensions []string
	for _, ext := range strings.Split(strings.ToLower(value), ";") {
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		extensions = append(extensions, ext)
	}
	return extensions
}

func sameName(a, b string) bool { return strings.EqualFold(a, b) }

// Bash is the bash that runs bash hooks and scripts on Windows, until the
// definitions are declarative (#77): Git for Windows' bash, found on PATH
// or beside git. Never System32's bash.exe, WSL's launcher, which would run
// the hook inside a Linux distribution with Windows paths it can't open.
func Bash(path string) (string, bool) {
	for _, dir := range filepath.SplitList(path) {
		if dir == "" || isWSLLauncherDir(dir) {
			continue
		}
		if found, ok := executableNamed(filepath.Join(dir, "bash")); ok {
			return found, true
		}
	}
	git, ok := LookPath("git", path)
	if !ok {
		return "", false
	}
	// <Git>\cmd\git.exe or <Git>\bin\git.exe or <Git>\mingw64\bin\git.exe:
	// bash is <Git>\bin\bash.exe.
	dir := filepath.Dir(git)
	for range 3 {
		if candidate := filepath.Join(dir, "bin", "bash.exe"); isFile(candidate) {
			return candidate, true
		}
		dir = filepath.Dir(dir)
	}
	return "", false
}

// isWSLLauncherDir is %SystemRoot%\System32 (and the WindowsApps aliases),
// where bash.exe starts WSL.
func isWSLLauncherDir(dir string) bool {
	clean := strings.ToLower(filepath.Clean(dir))
	if root := os.Getenv("SystemRoot"); root != "" && clean == strings.ToLower(filepath.Join(root, "System32")) {
		return true
	}
	return strings.HasSuffix(clean, `\microsoft\windowsapps`)
}
