// Package changed is `hct test --changed [ref]` (#91, #99): which
// languages, components and project tasks the files changed since a base
// commit touch, so a run covers only those.
//
// The base is the merge base of HEAD with the ref given, else with the
// pull or merge request's base in CI, else with the default branch; the
// changes are git's diff from it to the working tree (committed, staged
// and not) and the untracked files git doesn't ignore.
package changed

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"sort"
	"strings"

	"github.com/dnf/hecatron/internal/project"
)

// Base is what the changes are measured from.
type Base struct {
	Ref    string // as named: "origin/main", a commit
	Source string // what named it: "--changed", "GITHUB_BASE_REF", "the default branch"
	Commit string // the merge base of HEAD and Ref
}

// UsageError is a --changed the user must fix (a ref that names no
// commit, not a git repository).
type UsageError struct{ Message string }

func (e *UsageError) Error() string { return e.Message }

// prBases are the CI variables that name a pull or merge request's base.
var prBases = []struct{ variable, prefix string }{
	{"GITHUB_BASE_REF", "origin/"},                 // GitHub Actions: a branch
	{"CI_MERGE_REQUEST_DIFF_BASE_SHA", ""},         // GitLab CI: the merge base itself
	{"BITBUCKET_PR_DESTINATION_BRANCH", "origin/"}, // Bitbucket Pipelines: a branch
	{"SYSTEM_PULLREQUEST_TARGETBRANCH", "origin/"}, // Azure Pipelines: refs/heads/<branch>
}

// FindBase finds the base. A note, with no error, means there is none to
// measure from, so everything runs: a shallow clone without the base, a
// CI run that isn't a pull request, a repository without a default branch.
func FindBase(root, ref string, getenv func(string) string) (base Base, note string, err error) {
	if _, err := git(root, "rev-parse", "--verify", "-q", "HEAD^{commit}"); err != nil {
		if _, inRepo := git(root, "rev-parse", "--git-dir"); inRepo != nil {
			return Base{}, "", &UsageError{"--changed needs a git repository"}
		}
		return Base{}, "--changed: no commit yet; running everything", nil
	}
	switch {
	case ref != "":
		if !isCommit(root, ref) {
			return Base{}, "", &UsageError{fmt.Sprintf("--changed: '%s' is not a commit", ref)}
		}
		base = Base{Ref: ref, Source: "--changed"}
	case inCI(getenv):
		for _, pr := range prBases {
			value := strings.TrimPrefix(getenv(pr.variable), "refs/heads/")
			if value == "" {
				continue
			}
			base = Base{Ref: pr.prefix + value, Source: pr.variable}
			if !isCommit(root, base.Ref) && pr.prefix != "" && isCommit(root, value) {
				base.Ref = value
			}
			if !isCommit(root, base.Ref) {
				return base, fmt.Sprintf("--changed: %s's %s isn't in this clone (shallow? fetch it); running everything", pr.variable, value), nil
			}
			break
		}
		if base.Ref == "" {
			return Base{}, "--changed: a CI run outside a pull request; running everything", nil
		}
	default:
		branch := defaultBranch(root)
		if branch == "" {
			return Base{}, "--changed: no default branch (origin/HEAD, main or master); running everything", nil
		}
		base = Base{Ref: branch, Source: "the default branch"}
	}
	out, err := git(root, "merge-base", "HEAD", base.Ref)
	if err != nil {
		return base, fmt.Sprintf("--changed: no merge base with %s in this clone (shallow? fetch more history); running everything", base.Ref), nil
	}
	base.Commit = strings.TrimSpace(out)
	return base, "", nil
}

// inCI reports whether this is a CI run (CI is set by GitHub Actions,
// GitLab, Bitbucket and most others; TF_BUILD by Azure Pipelines).
func inCI(getenv func(string) string) bool {
	return getenv("CI") != "" && getenv("CI") != "false" || getenv("TF_BUILD") != ""
}

// defaultBranch is origin/HEAD's branch, else the first of origin/main,
// origin/master, main and master that exists.
func defaultBranch(root string) string {
	if out, err := git(root, "symbolic-ref", "-q", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if branch := strings.TrimSpace(out); branch != "" {
			return branch
		}
	}
	for _, branch := range []string{"origin/main", "origin/master", "main", "master"} {
		if isCommit(root, branch) {
			return branch
		}
	}
	return ""
}

func isCommit(root, ref string) bool {
	if strings.HasPrefix(ref, "-") {
		return false
	}
	_, err := git(root, "rev-parse", "--verify", "-q", ref+"^{commit}")
	return err == nil
}

// Files are the files changed since commit under root, relative to it:
// git's diff to the working tree (renames as their old and new paths), and
// the untracked files it doesn't ignore, less those in another repository
// or worktree below root (project.OutsideNestedRepos).
func Files(root, commit string) ([]string, error) {
	diff, err := git(root, "diff", "--name-only", "-z", "--no-renames", "--relative", commit, "--")
	if err != nil {
		return nil, err
	}
	untracked, err := git(root, "ls-files", "--others", "--exclude-standard", "-z", "--", ".")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var files []string
	for _, name := range strings.Split(diff+untracked, "\x00") {
		if name != "" && !seen[name] {
			seen[name] = true
			files = append(files, name)
		}
	}
	sort.Strings(files)
	return project.OutsideNestedRepos(root, files), nil
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-c", "core.quotepath=off"}, args...)...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	return string(out), nil
}

// matches reports whether a root-relative file matches a pattern: one
// with a slash against the whole path (a trailing slash, or a directory,
// matching everything under it), one without against the file's name.
func matches(pattern, file string) bool {
	pattern = strings.TrimPrefix(pattern, "./")
	if dir, ok := strings.CutSuffix(pattern, "/"); ok {
		return file == dir || strings.HasPrefix(file, dir+"/")
	}
	if !strings.Contains(pattern, "/") {
		ok, _ := path.Match(pattern, path.Base(file))
		return ok
	}
	if ok, _ := path.Match(pattern, file); ok {
		return true
	}
	return strings.HasPrefix(file, pattern+"/")
}
