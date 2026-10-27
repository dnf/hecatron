package project

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// A git repository with no .hecatron/config.json is read as detected
// (project.py's detected_config): the config hecatron init would write,
// cached in .git/hecatron/detected.json and keyed by the commit, git's index
// and hecatron's version, so detection runs again only when they change.

// LoadDetected reads a repository without a config of its own: from the
// cache of what was detected while it's current, else by detecting it and
// caching that, as project.py does. Announcement is then what project.py
// says on stderr the first time ("... has no .hecatron/config.json;
// detected go, rust (hecatron init saves this)"), and "" when the cache
// answered or had been written before.
func LoadDetected(root string, home Home) (*Project, error) {
	gitDir := gitOutput(root, "rev-parse", "--absolute-git-dir")
	hecatronDir := pyJoin(root, ".git", "hecatron")
	if gitDir != "" {
		hecatronDir = pyJoin(gitDir, "hecatron")
	}
	cachePath := pyJoin(hecatronDir, "detected.json")
	cache, err := readJSON(cachePath)
	if err != nil {
		return nil, err
	}
	key := detectionKey(root, gitDir, home)
	var config *Object
	announcement := ""
	if object, ok := cache.(*Object); ok && truthy(object) && sameKey(object.Values["key"], key) {
		if config, ok = object.Values["config"].(*Object); !ok || !truthy(config) {
			return nil, unsupported("detected.json has no config")
		}
	} else {
		if _, isObject := cache.(*Object); cache != nil && !isObject && truthy(cache) {
			return nil, unsupported("detected.json is not an object") // cached.get raises
		}
		if config, err = detect(root, home, cachePath, key); err != nil {
			return nil, err
		}
		if cache == nil {
			languages := strings.Join(configStrings(config, "languages"), ", ")
			if languages == "" {
				languages = "no languages"
			}
			announcement = fmt.Sprintf("hecatron: %s has no .hecatron/config.json; detected %s (hecatron init saves this)", root, languages)
		}
	}
	p := &Project{Root: root, Dir: pyJoin(root, ".hecatron"), Home: home, Detected: true, config: config,
		Announcement: announcement}
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

// detect is the config hecatron init would write, without "logs", written
// to the cache with its key.
func detect(root string, home Home, cachePath string, key []string) (*Object, error) {
	config, _, err := DetectedConfig(root, home, "", "", "")
	if err != nil {
		return nil, err
	}
	delete(config.Values, "logs")
	config.Keys = removeKey(config.Keys, "logs")
	entry := &Object{Values: map[string]any{}}
	entry.set("key", []any{key[0], json.Number(key[1]), json.Number(key[2]), key[3]})
	entry.set("config", config)
	if err := writePyJSONFile(cachePath, entry); err != nil {
		return nil, unsupported("writing %s: %v", cachePath, err)
	}
	return config, nil
}

func removeKey(keys []string, key string) []string {
	var result []string
	for _, k := range keys {
		if k != key {
			result = append(result, k)
		}
	}
	return result
}

func configStrings(config *Object, key string) []string {
	list, _ := config.Values[key].([]any)
	var result []string
	for _, item := range list {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
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
