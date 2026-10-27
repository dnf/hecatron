package advisor

import (
	"fmt"
	"strings"

	"github.com/dnf/hecatron/internal/project"
)

// ignore is one config.json "advisor": {"ignore": [...]} entry: a rule id,
// or a rule's name after the slash for every set ("toolchain-pinned"),
// optionally in one directory only ("path", relative to the root).
type ignore struct {
	id, path, reason string
}

type ignoreList []ignore

// Ignores are the project's silenced rules; entries hct check would
// reject are skipped.
func Ignores(p *project.Project) ignoreList {
	var list ignoreList
	for _, raw := range ignoreEntries(p) {
		entry, ok := raw.(*project.Object)
		if !ok {
			continue
		}
		id, _ := entry.Get("id")
		reason, _ := entry.Get("reason")
		path, _ := entry.Get("path")
		idText, _ := id.(string)
		reasonText, _ := reason.(string)
		pathText, _ := path.(string)
		if idText != "" && strings.TrimSpace(reasonText) != "" {
			list = append(list, ignore{idText, strings.Trim(pathText, "/"), reasonText})
		}
	}
	return list
}

func ignoreEntries(p *project.Project) []any {
	advisor, _ := p.ConfigValue("advisor")
	object, ok := advisor.(*project.Object)
	if !ok {
		return nil
	}
	value, _ := object.Get("ignore")
	entries, _ := value.([]any)
	return entries
}

// match is the reason a finding is ignored, if it is.
func (list ignoreList) match(f Finding) (string, bool) {
	for _, entry := range list {
		if !idMatches(entry.id, f.ID) {
			continue
		}
		if entry.path != "" && entry.path != f.Dir {
			continue
		}
		return entry.reason, true
	}
	return "", false
}

// idMatches: the whole id, or its rule name for every set.
func idMatches(pattern, id string) bool {
	if pattern == id {
		return true
	}
	_, name, found := strings.Cut(id, "/")
	return !strings.Contains(pattern, "/") && found && pattern == name
}

// ConfigProblems is what's wrong with config.json's "advisor" section, for
// hct check: each ignore entry needs an id naming a rule and a reason.
func ConfigProblems(p *project.Project) ([]string, error) {
	advisor, present := p.ConfigValue("advisor")
	if !present {
		return nil, nil
	}
	object, ok := advisor.(*project.Object)
	if !ok {
		return []string{`"advisor" must be an object`}, nil
	}
	var problems []string
	for _, key := range object.Keys {
		if key != "ignore" {
			problems = append(problems, fmt.Sprintf("advisor: unknown key %q (known: ignore)", key))
		}
	}
	value, present := object.Get("ignore")
	if !present {
		return problems, nil
	}
	entries, ok := value.([]any)
	if !ok {
		return append(problems, "advisor.ignore must be a list"), nil
	}
	defs, err := p.Definitions()
	if err != nil {
		return nil, err
	}
	lib, err := Load(p, defs.Names)
	if err != nil {
		return nil, err
	}
	ids := lib.IDs()
	for index, raw := range entries {
		entry, isObject := raw.(*project.Object)
		id, _ := entry.Get("id")
		idText, isString := id.(string)
		if !isObject || !isString || idText == "" {
			problems = append(problems, fmt.Sprintf(`advisor.ignore[%d] needs an "id"`, index))
			continue
		}
		where := "advisor.ignore " + idText
		if !knownID(ids, idText) {
			problems = append(problems, where+": no such rule (hct advisor --output json lists the ids it reports)")
		}
		if reason, _ := entry.Get("reason"); !isNonBlank(reason) {
			problems = append(problems, where+` needs a "reason"`)
		}
		if path, present := entry.Get("path"); present {
			if text, ok := path.(string); !ok || text == "" {
				problems = append(problems, where+`: "path" must be a directory relative to the root`)
			}
		}
		for _, key := range entry.Keys {
			if key != "id" && key != "reason" && key != "path" {
				problems = append(problems, fmt.Sprintf("%s: unknown key %q (known: id, reason, path)", where, key))
			}
		}
	}
	return problems, nil
}

func knownID(ids []string, pattern string) bool {
	for _, id := range ids {
		if idMatches(pattern, id) {
			return true
		}
	}
	return false
}

func isNonBlank(value any) bool {
	s, ok := value.(string)
	return ok && strings.TrimSpace(s) != ""
}
