package xcode

import (
	"os"
	"path/filepath"
	"strings"
)

// readXcconfig is an .xcconfig file's settings: KEY = value lines, later
// ones winning, with the files it #includes (#include? for one that may be
// missing) read where they appear. // starts a comment, as Xcode reads it.
// Conditional settings (KEY[sdk=...]) are skipped. A file that can't be
// read has no settings.
func readXcconfig(path string) map[string]string {
	settings := map[string]string{}
	readXcconfigInto(path, settings, map[string]bool{})
	return settings
}

func readXcconfigInto(path string, settings map[string]string, seen map[string]bool) {
	if seen[path] {
		return
	}
	seen[path] = true
	text, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(text), "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "#include"); ok {
			rest = strings.TrimPrefix(rest, "?")
			name := strings.Trim(strings.TrimSpace(rest), `"`)
			if name != "" {
				if !filepath.IsAbs(name) {
					name = filepath.Join(filepath.Dir(path), name)
				}
				readXcconfigInto(name, settings, seen)
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" || strings.Contains(key, "[") {
			continue
		}
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), ";"))
		// $(inherited) is an earlier line's value, else the next level's
		// (left for Project.Setting to fill in).
		if earlier, ok := settings[key]; ok {
			value = strings.ReplaceAll(value, "$(inherited)", earlier)
		}
		settings[key] = value
	}
}
