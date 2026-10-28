package project

import (
	"encoding/json"
	"os"
	"os/exec"
	"sort"
	"strings"
	"unicode/utf8"
)

// Detection (docs/design.md §10): which languages a repository uses and
// where, from its tracked files, as manage.py's detect_languages,
// infer_layout and detected_config decide it for hecatron init and for a
// repository read without a config of its own.

// skippedDirs hold code that isn't the project's own implementation:
// vendored dependencies, samples, tests, docs and tooling (hooks/: git
// hooks, build hooks). A language found only there isn't added, and no
// language's directory is placed there.
var skippedDirs = map[string]bool{"android": true, "benchmarks": true, "buildSrc": true, "demo": true, "demos": true,
	"doc": true, "docs": true, "example": true, "examples": true, "external": true, "fixtures": true,
	"hooks": true, "node_modules": true, "playground": true, "sample": true, "samples": true, "templates": true, "test": true,
	"testdata": true, "tests": true, "third_party": true, "tools": true, "tutorial": true, "tutorials": true, "vendor": true}

// preferredParents are where projects keep per-language code; when layouts
// disagree, one under these (or at the root) beats one covering more
// languages (Beam's sdks/{lang} over learning/katas/{lang}).
var preferredParents = map[string]bool{"bindings": true, "ffi": true, "lang": true, "lib": true, "packages": true,
	"sdk": true, "sdks": true}

// detection is what one built-in definition says about finding it.
type detection struct {
	lang                                   string
	detect, sources, unless, shared, names []string
	script                                 string
}

// detectionDefinitions are the built-in definitions' detection keys, in
// name order (manage.py's _detection_definitions: the project's own
// definitions don't take part).
func detectionDefinitions(home Home) ([]detection, error) {
	entries, err := os.ReadDir(home.Languages)
	if err != nil {
		return nil, unsupported("%v", err)
	}
	var defs []detection
	for _, entry := range entries {
		lang := entry.Name()
		path := pyJoin(home.Languages, lang, "config.json")
		if !isFile(path) {
			continue
		}
		value, err := readJSON(path)
		if err != nil {
			return nil, err
		}
		object, ok := value.(*Object)
		if !ok {
			return nil, unsupported("%s is not an object", path)
		}
		d := detection{lang: lang}
		for key, target := range map[string]*[]string{"detect": &d.detect, "detectSources": &d.sources,
			"detectUnless": &d.unless, "sharedBuildFiles": &d.shared, "dirNames": &d.names} {
			if raw, present := object.Get(key); present {
				if *target, err = toStrings(raw, lang+" "+key); err != nil {
					return nil, err
				}
			}
		}
		if raw, present := object.Get("detectScript"); present && truthy(raw) {
			if d.script, ok = raw.(string); !ok {
				return nil, unsupported("%s detectScript is not a string", lang)
			}
		}
		names := []string{lang}
		for _, name := range d.names {
			if name != lang {
				names = append(names, name)
			}
		}
		d.names = names
		defs = append(defs, d)
	}
	return defs, nil
}

// TrackedFiles are git ls-files' lines in root (paths git quotes stay
// quoted, as manage.py reads them); none outside a repository.
func TrackedFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}
	if !utf8.Valid(out) {
		return nil, unsupported("git ls-files printed text that isn't UTF-8")
	}
	return pySplitLines(pyText(out)), nil
}

func isSkipped(directory string) bool {
	for _, part := range strings.Split(directory, "/") {
		if skippedDirs[part] {
			return true
		}
	}
	return false
}

func under(directory, parent string) bool {
	return directory == parent || strings.HasPrefix(directory, parent+"/")
}

// dirSet is a set of directories.
type dirSet map[string]bool

func (s dirSet) sorted() []string {
	list := make([]string, 0, len(s))
	for d := range s {
		list = append(list, d)
	}
	sort.Strings(list)
	return list
}

// namesByDir are the file names in each directory.
type namesByDir map[string]map[string]bool

// splitByDir are the file names in each directory: outside skippedDirs,
// and inside.
func splitByDir(files []string) (byDir, skipped namesByDir) {
	byDir, skipped = namesByDir{}, namesByDir{}
	for _, path := range files {
		directory := pyDirname(path)
		if directory == "" {
			directory = "."
		}
		tree := byDir
		if isSkipped(directory) {
			tree = skipped
		}
		if tree[directory] == nil {
			tree[directory] = map[string]bool{}
		}
		_, name := pySplit(path)
		tree[directory][name] = true
	}
	return byDir, skipped
}

// holdsLanguage is whether a directory holds a language's code: one of
// its build or source files (detect, detectSources) in it or under it, or,
// for a language without detectSources, which can't tell its sources, any
// file when the directory sits at the root or under a preferredParents
// directory (Thrift's lib/go, not a languages/go/ holding Go's definition).
// A build file deeper than the hooks look for one (hookSearchDepth levels
// below) doesn't count, and with only such build files the directory
// doesn't hold the language: its hooks would find no build (antlr4's
// runtime/Go/antlr/v4/go.mod under the root).
func holdsLanguage(byDir namesByDir, defs []detection) func(lang, directory string) (bool, error) {
	byLang := map[string]detection{}
	for _, d := range defs {
		byLang[d.lang] = d
	}
	return func(lang, directory string) (bool, error) {
		d := byLang[lang]
		inside, nearBuild, deepBuild := false, false, false
		for sub, names := range byDir {
			if under(sub, directory) {
				inside = true
				if matched, err := anyMatch(names, d.sources); err != nil || matched {
					return matched, err
				}
			}
			if directory != "." && !under(sub, directory) {
				continue
			}
			matched, err := anyMatch(names, d.detect)
			if err != nil {
				return false, err
			}
			if matched && depthBelow(sub, directory) <= hookSearchDepth {
				nearBuild = true
			}
			deepBuild = deepBuild || matched
		}
		if nearBuild {
			return true, nil
		}
		return !deepBuild && len(d.sources) == 0 && inside && preference(directory)[0] == 0, nil
	}
}

// hookSearchDepth is how many levels below a directory the default hooks
// look for a build file (dirs_below_with in languages/_hooks.sh).
const hookSearchDepth = 3

// allDeeper is whether every directory is deeper below the root than the
// hooks look.
func allDeeper(dirs dirSet) bool {
	for directory := range dirs {
		if depthBelow(directory, ".") <= hookSearchDepth {
			return false
		}
	}
	return true
}

func depthBelow(sub, directory string) int {
	depth := 0
	if sub != "." {
		depth = len(strings.Split(sub, "/"))
	}
	if directory != "." {
		depth -= len(strings.Split(directory, "/"))
	}
	return depth
}

func anyMatch(names map[string]bool, patterns []string) (bool, error) {
	for name := range names {
		for _, pattern := range patterns {
			matched, err := fnmatch(name, pattern)
			if err != nil || matched {
				return matched, err
			}
		}
	}
	return false, nil
}

// hits are the directories with a name matching patterns and none matching
// unless.
func hits(dirs namesByDir, patterns, unless []string) (dirSet, error) {
	found := dirSet{}
	for directory, names := range dirs {
		matched, err := anyMatch(names, patterns)
		if err != nil {
			return nil, err
		}
		if !matched {
			continue
		}
		excluded, err := anyMatch(names, unless)
		if err != nil {
			return nil, err
		}
		if !excluded {
			found[directory] = true
		}
	}
	return found, nil
}

func countMatching(names map[string]bool, patterns []string) (int, error) {
	n := 0
	for name := range names {
		for _, pattern := range patterns {
			matched, err := fnmatch(name, pattern)
			if err != nil {
				return 0, err
			}
			if matched {
				n++
				break
			}
		}
	}
	return n, nil
}

// Detected is what detection found: each language's directories, and the
// languages left out with why.
type Detected struct {
	Found    map[string]dirSet
	NotAdded map[string]string
}

// DetectLanguages is manage.py's detect_languages over the tracked files.
func DetectLanguages(root string, defs []detection, files []string) (*Detected, error) {
	byDir, skipped := splitByDir(files)

	// A shared build file goes to the language with the most sources under
	// it (libphonenumber's cpp/CMakeLists.txt is C++'s, not its few C
	// files').
	sharedCounts := map[string]map[string]int{}
	for _, d := range defs {
		directories, err := hits(byDir, d.shared, nil)
		if err != nil {
			return nil, err
		}
		for directory := range directories {
			n, err := sharedSources(byDir, directory, d.sources)
			if err != nil {
				return nil, err
			}
			if n > 0 {
				if sharedCounts[directory] == nil {
					sharedCounts[directory] = map[string]int{}
				}
				sharedCounts[directory][d.lang] = n
			}
		}
	}
	sharedHits := func(lang string) dirSet {
		result := dirSet{}
		for directory, counts := range sharedCounts {
			most := 0
			for _, n := range counts {
				most = max(most, n)
			}
			if n, ok := counts[lang]; ok && n == most {
				result[directory] = true
			}
		}
		return result
	}

	result := &Detected{Found: map[string]dirSet{}, NotAdded: map[string]string{}}
	fromSources := map[string]bool{}
	for _, d := range defs {
		directories, err := hits(byDir, d.detect, d.unless)
		if err != nil {
			return nil, err
		}
		for directory := range sharedHits(d.lang) {
			directories[directory] = true
		}
		if d.script != "" && len(directories) > 0 {
			for directory := range directories {
				has, err := hasNpmScript(root, directory, d.script)
				if err != nil {
					return nil, err
				}
				if !has {
					delete(directories, directory)
				}
			}
			if len(directories) == 0 {
				result.NotAdded[d.lang] = "no " + d.script + " script"
				continue
			}
		}
		if len(directories) == 0 && len(d.sources) > 0 {
			if directories, err = hits(byDir, d.sources, d.unless); err != nil {
				return nil, err
			}
			fromSources[d.lang] = true
		}
		if len(directories) > 0 {
			result.Found[d.lang] = directories
			continue
		}
		inSkipped, err := hits(skipped, append(append([]string(nil), d.detect...), d.sources...), d.unless)
		if err != nil {
			return nil, err
		}
		if len(inSkipped) > 0 {
			result.NotAdded[d.lang] = "found only in examples, tests or vendored code"
		}
	}

	// A language found only by its sources keeps the directories outside
	// every other language's, and isn't added when there are none.
	names := map[string][]string{}
	for _, d := range defs {
		names[d.lang] = d.names
	}
	claims := map[string]dirSet{}
	for lang, dirs := range result.Found {
		claimed := dirSet{}
		for c := range languageRoots(dirs, names[lang]) {
			claimed[c] = true
		}
		if !fromSources[lang] {
			for c := range dirs {
				claimed[c] = true
			}
		}
		delete(claimed, ".")
		claims[lang] = claimed
	}
	var langs []string
	for lang := range fromSources {
		if result.Found[lang] != nil {
			langs = append(langs, lang)
		}
	}
	sort.Strings(langs)
	for _, lang := range langs {
		own, owners := dirSet{}, dirSet{}
		for directory := range result.Found[lang] {
			ownedBy := false
			for other, claimed := range claims {
				if other == lang {
					continue
				}
				for c := range claimed {
					if under(directory, c) {
						owners[other] = true
						ownedBy = true
						break
					}
				}
			}
			if !ownedBy {
				own[directory] = true
			}
		}
		if len(own) > 0 {
			result.Found[lang] = own
		} else {
			result.NotAdded[lang] = "its files are all inside " + strings.Join(owners.sorted(), ", ") + "'s directories"
			delete(result.Found, lang)
		}
	}
	return result, nil
}

// sharedSources counts the language's source files in a directory and
// under its src/ and include/.
func sharedSources(byDir namesByDir, directory string, sources []string) (int, error) {
	n, err := countMatching(byDir[directory], sources)
	if err != nil {
		return 0, err
	}
	var nested []string
	for _, sub := range []string{"src", "include"} {
		nested = append(nested, cleanSlashPath(pyJoin(directory, sub)))
	}
	for d, names := range byDir {
		if under(d, nested[0]) || under(d, nested[1]) {
			more, err := countMatching(names, sources)
			if err != nil {
				return 0, err
			}
			n += more
		}
	}
	return n, nil
}

// hasNpmScript is whether the directory's package.json has the npm script;
// one that can't be read counts as having it.
func hasNpmScript(root, directory, script string) (bool, error) {
	data, err := os.ReadFile(pyJoin(root, directory, "package.json"))
	if err != nil {
		return true, nil
	}
	if !utf8.Valid(data) {
		return true, nil // UnicodeDecodeError is a ValueError
	}
	document, ok, err := pyJSONLoads(pyText(data)) // json.load reads in text mode

	if err != nil {
		return false, err
	}
	object, isObject := document.(*Object)
	if !ok || !isObject {
		return true, nil
	}
	scripts, _ := object.Get("scripts")
	if !truthy(scripts) {
		return false, nil // {} has no script
	}
	scriptObject, isObject := scripts.(*Object)
	if !isObject {
		return false, nil
	}
	_, has := scriptObject.Get(script)
	return has, nil
}

// languageRoots cut each directory at the first part naming the language
// (age/impl/c, not age/impl/c/src).
func languageRoots(directories dirSet, names []string) dirSet {
	roots := dirSet{}
	for directory := range directories {
		parts := strings.Split(directory, "/")
		for i, part := range parts {
			if contains(names, part) {
				roots[strings.Join(parts[:i+1], "/")] = true
				break
			}
		}
	}
	return roots
}

func asTemplate(root string) string {
	parts := strings.Split(root, "/")
	return strings.Join(append(parts[:len(parts)-1], "{lang}"), "/")
}

// preference orders layouts, lower first: under a preferred parent or at
// the root, then shallower.
func preference(path string) [2]int {
	parts := strings.Split(path, "/")
	preferred := len(parts) == 1
	for _, part := range parts[:len(parts)-1] {
		preferred = preferred || preferredParents[part]
	}
	if preferred {
		return [2]int{0, len(parts)}
	}
	return [2]int{1, len(parts)}
}

func lessPreference(a, b [2]int) bool {
	return a[0] < b[0] || a[0] == b[0] && a[1] < b[1]
}

// ownDirectory is a language's directory when none is named after it: the
// shallowest directory holding its files, or the common parent of several
// equally shallow ones.
func ownDirectory(directories dirSet) string {
	depth := -1
	for d := range directories {
		if n := len(strings.Split(d, "/")); depth < 0 || n < depth {
			depth = n
		}
	}
	var shallowest []string
	for _, d := range directories.sorted() {
		if len(strings.Split(d, "/")) == depth {
			shallowest = append(shallowest, d)
		}
	}
	common := shallowest[0]
	if len(shallowest) > 1 {
		common = commonPath(shallowest)
	}
	if common == "" {
		return "."
	}
	return common
}

// commonPath is Python's os.path.commonpath of relative paths.
func commonPath(paths []string) string {
	var split [][]string
	for _, path := range paths {
		var parts []string
		for _, part := range strings.Split(path, "/") {
			if part != "" && part != "." {
				parts = append(parts, part)
			}
		}
		split = append(split, parts)
	}
	common := split[0]
	for _, parts := range split[1:] {
		n := 0
		for n < len(common) && n < len(parts) && common[n] == parts[n] {
			n++
		}
		common = common[:n]
	}
	return strings.Join(common, "/")
}

// Layout is where a project's languages live: its components, implPath and
// per-language implPaths.
type Layout struct {
	Components []string
	ImplPath   string
	ImplPaths  map[string]string
}

// InferLayout is manage.py's infer_layout: `age/impl/go` and `date/impl/go`
// give components [age, date] at `{component}/impl/{lang}`; `go/` and
// `rust/` one component at `{lang}`; a language whose directory doesn't fit
// gets its own implPaths entry, unless the shared template names a
// directory holding the language's build or source files (holds, which may
// be nil) that no other language was found in.
func InferLayout(found map[string]dirSet, names map[string][]string,
	holds func(lang, directory string) (bool, error)) (Layout, error) {
	namesOf := func(lang string) []string {
		if n, ok := names[lang]; ok {
			return n
		}
		return []string{lang}
	}
	roots := map[string]dirSet{}
	templates := map[string]dirSet{}
	for lang, dirs := range found {
		roots[lang] = languageRoots(dirs, namesOf(lang))
		for root := range roots[lang] {
			template := asTemplate(root)
			if templates[template] == nil {
				templates[template] = dirSet{}
			}
			templates[template][lang] = true
		}
	}
	if len(templates) == 0 {
		// Every language at the root, unless its hooks would find no build
		// from there: one whose files are all deeper than they look.
		overrides := map[string]string{}
		for lang, dirs := range found {
			if holds == nil || !allDeeper(dirs) {
				continue
			}
			held, err := holds(lang, ".")
			if err != nil {
				return Layout{}, err
			}
			if !held {
				overrides[lang] = ownDirectory(dirs)
			}
		}
		return Layout{Components: []string{"."}, ImplPath: ".", ImplPaths: overrides}, nil
	}
	var split [][]string
	lengths := map[int]bool{}
	for template := range templates {
		parts := strings.Split(template, "/")
		split = append(split, parts)
		lengths[len(parts)] = true
	}
	if len(templates) > 1 && len(lengths) == 1 {
		var varying []int
		for i := range split[0] {
			values := map[string]bool{}
			for _, parts := range split {
				values[parts[i]] = true
			}
			if len(values) > 1 {
				varying = append(varying, i)
			}
		}
		if len(varying) == 1 {
			i := varying[0]
			components := dirSet{}
			for _, parts := range split {
				components[parts[i]] = true
			}
			pattern := append(append(append([]string(nil), split[0][:i]...), "{component}"), split[0][i+1:]...)
			return Layout{Components: components.sorted(), ImplPath: strings.Join(pattern, "/"), ImplPaths: map[string]string{}}, nil
		}
	}
	template := ""
	for candidate := range templates {
		if template == "" || betterTemplate(candidate, template, templates) {
			template = candidate
		}
	}
	overrides := map[string]string{}
	for lang, dirs := range found {
		others := dirSet{}
		for other, d := range found {
			if other != lang {
				for directory := range d {
					others[directory] = true
				}
			}
		}
		fits := false
		for _, name := range namesOf(lang) {
			candidate := strings.ReplaceAll(template, "{lang}", name)
			if roots[lang][candidate] {
				fits = true
			} else if holds != nil && !others[candidate] {
				held, err := holds(lang, candidate)
				if err != nil {
					return Layout{}, err
				}
				fits = fits || held
			}
		}
		if fits {
			continue
		}
		if len(roots[lang]) > 0 {
			best := ""
			for _, root := range roots[lang].sorted() {
				if best == "" || lessPreference(preference(root), preference(best)) {
					best = root
				}
			}
			overrides[lang] = best
		} else {
			overrides[lang] = ownDirectory(dirs)
		}
	}
	return Layout{Components: []string{"."}, ImplPath: template, ImplPaths: overrides}, nil
}

// betterTemplate orders templates as min(key=(preference, -languages, text)).
func betterTemplate(a, b string, templates map[string]dirSet) bool {
	pa, pb := preference(a), preference(b)
	if pa != pb {
		return lessPreference(pa, pb)
	}
	if len(templates[a]) != len(templates[b]) {
		return len(templates[a]) > len(templates[b])
	}
	return a < b
}

// DetectedConfig is manage.py's detected_config: the config.json hecatron
// init writes for a repository, its languages and layout as detected or as
// given (comma-separated; "" for detected), and the languages found but
// not added, with why.
func DetectedConfig(root string, home Home, components, implPath, languages string) (*Object, map[string]string, error) {
	defs, err := detectionDefinitions(home)
	if err != nil {
		return nil, nil, err
	}
	files, err := TrackedFiles(root)
	if err != nil {
		return nil, nil, err
	}
	detected, err := DetectLanguages(root, defs, files)
	if err != nil {
		return nil, nil, err
	}
	names := map[string][]string{}
	for _, d := range defs {
		names[d.lang] = d.names
	}
	byDir, _ := splitByDir(files)
	layout, err := InferLayout(detected.Found, names, holdsLanguage(byDir, defs))
	if err != nil {
		return nil, nil, err
	}
	chosenComponents := layout.Components
	if components != "" {
		chosenComponents = nil
		for _, c := range strings.Split(components, ",") {
			if c != "" {
				chosenComponents = append(chosenComponents, c)
			}
		}
	}
	if implPath != "" {
		layout.ImplPath, layout.ImplPaths = implPath, map[string]string{}
	}
	var chosen []string
	if languages != "" {
		chosen = strings.Split(languages, ",")
	} else {
		for lang := range detected.Found {
			chosen = append(chosen, lang)
		}
	}
	sort.Strings(chosen)
	config := &Object{Values: map[string]any{}}
	config.set("schema", json.Number("1"))
	config.set("hecatron", ">="+home.Version())
	config.set("project", pyBasename(root))
	if len(chosenComponents) != 1 || chosenComponents[0] != "." {
		config.set("components", stringsToAny(chosenComponents))
	}
	config.set("implPath", layout.ImplPath)
	config.set("languages", stringsToAny(chosen))
	config.set("tools", &Object{Values: map[string]any{}})
	config.set("generated", &Object{Values: map[string]any{}})
	config.set("logs", ".logs")
	implPaths := &Object{Values: map[string]any{}}
	var withPaths []string
	for lang := range layout.ImplPaths {
		withPaths = append(withPaths, lang)
	}
	sort.Strings(withPaths)
	for _, lang := range withPaths {
		if contains(chosen, lang) {
			implPaths.set(lang, layout.ImplPaths[lang])
		}
	}
	if len(implPaths.Keys) > 0 {
		config.set("implPaths", implPaths)
	}
	return config, detected.NotAdded, nil
}
