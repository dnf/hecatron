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
	kept, _ := SplitNestedRepos(root, files)
	return kept
}

// SplitNestedRepos are the files outside another git repository below
// root (as OutsideNestedRepos), and those inside one.
func SplitNestedRepos(root string, files []string) (kept, nested []string) {
	isRepo := map[string]bool{}
	isNested := func(dir string) bool {
		known, ok := isRepo[dir]
		if !ok {
			_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(dir), ".git"))
			known = err == nil
			isRepo[dir] = known
		}
		return known
	}
	for _, file := range files {
		if inNestedRepo(file, isNested) {
			nested = append(nested, file)
		} else {
			kept = append(kept, file)
		}
	}
	return kept, nested
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
