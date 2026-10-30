package project

import (
	"os"
	"path/filepath"
	"strings"
)

// OutsideNestedRepos are the files (relative to root, with slashes) that
// aren't in another git repository below root: a directory holding a .git
// directory or file (a clone, a submodule, a worktree such as an agent's
// under .claude/worktrees/). Their changes are that repository's, not the
// project's. git status lists such a repository as one entry, its
// directory with a trailing slash; a file under one can still come from a
// listing that doesn't know it's a repository.
func OutsideNestedRepos(root string, files []string) []string {
	isRepo := map[string]bool{}
	nested := func(dir string) bool {
		known, ok := isRepo[dir]
		if !ok {
			_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(dir), ".git"))
			known = err == nil
			isRepo[dir] = known
		}
		return known
	}
	var kept []string
	for _, file := range files {
		if !inNestedRepo(file, nested) {
			kept = append(kept, file)
		}
	}
	return kept
}

// inNestedRepo is whether one of the file's directories below the root
// (or the file itself, a directory entry ending in a slash) is a
// repository.
func inNestedRepo(file string, nested func(dir string) bool) bool {
	parts := strings.Split(strings.TrimSuffix(file, "/"), "/")
	last := len(parts) - 1
	if strings.HasSuffix(file, "/") {
		last = len(parts)
	}
	for i := 1; i <= last; i++ {
		if nested(strings.Join(parts[:i], "/")) {
			return true
		}
	}
	return false
}
