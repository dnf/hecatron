package project

import (
	"bytes"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// rootRule is one entry of a definition's "detectRoot" "workspace" or
// "package" list: a file at the repository's root named like File, with a
// line matching Matches or its JSON Key (dotted for a path) non-empty when
// the rule has one.
type rootRule struct {
	File    string
	Matches *regexp.Regexp
	Key     string
}

// rootKindsOrder is the order rootKinds tries a definition's rules in.
var rootKindsOrder = []string{"workspace", "package"}

// parseRootRules reads a "detectRoot" value; nil when absent or null.
func parseRootRules(value any, present bool, lang string) (map[string][]rootRule, error) {
	if !present || !truthy(value) {
		return nil, nil
	}
	object, ok := value.(*Object)
	if !ok {
		return nil, unsupported("%s detectRoot is not an object", lang)
	}
	rules := map[string][]rootRule{}
	for _, kind := range rootKindsOrder {
		raw, present := object.Get(kind)
		if !present {
			continue
		}
		list, ok := raw.([]any)
		if !ok {
			return nil, unsupported("%s detectRoot %s is not a list", lang, kind)
		}
		for _, item := range list {
			entry, ok := item.(*Object)
			if !ok {
				return nil, unsupported("%s detectRoot %s has an entry that isn't an object", lang, kind)
			}
			var rule rootRule
			file, _ := entry.Get("file")
			if rule.File, ok = file.(string); !ok {
				return nil, unsupported("%s detectRoot %s has an entry without a file", lang, kind)
			}
			if raw, present := entry.Get("matches"); present {
				pattern, ok := raw.(string)
				if !ok {
					return nil, unsupported("%s detectRoot %s matches is not a string", lang, kind)
				}
				var err error
				if rule.Matches, err = regexp.Compile(pattern); err != nil {
					return nil, unsupported("%s detectRoot %s matches: %v", lang, kind, err)
				}
			} else if raw, present := entry.Get("key"); present {
				if rule.Key, ok = raw.(string); !ok {
					return nil, unsupported("%s detectRoot %s key is not a string", lang, kind)
				}
			}
			rules[kind] = append(rules[kind], rule)
		}
	}
	return rules, nil
}

// rootKinds is manage.py's root_kinds: language → what the repository's
// root is for it by its "detectRoot" rules, "workspace" (its build covers
// the language's packages below) or "package" (a package of its own,
// separate from those below). names are the files tracked at the root.
func rootKinds(root string, names map[string]bool, defs []detection) (map[string]string, error) {
	kinds := map[string]string{}
	for _, d := range defs {
		for _, kind := range rootKindsOrder {
			held := false
			for _, rule := range d.root[kind] {
				holds, err := rule.holds(root, names)
				if err != nil {
					return nil, err
				}
				if held = holds; held {
					break
				}
			}
			if held {
				kinds[d.lang] = kind
				break
			}
		}
	}
	return kinds, nil
}

// holds is manage.py's _root_rule_holds.
func (r rootRule) holds(root string, names map[string]bool) (bool, error) {
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	for _, name := range sorted {
		matched, err := fnmatch(name, r.File)
		if err != nil {
			return false, err
		}
		if !matched {
			continue
		}
		path := pyJoin(root, name)
		switch {
		case r.Matches != nil:
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			for _, line := range bytes.Split(data, []byte("\n")) {
				if r.Matches.Match(line) {
					return true, nil
				}
			}
		case r.Key != "":
			data, err := os.ReadFile(path)
			if err != nil || !utf8.Valid(data) {
				continue
			}
			document, ok, err := pyJSONLoads(pyText(data)) // json.load reads in text mode
			if err != nil {
				return false, err
			}
			if !ok {
				continue
			}
			var value any = document
			for _, part := range strings.Split(r.Key, ".") {
				if inner, isObject := value.(*Object); isObject {
					value, _ = inner.Get(part)
				} else {
					value = nil
				}
			}
			if truthy(value) {
				return true, nil
			}
		default:
			return true, nil
		}
	}
	return false, nil
}

// placeRootLanguages is manage.py's place_root_languages: the layout, with
// the languages whose build file at the root the directories named for
// them left out (#255, #238). A root that is a workspace for a language
// takes the language when some of its directories fall outside the ones
// the layout gave it (a Cargo workspace whose members include
// clients/rust/); a root that is a package of its own, beside packages of
// the same language in the layout's directories or in directories named
// for it, becomes a component with each of them (components ".",
// "layers/a/nodejs"..., the language at "{component}"), for a layout of one
// component or of that language alone, and for one such language.
func placeRootLanguages(found map[string]dirSet, layout Layout, names map[string][]string, kinds map[string]string) Layout {
	namesOf := func(lang string) []string {
		if n, ok := names[lang]; ok {
			return n
		}
		return []string{lang}
	}
	components, implPath := layout.Components, layout.ImplPath
	overrides := map[string]string{}
	for lang, path := range layout.ImplPaths {
		overrides[lang] = path
	}
	layoutDirs := func(lang string) dirSet {
		template := overrides[lang]
		if template == "" {
			template = implPath
		}
		dirs := dirSet{}
		for _, component := range components {
			for _, name := range namesOf(lang) {
				dirs[pyNormpath(strings.ReplaceAll(strings.ReplaceAll(template, "{component}", component), "{lang}", name))] = true
			}
		}
		return dirs
	}
	onlyLanguage := func(lang string) bool { return len(found) == 1 && found[lang] != nil }
	type rootPackage struct {
		lang     string
		separate dirSet
	}
	var packages []rootPackage
	langs := make([]string, 0, len(found))
	for lang := range found {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	for _, lang := range langs {
		kind, dirs := kinds[lang], layoutDirs(lang)
		if kind == "" || dirs["."] {
			continue
		}
		outside := false
		for d := range found[lang] {
			inside := false
			for placed := range dirs {
				inside = inside || under(d, placed)
			}
			outside = outside || !inside
		}
		switch {
		case kind == "workspace" && outside:
			if onlyLanguage(lang) {
				components, implPath, overrides = []string{"."}, ".", map[string]string{}
			} else {
				overrides[lang] = "."
			}
		case kind == "package" && found[lang]["."]:
			named := languageRoots(found[lang], namesOf(lang))
			separate := dirSet{}
			for d := range found[lang] {
				if dirs[d] || named[d] {
					separate[d] = true
				}
			}
			if len(separate) > 0 {
				packages = append(packages, rootPackage{lang, separate})
			}
		}
	}
	if len(packages) == 1 {
		lang, separate := packages[0].lang, packages[0].separate
		withRoot := append([]string{"."}, separate.sorted()...)
		if onlyLanguage(lang) {
			components, implPath, overrides = withRoot, "{component}", map[string]string{}
		} else if len(components) == 1 && components[0] == "." {
			components, overrides[lang] = withRoot, "{component}"
		}
	}
	return Layout{Components: components, ImplPath: implPath, ImplPaths: overrides}
}
