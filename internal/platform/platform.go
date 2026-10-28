// Package platform holds what differs between Unix and Windows for finding
// programs and reading environments (docs/design.md §19): PATH's
// separator, what counts as an executable (a mode bit, or an extension in
// PATHEXT), environment names (case-insensitive on Windows, where os.Environ
// says "Path") and which bash runs bash hooks.
package platform

import (
	"os"
	"path/filepath"
	"strings"
)

// ListSeparator separates PATH's directories: ':', or ';' on Windows.
const ListSeparator = string(os.PathListSeparator)

// JoinPath joins directories into a PATH value.
func JoinPath(dirs ...string) string { return strings.Join(dirs, ListSeparator) }

// LookPath finds an executable on a PATH value, as `command -v` (or, on
// Windows, cmd.exe with PATHEXT) does; an empty entry is the working
// directory. A name with a directory in it is checked as it is.
func LookPath(name, path string) (string, bool) {
	if strings.ContainsRune(name, '/') || strings.ContainsRune(name, filepath.Separator) {
		return executableNamed(name)
	}
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			dir = "."
		}
		if found, ok := executableNamed(filepath.Join(dir, name)); ok {
			return found, true
		}
	}
	return "", false
}

// Getenv is the last value of name in env ("" when unset).
func Getenv(env []string, name string) string {
	value, _ := LookupEnv(env, name)
	return value
}

// LookupEnv is the last value of name in env, and whether it is set.
func LookupEnv(env []string, name string) (string, bool) {
	for i := len(env) - 1; i >= 0; i-- {
		key, value, ok := strings.Cut(env[i], "=")
		if ok && sameName(key, name) {
			return value, true
		}
	}
	return "", false
}

// Setenv is env with name set to value, every other entry for name dropped.
func Setenv(env []string, name, value string) []string {
	result := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if key, _, ok := strings.Cut(entry, "="); !ok || !sameName(key, name) {
			result = append(result, entry)
		}
	}
	return append(result, name+"="+value)
}
