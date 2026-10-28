package project

import (
	"os/exec"
	"strings"
)

// yieldsRule is a definition's "detectYields": where its build file sits
// beside more files matching Where than matching Outnumber (less those
// matching Except), the directory is the language To's (a tsconfig.json
// beside mostly JavaScript, antlr4's runtime/JavaScript with its type
// declarations, is a JavaScript package's), and To's "detectUnless"
// doesn't keep To out of it. As project.py's yields.
type yieldsRule struct {
	To                       string
	Where, Outnumber, Except []string
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
	for key, target := range map[string]*[]string{"where": &rule.Where, "outnumber": &rule.Outnumber, "except": &rule.Except} {
		if raw, present := object.Get(key); present {
			var err error
			if *target, err = toStrings(raw, lang+" detectYields "+key); err != nil {
				return nil, err
			}
		}
	}
	return rule, nil
}

// holds says whether the rule holds for the names of the files in a
// directory and below it, dotfiles left out.
func (r *yieldsRule) holds(names []string) (bool, error) {
	matching := func(patterns, but []string) (int, error) {
		n := 0
		for _, name := range names {
			if strings.HasPrefix(name, ".") {
				continue
			}
			in, err := matchesAny(name, patterns)
			if err != nil {
				return 0, err
			}
			out, err := matchesAny(name, but)
			if err != nil {
				return 0, err
			}
			if in && !out {
				n++
			}
		}
		return n, nil
	}
	more, err := matching(r.Where, nil)
	if err != nil {
		return false, err
	}
	fewer, err := matching(r.Outnumber, r.Except)
	if err != nil {
		return false, err
	}
	return more > fewer, nil
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

// trackedNamesBelow is trackedNamesBelow, read once per directory, as
// project.py caches it.
func (p *Project) trackedNamesBelow(directory string) ([]string, error) {
	p.cacheMu.Lock()
	names, ok := p.tracked[directory]
	p.cacheMu.Unlock()
	if ok {
		return names, nil
	}
	names, err := trackedNamesBelow(directory)
	if err != nil {
		return nil, err
	}
	p.cacheMu.Lock()
	if p.tracked == nil {
		p.tracked = map[string][]string{}
	}
	p.tracked[directory] = names
	p.cacheMu.Unlock()
	return names, nil
}

// trackedNamesBelow are the names of the files git tracks in a directory
// and below it, outside skippedDirs and hidden directories, as detection
// counts them (project.py's tracked_names_below); none outside a
// repository.
func trackedNamesBelow(directory string) ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = directory
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}
	var names []string
	for _, path := range pySplitLines(strings.TrimSpace(pyText(out))) {
		parts := strings.Split(path, "/")
		inSkipped := false
		for _, part := range parts[:len(parts)-1] {
			if skippedDirs[strings.ToLower(part)] || strings.HasPrefix(part, ".") {
				inSkipped = true
				break
			}
		}
		if !inSkipped {
			names = append(names, parts[len(parts)-1])
		}
	}
	return names, nil
}
