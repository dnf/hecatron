package advisor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/dnf/hecatron/internal/project"
)

// lockFormats compare a manifest with its lock file: stale is true when
// the lock no longer records what the manifest asks for; ok is false when
// either can't be read as the format expects (nothing is said then).
var lockFormats = map[string]func(manifest, lock []byte, member string) (stale, ok bool){
	"npm": npmLockStale,
}

// lockFresh runs a set's lock-matches-manifest rules in one component: the
// lock is the nearest one from the component's directory up to the root,
// as the package manager finds a workspace's.
func (r *run) lockFresh(set *Set, lang string, d project.ComponentDir) {
	for _, l := range set.LockFresh {
		manifest, err := os.ReadFile(filepath.Join(d.Dir, l.Manifest))
		if err != nil {
			continue
		}
		dir := d.Dir
		for {
			if lock, err := os.ReadFile(filepath.Join(dir, l.Lock)); err == nil {
				member, _ := filepath.Rel(dir, d.Dir)
				if member == "." {
					member = ""
				}
				if stale, ok := lockFormats[l.Format](manifest, lock, filepath.ToSlash(member)); ok && stale {
					f := Finding{ID: set.name + "/" + idLockFresh, Level: Recommended, Category: "reproducibility",
						Message: fmt.Sprintf("Update %s: it doesn't record the dependencies %s declares", joinRel(filepath.ToSlash(project.PyRelpath(dir, r.p.Root)), l.Lock), l.Manifest),
						Why:     l.Why, Link: l.Link}
					r.add(lang, d.Component, r.at(f, d.Dir))
				}
				break
			}
			if dir == r.p.Root || filepath.Dir(dir) == dir {
				break
			}
			dir = filepath.Dir(dir)
		}
	}
}

// npmDependencyFields are the package.json fields package-lock.json
// (lockfileVersion 2 and 3) copies into each workspace package's entry
// under "packages".
var npmDependencyFields = []string{"dependencies", "devDependencies", "optionalDependencies", "peerDependencies"}

// npmLockStale compares package.json's dependencies with its entry in
// package-lock.json ("" for the root package, the workspace's path for a
// member), as npm ci does before refusing to install. Lock files before
// version 2 have no such entry and aren't judged.
func npmLockStale(manifestData, lockData []byte, member string) (bool, bool) {
	var manifest map[string]any
	var lock struct {
		LockfileVersion int                       `json:"lockfileVersion"`
		Packages        map[string]map[string]any `json:"packages"`
	}
	if json.Unmarshal(manifestData, &manifest) != nil || json.Unmarshal(lockData, &lock) != nil || lock.LockfileVersion < 2 {
		return false, false
	}
	entry, ok := lock.Packages[member]
	if !ok {
		return false, false // not a workspace of this lock: another package manager's or a nested project
	}
	for _, field := range npmDependencyFields {
		want := nonEmpty(manifest[field])
		got := nonEmpty(entry[field])
		if !reflect.DeepEqual(want, got) {
			return true, true
		}
	}
	return false, true
}

func nonEmpty(value any) any {
	if m, ok := value.(map[string]any); ok && len(m) == 0 {
		return nil
	}
	return value
}
