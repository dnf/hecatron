package project

import (
	"encoding/json"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

// yieldsRule is a definition's "detectYields": where its build file sits
// beside more files matching Where than matching Outnumber (less those
// matching Except), the directory is the language To's (a tsconfig.json
// beside mostly JavaScript, antlr4's runtime/JavaScript with its type
// declarations, is a JavaScript package's), and To's "detectUnless"
// doesn't keep To out of it. Build output doesn't count: the directories
// directly below named in SkipDirs, the one the build file names at
// OutputDir (tsconfig.json's compilerOptions.outDir) and, with Twins, a
// Where file whose stem an Outnumber file shares (a compiled index.js). As
// project.py's yields.
type yieldsRule struct {
	To                       string
	Where, Outnumber, Except []string
	SkipDirs                 []string
	OutputDir                []string // nil unless a list of strings
	Twins                    bool
}

// parseYields reads a "detectYields" value; nil when absent or null.
func parseYields(value any, present bool, lang string) (*yieldsRule, error) {
	if !present || !truthy(value) {
		return nil, nil
	}
	object, ok := value.(*Object)
	if !ok {
		return nil, unsupported("%s detectYields is not an object", lang)
	}
	rule := &yieldsRule{}
	to, ok := object.Get("to")
	if rule.To, _ = to.(string); !ok || rule.To == "" {
		return nil, unsupported("%s detectYields has no \"to\"", lang)
	}
	for key, target := range map[string]*[]string{"where": &rule.Where, "outnumber": &rule.Outnumber,
		"except": &rule.Except, "skipDirs": &rule.SkipDirs} {
		if raw, present := object.Get(key); present {
			var err error
			if *target, err = toStrings(raw, lang+" detectYields "+key); err != nil {
				return nil, err
			}
		}
	}
	if raw, present := object.Get("outputDir"); present {
		if list, ok := raw.([]any); ok && len(list) > 0 {
			for _, item := range list {
				key, ok := item.(string)
				if !ok {
					rule.OutputDir = nil
					break
				}
				rule.OutputDir = append(rule.OutputDir, key)
			}
		}
	}
	twins, _ := object.Get("twins")
	rule.Twins = truthy(twins)
	return rule, nil
}

// holds says whether the rule holds for the paths of the files in a
// directory and below it (relative to it), whose build file is buildFile
// ("" for none).
func (r *yieldsRule) holds(paths []string, buildFile string) (bool, error) {
	output := ""
	if buildFile != "" {
		var err error
		if output, err = outputDir(buildFile, r.OutputDir); err != nil {
			return false, err
		}
	}
	var names []string
	for _, p := range paths {
		parts := strings.Split(p, "/")
		if len(parts) > 1 && contains(r.SkipDirs, parts[0]) ||
			output != "" && (p == output || strings.HasPrefix(p, output+"/")) {
			continue
		}
		if name := parts[len(parts)-1]; !strings.HasPrefix(name, ".") {
			names = append(names, name)
		}
	}
	stems := map[string]bool{}
	own := 0
	for _, name := range names {
		in, err := matchesAny(name, r.Outnumber)
		if err != nil {
			return false, err
		}
		out, err := matchesAny(name, r.Except)
		if err != nil {
			return false, err
		}
		if in && !out {
			own++
			if r.Twins {
				stems[stem(name)] = true
			}
		}
	}
	others := 0
	for _, name := range names {
		in, err := matchesAny(name, r.Where)
		if err != nil {
			return false, err
		}
		if in && !stems[stem(name)] {
			others++
		}
	}
	return others > own, nil
}

// stem is a file name without its extension, as Python's os.path.splitext
// takes it off a name that doesn't start with a dot.
func stem(name string) string { return strings.TrimSuffix(name, path.Ext(name)) }

// outputDir is the directory a build file names at a key path, relative to
// the build file's directory and normalized; "" when it names none inside
// it, or the file can't be read (as project.py's _output_dir). JSON that
// only Python reads (NaN, Infinity) is declined.
func outputDir(buildFile string, keys []string) (string, error) {
	if len(keys) == 0 {
		return "", nil
	}
	data, err := os.ReadFile(buildFile)
	if err != nil || !utf8.Valid(data) {
		return "", nil
	}
	strict := looseJSON(string(data))
	if !json.Valid([]byte(strict)) {
		if strings.Contains(strict, "NaN") || strings.Contains(strict, "Infinity") {
			return "", unsupported("%s: JSON only Python reads", buildFile)
		}
		return "", nil
	}
	value, err := decodeJSON([]byte(strict))
	if err != nil {
		return "", err
	}
	for _, key := range keys {
		object, ok := value.(*Object)
		if !ok {
			return "", nil
		}
		value, _ = object.Get(key)
	}
	dir, ok := value.(string)
	if !ok {
		return "", nil
	}
	dir = path.Clean(dir)
	if dir == "." || dir == ".." || strings.HasPrefix(dir, "/") || strings.HasPrefix(dir, "../") {
		return "", nil
	}
	return dir, nil
}

// looseJSON is JSON with comments (// and /* */) and trailing commas, as
// tsconfig.json allows, made strict JSON; strings are kept as written
// (project.py's loose_json).
func looseJSON(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); {
		switch {
		case text[i] == '"':
			j := stringEnd(text, i)
			out.WriteString(text[i:j])
			i = j
		case strings.HasPrefix(text[i:], "//"):
			if j := strings.IndexByte(text[i:], '\n'); j >= 0 {
				i += j
			} else {
				i = len(text)
			}
		case strings.HasPrefix(text[i:], "/*"):
			if j := strings.Index(text[i+2:], "*/"); j >= 0 {
				i += 2 + j + 2
			} else {
				i = len(text)
			}
		default:
			out.WriteByte(text[i])
			i++
		}
	}
	text = out.String()
	var strict strings.Builder
	for i := 0; i < len(text); {
		c := text[i]
		if c == '"' {
			j := stringEnd(text, i)
			strict.WriteString(text[i:j])
			i = j
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(text) && strings.IndexByte(" \t\r\n", text[j]) >= 0 {
				j++
			}
			if j < len(text) && (text[j] == '}' || text[j] == ']') {
				i++
				continue
			}
		}
		strict.WriteByte(c)
		i++
	}
	return strict.String()
}

// stringEnd is the index just past the JSON string starting at text[i]
// (its closing quote), or len(text) when it isn't closed.
func stringEnd(text string, i int) int {
	j := i + 1
	for j < len(text) && text[j] != '"' {
		if text[j] == '\\' {
			j += 2
		} else {
			j++
		}
	}
	return min(j+1, len(text))
}

// buildFileIn is the first file, by pattern and then by name, of the names
// in a directory matching the patterns: the build file a hit was found by
// ("" for none; project.py's build_file_in).
func buildFileIn(directory string, names []string, patterns []string) (string, error) {
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	for _, pattern := range patterns {
		for _, name := range sorted {
			matched, err := fnmatch(name, pattern)
			if err != nil {
				return "", err
			}
			if matched {
				return pyJoin(directory, name), nil
			}
		}
	}
	return "", nil
}

// relativeTo is a tracked directory's path relative to another ("" for
// itself), when it is that directory or inside it ("." holds every one).
func relativeTo(sub, directory string) (string, bool) {
	switch {
	case directory == ".":
		if sub == "." {
			return "", true
		}
		return sub, true
	case sub == directory:
		return "", true
	case under(sub, directory):
		return sub[len(directory)+1:], true
	}
	return "", false
}

func matchesAny(name string, patterns []string) (bool, error) {
	for _, pattern := range patterns {
		matched, err := fnmatch(name, pattern)
		if err != nil || matched {
			return matched, err
		}
	}
	return false, nil
}

// trackedPathsBelow is trackedPathsBelow, read once per directory, as
// project.py caches it.
func (p *Project) trackedPathsBelow(directory string) ([]string, error) {
	p.cacheMu.Lock()
	paths, ok := p.tracked[directory]
	p.cacheMu.Unlock()
	if ok {
		return paths, nil
	}
	paths, err := trackedPathsBelow(directory)
	if err != nil {
		return nil, err
	}
	p.cacheMu.Lock()
	if p.tracked == nil {
		p.tracked = map[string][]string{}
	}
	p.tracked[directory] = paths
	p.cacheMu.Unlock()
	return paths, nil
}

// trackedPathsBelow are the paths, relative to it, of the files git tracks
// in a directory and below it, outside skippedDirs and hidden directories,
// as detection counts them (project.py's tracked_paths_below); none
// outside a repository.
func trackedPathsBelow(directory string) ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = directory
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}
	var paths []string
	for _, p := range pySplitLines(strings.TrimSpace(pyText(out))) {
		parts := strings.Split(p, "/")
		inSkipped := false
		for _, part := range parts[:len(parts)-1] {
			if skippedDirs[strings.ToLower(part)] || strings.HasPrefix(part, ".") {
				inSkipped = true
				break
			}
		}
		if !inSkipped {
			paths = append(paths, p)
		}
	}
	return paths, nil
}
