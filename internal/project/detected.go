package project

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// A git repository with no .hecatron/config.json is read as detected
// (project.py's detected_config): the config hecatron init would write,
// which project.py caches in .git/hecatron/detected.json, keyed by the
// commit, git's index and hecatron's version. Detecting needs init's code
// (manage.py), so the Go model reads only a current cache: project.py
// writes it whenever it finds the project (`project.py root`), and the
// model declines a repository whose cache is missing or stale.

// LoadDetected reads a repository without a config of its own from
// project.py's cache of what it detected.
func LoadDetected(root string, home Home) (*Project, error) {
	gitDir := gitOutput(root, "rev-parse", "--absolute-git-dir")
	hecatronDir := pyJoin(root, ".git", "hecatron")
	if gitDir != "" {
		hecatronDir = pyJoin(gitDir, "hecatron")
	}
	cache, err := readJSON(pyJoin(hecatronDir, "detected.json"))
	if err != nil {
		return nil, err
	}
	object, ok := cache.(*Object)
	if !ok {
		return nil, unsupported("no detected.json")
	}
	if key, _ := object.Get("key"); !sameKey(key, detectionKey(root, gitDir, home)) {
		return nil, unsupported("detected.json is stale")
	}
	config, ok := object.Values["config"].(*Object)
	if !ok || !truthy(config) {
		return nil, unsupported("detected.json has no config")
	}
	p := &Project{Root: root, Dir: pyJoin(root, ".hecatron"), Home: home, Detected: true, config: config}
	if err := p.checkSchema(pyJoin(p.Dir, "config.json")); err != nil {
		return nil, err
	}
	if err := p.checkHecatronVersion(); err != nil {
		return nil, err
	}
	if err := p.readConfig(); err != nil {
		return nil, err
	}
	if _, set := config.Get("logs"); !set {
		p.Logs = pyJoin(hecatronDir, "logs")
	}
	if err := p.readLocalFrom(pyJoin(hecatronDir, "local.json")); err != nil {
		return nil, err
	}
	return p, nil
}

// detectionKey is project.py's cache key: HEAD's commit, the index's
// modification time (ns) and size, and hecatron's version.
func detectionKey(root, gitDir string, home Home) []string {
	head := gitOutput(root, "rev-parse", "-q", "--verify", "HEAD")
	mtime, size := "0", "0"
	if info, err := os.Stat(filepath.Join(gitDir, "index")); gitDir != "" && err == nil && info.Mode().IsRegular() {
		mtime, size = strconv.FormatInt(info.ModTime().UnixNano(), 10), strconv.FormatInt(info.Size(), 10)
	}
	return []string{head, mtime, size, home.Version()}
}

func sameKey(cached any, want []string) bool {
	list, ok := cached.([]any)
	if !ok || len(list) != len(want) {
		return false
	}
	for i, item := range list {
		switch v := item.(type) {
		case string:
			if i != 0 && i != 3 || v != want[i] {
				return false
			}
		case json.Number:
			if i != 1 && i != 2 || string(v) != want[i] {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func gitOutput(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
