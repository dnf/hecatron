//go:build !windows

package platform

import "os"

// executableNamed is path when it is a regular file with an execute bit.
func executableNamed(path string) (string, bool) {
	info, err := os.Stat(path)
	return path, err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0
}

func sameName(a, b string) bool { return a == b }

// Bash is the bash that runs bash hooks and scripts: bash on PATH.
func Bash(path string) (string, bool) { return LookPath("bash", path) }
