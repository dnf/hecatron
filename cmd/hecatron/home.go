package main

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

// findHome is HECATRON_HOME, else the hecatron checkout or install this
// binary belongs to: the directory above the one it's in (bin/hct), or the
// one it's in (built at the top of a checkout with
// `go build -o hct ./cmd/hecatron`), else on Windows install.ps1's
// %LOCALAPPDATA%\hecatron\home. Links are followed, as bin/hecatron
// follows Homebrew's.
func findHome() string {
	if home := os.Getenv("HECATRON_HOME"); home != "" {
		return home
	}
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return "."
	}
	dir := filepath.Dir(exe)
	candidates := []string{filepath.Dir(dir), dir}
	if local := os.Getenv("LOCALAPPDATA"); runtime.GOOS == "windows" && local != "" {
		// Where install.ps1 puts it, for a copy of the binary elsewhere.
		candidates = append(candidates, filepath.Join(local, "hecatron", "home"))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(filepath.Join(candidate, "libexec", "lib", "project.py")); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return filepath.Dir(dir)
}

// workingDir is the working directory with links resolved, as Python's
// os.getcwd() gives it to project.py. os.Getwd would return $PWD, which
// keeps the links a shell followed to get there.
func workingDir() (string, error) {
	if dir, err := syscall.Getwd(); err == nil {
		return dir, nil
	}
	return os.Getwd()
}
